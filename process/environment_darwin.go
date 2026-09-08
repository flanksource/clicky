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
	index = firstNonEmpty(parts, index+1)
	for skipped := 0; skipped < argumentCount && index < len(parts); skipped++ {
		index = firstNonEmpty(parts, index+1)
	}
	entries := make([]string, 0, len(parts)-index)
	for ; index < len(parts); index++ {
		if len(parts[index]) > 0 {
			entries = append(entries, string(parts[index]))
		}
	}
	return entries, nil
}

func firstNonEmpty(parts [][]byte, start int) int {
	for start < len(parts) && len(parts[start]) == 0 {
		start++
	}
	return start
}
