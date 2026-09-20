//go:build windows

package logging

import "os"

// Windows has no SIGUSR1 equivalent. Log output remains available and file
// rotation can be handled by the hosting process without a signal listener.
func logRotationSignal() os.Signal {
	return nil
}
