package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestClassifyScriptKind(t *testing.T) {
	cases := []struct {
		name string
		want domain.JobKind
	}{
		{"dev", domain.JobKindService},
		{"start", domain.JobKindService},
		{"serve", domain.JobKindService},
		{"watch", domain.JobKindService},
		{"dev:api", domain.JobKindService},
		{"start:web", domain.JobKindService},
		{"serve:storybook", domain.JobKindService},
		{"watch:types", domain.JobKindService},
		{"api:dev", domain.JobKindService},
		{"web:start", domain.JobKindService},
		{"app:serve", domain.JobKindService},
		{"ts:watch", domain.JobKindService},
		{"build", domain.JobKindTask},
		{"test", domain.JobKindTask},
		{"lint", domain.JobKindTask},
		{"typecheck", domain.JobKindTask},
		{"deploy:prod", domain.JobKindTask},
		{"predev", domain.JobKindTask},
		{"startup", domain.JobKindTask},
		{"fresh-start", domain.JobKindTask},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyScriptKind(tc.name)
			if got != tc.want {
				t.Errorf("ClassifyScriptKind(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestPreselectScript(t *testing.T) {
	tests := []struct {
		script string
		want   bool
	}{
		{"dev", true},
		{"dev:api", true},
		{"api:dev", true},
		{"web-dev", true},
		{"start", false},
		{"serve", false},
		{"watch", false},
		{"preview", false},
		{"build", false},
		{"lint", false},
		{"format", false},
		{"check-types", false},
	}

	for _, tt := range tests {
		t.Run(tt.script, func(t *testing.T) {
			if got := PreselectScript(PreselectScriptParams{Script: domain.PackageScript{Name: tt.script}}); got != tt.want {
				t.Errorf("PreselectScript(%q) = %v, want %v", tt.script, got, tt.want)
			}
		})
	}
}

func TestPreselectScriptIsIndependentOfKind(t *testing.T) {
	// `start` est classé service — il tourne — mais on ne le coche pas : ce
	// n'est pas ce qu'on lance en dev. Les deux axes ne doivent pas se suivre.
	if ClassifyScriptKind("start") != domain.JobKindService {
		t.Fatal("le test suppose que start reste classé service")
	}
	if PreselectScript(PreselectScriptParams{Script: domain.PackageScript{Name: "start"}}) {
		t.Error("start ne doit pas être coché par défaut")
	}
}

func TestPreselectScriptLeavesAnOrchestratorUnchecked(t *testing.T) {
	// Un `dev` racine que des packages déclarent aussi est un orchestrateur :
	// le cocher démarrerait deux fois chaque package, sur les mêmes ports.
	all := []domain.PackageScript{
		{Name: "dev"},
		{Name: "dev", Workspace: "apps/api", PkgName: "api"},
		{Name: "dev", Workspace: "apps/web", PkgName: "web"},
	}

	if PreselectScript(PreselectScriptParams{Script: all[0], All: all}) {
		t.Error("le dev racine ne doit pas être coché quand des packages en ont un")
	}
	for _, script := range all[1:] {
		if !PreselectScript(PreselectScriptParams{Script: script, All: all}) {
			t.Errorf("%s doit rester coché", script.Workspace)
		}
	}
}

func TestPreselectScriptKeepsALoneRootDev(t *testing.T) {
	// Sans package concurrent, le dev racine est le seul serveur du repo.
	all := []domain.PackageScript{{Name: "dev"}, {Name: "build"}}
	if !PreselectScript(PreselectScriptParams{Script: all[0], All: all}) {
		t.Error("un dev racine sans concurrent doit rester coché")
	}
}
