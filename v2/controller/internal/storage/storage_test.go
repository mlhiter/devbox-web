package storage

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestAllocatedStorageLimitBytes(t *testing.T) {
	tests := []struct {
		name    string
		limit   string
		want    string
		wantErr bool
	}{
		{
			name:  "empty limit",
			limit: "",
			want:  "0",
		},
		{
			name:  "adds ten percent to create default limit",
			limit: "10Gi",
			want:  "11Gi",
		},
		{
			name:  "adds ten percent to maximum user-facing limit",
			limit: "50Gi",
			want:  "55Gi",
		},
		{
			name:  "trims whitespace before parsing",
			limit: " 20Gi ",
			want:  "22Gi",
		},
		{
			name:    "returns parse errors",
			limit:   "bad-limit",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AllocatedStorageLimitBytes(tt.limit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("AllocatedStorageLimitBytes() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("AllocatedStorageLimitBytes() error = %v", err)
			}

			wantQuantity := resource.MustParse(tt.want)
			want := wantQuantity.Value()
			if got != want {
				t.Fatalf("AllocatedStorageLimitBytes() = %d, want %d", got, want)
			}
		})
	}
}

func TestResolveAllocatedStorageLimit(t *testing.T) {
	got, err := ResolveAllocatedStorageLimit("50Gi")
	if err != nil {
		t.Fatalf("ResolveAllocatedStorageLimit() error = %v", err)
	}
	if got != "55Gi" {
		t.Fatalf("ResolveAllocatedStorageLimit() = %q, want %q", got, "55Gi")
	}
}
