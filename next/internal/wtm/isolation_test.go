package wtm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnUnsetIsolationReadsAsIsolated(t *testing.T) {
	assert.Equal(t, IsolationIsolated, EffectiveIsolation(""))
	assert.Equal(t, IsolationVerbatim, EffectiveIsolation(IsolationVerbatim))
}

func TestOnlyTheTwoIsolationsAreValid(t *testing.T) {
	assert.True(t, IsolationIsolated.Valid())
	assert.True(t, IsolationVerbatim.Valid())
	assert.False(t, Isolation("keep").Valid())
	assert.False(t, Isolation("").Valid())
}

func TestIsolationAppliesWhenRunTomlDeclaresSomethingToIsolate(t *testing.T) {
	cases := []struct {
		name string
		run  RunConfig
		want bool
	}{
		{name: "no run.toml", run: RunConfig{}, want: false},
		{name: "a task and nothing else", run: RunConfig{Jobs: []JobConfig{{Name: "lint", Kind: JobKindTask, Cmd: "pnpm lint"}}}, want: false},
		{name: "a port", run: RunConfig{Jobs: []JobConfig{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}, want: true},
		{name: "a compose stack", run: RunConfig{Jobs: []JobConfig{{Name: "db", Cmd: "docker compose up -d db"}}}, want: true},
		{name: "a compose stop", run: RunConfig{Jobs: []JobConfig{{Name: "db", Cmd: "./up.sh", Stop: "docker-compose down"}}}, want: true},
		{name: "a namespace", run: RunConfig{Jobs: []JobConfig{{Name: "pg", Scope: JobScopeShared, Namespace: &JobNamespaceConfig{Name: "{worktree}", Create: "createdb {namespace}"}}}}, want: true},
		{name: "a namespace that creates nothing", run: RunConfig{Jobs: []JobConfig{{Name: "pg", Namespace: &JobNamespaceConfig{Name: "{worktree}"}}}}, want: false},
		{name: "an [[env_port]] link", run: RunConfig{EnvPorts: []EnvPortLink{{File: ".env", Key: "API_URL", Job: "api", Port: "PORT"}}}, want: true},
		{name: "an [[env]] link", run: RunConfig{EnvValues: []EnvValueLink{{File: ".env", Key: "REALM", Job: "kc", Value: "{worktree}"}}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsolationApplies(tc.run))
		})
	}
}
