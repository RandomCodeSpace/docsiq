package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/obs"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsHandler returns the /metrics handler. Backed by the shared
// obs.Default registry. Public — no auth (Prometheus scrape cannot
// present a bearer token in typical configs).
//
// Retained signature (registry, stores, cfg) so NewRouter callers do
// not need to change; the args are currently unused here but kept as a
// seam for future per-project gauges.
//
// Note: obs.Init is idempotent and called on first scrape so tests
// that build a router without going through cmd/serve.go still get
// the full metric family set.
func metricsHandler(
	_ *project.Registry,
	_ *projectStores,
	_ *config.Config,
) http.Handler {
	obs.Init()
	return promhttp.HandlerFor(obs.Default, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})
}

// SetBuildInfo publishes binary version + commit to the
// docsiq_build_info gauge. Kept with its pre-existing signature so
// cmd/serve.go callers do not have to change. Safe to call before the
// first scrape — obs.Init self-initialises.
func SetBuildInfo(version, commit string) {
	if version == "" {
		version = "dev"
	}
	if commit == "" {
		commit = "unknown"
	}
	obs.Init()
	obs.Build.Set(version, commit)
}
