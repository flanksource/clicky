package entity

// PageInfo carries the effective paging window for a list response.
type PageInfo struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

// Paged is implemented by list results that carry total-count metadata.
type Paged interface {
	PageMetadata() PageInfo
	PageRows() any
}

// PagedResult is a typed list response with paging metadata.
type PagedResult[T any] struct {
	Data []T      `json:"data"`
	Page PageInfo `json:"page"`
}

// NewPagedResult returns a paged result with a stable non-nil data array.
func NewPagedResult[T any](rows []T, limit, offset int, total int64) PagedResult[T] {
	if rows == nil {
		rows = []T{}
	}
	if offset < 0 {
		offset = 0
	}
	return PagedResult[T]{
		Data: rows,
		Page: PageInfo{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	}
}

func (p PagedResult[T]) PageMetadata() PageInfo {
	return p.Page
}

func (p PagedResult[T]) PageRows() any {
	return p.Data
}
