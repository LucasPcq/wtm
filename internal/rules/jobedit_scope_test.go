package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func sharedJob() domain.JobConfig {
	return domain.JobConfig{
		Name:  "pg",
		Kind:  domain.JobKindService,
		Cmd:   "docker compose up -d pg",
		Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{
			Name:   "app_{worktree}",
			Create: "./db-add.sh",
			Remove: "./db-rm.sh",
			Env:    map[string]string{"DB": "app_{worktree}"},
		},
	}
}

func entries(values ...string) *[]string { return &values }

func TestApplyJobPatchScope(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  domain.JobScope
	}{
		{domain.ScopeValueShared, domain.JobScopeShared},
		{domain.ScopeValuePerWorktree, domain.JobScopePerWorktree},
		{"", domain.JobScopePerWorktree},
	} {
		got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
			Current: domain.JobConfig{Name: "pg", Kind: domain.JobKindService, Cmd: "true", Scope: domain.JobScopeShared},
			Patch:   rules.JobPatch{Scope: ptr(tc.value)},
		})
		if err != nil {
			t.Fatalf("--scope %q: %v", tc.value, err)
		}
		if got.Scope != tc.want {
			t.Errorf("--scope %q: scope = %q, want %q", tc.value, got.Scope, tc.want)
		}
	}
}

func TestApplyJobPatchRefusesAnUnknownScope(t *testing.T) {
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: fullJob(),
		Patch:   rules.JobPatch{Scope: ptr("global")},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagScope) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagScope)
	}
}

func TestApplyJobPatchBuildsANamespaceFromItsFlags(t *testing.T) {
	current := fullJob()
	current.Scope = domain.JobScopeShared
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: current,
		Patch: rules.JobPatch{
			NamespaceName:   ptr("app_{worktree}"),
			NamespaceCreate: ptr("./db-add.sh"),
			NamespaceEnv:    entries("DB=app_{worktree}", "URL=postgres://x/y?a=b"),
		},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	ns := got.Namespace
	if ns == nil || ns.Name != "app_{worktree}" || ns.Create != "./db-add.sh" || ns.Remove != "" {
		t.Fatalf("namespace = %+v, want name and create set", ns)
	}
	if ns.Env["DB"] != "app_{worktree}" || ns.Env["URL"] != "postgres://x/y?a=b" {
		t.Errorf("env = %v, want both entries, the value split on its first =", ns.Env)
	}
}

func TestApplyJobPatchChangesOneNamespaceFieldAndKeepsTheOthers(t *testing.T) {
	current := sharedJob()
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: current,
		Patch:   rules.JobPatch{NamespaceRemove: ptr("")},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	ns := got.Namespace
	if ns == nil || ns.Remove != "" || ns.Create != "./db-add.sh" || ns.Env["DB"] == "" {
		t.Fatalf("namespace = %+v, want only remove dropped", ns)
	}
	if current.Namespace.Remove == "" {
		t.Fatal("the patch mutated the job it was given")
	}
}

func TestApplyJobPatchNamespaceEnvReplacesTheTable(t *testing.T) {
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: sharedJob(),
		Patch:   rules.JobPatch{NamespaceEnv: entries("")},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if got.Namespace == nil || len(got.Namespace.Env) != 0 {
		t.Errorf("namespace = %+v, want the env table dropped", got.Namespace)
	}
}

func TestApplyJobPatchRefusesAMalformedNamespaceEnv(t *testing.T) {
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: sharedJob(),
		Patch:   rules.JobPatch{NamespaceEnv: entries("DB")},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagNamespaceEnv) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagNamespaceEnv)
	}
}

func TestApplyJobPatchEmptyNamespaceNameWithdrawsTheBlock(t *testing.T) {
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: sharedJob(),
		Patch:   rules.JobPatch{NamespaceName: ptr("")},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if got.Namespace != nil {
		t.Errorf("namespace = %+v, want the block withdrawn", got.Namespace)
	}
	if got.Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want the service still shared", got.Scope)
	}
}

func TestApplyJobPatchRefusesWithdrawingANamespaceWhileSettingIt(t *testing.T) {
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: sharedJob(),
		Patch:   rules.JobPatch{NamespaceName: ptr(""), NamespaceCreate: ptr("./x.sh")},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagNamespaceName) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagNamespaceName)
	}
}

func TestApplyJobPatchRefusesBindsNoPortOnATask(t *testing.T) {
	yes := true
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: domain.JobConfig{Name: "reset", Kind: domain.JobKindTask, Cmd: "pnpm reset"},
		Patch:   rules.JobPatch{BindsNoPort: &yes},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagBindsNoPort) {
		t.Fatalf("err = %v, want one naming --%s", err, domain.FlagBindsNoPort)
	}
}

func TestJobPatchWithOnlyScopeOrNamespaceIsNotEmpty(t *testing.T) {
	for name, patch := range map[string]rules.JobPatch{
		"scope":            {Scope: ptr("")},
		"namespace-name":   {NamespaceName: ptr("")},
		"namespace-create": {NamespaceCreate: ptr("x")},
		"namespace-remove": {NamespaceRemove: ptr("")},
		"namespace-env":    {NamespaceEnv: entries("")},
	} {
		if patch.Empty() {
			t.Errorf("a patch carrying only --%s reads as empty", name)
		}
	}
}
