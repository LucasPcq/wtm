package wt

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
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

func TestExecRefusesWhatItCannotRun(t *testing.T) {
	execRepo(t, "a")
	cases := map[string][]string{
		"missing dash":     {domain.CmdExec, "a", "--yes"},
		"empty command":    {domain.CmdExec, "a", "--yes", "--"},
		"all with names":   {domain.CmdExec, "a", "--all", "--yes", "--", "true"},
		"json without yes": {domain.CmdExec, "a", "--output", domain.OutputJSON, "--", "true"},
		"negative jobs":    {domain.CmdExec, "a", "--yes", "--jobs", "-1", "--", "true"},
		"no selection":     {domain.CmdExec, "--yes", "--", "true"},
		"bad shell syntax": {domain.CmdExec, "a", "--yes", "--", "if", "then"},
		"unknown worktree": {domain.CmdExec, "nope", "--yes", "--", "true"},
	}
	for name, args := range cases {
		if _, _, err := runWtCmd(t, args...); err == nil {
			t.Errorf("%s: expected a refusal", name)
		}
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
