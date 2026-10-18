package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheCatalogueListsEachCommandWithItsSchemaOutsideAnyRepository(t *testing.T) {
	var names []string
	for _, command := range catalogue() {
		names = append(names, command.Name)
		assert.NotEmpty(t, command.Form.Fields, command.Name)
	}

	assert.Equal(t, []string{"create", "extract"}, names)
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
		"an unknown field":        {Dir: main, Request: `{"branches": ["feat/x"], "brnach": "x"}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"a field of another type": {Dir: main, Request: `{"branches": "feat/x"}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no branch":               {Dir: main, Request: `{}`, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no repository":           {Dir: t.TempDir(), Request: `{"branches": ["feat/x"]}`, Status: http.StatusPreconditionFailed, Code: domain.CodeNotARepository},
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
