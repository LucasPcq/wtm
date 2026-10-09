package wt

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	extractflow "github.com/LucasPcq/wtm/internal/flow/extract"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func behindRepo(t *testing.T) string {
	t.Helper()
	work, err := filepath.EvalSymlinks(repoWithRemote(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(work, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, work)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	behindBranch(t, work, "parent")
	return work
}

// cancelledCreate answers every create question but the source update, and backs
// out at the recap so nothing is created.
func cancelledCreate(extra map[string]string) *flowtest.ScriptedPrompter {
	answers := map[string]string{
		createflow.KeyEnv:       "",
		createflow.KeyIsolation: string(domain.IsolationIsolated),
		createflow.KeyRecap:     domain.WizardCancelValue,
	}
	for key, value := range extra {
		answers[key] = value
	}
	return &flowtest.ScriptedPrompter{Answers: answers}
}

// LUC-281: on a terminal, --ff answered nothing and the wizard still offered the
// fast-forward the flag had already accepted.
func TestCreateFFIsNotAskedOnATerminal(t *testing.T) {
	behindRepo(t)
	prompter := cancelledCreate(nil)
	onATerminal(t, prompter)

	_, _, _ = runWtCmd(t, domain.CmdCreate, "feat/new", "--from", "parent", "--"+domain.FlagFF)
	if _, asked := prompter.Content[createflow.KeySourceUpdate]; asked {
		t.Fatalf("--ff answers the source update, yet asked %q", prompter.AskedKeys())
	}
	if _, reached := prompter.Content[createflow.KeyRecap]; !reached {
		t.Fatalf("the wizard must reach its recap, asked %q", prompter.AskedKeys())
	}
}

func TestCreateAsksTheSourceUpdateWithoutFF(t *testing.T) {
	behindRepo(t)
	prompter := cancelledCreate(map[string]string{createflow.KeySourceUpdate: domain.SourceUpdateKeep})
	onATerminal(t, prompter)

	_, _, _ = runWtCmd(t, domain.CmdCreate, "feat/new", "--from", "parent")
	if _, asked := prompter.Content[createflow.KeySourceUpdate]; !asked {
		t.Fatalf("without --ff a behind source must be offered its fast-forward, asked %q", prompter.AskedKeys())
	}
}

func TestExtractFFIsNotAskedOnATerminal(t *testing.T) {
	behindRepo(t)
	src := createWorktree(t, "src")
	writeWorktreeFile(t, src.Path, "x.txt", "x\n")
	prompter := cancelledCreate(map[string]string{
		extractflow.KeyMode:  "move",
		extractflow.KeyRecap: domain.WizardCancelValue,
	})
	onATerminal(t, prompter)

	_, _, _ = runWtCmd(t, domain.CmdExtract, "src", "--files", "x.txt", "--to", "split", "--from", "parent", "--"+domain.FlagFF)
	if _, asked := prompter.Content[createflow.KeySourceUpdate]; asked {
		t.Fatalf("--ff answers the source update, yet asked %q", prompter.AskedKeys())
	}
	if _, reached := prompter.Content[extractflow.KeyRecap]; !reached {
		t.Fatalf("the wizard must reach its recap, asked %q", prompter.AskedKeys())
	}
}
