package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// The golden freezes the recap `wtm extract` draws before its move to
// internal/flow (LUC-240); the flow reads it back (LUC-241). Regenerate it from
// this package only, with WTM_WRITE_EXTRACT_GOLDEN=1.

type recapCase struct {
	Name   string     `json:"name"`
	Input  recapInput `json:"input"`
	Recap  string     `json:"recap"`
	Action string     `json:"action"`
}

type recapInput struct {
	SourceArg        string           `json:"source_arg,omitempty"`
	SourcePicked     string           `json:"source_picked,omitempty"`
	FilesFlag        []string         `json:"files_flag,omitempty"`
	FilesPicked      []string         `json:"files_picked,omitempty"`
	TargetFlag       string           `json:"target_flag,omitempty"`
	TargetPicked     string           `json:"target_picked,omitempty"`
	CreateNew        bool             `json:"create_new,omitempty"`
	Branch           string           `json:"branch,omitempty"`
	From             string           `json:"from,omitempty"`
	BranchExists     bool             `json:"branch_exists,omitempty"`
	Behind           bool             `json:"behind,omitempty"`
	FastForward      bool             `json:"fast_forward,omitempty"`
	Diverged         bool             `json:"diverged,omitempty"`
	EnvFallback      bool             `json:"env_fallback,omitempty"`
	IsolationApplies bool             `json:"isolation_applies,omitempty"`
	IsolationFlag    domain.Isolation `json:"isolation_flag,omitempty"`
	IsolationAnswer  domain.Isolation `json:"isolation_answer,omitempty"`
	IsolationDefault domain.Isolation `json:"isolation_default,omitempty"`
	KeepFlag         bool             `json:"keep_flag,omitempty"`
	ModeAsked        bool             `json:"mode_asked,omitempty"`
	ModeKeep         bool             `json:"mode_keep,omitempty"`
}

func recapCases() []recapCase {
	return []recapCase{
		{Name: "everything picked, an existing target", Input: recapInput{SourcePicked: "feat/src", FilesPicked: []string{"a.txt", "b.txt"}, TargetPicked: "feat/dst", ModeAsked: true}},
		{Name: "everything from flags", Input: recapInput{SourceArg: "feat/src", FilesFlag: []string{"a.txt"}, TargetFlag: "feat/dst", KeepFlag: true}},
		{Name: "fixed source, copy asked", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, TargetPicked: "feat/dst", ModeAsked: true, ModeKeep: true}},
		{Name: "fixed files, a target flag, mode asked", Input: recapInput{SourceArg: "feat/src", FilesFlag: []string{"apps/api", "b.txt"}, TargetFlag: "feat/dst", ModeAsked: true}},
		{Name: "a new worktree", Input: recapInput{SourcePicked: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", ModeAsked: true}},
		{Name: "a new worktree, keep fixed", Input: recapInput{SourceArg: "feat/src", FilesFlag: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", KeepFlag: true}},
		{Name: "a new worktree, its source fast-forwarded", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", Behind: true, FastForward: true, ModeAsked: true}},
		{Name: "a new worktree, the offer declined", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", Behind: true, ModeAsked: true}},
		{Name: "a reused branch", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/old", From: "main", BranchExists: true, ModeAsked: true}},
		{Name: "a reused branch, fast-forwarded itself", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/old", From: "main", BranchExists: true, Behind: true, FastForward: true, ModeAsked: true}},
		{Name: "a diverged source", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", Diverged: true, ModeAsked: true}},
		{Name: "the parent env falls back to main", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "feat/base", EnvFallback: true, ModeAsked: true}},
		{Name: "both warnings", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "feat/base", Diverged: true, EnvFallback: true, ModeAsked: true}},
		{Name: "isolation asked", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", IsolationApplies: true, IsolationAnswer: domain.IsolationVerbatim, ModeAsked: true}},
		{Name: "isolation from its default", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", IsolationApplies: true, IsolationDefault: domain.IsolationVerbatim, IsolationAnswer: domain.IsolationVerbatim, ModeAsked: true}},
		{Name: "isolation from its flag", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", IsolationApplies: true, IsolationFlag: domain.IsolationVerbatim, ModeAsked: true}},
		{Name: "isolation flag where nothing isolates", Input: recapInput{SourceArg: "feat/src", FilesPicked: []string{"a.txt"}, CreateNew: true, Branch: "feat/new", From: "main", IsolationFlag: domain.IsolationIsolated, ModeAsked: true}},
	}
}

func selectStep(name, value string) components.Step {
	return components.Step{
		Name:  name,
		Model: components.NewSelectList(components.NewSelectListParams{Items: []components.SelectItem{{Label: value, Value: value}}}),
	}
}

func createParams(in recapInput) newpicker.WizardParams {
	subject := func(up newpicker.SourceUpdateParams) string {
		if in.BranchExists && up.Branch != "" {
			return up.Branch
		}
		return up.Source
	}
	return newpicker.WizardParams{
		IncludeBranch:     true,
		IsolationApplies:  in.IsolationApplies,
		IsolationOverride: in.IsolationFlag,
		IsolationDefault:  in.IsolationDefault,
		Target: func(string) domain.BranchTarget {
			if in.BranchExists {
				return domain.BranchTarget{State: domain.BranchTargetExisting}
			}
			return domain.BranchTarget{State: domain.BranchTargetNew}
		},
		SourceUpdate: func(up newpicker.SourceUpdateParams) newpicker.SourceUpdatePrompt {
			switch {
			case in.Diverged:
				return newpicker.SourceUpdatePrompt{
					Branch: subject(up), Show: true, AbortOnDecline: true,
					Params: components.NewConfirmParams{Warning: domain.SourceDivergedWarning},
				}
			case in.Behind:
				return newpicker.SourceUpdatePrompt{Branch: subject(up), Show: true}
			}
			return newpicker.SourceUpdatePrompt{Branch: subject(up)}
		},
		EnvFallback: func(string, string) (bool, components.NewConfirmParams) {
			if !in.EnvFallback {
				return false, components.NewConfirmParams{}
			}
			return true, components.NewConfirmParams{Warning: domain.EnvParentFallbackWarning}
		},
	}
}

func recapFor(in recapInput) components.RecapContent {
	params := RunParams{
		SourceBranch: in.SourceArg,
		FixedFiles:   in.FilesFlag,
		FixedTarget:  in.TargetFlag,
		FixedKeep:    in.KeepFlag,
		NeedMode:     in.ModeAsked,
		Create:       createParams(in),
	}

	var prev []components.Step
	if in.SourcePicked != "" {
		prev = append(prev, selectStep(stepSource, in.SourcePicked))
	}
	if len(in.FilesPicked) > 0 {
		items := make([]components.MultiSelectItem, 0, len(in.FilesPicked))
		for _, path := range in.FilesPicked {
			items = append(items, components.MultiSelectItem{Value: path, Selected: true})
		}
		prev = append(prev, components.Step{Name: stepFiles, Model: components.NewMultiSelect(components.NewMultiSelectParams{Items: items})})
	}
	switch {
	case in.CreateNew:
		prev = append(prev, selectStep(stepTarget, newWorktreeValue),
			components.Step{Name: "Branch name", Model: components.NewTextInput(components.NewTextInputParams{Default: in.Branch})},
			selectStep("Source branch", in.From))
		if in.IsolationApplies && in.IsolationFlag == "" {
			prev = append(prev, selectStep(domain.IsolationStepName, string(in.IsolationAnswer)))
		}
		update := "keep"
		if in.FastForward {
			update = "ff"
		}
		prev = append(prev, selectStep("Source update", update))
	case in.TargetPicked != "":
		prev = append(prev, selectStep(stepTarget, in.TargetPicked))
	}
	if in.ModeAsked {
		mode := modeMove
		if in.ModeKeep {
			mode = modeKeep
		}
		prev = append(prev, selectStep(stepMode, mode))
	}
	return buildCombinedRecap(prev, params)
}

func TestRecapGolden(t *testing.T) {
	path := filepath.Join("..", "..", "flow", "extract", "testdata", "recap.golden.json")
	cases := recapCases()
	for i := range cases {
		content := recapFor(cases[i].Input)
		cases[i].Recap = content.Description
		cases[i].Action = content.Actions[0].Label
	}

	if os.Getenv("WTM_WRITE_EXTRACT_GOLDEN") == "1" {
		data, err := json.MarshalIndent(cases, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden []recapCase
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden) != len(cases) {
		t.Fatalf("golden holds %d cases, the wizard %d", len(golden), len(cases))
	}
	for i, c := range cases {
		if c.Recap != golden[i].Recap || c.Action != golden[i].Action {
			t.Errorf("%s:\n got %q / %q\nwant %q / %q", c.Name, c.Recap, c.Action, golden[i].Recap, golden[i].Action)
		}
	}
}
