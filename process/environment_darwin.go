//go:build darwin

package process

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

func readEnvironment(ctx context.Context, pid int) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, err
	}
	return parseDarwinEnvironment(raw)
}

// parseDarwinEnvironment reads the environment out of a kern.procargs2
// payload: argc, the executable path and its NUL padding, argc arguments,
// the environment, an empty string ending it, then the apple strings
// (executable_file=, ptr_munge=, ...) the kernel passes, which are none of it.
func parseDarwinEnvironment(raw []byte) ([]string, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("kern.procargs2 returned %d bytes", len(raw))
	}
	argumentCount := int(binary.NativeEndian.Uint32(raw[:4]))
	parts := bytes.Split(raw[4:], []byte{0})
	index := firstNonEmpty(parts, 0)
	if index == len(parts) {
		return nil, fmt.Errorf("kern.procargs2 omitted executable path")
	}
	index = firstNonEmpty(parts, index+1) + argumentCount
	if index > len(parts) {
		return nil, fmt.Errorf("kern.procargs2 holds fewer than its %d arguments", argumentCount)
	}
	var entries []string
	for ; index < len(parts) && len(parts[index]) > 0; index++ {
		entries = append(entries, string(parts[index]))
	}
	return entries, nil
}

func firstNonEmpty(parts [][]byte, start int) int {
	for start < len(parts) && len(parts[start]) == 0 {
		start++
	}
	return start
}
