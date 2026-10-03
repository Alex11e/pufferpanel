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

func TestAllocateForwardStoresGuestPortAndProtocol(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:allocation-forward?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Allocation{}); err != nil {
		t.Fatal(err)
	}

	node := *models.LocalNode
	node.PortRangeStart = 25565
	node.PortRangeEnd = 25566
	service := &Allocation{DB: db}

	forward, err := service.AllocateForward(&node, "vps-one", 22, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if forward.Port != 25565 || forward.TargetPort != 22 || forward.Protocols != "tcp" || forward.Purpose != "forward" {
		t.Fatalf("unexpected forwarded allocation: %+v", forward)
	}

	second, err := service.AllocateForward(&node, "vps-one", 25565, "tcp,udp")
	if err != nil {
		t.Fatal(err)
	}
	if second.Port != 25566 || second.TargetPort != 25565 || second.Protocols != "tcp,udp" {
		t.Fatalf("unexpected second forwarded allocation: %+v", second)
	}

	trimmed, err := service.AllocateForward(&node, "vps-one", 80, " tcp, udp ")
	if err != nil {
		t.Fatal(err)
	}
	if trimmed.Protocols != "tcp,udp" {
		t.Fatalf("expected protocols to be normalized, got %q", trimmed.Protocols)
	}

	if _, err = service.AllocateForward(&node, "vps-one", 443, "icmp"); err == nil {
		t.Fatal("expected unsupported protocol to be rejected")
	}
}

func TestAllocateRangeRejectsInvalidRanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:allocation-range-invalid?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Allocation{}); err != nil {
		t.Fatal(err)
	}

	service := &Allocation{DB: db}
	if _, err = service.AllocateRange(models.LocalNode, "empty-range", 1000, 0); err == nil {
		t.Fatal("expected invalid range to be rejected")
	}
	if _, err = service.AllocateRange(models.LocalNode, "reversed-range", 2000, 1999); err == nil {
		t.Fatal("expected reversed range to be rejected")
	}
}

func TestListAllocationsByPurpose(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:allocation-list-purpose?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.Node{}, &models.Server{}, &models.Allocation{}); err != nil {
		t.Fatal(err)
	}

	service := &Allocation{DB: db}
	if _, err = service.AllocateRange(models.LocalNode, "server-one", 1000, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AllocateRangeWithPurpose(models.LocalNode, "server-two", 1001, 1001, "vnc"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AllocateForward(models.LocalNode, "server-three", 22, "tcp"); err != nil {
		t.Fatal(err)
	}

	all, err := service.List(models.LocalNode.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected all allocations to be listed, got %d", len(all))
	}

	forwardOnly, err := service.List(models.LocalNode.ID, "forward")
	if err != nil {
		t.Fatal(err)
	}
	if len(forwardOnly) != 1 || forwardOnly[0].TargetPort != 22 {
		t.Fatalf("expected a single forward allocation, got %+v", forwardOnly)
	}
}
