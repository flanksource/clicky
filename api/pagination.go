package api

// PageInfo describes a buffered list window. A negative Total means unknown.
type PageInfo struct {
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Total  int64 `json:"total"`
}

type Paged interface {
	PageMetadata() PageInfo
	PageRows() any
}

const LLMPageSize = 25
