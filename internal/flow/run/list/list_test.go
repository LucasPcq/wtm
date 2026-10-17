package list_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/list"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

var declared = domain.RunConfig{
	Jobs: []domain.JobConfig{
		{Name: "api", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
	},
	Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"api"}, Default: true}},
}

func run(t *testing.T, cfg domain.RunConfig, prompter flow.Prompter) (list.Selection, *flowtest.Recorder, error) {
	t.Helper()
	presenter := &flowtest.Recorder{}
	selection, err := list.Run(t.Context(), list.Params{Request: list.Request{Config: cfg}, Prompter: prompter, Presenter: presenter})
	return selection, presenter, err
}

func entry(kind, name string) string { return kind + domain.RunListKindSep + name }

func values(options []flow.Option) []string {
	var out []string
	for _, option := range options {
		if !option.Separator {
			out = append(out, option.Value)
		}
	}
	return out
}

func TestListOffersTheProfilesThenTheJobs(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		list.KeyEntry:  entry(domain.RunListKindProfile, "dev"),
		list.KeyAction: domain.RunListActionUp,
	}}

	if _, _, err := run(t, declared, prompter); err != nil {
		t.Fatalf("list: %v", err)
	}
	got := values(prompter.Content[list.KeyEntry].Options)
	want := []string{entry(domain.RunListKindProfile, "dev"), entry(domain.RunListKindJob, "api"), entry(domain.RunListKindJob, "migrate")}
	if len(got) != len(want) {
		t.Fatalf("entries = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %s, want %s", i, got[i], want[i])
		}
	}
}

// What the picked entry can be told to do follows its kind: a profile comes up
// and goes down whole, a job starts, stops and shows its output.
func TestTheActionsOfferedFollowTheKindPicked(t *testing.T) {
	cases := map[string]struct {
		entry string
		want  []string
	}{
		"profile": {entry: entry(domain.RunListKindProfile, "dev"), want: []string{domain.RunListActionUp, domain.RunListActionDown}},
		"job":     {entry: entry(domain.RunListKindJob, "api"), want: []string{domain.RunListActionStart, domain.RunListActionStop, domain.RunListActionLogs}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{list.KeyEntry: c.entry, list.KeyAction: c.want[0]}}

			selection, _, err := run(t, declared, prompter)

			if err != nil {
				t.Fatalf("list: %v", err)
			}
			got := values(prompter.Content[list.KeyAction].Options)
			if len(got) != len(c.want) {
				t.Fatalf("actions = %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("action %d = %s, want %s", i, got[i], c.want[i])
				}
			}
			if selection.Kind != name || selection.Action != c.want[0] {
				t.Errorf("selection = %+v, want a %s told to %s", selection, name, c.want[0])
			}
		})
	}
}

func TestListHandsBackTheEntryAndTheAction(t *testing.T) {
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		list.KeyEntry:  entry(domain.RunListKindJob, "migrate"),
		list.KeyAction: domain.RunListActionLogs,
	}}

	selection, _, err := run(t, declared, prompter)

	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := list.Selection{Kind: domain.RunListKindJob, Name: "migrate", Action: domain.RunListActionLogs}
	if selection != want {
		t.Errorf("selection = %+v, want %+v", selection, want)
	}
}

// Picking is the whole gesture: a run nobody can answer has nothing to fall
// back to, and says so rather than choosing.
func TestAnUnattendedListIsRefused(t *testing.T) {
	_, _, err := run(t, declared, flow.Unattended{})

	if err == nil {
		t.Fatal("an unattended list picked something")
	}
}

func TestAnEmptyRunTomlHasNothingToList(t *testing.T) {
	_, _, err := run(t, domain.RunConfig{}, &flowtest.ScriptedPrompter{})

	if !errors.Is(err, domain.ErrNoJobsDeclared) {
		t.Fatalf("err = %v, want ErrNoJobsDeclared", err)
	}
}

func TestListBackedOutOfSaysAborted(t *testing.T) {
	selection, presenter, err := run(t, declared, &flowtest.ScriptedPrompter{Abort: true})

	if err != nil || !selection.Aborted {
		t.Fatalf("selection = %+v, err = %v, want an abort", selection, err)
	}
	if len(presenter.Notices) != 1 || presenter.Notices[0].Text != flow.AbortedNotice.Text {
		t.Errorf("notices = %+v, want Aborted", presenter.Notices)
	}
}
