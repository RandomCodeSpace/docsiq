//go:build race

package vectorindex

// raceEnabled reports whether the package was compiled with -race.
// Some long-running deterministic benchmarks (e.g. TestHNSW_Recall10k)
// gain nothing from the race detector — they're sequential — and pay
// ~10× overhead that dominates CI time. Those tests skip when this is
// true, leaving the explicitly concurrent tests to exercise the detector.
const raceEnabled = true
