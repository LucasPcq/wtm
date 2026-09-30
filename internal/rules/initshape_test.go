package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

// La forme rapportée dans LUC-208, de bout en bout : un monorepo pnpm dont les
// packages sont deux niveaux sous apps/, avec des seeds et des migrations qu'un
// service attend. Détection, construction du run.toml et découpage en profils
// se répondent, et un test sur chacun pris isolément ne le montre pas.
func TestInitShapeOfADeepMonorepo(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pnpm-workspace.yaml", "packages:\n  - \"apps/**\"\n")
	write("package.json", `{"name":"root"}`)
	write("apps/app-1/back/package.json",
		`{"name":"@acme/back","scripts":{"dev":"nest start --watch","seed":"ts-node seed.ts","migration:run":"typeorm migration:run"}}`)
	write("apps/app-1/front/package.json", `{"name":"@acme/front","scripts":{"dev":"vite"}}`)

	scripts := detect.PackageJSONScripts(dir)
	if len(scripts) != 4 {
		t.Fatalf("scripts = %d, want 4: un package deux niveaux sous apps/ doit être vu", len(scripts))
	}

	selected := make([]domain.PackageScript, 0, len(scripts))
	for _, script := range scripts {
		script.Kind = rules.ClassifyScriptKind(script.Name)
		selected = append(selected, script)
	}

	cfg := rules.BuildInitRunConfig(
		domain.InitProjectAnswers{SelectedPackageScripts: selected},
		domain.PkgManagerPnpm,
	)
	cfg.Profiles = rules.ProposeProfiles(rules.ProposeProfilesParams{Config: cfg})

	back := findProfile(t, cfg.Profiles, "back")
	want := "back-migration:run,back-seed,back-dev"
	if got := strings.Join(back.Jobs, ","); got != want {
		t.Errorf("profil back = %s, want %s: les tasks entrent dans le profil, avant le service", got, want)
	}

	all := findProfile(t, cfg.Profiles, domain.ProfileAllName)
	for i, name := range all.Jobs[:2] {
		if !strings.Contains(name, "migration") && !strings.Contains(name, "seed") {
			t.Errorf("all[%d] = %s; les tasks ouvrent le profil global", i, name)
		}
	}
}
