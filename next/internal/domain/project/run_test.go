package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAJobRunsComposeInEitherSpellingOfItsStartOrStop(t *testing.T) {
	assert.True(t, Job{Cmd: "docker compose up -d"}.RunsCompose())
	assert.True(t, Job{Cmd: "make up", Stop: "docker-compose down"}.RunsCompose())
	assert.False(t, Job{Cmd: "pnpm dev"}.RunsCompose())
}

func TestAJobHasANamespaceOnlyWhenItNamesAndCreatesOne(t *testing.T) {
	assert.True(t, Job{Namespace: &Namespace{Name: "{worktree}", Create: "createdb {namespace}"}}.hasNamespace())
	assert.False(t, Job{Namespace: &Namespace{Name: "{worktree}"}}.hasNamespace())
	assert.False(t, Job{}.hasNamespace())
}

func TestAnUnsetIsolationIsEffectivelyIsolated(t *testing.T) {
	assert.Equal(t, Isolated, Isolation("").Effective())
	assert.Equal(t, Verbatim, Verbatim.Effective())
}

func TestOnlyTheTwoIsolationsAreValid(t *testing.T) {
	for _, isolation := range Isolations() {
		assert.True(t, isolation.Valid(), isolation)
	}
	assert.False(t, Isolation("keep").Valid())
}

func TestAnEmptyIsolationIsNotGivenRatherThanAValue(t *testing.T) {
	assert.False(t, Isolation("").Valid())
}

func TestTheIsolationListIsACopyACallerMayChange(t *testing.T) {
	isolations := Isolations()
	isolations[0] = "keep"

	assert.Equal(t, Isolated, Isolations()[0])
}

func TestIsolationAppliesWhenRunTomlDeclaresSomethingToIsolate(t *testing.T) {
	cases := []struct {
		name   string
		config Run
		want   bool
	}{
		{name: "no_run_toml", config: Run{}, want: false},
		{name: "task_only", config: Run{Jobs: []Job{{Name: "lint", Kind: Task, Cmd: "pnpm lint"}}}, want: false},
		{name: "port", config: Run{Jobs: []Job{{Name: "web", Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}, want: true},
		{name: "compose_stack", config: Run{Jobs: []Job{{Name: "db", Cmd: "docker compose up -d db"}}}, want: true},
		{name: "compose_stop", config: Run{Jobs: []Job{{Name: "db", Cmd: "./up.sh", Stop: "docker-compose down"}}}, want: true},
		{name: "namespace", config: Run{Jobs: []Job{{Name: "pg", Scope: Shared, Namespace: &Namespace{Name: "{worktree}", Create: "createdb {namespace}"}}}}, want: true},
		{name: "namespace_creates_nothing", config: Run{Jobs: []Job{{Name: "pg", Namespace: &Namespace{Name: "{worktree}"}}}}, want: false},
		{name: "env_port_link", config: Run{EnvPorts: []EnvPortLink{{File: ".env", Key: "API_URL", Job: "api", Port: "PORT"}}}, want: true},
		{name: "env_value_link", config: Run{EnvValues: []EnvValueLink{{File: ".env", Key: "REALM", Job: "kc", Value: "{worktree}"}}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.config.IsolationApplies())
		})
	}
}
