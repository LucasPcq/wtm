package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func newTarget(path string) domain.AgentTarget {
	return domain.AgentTarget{Kind: domain.AgentKindClaudeProject, Path: path}
}

func skillPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "skills", "using-wtm", "SKILL.md")
}

func assertInstalled(t *testing.T, dir string) {
	t.Helper()
	for name, content := range skillFiles() {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("%s not written: %v", name, err)
		}
		if string(got) != content {
			t.Fatalf("%s does not match the embedded skill", name)
		}
	}
}

func TestWriteSkill_CreatesEveryFile(t *testing.T) {
	path := skillPath(t)

	res := writeSkill(newTarget(path))

	if res.Action != agentActionCreated {
		t.Fatalf("action = %q, want %q", res.Action, agentActionCreated)
	}
	assertInstalled(t, filepath.Dir(path))
}

func TestWriteSkill_UnchangedWhenIdentical(t *testing.T) {
	path := skillPath(t)
	writeSkill(newTarget(path))
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	res := writeSkill(newTarget(path))

	if res.Action != agentActionUnchanged {
		t.Fatalf("action = %q, want %q", res.Action, agentActionUnchanged)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("SKILL.md was rewritten for identical content")
	}
}

// A skill installed by an older wtm is a single SKILL.md: updating it writes
// the references beside it.
func TestWriteSkill_UpdatesASingleFileSkill(t *testing.T) {
	path := skillPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale skill content"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := writeSkill(newTarget(path))

	if res.Action != agentActionUpdated {
		t.Fatalf("action = %q, want %q", res.Action, agentActionUpdated)
	}
	assertInstalled(t, filepath.Dir(path))
}

// A reference file this wtm no longer ships is removed, but nothing the user
// put beside the skill is touched.
func TestWriteSkill_RemovesOnlyItsOwnStaleReferences(t *testing.T) {
	path := skillPath(t)
	writeSkill(newTarget(path))
	dir := filepath.Dir(path)
	stale := filepath.Join(dir, skillReferencesDir, "retired.md")
	mine := filepath.Join(dir, "notes.md")
	for _, file := range []string{stale, mine} {
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	res := writeSkill(newTarget(path))

	if res.Action != agentActionUpdated {
		t.Fatalf("action = %q, want %q", res.Action, agentActionUpdated)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("a retired reference file was left behind")
	}
	if _, err := os.Stat(mine); err != nil {
		t.Errorf("a file of the user's was removed: %v", err)
	}
}

func TestWriteSkill_SkipsOnWriteError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	res := writeSkill(newTarget(filepath.Join(dir, "nested", "SKILL.md")))

	if res.Action != agentActionSkipped || res.Reason == "" {
		t.Fatalf("result = %+v, want skipped with a reason", res)
	}
}
