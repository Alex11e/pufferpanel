package services

import "testing"

func TestPanelNodeAddress(t *testing.T) {
	tests := []struct {
		input string
		host  string
		port  uint16
		valid bool
	}{
		{"http://localhost:8080", "localhost", 8080, true},
		{"https://example.com/panel", "example.com", 443, true},
		{"http://192.0.2.1", "192.0.2.1", 80, true},
		{"https://[2001:db8::1]:8443", "2001:db8::1", 8443, true},
		{"http://[::1]", "::1", 80, true},
		{"http://example.com:0", "", 0, false},
		{"http://example.com:65536", "", 0, false},
		{"http://user:pass@example.com", "", 0, false},
		{"example.com", "", 0, false},
		{"http://[::1", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port, err := panelNodeAddress(tt.input)
			if (err == nil) != tt.valid || (tt.valid && (host != tt.host || port != tt.port)) {
				t.Fatalf("got (%q, %d, %v); want (%q, %d), valid=%v", host, port, err, tt.host, tt.port, tt.valid)
			}
		})
	}
}

func TestSFTPNodePort(t *testing.T) {
	for _, tt := range []struct {
		input string
		port  uint16
		valid bool
	}{
		{"0.0.0.0:5657", 5657, true},
		{"[::]:5658", 5658, true},
		{":22", 22, true},
		{"[::1]:0", 0, false},
		{"[::1]:65536", 0, false},
		{"::1:5657", 0, false},
	} {
		port, err := sftpNodePort(tt.input)
		if (err == nil) != tt.valid || (tt.valid && port != tt.port) {
			t.Errorf("%q: got %d, %v; want %d, valid=%v", tt.input, port, err, tt.port, tt.valid)
		}

	}
}

func TestNodeHostPort(t *testing.T) {
	for _, tt := range []struct {
		host string
		want string
	}{
		{"example.com", "example.com:8080"},
		{"192.0.2.1", "192.0.2.1:8080"},
		{"2001:db8::1", "[2001:db8::1]:8080"},
	} {
		if got := nodeHostPort(tt.host, 8080); got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.host, got, tt.want)
		}
	}
}
