package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryEventComesBackAsItWasEncoded(t *testing.T) {
	for _, event := range everyEvent {
		t.Run(EventType(event), func(t *testing.T) {
			data, err := MarshalEvent(event)
			require.NoError(t, err)

			decoded, err := UnmarshalEvent(data)

			require.NoError(t, err)
			assert.Equal(t, event, decoded)
			assert.Regexp(t, `^\{"type":"`+EventType(event)+`"[,}]`, string(data))
		})
	}
}

func TestEachTypeOfEventHasItsDecoder(t *testing.T) {
	types := map[string]bool{}
	for _, event := range everyEvent {
		types[EventType(event)] = true
	}

	assert.Len(t, types, len(everyEvent), "two events share a type")
	for name := range eventDecoders {
		assert.True(t, types[name], "no event of type %s is checked", name)
	}
	assert.Len(t, eventDecoders, len(types), "an event type has no decoder")
}

func TestAnUnknownEventTypeIsRefused(t *testing.T) {
	_, err := UnmarshalEvent([]byte(`{"type": "worktree.numbered"}`))

	require.ErrorIs(t, err, ErrUnknownEventType)
	assert.EqualError(t, err, `unknown event type "worktree.numbered"`)
}

func TestASnapshotAlwaysCarriesItsWorktreesAsAnArray(t *testing.T) {
	data, err := MarshalEvent(Snapshot{})

	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"snapshot","worktrees":[]}`, string(data))
}
