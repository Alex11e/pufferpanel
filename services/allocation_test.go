package services

import (
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAllocateNextLocalNodeStartsAtRangeAndSkipsLegacyPorts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:allocation-local?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Allocation{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Server{Name: "existing", Identifier: "existing", Port: 1000, Type: "generic"}).Error; err != nil {
		t.Fatal(err)
	}

	allocation, err := (&Allocation{DB: db}).AllocateNext(models.LocalNode, "new-server")
	if err != nil {
		t.Fatal(err)
	}
	if allocation.Port != 1001 {
		t.Fatalf("expected first free LocalNode port 1001, got %d", allocation.Port)
	}
	if allocation.Protocols != "tcp,udp" {
		t.Fatalf("expected TCP and UDP reservation, got %q", allocation.Protocols)
	}
}
