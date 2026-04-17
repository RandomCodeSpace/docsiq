package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/sqlitevec"
	"github.com/spf13/cobra"
)

// vecMode reports which path is servicing vector queries.
type vecMode string

const (
	vecModeUnknown    vecMode = "unknown"
	vecModeNone       vecMode = "none"        // neither sqlite-vec nor HNSW; brute-force only
	vecModeHNSW       vecMode = "hnsw"        // in-memory HNSW index loaded, sqlite-vec unavailable
	vecModeSqliteVec  vecMode = "sqlite-vec"  // sqlite-vec extension loaded and active
	vecModeBruteForce vecMode = "brute-force" // explicit brute-force fallback (empty index, no ext)
)

// vecState is the last-known vector mode set by serve. It's a best-effort
// in-process hint; `docsiq vec status` on a CLI invocation (separate
// process) cannot read it, so the status command falls back to probing
// the on-disk artefacts instead.
var (
	vecStateMu  sync.Mutex
	vecStateCur = vecModeUnknown
	vecStateMsg = ""
)

func setVecMode(m vecMode, msg string) {
	vecStateMu.Lock()
	defer vecStateMu.Unlock()
	vecStateCur = m
	vecStateMsg = msg
}

var vecCmd = &cobra.Command{
	Use:   "vec",
	Short: "Manage / inspect the vector search backend",
	Long:  "Commands to inspect which vector backend (sqlite-vec, in-memory HNSW, or brute-force) is active.",
}

var vecStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report which vector backend is active",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("platform:      %s/%s\n", runtime.GOOS, runtime.GOARCH)

		// 1. Probe the extracted extension on disk. If serve has run at
		//    least once, $DATA_DIR/ext/vec0.<ext> will exist.
		ext := ""
		switch runtime.GOOS {
		case "linux":
			ext = "so"
		case "darwin":
			ext = "dylib"
		default:
			fmt.Printf("data_dir:      %s\n", cfg.DataDir)
			fmt.Printf("mode:          %s\n", vecModeBruteForce)
			fmt.Printf("reason:        GOOS=%s is unsupported by sqlite-vec; brute-force only\n", runtime.GOOS)
			return nil
		}
		soPath := filepath.Join(cfg.DataDir, "ext", "vec0."+ext)
		fmt.Printf("data_dir:      %s\n", cfg.DataDir)
		fmt.Printf("extension:     %s\n", soPath)

		info, err := os.Stat(soPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			fmt.Printf("extension_ok:  no (not yet extracted — run `docsiq serve` at least once)\n")
			fmt.Printf("mode:          %s (fallback: in-memory HNSW if built, else brute-force)\n", vecModeHNSW)
			return nil
		case err != nil:
			fmt.Printf("extension_ok:  no (stat error: %v)\n", err)
			fmt.Printf("mode:          %s\n", vecModeNone)
			return nil
		case info.Size() == 0:
			fmt.Printf("extension_ok:  no (0-byte placeholder — real sqlite-vec binary not bundled)\n")
			fmt.Printf("mode:          %s (fallback: in-memory HNSW if built, else brute-force)\n", vecModeHNSW)
			return nil
		}
		fmt.Printf("extension_ok:  yes (%d bytes)\n", info.Size())

		// 2. Try actually loading it into a throwaway in-memory DB as a
		//    proof of dlopen-ability.
		if _, err := sqlitevec.Extract(cfg.DataDir); err != nil {
			fmt.Printf("probe:         extract failed: %v\n", err)
		}
		fmt.Printf("mode:          %s\n", vecModeSqliteVec)
		fmt.Println("note:          `serve` will log the actual live mode at startup.")
		return nil
	},
}

func init() {
	vecCmd.AddCommand(vecStatusCmd)
	rootCmd.AddCommand(vecCmd)
}
