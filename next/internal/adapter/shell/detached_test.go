package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADetachedChildRunsInASessionOfItsOwnWithTheEnvironmentGiven(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")

	require.NoError(t, Detached(DetachedParams{Name: "sh", Args: []string{"-c", `echo "$WTM_SEEN $(ps -o pgid= -p $$ | tr -d ' ') $$" > "$OUT.part" && mv "$OUT.part" "$OUT"`}, Env: []string{"WTM_SEEN=yes", "OUT=" + out, "PATH=" + os.Getenv("PATH")}}))

	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 5*time.Second, 20*time.Millisecond)
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	fields := strings.Fields(string(data))
	require.Len(t, fields, 3)
	assert.Equal(t, "yes", fields[0])
	assert.Equal(t, fields[2], fields[1], "the child does not lead a group of its own")
}

func TestADetachedChildAppendsItsStandardErrorToItsLog(t *testing.T) {
	log := filepath.Join(t.TempDir(), "daemon.log")
	require.NoError(t, os.WriteFile(log, []byte("an older start\n"), 0o600))

	require.NoError(t, Detached(DetachedParams{Name: "sh", Args: []string{"-c", "echo out; echo listen: invalid argument >&2"}, Log: log}))

	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(log)
		return string(data) == "an older start\nlisten: invalid argument\n"
	}, 5*time.Second, 20*time.Millisecond)
}
