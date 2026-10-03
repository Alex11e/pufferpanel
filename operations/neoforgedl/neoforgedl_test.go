package neoforgedl

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_getLatestForMCVersion(t *testing.T) {
	type args struct {
		minecraftVersion string
	}
	tests := []struct {
		name    string
		args    args
		wantVer bool
		wantErr bool
	}{
		{
			name:    "ValidateParser",
			args:    args{minecraftVersion: "1.20.4"},
			wantVer: true,
			wantErr: false,
		},
		{
			name:    "NotSupportedVersion",
			args:    args{minecraftVersion: "1.12.2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`
					<metadata>
						<versioning>
							<versions>
								<version>1.20.4-47.1.98</version>
								<version>1.20.4-47.1.100</version>
								<version>1.20.5-47.1.0</version>
							</versions>
						</versioning>
					</metadata>`))
			}))
			defer server.Close()

			got, err := getLatestForMCVersionFromURL(tt.args.minecraftVersion, server.URL)
			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestForMCVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantVer {
				if got == "" {
					t.Errorf("getLatestForMCVersion() got nothing, but wanted something")
				}
			} else {
				if got != "" {
					t.Errorf("getLatestForMCVersion() got %s, but wanted nothing", got)
				}
			}
		})
	}
}
