package api

import (
	"encoding/json"
	"testing"
)

func TestPterodactylEggConversionPreservesVariableRequirements(t *testing.T) {
	raw := json.RawMessage(`{
		"name":"Paper server",
		"startup":"java -jar server.jar --port {{SERVER_PORT}} --memory {{SERVER_MEMORY}}",
		"docker_images":{"Java 21":"ghcr.io/example/java:21"},
		"variables":[
			{"name":"Memory","env_variable":"SERVER_MEMORY","default_value":"1024M","rules":"required|string"},
			{"name":"MOTD","env_variable":"SERVER_MOTD","default_value":"Hello","rules":"nullable|string"}
		]
	}`)

	template, err := pterodactylEggToTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if template.Identifier != "paper-server" {
		t.Fatalf("unexpected identifier: %q", template.Identifier)
	}
	if template.Execution.Command != "java -jar server.jar --port ${port} --memory ${server_memory}" {
		t.Fatalf("unexpected converted command: %q", template.Execution.Command)
	}
	if !template.Variables["server_memory"].Required {
		t.Fatal("required Pterodactyl variable must remain required")
	}
	if template.Variables["server_motd"].Required {
		t.Fatal("nullable Pterodactyl variable must remain optional")
	}
	if template.Execution.EnvironmentVariables["SERVER_MEMORY"] != "${server_memory}" {
		t.Fatal("environment variable was not mapped")
	}
}
