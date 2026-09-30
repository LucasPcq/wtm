package run

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

func loadRunConfig(t *testing.T, stateDir string) domain.RunConfig {
	t.Helper()
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	return cfg
}

// A data task added by hand has to be able to say what it touches, or it
// escapes the foreign-data guard the day it is declared.
func TestRunJobAdd_DeclaresTouchesAndRuns(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "reset",
		"--"+domain.FlagCmd, "pnpm db:reset",
		"--"+domain.FlagKind, string(domain.JobKindTask),
		"--"+domain.FlagTouches, "db",
	); err != nil {
		t.Fatalf("add reset: %v", err)
	}
	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "dev",
		"--"+domain.FlagCmd, "turbo run dev",
		"--"+domain.FlagRuns, "api",
		"--"+domain.FlagRuns, "web",
		"--"+domain.FlagBindsNoPort,
	); err != nil {
		t.Fatalf("add dev: %v", err)
	}

	cfg := loadRunConfig(t, stateDir)
	if got := findJob(t, cfg, "reset").Touches; len(got) != 1 || got[0] != "db" {
		t.Errorf("reset touches = %v, want [db]", got)
	}
	dev := findJob(t, cfg, "dev")
	if len(dev.Runs) != 2 || !dev.BindsNoPort {
		t.Errorf("dev = %+v, want runs [api web] and binds_no_port", dev)
	}
}

func TestRunJobAdd_TouchesAnUndeclaredJobIsRefused(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "reset",
		"--"+domain.FlagCmd, "pnpm db:reset",
		"--"+domain.FlagKind, string(domain.JobKindTask),
		"--"+domain.FlagTouches, "postgres",
	)
	if err == nil || !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("err = %v, want one naming the unknown job", err)
	}
}

func TestRunJobAdd_DeclaresASharedServiceWithItsNamespace(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "pg",
		"--"+domain.FlagCmd, "docker compose up -d pg",
		"--"+domain.FlagScope, domain.ScopeValueShared,
		"--"+domain.FlagNamespaceName, "app_{worktree}",
		"--"+domain.FlagNamespaceCreate, "./scripts/db-add.sh",
		"--"+domain.FlagNamespaceRemove, "./scripts/db-rm.sh",
		"--"+domain.FlagNamespaceEnv, "DB_NAME=app_{worktree}",
	); err != nil {
		t.Fatalf("add pg: %v", err)
	}

	pg := findJob(t, loadRunConfig(t, stateDir), "pg")
	if pg.Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want shared", pg.Scope)
	}
	ns := pg.Namespace
	if ns == nil || ns.Name != "app_{worktree}" || ns.Create != "./scripts/db-add.sh" || ns.Remove != "./scripts/db-rm.sh" || ns.Env["DB_NAME"] != "app_{worktree}" {
		t.Errorf("namespace = %+v, want every field the flags gave", ns)
	}
}

// The loader refuses a namespace on a per-worktree job, and a write must refuse
// what the next read would.
func TestRunJobAdd_NamespaceWithoutSharedScopeIsRefused(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	_, _, err := runCmd(t,
		domain.CmdJob, domain.CmdAdd, "pg",
		"--"+domain.FlagCmd, "docker compose up -d pg",
		"--"+domain.FlagNamespaceName, "app_{worktree}",
		"--"+domain.FlagNamespaceCreate, "./scripts/db-add.sh",
	)
	if err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("err = %v, want the loader's refusal of a namespace on a per-worktree job", err)
	}
}

func TestRunJobEdit_SharesAServiceAndWithdrawsItsNamespace(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdEdit, "db",
		"--"+domain.FlagScope, domain.ScopeValueShared,
		"--"+domain.FlagNamespaceName, "app_{worktree}",
		"--"+domain.FlagNamespaceCreate, "./scripts/db-add.sh",
	); err != nil {
		t.Fatalf("share db: %v", err)
	}
	db := findJob(t, loadRunConfig(t, stateDir), "db")
	if db.Scope != domain.JobScopeShared || db.Namespace == nil || db.Namespace.Create != "./scripts/db-add.sh" {
		t.Fatalf("db = %+v, want shared with a namespace", db)
	}

	if _, _, err := runCmd(t,
		domain.CmdJob, domain.CmdEdit, "db",
		"--"+domain.FlagNamespaceName, "",
		"--"+domain.FlagScope, domain.ScopeValuePerWorktree,
	); err != nil {
		t.Fatalf("unshare db: %v", err)
	}
	db = findJob(t, loadRunConfig(t, stateDir), "db")
	if db.Scope != domain.JobScopePerWorktree || db.Namespace != nil {
		t.Errorf("db = %+v, want per-worktree with no namespace", db)
	}
}

func TestRunJobEdit_NoFlagWithoutTTYNamesTheScopeAndNamespaceFlags(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, editableConfig())

	_, _, err := runCmd(t, domain.CmdJob, domain.CmdEdit, "api")
	for _, flag := range []string{domain.FlagScope, domain.FlagNamespaceName, domain.FlagNamespaceCreate, domain.FlagNamespaceRemove, domain.FlagNamespaceEnv} {
		if err == nil || !strings.Contains(err.Error(), "--"+flag) {
			t.Errorf("err = %v, want one naming --%s", err, flag)
		}
	}
}
