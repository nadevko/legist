package store

import "time"

type User struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
}

type Session struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

type PasswordReset struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

type File struct {
	ID        string    `db:"id"`
	UserID    *string   `db:"user_id"` // nil = public law
	Name      string    `db:"name"`
	MimeType  string    `db:"mime_type"`
	Size      int64     `db:"size"`
	Path      string    `db:"path"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
}

type IdempotencyKey struct {
	Key       string    `db:"key"`
	UserID    string    `db:"user_id"`
	Method    string    `db:"method"`
	Path      string    `db:"path"`
	Status    int       `db:"status"`
	Response  string    `db:"response"`
	CreatedAt time.Time `db:"created_at"`
}

type WebhookEndpoint struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	URL       string    `db:"url"`
	Secret    string    `db:"secret"`
	Enabled   bool      `db:"enabled"`
	CreatedAt time.Time `db:"created_at"`
	Events    []string  `db:"-"` // загружается отдельно
}

type WebhookEvent struct {
	ID         string    `db:"id"`
	EndpointID string    `db:"endpoint_id"`
	Type       string    `db:"type"`
	Payload    string    `db:"payload"`
	Status     string    `db:"status"`
	Attempts   int       `db:"attempts"`
	CreatedAt  time.Time `db:"created_at"`
}
