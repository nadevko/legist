package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/auth"
	"github.com/nadevko/legist/internal/pagination"
	"github.com/nadevko/legist/internal/store"
)

// bindListParams парсит и нормализует параметры пагинации из query string.
func bindListParams(c echo.Context) (listParams, error) {
	var p listParams
	if err := c.Bind(&p); err != nil {
		return p, errorf(http.StatusBadRequest, "invalid_request", "invalid query parameters")
	}
	p.normalize()
	return p, nil
}

// listResult применяет пагинацию, маппит элементы и возвращает listResponse.
func listResult[S any, T any](items []S, limit int, mapper func(S) T) listResponse[T] {
	items, hasMore := pagination.Page(items, limit)
	data := make([]T, len(items))
	for i, item := range items {
		data[i] = mapper(item)
	}
	return listResponse[T]{Object: "list", Data: data, HasMore: hasMore}
}

// parseExpand парсит query params expand[] в set.
func parseExpand(c echo.Context) map[string]bool {
	result := map[string]bool{}
	for _, v := range c.QueryParams()["expand[]"] {
		result[v] = true
	}
	return result
}

// ownerFilter возвращает FileFilter на основе query param owner.
// Без owner — файлы текущего юзера. owner=public — публичные (user_id IS NULL).
func ownerFilter(c echo.Context) (store.FileFilter, error) {
	filter := store.FileFilter{
		Status: c.QueryParam("status"),
	}
	switch c.QueryParam("owner") {
	case "public":
		// UserID остаётся nil → WHERE user_id IS NULL
	case "":
		userID := auth.UserID(c)
		filter.UserID = &userID
	default:
		return filter, errorf(http.StatusBadRequest, "invalid_parameter_value",
			"owner must be 'public'", "owner")
	}
	return filter, nil
}
