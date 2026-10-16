package runenv

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAValueResolvesTheClosedVocabulary(t *testing.T) {
	base := expandValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a", Ordinal: 2, Offset: 20, Shared: true, Origin: "http://keycloak.demo.localhost"}
	cases := []struct{ value, want string }{
		{"{namespace}", "realm_feat-a"},
		{"{worktree}", "feat-a"},
		{"{ordinal}", "2"},
		{"{origin}", "http://keycloak.demo.localhost"},
		{"{origin}/realms/{namespace}", "http://keycloak.demo.localhost/realms/realm_feat-a"},
		{"{port.KEYCLOAK_PORT}", "8080"},
		{"nothing to expand", "nothing to expand"},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			got, err := expandFor(t, tc.value, base)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAValueShiftsAPortOnlyWhenTheJobIsNotShared(t *testing.T) {
	job := sharedKeycloakJob()
	job.Scope = run.PerWorktree

	got, err := expandFor(t, "{port.KEYCLOAK_PORT}", expandValueParams{Job: job, Worktree: "feat-a", Offset: 20})

	require.NoError(t, err)
	assert.Equal(t, "8100", got)
}

func TestAValueRefusesAnUnknownPlaceholder(t *testing.T) {
	_, err := expandFor(t, "http://host/{realm}", expandValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "{realm}")
}

func TestAValueRefusesAPortTheJobDoesNotDeclare(t *testing.T) {
	_, err := expandFor(t, "{port.NOPE}", expandValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE")
}

func TestAValueRefusesAPortPlaceholderNeverClosed(t *testing.T) {
	_, err := expandFor(t, "{port.KEYCLOAK_PORT", expandValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})

	assert.EqualError(t, err, "env K in .env: a {port.…} placeholder is never closed")
}

func TestAValueRefusesANamespaceOnAJobWithoutOne(t *testing.T) {
	job := sharedKeycloakJob()
	job.Namespace = nil

	_, err := expandFor(t, "{namespace}", expandValueParams{Job: job, Worktree: "feat-a"})

	assert.EqualError(t, err, `env K in .env: job "keycloak" declares no [job.namespace], so {namespace} means nothing`)
}

func TestAValueRefusesAnOriginNothingServes(t *testing.T) {
	_, err := expandFor(t, "{origin}/realms/{namespace}", expandValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})

	assert.EqualError(t, err, `env K in .env: job "keycloak" publishes no address, so {origin} has no answer — publish a [job.url] for it, or write the host yourself`)
}

func TestEachValueLinkBecomesAKeyWtmOwns(t *testing.T) {
	config := run.Config{
		Jobs:      []run.Job{sharedKeycloakJob()},
		EnvValues: []run.EnvValueLink{{File: ".env", Key: "REALM", Job: "keycloak", Value: "{namespace}"}, {File: "apps/api/.env", Key: "KC_PORT", Job: "keycloak", Value: "{port.KEYCLOAK_PORT}"}},
	}

	writes, err := valueWrites(valueWritesParams{Run: config, Worktree: "feat-a", Ordinal: 2, Offset: 20})

	require.NoError(t, err)
	assert.Equal(t, []OwnedEntry{{File: ".env", Key: "REALM", Value: "realm_feat-a"}, {File: "apps/api/.env", Key: "KC_PORT", Value: "8080"}}, writes)
}

func TestAValueLinkOnAJobRunTomlDoesNotDeclareIsRefused(t *testing.T) {
	config := run.Config{EnvValues: []run.EnvValueLink{{File: ".env", Key: "REALM", Job: "ghost", Value: "{namespace}"}}}

	_, err := valueWrites(valueWritesParams{Run: config, Worktree: "feat-a"})

	assert.EqualError(t, err, `env REALM in .env: no job named "ghost"`)
}

func TestARunWithoutValueLinksWritesNoValue(t *testing.T) {
	writes, err := valueWrites(valueWritesParams{Run: run.Config{Jobs: []run.Job{sharedKeycloakJob()}}, Worktree: "feat-a"})

	require.NoError(t, err)
	assert.Empty(t, writes)
}

func TestANamespaceNameTakesTheWorktree(t *testing.T) {
	assert.Equal(t, "crm_feat_x", expandedNamespace(t, namespaceParams{Namespace: run.Namespace{Name: "crm_{worktree}"}, Worktree: "feat_x", Ordinal: 2}))
}

func TestANamespaceNameTakesTheOrdinal(t *testing.T) {
	assert.Equal(t, "slot3", expandedNamespace(t, namespaceParams{Namespace: run.Namespace{Name: "slot{ordinal}"}, Worktree: "feat_x", Ordinal: 3}))
}

func TestANamespaceNameRefusesAnUnknownPlaceholder(t *testing.T) {
	_, err := expandNamespace(namespaceParams{Namespace: run.Namespace{Name: "app_{branch}"}, Worktree: "feat_x"})

	assert.ErrorIs(t, err, errNamespaceUnknownToken)
	assert.ErrorContains(t, err, "{branch}")
}

func TestANamespaceNameLeavesShellVariablesAlone(t *testing.T) {
	assert.Equal(t, "db_$USER", expandedNamespace(t, namespaceParams{Namespace: run.Namespace{Name: "db_$USER"}, Worktree: "feat_x"}))
}

func TestANamespaceRefusesAnUnknownPlaceholderInItsEnv(t *testing.T) {
	_, err := expandNamespace(namespaceParams{Namespace: run.Namespace{Name: "realm_{worktree}", Env: map[string]string{"REALM": "{branch}"}}, Worktree: "feat-x"})

	assert.ErrorIs(t, err, errNamespaceUnknownToken)
	assert.ErrorContains(t, err, "{branch}")
}
