package components

import (
	"os"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// everyStepModel is one instance of each model a Step may carry. The wizard
// dispatches on the concrete type in half a dozen switches; a model missing from
// one of them renders blank and swallows every key, so this table is what keeps
// a new model from being wired into some of them only.
func everyStepModel() []Step {
	const desc = "the description"
	return []Step{
		{Name: "select", Model: NewSelectList(NewSelectListParams{Title: "t", Description: desc, Items: []SelectItem{{Label: "a"}}})},
		{Name: "text", Model: NewTextInput(NewTextInputParams{Title: "t", Description: desc})},
		{Name: "confirm", Model: NewConfirm(NewConfirmParams{Title: "t", Description: desc})},
		{Name: "multiselect", Model: NewMultiSelect(NewMultiSelectParams{Title: "t", Description: desc, Items: []MultiSelectItem{{Label: "a"}}})},
		{Name: "reorder", Model: NewReorderList(NewReorderListParams{Title: "t", Description: desc, Items: []ReorderItem{{Label: "a"}}})},
		{Name: "hooks", Model: NewHookList(NewHookListParams{Title: "t", Description: desc, Hooks: []domain.HookCommand{{Cmd: "echo"}}})},
		{Name: "env", Model: NewEnvResolve(NewEnvResolveParams{Title: "t", Description: desc, Files: []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{{Key: "PORT", Status: domain.EnvKeyMissing}}}}}})},
		{Name: "ports", Model: NewPortList(NewPortListParams{Title: "t", Description: desc, Entries: []domain.PortEntry{{Job: "web", Name: "PORT", Base: 3000}}})},
		{Name: "kinds", Model: NewKindList(NewKindListParams{Title: "t", Description: desc, Entries: []domain.JobKindChoice{{Label: "root / build", Cmd: "turbo run build", Name: "build", Kind: domain.JobKindTask}}})},
		{Name: "cmds", Model: NewCmdList(NewCmdListParams{Title: "t", Description: desc, Fixes: []domain.JobCmdFix{{Job: "web", Cmd: "vite", Vars: []string{"PORT"}}}})},
		{Name: "profiles", Model: NewProfileList(NewProfileListParams{Title: "t", Description: desc, Profiles: []domain.ProfileConfig{{Name: "all"}}})},
		{Name: "routes", Model: NewRouteList(NewRouteListParams{Title: "t", Description: desc, Rows: []domain.PortRouteRow{{Job: "web", Port: "PORT", Base: 3000, File: ".env"}}})},
		{Name: "runners", Model: NewRunnerList(NewRunnerListParams{Title: "t", Description: desc, Choices: []domain.JobRunnerChoice{{Job: "web", Label: "web  apps/web", Options: []string{"", "dev"}}}})},
		{Name: "scopes", Model: NewScopeList(NewScopeListParams{Title: "t", Description: desc, Entries: []rules.ServiceScopeChoice{{File: "docker-compose.yml", Service: "db"}}})},
		{Name: "namespaces", Model: NewNamespaceList(NewNamespaceListParams{Title: "t", Description: desc, Fields: []domain.NamespaceField{{Job: "db", Field: domain.NamespaceFieldName, Value: "app_{worktree}"}}})},
		{Name: "envvalues", Model: NewEnvValueList(NewEnvValueListParams{Title: "t", Description: desc, Fields: []domain.EnvValueField{{Job: "db", File: ".env", Key: "DATABASE_URL", Value: "{namespace}"}}})},
	}
}

func TestWizardRendersEveryStepModel(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			if view := m.viewStep(0); view == "" {
				t.Error("the step rendered nothing: its type is missing from viewStep")
			}
		})
	}
}

func TestWizardRoutesEscapeToEveryStepModel(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			s := step
			_, back, _ := (WizardModel{}).updateStep(&s, key(tea.KeyEsc))
			if !back {
				t.Error("escape did nothing: the type is missing from updateStep, so no key reaches the step")
			}
		})
	}
}

func TestWizardReadsEveryStepDescription(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			if got := m.stepDescription(step); got != "the description" {
				t.Errorf("description = %q, want the one it was built with", got)
			}
		})
	}
}

func TestWizardSizesEveryStepModel(t *testing.T) {
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			m := NewWizard([]Step{step})
			m.width, m.height = 120, 40
			m.propagateSize(0)

			if same := reflect.DeepEqual(m.steps[0].Model, step.Model); same {
				t.Error("the model came back untouched: its type is missing from propagateSize, so the step never resizes")
			}
		})
	}
}

func TestEveryTypeSwitchInWizardHandlesEveryStepModel(t *testing.T) {
	// The behavioural tests above can only assert what a model observably does;
	// initStep returns nil for most of them. This reads the dispatch itself, so
	// every switch is covered — including the ones whose omission is silent.
	src, err := os.ReadFile("wizard.go")
	if err != nil {
		t.Fatalf("read wizard.go: %v", err)
	}

	want := map[string]bool{}
	for _, step := range everyStepModel() {
		want[reflect.TypeOf(step.Model).Name()] = true
	}

	for _, block := range typeSwitchBlocks(string(src)) {
		for name := range want {
			if !strings.Contains(block.body, "case "+name+":") {
				t.Errorf("the type switch at line %d does not handle %s — that step would silently do nothing there",
					block.line, name)
			}
		}
	}
}

type typeSwitch struct {
	line int
	body string
}

// typeSwitchBlocks finds each type switch that BINDS the concrete value
// (`switch child := … .(type)`) and returns its body. Binding is what tells a
// dispatch apart from a lookup: renderHelpBar switches on the type without
// binding it and answers the rest from a default, so an omission there is not a
// hole. In a dispatch it is — the step silently does nothing.
func typeSwitchBlocks(src string) []typeSwitch {
	var blocks []typeSwitch
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if !strings.Contains(line, ".(type) {") || !strings.Contains(line, ":=") {
			continue
		}
		depth, body := 0, strings.Builder{}
		for _, l := range lines[i:] {
			depth += strings.Count(l, "{") - strings.Count(l, "}")
			body.WriteString(l + "\n")
			if depth == 0 {
				break
			}
		}
		blocks = append(blocks, typeSwitch{line: i + 1, body: body.String()})
	}
	return blocks
}

func TestWizardResetsEveryStepModel(t *testing.T) {
	// resetStep clears done/aborted on back-navigation; a type missing from it
	// keeps a stale flag and the wizard walks straight forward again.
	for _, step := range everyStepModel() {
		t.Run(step.Name, func(t *testing.T) {
			s := step
			_, _, _ = (WizardModel{}).updateStep(&s, key(tea.KeyEsc))

			m := NewWizard([]Step{s})
			m.resetStep(0)

			if _, back, _ := (WizardModel{}).updateStep(&m.steps[0], tea.WindowSizeMsg{}); back {
				t.Error("the step is still marked aborted: its type is missing from resetStep")
			}
		})
	}
}
