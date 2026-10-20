package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALineIsItsEnvelopeFollowedByItsEventsFields(t *testing.T) {
	line := Line{V: StreamVersion, TS: "2026-10-16T12:00:00Z", Repo: &Repo{Root: "/repo", CommonDir: "/repo/.git"}, CorrelationID: "popup-1", OpID: "op-1", Event: WorktreeCreated{Worktree: identity}}

	data, err := MarshalLine(line)

	require.NoError(t, err)
	assert.Equal(t, `{"v":2,"type":"worktree.created","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"correlation_id":"popup-1","op_id":"op-1","worktree":{"branch":"feat/x","path":"/trees/feat-x","parent":"","ordinal":null,"isolation":"","is_main":false,"created_at":""}}`, string(data))
}

func TestEveryLineComesBackAsItWasEncoded(t *testing.T) {
	for _, event := range everyEvent {
		t.Run(EventType(event), func(t *testing.T) {
			line := Line{V: StreamVersion, TS: "2026-10-16T12:00:00Z", Repo: &Repo{Root: "/repo", CommonDir: "/repo/.git"}, OpID: "op-1", Event: event}
			data, err := MarshalLine(line)
			require.NoError(t, err)

			decoded, err := UnmarshalLine(data)

			require.NoError(t, err)
			assert.Equal(t, line, decoded)
		})
	}
}

func TestReadyCarriesItsEnvelopeAlone(t *testing.T) {
	data, err := MarshalLine(Line{V: StreamVersion, TS: "2026-10-16T12:00:00Z", Event: Ready{}})

	require.NoError(t, err)
	assert.Equal(t, `{"v":2,"type":"ready","ts":"2026-10-16T12:00:00Z"}`, string(data))
}

func TestALineOfAnUnknownTypeKeepsItsEnvelope(t *testing.T) {
	line, err := UnmarshalLine([]byte(`{"v":2,"type":"job.started","ts":"t","repo":{"root":"/repo","common_dir":"/repo/.git"},"job":{"name":"web"}}`))

	require.ErrorIs(t, err, ErrUnknownEventType)
	assert.Equal(t, Line{V: 2, TS: "t", Repo: &Repo{Root: "/repo", CommonDir: "/repo/.git"}}, line)
}

func TestALineOfANewerVersionIsDecodedNoFurtherThanItsVersion(t *testing.T) {
	line, err := UnmarshalLine([]byte(`{"v":3,"type":"worktree.created","worktree":"a new shape"}`))

	require.NoError(t, err)
	assert.Equal(t, 3, line.V)
	assert.Nil(t, line.Event)
}
