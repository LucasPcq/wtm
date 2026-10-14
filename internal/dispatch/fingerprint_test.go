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
