package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/config"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSocialFlowStateIsConsumedOnceAcrossHosts(t *testing.T) {
	previous := config.SessionKey.Value()
	if err := config.SessionKey.Set(strings.Repeat("22", 32), false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.SessionKey.Set(previous, false) })

	db, err := gorm.Open(sqlite.Open("file:social-flow-once?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.SocialLoginFlow{}); err != nil {
		t.Fatal(err)
	}
	want := socialFlow{
		State: "opaque-state", Verifier: "pkce-verifier", Provider: "google",
		Mode: "connect", UserID: 42, SessionTokenHash: strings.Repeat("a", 64),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := storeSocialFlow(db, want); err != nil {
		t.Fatal(err)
	}
	got, err := consumeSocialFlow(db, want.State)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != want.State || got.Verifier != want.Verifier || got.Provider != want.Provider || got.Mode != want.Mode || got.UserID != want.UserID || got.SessionTokenHash != want.SessionTokenHash || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("consumed flow = %#v, want %#v", got, want)
	}
	if _, err := consumeSocialFlow(db, want.State); err == nil {
		t.Fatal("OAuth state was accepted more than once")
	}
}

func TestSocialRegistrationRequiresBothPolicies(t *testing.T) {
	previousRegistration := config.RegistrationEnabled.Value()
	previousSocialRegistration := config.SocialLoginAllowRegistration.Value()
	t.Cleanup(func() {
		_ = config.RegistrationEnabled.Set(previousRegistration, false)
		_ = config.SocialLoginAllowRegistration.Set(previousSocialRegistration, false)
	})

	tests := []struct {
		name               string
		registration       bool
		socialRegistration bool
		want               bool
	}{
		{name: "both enabled", registration: true, socialRegistration: true, want: true},
		{name: "panel registration disabled", registration: false, socialRegistration: true},
		{name: "social registration disabled", registration: true, socialRegistration: false},
		{name: "both disabled", registration: false, socialRegistration: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := config.RegistrationEnabled.Set(test.registration, false); err != nil {
				t.Fatal(err)
			}
			if err := config.SocialLoginAllowRegistration.Set(test.socialRegistration, false); err != nil {
				t.Fatal(err)
			}
			if got := socialRegistrationAllowed(); got != test.want {
				t.Fatalf("socialRegistrationAllowed() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSocialConnectFailureReturnsReasonToSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/callback", func(c *gin.Context) {
		socialFlowFailure(c, "connect", "exchange", nil)
	})
	request := httptest.NewRequest(http.MethodGet, "/callback", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusFound)
	}
	location := response.Header().Get("Location")
	if !strings.Contains(location, "/self?socialError=exchange#social") {
		t.Fatalf("location = %q, want exchange reason on social settings", location)
	}
}
