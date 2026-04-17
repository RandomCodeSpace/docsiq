package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/spf13/cobra"
)

var (
	statsJSON    bool
	statsProject string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show index statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := statsProject
		if slug == "" {
			slug = cfg.DefaultProject
		}
		if slug == "" {
			slug = "_default"
		}
		st, err := store.OpenForProject(cfg.DataDir, slug)
		if err != nil {
			return fmt.Errorf("open store for project %q: %w", slug, err)
		}
		defer st.Close()

		s, err := st.GetStats(cmd.Context())
		if err != nil {
			return err
		}

		if statsJSON {
			return json.NewEncoder(os.Stdout).Encode(s)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Project\t%s\n", slug)
		fmt.Fprintln(w, "Metric\tCount")
		fmt.Fprintln(w, "------\t-----")
		fmt.Fprintf(w, "Documents\t%d\n", s.Documents)
		fmt.Fprintf(w, "Chunks\t%d\n", s.Chunks)
		fmt.Fprintf(w, "Embeddings\t%d\n", s.Embeddings)
		fmt.Fprintf(w, "Entities\t%d\n", s.Entities)
		fmt.Fprintf(w, "Relationships\t%d\n", s.Relationships)
		fmt.Fprintf(w, "Claims\t%d\n", s.Claims)
		fmt.Fprintf(w, "Communities\t%d\n", s.Communities)
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output as JSON")
	statsCmd.Flags().StringVar(&statsProject, "project", "", "Project slug (default: config default_project, then _default)")
}
