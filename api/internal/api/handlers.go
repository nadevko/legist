package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// handleHealth — liveness probe.
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

// analyzeRequest — тело запроса POST /api/v1/analyze.
// Фронт присылает два файла multipart/form-data: old_doc и new_doc.
type analyzeRequest struct {
	OldDocName string `form:"old_doc_name"`
	NewDocName string `form:"new_doc_name"`
}

// handleAnalyze принимает два документа (.docx/.pdf) и ставит задачу в очередь.
// Возвращает report_id, по которому фронт потом поллит GET /api/v1/reports/:id.
func (s *Server) handleAnalyze(c echo.Context) error {
	var req analyzeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("bind request: %v", err))
	}

	oldFile, err := c.FormFile("old_doc")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "old_doc is required")
	}

	newFile, err := c.FormFile("new_doc")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "new_doc is required")
	}

	_ = oldFile
	_ = newFile

	// TODO: передать файлы в пайплайн (parser → diff → rag → ai → report)
	reportID := "stub-report-id"

	return c.JSON(http.StatusAccepted, echo.Map{
		"report_id": reportID,
		"status":    "processing",
	})
}

// handleGetReport возвращает итоговый JSON-отчёт по id.
func (s *Server) handleGetReport(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id is required")
	}

	// TODO: достать отчёт из хранилища по id
	return c.JSON(http.StatusOK, echo.Map{
		"id":     id,
		"status": "processing",
	})
}
