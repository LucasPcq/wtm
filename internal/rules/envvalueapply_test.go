package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func realmLink() domain.EnvValueLink {
	return domain.EnvValueLink{File: "apps/web/.env", Key: "KEYCLOAK_REALM", Job: "keycloak", Value: "{namespace}"}
}

func realmRef() domain.EnvKeyRef {
	return domain.EnvKeyRef{File: "apps/web/.env", Key: "KEYCLOAK_REALM"}
}

// A run that never put the question leaves run.toml exactly as it stands — the
// same (value, asked) pair every other step is read as.
func TestApplyEnvValuesLeavesTheConfigWhenNeverAsked(t *testing.T) {
	cfg := domain.RunConfig{EnvValues: []domain.EnvValueLink{realmLink()}}

	got := ApplyEnvValues(ApplyEnvValuesParams{Config: cfg, Asked: false})

	if len(got.EnvValues) != 1 {
		t.Errorf("got %d links, want the config untouched", len(got.EnvValues))
	}
}

// Asked and emptied withdraws the link, which is what unchecking a row means.
func TestApplyEnvValuesWithdrawsAnEmptiedAnswer(t *testing.T) {
	cfg := domain.RunConfig{EnvValues: []domain.EnvValueLink{realmLink()}}

	got := ApplyEnvValues(ApplyEnvValuesParams{
		Config:  cfg,
		Asked:   true,
		Offered: map[domain.EnvKeyRef]bool{realmRef(): true},
	})

	if len(got.EnvValues) != 0 {
		t.Errorf("got %+v, want the link withdrawn", got.EnvValues)
	}
}

// A step may only remove what it proposed: a link on a file it never showed —
// one .wtm.toml no longer configures — is not an answer to withdraw.
func TestApplyEnvValuesKeepsALinkTheStepNeverOffered(t *testing.T) {
	held := domain.EnvValueLink{File: "apps/api/.env", Key: "DB_NAME", Job: "postgres", Value: "{namespace}"}
	cfg := domain.RunConfig{EnvValues: []domain.EnvValueLink{held}}

	got := ApplyEnvValues(ApplyEnvValuesParams{
		Config:  cfg,
		Asked:   true,
		Offered: map[domain.EnvKeyRef]bool{realmRef(): true},
	})

	if len(got.EnvValues) != 1 || got.EnvValues[0].Key != "DB_NAME" {
		t.Errorf("got %+v, want the untouched link kept", got.EnvValues)
	}
}

// The migration: marking a key as following a namespace takes the [[env_port]]
// off it. Writing both would produce a config wtm refuses to read, which is the
// worst outcome a wizard can have.
func TestApplyEnvValuesTakesThePortLinkOffAKeyItNowWrites(t *testing.T) {
	cfg := domain.RunConfig{EnvPorts: []domain.EnvPortLink{
		{File: "apps/web/.env", Key: "KEYCLOAK_REALM", Job: "keycloak", Port: "KEYCLOAK_PORT"},
		{File: "apps/web/.env", Key: "KEYCLOAK_URL", Job: "keycloak", Port: "KEYCLOAK_PORT"},
	}}

	got := ApplyEnvValues(ApplyEnvValuesParams{
		Config:  cfg,
		Values:  []domain.EnvValueLink{realmLink()},
		Asked:   true,
		Offered: map[domain.EnvKeyRef]bool{realmRef(): true},
	})

	if len(got.EnvPorts) != 1 || got.EnvPorts[0].Key != "KEYCLOAK_URL" {
		t.Fatalf("port links = %+v, want only the address kept", got.EnvPorts)
	}
	// And the config it produced is one wtm will read back.
	if _, errs := ValidateRun(withKeycloakJob(got)); len(errs) > 0 {
		t.Errorf("the written config does not load: %v", errs)
	}
}

func withKeycloakJob(cfg domain.RunConfig) domain.RunConfig {
	cfg.Jobs = []domain.JobConfig{sharedKeycloakJob()}
	return cfg
}
