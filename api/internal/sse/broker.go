package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

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

// Publish отправляет событие всем подписчикам ключа.
func (b *Broker) Publish(key string, evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs[key] {
		select {
		case ch <- evt:
		default:
		}
	}
}

// Stream читает события из брокера по ключу и пишет их в SSE-соединение.
func Stream(c echo.Context, b *Broker, key string) error {
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
			if evt.Type == "done" || evt.Type == "failed" {
				return nil
			}
		}
	}
}
func Format(evt Event) (string, error) {
	data, err := json.Marshal(evt.Data)
	if err != nil {
		return "", fmt.Errorf("marshal sse event: %w", err)
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data), nil
}
