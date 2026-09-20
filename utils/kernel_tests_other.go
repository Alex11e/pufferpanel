//go:build !linux

package utils

// openat2 is a Linux-only syscall. The panel's production Docker image runs on
// Linux; returning false here keeps cross-platform builds and tooling usable.
func testOpenat2() bool {
	useOpenat2 = false
	return false
}
