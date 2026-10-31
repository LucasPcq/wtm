package daemon_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheIndexALegacyDaemonWroteIsTakenOver(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "legacy_jobs.json"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "jobs.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	records := (&daemon.Index{Path: path}).Load()

	require.Len(t, records, 3)
	api := records[0]
	assert.Equal(t, project.Job{Name: "api", Kind: project.Service, Cmd: "python3 ../../scripts/dev.py api", Cwd: "apps/api", Ports: map[string]int{"PORT": 27841}, URL: &project.URL{Port: "PORT"}}, api.Spec.Job)
	assert.Equal(t, &run.Origin{Root: "/private/tmp/wtm-sandbox.FI4ic8/acme", CommonDir: "/private/tmp/wtm-sandbox.FI4ic8/acme/.git", Branch: "main"}, api.Spec.Origin)
	assert.Equal(t, []run.Route{{Job: "api", Host: "api.main.acme.localhost", Port: "PORT"}}, api.Routes)
	assert.Equal(t, 2282, api.PGID)
	assert.True(t, api.Foreground())
	assert.True(t, records[1].Claim)
	assert.Equal(t, "/private/tmp/wtm-sandbox.FI4ic8/acme", records[1].SharedDir)
	assert.True(t, records[2].Claim, "an older daemon's attached is a claim")
}

func TestTheIndexKeepsWhatItSavesInTheLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	record := run.Record{
		Spec:      run.Spec{Job: project.Job{Name: "db", Kind: project.Service, Cmd: "up", Stop: "down", Scope: project.Shared, Namespace: &project.Namespace{Name: "app_${WTM_WORKTREE}", Create: "createdb", Remove: "dropdb"}}, WorkDir: "/wt/a", Env: map[string]string{"WTM_BRANCH": "feat/a"}, Unset: []string{"PORT"}, LogDir: "/repo/.git/wtm/logs/feat-a", Origin: &run.Origin{Root: "/repo", CommonDir: "/repo/.git", Branch: "feat/a", CorrelationID: "c1"}},
		StartedAt: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC),
		Claim:     true,
		SharedDir: "/repo",
		MainHolds: true,
		Routes:    []run.Route{{Job: "db", Host: "db.localhost", Port: "PORT"}},
	}

	require.NoError(t, (&daemon.Index{Path: path}).Save([]run.Record{record}))
	records := (&daemon.Index{Path: path}).Load()

	assert.Equal(t, []run.Record{record}, records)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `{"version":2,"jobs":[{"name":"db","work_dir":"/wt/a","config":{"name":"db","kind":"service","cmd":"up","stop":"down","scope":"shared","namespace":`)
	assert.Contains(t, string(data), `"joined":true,"shared_dir":"/repo","main_holds":true,"origin":{"repo":{"root":"/repo","common_dir":"/repo/.git"},"correlation_id":"c1"}`)
}

func TestAnIndexANewerWtmWroteIsReadNorWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	newer := `{"version":3,"jobs":[{"shape":"new"}]}`
	require.NoError(t, os.WriteFile(path, []byte(newer), 0o600))

	records := (&daemon.Index{Path: path}).Load()
	require.NoError(t, (&daemon.Index{Path: path}).Save(nil))

	assert.Empty(t, records)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, newer, string(data))
}

func TestAnIndexOfAnOlderVersionIsReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"jobs":[{"name":"web"}]}`), 0o600))

	records := (&daemon.Index{Path: path}).Load()
	require.NoError(t, (&daemon.Index{Path: path}).Save(nil))

	assert.Empty(t, records)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, `{"version":2,"jobs":[]}`, string(data))
}

func TestAnIndexTakenOverIsWrittenBackByteForByte(t *testing.T) {
	legacy := `{"version":2,"jobs":[{"name":"api","work_dir":"/repo","config":{"name":"api","kind":"service","cmd":"pnpm dev","cwd":"apps/api","ports":{"PORT":3001},"url":{"port":"PORT"},"probe":false,"binds_no_port":true,"runs":["migrate"],"touches":["apps/api"]},"env":{"PORT":"3001","WTM_BRANCH":"main"},"routes":[{"job":"api","host":"api.main.acme.localhost","port":"PORT"}],"log_dir":"/repo/.git/wtm/logs/main","started_at":"2026-10-31T00:51:37.358833+01:00","pid":2282,"pgid":2282,"origin":{"repo":{"root":"/repo","common_dir":"/repo/.git"}}},{"name":"db","work_dir":"/wt/a","config":{"name":"db","kind":"service","cmd":"docker compose up -d","stop":"docker compose down","scope":"shared","namespace":{"name":"app","create":"createdb app","remove":"dropdb app"}},"started_at":"2026-10-31T00:53:08.536943+01:00","joined":true,"shared_dir":"/repo","main_holds":true}]}`
	path := filepath.Join(t.TempDir(), "jobs.json")
	require.NoError(t, os.WriteFile(path, []byte(legacy), 0o600))
	index := &daemon.Index{Path: path}

	require.NoError(t, index.Save(index.Load()))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, legacy, string(data))
}

func TestALaterStartOfAJobInheritsNoneOfTheKeysItsOldRecordKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":2,"jobs":[{"name":"db","work_dir":"/wt/a","config":{"name":"db","kind":"service","cmd":"up","stop":"down","probe":false,"runs":["seed"]},"started_at":"2026-10-31T12:00:00Z"}]}`), 0o600))
	index := &daemon.Index{Path: path}
	restarted := index.Load()[0]
	restarted.StartedAt = restarted.StartedAt.Add(time.Hour)

	require.NoError(t, index.Save([]run.Record{restarted}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "probe")
	assert.NotContains(t, string(data), "seed")
}
