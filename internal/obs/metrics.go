// Package obs wires Prometheus metrics for docsiq. One registry per
// process (exposed via obs.Default). Metric families are grouped by
// subject (HTTP, pipeline, embed, LLM, workq, build-info) so handlers
// record through a thin typed API — callers never touch raw collectors.
package obs

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Default is the process-wide registry. Tests construct their own
// prometheus.NewRegistry() to avoid Register-twice panics.
var (
	Default = prometheus.NewRegistry()

	HTTP     *HTTPMetrics
	Pipeline *PipelineMetrics
	Embed    *EmbedMetrics
	LLM      *LLMMetrics
	Workq    *WorkqMetrics
	Build    *BuildInfoMetric
)

// Init wires the Default registry. Must be called exactly once at
// startup (from cmd/serve.go). Safe no-op on second call.
var (
	initOnce sync.Once
)

func Init() {
	initOnce.Do(func() {
		HTTP = NewHTTPMetrics(Default)
		Pipeline = NewPipelineMetrics(Default)
		Embed = NewEmbedMetrics(Default)
		LLM = NewLLMMetrics(Default)
		Workq = NewWorkqMetrics(Default)
		Build = NewBuildInfoMetric(Default)
	})
}

// ---- HTTP ---------------------------------------------------------------

// HTTPMetrics bundles the request counter + duration histogram.
type HTTPMetrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// NewHTTPMetrics constructs and registers the HTTP metric family on reg.
func NewHTTPMetrics(reg prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		Requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "docsiq_http_requests_total",
				Help: "Total HTTP requests by route, method, and status.",
			},
			[]string{"route", "method", "status"},
		),
		Duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds, by route.",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"route"},
		),
	}
	reg.MustRegister(m.Requests, m.Duration)
	return m
}

// Observe records one request. `route` MUST be the pattern (e.g.
// "GET /api/documents/{id}"), not r.URL.Path — raw paths have unbounded
// cardinality and will explode the scrape database.
func (m *HTTPMetrics) Observe(route, method string, status int, d time.Duration) {
	m.Requests.WithLabelValues(route, method, statusLabel(status)).Inc()
	m.Duration.WithLabelValues(route).Observe(d.Seconds())
}

// statusLabel collapses a status code to a two-digit bucket (2xx, 3xx,
// 4xx, 5xx). Bounds cardinality even when a handler emits non-standard
// codes.
func statusLabel(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

// ---- Pipeline -----------------------------------------------------------

// PipelineMetrics bundles the pipeline-stage histogram.
type PipelineMetrics struct {
	StageDuration *prometheus.HistogramVec
}

// NewPipelineMetrics constructs + registers the pipeline-stage family.
func NewPipelineMetrics(reg prometheus.Registerer) *PipelineMetrics {
	m := &PipelineMetrics{
		StageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_pipeline_stage_duration_seconds",
				Help:    "GraphRAG pipeline stage duration in seconds.",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 14), // 0.1s → ~27min
			},
			[]string{"stage"},
		),
	}
	reg.MustRegister(m.StageDuration)
	return m
}

// TimeStage measures the wall-clock duration of fn and records it
// against the given stage label. The error from fn is propagated.
func (m *PipelineMetrics) TimeStage(stage string, fn func() error) error {
	start := time.Now()
	err := fn()
	m.StageDuration.WithLabelValues(stage).Observe(time.Since(start).Seconds())
	return err
}

// ---- Embed --------------------------------------------------------------

// EmbedMetrics bundles the embed-latency histogram.
type EmbedMetrics struct {
	Latency *prometheus.HistogramVec
}

// NewEmbedMetrics constructs + registers the embed-latency family.
func NewEmbedMetrics(reg prometheus.Registerer) *EmbedMetrics {
	m := &EmbedMetrics{
		Latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "docsiq_embed_latency_seconds",
				Help:    "Per-batch embed call latency in seconds, by provider.",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"provider"},
		),
	}
	reg.MustRegister(m.Latency)
	return m
}

// Observe records a single embed-batch call duration.
func (m *EmbedMetrics) Observe(provider string, d time.Duration) {
	m.Latency.WithLabelValues(provider).Observe(d.Seconds())
}

// ---- LLM ----------------------------------------------------------------

// LLMMetrics bundles the token-counter family.
type LLMMetrics struct {
	Tokens *prometheus.CounterVec
}

// NewLLMMetrics constructs + registers the LLM-tokens family.
func NewLLMMetrics(reg prometheus.Registerer) *LLMMetrics {
	m := &LLMMetrics{
		Tokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "docsiq_llm_tokens_total",
				Help: "LLM tokens consumed, by provider and kind (prompt|completion|total).",
			},
			[]string{"provider", "kind"},
		),
	}
	reg.MustRegister(m.Tokens)
	return m
}

// RecordTokens increments the counter by n. Use kind="prompt",
// "completion", or "total" (when the provider cannot split usage).
// Non-positive n is ignored so callers don't have to guard.
func (m *LLMMetrics) RecordTokens(provider, kind string, n int) {
	if n <= 0 {
		return
	}
	m.Tokens.WithLabelValues(provider, kind).Add(float64(n))
}

// ---- Workq --------------------------------------------------------------

// WorkqStats is the snapshot surface the obs layer needs from workq.
// Workq owns the concrete Pool.Stats() method; obs only reads.
type WorkqStats struct {
	Depth    int64
	Rejected int64
}

// WorkqStatsProvider is a closure over pool.Stats(). Injected from
// cmd/serve.go after the pool is constructed, so the obs package does
// not take a hard dep on internal/workq (keeps the import DAG acyclic).
type WorkqStatsProvider func() WorkqStats

// WorkqMetrics wraps the workq gauges/counters; the actual values are
// read via a late-bound provider function so the pool can be swapped in
// after registration without re-registering collectors.
type WorkqMetrics struct {
	mu       sync.RWMutex
	provider WorkqStatsProvider
}

// NewWorkqMetrics registers the workq collectors. The provider is a
// no-op until BindStatsProvider is called; Prometheus scrapes before
// binding will see Depth=0, Rejected=0 (safe defaults).
func NewWorkqMetrics(reg prometheus.Registerer) *WorkqMetrics {
	m := &WorkqMetrics{
		provider: func() WorkqStats { return WorkqStats{} },
	}

	depth := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "docsiq_workq_depth",
			Help: "Current depth of the workq submission queue (jobs waiting).",
		},
		func() float64 {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return float64(m.provider().Depth)
		},
	)

	rejected := prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name: "docsiq_workq_rejected_total",
			Help: "Total workq submissions rejected because the queue was full.",
		},
		func() float64 {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return float64(m.provider().Rejected)
		},
	)
	reg.MustRegister(depth, rejected)
	return m
}

// BindStatsProvider wires a live snapshot source. Call from cmd/serve.go
// after the pool is created, before starting the HTTP server.
func (m *WorkqMetrics) BindStatsProvider(p WorkqStatsProvider) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provider = p
}

// ---- Build info ---------------------------------------------------------

// BuildInfoMetric wraps the docsiq_build_info gauge used for
// {version, commit} labels on dashboards.
type BuildInfoMetric struct {
	Info *prometheus.GaugeVec
}

// NewBuildInfoMetric constructs + registers the build-info gauge.
func NewBuildInfoMetric(reg prometheus.Registerer) *BuildInfoMetric {
	m := &BuildInfoMetric{
		Info: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "docsiq_build_info",
				Help: "Build metadata (value is always 1; labels carry version and commit).",
			},
			[]string{"version", "commit"},
		),
	}
	reg.MustRegister(m.Info)
	return m
}

// Set publishes the current build metadata. Subsequent Set calls
// overwrite rather than accumulate labels.
func (m *BuildInfoMetric) Set(version, commit string) {
	m.Info.Reset()
	m.Info.WithLabelValues(version, commit).Set(1)
}
