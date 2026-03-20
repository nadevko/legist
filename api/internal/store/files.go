package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type FileStore struct{ db *sqlx.DB }

func NewFileStore(db *sqlx.DB) *FileStore { return &FileStore{db} }

// FileFilter is used by List to narrow results.
type FileFilter struct {
	UserID     *string // nil = public (user_id IS NULL)
	DocumentID *string // nil = no filter
	Status     string  // "" = no filter
	MimeType   string  // "" = no filter
}

// FileMetaUpdate carries Expression-level fields written back after LLM.
// Nil pointer = do not touch.
type FileMetaUpdate struct {
	VersionDate   *string
	VersionNumber *string
	VersionLabel  *string
	Language      *string
	PubName       *string
	PubDate       *string
	PubNumber     *string
}

func (s *FileStore) Create(f *File) error {
	_, err := s.db.NamedExec(`
		INSERT INTO files
			(id, user_id, document_id, name, mime_type, size, path, status,
			 version_date, version_number, version_label, language,
			 pub_name, pub_date, pub_number)
		VALUES
			(:id, :user_id, :document_id, :name, :mime_type, :size, :path, :status,
			 :version_date, :version_number, :version_label, :language,
			 :pub_name, :pub_date, :pub_number)`, f)
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

	var q strings.Builder
	var args []any

	q.WriteString(`SELECT * FROM files WHERE 1=1`)

	if filter.UserID == nil {
		q.WriteString(` AND user_id IS NULL`)
	} else {
		q.WriteString(` AND user_id = ?`)
		args = append(args, *filter.UserID)
	}
	if filter.DocumentID != nil {
		q.WriteString(` AND document_id = ?`)
		args = append(args, *filter.DocumentID)
	}
	if filter.Status != "" {
		q.WriteString(` AND status = ?`)
		args = append(args, filter.Status)
	}
	if filter.MimeType != "" {
		q.WriteString(` AND mime_type = ?`)
		args = append(args, filter.MimeType)
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
	_, err := s.db.Exec(`UPDATE files SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update file status: %w", err)
	}
	return nil
}

// UpdateMeta persists Expression-level fields extracted by LLM.
// Only non-nil fields are written; existing values are preserved otherwise.
func (s *FileStore) UpdateMeta(id string, upd FileMetaUpdate) error {
	var setClauses []string
	var args []any

	set := func(col string, v *string) {
		if v != nil {
			setClauses = append(setClauses, col+" = ?")
			args = append(args, *v)
		}
	}
	set("version_date", upd.VersionDate)
	set("version_number", upd.VersionNumber)
	set("version_label", upd.VersionLabel)
	set("language", upd.Language)
	set("pub_name", upd.PubName)
	set("pub_date", upd.PubDate)
	set("pub_number", upd.PubNumber)

	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, id)
	q := "UPDATE files SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
	if _, err := s.db.Exec(q, args...); err != nil {
		return fmt.Errorf("update file meta: %w", err)
	}
	return nil
}

// SetDocumentID links a file to a document (used during atomic create).
func (s *FileStore) SetDocumentID(fileID, documentID string) error {
	_, err := s.db.Exec(`UPDATE files SET document_id = ? WHERE id = ?`, documentID, fileID)
	if err != nil {
		return fmt.Errorf("set document_id: %w", err)
	}
	return nil
}

func (s *FileStore) Delete(id string) error {
	res, err := s.db.Exec(`DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotOwner
	}
	return nil
}
