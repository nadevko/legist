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

// Page срезает limit+1 элемент и возвращает has_more.
func Page[T any](items []T, limit int) ([]T, bool) {
	if len(items) > limit {
		return items[:limit], true
	}
	return items, false
}

// Response — стандартный list ответ в Stripe стиле.
type Response[T any] struct {
	Object  string `json:"object"` // "list"
	Data    []T    `json:"data"`
	HasMore bool   `json:"has_more"`
}

// NewResponse применяет пагинацию и маппит элементы.
func NewResponse[S any, T any](items []S, limit int, mapper func(S) T) Response[T] {
	items, hasMore := Page(items, limit)
	data := make([]T, len(items))
	for i, item := range items {
		data[i] = mapper(item)
	}
	if data == nil {
		data = []T{}
	}
	return Response[T]{Object: "list", Data: data, HasMore: hasMore}
}
