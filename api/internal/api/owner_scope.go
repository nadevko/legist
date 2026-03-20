package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
)

// ownerListKind selects how list endpoints filter by resource owner (user_id).
type ownerListKind int

const (
	ownerListSelfOnly ownerListKind = iota
	ownerListPublicOnly
	ownerListSelfAndPublic
)

// resolveOwnerListQuery parses ?owner= for Stripe-like list semantics:
// - omitted: non-admin → own resources only; admin → own + public (user_id IS NULL)
// - "null": public only; admin only
// - own user id: filter to that user (must match caller; no other ids allowed)
func resolveOwnerListQuery(c echo.Context) (ownerListKind, string, error) {
	ownerRaw := strings.TrimSpace(c.QueryParam("owner"))
	uid := auth.UserID(c)
	isAdmin := auth.IsAdmin(c)

	switch ownerRaw {
	case "":
		if isAdmin {
			return ownerListSelfAndPublic, uid, nil
		}
		return ownerListSelfOnly, uid, nil
	case "null":
		if !isAdmin {
			return 0, "", errorf(http.StatusBadRequest, "invalid_parameter_value",
				"owner=null is only allowed for admins", "owner")
		}
		return ownerListPublicOnly, "", nil
	default:
		if ownerRaw != uid {
			return 0, "", errorf(http.StatusBadRequest, "invalid_parameter_value",
				"owner must be your user id or null (admins only)", "owner")
		}
		return ownerListSelfOnly, uid, nil
	}
}
