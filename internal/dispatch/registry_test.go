package dispatch_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/query/operations"
)

func TestACommandRunsFromItsJSONRequest(t *testing.T) {
	r := &repo{}
	entry, found := registry(t, r).Lookup(commandName)
	require.True(t, found)

	reply, err := entry.Call(t.Context(), dispatch.Call{Request: json.RawMessage(`{"branches":["feat/a"]}`)})

	require.NoError(t, err)
	command, ok := reply.(dispatch.CommandReply)
	require.True(t, ok, "got %T", reply)
	assert.JSONEq(t, `{"branches":["feat/a"],"isolation":"isolated"}`, string(command.Plan.Detail))
	require.NotNil(t, command.Outcome)
	require.Len(t, command.Outcome.Items, 1)
	assert.Equal(t, kernel.StatusDone, command.Outcome.Items[0].Status)
	assert.JSONEq(t, `"/wt/feat/a"`, string(command.Outcome.Items[0].Detail))
	assert.Equal(t, []string{"feat/a"}, r.listed())
}

func TestAFieldTheRequestDoesNotHaveIsRefusedByName(t *testing.T) {
	r := &repo{}
	entry, _ := registry(t, r).Lookup(commandName)

	_, err := entry.Call(t.Context(), dispatch.Call{Request: json.RawMessage(`{"branchs":["feat/a"]}`)})

	var invalid *kernel.InvalidError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, []kernel.FieldError{{Path: "branchs", Code: kernel.CodeUnknownField}}, invalid.Fields)
	assert.Empty(t, r.listed())
}

func TestAValueOfTheWrongShapeIsRefusedOnItsField(t *testing.T) {
	entry, _ := registry(t, &repo{}).Lookup(commandName)

	_, err := entry.Call(t.Context(), dispatch.Call{Request: json.RawMessage(`{"branches":"feat/a"}`)})

	var invalid *kernel.InvalidError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, []kernel.FieldError{{Path: "branches", Code: kernel.CodeInvalid}}, invalid.Fields)
}

func TestAQueryAnswersWithItsView(t *testing.T) {
	entry, found := registry(t, &repo{}).Lookup(operations.Name)
	require.True(t, found)

	reply, err := entry.Call(t.Context(), dispatch.Call{})

	require.NoError(t, err)
	assert.Equal(t, dispatch.QueryReply{View: json.RawMessage(`{"operations":[]}`)}, reply)
	assert.Nil(t, entry.Schema())
}

func TestANameTheRegistryDoesNotHoldIsNotFound(t *testing.T) {
	_, found := registry(t, &repo{}).Lookup("nowhere")
	assert.False(t, found)
}

func TestTheRegistryServesTheQueryOfTheOperationsInProgress(t *testing.T) {
	registry := bench(t).Dispatcher.Registry()

	entry, found := registry.Lookup(operations.Name)

	require.True(t, found)
	reply, err := entry.Call(t.Context(), dispatch.Call{})
	require.NoError(t, err)
	view, ok := reply.(dispatch.QueryReply)
	require.True(t, ok, "got %T", reply)
	assert.JSONEq(t, `{"operations":[]}`, string(view.View))
}
