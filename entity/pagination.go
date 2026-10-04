package entity

import "github.com/flanksource/clicky/api"

// PagedResult is a typed list response with paging metadata.
type PagedResult[T any] struct {
	Data []T          `json:"data"`
	Page api.PageInfo `json:"page"`
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
		Page: api.PageInfo{
			Limit:  limit,
			Offset: offset,
			Total:  total,
		},
	}
}

func (p PagedResult[T]) PageMetadata() api.PageInfo {
	return p.Page
}

func (p PagedResult[T]) PageRows() any {
	return p.Data
}
