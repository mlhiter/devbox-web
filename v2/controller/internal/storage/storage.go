package storage

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

func StorageLimitBytes(storageLimit string) (int64, error) {
	trimmed := strings.TrimSpace(storageLimit)
	if trimmed == "" {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(trimmed)
	if err != nil {
		return 0, err
	}

	value := quantity.Value()
	return value, nil
}

func ResolveStorageLimit(storageLimit string) (string, error) {
	trimmed := strings.TrimSpace(storageLimit)
	if trimmed == "" {
		return "", nil
	}

	bytes, err := StorageLimitBytes(trimmed)
	if err != nil {
		return "", err
	}
	return resource.NewQuantity(bytes, resource.BinarySI).String(), nil
}
