package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALockIsNamedByItsScopeAndItsWorktree(t *testing.T) {
	assert.Equal(t, "repo", Lock{Scope: ScopeRepo, Mode: LockShared}.Key())
	assert.Equal(t, "wt:feat/a", Lock{Scope: ScopeWorktree, Name: "feat/a", Mode: LockExclusive}.Key())
}

func TestAnOperationIsRecordedAsWtmRecordsIt(t *testing.T) {
	op := Operation{ID: "op-1", PID: 7, Command: "create", Surface: "cli", StartedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC), Locks: []Lock{{Scope: ScopeRepo, Mode: LockShared}, {Scope: ScopeWorktree, Name: "feat/a", Mode: LockExclusive}}}

	data, err := json.Marshal(op)

	require.NoError(t, err)
	assert.JSONEq(t, `{"op_id":"op-1","pid":7,"command":"create","surface":"cli","started_at":"2026-10-16T12:00:00Z","locks":[{"scope":"repo","mode":"shared"},{"scope":"wt","name":"feat/a","mode":"exclusive"}]}`, string(data))
}

func TestACorrelationIDIsShortAndPrintable(t *testing.T) {
	assert.NoError(t, CorrelationProblem("ci-run-42"))
	assert.EqualError(t, CorrelationProblem("bad\x01id"), "contains a control character")
	assert.EqualError(t, CorrelationProblem(strings.Repeat("x", 257)), "is longer than 256 bytes")
}

func TestWorktreeLocksHoldTheRepositoryAndEachWorktreeOnceInOrder(t *testing.T) {
	cases := map[string]struct {
		worktrees []string
		want      []Lock
	}{
		"none":                 {nil, []Lock{{Scope: ScopeRepo, Mode: LockShared}}},
		"an empty name":        {[]string{""}, []Lock{{Scope: ScopeRepo, Mode: LockShared}}},
		"sorted and compacted": {[]string{"b", "", "a", "b"}, []Lock{{Scope: ScopeRepo, Mode: LockShared}, {Scope: ScopeWorktree, Name: "a", Mode: LockExclusive}, {Scope: ScopeWorktree, Name: "b", Mode: LockExclusive}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, WorktreeLocks(c.worktrees))
		})
	}
}

func TestWorktreeLocksLeaveTheirArgumentAsItWas(t *testing.T) {
	worktrees := []string{"b", "", "a"}

	WorktreeLocks(worktrees)

	assert.Equal(t, []string{"b", "", "a"}, worktrees)
}
