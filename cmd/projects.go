package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/spf13/cobra"
)

var (
	projectsRegisterName string
	projectsDeletePurge  bool
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage docsiq projects (per-git-remote scopes)",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		rows, err := reg.List()
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "SLUG\tNAME\tREMOTE")
		for _, p := range rows {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Slug, p.Name, p.Remote)
		}
		return tw.Flush()
	},
}

var projectsRegisterCmd = &cobra.Command{
	Use:   "register <remote>",
	Short: "Register a project by explicit remote URL (no cwd/git required)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := args[0]
		slug, err := project.Slug(remote)
		if err != nil {
			return err
		}
		name := projectsRegisterName
		if name == "" {
			name = deriveProjectName(remote)
		}
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return err
		}
		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		if err := reg.Register(project.Project{Slug: slug, Name: name, Remote: remote}); err != nil {
			return err
		}
		fmt.Printf("✅ registered: slug=%s name=%s remote=%s\n", slug, name, remote)
		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Delete a project (use --purge to also remove its data directory)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		if err := reg.Delete(slug); err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return fmt.Errorf("slug %q not found", slug)
			}
			return err
		}
		fmt.Printf("✅ deleted registry entry: %s\n", slug)

		if projectsDeletePurge {
			dir := filepath.Join(cfg.DataDir, "projects", slug)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("purge %s: %w", dir, err)
			}
			fmt.Printf("🗑  purged data dir: %s\n", dir)
		}
		return nil
	},
}

var projectsShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show details about a single project (incl. on-disk DB size)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		reg, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return err
		}
		defer reg.Close()

		p, err := reg.Get(slug)
		if err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return fmt.Errorf("slug %q not found", slug)
			}
			return err
		}

		dbPath := filepath.Join(cfg.DataDir, "projects", slug, "docsiq.db")
		size, sizeErr := fileSize(dbPath)

		fmt.Printf("slug:       %s\n", p.Slug)
		fmt.Printf("name:       %s\n", p.Name)
		fmt.Printf("remote:     %s\n", p.Remote)
		fmt.Printf("created_at: %d\n", p.CreatedAt)
		fmt.Printf("db_path:    %s\n", dbPath)
		if sizeErr == nil {
			fmt.Printf("db_size:    %d bytes\n", size)
		} else {
			fmt.Printf("db_size:    (not found; run `docsiq init` or `docsiq serve` first)\n")
		}
		return nil
	},
}

func fileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, err
		}
		return 0, err
	}
	return st.Size(), nil
}

func init() {
	rootCmd.AddCommand(projectsCmd)
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsRegisterCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
	projectsCmd.AddCommand(projectsShowCmd)

	projectsRegisterCmd.Flags().StringVar(&projectsRegisterName, "name", "", "display name (defaults to last path component of remote)")
	projectsDeleteCmd.Flags().BoolVar(&projectsDeletePurge, "purge", false, "also remove $DATA_DIR/projects/<slug>/")
}
