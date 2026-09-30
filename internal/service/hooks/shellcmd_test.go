package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A hook's cmd is a shell line, the same rule a job's cmd follows: an install
// chained to a build is one hook, not two.
func TestRunHooksRunsCmdThroughTheShell(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "chained")

	var out bytes.Buffer
	if err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "echo first > " + marker + " && echo second >> " + marker}},
		WorkDir: dir,
		Output:  &out,
	}); err != nil {
		t.Fatalf("RunHooks: %v", err)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("hook left no marker: %v", err)
	}
	for _, want := range []string{"first", "second"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("hook wrote %q, missing %q", got, want)
		}
	}
}

func TestRunHooksSkipsBlankCmd(t *testing.T) {
	var out bytes.Buffer
	err := RunHooks(RunHooksParams{
		Hooks:   []domain.HookCommand{{Cmd: "   "}},
		WorkDir: t.TempDir(),
		Output:  &out,
	})
	if err != nil {
		t.Fatalf("a blank hook should be a no-op, got %v", err)
	}
}
