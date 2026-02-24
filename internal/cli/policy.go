package cli

import (
	"fmt"
	"strings"

	"db-snap/internal/config"
	"db-snap/internal/policy"
	"github.com/spf13/cobra"
)

func newPolicyCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "policy", Short: "Safety policy settings"}
	cmd.AddCommand(newPolicyShowCommand())
	cmd.AddCommand(newPolicySetCommand())
	cmd.AddCommand(newPolicyValidateCommand())
	return cmd
}

func newPolicyShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "allowCidrs=%s\n", strings.Join(cfg.Policy.AllowCIDRs, ","))
			fmt.Fprintf(cmd.OutOrStdout(), "warnEveryRestore=%v\n", cfg.Policy.WarnEveryRestore)
			fmt.Fprintf(cmd.OutOrStdout(), "denyKeywords=%s\n", strings.Join(cfg.Policy.DenyHostKeywords, ","))
			return nil
		},
	}
}

func newPolicySetCommand() *cobra.Command {
	var allow []string
	var deny []string
	var warn bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			cfg.Policy.AllowCIDRs = allow
			cfg.Policy.DenyHostKeywords = deny
			cfg.Policy.WarnEveryRestore = warn
			if err := config.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "policy updated")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&allow, "allow-cidr", nil, "Allow private CIDR (repeatable)")
	cmd.Flags().StringSliceVar(&deny, "deny-keyword", config.DefaultConfig().Policy.DenyHostKeywords, "Deny hostname keyword")
	cmd.Flags().BoolVar(&warn, "warn-every-restore", true, "Warn every restore on private target")
	return cmd
}

func newPolicyValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <host>",
		Short: "Validate host against policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return err
			}
			res, err := policy.ValidateTarget(args[0], cfg.Policy)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "allowed=%v requiresWarning=%v reason=%s\n", res.Allowed, res.RequiresWarningAck, res.Reason)
			return nil
		},
	}
}
