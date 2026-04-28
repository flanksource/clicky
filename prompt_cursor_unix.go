//go:build !windows

package clicky

import (
	"time"

	"golang.org/x/sys/unix"
)

func readPromptCursorResponse(fd int, timeout time.Duration) ([]byte, bool) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, 32)
	tmp := []byte{0}

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		timeoutMillis := 1
		if remaining > 0 {
			timeoutMillis = int(remaining / time.Millisecond)
			if timeoutMillis < 1 {
				timeoutMillis = 1
			}
		}

		pollfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(pollfds, timeoutMillis)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return nil, false
		}
		if ready == 0 || pollfds[0].Revents&unix.POLLIN == 0 {
			continue
		}

		n, err := unix.Read(fd, tmp)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return nil, false
		}
		if n == 0 {
			continue
		}

		buf = append(buf, tmp[0])
		if tmp[0] == 'R' {
			return buf, true
		}
	}

	return nil, false
}
