package monitoring

import (
	"context"
	"testing"

	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildRWStorageTargets(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			newDevboxTarget("owned", "node-a", v1alpha2.DevboxStateRunning),
			newDevboxTarget("other-node", "node-b", v1alpha2.DevboxStateRunning),
			newDevboxTarget("stopped", "node-a", v1alpha2.DevboxStateStopped),
			newDevboxWithoutRecord("missing-record"),
		).
		Build()

	targets, err := BuildRWStorageTargets(context.Background(), reader, "node-a")
	if err != nil {
		t.Fatalf("BuildRWStorageTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("BuildRWStorageTargets() got %d targets, want 1: %#v", len(targets), targets)
	}

	got := targets[0]
	if got.Namespace != "default" ||
		got.Devbox != "owned" ||
		got.ContentID != "content-owned" ||
		got.Node != "node-a" ||
		got.Snapshotter != "devbox" ||
		got.StorageLimit != "10Gi" {
		t.Fatalf("unexpected target: %#v", got)
	}
}

func TestBuildRWStorageTargetsFallsBackToEphemeralStorageLimit(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	devbox := newDevboxTarget("fallback", "node-a", v1alpha2.DevboxStateRunning)
	devbox.Spec.StorageLimit = ""
	devbox.Spec.Resource = corev1.ResourceList{
		corev1.ResourceEphemeralStorage: resource.MustParse("8Gi"),
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(devbox).Build()
	targets, err := BuildRWStorageTargets(context.Background(), reader, "node-a")
	if err != nil {
		t.Fatalf("BuildRWStorageTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("BuildRWStorageTargets() got %d targets, want 1", len(targets))
	}
	if targets[0].StorageLimit != "8Gi" {
		t.Fatalf("StorageLimit = %q, want 8Gi", targets[0].StorageLimit)
	}
}

func TestBuildRWStorageTargetsIncludesRunningSpecBeforeStatusSync(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	devbox := newDevboxTarget("pending-status", "node-a", v1alpha2.DevboxStateStopped)
	devbox.Spec.State = v1alpha2.DevboxStateRunning

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(devbox).Build()
	targets, err := BuildRWStorageTargets(context.Background(), reader, "node-a")
	if err != nil {
		t.Fatalf("BuildRWStorageTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("BuildRWStorageTargets() got %d targets, want 1", len(targets))
	}
	if targets[0].Devbox != "pending-status" {
		t.Fatalf("target devbox = %q, want pending-status", targets[0].Devbox)
	}
}

func TestFindSnapshotByContentID(t *testing.T) {
	service := fakeSnapshotService{
		infos: []snapshots.Info{
			{
				Name:   "unrelated",
				Labels: map[string]string{rwStorageContentIDLabel: "content-other"},
			},
			{
				Name:   "target-snapshot",
				Labels: map[string]string{rwStorageContentIDLabel: "content-a"},
			},
		},
	}

	name, err := findSnapshotByContentID(context.Background(), service, "content-a")
	if err != nil {
		t.Fatalf("findSnapshotByContentID() error = %v", err)
	}
	if name != "target-snapshot" {
		t.Fatalf("findSnapshotByContentID() = %q, want target-snapshot", name)
	}
}

func TestParseStorageLimitBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "gi", input: "10Gi", want: 10 * 1024 * 1024 * 1024},
		{name: "invalid", input: "not-a-size", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStorageLimitBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseStorageLimitBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parseStorageLimitBytes() = %d, want %d", got, tt.want)
			}
		})
	}
}

type fakeSnapshotService struct {
	infos []snapshots.Info
}

func (f fakeSnapshotService) Walk(
	ctx context.Context,
	fn snapshots.WalkFunc,
	_ ...string,
) error {
	for _, info := range f.infos {
		if err := fn(ctx, info); err != nil {
			return err
		}
	}
	return nil
}

func (f fakeSnapshotService) Usage(context.Context, string) (snapshots.Usage, error) {
	return snapshots.Usage{Size: 42}, nil
}

func newDevboxTarget(
	name string,
	node string,
	state v1alpha2.DevboxState,
) *v1alpha2.Devbox {
	contentID := "content-" + name
	return &v1alpha2.Devbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1alpha2.DevboxSpec{
			State:        state,
			StorageLimit: "10Gi",
		},
		Status: v1alpha2.DevboxStatus{
			State:     state,
			ContentID: contentID,
			CommitRecords: v1alpha2.CommitRecordMap{
				contentID: {
					Node:        node,
					Snapshotter: "devbox",
				},
			},
		},
	}
}

func newDevboxWithoutRecord(name string) *v1alpha2.Devbox {
	return &v1alpha2.Devbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1alpha2.DevboxSpec{
			State:        v1alpha2.DevboxStateRunning,
			StorageLimit: "10Gi",
		},
		Status: v1alpha2.DevboxStatus{
			State:     v1alpha2.DevboxStateRunning,
			ContentID: "missing-record",
		},
	}
}
