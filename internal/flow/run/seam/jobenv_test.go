package seam_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestTheFirstRunNumbersItsWorktreeAndPublishesIt(t *testing.T) {
	dir := gittest.InitRepo(t)
	path := filepath.Join(t.TempDir(), "feat-a")
	gittest.Git(t, dir, "worktree", "add", "-b", "feat/a", path)
	rec := &flowtest.Recorder{}
	params := seam.JobEnvParams{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), WorkDir: path, Publisher: rec}

	env, err := seam.JobEnv(params)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := seam.JobEnv(params); err != nil {
		t.Fatal(err)
	}

	if env[domain.EnvOrdinal] != "1" {
		t.Fatalf("%s = %q, want 1", domain.EnvOrdinal, env[domain.EnvOrdinal])
	}
	if !slices.Equal(rec.PublishedTypes(), []domain.EventType{domain.EventWorktreeUpdated}) {
		t.Fatalf("published %v, want one ordinal update", rec.PublishedTypes())
	}
}
