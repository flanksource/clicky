//go:build windows

package clicky

import (
	"syscall"
	"time"
)

func readPromptCursorResponse(fd int, timeout time.Duration) ([]byte, bool) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	bytes := make(chan byte, 32)
	errs := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		handle := syscall.Handle(fd)
		tmp := []byte{0}
		for {
			n, err := syscall.Read(handle, tmp)
			if err != nil {
				select {
				case errs <- struct{}{}:
				default:
				}
				return
			}
			if n == 0 {
				continue
			}
			select {
			case bytes <- tmp[0]:
			case <-done:
				return
			}
		}
	}()

	buf := make([]byte, 0, 32)
	for {
		select {
		case <-deadline.C:
			return nil, false
		case <-errs:
			return nil, false
		case b := <-bytes:
			buf = append(buf, b)
			if b == 'R' {
				return buf, true
			}
		}
	}
}
