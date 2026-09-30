package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

const namedCompose = `x-anchored: &shared pinned-by-anchor

services:
  postgres:
    image: postgres:16-alpine
    container_name: app-postgres
  redis:
    image: redis:7-alpine
    container_name: "${COMPOSE_PROJECT_NAME:-app}-redis"
  anchored:
    container_name: *shared
  plain:
    image: busybox

volumes:
  pgdata:
    name: app-pgdata
  cache: {}
  shared-with-others:
    name: team-cache
    external: true

networks:
  back:
    name: app-back
  front: {}
`

func findName(t *testing.T, scan domain.ComposeScan, kind domain.ComposeNameKind, owner string) domain.ComposeAbsoluteName {
	t.Helper()
	for _, n := range scan.Names {
		if n.Kind == kind && n.Owner == owner {
			return n
		}
	}
	t.Fatalf("no %s name for %q in %+v", kind, owner, scan.Names)
	return domain.ComposeAbsoluteName{}
}

func TestScanClassifiesAbsoluteNames(t *testing.T) {
	dir, file := writeCompose(t, namedCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file, Project: "app"})

	if scan.Err != "" {
		t.Fatalf("scan failed: %s", scan.Err)
	}

	t.Run("un container_name littéral est absolu et réécrit", func(t *testing.T) {
		n := findName(t, scan, domain.ComposeNameContainer, "postgres")
		if n.Status != domain.ComposeNameAbsolute {
			t.Fatalf("status = %s, want absolute", n.Status)
		}
		if n.Token != "app-postgres" {
			t.Errorf("token = %q", n.Token)
		}
		if n.Replacement != `"${COMPOSE_PROJECT_NAME:-app}-postgres"` {
			t.Errorf("replacement = %q", n.Replacement)
		}
	})

	t.Run("un container_name déjà interpolé est laissé tel quel", func(t *testing.T) {
		n := findName(t, scan, domain.ComposeNameContainer, "redis")
		if n.Status != domain.ComposeNameTemplated {
			t.Errorf("status = %s, want templated", n.Status)
		}
		if n.Replacement != "" {
			t.Errorf("a templated name must not be rewritten, got %q", n.Replacement)
		}
	})

	t.Run("un alias YAML est refusé plutôt que réécrit", func(t *testing.T) {
		n := findName(t, scan, domain.ComposeNameContainer, "anchored")
		if n.Status != domain.ComposeNameUnsupported {
			t.Errorf("status = %s, want unsupported", n.Status)
		}
		if n.Reason == "" {
			t.Error("an unsupported name must carry its reason")
		}
	})

	t.Run("un volume à name explicite est absolu", func(t *testing.T) {
		n := findName(t, scan, domain.ComposeNameVolume, "pgdata")
		if n.Status != domain.ComposeNameAbsolute {
			t.Fatalf("status = %s, want absolute", n.Status)
		}
		if n.Replacement != `"${COMPOSE_PROJECT_NAME:-app}-pgdata"` {
			t.Errorf("replacement = %q", n.Replacement)
		}
	})

	t.Run("un réseau à name explicite est absolu", func(t *testing.T) {
		n := findName(t, scan, domain.ComposeNameNetwork, "back")
		if n.Status != domain.ComposeNameAbsolute {
			t.Errorf("status = %s, want absolute", n.Status)
		}
	})

	t.Run("external est un partage voulu et n'est pas touché", func(t *testing.T) {
		for _, n := range scan.Names {
			if n.Owner == "shared-with-others" {
				t.Fatalf("an external volume must be left out entirely, got %+v", n)
			}
		}
	})

	t.Run("un owner sans name explicite n'apparaît pas", func(t *testing.T) {
		for _, n := range scan.Names {
			if n.Owner == "plain" || n.Owner == "cache" || n.Owner == "front" {
				t.Fatalf("%s pins no name and must not be reported", n.Owner)
			}
		}
	})
}

func TestPatchAllRewritesAbsoluteNames(t *testing.T) {
	dir, file := writeCompose(t, namedCompose)
	scan := Scan(ScanParams{ProjectDir: dir, File: file, Project: "app"})

	var absolute []domain.ComposeAbsoluteName
	for _, n := range scan.Names {
		if n.Status == domain.ComposeNameAbsolute {
			absolute = append(absolute, n)
		}
	}
	if len(absolute) != 3 {
		t.Fatalf("expected 3 absolute names, got %d", len(absolute))
	}

	if err := PatchAll(PatchAllParams{
		ProjectDir: dir,
		NamesByFile: map[string][]domain.ComposeAbsoluteName{
			file: absolute,
		},
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	patched := readCompose(t, dir, file)
	for _, want := range []string{
		`container_name: "${COMPOSE_PROJECT_NAME:-app}-postgres"`,
		`name: "${COMPOSE_PROJECT_NAME:-app}-pgdata"`,
		`name: "${COMPOSE_PROJECT_NAME:-app}-back"`,
	} {
		if !strings.Contains(patched, want) {
			t.Errorf("patched file is missing %q:\n%s", want, patched)
		}
	}

	t.Run("ce qui n'était pas visé est intact", func(t *testing.T) {
		for _, want := range []string{
			`container_name: "${COMPOSE_PROJECT_NAME:-app}-redis"`,
			`container_name: *shared`,
			`name: team-cache`,
			"x-anchored: &shared pinned-by-anchor",
		} {
			if !strings.Contains(patched, want) {
				t.Errorf("patched file lost %q:\n%s", want, patched)
			}
		}
	})
}

func TestPatchAllRewritesPortsAndNamesTogether(t *testing.T) {
	const both = `services:
  postgres:
    container_name: app-postgres
    ports:
      - "5432:5432"
`
	dir, file := writeCompose(t, both)
	scan := Scan(ScanParams{ProjectDir: dir, File: file, Project: "app"})

	if err := PatchAll(PatchAllParams{
		ProjectDir:  dir,
		ByFile:      map[string][]domain.ComposePortBinding{file: scan.Bindings},
		NamesByFile: map[string][]domain.ComposeAbsoluteName{file: scan.Names},
	}); err != nil {
		t.Fatalf("patch: %v", err)
	}

	patched := readCompose(t, dir, file)
	if !strings.Contains(patched, `container_name: "${COMPOSE_PROJECT_NAME:-app}-postgres"`) {
		t.Errorf("name not patched:\n%s", patched)
	}
	if !strings.Contains(patched, `"${POSTGRES_PORT:-5432}:5432"`) {
		t.Errorf("port not patched:\n%s", patched)
	}
}

func readCompose(t *testing.T, dir, file string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatalf("read patched fixture: %v", err)
	}
	return string(content)
}
