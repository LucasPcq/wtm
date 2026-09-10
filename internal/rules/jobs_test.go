package rules

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestJobUptime(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	running := func(startedAt time.Time) domain.JobInfo {
		return domain.JobInfo{Name: "dev", Status: domain.JobStatusRunning, StartedAt: startedAt}
	}
	tests := []struct {
		name string
		job  domain.JobInfo
		want string
	}{
		{"42 s", running(now.Add(-42 * time.Second)), "42s"},
		{"une minute pile", running(now.Add(-time.Minute)), "1m"},
		{"5 min", running(now.Add(-5 * time.Minute)), "5m"},
		{"une heure pile", running(now.Add(-time.Hour)), "1h00m"},
		{"3 h 7", running(now.Add(-3*time.Hour - 7*time.Minute)), "3h07m"},
		{"2 j 5 h", running(now.Add(-53 * time.Hour)), "2d05h"},
		{"démarrage dans le futur", running(now.Add(time.Hour)), "0s"},
		{"jamais démarré", running(time.Time{}), ""},
		{"arrêté", domain.JobInfo{Status: domain.JobStatusStopped, StartedAt: now.Add(-time.Hour)}, ""},
		{"crashé", domain.JobInfo{Status: domain.JobStatusCrashed, StartedAt: now.Add(-time.Hour)}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JobUptime(JobUptimeParams{Job: tt.job, Now: now}); got != tt.want {
				t.Errorf("JobUptime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestJobUptime_SaysNothingWithoutANow pins the degradation an unset Now would
// otherwise be: every row of the table reading 0s, i.e. every job just started.
func TestJobUptime_SaysNothingWithoutANow(t *testing.T) {
	job := domain.JobInfo{
		Name:      "dev",
		Status:    domain.JobStatusRunning,
		StartedAt: time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC),
	}
	if got := JobUptime(JobUptimeParams{Job: job}); got != "" {
		t.Errorf("JobUptime() = %q, want an empty column", got)
	}
}

func TestJobIsDetached(t *testing.T) {
	tests := []struct {
		name string
		job  domain.JobConfig
		want bool
	}{
		{"service no stop", domain.JobConfig{Kind: domain.JobKindService, Cmd: "pnpm dev"}, false},
		{"service with stop", domain.JobConfig{Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}, true},
		{"task no stop", domain.JobConfig{Kind: domain.JobKindTask, Cmd: "pnpm migrate"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDetached(tt.job); got != tt.want {
				t.Errorf("IsDetached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunConfigDefaultProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "back"},
			{Name: "full", Default: true},
			{Name: "front"},
		},
	}
	p, ok := DefaultProfile(cfg)
	if !ok || p.Name != "full" {
		t.Errorf("expected default profile 'full', got %q (ok=%v)", p.Name, ok)
	}
}

func TestRunConfigDefaultProfileFallback(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{{Name: "only"}},
	}
	p, ok := DefaultProfile(cfg)
	if !ok || p.Name != "only" {
		t.Errorf("expected fallback to first profile, got %q (ok=%v)", p.Name, ok)
	}
}

func TestProfileJobsPreservesOrder(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "a", Kind: domain.JobKindService, Cmd: "a"},
			{Name: "b", Kind: domain.JobKindTask, Cmd: "b"},
			{Name: "c", Kind: domain.JobKindService, Cmd: "c"},
		},
	}
	profile := domain.ProfileConfig{Jobs: []string{"c", "a", "b"}}
	got := ProfileJobs(cfg, profile)
	if len(got) != 3 || got[0].Name != "c" || got[1].Name != "a" || got[2].Name != "b" {
		t.Errorf("expected order [c, a, b], got %v", jobNames(got))
	}
}

func jobNames(jobs []domain.JobConfig) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}

func TestFindExistingDefaultProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
			{Name: "ci"},
		},
	}

	if got := FindExistingDefaultProfile(cfg, ""); got != "dev" {
		t.Errorf("expected 'dev', got %q", got)
	}
	if got := FindExistingDefaultProfile(cfg, "dev"); got != "" {
		t.Errorf("expected empty when excluding 'dev', got %q", got)
	}
	if got := FindExistingDefaultProfile(domain.RunConfig{}, ""); got != "" {
		t.Errorf("expected empty for empty cfg, got %q", got)
	}
}

func TestApplyDefaultOverride(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
			{Name: "ci", Default: true},
			{Name: "staging"},
		},
	}

	out := ApplyDefaultOverride(cfg, "ci")

	for _, p := range out.Profiles {
		switch p.Name {
		case "ci":
			if !p.Default {
				t.Errorf("expected 'ci' to keep Default=true, got false")
			}
		case "dev":
			if p.Default {
				t.Errorf("expected 'dev' to be flipped to Default=false, got true")
			}
		case "staging":
			if p.Default {
				t.Errorf("expected 'staging' Default to remain false")
			}
		}
	}

	// Input must not be mutated.
	if !cfg.Profiles[0].Default {
		t.Errorf("ApplyDefaultOverride must not mutate input — original 'dev' was flipped")
	}
}

func TestApplyDefaultOverride_NoDefaultsToFlip(t *testing.T) {
	cfg := domain.RunConfig{
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
		},
	}

	out := ApplyDefaultOverride(cfg, "dev")

	if !out.Profiles[0].Default {
		t.Errorf("expected 'dev' to remain Default=true")
	}
}

func TestIsAlreadyRunning(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"refus du démon", "job api is already running", true},
		{"autre refus", "job api: exit status 1", false},
		{"message vide", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAlreadyRunning(tt.message); got != tt.want {
				t.Errorf("IsAlreadyRunning(%q) = %v, want %v", tt.message, got, tt.want)
			}
		})
	}
}

func TestMergeRunConfigsKeepsThePortOffsetBlock(t *testing.T) {
	dst := domain.RunConfig{PortOffsetBlock: 25, Jobs: []domain.JobConfig{{Name: "a", Cmd: "x"}}}
	src := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "b", Cmd: "y"}}}

	got, _ := MergeRunConfigs(dst, src)
	if got.PortOffsetBlock != 25 {
		t.Errorf("block = %d; a project sets it precisely to keep its ports apart", got.PortOffsetBlock)
	}
}

// Sans profil, `run up` démarre tout ce qui est déclaré. Ne garder que les
// services supprimait en silence les migrations dont un service dépend, et le
// compteur d'étapes affichait [1/1] sans rien dire du job écarté (LUC-208).
func TestJobsWithoutProfileKeepsEveryDeclaredJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "docker-compose", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
		{Name: "dev", Kind: domain.JobKindService},
	}}

	got := JobsWithoutProfile(cfg)
	if len(got) != len(cfg.Jobs) {
		t.Fatalf("expected the %d declared jobs, got %d: %+v", len(cfg.Jobs), len(got), got)
	}
	for i, job := range got {
		if job.Name != cfg.Jobs[i].Name {
			t.Errorf("job %d = %s, want %s: l'ordre déclaré est l'ordre du run", i, job.Name, cfg.Jobs[i].Name)
		}
	}
}

// `wtm run profile add --default` writes the result of this straight back to
// run.toml. Rebuilding the config field by field silently dropped every setting
// the override does not care about — the [[env_port]] links first of all.
func TestApplyDefaultOverrideKeepsEverythingElse(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock:  20,
		PortProbeTimeout: 7,
		Addressing:       domain.AddressingPorts,
		EnvPorts:         []domain.EnvPortLink{{File: ".env", Key: "PORT", Job: "api", Port: "PORT"}},
		Jobs:             []domain.JobConfig{{Name: "api"}},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Default: true},
			{Name: "full", Default: true},
		},
	}

	out := ApplyDefaultOverride(cfg, "full")

	if out.PortOffsetBlock != 20 || out.PortProbeTimeout != 7 || out.Addressing != domain.AddressingPorts {
		t.Errorf("settings dropped: %+v", out)
	}
	if len(out.EnvPorts) != 1 {
		t.Errorf("env_port links dropped: %+v", out.EnvPorts)
	}
	if out.Profiles[0].Default || !out.Profiles[1].Default {
		t.Errorf("the override itself broke: %+v", out.Profiles)
	}
}

func TestMergeRunConfigsKeepsTheAddressing(t *testing.T) {
	dst := domain.RunConfig{Addressing: domain.AddressingPorts, PortProbeTimeout: 7}
	out, _ := MergeRunConfigs(dst, domain.RunConfig{Jobs: []domain.JobConfig{{Name: "new"}}})

	if out.Addressing != domain.AddressingPorts || out.PortProbeTimeout != 7 {
		t.Errorf("a re-init reset what the project settled: %+v", out)
	}
}

func TestFilterToProfileKeepsProjectSettings(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock:  50,
		PortProbeTimeout: 30,
		Addressing:       domain.AddressingPorts,
		Concurrency:      domain.ConcurrencyExclusive,
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm api"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "front", Jobs: []string{"web"}},
			{Name: "back", Jobs: []string{"api"}},
		},
		EnvPorts: []domain.EnvPortLink{
			{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"},
			{File: ".env", Key: "API_PORT", Job: "api", Port: "PORT"},
		},
	}

	got, err := FilterToProfile(cfg, "front")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.PortOffsetBlock != 50 {
		t.Errorf("PortOffsetBlock = %d, want 50", got.PortOffsetBlock)
	}
	if got.PortProbeTimeout != 30 {
		t.Errorf("PortProbeTimeout = %d, want 30", got.PortProbeTimeout)
	}
	if got.Addressing != domain.AddressingPorts {
		t.Errorf("Addressing = %q, want %q", got.Addressing, domain.AddressingPorts)
	}
	if got.Concurrency != domain.ConcurrencyExclusive {
		t.Errorf("Concurrency = %q, want %q", got.Concurrency, domain.ConcurrencyExclusive)
	}
	if len(got.EnvPorts) != 1 || got.EnvPorts[0].Key != "WEB_PORT" {
		t.Errorf("EnvPorts = %+v, want only the WEB_PORT link", got.EnvPorts)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].Name != "web" {
		t.Errorf("Jobs = %+v, want only web", got.Jobs)
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "front" {
		t.Errorf("Profiles = %+v, want only front", got.Profiles)
	}
}

func TestFilterToProfileDoesNotMutateInput(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web"}, {Name: "api"}},
		Profiles: []domain.ProfileConfig{{Name: "front", Jobs: []string{"web"}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "API_PORT", Job: "api", Port: "PORT"}},
	}

	if _, err := FilterToProfile(cfg, "front"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Jobs) != 2 || len(cfg.EnvPorts) != 1 {
		t.Errorf("input mutated: %+v", cfg)
	}
}

func TestJobUptimeCountsTheLifetimeOfAReapedOrphan(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	got := JobUptime(JobUptimeParams{
		Job: domain.JobInfo{
			Status:    domain.JobStatusReaped,
			StartedAt: now.Add(-12 * 24 * time.Hour),
		},
		Now: now,
	})

	if got == "" {
		t.Fatal("a reaped job ran until the instant it was reaped: its age is what the row is for")
	}
	if !strings.Contains(got, "12") {
		t.Fatalf("uptime = %q, want the twelve days it had been running", got)
	}
}

func TestJobUptimeStaysSilentForAJobThatDiedUnwatched(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for _, status := range []domain.JobStatus{domain.JobStatusCrashed, domain.JobStatusStopped} {
		got := JobUptime(JobUptimeParams{
			Job: domain.JobInfo{Status: status, StartedAt: now.Add(-12 * 24 * time.Hour)},
			Now: now,
		})
		if got != "" {
			t.Errorf("uptime for %q = %q, want none: it died at a moment nobody recorded", status, got)
		}
	}
}
