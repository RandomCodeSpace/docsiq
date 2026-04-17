package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RandomCodeSpace/docscontext/internal/project"
	"github.com/spf13/cobra"
)

var migrateInto string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a legacy single-DB install into a per-project layout",
	Long: `Moves $DATA_DIR/DocsContext.db (the pre-Phase-1 single DB) into
$DATA_DIR/projects/<slug>/docscontext.db and registers the slug in the
registry with remote="legacy".

No-op if the legacy DB does not exist. Refuses to overwrite an existing
per-project DB at the target path.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := migrateInto
		if slug == "" {
			return fmt.Errorf("--into <slug> is required")
		}
		if !project.IsValidSlug(slug) {
			return fmt.Errorf("--into %q is not a valid slug (allowed charset: [a-z0-9_-])", slug)
		}

		legacyPath := cfg.DBPath()
		if _, err := os.Stat(legacyPath); errors.Is(err, os.ErrNotExist) {
			fmt.Printf("ℹ️  no legacy DB at %s; nothing to migrate\n", legacyPath)
			return nil
		} else if err != nil {
			return fmt.Errorf("stat legacy db: %w", err)
		}

		targetDir := filepath.Join(cfg.DataDir, "projects", slug)
		targetPath := filepath.Join(targetDir, "docscontext.db")
		if _, err := os.Stat(targetPath); err == nil {
			return fmt.Errorf("refusing to overwrite existing %s; remove it first if you really mean to replace", targetPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat target: %w", err)
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return fmt.Errorf("mkdir target: %w", err)
		}

		if err := os.Rename(legacyPath, targetPath); err != nil {
			return fmt.Errorf("move legacy db: %w", err)
		}
		fmt.Printf("✅ moved %s → %s\n", legacyPath, targetPath)

		// Best-effort: also move the WAL + SHM sidecars if present. A
		// cleanly-closed SQLite won't leave them, but a crash-restarted
		// one will. Ignoring errors here is safe because SQLite recreates
		// them on next open.
		for _, suffix := range []string{"-wal", "-shm"} {
			src := legacyPath + suffix
			dst := targetPath + suffix
			if _, err := os.Stat(src); err == nil {
				if renameErr := os.Rename(src, dst); renameErr == nil {
					fmt.Printf("   (also moved %s)\n", suffix)
				}
			}
		}

		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		if existing, err := reg.Get(slug); err == nil {
			fmt.Printf("ℹ️  slug %q already registered (remote=%s); skipping registry insert\n", slug, existing.Remote)
			return nil
		} else if !errors.Is(err, project.ErrNotFound) {
			return err
		}

		if err := reg.Register(project.Project{
			Slug:   slug,
			Name:   slug,
			Remote: "legacy",
		}); err != nil {
			// A prior `projects register` may already own remote="legacy"
			// — surface the error but don't unwind the file move.
			return fmt.Errorf("registry insert: %w", err)
		}
		fmt.Printf("✅ registered slug=%s remote=legacy\n", slug)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().StringVar(&migrateInto, "into", "", "destination slug for the migrated DB (required)")
	_ = migrateCmd.MarkFlagRequired("into")
}
