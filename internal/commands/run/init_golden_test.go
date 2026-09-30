package run

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

var updateInitGolden = flag.Bool("update-init-golden", false, "rewrite the run init golden files")

// initGoldenCase pins what `wtm run init` writes and prints for one repository.
// The goldens were captured before the command moved into internal/flow and
// must pass unchanged after it: they are the proof the move changed nothing.
type initGoldenCase struct {
	name     string
	files    map[string]string
	envFiles []domain.EnvFile
	runTOML  string
	args     []string
}

func TestRunInitGolden(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range initGoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := runInitGolden(t, tc)
			path := filepath.Join(wd, "testdata", "initgolden", tc.name+".golden")
			if *updateInitGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update-init-golden to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("run init output drifted from %s\n--- got ---\n%s", path, got)
			}
		})
	}
}

func runInitGolden(t *testing.T, tc initGoldenCase) string {
	t.Helper()
	shortHome(t)
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Chdir(dir)

	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers: domain.InitProjectAnswers{
			BasePath:    "../.trees",
			BaseBranch:  "main",
			EnvStrategy: domain.EnvStrategyParent,
			EnvFiles:    tc.envFiles,
		},
	}); err != nil {
		t.Fatalf("setup project: %v", err)
	}
	for rel, content := range tc.files {
		writeProjectFile(t, rel, content)
	}
	gittest.Git(t, dir, "add", "-A")
	gittest.Git(t, dir, "commit", "-q", "-m", "fixture")
	if tc.runTOML != "" {
		if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte(tc.runTOML), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	before := snapshotTree(t, dir)
	stdout, stderr, err := runCmd(t, append([]string{domain.CmdInit}, tc.args...)...)
	after := snapshotTree(t, dir)

	var b strings.Builder
	b.WriteString("### args: " + strings.Join(tc.args, " ") + "\n")
	if err != nil {
		b.WriteString("### error: " + err.Error() + "\n")
	}
	b.WriteString("### stdout\n" + stdout + "\n### stderr\n" + stderr + "\n")
	paths := make([]string, 0, len(after))
	for path := range after {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if before[path] == after[path] {
			continue
		}
		b.WriteString("### file " + path + "\n" + after[path] + "\n")
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			b.WriteString("### removed " + path + "\n")
		}
	}

	normalized := b.String()
	for _, prefix := range []string{"/private" + dir, dir} {
		normalized = strings.ReplaceAll(normalized, prefix, "<repo>")
	}
	return normalized
}

// snapshotTree reads every file of the repository an init may write: the
// working tree, and wtm's own state under .git/wtm.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if entry.IsDir() {
			if rel == ".git" {
				return nil
			}
			if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) && rel != filepath.Join(".git", "wtm") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Dir(rel) == ".git" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return files
}

func initGoldenCases() []initGoldenCase {
	monorepo := monorepoFixture()
	monorepoEnv := monorepoEnvFiles()
	return []initGoldenCase{
		{name: "monorepo_yes", files: monorepo, envFiles: monorepoEnv, args: []string{"--yes"}},
		{name: "monorepo_all_flags", files: monorepo, envFiles: monorepoEnv,
			args: []string{"--yes", "--" + domain.FlagPatchCompose, "--" + domain.FlagLinkEnv, "--" + domain.FlagWritePortKeys}},
		{name: "monorepo_json", files: monorepo, envFiles: monorepoEnv, args: []string{"--yes", "--output", "json"}},
		{name: "single_package", files: singlePackageFixture(), envFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
			args: []string{"--yes", "--" + domain.FlagLinkEnv, "--" + domain.FlagWritePortKeys}},
		{name: "reinit_hand_set", files: monorepo, envFiles: monorepoEnv, runTOML: handSetRunTOML,
			args: []string{"--yes", "--" + domain.FlagPatchCompose, "--" + domain.FlagLinkEnv}},
		{name: "shared_postgres_namespace", files: sharedPostgresFixture(), envFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
			runTOML: sharedPostgresRunTOML,
			args:    []string{"--yes", "--" + domain.FlagPatchCompose, "--" + domain.FlagLinkEnv, "--" + domain.FlagWritePortKeys}},
		{name: "literal_compose_declined", files: sharedPostgresFixture(), envFiles: []domain.EnvFile{{Target: ".env", Template: ".env.example"}},
			args: []string{"--yes"}},
		{name: "nothing_detected", files: map[string]string{"README.md": "# empty\n"}, args: []string{"--yes"}},
	}
}

const monorepoCompose = `name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}"

services:
  postgres:
    image: postgres:18.4-alpine
    container_name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-postgres"
    environment:
      POSTGRES_USER: monorepo-exemple-wtm
      POSTGRES_PASSWORD: monorepo-exemple-wtm
      POSTGRES_DB: monorepo-exemple-wtm
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    volumes:
      - postgres-data:/var/lib/postgresql

  redis:
    image: redis:7-alpine
    container_name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-redis"
    ports:
      - "${REDIS_PORT:-6379}:6379"

  mailpit:
    image: axllent/mailpit:latest
    container_name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-mailpit"
    ports:
      - "${MAILPIT_SMTP_PORT:-1025}:1025"
      - "${MAILPIT_UI_PORT:-8025}:8025"

  minio:
    image: minio/minio:latest
    container_name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-minio"
    command: server /data --console-address ":9001"
    ports:
      - "${MINIO_PORT:-9000}:9000"
      - "${MINIO_CONSOLE_PORT:-9001}:9001"
    volumes:
      - minio-data:/data

  adminer:
    image: adminer:latest
    container_name: "${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-adminer"
    ports:
      - "${ADMINER_PORT:-8080}:8080"

volumes:
  postgres-data:
  minio-data:
`

const monorepoRootEnv = `COMPOSE_PROJECT_NAME=monorepo-exemple-wtm
POSTGRES_PORT=5432
REDIS_PORT=6379
MAILPIT_SMTP_PORT=1025
MAILPIT_UI_PORT=8025
MINIO_PORT=9000
MINIO_CONSOLE_PORT=9001
ADMINER_PORT=8080
`

func monorepoFixture() map[string]string {
	files := map[string]string{
		".gitignore":                 ".env\nnode_modules\n",
		"package.json":               `{"name":"monorepo-exemple-wtm","private":true,"scripts":{"build":"turbo run build","dev":"turbo run dev","dev:shop":"turbo run dev --filter='./apps/shop/*'","dev:crm":"turbo run dev --filter='./apps/crm/*'","lint":"turbo run lint","check-types":"turbo run check-types"},"packageManager":"pnpm@9.0.0"}`,
		"pnpm-workspace.yaml":        "packages:\n  - \"apps/*/*\"\n  - \"packages/*\"\n",
		"pnpm-lock.yaml":             "lockfileVersion: '9.0'\n",
		"turbo.json":                 `{"tasks":{"build":{},"dev":{"cache":false,"persistent":true}}}`,
		"docker-compose.yml":         monorepoCompose,
		".env.example":               monorepoRootEnv,
		".env":                       monorepoRootEnv,
		"packages/core/package.json": `{"name":"@repo/core","scripts":{"check-types":"tsc --noEmit"}}`,
	}
	for i, app := range []string{"shop", "crm"} {
		api := 4001 + i
		web, admin := 5173+2*i, 5174+2*i
		apiEnv := "PORT=" + strconv.Itoa(api) + "\nCORS_ORIGIN=http://localhost:" + strconv.Itoa(web) + ",http://localhost:" + strconv.Itoa(admin) + "\n\n" +
			"DATABASE_URL=postgresql://u:p@localhost:5432/monorepo-exemple-wtm\n" +
			"BETTER_AUTH_URL=http://localhost:" + strconv.Itoa(api) + "\n"
		files["apps/"+app+"/api/package.json"] = `{"name":"@` + app + `/api","scripts":{"dev":"tsx watch src/index.ts","build":"tsc","start":"node dist/index.js","db:migrate":"drizzle-kit migrate","db:seed":"tsx src/db/seed.ts"}}`
		files["apps/"+app+"/api/.env.example"] = apiEnv
		files["apps/"+app+"/api/.env"] = apiEnv
		for _, front := range []struct {
			name string
			port int
		}{{"web", web}, {"admin", admin}} {
			env := "VITE_PORT=" + strconv.Itoa(front.port) + "\nVITE_API_URL=http://localhost:" + strconv.Itoa(api) + "\n"
			files["apps/"+app+"/"+front.name+"/package.json"] = `{"name":"@` + app + `/` + front.name + `","scripts":{"dev":"vite","build":"tsc -b && vite build","preview":"vite preview"}}`
			files["apps/"+app+"/"+front.name+"/.env.example"] = env
			files["apps/"+app+"/"+front.name+"/.env"] = env
		}
	}
	return files
}

func monorepoEnvFiles() []domain.EnvFile {
	files := []domain.EnvFile{{Target: ".env", Template: ".env.example"}}
	for _, app := range []string{"crm", "shop"} {
		for _, part := range []string{"admin", "api", "web"} {
			dir := "apps/" + app + "/" + part + "/"
			files = append(files, domain.EnvFile{Target: dir + ".env", Template: dir + ".env.example"})
		}
	}
	return files
}

func singlePackageFixture() map[string]string {
	return map[string]string{
		".gitignore":        ".env\n",
		"package.json":      `{"name":"solo","scripts":{"dev":"next dev","build":"next build","start":"next start","lint":"eslint ."}}`,
		"package-lock.json": `{"lockfileVersion":3}`,
		".env.example":      "PORT=3000\nNEXT_PUBLIC_URL=http://localhost:3000\n",
		".env":              "PORT=3000\nNEXT_PUBLIC_URL=http://localhost:3000\n",
	}
}

const sharedPostgresCompose = `services:
  postgres:
    image: postgres:16-alpine
    container_name: app-postgres
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
  cache:
    image: redis:7-alpine
    ports:
      - "6379:6379"
volumes:
  pgdata:
    name: app-pgdata
`

func sharedPostgresFixture() map[string]string {
	env := "DB_PORT=5432\nDATABASE_URL=postgresql://u:p@localhost:5432/app\nPORT=4000\n"
	return map[string]string{
		".gitignore":         ".env\n",
		"docker-compose.yml": sharedPostgresCompose,
		"package.json":       `{"name":"app","scripts":{"dev":"node server.js","db:migrate":"prisma migrate deploy"}}`,
		"pnpm-lock.yaml":     "lockfileVersion: '9.0'\n",
		".env.example":       env,
		".env":               env,
	}
}

const sharedPostgresRunTOML = `[[job]]
name = "postgres"
kind = "service"
cmd = "docker compose -f docker-compose.yml up postgres"
scope = "shared"

[job.namespace]
name = "app_{worktree}"
create = "createdb -h localhost -p $DB_PORT $WTM_NAMESPACE"
remove = "dropdb -h localhost -p $DB_PORT $WTM_NAMESPACE"
`

const handSetRunTOML = `addressing = "names"
concurrency = "parallel"

[[job]]
name = "shop-api-dev"
kind = "service"
cmd = "pnpm --filter @shop/api exec tsx watch src/index.ts --port ${PORT}"
cwd = "apps/shop/api"
ports = { PORT = 4101 }
url = { port = "PORT" }

[[job]]
name = "tunnel"
kind = "service"
cmd = "cloudflared tunnel run"

[[profile]]
name = "shop"
jobs = ["shop-api-dev", "tunnel"]
default = true
`
