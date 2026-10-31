package checkout_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/require"
)

func (w world) facts(t *testing.T, req checkout.Request) checkout.Facts {
	t.Helper()
	facts, err := w.usecase.Observe(t.Context(), req)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req checkout.Request) checkout.Questions {
	t.Helper()
	questions, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return questions
}

func (w world) plan(t *testing.T, req checkout.Request) (checkout.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req checkout.Request) (checkout.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(t.Context(), planned, report.Add), report
}

// problems are the field errors err refuses the request with, each by its path.
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

func problemsOf(questions checkout.Questions) map[string]string {
	found := map[string]string{}
	for _, field := range questions.Errors {
		found[field.Path] = field.Code
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

func withHook(command string) string {
	return strings.Replace(projectToml, "on_create = []", "on_create = [{ cmd = "+strconv.Quote(command)+" }]", 1)
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
