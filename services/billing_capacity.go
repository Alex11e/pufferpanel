package services

import "github.com/pufferpanel/pufferpanel/v3/models"

type BillingCapacity struct {
	CPUCapacityMilli uint64
	MemoryCapacityMB uint64
}

func ResolveBillingCapacity(node *models.Node, detectedCPUCount int, detectedMemoryMB uint64) BillingCapacity {
	capacity := BillingCapacity{
		CPUCapacityMilli: uint64(max(detectedCPUCount, 0)) * 1000,
		MemoryCapacityMB: detectedMemoryMB,
	}
	if node.BillingCPUCapacityMilli > 0 {
		capacity.CPUCapacityMilli = node.BillingCPUCapacityMilli
	}
	if node.BillingMemoryCapacityMB > 0 {
		capacity.MemoryCapacityMB = node.BillingMemoryCapacityMB
	}
	return capacity
}

func HasBillingCapacity(capacity BillingCapacity, usedCPU, usedMemory, requestedCPU, requestedMemory uint64) bool {
	return usedCPU <= capacity.CPUCapacityMilli &&
		usedMemory <= capacity.MemoryCapacityMB &&
		requestedCPU <= capacity.CPUCapacityMilli-usedCPU &&
		requestedMemory <= capacity.MemoryCapacityMB-usedMemory
}
