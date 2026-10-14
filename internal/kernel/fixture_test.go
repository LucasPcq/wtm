package kernel_test

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/internal/kernel"
)

type isolation string

type target struct {
	From string `json:"from,omitempty"`
}

type request struct {
	Branches  []string          `json:"branches"`
	From      string            `json:"from,omitempty"`
	Isolation isolation         `json:"isolation,omitempty"`
	Push      bool              `json:"push,omitempty"`
	NoPush    bool              `json:"no_push,omitempty"`
	URLHost   string            `json:"url_host,omitempty"`
	URLPort   string            `json:"url_port,omitempty"`
	Decisions map[string]string `json:"decisions,omitempty"`
	Target    target            `json:"target"`
	Remember  []string          `json:"remember,omitempty"`
}

type facts struct {
	Existing []string
	Base     string
}

var errBoom = errors.New("boom")

func fail(context.Context, kernel.Reporter) error { return errBoom }

func failUndo(context.Context) error { return errBoom }

type disk struct {
	mu      sync.Mutex
	entries []string
}

func (d *disk) add(name string) func(context.Context, kernel.Reporter) error {
	return func(context.Context, kernel.Reporter) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.entries = append(d.entries, name)
		return nil
	}
}

func (d *disk) remove(name string) func(context.Context) error {
	return func(context.Context) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.entries = slices.DeleteFunc(d.entries, func(entry string) bool { return entry == name })
		return nil
	}
}

func (d *disk) list() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.entries)
}

type recorder struct {
	mu   sync.Mutex
	seen []kernel.Progress
}

func (r *recorder) Emit(progress kernel.Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, progress)
}

func (r *recorder) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := make([]string, 0, len(r.seen))
	for _, progress := range r.seen {
		lines = append(lines, progress.Base().Subject+" "+line(progress))
	}
	return lines
}

func line(progress kernel.Progress) string {
	switch p := progress.(type) {
	case *kernel.UnitStarted:
		return "unit.started"
	case *kernel.UnitFinished:
		return "unit.finished " + string(p.Status)
	case *kernel.PhaseStarted:
		return "phase.started " + p.Phase
	case *kernel.PhaseFinished:
		return "phase.finished " + p.Phase
	case *kernel.Output:
		return "output " + p.Line
	case *kernel.StatusChanged:
		return "status " + string(p.Code)
	}
	return ""
}

type createPlan struct {
	Branches []string
	From     string
}

var branchExists = kernel.Condition[request, facts]{
	Code: "test.branch_exists",
	Holds: func(req request, f facts) bool {
		return slices.ContainsFunc(req.Branches, func(branch string) bool { return slices.Contains(f.Existing, branch) })
	},
}

func createFields() []kernel.FieldDef[request, facts] {
	return []kernel.FieldDef[request, facts]{
		{
			Spec: kernel.FieldSpec{
				Path: "branches", Type: kernel.FieldTextList, Label: "Branches", Required: true,
				Constraints: kernel.Constraints{Pattern: `^[a-z0-9/-]+$`},
			},
		},
		{
			Spec: kernel.FieldSpec{Path: "from", Type: kernel.FieldSelect, Label: "Source", Choices: kernel.ChoicesSearch, DependsOn: []string{"branches"}},
			Default: func(req request, f facts) (kernel.Fallback, bool) {
				fresh := !branchExists.Holds(req, f)
				return kernel.Fallback{Value: kernel.Text(f.Base), Origin: kernel.OriginConfig}, fresh
			},
		},
		{
			Spec: kernel.FieldSpec{
				Path: "isolation", Type: kernel.FieldSelect, Label: "Isolation", Rememberable: true,
				Constraints: kernel.Constraints{Enum: []string{"isolated", "verbatim"}},
			},
			Default: func(request, facts) (kernel.Fallback, bool) {
				return kernel.Fallback{Value: kernel.Text("isolated"), Origin: kernel.OriginDefault}, true
			},
		},
	}
}

func createRules() rules {
	return rules{}.
		Distinct("branches").
		RequiredWhen("from", branchExists).
		NotSelfParent(kernel.SelfParentRule{Parent: "from", Children: "branches"})
}

func createUnit(d *disk, branch string) kernel.Unit[string] {
	configure := phase("configure", func(_ context.Context, path string) (string, error) { return path + " configured", nil })
	return worktreeUnit(d, branch, configure)
}

func createCommand(d *disk) kernel.Command[request, facts, createPlan, string] {
	return kernel.Command[request, facts, createPlan, string]{
		Name: "create",
		Observe: func(context.Context) (facts, error) {
			return facts{Existing: []string{"feat/old"}, Base: "main"}, nil
		},
		Fields: createFields(),
		Rules:  createRules(),
		Locks: func(req request) []kernel.LockKey {
			locks := []kernel.LockKey{{Scope: kernel.LockRepo, Mode: kernel.LockShared}}
			for _, branch := range req.Branches {
				locks = append(locks, kernel.LockKey{Scope: kernel.LockWorktree, Name: branch, Mode: kernel.LockExclusive})
			}
			return locks
		},
		Plan: func(_ context.Context, req request, _ facts) (kernel.Plan[createPlan], error) {
			return kernel.Plan[createPlan]{Detail: createPlan{Branches: req.Branches, From: req.From}}, nil
		},
		Apply: func(ctx context.Context, in kernel.ApplyInput[request, facts, createPlan, string]) (kernel.Outcome[string], error) {
			items := kernel.Each(ctx, kernel.EachParams[string, string]{
				Items:      in.Plan.Detail.Branches,
				Subject:    func(branch string) string { return branch },
				Unit:       func(branch string) kernel.Unit[string] { return createUnit(d, branch) },
				Emit:       in.Emit,
				Shield:     in.Shield,
				Milestones: in.Milestones,
			})
			return kernel.Outcome[string]{Items: items}, nil
		},
	}
}
