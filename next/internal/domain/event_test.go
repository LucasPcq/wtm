package domain

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryEventComesBackAsItWasEncoded(t *testing.T) {
	identity := worktree.Identity{Path: "/trees/feat-x", Branch: "feat/x"}
	for _, event := range []Event{
		UnitStarted{Subject: "feat/x"},
		PhaseStarted{Subject: "feat/x", Phase: "on_create", Log: "/logs/on_create.log"},
		PhaseFinished{Subject: "feat/x", Phase: "on_create", Failed: true},
		Output{Subject: "feat/x", Line: "set up <feat/x>"},
		UnitFinished{Subject: "feat/x", Status: StatusDone},
		WorktreeCreated{Worktree: identity},
		WorktreeNumbered{Worktree: identity},
		WorktreeProvisioned{Worktree: identity, Hook: "make setup", ExitCode: new(3)},
		OperationStarted{Command: "create", Surface: "cli"},
		OperationFinished{Command: "create", Surface: "cli", Code: "unit.phase_failed"},
	} {
		t.Run(EventType(event), func(t *testing.T) {
			data, err := MarshalEvent(event)
			require.NoError(t, err)

			decoded, err := UnmarshalEvent(data)

			require.NoError(t, err)
			assert.Equal(t, event, decoded)
			assert.Regexp(t, `^\{"type":"`+EventType(event)+`",`, string(data))
		})
	}
}

func TestAnUnknownEventTypeIsRefused(t *testing.T) {
	_, err := UnmarshalEvent([]byte(`{"type": "worktree.updated"}`))

	assert.EqualError(t, err, `unknown event type "worktree.updated"`)
}
