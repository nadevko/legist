package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/nadevko/legist/internal/pagination"
)

// bindListParams parses and normalises pagination query params.
func bindListParams(c echo.Context) (listParams, error) {
	var p listParams
	if err := c.Bind(&p); err != nil {
		return p, errorf(http.StatusBadRequest, "invalid_request", "invalid query parameters")
	}
	p.normalize()
	return p, nil
}

// listResult applies pagination, maps items, and returns a listResponse.
// Store methods already filter by cursor in SQL; this just slices the +1 item
// and builds the has_more / next_cursor fields.
func listResult[S any, T any](
	items []S,
	limit int,
	mapper func(S) T,
	getID func(S) string,
) listResponse[T] {
	items, nextCursor, hasMore := pagination.Page(items, limit, getID)
	data := make([]T, len(items))
	for i, item := range items {
		data[i] = mapper(item)
	}
	return listResponse[T]{
		Object:     "list",
		Data:       data,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}
}
