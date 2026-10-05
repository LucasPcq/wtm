package flowui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func textStep(key string) flow.Step {
	return flow.Step{Kind: flow.StepText, Key: key, Label: "Text " + key, Title: "Text"}
}

func selectStep(key string, values ...string) flow.Step {
	options := make([]flow.Option, 0, len(values))
	for _, value := range values {
		options = append(options, flow.Option{Label: value, Value: value})
	}
	return flow.Step{Kind: flow.StepSelect, Key: key, Label: "Select " + key, Options: options}
}

func recapStep(key string) flow.Step {
	return flow.Step{
		Kind: flow.StepRecap, Key: key, Label: "Recap",
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: "review",
				Options:     []flow.Option{{Label: "Yes, do it", Value: "go"}},
			}, nil
		},
	}
}

func names(steps []components.Step) []string {
	out := make([]string, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.Name)
	}
	return out
}

func TestBuildSkipsPresetSteps(t *testing.T) {
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"a": "given"}),
		Steps:   []flow.Step{textStep("a"), selectStep("b", "one"), recapStep("r")},
	}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(plan.steps); strings.Join(got, ",") != "Select b,Recap" {
		t.Errorf("steps = %v, want the preset one dropped", got)
	}
	if got := plan.known().Value("a"); got != "given" {
		t.Errorf("preset value = %q, want it available to every step", got)
	}
}

func TestBuildResolvesAConditionalFirstStepUpFront(t *testing.T) {
	conditional := func(skip bool) flow.Step {
		return flow.Step{
			Kind: flow.StepSelect, Key: "c", Label: "Conditional",
			Skip: func(flow.Answers) (bool, string) { return skip, "nothing to reconcile" },
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Options: []flow.Option{{Label: "do", Value: "do"}}}, nil
			},
		}
	}

	dropped, err := build(flow.Session{Steps: []flow.Step{conditional(true), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(dropped.steps); strings.Join(got, ",") != "Recap" {
		t.Errorf("steps = %v, want the irrelevant step dropped", got)
	}
	answer, _ := dropped.known().Get("c")
	if !answer.Skipped || answer.SkipReason != "nothing to reconcile" {
		t.Errorf("answer = %+v, want it skipped with its reason", answer)
	}

	kept, err := build(flow.Session{Steps: []flow.Step{conditional(false), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(kept.steps); strings.Join(got, ",") != "Conditional,Recap" {
		t.Errorf("steps = %v, want the applicable step kept", got)
	}
	if kept.steps[0].AutoSkip != nil {
		t.Error("a step resolved up front should carry no auto-skip")
	}
}

func TestBuildKeepsALaterConditionalStepConditional(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{
		textStep("a"),
		{
			Kind: flow.StepSelect, Key: "c", Label: "Conditional",
			Skip:  func(flow.Answers) (bool, string) { return true, "later" },
			Build: func(flow.Answers) (flow.StepContent, error) { return flow.StepContent{}, nil },
		},
		recapStep("r"),
	}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := names(plan.steps); strings.Join(got, ",") != "Text a,Conditional,Recap" {
		t.Errorf("steps = %v, want the conditional step kept for entry-time decision", got)
	}
	if plan.steps[1].AutoSkip == nil {
		t.Error("a conditional step that is not first must decide on entry")
	}
}

// A picker with nothing to offer must refuse before anything is displayed.
func TestBuildSurfacesAStepThatCannotBeBuilt(t *testing.T) {
	refusal := errors.New("nothing to pick")
	_, err := build(flow.Session{Steps: []flow.Step{{
		Kind: flow.StepSelect, Key: "a", Label: "Picker",
		Build: func(flow.Answers) (flow.StepContent, error) { return flow.StepContent{}, refusal },
	}}})
	if !errors.Is(err, refusal) {
		t.Fatalf("err = %v, want the step's own refusal", err)
	}
}

func TestBuildRejectsAKindItCannotRender(t *testing.T) {
	_, err := build(flow.Session{Steps: []flow.Step{{Kind: flow.StepKind(99), Key: "a", Label: "Odd"}}})
	if err == nil {
		t.Fatal("expected an unrenderable kind to be refused rather than guessed")
	}
	if !strings.Contains(err.Error(), "a") {
		t.Errorf("error %q should name the step", err)
	}
}

func TestRecapAppendsTheCancelRow(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	view := plan.steps[0].Model.(components.SelectListModel).View()
	if !strings.Contains(view, domain.WizardCancelLabel) {
		t.Errorf("recap should offer %q:\n%s", domain.WizardCancelLabel, view)
	}
	if !plan.steps[0].Recap {
		t.Error("a recap step should read as the review point")
	}
}

func TestReadAnswersTheWholeSession(t *testing.T) {
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"preset": "kept"}),
		Steps: []flow.Step{
			textStep("name"),
			selectStep("choice", "first", "second"),
			{Kind: flow.StepText, Key: "preset", Label: "Preset"},
			recapStep("r"),
		},
	}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	for _, r := range "feat/x" {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // name → choice
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyDown})  // second
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // choice → recap
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // confirm

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := answers.Value("name"); got != "feat/x" {
		t.Errorf("name = %q, want the typed value", got)
	}
	if got := answers.Value("choice"); got != "second" {
		t.Errorf("choice = %q, want the chosen option", got)
	}
	if got := answers.Value("preset"); got != "kept" {
		t.Errorf("preset = %q, want it preserved", got)
	}
	if got := answers.Value("r"); got != "go" {
		t.Errorf("recap = %q, want the confirmation", got)
	}
	if !answers.Answered("choice") {
		t.Error("a chosen option was answered by a human")
	}
	if answers.Answered("preset") {
		t.Error("a preset is not an answer a human gave")
	}
}

func TestReadTurnsTheCancelRowIntoAnAbort(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyDown}) // past the separator
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := plan.read(wizard); !errors.Is(err, domain.ErrUserAborted) {
		t.Fatalf("err = %v, want ErrUserAborted", err)
	}
}

func TestAnswersFromFeedsLaterSteps(t *testing.T) {
	var seen string
	session := flow.Session{Steps: []flow.Step{
		textStep("name"),
		{
			Kind: flow.StepSelect, Key: "derived", Label: "Derived",
			Build: func(answers flow.Answers) (flow.StepContent, error) {
				seen = answers.Value("name")
				return flow.StepContent{Options: []flow.Option{{Label: "ok", Value: "ok"}}}, nil
			},
		},
	}}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	for _, r := range "feat/y" {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	update(wizard, tea.KeyMsg{Type: tea.KeyEnter}) // advance into the derived step

	if seen != "feat/y" {
		t.Errorf("the later step saw %q, want the earlier answer", seen)
	}
}

func TestSummarizeUsesTheFlowWording(t *testing.T) {
	step := selectStep("a", "ff")
	step.Summarize = func(answer flow.Answer) string {
		if answer.Value == "ff" {
			return "fast-forward to origin"
		}
		return "keep"
	}
	plan, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := plan.steps[0].Summary(plan.steps[0].Model); got != "fast-forward to origin" {
		t.Errorf("summary = %q, want the flow's own wording", got)
	}
}

func TestInteractivePrompterReportsItself(t *testing.T) {
	if !New(t.Context(), Params{}).Interactive() {
		t.Error("the wizard prompter must report that it can ask")
	}
}

func TestConfirmItemsLeadsWithTheHarmlessOutcome(t *testing.T) {
	items := confirmItems(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local"})

	if len(items) != 3 {
		t.Fatalf("a labelled confirm must offer both outcomes, got %d items", len(items))
	}
	if items[0].Label != "Keep local" || items[2].Label != "Push to origin" {
		t.Fatalf("the harmless outcome must lead when DefaultYes is false, got %+v", items)
	}
}

func TestConfirmItemsLeadsWithYesWhenItIsTheDefault(t *testing.T) {
	items := confirmItems(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local", DefaultYes: true})

	if items[0].Label != "Push to origin" || items[2].Label != "Keep local" {
		t.Fatalf("the default outcome must lead, got %+v", items)
	}
}

func update(m components.WizardModel, msg tea.Msg) components.WizardModel {
	model, _ := m.Update(msg)
	updated, ok := model.(components.WizardModel)
	if !ok {
		panic("wizard Update returned a foreign model")
	}
	return updated
}

func multiSelectStep(key string, values ...string) flow.Step {
	options := make([]flow.Option, 0, len(values))
	for _, value := range values {
		options = append(options, flow.Option{Label: value, Value: value})
	}
	return flow.Step{Kind: flow.StepMultiSelect, Key: key, Label: "Multi " + key, Options: options}
}

func TestBuildRendersAMultiSelectStep(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{multiSelectStep("a", "one", "two"), recapStep("r")}}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	model, ok := plan.steps[0].Model.(components.MultiSelectModel)
	if !ok {
		t.Fatalf("model = %T, want a MultiSelectModel", plan.steps[0].Model)
	}
	if got := model.Values(); len(got) != 0 {
		t.Errorf("values = %v, want nothing checked before the user answers", got)
	}
}

// A set answer travels in Answer.Values; reading it as a single value would
// silently collapse the selection to nothing.
func TestAnswerOfReadsAMultiSelectAsASet(t *testing.T) {
	model := components.NewMultiSelect(components.NewMultiSelectParams{
		Items: []components.MultiSelectItem{
			{Label: "one", Value: "one", Selected: true},
			{Label: "two", Value: "two"},
			{Label: "three", Value: "three", Selected: true},
		},
	})

	answer := answerOf(flow.StepMultiSelect, model)
	if len(answer.Values) != 2 || answer.Values[0] != "one" || answer.Values[1] != "three" {
		t.Errorf("values = %v, want the checked items", answer.Values)
	}
	if !answer.Asked {
		t.Error("an answered step must read as asked")
	}
}

// The dashboard's modal refuses an unknown kind rather than guessing; the CLI
// wizard must do the same, so a new kind is heard about immediately.
func TestBuildRefusesAnUnknownKind(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{Kind: flow.StepKind(99), Key: "a", Label: "Mystery"}}}

	if _, err := build(session); err == nil {
		t.Fatal("an unsupported step kind must refuse the run")
	}
}

// The CLI wizard must carry a step's pre-checked options too: prune's picker
// opens with the safe candidates already checked, and losing that would turn a
// confirmation into a full re-selection.
func TestMultiSelectKeepsPreCheckedOptions(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
		Options: []flow.Option{
			{Label: "merged-wt", Value: "merged-wt", Selected: true},
			{Label: "dirty-wt", Value: "dirty-wt", Tag: "dirty", Tone: domain.ToneDanger},
		},
	}

	model := multiSelect(step, flow.StepContent{Options: step.Options})

	got := model.Values()
	if len(got) != 1 || got[0] != "merged-wt" {
		t.Errorf("values = %v, want only the pre-checked option", got)
	}
}

func loadedRecap(key string) flow.Step {
	return flow.Step{
		Kind: flow.StepRecap, Key: key, Label: "Recap", Title: "Recap",
		LoadingMessage: "Computing…",
		Load: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: "loaded body",
				Options:     []flow.Option{{Label: "Yes, do it", Value: "go"}},
			}, nil
		},
	}
}

// The wizard never runs OnEnter on the step it starts on, so a session that
// reduces to a single loaded step would otherwise sit on its placeholder.
func TestALoadedStepLoadsEvenWhenItComesFirst(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{loadedRecap("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if plan.initCmd == nil {
		t.Fatal("a load landing first must be fired at init")
	}
	if plan.loadingText != "Computing…" {
		t.Errorf("loadingText = %q, want the step's message", plan.loadingText)
	}

	wizard := components.NewWizardWithParams(components.WizardParams{Steps: plan.steps})
	handle := plan.handler(t.Context())

	cmd, handled := handle(&wizard, plan.initCmd())
	if !handled {
		t.Fatal("the load request must be handled")
	}
	for _, sub := range cmd().(tea.BatchMsg) {
		if done, ok := sub().(loadDoneMsg); ok {
			handle(&wizard, done)
		}
	}

	// The placeholder carries no option at all, so the loaded row is the proof.
	view := wizard.Steps()[0].Model.(components.SelectListModel).View()
	if !strings.Contains(view, "Yes, do it") {
		t.Errorf("the first step must show its loaded content, not the placeholder:\n%s", view)
	}
}

func TestSelectOpensOnTheStepsStartingValue(t *testing.T) {
	content := flow.StepContent{
		Options: []flow.Option{
			{Label: "feature-a", Value: "/wt/a"},
			{Label: "feature-b", Value: "/wt/b"},
			{Label: "feature-c", Value: "/wt/c"},
		},
		Start: "/wt/c",
	}

	if got := selectList(content).Value(); got != "/wt/c" {
		t.Errorf("cursor = %q, want the starting value", got)
	}
}

func TestSelectWithoutAStartOpensOnTheFirstOption(t *testing.T) {
	content := flow.StepContent{Options: []flow.Option{
		{Label: "feature-a", Value: "/wt/a"},
		{Label: "feature-b", Value: "/wt/b"},
	}}

	if got := selectList(content).Value(); got != "/wt/a" {
		t.Errorf("cursor = %q, want the first option", got)
	}
}

func TestSelectRendersTheBadgesAStepDeclares(t *testing.T) {
	content := flow.StepContent{Options: []flow.Option{{
		Label:  "feature-a",
		Value:  "/wt/a",
		Badges: []flow.Badge{{Text: "3 jobs", Tone: domain.ToneSuccess}, {Text: "current"}},
	}}}

	view := selectList(content).View()
	for _, want := range []string{"3 jobs", "current"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

// A conditional step used to be replaced by a plain choice list whatever its
// kind, which left sync's `--dry-run`-gated recap with no options at all: the
// wizard showed "No matches" instead of the cascade it was about to run.
func TestAConditionalLoadedRecapKeepsItsLoad(t *testing.T) {
	step := loadedRecap("r")
	step.Skip = func(flow.Answers) (bool, string) { return false, "" }

	plan, err := build(flow.Session{Steps: []flow.Step{selectStep("a", "one"), step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	recap := plan.steps[1]
	if !recap.Recap {
		t.Error("a conditional recap must still read as a recap")
	}
	if recap.Build == nil || recap.AutoSkip == nil {
		t.Fatal("a conditional recap must still decide on entry")
	}

	wizard := components.NewWizardWithParams(components.WizardParams{Steps: plan.steps})
	prev := plan.steps[:1]
	wizard.UpdateStepModel(1, func(any) any { return recap.Build(prev) })
	if recap.AutoSkip(wizard) {
		t.Fatal("the step must be shown when its Skip says so")
	}
	if recap.OnEnter == nil {
		t.Fatal("a conditional recap must still fire its load on entry")
	}

	handle := plan.handler(t.Context())
	cmd, handled := handle(&wizard, recap.OnEnter(prev)())
	if !handled {
		t.Fatal("the load request must be handled")
	}
	for _, sub := range cmd().(tea.BatchMsg) {
		if done, ok := sub().(loadDoneMsg); ok {
			handle(&wizard, done)
		}
	}

	view := wizard.Steps()[1].Model.(components.SelectListModel).View()
	if !strings.Contains(view, "Yes, do it") {
		t.Errorf("a conditional recap must show its loaded content:\n%s", view)
	}
	if !strings.Contains(view, domain.WizardCancelLabel) {
		t.Errorf("a conditional recap must keep its cancel row:\n%s", view)
	}
}

func TestAConditionalRecapIsStillSkipped(t *testing.T) {
	step := loadedRecap("r")
	step.Skip = func(flow.Answers) (bool, string) { return true, "previewing only" }

	plan, err := build(flow.Session{Steps: []flow.Step{selectStep("a", "one"), step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	recap := plan.steps[1]
	recap.Build(plan.steps[:1])
	if !recap.AutoSkip(components.WizardModel{}) {
		t.Fatal("a recap its flow skips must not be shown")
	}
	if got := recap.SkipReason(); got != "previewing only" {
		t.Errorf("reason = %q, want the flow's own", got)
	}
}

// A conditional multi-select keeps its own model and is gated, exactly as a
// conditional recap is: only a select is folded into the list choiceStep draws,
// because only a select's whole model can be rebuilt from its answer. Drawing
// the others as a picker was the shape the recap bug had; refusing them
// outright left `run up --profile` unable to ask on a terminal.
func TestAConditionalMultiSelectIsGatedNotRedrawn(t *testing.T) {
	step := multiSelectStep("b", "one")
	step.Skip = func(flow.Answers) (bool, string) { return true, "only one profile" }

	plan, err := build(flow.Session{Steps: []flow.Step{selectStep("a", "one"), step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	gated := plan.steps[1]
	if _, ok := gated.Model.(components.MultiSelectModel); !ok {
		t.Errorf("model = %T, want a MultiSelectModel", gated.Model)
	}
	gated.Build(plan.steps[:1])
	if !gated.AutoSkip(components.WizardModel{}) {
		t.Fatal("a step its flow skips must not be shown")
	}
	if got := gated.SkipReason(); got != "only one profile" {
		t.Errorf("reason = %q, want the flow's own", got)
	}
}

// A conditional step landing first is decided up front, then rendered by its
// own kind.
func TestAConditionalFirstStepKeepsItsOwnKind(t *testing.T) {
	step := multiSelectStep("a", "one")
	step.Skip = func(flow.Answers) (bool, string) { return false, "" }

	plan, err := build(flow.Session{Steps: []flow.Step{step, recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, ok := plan.steps[0].Model.(components.MultiSelectModel); !ok {
		t.Errorf("model = %T, want a MultiSelectModel", plan.steps[0].Model)
	}
}

// Every step kind the wizard can draw must be readable back. A kind rendered
// but not read answers empty, and the flow writes the absence as if it were the
// answer — which is how a profile came out with no jobs.
func TestEveryDrawableKindIsReadBack(t *testing.T) {
	kinds := []struct {
		kind  flow.StepKind
		model any
		want  func(flow.Answer) bool
	}{
		{flow.StepText, components.NewTextInput(components.NewTextInputParams{Default: "x"}), func(a flow.Answer) bool { return a.Value == "x" }},
		{flow.StepSelect, components.NewSelectList(components.NewSelectListParams{
			Items: []components.SelectItem{{Label: "a", Value: "a"}},
		}), func(a flow.Answer) bool { return a.Value == "a" }},
		{flow.StepMultiSelect, components.NewMultiSelect(components.NewMultiSelectParams{
			Items: []components.MultiSelectItem{{Label: "a", Value: "a", Selected: true}},
		}), func(a flow.Answer) bool { return len(a.Values) == 1 }},
		{flow.StepReorder, components.NewReorderList(components.NewReorderListParams{
			Items: []components.ReorderItem{{Label: "a", Value: "a"}},
		}), func(a flow.Answer) bool { return len(a.Values) == 1 }},
	}

	for _, tc := range kinds {
		answer := answerOf(tc.kind, tc.model)
		if !answer.Asked {
			t.Errorf("kind %d is drawn but never read back: the flow would take the empty answer for a real one", tc.kind)
			continue
		}
		if !tc.want(answer) {
			t.Errorf("kind %d read back as %+v", tc.kind, answer)
		}
	}
}

// A step that builds its content — the worktree picker of every run command —
// sets its cursor there too. The merge kept the options and dropped the start,
// so the picker opened on main whatever worktree the command was typed in.
func TestABuiltStepKeepsItsStartingValue(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepSelect, Key: "worktree", Label: "Worktree",
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Options: []flow.Option{{Label: "main", Value: "/wt/main"}, {Label: "feat", Value: "/wt/feat"}},
				Start:   "/wt/feat",
			}, nil
		},
	}

	content, err := (&plan{}).content(step, flow.Answers{})
	if err != nil {
		t.Fatalf("content: %v", err)
	}
	if got := selectList(content).Value(); got != "/wt/feat" {
		t.Errorf("cursor = %q, want the start the step built", got)
	}
}

// The worktree step declares "N running" and "current" and where the cursor
// opens; the wizard drew neither.
func TestAMultiSelectCarriesItsBadgesAndStart(t *testing.T) {
	step := flow.Step{Kind: flow.StepMultiSelect, Key: "w"}
	model := multiSelect(step, flow.StepContent{
		Options: []flow.Option{
			{Label: "main", Value: "/wt/main", Badges: []flow.Badge{{Text: "2 running", Tone: domain.ToneSuccess}}},
			{Label: "feat/x", Value: "/wt/x", Badges: []flow.Badge{{Text: "current"}}},
		},
		Start: "/wt/x",
	})
	view := model.View()
	for _, want := range []string{"2 running", "current"} {
		if !strings.Contains(view, want) {
			t.Errorf("view = %q, want %q", view, want)
		}
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if got := strings.Join(model.Values(), ","); got != "/wt/x" {
		t.Errorf("space toggled %q, want the row the step starts on", got)
	}
}

// A conditional step whose Build fails must fail with its own cause: "has no
// conditional renderer" sent the reader after the wizard over a git error.
func TestAConditionalStepsBuildErrorIsItsOwn(t *testing.T) {
	step := multiSelectStep("b")
	step.Options = nil
	step.Skip = func(flow.Answers) (bool, string) { return false, "" }
	step.Build = func(flow.Answers) (flow.StepContent, error) {
		return flow.StepContent{}, errors.New("list worktrees: boom")
	}

	_, err := build(flow.Session{Steps: []flow.Step{selectStep("a", "one"), step}})
	if err == nil || !strings.Contains(err.Error(), "list worktrees: boom") {
		t.Fatalf("err = %v, want the step's own cause", err)
	}
}

// A select with a Skip is drawn through ChoiceStep; it dropped the start, so
// `run job edit` on a shared service opened its scope on "per worktree" and
// a plain enter un-shared it.
func TestAConditionalSelectKeepsItsStartingValue(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepSelect, Key: "scope", Label: "Scope",
		Options: []flow.Option{{Label: "per worktree", Value: "worktree"}, {Label: "shared", Value: "shared"}},
		Skip:    func(flow.Answers) (bool, string) { return false, "" },
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Start: "shared"}, nil
		},
	}

	built := (&plan{}).choiceStep(step)
	list, ok := built.Build(nil).(components.SelectListModel)
	if !ok {
		t.Fatalf("choice step built %T, want a select list", built.Build(nil))
	}
	if got := list.Value(); got != "shared" {
		t.Errorf("cursor = %q, want the start the step built", got)
	}
}

func TestBuildRendersATextListPrefilledFromItsContent(t *testing.T) {
	step := flow.Step{
		Kind:  flow.StepTextList,
		Key:   "branches",
		Label: "Branches",
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Entries: []string{"feat/a"}}, nil
		},
	}
	plan, err := build(flow.Session{Steps: []flow.Step{step, recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	model, ok := plan.steps[0].Model.(components.TextListModel)
	if !ok {
		t.Fatalf("model = %T, want a TextListModel", plan.steps[0].Model)
	}
	if got := strings.Join(model.Values(), ","); got != "feat/a" {
		t.Errorf("values = %q, want the pre-fill", got)
	}
}

func TestAnswerOfReadsATextListAsASet(t *testing.T) {
	model := components.NewTextList(components.NewTextListParams{Entries: []string{"feat/a", "feat/b"}})
	answer := answerOf(flow.StepTextList, model)
	if strings.Join(answer.Values, ",") != "feat/a,feat/b" || !answer.Asked {
		t.Errorf("answer = %+v, want both entries, asked", answer)
	}
}

func TestBuildRendersTheEnvResolverAndReadsItsDecisions(t *testing.T) {
	files := []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
		{Key: "OLD", Status: domain.EnvKeyOrphan, CurrentValue: "1"},
	}}}}
	step := flow.Step{Kind: flow.StepEnvResolve, Key: "resolve", Label: "Resolve", Build: func(flow.Answers) (flow.StepContent, error) {
		return flow.StepContent{EnvFiles: files}, nil
	}}

	p, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(p.steps) != 1 || !p.steps[0].Callout {
		t.Fatalf("steps = %+v, want the resolver with its glossary as a callout", p.steps)
	}
	answer := answerOf(flow.StepEnvResolve, p.steps[0].Model)
	if !answer.Asked || len(answer.EnvDecisions) != 1 || answer.EnvDecisions[0].Target != ".env" {
		t.Errorf("answer = %+v, want the file's decision", answer)
	}
}
