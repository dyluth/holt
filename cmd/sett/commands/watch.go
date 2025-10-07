package commands

import (
	"github.com/dyluth/sett/internal/printer"
	"github.com/spf13/cobra"
)

var (
	watchInstanceName string
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Monitor workflow progress (Phase 2+)",
	Long: `Monitor real-time workflow progress and agent activity.

Phase 1 Note:
  This command will be fully implemented in Phase 2.
  For Phase 1 validation, use: sett forage --watch --goal "..."

Examples (Phase 2+):
  # Watch all activity on inferred instance
  sett watch

  # Watch specific instance
  sett watch --name prod`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInstanceName, "name", "n", "", "Target instance name (auto-inferred if omitted)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	printer.Info("The 'sett watch' command will be available in Phase 2.\n\n")
	printer.Info("For Phase 1, use the --watch flag with forage:\n")
	printer.Info("  sett forage -w -g \"your goal\"\n\n")
	printer.Info("This validates the E2E pipeline:\n")
	printer.Info("  CLI → Artefact → Orchestrator → Claim\n")

	return nil
}
