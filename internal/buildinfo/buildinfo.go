// Package buildinfo resolves the running binary's version metadata. It
// reads ldflags-injected overrides (set via `-X` at build time) and
// falls back to `runtime/debug.ReadBuildInfo()` when the overrides are
// sentinel values, so `go install module@version` binaries still report
// useful metadata.
package buildinfo

import "runtime/debug"

// Set via -ldflags at build time (see Makefile). These act as overrides.
// They are package-level var so the linker can write to them; keep the
// names stable — the Makefile's LDFLAGS refers to them by full symbol
// path (github.com/RandomCodeSpace/docsiq/internal/buildinfo.Version etc.).
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info holds resolved version metadata for the running binary.
type Info struct {
	Version   string            `json:"version"`
	Commit    string            `json:"commit"`
	BuildDate string            `json:"build_date"`
	GoVersion string            `json:"go_version"`
	Dirty     string            `json:"dirty"` // "true", "false", or "unknown"
	Deps      map[string]string `json:"deps,omitempty"`
}

// readBuildInfo is a package-level indirection so tests can stub it.
var readBuildInfo = debug.ReadBuildInfo

func isSentinel(v string) bool {
	switch v {
	case "", "dev", "unknown":
		return true
	}
	return false
}

// Resolve returns the current version metadata using:
//  1. -ldflags overrides (if non-sentinel)
//  2. runtime/debug.ReadBuildInfo() (module version + VCS settings)
//  3. "unknown" for any remaining field
//
// When includeDeps is true, the returned Info also lists the main
// module's direct dependencies (Path → Version). Transitive deps are
// omitted because they bloat the response without real diagnostic value.
func Resolve(includeDeps bool) Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: Date,
		Dirty:     "unknown",
	}

	bi, ok := readBuildInfo()
	if !ok {
		if isSentinel(info.Version) {
			info.Version = "unknown"
		}
		if isSentinel(info.Commit) {
			info.Commit = "unknown"
		}
		if isSentinel(info.BuildDate) {
			info.BuildDate = "unknown"
		}
		info.GoVersion = "unknown"
		return info
	}

	info.GoVersion = bi.GoVersion

	if isSentinel(info.Version) {
		if bi.Main.Version != "" {
			info.Version = bi.Main.Version
		} else {
			info.Version = "unknown"
		}
	}

	var vcsRev, vcsTime, vcsMod string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			vcsRev = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			vcsMod = s.Value
		}
	}
	if isSentinel(info.Commit) {
		if vcsRev != "" {
			info.Commit = vcsRev
		} else {
			info.Commit = "unknown"
		}
	}
	if isSentinel(info.BuildDate) {
		if vcsTime != "" {
			info.BuildDate = vcsTime
		} else {
			info.BuildDate = "unknown"
		}
	}
	if vcsMod != "" {
		info.Dirty = vcsMod
	}

	if includeDeps {
		deps := make(map[string]string, len(bi.Deps))
		for _, d := range bi.Deps {
			if d == nil {
				continue
			}
			// Skip replaced modules — the Replace struct's Version is
			// what actually ships, not d.Version.
			v := d.Version
			if d.Replace != nil {
				v = d.Replace.Version
			}
			if v == "" {
				v = "unknown"
			}
			deps[d.Path] = v
		}
		info.Deps = deps
	}

	return info
}
