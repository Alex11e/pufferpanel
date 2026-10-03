package services

import (
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateServersWithoutSubdomain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:servers-without-subdomain?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Server{}); err != nil {
		t.Fatal(err)
	}

	service := &Server{DB: db}
	for _, identifier := range []string{"first", "second"} {
		if err = service.Create(&models.Server{Name: identifier, Identifier: identifier, Type: "generic"}); err != nil {
			t.Fatalf("create server %q without a subdomain: %v", identifier, err)
		}
	}
}
