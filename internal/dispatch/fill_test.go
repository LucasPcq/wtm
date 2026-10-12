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

func TestOnlyACommandIsFillable(t *testing.T) {
	b := bench(t)

	_, command := handle(b, &repo{}).Fillable()
	_, query := dispatch.RegisterQuery(operations.Query(b.Dispatcher.Operations)).Fillable()

	assert.True(t, command)
	assert.False(t, query)
}

func TestAFillableObservesWhatItsHandleObserves(t *testing.T) {
	h := handle(bench(t), &repo{existing: []string{"feat/old"}})
	typed, err := h.Observe(t.Context())
	require.NoError(t, err)

	observed, err := fillable(t, h).Observe(t.Context())

	require.NoError(t, err)
	assert.Equal(t, typed.Fingerprint, observed.Fingerprint)
}

func TestTheStateOfAJSONRequestIsTheStateOfItsTypedOne(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{}))
	observed, err := f.Observe(t.Context())
	require.NoError(t, err)

	form, err := f.State(dispatch.FillParams{Request: json.RawMessage(`{"branches":["feat/a"]}`), Facts: observed})

	require.NoError(t, err)
	assert.True(t, form.Complete)
	assert.JSONEq(t, `{"branches":["feat/a"],"from":"","isolation":"isolated"}`, string(form.Request))
	assert.Equal(t, kernel.OriginDefault, form.States[2].Origin)
}

func TestAnUnknownFieldInAFilledRequestIsA422(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{}))
	observed, err := f.Observe(t.Context())
	require.NoError(t, err)

	_, err = f.State(dispatch.FillParams{Request: json.RawMessage(`{"branch":"feat/a"}`), Facts: observed})

	assert.Equal(t, kernel.KindInvalid, problem(t, err).Kind())
}

func TestForgettingKeepsTheFingerprintAndDropsTheRememberedAnswer(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{remembered: verbatim}))
	observed, err := f.Observe(t.Context())
	require.NoError(t, err)

	forgotten := f.Forget(observed)
	form, err := f.State(dispatch.FillParams{Request: json.RawMessage(`{"branches":["feat/a"]}`), Facts: forgotten})

	require.NoError(t, err)
	assert.Equal(t, observed.Fingerprint, forgotten.Fingerprint)
	assert.Equal(t, kernel.Text(isolated), form.States[2].Value)
}

func TestTheChoicesOfAFillableCarryTheirRefusal(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{existing: []string{"feat/a"}}))
	observed, err := f.Observe(t.Context())
	require.NoError(t, err)

	choices, err := f.Choices(t.Context(), dispatch.FillChoicesParams{Path: "from", Request: json.RawMessage(`{"branches":["feat/a"]}`), Facts: observed})

	require.NoError(t, err)
	require.Len(t, choices.Options, 2)
	require.NotNil(t, choices.Options[1].Refusal)
	assert.Equal(t, kernel.CodeSelfParent, choices.Options[1].Refusal.Code)
}

func TestFactsAnotherEntryObservedAreAnInternalError(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{}))

	_, err := f.State(dispatch.FillParams{Request: json.RawMessage(`{}`)})

	assert.Equal(t, kernel.KindInternal, problem(t, err).Kind())
}

func TestSetWritesAValueIntoTheRequestByItsPath(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{}))

	request, err := f.Set(dispatch.SetParams{Request: json.RawMessage(`{"branches":["feat/a"]}`), Path: "from", Value: kernel.Text("main")})

	require.NoError(t, err)
	assert.JSONEq(t, `{"branches":["feat/a"],"from":"main","isolation":""}`, string(request))
}

func TestSetRefusesAValueOfTheWrongShape(t *testing.T) {
	f := fillable(t, handle(bench(t), &repo{}))

	_, err := f.Set(dispatch.SetParams{Path: "branches", Value: kernel.Bool(true)})

	failure := problem(t, err)
	assert.Equal(t, kernel.KindInvalid, failure.Kind())
}
