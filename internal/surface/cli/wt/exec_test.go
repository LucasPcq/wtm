package wt

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func execRepo(t *testing.T, branches ...string) string {
	t.Helper()
	dir := batchRepo(t)
	for _, branch := range branches {
		gittest.Git(t, dir, "worktree", "add", "-b", branch, filepath.Join(t.TempDir(), branch), "main")
	}
	return dir
}

func TestExecJSONRunsNamedWorktrees(t *testing.T) {
	execRepo(t, "a", "b")
	stdout, _, err := runWtCmd(t, domain.CmdExec, "a", "b", "--yes", "--output", domain.OutputJSON, "--", "echo", "hi")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Command string `json:"command"`
		Results []struct {
			Branch string   `json:"branch"`
			Status string   `json:"status"`
			Tail   []string `json:"tail"`
		} `json:"results"`
		Failed []string `json:"failed"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	if doc.Command != "echo hi" || len(doc.Results) != 2 || doc.Results[0].Branch != "a" || doc.Results[0].Tail[0] != "hi" || doc.Failed == nil || len(doc.Failed) != 0 {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestExecRefusesWhatItCannotRunWithItsExitCode(t *testing.T) {
	execRepo(t, "a")
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"missing dash", []string{domain.CmdExec, "a", "--yes"}, domain.ExitCodeUsage},
		{"empty command", []string{domain.CmdExec, "a", "--yes", "--"}, domain.ExitCodeUsage},
		{"all with names", []string{domain.CmdExec, "a", "--all", "--yes", "--", "true"}, domain.ExitCodeUsage},
		{"negative jobs", []string{domain.CmdExec, "a", "--yes", "--jobs", "-1", "--", "true"}, domain.ExitCodeUsage},
		{"bad shell syntax", []string{domain.CmdExec, "a", "--yes", "--", "if", "then"}, domain.ExitCodeUsage},
		{"unknown worktree", []string{domain.CmdExec, "nope", "--yes", "--", "true"}, domain.ExitCodeBranchNotFound},
		{"json without yes", []string{domain.CmdExec, "a", "--output", domain.OutputJSON, "--", "true"}, domain.ExitCodeUsage},
		{"no selection", []string{domain.CmdExec, "--yes", "--", "true"}, domain.ExitCodeError},
		{"no selection, no terminal", []string{domain.CmdExec, "--", "true"}, domain.ExitCodeError},
	}
	for _, c := range cases {
		_, _, err := runWtCmd(t, c.args...)
		if err == nil {
			t.Errorf("%s: expected a refusal", c.name)
			continue
		}
		if got := rules.ExitCode(err); got != c.code {
			t.Errorf("%s: exit %d, want %d (%v)", c.name, got, c.code, err)
		}
	}
}

func TestExecWithNamesRunsWithoutATerminalOrYes(t *testing.T) {
	execRepo(t, "a")
	if _, _, err := runWtCmd(t, domain.CmdExec, "a", "--", "true"); err != nil {
		t.Fatalf("nothing needs picking, so nothing needs a terminal: %v", err)
	}
}

func TestExecFailureExitsAbortedAfterTheReport(t *testing.T) {
	execRepo(t, "a")
	stdout, _, err := runWtCmd(t, domain.CmdExec, "a", "--yes", "--", "exit", "3")
	if !errors.Is(err, domain.ErrAborted) || !strings.Contains(ansi.Strip(stdout), "exit 3") {
		t.Fatalf("err = %v, stdout = %q", err, stdout)
	}
}

func TestExecPrintShowsEachOutput(t *testing.T) {
	execRepo(t, "a", "b")
	stdout, _, err := runWtCmd(t, domain.CmdExec, "a", "b", "--yes", "--print", "--", "git", "branch", "--show-current")
	got := ansi.Strip(stdout)
	if err != nil || !strings.Contains(got, "a\n") || !strings.Contains(got, "2 worktrees") {
		t.Fatalf("err = %v, stdout = %q", err, got)
	}
}

func TestExecJobsDefaultDoesNotDependOnTheMachine(t *testing.T) {
	if def := newExecCmd().Flags().Lookup(domain.FlagJobs).DefValue; def != "0" {
		t.Fatalf("--jobs default = %q: the generated docs would carry this machine's CPU count", def)
	}
}
