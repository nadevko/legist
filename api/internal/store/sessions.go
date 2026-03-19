package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type SessionStore struct{ db *sqlx.DB }

func NewSessionStore(db *sqlx.DB) *SessionStore { return &SessionStore{db} }

func (s *SessionStore) Create(sess *Session) error {
	_, err := s.db.NamedExec(`
		INSERT INTO sessions (id, user_id, token_hash, expires_at)
		VALUES (:id, :user_id, :token_hash, :expires_at)`, sess)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *SessionStore) GetByTokenHash(hash string) (*Session, error) {
	var sess Session
	if err := s.db.Get(&sess,
		`SELECT * FROM sessions WHERE token_hash = ? AND expires_at > datetime('now')`,
		hash); err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return &sess, nil
}

func (s *SessionStore) ListByUser(userID string, p pagination.Params) ([]Session, error) {
	p.Normalize()

	var q strings.Builder
	args := []any{userID}

	q.WriteString(`SELECT * FROM sessions WHERE user_id = ? AND expires_at > datetime('now')`)
	if p.StartingAfter != "" {
		q.WriteString(` AND (created_at < (SELECT created_at FROM sessions WHERE id = ?)
			OR (created_at = (SELECT created_at FROM sessions WHERE id = ?) AND id < ?))`)
		args = append(args, p.StartingAfter, p.StartingAfter, p.StartingAfter)
	}
	if p.EndingBefore != "" {
		q.WriteString(` AND (created_at > (SELECT created_at FROM sessions WHERE id = ?)
			OR (created_at = (SELECT created_at FROM sessions WHERE id = ?) AND id > ?))`)
		args = append(args, p.EndingBefore, p.EndingBefore, p.EndingBefore)
	}
	q.WriteString(` ORDER BY created_at DESC LIMIT ?`)
	args = append(args, p.Limit+1)

	var sessions []Session
	if err := s.db.Select(&sessions, q.String(), args...); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

// DeleteByID removes the session only if it belongs to userID.
// Returns ErrNotOwner if the session exists but belongs to another user,
// or if it simply does not exist — caller returns 404 in both cases
// to avoid leaking session existence.
func (s *SessionStore) DeleteByID(id, userID string) error {
	res, err := s.db.Exec(
		`DELETE FROM sessions WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotOwner
	}
	return nil
}

func (s *SessionStore) DeleteAllByUser(userID string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}
