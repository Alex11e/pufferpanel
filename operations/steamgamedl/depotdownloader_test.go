package steamgamedl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLatestVersionFrom(t *testing.T) {
	t.Run("resolved depotdownloader asset for linux amd64", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
				t.Errorf("unexpected Accept header %q", got)
			}
			_, _ = w.Write([]byte(`[{"assets":[{"name":"DepotDownloader-windows-x64.zip","browser_download_url":"https://example.test/windows.zip"},{"name":"DepotDownloader-linux-arm64.zip","browser_download_url":"https://example.test/arm64.zip"},{"name":"DepotDownloader-linux-x64.zip","browser_download_url":"https://example.test/linux-x64.zip"}]}]`))
		}))
		defer server.Close()

		got, err := getLatestVersionFrom(server.Client(), server.URL, "linux", "amd64")
		if err != nil {
			t.Fatalf("getLatestVersionFrom() error = %v", err)
		}
		if got != "https://example.test/linux-x64.zip" {
			t.Fatalf("getLatestVersionFrom() = %q, want linux x64 download URL", got)
		}
	})

	t.Run("reports upstream status errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		if _, err := getLatestVersionFrom(server.Client(), server.URL, "linux", "amd64"); err == nil {
			t.Fatal("getLatestVersionFrom() should fail for a non-200 response")
		}
	})
}
