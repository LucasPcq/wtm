package compose_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/service/compose"
)

func writeCompose(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

// The scope step enumerates services, and it needs the one structural fact that
// separates a candidate for sharing from a service that can never be one: a
// service built here serves this worktree's own source.
func TestScanReportsServicesWithImageAndBuild(t *testing.T) {
	dir := writeCompose(t, `services:
  db:
    image: postgres:16
    ports:
      - "5432:5432"
  api:
    build: .
    ports:
      - "3000:3000"
`)

	scan := compose.Scan(compose.ScanParams{ProjectDir: dir, File: "docker-compose.yml", Project: "demo"})
	if scan.Err != "" {
		t.Fatalf("scan: %s", scan.Err)
	}
	if len(scan.Services) != 2 {
		t.Fatalf("services = %v, want 2", scan.Services)
	}

	if scan.Services[0].Name != "db" || scan.Services[0].Image != "postgres:16" || scan.Services[0].HasBuild {
		t.Errorf("db = %+v", scan.Services[0])
	}
	if scan.Services[1].Name != "api" || !scan.Services[1].HasBuild {
		t.Errorf("api = %+v", scan.Services[1])
	}
}

// Declaration order is what the step lists them in; a map range would permute
// the question between two runs of the wizard.
func TestScanKeepsServiceDeclarationOrder(t *testing.T) {
	dir := writeCompose(t, `services:
  zebra:
    image: a
  alpha:
    image: b
  middle:
    image: c
`)

	scan := compose.Scan(compose.ScanParams{ProjectDir: dir, File: "docker-compose.yml", Project: "demo"})
	want := []string{"zebra", "alpha", "middle"}
	for i, name := range want {
		if scan.Services[i].Name != name {
			t.Errorf("service %d = %q, want %q", i, scan.Services[i].Name, name)
		}
	}
}
