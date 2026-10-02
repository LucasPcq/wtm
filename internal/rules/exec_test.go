package rules

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestExecCommandLineJoinsArgsAndRefusesEmpty(t *testing.T) {
	line, err := ExecCommandLine([]string{"pnpm", "test", "&&", "pnpm", "lint"})
	if err != nil || line != "pnpm test && pnpm lint" {
		t.Fatalf("got %q, %v", line, err)
	}
	for _, args := range [][]string{nil, {}, {"  ", ""}} {
		if _, err := ExecCommandLine(args); !errors.Is(err, domain.ErrExecNoCommand) {
			t.Errorf("args %q: err = %v", args, err)
		}
	}
}

func TestExecLogPathEncodesTheBranch(t *testing.T) {
	got := ExecLogPath(ExecLogPathParams{StateDir: "/s", Branch: "feat/x"})
	want := filepath.Join("/s", domain.ExecLogDirName, EncodeBranchSegment("feat/x")+domain.ExecLogFileExt)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	other := ExecLogPath(ExecLogPathParams{StateDir: "/s", Branch: "feat-x"})
	if other == got {
		t.Fatalf("feat/x and feat-x share a log path: %q", got)
	}
	if ExecLogPath(ExecLogPathParams{Branch: "a"}) != "" {
		t.Fatal("no state dir must mean no log")
	}
}

func TestResolveExecTargetsKeepsTheOrderTypedAndRefusesUnknown(t *testing.T) {
	candidates := []domain.GitWorktree{{Branch: "main", Path: "/r"}, {Branch: "a", Path: "/w/a"}, {Branch: "b", Path: "/w/b"}}
	got, err := ResolveExecTargets(ResolveExecTargetsParams{Candidates: candidates, Names: []string{"b", "main"}})
	if err != nil || len(got) != 2 || got[0].Branch != "b" || got[1].Branch != "main" {
		t.Fatalf("got %v, %v", got, err)
	}
	_, err = ResolveExecTargets(ResolveExecTargetsParams{Candidates: candidates, Names: []string{"a", "nope", "zip"}})
	if !errors.Is(err, domain.ErrExecUnknownWorktree) {
		t.Fatalf("err = %v", err)
	}
}

func TestExecCurrentPicksTheDeepestWorktreeHoldingTheDir(t *testing.T) {
	candidates := []domain.GitWorktree{{Branch: "main", Path: "/r"}, {Branch: "a", Path: "/r/.worktrees/a"}}
	cases := map[string]string{
		"/r":                    "main",
		"/r/src":                "main",
		"/r/.worktrees/a":       "a",
		"/r/.worktrees/a/pkg/x": "a",
		"/r/.worktrees/ab":      "main",
		"/elsewhere":            "",
	}
	for dir, want := range cases {
		if got := ExecCurrent(ExecCurrentParams{Candidates: candidates, Dir: dir}); got != want {
			t.Errorf("dir %q: got %q, want %q", dir, got, want)
		}
	}
}

func TestCountExecAndFailedBranches(t *testing.T) {
	results := []domain.ExecResult{
		{Branch: "a", Status: domain.ExecStatusPassed},
		{Branch: "b", Status: domain.ExecStatusFailed},
		{Branch: "c", Status: domain.ExecStatusInterrupted},
		{Branch: "d", Status: domain.ExecStatusNotStarted},
	}
	if got := CountExec(results); got != (domain.ExecCounts{Passed: 1, Failed: 1, Interrupted: 1, NotStarted: 1}) {
		t.Fatalf("counts = %+v", got)
	}
	failed := ExecFailedBranches(results)
	if len(failed) != 3 || failed[0] != "b" || failed[2] != "d" {
		t.Fatalf("failed = %v", failed)
	}
	if ExecFailedBranches(nil) == nil {
		t.Fatal("failed must never be nil: it is a JSON array")
	}
}

func TestExecResultLabel(t *testing.T) {
	one := 1
	zero := 0
	cases := []struct {
		result domain.ExecResult
		want   string
	}{
		{domain.ExecResult{Branch: "a", Status: domain.ExecStatusPassed, ExitCode: &zero, DurationMs: 12400}, "a (12.4s)"},
		{domain.ExecResult{Branch: "b", Status: domain.ExecStatusFailed, ExitCode: &one, DurationMs: 3200}, "b (exit 1, 3.2s)"},
		{domain.ExecResult{Branch: "c", Status: domain.ExecStatusInterrupted}, "c  interrupted"},
		{domain.ExecResult{Branch: "d", Status: domain.ExecStatusNotStarted}, "d  not started"},
		{domain.ExecResult{Branch: "e", Status: domain.ExecStatusFailed, Error: "chdir: no such file"}, "e  chdir: no such file"},
	}
	for _, c := range cases {
		if got := ExecResultLabel(c.result); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
