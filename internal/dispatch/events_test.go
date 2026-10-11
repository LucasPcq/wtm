package dispatch_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAnOperationIsAnnouncedThenFinishedUnitByUnit(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{
		Bench:   b,
		Repo:    &repo{},
		Request: request{Branches: []string{"feat/a", "feat/b"}},
		Options: dispatch.Options{Surface: dispatch.SurfaceCLI, CorrelationID: "agent-7"},
	})
	require.NoError(t, ran.Err)

	ok := true
	operation := &domain.EventOperation{Command: commandName, Surface: string(dispatch.SurfaceCLI)}
	assert.Equal(t, []domain.Event{
		{Type: domain.EventOperationStarted, CorrelationID: "agent-7", OpID: "op-1", Operation: operation},
		{Type: domain.EventUnitFinished, CorrelationID: "agent-7", OpID: "op-1", Unit: &domain.EventUnit{Subject: "feat/a", Status: string(kernel.StatusDone)}},
		{Type: domain.EventUnitFinished, CorrelationID: "agent-7", OpID: "op-1", Unit: &domain.EventUnit{Subject: "feat/b", Status: string(kernel.StatusDone)}},
		{Type: domain.EventOperationFinished, CorrelationID: "agent-7", OpID: "op-1", Operation: operation, OK: &ok},
	}, b.Bus.Events())
}

func TestAnOperationThatFailsEndsWithItsCode(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{applyErr: errors.New("disk full")}, Request: request{Branches: []string{"feat/a"}}})
	require.Error(t, ran.Err)

	events := b.Bus.Events()
	require.Equal(t, []domain.EventType{domain.EventOperationStarted, domain.EventOperationFinished}, b.Bus.Types())
	finished := events[1]
	require.NotNil(t, finished.OK)
	assert.False(t, *finished.OK)
	assert.Equal(t, string(kernel.CodeInternal), finished.Code)
}

func TestARefusedRunPublishesNothing(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{}})
	require.Error(t, ran.Err)

	assert.Empty(t, b.Bus.Events())
}
