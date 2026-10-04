package checkout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// The golden was written from the recap `wtm checkout` drew before it moved here
// (LUC-237); every case must read exactly as it did.
type recapCase struct {
	Name  string     `json:"name"`
	Input recapInput `json:"input"`
	Recap string     `json:"recap"`
}

type recapInput struct {
	PR                domain.PRInfo    `json:"pr"`
	Picked            bool             `json:"picked,omitempty"`
	FromFlag          string           `json:"from_flag,omitempty"`
	EnvFlag           string           `json:"env_flag,omitempty"`
	IsolationFlag     domain.Isolation `json:"isolation_flag,omitempty"`
	ParentAnswer      string           `json:"parent_answer,omitempty"`
	EnvAnswer         string           `json:"env_answer,omitempty"`
	EnvAsked          bool             `json:"env_asked,omitempty"`
	IsolationAnswer   domain.Isolation `json:"isolation_answer,omitempty"`
	IsolationApplies  bool             `json:"isolation_applies,omitempty"`
	IsolationDefault  domain.Isolation `json:"isolation_default,omitempty"`
	BranchExists      bool             `json:"branch_exists,omitempty"`
	ParentFallsToMain bool             `json:"parent_falls_to_main,omitempty"`
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

func flowFor(t *testing.T, in recapInput) *checkoutFlow {
	t.Helper()
	config := domain.Config{}
	config.Project.Env.Strategy = domain.EnvStrategyExample
	config.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	request := Request{From: in.FromFlag, EnvFrom: in.EnvFlag, Isolation: in.IsolationFlag}
	if !in.Picked {
		request.Number = in.PR.Number
	}
	f := &checkoutFlow{
		ctx:      flow.Context{ProjectDir: gittest.InitRepo(t), StateDir: t.TempDir(), Config: config},
		request:  request,
		prompter: flow.Unattended{},
		applies:  in.IsolationApplies,
		target: func(string) domain.BranchTarget {
			if in.BranchExists {
				return domain.BranchTarget{State: domain.BranchTargetExisting}
			}
			return domain.BranchTarget{State: domain.BranchTargetNew}
		},
	}
	f.setPRs([]domain.PRInfo{in.PR})
	return f
}

func answersFor(in recapInput) flow.Answers {
	values := map[string]string{
		KeyPR:     strconv.Itoa(in.PR.Number),
		KeyParent: in.FromFlag,
		KeyEnv:    in.EnvFlag,
	}
	if in.FromFlag == "" {
		values[KeyParent] = in.ParentAnswer
	}
	if in.EnvFlag == "" {
		values[KeyEnv] = in.EnvAnswer
	}
	switch {
	case in.IsolationFlag != "":
		values[KeyIsolation] = string(in.IsolationFlag)
	case in.IsolationApplies:
		values[KeyIsolation] = string(in.IsolationAnswer)
	}
	return flow.NewAnswers(values)
}

func TestRecapReadsAsItAlwaysDid(t *testing.T) {
	cases := loadRecapGolden(t)
	if len(cases) == 0 {
		t.Fatal("the golden holds no case")
	}
	for _, c := range cases {
		t.Run(c.Name[:1], func(t *testing.T) {
			if got := flowFor(t, c.Input).recap(answersFor(c.Input)); got != c.Recap {
				t.Errorf("%s:\n got %q\nwant %q", c.Name, got, c.Recap)
			}
		})
	}
}
