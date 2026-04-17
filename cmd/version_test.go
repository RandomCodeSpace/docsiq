package cmd

import (
	"runtime/debug"
	"testing"
)

// withLdflags temporarily overrides the package-level ldflags vars.
func withLdflags(t *testing.T, ver, commit, date string) {
	t.Helper()
	origV, origC, origD := Version, Commit, Date
	Version, Commit, Date = ver, commit, date
	t.Cleanup(func() {
		Version, Commit, Date = origV, origC, origD
	})
}

// withBuildInfo swaps the readBuildInfo indirection.
func withBuildInfo(t *testing.T, fn func() (*debug.BuildInfo, bool)) {
	t.Helper()
	orig := readBuildInfo
	readBuildInfo = fn
	t.Cleanup(func() { readBuildInfo = orig })
}

func TestVersionInfo_LdflagsOverrideWins(t *testing.T) {
	withLdflags(t, "v1.2.3", "deadbeef", "2026-01-02T03:04:05Z")
	// Even with BuildInfo available, ldflags should take precedence.
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v9.9.9"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "cafef00d"},
				{Key: "vcs.time", Value: "2099-12-31T00:00:00Z"},
			},
		}, true
	})

	vi := versionInfo()
	if vi.Version != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", vi.Version)
	}
	if vi.Commit != "deadbeef" {
		t.Errorf("Commit = %q, want deadbeef", vi.Commit)
	}
	if vi.Date != "2026-01-02T03:04:05Z" {
		t.Errorf("Date = %q, want date literal", vi.Date)
	}
}

func TestVersionInfo_EmptyLdflagsFallsThroughToBuildInfo(t *testing.T) {
	withLdflags(t, "", "", "")
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v0.5.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
				{Key: "vcs.time", Value: "2026-04-17T10:00:00Z"},
				{Key: "vcs.modified", Value: "false"},
			},
		}, true
	})

	vi := versionInfo()
	if vi.Version != "v0.5.0" {
		t.Errorf("Version = %q, want v0.5.0", vi.Version)
	}
	if vi.Commit != "abc123" {
		t.Errorf("Commit = %q, want abc123", vi.Commit)
	}
	if vi.Date != "2026-04-17T10:00:00Z" {
		t.Errorf("Date = %q, want timestamp", vi.Date)
	}
	if vi.Dirty != "false" {
		t.Errorf("Dirty = %q, want false", vi.Dirty)
	}
}

func TestVersionInfo_DevSentinelTriggersBuildInfo(t *testing.T) {
	// Default "dev"/"unknown" sentinels should be overridden by BuildInfo.
	withLdflags(t, "dev", "unknown", "unknown")
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "(devel)"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "feedface"},
				{Key: "vcs.time", Value: "2026-02-02T02:02:02Z"},
				{Key: "vcs.modified", Value: "true"},
			},
		}, true
	})

	vi := versionInfo()
	if vi.Version != "(devel)" {
		t.Errorf("Version = %q, want (devel)", vi.Version)
	}
	if vi.Commit != "feedface" {
		t.Errorf("Commit = %q, want feedface", vi.Commit)
	}
	if vi.Dirty != "true" {
		t.Errorf("Dirty = %q, want true", vi.Dirty)
	}
}

func TestVersionInfo_LetterOnlyCommitPassedThrough(t *testing.T) {
	withLdflags(t, "", "", "")
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.0.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "ZZZZZZZZ"}, // not a hex hash
				{Key: "vcs.time", Value: "2026-03-03T00:00:00Z"},
			},
		}, true
	})

	vi := versionInfo()
	if vi.Commit != "ZZZZZZZZ" {
		t.Errorf("Commit = %q, want ZZZZZZZZ (no validation)", vi.Commit)
	}
}

func TestVersionInfo_EmptyVcsTimeBecomesUnknown(t *testing.T) {
	withLdflags(t, "", "", "")
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.0.0"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc"},
				// vcs.time omitted
			},
		}, true
	})

	vi := versionInfo()
	if vi.Date != "unknown" {
		t.Errorf("Date = %q, want unknown when vcs.time missing", vi.Date)
	}
	if vi.Dirty != "unknown" {
		t.Errorf("Dirty = %q, want unknown when vcs.modified missing", vi.Dirty)
	}
}

func TestVersionInfo_NoBuildInfoAllSentinelsBecomeUnknown(t *testing.T) {
	withLdflags(t, "dev", "unknown", "")
	withBuildInfo(t, func() (*debug.BuildInfo, bool) {
		return nil, false
	})

	vi := versionInfo()
	if vi.Version != "unknown" {
		t.Errorf("Version = %q, want unknown", vi.Version)
	}
	if vi.Commit != "unknown" {
		t.Errorf("Commit = %q, want unknown", vi.Commit)
	}
	if vi.Date != "unknown" {
		t.Errorf("Date = %q, want unknown", vi.Date)
	}
}

func TestVersionInfo_RealBuildInfoDuringGoTest(t *testing.T) {
	// During `go test`, debug.ReadBuildInfo returns real data for the test
	// binary. With empty ldflags, Version should be non-empty (typically
	// "(devel)" when run from a local checkout, or a module version in CI).
	withLdflags(t, "", "", "")
	vi := versionInfo()
	if vi.Version == "" {
		t.Error("Version should be non-empty when falling through to real ReadBuildInfo")
	}
}
