package cli

import (
	"fmt"
	"os"

	"db-snap/internal/config"
	"db-snap/internal/snapshot"
	"db-snap/internal/tui"
	"github.com/spf13/cobra"
)

func NewRoot(version string) *cobra.Command {
	svc := snapshot.Service{AppVersion: version}
	cmd := &cobra.Command{
		Use:   "db-snap",
		Short: "Local Postgres snapshot/restore tool",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return config.EnsureLayout()
		},
	}

	cmd.AddCommand(newProfileCommand())
	cmd.AddCommand(newSnapshotCommand(svc))
	cmd.AddCommand(newRestoreCommand(svc))
	cmd.AddCommand(newRulesCommand())
	cmd.AddCommand(newPolicyCommand())
	cmd.AddCommand(newHistoryCommand())
	cmd.AddCommand(&cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(version, svc)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	return cmd
}
