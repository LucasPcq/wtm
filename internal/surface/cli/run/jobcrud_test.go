package run

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
)

func TestRunJobAdd_OK(t *testing.T) {
	stateDir := setupTestProject(t)

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "go run ./cmd/api",
		"--"+domain.FlagKind, string(domain.JobKindService),
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job add: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionAdded {
		t.Errorf("unexpected result: %+v", result)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "api" {
		t.Errorf("expected one job 'api', got %+v", cfg.Jobs)
	}
}

func TestRunJobAdd_Duplicate(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "echo dup",
	)
	if err == nil {
		t.Fatal("expected error for duplicate job name")
	}
	if !strings.Contains(err.Error(), "duplicate job name") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestRunJobRm_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api")
	if err != nil {
		t.Fatalf("run job rm: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 0 {
		t.Errorf("expected zero jobs, got %+v", cfg.Jobs)
	}
}

func TestRunJobRm_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobRm_Referenced(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api")
	if err == nil {
		t.Fatal("expected error for referenced job")
	}
	if !strings.Contains(err.Error(), "referenced") || !strings.Contains(err.Error(), "dev") {
		t.Errorf("expected error to mention 'referenced' and 'dev', got: %v", err)
	}
}

func TestRunJobRm_ReferencedForce(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "echo build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api", "build"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api", "--"+domain.FlagForce)
	if err != nil {
		t.Fatalf("run job rm --force: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "build" {
		t.Errorf("expected only 'build' remaining, got %+v", cfg.Jobs)
	}
	if len(cfg.Profiles) != 1 || slices.Contains(cfg.Profiles[0].Jobs, "api") {
		t.Errorf("expected 'api' stripped from profile 'dev', got %+v", cfg.Profiles)
	}
}

// An [[env_port]] naming a removed job pointed at nothing, which the loader
// refuses. It is a reference like any other: refused without --force, stripped
// with it.
func TestRunJobRm_DelieSesEnvPorts(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi", Ports: map[string]int{domain.PortNameDefault: 4001}},
			{Name: "web", Kind: domain.JobKindService, Cmd: "echo web", Ports: map[string]int{"WEB_PORT": 5173}},
		},
		EnvPorts: []domain.EnvPortLink{
			{File: ".env", Key: "PORT", Job: "api", Port: domain.PortNameDefault},
			{File: ".env", Key: "WEB_PORT", Job: "web", Port: "WEB_PORT"},
		},
	})

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api"); err == nil || !strings.Contains(err.Error(), "[[env_port]] PORT") {
		t.Fatalf("run job rm = %v, want a refusal naming the link", err)
	}
	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api", "--"+domain.FlagForce); err != nil {
		t.Fatalf("run job rm --force: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.EnvPorts) != 1 || cfg.EnvPorts[0].Job != "web" {
		t.Errorf("env_port = %+v, want seulement celui de web", cfg.EnvPorts)
	}
}

// Un profil qui ne garde aucun job ne démarre plus rien : le laisser en
// ferait une entrée morte dans le picker de `run up`.
func TestRunJobRm_RetireUnProfilDevenuVide(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "echo build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api", "build"}},
			{Name: "api-only", Jobs: []string{"api"}},
		},
	})

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdRm, "api", "--"+domain.FlagForce); err != nil {
		t.Fatalf("run job rm --force: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" {
		t.Errorf("profils = %+v, want seulement dev", cfg.Profiles)
	}
}

func TestRunProfileAdd_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "api",
		"--"+domain.FlagDefault,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile add: %v", err)
	}

	var result output.ProfileActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "dev" || result.Status != domain.JobActionAdded {
		t.Errorf("unexpected result: %+v", result)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" || !cfg.Profiles[0].Default {
		t.Errorf("expected default profile 'dev', got %+v", cfg.Profiles)
	}
}

func TestRunProfileAdd_UnknownJob(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "ghost",
	)
	if err == nil {
		t.Fatal("expected error for unknown job ref")
	}
	if !strings.Contains(err.Error(), "unknown job") {
		t.Errorf("expected 'unknown job' error, got: %v", err)
	}
}

func TestRunProfileAdd_Duplicate(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "dev",
		"--"+domain.FlagJobs, "api",
	)
	if err == nil {
		t.Fatal("expected error for duplicate profile name")
	}
	if !strings.Contains(err.Error(), "duplicate profile name") {
		t.Errorf("expected duplicate error, got: %v", err)
	}
}

func TestRunProfileRm_OK(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdRm, "dev")
	if err != nil {
		t.Fatalf("run profile rm: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected zero profiles, got %+v", cfg.Profiles)
	}
	if len(cfg.Jobs) != 1 {
		t.Errorf("jobs should be untouched, got %+v", cfg.Jobs)
	}
}

func TestRunProfileRm_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdRm, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobAdd_TaskWithStop(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "build",
		"--"+domain.FlagCmd, "make build",
		"--"+domain.FlagKind, string(domain.JobKindTask),
		"--"+domain.FlagStop, "echo nope",
	)
	if err == nil {
		t.Fatal("expected error: tasks cannot have stop")
	}
	if !strings.Contains(err.Error(), "tasks cannot declare a stop command") {
		t.Errorf("expected stop-on-task error, got: %v", err)
	}
}

func TestRunJobAdd_InvalidKind(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "weird",
		"--"+domain.FlagCmd, "echo hi",
		"--"+domain.FlagKind, "daemon",
	)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("expected 'unknown kind' error, got: %v", err)
	}
}

func TestRunJobRm_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdRm, "api",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job rm --output json: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionRemoved {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRunJobEdit_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
	})

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown job")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunProfileAdd_DefaultOverride(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	_, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdAdd, "ci",
		"--"+domain.FlagJobs, "api",
		"--"+domain.FlagDefault,
	)
	if err != nil {
		t.Fatalf("run profile add --default with existing default: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	for _, p := range cfg.Profiles {
		switch p.Name {
		case "dev":
			if p.Default {
				t.Errorf("expected old default 'dev' to be unset, but Default=true")
			}
		case "ci":
			if !p.Default {
				t.Errorf("expected new 'ci' to be default, but Default=false")
			}
		default:
			t.Errorf("unexpected profile %q", p.Name)
		}
	}
}

func TestRunProfileRm_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdRm, "dev",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile rm --output json: %v", err)
	}

	var result output.ProfileActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "dev" || result.Status != domain.JobActionRemoved {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRunProfileEdit_NotFound(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}},
		},
	})

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdEdit, "ghost")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("expected error to mention 'ghost', got: %v", err)
	}
}

func TestRunJobList_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "make"},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job list --output json: %v", err)
	}

	var jobs []domain.JobConfig
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(jobs) != 2 || jobs[0].Name != "api" || jobs[1].Name != "build" {
		t.Errorf("unexpected jobs: %+v", jobs)
	}
}

func TestRunProfileList_JSONOutput(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile list --output json: %v", err)
	}

	var profiles []domain.ProfileConfig
	if err := json.Unmarshal([]byte(stdout), &profiles); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(profiles) != 1 || profiles[0].Name != "dev" || !profiles[0].Default {
		t.Errorf("unexpected profiles: %+v", profiles)
	}
}

// TestRunJobList_NotInitialized verifies the opt-in guard: on an uninitialized
// run module (no run.toml jobs/profiles), a blocked command like `job list`
// fails with the pedagogical ErrRunNotInitialized rather than emitting output.
func TestRunJobList_NotInitialized(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdList,
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err == nil {
		t.Fatal("expected error on uninitialized run module")
	}
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Errorf("expected ErrRunNotInitialized, got: %v", err)
	}
}

func TestRunJobAdd_WithPorts(t *testing.T) {
	stateDir := setupTestProject(t)

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagKind, string(domain.JobKindService),
		"--"+domain.FlagPort, "PORT=3000",
		"--"+domain.FlagPort, "ADMIN=9000",
		"--"+domain.FlagOutput, domain.OutputJSON,
	); err != nil {
		t.Fatalf("run job add: %v", err)
	}

	// Read back through the loader: the ports must survive the encode/decode
	// round trip, not merely reach the config in memory.
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := cfg.Jobs[0].Ports
	if ports["PORT"] != 3000 || ports["ADMIN"] != 9000 {
		t.Errorf("got %v, want PORT=3000 ADMIN=9000", ports)
	}
}

func TestRunJobAdd_RejectsMalformedPort(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagPort, "3000",
	)
	if err == nil {
		t.Fatal("expected a malformed --port to be refused")
	}
	if !strings.Contains(err.Error(), "NAME=PORT") {
		t.Errorf("the error should say the expected form, got: %v", err)
	}
}

// A base that collides with one already declared is caught on save, with the
// message that names both sides.
func TestRunJobAdd_RejectsCollidingPort(t *testing.T) {
	setupTestProject(t)

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, "pnpm dev",
		"--"+domain.FlagPort, "PORT=3000",
	); err != nil {
		t.Fatalf("run job add: %v", err)
	}

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "api",
		"--"+domain.FlagCmd, "pnpm api",
		"--"+domain.FlagPort, "API_PORT=3010",
	)
	if err == nil {
		t.Fatal("expected the colliding base to be refused")
	}
	if !strings.Contains(err.Error(), "PORT") || !strings.Contains(err.Error(), "API_PORT") {
		t.Errorf("the error should name both sides, got: %v", err)
	}
}

// A cmd the shell cannot parse is refused when the job is written — the last
// moment the problem can be named, rather than a job that dies at startup.
func TestRunJobAdd_RejectsUnparseableCmd(t *testing.T) {
	stateDir := setupTestProject(t)

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "broken",
		"--"+domain.FlagCmd, `echo "unterminated`,
		"--"+domain.FlagKind, string(domain.JobKindTask),
	)
	if err == nil {
		t.Fatal("expected an unparseable cmd to be refused")
	}
	if !strings.Contains(err.Error(), "not a valid shell command") {
		t.Errorf("got %v, want a shell syntax error naming the job", err)
	}

	cfg, loadErr := config.LoadRun(stateDir)
	if loadErr != nil {
		t.Fatalf("load run: %v", loadErr)
	}
	if len(cfg.Jobs) != 0 {
		t.Errorf("expected nothing written, got %+v", cfg.Jobs)
	}
}

// A cmd carrying a declared port as a CLI flag is exactly what the shell line
// exists for, and must survive a write/read round-trip verbatim.
func TestRunJobAdd_KeepsPortVariableInCmd(t *testing.T) {
	stateDir := setupTestProject(t)

	cmdLine := "pnpm dev --port ${PORT}"
	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "web",
		"--"+domain.FlagCmd, cmdLine,
		"--"+domain.FlagKind, string(domain.JobKindService),
		"--"+domain.FlagPort, "PORT=3000",
	); err != nil {
		t.Fatalf("run job add: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Cmd != cmdLine {
		t.Errorf("got %+v, want cmd %q", cfg.Jobs, cmdLine)
	}
}

// editableConfig is the fixture the `run job edit` tests patch: a fully
// populated job, one before and one after it, and a profile referencing it.
func editableConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up db"},
			{
				Name:  "api",
				Kind:  domain.JobKindService,
				Cmd:   "pnpm dev",
				Stop:  "docker compose down",
				Cwd:   "apps/api",
				Ports: map[string]int{"PORT": 3000, "DB_PORT": 5432},
				URL:   &domain.JobURLConfig{Port: "PORT", Host: "api.app"},
			},
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev:web"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"db", "api", "web"}, Default: true},
		},
	}
}

func TestRunJobEdit_PatchesOneFieldAndLeavesTheRest(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdEdit, "api",
		"--"+domain.FlagCmd, "go run ./cmd/api",
	); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	job := cfg.Jobs[1]
	if job.Cmd != "go run ./cmd/api" {
		t.Errorf("cmd = %q, want the new one", job.Cmd)
	}
	if job.Kind != domain.JobKindService || job.Stop != "docker compose down" || job.Cwd != "apps/api" {
		t.Errorf("job = %+v, want kind, stop and cwd untouched", job)
	}
	if job.Ports["PORT"] != 3000 || job.Ports["DB_PORT"] != 5432 {
		t.Errorf("ports = %v, want them untouched", job.Ports)
	}
	if job.URL == nil || job.URL.Port != "PORT" || job.URL.Host != "api.app" {
		t.Errorf("url = %+v, want it untouched", job.URL)
	}
}

func TestRunJobEdit_PreservesPositionInTheFile(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api", "--"+domain.FlagCwd, ""); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	names := []string{cfg.Jobs[0].Name, cfg.Jobs[1].Name, cfg.Jobs[2].Name}
	if !slices.Equal(names, []string{"db", "api", "web"}) {
		t.Errorf("jobs = %v, want the declared order preserved", names)
	}
	if cfg.Jobs[1].Cwd != "" {
		t.Errorf("cwd = %q, want it cleared by an explicit empty value", cfg.Jobs[1].Cwd)
	}
}

func TestRunJobEdit_MergesPorts(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdEdit, "api",
		"--"+domain.FlagPort, "DB_PORT=5433",
		"--"+domain.FlagPort, "REDIS_PORT=6379",
	); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := cfg.Jobs[1].Ports
	if ports["PORT"] != 3000 || ports["DB_PORT"] != 5433 || ports["REDIS_PORT"] != 6379 {
		t.Errorf("ports = %v, want PORT kept, DB_PORT changed, REDIS_PORT added", ports)
	}
}

func TestRunJobEdit_PortClearEmptiesTheTable(t *testing.T) {
	stateDir := setupTestProject(t)
	cfgIn := editableConfig()
	cfgIn.Jobs[1].URL = nil
	writeRunTOML(t, stateDir, cfgIn)

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api", "--"+domain.FlagPortClear); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs[1].Ports) != 0 {
		t.Errorf("ports = %v, want none", cfg.Jobs[1].Ports)
	}
}

func TestRunJobEdit_RenameRewritesProfileReferences(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api", "--"+domain.FlagName, "backend"); err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Jobs[1].Name != "backend" {
		t.Errorf("job name = %q, want backend", cfg.Jobs[1].Name)
	}
	if !slices.Equal(cfg.Profiles[0].Jobs, []string{"db", "backend", "web"}) {
		t.Errorf("profile jobs = %v, want the reference renamed in place", cfg.Profiles[0].Jobs)
	}
}

func TestRunJobEdit_WithdrawsAndPublishesTheURL(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api", "--"+domain.FlagURLHost, "api-2.app"); err != nil {
		t.Fatalf("run job edit --url-host: %v", err)
	}
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Jobs[1].URL == nil || cfg.Jobs[1].URL.Port != "PORT" || cfg.Jobs[1].URL.Host != "api-2.app" {
		t.Fatalf("url = %+v, want the host changed and the port kept", cfg.Jobs[1].URL)
	}

	if _, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api", "--"+domain.FlagURLPort, ""); err != nil {
		t.Fatalf("run job edit --url-port '': %v", err)
	}
	cfg, err = config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Jobs[1].URL != nil {
		t.Errorf("url = %+v, want it withdrawn", cfg.Jobs[1].URL)
	}
}

func TestRunJobEdit_NoArgWithoutTTYNamesTheArgument(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "--"+domain.FlagCmd, "echo hi")
	if err == nil || !strings.Contains(err.Error(), "argument") {
		t.Errorf("err = %v, want one naming the missing argument", err)
	}
}

func TestRunJobEdit_NoFlagWithoutTTYNamesTheFlags(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api")
	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagCmd) {
		t.Errorf("err = %v, want one naming the flags that could change something", err)
	}
}

func TestRunJobEdit_JSON(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	stdout, _, err := runCmd(t,
		domain.CmdJob, domain.CmdEdit, "api",
		"--"+domain.FlagStop, "",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run job edit: %v", err)
	}

	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionUpdated {
		t.Errorf("unexpected result: %+v", result)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Jobs[1].Stop != "" {
		t.Errorf("stop = %q, want it cleared", cfg.Jobs[1].Stop)
	}
}

// editableProfileConfig is the fixture the `run profile edit` tests patch: two
// profiles, one of them the default.
func editableProfileConfig() domain.RunConfig {
	return domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up db"},
			{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev:web"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "back", Jobs: []string{"db", "api"}},
			{Name: "dev", Jobs: []string{"db", "api", "web"}, Default: true},
		},
	}
}

func TestRunProfileEdit_PatchesJobsAndLeavesTheRest(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	if _, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdEdit, "dev",
		"--"+domain.FlagJobs, "api,web",
	); err != nil {
		t.Fatalf("run profile edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if !slices.Equal(cfg.Profiles[1].Jobs, []string{"api", "web"}) {
		t.Errorf("jobs = %v, want the list replaced in the given order", cfg.Profiles[1].Jobs)
	}
	if !cfg.Profiles[1].Default {
		t.Error("default was lost — only --jobs was passed")
	}
	names := []string{cfg.Profiles[0].Name, cfg.Profiles[1].Name}
	if !slices.Equal(names, []string{"back", "dev"}) {
		t.Errorf("profiles = %v, want the declared order preserved", names)
	}
}

func TestRunProfileEdit_RenamesAndTakesTheDefault(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	if _, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdEdit, "back",
		"--"+domain.FlagName, "backend",
		"--"+domain.FlagDefault,
	); err != nil {
		t.Fatalf("run profile edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Profiles[0].Name != "backend" || !cfg.Profiles[0].Default {
		t.Errorf("profile = %+v, want it renamed and default", cfg.Profiles[0])
	}
	if cfg.Profiles[1].Default {
		t.Error("dev is still default — only one profile can be")
	}
}

func TestRunProfileEdit_UnsetsTheDefault(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	if _, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdEdit, "dev",
		"--"+domain.FlagDefault+"=false",
	); err != nil {
		t.Fatalf("run profile edit: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if cfg.Profiles[1].Default {
		t.Error("default = true, want it taken away without handing it to another profile")
	}
	if cfg.Profiles[0].Default {
		t.Error("back became default on its own")
	}
}

func TestRunProfileEdit_UnknownJobIsRefused(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdEdit, "dev", "--"+domain.FlagJobs, "api,ghost")
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("err = %v, want one naming the unknown job", err)
	}
}

func TestRunProfileEdit_NoArgWithoutTTYNamesTheArgument(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdEdit, "--"+domain.FlagJobs, "api")
	if err == nil || !strings.Contains(err.Error(), "argument") {
		t.Errorf("err = %v, want one naming the missing argument", err)
	}
}

func TestRunProfileEdit_NoFlagWithoutTTYNamesTheFlags(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	_, _, err := runCmd(t, domain.CmdProfile, domain.CmdEdit, "dev")
	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagJobs) {
		t.Errorf("err = %v, want one naming the flags that could change something", err)
	}
}

func TestRunProfileEdit_JSON(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableProfileConfig())

	stdout, _, err := runCmd(t,
		domain.CmdProfile, domain.CmdEdit, "dev",
		"--"+domain.FlagJobs, "api",
		"--"+domain.FlagOutput, domain.OutputJSON,
	)
	if err != nil {
		t.Fatalf("run profile edit: %v", err)
	}

	var result output.ProfileActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "dev" || result.Status != domain.JobActionUpdated {
		t.Errorf("unexpected result: %+v", result)
	}
}
