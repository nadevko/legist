package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

const refreshTTL = 7 * 24 * time.Hour

type registerRequest struct {
	Email    string `json:"email"    example:"user@example.com"`
	Password string `json:"password" example:"secret"`
}

type loginRequest struct {
	Email    string `json:"email"    example:"user@example.com"`
	Password string `json:"password" example:"secret"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" example:"a3f1..."`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type errorResponse struct {
	Message string `json:"message"`
}

// handleRegister godoc
// @Summary     Register a new user
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body registerRequest true "Email and password"
// @Success     201 {object} userResponse
// @Failure     400 {object} errorResponse
// @Failure     409 {object} errorResponse
// @Router      /users [post]
func (s *Server) handleRegister(c echo.Context) error {
	var body registerRequest
	if err := c.Bind(&body); err != nil || body.Email == "" || body.Password == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email and password required")
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	u := &store.User{ID: uuid.NewString(), Email: body.Email, Password: hash}
	if err = s.users.Create(u); err != nil {
		return echo.NewHTTPError(http.StatusConflict, "email already taken")
	}

	return c.JSON(http.StatusCreated, userResponse{ID: u.ID, Email: u.Email})
}

// handleLogin godoc
// @Summary     Login
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body loginRequest true "Email and password"
// @Success     200 {object} tokenResponse
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Router      /sessions [post]
func (s *Server) handleLogin(c echo.Context) error {
	var body loginRequest
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}

	u, err := s.users.GetByEmail(body.Email)
	if err != nil || !auth.CheckPassword(u.Password, body.Password) {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	accessToken, err := auth.NewAccessToken(u.ID, s.cfg.JWTSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	refreshToken, tokenHash, err := newRefreshToken()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	sess := &store.Session{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(refreshTTL),
	}
	if err = s.sessions.Create(sess); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	return c.JSON(http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// handleRefresh godoc
// @Summary     Refresh access token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body refreshRequest true "Refresh token"
// @Success     200 {object} tokenResponse
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Router      /tokens [post]
func (s *Server) handleRefresh(c echo.Context) error {
	var body refreshRequest
	if err := c.Bind(&body); err != nil || body.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refresh_token required")
	}

	hash := hashToken(body.RefreshToken)
	sess, err := s.sessions.GetByTokenHash(hash)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired refresh token")
	}

	accessToken, err := auth.NewAccessToken(sess.UserID, s.cfg.JWTSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	return c.JSON(http.StatusOK, tokenResponse{AccessToken: accessToken})
}

// handleLogout godoc
// @Summary     Logout
// @Tags        auth
// @Security    BearerAuth
// @Success     204
// @Failure     401 {object} errorResponse
// @Router      /sessions [delete]
func (s *Server) handleLogout(c echo.Context) error {
	if err := s.sessions.Delete(auth.UserID(c)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}

// handleMe godoc
// @Summary     Get current user
// @Tags        auth
// @Security    BearerAuth
// @Produce     json
// @Success     200 {object} userResponse
// @Failure     401 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Router      /me [get]
func (s *Server) handleMe(c echo.Context) error {
	u, err := s.users.GetByID(auth.UserID(c))
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.JSON(http.StatusOK, userResponse{ID: u.ID, Email: u.Email})
}

func newRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(b)
	hash = hashToken(token)
	return token, hash, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
