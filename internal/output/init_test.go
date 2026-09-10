package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestInitGlobalRecap(t *testing.T) {
	var buf bytes.Buffer
	InitGlobalRecap(&buf, InitGlobalRecapParams{
		Fields:    []domain.RecapField{{Label: domain.InitRecapLabelShell, Value: "zsh"}},
		NextSteps: []NextStepParams{{Command: domain.InitNextStepShell, Note: domain.InitNextStepShellNote}},
	})
	out := buf.String()

	for _, want := range []string{
		domain.InitRecapTitleGlobal,
		domain.InitRecapLabelShell,
		"zsh",
		domain.InitRecapNextSteps,
		"wtm shell-init",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("global recap missing %q\n%s", want, out)
		}
	}
}

func TestInitProjectRecap(t *testing.T) {
	var buf bytes.Buffer
	InitProjectRecap(&buf, InitProjectRecapParams{
		ConfigPath: ".git/wtm/config.toml",
		Fields: []domain.RecapField{
			{Label: domain.InitRecapLabelBasePath, Value: "../.trees"},
			{Label: domain.InitRecapLabelBaseBranch, Value: "main"},
			{Label: domain.InitRecapLabelEnvStrategy, Value: "example"},
		},
		NextSteps: []NextStepParams{
			{Command: domain.InitNextStepCreate, Note: domain.InitNextStepCreateNote},
			{Command: domain.InitNextStepRelocate, Note: domain.InitNextStepRelocateNote},
			{Command: domain.InitNextStepRunInit, Note: domain.InitNextStepRunInitNote},
		},
	})
	out := buf.String()

	for _, want := range []string{
		domain.InitRecapTitleProject,
		"Created .git/wtm/config.toml",
		domain.InitRecapLabelBasePath,
		"../.trees",
		domain.InitRecapNextSteps,
		domain.InitNextStepCreate,
		domain.InitNextStepRelocate,
		domain.InitNextStepRunInit,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("project recap missing %q\n%s", want, out)
		}
	}
}
