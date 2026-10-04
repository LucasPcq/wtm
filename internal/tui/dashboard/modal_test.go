package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// The sessions below are synthetic on purpose: the modal is a renderer of
// flow.Step values, and what create and clean declare is their own business.
func stepperSession() flow.Session {
	return flow.Session{Steps: []flow.Step{
		{Kind: flow.StepText, Key: "branch", Title: "Branch name"},
		{
			Kind: flow.StepSelect, Key: "env", Title: "Env",
			Options: []flow.Option{{Label: "config default", Value: ""}, {Label: "main", Value: "main"}},
		},
		{
			Kind: flow.StepRecap, Key: "recap", Title: "Confirm",
			Options: []flow.Option{{Label: "Yes, create", Value: "create"}},
			Build: func(answers flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Description: "branch " + answers.Value("branch")}, nil
			},
		},
	}}
}

// prompt hands the model a session the way a running flow does. It goes through
// applyFlow rather than Update so the test never re-arms the channel listener,
// which would block on a channel nothing is writing to.
func prompt(t *testing.T, model Model, msg promptMsg) Model {
	t.Helper()
	next, cmd := model.applyFlow(msg)
	return pump(t, next, cmd)
}

func openStepper(t *testing.T, session flow.Session) (Model, chan promptReply) {
	t.Helper()
	model := newTestModel(t, testWidth, testHeight, "a")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{
		title:   domain.DashboardCreateTitle,
		shape:   modalStepper,
		session: session,
		reply:   reply,
	})
	if !model.modal.open {
		t.Fatal("a prompt from a running flow must open its modal")
	}
	return model, reply
}

// press runs one message through the model and then everything its command
// produces, the way the bubbletea runtime would: a modal answers over a command,
// not by mutating the model.
func press(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := updateCmd(model, msg)
	return pump(t, next, cmd)
}

func pump(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return model
	}
	switch msg := cmd().(type) {
	case nil:
		return model
	case tea.BatchMsg:
		for _, inner := range msg {
			model = pump(t, model, inner)
		}
		return model
	default:
		next, nextCmd := model.Update(msg)
		return pump(t, next.(Model), nextCmd)
	}
}

func typeText(t *testing.T, model Model, text string) Model {
	t.Helper()
	for _, char := range text {
		model = press(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
	}
	return model
}

func waitReply(t *testing.T, reply chan promptReply) promptReply {
	t.Helper()
	select {
	case answered := <-reply:
		return answered
	default:
		t.Fatal("the flow is still waiting on an answer")
	}
	return promptReply{}
}

func TestStepperWalksTheSessionAndAnswersEveryStep(t *testing.T) {
	model, reply := openStepper(t, stepperSession())

	model = typeText(t, model, "feat")
	model = press(t, model, namedKey(tea.KeyEnter))
	model = press(t, model, namedKey(tea.KeyDown))
	model = press(t, model, namedKey(tea.KeyEnter))

	if !model.modal.open {
		t.Fatal("the recap is a step of its own; the run must not fire before it")
	}
	if body := strings.Join(model.modal.body(m0zones{}), "\n"); !strings.Contains(body, "branch feat") {
		t.Errorf("recap = %q, want it rebuilt from the answers so far", body)
	}

	model = press(t, model, namedKey(tea.KeyEnter))

	answered := waitReply(t, reply)
	if answered.err != nil {
		t.Fatalf("session answered with %v", answered.err)
	}
	if got := answered.answers.Value("branch"); got != "feat" {
		t.Errorf("branch = %q, want what was typed", got)
	}
	if got := answered.answers.Value("env"); got != "main" {
		t.Errorf("env = %q, want the row the cursor was on", got)
	}
	if model.modal.open {
		t.Error("the modal must close once the session is answered")
	}
}

func TestStepperSkipsWhatTheFlowDeclaresIrrelevant(t *testing.T) {
	session := stepperSession()
	session.Steps[1].Skip = func(answers flow.Answers) (bool, string) {
		return answers.Value("branch") == "feat", "nothing to choose"
	}
	model, reply := openStepper(t, session)

	model = typeText(t, model, "feat")
	model = press(t, model, namedKey(tea.KeyEnter))
	press(t, model, namedKey(tea.KeyEnter))

	answered := waitReply(t, reply)
	skipped, known := answered.answers.Get("env")
	if !known || !skipped.Skipped {
		t.Fatalf("env = %+v, want it recorded as skipped", skipped)
	}
	if skipped.SkipReason != "nothing to choose" {
		t.Errorf("skip reason = %q, want the one the step gave", skipped.SkipReason)
	}
}

func TestStepperPresetsAreNotAskedAgain(t *testing.T) {
	session := stepperSession()
	session.Presets = flow.NewAnswers(map[string]string{"branch": "given"})
	model, reply := openStepper(t, session)

	// The first question on screen is the env one: the branch came in preset.
	model = press(t, model, namedKey(tea.KeyEnter))
	press(t, model, namedKey(tea.KeyEnter))

	answered := waitReply(t, reply)
	if got := answered.answers.Value("branch"); got != "given" {
		t.Errorf("branch = %q, want the preset read back", got)
	}
}

func TestStepperEscGoesBackThenCancels(t *testing.T) {
	model, reply := openStepper(t, stepperSession())

	model = typeText(t, model, "feat")
	model = press(t, model, namedKey(tea.KeyEnter))
	model = press(t, model, namedKey(tea.KeyEsc))

	if !model.modal.open {
		t.Fatal("esc on the second step goes back, it does not cancel the session")
	}

	model = press(t, model, namedKey(tea.KeyEsc))

	answered := waitReply(t, reply)
	if !errors.Is(answered.err, domain.ErrUserAborted) {
		t.Errorf("err = %v, want the run aborted", answered.err)
	}
	if model.modal.open {
		t.Error("a cancelled session closes its modal")
	}
}

func TestStepperCancelRowAbortsTheRun(t *testing.T) {
	model, reply := openStepper(t, stepperSession())

	model = typeText(t, model, "feat")
	model = press(t, model, namedKey(tea.KeyEnter))
	model = press(t, model, namedKey(tea.KeyEnter))
	model = press(t, model, namedKey(tea.KeyDown))
	press(t, model, namedKey(tea.KeyEnter))

	answered := waitReply(t, reply)
	if !errors.Is(answered.err, domain.ErrUserAborted) {
		t.Errorf("err = %v, want the recap's cancel row to abort", answered.err)
	}
}

func TestModalRefusesAStepKindItCannotDraw(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{Kind: flow.StepKind(42), Key: "odd"}}}

	model := newTestModel(t, testWidth, testHeight, "a")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{shape: modalStepper, session: session, reply: reply})

	answered := waitReply(t, reply)
	if answered.err == nil || !strings.Contains(answered.err.Error(), "odd") {
		t.Fatalf("err = %v, want a refusal naming the step rather than a guessed widget", answered.err)
	}
	if model.modal.open {
		t.Error("a session the modal cannot draw must not stay on screen")
	}
}

func TestModalKeepsTheDashboardKeysToItself(t *testing.T) {
	model, _ := openStepper(t, stepperSession())

	model, cmd := updateCmd(model, key(domain.KeyQuit))

	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q typed into a branch name must not quit the dashboard")
		}
	}
	if !model.modal.open {
		t.Fatal("the modal stays up")
	}
	if got := model.modal.text.Value(); got != "q" {
		t.Errorf("input = %q, want the keystroke to have reached the text field", got)
	}
}

func TestModalLoadsSlowContentInACommandNotOnTheUpdatePath(t *testing.T) {
	ran := make(chan struct{}, 1)
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepRecap, Key: "recap", Title: "Confirm",
		LoadingMessage: domain.CleanCheckLoading,
		Options:        []flow.Option{{Label: "Go", Value: "go"}},
		Load: func(flow.Answers) (flow.StepContent, error) {
			ran <- struct{}{}
			return flow.StepContent{Description: "checked"}, nil
		},
	}}}

	model := newTestModel(t, testWidth, testHeight, "a")
	reply := make(chan promptReply, 1)
	model, cmd := model.applyFlow(promptMsg{shape: modalStepper, session: session, reply: reply})

	if model.modal.loading != domain.CleanCheckLoading {
		t.Fatalf("loading = %q, want the step's loading message while the I/O runs", model.modal.loading)
	}
	select {
	case <-ran:
		t.Fatal("Load ran on the update path — a slow step would freeze the dashboard")
	default:
	}

	model = pump(t, model, cmd)

	if model.modal.loading != "" {
		t.Error("the loaded content must replace the loading state")
	}
	if !strings.Contains(strings.Join(model.modal.body(m0zones{}), "\n"), "checked") {
		t.Error("the loaded description must be what the modal shows")
	}
}

// m0zones is a marker that records nothing: the body of a stepper has no zones,
// and a test that only reads text should not need a manager.
type m0zones struct{}

func (m0zones) Mark(_, content string) string { return content }

// The dashboard refused StepMultiSelect until the batch reparent needed it. What
// matters is that the answer travels as a set: read as a single value it would
// collapse the whole selection to nothing.
func TestTheModalAnswersAMultiSelectWithASet(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
		Title: "Select worktrees",
		Options: []flow.Option{
			{Label: "feat", Value: "feat"},
			{Label: "dev-a", Value: "dev-a"},
			{Label: "dev-b", Value: "dev-b"},
		},
	}}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	// Check the first and the third, leaving the second out.
	mo, _ = mo.update(key(" "))
	mo, _ = mo.update(namedKey(tea.KeyDown))
	mo, _ = mo.update(namedKey(tea.KeyDown))
	mo, _ = mo.update(key(" "))
	_, cmd := mo.update(namedKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("confirming the last step must answer the session")
	}
	cmd()
	answered := <-reply
	if answered.err != nil {
		t.Fatalf("reply: %v", answered.err)
	}
	got := answered.answers.Values("branches")
	if len(got) != 2 || got[0] != "feat" || got[1] != "dev-b" {
		t.Errorf("values = %v, want the two checked worktrees", got)
	}
}

func TestTheModalRendersAMultiSelectBody(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
		Title:   "Select worktrees",
		Options: []flow.Option{{Label: "feat", Value: "feat"}},
	}}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	body := strings.Join(mo.body(noMarks{}), "\n")
	if !strings.Contains(body, "feat") {
		t.Errorf("the modal must draw the choices:\n%s", body)
	}
	if !strings.Contains(body, domain.DashboardStepperMultiHint) {
		t.Errorf("the footer must name the multi-select controls:\n%s", body)
	}
}

// A step that pre-checks part of its set — prune offers the safe candidates
// checked and the unsafe ones tagged and left out — must reach the component
// with both, or the surface silently changes what the flow proposed.
func TestTheModalCarriesPreCheckedAndTaggedOptions(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
		Title: "Select worktrees",
		Options: []flow.Option{
			{Label: "merged-wt", Value: "merged-wt", Selected: true, Tag: "merged"},
			{Label: "dirty-wt", Value: "dirty-wt", Tag: "dirty", Tone: domain.ToneDanger},
		},
	}}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	body := strings.Join(mo.body(noMarks{}), "\n")
	for _, tag := range []string{"merged", "dirty"} {
		if !strings.Contains(body, tag) {
			t.Errorf("the modal must draw the %q tag:\n%s", tag, body)
		}
	}

	_, cmd := mo.update(namedKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("confirming the only step must answer the session")
	}
	cmd()
	answered := <-reply
	got := answered.answers.Values("branches")
	if len(got) != 1 || got[0] != "merged-wt" {
		t.Errorf("values = %v, want only the pre-checked option", got)
	}
}

// Going back and forward must re-ask, not skip. advance() has to tell a preset —
// answered before the session started, so never a question — from an answer the
// user gave in this modal, which they are entitled to see again.
func TestGoingBackThenForwardAsksTheStepAgain(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{
		{Kind: flow.StepSelect, Key: "one", Label: "One", Options: []flow.Option{{Label: "a", Value: "a"}}},
		{Kind: flow.StepSelect, Key: "two", Label: "Two", Options: []flow.Option{{Label: "b", Value: "b"}}},
		{Kind: flow.StepRecap, Key: "three", Label: "Three", Options: []flow.Option{{Label: "go", Value: "go"}}},
	}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	mo, _ = mo.update(namedKey(tea.KeyEnter)) // answer step one
	mo, _ = mo.update(namedKey(tea.KeyEnter)) // answer step two
	if mo.index != 2 {
		t.Fatalf("index = %d, want to have reached the recap", mo.index)
	}
	mo, _ = mo.update(namedKey(tea.KeyEsc)) // back to step two
	mo, _ = mo.update(namedKey(tea.KeyEsc)) // back to step one
	if mo.index != 0 {
		t.Fatalf("index = %d, want to be back on the first step", mo.index)
	}

	mo, _ = mo.update(namedKey(tea.KeyEnter)) // forward again

	if mo.index != 1 {
		t.Errorf("index = %d, want the second step asked again rather than skipped", mo.index)
	}
}

// A preset is not a question, so it stays skipped however the user navigates.
func TestAPresetIsNeverAskedOnTheWayForward(t *testing.T) {
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"two": "b"}),
		Steps: []flow.Step{
			{Kind: flow.StepSelect, Key: "one", Label: "One", Options: []flow.Option{{Label: "a", Value: "a"}}},
			{Kind: flow.StepSelect, Key: "two", Label: "Two", Options: []flow.Option{{Label: "b", Value: "b"}}},
			{Kind: flow.StepRecap, Key: "three", Label: "Three", Options: []flow.Option{{Label: "go", Value: "go"}}},
		},
	}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	mo, _ = mo.update(namedKey(tea.KeyEnter))

	if mo.index != 2 {
		t.Errorf("index = %d, want the preset step passed over", mo.index)
	}
}

// Returning to a multi-select must not throw away what the user checked: the
// step is rebuilt from Build, whose Selected flags are the flow's opening
// proposal, not the user's edits.
func TestGoingBackToAMultiSelectKeepsTheEdits(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{
		{
			Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Options: []flow.Option{
					{Label: "a", Value: "a", Selected: true},
					{Label: "b", Value: "b", Selected: true},
				}}, nil
			},
		},
		{Kind: flow.StepRecap, Key: "recap", Label: "Recap", Options: []flow.Option{{Label: "go", Value: "go"}}},
	}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	mo, _ = mo.update(key(" "))               // uncheck "a"
	mo, _ = mo.update(namedKey(tea.KeyEnter)) // on to the recap
	mo, _ = mo.update(namedKey(tea.KeyEsc))   // back to the selection

	if got := mo.multi.Values(); len(got) != 1 || got[0] != "b" {
		t.Errorf("values = %v, want the edit kept (only b checked)", got)
	}
}

// A skipped step is not an answer either — it is the absence of a question,
// decided from answers the user can still go back and change. Stepping back and
// forward has to put the question back when the condition that removed it no
// longer holds, or the run proceeds on a decision nobody was offered.
func TestASkippedStepIsReconsideredOnTheWayForward(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{
		{
			Kind: flow.StepMultiSelect, Key: "branches", Label: "Worktrees",
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Options: []flow.Option{
					{Label: "a", Value: "a", Selected: true},
					{Label: "b", Value: "b", Selected: true},
				}}, nil
			},
		},
		{
			Kind: flow.StepSelect, Key: "reparent", Label: "Reparent children",
			// Stands in for prune's step: with everything selected nothing
			// survives to reparent, so the question does not arise.
			Skip: func(answers flow.Answers) (bool, string) {
				return len(answers.Values("branches")) == 2, "no children"
			},
			Options: []flow.Option{{Label: "Reparent", Value: "reparent"}},
		},
		{Kind: flow.StepRecap, Key: "recap", Label: "Recap", Options: []flow.Option{{Label: "go", Value: "go"}}},
	}}

	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})

	mo, _ = mo.update(namedKey(tea.KeyEnter)) // both checked → reparent skipped
	if mo.index != 2 {
		t.Fatalf("index = %d, want the reparent step skipped on the way out", mo.index)
	}

	mo, _ = mo.update(namedKey(tea.KeyEsc)) // back past the skipped step
	if mo.index != 0 {
		t.Fatalf("index = %d, want to land back on the selection", mo.index)
	}
	mo, _ = mo.update(key(" ")) // uncheck one — a child now survives
	mo, _ = mo.update(namedKey(tea.KeyEnter))

	if mo.index != 1 {
		t.Errorf("index = %d, want the reparent question asked now that it applies", mo.index)
	}
}

// A session that reduces to a single loaded step must still load: the modal
// enters its first step like any other, rather than starting on it.
func TestTheModalLoadsAStepThatComesFirst(t *testing.T) {
	session := flow.Session{Steps: []flow.Step{{
		Kind: flow.StepRecap, Key: "confirm", Label: "Confirm", Title: "Confirm",
		LoadingMessage: "Computing…",
		Load: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: "loaded body",
				Options:     []flow.Option{{Label: "Yes, do it", Value: "go"}},
			}, nil
		},
	}}}

	reply := make(chan promptReply, 1)
	mo, cmd := newModal(modalParams{Shape: modalStepper, Session: session, Reply: reply, Width: testWidth, Height: testHeight})
	if cmd == nil {
		t.Fatal("entering a loaded step must fire its load")
	}

	loaded, ok := cmd().(modalLoadedMsg)
	if !ok {
		t.Fatalf("the modal must ask for its content, got %T", cmd())
	}
	mo, _ = mo.update(loaded)

	body := strings.Join(mo.body(noMarks{}), "\n")
	if !strings.Contains(body, "loaded body") {
		t.Errorf("the modal must draw the loaded content:\n%s", body)
	}
}

func TestTheModalOpensASelectOnItsStartingValue(t *testing.T) {
	content := flow.StepContent{
		Options: []flow.Option{
			{Label: "feature-a", Value: "/wt/a"},
			{Label: "feature-b", Value: "/wt/b"},
		},
		Start: "/wt/b",
	}

	if got := newSelectList(content).Value(); got != "/wt/b" {
		t.Errorf("cursor = %q, want the starting value", got)
	}
}

// The two surfaces must agree on what a step's badges look like, so the modal
// renders them like the wizard does rather than dropping them.
func TestTheModalRendersTheBadgesAStepDeclares(t *testing.T) {
	content := flow.StepContent{Options: []flow.Option{{
		Label:  "feature-a",
		Value:  "/wt/a",
		Badges: []flow.Badge{{Text: "3 jobs", Tone: domain.ToneSuccess}, {Text: "current"}},
	}}}

	view := newSelectList(content).View()
	for _, want := range []string{"3 jobs", "current"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

func TestTheModalMultiSelectCarriesItsBadgesAndStart(t *testing.T) {
	multi := newMultiSelect(flow.Step{Kind: flow.StepMultiSelect, Key: "w"}, flow.StepContent{
		Options: []flow.Option{
			{Label: "main", Value: "/wt/main", Badges: []flow.Badge{{Text: "2 running", Tone: domain.ToneSuccess}}},
			{Label: "feat/x", Value: "/wt/x", Badges: []flow.Badge{{Text: "current"}}},
		},
		Start: "/wt/x",
	})
	view := multi.View()
	for _, want := range []string{"2 running", "current"} {
		if !strings.Contains(view, want) {
			t.Errorf("view = %q, want %q", view, want)
		}
	}
	multi, _ = multi.Update(key(" "))
	if got := strings.Join(multi.Values(), ","); got != "/wt/x" {
		t.Errorf("space toggled %q, want the row the step starts on", got)
	}
}

func textListSession() flow.Session {
	return flow.Session{Steps: []flow.Step{
		{
			Kind: flow.StepTextList, Key: "branches", Label: "Branches", Title: "Branches",
			ValidateEntry: func(check flow.EntryCheck) error {
				if check.Entry == "taken" {
					return errors.New("taken already")
				}
				return nil
			},
			EntryBadge: func(entry string) flow.Badge { return flow.Badge{Text: "new " + entry, Tone: domain.ToneSuccess} },
		},
		{Kind: flow.StepRecap, Key: "recap", Label: "Recap", Options: []flow.Option{{Label: "go", Value: "go"}}},
	}}
}

func typeInto(mo modal, text string) modal {
	for _, char := range text {
		mo, _ = mo.update(key(string(char)))
	}
	return mo
}

func TestTheModalAnswersATextListWithEveryEntry(t *testing.T) {
	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: textListSession(), Reply: reply, Width: testWidth, Height: testHeight})

	mo = typeInto(mo, "feat/a")
	mo, _ = mo.update(namedKey(tea.KeyTab))
	mo = typeInto(mo, "feat/b")
	mo, _ = mo.update(namedKey(tea.KeyEnter))
	_, cmd := mo.update(namedKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("confirming the recap must answer the session")
	}
	cmd()
	answered := <-reply
	if got := answered.answers.Values("branches"); len(got) != 2 || got[0] != "feat/a" || got[1] != "feat/b" {
		t.Errorf("values = %v, want both entries in the order typed", got)
	}
}

func TestTheModalTextListRefusesWhatTheStepRefuses(t *testing.T) {
	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: textListSession(), Reply: reply, Width: testWidth, Height: testHeight})

	mo = typeInto(mo, "taken")
	mo, _ = mo.update(namedKey(tea.KeyTab))

	if got := mo.textList.Values(); len(got) != 0 {
		t.Errorf("values = %v, want the refused entry kept out of the list", got)
	}
	body := strings.Join(mo.body(noMarks{}), "\n")
	if !strings.Contains(strings.ToLower(body), "taken already") {
		t.Errorf("the refusal must be on screen:\n%s", body)
	}
}

func TestTheModalRendersATextListBody(t *testing.T) {
	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: textListSession(), Reply: reply, Width: testWidth, Height: testHeight})

	mo = typeInto(mo, "feat/a")
	mo, _ = mo.update(namedKey(tea.KeyTab))

	body := strings.Join(mo.body(noMarks{}), "\n")
	if !strings.Contains(body, "new feat/a") {
		t.Errorf("each entry must carry the badge the step declares:\n%s", body)
	}
	if !strings.Contains(body, domain.DashboardStepperListHint) {
		t.Errorf("the footer must name the list controls:\n%s", body)
	}
}

// The step's Build only knows the entries the request carried — none, on this
// surface — so stepping back must put back what the user typed.
func TestGoingBackToATextListKeepsTheEntries(t *testing.T) {
	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: textListSession(), Reply: reply, Width: testWidth, Height: testHeight})

	mo = typeInto(mo, "feat/a")
	mo, _ = mo.update(namedKey(tea.KeyEnter))
	mo, _ = mo.update(namedKey(tea.KeyEsc))

	if got := mo.textList.Values(); len(got) != 1 || got[0] != "feat/a" {
		t.Errorf("values = %v, want the list typed before stepping forward", got)
	}
}

func TestEscOnTheFirstTextListCancelsTheRun(t *testing.T) {
	reply := make(chan promptReply, 1)
	mo, _ := newModal(modalParams{Shape: modalStepper, Session: textListSession(), Reply: reply, Width: testWidth, Height: testHeight})

	_, cmd := mo.update(namedKey(tea.KeyEsc))
	cmd()
	if answered := <-reply; !errors.Is(answered.err, domain.ErrUserAborted) {
		t.Errorf("err = %v, want the run cancelled", answered.err)
	}
}
