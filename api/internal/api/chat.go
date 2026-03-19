package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type chatRequest struct {
	Message string `json:"message" example:"What does the Constitution say about the right to work?"`
}

type chatResponse struct {
	Object  string       `json:"object"` // "chat.completion"
	Answer  string       `json:"answer"`
	Sources []chatSource `json:"sources"`
}

type chatSource struct {
	Text    string `json:"text"`
	Source  string `json:"source"`
	Article string `json:"article"`
	Level   int    `json:"level"`
}

// handleChat godoc
// @Summary     Ask a question about laws (RAG-based Q&A)
// @Tags        chat
// @Security    BearerAuth
// @Accept      json
// @Produce     json
// @Produce     text/event-stream
// @Param       body   body   chatRequest true  "Question"
// @Param       Accept header string      false "application/json | text/event-stream"
// @Success     200 {object} chatResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /chat [post]
func (s *Server) handleChat(c echo.Context) error {
	var body chatRequest
	if err := c.Bind(&body); err != nil || body.Message == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "message is required", "message")
	}

	if c.Request().Header.Get("Accept") == "text/event-stream" {
		// TODO: SSE streaming from Ollama
		return nil
	}

	// TODO: embed → qdrant search → qwen2.5:7b
	return c.JSON(http.StatusOK, chatResponse{
		Object:  "chat.completion",
		Answer:  "",
		Sources: []chatSource{},
	})
}
