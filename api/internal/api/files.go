package api

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/parser"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
	"github.com/nadevko/legist/internal/upload"
	"github.com/nadevko/legist/internal/webhook"
)

const mimeTypeLegistoso = "application/legistoso"

func badUpload(err error) error {
	if err == nil {
		return nil
	}
	return errorf(http.StatusBadRequest, "invalid_request", err.Error())
}

// filePatchRequest carries the fields that can be updated after upload.
// All fields are optional pointers — null means "do not touch".
type filePatchRequest struct {
	VersionDate   *string `json:"version_date"`
	VersionNumber *string `json:"version_number"`
	VersionLabel  *string `json:"version_label"`
	Language      *string `json:"language"`
	PubName       *string `json:"pub_name"`
	PubDate       *string `json:"pub_date"`
	PubNumber     *string `json:"pub_number"`
}

// handleListFiles godoc
// @Summary     List files; if document_id is set, forwards to /documents/:id/files
// @Tags        Files
// @Security    BearerAuth
// @Produce     json
// @Param       owner          query  string   false "omit: own (user) or own+public (admin); null|public=public only (admin); or your user id"
// @Param       document_id    query  string   false "Forward to /documents/:id/files"
// @Param       type           query  string   false "pdf|docx"
// @Param       status         query  string   false "pending|processing|done|failed"
// @Param       limit          query  int      false "Limit (default 20, max 100)"
// @Param       starting_after query  string   false "Cursor"
// @Param       ending_before  query  string   false "Cursor"
// @Param       expand[]       query  []string false "Expand: document"
// @Success     200 {object} listResponse[fileResponse]
// @Failure     401 {object} apiErrorResponse
// @Router      /files [get]
func (s *Server) handleListFiles(c echo.Context) error {
	if docID := c.QueryParam("document_id"); docID != "" {
		if _, err := s.documents.GetByID(docID); errors.Is(err, sql.ErrNoRows) {
			return errorf(http.StatusNotFound, "resource_missing", "no such document: "+docID)
		} else if err != nil {
			return errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
		return s.listFilesCore(c, &docID)
	}
	return s.listFilesCore(c, nil)
}

// handleListDocumentFiles godoc
// @Summary     List file versions of a document
// @Tags        Documents
// @Security    BearerAuth
// @Produce     json
// @Param       id             path   string   true  "Document ID"
// @Param       type           query  string   false "pdf|docx"
// @Param       status         query  string   false "pending|processing|done|failed"
// @Param       limit          query  int      false "Limit"
// @Param       starting_after query  string   false "Cursor"
// @Param       ending_before  query  string   false "Cursor"
// @Success     200 {object} listResponse[fileResponse]
// @Failure     404 {object} apiErrorResponse
// @Router      /documents/{id}/files [get]
func (s *Server) handleListDocumentFiles(c echo.Context) error {
	docID := c.Param("id")
	if _, err := s.documents.GetByID(docID); errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusNotFound, "resource_missing", "no such document: "+docID)
	} else if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return s.listFilesCore(c, &docID)
}

// listFilesCore is the shared list implementation.
func (s *Server) listFilesCore(c echo.Context, documentID *string) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}

	filter := store.FileFilter{Status: c.QueryParam("status")}
	if typ := strings.TrimSpace(c.QueryParam("type")); typ != "" {
		mime, ok := typeToMime(typ)
		if !ok {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				"type must be 'pdf' or 'docx'", "type")
		}
		filter.MimeType = mime
	}

	kind, uid, err := resolveOwnerListQuery(c)
	if err != nil {
		return err
	}
	switch kind {
	case ownerListSelfOnly:
		filter.UserID = &uid
	case ownerListPublicOnly:
		filter.UserID = nil
	case ownerListSelfAndPublic:
		filter.UserID = &uid
		filter.SelfOrPublic = true
	}

	if documentID != nil {
		filter.DocumentID = documentID
	}

	files, err := s.files.List(filter, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(files, p.Limit,
		toFileResponse, func(f store.File) string { return f.ID }))
}

// handleGetFile godoc
// @Summary     Get file metadata, parsed artifact, download, or stream status
// @Tags        Files
// @Security    BearerAuth
// @Param       id       path   string   true  "File ID"
// @Param       expand[] query  []string false "Expand: document"
// @Param       Accept   header string   false "application/json | application/legistoso | application/pdf | application/vnd...docx | text/event-stream"
// @Produce     json
// @Success     200 {object} fileResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     409 {object} apiErrorResponse "not_ready when Accept: application/legistoso and parsing incomplete"
// @Router      /files/{id} [get]
func (s *Server) handleGetFile(c echo.Context) error {
	f, err := s.resolveFile(c)
	if err != nil {
		return err
	}

	switch c.Request().Header.Get("Accept") {
	case "text/event-stream":
		return sse.Stream(c, s.broker, f.ID)
	case mimeTypeLegistoso:
		return s.serveParsedArtifact(c, f)
	case "application/json", "":
		return c.JSON(http.StatusOK, toFileResponse(*f))
	default:
		return c.File(f.Path)
	}
}

func (s *Server) serveParsedArtifact(c echo.Context, f *store.File) error {
	if f.Status != "done" {
		return errorf(http.StatusConflict, "not_ready",
			"file is not yet parsed (status: "+f.Status+")")
	}

	parsedPath := filepath.Join(s.cfg.DataPath, "legistoso", f.ID)
	data, err := os.ReadFile(parsedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errorf(http.StatusNotFound, "resource_missing",
				"legistoso artifact not found for file: "+f.ID)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.Blob(http.StatusOK, mimeTypeLegistoso, data)
}

// handleUploadFile godoc
// @Summary     Upload a file; creates a new Document automatically
// @Tags        Files
// @Security    BearerAuth
// @Accept      multipart/form-data
// @Produce     json
// @Param       file            formData file   true  "PDF or DOCX"
// @Param       document_id     formData string false "Forward to /documents/:id/files"
// @Param       subtype         formData string false "AKN subtype"
// @Param       number          formData string false "Document number"
// @Param       author          formData string false "Issuing body"
// @Param       date            formData string false "Adoption date YYYY-MM-DD"
// @Param       country         formData string false "Country code (default: by)"
// @Param       name            formData string false "Short title"
// @Param       version_date    formData string false "Amendment date YYYY-MM-DD"
// @Param       version_number  formData string false "Version number"
// @Param       version_label   formData string false "Version label"
// @Param       language        formData string false "Language (rus|bel)"
// @Param       pub_name        formData string false "Publication name"
// @Param       pub_date        formData string false "Publication date YYYY-MM-DD"
// @Param       pub_number      formData string false "Publication number"
// @Param       Accept          header   string false "application/json | text/event-stream"
// @Param       Idempotency-Key header   string false "Idempotency key"
// @Success     201 {object} fileResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse "document_id not found"
// @Router      /files [post]
func (s *Server) handleUploadFile(c echo.Context) error {
	// Parse multipart before any FormValue — otherwise Echo uses ~32MiB default and our 52MiB cap never applies.
	if err := upload.ParseMultipart(c); err != nil {
		return badUpload(err)
	}
	docID := strings.TrimSpace(c.FormValue("document_id"))
	if docID != "" {
		if _, err := s.documents.GetByID(docID); errors.Is(err, sql.ErrNoRows) {
			return errorf(http.StatusNotFound, "resource_missing", "no such document: "+docID)
		} else if err != nil {
			return errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
		return s.uploadFile(c, &docID)
	}
	return s.uploadFile(c, nil)
}

// handleUploadDocumentFile godoc
// @Summary     Upload a file as a new version of an existing Document
// @Tags        Documents
// @Security    BearerAuth
// @Accept      multipart/form-data
// @Produce     json
// @Param       id              path     string true  "Document ID"
// @Param       file            formData file   true  "PDF or DOCX"
// @Param       version_date    formData string false "Amendment date YYYY-MM-DD"
// @Param       version_number  formData string false "Version number"
// @Param       version_label   formData string false "Version label"
// @Param       language        formData string false "Language (rus|bel)"
// @Param       pub_name        formData string false "Publication name"
// @Param       pub_date        formData string false "Publication date"
// @Param       pub_number      formData string false "Publication number"
// @Param       Accept          header   string false "application/json | text/event-stream"
// @Param       Idempotency-Key header   string false "Idempotency key"
// @Success     201 {object} fileResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /documents/{id}/files [post]
func (s *Server) handleUploadDocumentFile(c echo.Context) error {
	docID := c.Param("id")
	if _, err := s.documents.GetByID(docID); errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusNotFound, "resource_missing", "no such document: "+docID)
	} else if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err := upload.ParseMultipart(c); err != nil {
		return badUpload(err)
	}
	return s.uploadFile(c, &docID)
}

// handlePatchFile godoc
// @Summary     Update file Expression-level metadata
// @Tags        Files
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Param       id              path   string         true  "File ID"
// @Param       body            body   filePatchRequest true  "Fields to update (all optional)"
// @Param       Idempotency-Key header string         false "Idempotency key"
// @Success     200 {object} fileResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /files/{id} [patch]
func (s *Server) handlePatchFile(c echo.Context) error {
	f, err := s.resolveFile(c)
	if err != nil {
		return err
	}

	var body filePatchRequest
	if err = c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}

	upd := store.FileMetaUpdate{
		VersionDate:   body.VersionDate,
		VersionNumber: body.VersionNumber,
		VersionLabel:  body.VersionLabel,
		Language:      body.Language,
		PubName:       body.PubName,
		PubDate:       body.PubDate,
		PubNumber:     body.PubNumber,
	}
	if err = s.files.UpdateMeta(f.ID, upd); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	// Reload to return the updated state.
	updated, err := s.files.GetByID(f.ID)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, toFileResponse(*updated))
}

// handleDeleteFile godoc
// @Summary     Delete file
// @Tags        Files
// @Security    BearerAuth
// @Param       id              path   string true  "File ID"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     409 {object} apiErrorResponse "File is currently processing"
// @Router      /files/{id} [delete]
func (s *Server) handleDeleteFile(c echo.Context) error {
	f, err := s.resolveFile(c)
	if err != nil {
		return err
	}

	// Block deletion while the parser goroutine is running.
	// The goroutine writes to f.Path and updates DB status — deleting mid-flight
	// would cause it to write to a non-existent path and leave the DB in "done"
	// with no file on disk.
	if f.Status == "processing" {
		return errorf(http.StatusConflict, "file_processing",
			"file is currently being processed; wait for it to complete or fail")
	}

	sourceLink := filepath.Join(s.cfg.DataPath, "sources", f.ID)
	targetPath, readErr := os.Readlink(sourceLink)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = os.Remove(sourceLink); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if targetPath == "" {
		targetPath = filepath.Join(s.cfg.DataPath, fileTypeDir(f.MimeType), f.ID)
	} else if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(filepath.Dir(sourceLink), targetPath)
	}
	if err = os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = os.Remove(filepath.Join(s.cfg.DataPath, "plain", f.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = os.Remove(filepath.Join(s.cfg.DataPath, "legistoso", f.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = s.files.Delete(f.ID); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	s.dispatcher.Dispatch(webhook.EventFileDeleted, toFileResponse(*f))
	return c.JSON(http.StatusOK, deleted(f.ID, "file"))
}

// processFileChannel configures SSE publishing when the pipeline runs in a diff context.
// Key is the broker channel (e.g. diff ID). DoneEvent/FailedEvent rename terminal events so
// diff SSE streams do not end on per-file completion.
type processFileChannel struct {
	Key         string
	DoneEvent   string // default "done"
	FailedEvent string // default "failed"
}

// processFile runs the parse+extract pipeline in a goroutine.
func (s *Server) processFile(f *store.File, doc *store.Document) {
	s.processFileWithChannel(f, doc, nil)
}

func (s *Server) processFileWithChannel(f *store.File, doc *store.Document, ch *processFileChannel) {
	ctx := context.Background()

	key := f.ID
	doneType := "done"
	failedType := "failed"
	diffChannel := false
	if ch != nil && ch.Key != "" {
		key = ch.Key
		if ch.Key != f.ID {
			diffChannel = true
		}
		if ch.DoneEvent != "" {
			doneType = ch.DoneEvent
		}
		if ch.FailedEvent != "" {
			failedType = ch.FailedEvent
		}
	}

	publish := func(evtType string, data any) {
		outType := evtType
		outData := data
		if diffChannel {
			switch evtType {
			case "progress":
				if p, ok := data.(parser.ParseProgress); ok {
					outData = map[string]any{"file_id": f.ID, "progress": p}
				}
			case "done":
				outType = doneType
				outData = map[string]any{"file_id": f.ID, "status": "done"}
			case "failed":
				outType = failedType
				if m, ok := data.(map[string]any); ok {
					outData = map[string]any{"file_id": f.ID, "error": m["error"]}
				}
			}
		} else {
			if evtType == "done" {
				outData = map[string]any{"file_id": f.ID, "status": "done"}
			}
		}
		s.broker.Publish(key, sse.Event{Type: outType, Data: outData})
	}

	s.files.UpdateStatus(f.ID, "processing")

	cfg := parser.PipelineConfig{
		FileID:     f.ID,
		DocumentID: doc.ID,
		Path:       f.Path,
		DataPath:   s.cfg.DataPath,
		MimeType:   f.MimeType,

		KnownSubtype: doc.Subtype,
		KnownNumber:  doc.Number,
		KnownAuthor:  doc.Author,
		KnownDate:    doc.Date,
		KnownCountry: doc.Country,
		KnownName:    doc.Name,

		KnownVersionDate:   strVal(f.VersionDate),
		KnownVersionNumber: strVal(f.VersionNumber),
		KnownVersionLabel:  strVal(f.VersionLabel),
		KnownLanguage:      strVal(f.Language),
		KnownPubName:       strVal(f.PubName),
		KnownPubDate:       strVal(f.PubDate),
		KnownPubNumber:     strVal(f.PubNumber),

		MetaExtractor: parser.MetaExtractorConfig{
			OllamaBaseURL: s.cfg.OllamaBaseURL,
			MetadataModel: s.cfg.MetadataModel,
			MaxRetries:    s.cfg.MetadataMaxRetries,
		},
		WindowSize:    s.cfg.MetadataWindowSize,
		ParserVersion: "1",
	}

	res, err := parser.Run(ctx, cfg, func(p parser.ParseProgress) {
		if p.Stage == parser.StageDone || p.Stage == parser.StageFailed {
			return
		}
		publish("progress", p)
	})

	if err != nil {
		s.files.UpdateStatus(f.ID, "failed")
		publish("failed", map[string]any{"file_id": f.ID, "error": err.Error()})
		s.dispatcher.Dispatch(webhook.EventFileFailed, toFileResponse(*f))
		return
	}

	// Write back Work-level fields to Document.
	upd := store.DocumentUpdate{
		Subtype:  &res.Subtype,
		Number:   &res.Number,
		Author:   &res.Author,
		Date:     &res.Date,
		Country:  &res.Country,
		NPALevel: &res.NPALevel,
	}
	if res.Name != "" {
		upd.Name = &res.Name
	}
	_ = s.documents.ApplyUpdate(doc, upd)

	// Write back Expression-level fields to File.
	_ = s.files.UpdateMeta(f.ID, store.FileMetaUpdate{
		VersionDate:   ptrStr(res.VersionDate),
		VersionNumber: ptrStr(res.VersionNumber),
		VersionLabel:  ptrStr(res.VersionLabel),
		Language:      ptrStr(res.Language),
		PubName:       ptrStr(res.PubName),
		PubDate:       ptrStr(res.PubDate),
		PubNumber:     ptrStr(res.PubNumber),
	})

	s.files.UpdateStatus(f.ID, "done")
	publish("done", map[string]any{"file_id": f.ID, "status": "done"})
	s.dispatcher.Dispatch(webhook.EventFileParsed, toFileResponse(*f))
}

// saveFormFile persists one multipart file field and creates DB rows.
// Prerequisites: upload.ParseMultipart(c) then upload.BindFormData(c) (or upload.PrepareForm for both in one call).
// It does not dispatch webhooks or start the parse pipeline.
func (s *Server) saveFormFile(c echo.Context, fieldName string, documentID *string, req upload.FormData) (*store.File, *store.Document, error) {
	userID := auth.UserID(c)

	fh, err := c.FormFile(fieldName)
	if err != nil {
		return nil, nil, errorf(http.StatusBadRequest, "parameter_missing", "file is required", fieldName)
	}
	mime := strings.Split(fh.Header.Get("Content-Type"), ";")[0]
	if !allowedMIME[mime] {
		return nil, nil, errorf(http.StatusBadRequest, "invalid_parameter_value",
			"unsupported mime type: "+mime, fieldName)
	}
	src, err := fh.Open()
	if err != nil {
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	defer src.Close()

	if err = parser.ValidateReader(src, fh.Size, mime); err != nil {
		return nil, nil, errorf(http.StatusBadRequest, "invalid_file", err.Error(), fieldName)
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	fileID := newID("file")
	targetDir := filepath.Join(s.cfg.DataPath, fileTypeDir(mime))
	targetPath := filepath.Join(targetDir, fileID)
	if err = saveFile(src, targetDir, targetPath); err != nil {
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	sourceDir := filepath.Join(s.cfg.DataPath, "sources")
	sourceLink := filepath.Join(sourceDir, fileID)
	if err = os.MkdirAll(sourceDir, 0755); err != nil {
		_ = os.Remove(targetPath)
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = os.Symlink(targetPath, sourceLink); err != nil {
		_ = os.Remove(targetPath)
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	var doc *store.Document
	if documentID == nil {
		doc = &store.Document{
			ID:      newID("doc"),
			UserID:  &userID,
			Subtype: req.Subtype,
			Number:  req.Number,
			Author:  req.Author,
			Date:    req.Date,
			Country: req.Country,
			Name:    req.Name,
		}
		if doc.Country == "" {
			doc.Country = "by"
		}
		if err = s.documents.Create(doc); err != nil {
			_ = os.Remove(sourceLink)
			_ = os.Remove(targetPath)
			if errors.Is(err, store.ErrDuplicate) {
				return nil, nil, errorf(http.StatusConflict, "duplicate_document",
					"document with this subtype, number and date already exists")
			}
			return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
		documentID = &doc.ID
	} else {
		doc, err = s.documents.GetByID(*documentID)
		if err != nil {
			_ = os.Remove(sourceLink)
			_ = os.Remove(targetPath)
			return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
	}

	uid := userID
	f := &store.File{
		ID:         fileID,
		UserID:     &uid,
		DocumentID: documentID,
		Name:       fh.Filename,
		MimeType:   mime,
		Size:       fh.Size,
		Path:       sourceLink,
		Status:     "pending",
	}
	applyExpressionFields(f, req)

	if err = s.files.Create(f); err != nil {
		_ = os.Remove(sourceLink)
		_ = os.Remove(targetPath)
		return nil, nil, errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return f, doc, nil
}

// uploadFile completes upload after upload.ParseMultipart(c) has been called (bind + save + pipeline).
func (s *Server) uploadFile(c echo.Context, documentID *string) error {
	req, err := upload.BindFormData(c)
	if err != nil {
		return badUpload(err)
	}

	f, doc, err := s.saveFormFile(c, "file", documentID, req)
	if err != nil {
		return err
	}

	s.dispatcher.Dispatch(webhook.EventFileCreated, toFileResponse(*f))
	go s.processFile(f, doc)

	if c.Request().Header.Get("Accept") == "text/event-stream" {
		return sse.Stream(c, s.broker, f.ID)
	}
	return c.JSON(http.StatusCreated, toFileResponse(*f))
}

// resolveFile loads the file by id and enforces ownership.
// Returns 404 for both "not found" and "belongs to another user".
func (s *Server) resolveFile(c echo.Context) (*store.File, error) {
	id := c.Param("id")
	f, err := s.files.GetByID(id)
	if err != nil {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}
	if f.UserID != nil && *f.UserID != auth.UserID(c) {
		return nil, errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}
	return f, nil
}

// --- helpers ---

func applyExpressionFields(f *store.File, req upload.FormData) {
	if req.VersionDate != "" {
		f.VersionDate = &req.VersionDate
	}
	if req.VersionNumber != "" {
		f.VersionNumber = &req.VersionNumber
	}
	if req.VersionLabel != "" {
		f.VersionLabel = &req.VersionLabel
	}
	if req.Language != "" {
		f.Language = &req.Language
	}
	if req.PubName != "" {
		f.PubName = &req.PubName
	}
	if req.PubDate != "" {
		f.PubDate = &req.PubDate
	}
	if req.PubNumber != "" {
		f.PubNumber = &req.PubNumber
	}
}

func toFileResponse(f store.File) fileResponse {
	return fileResponse{
		ID:            f.ID,
		Object:        "file",
		DocumentID:    f.DocumentID,
		UserID:        f.UserID,
		Name:          f.Name,
		MimeType:      f.MimeType,
		Size:          f.Size,
		Status:        f.Status,
		VersionDate:   f.VersionDate,
		VersionNumber: f.VersionNumber,
		VersionLabel:  f.VersionLabel,
		Language:      f.Language,
		Created:       toUnix(f.CreatedAt),
	}
}

func saveFile(src io.Reader, dir, dst string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func fileTypeDir(mime string) string {
	switch mime {
	case "application/pdf":
		return "pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	default:
		return "other"
	}
}

func typeToMime(typ string) (string, bool) {
	switch typ {
	case "pdf":
		return "application/pdf", true
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true
	default:
		return "", false
	}
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptr[T any](v T) *T { return &v }
