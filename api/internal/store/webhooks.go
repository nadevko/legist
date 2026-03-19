package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type WebhookStore struct{ db *sqlx.DB }

func NewWebhookStore(db *sqlx.DB) *WebhookStore { return &WebhookStore{db} }

func (s *WebhookStore) CreateEndpoint(e *WebhookEndpoint) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.NamedExec(
		`INSERT INTO webhook_endpoints (id, user_id, url, secret, enabled)
		 VALUES (:id, :user_id, :url, :secret, :enabled)`, e,
	); err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}

	if err = insertEvents(tx, e.ID, e.Events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *WebhookStore) GetEndpoint(id, userID string) (*WebhookEndpoint, error) {
	var e WebhookEndpoint
	if err := s.db.Get(&e,
		`SELECT * FROM webhook_endpoints WHERE id = ? AND user_id = ?`, id, userID,
	); err != nil {
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}
	events, err := s.getEvents(id)
	if err != nil {
		return nil, err
	}
	e.Events = events
	return &e, nil
}

func (s *WebhookStore) ListEndpoints(userID string) ([]WebhookEndpoint, error) {
	var endpoints []WebhookEndpoint
	if err := s.db.Select(&endpoints,
		`SELECT * FROM webhook_endpoints WHERE user_id = ? ORDER BY created_at DESC`, userID,
	); err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	for i := range endpoints {
		events, err := s.getEvents(endpoints[i].ID)
		if err != nil {
			return nil, err
		}
		endpoints[i].Events = events
	}
	return endpoints, nil
}

func (s *WebhookStore) UpdateEndpoint(e *WebhookEndpoint) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.Exec(
		`UPDATE webhook_endpoints SET url = ?, enabled = ? WHERE id = ? AND user_id = ?`,
		e.URL, e.Enabled, e.ID, e.UserID,
	); err != nil {
		return fmt.Errorf("update webhook endpoint: %w", err)
	}

	if _, err = tx.Exec(
		`DELETE FROM webhook_endpoint_events WHERE endpoint_id = ?`, e.ID,
	); err != nil {
		return fmt.Errorf("delete webhook events: %w", err)
	}

	if err = insertEvents(tx, e.ID, e.Events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *WebhookStore) DeleteEndpoint(id, userID string) error {
	if _, err := s.db.Exec(
		`DELETE FROM webhook_endpoints WHERE id = ? AND user_id = ?`, id, userID,
	); err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}

// ListEndpointsByEvent возвращает все включённые эндпоинты подписанные на событие.
func (s *WebhookStore) ListEndpointsByEvent(event string) ([]WebhookEndpoint, error) {
	var endpoints []WebhookEndpoint
	if err := s.db.Select(&endpoints,
		`SELECT e.* FROM webhook_endpoints e
		 JOIN webhook_endpoint_events ee ON ee.endpoint_id = e.id
		 WHERE e.enabled = 1 AND ee.event = ?`, event,
	); err != nil {
		return nil, fmt.Errorf("list endpoints by event: %w", err)
	}
	for i := range endpoints {
		events, err := s.getEvents(endpoints[i].ID)
		if err != nil {
			return nil, err
		}
		endpoints[i].Events = events
	}
	return endpoints, nil
}

func (s *WebhookStore) CreateEvent(e *WebhookEvent) error {
	_, err := s.db.NamedExec(
		`INSERT INTO webhook_events (id, endpoint_id, type, payload, status)
		 VALUES (:id, :endpoint_id, :type, :payload, :status)`, e,
	)
	if err != nil {
		return fmt.Errorf("create webhook event: %w", err)
	}
	return nil
}

func (s *WebhookStore) UpdateEventStatus(id, status string, attempts int) error {
	if _, err := s.db.Exec(
		`UPDATE webhook_events SET status = ?, attempts = ? WHERE id = ?`,
		status, attempts, id,
	); err != nil {
		return fmt.Errorf("update webhook event: %w", err)
	}
	return nil
}

func (s *WebhookStore) ListEvents(endpointID string, p pagination.Params) ([]WebhookEvent, error) {
	p.Normalize()

	q := strings.Builder{}
	args := []any{endpointID}

	q.WriteString(`SELECT * FROM webhook_events WHERE endpoint_id = ?`)

	if p.StartingAfter != "" {
		q.WriteString(` AND (created_at < (SELECT created_at FROM webhook_events WHERE id = ?)
			OR (created_at = (SELECT created_at FROM webhook_events WHERE id = ?) AND id < ?))`)
		args = append(args, p.StartingAfter, p.StartingAfter, p.StartingAfter)
	}
	if p.EndingBefore != "" {
		q.WriteString(` AND (created_at > (SELECT created_at FROM webhook_events WHERE id = ?)
			OR (created_at = (SELECT created_at FROM webhook_events WHERE id = ?) AND id > ?))`)
		args = append(args, p.EndingBefore, p.EndingBefore, p.EndingBefore)
	}

	q.WriteString(` ORDER BY created_at DESC LIMIT ?`)
	args = append(args, p.Limit+1)

	var events []WebhookEvent
	if err := s.db.Select(&events, q.String(), args...); err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	return events, nil
}

func (s *WebhookStore) ListPendingEvents() ([]WebhookEvent, error) {
	var events []WebhookEvent
	if err := s.db.Select(&events,
		`SELECT * FROM webhook_events WHERE status = 'pending' ORDER BY created_at ASC`,
	); err != nil {
		return nil, fmt.Errorf("list pending events: %w", err)
	}
	return events, nil
}

func (s *WebhookStore) getEvents(endpointID string) ([]string, error) {
	var events []string
	if err := s.db.Select(&events,
		`SELECT event FROM webhook_endpoint_events WHERE endpoint_id = ?`, endpointID,
	); err != nil {
		return nil, fmt.Errorf("get webhook events: %w", err)
	}
	return events, nil
}

func insertEvents(tx *sqlx.Tx, endpointID string, events []string) error {
	for _, event := range events {
		if _, err := tx.Exec(
			`INSERT INTO webhook_endpoint_events (endpoint_id, event) VALUES (?, ?)`,
			endpointID, event,
		); err != nil {
			return fmt.Errorf("insert webhook event %s: %w", event, err)
		}
	}
	return nil
}
