package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func ptr(s string) *string { return &s }

func fullJob() domain.JobConfig {
	return domain.JobConfig{
		Name:  "api",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm dev",
		Stop:  "docker compose down",
		Cwd:   "apps/api",
		Ports: map[string]int{"PORT": 3000, "DB_PORT": 5432},
		URL:   &domain.JobURLConfig{Port: "PORT", Host: "api.app"},
	}
}

func TestApplyJobPatchLeavesEveryUntouchedField(t *testing.T) {
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: fullJob(),
		Patch:   rules.JobPatch{Cmd: ptr("go run ./cmd/api")},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}

	want := fullJob()
	want.Cmd = "go run ./cmd/api"
	if got.Name != want.Name || got.Kind != want.Kind || got.Stop != want.Stop || got.Cwd != want.Cwd {
		t.Errorf("got %+v, want only cmd changed from %+v", got, want)
	}
	if got.Cmd != want.Cmd {
		t.Errorf("cmd = %q, want %q", got.Cmd, want.Cmd)
	}
	if len(got.Ports) != 2 || got.Ports["PORT"] != 3000 || got.Ports["DB_PORT"] != 5432 {
		t.Errorf("ports = %v, want the table untouched", got.Ports)
	}
	if got.URL == nil || got.URL.Port != "PORT" || got.URL.Host != "api.app" {
		t.Errorf("url = %+v, want the published name untouched", got.URL)
	}
}

func TestApplyJobPatchClearsOnExplicitEmptyString(t *testing.T) {
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: fullJob(),
		Patch:   rules.JobPatch{Stop: ptr(""), Cwd: ptr("")},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if got.Stop != "" || got.Cwd != "" {
		t.Errorf("stop = %q, cwd = %q — both should be cleared", got.Stop, got.Cwd)
	}
	if got.Cmd != "pnpm dev" {
		t.Errorf("cmd = %q, want it untouched", got.Cmd)
	}
}

func TestApplyJobPatchMergesPorts(t *testing.T) {
	current := fullJob()
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: current,
		Patch:   rules.JobPatch{Ports: map[string]int{"DB_PORT": 5433, "REDIS_PORT": 6379}},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if got.Ports["PORT"] != 3000 || got.Ports["DB_PORT"] != 5433 || got.Ports["REDIS_PORT"] != 6379 {
		t.Errorf("ports = %v, want PORT kept, DB_PORT overwritten, REDIS_PORT added", got.Ports)
	}
	if current.Ports["DB_PORT"] != 5432 {
		t.Errorf("the current job's table was mutated: %v", current.Ports)
	}
}

func TestApplyJobPatchClearPortsEmptiesTheTable(t *testing.T) {
	got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: fullJob(),
		Patch:   rules.JobPatch{ClearPorts: true},
	})
	if err != nil {
		t.Fatalf("ApplyJobPatch: %v", err)
	}
	if len(got.Ports) != 0 {
		t.Errorf("ports = %v, want none", got.Ports)
	}
}

func TestApplyJobPatchRefusesClearPortsWithPort(t *testing.T) {
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: fullJob(),
		Patch:   rules.JobPatch{ClearPorts: true, Ports: map[string]int{"PORT": 3000}},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagPortClear) {
		t.Errorf("err = %v, want one naming --%s", err, domain.FlagPortClear)
	}
}

func TestApplyJobPatchURL(t *testing.T) {
	tests := []struct {
		name       string
		patch      rules.JobPatch
		wantPort   string
		wantHost   string
		wantAbsent bool
	}{
		{name: "host alone keeps the port", patch: rules.JobPatch{URLHost: ptr("api-2.app")}, wantPort: "PORT", wantHost: "api-2.app"},
		{name: "empty host keeps the port", patch: rules.JobPatch{URLHost: ptr("")}, wantPort: "PORT"},
		{name: "empty port withdraws the url", patch: rules.JobPatch{URLPort: ptr("")}, wantAbsent: true},
		{name: "port alone keeps the host", patch: rules.JobPatch{URLPort: ptr("DB_PORT")}, wantPort: "DB_PORT", wantHost: "api.app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{Current: fullJob(), Patch: tt.patch})
			if err != nil {
				t.Fatalf("ApplyJobPatch: %v", err)
			}
			if tt.wantAbsent {
				if got.URL != nil {
					t.Fatalf("url = %+v, want none", got.URL)
				}
				return
			}
			if got.URL == nil || got.URL.Port != tt.wantPort || got.URL.Host != tt.wantHost {
				t.Errorf("url = %+v, want %s %s", got.URL, tt.wantPort, tt.wantHost)
			}
		})
	}
}

func TestApplyJobPatchRefusesAHostWithoutAPort(t *testing.T) {
	_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{
		Current: domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		Patch:   rules.JobPatch{URLHost: ptr("api.app")},
	})
	if err == nil || !strings.Contains(err.Error(), domain.FlagURLPort) {
		t.Errorf("err = %v, want one naming --%s", err, domain.FlagURLPort)
	}
}

func TestApplyJobPatchRefusesEmptyRequiredFields(t *testing.T) {
	tests := []struct {
		name  string
		patch rules.JobPatch
		flag  string
	}{
		{name: "name", patch: rules.JobPatch{Name: ptr("  ")}, flag: domain.FlagName},
		{name: "cmd", patch: rules.JobPatch{Cmd: ptr("")}, flag: domain.FlagCmd},
		{name: "kind", patch: rules.JobPatch{Kind: ptr("daemon")}, flag: domain.FlagKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rules.ApplyJobPatch(rules.ApplyJobPatchParams{Current: fullJob(), Patch: tt.patch})
			if err == nil || !strings.Contains(err.Error(), tt.flag) {
				t.Errorf("err = %v, want one naming --%s", err, tt.flag)
			}
		})
	}
}

func TestJobPatchEmpty(t *testing.T) {
	if !(rules.JobPatch{}).Empty() {
		t.Error("a zero patch changes nothing")
	}
	if (rules.JobPatch{Stop: ptr("")}).Empty() {
		t.Error("clearing stop is a change")
	}
	if (rules.JobPatch{ClearPorts: true}).Empty() {
		t.Error("clearing ports is a change")
	}
}

func TestRenameJobRefs(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "api"}, {Name: "web"}},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api", "web"}},
			{Name: "back", Jobs: []string{"api"}},
		},
	}

	got := rules.RenameJobRefs(rules.RenameJobRefsParams{Config: cfg, From: "api", To: "backend"})

	if got.Profiles[0].Jobs[0] != "backend" || got.Profiles[0].Jobs[1] != "web" {
		t.Errorf("dev jobs = %v, want [backend web]", got.Profiles[0].Jobs)
	}
	if got.Profiles[1].Jobs[0] != "backend" {
		t.Errorf("back jobs = %v, want [backend]", got.Profiles[1].Jobs)
	}
	if cfg.Profiles[0].Jobs[0] != "api" {
		t.Errorf("the input config was mutated: %v", cfg.Profiles[0].Jobs)
	}
}

// A renamed job is also named by the env_port links that follow its ports —
// leaving those behind makes the file unsavable.
func TestRenameJobRefsRewritesEnvPortLinks(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{{Name: "api", Ports: map[string]int{"PORT": 4001}}},
		EnvPorts: []domain.EnvPortLink{
			{File: "apps/api/.env", Key: "PORT", Job: "api", Port: "PORT"},
			{File: "apps/web/.env", Key: "VITE_API_URL", Job: "api", Port: "PORT"},
			{File: "apps/api/.env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
		},
	}

	got := rules.RenameJobRefs(rules.RenameJobRefsParams{Config: cfg, From: "api", To: "api-server"})

	if got.EnvPorts[0].Job != "api-server" || got.EnvPorts[1].Job != "api-server" {
		t.Errorf("env_port jobs = %q/%q, want both renamed", got.EnvPorts[0].Job, got.EnvPorts[1].Job)
	}
	if got.EnvPorts[2].Job != "db" {
		t.Errorf("env_port job = %q, want the other job left alone", got.EnvPorts[2].Job)
	}
	if cfg.EnvPorts[0].Job != "api" {
		t.Errorf("the input config was mutated: %+v", cfg.EnvPorts[0])
	}
}

func TestSameJobReadsNilAndEmptyAlike(t *testing.T) {
	a := domain.JobConfig{Name: "api", Cmd: "pnpm dev"}
	b := domain.JobConfig{Name: "api", Cmd: "pnpm dev", Runs: []string{}, Ports: map[string]int{}}

	if !rules.SameJob(a, b) {
		t.Error("an empty list reads as a change")
	}
	b.Touches = []string{"db"}
	if rules.SameJob(a, b) {
		t.Error("a new touches list reads as no change")
	}
}
