//go:build windows

package process

import (
	"context"
	"fmt"
)

// Descriptors is not available on Windows, which has no lsof.
func Descriptors(_ context.Context, pids ...int) (map[int][]Descriptor, error) {
	return nil, fmt.Errorf("listing open files of pids %v is not supported on windows", pids)
}
