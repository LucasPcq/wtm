package compose

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const richCompose = `# une stack de dev
x-common: &common
  restart: unless-stopped

services:
  postgres:                    # la base
    image: postgres:16-alpine
    ports:
      - "${DB_PORT:-5432}:5432"
      - 9187:9187
  redis:
    ports: ['6379:6379']
  mailhog:
    ports:
      - "127.0.0.1:8025:8025"
      - target: 1025
        published: 1025
  edge:
    ports:
      - "3000-3005:3000-3005"
      - "9999"
  novars:
    image: busybox
`

func writeCompose(t *testing.T, content string) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	file = "docker-compose.yml"
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir, file
}

func findBinding(t *testing.T, scan domain.ComposeScan, service string, container int) domain.ComposePortBinding {
	t.Helper()
	for _, b := range scan.Bindings {
		if b.Service == service && b.Container == container {
			return b
		}
	}
	t.Fatalf("no binding for %s/%d in %+v", service, container, scan.Bindings)
	return domain.ComposePortBinding{}
}

func TestScanClassifiesEveryMappingForm(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	if scan.Err != "" {
		t.Fatalf("scan: %s", scan.Err)
	}

	templated := findBinding(t, scan, "postgres", 5432)
	if templated.Status != domain.ComposePortTemplated || templated.Var != "DB_PORT" || templated.Base != 5432 {
		t.Errorf("postgres/5432 = %+v, want templated DB_PORT=5432", templated)
	}
	if templated.Token != `"${DB_PORT:-5432}:5432"` {
		t.Errorf("token = %s, want the quoted source scalar", templated.Token)
	}

	frozen := findBinding(t, scan, "postgres", 9187)
	if frozen.Status != domain.ComposePortFrozen || frozen.Var != "POSTGRES_PORT" || frozen.Token != "9187:9187" {
		t.Errorf("postgres/9187 = %+v, want frozen POSTGRES_PORT with an unquoted token", frozen)
	}

	flow := findBinding(t, scan, "redis", 6379)
	if flow.Status != domain.ComposePortFrozen || flow.Var != "REDIS_PORT" || flow.Token != "'6379:6379'" {
		t.Errorf("redis/6379 = %+v, want frozen REDIS_PORT with a single-quoted token", flow)
	}

	withIP := findBinding(t, scan, "mailhog", 8025)
	if withIP.Replacement != `"127.0.0.1:${MAILHOG_PORT:-8025}:8025"` {
		t.Errorf("mailhog/8025 replacement = %s, want the host IP kept", withIP.Replacement)
	}

	long := findBinding(t, scan, "mailhog", 1025)
	if long.Status != domain.ComposePortFrozen || long.Var != "MAILHOG_PORT_1025" || long.Token != "1025" {
		t.Errorf("mailhog/1025 = %+v, want frozen MAILHOG_PORT_1025 on the published scalar", long)
	}
	if long.Replacement != `"${MAILHOG_PORT_1025:-1025}"` {
		t.Errorf("mailhog/1025 replacement = %s", long.Replacement)
	}

	var reasons []string
	for _, b := range scan.Bindings {
		if b.Service != "edge" {
			continue
		}
		reasons = append(reasons, b.Reason)
		if b.Status != domain.ComposePortUnsupported {
			t.Errorf("edge binding %+v must be unsupported", b)
		}
		if b.Replacement != "" {
			t.Errorf("an unsupported binding is never rewritten, got %q", b.Replacement)
		}
	}
	if len(reasons) != 2 {
		t.Fatalf("edge must report both its mappings, got %v", reasons)
	}
	for _, want := range []string{"range", "no host port"} {
		if !slices.ContainsFunc(reasons, func(r string) bool { return strings.Contains(r, want) }) {
			t.Errorf("no reason mentions %q, got %v", want, reasons)
		}
	}
}

func TestScanIgnoresServicesWithoutPorts(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	for _, b := range scan.Bindings {
		if b.Service == "novars" {
			t.Errorf("a service declaring no ports must produce nothing, got %+v", b)
		}
	}
}

func TestScanReportsAnAliasedPortsList(t *testing.T) {
	dir, file := writeCompose(t, `x-ports: &shared
  - "5432:5432"

services:
  a:
    ports: *shared
`)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	if len(scan.Bindings) != 1 || scan.Bindings[0].Status != domain.ComposePortUnsupported {
		t.Fatalf("got %+v, want one unsupported binding", scan.Bindings)
	}
	if !strings.Contains(scan.Bindings[0].Reason, "alias") {
		t.Errorf("reason = %q, want it to name the alias", scan.Bindings[0].Reason)
	}
}

func TestScanReportsUnreadableAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if missing := Scan(ScanParams{ProjectDir: dir, File: "nope.yml"}); missing.Err == "" {
		t.Error("a missing file must come back with Err set, not empty bindings")
	}

	dir, file := writeCompose(t, "services:\n  a:\n   ports: [\n")
	if broken := Scan(ScanParams{ProjectDir: dir, File: file}); broken.Err == "" {
		t.Error("unparseable YAML must come back with Err set")
	}
}

func TestPatchRewritesOnlyThePortLines(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	var frozen []domain.ComposePortBinding
	for _, b := range scan.Bindings {
		if b.Status == domain.ComposePortFrozen {
			frozen = append(frozen, b)
		}
	}
	if len(frozen) != 4 {
		t.Fatalf("want 4 frozen mappings, got %d", len(frozen))
	}

	if err := PatchAll(PatchAllParams{ProjectDir: dir, ByFile: map[string][]domain.ComposePortBinding{file: frozen}}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	patched, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(patched)

	for _, want := range []string{
		`- "${POSTGRES_PORT:-9187}:9187"`,
		`ports: ["${REDIS_PORT:-6379}:6379"]`,
		`- "127.0.0.1:${MAILHOG_PORT:-8025}:8025"`,
		`published: "${MAILHOG_PORT_1025:-1025}"`,
		`- "${DB_PORT:-5432}:5432"`,
		"# une stack de dev",
		"  postgres:                    # la base",
		"x-common: &common",
		`- "3000-3005:3000-3005"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("patched file is missing %q:\n%s", want, got)
		}
	}

	if before, after := strings.Count(richCompose, "\n"), strings.Count(got, "\n"); before != after {
		t.Errorf("line count changed: %d → %d", before, after)
	}
}

func TestPatchRefusesAFileChangedSinceTheScan(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	if err := os.WriteFile(filepath.Join(dir, file), []byte("services:\n  postgres:\n    ports:\n      - \"1234:1234\"\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	var frozen []domain.ComposePortBinding
	for _, b := range scan.Bindings {
		if b.Status == domain.ComposePortFrozen {
			frozen = append(frozen, b)
		}
	}

	err := PatchAll(PatchAllParams{ProjectDir: dir, ByFile: map[string][]domain.ComposePortBinding{file: frozen}})
	if err == nil {
		t.Fatal("patching a file that moved under us must fail")
	}

	after, readErr := os.ReadFile(filepath.Join(dir, file))
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if !strings.Contains(string(after), `"1234:1234"`) {
		t.Errorf("the file must be left untouched on failure, got:\n%s", after)
	}
}

func TestScanAllKeysByRelativePath(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	scans := ScanAll(ScanAllParams{ProjectDir: dir, Files: []string{file, "absent.yml"}})
	if len(scans) != 2 {
		t.Fatalf("got %d scans, want 2", len(scans))
	}
	if len(scans[file].Bindings) == 0 {
		t.Error("the existing file must carry its bindings")
	}
	if scans["absent.yml"].Err == "" {
		t.Error("the missing file must carry its error")
	}
	if ScanAll(ScanAllParams{ProjectDir: dir}) != nil {
		t.Error("no file means no map")
	}
}

// TestScanNeverReusesANameTheFileAlreadyUses is the guard against wtm breaking a
// compose that worked: naming a literal port after a variable used elsewhere
// would make wtm's own injection rewrite a value the project depends on.
func TestScanNeverReusesANameTheFileAlreadyUses(t *testing.T) {
	dir, file := writeCompose(t, `services:
  postgres:
    ports:
      - "5432:5432"
    environment:
      DSN: postgres://app@db:${POSTGRES_PORT}/app
  metrics:
    ports:
      - "${POSTGRES_PORT:-6000}:9187"
`)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	frozen := findBinding(t, scan, "postgres", 5432)
	if frozen.Var == "POSTGRES_PORT" {
		t.Fatalf("wtm claimed %q, a name the file already reads", frozen.Var)
	}
	if !strings.HasPrefix(frozen.Var, "POSTGRES_PORT_") {
		t.Errorf("var = %q, want a name derived from the service but distinct", frozen.Var)
	}

	templated := findBinding(t, scan, "metrics", 9187)
	if templated.Var != "POSTGRES_PORT" || templated.Base != 6000 {
		t.Errorf("the user's own declaration must be read as written, got %s=%d", templated.Var, templated.Base)
	}
}

func TestScanRefusesAnAnchoredPortsList(t *testing.T) {
	dir, file := writeCompose(t, `services:
  b:
    ports: &bports
      - "6379:6379"
  c:
    ports: *bports
`)
	scan := Scan(ScanParams{ProjectDir: dir, File: file})

	if len(scan.Bindings) != 2 {
		t.Fatalf("got %+v, want one binding per service", scan.Bindings)
	}
	for _, b := range scan.Bindings {
		if b.Status != domain.ComposePortUnsupported {
			t.Errorf("%s: rewriting an anchored list moves every service aliasing it, got %+v", b.Service, b)
		}
		if b.Replacement != "" {
			t.Errorf("%s: must not be rewritten, got %q", b.Service, b.Replacement)
		}
	}
}

// TestPatchAllWritesNothingWhenOneFileMoved is the guard against a half-applied
// rewrite: everything is rendered before anything is written, so a file that
// changed under wtm aborts the whole batch instead of leaving the tree mixed.
func TestPatchAllWritesNothingWhenOneFileMoved(t *testing.T) {
	dir := t.TempDir()
	good, moved := "a-compose.yml", "b-compose.yml"
	for _, f := range []string{good, moved} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("services:\n  db:\n    ports:\n      - \"5432:5432\"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	byFile := map[string][]domain.ComposePortBinding{}
	for _, f := range []string{good, moved} {
		scan := Scan(ScanParams{ProjectDir: dir, File: f})
		byFile[f] = scan.Bindings
	}

	if err := os.WriteFile(filepath.Join(dir, moved), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	if failures := VerifyAll(VerifyAllParams{ProjectDir: dir, ByFile: byFile}); len(failures) != 1 || failures[moved] == "" {
		t.Errorf("VerifyAll must name %s and only it, got %v", moved, failures)
	}

	if err := PatchAll(PatchAllParams{ProjectDir: dir, ByFile: byFile}); err == nil {
		t.Fatal("PatchAll must refuse the batch")
	}

	after, err := os.ReadFile(filepath.Join(dir, good))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(after), `"5432:5432"`) {
		t.Errorf("the healthy file must be left untouched:\n%s", after)
	}
}

func TestPatchAllPreservesFileMode(t *testing.T) {
	dir, file := writeCompose(t, richCompose)
	if err := os.Chmod(filepath.Join(dir, file), 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	scan := Scan(ScanParams{ProjectDir: dir, File: file})
	var frozen []domain.ComposePortBinding
	for _, b := range scan.Bindings {
		if b.Status == domain.ComposePortFrozen {
			frozen = append(frozen, b)
		}
	}
	if err := PatchAll(PatchAllParams{ProjectDir: dir, ByFile: map[string][]domain.ComposePortBinding{file: frozen}}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, file))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 kept across the rename", info.Mode().Perm())
	}
}

func TestPatchAllFollowsASymlinkInsteadOfReplacingIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "infra"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(dir, "infra", "compose.yml")
	if err := os.WriteFile(target, []byte("services:\n  db:\n    ports:\n      - \"5432:5432\"\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := "docker-compose.yml"
	if err := os.Symlink(filepath.Join("infra", "compose.yml"), filepath.Join(dir, link)); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	scan := Scan(ScanParams{ProjectDir: dir, File: link})
	if err := PatchAll(PatchAllParams{ProjectDir: dir, ByFile: map[string][]domain.ComposePortBinding{link: scan.Bindings}}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	info, err := os.Lstat(filepath.Join(dir, link))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file, forking the compose in two")
	}

	patched, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !strings.Contains(string(patched), `"${DB_PORT:-5432}:5432"`) {
		t.Errorf("the rewrite must land on the target:\n%s", patched)
	}
}
