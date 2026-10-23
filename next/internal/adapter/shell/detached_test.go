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

	require.NoError(t, Detached("sh", []string{"-c", `echo "$WTM_SEEN $(ps -o pgid= -p $$ | tr -d ' ') $$" > "$OUT.part" && mv "$OUT.part" "$OUT"`}, []string{"WTM_SEEN=yes", "OUT=" + out, "PATH=" + os.Getenv("PATH")}))

	require.Eventually(t, func() bool { _, err := os.Stat(out); return err == nil }, 5*time.Second, 20*time.Millisecond)
	data, err := os.ReadFile(out)
	require.NoError(t, err)
	fields := strings.Fields(string(data))
	require.Len(t, fields, 3)
	assert.Equal(t, "yes", fields[0])
	assert.Equal(t, fields[2], fields[1], "the child does not lead a group of its own")
}
