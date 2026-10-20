package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorded publishes nothing and records what the publisher of each operation would stamp on its events.
type recorded struct {
	wireParams
	published *[]events.PublisherParams
}

func (r recorded) Open(op domain.Operation) domain.Report {
	*r.published = append(*r.published, r.publisherParams(op))
	return func(domain.Event) {}
}

// A mutation is a command of the catalogue the contract drives through api alone, on a repository configured for wtm: the answers that change it, its subject's first, what it needs first, what makes its preview stale, what a refused apply leaves as it was, and what its apply leaves.
type mutation struct {
	name      string
	answers   []domain.Answered
	setup     func(t *testing.T, main string)
	stale     func(t *testing.T, main string)
	untouched func(t *testing.T, main string)
	applied   func(t *testing.T, main string)
}

// mutations are the commands of the catalogue that change a repository.
func mutations() []mutation {
	return []mutation{
		{
			name:    "create",
			answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}},
			// the env strategy settles where the new worktree's .env comes from: a plan made before no longer holds
			stale: func(t *testing.T, main string) {
				writeConfig(t, main, "[env]\nstrategy = \"main\"\n")
			},
			untouched: func(t *testing.T, main string) {
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
			},
			applied: func(t *testing.T, main string) {
				assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
			},
		},
		{
			name:    "extract",
			answers: []domain.Answered{{Path: "source", Values: []string{"main"}}, {Path: "files", Values: []string{"notes.txt"}}, {Path: "to", Values: []string{"feat/notes"}}},
			setup: func(t *testing.T, main string) {
				require.NoError(t, os.WriteFile(filepath.Join(main, "notes.txt"), []byte("notes\n"), 0o644))
			},
			// staging the file changes its status, which the plan moves by
			stale: func(t *testing.T, main string) {
				gittest.Git(t, main, "add", "notes.txt")
			},
			untouched: func(t *testing.T, main string) {
				assert.FileExists(t, filepath.Join(main, "notes.txt"))
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-notes"))
			},
			applied: func(t *testing.T, main string) {
				assert.FileExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-notes", "notes.txt"))
				assert.NoFileExists(t, filepath.Join(main, "notes.txt"))
			},
		},
	}
}

// reads are the commands of the catalogue that change nothing.
var reads = []string{"list", "status", "tree"}
