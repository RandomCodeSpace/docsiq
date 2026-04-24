package cmd

import (
	"fmt"

	"github.com/RandomCodeSpace/docsiq/internal/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of docsiq",
	Run: func(cmd *cobra.Command, args []string) {
		info := buildinfo.Resolve(false)
		dirtySuffix := ""
		if info.Dirty == "true" {
			dirtySuffix = " (dirty)"
		}
		fmt.Printf("docsiq %s (commit: %s, built: %s)%s\n",
			info.Version, info.Commit, info.BuildDate, dirtySuffix)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
