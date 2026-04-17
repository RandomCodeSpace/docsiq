package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/spf13/cobra"
)

var (
	initName  string
	initForce bool
)

// deriveProjectName returns a human-readable name from a normalized git
// remote: the last non-empty path component with any .git suffix stripped.
// If the remote is empty after trimming, falls back to "project".
func deriveProjectName(remote string) string {
	r := strings.TrimSpace(remote)
	r = strings.TrimSuffix(r, "/")
	r = strings.TrimSuffix(r, ".git")
	r = strings.TrimSuffix(r, ".GIT")
	if r == "" {
		return "project"
	}
	// Split on '/' OR ':' (scp-style URLs separate path with ':').
	fields := strings.FieldsFunc(r, func(c rune) bool { return c == '/' || c == ':' })
	if len(fields) == 0 {
		return "project"
	}
	last := fields[len(fields)-1]
	last = strings.TrimSuffix(last, ".git")
	if last == "" {
		return "project"
	}
	return last
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a docsiq project for the current git repo",
	Long: `Detects the current directory's git remote, derives a slug, and
registers the project in $DATA_DIR/registry.db. Creates the per-project
SQLite database at $DATA_DIR/projects/<slug>/docsiq.db.

Runs ` + "`git -C <cwd> remote get-url origin`" + ` so it only works inside a
git repository with an origin remote configured.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getwd: %w", err)
		}

		remote, err := project.DetectRemote(cwd)
		if err != nil {
			return err
		}
		slug, err := project.Slug(remote)
		if err != nil {
			return err
		}

		name := initName
		if name == "" {
			name = deriveProjectName(remote)
		}

		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return fmt.Errorf("mkdir data dir: %w", err)
		}
		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		existing, err := reg.Get(slug)
		switch {
		case err == nil:
			if !initForce {
				return fmt.Errorf("slug %q already registered (remote=%s); re-run with --force to overwrite", existing.Slug, existing.Remote)
			}
			if err := reg.Delete(slug); err != nil {
				return fmt.Errorf("overwrite: delete old entry: %w", err)
			}
		case errors.Is(err, project.ErrNotFound):
			// expected path
		default:
			return err
		}

		if err := reg.Register(project.Project{
			Slug:   slug,
			Name:   name,
			Remote: remote,
		}); err != nil {
			return err
		}

		st, err := store.OpenForProject(cfg.DataDir, slug)
		if err != nil {
			return err
		}
		defer st.Close()

		dbPath := filepath.Join(cfg.DataDir, "projects", slug, "docsiq.db")
		fmt.Printf("✅ project registered\n  slug:   %s\n  name:   %s\n  remote: %s\n  db:     %s\n",
			slug, name, remote, dbPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initName, "name", "", "project display name (defaults to last path component of origin)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing registry entry with the same slug")
}
