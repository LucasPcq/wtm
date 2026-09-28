package run

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

func TestRunAddressingWritesTheModeAndReportsJSON(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdAddressing, string(domain.AddressingPorts), "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run addressing: %v", err)
	}

	var result output.AddressingResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	if !result.Changed || result.Previous != domain.AddressingNames || result.Addressing != domain.AddressingPorts {
		t.Errorf("result = %+v, want names → ports", result)
	}
	cfg, err := runconfig.Load(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addressing != domain.AddressingPorts {
		t.Errorf("run.toml addressing = %q, want ports", cfg.Addressing)
	}
}

func TestRunAddressingNeedsTheModeWithoutATerminal(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{published("web", 3000, "")}})
	fakeTTY(t, false)

	_, _, err := runCmd(t, domain.CmdAddressing)
	if err == nil || !strings.Contains(err.Error(), "argument") {
		t.Fatalf("err = %v, want the refusal naming the argument", err)
	}
}
