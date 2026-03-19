package api

import (
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
	"github.com/nadevko/legist/internal/webhook"
)

// handleListFiles godoc
// @Summary     List files
// @Tags        files
// @Security    BearerAuth
// @Produce     json
// @Param       owner          query  string   false "omit=current user files, public=laws only"
// @Param       status         query  string   false "Filter by status: pending|processing|done|failed"
// @Param       limit          query  int      false "Limit (default 20, max 100)"
// @Param       starting_after query  string   false "Cursor: last file ID from previous page"
// @Param       ending_before  query  string   false "Cursor: first file ID from next page"
// @Param       expand[]       query  []string false "Expand related objects: user"
// @Success     200 {object} listResponse[fileResponse]
// @Failure     401 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /files [get]
func (s *Server) handleListFiles(c echo.Context) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	filter, err := ownerFilter(c)
	if err != nil {
		return err
	}
	files, err := s.files.List(filter, p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(files, p.Limit, toFileResponse, func(f store.File) string { return f.ID }))
}

// handleGetFile godoc
// @Summary     Get file metadata, download or stream status
// @Tags        files
// @Security    BearerAuth
// @Param       id       path   string   true  "File ID"
// @Param       expand[] query  []string false "Expand related objects: user"
// @Param       Accept   header string   false "application/json | application/pdf | application/vnd...docx | text/event-stream"
// @Produce     json
// @Success     200 {object} fileResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /files/{id} [get]
func (s *Server) handleGetFile(c echo.Context) error {
	id := c.Param("id")
	f, err := s.files.GetByID(id)
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}

	userID := auth.UserID(c)
	if f.UserID != nil && *f.UserID != userID {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}

	switch c.Request().Header.Get("Accept") {
	case "text/event-stream":
		return sse.Stream(c, s.broker, f.ID)
	case "application/json", "":
		return c.JSON(http.StatusOK, toFileResponse(*f))
	default:
		return c.File(f.Path)
	}
}

// handleUploadFile godoc
// @Summary     Upload a file
// @Tags        files
// @Security    BearerAuth
// @Accept      multipart/form-data
// @Produce     json
// @Produce     text/event-stream
// @Param       file            formData file   true  "File to upload (pdf/docx)"
// @Param       Accept          header   string false "application/json (async, default) | text/event-stream (sync stream)"
// @Param       Idempotency-Key header   string false "Idempotency key"
// @Success     201 {object} fileResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /files [post]
func (s *Server) handleUploadFile(c echo.Context) error {
	userID := auth.UserID(c)

	fh, err := c.FormFile("file")
	if err != nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "file is required", "file")
	}

	mime := strings.Split(fh.Header.Get("Content-Type"), ";")[0]
	if !allowedMIME[mime] {
		return errorf(http.StatusBadRequest, "invalid_parameter_value",
			"unsupported mime type: "+mime, "file")
	}

	src, err := fh.Open()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	defer src.Close()

	if err = parser.ValidateReader(src, fh.Size, mime); err != nil {
		return errorf(http.StatusBadRequest, "invalid_file", err.Error(), "file")
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	fileID := newID("file")
	dir := filepath.Join(s.cfg.DataPath, "files", fileID)
	dst := filepath.Join(dir, fh.Filename)

	if err = saveFile(src, dir, dst); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	f := &store.File{
		ID:       fileID,
		UserID:   &userID,
		Name:     fh.Filename,
		MimeType: mime,
		Size:     fh.Size,
		Path:     dst,
		Status:   "pending",
	}
	if err = s.files.Create(f); err != nil {
		os.RemoveAll(dir)
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	s.dispatcher.Dispatch(webhook.EventFileCreated, toFileResponse(*f))
	go s.parseFile(f)

	if c.Request().Header.Get("Accept") == "text/event-stream" {
		return sse.Stream(c, s.broker, f.ID)
	}
	return c.JSON(http.StatusCreated, toFileResponse(*f))
}

// handleDeleteFile godoc
// @Summary     Delete file
// @Tags        files
// @Security    BearerAuth
// @Param       id              path   string true  "File ID"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /files/{id} [delete]
func (s *Server) handleDeleteFile(c echo.Context) error {
	id := c.Param("id")
	f, err := s.files.GetByID(id)
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}
	if f.UserID == nil || *f.UserID != auth.UserID(c) {
		return errorf(http.StatusNotFound, "resource_missing", "no such file: "+id)
	}
	if err = os.RemoveAll(filepath.Dir(f.Path)); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	if err = s.files.Delete(f.ID); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	s.dispatcher.Dispatch(webhook.EventFileDeleted, toFileResponse(*f))
	return c.JSON(http.StatusOK, deleted(id, "file"))
}

func (s *Server) parseFile(f *store.File) {
	publish := func(evtType string, data any) {
		s.broker.Publish(f.ID, sse.Event{Type: evtType, Data: data})
	}

	s.files.UpdateStatus(f.ID, "processing")
	publish("progress", map[string]any{"file_id": f.ID, "status": "processing"})

	if _, err := parser.ParseFile(f.Path, f.MimeType); err != nil {
		s.files.UpdateStatus(f.ID, "failed")
		publish("failed", map[string]any{"file_id": f.ID, "status": "failed", "error": err.Error()})
		s.dispatcher.Dispatch(webhook.EventFileFailed, toFileResponse(*f))
		return
	}

	// TODO: сохранить распаршенный документ для диффа

	s.files.UpdateStatus(f.ID, "done")
	publish("done", map[string]any{"file_id": f.ID, "status": "done"})
	s.dispatcher.Dispatch(webhook.EventFileParsed, toFileResponse(*f))
}

func toFileResponse(f store.File) fileResponse {
	return fileResponse{
		ID:       f.ID,
		Object:   "file",
		Name:     f.Name,
		MimeType: f.MimeType,
		Size:     f.Size,
		Status:   f.Status,
		UserID:   f.UserID,
		Created:  toUnix(f.CreatedAt),
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
