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
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
	id          TEXT PRIMARY KEY,
	user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash  TEXT NOT NULL,
	expires_at  DATETIME NOT NULL,
	created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS sessions_token_hash ON sessions(token_hash);

CREATE TABLE IF NOT EXISTS password_resets (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS files (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	mime_type  TEXT NOT NULL,
	size       INTEGER NOT NULL,
	path       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'pending',
	created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS jobs (
	id         TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	type       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'pending',
	created_at DATETIME NOT NULL DEFAULT (datetime('now')),
	expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS job_files (
	job_id  TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
	file_id TEXT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
	status  TEXT NOT NULL DEFAULT 'pending',
	error   TEXT,
	PRIMARY KEY (job_id, file_id)
);

CREATE INDEX IF NOT EXISTS files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS jobs_user_id ON jobs(user_id);
`

func Open(path string) (*sqlx.DB, error) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
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

	if _, err = db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return db, nil
}
