//go:build !windows

package logging

import (
	"os"
	"syscall"
)

func logRotationSignal() os.Signal {
	return syscall.SIGUSR1
}
