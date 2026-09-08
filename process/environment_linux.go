//go:build linux

package process

import (
	"bytes"
	"context"
	"os"
	"strconv"
)

func readEnvironment(ctx context.Context, pid int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(content, []byte{0})
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			entries = append(entries, string(part))
		}
	}
	return entries, nil
}
