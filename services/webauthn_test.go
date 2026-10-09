package services

import "testing"

func TestWebAuthnRPIDUsesHostnameWithoutPort(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "localhost with port", url: "http://localhost:8080", want: "localhost"},
		{name: "domain with port", url: "https://panel.example.com:8443", want: "panel.example.com"},
		{name: "IPv6 with port", url: "http://[::1]:8080", want: "::1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := webAuthnRPID(test.url)
			if err != nil {
				t.Fatalf("webAuthnRPID(%q) error = %v", test.url, err)
			}
			if got != test.want {
				t.Errorf("webAuthnRPID(%q) = %q, want %q", test.url, got, test.want)
			}
		})
	}
}
