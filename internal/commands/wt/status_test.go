package wt

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/commands/run/runctx"
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func statusDoc(t *testing.T, args ...string) domain.StatusDocument {
	t.Helper()
	stdout, _, err := runWtCmd(t, append([]string{domain.CmdStatus, "--" + domain.FlagOutput, domain.OutputJSON}, args...)...)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var doc domain.StatusDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	return doc
}

// No --yes in JSON: the command asks nothing, so nothing has to be bypassed.
func TestStatusReadsTheWorktreeItIsRunFromOrTheOneItNames(t *testing.T) {
	dir := createRepo(t)
	branch := "feat/status"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}

	restore := chdir(t, resolveWorktreePath(t, dir, branch))
	inside := statusDoc(t)
	restore()
	named := statusDoc(t, branch)
	restore = chdir(t, dir)
	main := statusDoc(t)
	restore()

	if inside.Branch != branch || named.Branch != branch {
		t.Errorf("inside = %q, named = %q, want %q both", inside.Branch, named.Branch, branch)
	}
	if main.Branch != "main" || !main.Main {
		t.Errorf("main = %+v, want the main checkout", main)
	}
	if inside.Problems == nil || inside.Jobs == nil || inside.Env.Missing == nil {
		t.Errorf("doc = %+v, want empty lists rather than null", inside)
	}
}

func TestStatusAllWritesOneDocumentPerWorktree(t *testing.T) {
	createRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/all", "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdStatus, "--"+domain.FlagAll, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("status --all: %v", err)
	}
	var docs []domain.StatusDocument
	if err := json.Unmarshal([]byte(stdout), &docs); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if len(docs) != 2 {
		t.Errorf("docs = %+v, want main and feat/all", docs)
	}
}

func TestStatusAllRefusesAWorktreeAsAUsageError(t *testing.T) {
	createRepo(t)

	_, _, err := runWtCmd(t, domain.CmdStatus, "main", "--"+domain.FlagAll)

	if !errors.Is(err, domain.ErrUsage) {
		t.Errorf("err = %v, want a usage error", err)
	}
}

func TestStatusTextConcludesOnWhatIsLeftToFix(t *testing.T) {
	restore := chdir(t, createRepo(t))
	defer restore()

	stdout, _, err := runWtCmd(t, domain.CmdStatus)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(stdout, "main — nothing to fix") {
		t.Errorf("stdout = %q, want the headline", stdout)
	}
}

type promptedRun struct {
	interactive []bool
	prompter    *flowtest.ScriptedPrompter
}

// fakeTerminal stands in for a terminal and records which prompter each run
// installed, answering the picker with answer when it is the one asked.
func fakeTerminal(t *testing.T, tty bool, answer string) *promptedRun {
	t.Helper()
	run := &promptedRun{prompter: &flowtest.ScriptedPrompter{Answers: map[string]string{target.KeyWorktree: answer}}}
	previousTTY, previousPrompter := runctx.IsTTY, statusPrompter
	runctx.IsTTY = func() bool { return tty }
	statusPrompter = func(interactive bool) flow.Prompter {
		run.interactive = append(run.interactive, interactive)
		if !interactive {
			return flow.Unattended{}
		}
		return run.prompter
	}
	t.Cleanup(func() { runctx.IsTTY, statusPrompter = previousTTY, previousPrompter })
	return run
}

func TestStatusWithoutAWorktreeOpensThePickerOnTheCurrentOneInATerminal(t *testing.T) {
	dir := createRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/pick", "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}
	picked := resolveWorktreePath(t, dir, "feat/pick")
	run := fakeTerminal(t, true, picked)
	restore := chdir(t, dir)
	defer restore()

	stdout, _, err := runWtCmd(t, domain.CmdStatus)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if run.prompter.AskedKeys() != target.KeyWorktree {
		t.Fatalf("asked %q, want the worktree picker", run.prompter.AskedKeys())
	}
	if start := run.prompter.Content[target.KeyWorktree].Start; !sameDir(t, start, dir) {
		t.Errorf("picker opens on %q, want the current worktree %q", start, dir)
	}
	if !strings.Contains(stdout, "feat/pick — nothing to fix") {
		t.Errorf("stdout = %q, want the picked worktree read", stdout)
	}
}

// An agent never meets the picker: no terminal, or JSON, reads the current
// worktree and asks nothing, so --yes is never needed.
func TestStatusAsksNothingWithoutATerminalOrInJSON(t *testing.T) {
	cases := map[string]struct {
		tty  bool
		args []string
	}{
		"no terminal": {tty: false},
		"json":        {tty: true, args: []string{"--" + domain.FlagOutput, domain.OutputJSON}},
		"--yes":       {tty: true, args: []string{"--" + domain.FlagYes}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := createRepo(t)
			if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/other", "--from", "main", "--"+domain.FlagYes); err != nil {
				t.Fatalf("create: %v", err)
			}
			run := fakeTerminal(t, tc.tty, "")
			restore := chdir(t, dir)
			defer restore()

			stdout, _, err := runWtCmd(t, append([]string{domain.CmdStatus}, tc.args...)...)
			if err != nil {
				t.Fatalf("status: %v", err)
			}
			if len(run.interactive) != 1 || run.interactive[0] || len(run.prompter.Asked) != 0 {
				t.Errorf("interactive = %v, asked %v; want no picker", run.interactive, run.prompter.Asked)
			}
			if !strings.Contains(stdout, "main") {
				t.Errorf("stdout = %q, want the current worktree read", stdout)
			}
		})
	}
}

func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// Backing out of the picker exits like every other picker: 19, nothing read.
// Returning an error there would skip the root's cancelled mark and exit 1.
func TestBackingOutOfTheStatusPickerExitsCancelled(t *testing.T) {
	dir := createRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/esc", "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}
	run := fakeTerminal(t, true, "")
	run.prompter.Abort = true
	restore := chdir(t, dir)
	defer restore()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	err := cmd.Execute()

	if err != nil || !shared.Cancelled(cmd) {
		t.Errorf("err = %v, cancelled = %v; want no error and the cancelled mark (exit 19)", err, shared.Cancelled(cmd))
	}
	if !strings.Contains(out.String(), domain.AbortedMessage) {
		t.Errorf("output = %q, want the aborted line", out.String())
	}
}
