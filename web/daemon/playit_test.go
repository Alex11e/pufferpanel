package daemon

import (
	"context"
	"errors"
	"testing"
)

func TestPlayitIPCRequestReportsMissingAgent(t *testing.T) {
	_, err := playitIPCRequest(context.Background(), "get_state", nil)
	if !errors.Is(err, ErrPlayitAgentUnavailable) {
		t.Fatalf("playitIPCRequest() error = %v; want ErrPlayitAgentUnavailable", err)
	}
}
