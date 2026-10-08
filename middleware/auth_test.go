package middleware

import (
	"net/http"
	"testing"
)

func TestIsCorsPreflightRequest(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		requestedMethod string
		want            bool
	}{
		{name: "browser preflight", method: http.MethodOptions, requestedMethod: http.MethodPost, want: true},
		{name: "ordinary options request", method: http.MethodOptions, want: false},
		{name: "actual post request", method: http.MethodPost, requestedMethod: http.MethodPost, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isCorsPreflightRequest(test.method, test.requestedMethod); got != test.want {
				t.Fatalf("isCorsPreflightRequest() = %t, want %t", got, test.want)
			}
		})
	}
}
