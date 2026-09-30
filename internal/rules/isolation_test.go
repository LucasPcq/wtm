package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestEffectiveIsolationReadsUnsetAsIsolated(t *testing.T) {
	if got := EffectiveIsolation(""); got != domain.IsolationIsolated {
		t.Errorf("EffectiveIsolation(\"\") = %q, want %q", got, domain.IsolationIsolated)
	}
	if !IsVerbatim(domain.IsolationVerbatim) || IsVerbatim("") {
		t.Error("only an explicit verbatim is verbatim")
	}
}

func TestParseIsolation(t *testing.T) {
	for _, value := range []string{"", "isolated", "verbatim"} {
		if _, err := ParseIsolation(value); err != nil {
			t.Errorf("ParseIsolation(%q): %v", value, err)
		}
	}
	if _, err := ParseIsolation("keep"); err == nil {
		t.Error("ParseIsolation(\"keep\") must be refused")
	}
	if errs := ValidateIsolation(domain.RunConfig{Isolation: "copy"}); len(errs) != 1 {
		t.Errorf("ValidateIsolation = %v, want one refusal", errs)
	}
}

func TestIsolationApplies(t *testing.T) {
	cases := []struct {
		name string
		cfg  domain.RunConfig
		want bool
	}{
		{name: "no run.toml", cfg: domain.RunConfig{}, want: false},
		{name: "a task and nothing else", cfg: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "lint", Kind: domain.JobKindTask, Cmd: "pnpm lint"}}}, want: false},
		{name: "a port", cfg: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}}}}, want: true},
		{name: "a compose stack", cfg: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d db"}}}, want: true},
		{name: "an [[env]] link", cfg: domain.RunConfig{EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM", Job: "kc", Value: "{worktree}"}}}, want: true},
	}
	for _, tc := range cases {
		if got := IsolationApplies(tc.cfg); got != tc.want {
			t.Errorf("%s: IsolationApplies = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsolationAdoptionPending(t *testing.T) {
	compose := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db", Cmd: "docker compose up -d"}}}
	cases := []struct {
		name     string
		params   IsolationAdoptionPendingParams
		expected bool
	}{
		{"a worktree that never chose", IsolationAdoptionPendingParams{Config: compose}, true},
		{"a worktree that chose", IsolationAdoptionPendingParams{Config: compose, Recorded: domain.IsolationIsolated}, false},
		{"the main checkout", IsolationAdoptionPendingParams{Config: compose, IsMain: true}, false},
		{"nothing to isolate", IsolationAdoptionPendingParams{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsolationAdoptionPending(tc.params); got != tc.expected {
				t.Errorf("IsolationAdoptionPending = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestDefaultComposeProjectNameFollowsDocker(t *testing.T) {
	if got := DefaultComposeProjectName("Feat.Old_1-x"); got != "featold_1-x" {
		t.Errorf("DefaultComposeProjectName = %q, want %q", got, "featold_1-x")
	}
}

func TestIsolationAdoptOptionLabelNamesBothProjects(t *testing.T) {
	label := IsolationAdoptOptionLabel(domain.IsolationAdoptionPlan{Pending: true, ComposeProject: "repo-feat-x", CurrentComposeProject: "repo"})
	if !strings.Contains(label, "repo-feat-x") || !strings.Contains(label, "(repo_*)") {
		t.Errorf("label = %q, want the new project and the volumes left behind", label)
	}
}

func TestIsolationIgnoredWarningOnlyWhenTheFlagWasOverruled(t *testing.T) {
	if got := IsolationIgnoredWarning(IsolationIgnoredParams{Branch: "feat/x", Current: domain.IsolationIsolated}); got != "" {
		t.Errorf("no flag: warning = %q, want none", got)
	}
	if got := IsolationIgnoredWarning(IsolationIgnoredParams{Branch: "feat/x", Requested: domain.IsolationIsolated, Current: domain.IsolationIsolated}); got != "" {
		t.Errorf("flag matching the worktree: warning = %q, want none", got)
	}
	got := IsolationIgnoredWarning(IsolationIgnoredParams{Branch: "feat/x", Requested: domain.IsolationVerbatim, Current: domain.IsolationIsolated})
	for _, want := range []string{"--isolation verbatim", "feat/x already exists", "isolated", "wtm env feat/x --isolation verbatim"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning = %q, want it to contain %q", got, want)
		}
	}
}
