package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const idempotencyTTLHours = 24

type IdempotencyStore struct{ db *sqlx.DB }

func NewIdempotencyStore(db *sqlx.DB) *IdempotencyStore { return &IdempotencyStore{db} }

func (s *IdempotencyStore) Get(key, userID string) (*IdempotencyKey, error) {
	var ik IdempotencyKey
	err := s.db.Get(&ik,
		`SELECT * FROM idempotency_keys
		 WHERE key = ? AND user_id = ?
		 AND created_at > datetime('now', ? )`,
		key, userID, fmt.Sprintf("-%d hours", idempotencyTTLHours),
	)
	if err != nil {
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return &ik, nil
}

var ErrConflict = errors.New("idempotency key in use")

func (s *IdempotencyStore) Lock(key, userID, method, path string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO idempotency_keys
		 (key, user_id, method, path, status, response)
		 VALUES (?, ?, ?, ?, 0, '')`,
		key, userID, method, path,
	)
	if err != nil {
		return fmt.Errorf("lock idempotency key: %w", err)
	}

	var ik IdempotencyKey
	if err = s.db.Get(&ik,
		`SELECT * FROM idempotency_keys WHERE key = ? AND user_id = ?`, key, userID,
	); err != nil {
		return fmt.Errorf("get locked key: %w", err)
	}

	if ik.Response != "" {
		return nil
	}
	if ik.Status == 0 && ik.Method == method && ik.Path == path {
		return nil
	}

	return ErrConflict
}

func (s *IdempotencyStore) Set(ik *IdempotencyKey) error {
	_, err := s.db.Exec(
		`UPDATE idempotency_keys SET status = ?, response = ?
		 WHERE key = ? AND user_id = ?`,
		ik.Status, ik.Response, ik.Key, ik.UserID,
	)
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}
	return nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
