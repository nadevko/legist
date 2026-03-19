package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/webhook"
)

type webhookEndpointRequest struct {
	URL    string   `json:"url"    example:"https://example.com/webhook"`
	Events []string `json:"events" example:"[\"file.created\",\"diff.done\"]"`
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
// @Failure     500 {object} apiErrorResponse
// @Router      /webhooks [post]
func (s *Server) handleCreateWebhook(c echo.Context) error {
	var body webhookEndpointRequest
	if err := c.Bind(&body); err != nil || body.URL == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "url is required", "url")
	}
	if len(body.Events) == 0 {
		return errorf(http.StatusBadRequest, "parameter_missing", "events is required", "events")
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
// @Failure     500 {object} apiErrorResponse
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
	return c.JSON(http.StatusOK, listResult(endpoints, p.Limit, toWebhookResponse, func(e store.WebhookEndpoint) string { return e.ID }))
}

// handleGetWebhook godoc
// @Summary     Get webhook endpoint
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id path string true "Webhook endpoint ID"
// @Produce     json
// @Success     200 {object} webhookEndpointResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /webhooks/{id} [get]
func (s *Server) handleGetWebhook(c echo.Context) error {
	id := c.Param("id")
	ep, err := s.webhooks.GetEndpoint(id, auth.UserID(c))
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such webhook endpoint: "+id)
	}
	return c.JSON(http.StatusOK, toWebhookResponse(*ep))
}

// handleUpdateWebhook godoc
// @Summary     Update webhook endpoint
// @Tags        webhooks
// @Security    BearerAuth
// @Param       id              path   string                 true  "Webhook endpoint ID"
// @Param       body            body   webhookEndpointRequest true  "Endpoint config"
// @Param       Idempotency-Key header string                 false "Idempotency key"
// @Accept      json
// @Produce     json
// @Success     200 {object} webhookEndpointResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /webhooks/{id} [patch]
func (s *Server) handleUpdateWebhook(c echo.Context) error {
	id := c.Param("id")
	ep, err := s.webhooks.GetEndpoint(id, auth.UserID(c))
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such webhook endpoint: "+id)
	}

	var body webhookEndpointRequest
	if err = c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if body.URL != "" {
		ep.URL = body.URL
	}
	if len(body.Events) > 0 {
		ep.Events = body.Events
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
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /webhooks/{id} [delete]
func (s *Server) handleDeleteWebhook(c echo.Context) error {
	id := c.Param("id")
	if _, err := s.webhooks.GetEndpoint(id, auth.UserID(c)); err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such webhook endpoint: "+id)
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
// @Param       limit          query  int    false "Limit"
// @Param       starting_after query  string false "Cursor"
// @Produce     json
// @Success     200 {object} listResponse[webhookEventResponse]
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /webhooks/{id}/events [get]
func (s *Server) handleListWebhookEvents(c echo.Context) error {
	id := c.Param("id")
	if _, err := s.webhooks.GetEndpoint(id, auth.UserID(c)); err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such webhook endpoint: "+id)
	}

	p, err := bindListParams(c)
	if err != nil {
		return err
	}

	events, err := s.webhooks.ListEvents(id, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusOK, listResult(events, p.Limit, func(e store.WebhookEvent) webhookEventResponse {
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

// AllEvents возвращает все поддерживаемые типы событий.
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
