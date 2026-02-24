package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"db-snap/internal/config"
	"db-snap/internal/model"
	"db-snap/internal/snapshot"
	"github.com/spf13/cobra"
)

func newRestoreCommand(svc snapshot.Service) *cobra.Command {
	var opts model.RestoreOptions
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Hour)
			defer cancel()

			plan, err := svc.PlanRestore(ctx, opts.Profile, opts.SnapshotID)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compatible=%v warnings=%d requires_prompt=%d\n", plan.Compatible, len(plan.Warnings), len(plan.RequiresPrompt))
			for _, w := range plan.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", w)
			}

			if !opts.AutoFill && len(plan.RequiresPrompt) > 0 {
				if err := promptAndPersistRules(cmd, opts.Profile, plan.RequiresPrompt); err != nil {
					return err
				}
			}

			if opts.DryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "dry-run complete")
				return nil
			}
			if err := svc.Restore(ctx, opts, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "restore complete")
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Profile")
	cmd.Flags().StringVar(&opts.SnapshotID, "snapshot", "", "Snapshot ID")
	cmd.Flags().BoolVar(&opts.AutoFill, "auto", true, "Auto-fill unresolved fields")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Plan only")
	cmd.Flags().BoolVar(&opts.ForceUnsafe, "force-unsafe", false, "Acknowledge warning for private allowlisted hosts")
	cmd.Flags().BoolVar(&opts.TruncateBefore, "truncate-before", true, "Truncate tables before restore")
	_ = cmd.MarkFlagRequired("profile")
	_ = cmd.MarkFlagRequired("snapshot")
	return cmd
}

func promptAndPersistRules(cmd *cobra.Command, profile string, cols []model.ColumnSummary) error {
	rp, err := config.LoadRulePack(profile)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	for _, col := range cols {
		schema, table, column, ok := splitQualified(col.Name)
		if !ok {
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "value for %s (%s), SQL literal or expression (or blank to skip): ", col.Name, col.DataType)
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		if strings.EqualFold(value, "null") {
			value = "NULL"
		}
		rp.Rules = append(rp.Rules, model.ColumnRule{Schema: schema, Table: table, Column: column, Value: value, Type: "prompt", Persistent: true})
	}
	return config.SaveRulePack(rp)
}

func splitQualified(v string) (string, string, string, bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
