package task

import "errors"

var (
	// ErrBatchTimeout is returned when the entire batch exceeds its timeout
	ErrBatchTimeout = errors.New("batch execution exceeded timeout")

	// ErrItemTimeout is returned when an individual item exceeds its timeout
	ErrItemTimeout = errors.New("item execution exceeded timeout")
)
