package api

import "testing"

func TestNormalizeSocialRedirectBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty uses panel fallback", input: "", want: ""},
		{name: "public https domain", input: "https://kezelopanel.elementalhost.eu/", want: "https://kezelopanel.elementalhost.eu"},
		{name: "localhost http for development", input: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "private http ip rejected", input: "http://192.168.206.10", wantErr: true},
		{name: "path rejected", input: "https://kezelopanel.elementalhost.eu/panel", wantErr: true},
		{name: "query rejected", input: "https://kezelopanel.elementalhost.eu?next=/", wantErr: true},
		{name: "userinfo rejected", input: "https://admin:secret@kezelopanel.elementalhost.eu", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeSocialRedirectBaseURL(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("normalized URL = %q, want %q", got, test.want)
			}
		})
	}
}
