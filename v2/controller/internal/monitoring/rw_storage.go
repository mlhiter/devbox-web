package monitoring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
	"github.com/go-logr/logr"
	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	"github.com/sealos-apps/devbox/v2/controller/internal/commit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rwStorageContentIDLabel = commit.SnapshotLabelPrefix + "content-id"
	defaultCollectInterval  = 30 * time.Second
	defaultCollectTimeout   = 20 * time.Second
)

// SnapshotService is the subset of containerd snapshotter behavior needed by
// the collector. Keeping this small makes the target selection and metrics code
// testable without a running containerd.
type SnapshotService interface {
	Walk(ctx context.Context, fn snapshots.WalkFunc, filters ...string) error
	Usage(ctx context.Context, key string) (snapshots.Usage, error)
}

type SnapshotServiceFactory func(snapshotter string) SnapshotService

// RWStorageCollector periodically observes node-local devbox rw layer storage
// usage and publishes it through Prometheus metrics.
type RWStorageCollector struct {
	Client                 client.Client
	NodeName               string
	SnapshotServiceFactory SnapshotServiceFactory
	Recorder               RWStorageMetricsRecorder
	Logger                 logr.Logger
	Interval               time.Duration
	CollectTimeout         time.Duration
}

type rwStorageSample struct {
	Target     RWStorageTarget
	UsedBytes  int64
	LimitBytes int64
}

type collectError struct {
	Reason string
	Err    error
}

func (e collectError) Error() string {
	if e.Err == nil {
		return e.Reason
	}
	return e.Err.Error()
}

func (e collectError) Unwrap() error {
	return e.Err
}

// NewContainerdSnapshotServiceFactory returns a snapshot service factory backed
// by containerd's snapshot service API.
func NewContainerdSnapshotServiceFactory(containerdClient *containerd.Client) SnapshotServiceFactory {
	return func(snapshotter string) SnapshotService {
		if containerdClient == nil {
			return nil
		}
		return containerdClient.SnapshotService(snapshotter)
	}
}

func NewContainerdSnapshotServiceFactoryFromAddress(
	address string,
) (SnapshotServiceFactory, func() error, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		address = commit.DefaultContainerdAddress
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	containerdClient, err := containerd.NewWithConn(
		conn,
		containerd.WithDefaultNamespace(commit.DefaultNamespace),
	)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	closeFn := func() error {
		return containerdClient.Close()
	}
	return NewContainerdSnapshotServiceFactory(containerdClient), closeFn, nil
}

func (c *RWStorageCollector) Start(ctx context.Context) error {
	if c.Interval <= 0 {
		c.Interval = defaultCollectInterval
	}
	if c.CollectTimeout <= 0 {
		c.CollectTimeout = defaultCollectTimeout
	}
	if c.Client == nil {
		return errors.New("rw storage collector client is nil")
	}
	if c.SnapshotServiceFactory == nil {
		return errors.New("rw storage collector snapshot service factory is nil")
	}
	if c.Recorder == nil {
		c.Recorder = NewPrometheusRWStorageMetricsRecorder()
	}

	c.collectOnce(ctx)

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.collectOnce(ctx)
		}
	}
}

func (c *RWStorageCollector) collectOnce(ctx context.Context) {
	collectCtx, cancel := context.WithTimeout(ctx, c.CollectTimeout)
	defer cancel()

	targets, err := BuildRWStorageTargets(collectCtx, c.Client, c.NodeName)
	if err != nil {
		c.Recorder.RecordCollectError(c.NodeName, "list_devboxes")
		c.Logger.Error(err, "failed to build rw storage targets", "node", c.NodeName)
		return
	}

	activeKeys := make(map[RWStorageMetricKey]struct{}, len(targets))
	for _, target := range targets {
		key := target.MetricKey()
		activeKeys[key] = struct{}{}

		sample, err := c.collectTarget(collectCtx, target)
		if err != nil {
			var classified collectError
			reason := "collect"
			if errors.As(err, &classified) && strings.TrimSpace(classified.Reason) != "" {
				reason = classified.Reason
			}
			c.Recorder.RecordCollectError(c.NodeName, reason)
			c.Logger.Error(
				err,
				"failed to collect devbox rw storage usage",
				"namespace",
				target.Namespace,
				"devbox",
				target.Devbox,
				"contentID",
				target.ContentID,
				"snapshotter",
				target.Snapshotter,
			)
			continue
		}

		c.Recorder.RecordSample(sample.Target, sample.UsedBytes, sample.LimitBytes, time.Now())
	}
	c.Recorder.DeleteStale(activeKeys)
}

func (c *RWStorageCollector) collectTarget(
	ctx context.Context,
	target RWStorageTarget,
) (rwStorageSample, error) {
	ctx = withContainerdNamespace(ctx)
	snapshotter := strings.TrimSpace(target.Snapshotter)
	if snapshotter == "" {
		snapshotter = commit.DefaultDevboxSnapshotter
	}
	snapshotService := c.SnapshotServiceFactory(snapshotter)
	if snapshotService == nil {
		return rwStorageSample{}, collectError{Reason: "snapshot_service", Err: fmt.Errorf("snapshot service %q is nil", snapshotter)}
	}

	snapshotName, err := findSnapshotByContentID(ctx, snapshotService, target.ContentID)
	if err != nil {
		return rwStorageSample{}, err
	}

	usage, err := snapshotService.Usage(ctx, snapshotName)
	if err != nil {
		reason := "snapshot_usage"
		if errdefs.IsNotFound(err) {
			reason = "snapshot_not_found"
		}
		return rwStorageSample{}, collectError{Reason: reason, Err: err}
	}

	limitBytes, err := parseStorageLimitBytes(target.StorageLimit)
	if err != nil {
		return rwStorageSample{}, collectError{Reason: "parse_storage_limit", Err: err}
	}

	return rwStorageSample{
		Target:     target,
		UsedBytes:  usage.Size,
		LimitBytes: limitBytes,
	}, nil
}

func findSnapshotByContentID(
	ctx context.Context,
	snapshotService SnapshotService,
	contentID string,
) (string, error) {
	contentID = strings.TrimSpace(contentID)
	if contentID == "" {
		return "", collectError{Reason: "empty_content_id", Err: errors.New("contentID is empty")}
	}

	var snapshotName string
	err := snapshotService.Walk(ctx, func(_ context.Context, info snapshots.Info) error {
		if info.Labels[rwStorageContentIDLabel] != contentID {
			return nil
		}
		snapshotName = info.Name
		return errStopSnapshotWalk
	})
	if errors.Is(err, errStopSnapshotWalk) {
		return snapshotName, nil
	}
	if err != nil {
		return "", collectError{Reason: "snapshot_walk", Err: err}
	}
	return "", collectError{
		Reason: "snapshot_not_found",
		Err:    fmt.Errorf("snapshot with contentID %q not found", contentID),
	}
}

var errStopSnapshotWalk = errors.New("stop snapshot walk")

func parseStorageLimitBytes(storageLimit string) (int64, error) {
	storageLimit = strings.TrimSpace(storageLimit)
	if storageLimit == "" {
		return 0, nil
	}
	quantity, err := resource.ParseQuantity(storageLimit)
	if err != nil {
		return 0, err
	}
	value := quantity.Value()
	if value < 0 {
		return 0, fmt.Errorf("storage limit %q resolves to negative bytes", storageLimit)
	}
	return value, nil
}

func devboxCurrentRecord(devbox *devboxv1alpha2.Devbox) *devboxv1alpha2.CommitRecord {
	if devbox == nil || devbox.Status.CommitRecords == nil || devbox.Status.ContentID == "" {
		return nil
	}
	return devbox.Status.CommitRecords[devbox.Status.ContentID]
}

func devboxIsRWStorageObservable(devbox *devboxv1alpha2.Devbox) bool {
	if devbox == nil || !devbox.DeletionTimestamp.IsZero() {
		return false
	}
	if devbox.Spec.State == devboxv1alpha2.DevboxStateRunning {
		return true
	}
	switch devbox.Status.State {
	case devboxv1alpha2.DevboxStateRunning, devboxv1alpha2.DevboxStatePending:
		return true
	default:
		return false
	}
}

func commitRecordSnapshotter(record *devboxv1alpha2.CommitRecord) string {
	if record == nil || strings.TrimSpace(record.Snapshotter) == "" {
		return commit.DefaultDevboxSnapshotter
	}
	return strings.TrimSpace(record.Snapshotter)
}

func devboxStorageLimit(devbox *devboxv1alpha2.Devbox) string {
	if devbox == nil {
		return ""
	}
	if strings.TrimSpace(devbox.Spec.StorageLimit) != "" {
		return strings.TrimSpace(devbox.Spec.StorageLimit)
	}
	if devbox.Spec.Resource != nil {
		if quantity, ok := devbox.Spec.Resource[corev1.ResourceEphemeralStorage]; ok {
			return quantity.String()
		}
	}
	return ""
}

// RWStorageTarget describes a single Devbox content rw layer to observe on the
// local node.
type RWStorageTarget struct {
	Namespace    string
	Devbox       string
	ContentID    string
	Node         string
	Snapshotter  string
	StorageLimit string
}

func (t RWStorageTarget) MetricKey() RWStorageMetricKey {
	return RWStorageMetricKey{
		Namespace:   t.Namespace,
		Devbox:      t.Devbox,
		ContentID:   t.ContentID,
		Node:        t.Node,
		Snapshotter: t.Snapshotter,
	}
}

// BuildRWStorageTargets selects Devboxes whose current content is owned by the
// local node. It avoids Pod lookups so stopped Pods or cache lag do not prevent
// cleanup of stale metric series.
func BuildRWStorageTargets(
	ctx context.Context,
	reader client.Reader,
	nodeName string,
) ([]RWStorageTarget, error) {
	if reader == nil {
		return nil, errors.New("client reader is nil")
	}
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return nil, errors.New("nodeName is empty")
	}

	devboxList := &devboxv1alpha2.DevboxList{}
	if err := reader.List(ctx, devboxList); err != nil {
		return nil, err
	}

	targets := make([]RWStorageTarget, 0, len(devboxList.Items))
	for i := range devboxList.Items {
		devbox := &devboxList.Items[i]
		if !devboxIsRWStorageObservable(devbox) {
			continue
		}
		record := devboxCurrentRecord(devbox)
		if record == nil || strings.TrimSpace(record.Node) != nodeName {
			continue
		}
		contentID := strings.TrimSpace(devbox.Status.ContentID)
		if contentID == "" {
			continue
		}

		targets = append(targets, RWStorageTarget{
			Namespace:    devbox.Namespace,
			Devbox:       devbox.Name,
			ContentID:    contentID,
			Node:         nodeName,
			Snapshotter:  commitRecordSnapshotter(record),
			StorageLimit: devboxStorageLimit(devbox),
		})
	}

	return targets, nil
}

func withContainerdNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, commit.DefaultNamespace)
}
