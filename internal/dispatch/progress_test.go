package dispatch_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestTheOutputOfAHookPhaseIsKeptInItsLog(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	log, err := os.ReadFile(rules.HooksLogPath(rules.HooksLogPathParams{StateDir: b.StateDir, Phase: domain.HookOnCreate, Branch: "feat/a"}))

	require.NoError(t, err)
	assert.Equal(t, hookLine+"\n", string(log))
}

func TestTheSurfaceSeesEveryEventStampedWithTheOperation(t *testing.T) {
	ran := run(t.Context(), runParams{Bench: bench(t), Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	kinds := make([]kernel.ProgressKind, 0, len(ran.Progress))
	for _, progress := range ran.Progress {
		assert.Equal(t, kernel.OpID("op-1"), progress.Op)
		kinds = append(kinds, progress.Kind)
	}
	assert.Equal(t, []kernel.ProgressKind{
		kernel.ProgressUnitStarted,
		kernel.ProgressPhaseStarted, kernel.ProgressPhaseFinished,
		kernel.ProgressPhaseStarted, kernel.ProgressOutput, kernel.ProgressPhaseFinished,
		kernel.ProgressUnitFinished,
	}, kinds)
}
