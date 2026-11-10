package runinit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// record numbers feat/billing 2.
func (w *world) record(t *testing.T, meta store.Meta) {
	t.Helper()
	_, err := meta.Update(t.Context(), worktree.MetaUpdate{Branch: billing, Change: func(stored *worktree.Meta) error {
		stored.Ordinal, stored.SourceBranch = 2, "main"
		return nil
	}})
	require.NoError(t, err)
}

// apply runs req to its end, every file written.
func (w *world) apply(t *testing.T, req runinit.Request) runinit.Outcome {
	t.Helper()
	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	require.NotNil(t, outcome.Result)
	require.Nil(t, outcome.Result.Failure(), "%+v", outcome.Result.Items)
	return outcome
}

func (w *world) runToml(t *testing.T) string {
	t.Helper()
	return readFile(t, filepath.Join(w.stateDir, "run.toml"))
}

func (w *world) run(t *testing.T) project.Run {
	t.Helper()
	run, err := config.Project{StateDir: w.stateDir}.LoadRun()
	require.NoError(t, err)
	return run
}

// job is the job of name run.toml declares.
func (w *world) job(t *testing.T, name string) project.Job {
	t.Helper()
	for _, job := range w.run(t).Jobs {
		if job.Name == name {
			return job
		}
	}
	t.Fatalf("run.toml declares no job %s", name)
	return project.Job{}
}

func linkedKeys(run project.Run) []string {
	var keys []string
	for _, link := range run.EnvPorts {
		keys = append(keys, link.File+"#"+link.Key)
	}
	return keys
}

func subjects(items []runinit.Item) []string {
	found := []string{}
	for _, item := range items {
		found = append(found, item.Subject)
	}
	return found
}

func codes(followUps []domain.FollowUp) []string {
	var found []string
	for _, followUp := range followUps {
		found = append(found, followUp.Code)
	}
	return found
}

func warningCodes(warnings []domain.Warning) []string {
	var found []string
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func toCheckCodes(problems []domain.FieldError) []string {
	var found []string
	for _, problem := range problems {
		found = append(found, problem.Code)
	}
	return found
}

func candidates(d runinit.Decision) []string {
	var found []string
	for _, c := range d.Candidates {
		found = append(found, c.Name)
	}
	return found
}

func sectionOf(t *testing.T, form domain.Form, id string) domain.Section {
	t.Helper()
	for _, section := range form.Sections {
		if section.ID == id {
			return section
		}
	}
	t.Fatalf("no section %s", id)
	return domain.Section{}
}

func recapRow(t *testing.T, recap []domain.RecapRow, key string) domain.RecapRow {
	t.Helper()
	for _, row := range recap {
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("no recap row %s", key)
	return domain.RecapRow{}
}

func profileNames(profiles []project.Profile) []string {
	var found []string
	for _, profile := range profiles {
		found = append(found, profile.Name)
	}
	return found
}
