package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nadevko/legist/internal/store"
)

const (
	maxAttempts = 3
	timeout     = 10 * time.Second
)

// Event types
const (
	EventFileCreated = "file.created"
	EventFileParsed  = "file.parsed"
	EventFileFailed  = "file.failed"
	EventFileDeleted = "file.deleted"
	EventDiffCreated = "diff.created"
	EventDiffDone    = "diff.done"
	EventDiffFailed  = "diff.failed"
	EventUserCreated = "user.created"
	EventUserDeleted = "user.deleted"
)

type Dispatcher struct {
	store  *store.WebhookStore
	client *http.Client
}

func NewDispatcher(s *store.WebhookStore) *Dispatcher {
	return &Dispatcher{
		store:  s,
		client: &http.Client{Timeout: timeout},
	}
}

// Dispatch отправляет событие всем подписанным эндпоинтам.
func (d *Dispatcher) Dispatch(eventType string, payload any) {
	endpoints, err := d.store.ListEndpointsByEvent(eventType)
	if err != nil {
		log.Printf("webhook: list endpoints: %v", err)
		return
	}

	body, err := json.Marshal(map[string]any{
		"object":  "event",
		"type":    eventType,
		"created": time.Now().Unix(),
		"data":    payload,
	})
	if err != nil {
		log.Printf("webhook: marshal payload: %v", err)
		return
	}

	for _, ep := range endpoints {
		ev := &store.WebhookEvent{
			ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			EndpointID: ep.ID,
			Type:       eventType,
			Payload:    string(body),
			Status:     "pending",
		}
		if err = d.store.CreateEvent(ev); err != nil {
			log.Printf("webhook: create event: %v", err)
			continue
		}
		go d.deliver(ep, ev, body)
	}
}

func (d *Dispatcher) deliver(ep store.WebhookEndpoint, ev *store.WebhookEvent, body []byte) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := d.send(ep.URL, ep.Secret, body)
		if err == nil {
			d.store.UpdateEventStatus(ev.ID, "delivered", attempt)
			return
		}
		log.Printf("webhook: attempt %d failed for %s: %v", attempt, ep.URL, err)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt*attempt) * time.Second) // exponential backoff
		}
	}
	d.store.UpdateEventStatus(ev.ID, "failed", maxAttempts)
}

func (d *Dispatcher) send(url, secret string, body []byte) error {
	sig := sign(secret, body)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Legist-Signature", "sha256="+sig)
	req.Header.Set("Legist-Version", "v1-alpha")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// sign вычисляет HMAC-SHA256 подпись тела запроса.
func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
