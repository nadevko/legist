package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

type documentRequest struct {
	Subtype string `json:"subtype" example:"закон"`
	Number  string `json:"number"  example:"296-З"`
	Author  string `json:"author"  example:"Парламент"`
	Date    string `json:"date"    example:"1999-07-26"`
	Country string `json:"country" example:"by"`
	Name    string `json:"name"    example:"Трудовой кодекс"`
}

type documentResponse struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	UserID   string `json:"user_id,omitempty"`
	Subtype  string `json:"subtype"`
	Number   string `json:"number"`
	Author   string `json:"author"`
	Date     string `json:"date"`
	Country  string `json:"country"`
	Name     string `json:"name,omitempty"`
	NPALevel int    `json:"npa_level"`
	Complete bool   `json:"complete"`
	Created  int64  `json:"created"`
}

// handleCreateDocument godoc
// @Summary     Create a document
// @Tags        documents
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       body            body   documentRequest true  "Document fields (all optional)"
// @Param       Idempotency-Key header string          false "Idempotency key"
// @Success     201 {object} documentResponse
// @Failure     409 {object} apiErrorResponse "Duplicate subtype+number+date"
// @Failure     401 {object} apiErrorResponse
// @Router      /documents [post]
func (s *Server) handleCreateDocument(c echo.Context) error {
	var body documentRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	doc := buildDocument(body, auth.UserID(c))
	doc.ID = newID("doc")
	if err := s.documents.Create(doc); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return errorf(http.StatusConflict, "duplicate_document",
				"document with this subtype, number and date already exists")
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusCreated, toDocumentResponse(*doc))
}

// handleListDocuments godoc
// @Summary     List documents
// @Tags        documents
// @Security    BearerAuth
// @Produce     json
// @Param       owner          query string false "omit: own only (user) or own+public (admin); null=public only (admin); or your user id"
// @Param       limit          query int    false "Limit"
// @Param       starting_after query string false "Cursor"
// @Param       ending_before  query string false "Cursor"
// @Success     200 {object} listResponse[documentResponse]
// @Failure     401 {object} apiErrorResponse
// @Router      /documents [get]
func (s *Server) handleListDocuments(c echo.Context) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	kind, uid, err := resolveOwnerListQuery(c)
	if err != nil {
		return err
	}
	var docFilter store.DocumentListFilter
	switch kind {
	case ownerListSelfOnly:
		docFilter.UserID = &uid
	case ownerListPublicOnly:
		docFilter.UserID = nil
	case ownerListSelfAndPublic:
		docFilter.UserID = &uid
		docFilter.SelfOrPublic = true
	}
	docs, err := s.documents.List(docFilter, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(docs, p.Limit,
		toDocumentResponse, func(d store.Document) string { return d.ID }))
}

// handleGetDocument godoc
// @Summary     Get document
// @Tags        documents
// @Security    BearerAuth
// @Param       id       path   string   true  "Document ID"
// @Param       expand[] query  []string false "Expand: files"
// @Produce     json
// @Success     200 {object} documentResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /documents/{id} [get]
func (s *Server) handleGetDocument(c echo.Context) error {
	doc, err := s.resolveDocument(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toDocumentResponse(*doc))
}

// handleUpdateDocument godoc
// @Summary     Update document Work-level fields
// @Tags        documents
// @Security    BearerAuth
// @Param       id              path   string          true  "Document ID"
// @Param       body            body   documentRequest true  "Fields to update"
// @Param       Idempotency-Key header string          false "Idempotency key"
// @Accept      json
// @Produce     json
// @Success     200 {object} documentResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     409 {object} apiErrorResponse
// @Router      /documents/{id} [patch]
func (s *Server) handleUpdateDocument(c echo.Context) error {
	doc, err := s.resolveDocument(c)
	if err != nil {
		return err
	}

	var body documentRequest
	if err = c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}

	upd := store.DocumentUpdate{}
	if body.Subtype != "" {
		upd.Subtype = &body.Subtype
	}
	if body.Number != "" {
		upd.Number = &body.Number
	}
	if body.Author != "" {
		upd.Author = &body.Author
	}
	if body.Date != "" {
		upd.Date = &body.Date
	}
	if body.Country != "" {
		upd.Country = &body.Country
	}
	if body.Name != "" {
		upd.Name = &body.Name
	}

	if err = s.documents.ApplyUpdate(doc, upd); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return errorf(http.StatusConflict, "duplicate_document",
				"document with this subtype, number and date already exists")
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, toDocumentResponse(*doc))
}

// handleDeleteDocument godoc
// @Summary     Delete document
// @Tags        documents
// @Security    BearerAuth
// @Param       id              path   string true  "Document ID"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /documents/{id} [delete]
func (s *Server) handleDeleteDocument(c echo.Context) error {
	id := c.Param("id")
	// resolveDocument already checks existence + ownership.
	if _, err := s.resolveDocument(c); err != nil {
		return err
	}
	if err := s.documents.Delete(id, auth.UserID(c)); err != nil {
		if errors.Is(err, store.ErrNotOwner) {
			return errorf(http.StatusForbidden, "forbidden", "you do not own this document")
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "document"))
}

// resolveDocument loads the document by id and enforces ownership.
// Returns 404 for both "not found" and "belongs to another user" to avoid
// leaking existence of other users' documents.
// Public documents (UserID == nil) are readable by anyone via GET,
// but mutations are blocked — SQL WHERE user_id=? returns 0 rows for public docs.
func (s *Server) resolveDocument(c echo.Context) (*store.Document, error) {
	id := c.Param("id")
	doc, err := s.documents.GetByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such document: "+id)
	}
	if err != nil {
		return nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	// For mutating methods, block if caller doesn't own it.
	// OwnerID() returns "" for public docs — never matches a real JWT user_id.
	if c.Request().Method != http.MethodGet && doc.OwnerID() != auth.UserID(c) {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such document: "+id)
	}
	return doc, nil
}

// --- helpers ---

func buildDocument(r documentRequest, userID string) *store.Document {
	d := &store.Document{
		UserID:  &userID,
		Subtype: r.Subtype,
		Number:  r.Number,
		Author:  r.Author,
		Date:    r.Date,
		Country: r.Country,
		Name:    r.Name,
	}
	if d.Country == "" {
		d.Country = "by"
	}
	return d
}

func toDocumentResponse(d store.Document) documentResponse {
	return documentResponse{
		ID:       d.ID,
		Object:   "document",
		UserID:   d.OwnerID(),
		Subtype:  d.Subtype,
		Number:   d.Number,
		Author:   d.Author,
		Date:     d.Date,
		Country:  d.Country,
		Name:     d.Name,
		NPALevel: d.NPALevel,
		Complete: d.IsComplete(),
		Created:  toUnix(d.CreatedAt),
	}
}
