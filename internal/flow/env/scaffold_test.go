package env

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// exampleWorktree is a worktree under the example strategy whose .env was
// deleted: every key of its template is a placeholder, so no source can fill it.
func exampleWorktree(t *testing.T) (flow.Context, string) {
	t.Helper()
	ctx := testContext(t)
	withPorts(t, ctx)
	ctx.Config.Project.Env.Strategy = domain.EnvStrategyExample
	ctx.Config.Project.Env.Files = []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
	write(t, filepath.Join(ctx.ProjectDir, ".env.example"), "SHARED=changeme\nWEB_PORT=3000\n")
	gitCommitAll(t, ctx.ProjectDir)
	path := makeWorktree(t, ctx, "feat/a")
	setIsolation(t, ctx, "feat/a", domain.IsolationIsolated)
	if err := os.Remove(filepath.Join(path, ".env")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return ctx, path
}

// A declared .env missing from the worktree is rebuilt as create builds it —
// the template under example — and its ports settled, instead of left absent
// because nobody was there to fill the placeholders.
func TestAnUnattendedRunRebuildsAMissingEnvFromItsTemplate(t *testing.T) {
	ctx, path := exampleWorktree(t)

	outcome, _, err := run(ctx, Request{Worktree: "feat/a"}, flow.Unattended{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := read(t, filepath.Join(path, ".env"))
	if got == "" || !containsLine(got, "SHARED=changeme") || containsLine(got, "WEB_PORT=3000") {
		t.Errorf(".env = %q, want the template with WEB_PORT moved off main's port", got)
	}
	if len(outcome.Result.Files) != 1 || !outcome.Result.Files[0].Created || !outcome.Result.Files[0].Applied {
		t.Errorf("files = %+v, want the file reported created", outcome.Result.Files)
	}
}

func TestTheWizardRebuildsAMissingEnvWithoutAskingForItsPlaceholders(t *testing.T) {
	ctx, path := exampleWorktree(t)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyWorktree: "feat/a", KeyIsolation: domain.EnvKeepValue, KeyRecap: domain.EnvApplyValue}}
	if _, _, err := run(ctx, Request{}, prompter); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(path, ".env")); err != nil {
		t.Errorf(".env not rebuilt: %v", err)
	}
	for _, key := range prompter.Asked {
		if key == KeyResolve {
			t.Errorf("asked %s: a rebuilt file has nothing to resolve", prompter.AskedKeys())
		}
	}
}

func TestACheckReportsAMissingEnvAsDriftAndWritesNothing(t *testing.T) {
	ctx, path := exampleWorktree(t)

	_, _, err := run(ctx, Request{Worktree: "feat/a", Check: true}, flow.Unattended{})

	if !errors.Is(err, domain.ErrEnvDrift) {
		t.Errorf("err = %v, want drift", err)
	}
	if _, statErr := os.Stat(filepath.Join(path, ".env")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("--check wrote the .env: %v", statErr)
	}
}

func gitCommitAll(t *testing.T, dir string) {
	t.Helper()
	gittest.Git(t, dir, "add", ".env.example")
	gittest.Git(t, dir, "commit", "-m", "template")
}

func containsLine(body, line string) bool {
	return slices.Contains(strings.Split(body, "\n"), line)
}
