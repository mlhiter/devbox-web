package monitoring

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	rwStorageMetricsRegisterOnce sync.Once

	rwStorageUsedBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "devbox_rw_storage_used_bytes",
			Help: "Node-local Devbox read-write layer storage usage in bytes.",
		},
		rwStorageMetricLabels,
	)
	rwStorageLimitBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "devbox_rw_storage_limit_bytes",
			Help: "Configured Devbox read-write layer storage limit in bytes.",
		},
		rwStorageMetricLabels,
	)
	rwStorageUsageRatio = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "devbox_rw_storage_usage_ratio",
			Help: "Ratio of Devbox read-write layer storage usage to configured storage limit.",
		},
		rwStorageMetricLabels,
	)
	rwStorageLastCollectTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "devbox_rw_storage_last_collect_timestamp_seconds",
			Help: "Unix timestamp of the last successful Devbox read-write layer storage collection.",
		},
		rwStorageMetricLabels,
	)
	rwStorageCollectActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "devbox_rw_storage_collect_active",
			Help: "Whether Devbox read-write layer storage is actively collected by this exporter. 1 means live collection, 0 means the last successful sample is retained.",
		},
		rwStorageMetricLabels,
	)
	rwStorageCollectErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "devbox_rw_storage_collect_errors_total",
			Help: "Total number of Devbox read-write layer storage collection errors.",
		},
		[]string{"node", "reason"},
	)
)

var rwStorageMetricLabels = []string{"namespace", "devbox", "content_id", "node", "snapshotter"}

// RWStorageMetricKey identifies one exported metric series.
type RWStorageMetricKey struct {
	Namespace   string
	Devbox      string
	ContentID   string
	Node        string
	Snapshotter string
}

func (k RWStorageMetricKey) labelValues() []string {
	return []string{k.Namespace, k.Devbox, k.ContentID, k.Node, k.Snapshotter}
}

type RWStorageMetricsRecorder interface {
	RecordSample(target RWStorageTarget, usedBytes, limitBytes int64, collectedAt time.Time)
	RecordTargetActive(target RWStorageTarget, active bool)
	RecordCollectError(node, reason string)
	DeleteUnretained(retainKeys map[RWStorageMetricKey]struct{})
}

type PrometheusRWStorageMetricsRecorder struct {
	mu       sync.Mutex
	knownSet map[RWStorageMetricKey]struct{}
}

func NewPrometheusRWStorageMetricsRecorder() *PrometheusRWStorageMetricsRecorder {
	rwStorageMetricsRegisterOnce.Do(func() {
		ctrlmetrics.Registry.MustRegister(
			rwStorageUsedBytes,
			rwStorageLimitBytes,
			rwStorageUsageRatio,
			rwStorageLastCollectTimestamp,
			rwStorageCollectActive,
			rwStorageCollectErrors,
		)
	})
	return &PrometheusRWStorageMetricsRecorder{
		knownSet: make(map[RWStorageMetricKey]struct{}),
	}
}

func (r *PrometheusRWStorageMetricsRecorder) RecordSample(
	target RWStorageTarget,
	usedBytes,
	limitBytes int64,
	collectedAt time.Time,
) {
	key := target.MetricKey()
	labelValues := key.labelValues()

	rwStorageUsedBytes.WithLabelValues(labelValues...).Set(float64(usedBytes))
	rwStorageLimitBytes.WithLabelValues(labelValues...).Set(float64(limitBytes))
	if limitBytes > 0 {
		rwStorageUsageRatio.WithLabelValues(labelValues...).Set(float64(usedBytes) / float64(limitBytes))
	} else {
		rwStorageUsageRatio.WithLabelValues(labelValues...).Set(0)
	}
	rwStorageLastCollectTimestamp.WithLabelValues(labelValues...).Set(float64(collectedAt.Unix()))

	r.RecordTargetActive(target, true)
}

func (r *PrometheusRWStorageMetricsRecorder) RecordTargetActive(
	target RWStorageTarget,
	active bool,
) {
	key := target.MetricKey()
	labelValues := key.labelValues()
	value := 0.0
	if active {
		value = 1
	}
	rwStorageCollectActive.WithLabelValues(labelValues...).Set(value)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.knownSet[key] = struct{}{}
}

func (r *PrometheusRWStorageMetricsRecorder) RecordCollectError(node, reason string) {
	rwStorageCollectErrors.WithLabelValues(node, reason).Inc()
}

func (r *PrometheusRWStorageMetricsRecorder) DeleteUnretained(
	retainKeys map[RWStorageMetricKey]struct{},
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key := range r.knownSet {
		if _, ok := retainKeys[key]; ok {
			continue
		}
		labelValues := key.labelValues()
		rwStorageUsedBytes.DeleteLabelValues(labelValues...)
		rwStorageLimitBytes.DeleteLabelValues(labelValues...)
		rwStorageUsageRatio.DeleteLabelValues(labelValues...)
		rwStorageLastCollectTimestamp.DeleteLabelValues(labelValues...)
		rwStorageCollectActive.DeleteLabelValues(labelValues...)
		delete(r.knownSet, key)
	}
}
