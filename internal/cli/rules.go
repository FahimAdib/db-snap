package cli

import (
	"fmt"

	"db-snap/internal/config"
	"db-snap/internal/model"
	"github.com/spf13/cobra"
)

func newRulesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "Manage auto-fill rules"}
	cmd.AddCommand(newRulesListCommand())
	cmd.AddCommand(newRulesSetCommand())
	cmd.AddCommand(newRulesUnsetCommand())
	return cmd
}

func newRulesListCommand() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := config.LoadRulePack(profile)
			if err != nil {
				return err
			}
			for _, rule := range r.Rules {
				fmt.Fprintf(cmd.OutOrStdout(), "%s.%s.%s\t%s\n", rule.Schema, rule.Table, rule.Column, rule.Value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

func newRulesSetCommand() *cobra.Command {
	var profile, schema, table, column, value, typ string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := config.LoadRulePack(profile)
			if err != nil {
				return err
			}
			next := make([]model.ColumnRule, 0, len(r.Rules)+1)
			replaced := false
			for _, existing := range r.Rules {
				if existing.Schema == schema && existing.Table == table && existing.Column == column {
					next = append(next, model.ColumnRule{Schema: schema, Table: table, Column: column, Value: value, Type: typ, Persistent: true})
					replaced = true
					continue
				}
				next = append(next, existing)
			}
			if !replaced {
				next = append(next, model.ColumnRule{Schema: schema, Table: table, Column: column, Value: value, Type: typ, Persistent: true})
			}
			r.Rules = next
			if err := config.SaveRulePack(r); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "rule saved")
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile")
	cmd.Flags().StringVar(&schema, "schema", "public", "Schema")
	cmd.Flags().StringVar(&table, "table", "", "Table")
	cmd.Flags().StringVar(&column, "column", "", "Column")
	cmd.Flags().StringVar(&value, "value", "", "SQL value expression, e.g. 'foo' or 0")
	cmd.Flags().StringVar(&typ, "type", "static", "Rule type")
	_ = cmd.MarkFlagRequired("profile")
	_ = cmd.MarkFlagRequired("table")
	_ = cmd.MarkFlagRequired("column")
	_ = cmd.MarkFlagRequired("value")
	return cmd
}

func newRulesUnsetCommand() *cobra.Command {
	var profile, schema, table, column string
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Remove rule",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := config.LoadRulePack(profile)
			if err != nil {
				return err
			}
			next := make([]model.ColumnRule, 0, len(r.Rules))
			for _, existing := range r.Rules {
				if existing.Schema == schema && existing.Table == table && existing.Column == column {
					continue
				}
				next = append(next, existing)
			}
			r.Rules = next
			if err := config.SaveRulePack(r); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "rule removed")
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile")
	cmd.Flags().StringVar(&schema, "schema", "public", "Schema")
	cmd.Flags().StringVar(&table, "table", "", "Table")
	cmd.Flags().StringVar(&column, "column", "", "Column")
	_ = cmd.MarkFlagRequired("profile")
	_ = cmd.MarkFlagRequired("table")
	_ = cmd.MarkFlagRequired("column")
	return cmd
}
