package addressing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

const namedConfig = `
addressing = "names"

[[job]]
name = "api-dev"
kind = "service"
cmd = "pnpm dev"
ports = { PORT = 4001 }
url = { port = "PORT" }

[[env_port]]
file = ".env"
key = "VITE_API_URL"
job = "api-dev"
port = "PORT"
`

type repo struct {
	dir      string
	stateDir string
}

func newRepo(t *testing.T, runConfig, env string) repo {
	t.Helper()
	globaldir.Isolate(t)
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, domain.RunFileName), []byte(runConfig), 0o644); err != nil {
		t.Fatalf("write run.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return repo{dir: dir, stateDir: stateDir}
}

func (r repo) params() Params {
	cfg := domain.Config{}
	cfg.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	return Params{
		Context:  flow.Context{ProjectDir: r.dir, StateDir: r.stateDir, Config: cfg},
		WorkDirs: []string{r.dir},
	}
}

func TestNoticeNamesTheWorktreeStillOnPorts(t *testing.T) {
	r := newRepo(t, namedConfig, "VITE_API_URL=http://localhost:4001\n")

	notice, ok := Notice(r.params())

	if !ok {
		t.Fatal("no notice for a .env the published names do not answer on")
	}
	if notice.Kind != flow.NoticeWarning || notice.Text != domain.AddressingDriftTitle {
		t.Errorf("notice = %+v, want the addressing warning", notice)
	}
	if len(notice.Lines) == 0 || !strings.Contains(notice.Lines[0], "wtm env main") {
		t.Errorf("lines = %+v, want the command that aligns the worktree", notice.Lines)
	}
}

func TestNoticeStaysSilentOnPortAddressing(t *testing.T) {
	r := newRepo(t, strings.Replace(namedConfig, `addressing = "names"`, `addressing = "ports"`, 1),
		"VITE_API_URL=http://localhost:4001\n")

	if _, ok := Notice(r.params()); ok {
		t.Error("a project addressing by port publishes no name to be wrong about")
	}
}

// A directory git cannot name is not a reason to end a run on an error: the
// warning is about an address, the run is about processes.
func TestNoticeStaysSilentWithoutAWorktree(t *testing.T) {
	r := newRepo(t, namedConfig, "VITE_API_URL=http://localhost:4001\n")
	params := r.params()
	params.WorkDirs = []string{filepath.Join(t.TempDir(), "gone")}

	if _, ok := Notice(params); ok {
		t.Error("a directory git cannot name must yield no warning")
	}
}
