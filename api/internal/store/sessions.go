package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type SessionStore struct{ db *sqlx.DB }

func NewSessionStore(db *sqlx.DB) *SessionStore { return &SessionStore{db} }

func (s *SessionStore) Create(sess *Session) error {
	_, err := s.db.NamedExec(
		`INSERT INTO sessions (id, user_id, token_hash, expires_at)
		 VALUES (:id, :user_id, :token_hash, :expires_at)`, sess,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *SessionStore) GetByTokenHash(hash string) (*Session, error) {
	var sess Session
	if err := s.db.Get(&sess,
		`SELECT * FROM sessions WHERE token_hash = ? AND expires_at > datetime('now')`, hash,
	); err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &sess, nil
}

func (s *SessionStore) Delete(userID string) error {
	if _, err := s.db.Exec(
		`DELETE FROM sessions WHERE user_id = ?`, userID,
	); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
