package fastforward_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/require"
)

func (w world) at(name string) string {
	return linkedPath(w.root, name)
}

func (w world) tip(t *testing.T, ref string) string {
	t.Helper()
	return gittest.Output(t, w.main, "rev-parse", ref)
}

// moved reports whether the branch name stands where origin's does.
func (w world) moved(t *testing.T, name string) bool {
	t.Helper()
	return w.tip(t, name) == w.tip(t, "origin/"+name)
}

func (w world) questions(t *testing.T, req fastforward.Request) fastforward.Questions {
	t.Helper()
	q, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return q
}

func (w world) plan(t *testing.T, req fastforward.Request) (fastforward.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req, w.usecase.Git.Fetch(t.Context()))
	require.NoError(t, err)
	return planned, warnings
}

func (w world) run(t *testing.T, req fastforward.Request) (fastforward.Outcome, *operationtest.Recorder) {
	t.Helper()
	report := &operationtest.Recorder{}
	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{Surface: "cli", Report: report.Add})
	require.NoError(t, err)
	return outcome, report
}

// pushed moves origin's branch name on by one commit, as a teammate's push does.
func (w world) pushed(t *testing.T, name string) {
	t.Helper()
	commit := gittest.Output(t, w.main, "commit-tree", "origin/"+name+"^{tree}", "-p", "origin/"+name, "-m", "pushed since")
	gittest.Git(t, w.main, "push", "-q", "origin", commit+":refs/heads/"+name)
}

// stopInARebase starts a rebase in the worktree of name that stops on its first commit, as one waiting for the user.
func (w world) stopInARebase(t *testing.T, name string) {
	t.Helper()
	// the rebase stops on purpose, so git exits non-zero
	_ = exec.Command("git", "-C", w.at(name), "-c", "user.email=dev@acme.dev", "-c", "user.name=Dev", "rebase", "-q", "--exec", "false", "--root").Run()
	require.DirExists(t, gittest.Output(t, w.at(name), "rev-parse", "--git-path", "rebase-merge"))
}

// dirtied leaves an untracked file in the worktree of name.
func (w world) dirtied(t *testing.T, name string) {
	t.Helper()
	writeFile(t, filepath.Join(w.at(name), "untracked"), "work\n")
}

func optionOf(options []domain.Option, value string) domain.Option {
	return options[slices.IndexFunc(options, func(o domain.Option) bool { return o.Value == value })]
}

func naming(names ...string) fastforward.Request {
	return fastforward.Request{Worktrees: names}
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func fieldCodes(fields []domain.FieldError) map[string][]string {
	found := map[string][]string{}
	for _, field := range fields {
		found[field.Path] = append(found[field.Path], field.Code)
	}
	return found
}

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func standings(planned fastforward.Plan) map[string]fastforward.Standing {
	found := map[string]fastforward.Standing{}
	for _, a := range planned.Worktrees {
		found[a.Worktree] = a.Standing
	}
	return found
}

func moving(planned fastforward.Plan) []string {
	var found []string
	for _, a := range planned.Worktrees {
		if len(a.Phases) > 0 {
			found = append(found, a.Worktree)
		}
	}
	return found
}

func itemsOf(outcome fastforward.Outcome) map[string]fastforward.Item {
	found := map[string]fastforward.Item{}
	for _, item := range outcome.Result.Items {
		found[item.Subject] = item
	}
	return found
}

func keptOf(warnings []domain.Warning) []domain.Warning {
	var found []domain.Warning
	for _, warning := range warnings {
		if warning.Code == fastforward.CodeKept {
			found = append(found, warning)
		}
	}
	return found
}

func sortedSubjects(outcome fastforward.Outcome) []string {
	var found []string
	for _, item := range outcome.Result.Items {
		found = append(found, item.Subject)
	}
	return found
}

func eventsOf[T domain.Event](events []domain.Event) []T {
	var found []T
	for _, event := range events {
		if e, is := event.(T); is {
			found = append(found, e)
		}
	}
	return found
}

func valuesOf(options []domain.Option) []string {
	found := make([]string, 0, len(options))
	for _, option := range options {
		found = append(found, option.Value)
	}
	return found
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func fieldOf(form domain.Form, path string) domain.Field {
	at := slices.IndexFunc(form.Fields, func(field domain.Field) bool { return field.Spec().Path == path })
	return form.Fields[at]
}
