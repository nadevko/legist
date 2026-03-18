package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/store"
)

const passwordResetTTL = 15 * time.Minute

type passwordResetRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

type passwordResetResponse struct {
	ResetToken string `json:"reset_token"`
}

type changePasswordRequest struct {
	ResetToken  string `json:"reset_token"  example:"a3f1..."`
	NewPassword string `json:"new_password" example:"newsecret"`
}

// handleDeleteMe godoc
// @Summary     Delete current user
// @Tags        users
// @Security    BearerAuth
// @Success     204
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /me [delete]
func (s *Server) handleDeleteMe(c echo.Context) error {
	if err := s.users.Delete(auth.UserID(c)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	return c.NoContent(http.StatusNoContent)
}

// handleRequestPasswordReset godoc
// @Summary     Request password reset token
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       body body passwordResetRequest true "Email"
// @Success     200 {object} passwordResetResponse
// @Failure     400 {object} errorResponse
// @Failure     404 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /tokens/password-reset [post]
func (s *Server) handleRequestPasswordReset(c echo.Context) error {
	var body passwordResetRequest
	if err := c.Bind(&body); err != nil || body.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "email required")
	}

	u, err := s.users.GetByEmail(body.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	token, tokenHash, err := newRefreshToken()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	r := &store.PasswordReset{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(passwordResetTTL),
	}
	if err = s.resets.Create(r); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	return c.JSON(http.StatusOK, passwordResetResponse{ResetToken: token})
}

// handleChangePassword godoc
// @Summary     Change password using reset token
// @Tags        users
// @Accept      json
// @Produce     json
// @Param       body body changePasswordRequest true "Reset token and new password"
// @Success     204
// @Failure     400 {object} errorResponse
// @Failure     401 {object} errorResponse
// @Failure     500 {object} errorResponse
// @Router      /tokens/password-reset [patch]
func (s *Server) handleChangePassword(c echo.Context) error {
	var body changePasswordRequest
	if err := c.Bind(&body); err != nil || body.ResetToken == "" || body.NewPassword == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "reset_token and new_password required")
	}

	hash := hashToken(body.ResetToken)
	r, err := s.resets.GetByTokenHash(hash)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired reset token")
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	if err = s.users.UpdatePassword(r.UserID, newHash); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	if err = s.resets.Delete(r.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	return c.NoContent(http.StatusNoContent)
}
