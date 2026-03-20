package api

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

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

type updateUserRequest struct {
	Email *string `json:"email,omitempty" example:"new@example.com"`
	Role  *string `json:"role,omitempty"  example:"admin"`
}

// handleRegister godoc
// @Summary     Register a new user
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body            body   registerRequest true  "Email and password"
// @Param       Idempotency-Key header string          false "Idempotency key"
// @Success     201 {object} userResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     409 {object} apiErrorResponse
// @Router      /users [post]
func (s *Server) handleRegister(c echo.Context) error {
	var body registerRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "invalid request body")
	}
	if body.Email == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "email is required", "email")
	}
	if body.Password == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "password is required", "password")
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	role := store.RoleUser
	if s.cfg.Dev {
		role = store.RoleAdmin
	}
	u := &store.User{ID: newID("user"), Email: body.Email, Password: hash, Role: role}
	if err = s.users.Create(u); err != nil {
		if store.IsUniqueViolation(err) {
			return errorf(http.StatusConflict, "email_taken", "email already taken", "email")
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusCreated, toUserResponse(*u))
}

// handleLogin godoc
// @Summary     Login
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body loginRequest true "Email and password"
// @Success     200 {object} tokenResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Router      /sessions [post]
func (s *Server) handleLogin(c echo.Context) error {
	var body loginRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "invalid request body")
	}

	u, err := s.users.GetByEmail(body.Email)
	if err != nil || !auth.CheckPassword(u.Password, body.Password) {
		return errorf(http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	}

	accessToken, err := auth.NewAccessToken(u.ID, u.Role, s.cfg.JWTSecret)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	refreshToken, tokenHash, err := newRefreshToken()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	sess := &store.Session{
		ID:        newID("sess"),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(refreshTTL),
	}
	if err = s.sessions.Create(sess); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusOK, tokenResponse{
		Object:       "token",
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
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Router      /tokens/refresh [post]
func (s *Server) handleRefresh(c echo.Context) error {
	var body refreshRequest
	if err := c.Bind(&body); err != nil || body.RefreshToken == "" {
		return errorf(http.StatusBadRequest, "parameter_missing",
			"refresh_token is required", "refresh_token")
	}

	sess, err := s.sessions.GetByTokenHash(hashToken(body.RefreshToken))
	if err != nil {
		return errorf(http.StatusUnauthorized, "invalid_token",
			"invalid or expired refresh token", "refresh_token")
	}

	u, err := s.users.GetByID(sess.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusUnauthorized, "invalid_token",
			"invalid or expired refresh token", "refresh_token")
	}
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	accessToken, err := auth.NewAccessToken(u.ID, u.Role, s.cfg.JWTSecret)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusOK, tokenResponse{
		Object:      "token",
		AccessToken: accessToken,
	})
}

// handleLogout godoc
// @Summary     Logout (delete session)
// @Tags        auth
// @Security    BearerAuth
// @Param       id              path   string true  "Session ID"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /sessions/{id} [delete]
func (s *Server) handleLogout(c echo.Context) error {
	id := c.Param("id")
	if err := s.sessions.DeleteByID(id, auth.UserID(c)); err != nil {
		// DeleteByID uses WHERE id=? AND user_id=? — 0 rows affected = not found or not owner.
		// We return 404 in both cases to avoid leaking session existence.
		if errors.Is(err, store.ErrNotOwner) {
			return errorf(http.StatusNotFound, "resource_missing", "no such session: "+id)
		}
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "session"))
}

// handleListSessions godoc
// @Summary     List active sessions
// @Tags        auth
// @Security    BearerAuth
// @Produce     json
// @Param       limit          query int    false "Limit"
// @Param       starting_after query string false "Cursor"
// @Param       ending_before  query string false "Cursor"
// @Success     200 {object} listResponse[sessionResponse]
// @Failure     401 {object} apiErrorResponse
// @Router      /sessions [get]
func (s *Server) handleListSessions(c echo.Context) error {
	p, err := bindListParams(c)
	if err != nil {
		return err
	}
	sessions, err := s.sessions.ListByUser(auth.UserID(c), p.toStore())
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, listResult(sessions, p.Limit,
		toSessionResponse, func(s store.Session) string { return s.ID }))
}

// handleGetUser godoc
// @Summary     Get user
// @Tags        users
// @Security    BearerAuth
// @Param       id path string true "User ID or 'me'"
// @Produce     json
// @Success     200 {object} userResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Router      /users/{id} [get]
func (s *Server) handleGetUser(c echo.Context) error {
	id := resolveUserID(c)
	if auth.UserID(c) != id && !auth.IsAdmin(c) {
		return errorf(http.StatusNotFound, "resource_missing", "no such user: "+id)
	}
	u, err := s.users.GetByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusNotFound, "resource_missing", "no such user: "+id)
	}
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, toUserResponse(*u))
}

// handleUpdateUser godoc
// @Summary     Update user (email for self; role for admins)
// @Tags        users
// @Security    BearerAuth
// @Param       id              path   string            true  "User ID or 'me'"
// @Param       body            body   updateUserRequest true  "Fields to update (email and/or role)"
// @Param       Idempotency-Key header string            false "Idempotency key"
// @Accept      json
// @Produce     json
// @Success     200 {object} userResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     409 {object} apiErrorResponse
// @Router      /users/{id} [patch]
func (s *Server) handleUpdateUser(c echo.Context) error {
	targetID := resolveUserID(c)
	caller := auth.UserID(c)
	isAdmin := auth.IsAdmin(c)

	if targetID != caller && !isAdmin {
		return errorf(http.StatusNotFound, "resource_missing", "no such user: "+targetID)
	}

	var body updateUserRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "invalid_request", "invalid body")
	}
	if body.Email == nil && body.Role == nil {
		return errorf(http.StatusBadRequest, "parameter_missing",
			"at least one of email or role is required")
	}

	if body.Role != nil && !isAdmin {
		return errorf(http.StatusBadRequest, "invalid_parameter_value",
			"only admins may change role", "role")
	}

	if body.Email != nil && targetID != caller {
		return errorf(http.StatusBadRequest, "invalid_request",
			"cannot change another user's email", "email")
	}

	if targetID != caller && isAdmin {
		if body.Email != nil {
			return errorf(http.StatusBadRequest, "invalid_request",
				"cannot change another user's email", "email")
		}
		if body.Role == nil {
			return errorf(http.StatusBadRequest, "parameter_missing",
				"role is required when updating another user", "role")
		}
	}

	if body.Email != nil {
		em := strings.TrimSpace(*body.Email)
		if em == "" {
			return errorf(http.StatusBadRequest, "parameter_missing", "email is required", "email")
		}
		if err := s.users.UpdateEmail(targetID, em); err != nil {
			if store.IsUniqueViolation(err) {
				return errorf(http.StatusConflict, "email_taken", "email already taken", "email")
			}
			return errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
	}

	if body.Role != nil {
		r := strings.TrimSpace(*body.Role)
		if r != store.RoleUser && r != store.RoleAdmin {
			return errorf(http.StatusBadRequest, "invalid_parameter_value",
				"role must be 'user' or 'admin'", "role")
		}
		if err := s.users.UpdateRole(targetID, r); err != nil {
			return errorf(http.StatusInternalServerError, "server_error", "internal error")
		}
	}

	u, err := s.users.GetByID(targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return errorf(http.StatusNotFound, "resource_missing", "no such user: "+targetID)
	}
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, toUserResponse(*u))
}

// handleDeleteUser godoc
// @Summary     Delete user
// @Tags        users
// @Security    BearerAuth
// @Param       id              path   string true  "User ID or 'me'"
// @Param       Idempotency-Key header string false "Idempotency key"
// @Success     200 {object} deletedResponse
// @Failure     403 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Router      /users/{id} [delete]
func (s *Server) handleDeleteUser(c echo.Context) error {
	id := resolveUserID(c)
	if err := requireSelf(c, id); err != nil {
		return err
	}
	if err := s.users.Delete(id); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}
	return c.JSON(http.StatusOK, deleted(id, "user"))
}

// --- helpers ---

// resolveUserID returns the user ID from path param, substituting the
// authenticated user's ID when param is empty or "me".
func resolveUserID(c echo.Context) string {
	id := c.Param("id")
	if id == "" || id == "me" {
		return auth.UserID(c)
	}
	return id
}

// requireSelf returns 403 if the authenticated user is trying to act on
// another user's account. We return 404 shape to avoid leaking user existence.
func requireSelf(c echo.Context, targetID string) error {
	if auth.UserID(c) != targetID {
		return errorf(http.StatusNotFound, "resource_missing", "no such user: "+targetID)
	}
	return nil
}

func toUserResponse(u store.User) userResponse {
	role := u.Role
	if role == "" {
		role = store.RoleUser
	}
	return userResponse{
		ID:      u.ID,
		Object:  "user",
		Email:   u.Email,
		Role:    role,
		Created: toUnix(u.CreatedAt),
	}
}

func toSessionResponse(s store.Session) sessionResponse {
	return sessionResponse{
		ID:        s.ID,
		Object:    "session",
		UserID:    s.UserID,
		ExpiresAt: toUnix(s.ExpiresAt),
		Created:   toUnix(s.CreatedAt),
	}
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
