package dispatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/query/operations"
)

func TestANameTheRegistryDoesNotHoldIsNotFound(t *testing.T) {
	_, found := registry(t, &repo{}).Lookup("nowhere")
	assert.False(t, found)
}

func TestAnEntryCarriesTheMessagesAndExitCodesItWasRegisteredWith(t *testing.T) {
	b := bench(t)
	messages := text.Messages{codeBranchTaken: "{branch} is taken"}
	exitCodes := map[kernel.Code]int{codeBranchTaken: 7}

	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, detail]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: messages, ExitCodes: exitCodes})
	query := dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(b.Dispatcher.Operations), Messages: messages, ExitCodes: exitCodes})

	for _, entry := range []dispatch.Entry{command, query} {
		assert.Equal(t, messages, entry.Messages())
		assert.Equal(t, exitCodes, entry.ExitCodes())
	}
}

func TestTheCatalogueOfARegistrySpeaksTheKernelAndEveryEntry(t *testing.T) {
	b := bench(t)
	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, detail]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: text.Messages{codeBranchTaken: "{branch} is taken"}})

	catalog := dispatch.Registry{command, operationsQuery(b)}.Catalog()

	assert.Equal(t, "feat/a is taken", catalog.Message(codeBranchTaken, kernel.Params{"branch": "feat/a"}))
	assert.Equal(t, "the facts changed since the preview", catalog.Message(kernel.CodePlanStale, nil))
}

func TestTwoEntriesSpellingOneCodeStopTheStart(t *testing.T) {
	b := bench(t)
	messages := text.Messages{codeBranchTaken: "{branch} is taken"}
	command := dispatch.Register(dispatch.RegisterParams[request, facts, plan, detail]{Dispatcher: b.Dispatcher, Command: createCommand(&repo{}), Messages: messages})
	query := dispatch.RegisterQuery(dispatch.RegisterQueryParams[operations.Request, operations.View]{Query: operations.Query(b.Dispatcher.Operations), Messages: messages})

	assert.PanicsWithValue(t, `text: code "branch.taken" has two messages`, func() { dispatch.Registry{command, query}.Catalog() })
}
