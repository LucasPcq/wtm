package cli

import "github.com/spf13/cobra"

const codeConfigNotFound = "create.config_not_found"

func probe() *cobra.Command {
	return &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
}
