package services

import (
	"testing"

	"github.com/pufferpanel/pufferpanel/v3/models"
)

func TestResolveBillingCapacity(t *testing.T) {
	node := &models.Node{}
	capacity := ResolveBillingCapacity(node, 8, 16384)
	if capacity.CPUCapacityMilli != 8000 || capacity.MemoryCapacityMB != 16384 {
		t.Fatalf("detected capacity = %+v, want 8 cores and 16384 MB", capacity)
	}

	node.BillingCPUCapacityMilli = 6000
	node.BillingMemoryCapacityMB = 8192
	capacity = ResolveBillingCapacity(node, 8, 16384)
	if capacity.CPUCapacityMilli != 6000 || capacity.MemoryCapacityMB != 8192 {
		t.Fatalf("overridden capacity = %+v, want 6000 milli-cores and 8192 MB", capacity)
	}
}

func TestHasBillingCapacity(t *testing.T) {
	capacity := BillingCapacity{CPUCapacityMilli: 8000, MemoryCapacityMB: 16384}
	if !HasBillingCapacity(capacity, 4000, 8192, 4000, 8192) {
		t.Fatal("a purchase that exactly fills the node should fit")
	}
	if HasBillingCapacity(capacity, 4000, 8192, 4001, 8192) {
		t.Fatal("a purchase exceeding CPU capacity should not fit")
	}
	if HasBillingCapacity(capacity, 8001, 0, 0, 1) {
		t.Fatal("existing over-capacity reservations should reject more purchases")
	}
}
