package config

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnAbsentRunTomlIsAnEmptyRunConfig(t *testing.T) {
	cfg, err := Project{StateDir: t.TempDir()}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, project.Run{}, cfg)
}

func TestARunTomlWithNoJobIsStillDeclared(t *testing.T) {
	cfg, err := Project{StateDir: stateWithRun(t, "addressing = \"ports\"\n")}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, project.Run{Declared: true, Addressing: project.AddressPorts}, cfg)
}

func TestATypoedSectionIsRefusedByName(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n\n[[profiles]]\nname = \"full\"\njobs = [\"dev\"]\n")}.LoadRun()

	assert.ErrorContains(t, err, "profiles")
}

func TestPortsAreReadAsASubTableOrInline(t *testing.T) {
	cfg, err := Project{StateDir: stateWithRun(t, `
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
`)}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, 100, cfg.PortOffsetBlock)
	assert.Equal(t, map[string]int{"PORT": 3000}, cfg.Jobs[0].Ports)
	assert.Equal(t, map[string]int{"DB_PORT": 5432}, cfg.Jobs[1].Ports)
}

func TestAPortRunTomlWouldWriteWrongIsRefused(t *testing.T) {
	cases := []struct {
		name string
		jobs string
		want string
	}{
		{"port_out_of_range", "[[job]]\nname = \"web\"\nports = { PORT = 99999 }\n", `invalid run config: job "web": port PORT is 99999, outside 1-65535`},
		{"invalid_port_name", "[[job]]\nname = \"web\"\nports = { 1PORT = 3000 }\n", `invalid run config: job "web": port "1PORT" is not a valid environment variable name`},
		{"jobs_share_base", "[[job]]\nname = \"a\"\nports = { A = 3000 }\n\n[[job]]\nname = \"b\"\nports = { B = 3000 }\n", `invalid run config: ports A (job "a") and B (job "b") both declare base 3000 — they would bind the same port in every worktree`},
		{"bases_block_apart", "[[job]]\nname = \"web\"\nports = { ADMIN = 3010, PORT = 3000 }\n", `invalid run config: ports ADMIN (job "web", base 3010) and PORT (job "web", base 3000) collide 1 worktree(s) apart with a port_offset_block of 10 — move one base so the gap between them is not a multiple of 10`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Project{StateDir: stateWithRun(t, tc.jobs)}.LoadRun()

			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestTwoSharedJobsOnDifferentBasesNeverCollide(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "[[job]]\nname = \"pg\"\nscope = \"shared\"\nports = { PG = 5432 }\n\n[[job]]\nname = \"redis\"\nscope = \"shared\"\nports = { REDIS = 5442 }\n")}.LoadRun()

	assert.NoError(t, err)
}

func TestADuplicateJobNameStillLoads(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n\n[[job]]\nname = \"dev\"\nkind = \"service\"\ncmd = \"echo dup\"\n")}.LoadRun()

	assert.NoError(t, err)
}

func TestANegativeOrZeroBlockLoadsAndReadsAsTheDefault(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"negative", "-10"},
		{"zero", "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Project{StateDir: stateWithRun(t, "port_offset_block = "+tc.block+"\n\n[[job]]\nname = \"web\"\nkind = \"service\"\ncmd = \"pnpm dev\"\n")}.LoadRun()

			assert.NoError(t, err)
		})
	}
}

func TestAJobURLIsReadWithItsOptionalHost(t *testing.T) {
	cfg, err := Project{StateDir: stateWithRun(t, `
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
`)}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, &project.URL{Port: "PORT"}, cfg.Jobs[0].URL)
	assert.Equal(t, &project.URL{Port: "PORT", Host: "api.app-1"}, cfg.Jobs[1].URL)
	assert.Nil(t, cfg.Jobs[2].URL)
}

func TestEveryKeyWtmWritesInRunTomlIsReadIntoTheRunConfig(t *testing.T) {
	cfg, err := Project{StateDir: stateWithRun(t, `
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
`)}.LoadRun()

	require.NoError(t, err)
	assert.Equal(t, project.Run{
		Declared:        true,
		PortOffsetBlock: 20,
		Jobs: []project.Job{
			{
				Name: "db", Kind: project.Service, Cmd: "docker compose up -d db", Stop: "docker compose stop db", Cwd: "infra",
				Ports: map[string]int{"PG_PORT": 5432}, URL: &project.URL{Port: "PG_PORT", Host: "pg"}, Scope: project.Shared,
				Namespace: &project.Namespace{Name: "app_{worktree}", Create: "createdb {namespace}", Remove: "dropdb {namespace}", Env: map[string]string{"PGDATABASE": "app_{worktree}"}},
			},
			{Name: "migrate", Kind: project.Task, Cmd: "pnpm migrate"},
		},
		Profiles:   []project.Profile{{Name: "full", Jobs: []string{"db", "migrate"}, Default: true}},
		EnvPorts:   []project.EnvPortLink{{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "PG_PORT"}},
		EnvValues:  []project.EnvValueLink{{File: ".env", Key: "PGDATABASE", Job: "db", Value: "{namespace}"}},
		Addressing: project.AddressPorts,
		Isolation:  project.Verbatim,
	}, cfg)
}

func TestARunTomlAtANewerSchemaIsStillRead(t *testing.T) {
	cfg, err := Project{StateDir: stateWithRun(t, newerSchema+"[[job]]\nname = \"web\"\ncmd = \"npm run dev\"\n")}.LoadRun()

	require.NoError(t, err)
	assert.Len(t, cfg.Jobs, 1)
}

func TestARunTomlThatIsNotTOMLIsAnErrorTheCallerKeeps(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "[[job]\nname = \n")}.LoadRun()

	assert.Error(t, err)
}

func TestAnInvalidRunTomlIsRefusedSayingWhy(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"unknown_addressing", "addressing = \"nope\"\n", `invalid run config: unknown addressing "nope" (expected "ports" or "names")`},
		{"unknown_isolation", "isolation = \"keep\"\n", `invalid run config: unknown isolation "keep" (expected "isolated" or "verbatim")`},
		{"unknown_kind", "[[job]]\nname = \"api\"\nkind = \"servce\"\ncmd = \"pnpm dev\"\n", `invalid run config: job "api": unknown kind "servce" (expected service or task)`},
		{"unknown_scope", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"global\"\n", `invalid run config: job "db": unknown scope "global" (expected "shared")`},
		{"namespace_on_per_worktree_job", "[[job]]\nname = \"db\"\ncmd = \"x\"\n\n[job.namespace]\nname = \"app_{worktree}\"\ncreate = \"createdb\"\n", `invalid run config: job "db": [job.namespace] only means something on a job with scope = "shared"`},
		{"incomplete_namespace", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{worktree}\"\n", `invalid run config: job "db": a [job.namespace] block needs both a name and a create command`},
		{"namespace_bad_token", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{branch}\"\ncreate = \"createdb\"\n", `invalid run config: job "db": unknown placeholder in a [job.namespace] value: {branch}`},
		{"namespace_env_bad_token", "[[job]]\nname = \"db\"\ncmd = \"x\"\nscope = \"shared\"\n\n[job.namespace]\nname = \"app_{worktree}\"\ncreate = \"createdb\"\nenv = { DB = \"{branch}\" }\n", `invalid run config: job "db": unknown placeholder in a [job.namespace] value: {branch}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Project{StateDir: stateWithRun(t, tc.content)}.LoadRun()

			assert.EqualError(t, err, tc.want)
		})
	}
}

func TestEveryProblemOfARunTomlIsNamedAtOnce(t *testing.T) {
	_, err := Project{StateDir: stateWithRun(t, "addressing = \"nope\"\nisolation = \"keep\"\n")}.LoadRun()

	assert.EqualError(t, err, `invalid run config: unknown addressing "nope" (expected "ports" or "names"); unknown isolation "keep" (expected "isolated" or "verbatim")`)
}
