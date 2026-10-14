package dispatch_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
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

func TestAnEntryCarriesTheMessagesAndExitCodesItWasRegisteredWith(t *testing.T) {
	b := bench(t)
	messages := text.Messages{codeBranchTaken: "{branch} is taken"}
	exitCodes := map[kernel.Code]int{codeBranchTaken: 7}

	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: messages, ExitCodes: exitCodes})
	query := dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(b.Dispatcher.Operations), Messages: messages, ExitCodes: exitCodes})

	for _, entry := range []dispatch.Entry{command, query} {
		assert.Equal(t, messages, entry.Messages())
		assert.Equal(t, exitCodes, entry.ExitCodes())
	}
}

func TestTheCatalogueOfARegistrySpeaksTheKernelAndEveryEntry(t *testing.T) {
	b := bench(t)
	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: text.Messages{codeBranchTaken: "{branch} is taken"}})

	catalog := dispatch.Registry{command, operationsQuery(b)}.Catalog()

	assert.Equal(t, "feat/a is taken", catalog.Message(codeBranchTaken, kernel.Params{"branch": "feat/a"}))
	assert.Equal(t, "the facts changed since the preview", catalog.Message(kernel.CodePlanStale, nil))
}

func TestTwoEntriesSpellingOneCodeStopTheStart(t *testing.T) {
	b := bench(t)
	messages := text.Messages{codeBranchTaken: "{branch} is taken"}
	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: messages})
	query := dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(b.Dispatcher.Operations), Messages: messages})

	assert.PanicsWithValue(t, `text: code "branch.taken" has two messages`, func() { dispatch.Registry{command, query}.Catalog() })
}
