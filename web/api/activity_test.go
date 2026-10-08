package api

import (
	"net/http"
	"testing"
)

func TestClassifyActivity(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantAction  string
		wantServer  string
		wantDetails string
	}{
		{name: "server created", method: http.MethodPut, path: "/api/servers/test-server", wantAction: "server.create", wantServer: "test-server"},
		{name: "server started", method: http.MethodPost, path: "/api/servers/test-server/start", wantAction: "server.start", wantServer: "test-server"},
		{name: "console command excludes command content", method: http.MethodPost, path: "/api/servers/test-server/console", wantAction: "server.console.command", wantServer: "test-server"},
		{name: "backup restored", method: http.MethodPost, path: "/api/servers/test-server/backup/restore/42", wantAction: "server.backup.restore", wantServer: "test-server", wantDetails: "42"},
		{name: "database linked", method: http.MethodPut, path: "/api/servers/test-server/database", wantAction: "server.database.attach", wantServer: "test-server"},
		{name: "database detached", method: http.MethodDelete, path: "/api/servers/test-server/database", wantAction: "server.database.detach", wantServer: "test-server"},
		{name: "file edited", method: http.MethodPut, path: "/api/servers/test-server/file/config/server.properties", wantAction: "server.file.write", wantServer: "test-server", wantDetails: "config/server.properties"},
		{name: "read is not a mutation", method: http.MethodGet, path: "/api/servers/test-server", wantServer: "test-server"},
		{name: "unrelated path ignored", method: http.MethodPost, path: "/api/users", wantServer: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, server, details := classifyActivity(test.method, test.path)
			if action != test.wantAction || server != test.wantServer || details != test.wantDetails {
				t.Fatalf("classifyActivity() = (%q, %q, %q), want (%q, %q, %q)", action, server, details, test.wantAction, test.wantServer, test.wantDetails)
			}
		})
	}
}
