package wtm

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
