package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type FileStore struct{ db *sqlx.DB }

func NewFileStore(db *sqlx.DB) *FileStore { return &FileStore{db} }

func (s *FileStore) Create(f *File) error {
	_, err := s.db.NamedExec(
		`INSERT INTO files (id, user_id, name, mime_type, size, path, status)
		 VALUES (:id, :user_id, :name, :mime_type, :size, :path, :status)`, f,
	)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	return nil
}

func (s *FileStore) GetByID(id string) (*File, error) {
	var f File
	if err := s.db.Get(&f, `SELECT * FROM files WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return &f, nil
}

func (s *FileStore) ListByUser(userID string) ([]File, error) {
	var files []File
	if err := s.db.Select(&files,
		`SELECT * FROM files WHERE user_id = ? ORDER BY created_at DESC`, userID,
	); err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	return files, nil
}

func (s *FileStore) UpdateStatus(id, status string) error {
	if _, err := s.db.Exec(
		`UPDATE files SET status = ? WHERE id = ?`, status, id,
	); err != nil {
		return fmt.Errorf("update file status: %w", err)
	}
	return nil
}

func (s *FileStore) Delete(id string) error {
	if _, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}
