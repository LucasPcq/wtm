package create_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (w world) facts(t *testing.T, branches ...string) create.Facts {
	t.Helper()
	facts, err := w.usecase.Observe(t.Context(), branches)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req create.Request) create.Questions {
	t.Helper()
	return create.NewQuestions(w.facts(t, req.Branches...), req)
}

func (w world) plan(t *testing.T, req create.Request) (create.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func decided(t *testing.T, facts create.Facts, req create.Request) create.Plan {
	t.Helper()
	planned, _ := create.Decide(facts, req)
	return planned
}

// shown is the plan as its JSON shows it, without the facts and resolved answers it carries unexported.
func shown(planned create.Plan) create.Plan {
	return create.Plan{Branches: planned.Branches, From: planned.From, EnvFrom: planned.EnvFrom, Isolation: planned.Isolation, FastForward: planned.FastForward, Remember: planned.Remember}
}

func (w world) apply(t *testing.T, req create.Request) (create.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req create.Request) (create.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

func problems(err error) map[string]string {
	found := map[string]string{}
	invalid, ok := errors.AsType[*domain.Error](err)
	if !ok {
		return found
	}
	for _, field := range invalid.Fields {
		found[field.Path] = field.Code
	}
	return found
}

func statuses(items []create.Item) map[string]domain.Status {
	found := map[string]domain.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func withHook(command string) string {
	return strings.Replace(projectToml, "on_create = []", "on_create = [{ cmd = "+strconv.Quote(command)+" }]", 1)
}

// appears is closed once a file exists at path, polled every 10ms.
func appears(path string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range time.Tick(10 * time.Millisecond) {
			if _, err := os.Stat(path); err == nil {
				return
			}
		}
	}()
	return done
}

func gitAddWorktree(t *testing.T, w world, name, path string) {
	t.Helper()
	gittest.Git(t, w.main, "worktree", "add", "-q", "-b", name, path)
}

func problemsOf(questions create.Questions) map[string]string {
	found := map[string]string{}
	for _, field := range questions.Errors {
		found[field.Path] = field.Code
	}
	return found
}

// behindOrigin puts origin's copy of the branch name one commit ahead of the local branch.
func behindOrigin(t *testing.T, w world, name string) {
	t.Helper()
	ahead := gittest.Output(t, w.main, "commit-tree", name+"^{tree}", "-p", name, "-m", "upstream "+name)
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/"+name, ahead)
}

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func resolve(t *testing.T, w world, req create.Request) (create.Request, error) {
	t.Helper()
	return create.Resolve(w.facts(t, req.Branches...), req)
}

func fieldsOf(t *testing.T, err error) []domain.FieldError {
	t.Helper()
	invalid, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindInvalid, invalid.Kind)
	assert.Equal(t, "request.invalid", invalid.Code)
	return invalid.Fields
}

func messages(t *testing.T, err error) map[string]string {
	t.Helper()
	found := map[string]string{}
	for _, field := range fieldsOf(t, err) {
		found[field.Path] = field.Message("<" + field.Path + ">")
	}
	return found
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func containsKind[T domain.Event](events []domain.Event) bool {
	return slices.ContainsFunc(events, func(event domain.Event) bool {
		_, is := event.(T)
		return is
	})
}

// rebasing checks the branch name out, detached, in a worktree stopped in the middle of a rebase of it.
func rebasing(t *testing.T, w world, name string) {
	t.Helper()
	path := filepath.Join(w.root, "wt-rebasing")
	gittest.Git(t, w.main, "worktree", "add", "-q", "--detach", path, name)
	writeFile(t, filepath.Join(gittest.Output(t, path, "rev-parse", "--absolute-git-dir"), "rebase-merge", "head-name"), "refs/heads/"+name+"\n")
}

// answered is the request the fields of form answer, built by Request.Answer from an empty one, the fields it remembers included.
func answered(t *testing.T, form domain.Form) create.Request {
	t.Helper()
	req, err := domain.Fold(create.Request{}, form.Answers())
	require.NoError(t, err)
	return req
}

func paths(problems []domain.FieldError) []string {
	found := make([]string, 0, len(problems))
	for _, problem := range problems {
		found = append(found, problem.Path)
	}
	return found
}

func fieldAt(form domain.Form, path string) domain.Field {
	for _, field := range form.Fields {
		if field.Spec().Path == path {
			return field
		}
	}

	return nil
}

func valuesOf(options []domain.Option) []string {
	values := make([]string, 0, len(options))
	for _, option := range options {
		values = append(values, option.Value)
	}

	return values
}
