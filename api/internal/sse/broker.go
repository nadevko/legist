package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// Event — одно SSE событие.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Broker раздаёт события подписчикам по ключу (job_id, file_id и т.д.).
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]chan Event
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[string][]chan Event)}
}

// Subscribe возвращает канал событий для ключа.
// Вызывающий должен вызвать unsubscribe когда закончит.
func (b *Broker) Subscribe(key string) (ch chan Event, unsubscribe func()) {
	ch = make(chan Event, 16)
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], ch)
	b.mu.Unlock()

	unsubscribe = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[key]
		for i, s := range subs {
			if s == ch {
				b.subs[key] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(b.subs[key]) == 0 {
			delete(b.subs, key)
		}
	}
	return ch, unsubscribe
}

func (b *Broker) Publish(key string, evt Event) {
	b.mu.RLock()
	subs := b.subs[key]
	b.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, ch := range subs {
		select {
		case ch <- evt:
		case <-ctx.Done():
			return
		}
	}
}

// Stream streams SSE events for key until one of terminalTypes is received.
// If terminalTypes is empty, defaults to "done" and "failed".
func Stream(c echo.Context, b *Broker, key string, terminalTypes ...string) error {
	term := terminalTypes
	if len(term) == 0 {
		term = []string{"done", "failed"}
	}
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	ch, unsub := b.Subscribe(key)
	defer unsub()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			msg, err := Format(evt)
			if err != nil {
				return nil
			}
			if _, err = fmt.Fprint(c.Response(), msg); err != nil {
				return nil
			}
			c.Response().Flush()
			for _, t := range term {
				if evt.Type == t {
					return nil
				}
			}
		}
	}
}

// StreamWithInitialEvent streams SSE events from broker (by key) and guarantees that
// the `initial` event is written to the response right after subscribing and before
// reading from broker channels.
//
// If `initialType` is present in `terminalTypes`, the function returns immediately
// after writing the initial event (useful for "lazy upload" where we only want to
// report successful upload and close the stream).
func StreamWithInitialEvent(
	c echo.Context,
	b *Broker,
	key string,
	initialType string,
	initialData any,
	afterSubscribe func(),
	terminalTypes ...string,
) error {
	term := terminalTypes
	if len(term) == 0 {
		term = []string{"done", "failed"}
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	ch, unsub := b.Subscribe(key)
	defer unsub()

	// Write initial event before starting to forward events from broker.
	if initialType != "" {
		msg, err := Format(Event{Type: initialType, Data: initialData})
		if err != nil {
			return fmt.Errorf("format initial sse event: %w", err)
		}
		if _, err = fmt.Fprint(c.Response(), msg); err != nil {
			return err
		}
		c.Response().Flush()
	}

	shouldTerminate := false
	for _, t := range term {
		if initialType != "" && initialType == t {
			shouldTerminate = true
			break
		}
	}
	if shouldTerminate {
		return nil
	}

	if afterSubscribe != nil {
		afterSubscribe()
	}

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, ok := <-ch:
			if !ok {
				return nil
			}
			msg, err := Format(evt)
			if err != nil {
				return fmt.Errorf("format sse event: %w", err)
			}
			if _, err = fmt.Fprint(c.Response(), msg); err != nil {
				return err
			}
			c.Response().Flush()
			for _, t := range term {
				if evt.Type == t {
					return nil
				}
			}
		}
	}
}
func Format(evt Event) (string, error) {
	payload := struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}{
		Type: evt.Type,
		Data: evt.Data,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal sse event: %w", err)
	}
	// Stripe-style SSE: fixed `event: message` and JSON payload includes `type`.
	return fmt.Sprintf("event: message\ndata: %s\n\n", data), nil
}
