package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/webhook"
)

type webhookEndpointRequest struct {
	URL     string   `json:"url"     example:"https://example.com/webhook"`
	Events  []string `json:"events"  example:"[\"file.created\",\"diff.done\"]"`
	Enabled *bool    `json:"enabled"` // nil = do not change; false = disable; true = enable
}

type webhookEndpointResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // "webhook.endpoint"
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
	Created int64    `json:"created"`
}

type webhookEventResponse struct {
	ID         string `json:"id"`
	Object     string `json:"object"` // "webhook.event"
	EndpointID string `json:"endpoint_id"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	Created    int64  `json:"created"`
}

// handleCreateWebhook godoc
// @Summary     Create webhook endpoint
// @Tags        webhooks
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body            body   webhookEndpointRequest true  "Endpoint config"
// @Param       Idempotency-Key header string                 false "Idempotency key"
// @Success     201 {object} webhookEndpointResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Router      /webhooks [post]
func (s *Server) handleCreateWebhook(c echo.Context) error {
	var body webhookEndpointRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "invalid body")
	}
	if body.URL == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "url is required", "url")
	}
	if err := validateWebhookURL(body.URL); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", err.Error(), "url")
	}
	if len(body.Events) == 0 {
		return errorf(http.StatusBadRequest, "parameter_missing", "events is required", "events")
	}
	if err := validateEvents(body.Events); err != nil {
		return errorf(http.StatusBadRequest, "invalid_parameter_value", err.Error(), "events")
	}

	ep := &store.WebhookEndpoint{
		ID:      newID("whep"),
		UserID:  auth.UserID(c),
		URL:     body.URL,
		Secret:  generateSecret(),
		Events:  body.Events,
		Enabled: true,
	}
	if err := s.webhooks.CreateEndpoint(ep); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusCreated, toWebhookResponse(*ep))
}

// handleListWebhooks godoc
// @Summary     List webhook endpoints
// @Tags        webhooks
// @Security    BearerAuth
// @Produce     json
// @Param       limit          query int    false "Limit"
// @Param       starting_after query string false "Cursor"
// @Success     200 {object} listResponse[webhookEndpointResponse]
// @Failure     401 {object} apiErrorResponse
// @Router      /webhooks [get]
func (s *Server) handleListWebhooks(c echo.Context) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	endpoints, err := s.webhooks.ListEndpoints(auth.UserID(c), p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(endpoints, p.Limit,
		toWebhookResponse, func(e store.WebhookEndpoint) string { return e.ID }))
}

// handleGetWebhook godoc
// @Summary     Get webhook endpoint
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id path string true "Webhook endpoint ID"
// @Produce     json
// @Success     200 {object} webhookEndpointResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /webhooks/{id} [get]
func (s *Server) handleGetWebhook(c echo.Context) error {
	id := c.Param("id")
	ep, err := s.webhooks.GetEndpoint(id, auth.UserID(c))
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing",
			"no such webhook endpoint: "+id)
	}
	return c.JSON(http.StatusOK, toWebhookResponse(*ep))
}

// handleUpdateWebhook godoc
// @Summary     Update webhook endpoint (url, events, enabled)
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id              path   string                 true  "Webhook endpoint ID"
// @Param       body            body   webhookEndpointRequest true  "Fields to update"
// @Param       Idempotency-Key header string                 false "Idempotency key"
// @Accept      json
// @Produce     json
// @Success     200 {object} webhookEndpointResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /webhooks/{id} [patch]
func (s *Server) handleUpdateWebhook(c echo.Context) error {
	id := c.Param("id")
	ep, err := s.webhooks.GetEndpoint(id, auth.UserID(c))
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing",
			"no such webhook endpoint: "+id)
	}

	var body webhookEndpointRequest
	if err = c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}

	if body.URL != "" {
		if err = validateWebhookURL(body.URL); err != nil {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				err.Error(), "url")
		}
		ep.URL = body.URL
	}
	if len(body.Events) > 0 {
		if err = validateEvents(body.Events); err != nil {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				err.Error(), "events")
		}
		ep.Events = body.Events
	}
	if body.Enabled != nil {
		ep.Enabled = *body.Enabled
	}

	if err = s.webhooks.UpdateEndpoint(ep); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, toWebhookResponse(*ep))
}

// handleDeleteWebhook godoc
// @Summary     Delete webhook endpoint
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id              path   string true  "Webhook endpoint ID"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /webhooks/{id} [delete]
func (s *Server) handleDeleteWebhook(c echo.Context) error {
	id := c.Param("id")
	if _, err := s.webhooks.GetEndpoint(id, auth.UserID(c)); err != nil {
		return errorf(http.StatusNotFound, "resource_missing",
			"no such webhook endpoint: "+id)
	}
	if err := s.webhooks.DeleteEndpoint(id, auth.UserID(c)); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "webhook.endpoint"))
}

// handleListWebhookEvents godoc
// @Summary     List webhook delivery attempts
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id             path   string true  "Webhook endpoint ID"
// @Param       status         query  string false "pending|delivered|failed"
// @Param       limit          query  int    false "Limit"
// @Param       starting_after query  string false "Cursor"
// @Produce     json
// @Success     200 {object} listResponse[webhookEventResponse]
// @Failure     404 {object} apiErrorResponse
// @Router      /webhooks/{id}/events [get]
func (s *Server) handleListWebhookEvents(c echo.Context) error {
	id := c.Param("id")
	if _, err := s.webhooks.GetEndpoint(id, auth.UserID(c)); err != nil {
		return errorf(http.StatusNotFound, "resource_missing",
			"no such webhook endpoint: "+id)
	}
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	events, err := s.webhooks.ListEvents(id, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(events, p.Limit,
		func(e store.WebhookEvent) webhookEventResponse {
			return webhookEventResponse{
				ID:         e.ID,
				Object:     "webhook.event",
				EndpointID: e.EndpointID,
				Type:       e.Type,
				Status:     e.Status,
				Attempts:   e.Attempts,
				Created:    toUnix(e.CreatedAt),
			}
		}, func(e store.WebhookEvent) string { return e.ID }))
}

// --- helpers ---

// validEvents is the set of allowed event type strings.
var validEvents = func() map[string]struct{} {
	m := make(map[string]struct{})
	for _, e := range AllEvents() {
		m[e] = struct{}{}
	}
	return m
}()

// validateEvents returns an error if any event string is not in AllEvents().
func validateEvents(events []string) error {
	for _, e := range events {
		if _, ok := validEvents[e]; !ok {
			return fmt.Errorf("unknown event type: %q", e)
		}
	}
	return nil
}

// validateWebhookURL checks that the URL is absolute and uses http or https.
func validateWebhookURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

func toWebhookResponse(ep store.WebhookEndpoint) webhookEndpointResponse {
	events := ep.Events
	if events == nil {
		events = []string{}
	}
	return webhookEndpointResponse{
		ID:      ep.ID,
		Object:  "webhook.endpoint",
		URL:     ep.URL,
		Events:  events,
		Enabled: ep.Enabled,
		Created: toUnix(ep.CreatedAt),
	}
}

func generateSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "whsec_" + hex.EncodeToString(b)
}

// AllEvents returns all supported event types.
func AllEvents() []string {
	return []string{
		webhook.EventFileCreated,
		webhook.EventFileParsed,
		webhook.EventFileFailed,
		webhook.EventFileDeleted,
		webhook.EventDiffCreated,
		webhook.EventDiffDone,
		webhook.EventDiffFailed,
		webhook.EventUserCreated,
		webhook.EventUserDeleted,
	}
}
