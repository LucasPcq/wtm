package rules

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestValidateEnvFiles(t *testing.T) {
	cases := []struct {
		name    string
		files   []domain.EnvFile
		wantErr error
	}{
		{"valid pair", []domain.EnvFile{{Target: ".env", Template: ".env.example"}}, nil},
		{"valid local", []domain.EnvFile{{Target: ".env.local", Local: true}}, nil},
		{"empty target", []domain.EnvFile{{Target: ""}}, domain.ErrEnvFileNoTarget},
		{"duplicate target", []domain.EnvFile{{Target: ".env"}, {Target: ".env"}}, domain.ErrEnvFileDuplicateTarget},
		{"bad template", []domain.EnvFile{{Target: ".env", Template: ".env.bogus"}}, domain.ErrEnvFileBadTemplate},
		{"template mismatched target", []domain.EnvFile{{Target: ".env", Template: "apps/.env.example"}}, domain.ErrEnvFileBadTemplate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEnvFiles(tc.files)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateSudoDeletePath(t *testing.T) {
	const home = "/home/dev"
	const project = "/home/dev/repo"

	cases := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"safe worktree under project", "/home/dev/repo/.worktrees/feat", false},
		{"safe sibling worktree", "/home/dev/worktrees/feat", false},
		{"filesystem root", "/", true},
		{"home dir", home, true},
		{"project root", project, true},
		{"ancestor of project", "/home/dev", true},
		{"relative path", "repo/.worktrees/feat", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSudoDeletePath(SudoDeletePathParams{
				Path:       tc.path,
				HomeDir:    home,
				ProjectDir: project,
			})
			if tc.wantErr && !errors.Is(err, domain.ErrUnsafeSudoDeletePath) {
				t.Fatalf("expected ErrUnsafeSudoDeletePath for %q, got %v", tc.path, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.path, err)
			}
		})
	}
}

func TestValidateRelocateTarget(t *testing.T) {
	cases := []struct {
		name    string
		to      string
		wantErr bool
	}{
		{"empty means not provided", "", false},
		{"relative path", "../.worktrees", false},
		{"nested relative path", "worktrees/sub", false},
		{"whitespace only", "   ", true},
		{"absolute path", "/tmp/worktrees", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRelocateTarget(tc.to)
			if tc.wantErr && !errors.Is(err, domain.ErrInvalidBasePath) {
				t.Fatalf("expected ErrInvalidBasePath for %q, got %v", tc.to, err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.to, err)
			}
		})
	}
}

// --- Validate tests (migrated from config/config_test.go) ---

func TestValidateInvalidEnvStrategy(t *testing.T) {
	cfg := validConfig()
	cfg.Project.Env.Strategy = "invalid"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidEnvStrategy) {
		t.Errorf("expected ErrInvalidEnvStrategy, got %v", err)
	}
}

func TestValidateInvalidShellType(t *testing.T) {
	cfg := validConfig()
	cfg.Global.Shell = "powershell"

	err := Validate(cfg)
	if !errors.Is(err, domain.ErrInvalidShellType) {
		t.Errorf("expected ErrInvalidShellType, got %v", err)
	}
}

func validConfig() domain.Config {
	return domain.Config{
		Project: domain.ProjectConfig{
			Worktrees: domain.WorktreesConfig{
				BasePath:   domain.DefaultBasePath,
				BaseBranch: domain.DefaultBaseBranch,
			},
			Env: domain.EnvConfig{
				Strategy: domain.DefaultEnvStrategy,
			},
		},
		Global: domain.GlobalConfig{
			Shell: domain.DefaultShell,
		},
	}
}

// --- ValidateRun tests (migrated from config/jobs_test.go) ---

func TestValidateRunOK(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"migrate", "dev"}, Default: true},
		},
	}
	warnings, errs := ValidateRun(cfg)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
}

func TestValidateRunTaskWithStop(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate", Stop: "echo nope"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for task with stop command")
	}
	if !strings.Contains(strings.Join(errs, " "), "tasks cannot declare a stop command") {
		t.Errorf("unexpected error message: %v", errs)
	}
}

func TestValidateRunMissingKind(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "thing", Cmd: "echo hi"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for missing kind")
	}
}

func TestValidateRunUnknownKind(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "thing", Kind: "wat", Cmd: "echo hi"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown kind")
	}
}

func TestValidateRunProfileReferencesUnknownJob(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev", "ghost"}},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for unknown job reference")
	}
}

func TestValidateRunDuplicateJob(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "dev", Kind: domain.JobKindService, Cmd: "echo dup"},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate job name")
	}
	if !strings.Contains(strings.Join(errs, " "), "duplicate job name") {
		t.Errorf("unexpected error: %v", errs)
	}
}

func TestValidateRunDuplicateProfile(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev"}},
			{Name: "full", Jobs: []string{"dev"}},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate profile name")
	}
	if !strings.Contains(strings.Join(errs, " "), "duplicate profile name") {
		t.Errorf("unexpected error: %v", errs)
	}
}

func TestValidateRunMultipleDefaults(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "a", Jobs: []string{"dev"}, Default: true},
			{Name: "b", Jobs: []string{"dev"}, Default: true},
		},
	}
	_, errs := ValidateRun(cfg)
	if len(errs) == 0 {
		t.Fatal("expected error for multiple default profiles")
	}
	if !strings.Contains(strings.Join(errs, " "), "default") {
		t.Errorf("unexpected error: %v", errs)
	}
}

func TestValidateRunPortsAccepted(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d", Ports: map[string]int{"DB_PORT": 5432}},
		},
	}
	if _, errs := ValidateRun(cfg); len(errs) != 0 {
		t.Fatalf("expected a clean config, got %v", errs)
	}
}

func TestValidateRunPortInvalidName(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"DB-PORT": 5432}},
		},
	}
	_, errs := ValidateRun(cfg)
	if !strings.Contains(strings.Join(errs, " "), "not a valid environment variable name") {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateRunPortOutOfRange(t *testing.T) {
	for _, base := range []int{0, -1, 70000} {
		cfg := domain.RunConfig{
			Jobs: []domain.JobConfig{
				{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": base}},
			},
		}
		_, errs := ValidateRun(cfg)
		if !strings.Contains(strings.Join(errs, " "), "outside") {
			t.Errorf("base %d: unexpected errors: %v", base, errs)
		}
	}
}

func TestValidateRunNegativeBlock(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock: -10,
		Jobs:            []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"}},
	}
	_, errs := ValidateRun(cfg)
	if !strings.Contains(strings.Join(errs, " "), "port_offset_block") {
		t.Errorf("unexpected errors: %v", errs)
	}
}

// The error has to say what to change: at runtime the user only sees an
// EADDRINUSE with nothing pointing back at run.toml.
func TestValidateRunPortCollisionIsActionable(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}},
			{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm api", Ports: map[string]int{"ADMIN": 3010}},
		},
	}
	_, errs := ValidateRun(cfg)
	joined := strings.Join(errs, " ")
	for _, want := range []string{"PORT", "ADMIN", "web", "api", "3000", "3010", "port_offset_block"} {
		if !strings.Contains(joined, want) {
			t.Errorf("error should mention %q, got %v", want, errs)
		}
	}
}

func TestValidateRunNeighbouringBasesAccepted(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d", Ports: map[string]int{
				"PG_A": 5434, "PG_B": 5435, "PG_C": 5436,
			}},
		},
	}
	if _, errs := ValidateRun(cfg); len(errs) != 0 {
		t.Fatalf("bases a single unit apart never collide under a uniform offset, got %v", errs)
	}
}

func runnerConfig(runs ...string) domain.RunConfig {
	return domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev:crm", Kind: domain.JobKindService, Cmd: "pnpm run dev:crm", Runs: runs},
		{Name: "crm-web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Ports: map[string]int{"VITE_PORT": 5175}},
	}}
}

func TestValidateRunAcceptsARunnerNamingItsChildren(t *testing.T) {
	if _, errs := ValidateRun(runnerConfig("crm-web-dev")); len(errs) != 0 {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateRunRefusesARunnerNamingAnUnknownJob(t *testing.T) {
	_, errs := ValidateRun(runnerConfig("nope"))

	if len(errs) != 1 || !strings.Contains(errs[0], "runs unknown job") {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateRunRefusesARunnerNamingItself(t *testing.T) {
	_, errs := ValidateRun(runnerConfig("dev:crm"))

	if len(errs) != 1 || !strings.Contains(errs[0], "runs itself") {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateRunRefusesACycleOfRunners(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "a", Kind: domain.JobKindService, Cmd: "x", Runs: []string{"b"}},
		{Name: "b", Kind: domain.JobKindService, Cmd: "y", Runs: []string{"a"}},
	}}

	if _, errs := ValidateRun(cfg); len(errs) == 0 {
		t.Fatal("a runner reachable from itself makes every walk of the relation recurse forever")
	}
}

func TestValidateRunRefusesBindsNoPortOnAJobDeclaringOne(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Kind: domain.JobKindService, Cmd: "x", Ports: map[string]int{"PORT": 3000}, BindsNoPort: true},
	}}

	_, errs := ValidateRun(cfg)
	if len(errs) != 1 || !strings.Contains(errs[0], "contradicts") {
		t.Fatalf("got %v", errs)
	}
}

func TestValidateRunRefusesBindsNoPortOnATask(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "migrate", Kind: domain.JobKindTask, Cmd: "x", BindsNoPort: true},
	}}

	if _, errs := ValidateRun(cfg); len(errs) != 1 {
		t.Fatalf("got %v", errs)
	}
}
