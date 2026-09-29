package rules

import (
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
