package dispatch_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestTheSameFactsGiveTheSameFingerprint(t *testing.T) {
	h := handle(bench(t), &repo{existing: []string{"feat/old"}})
	first, err := h.Observe(t.Context())
	require.NoError(t, err)
	again, err := h.Observe(t.Context())
	require.NoError(t, err)

	assert.Equal(t, first.Fingerprint, again.Fingerprint)
	assert.Equal(t, facts{Existing: []string{"feat/old"}}, first.Facts)
}

func TestOtherFactsGiveAnotherFingerprint(t *testing.T) {
	r := &repo{}
	h := handle(bench(t), r)
	before, err := h.Observe(t.Context())
	require.NoError(t, err)
	r.existing = []string{"feat/new"}
	after, err := h.Observe(t.Context())
	require.NoError(t, err)

	assert.NotEqual(t, before.Fingerprint, after.Fingerprint)
}

func TestTheStateOfAFormFillsInTheDefaults(t *testing.T) {
	form, err := handle(bench(t), &repo{}).State(dispatch.FormParams[request, facts]{Request: request{Branches: []string{"feat/a"}}})

	require.NoError(t, err)
	assert.True(t, form.Complete)
	assert.Equal(t, isolated, form.Request.Isolation)
	assert.Equal(t, kernel.FieldDefaulted, form.States[2].Status)
}

func TestAnOptionThatWouldBreakARuleArrivesWithItsRefusal(t *testing.T) {
	h := handle(bench(t), &repo{})

	choices, err := h.Choices(t.Context(), dispatch.ChoicesParams[request, facts]{
		Path:    "from",
		Request: request{Branches: []string{"feat/a"}},
		Facts:   facts{Existing: []string{"feat/a"}},
	})

	require.NoError(t, err)
	require.Len(t, choices.Options, 2)
	assert.Nil(t, choices.Options[0].Refusal, "main breaks no rule")
	require.NotNil(t, choices.Options[1].Refusal)
	assert.Equal(t, kernel.CodeSelfParent, choices.Options[1].Refusal.Code)
}

func TestAFieldWithoutChoicesHasNone(t *testing.T) {
	choices, err := handle(bench(t), &repo{}).Choices(t.Context(), dispatch.ChoicesParams[request, facts]{Path: "branches"})

	require.NoError(t, err)
	assert.Empty(t, choices.Options)
}

func TestTheChoicesOfAPathThatIsNoFieldAreNotFound(t *testing.T) {
	_, err := handle(bench(t), &repo{}).Choices(t.Context(), dispatch.ChoicesParams[request, facts]{Path: "nowhere"})

	failure := problem(t, err)
	assert.Equal(t, kernel.KindNotFound, failure.Kind())
	assert.Equal(t, kernel.CodeUnknownField, failure.Base().Code)
	assert.Equal(t, kernel.Params{kernel.ParamPath: "nowhere"}, failure.Base().Params, "the path is no value that was not found")
}

func TestABoolOptionIsJudgedAsTheBoolItSets(t *testing.T) {
	h := dispatch.Register(dispatch.RegisterParams[toggles, struct{}, plan, detail]{Dispatcher: bench(t).Dispatcher, Command: togglesCommand()})

	choices, err := h.Choices(t.Context(), dispatch.ChoicesParams[toggles, struct{}]{Path: "push", Request: toggles{NoPush: true}})

	require.NoError(t, err)
	require.Len(t, choices.Options, 2)
	require.NotNil(t, choices.Options[0].Refusal, "pushing would break the rule no_push holds")
	assert.Equal(t, kernel.CodeExclusiveWith, choices.Options[0].Refusal.Code)
	assert.Nil(t, choices.Options[1].Refusal)
}

func TestTheOptionsOfADecisionsFieldAreNotJudgedAlone(t *testing.T) {
	h := dispatch.Register(dispatch.RegisterParams[toggles, struct{}, plan, detail]{Dispatcher: bench(t).Dispatcher, Command: togglesCommand()})

	choices, err := h.Choices(t.Context(), dispatch.ChoicesParams[toggles, struct{}]{Path: "decisions", Request: toggles{}})

	require.NoError(t, err)
	assert.Len(t, choices.Options, 2)
}

func TestFactsThatCannotBeFingerprintedAreAnInternalError(t *testing.T) {
	h := dispatch.Register(dispatch.RegisterParams[request, unencodable, plan, detail]{
		Dispatcher: bench(t).Dispatcher,
		Command: kernel.Command[request, unencodable, plan, detail]{
			Name:    "broken",
			Observe: func(context.Context) (unencodable, error) { return unencodable{Watch: make(chan int)}, nil },
		},
	})

	_, err := h.Observe(t.Context())

	assert.Equal(t, kernel.KindInternal, problem(t, err).Kind(), "facts no fingerprint can guard are never run on")
}

func TestAFormAskingAgainSeesTheFactsWithoutTheRememberedAnswers(t *testing.T) {
	h := handle(bench(t), &repo{})

	assert.Equal(t, facts{Existing: []string{"feat/old"}}, h.Forget(facts{Existing: []string{"feat/old"}, Remembered: verbatim}))
}
