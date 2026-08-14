package inittui

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// TestDetectedHooks asserts the hooks-gate summary reflects what init would
// actually seed: a workspace monorepo shows a single install, an independent one
// shows the per-directory fan-out.
func TestDetectedHooks(t *testing.T) {
	workspace := domain.InitDetectionResult{
		InstallCommand: domain.InstallCommandPnpm,
		Monorepo: domain.MonorepoLayout{
			Kind:              domain.MonorepoKindWorkspace,
			WorkspacePackages: []string{"apps/web", "apps/api"},
		},
	}
	if got := detectedHooks(workspace); got != domain.InstallCommandPnpm {
		t.Errorf("workspace summary = %q, want %q", got, domain.InstallCommandPnpm)
	}

	independent := domain.InitDetectionResult{
		Monorepo: domain.MonorepoLayout{
			Kind: domain.MonorepoKindIndependent,
			SubProjects: []domain.SubProject{
				{Dir: "front", PackageManager: domain.PkgManagerNpm},
				{Dir: "api", PackageManager: domain.PkgManagerPip},
			},
		},
	}
	want := "npm install  (+1 more)"
	if got := detectedHooks(independent); got != want {
		t.Errorf("independent summary = %q, want %q", got, want)
	}
}

func TestMoveToFront(t *testing.T) {
	items := []components.SelectItem{
		{Value: "example"},
		{Value: "main"},
		{Value: "parent"},
	}

	got := moveToFront(items, "parent")
	if got[0].Value != "parent" {
		t.Errorf("expected parent first, got %q", got[0].Value)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}

	// Unknown / empty value leaves order untouched.
	if moveToFront(items, "")[0].Value != "example" {
		t.Error("empty value should not reorder")
	}
	if moveToFront(items, "nope")[0].Value != "example" {
		t.Error("unknown value should not reorder")
	}
}

func TestPrefillSelected(t *testing.T) {
	// Full init (nil prefill) uses the detection default.
	if !prefillSelected(nil, false, true) {
		t.Error("nil prefill should use the full-init default (true)")
	}
	if prefillSelected(nil, true, false) {
		t.Error("nil prefill should use the full-init default (false)")
	}

	// Re-init checks items already in the current config.
	if !prefillSelected(&SectionPrefill{}, true, false) {
		t.Error("configured item should be checked")
	}
	if prefillSelected(&SectionPrefill{}, false, true) {
		t.Error("non-configured item should be unchecked regardless of detection default")
	}
}
