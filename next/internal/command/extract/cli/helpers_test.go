package cli

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runExtract(t *testing.T, w world, words ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open, formtest.CommandLine)})
	code := uicli.Execute(t.Context(), root, append([]string{"extract"}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

// plan is the plan of extract's envelope, as the tests read it.
type plan struct {
	Source string `json:"source"`
	Target struct {
		Branch string `json:"branch"`
		Action string `json:"action"`
	} `json:"target"`
	Keep bool `json:"keep"`
}

func decoded(t *testing.T, stdout string) formtest.Envelope[plan] {
	t.Helper()
	return formtest.Decoded[plan](t, stdout)
}

func paths(changes []worktree.Change) []string {
	found := make([]string, 0, len(changes))
	for _, change := range changes {
		found = append(found, change.Path)
	}
	return found
}

// A driver plays the wizard of a session.
type driver struct {
	*formtest.Driver
	t *testing.T
	s *session
}

func open(t *testing.T, params sessionParams) *driver {
	t.Helper()
	s := newForm(nil).Session(t.Context(), params.App, params.Request, params.Options)
	return start(t, s, form.New(t.Context(), s.Script()))
}

func start(t *testing.T, s *session, w *form.Wizard) *driver {
	t.Helper()
	return &driver{Driver: formtest.Start(w, tea.WindowSizeMsg{Width: 100, Height: 40}), t: t, s: s}
}

// looksLike compares the screen with the frame testdata/frame_<name>.golden holds.
func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "frame_"+name, d.Screen()+"\n")
}

// unfinished is the outcome of moving README.md to a new target, ended as item.
func unfinished(item extract.Item) extract.Outcome {
	plan := extract.Plan{
		Source: "feat/login",
		Files:  []worktree.Change{{Path: "README.md", Status: worktree.ChangeModified}},
		Target: extract.Target{Branch: item.Subject, Action: extract.ActionCreate},
		From:   "main",
		Recapped: domain.Recapped{Recap: []domain.RecapRow{
			recapRow("Source", "feat/login"), recapRow("Files", "mod README.md"),
			recapRow("Target", "new worktree "+item.Subject+" from main"), recapRow("Mode", "move"),
		}},
	}
	result := &extract.Result{Items: []extract.Item{item}}
	if item.Detail != nil {
		result.FollowUps = []domain.FollowUp{{Command: "go", Answers: []domain.Answered{{Path: "worktree", Values: []string{item.Subject}}}, Code: extract.CodeGo}}
	}
	return extract.Outcome{Plan: plan, Result: result}
}

func drawn(outcome extract.Outcome) string {
	var out bytes.Buffer
	drawOutcome(&out, outcome, output.View{Cwd: "/repo", CommandLine: formtest.CommandLine})
	return out.String()
}

func untouched(message string) domain.Warning {
	return domain.Warning{Code: "extract.untouched", Message: message}
}

// recapRow is a row of a plan's recap labelled label, each of values its text.
func recapRow(label string, values ...string) domain.RecapRow {
	return domain.RecapRow{Label: label, Values: domain.RecapValues(values)}
}
