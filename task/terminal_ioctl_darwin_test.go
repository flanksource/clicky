package task_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func getTerminalFlags(t *testing.T, fd int) terminalFlags {
	t.Helper()
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	require.NoError(t, err, "failed to get termios")
	return terminalFlags{
		iflag: uint64(termios.Iflag),
		oflag: uint64(termios.Oflag),
		lflag: uint64(termios.Lflag),
	}
}
