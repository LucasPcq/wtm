package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func scriptJob(name, script, workspace string) domain.JobConfig {
	return domain.JobConfig{
		Name: name,
		Cmd:  rules.ScriptJobCmd(domain.PkgManagerPnpm, script),
		Cwd:  rules.ScriptJobCwd(workspace),
	}
}

// Un script détecté, configuré, puis décoché : son job doit partir. C'est la
// seule famille que le wizard a le droit de retirer, parce que c'est la seule
// qu'il a lui-même proposée.
func TestDeselectedJobsNommeLeJobDunScriptDecoche(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		scriptJob("dev", "dev", ""),
		scriptJob("build", "build", ""),
	}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:           true,
		Existing:        cfg,
		PackageManager:  domain.PkgManagerPnpm,
		DetectedScripts: []domain.PackageScript{{Name: "dev"}, {Name: "build"}},
		SelectedScripts: []domain.PackageScript{{Name: "dev"}},
	})

	if len(got) != 1 || got[0] != "build" {
		t.Errorf("jobs à retirer = %v, want [build]", got)
	}
}

// Un job que le wizard n'a jamais proposé — ajouté par `run job add` — ne
// figure dans aucune liste affichée, donc rien ne l'a décoché.
func TestDeselectedJobsIgnoreUnJobEcritALaMain(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "tunnel", Cmd: "cloudflared tunnel run"},
	}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:           true,
		Existing:        cfg,
		PackageManager:  domain.PkgManagerPnpm,
		DetectedScripts: []domain.PackageScript{{Name: "dev"}},
	})

	if len(got) != 0 {
		t.Errorf("jobs à retirer = %v, want aucun", got)
	}
}

// Deux workspaces déclarant "dev" sont deux jobs : décocher l'un ne retire
// pas l'autre.
func TestDeselectedJobsSepareLesWorkspaces(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		scriptJob("web-dev", "dev", "apps/web"),
		scriptJob("api-dev", "dev", "apps/api"),
	}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:          true,
		Existing:       cfg,
		PackageManager: domain.PkgManagerPnpm,
		DetectedScripts: []domain.PackageScript{
			{Name: "dev", Workspace: "apps/web"},
			{Name: "dev", Workspace: "apps/api"},
		},
		SelectedScripts: []domain.PackageScript{{Name: "dev", Workspace: "apps/api"}},
	})

	if len(got) != 1 || got[0] != "web-dev" {
		t.Errorf("jobs à retirer = %v, want [web-dev]", got)
	}
}

// Un job empilant plusieurs fichiers survit tant que l'un d'eux reste coché :
// c'est le même job, qui fait tourner moins de choses.
func TestDeselectedJobsGardeUnJobDontUnFichierResteCoche(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "docker-compose",
		Cmd:  "docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d",
	}}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:                true,
		Existing:             cfg,
		PackageManager:       domain.PkgManagerPnpm,
		DetectedComposeFiles: []string{"docker-compose.yml", "docker-compose.dev.yml"},
		SelectedComposeFiles: []string{"docker-compose.yml"},
	})

	if len(got) != 0 {
		t.Errorf("jobs à retirer = %v, want aucun", got)
	}
}

// Tous ses fichiers décochés, le job compose n'a plus rien à lancer.
func TestDeselectedJobsRetireUnJobComposeEntierementDecoche(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "docker-compose",
		Cmd:  "docker compose -f docker-compose.yml up -d",
	}}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:                true,
		Existing:             cfg,
		PackageManager:       domain.PkgManagerPnpm,
		DetectedComposeFiles: []string{"docker-compose.yml"},
	})

	if len(got) != 1 || got[0] != "docker-compose" {
		t.Errorf("jobs à retirer = %v, want [docker-compose]", got)
	}
}

// Un init non interactif ne sélectionne que ce qu'il aurait pré-coché : lire
// cette absence comme un refus supprimerait les jobs qu'un run précédent avait
// configurés à partir des autres scripts.
func TestDeselectedJobsNeRetireRienQuandRienNaEteDemande(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		scriptJob("dev", "dev", ""),
		scriptJob("build", "build", ""),
	}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Existing:        cfg,
		PackageManager:  domain.PkgManagerPnpm,
		DetectedScripts: []domain.PackageScript{{Name: "dev"}, {Name: "build"}},
		SelectedScripts: []domain.PackageScript{{Name: "dev"}},
	})

	if len(got) != 0 {
		t.Errorf("jobs à retirer = %v, want aucun", got)
	}
}

func TestJobsByCwdNommeLeJobDeChaqueRepertoire(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cwd: "apps/web", Ports: map[string]int{domain.PortNameDefault: 3000}},
		{Name: "api", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"API_PORT": 4000}},
	}}

	got := rules.JobsByCwd(cfg)

	if got["apps/web"] != "web" || got["apps/api"] != "api" {
		t.Errorf("jobs par répertoire = %v", got)
	}
}

// Deux jobs dans le même répertoire ne désignent rien : un lien déduit là
// déplacerait le port de l'un ou de l'autre, sans que rien ne tranche.
func TestJobsByCwdIgnoreUnRepertoirePartage(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cwd: "apps/web", Ports: map[string]int{domain.PortNameDefault: 3000}},
		{Name: "storybook", Kind: domain.JobKindService, Cwd: "apps/web", Ports: map[string]int{"STORYBOOK_PORT": 6006}},
	}}

	if got := rules.JobsByCwd(cfg); len(got) != 0 {
		t.Errorf("jobs par répertoire = %v, want aucun", got)
	}
}

// La racine est nommée "." comme partout ailleurs dans run.toml.
func TestJobsByCwdNormaliseLaRacine(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "root", Kind: domain.JobKindService, Ports: map[string]int{domain.PortNameDefault: 3000}},
	}}

	if got := rules.JobsByCwd(cfg); got["."] != "root" {
		t.Errorf("jobs par répertoire = %v, want la racine sous \".\"", got)
	}
}

// Les tasks d'un package n'écoutent rien : elles ne rendent pas ambigu le
// serveur qui vit à côté d'elles, ce qui est la forme normale d'un monorepo.
func TestJobsByCwdIgnoreLesTasksVoisines(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "api-db:migrate", Kind: domain.JobKindTask, Cwd: "apps/api"},
		{Name: "api-db:seed", Kind: domain.JobKindTask, Cwd: "apps/api"},
		{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{domain.PortNameDefault: 4001}},
	}}

	if got := rules.JobsByCwd(cfg); got["apps/api"] != "api-dev" {
		t.Errorf("jobs par répertoire = %v, want apps/api -> api-dev", got)
	}
}

// The jobs lifted out of a file go with it: they run nothing else, and keeping
// them left a shared service starting a stack nobody selected any more.
func TestDeselectedJobsTakesTheSharedJobsOfAnUncheckedFile(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Scope: domain.JobScopeShared, Cmd: "docker compose -f docker-compose.yml up -d db"},
		{Name: "redis", Scope: domain.JobScopeShared, Cmd: "docker compose -f docker-compose.yml up -d redis"},
	}}

	got := rules.DeselectedJobs(rules.DeselectedJobsParams{
		Asked:                true,
		Existing:             cfg,
		PackageManager:       domain.PkgManagerPnpm,
		DetectedComposeFiles: []string{"docker-compose.yml"},
	})

	if len(got) != 2 {
		t.Errorf("jobs to remove = %v, want both shared jobs", got)
	}
}
