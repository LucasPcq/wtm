package oplock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnOperationHoldsItsLocksAndRecordsItselfWhereWtmLooks(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}

	held, err := locks.Take(op("op-1", repoShared, featA))

	require.NoError(t, err)
	assert.True(t, held.Protected())
	assert.FileExists(t, filepath.Join(locks.StateDir, "locks", "repo.lock"))
	assert.FileExists(t, filepath.Join(locks.StateDir, "locks", "wt", "feat%2Fa.lock"))
	var recorded domain.Operation
	require.NoError(t, json.Unmarshal(read(t, filepath.Join(locks.StateDir, "locks", "ops", "op-1.json")), &recorded))
	assert.Equal(t, op("op-1", repoShared, featA), recorded)
	held.Release()
	assert.NoFileExists(t, filepath.Join(locks.StateDir, "locks", "ops", "op-1.json"))
}

func TestAWorktreeHeldByAnotherOperationIsAConflictNamingItsHolder(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}
	take(t, func() (*Held, error) { return locks.Take(op("op-1", repoShared, featA)) })

	_, err := locks.Take(op("op-2", repoShared, featB, featA))

	failure := refusal(t, err)
	assert.Equal(t, domain.KindConflict, failure.Kind)
	assert.Equal(t, "lock.held", failure.Code)
	assert.Equal(t, "wt:feat/a is held by create (pid 4242)", failure.Message)
	assert.Equal(t, domain.Params{"key": "wt:feat/a", "op": "op-1", "command": "create", "pid": "4242", "surface": "tui", "started_at": "2026-10-16T12:00:00Z"}, failure.Params)
	again, err := locks.Take(op("op-3", repoShared, featB))
	require.NoError(t, err, "the refused operation kept feat/b")
	again.Release()
}

func TestALockHeldWithoutARecordNamesAnUnknownHolder(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}
	file, err := os.OpenFile(filepath.Join(mkdir(t, filepath.Join(locks.StateDir, "locks")), "repo.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, flockExclusive(file))

	_, takeErr := locks.Take(op("op-1", repoShared))

	assert.Equal(t, domain.Params{"key": "repo", "op": "unknown", "command": "unknown", "pid": "unknown"}, refusal(t, takeErr).Params)
	assert.NoFileExists(t, filepath.Join(locks.StateDir, "locks", "ops", "op-1.json"))
}

func TestGrowingTakesTheLocksMissingAndRecordsThemAll(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}
	held := take(t, func() (*Held, error) { return locks.Take(op("op-1", repoShared, featA)) })

	grown, err := held.Grow([]domain.Lock{repoShared, featA, featB})
	unchanged, sameErr := held.Grow([]domain.Lock{repoShared, featA, featB})

	require.NoError(t, err)
	require.NoError(t, sameErr)
	assert.True(t, grown)
	assert.False(t, unchanged)
	var recorded domain.Operation
	require.NoError(t, json.Unmarshal(read(t, filepath.Join(locks.StateDir, "locks", "ops", "op-1.json")), &recorded))
	assert.Equal(t, []domain.Lock{repoShared, featA, featB}, recorded.Locks)
	_, busy := locks.Take(op("op-2", featB))
	assert.Equal(t, "lock.held", refusal(t, busy).Code)
}

func TestAStateDirThatRefusesLocksRunsUnprotected(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(stateDir, nil, 0o644))

	held, err := Locks{StateDir: stateDir}.Take(op("op-1", repoShared, featA))

	require.NoError(t, err)
	assert.False(t, held.Protected())
	grown, err := held.Grow([]domain.Lock{repoShared, featA, featB})
	require.NoError(t, err)
	assert.False(t, grown)
	held.Release()
}

func TestTheRecordOfADeadOperationIsNeverItsHolder(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}
	writeFile(t, filepath.Join(locks.StateDir, "locks", "ops", "op-dead.json"), `{"op_id":"op-dead","pid":1,"command":"create","locks":[{"scope":"repo","mode":"exclusive"}]}`)
	file, err := os.OpenFile(filepath.Join(locks.StateDir, "locks", "repo.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, flockExclusive(file))

	_, takeErr := locks.Take(op("op-1", repoShared))

	assert.Equal(t, "unknown", refusal(t, takeErr).Params["op"])
}

func TestGrowingKeepsWhatIsHeldWhenTheDefaultsMove(t *testing.T) {
	locks := Locks{StateDir: t.TempDir()}
	held := take(t, func() (*Held, error) { return locks.Take(op("op-1", repoShared, featA)) })

	moved, movedErr := held.Grow([]domain.Lock{repoShared, featB})
	back, backErr := held.Grow([]domain.Lock{repoShared, featA})

	require.NoError(t, movedErr)
	require.NoError(t, backErr)
	assert.True(t, moved)
	assert.False(t, back)
	var recorded domain.Operation
	require.NoError(t, json.Unmarshal(read(t, filepath.Join(locks.StateDir, "locks", "ops", "op-1.json")), &recorded))
	assert.Equal(t, []domain.Lock{repoShared, featA, featB}, recorded.Locks)
}
