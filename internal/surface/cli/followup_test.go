package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
)

func TestAFollowUpIsTheCommandLineThatRunsItWithItsReason(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.root()
	b := w.Engine.bindings[commandName]

	steps := b.followUps([]kernel.FollowUp{{
		Command: commandName,
		Request: json.RawMessage(`{"branches":["feat/a","feat/b"],"fast_forward":true,"env":{"PORT":"3000"},"target":{"base":"main"},"isolation":"verbatim"}`),
		Code:    kernel.CodeLocksUnavailable,
	}})

	assert.Len(t, steps, 1)
	assert.Equal(t, "wtm make feat/a feat/b --env PORT=3000 --ff --isolation verbatim --target-base main", steps[0].Command)
	assert.Equal(t, text.Message(kernel.CodeLocksUnavailable, nil), steps[0].Note)
}

func TestAFollowUpForACommandNotServedHereIsSpelledInKebabCase(t *testing.T) {
	w := newWorld(t, worldParams{})

	line := w.Engine.commandLine(lineParams{Target: "clean", Request: json.RawMessage(`{"branches":["feat/a"],"keep_branch":false}`)})

	assert.Equal(t, "wtm clean --branches feat/a --keep-branch=false", line)
}

func TestAWordAShellWouldReadIsQuoted(t *testing.T) {
	assert.Equal(t, "feat/a", quote("feat/a"))
	assert.Equal(t, "'two words'", quote("two words"))
	assert.Equal(t, `'it'\''s'`, quote("it's"))
	assert.Equal(t, "''", quote(""))
}

func TestARerunRepeatsTheRequestWithOneEngineFlag(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.root()
	b := w.Engine.bindings[commandName]

	line := b.rerun(rerunParams{Request: json.RawMessage(`{"branches":["feat/a"]}`), Flag: flagExpectFingerprint, Value: "abc"})

	assert.Equal(t, "wtm make feat/a --expect-fingerprint abc", line)
}

func TestAFailureCarryingAFollowUpPointsAtIt(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.root()
	b := w.Engine.bindings[commandName]
	failure := kernel.Conflict(kernel.Problem{Code: kernel.CodeLockHeld, FollowUp: &kernel.FollowUp{Command: commandName, Request: json.RawMessage(`{"branches":["feat/a"]}`), Code: kernel.CodeInterrupted}})

	steps := b.failureNext(failParams{Err: failure})

	assert.Equal(t, "wtm make feat/a", steps[0].Command)
}
