//go:build !windows

package daemon

import (
	"context"
	"net"
)

func dialPlayitAgent(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}
