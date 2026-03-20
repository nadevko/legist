package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT UNIQUE NOT NULL,
	password   TEXT NOT NULL,
	role       TEXT NOT NULL DEFAULT 'user',
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS sessions_token_hash ON sessions(token_hash);

CREATE TABLE IF NOT EXISTS password_resets (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS password_resets_token_hash ON password_resets(token_hash);

-- AKN Work-level entity. Groups all file versions of one НПА.
-- subtype/number/author/date may be NULL while the first file is processing.
CREATE TABLE IF NOT EXISTS documents (
	id        TEXT PRIMARY KEY,
	user_id   TEXT REFERENCES users(id) ON DELETE CASCADE,

	subtype   TEXT,
	number    TEXT,
	author    TEXT,
	date      TEXT,

	country   TEXT NOT NULL DEFAULT 'by',
	name      TEXT,
	npa_level INTEGER NOT NULL DEFAULT 8,

	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS documents_user_id ON documents(user_id);
-- Prevent duplicate documents for the same owner+identity.
-- WHERE clause allows NULLs (incomplete docs) to coexist freely.
CREATE UNIQUE INDEX IF NOT EXISTS documents_identity
	ON documents(user_id, subtype, number, date)
	WHERE subtype IS NOT NULL AND number IS NOT NULL AND date IS NOT NULL;

CREATE TABLE IF NOT EXISTS files (
	id          TEXT PRIMARY KEY,
	user_id     TEXT REFERENCES users(id) ON DELETE CASCADE,
	document_id TEXT REFERENCES documents(id) ON DELETE SET NULL,

	name      TEXT    NOT NULL,
	mime_type TEXT    NOT NULL,
	size      INTEGER NOT NULL,
	path      TEXT    NOT NULL,
	status    TEXT    NOT NULL DEFAULT 'pending',

	-- AKN FRBRExpression (all optional)
	version_date   TEXT,
	version_number TEXT,
	version_label  TEXT,
	language       TEXT,

	-- Publication (all optional)
	pub_name   TEXT,
	pub_date   TEXT,
	pub_number TEXT,

	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS files_user_id     ON files(user_id);
CREATE INDEX IF NOT EXISTS files_document_id ON files(document_id);
CREATE INDEX IF NOT EXISTS files_mime_type   ON files(mime_type);

CREATE TABLE IF NOT EXISTS idempotency_keys (
	key        TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	method     TEXT NOT NULL,
	path       TEXT NOT NULL,
	status     INTEGER NOT NULL DEFAULT 0,
	response   TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idempotency_keys_user ON idempotency_keys(user_id);

CREATE TABLE IF NOT EXISTS webhook_endpoints (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	url        TEXT NOT NULL,
	secret     TEXT NOT NULL,
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS webhook_endpoint_events (
	endpoint_id TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
	event       TEXT NOT NULL,
	PRIMARY KEY (endpoint_id, event)
);
CREATE TABLE IF NOT EXISTS webhook_events (
	id          TEXT PRIMARY KEY,
	endpoint_id TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
	type        TEXT NOT NULL,
	payload     TEXT NOT NULL,
	status      TEXT NOT NULL DEFAULT 'pending',
	attempts    INTEGER NOT NULL DEFAULT 0,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS webhook_events_endpoint ON webhook_events(endpoint_id);
CREATE INDEX IF NOT EXISTS webhook_events_status   ON webhook_events(status);

CREATE TABLE IF NOT EXISTS diffs (
	id            TEXT PRIMARY KEY,
	user_id       TEXT REFERENCES users(id) ON DELETE CASCADE,

	document_id   TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
	left_file_id  TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
	right_file_id TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,

	status        TEXT NOT NULL DEFAULT 'pending',

	similarity_percent REAL,
	diff_data          TEXT,

	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS diffs_user_id      ON diffs(user_id);
CREATE INDEX IF NOT EXISTS diffs_document_id  ON diffs(document_id);
CREATE INDEX IF NOT EXISTS diffs_left_file_id ON diffs(left_file_id);
CREATE INDEX IF NOT EXISTS diffs_right_file_id ON diffs(right_file_id);
`

func Open(path string) (*sqlx.DB, error) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err = os.Stat(dir); os.IsNotExist(err) {
			return nil, fmt.Errorf("directory %q does not exist", dir)
		}
	}
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err = db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA foreign_keys=ON;
		PRAGMA busy_timeout=5000;
		PRAGMA cache_size=-20000;
	`); err != nil {
		return nil, fmt.Errorf("apply pragmas: %w", err)
	}

	// ХАК-МИГРАЦИЯ: пытаемся добавить колонку document_id в таблицу files, если ее там еще нет.
	// Ошибку игнорируем, т.к. если базы еще не было или колонка уже добавлена — запрос вернет ошибку,
	// и это нормальное поведение.
	_, _ = db.Exec(`ALTER TABLE files ADD COLUMN document_id TEXT REFERENCES documents(id) ON DELETE SET NULL;`)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';`)

	if _, err = db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}
