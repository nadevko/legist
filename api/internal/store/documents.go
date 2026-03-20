package store

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/nadevko/legist/internal/pagination"
)

type DocumentStore struct{ db *sqlx.DB }

func NewDocumentStore(db *sqlx.DB) *DocumentStore { return &DocumentStore{db} }

// DocumentUpdate carries fields that can be patched. Nil = do not touch.
type DocumentUpdate struct {
	Subtype  *string
	Number   *string
	Author   *string
	Date     *string
	Country  *string
	Name     *string
	NPALevel *int

	// RAG enrichment (Work-level). Nil pointer = do not touch.
	RagTags       *string
	RagCategories *string
	RagKeywords   *string
	RagSummary    *string
	Jurisdiction  *string
	ContractType  *string
}

func (s *DocumentStore) Create(d *Document) error {
	_, err := s.db.NamedExec(`
		INSERT INTO documents
			(id, user_id, subtype, number, author, date, country, name, npa_level)
		VALUES
			(:id, :user_id, :subtype, :number, :author, :date, :country, :name, :npa_level)`, d)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

// GetByID returns the document regardless of owner. Caller enforces ownership.
func (s *DocumentStore) GetByID(id string) (*Document, error) {
	var d Document
	if err := s.db.Get(&d, `SELECT * FROM documents WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return &d, nil
}

// DocumentListFilter narrows List by document owner (user_id).
type DocumentListFilter struct {
	UserID       *string
	SelfOrPublic bool // when true and UserID set: user_id = ? OR user_id IS NULL
}

func (s *DocumentStore) List(filter DocumentListFilter, p pagination.Params) ([]Document, error) {
	p.Normalize()

	var q strings.Builder
	var args []any

	q.WriteString(`SELECT * FROM documents WHERE 1=1`)
	switch {
	case filter.SelfOrPublic && filter.UserID != nil:
		q.WriteString(` AND (user_id = ? OR user_id IS NULL)`)
		args = append(args, *filter.UserID)
	case filter.UserID == nil:
		q.WriteString(` AND user_id IS NULL`)
	default:
		q.WriteString(` AND user_id = ?`)
		args = append(args, *filter.UserID)
	}
	if p.StartingAfter != "" {
		q.WriteString(` AND (created_at < (SELECT created_at FROM documents WHERE id = ?)
			OR (created_at = (SELECT created_at FROM documents WHERE id = ?) AND id < ?))`)
		args = append(args, p.StartingAfter, p.StartingAfter, p.StartingAfter)
	}
	if p.EndingBefore != "" {
		q.WriteString(` AND (created_at > (SELECT created_at FROM documents WHERE id = ?)
			OR (created_at = (SELECT created_at FROM documents WHERE id = ?) AND id > ?))`)
		args = append(args, p.EndingBefore, p.EndingBefore, p.EndingBefore)
	}
	q.WriteString(` ORDER BY created_at DESC LIMIT ?`)
	args = append(args, p.Limit+1)

	var docs []Document
	if err := s.db.Select(&docs, q.String(), args...); err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return docs, nil
}

// ApplyUpdate writes non-nil fields from upd into doc and persists to DB.
func (s *DocumentStore) ApplyUpdate(doc *Document, upd DocumentUpdate) error {
	if upd.Subtype != nil {
		doc.Subtype = *upd.Subtype
	}
	if upd.Number != nil {
		doc.Number = *upd.Number
	}
	if upd.Author != nil {
		doc.Author = *upd.Author
	}
	if upd.Date != nil {
		doc.Date = *upd.Date
	}
	if upd.Country != nil {
		doc.Country = *upd.Country
	}
	if upd.Name != nil {
		doc.Name = *upd.Name
	}
	if upd.NPALevel != nil {
		doc.NPALevel = *upd.NPALevel
	}
	if upd.RagTags != nil {
		doc.RagTags = *upd.RagTags
	}
	if upd.RagCategories != nil {
		doc.RagCategories = *upd.RagCategories
	}
	if upd.RagKeywords != nil {
		doc.RagKeywords = *upd.RagKeywords
	}
	if upd.RagSummary != nil {
		doc.RagSummary = *upd.RagSummary
	}
	if upd.Jurisdiction != nil {
		doc.Jurisdiction = *upd.Jurisdiction
	}
	if upd.ContractType != nil {
		doc.ContractType = *upd.ContractType
	}
	if err := s.persist(doc); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func (s *DocumentStore) persist(d *Document) error {
	_, err := s.db.NamedExec(`
		UPDATE documents SET
			subtype   = :subtype,
			number    = :number,
			author    = :author,
			date      = :date,
			country   = :country,
			name      = :name,
			npa_level = :npa_level,

			rag_tags       = :rag_tags,
			rag_categories = :rag_categories,
			rag_keywords   = :rag_keywords,
			rag_summary    = :rag_summary,
			jurisdiction   = :jurisdiction,
			contract_type  = :contract_type
		WHERE id = :id`, d)
	if err != nil {
		return fmt.Errorf("persist document: %w", err)
	}
	return nil
}

// Delete removes a document only if it belongs to userID.
// Public documents (UserID == nil) cannot be deleted via this method.
func (s *DocumentStore) Delete(id, userID string) error {
	res, err := s.db.Exec(
		`DELETE FROM documents WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotOwner
	}
	return nil
}
