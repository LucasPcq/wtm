package runenv_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/runenv"
	"github.com/LucasPcq/wtm/internal/testutil/storetest"
)

func TestApplyWritesThePortsAndTheKeysWtmOwnsAndSaysWhatChanged(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{Files: map[string]string{worktree + "/.env": "# web\nPORT=3000\nDEBUG=1\n"}})

	applied, err := runenv.Apply(t.Context(), runenv.ApplyParams{Files: files, Writes: writes(t, linked())})

	require.NoError(t, err)
	assert.Equal(t, "# web\nPORT=3020\nDEBUG=1\n", file(t, files, worktree+"/.env"))
	assert.Equal(t, "COMPOSE_PROJECT_NAME=shop-feat-a", file(t, files, worktree+"/infra/.env"))
	assert.Equal(t, []string{".env", "infra/.env"}, applied.Files)
	assert.True(t, applied.Plan.Applied)
	require.Len(t, applied.Plan.Entries, 1)
	assert.Equal(t, domain.EnvPortStatusRewrite, applied.Plan.Entries[0].Status)
	assert.Equal(t, []domain.EnvOwnedEntry{{File: "infra/.env", Key: domain.EnvComposeProjectName, Value: "shop-feat-a", Changed: true}}, applied.Plan.Owned)
	assert.False(t, applied.Unlocked)
}

func TestApplyPlansFromTheLinesReadUnderTheLockNotFromAnEarlierRead(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{Files: map[string]string{worktree + "/.env": "PORT=3000\n"}})
	resolved := writes(t, linked())
	_, err := files.Edit(t.Context(), editTo(worktree+"/.env", "PORT=3020\nADDED=since\n"))
	require.NoError(t, err)

	applied, err := runenv.Apply(t.Context(), runenv.ApplyParams{Files: files, Writes: resolved})

	require.NoError(t, err)
	assert.Equal(t, "PORT=3020\nADDED=since\n", file(t, files, worktree+"/.env"))
	assert.Equal(t, []string{"infra/.env"}, applied.Files)
	assert.Equal(t, domain.EnvPortStatusUnchanged, applied.Plan.Entries[0].Status)
}

func TestApplyTwiceChangesNothingTheSecondTime(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{Files: map[string]string{worktree + "/.env": "PORT=3000\n"}})
	params := runenv.ApplyParams{Files: files, Writes: writes(t, linked())}
	_, err := runenv.Apply(t.Context(), params)
	require.NoError(t, err)

	applied, err := runenv.Apply(t.Context(), params)

	require.NoError(t, err)
	assert.Empty(t, applied.Files)
	assert.False(t, applied.Plan.Owned[0].Changed)
}

func TestApplyLeavesAnAbsentLinkedFileAbsentAndReportsItsKeyMissing(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{})

	applied, err := runenv.Apply(t.Context(), runenv.ApplyParams{Files: files, Writes: writes(t, linked())})

	require.NoError(t, err)
	_, created := files.File(worktree + "/.env")
	assert.False(t, created)
	assert.Equal(t, domain.EnvPortStatusMissingKey, applied.Plan.Entries[0].Status)
}

func TestApplyWritesNothingForAVerbatimWorktree(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{})
	params := linked()
	params.Identity.Isolation = domain.IsolationVerbatim

	applied, err := runenv.Apply(t.Context(), runenv.ApplyParams{Files: files, Writes: writes(t, params)})

	require.NoError(t, err)
	assert.Equal(t, runenv.Applied{}, applied)
	assert.Empty(t, files.Calls())
}

func TestApplyReturnsAWriteThatFails(t *testing.T) {
	files := storetest.NewFake(storetest.FakeParams{Fail: map[string]error{"Edit": errors.New("disk full")}})

	_, err := runenv.Apply(t.Context(), runenv.ApplyParams{Files: files, Writes: writes(t, linked())})

	require.Error(t, err)
}
