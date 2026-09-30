package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestWritePortKeysWritesTheValueFileAndItsTemplate(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "apps", "shop", "web")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".env", ".env.example"} {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte("VITE_API_URL=http://localhost:4001\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	applied, err := WritePortKeys(WritePortKeysParams{
		ProjectDir: dir,
		Writes: []domain.PortKeyWrite{{
			Job: "shop-web-dev", Port: "VITE_PORT", Base: 5173,
			File: "apps/shop/web/.env", Template: "apps/shop/web/.env.example",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 1 {
		t.Fatalf("got %d applied, want 1", len(applied))
	}

	for _, name := range []string{".env", ".env.example"} {
		content, readErr := os.ReadFile(filepath.Join(envDir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(content), "VITE_PORT=5173") {
			t.Fatalf("%s: got %q, want it to contain VITE_PORT=5173", name, content)
		}
		if !strings.Contains(string(content), "VITE_API_URL=http://localhost:4001") {
			t.Fatalf("%s: the lines it did not touch must survive: %q", name, content)
		}
	}
}

func TestWritePortKeysSkipsAnAbsentTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := WritePortKeys(WritePortKeysParams{
		ProjectDir: dir,
		Writes:     []domain.PortKeyWrite{{Job: "api", Port: "PORT", Base: 4001, File: ".env", Template: ".env.example"}},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".env.example")); !os.IsNotExist(err) {
		t.Fatal("wtm must not create a committed template nobody asked for")
	}
}

func TestWritePortKeysCreatesTheValueFileItNeeds(t *testing.T) {
	dir := t.TempDir()

	if _, err := WritePortKeys(WritePortKeysParams{
		ProjectDir: dir,
		Writes:     []domain.PortKeyWrite{{Job: "reports", Port: "PORT", Base: 5177, File: "apps/reports/.env"}},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "apps", "reports", ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "PORT=5177") {
		t.Fatalf("got %q", content)
	}
}

func TestWritePortKeysIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=4001\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	applied, err := WritePortKeys(WritePortKeysParams{
		ProjectDir: dir,
		Writes:     []domain.PortKeyWrite{{Job: "api", Port: "PORT", Base: 4001, File: ".env"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Fatalf("nothing changed, nothing to report: %+v", applied)
	}
}
