package shell

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEnvironmentDropsWhatIsUnsetAndSetsWhatIsGiven(t *testing.T) {
	t.Setenv("WTM_NEXT_KEPT", "kept")
	t.Setenv("WTM_NEXT_DROPPED", "dropped")
	t.Setenv("WTM_NEXT_REPLACED", "old")

	env := Environ([]string{"WTM_NEXT_DROPPED"}, map[string]string{"WTM_NEXT_REPLACED": "new", "WTM_NEXT_ADDED": "added"})

	assert.Contains(t, env, "WTM_NEXT_KEPT=kept")
	assert.Contains(t, env, "WTM_NEXT_REPLACED=new")
	assert.Contains(t, env, "WTM_NEXT_ADDED=added")
	assert.NotContains(t, env, "WTM_NEXT_DROPPED=dropped")
	assert.NotContains(t, env, "WTM_NEXT_REPLACED=old")
}

func TestAnExitCodeIsKnownOnlyForAProcessThatExited(t *testing.T) {
	exited := exec.Command("sh", "-c", "exit 3").Run()
	killed := exec.Command("sh", "-c", "kill -9 $$").Run()

	code, ok := ExitCode(exited)
	require.True(t, ok)
	assert.Equal(t, 3, code)
	_, ok = ExitCode(killed)
	assert.False(t, ok)
	_, ok = ExitCode(assert.AnError)
	assert.False(t, ok)
}
