package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// handleHealth godoc
// @Summary     Health check
// @Tags        System
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /health [get]
func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}
