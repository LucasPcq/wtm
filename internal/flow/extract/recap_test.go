package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// The golden was written from the recap `wtm extract` drew before it moved here
// (LUC-240); every case must read exactly as it did.
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

func loadRecapGolden(t *testing.T) []recapCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "recap.golden.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var cases []recapCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	return cases
}

func recapFlowFor(t *testing.T, in recapInput) *extractFlow {
	t.Helper()
	config := domain.Config{}
	config.Project.Env.Strategy = domain.EnvStrategyExample
	if in.EnvFallback {
		config.Project.Env.Strategy = domain.EnvStrategyParent
		config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	}
	f := &extractFlow{
		ctx:     flow.Context{ProjectDir: gittest.InitRepo(t), StateDir: t.TempDir(), Config: config},
		request: Request{Source: in.SourceArg, Files: in.FilesFlag, To: in.TargetFlag, Isolation: in.IsolationFlag},
		target: func(string) domain.BranchTarget {
			if in.BranchExists {
				return domain.BranchTarget{State: domain.BranchTargetExisting}
			}
			return domain.BranchTarget{State: domain.BranchTargetNew}
		},
		update: func(answers flow.Answers) decide.SourceUpdatePrompt {
			subject := answers.Value(create.KeySource)
			if in.BranchExists {
				subject = answers.Value(create.KeyBranch)
			}
			switch {
			case in.Diverged:
				return decide.SourceUpdatePrompt{Branch: subject, Show: true, AbortOnDecline: true, Warning: domain.SourceDivergedWarning}
			case in.Behind:
				return decide.SourceUpdatePrompt{Branch: subject, Show: true}
			}
			return decide.SourceUpdatePrompt{Branch: subject}
		},
	}
	f.create = f.embed()
	return f
}

func recapAnswersFor(in recapInput) flow.Answers {
	values := map[string]string{
		KeySource: in.SourceArg + in.SourcePicked,
		KeyTarget: in.TargetFlag + in.TargetPicked,
		KeyMode:   modeOf(in.KeepFlag || in.ModeKeep),
	}
	if in.CreateNew {
		values[KeyTarget] = targetCreate
		values[create.KeyBranch] = in.Branch
		values[create.KeySource] = in.From
		values[create.KeySourceUpdate] = decide.UpdateKeep
		if in.FastForward {
			values[create.KeySourceUpdate] = decide.UpdateFastForward
		}
		switch {
		case in.IsolationFlag != "":
			values[create.KeyIsolation] = string(in.IsolationFlag)
		case in.IsolationApplies:
			values[create.KeyIsolation] = string(in.IsolationAnswer)
		}
	}
	return flow.NewAnswers(values).WithValues(KeyFiles, append(in.FilesFlag, in.FilesPicked...))
}

func TestRecapReadsAsItAlwaysDid(t *testing.T) {
	cases := loadRecapGolden(t)
	if len(cases) == 0 {
		t.Fatal("the golden holds no case")
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			f := recapFlowFor(t, c.Input)
			content, err := f.recapStep().Build(recapAnswersFor(c.Input))
			if err != nil {
				t.Fatal(err)
			}
			if content.Description != c.Recap {
				t.Errorf("recap:\n got %q\nwant %q", content.Description, c.Recap)
			}
			if got := content.Options[0].Label; got != c.Action {
				t.Errorf("action = %q, want %q", got, c.Action)
			}
		})
	}
}
