package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func runCheckout(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), w.command, append([]string{"checkout"}, words...)...)
}

// plan is the plan of checkout's envelope, as the tests read it.
type plan struct {
	PullRequest struct {
		Number int    `json:"number"`
		Branch string `json:"branch"`
		Base   string `json:"base"`
	} `json:"pull_request"`
	Action      string   `json:"action"`
	From        string   `json:"from"`
	EnvFrom     string   `json:"env_from"`
	FastForward []string `json:"fast_forward"`
}

func decoded(t *testing.T, stdout string) formtest.Envelope[plan] {
	t.Helper()
	return formtest.Decoded[plan](t, stdout)
}

func wizard(t *testing.T, req checkout.Request) *formtest.Driver {
	t.Helper()
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), newWorld(seed{}).usecase(), req, domain.RunOptions{})
	return formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40})
}
