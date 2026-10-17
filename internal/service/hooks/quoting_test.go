package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// A hook is a /bin/sh line, and a path is data: a quote or a dollar in a
// worktree's directory must reach the command as it is spelled on disk.
func TestRunHooksPassesPlaceholderPathsVerbatim(t *testing.T) {
	for _, name := range []string{"it's", "cost$HOME", `a"b`, "sp ace", "back`tick`"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			worktree := filepath.Join(root, name)
			if err := os.MkdirAll(worktree, 0o755); err != nil {
				t.Fatal(err)
			}

			for _, cmd := range []string{
				"printf %s {{worktree}} > {{root}}/unquoted",
				`printf %s "{{worktree}}" > "{{root}}/double"`,
				"printf %s '{{worktree}}' > '{{root}}/single'",
			} {
				var out bytes.Buffer
				if err := RunHooks(t.Context(), RunHooksParams{
					Hooks:   []domain.HookCommand{{Cmd: cmd}},
					WorkDir: worktree,
					Vars:    rules.TemplateVars{Worktree: worktree, Root: root},
					Output:  &out,
				}); err != nil {
					t.Fatalf("%s: %v\n%s", cmd, err, out.String())
				}
			}

			for _, file := range []string{"unquoted", "double", "single"} {
				got, err := os.ReadFile(filepath.Join(root, file))
				if err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				if string(got) != worktree {
					t.Errorf("%s: hook saw %q, want %q", file, got, worktree)
				}
			}
		})
	}
}
