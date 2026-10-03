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

func TestAllocateRangeSkipsLegacyAndReservedPorts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:allocation-range?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Allocation{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Server{Name: "legacy", Identifier: "legacy", Port: 5902, Type: "generic"}).Error; err != nil {
		t.Fatal(err)
	}

	service := &Allocation{DB: db}
	first, err := service.AllocateRange(models.LocalNode, "first", 5902, 5903)
	if err != nil {
		t.Fatal(err)
	}
	if first.Port != 5903 {
		t.Fatalf("expected legacy port 5902 to be skipped, got %d", first.Port)
	}
	if _, err = service.AllocateRange(models.LocalNode, "second", 5902, 5903); err == nil {
		t.Fatal("expected all ports in the range to be reserved")
	}
	if _, err = service.AllocateRangeWithPurpose(models.LocalNode, "vnc-server", 5904, 5904, "vnc"); err != nil {
		t.Fatal(err)
	}
	visible, err := service.List(models.LocalNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].Port != 5903 {
		t.Fatalf("expected only the normal port allocation to be visible, got %+v", visible)
	}
}
