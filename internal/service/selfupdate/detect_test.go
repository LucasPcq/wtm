package selfupdate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestDetectInstallReportsSourceForDevBuild(t *testing.T) {
	got := selfupdate.DetectInstall(t.Context(), domain.Version)
	if got.Method != domain.InstallSource {
		t.Fatalf("Method = %q, want %q", got.Method, domain.InstallSource)
	}
}

func TestDetectInstallResolvesARealBinaryPath(t *testing.T) {
	got := selfupdate.DetectInstall(t.Context(), "0.26.1")
	if got.BinaryPath == "" {
		t.Fatal("BinaryPath is empty; DetectInstall must resolve the running executable")
	}
	if !filepath.IsAbs(got.BinaryPath) {
		t.Fatalf("BinaryPath = %q, want an absolute path", got.BinaryPath)
	}
}

func TestGoBinDirResolvesSymlinks(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "gobin-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	t.Setenv("GOBIN", link)

	got := selfupdate.GoBinDirForTest()

	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if got != resolvedTarget {
		t.Fatalf("goBinDir() = %q, want %q — an unresolved GOBIN never matches the resolved executable path", got, resolvedTarget)
	}
}
