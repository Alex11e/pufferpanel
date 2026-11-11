package daemon

import "os"

func uploadOpenFlags(exclusive bool) int {
	if exclusive {
		return os.O_CREATE | os.O_EXCL | os.O_RDWR
	}
	return os.O_CREATE | os.O_TRUNC | os.O_RDWR
}
