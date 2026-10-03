package contabo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	uuid "github.com/gofrs/uuid/v5"
)

func TestClientAuthenticatesAndSendsRequestID(t *testing.T) {
	apiCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("grant_type") != "password" || request.Form.Get("username") != "api-user" {
				t.Errorf("unexpected OAuth form: %v", request.Form)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
		case "/v1/compute/instances":
			apiCalls++
			if request.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("unexpected authorization header %q", request.Header.Get("Authorization"))
			}
			if _, err := uuid.FromString(request.Header.Get("x-request-id")); err != nil {
				t.Errorf("invalid request id: %v", err)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[{"instanceId":42}]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(Config{
		ClientID: "client-id", ClientSecret: "client-secret", APIUser: "api-user", APIPassword: "password",
		APIURL: server.URL, AuthURL: server.URL + "/token",
	}, server.Client())
	first, err := client.Do(context.Background(), http.MethodGet, "/v1/compute/instances", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Do(context.Background(), http.MethodGet, "/v1/compute/instances", nil)
	if err != nil {
		t.Fatal(err)
	}
	if apiCalls != 2 {
		t.Fatalf("expected two API requests, got %d", apiCalls)
	}
	for _, response := range []json.RawMessage{first, second} {
		if !json.Valid(response) {
			t.Fatalf("invalid response JSON: %s", response)
		}
	}
}

func TestNewClientFromEnvRequiresAllCredentials(t *testing.T) {
	for _, key := range []string{"PUFFER_CONTABO_CLIENT_ID", "PUFFER_CONTABO_CLIENT_SECRET", "PUFFER_CONTABO_API_USER", "PUFFER_CONTABO_API_PASSWORD"} {
		t.Setenv(key, "")
	}
	if _, err := NewClientFromEnv(); err == nil {
		t.Fatal("expected missing credentials to be rejected")
	}
}
