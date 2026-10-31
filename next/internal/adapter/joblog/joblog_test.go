package joblog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALogIsALinePerLineItsTimeFirstAsTheLegacyWtmReadsIt(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(run.LogFile{Dir: dir, Job: "web"})
	require.NoError(t, err)

	log.Write([]run.Line{{At: at, Text: "ready in 3ms"}, {At: at.Add(time.Second), Text: "GET / 200"}})
	log.Close()

	data, err := os.ReadFile(filepath.Join(dir, "web.log"))
	require.NoError(t, err)
	assert.Equal(t, "2026-10-31T12:00:00Z  ready in 3ms\n2026-10-31T12:00:01Z  GET / 200\n", string(data))
}

func TestARunEmptiesItsLogAndItsBackupsUnlessItAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "web.log")
	require.NoError(t, os.WriteFile(path, []byte("2026-10-30T12:00:00Z  old run\n"), 0o644))
	require.NoError(t, os.WriteFile(path+".1", []byte("older\n"), 0o644))

	appended, err := Open(run.LogFile{Dir: dir, Job: "web", Append: true})
	require.NoError(t, err)
	appended.Write([]run.Line{{At: at, Text: "relaunched"}})
	appended.Close()
	assert.Equal(t, []string{"older", "old run", "relaunched"}, texts(Last(run.LogFile{Dir: dir, Job: "web"}, 10)), "an appended run keeps its backup")

	fresh, err := Open(run.LogFile{Dir: dir, Job: "web"})
	require.NoError(t, err)
	fresh.Close()
	assert.Empty(t, Last(run.LogFile{Dir: dir, Job: "web"}, 10))
	assert.NoFileExists(t, path+".1")
}

func TestALogPast5MiBRotatesAndItsLastLinesReachIntoTheBackup(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(run.LogFile{Dir: dir, Job: "web"})
	require.NoError(t, err)
	big := strings.Repeat("x", 1<<20)

	for range 5 {
		log.Write([]run.Line{{At: at, Text: big}})
	}
	log.Write([]run.Line{{At: at, Text: "after"}})
	log.Close()

	assert.FileExists(t, filepath.Join(dir, "web.log.1"))
	assert.Equal(t, []string{big, "after"}, texts(Last(run.LogFile{Dir: dir, Job: "web"}, 2)))
}

func TestALineNotStampedKeepsNoTime(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "web.log"), []byte("a hand-written line\n2026-10-31T12:00:00Z  stamped\n"), 0o644))

	lines := Last(run.LogFile{Dir: dir, Job: "web"}, 10)

	assert.Equal(t, []run.Line{{Text: "a hand-written line"}, {At: at, Text: "stamped"}}, lines)
}

func TestAJobNameIsEscapedIntoOneFile(t *testing.T) {
	assert.Equal(t, "/logs/api%2Fv2.log", Path("/logs", "api/v2"))
	assert.Empty(t, Path("/logs", ".."))
	assert.Empty(t, Path("", "web"))
	_, err := Open(run.LogFile{Dir: t.TempDir(), Job: ".."})
	assert.Error(t, err)
}
