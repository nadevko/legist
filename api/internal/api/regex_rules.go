package api

import (
	"net/http"
	"regexp"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

type regexWeightRuleCreateRequest struct {
	Regex   string  `json:"regex"`
	Enabled bool    `json:"enabled"`
	Weight  float64 `json:"weight"`
}

type regexWeightRulePatchRequest struct {
	Regex   *string  `json:"regex,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
	Weight  *float64 `json:"weight,omitempty"`
}

type regexWeightReplaceRequest struct {
	Rules []regexWeightRuleCreateRequest `json:"rules"`
}

type regexWeightRuleResponse struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Regex   string  `json:"regex"`
	Enabled bool    `json:"enabled"`
	Weight  float64 `json:"weight"`
	Created int64   `json:"created"`
}

type regexOmitRuleCreateRequest struct {
	Regex   string `json:"regex"`
	Enabled bool   `json:"enabled"`
}

type regexOmitRulePatchRequest struct {
	Regex   *string `json:"regex,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type regexOmitReplaceRequest struct {
	Rules []regexOmitRuleCreateRequest `json:"rules"`
}

type regexOmitRuleResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Regex   string `json:"regex"`
	Enabled bool   `json:"enabled"`
	Created int64  `json:"created"`
}

func validateRegexString(pattern string) error {
	if pattern == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "regex is required", "regex")
	}
	_, err := regexp.Compile(pattern)
	if err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "invalid regexp", "regex")
	}
	return nil
}

// --- weights ---

// handleListWeightRegexRules godoc
// @Summary     List regex weight rules
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} listResponse[regexWeightRuleResponse]
// @Router      /regex/weights [get]
func (s *Server) handleListWeightRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may read/modify regex rules", "")
	}
	rules, err := s.regexRules.ListWeightRules()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	out := make([]regexWeightRuleResponse, len(rules))
	for i, r := range rules {
		out[i] = regexWeightRuleResponse{
			ID:      r.ID,
			Object:  "regex.weight.rule",
			Regex:   r.Regex,
			Enabled: r.Enabled,
			Weight:  r.Weight,
			Created: toUnix(r.CreatedAt),
		}
	}
	return c.JSON(http.StatusOK, listResponse[regexWeightRuleResponse]{
		Object:  "list",
		Data:    out,
		HasMore: false,
	})
}

// handleReplaceWeightRegexRules godoc
// @Summary     Replace all regex weight rules
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body  body  regexWeightReplaceRequest  true "Rules payload"
// @Success     200 {object} listResponse[regexWeightRuleResponse]
// @Failure     400 {object} apiErrorResponse
// @Router      /regex/weights [put]
func (s *Server) handleReplaceWeightRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}

	var req regexWeightReplaceRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}

	rules := make([]store.RegexWeightRuleCreate, 0, len(req.Rules))
	for _, r := range req.Rules {
		if err := validateRegexString(r.Regex); err != nil {
			return err
		}
		if r.Weight < 0 {
			return errorf(http.StatusBadRequest, "invalid_parameter_value", "weight must be >= 0", "weight")
		}
		rules = append(rules, store.RegexWeightRuleCreate{
			Regex:   r.Regex,
			Enabled: r.Enabled,
			Weight:  r.Weight,
		})
	}

	if err := s.regexRules.ReplaceWeightRules(rules, nil); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	updated, err := s.regexRules.ListWeightRules()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	out := make([]regexWeightRuleResponse, len(updated))
	for i, rr := range updated {
		out[i] = regexWeightRuleResponse{
			ID:      rr.ID,
			Object:  "regex.weight.rule",
			Regex:   rr.Regex,
			Enabled: rr.Enabled,
			Weight:  rr.Weight,
			Created: toUnix(rr.CreatedAt),
		}
	}
	return c.JSON(http.StatusOK, listResponse[regexWeightRuleResponse]{
		Object:  "list",
		Data:    out,
		HasMore: false,
	})
}

// handleResetWeightRegexRules godoc
// @Summary     Reset all regex weight rules to template
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} listResponse[regexWeightRuleResponse]
// @Router      /regex/weights [delete]
func (s *Server) handleResetWeightRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	if err := s.regexRules.ResetWeightRulesFromTemplate(s.cfg.WeightRegexFile); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "failed to reset weight rules", "")
	}
	return s.handleListWeightRegexRules(c)
}

// handleCreateWeightRegexRule godoc
// @Summary     Create regex weight rule
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body  body  regexWeightRuleCreateRequest  true "Rule payload"
// @Success     201 {object} regexWeightRuleResponse
// @Failure     400 {object} apiErrorResponse
// @Router      /regex/weights [post]
func (s *Server) handleCreateWeightRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}

	var req regexWeightRuleCreateRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if err := validateRegexString(req.Regex); err != nil {
		return err
	}
	if req.Weight < 0 {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "weight must be >= 0", "weight")
	}

	id := newID("wrule")
	if err := s.regexRules.CreateWeightRule(id, store.RegexWeightRuleCreate{
		Regex:   req.Regex,
		Enabled: req.Enabled,
		Weight:  req.Weight,
	}); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	rule, err := s.regexRules.GetWeightRule(id)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusCreated, regexWeightRuleResponse{
		ID:      rule.ID,
		Object:  "regex.weight.rule",
		Regex:   rule.Regex,
		Enabled: rule.Enabled,
		Weight:  rule.Weight,
		Created: toUnix(rule.CreatedAt),
	})
}

// handleUpdateWeightRegexRule godoc
// @Summary     Update regex weight rule
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id    path   string true "Rule ID"
// @Param       body  body   regexWeightRulePatchRequest  true "Fields to update"
// @Success     200 {object} regexWeightRuleResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /regex/weights/{id} [patch]
func (s *Server) handleUpdateWeightRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	id := c.Param("id")

	var req regexWeightRulePatchRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if req.Regex == nil && req.Enabled == nil && req.Weight == nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "at least one field is required")
	}
	if req.Regex != nil {
		if err := validateRegexString(*req.Regex); err != nil {
			return err
		}
	}
	if req.Weight != nil && *req.Weight < 0 {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "weight must be >= 0", "weight")
	}

	if err := s.regexRules.UpdateWeightRule(id, store.RegexWeightRuleUpdate{
		Regex:   req.Regex,
		Enabled: req.Enabled,
		Weight:  req.Weight,
	}); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	rule, err := s.regexRules.GetWeightRule(id)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such weight regex rule: "+id)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, regexWeightRuleResponse{
		ID:      rule.ID,
		Object:  "regex.weight.rule",
		Regex:   rule.Regex,
		Enabled: rule.Enabled,
		Weight:  rule.Weight,
		Created: toUnix(rule.CreatedAt),
	})
}

// handleDeleteWeightRegexRule godoc
// @Summary     Delete regex weight rule
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Param       id    path   string true "Rule ID"
// @Success     200 {object} deletedResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /regex/weights/{id} [delete]
func (s *Server) handleDeleteWeightRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	id := c.Param("id")

	if _, err := s.regexRules.GetWeightRule(id); err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such weight regex rule: "+id)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	if err := s.regexRules.DeleteWeightRule(id); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "regex.weight.rule"))
}

// --- omits ---

// handleListOmitRegexRules godoc
// @Summary     List regex omit rules
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} listResponse[regexOmitRuleResponse]
// @Router      /regex/omits [get]
func (s *Server) handleListOmitRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may read/modify regex rules", "")
	}
	rules, err := s.regexRules.ListOmitRules()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	out := make([]regexOmitRuleResponse, len(rules))
	for i, r := range rules {
		out[i] = regexOmitRuleResponse{
			ID:      r.ID,
			Object:  "regex.omit.rule",
			Regex:   r.Regex,
			Enabled: r.Enabled,
			Created: toUnix(r.CreatedAt),
		}
	}
	return c.JSON(http.StatusOK, listResponse[regexOmitRuleResponse]{
		Object:  "list",
		Data:    out,
		HasMore: false,
	})
}

// handleReplaceOmitRegexRules godoc
// @Summary     Replace all regex omit rules
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body  body  regexOmitReplaceRequest  true "Rules payload"
// @Success     200 {object} listResponse[regexOmitRuleResponse]
// @Failure     400 {object} apiErrorResponse
// @Router      /regex/omits [put]
func (s *Server) handleReplaceOmitRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	var req regexOmitReplaceRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	rules := make([]store.RegexOmitRuleCreate, 0, len(req.Rules))
	for _, r := range req.Rules {
		if err := validateRegexString(r.Regex); err != nil {
			return err
		}
		rules = append(rules, store.RegexOmitRuleCreate{
			Regex:   r.Regex,
			Enabled: r.Enabled,
		})
	}
	if err := s.regexRules.ReplaceOmitRules(rules, nil); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	updated, err := s.regexRules.ListOmitRules()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	out := make([]regexOmitRuleResponse, len(updated))
	for i, rr := range updated {
		out[i] = regexOmitRuleResponse{
			ID:      rr.ID,
			Object:  "regex.omit.rule",
			Regex:   rr.Regex,
			Enabled: rr.Enabled,
			Created: toUnix(rr.CreatedAt),
		}
	}
	return c.JSON(http.StatusOK, listResponse[regexOmitRuleResponse]{
		Object:  "list",
		Data:    out,
		HasMore: false,
	})
}

// handleResetOmitRegexRules godoc
// @Summary     Reset all regex omit rules to template
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} listResponse[regexOmitRuleResponse]
// @Router      /regex/omits [delete]
func (s *Server) handleResetOmitRegexRules(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	if err := s.regexRules.ResetOmitRulesFromTemplate(s.cfg.DiffMatchRegexFile); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "failed to reset omit rules", "")
	}
	return s.handleListOmitRegexRules(c)
}

// handleCreateOmitRegexRule godoc
// @Summary     Create regex omit rule
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body  body  regexOmitRuleCreateRequest  true "Rule payload"
// @Success     201 {object} regexOmitRuleResponse
// @Failure     400 {object} apiErrorResponse
// @Router      /regex/omits [post]
func (s *Server) handleCreateOmitRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	var req regexOmitRuleCreateRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if err := validateRegexString(req.Regex); err != nil {
		return err
	}
	id := newID("orule")
	if err := s.regexRules.CreateOmitRule(id, store.RegexOmitRuleCreate{
		Regex:   req.Regex,
		Enabled: req.Enabled,
	}); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	rule, err := s.regexRules.GetOmitRule(id)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusCreated, regexOmitRuleResponse{
		ID:      rule.ID,
		Object:  "regex.omit.rule",
		Regex:   rule.Regex,
		Enabled: rule.Enabled,
		Created: toUnix(rule.CreatedAt),
	})
}

// handleUpdateOmitRegexRule godoc
// @Summary     Update regex omit rule
// @Tags        System
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id    path   string true "Rule ID"
// @Param       body  body   regexOmitRulePatchRequest  true "Fields to update"
// @Success     200 {object} regexOmitRuleResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /regex/omits/{id} [patch]
func (s *Server) handleUpdateOmitRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	id := c.Param("id")

	var req regexOmitRulePatchRequest
	if err := c.Bind(&req); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if req.Regex == nil && req.Enabled == nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "at least one field is required")
	}
	if req.Regex != nil {
		if err := validateRegexString(*req.Regex); err != nil {
			return err
		}
	}
	if err := s.regexRules.UpdateOmitRule(id, store.RegexOmitRuleUpdate{
		Regex:   req.Regex,
		Enabled: req.Enabled,
	}); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	rule, err := s.regexRules.GetOmitRule(id)
	if err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such omit regex rule: "+id)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, regexOmitRuleResponse{
		ID:      rule.ID,
		Object:  "regex.omit.rule",
		Regex:   rule.Regex,
		Enabled: rule.Enabled,
		Created: toUnix(rule.CreatedAt),
	})
}

// handleDeleteOmitRegexRule godoc
// @Summary     Delete regex omit rule
// @Tags        System
// @Security    BearerAuth
// @Produce     json
// @Param       id    path   string true "Rule ID"
// @Success     200 {object} deletedResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /regex/omits/{id} [delete]
func (s *Server) handleDeleteOmitRegexRule(c echo.Context) error {
	if !auth.IsAdmin(c) {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", "only admins may modify regex rules", "")
	}
	id := c.Param("id")

	if _, err := s.regexRules.GetOmitRule(id); err != nil {
		if store.IsNotFound(err) {
			return errorf(http.StatusNotFound, "resource_missing", "no such omit regex rule: "+id)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	if err := s.regexRules.DeleteOmitRule(id); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "regex.omit.rule"))
}

