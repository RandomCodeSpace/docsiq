package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// numHistogramBuckets is the fixed number of finite upper-bound buckets
// in the request-duration histogram. Kept as a typed const so
// histogramCell.buckets can be a fixed-size array (vet rejects
// len(slice)+1 as an array size).
const numHistogramBuckets = 11

// histogramBuckets are the upper bounds (in seconds) used for the
// docsiq_request_duration_seconds histogram. Deliberately simple —
// Prometheus default-style powers of √10 tweaked for typical HTTP latency.
var histogramBuckets = [numHistogramBuckets]float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// labelKey is the composite key for a (method, path, status) counter
// cell. Using a struct rather than a stringified concat keeps the key
// comparable and avoids accidental collisions from delimiter-in-value.
type labelKey struct {
	Method string
	Path   string
	Status int
}

type histogramCell struct {
	buckets [numHistogramBuckets + 1]uint64 // +1 for +Inf
	sum     float64
	count   uint64
}

type histogramKey struct {
	Method string
	Path   string
}

// metrics is the package-level collector. All mutations must take its
// mutex; reads under the same mutex guarantee a consistent scrape.
type metricsRegistry struct {
	mu sync.Mutex

	// Monotonic counter: docsiq_requests_total{method,path,status}
	requestsTotal map[labelKey]uint64

	// Histogram: docsiq_request_duration_seconds{method,path}
	requestDuration map[histogramKey]*histogramCell

	// Build info (set once at startup).
	buildVersion string
	buildCommit  string
}

func newMetricsRegistry() *metricsRegistry {
	return &metricsRegistry{
		requestsTotal:   map[labelKey]uint64{},
		requestDuration: map[histogramKey]*histogramCell{},
		buildVersion:    "dev",
		buildCommit:     "unknown",
	}
}

// globalMetrics is the single collector shared by the logging middleware
// (writer) and /metrics handler (reader).
var globalMetrics = newMetricsRegistry()

// SetBuildInfo lets cmd/ wire the binary version + commit into the
// docsiq_build_info gauge. Safe to call from init or main; zero-value
// defaults ("dev"/"unknown") are used if never called.
func SetBuildInfo(version, commit string) {
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "unknown"
	}
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.buildVersion = version
	globalMetrics.buildCommit = commit
}

// recordRequest is called by loggingMiddleware after every HTTP request.
// method/path labels are NOT sanitized here — callers must pass values
// they are willing to expose on the scrape endpoint.
func recordRequest(method, path string, status int, durationSeconds float64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	globalMetrics.requestsTotal[labelKey{method, path, status}]++

	hk := histogramKey{method, path}
	cell, ok := globalMetrics.requestDuration[hk]
	if !ok {
		cell = &histogramCell{}
		globalMetrics.requestDuration[hk] = cell
	}
	cell.count++
	cell.sum += durationSeconds
	placed := false
	for i, ub := range histogramBuckets {
		if durationSeconds <= ub {
			cell.buckets[i]++
			placed = true
			break
		}
	}
	if !placed {
		cell.buckets[len(histogramBuckets)]++ // +Inf
	}
}

// metricsHandler writes the Prometheus text exposition format.
// Ops endpoint — NOT gated by auth. Mounted on /metrics.
//
// The projects gauge + notes gauge require a registry + per-project
// store cache; those are resolved lazily via closures captured at
// NewRouter time.
func metricsHandler(
	registry *project.Registry,
	stores *projectStores,
	_ *config.Config,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		globalMetrics.mu.Lock()
		// Snapshot under the lock, release before formatting.
		reqs := make(map[labelKey]uint64, len(globalMetrics.requestsTotal))
		for k, v := range globalMetrics.requestsTotal {
			reqs[k] = v
		}
		hists := make(map[histogramKey]histogramCell, len(globalMetrics.requestDuration))
		for k, v := range globalMetrics.requestDuration {
			hists[k] = *v
		}
		version := globalMetrics.buildVersion
		commit := globalMetrics.buildCommit
		globalMetrics.mu.Unlock()

		var b strings.Builder
		writeBuildInfo(&b, version, commit)
		writeProjectsGauge(&b, registry)
		writeNotesGauge(r.Context(), &b, registry, stores)
		writeRequestsTotal(&b, reqs)
		writeRequestDuration(&b, hists)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
	})
}

func writeBuildInfo(b *strings.Builder, version, commit string) {
	b.WriteString("# HELP docsiq_build_info Build metadata (always 1)\n")
	b.WriteString("# TYPE docsiq_build_info gauge\n")
	fmt.Fprintf(b, "docsiq_build_info{version=%q,commit=%q} 1\n",
		version, commit)
}

func writeProjectsGauge(b *strings.Builder, registry *project.Registry) {
	b.WriteString("# HELP docsiq_projects_total Number of registered projects\n")
	b.WriteString("# TYPE docsiq_projects_total gauge\n")
	count := 0
	if registry != nil {
		if list, err := registry.List(); err == nil {
			count = len(list)
		}
	}
	fmt.Fprintf(b, "docsiq_projects_total %d\n", count)
}

func writeNotesGauge(
	ctx context.Context,
	b *strings.Builder,
	registry *project.Registry,
	stores *projectStores,
) {
	b.WriteString("# HELP docsiq_notes_total Number of notes indexed per project\n")
	b.WriteString("# TYPE docsiq_notes_total gauge\n")
	if registry == nil || stores == nil {
		return
	}
	list, err := registry.List()
	if err != nil {
		return
	}
	// Sort for stable scrape output (makes diffs + tests deterministic).
	sort.Slice(list, func(i, j int) bool { return list[i].Slug < list[j].Slug })
	for _, p := range list {
		st, err := stores.Get(p.Slug)
		if err != nil {
			continue
		}
		n, err := st.CountNotes(ctx)
		if err != nil {
			continue
		}
		fmt.Fprintf(b, "docsiq_notes_total{project=%q} %d\n", p.Slug, n)
	}
}

func writeRequestsTotal(b *strings.Builder, reqs map[labelKey]uint64) {
	b.WriteString("# HELP docsiq_requests_total Total HTTP requests processed\n")
	b.WriteString("# TYPE docsiq_requests_total counter\n")

	keys := make([]labelKey, 0, len(reqs))
	for k := range reqs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Path != keys[j].Path {
			return keys[i].Path < keys[j].Path
		}
		return keys[i].Status < keys[j].Status
	})
	for _, k := range keys {
		fmt.Fprintf(b, "docsiq_requests_total{method=%q,path=%q,status=%q} %d\n",
			k.Method, k.Path, strconv.Itoa(k.Status), reqs[k])
	}
}

func writeRequestDuration(b *strings.Builder, hists map[histogramKey]histogramCell) {
	b.WriteString("# HELP docsiq_request_duration_seconds HTTP request latency distribution\n")
	b.WriteString("# TYPE docsiq_request_duration_seconds histogram\n")

	keys := make([]histogramKey, 0, len(hists))
	for k := range hists {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Path < keys[j].Path
	})

	for _, k := range keys {
		cell := hists[k]
		// Cumulative bucket counts — Prometheus requires le is cumulative.
		var cum uint64
		for i, ub := range histogramBuckets {
			cum += cell.buckets[i]
			fmt.Fprintf(b, "docsiq_request_duration_seconds_bucket{method=%q,path=%q,le=%q} %d\n",
				k.Method, k.Path, formatFloat(ub), cum)
		}
		cum += cell.buckets[len(histogramBuckets)]
		fmt.Fprintf(b, "docsiq_request_duration_seconds_bucket{method=%q,path=%q,le=\"+Inf\"} %d\n",
			k.Method, k.Path, cum)
		fmt.Fprintf(b, "docsiq_request_duration_seconds_sum{method=%q,path=%q} %s\n",
			k.Method, k.Path, formatFloat(cell.sum))
		fmt.Fprintf(b, "docsiq_request_duration_seconds_count{method=%q,path=%q} %d\n",
			k.Method, k.Path, cell.count)
	}
}

// formatFloat renders a float in a Prometheus-friendly form: integer
// values drop the decimal, fractionals use minimum-precision "%g".
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
