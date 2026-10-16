package create_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

type ran = dispatchtest.Ran[create.Plan, provision.Detail]

type runParams struct {
	Request create.Request
	Options dispatch.Options
}

func (w world) run(ctx context.Context, params runParams) ran {
	return dispatchtest.Run(ctx, dispatchtest.RunParams[create.Request, create.Facts, create.Plan, provision.Detail]{
		Bench:   w.bench,
		Command: create.Command(w.ports),
		Events:  events(),
		Request: params.Request,
		Options: params.Options,
	})
}

func (w world) apply(t *testing.T, req create.Request) ran {
	t.Helper()
	result := w.run(t.Context(), runParams{Request: req})
	require.NoError(t, result.Err)
	return result
}

func (w world) preview(t *testing.T, req create.Request) kernel.Plan[create.Plan] {
	t.Helper()
	result := w.run(t.Context(), runParams{Request: req, Options: dispatch.Options{DryRun: true}})
	require.NoError(t, result.Err)
	return result.Result.Plan
}

func (w world) facts(t *testing.T) create.Facts {
	t.Helper()
	facts, err := create.Command(w.ports).Observe(t.Context())
	require.NoError(t, err)
	return facts
}

func (w world) form(t *testing.T, req create.Request) kernel.Form[create.Request] {
	t.Helper()
	command := create.Command(w.ports)
	form, err := kernel.Evaluate(kernel.EvaluateParams[create.Request, create.Facts]{Fields: command.Fields, Rules: command.Rules, Request: req, Facts: w.facts(t)})
	require.NoError(t, err)
	return form
}

func events() dispatch.Events[create.Facts, provision.Detail] {
	return dispatch.Events[create.Facts, provision.Detail]{
		Committed: func(e dispatch.EventOf[create.Facts, kernel.Committed[provision.Detail]]) []domain.Event {
			return create.Created(e.Facts, e.Milestone)
		},
		PhaseDone: func(e dispatch.EventOf[create.Facts, kernel.PhaseDone[provision.Detail]]) []domain.Event {
			return create.Updated(e.Facts, e.Milestone)
		},
		Finished: func(e dispatch.EventOf[create.Facts, kernel.Item[provision.Detail]]) []domain.Event {
			return create.Provisioned(e.Facts, e.Milestone)
		},
	}
}

func stateOf(form kernel.Form[create.Request], path string) kernel.FieldState {
	for _, state := range form.States {
		if state.Path == path {
			return state
		}
	}
	return kernel.FieldState{}
}

func problems(form kernel.Form[create.Request]) map[string]kernel.Code {
	found := map[string]kernel.Code{}
	for _, problem := range form.Errors {
		found[problem.Path] = problem.Code
	}
	return found
}

func statuses(items []kernel.Item[provision.Detail]) map[string]kernel.Status {
	found := map[string]kernel.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
	}
	return found
}

func codes(warnings []kernel.Warning) []kernel.Code {
	found := make([]kernel.Code, 0, len(warnings))
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
	return strings.Replace(project, "on_create = []", "on_create = [{ cmd = "+strconv.Quote(command)+" }]", 1)
}

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

func lastOf(events []domain.Event, kind domain.EventType) domain.Event {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type == kind {
			return events[index]
		}
	}
	return domain.Event{}
}
