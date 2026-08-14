package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// TestBuildInstallHooksWorkspaceEmitsRootOnly is the regression guard for the
// reported bug: a workspace monorepo used to get one install per package even
// though the root install already covers them all.
func TestBuildInstallHooksWorkspaceEmitsRootOnly(t *testing.T) {
	got := rules.BuildInstallHooks(rules.InstallHooksParams{
		RootCommand: domain.InstallCommandPnpm,
		Kind:        domain.MonorepoKindWorkspace,
		SubProjects: []domain.SubProject{{Dir: "apps/web", PackageManager: domain.PkgManagerPnpm}},
	})

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 hook for a workspace, got %d: %+v", len(got), got)
	}
	if got[0].Cmd != domain.InstallCommandPnpm || got[0].Cwd != "" {
		t.Errorf("hook = %+v, want the bare root install", got[0])
	}
}

func TestBuildInstallHooksIndependentUsesPerDirPackageManager(t *testing.T) {
	got := rules.BuildInstallHooks(rules.InstallHooksParams{
		Kind: domain.MonorepoKindIndependent,
		SubProjects: []domain.SubProject{
			{Dir: "front", PackageManager: domain.PkgManagerNpm},
			{Dir: "api", PackageManager: domain.PkgManagerPip},
		},
	})

	want := []domain.HookCommand{
		{Cmd: domain.InstallCommandNpm, Cwd: "front"},
		{Cmd: domain.InstallCommandPip, Cwd: "api"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d hooks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hook[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBuildInstallHooksIndependentKeepsExplicitRootCommand(t *testing.T) {
	got := rules.BuildInstallHooks(rules.InstallHooksParams{
		RootCommand: "make install",
		Kind:        domain.MonorepoKindIndependent,
		SubProjects: []domain.SubProject{
			{Dir: "front", PackageManager: domain.PkgManagerNpm},
			{Dir: "api", PackageManager: domain.PkgManagerPip},
		},
	})

	if len(got) != 3 {
		t.Fatalf("expected 3 hooks, got %d: %+v", len(got), got)
	}
	if got[0].Cmd != "make install" || got[0].Cwd != "" {
		t.Errorf("first hook = %+v, want the bare root command", got[0])
	}
}

func TestBuildInstallHooksNoneEmitsRootOnly(t *testing.T) {
	got := rules.BuildInstallHooks(rules.InstallHooksParams{
		RootCommand: domain.InstallCommandNpm,
		Kind:        domain.MonorepoKindNone,
	})

	if len(got) != 1 || got[0].Cwd != "" {
		t.Errorf("got %+v, want a single bare root hook", got)
	}
}

func TestBuildInstallHooksEmptyRootAndNoneReturnsNil(t *testing.T) {
	if got := rules.BuildInstallHooks(rules.InstallHooksParams{Kind: domain.MonorepoKindNone}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestBuildInstallHooksSkipsSubProjectWithUnknownManager(t *testing.T) {
	got := rules.BuildInstallHooks(rules.InstallHooksParams{
		Kind: domain.MonorepoKindIndependent,
		SubProjects: []domain.SubProject{
			{Dir: "front", PackageManager: domain.PkgManagerNpm},
			{Dir: "docs", PackageManager: domain.PkgManagerNone},
		},
	})

	if len(got) != 1 || got[0].Cwd != "front" {
		t.Errorf("got %+v, want only the front hook", got)
	}
}
