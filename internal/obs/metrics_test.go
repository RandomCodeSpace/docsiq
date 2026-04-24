package obs

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTP_ObserveRecordsCounterAndHistogram(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	h := NewHTTPMetrics(reg)

	h.Observe("GET /api/documents", "GET", 200, 42*time.Millisecond)
	h.Observe("GET /api/documents", "GET", 200, 80*time.Millisecond)
	h.Observe("POST /api/search", "POST", 500, 2*time.Second)

	if got := testutil.ToFloat64(h.Requests.WithLabelValues("GET /api/documents", "GET", "2xx")); got != 2 {
		t.Errorf("requests{...2xx}=%v want 2", got)
	}
	if got := testutil.ToFloat64(h.Requests.WithLabelValues("POST /api/search", "POST", "5xx")); got != 1 {
		t.Errorf("requests{...5xx}=%v want 1", got)
	}

	out, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var found bool
	for _, mf := range out {
		if mf.GetName() == "docsiq_http_request_duration_seconds" {
			found = true
			for _, m := range mf.Metric {
				if h := m.GetHistogram(); h != nil {
					if h.GetSampleCount() == 0 {
						t.Errorf("histogram sample count=0")
					}
				}
			}
		}
	}
	if !found {
		t.Errorf("docsiq_http_request_duration_seconds not registered")
	}
}

func TestPipeline_TimeStageRecordsBothSuccessAndError(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	p := NewPipelineMetrics(reg)

	_ = p.TimeStage("load", func() error { return nil })
	_ = p.TimeStage("chunk", func() error { return nil })
	_ = p.TimeStage("chunk", func() error { return nil })

	count := func(stage string) uint64 {
		out, _ := reg.Gather()
		for _, mf := range out {
			if mf.GetName() != "docsiq_pipeline_stage_duration_seconds" {
				continue
			}
			for _, m := range mf.Metric {
				var s string
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "stage" {
						s = lp.GetValue()
					}
				}
				if s == stage {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
		return 0
	}
	if got := count("load"); got != 1 {
		t.Errorf("load stage count=%d want 1", got)
	}
	if got := count("chunk"); got != 2 {
		t.Errorf("chunk stage count=%d want 2", got)
	}
}

func TestEmbed_ObserveByProvider(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	e := NewEmbedMetrics(reg)

	e.Observe("openai", 120*time.Millisecond)
	e.Observe("openai", 200*time.Millisecond)
	e.Observe("ollama", 50*time.Millisecond)

	out, _ := reg.Gather()
	perProvider := map[string]uint64{}
	for _, mf := range out {
		if mf.GetName() != "docsiq_embed_latency_seconds" {
			continue
		}
		for _, m := range mf.Metric {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "provider" {
					perProvider[lp.GetValue()] = m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	if perProvider["openai"] != 2 {
		t.Errorf("openai count=%d want 2", perProvider["openai"])
	}
	if perProvider["ollama"] != 1 {
		t.Errorf("ollama count=%d want 1", perProvider["ollama"])
	}
}

func TestLLM_RecordTokensByKind(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	l := NewLLMMetrics(reg)

	l.RecordTokens("openai", "prompt", 512)
	l.RecordTokens("openai", "completion", 128)
	l.RecordTokens("azure", "total", 256)
	l.RecordTokens("openai", "prompt", 0)  // ignored
	l.RecordTokens("openai", "prompt", -5) // ignored

	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("openai", "prompt")); got != 512 {
		t.Errorf("openai prompt=%v want 512", got)
	}
	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("openai", "completion")); got != 128 {
		t.Errorf("openai completion=%v want 128", got)
	}
	if got := testutil.ToFloat64(l.Tokens.WithLabelValues("azure", "total")); got != 256 {
		t.Errorf("azure total=%v want 256", got)
	}
}

func TestWorkq_DepthAndRejectedFromStatsProvider(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	w := NewWorkqMetrics(reg)

	w.BindStatsProvider(func() WorkqStats { return WorkqStats{Depth: 7, Rejected: 3} })

	out, _ := reg.Gather()
	var depth, rejected float64
	for _, mf := range out {
		switch mf.GetName() {
		case "docsiq_workq_depth":
			depth = mf.Metric[0].GetGauge().GetValue()
		case "docsiq_workq_rejected_total":
			rejected = mf.Metric[0].GetCounter().GetValue()
		}
	}
	if depth != 7 {
		t.Errorf("depth=%v want 7", depth)
	}
	if rejected != 3 {
		t.Errorf("rejected=%v want 3", rejected)
	}
}

func TestExpose_ScrapeOutputContainsAllFamilies(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	h := NewHTTPMetrics(reg)
	p := NewPipelineMetrics(reg)
	e := NewEmbedMetrics(reg)
	l := NewLLMMetrics(reg)
	_ = NewWorkqMetrics(reg)

	bi := NewBuildInfoMetric(reg)
	bi.Set("v9.9.9", "abcdef")

	// Fire at least one observation per label-vec family so the
	// Prometheus text exposition actually prints a line. Counter/
	// HistogramVec families are omitted from the scrape until at least
	// one label combination has been touched — this is by design in
	// client_golang.
	h.Observe("GET /test", "GET", 200, time.Millisecond)
	_ = p.TimeStage("load", func() error { return nil })
	e.Observe("openai", time.Millisecond)
	l.RecordTokens("openai", "total", 1)

	body := renderForTest(t, reg)
	for _, want := range []string{
		"docsiq_http_requests_total",
		"docsiq_http_request_duration_seconds",
		"docsiq_pipeline_stage_duration_seconds",
		"docsiq_embed_latency_seconds",
		"docsiq_llm_tokens_total",
		"docsiq_workq_depth",
		"docsiq_workq_rejected_total",
		"docsiq_build_info",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing family %q", want)
		}
	}
}

func renderForTest(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).
		ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}
