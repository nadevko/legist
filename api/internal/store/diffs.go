package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type DiffStore struct{ db *sqlx.DB }

func NewDiffStore(db *sqlx.DB) *DiffStore { return &DiffStore{db} }

// DiffFilter narrows List results.
type DiffFilter struct {
	UserID     *string // nil = public diffs only (user_id IS NULL)
	DocumentID *string
	FileID     string // matches left_file_id OR right_file_id; empty = no filter
	Status     string // empty = no filter
}

func (s *DiffStore) Create(d *Diff) error {
	_, err := s.db.NamedExec(`
		INSERT INTO diffs
			(id, user_id, document_id, left_file_id, right_file_id, status, similarity_percent, diff_data)
		VALUES
			(:id, :user_id, :document_id, :left_file_id, :right_file_id, :status, :similarity_percent, :diff_data)`, d)
	if err != nil {
		return fmt.Errorf("create diff: %w", err)
	}
	return nil
}

func (s *DiffStore) GetByID(id string) (*Diff, error) {
	var d Diff
	if err := s.db.Get(&d, `SELECT * FROM diffs WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *DiffStore) List(filter DiffFilter, p pagination.Params) ([]Diff, error) {
	p.Normalize()

	var q strings.Builder
	var args []any

	q.WriteString(`SELECT * FROM diffs WHERE 1=1`)

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
	if filter.FileID != "" {
		q.WriteString(` AND (left_file_id = ? OR right_file_id = ?)`)
		args = append(args, filter.FileID, filter.FileID)
	}
	if filter.Status != "" {
		q.WriteString(` AND status = ?`)
		args = append(args, filter.Status)
	}

	if p.StartingAfter != "" {
		q.WriteString(` AND (created_at < (SELECT created_at FROM diffs WHERE id = ?)
			OR (created_at = (SELECT created_at FROM diffs WHERE id = ?) AND id < ?))`)
		args = append(args, p.StartingAfter, p.StartingAfter, p.StartingAfter)
	}
	if p.EndingBefore != "" {
		q.WriteString(` AND (created_at > (SELECT created_at FROM diffs WHERE id = ?)
			OR (created_at = (SELECT created_at FROM diffs WHERE id = ?) AND id > ?))`)
		args = append(args, p.EndingBefore, p.EndingBefore, p.EndingBefore)
	}
	q.WriteString(` ORDER BY created_at DESC LIMIT ?`)
	args = append(args, p.Limit+1)

	var rows []Diff
	if err := s.db.Select(&rows, q.String(), args...); err != nil {
		return nil, fmt.Errorf("list diffs: %w", err)
	}
	return rows, nil
}

func (s *DiffStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE diffs SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update diff status: %w", err)
	}
	return nil
}

// UpdateResult writes similarity, diff payload, and status (typically "done").
func (s *DiffStore) UpdateResult(id string, similarity *float64, diffData DiffData, status string) error {
	_, err := s.db.Exec(`
		UPDATE diffs SET similarity_percent = ?, diff_data = ?, status = ? WHERE id = ?`,
		similarity, diffData, status, id)
	if err != nil {
		return fmt.Errorf("update diff result: %w", err)
	}
	return nil
}
