package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/models"
)

func runBlockExpired(expiresAt *time.Time) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("server", &models.Server{ExpiresAt: expiresAt})
	blockExpiredServer(c)
	return w
}

func TestBlockExpiredServerAllowsServersWithoutExpiry(t *testing.T) {
	if w := runBlockExpired(nil); w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", w.Code)
	}
}

func TestBlockExpiredServerAllowsFutureExpiry(t *testing.T) {
	future := time.Now().Add(time.Hour)
	if w := runBlockExpired(&future); w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", w.Code)
	}
}
