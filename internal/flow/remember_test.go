package flow

import (
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func envStep() Step {
	return Step{
		Kind:    StepSelect,
		Key:     "env",
		Options: []Option{{Value: "example"}, {Value: "main"}, {Value: "parent"}},
		Resolve: func(Answers) (Answer, error) { return Answer{Value: ""}, nil },
		Memory:  Memory{ID: domain.RememberEnvStrategy},
	}
}

func recalled(step Step, remembered string, ask bool) Step {
	session := Recall(RecallParams{
		Session:    Session{Steps: []Step{step}},
		Remembered: map[string]string{step.Memory.ID: remembered},
		Ask:        ask,
	})
	return session.Steps[0]
}

func TestUnattendedTakesTheRememberedAnswerOverTheSafeDefault(t *testing.T) {
	session := Session{Steps: []Step{recalled(envStep(), "parent", false)}}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	answer, _ := answers.Get("env")
	if answer.Value != "parent" || !answer.Recalled {
		t.Errorf("answer = %+v, want the remembered parent", answer)
	}
}

func TestAFlagWinsOverTheRememberedAnswer(t *testing.T) {
	session := Session{
		Presets: NewAnswers(map[string]string{"env": "main"}),
		Steps:   []Step{recalled(envStep(), "parent", false)},
	}

	answers, err := (Unattended{}).Ask(session)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer, _ := answers.Get("env"); answer.Value != "main" || answer.Recalled {
		t.Errorf("answer = %+v, want the flag's main", answer)
	}
}

// A remembered fast-forward must not fire for a source that is not behind: the
// step's own condition is read before the memory.
func TestASkipWinsOverTheRememberedAnswer(t *testing.T) {
	step := envStep()
	step.Skip = func(Answers) (bool, string) { return true, "nothing behind" }

	answer, settled := Settle(recalled(step, "parent", false), Answers{})
	if !settled || !answer.Skipped || answer.Value != "" {
		t.Errorf("answer = %+v, want the step skipped with no value", answer)
	}
}

func TestAskSettlesNothingFromMemory(t *testing.T) {
	if _, settled := Settle(recalled(envStep(), "parent", true), Answers{}); settled {
		t.Error("--ask settled the step from memory, want it asked")
	}
}

func TestARememberedValueTheStepDoesNotOfferIsAsked(t *testing.T) {
	step := envStep()
	step.Options = step.Options[:2]

	if _, settled := Settle(recalled(step, "parent", false), Answers{}); settled {
		t.Error("settled on a value the step no longer offers, want the question asked")
	}
}

func TestADangerousOptionIsNeverRemembered(t *testing.T) {
	step := envStep()
	step.Options[2].Danger = true

	if Rememberable(step, "parent") {
		t.Error("a dangerous option was rememberable")
	}
	if _, settled := Settle(recalled(step, "parent", false), Answers{}); settled {
		t.Error("a remembered dangerous option was settled without asking")
	}
}

func TestOnlyASelectStepRemembers(t *testing.T) {
	for _, kind := range []StepKind{StepRecap, StepText, StepMultiSelect, StepBranchSelect} {
		step := envStep()
		step.Kind = kind
		if Rememberable(step, "parent") {
			t.Errorf("kind %d remembered an answer", kind)
		}
	}
}

func TestRememberingKeepsWhatWasTickedAndForgetsWhatWasUnticked(t *testing.T) {
	isolation := Step{Kind: StepSelect, Key: "iso", Memory: Memory{ID: domain.RememberIsolation, Value: "verbatim", Reask: true}}
	update := Step{Kind: StepSelect, Key: "update", Memory: Memory{ID: domain.RememberSourceUpdate}}
	session := Session{Steps: []Step{recalled(envStep(), "", false), isolation, update}}
	answers := NewAnswers(nil).
		With("env", Answer{Value: "main", Asked: true, Remember: true}).
		With("iso", Answer{Value: "isolated", Asked: true}).
		With("update", Answer{Value: "ff", Asked: true})

	change := Remembering(session, answers)

	if change.Remember[domain.RememberEnvStrategy] != "main" || len(change.Remember) != 1 {
		t.Errorf("remember = %v, want env_strategy=main alone", change.Remember)
	}
	if !slices.Equal(change.Forget, []string{domain.RememberIsolation}) {
		t.Errorf("forget = %v, want the isolation asked again and left unticked", change.Forget)
	}
}

func TestRememberingIgnoresWhatNobodyWasAsked(t *testing.T) {
	step := recalled(envStep(), "parent", false)
	answers := NewAnswers(nil).With("env", Answer{Value: "parent", Recalled: true})

	change := Remembering(Session{Steps: []Step{step}}, answers)
	if len(change.Remember) != 0 || len(change.Forget) != 0 {
		t.Errorf("change = %+v, want nothing for a recalled answer", change)
	}
}

func TestOriginNamesWhatSettledTheAnswer(t *testing.T) {
	answers := NewAnswers(nil).
		With("flag", Answer{Value: "main"}).
		With("memory", Answer{Value: "parent", Recalled: true}).
		With("asked", Answer{Value: "main", Asked: true}).
		With("fallback", Answer{Value: ""}).
		With("skipped", Answer{Skipped: true})
	cases := []struct {
		key  string
		flag bool
		want domain.AnswerOrigin
	}{
		{key: "flag", flag: true, want: domain.AnswerOriginFlag},
		{key: "memory", want: domain.AnswerOriginRemembered},
		{key: "asked", want: domain.AnswerOriginPrompt},
		{key: "fallback", want: domain.AnswerOriginConfig},
	}
	for _, tc := range cases {
		got, settled := OriginOf(OriginParams{Answers: answers, Key: tc.key, Flag: tc.flag, Fallback: domain.AnswerOriginConfig})
		if !settled || got != tc.want {
			t.Errorf("%s: origin = %q (%v), want %q", tc.key, got, settled, tc.want)
		}
	}
	if _, settled := OriginOf(OriginParams{Answers: answers, Key: "skipped"}); settled {
		t.Error("a skipped step reported an origin")
	}
}
