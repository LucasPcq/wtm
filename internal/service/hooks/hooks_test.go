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

// unguardedSink is a sink that keeps state without protecting it, which is what
// every surface's sink is: os/exec hands a hook two copier goroutines — Stdout
// and Stderr are distinct writer values, so it never dedupes them — and both
// land here. Under -race this fails unless the runner serializes them.
type unguardedSink struct {
	lines int
	body  []byte
}

func (s *unguardedSink) Write(p []byte) (int, error) {
	s.body = append(s.body, p...)
	s.lines++
	return len(p), nil
}

func TestRunHooksSerializesTheTwoStreamsOntoOneSink(t *testing.T) {
	sink := &unguardedSink{}
	err := RunHooks(RunHooksParams{
		Hooks: []domain.HookCommand{{
			Cmd: "for i in 1 2 3 4 5 6 7 8 9 10; do echo out; echo err >&2; done",
		}},
		WorkDir: t.TempDir(),
		Output:  sink,
		OnHook:  func(domain.HookBeat) {},
	})
	if err != nil {
		t.Fatalf("RunHooks() = %v, want the hook to succeed", err)
	}
	if !strings.Contains(string(sink.body), "out") || !strings.Contains(string(sink.body), "err") {
		t.Errorf("sink holds %q, want both streams", sink.body)
	}
}

// With no reporter installed nobody has drawn the hook's result line, so the
// error is the only place its command can still appear.
func TestRunHooksNamesTheHookWhenNoSurfaceReportedIt(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "exit 3"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
	})
	if err == nil {
		t.Fatal("RunHooks() = nil, want the failing hook reported")
	}
	if !errors.Is(err, domain.ErrHookFailed) {
		t.Errorf("RunHooks() = %v, want it to wrap ErrHookFailed", err)
	}
	if !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("RunHooks() = %q, want the hook named", err)
	}
}

// A surface that reported the beats already printed the command; repeating it in
// the error spells a long install line twice on one screen.
func TestRunHooksLeavesTheHookUnnamedWhenASurfaceReportedIt(t *testing.T) {
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "exit 3"}},
		WorkDir: t.TempDir(),
		Output:  io.Discard,
		OnHook:  func(domain.HookBeat) {},
	})
	if err == nil {
		t.Fatal("RunHooks() = nil, want the failing hook reported")
	}
	if strings.Contains(err.Error(), "exit 3") {
		t.Errorf("RunHooks() = %q, want the hook left unnamed", err)
	}
}
