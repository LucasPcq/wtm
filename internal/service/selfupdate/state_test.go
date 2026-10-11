package selfupdate_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestSaveAndLoadState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	want := domain.UpdateState{
		CheckedAt:     time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		LatestVersion: "0.27.0",
	}
	if err := selfupdate.SaveState(t.Context(), want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got := selfupdate.LoadState()
	if !got.CheckedAt.Equal(want.CheckedAt) {
		t.Fatalf("CheckedAt = %v, want %v", got.CheckedAt, want.CheckedAt)
	}
	if got.LatestVersion != want.LatestVersion {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, want.LatestVersion)
	}
}

func TestLoadStateMissingFileIsZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	got := selfupdate.LoadState()
	if !got.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt = %v, want zero on a missing state file", got.CheckedAt)
	}
}

func TestLoadStateCorruptFileIsZero(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	path, err := selfupdate.StatePath()
	if err != nil {
		t.Fatalf("StatePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := selfupdate.LoadState()
	if !got.CheckedAt.IsZero() {
		t.Fatalf("CheckedAt = %v, want zero on a corrupt state file", got.CheckedAt)
	}
}
