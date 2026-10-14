package dispatch_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
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
	assert.JSONEq(t, `{"path":"/wt/feat/a","ordinal":1}`, string(command.Outcome.Items[0].Detail))
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
