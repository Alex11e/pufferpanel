package api

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterContaboRoutes(t *testing.T) {
	router := gin.New()
	registerContabo(router.Group("/api/contabo"))

	options := make(map[string]int)
	for _, route := range router.Routes() {
		if route.Method == "OPTIONS" {
			options[route.Path]++
		}
	}
	for _, path := range []string{
		"/api/contabo/images",
		"/api/contabo/firewalls",
		"/api/contabo/firewalls/:id/instances/:instanceId",
	} {
		if options[path] != 1 {
			t.Fatalf("expected one OPTIONS handler for %s, got %d", path, options[path])
		}
	}
}
