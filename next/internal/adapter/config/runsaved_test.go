package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavedRunWithoutARunTomlIsEmpty(t *testing.T) {
	saved, err := Project{StateDir: t.TempDir()}.SavedRun()

	require.NoError(t, err)
	assert.Equal(t, project.SavedRun{}, saved)
}

func TestSavedRunIsTheRunTomlWithItsDigest(t *testing.T) {
	stateDir := stateWithRun(t, golden(t, "run.saved.toml"))

	saved, err := Project{StateDir: stateDir}.SavedRun()

	require.NoError(t, err)
	assert.Equal(t, savedRun(), saved.Run)
	assert.Equal(t, digest([]byte(golden(t, "run.saved.toml"))), saved.Digest)
}

func TestSavedRunRefusesARunTomlThatDoesNotValidate(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "concurrency = \"sometimes\"\n")}.SavedRun()

	assert.ErrorContains(t, err, "unknown concurrency")
}

func TestSaveRunWritesTheLegacyLayoutAndItsSchema(t *testing.T) {
	stateDir := t.TempDir()

	written, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: savedRun()})

	require.NoError(t, err)
	assert.True(t, written.Changed)
	assert.Equal(t, golden(t, "run.saved.toml"), readFile(t, filepath.Join(stateDir, "run.toml")))
	assert.FileExists(t, filepath.Join(stateDir, "schemas", "run.schema.json"))
}

func TestWhatSaveRunWritesIsLoadedBack(t *testing.T) {
	stateDir := t.TempDir()
	_, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: savedRun()})
	require.NoError(t, err)

	loaded, err := Project{StateDir: stateDir}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, savedRun(), loaded)
}

func TestSaveRunReplacesTheRunTomlItRead(t *testing.T) {
	stateDir := stateWithRun(t, golden(t, "run.saved.toml"))
	saved, err := Project{StateDir: stateDir}.SavedRun()
	require.NoError(t, err)
	saved.Run.Concurrency = project.Parallel

	_, err = Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: saved.Run, Replaces: saved.Digest})

	require.NoError(t, err)
	loaded, err := Project{StateDir: stateDir}.LoadRun()
	require.NoError(t, err)
	assert.Equal(t, project.Parallel, loaded.Concurrency)
}

func TestSaveRunRefusesARunTomlChangedSinceItWasRead(t *testing.T) {
	cases := map[string]struct {
		onDisk   string
		replaces string
	}{
		"edited":  {onDisk: "addressing = \"names\"\n", replaces: digest([]byte("addressing = \"ports\"\n"))},
		"created": {onDisk: "addressing = \"names\"\n", replaces: ""},
		"removed": {replaces: digest([]byte("addressing = \"ports\"\n"))},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stateDir := t.TempDir()
			if c.onDisk != "" {
				writeFile(t, filepath.Join(stateDir, "run.toml"), c.onDisk)
			}

			_, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: savedRun(), Replaces: c.replaces})

			require.ErrorIs(t, err, project.ErrRunChanged)
			if c.onDisk != "" {
				assert.Equal(t, c.onDisk, readFile(t, filepath.Join(stateDir, "run.toml")))
			}
		})
	}
}

func TestSaveRunRefusesARunTomlANewerWtmWrote(t *testing.T) {
	stateDir := stateWithRun(t, newerSchema)

	_, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: savedRun(), Replaces: digest([]byte(newerSchema))})

	_, newer := errors.AsType[*domain.SchemaNewer](err)
	assert.True(t, newer, "SaveRun = %v, want a *domain.SchemaNewer", err)
}

func TestSaveRunNeverWritesWhatLoadRunRefuses(t *testing.T) {
	stateDir := t.TempDir()
	run := project.Run{Jobs: []project.Job{{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Ports: map[string]int{"web port": 3000}}}}

	_, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: run})

	assert.ErrorContains(t, err, "not a valid environment variable name")
	assert.NoFileExists(t, filepath.Join(stateDir, "run.toml"))
	_, statErr := os.Stat(filepath.Join(stateDir, "schemas"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSaveRunNeverWritesWhatTheLegacyWtmRefuses(t *testing.T) {
	web := project.Job{Name: "web", Kind: project.Service, Cmd: "pnpm dev", Ports: map[string]int{"WEB_PORT": 3000}}
	cases := map[string]struct {
		run  project.Run
		want string
	}{
		"job_twice":        {run: project.Run{Jobs: []project.Job{web, web}}, want: `job "web" is declared twice`},
		"url_port":         {run: project.Run{Jobs: []project.Job{{Name: "web", Kind: project.Service, Cmd: "x", URL: &project.URL{Port: "PORT"}}}}, want: "url.port names PORT"},
		"host_taken":       {run: project.Run{Jobs: []project.Job{withURL(web, "app"), withURL(project.Job{Name: "admin", Kind: project.Service, Cmd: "x", Ports: map[string]int{"ADMIN_PORT": 3001}}, "app")}}, want: `both publish host "app"`},
		"env_port_none":    {run: project.Run{Jobs: []project.Job{web}, EnvPorts: []project.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "API_PORT"}}}, want: "does not declare it"},
		"env_constant":     {run: project.Run{Jobs: []project.Job{web}, EnvValues: []project.EnvValueLink{{File: ".env", Key: "URL", Job: "web", Value: "http://localhost"}}}, want: "holds no placeholder"},
		"env_placeholder":  {run: project.Run{Jobs: []project.Job{web}, EnvValues: []project.EnvValueLink{{File: ".env", Key: "URL", Job: "web", Value: "{port}"}}}, want: "unknown placeholder {port}"},
		"env_namespace":    {run: project.Run{Jobs: []project.Job{web}, EnvValues: []project.EnvValueLink{{File: ".env", Key: "DB", Job: "web", Value: "{namespace}"}}}, want: "unknown placeholder {namespace}"},
		"env_clashes_port": {run: project.Run{Jobs: []project.Job{web}, EnvPorts: []project.EnvPortLink{{File: ".env", Key: "PORT", Job: "web", Port: "WEB_PORT"}}, EnvValues: []project.EnvValueLink{{File: ".env", Key: "PORT", Job: "web", Value: "{port.WEB_PORT}"}}}, want: "also an env_port"},
		"env_no_job":       {run: project.Run{EnvValues: []project.EnvValueLink{{File: ".env", Key: "URL", Job: "api", Value: "{worktree}"}}}, want: `no job named "api"`},
		"no_cmd":           {run: project.Run{Jobs: []project.Job{{Name: "web", Kind: project.Service}}}, want: "cmd is required"},
		"no_kind":          {run: project.Run{Jobs: []project.Job{{Name: "web", Cmd: "x"}}}, want: "kind is required"},
		"name_space":       {run: project.Run{Jobs: []project.Job{{Name: "my web", Kind: project.Service, Cmd: "x"}}}, want: "a name holds no space"},
		"task_stop":        {run: project.Run{Jobs: []project.Job{{Name: "seed", Kind: project.Task, Cmd: "x", Stop: "y"}}}, want: "a task declares no stop command"},
		"binds_and_ports":  {run: project.Run{Jobs: []project.Job{{Name: "web", Kind: project.Service, Cmd: "x", BindsNoPort: true, Ports: map[string]int{"PORT": 3000}}}}, want: "binds_no_port contradicts"},
		"touches_unknown":  {run: project.Run{Jobs: []project.Job{{Name: "seed", Kind: project.Task, Cmd: "x", Touches: []string{"postgres"}}}}, want: `touches unknown job "postgres"`},
		"runs_unknown":     {run: project.Run{Jobs: []project.Job{{Name: "dev", Kind: project.Service, Cmd: "x", Runs: []string{"web"}}}}, want: `runs unknown job "web"`},
		"runs_cycle":       {run: project.Run{Jobs: []project.Job{{Name: "a", Kind: project.Service, Cmd: "x", Runs: []string{"b"}}, {Name: "b", Kind: project.Service, Cmd: "x", Runs: []string{"a"}}}}, want: "runs itself through"},
		"profile_unknown":  {run: project.Run{Jobs: []project.Job{web}, Profiles: []project.Profile{{Name: "all", Jobs: []string{"dev"}}}}, want: `profile "all" references unknown job "dev"`},
		"profile_twice":    {run: project.Run{Jobs: []project.Job{web}, Profiles: []project.Profile{{Name: "all", Jobs: []string{"web"}}, {Name: "all", Jobs: []string{"web"}}}}, want: `profile "all" is declared twice`},
		"two_defaults":     {run: project.Run{Jobs: []project.Job{web}, Profiles: []project.Profile{{Name: "a", Jobs: []string{"web"}, Default: true}, {Name: "b", Jobs: []string{"web"}, Default: true}}}, want: "2 profiles are the default"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stateDir := t.TempDir()

			_, err := Project{StateDir: stateDir}.SaveRun(t.Context(), project.RunRewrite{Run: c.run})

			assert.ErrorContains(t, err, c.want)
			assert.NoFileExists(t, filepath.Join(stateDir, "run.toml"))
		})
	}
}

func TestARunTomlAtANewerSchemaIsNeverReadToBeRewritten(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, newerSchema+"future = true\n[[job]]\nname = \"web\"\ncmd = \"npm run dev\"\n")}.SavedRun()

	var newer *domain.SchemaNewer
	require.ErrorAs(t, err, &newer)
	assert.Equal(t, 99, newer.Found)
}
