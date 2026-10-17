package env

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAValueResolvesTheClosedVocabulary(t *testing.T) {
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
			got, err := keycloakWorktree(sharedKeycloakJob()).expand(keycloakValue(tc.value))

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAValueShiftsAPortOnlyWhenTheJobIsNotShared(t *testing.T) {
	job := sharedKeycloakJob()
	job.Scope = project.PerWorktree

	got, err := keycloakWorktree(job).expand(keycloakValue("{port.KEYCLOAK_PORT}"))

	require.NoError(t, err)
	assert.Equal(t, "8100", got)
}

func TestAValueRefusesAnUnknownPlaceholder(t *testing.T) {
	_, err := keycloakWorktree(sharedKeycloakJob()).expand(keycloakValue("http://host/{realm}"))

	assert.EqualError(t, err, "env K in .env: unknown placeholder {realm}")
}

func TestAValueRefusesAPortTheJobDoesNotDeclare(t *testing.T) {
	_, err := keycloakWorktree(sharedKeycloakJob()).expand(keycloakValue("{port.NOPE}"))

	assert.EqualError(t, err, `env K in .env: job "keycloak" declares no port named "NOPE"`)
}

func TestAValueRefusesAPortPlaceholderNeverClosed(t *testing.T) {
	_, err := keycloakWorktree(sharedKeycloakJob()).expand(keycloakValue("{port.KEYCLOAK_PORT"))

	assert.EqualError(t, err, "env K in .env: a {port.…} placeholder is never closed")
}

func TestAValueRefusesANamespaceOnAJobWithoutOne(t *testing.T) {
	job := sharedKeycloakJob()
	job.Namespace = nil

	_, err := keycloakWorktree(job).expand(keycloakValue("{namespace}"))

	assert.EqualError(t, err, `env K in .env: job "keycloak" declares no [job.namespace], so {namespace} means nothing`)
}

func TestAValueRefusesAnOriginOfAJobWithoutAddress(t *testing.T) {
	job := sharedKeycloakJob()
	job.URL = nil

	_, err := keycloakWorktree(job).expand(keycloakValue("{origin}/realms/{namespace}"))

	assert.EqualError(t, err, `env K in .env: job "keycloak" publishes no address, so {origin} has no answer — publish a [job.url] for it, or write the host yourself`)
}

func TestAValueRefusesAnOriginWhileTheRunProxyServesNoPort(t *testing.T) {
	settled := keycloakWorktree(sharedKeycloakJob())
	settled.PublicPort = 0

	_, err := settled.expand(keycloakValue("{origin}/realms/{namespace}"))

	assert.ErrorIs(t, err, errNoProxy)
	assert.EqualError(t, err, `env K in .env: {origin} of job "keycloak" is served by the run proxy, which serves no port: enable the proxy, or write the host yourself`)
}

func TestAValueOnAJobRunTomlDoesNotDeclareIsRefused(t *testing.T) {
	_, err := settling{}.expand(project.EnvValueLink{File: ".env", Key: "REALM", Job: "ghost", Value: "{namespace}"})

	assert.EqualError(t, err, `env REALM in .env: no job named "ghost"`)
}

func TestEachValueLinkBecomesAKeyWtmOwns(t *testing.T) {
	links := []project.EnvValueLink{{File: ".env", Key: "REALM", Job: "keycloak", Value: "{namespace}"}, {File: "apps/api/.env", Key: "KC_PORT", Job: "keycloak", Value: "{port.KEYCLOAK_PORT}"}}

	writes, err := keycloakWorktree(sharedKeycloakJob()).valueWrites(links)

	require.NoError(t, err)
	assert.Equal(t, []OwnedEntry{{File: ".env", Key: "REALM", Value: "realm_feat-a"}, {File: "apps/api/.env", Key: "KC_PORT", Value: "8080"}}, writes)
}

func TestANamespaceNameTakesTheWorktreeAndTheOrdinal(t *testing.T) {
	name, err := settling{Slug: "feat_x", Ordinal: 3}.namespace(project.Namespace{Name: "crm_{worktree}_slot{ordinal}"})

	require.NoError(t, err)
	assert.Equal(t, "crm_feat_x_slot3", name)
}

func TestANamespaceNameLeavesShellVariablesAlone(t *testing.T) {
	name, err := settling{Slug: "feat_x"}.namespace(project.Namespace{Name: "db_$USER"})

	require.NoError(t, err)
	assert.Equal(t, "db_$USER", name)
}

func TestANamespaceRefusesAnUnknownPlaceholderInItsNameOrItsEnv(t *testing.T) {
	for _, namespace := range []project.Namespace{{Name: "app_{branch}"}, {Name: "realm_{worktree}", Env: map[string]string{"REALM": "{branch}"}}} {
		_, err := settling{Slug: "feat_x"}.namespace(namespace)

		assert.ErrorIs(t, err, errNamespaceUnknownToken)
		assert.ErrorContains(t, err, "{branch}")
	}
}
