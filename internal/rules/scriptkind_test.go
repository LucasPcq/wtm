package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Un kind corrigé à la main dans run.toml est la vérité du fichier : le
// reproposer d'après le nom du script ferait mentir l'écran.
func TestProposedScriptKindPrefereLeJobDeclare(t *testing.T) {
	script := domain.PackageScript{Name: "preview"}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "preview",
		Kind: domain.JobKindService,
		Cmd:  rules.ScriptJobCmd(domain.PkgManagerPnpm, "preview"),
		Cwd:  rules.ScriptJobCwd(""),
	}}}

	got := rules.ProposedScriptKind(rules.ProposedScriptKindParams{
		Script:         script,
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
	})

	if got != domain.JobKindService {
		t.Errorf("kind = %q, want %q", got, domain.JobKindService)
	}
}

// Deux workspaces déclarant "build" sont deux jobs distincts : le cwd sépare
// leurs réponses, sans quoi l'un imposerait son kind à l'autre.
func TestProposedScriptKindSepareLesWorkspaces(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "web-build",
		Kind: domain.JobKindService,
		Cmd:  rules.ScriptJobCmd(domain.PkgManagerPnpm, "build"),
		Cwd:  "apps/web",
	}}}

	got := rules.ProposedScriptKind(rules.ProposedScriptKindParams{
		Script:         domain.PackageScript{Name: "build", Workspace: "apps/api"},
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
	})

	if got != rules.ClassifyScriptKind("build") {
		t.Errorf("kind = %q, want la classification par défaut", got)
	}
}

// Un script que run.toml ne connaît pas retombe sur la classification par
// défaut : c'est un premier init, il n'y a rien d'autre à lire.
func TestProposedScriptKindRetombeSurLaClassification(t *testing.T) {
	got := rules.ProposedScriptKind(rules.ProposedScriptKindParams{
		Script:         domain.PackageScript{Name: "build"},
		PackageManager: domain.PkgManagerPnpm,
	})

	if got != rules.ClassifyScriptKind("build") {
		t.Errorf("kind = %q, want la classification par défaut", got)
	}
}
