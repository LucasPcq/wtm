package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func recapConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "docker-compose", Kind: domain.JobKindService, Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379}},
			{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api", Ports: map[string]int{"PORT": 3001}},
			{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"},
			{Name: "seed", Kind: domain.JobKindTask, Cwd: "apps/api"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "api", Jobs: []string{"docker-compose", "api-dev"}},
			{Name: "all", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
		},
	}
}

func TestRecapJobLinesShowsEachJobsPorts(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	for _, want := range []string{"POSTGRES_PORT 5432", "REDIS_PORT 6379", "PORT 3001"} {
		if !strings.Contains(got, want) {
			t.Errorf("the recap must show the port, not a count — missing %q:\n%s", want, got)
		}
	}
}

func TestRecapJobLinesFlagsAServiceWithNoPort(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	if !strings.Contains(got, domain.RecapNoPort) {
		t.Errorf("web-dev declares no port and the recap must say so:\n%s", got)
	}
}

func TestRecapJobLinesNamesATaskAsOne(t *testing.T) {
	got := strings.Join(RecapJobLines(recapConfig()), "\n")

	if !strings.Contains(got, domain.RecapTask) {
		t.Errorf("a task binds nothing — it must not read as a service missing a port:\n%s", got)
	}
}

func TestRecapProfileLinesShowsTheJobsAndTheDefault(t *testing.T) {
	got := strings.Join(RecapProfileLines(recapConfig()), "\n")

	if !strings.Contains(got, "docker-compose, api-dev") {
		t.Errorf("a profile must list its jobs:\n%s", got)
	}
	if !strings.Contains(got, domain.RecapDefaultSuffix) {
		t.Errorf("the default profile must be marked:\n%s", got)
	}
}

func TestRecapLinesAlignOnTheLongestName(t *testing.T) {
	lines := RecapJobLines(recapConfig())

	at := strings.Index(lines[0], "POSTGRES_PORT")
	for _, line := range lines[1:] {
		if fields := strings.Fields(line); len(fields) > 1 && strings.Index(line, fields[1]) != at {
			t.Errorf("column drifted:\n%s", strings.Join(lines, "\n"))
			break
		}
	}
}

// Le récap sert à juger la config avant qu'elle soit écrite : un job qui
// répondra sous son propre nom doit le dire là, pas seulement dans run.toml.
func TestRecapJobLinesMarksAPublishedJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Ports: map[string]int{domain.PortNameDefault: 3000},
			URL: &domain.JobURLConfig{Port: domain.PortNameDefault}},
		{Name: "mailhog", Kind: domain.JobKindService, Ports: map[string]int{domain.PortNameDefault: 8025}},
	}}

	lines := RecapJobLines(cfg)
	if !strings.HasSuffix(lines[0], domain.RecapURLSuffix) {
		t.Errorf("ligne web = %q, want le suffixe %q", lines[0], domain.RecapURLSuffix)
	}
	if strings.HasSuffix(lines[1], domain.RecapURLSuffix) {
		t.Errorf("ligne mailhog = %q, want aucun suffixe", lines[1])
	}
}

func TestRecapJobLinesSayWhyAServiceHasNoPort(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "pnpm run dev:crm", Runs: []string{"crm-web-dev", "crm-api-dev"}},
		{Name: "ui-watch", Kind: domain.JobKindService, Cmd: "tsc --watch", BindsNoPort: true},
		{Name: "forgotten", Kind: domain.JobKindService, Cmd: "vite"},
		{Name: "crm-web-dev", Kind: domain.JobKindService, Cmd: "vite", Ports: map[string]int{"VITE_PORT": 5175}},
		{Name: "crm-api-dev", Kind: domain.JobKindService, Cmd: "tsx", Ports: map[string]int{"PORT": 4002}},
	}}

	lines := RecapJobLines(cfg)

	if !strings.Contains(lines[0], "runs crm-web-dev") {
		t.Errorf("a runner says what it starts: %q", lines[0])
	}
	if strings.Contains(lines[0], domain.RecapNoPort) {
		t.Errorf("and is not warned about: %q", lines[0])
	}
	if !strings.Contains(lines[1], domain.RecapBindsNoPort) || strings.Contains(lines[1], domain.RecapNoPort) {
		t.Errorf("a service that answered is not warned about either: %q", lines[1])
	}
	if !strings.Contains(lines[2], domain.RecapNoPort) {
		t.Errorf("one that answered nothing still is: %q", lines[2])
	}
}
