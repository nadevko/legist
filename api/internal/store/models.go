package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// --- sentinel errors ---

// ErrDuplicate is returned when a unique constraint is violated.
var ErrDuplicate = errors.New("duplicate")

// ErrNotOwner is returned when a mutation targets a row that belongs to another user.
var ErrNotOwner = errors.New("not owner")

// IsUniqueViolation detects SQLite UNIQUE constraint errors without CGO.
// modernc.org/sqlite returns errors whose message contains "UNIQUE constraint failed".
func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// isUniqueViolation is the package-internal alias.
var isUniqueViolation = IsUniqueViolation

// IsNotFound reports whether err is a sql.ErrNoRows (record not found).
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// --- models ---

// User roles (stored in users.role, echoed in JWT).
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

type User struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	Password  string    `db:"password"`
	Role      string    `db:"role"`
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

// Document is the AKN Work-level entity grouping all versions of one НПА.
// UserID is a pointer because public laws (user_id IS NULL) are valid documents.
type Document struct {
	ID     string  `db:"id"`
	UserID *string `db:"user_id"`

	Subtype  string `db:"subtype"`
	Number   string `db:"number"`
	Author   string `db:"author"`
	Date     string `db:"date"`
	Country  string `db:"country"`
	Name     string `db:"name"`
	NPALevel int    `db:"npa_level"`

	// RAG enrichment (Work-level): JSON arrays stored as TEXT in SQLite.
	RagTags       string `db:"rag_tags"`
	RagCategories string `db:"rag_categories"`
	RagKeywords   string `db:"rag_keywords"`
	RagSummary    string `db:"rag_summary"`
	Jurisdiction  string `db:"jurisdiction"`
	ContractType  string `db:"contract_type"`

	CreatedAt time.Time `db:"created_at"`
}

func (d *Document) IsComplete() bool {
	return d.Subtype != "" && d.Number != "" && d.Author != "" && d.Date != ""
}

func (d *Document) MissingFields() []string {
	var m []string
	if d.Subtype == "" {
		m = append(m, "subtype")
	}
	if d.Number == "" {
		m = append(m, "number")
	}
	if d.Author == "" {
		m = append(m, "author")
	}
	if d.Date == "" {
		m = append(m, "date")
	}
	return m
}

// OwnerID returns the user ID string, or "" for public documents.
func (d *Document) OwnerID() string {
	if d.UserID == nil {
		return ""
	}
	return *d.UserID
}

// File is one physical version of a Document.
type File struct {
	ID         string  `db:"id"`
	UserID     *string `db:"user_id"`
	DocumentID *string `db:"document_id"`

	Name     string `db:"name"`
	MimeType string `db:"mime_type"`
	Size     int64  `db:"size"`
	Path     string `db:"path"`
	Status   string `db:"status"`

	VersionDate   *string `db:"version_date"`
	VersionNumber *string `db:"version_number"`
	VersionLabel  *string `db:"version_label"`
	Language      *string `db:"language"`

	PubName   *string `db:"pub_name"`
	PubDate   *string `db:"pub_date"`
	PubNumber *string `db:"pub_number"`

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
	Events    []string  `db:"-"`
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

// Diff compares two file versions of the same document.
// UserID is nil for diffs on public documents (future); owned diffs use the authenticated user.
type Diff struct {
	ID          string  `db:"id"`
	UserID      *string `db:"user_id"`
	DocumentID  string  `db:"document_id"`
	LeftFileID  string  `db:"left_file_id"`
	RightFileID string  `db:"right_file_id"`

	Status string `db:"status"` // pending | processing | done | failed

	SimilarityPercent *float64 `db:"similarity_percent"`
	DiffData          DiffData `db:"diff_data"`

	CreatedAt time.Time `db:"created_at"`
}
