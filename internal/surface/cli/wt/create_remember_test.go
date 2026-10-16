package wt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func rememberInConfig(t *testing.T, dir, section string) {
	t.Helper()
	path := filepath.Join(dir, ".git", "wtm", domain.ConfigFileName)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("\n[wizard.remembered]\n" + section); err != nil {
		t.Fatal(err)
	}
}

// A remembered answer is a local default: --yes takes it, a flag wins over it,
// and --ask ignores it.
func TestWtCreateYesTakesTheRememberedAnswer(t *testing.T) {
	dir := createRepo(t)
	rememberInConfig(t, dir, "env_strategy = \"main\"\n")

	tests := []struct {
		name string
		args []string
		want domain.EnvStrategy
	}{
		{"remembered", nil, domain.EnvStrategyMain},
		{"--env-from wins", []string{"--" + domain.FlagEnvFrom, string(domain.EnvStrategyParent)}, domain.EnvStrategyParent},
		{"--ask ignores the memory", []string{"--" + domain.FlagAsk}, domain.EnvStrategyExample},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{domain.CmdCreate, "feat/mem-" + string(rune('a'+i)), "--from", "main",
				"--output", domain.OutputJSON, "--" + domain.FlagYes}, tt.args...)
			stdout, _, err := runWtCmd(t, args...)
			if err != nil {
				t.Fatalf("wt create: %v", err)
			}
			got := decodeCreated(t, stdout)
			if got.Metadata.EnvStrategy != tt.want {
				t.Errorf("env_strategy = %q, want %q", got.Metadata.EnvStrategy, tt.want)
			}
		})
	}
}

func TestWtCreateRefusesAnUnknownRememberedAnswer(t *testing.T) {
	dir := createRepo(t)
	rememberInConfig(t, dir, "delete = \"force\"\n")

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "main", "--"+domain.FlagYes); err == nil {
		t.Fatal("a config remembering an answer to no question was accepted")
	}
}
