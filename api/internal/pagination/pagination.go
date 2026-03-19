package pagination

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Params — cursor-based pagination parameters.
type Params struct {
	Limit         int
	StartingAfter string
	EndingBefore  string
}

// Normalize clamps limit to [1, MaxLimit].
func (p *Params) Normalize() {
	if p.Limit <= 0 || p.Limit > MaxLimit {
		p.Limit = DefaultLimit
	}
}

// Response — стандартный list ответ в Stripe стиле.
type Response[T any] struct {
	Object     string `json:"object"` // "list"
	Data       []T    `json:"data"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Page обрабатывает количество элементов (limit+1) и возвращает:
// - items[:limit] - элементы для текущей страницы
// - nextCursor - ID последнего элемента (использовать как starting_after для следующей страницы)
// - hasMore - есть ли ещё элементы
// Store методы должны сами обрабатывать StartingAfter/EndingBefore в SQL.
func Page[T any](items []T, limit int, getID func(T) string) ([]T, string, bool) {
	nextCursor := ""
	hasMore := false
	if len(items) > limit {
		hasMore = true
		nextCursor = getID(items[limit-1])
		items = items[:limit]
	}
	return items, nextCursor, hasMore
}

// NewResponse применяет пагинацию и маппит элементы.
func NewResponse[S any, T any](items []S, limit int, mapper func(S) T) Response[T] {
	data := make([]T, len(items))
	for i, item := range items {
		data[i] = mapper(item)
	}
	return Response[T]{Object: "list", Data: data, HasMore: false}
}
