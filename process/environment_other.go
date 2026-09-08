//go:build !linux && !darwin

package process

import (
	"context"

	gops "github.com/shirou/gopsutil/v3/process"
)

func readEnvironment(ctx context.Context, pid int) ([]string, error) {
	process, err := gops.NewProcessWithContext(ctx, int32(pid))
	if err != nil {
		return nil, err
	}
	return process.EnvironWithContext(ctx)
}
