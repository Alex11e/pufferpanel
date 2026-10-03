package api

import "testing"

func TestHostingTemplates(t *testing.T) {
	for _, kind := range []string{"web", "mariadb", "postgres"} {
		template := hostingTemplate(kind)
		if template == nil {
			t.Fatalf("template %q missing", kind)
		}
		if _, ok := template.Variables["port"]; !ok {
			t.Errorf("%s: a port variable is required so the panel allocates a port", kind)
		}
		image, _ := template.Environment.Metadata["image"].(string)
		bindings, _ := template.Environment.Metadata["portBindings"].([]string)
		if image == "" || len(bindings) != 1 || bindings[0][:len("${ip}:${port}:")] != "${ip}:${port}:" {
			t.Errorf("%s: unexpected docker metadata %+v", kind, template.Environment.Metadata)
		}
	}
	if hostingTemplate("nope") != nil {
		t.Fatal("unknown kind should not resolve")
	}
	bot := hostingTemplate("discordbot")
	if bot == nil || bot.Type.Type != discordBotType {
		t.Fatal("Discord bot template should be available")
	}
	if token := bot.Variables["token"]; !token.Internal || token.UserEditable {
		t.Fatalf("Discord token must remain private: %+v", token)
	}
	if bot.Environment.Metadata["image"] != "node:22-alpine" || bot.Execution.EnvironmentVariables["DISCORD_TOKEN"] != "${token}" {
		t.Fatalf("unexpected Discord bot runtime: %+v", bot)
	}
	pythonBot := hostingTemplate("discordbot-python")
	if pythonBot == nil || pythonBot.Type.Type != discordBotType {
		t.Fatal("Python Discord bot template should be available")
	}
	if pythonBot.Environment.Metadata["image"] != "python:3.12-alpine" || pythonBot.Execution.EnvironmentVariables["DISCORD_TOKEN"] != "${token}" {
		t.Fatalf("unexpected Python Discord bot runtime: %+v", pythonBot)
	}
	pma := hostingTemplate("phpmyadmin")
	if pma == nil || pma.Execution.EnvironmentVariables["APACHE_PORT"] != "${port}" {
		t.Fatal("phpMyAdmin must listen on the allocated port")
	}
	// Versions are a fixed list so ${version} cannot select an arbitrary image.
	if v := hostingTemplate("mariadb").Variables["version"]; v.Type.Type != "option" || len(v.Options) == 0 {
		t.Fatalf("version must be an option variable: %+v", v)
	}
	if hostingTemplate("postgres").Variables["engine"].Internal != true {
		t.Fatal("engine must be internal")
	}
}
