package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type UserStore struct{ db *sqlx.DB }

func NewUserStore(db *sqlx.DB) *UserStore { return &UserStore{db} }

func (s *UserStore) Create(u *User) error {
	_, err := s.db.NamedExec(
		`INSERT INTO users (id, email, password) VALUES (:id, :email, :password)`, u)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *UserStore) GetByID(id string) (*User, error) {
	var u User
	if err := s.db.Get(&u, `SELECT * FROM users WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (s *UserStore) GetByEmail(email string) (*User, error) {
	var u User
	if err := s.db.Get(&u, `SELECT * FROM users WHERE email = ?`, email); err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *UserStore) UpdateEmail(id, email string) error {
	_, err := s.db.Exec(`UPDATE users SET email = ? WHERE id = ?`, email, id)
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	return nil
}

func (s *UserStore) UpdatePassword(id, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *UserStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
