package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"db-snap/internal/config"
	"db-snap/internal/db"
	"db-snap/internal/model"
	"db-snap/internal/policy"
	"github.com/spf13/cobra"
)

func newProfileCommand() *cobra.Command {
	var cmd = &cobra.Command{Use: "profile", Short: "Manage database profiles"}
	cmd.AddCommand(newProfileAddCommand())
	cmd.AddCommand(newProfileListCommand())
	cmd.AddCommand(newProfileRemoveCommand())
	cmd.AddCommand(newProfileTestCommand())
	return cmd
}

func newProfileAddCommand() *cobra.Command {
	var p model.DBProfile
	var password string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			res, err := policy.ValidateTarget(p.Host, cfg.Policy)
			if err != nil {
				return err
			}
			if res.RequiresWarningAck {
				fmt.Fprintln(cmd.OutOrStdout(), "warning: profile target is private network allowlisted")
			}
			now := time.Now().UTC().Format(time.RFC3339)
			existing, err := config.LoadProfile(p.Name)
			if err == nil && existing.CreatedAt != "" {
				p.CreatedAt = existing.CreatedAt
			} else {
				p.CreatedAt = now
			}
			p.UpdatedAt = now
			if err := config.SaveProfile(p); err != nil {
				return err
			}
			if password == "" {
				reader := bufio.NewReader(os.Stdin)
				fmt.Fprint(cmd.OutOrStdout(), "password (optional, press enter to skip): ")
				line, _ := reader.ReadString('\n')
				password = strings.TrimSpace(line)
			}
			if password != "" {
				if err := config.SavePassword(p.Name, password); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved profile %s\n", p.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&p.Name, "name", "", "Profile name")
	cmd.Flags().StringVar(&p.Host, "host", "localhost", "Host")
	cmd.Flags().IntVar(&p.Port, "port", 5432, "Port")
	cmd.Flags().StringVar(&p.Database, "database", "", "Database")
	cmd.Flags().StringVar(&p.User, "user", "", "User")
	cmd.Flags().StringVar(&p.SSLMode, "sslmode", "disable", "sslmode")
	cmd.Flags().StringSliceVar(&p.Tags, "tag", nil, "Tag")
	cmd.Flags().StringVar(&password, "password", "", "Password (stored in keyring)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("database")
	_ = cmd.MarkFlagRequired("user")
	return cmd
}

func newProfileListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, err := config.ListProfiles()
			if err != nil {
				return err
			}
			for _, p := range profiles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s:%d/%s\t%s\n", p.Name, p.Host, p.Port, p.Database, strings.Join(p.Tags, ","))
			}
			return nil
		},
	}
}

func newProfileRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.DeleteProfile(args[0]); err != nil {
				return err
			}
			_ = config.DeletePassword(args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "removed profile %s\n", args[0])
			return nil
		},
	}
}

func newProfileTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test <name>",
		Short: "Test profile connectivity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := config.LoadProfile(args[0])
			if err != nil {
				return err
			}
			pw, _ := config.LoadPassword(p.Name)
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			if err := db.Ping(ctx, p, pw); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
}
