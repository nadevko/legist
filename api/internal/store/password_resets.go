package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type PasswordResetStore struct{ db *sqlx.DB }

func NewPasswordResetStore(db *sqlx.DB) *PasswordResetStore { return &PasswordResetStore{db} }

func (s *PasswordResetStore) Create(r *PasswordReset) error {
	_, err := s.db.NamedExec(
		`INSERT INTO password_resets (id, user_id, token_hash, expires_at)
		 VALUES (:id, :user_id, :token_hash, :expires_at)`, r,
	)
	if err != nil {
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

func (s *PasswordResetStore) GetByTokenHash(hash string) (*PasswordReset, error) {
	var r PasswordReset
	if err := s.db.Get(&r,
		`SELECT * FROM password_resets WHERE token_hash = ? AND expires_at > datetime('now')`, hash,
	); err != nil {
		return nil, fmt.Errorf("get password reset: %w", err)
	}
	return &r, nil
}

func (s *PasswordResetStore) Delete(id string) error {
	if _, err := s.db.Exec(
		`DELETE FROM password_resets WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("delete password reset: %w", err)
	}
	return nil
}
