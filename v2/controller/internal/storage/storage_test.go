package storage

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestStorageLimitBytes(t *testing.T) {
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
			name:  "keeps create default limit unchanged",
			limit: "10Gi",
			want:  "10Gi",
		},
		{
			name:  "keeps maximum user-facing limit unchanged",
			limit: "50Gi",
			want:  "50Gi",
		},
		{
			name:  "trims whitespace before parsing",
			limit: " 20Gi ",
			want:  "20Gi",
		},
		{
			name:  "keeps non-user-facing values unchanged",
			limit: "5Gi",
			want:  "5Gi",
		},
		{
			name:    "returns parse errors",
			limit:   "bad-limit",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StorageLimitBytes(tt.limit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("StorageLimitBytes() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("StorageLimitBytes() error = %v", err)
			}

			wantQuantity := resource.MustParse(tt.want)
			want := wantQuantity.Value()
			if got != want {
				t.Fatalf("StorageLimitBytes() = %d, want %d", got, want)
			}
		})
	}
}

func TestResolveStorageLimit(t *testing.T) {
	got, err := ResolveStorageLimit("50Gi")
	if err != nil {
		t.Fatalf("ResolveStorageLimit() error = %v", err)
	}
	if got != "50Gi" {
		t.Fatalf("ResolveStorageLimit() = %q, want %q", got, "50Gi")
	}
}
