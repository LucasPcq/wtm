package hooks

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestRunHooksSuccess(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo hello"}},
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHooksFailure(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "false"}},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestRunHooksStopsOnFirstError(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false"},
			{Cmd: "echo should-not-run"},
		},
		WorkDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunHooksContinueOnError(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{
			{Cmd: "false", ContinueOnError: true},
			{Cmd: "echo should-run"},
		},
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error with continue_on_error: %v", err)
	}
}

func TestRunHooksEmptyList(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   nil,
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("empty hooks should be no-op: %v", err)
	}
}

func TestRunHooksCwdOverride(t *testing.T) {
	dir := t.TempDir()
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "pwd", Cwd: dir}},
		WorkDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInterpolate(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree:   "/trees/feat-auth",
		Branch:     "feature/auth",
		Root:       "/repo",
		FromBranch: "main",
	}

	tests := []struct {
		input string
		want  string
	}{
		{"echo {{branch}}", "echo feature/auth"},
		{"cp {{root}}/.env {{worktree}}/.env", "cp /repo/.env /trees/feat-auth/.env"},
		{"echo from {{from_branch}}", "echo from main"},
		{"no vars here", "no vars here"},
		{"", ""},
	}

	for _, tt := range tests {
		got := rules.Interpolate(tt.input, vars)
		if got != tt.want {
			t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveTemplateVars(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree: "/trees/feat",
		Branch:   "feat",
	}

	hook := domain.HookCommand{
		Cmd: "echo {{branch}}",
		Cwd: "{{worktree}}/apps",
	}

	resolved := rules.ResolveTemplateVars(hook, vars)

	if resolved.Cmd != "echo feat" {
		t.Errorf("cmd: expected 'echo feat', got %q", resolved.Cmd)
	}
	if resolved.Cwd != "/trees/feat/apps" {
		t.Errorf("cwd: expected '/trees/feat/apps', got %q", resolved.Cwd)
	}
}

// A caller that reports the beats itself gets them, and the runner writes no
// decoration of its own — the whole point of the seam is that only one of the
// two draws the phase.
func TestRunHooksReportsEachBeatToTheCaller(t *testing.T) {
	var beats []domain.HookBeat
	var out strings.Builder
	if err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo hello"}},
		WorkDir: t.TempDir(),
		Output:  &out,
		OnHook:  func(beat domain.HookBeat) { beats = append(beats, beat) },
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	if len(beats) != 2 || !beats[0].Started || beats[1].Started {
		t.Fatalf("beats = %+v, want the hook starting then finished", beats)
	}
	if beats[1].Err != "" {
		t.Errorf("a hook that succeeded reported %q", beats[1].Err)
	}
	if got := out.String(); got != "hello\n" {
		t.Errorf("output = %q, want the hook's own output and nothing else", got)
	}
}

// A failing hook hands its stderr to the caller: the surface decides whether to
// show it, and it is gone from the stream by then.
func TestRunHooksCarriesTheFailureStderrOnTheBeat(t *testing.T) {
	var beats []domain.HookBeat
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo boom >&2; false"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
		OnHook:  func(beat domain.HookBeat) { beats = append(beats, beat) },
	})
	if err == nil {
		t.Fatal("expected the failing hook to abort")
	}
	if len(beats) != 2 || beats[1].Err == "" {
		t.Fatalf("beats = %+v, want the failure on the closing beat", beats)
	}
	if beats[1].Stderr != "boom" {
		t.Errorf("stderr = %q, want %q", beats[1].Stderr, "boom")
	}
}

// The error a failed hook returns names the phase and what went wrong, never the
// command: the beat that just went to the surface already spelled it out, and a
// long install command printed twice is the noise this whole seam exists to
// remove.
func TestRunHooksFailureDoesNotRepeatTheCommand(t *testing.T) {
	cmd := "exit 3"
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: cmd}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
		OnHook:  func(domain.HookBeat) {},
	})
	if !errors.Is(err, domain.ErrHookFailed) {
		t.Fatalf("err = %v, want it to identify as a hook failure", err)
	}
	if strings.Contains(err.Error(), cmd) {
		t.Errorf("err = %q, want the command left to the beat", err)
	}
}
