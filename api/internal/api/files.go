package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/sse"
	"github.com/nadevko/legist/internal/store"
)

type fileResult struct {
	FileID string  `json:"file_id,omitempty"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Error  *string `json:"error,omitempty"`
}

type uploadResponse struct {
	JobID string       `json:"job_id"`
	Files []fileResult `json:"files"`
}

// handleUploadFiles godoc
// @Summary     Upload files (batch)
// @Tags        files
// @Security    BearerAuth
// @Accept      multipart/form-data
// @Produce     json
// @Param       file[] formData file true "Files to upload (pdf/docx)"
// @Success     202 {object} uploadResponse
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /files [post]
func (s *Server) handleUploadFiles(c echo.Context) error {
	userID := auth.UserID(c)
	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}

	formFiles := form.File["file[]"]
	if len(formFiles) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "file[] is required")
	}

	job := &store.Job{
		ID:        uuid.NewString(),
		UserID:    userID,
		Type:      "file_upload",
		Status:    "pending",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err = s.jobs.Create(job); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	results := make([]fileResult, 0, len(formFiles))

	for _, fh := range formFiles {
		mime := fh.Header.Get("Content-Type")
		if !allowedMIME[mime] {
			errMsg := fmt.Sprintf("unsupported mime type: %s", mime)
			results = append(results, fileResult{Name: fh.Filename, Status: "failed", Error: &errMsg})
			continue
		}

		fileID := uuid.NewString()
		dir := filepath.Join(s.cfg.DataPath, "files", fileID)
		dst := filepath.Join(dir, fh.Filename)

		saveErr := func() error {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			src, err := fh.Open()
			if err != nil {
				return err
			}
			defer src.Close()

			out, err := os.Create(dst)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, src)
			return err
		}()

		if saveErr != nil {
			errMsg := fmt.Sprintf("failed to save file: %v", saveErr)
			results = append(results, fileResult{Name: fh.Filename, Status: "failed", Error: &errMsg})
			continue
		}

		f := &store.File{
			ID:       fileID,
			UserID:   userID,
			Name:     fh.Filename,
			MimeType: mime,
			Size:     fh.Size,
			Path:     dst,
			Status:   "pending",
		}
		if err = s.files.Create(f); err != nil {
			errMsg := "failed to save file metadata"
			results = append(results, fileResult{Name: fh.Filename, Status: "failed", Error: &errMsg})
			continue
		}

		if err = s.jobs.AddFile(&store.JobFile{
			JobID:  job.ID,
			FileID: fileID,
			Status: "pending",
		}); err != nil {
			errMsg := "failed to link file to job"
			results = append(results, fileResult{Name: fh.Filename, Status: "failed", Error: &errMsg})
			continue
		}

		results = append(results, fileResult{FileID: fileID, Name: fh.Filename, Status: "pending"})
		go s.parseFile(job.ID, f)
	}

	return c.JSON(http.StatusAccepted, uploadResponse{JobID: job.ID, Files: results})
}

// handleGetFile godoc
// @Summary     Download file
// @Tags        files
// @Security    BearerAuth
// @Param       id path string true "File ID"
// @Success     200
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /files/{id} [get]
func (s *Server) handleGetFile(c echo.Context) error {
	f, err := s.files.GetByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	if f.UserID != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	return c.File(f.Path)
}

// handleListFiles godoc
// @Summary     List files
// @Tags        files
// @Security    BearerAuth
// @Produce     json
// @Success     200 {array} fileResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /files [get]
func (s *Server) handleListFiles(c echo.Context) error {
	files, err := s.files.ListByUser(auth.UserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	resp := make([]fileResponse, len(files))
	for i, f := range files {
		resp[i] = fileResponse{
			ID:        f.ID,
			Name:      f.Name,
			MimeType:  f.MimeType,
			Size:      f.Size,
			Status:    f.Status,
			CreatedAt: f.CreatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// handleDeleteFile godoc
// @Summary     Delete file
// @Tags        files
// @Security    BearerAuth
// @Param       id path string true "File ID"
// @Success     204
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /files/{id} [delete]
func (s *Server) handleDeleteFile(c echo.Context) error {
	f, err := s.files.GetByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	if f.UserID != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	if err = os.RemoveAll(filepath.Dir(f.Path)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if err = s.files.Delete(f.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}

// handleFileSSE godoc
// @Summary     Stream file parsing status
// @Tags        files
// @Security    BearerAuth
// @Param       id path string true "File ID"
// @Success     200
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /files/{id}.sse [get]
func (s *Server) handleFileSSE(c echo.Context) error {
	f, err := s.files.GetByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	if f.UserID != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	return sse.Stream(c, s.broker, f.ID)
}

// parseFile запускается в горутине — парсит файл и публикует события.
func (s *Server) parseFile(jobID string, f *store.File) {
	publish := func(evtType string, data any) {
		s.broker.Publish(f.ID, sse.Event{Type: evtType, Data: data})
		s.broker.Publish(jobID, sse.Event{Type: evtType, Data: data})
	}

	s.files.UpdateStatus(f.ID, "processing")
	publish("progress", map[string]any{"file_id": f.ID, "status": "processing"})

	// TODO: вызвать internal/parser

	s.files.UpdateStatus(f.ID, "done")
	s.jobs.UpdateFileStatus(jobID, f.ID, "done", nil)
	publish("done", map[string]any{"file_id": f.ID, "status": "done"})

	s.updateJobStatus(jobID)
}

func (s *Server) updateJobStatus(jobID string) {
	jfs, err := s.jobs.ListFiles(jobID)
	if err != nil {
		return
	}
	for _, jf := range jfs {
		if jf.Status == "pending" || jf.Status == "processing" {
			return
		}
	}
	s.jobs.UpdateStatus(jobID, "done")
	s.broker.Publish(jobID, sse.Event{Type: "done", Data: map[string]any{"job_id": jobID}})
}
