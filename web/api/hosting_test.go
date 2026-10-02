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
