package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/buildkite/shellwords"
	"github.com/pufferpanel/pufferpanel/v3/models"
)

func TestPterodactylShellStartupIsWrapped(t *testing.T) {
	raw := json.RawMessage(`{
		"name":"VM",
		"startup":"qemu -m {{RAM}} $( [ \"${USE_KVM}\" == \"1\" ] && echo --enable-kvm ) -net user,{{FORWARD_PORTS}} --port {{server.build.default.port}}",
		"config":{"stop":"^^C"},
		"docker_images":{"qemu":"ghcr.io/example/vm:main"},
		"variables":[{"name":"RAM","env_variable":"RAM","default_value":"2048","rules":"required|integer"}]
	}`)
	template, err := pterodactylEggToTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	command, _ := template.Execution.Command.(string)
	parts, err := shellwords.Split(command)
	if err != nil || len(parts) != 3 || parts[0] != "bash" || parts[1] != "-c" {
		t.Fatalf("shell startup was not wrapped as bash -c <script>: %q (%v)", command, err)
	}
	script := parts[2]
	if !strings.Contains(script, "${RAM:-}") || !strings.Contains(script, "${SERVER_PORT:-}") || !strings.Contains(script, "${USE_KVM}") {
		t.Fatalf("tokens were not converted to environment lookups: %q", script)
	}
	if template.Execution.EnvironmentVariables["RAM"] != "${ram}" || template.Execution.EnvironmentVariables["SERVER_PORT"] != "${port}" {
		t.Fatalf("environment not mapped: %v", template.Execution.EnvironmentVariables)
	}
	if template.Execution.StopCode != 2 {
		t.Fatalf("expected SIGINT stop code, got %d", template.Execution.StopCode)
	}
	if template.SupportedEnvironments[0].Metadata["image"] != "ghcr.io/example/vm:main" {
		t.Fatalf("image missing from supported environments: %+v", template.SupportedEnvironments)
	}
}

func TestVpsTemplate(t *testing.T) {
	template := vpsTemplate()
	if template.Type.Type != vpsType {
		t.Fatalf("unexpected type %q", template.Type.Type)
	}
	for _, v := range vpsVariables {
		if _, ok := template.Variables[v.key]; !ok {
			t.Errorf("variable %q missing", v.key)
		}
		if template.Execution.EnvironmentVariables[v.env] != "${"+v.key+"}" {
			t.Errorf("environment %q not mapped", v.env)
		}
	}
	if len(template.Installation) != 1 || template.Installation[0].Type != "download" || len(template.Execution.PreExecution) != 1 {
		t.Fatal("a download install step and a pre-start step are required")
	}
	// A lowercase ${key} in a script would be replaced by the panel's own token substitution.
	for key := range template.Variables {
		if strings.Contains(vpsStartScript, "${"+key+"}") {
			t.Errorf("script contains panel token ${%s}", key)
		}
	}
}

func TestNormalizeVpsPortForwards(t *testing.T) {
	forwards, err := normalizeVpsPortForwards([]models.ServerPortForward{
		{GuestPort: 22, Protocol: "TCP"},
		{GuestPort: 25565, Protocol: "tcp,udp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if forwards[0].Protocol != "tcp" || forwards[1].Protocol != "tcp,udp" {
		t.Fatalf("protocols were not normalized: %+v", forwards)
	}

	if _, err = normalizeVpsPortForwards([]models.ServerPortForward{
		{GuestPort: 25565, Protocol: "tcp"},
		{GuestPort: 25565, Protocol: "udp"},
	}); err == nil {
		t.Fatal("expected duplicate guest ports to be rejected")
	}
	if _, err = normalizeVpsPortForwards([]models.ServerPortForward{{GuestPort: 22, Protocol: "icmp"}}); err == nil {
		t.Fatal("expected unsupported protocol to be rejected")
	}
}

func TestDefaultVpsPortForwardsUseTCPAndUDP(t *testing.T) {
	for _, forward := range defaultVpsPortForwards() {
		if forward.Protocol != "tcp,udp" {
			t.Errorf("default port %d uses %q instead of tcp,udp", forward.GuestPort, forward.Protocol)
		}
	}
}

func TestQemuPortForwardArgs(t *testing.T) {
	args := qemuPortForwardArgs([]models.Allocation{
		{Port: 3001, TargetPort: 22, Protocols: "tcp", Purpose: "forward"},
		{Port: 3002, TargetPort: 25565, Protocols: "tcp,udp", Purpose: "forward"},
		{Port: 5902, TargetPort: 0, Purpose: "vnc"},
	})
	want := "hostfwd=tcp::3001-:22,hostfwd=tcp::3002-:25565,hostfwd=udp::3002-:25565"
	if args != want {
		t.Fatalf("QEMU forwards = %q, want %q", args, want)
	}
}
