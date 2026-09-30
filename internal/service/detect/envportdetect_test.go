package detect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeEnvFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestScanEnvPortsGroupsByDirectory(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "apps/web/.env", "PORT=3000\nDATABASE_URL=postgres://localhost:5432/app\n")
	writeEnvFile(t, dir, "apps/api/.env", "API_PORT=4000\n")

	scans := ScanEnvPorts(ScanEnvPortsParams{ProjectDir: dir, Files: EnvFiles(dir)})

	if got := scans["apps/web"].Ports; !reflect.DeepEqual(got, map[string]int{"PORT": 3000}) {
		t.Errorf("apps/web ports = %v", got)
	}
	if got := scans["apps/api"].Ports; !reflect.DeepEqual(got, map[string]int{"API_PORT": 4000}) {
		t.Errorf("apps/api ports = %v", got)
	}
	if got := scans["apps/web"].SourceByVar["PORT"]; got != filepath.Join("apps/web", ".env") {
		t.Errorf("source = %q, want apps/web/.env", got)
	}
}

func TestScanEnvPortsPrefersLocalOverValueOverTemplate(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, ".env.example", "PORT=3000\nTEMPLATE_ONLY_PORT=9000\n")
	writeEnvFile(t, dir, ".env", "PORT=3100\n")
	writeEnvFile(t, dir, ".env.local", "PORT=3200\n")

	scan := ScanEnvPorts(ScanEnvPortsParams{ProjectDir: dir, Files: EnvFiles(dir)})["."]

	want := map[string]int{"PORT": 3200, "TEMPLATE_ONLY_PORT": 9000}
	if !reflect.DeepEqual(scan.Ports, want) {
		t.Errorf("ports = %v, want %v", scan.Ports, want)
	}
	if got := scan.SourceByVar["PORT"]; got != ".env.local" {
		t.Errorf("PORT source = %q, want .env.local", got)
	}
	if got := scan.SourceByVar["TEMPLATE_ONLY_PORT"]; got != ".env.example" {
		t.Errorf("TEMPLATE_ONLY_PORT source = %q, want .env.example", got)
	}
}

// A fresh clone has no .env — the committed template is all there is.
func TestScanEnvPortsFallsBackToTheTemplateAlone(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "apps/web/.env.example", "PORT=3000\n")

	scan := ScanEnvPorts(ScanEnvPortsParams{ProjectDir: dir, Files: EnvFiles(dir)})["apps/web"]

	if !reflect.DeepEqual(scan.Ports, map[string]int{"PORT": 3000}) {
		t.Errorf("ports = %v, want the template's", scan.Ports)
	}
	if scan.Err != "" {
		t.Errorf("a missing .env next to a template is not a failure, got %q", scan.Err)
	}
}

func TestScanEnvPortsSkipsADirectoryWithNoPort(t *testing.T) {
	dir := t.TempDir()
	writeEnvFile(t, dir, "apps/web/.env", "NODE_ENV=development\n")

	scans := ScanEnvPorts(ScanEnvPortsParams{ProjectDir: dir, Files: EnvFiles(dir)})

	if _, found := scans["apps/web"]; found {
		t.Errorf("a directory declaring no port yields no scan, got %v", scans["apps/web"])
	}
}

func TestScanEnvPortsReportsAnUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads regardless of mode")
	}

	dir := t.TempDir()
	writeEnvFile(t, dir, "apps/web/.env", "PORT=3000\n")
	if err := os.Chmod(filepath.Join(dir, "apps/web/.env"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	scan := ScanEnvPorts(ScanEnvPortsParams{ProjectDir: dir, Files: EnvFiles(dir)})["apps/web"]

	if scan.Err == "" {
		t.Fatal("an unreadable file is reported, not silently dropped")
	}
	if len(scan.Ports) != 0 {
		t.Errorf("ports = %v, want none", scan.Ports)
	}
}
