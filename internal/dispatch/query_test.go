package dispatch_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/query/operations"
)

func TestAQueryAnswersWithItsView(t *testing.T) {
	entry, found := registry(t, &repo{}).Lookup(operations.Name)
	require.True(t, found)

	reply, err := entry.Call(t.Context(), dispatch.Call{})

	require.NoError(t, err)
	assert.Equal(t, dispatch.QueryReply{View: json.RawMessage(`{"operations":[]}`)}, reply)
	assert.Nil(t, entry.Schema())
}
