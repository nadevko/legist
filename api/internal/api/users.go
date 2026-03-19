package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

const passwordResetTTL = 15 * time.Minute

type passwordResetRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

type passwordResetResponse struct {
	Object     string `json:"object"` // "token.password_reset"
	ResetToken string `json:"reset_token"`
}

type changePasswordRequest struct {
	ResetToken  string `json:"reset_token"  example:"a3f1..."`
	NewPassword string `json:"new_password" example:"newsecret"`
}

// handleRequestPasswordReset godoc
// @Summary     Request password reset token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body            body   passwordResetRequest true  "Email"
// @Param       Idempotency-Key header string               false "Idempotency key"
// @Success     200 {object} passwordResetResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     404 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /tokens/password-reset [post]
func (s *Server) handleRequestPasswordReset(c echo.Context) error {
	var body passwordResetRequest
	if err := c.Bind(&body); err != nil || body.Email == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "email is required", "email")
	}

	u, err := s.users.GetByEmail(body.Email)
	if err != nil {
		return errorf(http.StatusNotFound, "resource_missing", "no such user with email: "+body.Email, "email")
	}

	token, tokenHash, err := newRefreshToken()
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	r := &store.PasswordReset{
		ID:        newID("pwdr"),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTTL),
	}
	if err = s.resets.Create(r); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusOK, passwordResetResponse{
		Object:     "token.password_reset",
		ResetToken: token,
	})
}

// handleChangePassword godoc
// @Summary     Change password using reset token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body changePasswordRequest true "Reset token and new password"
// @Success     200 {object} deletedResponse
// @Failure     400 {object} apiErrorResponse
// @Failure     401 {object} apiErrorResponse
// @Failure     500 {object} apiErrorResponse
// @Router      /tokens/password-reset [patch]
func (s *Server) handleChangePassword(c echo.Context) error {
	var body changePasswordRequest
	if err := c.Bind(&body); err != nil {
		return errorf(http.StatusBadRequest, "parameter_missing", "invalid request body")
	}
	if body.ResetToken == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "reset_token is required", "reset_token")
	}
	if body.NewPassword == "" {
		return errorf(http.StatusBadRequest, "parameter_missing", "new_password is required", "new_password")
	}

	r, err := s.resets.GetByTokenHash(hashToken(body.ResetToken))
	if err != nil {
		return errorf(http.StatusUnauthorized, "invalid_token", "invalid or expired reset token", "reset_token")
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	if err = s.users.UpdatePassword(r.UserID, newHash); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	if err = s.resets.Delete(r.ID); err != nil {
		return errorf(http.StatusInternalServerError, "server_error", "internal error")
	}

	return c.JSON(http.StatusOK, deleted(r.ID, "token.password_reset"))
}
