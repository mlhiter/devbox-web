package storage

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

const storageLimitRedundancyDivisor int64 = 10

var userStorageLimitBytes = newUserStorageLimitBytes("10Gi", "20Gi", "30Gi", "40Gi", "50Gi")

func newUserStorageLimitBytes(limits ...string) map[int64]struct{} {
	values := make(map[int64]struct{}, len(limits))
	for _, limit := range limits {
		quantity := resource.MustParse(limit)
		values[quantity.Value()] = struct{}{}
	}
	return values
}

func AllocatedStorageLimitBytes(storageLimit string) (int64, error) {
	trimmed := strings.TrimSpace(storageLimit)
	if trimmed == "" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(trimmed)
	if err != nil {
		return 0, err
	}

	value := quantity.Value()
	if value <= 0 {
		return value, nil
	}
	if _, ok := userStorageLimitBytes[value]; !ok {
		return value, nil
	}

	buffer := value / storageLimitRedundancyDivisor
	if value%storageLimitRedundancyDivisor != 0 {
		buffer++
	}
	if value > int64(^uint64(0)>>1)-buffer {
		return 0, fmt.Errorf("storage limit %q overflows after adding redundancy", trimmed)
	}
	return value + buffer, nil
}

func ResolveAllocatedStorageLimit(storageLimit string) (string, error) {
	trimmed := strings.TrimSpace(storageLimit)
	if trimmed == "" {
		return "", nil
	}

	allocatedBytes, err := AllocatedStorageLimitBytes(trimmed)
	if err != nil {
		return "", err
	}
	return resource.NewQuantity(allocatedBytes, resource.BinarySI).String(), nil
}
