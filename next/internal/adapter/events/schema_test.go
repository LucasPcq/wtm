package events

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALineOfEachTypeTheStreamCarriesMatchesTheSchema(t *testing.T) {
	schema := schemaV2(t)

	for _, event := range carried() {
		t.Run(domain.EventType(event), func(t *testing.T) {
			assert.NoError(t, validates(t, schema, lineOf(t, event)))
		})
	}
}

func TestEveryLineThePublisherWritesMatchesTheSchema(t *testing.T) {
	schema, daemon := schemaV2(t), listen(t)
	publisher := publisherOn(t, daemon)

	for _, event := range carried() {
		if !published(event) {
			continue
		}
		publisher.Publish(event)

		assert.NoError(t, validates(t, schema, payloadOf(t, daemon.next(t))), domain.EventType(event))
	}
}

// Legacy v1 listed its types in a oneOf: a type it did not know failed the whole line.
func TestALineOfATypeTheSchemaDoesNotListIsValidAgainstTheEnvelope(t *testing.T) {
	assert.NoError(t, validates(t, schemaV2(t), `{"v":2,"type":"job.restarted","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"job":{"name":"web"}}`))
}

func TestTheSchemaRefusesWhatTheContractForbids(t *testing.T) {
	schema := schemaV2(t)

	for name, line := range map[string]string{
		"version_one":      `{"v":1,"type":"ready","ts":"2026-10-16T12:00:00Z"}`,
		"no_type":          `{"v":2,"ts":"2026-10-16T12:00:00Z"}`,
		"snapshot_no_list": `{"v":2,"type":"snapshot","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"}}`,
		"updated_nothing":  `{"v":2,"type":"worktree.updated","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktree":{"branch":"a","path":"/a","parent":"","ordinal":null,"isolation":"isolated","is_main":false,"created_at":""},"changed":[]}`,
		"unknown_change":   `{"v":2,"type":"worktree.updated","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktree":{"branch":"a","path":"/a","parent":"","ordinal":null,"isolation":"isolated","is_main":false,"created_at":""},"changed":["path"]}`,
		"changed_nothing":  `{"v":2,"type":"worktree.changed","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktree":{"branch":"a","path":"/a","parent":"","ordinal":null,"isolation":"isolated","is_main":false,"created_at":""},"changed":[]}`,
		"created_no_repo":  `{"v":2,"type":"worktree.created","ts":"2026-10-16T12:00:00Z","worktree":{"branch":"a","path":"/a","parent":"","ordinal":null,"isolation":"isolated","is_main":false,"created_at":""}}`,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, validates(t, schema, line))
		})
	}
}

func TestTheSchemasPhasesSurfacesAndHoldsAreTheCoresEveryOne(t *testing.T) {
	data, err := os.ReadFile("events.v2.json")
	require.NoError(t, err)
	var doc struct {
		Defs struct {
			Phase struct {
				Enum []domain.Phase `json:"enum"`
			} `json:"phase"`
			Surface struct {
				Enum []domain.Surface `json:"enum"`
			} `json:"surface"`
			Job struct {
				Properties struct {
					Hold struct {
						Enum []run.Hold `json:"enum"`
					} `json:"hold"`
				} `json:"properties"`
			} `json:"job"`
		} `json:"$defs"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))

	assert.Equal(t, domain.Phases(), doc.Defs.Phase.Enum)
	assert.Equal(t, domain.Surfaces(), doc.Defs.Surface.Enum)
	assert.Equal(t, run.Holds(), doc.Defs.Job.Properties.Hold.Enum)
}
