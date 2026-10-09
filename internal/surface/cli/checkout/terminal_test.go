package checkout

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	checkoutflow "github.com/LucasPcq/wtm/internal/flow/checkout"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

func onATerminal(t *testing.T, prompter *flowtest.ScriptedPrompter) {
	t.Helper()
	previousTTY, previousPrompter := shared.StdinIsTerminal, shared.InteractivePrompter
	shared.StdinIsTerminal = func() bool { return true }
	shared.InteractivePrompter = func(context.Context, shared.FlowPrompterParams) flow.Prompter { return prompter }
	t.Cleanup(func() { shared.StdinIsTerminal, shared.InteractivePrompter = previousTTY, previousPrompter })
}

func behindReusedBranch(t *testing.T) {
	t.Helper()
	repo := newCheckoutRepo(t)
	git(t, repo.work, "branch", "feat/reused")
	git(t, repo.work, "push", "origin", "feat/reused")
	git(t, repo.work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, repo.work, "push", "origin", "main:feat/reused")
}

func cancelledCheckout(extra map[string]string) *flowtest.ScriptedPrompter {
	answers := map[string]string{
		checkoutflow.KeyParent:    "main",
		checkoutflow.KeyEnv:       "",
		checkoutflow.KeyIsolation: string(domain.IsolationIsolated),
		checkoutflow.KeyRecap:     domain.WizardCancelValue,
	}
	for key, value := range extra {
		answers[key] = value
	}
	return &flowtest.ScriptedPrompter{Answers: answers}
}

// LUC-281: on a terminal, --ff answered nothing and the wizard still offered the
// fast-forward the flag had already accepted.
func TestCheckoutFFIsNotAskedOnATerminal(t *testing.T) {
	behindReusedBranch(t)
	prompter := cancelledCheckout(nil)
	onATerminal(t, prompter)

	_, _, _ = runCheckoutCmd(t, "51", "--"+domain.FlagFF)
	if _, asked := prompter.Content[checkoutflow.KeySourceUpdate]; asked {
		t.Fatalf("--ff answers the source update, yet asked %q", prompter.AskedKeys())
	}
	if _, reached := prompter.Content[checkoutflow.KeyRecap]; !reached {
		t.Fatalf("the wizard must reach its recap, asked %q", prompter.AskedKeys())
	}
}

func TestCheckoutAsksTheSourceUpdateWithoutFF(t *testing.T) {
	behindReusedBranch(t)
	prompter := cancelledCheckout(map[string]string{checkoutflow.KeySourceUpdate: domain.SourceUpdateKeep})
	onATerminal(t, prompter)

	_, _, _ = runCheckoutCmd(t, "51")
	if _, asked := prompter.Content[checkoutflow.KeySourceUpdate]; !asked {
		t.Fatalf("without --ff a behind branch must be offered its fast-forward, asked %q", prompter.AskedKeys())
	}
}
