package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
)

// namedImportPayload publishes api-dev by name and links a .env key to its port,
// which is the pair that makes an origin — and a drift — possible at all.
const namedImportPayload = `{"addressing":"names",` +
	`"job":[{"name":"api-dev","kind":"service","cmd":"pnpm dev","ports":{"PORT":4001},"url":{"port":"PORT"}}],` +
	`"profile":[],` +
	`"env_port":[{"file":".env","key":"VITE_API_URL","job":"api-dev","port":"PORT"}]}`

func withEnvFile(t *testing.T, stateDir string) {
	t.Helper()
	if err := config.WriteProject(config.WriteProjectParams{
		StateDir: stateDir,
		Answers: domain.InitProjectAnswers{
			BasePath:    "../.trees",
			BaseBranch:  "main",
			EnvStrategy: domain.EnvStrategyExample,
			EnvFiles:    []domain.EnvFile{{Target: ".env"}},
		},
	}); err != nil {
		t.Fatalf("write project config: %v", err)
	}
}

func writeMainEnv(t *testing.T, stateDir, body string) {
	t.Helper()
	// The main checkout is the repository root, two levels above .git/wtm.
	root := filepath.Dir(filepath.Dir(stateDir))
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte(body), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}

func importPayload(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "layout.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A teammate receives the addressing by import — run.toml is not committed — so
// that is where the main checkout's .env has to be named.
func TestImportWarnsThatTheMainCheckoutStillAddressesPorts(t *testing.T) {
	stateDir := setupTestProject(t)
	withEnvFile(t, stateDir)
	writeMainEnv(t, stateDir, "VITE_API_URL=http://localhost:4001\n")

	_, stderr, err := runCmd(t, domain.CmdImport, importPayload(t, namedImportPayload), "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run import: %v", err)
	}

	if !strings.Contains(stderr, "wtm env main") {
		t.Errorf("stderr = %q, want the command that aligns the main checkout", stderr)
	}
}

func TestImportSaysNothingWhenTheProjectAddressesByPort(t *testing.T) {
	stateDir := setupTestProject(t)
	withEnvFile(t, stateDir)
	writeMainEnv(t, stateDir, "VITE_API_URL=http://localhost:4001\n")

	payload := strings.Replace(namedImportPayload, `"addressing":"names"`, `"addressing":"ports"`, 1)
	_, stderr, err := runCmd(t, domain.CmdImport, importPayload(t, payload), "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run import: %v", err)
	}

	if strings.Contains(stderr, domain.AddressingDriftTitle) {
		t.Errorf("stderr = %q, want no addressing warning under port addressing", stderr)
	}
}

// The machine surface keeps its stream clean: the document is the answer.
func TestImportJSONCarriesNoCallout(t *testing.T) {
	stateDir := setupTestProject(t)
	withEnvFile(t, stateDir)
	writeMainEnv(t, stateDir, "VITE_API_URL=http://localhost:4001\n")

	stdout, stderr, err := runCmd(t, domain.CmdImport, importPayload(t, namedImportPayload),
		"--"+domain.FlagYes, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run import: %v", err)
	}

	if strings.Contains(stderr, domain.AddressingDriftTitle) {
		t.Errorf("stderr = %q, want nothing on a machine run", stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Errorf("stdout is not the JSON document: %v", err)
	}
}
