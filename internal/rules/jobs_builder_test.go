package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestBuildScriptJobsRoot(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Cmd: "vite", Workspace: "", PkgName: "app"},
		{Name: "build", Cmd: "vite build", Workspace: "", PkgName: "app"},
		{Name: "test", Cmd: "vitest", Workspace: "", PkgName: "app"},
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        scripts,
	})

	if len(cfg.Jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(cfg.Jobs))
	}

	devJob := cfg.Jobs[0]
	if devJob.Name != "dev" {
		t.Errorf("expected name=dev, got %q", devJob.Name)
	}
	if devJob.Kind != domain.JobKindService {
		t.Errorf("expected kind=service, got %q", devJob.Kind)
	}
	if devJob.Cmd != "pnpm run dev" {
		t.Errorf("expected cmd=pnpm run dev, got %q", devJob.Cmd)
	}
	if devJob.Cwd != "." {
		t.Errorf("expected cwd=., got %q", devJob.Cwd)
	}
	if devJob.Stop != "" {
		t.Error("expected no stop command on PID-tracked service")
	}

	buildJob := cfg.Jobs[1]
	if buildJob.Kind != domain.JobKindTask {
		t.Errorf("expected build kind=task, got %q", buildJob.Kind)
	}
}

func TestBuildScriptJobsWorkspace(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Cmd: "next dev", Workspace: "apps/web", PkgName: "web"},
		{Name: "dev", Cmd: "tsx watch src", Workspace: "apps/api", PkgName: "api"},
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerNpm,
		Scripts:        scripts,
	})

	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(cfg.Jobs))
	}

	if cfg.Jobs[0].Name != "web-dev" {
		t.Errorf("expected web-dev, got %q", cfg.Jobs[0].Name)
	}
	if cfg.Jobs[0].Cwd != "apps/web" {
		t.Errorf("expected cwd=apps/web, got %q", cfg.Jobs[0].Cwd)
	}
	if cfg.Jobs[0].Cmd != "npm run dev" {
		t.Errorf("expected npm run dev, got %q", cfg.Jobs[0].Cmd)
	}
	if cfg.Jobs[1].Name != "api-dev" {
		t.Errorf("expected api-dev, got %q", cfg.Jobs[1].Name)
	}
}

func TestBuildScriptJobsNameDedup(t *testing.T) {
	scripts := []domain.PackageScript{
		{Name: "dev", Workspace: "", PkgName: "a"},
		{Name: "dev", Workspace: "", PkgName: "b"},
	}
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerYarn,
		Scripts:        scripts,
	})

	if cfg.Jobs[0].Name == cfg.Jobs[1].Name {
		t.Errorf("expected deduped names, both are %q", cfg.Jobs[0].Name)
	}
}

func TestBuildDockerJobsKind(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{ComposeCmd: "docker compose", Files: []string{"docker-compose.yml"}})
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	j := cfg.Jobs[0]
	if j.Kind != domain.JobKindService {
		t.Errorf("expected Kind=service, got %q", j.Kind)
	}
	if j.Stop == "" {
		t.Error("expected stop command on docker job")
	}
	if !IsDetached(j) {
		t.Error("expected docker job to be detached")
	}
}

func TestBuildScriptJobsHonoursAnExplicitKind(t *testing.T) {
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts: []domain.PackageScript{
			// `preview` sert des requêtes : classé task par son nom, il bloquerait
			// le profil pour toujours.
			{Name: "preview", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindService},
		},
	})

	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}
	if cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("kind = %s, want service", cfg.Jobs[0].Kind)
	}
}

func TestBuildScriptJobsFallsBackToTheNameWhenKindIsUnset(t *testing.T) {
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{{Name: "dev", PkgName: "root"}},
	})

	if cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("kind = %s, want service from the name", cfg.Jobs[0].Kind)
	}
}

// Two products holding a package of the same name gave `admin-dev` and
// `admin-dev-2`: a suffix that says nothing, on whichever happened to be
// second. The directory above the package is what tells them apart.
func TestScriptJobsNameCollisionsByTheirProduct(t *testing.T) {
	cfg := BuildScriptJobs(BuildScriptJobsParams{
		PackageManager: domain.PkgManagerPnpm,
		Scripts: []domain.PackageScript{
			{Name: "dev", PkgName: "admin", Workspace: "apps/crm/admin", Kind: domain.JobKindService},
			{Name: "dev", PkgName: "admin", Workspace: "apps/shop/admin", Kind: domain.JobKindService},
			{Name: "dev", PkgName: "api", Workspace: "apps/crm/api", Kind: domain.JobKindService},
		},
	})

	var names []string
	for _, job := range cfg.Jobs {
		names = append(names, job.Name)
	}

	want := []string{"crm-admin-dev", "shop-admin-dev", "api-dev"}
	for i, expected := range want {
		if names[i] != expected {
			t.Errorf("names = %v, want %v", names, want)
			break
		}
	}
	for _, name := range names {
		if strings.HasSuffix(name, "-2") {
			t.Errorf("names = %v, want no positional suffix left", names)
		}
	}
}
