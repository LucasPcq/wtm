package proxycmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	// `run proxy` reads the machine, not a project: an isolated home is what
	// keeps these tests from depending on the developer's own config, daemon
	// and LaunchAgents — and on the CI having neither.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	var buf bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestStatusJSONCarriesBothPorts(t *testing.T) {
	out, err := runCmd(t, domain.CmdStatus, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatal(err)
	}

	var status domain.ProxyStatus
	if unmarshalErr := json.Unmarshal([]byte(out), &status); unmarshalErr != nil {
		t.Fatalf("sortie non-JSON: %v\n%s", unmarshalErr, out)
	}
	if status.ConfiguredPort != domain.ProxyDefaultPort {
		t.Errorf("le port configuré doit toujours être présent : got %d", status.ConfiguredPort)
	}
	if status.PublicPort == 0 {
		t.Error("le port public doit être résolu")
	}
}

func TestInstallRefusesAnUnanswerableRun(t *testing.T) {
	_, err := runCmd(t, domain.CmdInstall)

	if !errors.Is(err, domain.ErrProxyInstallNeedsYes) {
		t.Errorf("got %v, want ErrProxyInstallNeedsYes", err)
	}
}

func TestUninstallRefusesAnUnanswerableRun(t *testing.T) {
	_, err := runCmd(t, domain.CmdUninstall)

	if !errors.Is(err, domain.ErrProxyInstallNeedsYes) {
		t.Errorf("got %v, want ErrProxyInstallNeedsYes", err)
	}
}

// The proxy's port and its redirection belong to the machine, so this command
// must answer from anywhere — a repo with no wtm config included.
func TestStatusAnswersOutsideAWtmProject(t *testing.T) {
	out, err := runCmd(t, domain.CmdStatus, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("run proxy status hors projet: %v\n%s", err, out)
	}

	var status domain.ProxyStatus
	if unmarshalErr := json.Unmarshal([]byte(out), &status); unmarshalErr != nil {
		t.Fatalf("sortie non-JSON: %v\n%s", unmarshalErr, out)
	}
	if status.Installed {
		t.Error("home isolé : aucune redirection installée")
	}
}

// Removing what was never installed says so instead of announcing a removal.
func TestUninstallWithNothingInstalledSaysSo(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the redirection exists on macOS only")
	}
	out, err := runCmd(t, domain.CmdUninstall, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if strings.Contains(out, domain.ProxyUninstallDone) || !strings.Contains(out, domain.ProxyUninstallNothing) {
		t.Errorf("uninstall with nothing installed printed:\n%s", out)
	}
}
