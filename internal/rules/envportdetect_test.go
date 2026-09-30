package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestIsPortKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"PORT", true},
		{"DB_PORT", true},
		{"API_PORT", true},
		{"_PORT", true},
		{"PORTAL", false},
		{"EXPORT", false},
		{"PORT_RANGE", false},
		{"port", false},
		{"", false},
		{"MY-PORT", false},
	}

	for _, c := range cases {
		if got := IsPortKey(c.key); got != c.want {
			t.Errorf("IsPortKey(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestExtractEnvPortsKeepsOnlyUsableValues(t *testing.T) {
	lines := ParseEnv(`# a comment
PORT=3000
DB_PORT="5432"
EMPTY_PORT=
PLACEHOLDER_PORT=<your-port>
HUGE_PORT=99999
ZERO_PORT=0
export API_PORT=4000
NOT_A_PORT=hello
DATABASE_URL=postgres://localhost:5432/app
`)

	got := ExtractEnvPorts(lines)
	want := map[string]int{"PORT": 3000, "DB_PORT": 5432, "API_PORT": 4000}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractEnvPorts = %v, want %v", got, want)
	}
}

func TestExtractEnvPortsIgnoresPortsInsideURLs(t *testing.T) {
	lines := ParseEnv("VITE_API_URL=http://localhost:8080\n")

	if got := ExtractEnvPorts(lines); len(got) != 0 {
		t.Errorf("a URL names the service consumed, not the one exposed; got %v", got)
	}
}

func TestMergeEnvPortSourcesPrefersLocalThenValueThenTemplate(t *testing.T) {
	scan := MergeEnvPortSources(MergeEnvPortSourcesParams{
		Dir: "apps/web",
		Sources: []EnvPortSource{
			{File: "apps/web/.env.example", Ports: map[string]int{"PORT": 3000, "TEMPLATE_ONLY_PORT": 9000}},
			{File: "apps/web/.env", Ports: map[string]int{"PORT": 3100, "ENV_ONLY_PORT": 8000}},
			{File: "apps/web/.env.local", Ports: map[string]int{"PORT": 3200}},
		},
	})

	wantPorts := map[string]int{"PORT": 3200, "ENV_ONLY_PORT": 8000, "TEMPLATE_ONLY_PORT": 9000}
	if !reflect.DeepEqual(scan.Ports, wantPorts) {
		t.Errorf("ports = %v, want %v", scan.Ports, wantPorts)
	}

	wantSources := map[string]string{
		"PORT":               "apps/web/.env.local",
		"ENV_ONLY_PORT":      "apps/web/.env",
		"TEMPLATE_ONLY_PORT": "apps/web/.env.example",
	}
	if !reflect.DeepEqual(scan.SourceByVar, wantSources) {
		t.Errorf("sources = %v, want %v", scan.SourceByVar, wantSources)
	}
}

func serviceScript(name, workspace, pkg string) domain.PackageScript {
	return domain.PackageScript{Name: name, Workspace: workspace, PkgName: pkg, Kind: domain.JobKindService}
}

func TestBackfillScriptPortsDeclaresPerWorkspace(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/web"},
		{Name: "api-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/api"},
	}}

	result := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts: []domain.PackageScript{
			serviceScript("dev", "apps/web", "web"),
			serviceScript("dev", "apps/api", "api"),
		},
		ScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {Dir: "apps/web", Ports: map[string]int{"PORT": 3000}},
			"apps/api": {Dir: "apps/api", Ports: map[string]int{"API_PORT": 4000}},
		},
	})

	if got := result.Config.Jobs[0].Ports; !reflect.DeepEqual(got, map[string]int{"PORT": 3000}) {
		t.Errorf("web-dev ports = %v", got)
	}
	if got := result.Config.Jobs[1].Ports; !reflect.DeepEqual(got, map[string]int{"API_PORT": 4000}) {
		t.Errorf("api-dev ports = %v", got)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want both jobs", result.Added)
	}
}

func TestBackfillScriptPortsNeverOverwritesADeclaredVar(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/web", Ports: map[string]int{"PORT": 7777}},
	}}

	result := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{serviceScript("dev", "apps/web", "web")},
		ScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {Dir: "apps/web", Ports: map[string]int{"PORT": 3000}},
		},
	})

	if got := result.Config.Jobs[0].Ports["PORT"]; got != 7777 {
		t.Errorf("PORT = %d, want the hand-written 7777 to survive a re-run", got)
	}
	if len(result.Added) != 0 {
		t.Errorf("Added = %v, want nothing added", result.Added)
	}
}

func TestBackfillScriptPortsSkipsTasks(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm run build", Cwd: "apps/web"},
	}}

	result := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{{Name: "build", Workspace: "apps/web", PkgName: "web", Kind: domain.JobKindTask}},
		ScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {Dir: "apps/web", Ports: map[string]int{"PORT": 3000}},
		},
	})

	if result.Config.Jobs[0].Ports != nil {
		t.Errorf("a task binds nothing, got %v", result.Config.Jobs[0].Ports)
	}
}

// The root .env and a docker-compose job share cwd ".", so matching on the
// directory alone would hand the root PORT to the compose job.
func TestBackfillScriptPortsLeavesComposeJobsAlone(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{
			Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".",
			Cmd:  "docker compose -f docker-compose.yml up -d",
			Stop: "docker compose -f docker-compose.yml down --remove-orphans",
		},
	}}

	result := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        nil,
		ScansByDir: map[string]domain.EnvPortScan{
			".": {Dir: ".", Ports: map[string]int{"PORT": 3000}},
		},
	})

	if result.Config.Jobs[0].Ports != nil {
		t.Errorf("compose jobs get their ports from the compose file, got %v", result.Config.Jobs[0].Ports)
	}
}

func TestBackfillScriptPortsIgnoresAnUnselectedScript(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/web"},
	}}

	result := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{serviceScript("dev", "apps/api", "api")},
		ScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {Dir: "apps/web", Ports: map[string]int{"PORT": 3000}},
		},
	})

	if result.Config.Jobs[0].Ports != nil {
		t.Errorf("only a selected script carries its workspace's ports, got %v", result.Config.Jobs[0].Ports)
	}
}

func TestBackfillScriptPortsDoesNotMutateTheInputConfig(t *testing.T) {
	original := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web-dev", Kind: domain.JobKindService, Cmd: "pnpm run dev", Cwd: "apps/web"},
	}}

	BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         original,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{serviceScript("dev", "apps/web", "web")},
		ScansByDir: map[string]domain.EnvPortScan{
			"apps/web": {Dir: "apps/web", Ports: map[string]int{"PORT": 3000}},
		},
	})

	if original.Jobs[0].Ports != nil {
		t.Errorf("the caller's config was mutated: %v", original.Jobs[0].Ports)
	}
}

// A root .env holds the compose stack's ports. Every script sharing that
// directory used to claim them, and the collision resolver then kept whichever
// copy it saw last — leaving `docker compose`, the job that actually binds
// them, with none. Reproduced on a real monorepo.
func TestScriptPortsNeverClaimWhatAnotherJobAlreadyBinds(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{
			Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".",
			Cmd:   "docker compose -f docker-compose.yml up -d",
			Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379},
		},
		{Name: "dev:shop", Kind: domain.JobKindService, Cwd: ".", Cmd: "pnpm run dev:shop"},
	}}

	got := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{{Name: "dev:shop", Kind: domain.JobKindService}},
		ScansByDir: map[string]domain.EnvPortScan{
			".": {Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379}},
		},
	})

	for _, job := range got.Config.Jobs {
		if job.Name != "dev:shop" {
			continue
		}
		if len(job.Ports) != 0 {
			t.Errorf("dev:shop claimed %v, want none: those ports are the compose job's", job.Ports)
		}
	}
	for _, job := range got.Config.Jobs {
		if job.Name == "docker-compose" && len(job.Ports) != 2 {
			t.Errorf("docker-compose = %v, want its two ports kept", job.Ports)
		}
	}
}

// The same variable at two different bases is two ports, and each app keeps its
// own: the rule is about a port, not about a name.
func TestScriptPortsKeepTheSameNameAtADifferentBase(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "crm-api", Kind: domain.JobKindService, Cwd: "apps/crm/api", Cmd: "pnpm run dev", Ports: map[string]int{"PORT": 4002}},
		{Name: "shop-api", Kind: domain.JobKindService, Cwd: "apps/shop/api", Cmd: "pnpm run dev"},
	}}

	got := BackfillScriptPorts(BackfillScriptPortsParams{
		Config:         cfg,
		PackageManager: domain.PkgManagerPnpm,
		Scripts:        []domain.PackageScript{{Name: "dev", Kind: domain.JobKindService, Workspace: "apps/shop/api"}},
		ScansByDir:     map[string]domain.EnvPortScan{"apps/shop/api": {Ports: map[string]int{"PORT": 4001}}},
	})

	for _, job := range got.Config.Jobs {
		if job.Name == "shop-api" && job.Ports["PORT"] != 4001 {
			t.Errorf("shop-api PORT = %v, want 4001 kept alongside crm-api's 4002", job.Ports)
		}
	}
}
