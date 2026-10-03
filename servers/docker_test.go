package servers

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
)

func TestExecutionCommandPreservesImageEntrypoint(t *testing.T) {
	config := container.Config{Entrypoint: strslice.StrSlice{"/usr/local/bin/docker-entrypoint.sh"}}
	command := strslice.StrSlice{"mariadbd", "--console"}

	applyExecutionCommand(&config, command)

	if len(config.Entrypoint) != 1 || config.Entrypoint[0] != "/usr/local/bin/docker-entrypoint.sh" {
		t.Fatalf("image entrypoint was changed: %+v", config.Entrypoint)
	}
	if len(config.Cmd) != 2 || config.Cmd[0] != "mariadbd" || config.Cmd[1] != "--console" {
		t.Fatalf("execution command was not assigned as Cmd: %+v", config.Cmd)
	}
}