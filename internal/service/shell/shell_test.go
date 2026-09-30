package shell

import (
	"os"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestGenerateShellInitZsh(t *testing.T) {
	out := rules.GenerateShellInit(domain.ShellZsh)

	if !strings.Contains(out, "wtm()") {
		t.Error("expected bash/zsh function definition")
	}
	if !strings.Contains(out, `command wtm resolve`) {
		t.Error("expected resolve call")
	}
	if !strings.Contains(out, `cd "$dir"`) {
		t.Error("expected cd command")
	}
	if !strings.Contains(out, `"$1" = "go"`) {
		t.Error("expected go subcommand interception")
	}
	if strings.Contains(out, `"$1" = "switch"`) {
		t.Error("switch is gone: the wrapper must not intercept it any more")
	}
	if !strings.Contains(out, domain.EnvGoFile) {
		t.Error("expected WTM_GO_FILE bridge for cd-after-command (clean redirect)")
	}
}

func TestGenerateShellInitBash(t *testing.T) {
	out := rules.GenerateShellInit(domain.ShellBash)

	if !strings.Contains(out, "wtm()") {
		t.Error("expected bash function definition")
	}
}

func TestGenerateShellInitFish(t *testing.T) {
	out := rules.GenerateShellInit(domain.ShellFish)

	if !strings.Contains(out, "function wtm") {
		t.Error("expected fish function definition")
	}
	if !strings.Contains(out, "command wtm resolve") {
		t.Error("expected resolve call")
	}
	if !strings.Contains(out, `"$argv[1]" = "go"`) {
		t.Error("expected go subcommand interception")
	}
}

func TestDetectShellZsh(t *testing.T) {
	os.Setenv("SHELL", "/bin/zsh")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellZsh {
		t.Errorf("expected zsh, got %s", got)
	}
}

func TestDetectShellBash(t *testing.T) {
	os.Setenv("SHELL", "/usr/bin/bash")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellBash {
		t.Errorf("expected bash, got %s", got)
	}
}

func TestDetectShellFish(t *testing.T) {
	os.Setenv("SHELL", "/opt/homebrew/bin/fish")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.ShellFish {
		t.Errorf("expected fish, got %s", got)
	}
}

func TestDetectShellDefault(t *testing.T) {
	os.Setenv("SHELL", "")
	defer os.Unsetenv("SHELL")

	if got := DetectShell(); got != domain.DefaultShell {
		t.Errorf("expected default %s, got %s", domain.DefaultShell, got)
	}
}
