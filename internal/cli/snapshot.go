package cli

import (
	"context"
	"fmt"
	"time"

	"db-snap/internal/config"
	"db-snap/internal/snapshot"
	"github.com/spf13/cobra"
)

func newSnapshotCommand(svc snapshot.Service) *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Short: "Manage snapshots"}
	cmd.AddCommand(newSnapshotCreateCommand(svc))
	cmd.AddCommand(newSnapshotListCommand())
	cmd.AddCommand(newSnapshotShowCommand())
	cmd.AddCommand(newSnapshotDeleteCommand())
	cmd.AddCommand(newSnapshotTagCommand())
	cmd.AddCommand(newSnapshotExportCommand(svc))
	cmd.AddCommand(newSnapshotImportCommand(svc))
	return cmd
}

func newSnapshotCreateCommand(svc snapshot.Service) *cobra.Command {
	var opts snapshot.CreateOptions
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Hour)
			defer cancel()
			m, err := svc.Create(ctx, opts)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshot created: %s\n", m.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Profile, "profile", "", "Profile name")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "Snapshot tags")
	cmd.Flags().StringSliceVar(&opts.IncludeTables, "include-table", nil, "Include table(s)")
	cmd.Flags().StringSliceVar(&opts.ExcludeTables, "exclude-table", nil, "Exclude table(s)")
	cmd.Flags().BoolVar(&opts.Deterministic, "deterministic", false, "Deterministic mode")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotListCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			snaps, err := config.ListSnapshots(profile)
			if err != nil {
				return err
			}
			for _, s := range snaps {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", s.ID, s.CreatedAt.Format(time.RFC3339), s.SchemaFingerprint[:12])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotShowCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "show <snapshot-id>",
		Short: "Show snapshot manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest(profile, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "id=%s profile=%s created=%s tags=%v\n", m.ID, m.Profile, m.CreatedAt.Format(time.RFC3339), m.Tags)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotDeleteCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "delete <snapshot-id>",
		Short: "Delete snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.DeleteSnapshot(profile, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotTagCommand() *cobra.Command {
	var profile string
	var tags []string
	cmd := &cobra.Command{
		Use:   "tag <snapshot-id>",
		Short: "Replace snapshot tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest(profile, args[0])
			if err != nil {
				return err
			}
			m.Tags = tags
			if err := config.SaveManifest(profile, m); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated tags for %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tags")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotExportCommand(svc snapshot.Service) *cobra.Command {
	var profile, out string
	cmd := &cobra.Command{
		Use:   "export <snapshot-id>",
		Short: "Export a snapshot directory as tar.gz",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Export(profile, args[0], out); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "snapshot exported")
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringVar(&out, "out", "", "Output tar.gz file")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newSnapshotImportCommand(svc snapshot.Service) *cobra.Command {
	var profile, in string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import snapshot tar.gz into profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := svc.Import(profile, in)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshot imported: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringVar(&in, "in", "", "Input tar.gz file")
	_ = cmd.MarkFlagRequired("profile")
	_ = cmd.MarkFlagRequired("in")
	return cmd
}
