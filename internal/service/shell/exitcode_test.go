package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

const fakeWtm = `#!/bin/sh
if [ "$1" = "resolve" ]; then
  [ -n "$FAKE_DIR" ] && echo "$FAKE_DIR"
  exit "${FAKE_CODE:-0}"
fi
if [ -n "$FAKE_DIR" ] && [ -n "$WTM_GO_FILE" ]; then
  printf %s "$FAKE_DIR" > "$WTM_GO_FILE"
fi
exit "${FAKE_CODE:-0}"
`

type wrapperCase struct {
	name     string
	args     string
	code     string
	cdTo     bool
	wantCode string
}

func TestWrapperKeepsTheCommandsExitCode(t *testing.T) {
	cases := []wrapperCase{
		{name: "failing command", args: "clean feat", code: "3", wantCode: "3"},
		{name: "failing bare call", args: "", code: "5", wantCode: "5"},
		{name: "failing resolve", args: "go nowhere", code: "1", wantCode: "1"},
		{name: "success that moves", args: "create feat", code: "0", cdTo: true, wantCode: "0"},
		{name: "failure that still moves", args: "create feat", code: "4", cdTo: true, wantCode: "4"},
		{name: "go that moves", args: "go feat", code: "0", cdTo: true, wantCode: "0"},
	}
	for _, shellType := range []domain.ShellType{domain.ShellBash, domain.ShellZsh} {
		bin, err := exec.LookPath(string(shellType))
		if err != nil {
			t.Logf("%s not installed", shellType)
			continue
		}
		for _, c := range cases {
			t.Run(string(shellType)+"/"+c.name, func(t *testing.T) {
				runWrapperCase(t, bin, shellType, c)
			})
		}
	}
}

func runWrapperCase(t *testing.T, bin string, shellType domain.ShellType, c wrapperCase) {
	t.Helper()
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "wtm"), []byte(fakeWtm), 0o755); err != nil {
		t.Fatal(err)
	}
	initFile := filepath.Join(dir, "init.sh")
	if err := os.WriteFile(initFile, []byte(rules.GenerateShellInit(shellType)), 0o644); err != nil {
		t.Fatal(err)
	}

	script := ". " + initFile + "; wtm " + c.args + "; echo \"$?\"; pwd -P"
	cmd := exec.Command(bin, "-c", script)
	cmd.Dir = dir
	fakeDir := ""
	if c.cdTo {
		fakeDir = target
	}
	cmd.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_CODE="+c.code,
		"FAKE_DIR="+fakeDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", bin, err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		t.Fatalf("unexpected output:\n%s", out)
	}
	gotCode, gotDir := lines[len(lines)-2], lines[len(lines)-1]
	if gotCode != c.wantCode {
		t.Errorf("exit code = %s, want %s", gotCode, c.wantCode)
	}
	if c.cdTo && gotDir != target {
		t.Errorf("cwd = %s, want %s", gotDir, target)
	}
}

func TestFishWrapperReturnsTheCommandsStatus(t *testing.T) {
	out := rules.GenerateShellInit(domain.ShellFish)
	if strings.Count(out, "return $wtm_status") != 2 {
		t.Errorf("fish wrapper must return the command's status on both paths:\n%s", out)
	}
}
