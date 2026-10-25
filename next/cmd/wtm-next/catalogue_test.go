package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
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

	assert.Equal(t, []string{"create", "extract", "clean", "list", "resolve", "status", "tree"}, names)
}

func TestAnInvalidCallIsRefusedWithItsKind(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	create := command(t, "create")

	for name, c := range map[string]struct {
		Dir     string
		Answers []domain.Answered
		Kind    domain.Kind
		Code    string
	}{
		"unknown_field":     {Dir: main, Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}, {Path: "brnach", Values: []string{"x"}}}, Kind: domain.KindInvalid, Code: domain.CodeInvalidRequest},
		"two_values_of_one": {Dir: main, Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}, {Path: "from", Values: []string{"main", "dev"}}}, Kind: domain.KindInvalid, Code: domain.CodeInvalidRequest},
		"no_branch":         {Dir: main, Kind: domain.KindInvalid, Code: domain.CodeInvalidRequest},
		"no_repository":     {Dir: t.TempDir(), Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}}, Kind: domain.KindPrecondition, Code: domain.CodeNotARepository},
	} {
		t.Run(name, func(t *testing.T) {
			got := create.Execute(t.Context(), api.Call{Dir: c.Dir, Answers: c.Answers})

			require.NotNil(t, got.Envelope.Err)
			assert.Equal(t, c.Kind, got.Envelope.Err.Kind)
			assert.Equal(t, c.Code, got.Envelope.Err.Code)
			assert.Equal(t, "create", got.Envelope.Command)
			if c.Dir == main {
				shapes{t: t, command: "create", root: filepath.Dir(main)}.golden("invalid_"+name, got.Envelope)
			}
		})
	}
}

// A surface's status read the main worktree whatever its call's directory: the worktree the caller stood in was set by the command line alone.
func TestStatusOfASurfaceReadsTheWorktreeOfTheCallsDirectory(t *testing.T) {
	main := configured(t)
	ownConfigDir(t)
	linked := filepath.Join(filepath.Dir(main), ".trees", "feat-a")
	gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/a", linked)
	// git lists a worktree by its real path: a call through a link stands in it all the same
	link := filepath.Join(t.TempDir(), "feat-a")
	require.NoError(t, os.Symlink(linked, link))

	for name, dir := range map[string]string{"direct": linked, "symlinked": link} {
		t.Run(name, func(t *testing.T) {
			got := command(t, "status").Execute(t.Context(), api.Call{Dir: dir})

			assert.Nil(t, got.Envelope.Err)
			require.Len(t, got.Envelope.Items, 1)
			assert.Equal(t, "feat/a", got.Envelope.Items[0].Subject)
		})
	}
}
