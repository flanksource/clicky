//go:build windows

package process

import (
	"context"
	"fmt"
)

// Descriptors is not available on Windows, which has no lsof.
func Descriptors(_ context.Context, pid int) ([]Descriptor, error) {
	return nil, fmt.Errorf("listing open files of pid %d is not supported on windows", pid)
}
