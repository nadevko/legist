package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

// ── AI Config ─────────────────────────────────────────────

const defaultAIURL = "https://api.intelligence.io.solutions/api/v1/chat/completions"
const defaultAIModel = "moonshotai/Kimi-K2.5"
const defaultAIKey = "io-v2-eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJvd25lciI6IjczZjRmODdjLTI0N2MtNGRmOS1hZTcyLWNkZWU0MWQ4ZDJkYSIsImV4cCI6NDkyNzUwOTE0NH0.MAfbt3YSlIsp7OS8zHr2u6BwyK45-ckVp6PKG7DXs3oWDcapCjcpUBnlBUJB6xAP2G0M0Fm1wjPbJDVhIHJvAg"

const systemPrompt = `Ты — профессиональный юридический AI-ассистент платформы «Легист.бел».
Твоя специализация — анализ нормативных правовых актов Республики Беларусь.

Твои возможности:
1. Сравнение редакций НПА — выявление структурных и семантических изменений.
2. Проверка соответствия Трудовому кодексу РБ и базовому законодательству.
3. Выявление «красных зон» — потенциальных нарушений прав или законодательства.
4. Формирование юридических заключений.

Правила ответа:
- Отвечай на русском языке.
- Будь точным и конкретным, ссылайся на статьи законов РБ.
- Используй форматирование: **жирный** для важных терминов.
- Структурируй ответы по пунктам.
- Если не уверен — честно скажи об этом.`

func getAIConfig() (apiURL, apiKey, model string) {
	apiURL = os.Getenv("AI_API_URL")
	if apiURL == "" {
		apiURL = defaultAIURL
	}
	apiKey = os.Getenv("AI_API_KEY")
	if apiKey == "" {
		apiKey = defaultAIKey
	}
	model = os.Getenv("AI_MODEL")
	if model == "" {
		model = defaultAIModel
	}
	return
}

// ── OpenAI-compatible request/response types ──────────────

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiRequest struct {
	Model               string      `json:"model"`
	Messages            []aiMessage `json:"messages"`
	Temperature         float64     `json:"temperature"`
	MaxCompletionTokens int         `json:"max_completion_tokens"`
}

type aiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ── Handler ───────────────────────────────────────────────

// handleChat godoc
// @Summary     Ask a question about laws (RAG-based Q&A)
// @Tags        Chat
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
		return s.handleChatSSE(c, body.Message)
	}

	answer, err := callAI(body.Message)
	if err != nil {
		return errorf(http.StatusInternalServerError, "ai_error", err.Error(), "")
	}

	return c.JSON(http.StatusOK, chatResponse{
		Object:  "chat.completion",
		Answer:  answer,
		Sources: []chatSource{},
	})
}

// handleChatSSE streams the AI response as SSE
func (s *Server) handleChatSSE(c echo.Context, message string) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	answer, err := callAI(message)
	if err != nil {
		fmt.Fprintf(c.Response(), "data: {\"error\": \"%s\"}\n\n", err.Error())
		c.Response().Flush()
		return nil
	}

	// Отправляем ответ целиком (для потоковой передачи нужна поддержка streaming в AI API)
	chunk, _ := json.Marshal(map[string]string{"answer": answer})
	fmt.Fprintf(c.Response(), "data: %s\n\n", chunk)
	c.Response().Flush()

	return nil
}

// callAI вызывает intelligence.io API (OpenAI-compatible)
func callAI(userMessage string) (string, error) {
	apiURL, apiKey, model := getAIConfig()

	reqBody := aiRequest{
		Model: model,
		Messages: []aiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature:         0.1,
		MaxCompletionTokens: 3000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read AI response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI returned status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var aiResp aiResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return "", fmt.Errorf("failed to parse AI response: %w", err)
	}

	if aiResp.Error != nil {
		return "", fmt.Errorf("AI error: %s", aiResp.Error.Message)
	}

	if len(aiResp.Choices) == 0 || aiResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("AI returned empty response")
	}

	return strings.TrimSpace(aiResp.Choices[0].Message.Content), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
