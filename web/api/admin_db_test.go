package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func adminTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Server{Name: "s", Identifier: "srv1", Type: "generic"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("db", db) })
	r.PUT("/servers/:id/expiry", setServerExpiry)
	r.PUT("/servers/:id/backup-limit", setServerBackupLimit)
	r.GET("/servers/:id/limits", getServerLimits)
	return r, db
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestSetServerExpiryAndLimits(t *testing.T) {
	r, db := adminTestRouter(t)

	if w := doJSON(r, http.MethodPut, "/servers/srv1/expiry", `{"expiresAt":"2030-01-02T03:04:05Z"}`); w.Code != http.StatusNoContent {
		t.Fatalf("set expiry: status %d", w.Code)
	}
	if w := doJSON(r, http.MethodPut, "/servers/srv1/backup-limit", `{"limit":3}`); w.Code != http.StatusNoContent {
		t.Fatalf("set limit: status %d", w.Code)
	}
	var server models.Server
	if err := db.Where("identifier = ?", "srv1").First(&server).Error; err != nil {
		t.Fatal(err)
	}
	if server.ExpiresAt == nil || server.ExpiresAt.Year() != 2030 || server.BackupLimit != 3 {
		t.Fatalf("unexpected stored values: %+v", server)
	}

	if w := doJSON(r, http.MethodPut, "/servers/srv1/expiry", `{"expiresAt":null}`); w.Code != http.StatusNoContent {
		t.Fatalf("clear expiry: status %d", w.Code)
	}
	// Re-reading into the same struct keeps stale pointer values, so ask the database directly.
	var stillSet int64
	if err := db.Model(&models.Server{}).Where("identifier = ? AND expires_at IS NOT NULL", "srv1").Count(&stillSet).Error; err != nil {
		t.Fatal(err)
	}
	if stillSet != 0 {
		t.Fatal("expiry was not cleared")
	}
}

func TestAdminLimitEndpointsValidation(t *testing.T) {
	r, _ := adminTestRouter(t)

	if w := doJSON(r, http.MethodPut, "/servers/missing/expiry", `{"expiresAt":null}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown server: status %d", w.Code)
	}
	if w := doJSON(r, http.MethodPut, "/servers/srv1/backup-limit", `{"limit":5000}`); w.Code != http.StatusBadRequest {
		t.Fatalf("oversized limit: status %d", w.Code)
	}
	if w := doJSON(r, http.MethodGet, "/servers/srv1/limits", ""); w.Code != http.StatusOK {
		t.Fatalf("get limits: status %d", w.Code)
	}
}
