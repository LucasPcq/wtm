package dispatch_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestTheOutputOfAPhaseThatNamesItsLogIsKeptThere(t *testing.T) {
	r := &repo{logDir: t.TempDir()}
	ran := run(t.Context(), runParams{Bench: bench(t), Repo: r, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	log, err := os.ReadFile(r.logOf("feat/a"))

	require.NoError(t, err)
	assert.Equal(t, hookLine+"\n", string(log))
}

func TestTheSurfaceSeesEveryEventStampedWithTheOperation(t *testing.T) {
	ran := run(t.Context(), runParams{Bench: bench(t), Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	kinds := make([]string, 0, len(ran.Progress))
	for _, progress := range ran.Progress {
		assert.Equal(t, kernel.OpID("op-1"), progress.Base().Op)
		kinds = append(kinds, kindOf(progress))
	}
	assert.Equal(t, []string{
		"unit.started",
		"phase.started", "phase.finished",
		"phase.started", "phase.finished",
		"phase.started", "output", "phase.finished",
		"unit.finished",
	}, kinds)
}
