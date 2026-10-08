//go:build windows

package daemon

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

func dialPlayitAgent(ctx context.Context, path string) (net.Conn, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		timeout := 5 * time.Second
		return winio.DialPipe(path, &timeout)
	}
	timeout := time.Until(deadline)
	return winio.DialPipe(path, &timeout)
}
