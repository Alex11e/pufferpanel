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

func TestPterodactylEggConversionInstallScript(t *testing.T) {
	raw := json.RawMessage(`{"name":"S","startup":"run","scripts":{"installation":{"script":"cd /mnt/server\necho hi"}}}`)
	template, err := pterodactylEggToTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Installation) != 2 || template.Installation[0].Metadata["text"] != "cd /pufferpanel\necho hi" {
		t.Fatalf("unexpected install: %+v", template.Installation)
	}
}

func TestPterodactylEggConversionLegacyFormat(t *testing.T) {
	raw := json.RawMessage(`{
		"name":"Legacy",
		"startup":"run {{server.build.default.port}} {{server.build.env.SLOTS}}",
		"images":["ghcr.io/example/legacy:1"],
		"config":{"stop":"^C"},
		"variables":[
			{"name":"Slots","env_variable":"SLOTS","default_value":"20","rules":"required|numeric","user_editable":false},
			{"name":"Flag","env_variable":"FLAG","default_value":"1","rules":"boolean"}
		]
	}`)

	template, err := pterodactylEggToTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if template.Execution.Command != "run ${port} ${slots}" {
		t.Fatalf("unexpected command: %q", template.Execution.Command)
	}
	if template.Environment.Metadata["image"] != "ghcr.io/example/legacy:1" {
		t.Fatalf("unexpected image: %v", template.Environment.Metadata["image"])
	}
	if template.Execution.StopCommand != "^C" {
		t.Fatalf("unexpected stop: %q", template.Execution.StopCommand)
	}
	if v := template.Variables["slots"]; v.Type.Type != "integer" || v.UserEditable {
		t.Fatalf("unexpected slots variable: %+v", v)
	}
	if v := template.Variables["flag"]; v.Type.Type != "boolean" || v.Value != true {
		t.Fatalf("unexpected flag variable: %+v", v)
	}
}
