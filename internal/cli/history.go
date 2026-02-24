package cli

import (
	"fmt"
	"time"

	"db-snap/internal/history"
	"github.com/spf13/cobra"
)

func newHistoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show audit history",
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := history.List()
			if err != nil {
				return err
			}
			for _, e := range events {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", e.CreatedAt.Format(time.RFC3339), e.Type, e.Profile, e.Snapshot, e.Status)
			}
			return nil
		},
	}
}
