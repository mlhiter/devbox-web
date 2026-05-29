package storage

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

const storageLimitRedundancyDivisor int64 = 10

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
