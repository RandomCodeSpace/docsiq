package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Set via -ldflags at build time (see Makefile). These act as overrides.
// When the binary is installed via `go install <module>@<version>`, go install
// cannot pass -ldflags, so these remain at their sentinel defaults and we fall
// back to runtime/debug.ReadBuildInfo() to populate version info from the VCS
// data that `go install` embeds automatically.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// VersionInfo holds resolved version metadata for the running binary.
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
	Dirty   string // "true", "false", or "unknown"
}

// isSentinel reports whether an ldflags variable is empty or equal to a
// known default placeholder, meaning we should defer to ReadBuildInfo.
func isSentinel(v string) bool {
	switch v {
	case "", "dev", "unknown":
		return true
	}
	return false
}

// readBuildInfo is a package-level indirection so tests can substitute a
// stub when needed. It mirrors the signature of debug.ReadBuildInfo.
var readBuildInfo = debug.ReadBuildInfo

// versionInfo resolves the current version metadata using the following order:
//  1. -ldflags overrides (if non-sentinel)
//  2. runtime/debug.ReadBuildInfo() (module version + VCS settings)
//  3. "unknown" for any remaining field
func versionInfo() VersionInfo {
	vi := VersionInfo{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		Dirty:   "unknown",
	}

	info, ok := readBuildInfo()
	if !ok {
		if isSentinel(vi.Version) {
			vi.Version = "unknown"
		}
		if isSentinel(vi.Commit) {
			vi.Commit = "unknown"
		}
		if isSentinel(vi.Date) {
			vi.Date = "unknown"
		}
		return vi
	}

	// Version: fall back to module version (e.g. "v0.5.0" or "(devel)").
	if isSentinel(vi.Version) {
		if info.Main.Version != "" {
			vi.Version = info.Main.Version
		} else {
			vi.Version = "unknown"
		}
	}

	// Walk VCS settings for commit/time/modified.
	var vcsRev, vcsTime, vcsMod string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			vcsRev = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			vcsMod = s.Value
		}
	}

	if isSentinel(vi.Commit) {
		if vcsRev != "" {
			vi.Commit = vcsRev
		} else {
			vi.Commit = "unknown"
		}
	}
	if isSentinel(vi.Date) {
		if vcsTime != "" {
			vi.Date = vcsTime
		} else {
			vi.Date = "unknown"
		}
	}
	if vcsMod != "" {
		vi.Dirty = vcsMod
	}

	return vi
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of docsiq",
	Run: func(cmd *cobra.Command, args []string) {
		vi := versionInfo()
		dirtySuffix := ""
		if vi.Dirty == "true" {
			dirtySuffix = " (dirty)"
		}
		fmt.Printf("docsiq %s (commit: %s, built: %s)%s\n",
			vi.Version, vi.Commit, vi.Date, dirtySuffix)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
