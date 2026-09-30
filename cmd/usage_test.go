package cmd

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestCobraRefusalsExitWithTheUsageCode(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown command", []string{"bogus"}},
		{"unknown flag", []string{domain.CmdRun, domain.CmdUp, "--bogus"}},
		{"too many arguments", []string{domain.CmdRun, domain.CmdStart, "a", "b"}},
		{"unparsable value", []string{domain.CmdRun, domain.CmdUp, "--detach=maybe"}},
		{"missing value", []string{domain.CmdRun, domain.CmdStop, "--job"}},
		{"unknown output format", []string{domain.CmdRun, domain.CmdPs, "--output", "yaml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(tc.args)
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})
			err := rootCmd.Execute()
			if got := rules.ExitCode(err); got != domain.ExitCodeUsage {
				t.Errorf("exit code = %d (%v), want %d", got, err, domain.ExitCodeUsage)
			}
		})
	}
}

func TestOutputFormatAcceptsWhatTheCommandDeclares(t *testing.T) {
	tree, _, err := rootCmd.Find([]string{domain.CmdTree})
	if err != nil {
		t.Fatalf("find tree: %v", err)
	}
	if err := tree.Flags().Set(domain.FlagOutput, domain.OutputMermaid); err != nil {
		t.Fatalf("set --output: %v", err)
	}
	t.Cleanup(func() { _ = tree.Flags().Set(domain.FlagOutput, domain.OutputText) })
	if err := validateOutputFormat(tree); err != nil {
		t.Errorf("tree --output mermaid refused: %v", err)
	}
}
