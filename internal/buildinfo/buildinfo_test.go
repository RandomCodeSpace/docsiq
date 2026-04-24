package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestResolve_SentinelFallsBackToBuildInfo(t *testing.T) {
	origVersion, origCommit, origDate, origRead :=
		Version, Commit, Date, readBuildInfo
	defer func() {
		Version, Commit, Date, readBuildInfo =
			origVersion, origCommit, origDate, origRead
	}()

	Version, Commit, Date = "dev", "unknown", "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main: debug.Module{
				Path:    "github.com/RandomCodeSpace/docsiq",
				Version: "v0.5.0",
			},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123def"},
				{Key: "vcs.time", Value: "2026-04-23T10:00:00Z"},
				{Key: "vcs.modified", Value: "false"},
			},
		}, true
	}

	got := Resolve(false)
	if got.Version != "v0.5.0" {
		t.Errorf("Version=%q want v0.5.0", got.Version)
	}
	if got.Commit != "abc123def" {
		t.Errorf("Commit=%q want abc123def", got.Commit)
	}
	if got.BuildDate != "2026-04-23T10:00:00Z" {
		t.Errorf("BuildDate=%q", got.BuildDate)
	}
	if got.GoVersion != "go1.25.5" {
		t.Errorf("GoVersion=%q", got.GoVersion)
	}
	if got.Dirty != "false" {
		t.Errorf("Dirty=%q", got.Dirty)
	}
}

func TestResolve_LdflagsOverridesWin(t *testing.T) {
	origVersion, origCommit, origDate, origRead :=
		Version, Commit, Date, readBuildInfo
	defer func() {
		Version, Commit, Date, readBuildInfo =
			origVersion, origCommit, origDate, origRead
	}()

	Version, Commit, Date = "v9.9.9", "ffffff", "2026-01-01T00:00:00Z"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main:      debug.Module{Version: "v0.0.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "DO-NOT-USE"},
			},
		}, true
	}

	got := Resolve(false)
	if got.Version != "v9.9.9" {
		t.Errorf("ldflags Version should win; got %q", got.Version)
	}
	if got.Commit != "ffffff" {
		t.Errorf("ldflags Commit should win; got %q", got.Commit)
	}
}

func TestResolve_IncludeDepsPopulatesMap(t *testing.T) {
	origRead := readBuildInfo
	defer func() { readBuildInfo = origRead }()
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			GoVersion: "go1.25.5",
			Main:      debug.Module{Version: "v0.5.0"},
			Deps: []*debug.Module{
				{Path: "github.com/spf13/cobra", Version: "v1.10.2"},
				{Path: "github.com/tmc/langchaingo", Version: "v0.1.14"},
			},
		}, true
	}

	got := Resolve(true)
	if got.Deps["github.com/spf13/cobra"] != "v1.10.2" {
		t.Errorf("deps missing cobra; got %+v", got.Deps)
	}
	if got.Deps["github.com/tmc/langchaingo"] != "v0.1.14" {
		t.Errorf("deps missing langchaingo")
	}
}

func TestResolve_ReadBuildInfoUnavailable(t *testing.T) {
	origRead := readBuildInfo
	origVersion, origCommit, origDate :=
		Version, Commit, Date
	defer func() {
		readBuildInfo = origRead
		Version, Commit, Date = origVersion, origCommit, origDate
	}()

	Version, Commit, Date = "dev", "unknown", "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	got := Resolve(true)
	if got.Version != "unknown" || got.Commit != "unknown" || got.BuildDate != "unknown" {
		t.Errorf("all fields should be 'unknown'; got %+v", got)
	}
	if got.GoVersion != "unknown" {
		t.Errorf("GoVersion should be 'unknown'; got %q", got.GoVersion)
	}
	if got.Deps != nil {
		t.Errorf("Deps should be nil when ReadBuildInfo fails; got %+v", got.Deps)
	}
	if strings.Contains(got.Dirty, "true") {
		t.Errorf("Dirty should default to 'unknown'; got %q", got.Dirty)
	}
}
