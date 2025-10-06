package commands

import (
	"fmt"

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
	watchCmd.Flags().StringVar(&watchInstanceName, "name", "", "Target instance name (auto-inferred if omitted)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	fmt.Printf("The 'sett watch' command will be available in Phase 2.\n\n")
	fmt.Printf("For Phase 1, use the --watch flag with forage:\n")
	fmt.Printf("  sett forage --watch --goal \"your goal\"\n\n")
	fmt.Printf("This validates the E2E pipeline:\n")
	fmt.Printf("  CLI → Artefact → Orchestrator → Claim\n")

	return nil
}
