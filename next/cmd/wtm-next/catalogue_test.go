package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheCatalogueListsEachCommandWithItsSchemaOutsideAnyRepository(t *testing.T) {
	var names []string
	for _, command := range catalogue() {
		names = append(names, command.Name)
		assert.NotEmpty(t, command.Form.Fields, command.Name)
		assert.NotNil(t, command.Questions, "%s asks no questions through the catalogue", command.Name)
	}

	assert.Equal(t, []string{"create", "extract", "list"}, names)
}

func TestCreateIsDrivenThroughTheCataloguePreviewedThenAppliedWithItsFingerprint(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	create := command(t, "create")
	request := json.RawMessage(`{"branches": ["feat/x"]}`)

	preview := create.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{DryRun: true}})
	stale := create.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{ExpectFingerprint: "000000000000000000000000"}})
	applied := create.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{ExpectFingerprint: preview.Envelope.Fingerprint}})

	require.Equal(t, http.StatusOK, preview.Status, preview.Envelope.Err)
	assert.NotEmpty(t, preview.Envelope.Fingerprint)
	assert.NotNil(t, preview.Envelope.Plan)
	assert.Equal(t, http.StatusPreconditionFailed, stale.Status)
	assert.Equal(t, domain.CodePlanStale, stale.Envelope.Err.Code)
	require.Equal(t, http.StatusOK, applied.Status, applied.Envelope.Err)
	require.Len(t, applied.Envelope.Items, 1)
	assert.Equal(t, domain.StatusDone, applied.Envelope.Items[0].Status)
	assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
}

func TestExtractIsDrivenThroughTheCataloguePreviewedThenAppliedWithItsFingerprint(t *testing.T) {
	main := configured(t)
	require.NoError(t, os.WriteFile(filepath.Join(main, "notes.txt"), []byte("notes\n"), 0o644))
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	extract := command(t, "extract")
	request := json.RawMessage(`{"source": "main", "files": ["notes.txt"], "to": "feat/notes"}`)

	preview := extract.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{DryRun: true}})
	stale := extract.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{ExpectFingerprint: "000000000000000000000000"}})
	applied := extract.Execute(t.Context(), api.Call{Dir: main, Request: request, Options: domain.RunOptions{ExpectFingerprint: preview.Envelope.Fingerprint}})

	require.Equal(t, http.StatusOK, preview.Status, preview.Envelope.Err)
	assert.NotEmpty(t, preview.Envelope.Fingerprint)
	assert.Equal(t, http.StatusPreconditionFailed, stale.Status)
	assert.Equal(t, domain.CodePlanStale, stale.Envelope.Err.Code)
	require.Equal(t, http.StatusOK, applied.Status, applied.Envelope.Err)
	assert.Equal(t, domain.StatusDone, applied.Envelope.Items[0].Status)
	assert.FileExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-notes", "notes.txt"))
	assert.NoFileExists(t, filepath.Join(main, "notes.txt"))
}

func TestCreateIsDrivenThroughTheCatalogueByItsAnswers(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")
	create := command(t, "create")
	answers := []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}}

	empty, err := create.Questions(t.Context(), api.Call{Dir: main})
	require.NoError(t, err)
	asked, err := create.Questions(t.Context(), api.Call{Dir: main, Answers: answers})
	require.NoError(t, err)
	preview := create.Execute(t.Context(), api.Call{Dir: main, Answers: asked.Answers(), Options: domain.RunOptions{DryRun: true}})
	applied := create.Execute(t.Context(), api.Call{Dir: main, Answers: asked.Answers(), Options: domain.RunOptions{ExpectFingerprint: preview.Envelope.Fingerprint}})

	assert.NotEmpty(t, domain.ProblemsOf(empty.Problems, "branches"), "an empty request is asked its branches")
	assert.Empty(t, domain.ProblemsOf(asked.Problems, "branches"))
	require.Equal(t, http.StatusOK, preview.Status, preview.Envelope.Err)
	require.Equal(t, http.StatusOK, applied.Status, applied.Envelope.Err)
	assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
}

func TestListIsReadThroughTheCatalogueWithNoOperation(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(stateDirEnv, "")

	read := command(t, "list").Execute(t.Context(), api.Call{Dir: main, Request: json.RawMessage(`{}`)})

	require.Equal(t, http.StatusOK, read.Status, read.Envelope.Err)
	require.Len(t, read.Envelope.Items, 1)
	assert.Equal(t, "main", read.Envelope.Items[0].Subject)
	detail, ok := read.Envelope.Items[0].Detail.(*list.Detail)
	require.True(t, ok)
	assert.Equal(t, "go", detail.Actions[0].Command, "a worktree's actions are commands a surface runs with their answers")
	assert.Empty(t, read.Envelope.Op)
	assert.NoDirExists(t, filepath.Join(main, ".git", "wtm", "ops"))
}

func TestAnInvalidCallIsRefusedWithTheStatusOfItsKind(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	create := command(t, "create")

	for name, c := range map[string]struct {
		Dir     string
		Request string
		Status  int
		Code    string
	}{
		"unknown_field":    {Dir: main, Request: `{"branches": ["feat/x"], "brnach": "x"}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"wrong_field_type": {Dir: main, Request: `{"branches": "feat/x"}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no_branch":        {Dir: main, Request: `{}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no_repository":    {Dir: t.TempDir(), Request: `{"branches": ["feat/x"]}`, Status: http.StatusPreconditionFailed, Code: domain.CodeNotARepository},
	} {
		t.Run(name, func(t *testing.T) {
			got := create.Execute(t.Context(), api.Call{Dir: c.Dir, Request: json.RawMessage(c.Request)})

			assert.Equal(t, c.Status, got.Status)
			require.NotNil(t, got.Envelope.Err)
			assert.Equal(t, c.Code, got.Envelope.Err.Code)
			assert.Equal(t, "create", got.Envelope.Command)
		})
	}
}
