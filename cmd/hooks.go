package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/RandomCodeSpace/docsiq/internal/hookinstaller"
	"github.com/spf13/cobra"
)

var (
	hooksClient  string
	hooksDryRun  bool
	hooksHookURL string
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Install / remove / inspect SessionStart hooks for AI clients",
	Long: `Manage docsiq's SessionStart hook registration with supported AI clients.

Supported clients: claude, cursor, copilot, codex (use --client=all to target all).
The hook script is extracted to $DATA_DIR/hooks/hook.sh and invoked by each
client when a coding session starts.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install hook.sh and register with selected client(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd.OutOrStdout())
	},
}

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove hook registration from selected client(s)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUninstall(cmd.OutOrStdout())
	},
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report hook installation status per client",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus(cmd.OutOrStdout())
	},
}

// hookDestPath returns the canonical location for the extracted hook
// script: $DATA_DIR/hooks/hook.sh.
func hookDestPath() string {
	return filepath.Join(cfg.DataDir, "hooks", "hook.sh")
}

// selectedInstallers resolves --client=... to the list of installers to
// operate on. Empty / "all" → everything; specific name → 1 installer;
// unknown name → error.
func selectedInstallers(client string) ([]hookinstaller.Installer, error) {
	client = strings.TrimSpace(strings.ToLower(client))
	if client == "" || client == "all" {
		return hookinstaller.All(), nil
	}
	inst, err := hookinstaller.ByName(client)
	if err != nil {
		return nil, err
	}
	return []hookinstaller.Installer{inst}, nil
}

func runInstall(out io.Writer) error {
	insts, err := selectedInstallers(hooksClient)
	if err != nil {
		return err
	}
	dest := hookDestPath()

	if hooksDryRun {
		fmt.Fprintf(out, "[dry-run] would extract hook script to %s\n", dest)
		for _, inst := range insts {
			path, pErr := inst.ConfigPath()
			if pErr != nil {
				fmt.Fprintf(out, "[dry-run] %s: config unavailable (%v)\n", inst.Name(), pErr)
				continue
			}
			fmt.Fprintf(out, "[dry-run] %s: would register hook at %s in %s\n",
				inst.Name(), dest, path)
		}
		return nil
	}

	if err := hookinstaller.ExtractHookScript(dest); err != nil {
		return fmt.Errorf("extract hook script: %w", err)
	}
	fmt.Fprintf(out, "✅ hook script installed at %s\n", dest)

	// Install per client. One client's failure doesn't stop the others —
	// we collect them and return a combined error at the end.
	var errs []string
	for _, inst := range insts {
		if err := inst.Install(dest); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", inst.Name(), err))
			fmt.Fprintf(out, "⚠ %s: install failed: %v\n", inst.Name(), err)
			continue
		}
		path, _ := inst.ConfigPath()
		fmt.Fprintf(out, "✅ %s: registered in %s\n", inst.Name(), path)
	}
	if len(errs) > 0 {
		return errors.New("some installers failed: " + strings.Join(errs, "; "))
	}
	return nil
}

func runUninstall(out io.Writer) error {
	insts, err := selectedInstallers(hooksClient)
	if err != nil {
		return err
	}
	if hooksDryRun {
		for _, inst := range insts {
			path, pErr := inst.ConfigPath()
			if pErr != nil {
				fmt.Fprintf(out, "[dry-run] %s: config unavailable\n", inst.Name())
				continue
			}
			fmt.Fprintf(out, "[dry-run] %s: would remove entry from %s\n", inst.Name(), path)
		}
		return nil
	}

	var errs []string
	for _, inst := range insts {
		if err := inst.Uninstall(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", inst.Name(), err))
			fmt.Fprintf(out, "⚠ %s: uninstall failed: %v\n", inst.Name(), err)
			continue
		}
		fmt.Fprintf(out, "✅ %s: uninstalled\n", inst.Name())
	}
	if len(errs) > 0 {
		return errors.New("some uninstallers failed: " + strings.Join(errs, "; "))
	}
	return nil
}

func runStatus(out io.Writer) error {
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CLIENT\tSTATUS\tDETAIL")
	for _, inst := range hookinstaller.All() {
		installed, detail := inst.Status()
		label := "⚠ not installed"
		if installed {
			label = "✅ installed"
		}
		path, pErr := inst.ConfigPath()
		if pErr != nil {
			label = "⚠ config unavailable"
			detail = pErr.Error()
		} else if detail == "" {
			detail = path
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", inst.Name(), label, detail)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	// Report the hook script presence separately.
	dest := hookDestPath()
	if _, err := os.Stat(dest); err == nil {
		fmt.Fprintf(out, "\nhook script: %s\n", dest)
	} else {
		fmt.Fprintf(out, "\nhook script: not extracted (run `docsiq hooks install`)\n")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(hooksCmd)
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)
	hooksCmd.AddCommand(hooksStatusCmd)

	for _, c := range []*cobra.Command{hooksInstallCmd, hooksUninstallCmd} {
		c.Flags().StringVar(&hooksClient, "client", "all",
			"which client to (un)install hooks for: all|claude|cursor|copilot|codex")
		c.Flags().BoolVar(&hooksDryRun, "dry-run", false,
			"print what would happen without writing files")
	}
	hooksInstallCmd.Flags().StringVar(&hooksHookURL, "hook-url", "",
		"override hook server URL (default: http://127.0.0.1:$DOCSIQ_SERVER_PORT)")
}
