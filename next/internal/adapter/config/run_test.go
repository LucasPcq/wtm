package config

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnAbsentRunTomlIsAnEmptyRunConfig(t *testing.T) {
	cfg, err := LoadRun(t.TempDir())

	require.NoError(t, err)
	assert.Equal(t, run.Config{}, cfg)
}

func TestATypoedSectionIsRefusedByName(t *testing.T) {
	_, err := LoadRun(stateWithRun(t, "[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n\n[[profiles]]\nname = \"full\"\njobs = [\"dev\"]\n"))

	assert.ErrorContains(t, err, "profiles")
}

func TestPortsAreReadAsASubTableOrInline(t *testing.T) {
	cfg, err := LoadRun(stateWithRun(t, `
port_offset_block = 100

[[job]]
name = "web"
kind = "service"
cmd = "pnpm dev"
  [job.ports]
  PORT = 3000

[[job]]
name = "db"
kind = "service"
cmd = "docker compose up -d"
ports = { DB_PORT = 5432 }
`))

	require.NoError(t, err)
	assert.Equal(t, 100, cfg.PortOffsetBlock)
	assert.Equal(t, map[string]int{"PORT": 3000}, cfg.Jobs[0].Ports)
	assert.Equal(t, map[string]int{"DB_PORT": 5432}, cfg.Jobs[1].Ports)
}

func TestCollidingPortsAreNoLongerRefusedOnRead(t *testing.T) {
	cfg, err := LoadRun(stateWithRun(t, "[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"pnpm dev\"\nports = { PORT = 3000, ADMIN = 3010 }\n"))

	require.NoError(t, err)
	assert.Len(t, cfg.Jobs, 1)
}

func TestADuplicateJobNameStillLoads(t *testing.T) {
	_, err := LoadRun(stateWithRun(t, "[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n\n[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"echo dup\"\n"))

	assert.NoError(t, err)
}

func TestANegativeOrZeroBlockLoadsAndReadsAsTheDefault(t *testing.T) {
	for _, block := range []string{"-10", "0"} {
		t.Run(block, func(t *testing.T) {
			_, err := LoadRun(stateWithRun(t, "port_offset_block = "+block+"\n\n[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n"))

			assert.NoError(t, err)
		})
	}
}

func TestAJobURLIsReadWithItsOptionalHost(t *testing.T) {
	cfg, err := LoadRun(stateWithRun(t, `
[[job]]
name  = "web"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 3000 }
url   = { port = "PORT" }

[[job]]
name  = "api"
kind  = "service"
cmd   = "pnpm dev --port ${PORT}"
ports = { PORT = 4000 }
url   = { port = "PORT", host = "api.app-1" }

[[job]]
name  = "db"
kind  = "service"
cmd   = "docker compose up"
ports = { PG_PORT = 5432 }
`))

	require.NoError(t, err)
	assert.Equal(t, &run.URL{Port: "PORT"}, cfg.Jobs[0].URL)
	assert.Equal(t, &run.URL{Port: "PORT", Host: "api.app-1"}, cfg.Jobs[1].URL)
	assert.Nil(t, cfg.Jobs[2].URL)
}

func TestEveryKeyWtmWritesInRunTomlIsReadIntoTheRunConfig(t *testing.T) {
	cfg, err := LoadRun(stateWithRun(t, `
schema = 1
port_offset_block = 20
port_probe_timeout = 5
addressing = "ports"
concurrency = "parallel"
isolation = "verbatim"

[[job]]
name = "db"
kind = "service"
cmd = "docker compose up -d db"
stop = "docker compose stop db"
cwd = "infra"
probe = false
binds_no_port = false
runs = []
touches = []
scope = "shared"
ports = { PG_PORT = 5432 }
url = { port = "PG_PORT", host = "pg" }

[job.namespace]
name = "app_{worktree}"
create = "createdb {namespace}"
remove = "dropdb {namespace}"
env = { PGDATABASE = "app_{worktree}" }

[[job]]
name = "migrate"
kind = "task"
cmd = "pnpm migrate"
touches = ["db"]

[[profile]]
name = "full"
jobs = ["db", "migrate"]
default = true

[[env_port]]
file = ".env"
key = "DATABASE_URL"
job = "db"
port = "PG_PORT"

[[env]]
file = ".env"
key = "PGDATABASE"
job = "db"
value = "{namespace}"
`))

	require.NoError(t, err)
	assert.Equal(t, run.Config{
		PortOffsetBlock: 20,
		Jobs: []run.Job{
			{
				Name: "db", Kind: run.Service, Cmd: "docker compose up -d db", Stop: "docker compose stop db", Cwd: "infra",
				Ports: map[string]int{"PG_PORT": 5432}, URL: &run.URL{Port: "PG_PORT", Host: "pg"}, Scope: run.Shared,
				Namespace: &run.Namespace{Name: "app_{worktree}", Create: "createdb {namespace}", Remove: "dropdb {namespace}", Env: map[string]string{"PGDATABASE": "app_{worktree}"}},
			},
			{Name: "migrate", Kind: run.Task, Cmd: "pnpm migrate"},
		},
		Profiles:   []run.Profile{{Name: "full", Jobs: []string{"db", "migrate"}, Default: true}},
		EnvPorts:   []run.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "PG_PORT"}},
		EnvValues:  []run.EnvValueLink{{File: ".env", Key: "PGDATABASE", Job: "db", Value: "{namespace}"}},
		Addressing: run.AddressPorts,
		Isolation:  run.Verbatim,
	}, cfg)
}

func TestARunTomlAtANewerSchemaIsStillRead(t *testing.T) {
	cfg, err := LoadRun(stateWithRun(t, newerSchema+"[[job]]\nname = \"web\"\ncmd = \"npm run dev\"\n"))

	require.NoError(t, err)
	assert.Len(t, cfg.Jobs, 1)
}

func TestARunTomlThatIsNotTOMLIsAnErrorTheCallerKeeps(t *testing.T) {
	_, err := LoadRun(stateWithRun(t, "[[job]\nname = \n"))

	assert.Error(t, err)
}

func TestAnInvalidRunTomlIsRefusedSayingWhy(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"an unknown addressing", "addressing = \"nope\"\n", `invalid run config: unknown addressing "nope" (expected "ports" or "names")`},
		{"an unknown isolation", "isolation = \"keep\"\n", `invalid run config: unknown isolation "keep" (expected "isolated" or "verbatim")`},
		{"an unknown kind", "[[job]]\nname = \"api\"\nkind = \"servce\"\ncmd = \"pnpm dev\"\n", `invalid run config: job "api": unknown kind "servce" (expected service or task)`},
		{"an unknown scope", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"global\"\n", `invalid run config: job "db": unknown scope "global" (expected "shared")`},
		{"a namespace on a per-worktree job", "[[job]]\nname = \"db\"\ncmd = \"x\"\n\n[job.namespace]\nname = \"app_{worktree}\"\ncreate = \"createdb\"\n", `invalid run config: job "db": [job.namespace] only means something on a job with scope = "shared"`},
		{"an incomplete namespace", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{worktree}\"\n", `invalid run config: job "db": a [job.namespace] block needs both a name and a create command`},
		{"a namespace with a bad token", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{branch}\"\ncreate = \"createdb\"\n", `invalid run config: job "db": unknown placeholder in a [job.namespace] value: {branch}`},
		{"a namespace env with a bad token", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{worktree}\"\ncreate = \"createdb\"\nenv = { DB = \"{branch}\" }\n", `invalid run config: job "db": unknown placeholder in a [job.namespace] value: {branch}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadRun(stateWithRun(t, tc.content))

			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestEveryProblemOfARunTomlIsNamedAtOnce(t *testing.T) {
	_, err := LoadRun(stateWithRun(t, "addressing = \"nope\"\nisolation = \"keep\"\n"))

	assert.EqualError(t, err, `invalid run config: unknown addressing "nope" (expected "ports" or "names"); unknown isolation "keep" (expected "isolated" or "verbatim")`)
}
