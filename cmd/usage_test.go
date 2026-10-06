package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestCobraRefusalsExitWithTheUsageCode(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown command", []string{"bogus"}},
		{"unknown subcommand", []string{domain.CmdRun, "bogus"}},
		{"unknown flag", []string{domain.CmdRun, domain.CmdUp, "--bogus"}},
		{"too many arguments", []string{domain.CmdRun, domain.CmdStart, "a", "b"}},
		{"unparsable value", []string{domain.CmdRun, domain.CmdUp, "--detach=maybe"}},
		{"missing value", []string{domain.CmdRun, domain.CmdStop, "--job"}},
		{"unknown output format", []string{domain.CmdRun, domain.CmdPs, "--output", "yaml"}},
		{"checkout of something that is not a PR number", []string{domain.CmdCheckout, "feat/c"}},
		{"checkout of a PR number that is not positive", []string{domain.CmdCheckout, "0", "--yes"}},
		{"checkout of something that is not a PR number, in JSON", []string{domain.CmdCheckout, "feat/c", "--output", "json", "--yes"}},
		{"sync --push --no-push", []string{domain.CmdSync, "--push", "--no-push"}},
		{"sync --ff-parents --no-ff-parents", []string{domain.CmdSync, "--ff-parents", "--no-ff-parents"}},
		{"sync --all with a branch", []string{domain.CmdSync, "--all", "main"}},
		{"fast-forward --all with a branch", []string{domain.CmdFastForward, "--all", "main"}},
		{"run down --all with a worktree", []string{domain.CmdRun, domain.CmdDown, "--all", "main"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if target, _, err := rootCmd.Find(tc.args); err == nil {
				t.Cleanup(func() { resetFlags(target) })
			}
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

// A group run bare still shows its help, and exits 0.
func TestABareGroupShowsItsHelp(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetArgs([]string{domain.CmdRun})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&bytes.Buffer{})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("wtm run: %v", err)
	}
	if out.Len() == 0 {
		t.Error("wtm run printed no help")
	}
}

// `wtm version` repeats cobra's --version line, and --version keeps printing it.
func TestVersionCommandAndFlagPrintTheSameLine(t *testing.T) {
	line := func(args ...string) string {
		var out bytes.Buffer
		rootCmd.SetArgs(args)
		rootCmd.SetOut(&out)
		t.Cleanup(func() { rootCmd.SetArgs(nil); rootCmd.SetOut(nil) })
		if err := rootCmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	flag := line("--version")
	if want := "wtm version " + effectiveVersion + "\n"; flag != want {
		t.Errorf("--version = %q, want %q", flag, want)
	}
	if command := line(domain.CmdVersion); command != flag {
		t.Errorf("wtm version = %q, --version = %q", command, flag)
	}
}

func TestAnInvalidCorrelationIDIsAUsageError(t *testing.T) {
	t.Setenv(domain.EnvCorrelationID, "a\nb")
	rootCmd.SetArgs([]string{domain.CmdVersion})
	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	err := rootCmd.Execute()
	if got := rules.ExitCode(err); got != domain.ExitCodeUsage {
		t.Fatalf("exit code = %d (%v), want %d", got, err, domain.ExitCodeUsage)
	}
}

func TestEveryMutuallyExclusivePairExitsWithTheUsageCode(t *testing.T) {
	pairs := exclusivePairs(rootCmd)
	if len(pairs) == 0 {
		t.Fatal("no mutually exclusive flag group found in the command tree")
	}
	for _, pair := range pairs {
		t.Run(strings.Join(pair.args, " "), func(t *testing.T) {
			t.Cleanup(func() { resetFlags(pair.cmd) })
			rootCmd.SetArgs(pair.args)
			rootCmd.SetOut(&bytes.Buffer{})
			rootCmd.SetErr(&bytes.Buffer{})
			err := rootCmd.Execute()
			if got := rules.ExitCode(err); got != domain.ExitCodeUsage {
				t.Errorf("exit code = %d (%v), want %d", got, err, domain.ExitCodeUsage)
			}
		})
	}
}

type exclusivePair struct {
	cmd  *cobra.Command
	args []string
}

func exclusivePairs(cmd *cobra.Command) []exclusivePair {
	var pairs []exclusivePair
	seen := map[string]bool{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		for _, group := range f.Annotations[domain.AnnotationMutuallyExclusive] {
			if seen[group] {
				continue
			}
			seen[group] = true
			names := strings.Fields(group)
			for i, first := range names {
				for _, second := range names[i+1:] {
					path := strings.Fields(cmd.CommandPath())[1:]
					args := append(path, flagArg(cmd, first), flagArg(cmd, second))
					pairs = append(pairs, exclusivePair{cmd: cmd, args: args})
				}
			}
		}
	})
	for _, sub := range cmd.Commands() {
		pairs = append(pairs, exclusivePairs(sub)...)
	}
	return pairs
}

func flagArg(cmd *cobra.Command, name string) string {
	if cmd.Flags().Lookup(name).Value.Type() == "bool" {
		return "--" + name
	}
	return "--" + name + "=x"
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

// Every command that could ask refuses --output json without --yes as a usage
// error, before it reads anything (LUC-273). The list is spelled out so that a
// command losing its mark fails here rather than slipping back to exit 1.
func TestJSONWithoutYesIsAUsageError(t *testing.T) {
	want := map[string]string{
		"wtm create":       "--yes",
		"wtm checkout":     "--yes",
		"wtm extract":      "--yes",
		"wtm exec":         "--yes",
		"wtm clean":        "--yes",
		"wtm fast-forward": "--yes",
		"wtm reparent":     "--yes",
		"wtm sync":         "--yes or --dry-run",
		"wtm prune":        "--yes or --dry-run",
		"wtm relocate":     "--yes or --dry-run",
		"wtm env":          "--yes or --check",
		"wtm upgrade":      "--yes or --check",
		"wtm run import":   "--yes",
	}
	marked := map[string]*cobra.Command{}
	walk(rootCmd, func(cmd *cobra.Command) {
		if _, ok := cmd.Annotations[domain.AnnotationJSONNeedsYes]; ok {
			marked[cmd.CommandPath()] = cmd
		}
	})
	for path := range want {
		if marked[path] == nil {
			t.Errorf("%s does not refuse --output json without --yes", path)
		}
	}
	for path := range marked {
		if _, ok := want[path]; !ok {
			t.Errorf("%s refuses --output json without --yes but is not listed here", path)
		}
	}

	t.Chdir(t.TempDir())
	t.Setenv("HOME", t.TempDir())
	for path, flags := range want {
		t.Run(path, func(t *testing.T) {
			if cmd := marked[path]; cmd != nil {
				t.Cleanup(func() { resetFlags(cmd) })
			}
			rootCmd.SetArgs(append(strings.Fields(path)[1:], "--output", "json"))
			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&bytes.Buffer{})
			err := rootCmd.Execute()
			if got := rules.ExitCode(err); got != domain.ExitCodeUsage {
				t.Errorf("exit code = %d (%v), want %d", got, err, domain.ExitCodeUsage)
			}
			if message := fmt.Sprintf(domain.JSONNeedsYesFmt, flags); err == nil || err.Error() != message {
				t.Errorf("error = %v, want %q", err, message)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want nothing", out.String())
			}
		})
	}
}

func TestJSONRunsUnattendedUnderAnyFlagThatKeepsTheCommandFromAsking(t *testing.T) {
	cases := [][]string{
		{domain.CmdCheckout, "--output", "json", "--yes"},
		{domain.CmdSync, "--output", "json", "--dry-run"},
		{domain.CmdEnv, "--output", "json", "--check"},
		{domain.CmdCheckout, "--output", "text"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd, flags, err := rootCmd.Find(args)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { resetFlags(cmd) })
			if err := cmd.ParseFlags(flags); err != nil {
				t.Fatal(err)
			}
			if err := shared.RefuseJSONWithoutYes(cmd); err != nil {
				t.Errorf("refused: %v", err)
			}
		})
	}
}

func walk(cmd *cobra.Command, visit func(*cobra.Command)) {
	visit(cmd)
	for _, sub := range cmd.Commands() {
		walk(sub, visit)
	}
}
