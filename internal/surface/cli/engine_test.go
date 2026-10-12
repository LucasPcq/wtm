package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTheProductionEngineRunsOnTheProjectItIsPointedAt(t *testing.T) {
	project, state := t.TempDir(), t.TempDir()
	t.Setenv(domain.EnvProjectDir, project)
	t.Setenv(domain.EnvStateDir, state)

	dispatcher, err := production.dispatcher(t.Context())

	require.NoError(t, err)
	entry, found := production.registry(dispatcher).Lookup("operations")
	require.True(t, found)
	reply, err := entry.Call(t.Context(), dispatch.Call{})
	require.NoError(t, err)
	view, ok := reply.(dispatch.QueryReply)
	require.True(t, ok, "got %T", reply)
	assert.JSONEq(t, `{"operations":[]}`, string(view.View))
}

func TestTheSchemaIsReadWithNoRepository(t *testing.T) {
	_, found := production.schema().Lookup("operations")

	assert.True(t, found)
}
