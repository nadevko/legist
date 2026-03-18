package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/sse"
)

// handleGetJob godoc
// @Summary     Get job status
// @Tags        jobs
// @Security    BearerAuth
// @Param       id path string true "Job ID"
// @Produce     json
// @Success     200 {object} jobResponse
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /jobs/{id} [get]
func (s *Server) handleGetJob(c echo.Context) error {
	j, err := s.jobs.GetByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	if j.UserID != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	jfs, err := s.jobs.ListFiles(j.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	files := make([]jobFileResponse, len(jfs))
	for i, jf := range jfs {
		files[i] = jobFileResponse{
			FileID: jf.FileID,
			Status: jf.Status,
			Error:  jf.Error,
		}
	}

	return c.JSON(http.StatusOK, jobResponse{
		ID:        j.ID,
		Type:      j.Type,
		Status:    j.Status,
		CreatedAt: j.CreatedAt.Format(time.RFC3339),
		ExpiresAt: j.ExpiresAt.Format(time.RFC3339),
		Files:     files,
	})
}

// handleJobSSE godoc
// @Summary     Stream job progress
// @Tags        jobs
// @Security    BearerAuth
// @Param       id path string true "Job ID"
// @Success     200
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /jobs/{id}.sse [get]
func (s *Server) handleJobSSE(c echo.Context) error {
	j, err := s.jobs.GetByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	if j.UserID != auth.UserID(c) {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	return sse.Stream(c, s.broker, j.ID)
}
