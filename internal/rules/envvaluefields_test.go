package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func keycloakEnvLines() map[string][]domain.EnvLine {
	return map[string][]domain.EnvLine{
		"apps/web/.env": {
			{Kind: domain.EnvLinePair, Key: "KEYCLOAK_URL", Value: "http://localhost:8080"},
			{Kind: domain.EnvLinePair, Key: "KEYCLOAK_REALM", Value: "myapp"},
			{Kind: domain.EnvLineComment},
			{Kind: domain.EnvLinePair, Key: "SENTRY_DSN", Value: "https://example"},
		},
	}
}

func keycloakFieldsParams() EnvValueFieldsParams {
	return EnvValueFieldsParams{
		Shared: []domain.SharedComposeService{{
			File: "docker-compose.yml", Service: "keycloak",
			Namespace: &domain.JobNamespaceConfig{Name: "realm_{worktree}", Create: "true"},
		}},
		Lines: keycloakEnvLines(),
		Files: []domain.EnvFile{{Target: "apps/web/.env"}},
		Ports: map[string][]string{"keycloak": {"KEYCLOAK_PORT"}},
		Bases: map[string][]int{"keycloak": {8080}},
	}
}

func fieldFor(t *testing.T, fields []domain.EnvValueField, key string) domain.EnvValueField {
	t.Helper()
	for _, field := range fields {
		if field.Key == key {
			return field
		}
	}
	t.Fatalf("%s missing from the rows", key)
	return domain.EnvValueField{}
}

// Every managed key is offered, not a filtered subset: wtm cannot recognize a
// realm name, so hiding a key would hide the only one the reader wanted.
func TestEnvValueFieldsOffersEveryManagedKey(t *testing.T) {
	fields := EnvValueFields(keycloakFieldsParams())

	if len(fields) != 3 {
		t.Fatalf("got %d rows, want one per managed key (3)", len(fields))
	}
	fieldFor(t, fields, "SENTRY_DSN")
}

// The one deduction wtm makes is from the job's own name, which the user chose.
// KEYCLOAK_REALM is pre-checked beside a job called keycloak; SENTRY_DSN is not.
func TestEnvValueFieldsPrechecksByTheJobsOwnName(t *testing.T) {
	fields := EnvValueFields(keycloakFieldsParams())

	if !fieldFor(t, fields, "KEYCLOAK_REALM").Linked {
		t.Error("KEYCLOAK_REALM: not linked, want it pre-checked by name affinity")
	}
	if fieldFor(t, fields, "SENTRY_DSN").Linked {
		t.Error("SENTRY_DSN: linked, want it left alone")
	}
	// The name affinity would claim it — it starts with KEYCLOAK_ — but its
	// value carries the port the service binds, so it is the address.
	if fieldFor(t, fields, "KEYCLOAK_URL").Linked {
		t.Error("KEYCLOAK_URL: linked, want the address left to [[env_port]]")
	}
	if got := fieldFor(t, fields, "KEYCLOAK_REALM").Value; got != "{namespace}" {
		t.Errorf("value = %q, want the one proposal wtm makes", got)
	}
}

// run.toml outranks the deduction, both ways: a key it links is checked whatever
// its name, and a key it deliberately does not link stays unchecked.
func TestEnvValueFieldsReadTheExistingConfigFirst(t *testing.T) {
	params := keycloakFieldsParams()
	params.Existing = domain.RunConfig{EnvValues: []domain.EnvValueLink{
		{File: "apps/web/.env", Key: "SENTRY_DSN", Job: "keycloak", Value: "{origin}/{namespace}"},
	}}

	fields := EnvValueFields(params)

	sentry := fieldFor(t, fields, "SENTRY_DSN")
	if !sentry.Linked || sentry.Value != "{origin}/{namespace}" {
		t.Errorf("SENTRY_DSN = %+v, want the link run.toml already holds", sentry)
	}
}

// A shared service that carves nothing out has no slice for a key to name, so
// the step has nothing to ask and skips.
func TestEnvValueFieldsSkipAServiceWithoutANamespace(t *testing.T) {
	params := keycloakFieldsParams()
	params.Shared[0].Namespace = nil

	if fields := EnvValueFields(params); len(fields) != 0 {
		t.Errorf("got %d rows, want none", len(fields))
	}
}

// The whole point of the step, folded back: the URL of the shared instance stays
// an [[env_port]] concern, the realm becomes an [[env]] link.
func TestEnvValuesFromFieldsKeepsOnlyLinkedRows(t *testing.T) {
	fields := EnvValueFields(keycloakFieldsParams())

	links := EnvValuesFromFields(fields)
	if len(links) != 1 {
		t.Fatalf("got %d links, want only the realm (%+v)", len(links), links)
	}
	if links[0].Key != "KEYCLOAK_REALM" || links[0].Job != "keycloak" || links[0].Value != "{namespace}" {
		t.Errorf("link = %+v, want the realm following keycloak's namespace", links[0])
	}
}
