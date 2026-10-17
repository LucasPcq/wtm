package run

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// TestRunUp_NotInitialized asserts a blocked run command fails with the opt-in
// guard (ErrRunNotInitialized) when run.toml declares no job/profile.
func TestRunUp_NotInitialized(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t, domain.CmdUp)
	if err == nil {
		t.Fatal("expected error on uninitialized run module")
	}
	if !errors.Is(err, domain.ErrRunNotInitialized) {
		t.Errorf("expected ErrRunNotInitialized, got: %v", err)
	}
}

// TestRunInit_YesAutoGenerates verifies `wtm run init --yes`
// turns a detected docker-compose file into a run.toml job without prompting.
func TestRunInit_YesAutoGenerates(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv(domain.EnvProjectDir)
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write docker-compose: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("expected one docker service job, got %+v", cfg.Jobs)
	}
}

// TestRunInit_ReRunMergesAndPreservesProfiles verifies re-running init is
// additive: existing jobs/profiles are kept and detected jobs merge in without
// duplication.
func TestRunInit_ReRunMergesAndPreservesProfiles(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"api"}, Default: true},
		},
	})

	projectDir := os.Getenv(domain.EnvProjectDir)
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write docker-compose: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}

	// The pre-existing job survives and the detected docker job is added.
	if len(cfg.Jobs) != 2 {
		t.Errorf("expected 2 jobs after merge, got %+v", cfg.Jobs)
	}
	found := false
	for _, j := range cfg.Jobs {
		if j.Name == "api" {
			found = true
		}
	}
	if !found {
		t.Error("existing job 'api' was dropped by re-run")
	}
	// The profile is untouched.
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "dev" || !cfg.Profiles[0].Default {
		t.Errorf("profile not preserved: %+v", cfg.Profiles)
	}
}

const composeWithPorts = `services:
  postgres:                    # la base
    image: postgres:16-alpine
    ports:
      - "5432:5432"
  cache:
    ports:
      - "${CACHE_PORT:-6379}:6379"
  edge:
    ports:
      - "3000-3005:3000-3005"
`

// `run init --yes` ne coche que les scripts qu'il aurait
// pré-cochés. Lire cette absence comme un décochage supprimerait les jobs
// qu'un run précédent avait configurés — une perte silencieuse.
func TestRunInit_YesNeSupprimeAucunJob(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm run build", Cwd: "."},
			{Name: "tunnel", Kind: domain.JobKindService, Cmd: "cloudflared tunnel run"},
		},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"build"}}},
	})
	writeProjectFile(t, "package.json", `{"scripts":{"dev":"vite","build":"vite build"}}`)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	for _, name := range []string{"build", "tunnel"} {
		if _, found := jobByName(cfg, name); !found {
			t.Errorf("job %q a disparu — jobs restants %+v", name, cfg.Jobs)
		}
	}
}

func writeCompose(t *testing.T, name, content string) string {
	t.Helper()
	projectDir := os.Getenv(domain.EnvProjectDir)
	if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return filepath.Join(projectDir, name)
}

func dockerJobPorts(t *testing.T, cfg domain.RunConfig) map[string]int {
	t.Helper()
	for _, j := range cfg.Jobs {
		if strings.Contains(j.Cmd, "docker") {
			return j.Ports
		}
	}
	t.Fatalf("no docker job in %+v", cfg.Jobs)
	return nil
}

// TestRunInit_DeclaresTemplatedPortsAndWithholdsFrozenOnes verifies the
// detection only declares what it can actually isolate: a mapping already
// reading a variable is picked up, a literal one is reported and left out, and
// no project file is touched without --patch-compose.
func TestRunInit_DeclaresTemplatedPortsAndWithholdsFrozenOnes(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := dockerJobPorts(t, cfg)
	if len(ports) != 1 || ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want only the templated CACHE_PORT=6379", ports)
	}

	after, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	if string(after) != composeWithPorts {
		t.Errorf("the compose file must be untouched without --%s:\n%s", domain.FlagPatchCompose, after)
	}

	for _, want := range []string{"5432:5432", "POSTGRES_PORT", "run job edit", "range"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report must mention %q, got:\n%s", want, stdout)
		}
	}
}

// TestRunInit_PatchComposeTemplatizesAndDeclares verifies --patch-compose
// rewrites the literal mappings and declares the ports they now read, leaving
// the rest of the file byte for byte as it was.
func TestRunInit_PatchComposeTemplatizesAndDeclares(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes, "--"+domain.FlagPatchCompose); err != nil {
		t.Fatalf("run init: %v", err)
	}

	after, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}
	got := string(after)
	if !strings.Contains(got, `- "${POSTGRES_PORT:-5432}:5432"`) {
		t.Errorf("the literal mapping was not templatized:\n%s", got)
	}
	for _, untouched := range []string{"  postgres:                    # la base", "image: postgres:16-alpine", `- "3000-3005:3000-3005"`} {
		if !strings.Contains(got, untouched) {
			t.Errorf("the rewrite must leave %q alone:\n%s", untouched, got)
		}
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	ports := dockerJobPorts(t, cfg)
	if ports["POSTGRES_PORT"] != 5432 || ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want POSTGRES_PORT=5432 and CACHE_PORT=6379", ports)
	}
}

// TestRunInit_BackfillsAJobConfiguredBeforePortsExisted verifies re-running
// init gives an existing compose job the ports it never declared, without
// touching anything else about it.
func TestRunInit_BackfillsAJobConfiguredBeforePortsExisted(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", composeWithPorts)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "docker-compose",
			Kind: domain.JobKindService,
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
			Cwd:  ".",
		}},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("the existing job must not be duplicated, got %+v", cfg.Jobs)
	}
	job := cfg.Jobs[0]
	if job.Ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want the detected CACHE_PORT backfilled", job.Ports)
	}
	if job.Cmd != "docker compose -f docker-compose.yml up -d" || job.Cwd != "." {
		t.Errorf("backfill must touch nothing but ports, got %+v", job)
	}
}

// TestRunInit_NeverWritesAConfigItsOwnLoaderRefuses verifies two compose files
// exposing the same host port produce a loadable run.toml: the second
// declaration is withdrawn and reported rather than written.
func TestRunInit_NeverWritesAConfigItsOwnLoaderRefuses(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", "services:\n  a:\n    ports:\n      - \"${A_PORT:-5432}:5432\"\n")
	writeCompose(t, "docker-compose.other.yml", "services:\n  b:\n    ports:\n      - \"${B_PORT:-5442}:5432\"\n")

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("the written config must load: %v", err)
	}

	// Both sides were inferred, so wtm has nothing to arbitrate on and withdraws
	// the pair rather than picking a winner.
	declared := 0
	for _, j := range cfg.Jobs {
		declared += len(j.Ports)
	}
	if declared != 0 {
		t.Errorf("two colliding detected bases must both be withdrawn, got %+v", cfg.Jobs)
	}
	for _, want := range []string{"withdrawn", "A_PORT", "B_PORT"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report must name %q, got:\n%s", want, stdout)
		}
	}
}

// TestRunInit_BackfillsAJobWhoseNameWasChanged verifies a compose file already
// run by a job does not get a second one just because that job was renamed —
// the two would declare the same ports and both lose them to the collision
// check, leaving the file with no isolation at all.
func TestRunInit_BackfillsAJobWhoseNameWasChanged(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", composeWithPorts)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "docker",
			Kind: domain.JobKindService,
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
			Cwd:  ".",
		}},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 || cfg.Jobs[0].Name != "docker" {
		t.Fatalf("the renamed job must stay the only one, got %+v", cfg.Jobs)
	}
	if cfg.Jobs[0].Ports["CACHE_PORT"] != 6379 {
		t.Errorf("ports = %v, want the detected port backfilled onto it", cfg.Jobs[0].Ports)
	}
}

// TestRunInit_TwoComposeFilesOnTheSameBase is the case the spec names: the two
// declarations meet inside a single worktree, not some worktrees apart.
func TestRunInit_TwoComposeFilesOnTheSameBase(t *testing.T) {
	stateDir := setupTestProject(t)
	writeCompose(t, "docker-compose.yml", "services:\n  a:\n    ports:\n      - \"${A_PORT:-5432}:5432\"\n")
	writeCompose(t, "docker-compose.other.yml", "services:\n  b:\n    ports:\n      - \"${B_PORT:-5432}:5432\"\n")

	stdout, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("the written config must load: %v", err)
	}
	for _, j := range cfg.Jobs {
		if len(j.Ports) != 0 {
			t.Errorf("job %q kept %v; two identical bases can never coexist", j.Name, j.Ports)
		}
	}
	if !strings.Contains(stdout, "withdrawn") {
		t.Errorf("the withdrawal must be reported, got:\n%s", stdout)
	}
}

// TestRunInit_PatchComposeIsIdempotent verifies a second run neither rewrites
// the file again nor duplicates anything.
func TestRunInit_PatchComposeIsIdempotent(t *testing.T) {
	stateDir := setupTestProject(t)
	composePath := writeCompose(t, "docker-compose.yml", composeWithPorts)

	flags := []string{domain.CmdInit, "--" + domain.FlagYes, "--" + domain.FlagPatchCompose}
	if _, _, err := runCmd(t, flags...); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	firstCfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if _, _, err := runCmd(t, flags...); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	secondCfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("the compose file changed on the second run:\n%s", second)
	}
	if len(firstCfg.Jobs) != len(secondCfg.Jobs) {
		t.Errorf("jobs went from %d to %d", len(firstCfg.Jobs), len(secondCfg.Jobs))
	}
}

// writeProjectFile writes a file under the project dir, creating parents.
func writeProjectFile(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(os.Getenv(domain.EnvProjectDir), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func jobByName(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job, true
		}
	}
	return domain.JobConfig{}, false
}

// TestRunInit_DeclaresPortsFromEnvFiles covers the pnpm monorepo case: each
// dev server takes the port its own package's env file declares, and the root
// .env never reaches a package that has none of its own.
func TestRunInit_DeclaresPortsFromEnvFiles(t *testing.T) {
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"pnpm -r dev"}}`)
	writeProjectFile(t, "pnpm-workspace.yaml", "packages:\n  - \"apps/*\"\n")
	writeProjectFile(t, ".env", "PORT=3000\n")

	writeProjectFile(t, "apps/web/package.json", `{"name":"web","scripts":{"dev":"vite"}}`)
	writeProjectFile(t, "apps/web/.env", "PORT=5173\nDATABASE_URL=postgres://localhost:5432/app\n")

	writeProjectFile(t, "apps/api/package.json", `{"name":"api","scripts":{"dev":"node server.js","build":"tsc"}}`)
	writeProjectFile(t, "apps/api/.env.example", "API_PORT=4000\n")

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}

	web, found := jobByName(cfg, "web-dev")
	if !found {
		t.Fatalf("no web-dev job in %+v", cfg.Jobs)
	}
	if web.Ports["PORT"] != 5173 {
		t.Errorf("web-dev ports = %v, want PORT=5173 from apps/web/.env", web.Ports)
	}

	api, found := jobByName(cfg, "api-dev")
	if !found {
		t.Fatalf("no api-dev job in %+v", cfg.Jobs)
	}
	if api.Ports["API_PORT"] != 4000 {
		t.Errorf("api-dev ports = %v, want API_PORT=4000 from the committed template", api.Ports)
	}
	if _, leaked := api.Ports["PORT"]; leaked {
		t.Errorf("api-dev inherited the root PORT: %v", api.Ports)
	}

	if root, found := jobByName(cfg, "dev"); found && root.Ports["PORT"] != 3000 {
		t.Errorf("root dev ports = %v, want PORT=3000", root.Ports)
	}

	if build, found := jobByName(cfg, "api-build"); found && len(build.Ports) > 0 {
		t.Errorf("a task binds nothing, got %v", build.Ports)
	}
}

// TestRunInit_EnvPortsAreIdempotent asserts a second run changes nothing.
func TestRunInit_EnvPortsAreIdempotent(t *testing.T) {
	stateDir := setupTestProject(t)

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"next dev"}}`)
	writeProjectFile(t, ".env", "PORT=3000\n")

	for i := 0; i < 2; i++ {
		if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
			t.Fatalf("run init #%d: %v", i+1, err)
		}
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("jobs = %+v, want exactly one", cfg.Jobs)
	}
	if cfg.Jobs[0].Ports["PORT"] != 3000 {
		t.Errorf("ports = %v, want PORT=3000", cfg.Jobs[0].Ports)
	}
}

// TestRunInit_HandWrittenPortSurvivesDetection asserts a base already in
// run.toml outranks the one the .env declares.
func TestRunInit_HandWrittenPortSurvivesDetection(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: ".", Ports: map[string]int{"PORT": 7777}},
		},
	})

	writeProjectFile(t, "package.json", `{"name":"root","scripts":{"dev":"next dev"}}`)
	writeProjectFile(t, ".env", "PORT=3000\n")

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	job, _ := jobByName(cfg, "dev")
	if job.Ports["PORT"] != 7777 {
		t.Errorf("PORT = %d, want the hand-written 7777", job.Ports["PORT"])
	}
}

// TestRunInit_ProducesAStartableConfig is the whole point of the rework: an
// init that writes a configuration, not an inventory. Before it, this fixture
// produced five jobs and no profile, so `run up` started the linter, the build
// and the production server alongside the dev one.
func TestRunInit_ProducesAStartableConfig(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv(domain.EnvProjectDir)

	writeCompose(t, "docker-compose.yml", "services:\n  db:\n    image: alpine\n    ports:\n      - \"5432:5432\"\n")
	pkg := `{"name":"demo","scripts":{"dev":"vite","build":"tsc","lint":"eslint .","start":"node dist/i.js"}}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}

	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected compose + dev only, got %+v", cfg.Jobs)
	}
	for _, job := range cfg.Jobs {
		switch job.Name {
		case "build", "lint", "start":
			t.Errorf("%s ne doit pas devenir un job", job.Name)
		}
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected exactly one profile, got %+v", cfg.Profiles)
	}
	if !cfg.Profiles[0].Default {
		t.Error("le profil unique doit porter Default")
	}
	for _, job := range cfg.Jobs {
		if !hasProfileJob(cfg.Profiles[0], job.Name) {
			t.Errorf("%s n'est dans aucun profil, donc run up ne le lancera pas", job.Name)
		}
	}
}

func hasProfileJob(profile domain.ProfileConfig, name string) bool {
	for _, job := range profile.Jobs {
		if job == name {
			return true
		}
	}
	return false
}

// Un init non interactif publie ce que le wizard aurait pré-coché : le service
// qui déclare le port qu'il écoute répond sous son propre nom, sans qu'il faille
// éditer run.toml après coup.
func TestRunInitPublishesTheServiceItPorts(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv(domain.EnvProjectDir)
	pkg := `{"name":"app","scripts":{"dev":"vite --port ${PORT}"}}`
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte("PORT=3000\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	job := findJob(t, cfg, "dev")
	if job.URL == nil {
		t.Fatalf("job dev n'a pas d'url : %+v", job)
	}
	if _, ok := job.Ports[job.URL.Port]; !ok {
		t.Errorf("url.port = %q, absent des ports du job %v", job.URL.Port, job.Ports)
	}
}

// Un job dont aucun port n'est celui qu'il écoute ne reçoit pas de nom : le
// publier annoncerait une adresse que rien ne sert.
func TestRunInitLeavesADialledPortUnpublished(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "api", Kind: domain.JobKindService, Cmd: "echo hi", Ports: map[string]int{"DB_PORT": 5432}},
		},
	})

	if _, _, err := runCmd(t, domain.CmdInit, "--"+domain.FlagYes); err != nil {
		t.Fatalf("run init: %v", err)
	}

	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if job := findJob(t, cfg, "api"); job.URL != nil {
		t.Errorf("url = %+v, want aucune : DB_PORT est un port composé, pas écouté", job.URL)
	}
}

func findJob(t *testing.T, cfg domain.RunConfig, name string) domain.JobConfig {
	t.Helper()
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return job
		}
	}
	t.Fatalf("job %q absent de %+v", name, cfg.Jobs)
	return domain.JobConfig{}
}

func TestRunInit_NoLongerAcceptsNonInteractive(t *testing.T) {
	setupTestProject(t)

	_, _, err := runCmd(t, domain.CmdInit, "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %v, want an unknown flag", err)
	}
}

func TestRunInit_YShorthandRunsUnattended(t *testing.T) {
	stateDir := setupTestProject(t)
	projectDir := os.Getenv(domain.EnvProjectDir)
	if err := os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := runCmd(t, domain.CmdInit, "-y"); err != nil {
		t.Fatalf("run init -y: %v", err)
	}
	cfg, err := config.LoadRun(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("jobs = %+v, want the detected compose file", cfg.Jobs)
	}
}
