package run

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   domain.CmdImport + " [file]",
		Short: "Replace run.toml with a JSON run config",
		Long: `Read a JSON run config payload from a file (or stdin) and make it the run.toml.

Pass "-" or omit the argument to read from stdin.

The payload replaces the whole file — jobs, profiles, .env port links and
project settings alike. The run is confirmed before anything is written; pass
--yes to run unattended.

Nothing is reconciled after the write: run wtm env to settle the .env files
against the new configuration.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runImport,
	}
	shared.AddYesFlag(cmd, "Replace run.toml without confirming")
	shared.AddOutputFlag(cmd)
	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	result, err := shared.LoadConfig(cmd, dir)
	if err != nil {
		return err
	}

	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	yes, _ := cmd.Flags().GetBool(domain.FlagYes)
	if format == domain.OutputJSON && !yes {
		return fmt.Errorf("--%s %s requires --%s", domain.FlagOutput, domain.OutputJSON, domain.FlagYes)
	}

	data, err := readImportSource(args)
	if err != nil {
		return err
	}

	var incoming domain.RunConfig
	if err := json.Unmarshal(data, &incoming); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	if _, errs := rules.ValidateRun(incoming); len(errs) > 0 {
		return fmt.Errorf("invalid run config:\n  %s", strings.Join(errs, "\n  "))
	}

	// Replacing run.toml is destructive, so it is never the default of a run that
	// cannot be asked — a piped payload included, where stdin carries the config
	// and there is nothing left to prompt on.
	interactive := shared.Interactive(shared.UnattendedParams{TTY: isTTY(), Format: format, Yes: yes}) &&
		!readsStdin(args)
	if !interactive && !yes {
		return fmt.Errorf(domain.ImportNeedsYesFmt, domain.FlagYes)
	}

	if !confirmImport(confirmImportParams{Interactive: interactive, Incoming: incoming}) {
		output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
			output.Message(w, domain.ImportDeclined)
		})
		return nil
	}

	if err := config.WriteRun(config.WriteRunParams{
		StateDir: result.StateDir,
		Config:   incoming,
		Force:    true,
	}); err != nil {
		return fmt.Errorf("write run config: %w", err)
	}

	if err := reportImport(cmd, incoming, format); err != nil {
		return err
	}
	if rules.IsHumanFormat(format) {
		output.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
			noticeAddressingDrift(w, result, result.ProjectDir)
		})
	}
	return nil
}

type confirmImportParams struct {
	Interactive bool
	Incoming    domain.RunConfig
}

// confirmImport reads Esc as a refusal, like every other standalone confirm in
// the CLI does.
func confirmImport(params confirmImportParams) bool {
	if !params.Interactive {
		return true
	}
	confirmed, _ := components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
		Title: domain.ImportConfirmTitle,
		Description: fmt.Sprintf(domain.ImportConfirmDescFmt,
			len(params.Incoming.Jobs), len(params.Incoming.Profiles)),
		DefaultYes: false,
	}))
	return confirmed
}

// readsStdin says the payload comes from the same stream a prompt would read.
func readsStdin(args []string) bool { return len(args) == 0 || args[0] == "-" }

func reportImport(cmd *cobra.Command, cfg domain.RunConfig, format string) error {
	ir := output.ImportResult{EnvPorts: len(cfg.EnvPorts)}
	for _, job := range cfg.Jobs {
		ir.Jobs = append(ir.Jobs, job.Name)
	}
	for _, profile := range cfg.Profiles {
		ir.Profiles = append(ir.Profiles, profile.Name)
	}

	if format == domain.OutputJSON {
		return output.WriteImportResultJSON(cmd.OutOrStdout(), ir)
	}
	output.Frame(cmd.OutOrStdout(), func(w io.Writer) {
		output.WriteImportResultText(w, ir)
	})
	return nil
}

func readImportSource(args []string) ([]byte, error) {
	if len(args) == 0 || args[0] == "-" {
		return io.ReadAll(os.Stdin)
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", args[0], err)
	}
	return data, nil
}
