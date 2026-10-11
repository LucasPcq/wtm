package dispatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestTheSchemaIsTheFieldsAsData(t *testing.T) {
	specs := handle(bench(t), &repo{}).Schema()

	paths := make([]string, 0, len(specs))
	for _, spec := range specs {
		paths = append(paths, spec.Path)
	}
	assert.Equal(t, []string{"branches", "from", "isolation"}, paths)
}

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

	assert.Equal(t, kernel.KindNotFound, problem(t, err).Kind())
}

func TestAFormAskingAgainSeesTheFactsWithoutTheRememberedAnswers(t *testing.T) {
	h := handle(bench(t), &repo{})

	assert.Equal(t, facts{Existing: []string{"feat/old"}}, h.Forget(facts{Existing: []string{"feat/old"}, Remembered: verbatim}))
}
