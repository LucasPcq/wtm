package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func scanOf(file string, names ...string) domain.ComposeScan {
	services := make([]domain.ComposeService, 0, len(names))
	for _, name := range names {
		services = append(services, domain.ComposeService{Name: name, Image: name + ":latest"})
	}
	return domain.ComposeScan{File: file, Services: services}
}

func findJob(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job, true
		}
	}
	return domain.JobConfig{}, false
}

// A shared service needs a job of its own: scope lives on a job, and wtm
// generates one job per compose file.
func TestBuildDockerJobsLiftsASharedServiceIntoItsOwnJob(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"docker-compose.yml"},
		Scans:      map[string]domain.ComposeScan{"docker-compose.yml": scanOf("docker-compose.yml", "db", "api", "web")},
		Shared: []domain.SharedComposeService{{
			File: "docker-compose.yml", Service: "db",
			Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true"},
		}},
	})

	shared, ok := findJob(cfg, "db")
	if !ok {
		t.Fatalf("no job for the shared service; jobs = %v", cfg.Jobs)
	}
	if shared.Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want shared", shared.Scope)
	}
	if shared.Namespace == nil || shared.Namespace.Name != "app_{worktree}" {
		t.Errorf("namespace = %+v", shared.Namespace)
	}
	if !strings.HasSuffix(shared.Cmd, "up -d db") {
		t.Errorf("cmd = %q, want it to start only db", shared.Cmd)
	}
	// `down` would take the whole stack with it, including the services that
	// stayed in the file's own job.
	if strings.Contains(shared.Stop, "down") {
		t.Errorf("stop = %q, must not tear the whole file down", shared.Stop)
	}
}

// `docker compose up` starts the whole file, so the file's job has to name the
// services that stayed or it would start the shared one a second time.
func TestBuildDockerJobsNamesTheServicesThatStay(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"docker-compose.yml"},
		Scans:      map[string]domain.ComposeScan{"docker-compose.yml": scanOf("docker-compose.yml", "db", "api", "web")},
		Shared:     []domain.SharedComposeService{{File: "docker-compose.yml", Service: "db"}},
	})

	file, ok := findJob(cfg, "docker-compose")
	if !ok {
		t.Fatalf("no job for the file; jobs = %v", cfg.Jobs)
	}
	if !strings.HasSuffix(file.Cmd, "up -d api web") {
		t.Errorf("cmd = %q, want only the services that stayed", file.Cmd)
	}
	if strings.Contains(file.Cmd, " db") {
		t.Errorf("cmd = %q still starts the shared service", file.Cmd)
	}
}

// Nothing left to run means no job at all, rather than one that starts the
// whole file behind the shared services' backs.
func TestBuildDockerJobsDropsAFileWhoseServicesAreAllShared(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"infra.yml"},
		Scans:      map[string]domain.ComposeScan{"infra.yml": scanOf("infra.yml", "db", "keycloak")},
		Shared: []domain.SharedComposeService{
			{File: "infra.yml", Service: "db"},
			{File: "infra.yml", Service: "keycloak"},
		},
	})

	if _, ok := findJob(cfg, "docker-compose-infra"); ok {
		t.Errorf("a file with nothing left kept a job; jobs = %v", cfg.Jobs)
	}
	if len(cfg.Jobs) != 2 {
		t.Errorf("jobs = %v, want the two shared services only", cfg.Jobs)
	}
}

// Without a scope answer, nothing changes: every run.toml generated before this
// existed must keep being generated the same way.
func TestBuildDockerJobsUnchangedWithoutAnySharedService(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"docker-compose.yml"},
	})

	file, ok := findJob(cfg, "docker-compose")
	if !ok {
		t.Fatalf("jobs = %v", cfg.Jobs)
	}
	if !strings.HasSuffix(file.Cmd, "up -d") {
		t.Errorf("cmd = %q, want the whole file started", file.Cmd)
	}
	if !strings.Contains(file.Stop, "down --remove-orphans") {
		t.Errorf("stop = %q", file.Stop)
	}
}

// Two files may both declare "db"; the shared entry names its file, and only
// that file's service is lifted.
func TestBuildDockerJobsKeepsFilesApart(t *testing.T) {
	cfg := BuildDockerJobs(BuildDockerJobsParams{
		ComposeCmd: "docker compose",
		Files:      []string{"a.yml", "b.yml"},
		Scans: map[string]domain.ComposeScan{
			"a.yml": scanOf("a.yml", "db", "api"),
			"b.yml": scanOf("b.yml", "db", "worker"),
		},
		Shared: []domain.SharedComposeService{{File: "a.yml", Service: "db"}},
	})

	jobB, ok := findJob(cfg, "docker-compose-b")
	if !ok {
		t.Fatalf("jobs = %v", cfg.Jobs)
	}
	// b.yml lifted nothing, so its command keeps the plain form: naming its
	// services explicitly would be a change nobody asked for.
	if !strings.HasSuffix(jobB.Cmd, "-f b.yml up -d") {
		t.Errorf("cmd = %q, want b.yml untouched", jobB.Cmd)
	}

	shared, ok := findJob(cfg, "db")
	if !ok {
		t.Fatal("a.yml's db was not lifted")
	}
	if !strings.Contains(shared.Cmd, "-f a.yml ") {
		t.Errorf("the shared job reads %q; it must be a.yml's db, not b.yml's", shared.Cmd)
	}

	jobA, _ := findJob(cfg, "docker-compose-a")
	if !strings.HasSuffix(jobA.Cmd, "up -d api") {
		t.Errorf("cmd = %q, want a.yml left with api only", jobA.Cmd)
	}
}
