package worktree

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// Stack is the repository's worktrees as a forest, each under the branch it was created from. A node is a live worktree on a branch; a parent with no live worktree, deleted or never managed, is a virtual root holding its children.
type Stack struct {
	parents  map[string]string
	children map[string][]string
	roots    []string
	steps    map[string]bool
	cycles   map[string][]string
}

// StackOfParams are the live worktrees, the meta record of each branch, and the base branch.
type StackOfParams struct {
	Worktrees []Worktree
	Records   map[string]Meta
	Base      string
}

// StackOf builds the stack of the live worktrees from their records' source branches. It never fails: the main worktree and the base branch are roots whatever their record says, a worktree with no record or no source branch is a root, and the members of a cycle are promoted to roots, the cycle recorded for Cascade to refuse. Siblings, and the roots after the main one, are ordered by creation time, then by name; the virtual roots come last, by name.
func StackOf(params StackOfParams) Stack {
	stack := Stack{parents: map[string]string{}, children: map[string][]string{}, steps: map[string]bool{}, cycles: map[string][]string{}}
	created := map[string]time.Time{}
	main := ""
	for _, w := range params.Worktrees {
		branch := w.Branch()
		if branch == "" {
			continue
		}

		created[branch], _ = time.Parse(time.RFC3339, params.Records[branch].CreatedAt)
		if w.Main {
			main = branch
			continue
		}
		if branch == params.Base {
			continue
		}

		stack.steps[branch] = true
		if source := params.Records[branch].SourceBranch; source != "" {
			stack.parents[branch] = source
		}
	}

	for branch := range stack.parents {
		if members, loops := stack.cycleThrough(branch); loops {
			stack.cycles[branch] = members
		}
	}
	for branch := range stack.cycles {
		delete(stack.parents, branch)
	}

	byAge := func(a, b string) int {
		return cmp.Or(created[a].Compare(created[b]), strings.Compare(a, b))
	}
	var virtual []string
	for branch, parent := range stack.parents {
		if _, live := created[parent]; !live && !slices.Contains(virtual, parent) {
			virtual = append(virtual, parent)
		}
		stack.children[parent] = append(stack.children[parent], branch)
	}
	for _, children := range stack.children {
		slices.SortFunc(children, byAge)
	}

	for branch := range created {
		if _, child := stack.parents[branch]; !child && branch != main {
			stack.roots = append(stack.roots, branch)
		}
	}
	slices.SortFunc(stack.roots, byAge)
	slices.Sort(virtual)
	if main != "" {
		stack.roots = append([]string{main}, stack.roots...)
	}
	stack.roots = append(stack.roots, virtual...)
	return stack
}

// Roots returns the roots of the stack: the main worktree's branch, the other roots by creation time, then the virtual roots by name.
func (s Stack) Roots() []string {
	return slices.Clone(s.roots)
}

// Children returns the branches created from branch, by creation time.
func (s Stack) Children(branch string) []string {
	return slices.Clone(s.children[branch])
}

// Parent returns the branch branch was created from, and reports whether it has one: false for a root.
func (s Stack) Parent(branch string) (string, bool) {
	parent, ok := s.parents[branch]
	return parent, ok
}

// Cascade returns the branches to rebase from roots down, each parent before its children, level by level: roots included, the main worktree, the base branch and virtual roots never. A cycle reachable from roots refuses the cascade with a *CycleError.
func (s Stack) Cascade(roots []string) ([]string, error) {
	var steps []string
	seen := map[string]bool{}
	for queue := slices.Clone(roots); len(queue) > 0; queue = queue[1:] {
		branch := queue[0]
		if seen[branch] {
			continue
		}
		seen[branch] = true

		if members, loops := s.cycles[branch]; loops {
			return nil, &CycleError{Branches: members}
		}
		if s.steps[branch] {
			steps = append(steps, branch)
		}
		queue = append(queue, s.children[branch]...)
	}

	return steps, nil
}

// cycleThrough returns the branches of the cycle branch's parents lead back to it through, sorted, and reports whether they do.
func (s Stack) cycleThrough(branch string) ([]string, bool) {
	members := []string{branch}
	for parent, ok := s.parents[branch]; ok; parent, ok = s.parents[parent] {
		if parent == branch {
			slices.Sort(members)
			return members, true
		}
		if slices.Contains(members, parent) {
			return nil, false
		}
		members = append(members, parent)
	}

	return nil, false
}

// CycleError is the error of a cascade that reaches worktrees whose parents lead back to themselves.
type CycleError struct{ Branches []string }

// Error names the branches of the cycle.
func (e *CycleError) Error() string {
	return "the parents of these worktrees form a cycle: " + strings.Join(e.Branches, ", ")
}
