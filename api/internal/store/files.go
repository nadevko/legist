package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type FileStore struct{ db *sqlx.DB }

func NewFileStore(db *sqlx.DB) *FileStore { return &FileStore{db} }

// FileFilter — фильтры для List.
type FileFilter struct {
	UserID *string // nil = публичные (user_id IS NULL), ptr = конкретный юзер
	Status string  // "" = все
}

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

func (s *FileStore) List(filter FileFilter, p pagination.Params) ([]File, error) {
	p.Normalize()

	q := strings.Builder{}
	args := []any{}

	q.WriteString(`SELECT * FROM files WHERE 1=1`)

	if filter.UserID == nil {
		q.WriteString(` AND user_id IS NULL`)
	} else {
		q.WriteString(` AND user_id = ?`)
		args = append(args, *filter.UserID)
	}

	if filter.Status != "" {
		q.WriteString(` AND status = ?`)
		args = append(args, filter.Status)
	}

	if p.StartingAfter != "" {
		q.WriteString(` AND (created_at < (SELECT created_at FROM files WHERE id = ?)
			OR (created_at = (SELECT created_at FROM files WHERE id = ?) AND id < ?))`)
		args = append(args, p.StartingAfter, p.StartingAfter, p.StartingAfter)
	}

	if p.EndingBefore != "" {
		q.WriteString(` AND (created_at > (SELECT created_at FROM files WHERE id = ?)
			OR (created_at = (SELECT created_at FROM files WHERE id = ?) AND id > ?))`)
		args = append(args, p.EndingBefore, p.EndingBefore, p.EndingBefore)
	}

	q.WriteString(` ORDER BY created_at DESC LIMIT ?`)
	args = append(args, p.Limit+1)

	var files []File
	if err := s.db.Select(&files, q.String(), args...); err != nil {
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
