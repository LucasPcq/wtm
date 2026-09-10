package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sharedKeycloakJob() domain.JobConfig {
	return domain.JobConfig{
		Name:  "keycloak",
		Scope: domain.JobScopeShared,
		Ports: map[string]int{"KEYCLOAK_PORT": 8080},
		URL:   &domain.JobURLConfig{Port: "KEYCLOAK_PORT"},
		Namespace: &domain.JobNamespaceConfig{
			Name: "realm_{worktree}", Create: "true",
		},
	}
}

func expandFor(t *testing.T, value string, params ExpandEnvValueParams) (string, error) {
	t.Helper()
	params.Link = domain.EnvValueLink{File: ".env", Key: "K", Job: params.Job.Name, Value: value}
	return ExpandEnvValue(params)
}

// The whole vocabulary, on the case that motivated the table: a realm per
// worktree behind one shared keycloak.
func TestExpandEnvValueResolvesTheClosedVocabulary(t *testing.T) {
	base := ExpandEnvValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a", Ordinal: 2, Offset: 20, Shared: true, Origin: "http://keycloak.demo.localhost"}

	for _, tc := range []struct{ value, want string }{
		{"{namespace}", "realm_feat-a"},
		{"{worktree}", "feat-a"},
		{"{ordinal}", "2"},
		{"{origin}", "http://keycloak.demo.localhost"},
		{"{origin}/realms/{namespace}", "http://keycloak.demo.localhost/realms/realm_feat-a"},
		{"{port.KEYCLOAK_PORT}", "8080"},
		{"nothing to expand", "nothing to expand"},
	} {
		got, err := expandFor(t, tc.value, base)
		if err != nil {
			t.Errorf("%s: %v", tc.value, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// A shared service binds its declared port in every worktree, so a value
// addressing it must not be shifted. That rule already exists for [[env_port]]
// and this table must not grow a second answer to it.
func TestExpandEnvValueShiftsAPortOnlyWhenTheJobIsNotShared(t *testing.T) {
	job := sharedKeycloakJob()
	job.Scope = ""

	got, err := expandFor(t, "{port.KEYCLOAK_PORT}", ExpandEnvValueParams{Job: job, Worktree: "feat-a", Offset: 20})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if got != "8100" {
		t.Errorf("port = %s, want the base shifted by the offset (8100)", got)
	}
}

// An unknown placeholder is refused rather than written: braces reaching a .env
// as literal text is the failure this whole vocabulary exists to prevent.
func TestExpandEnvValueRefusesAnUnknownPlaceholder(t *testing.T) {
	_, err := expandFor(t, "http://host/{realm}", ExpandEnvValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})
	if err == nil {
		t.Fatal("expand: no error, want the unknown placeholder refused")
	}
	if !strings.Contains(err.Error(), "{realm}") {
		t.Errorf("error = %q, want it to name {realm}", err)
	}
}

// A port the job does not declare is the same class of mistake, and is named the
// same way.
func TestExpandEnvValueRefusesAPortTheJobDoesNotDeclare(t *testing.T) {
	_, err := expandFor(t, "{port.NOPE}", ExpandEnvValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})
	if err == nil {
		t.Fatal("expand: no error, want the undeclared port refused")
	}
	if !strings.Contains(err.Error(), "NOPE") {
		t.Errorf("error = %q, want it to name NOPE", err)
	}
}

// {namespace} on a job that carves nothing out has no answer, and guessing one
// would write an empty database name into a URL.
func TestExpandEnvValueRefusesANamespaceOnAJobWithoutOne(t *testing.T) {
	job := sharedKeycloakJob()
	job.Namespace = nil

	_, err := expandFor(t, "{namespace}", ExpandEnvValueParams{Job: job, Worktree: "feat-a"})
	if err == nil {
		t.Fatal("expand: no error, want the missing namespace refused")
	}
}

// An origin nothing serves is empty, and writing "/realms/x" into a .env is
// worse than refusing: the value looks settled and addresses nothing.
func TestExpandEnvValueRefusesAnOriginNothingServes(t *testing.T) {
	_, err := expandFor(t, "{origin}/realms/{namespace}", ExpandEnvValueParams{Job: sharedKeycloakJob(), Worktree: "feat-a"})
	if err == nil {
		t.Fatal("expand: no error, want the absent origin refused")
	}
}

func linkConfig(values []domain.EnvValueLink, ports []domain.EnvPortLink) domain.RunConfig {
	return domain.RunConfig{
		Jobs:      []domain.JobConfig{sharedKeycloakJob()},
		EnvValues: values,
		EnvPorts:  ports,
	}
}

// The two tables are not complementary on one key: an [[env]] value writes its
// own port when it needs one, so a key both claim is a line to delete rather
// than a merge order to invent.
func TestValidateRefusesAKeyWrittenByBothTables(t *testing.T) {
	_, errs := ValidateRun(linkConfig(
		[]domain.EnvValueLink{{File: ".env", Key: "KEYCLOAK_URL", Job: "keycloak", Value: "{origin}"}},
		[]domain.EnvPortLink{{File: ".env", Key: "KEYCLOAK_URL", Job: "keycloak", Port: "KEYCLOAK_PORT"}},
	))

	if !strings.Contains(strings.Join(errs, "\n"), "both an [[env]] link and an [[env_port]] link") {
		t.Errorf("errors = %v, want the double claim refused", errs)
	}
}

// A placeholder nobody defined is caught when run.toml is read, not the first
// time a worktree is created — the config is wrong for every worktree at once.
func TestValidateRefusesAnUnknownPlaceholderAtLoad(t *testing.T) {
	_, errs := ValidateRun(linkConfig(
		[]domain.EnvValueLink{{File: ".env", Key: "K", Job: "keycloak", Value: "http://host/{realm}"}},
		nil,
	))

	if !strings.Contains(strings.Join(errs, "\n"), "{realm}") {
		t.Errorf("errors = %v, want {realm} named", errs)
	}
}

// A link naming a job that does not exist is the same class, and names the job.
func TestValidateRefusesALinkOnAnUnknownJob(t *testing.T) {
	_, errs := ValidateRun(linkConfig(
		[]domain.EnvValueLink{{File: ".env", Key: "K", Job: "nope", Value: "{worktree}"}},
		nil,
	))

	if !strings.Contains(strings.Join(errs, "\n"), "nope") {
		t.Errorf("errors = %v, want the unknown job named", errs)
	}
}
