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
	virtual  map[string]bool
}

// StackOf builds the stack of the live worktrees from records, the meta record of each branch, by their source branches; base is the base branch. It never fails: the main worktree is a root whatever its record says, and a main worktree on no branch is no node, its children then under a virtual root of the branch they name; the base branch keeps its recorded parent; a worktree with no record or no source branch is a root; and the members of a cycle are promoted to roots, the cycle recorded for Cascade to refuse. Siblings, and the roots after the main one, are ordered by creation time, then by name; the virtual roots come last, by name.
func StackOf(worktrees []Worktree, records map[string]Meta, base string) Stack {
	stack := Stack{parents: map[string]string{}, children: map[string][]string{}, steps: map[string]bool{}, cycles: map[string][]string{}, virtual: map[string]bool{}}
	created := map[string]time.Time{}
	main := ""
	for _, w := range worktrees {
		branch := w.Branch()
		if branch == "" {
			continue
		}

		created[branch], _ = time.Parse(time.RFC3339, records[branch].CreatedAt)
		if w.Main {
			main = branch
			continue
		}

		stack.steps[branch] = branch != base
		if source := records[branch].SourceBranch; source != "" {
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
		if _, live := created[parent]; !live && !stack.virtual[parent] {
			stack.virtual[parent] = true
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

// Virtual reports whether branch is a virtual root: a parent with no live worktree, holding its children.
func (s Stack) Virtual(branch string) bool {
	return s.virtual[branch]
}

// InCycle reports whether branch's parents lead back to it: a member of a cycle, promoted to a root.
func (s Stack) InCycle(branch string) bool {
	_, loops := s.cycles[branch]
	return loops
}

// Cascade returns roots and the branches under them to rebase, each parent before its children, level by level from the stack's roots whatever the order roots are given in: the main worktree, the base branch and virtual roots are never a step. A member of a cycle among roots refuses the cascade with a *CycleError (a cycle's members being roots, roots is the only way to reach one); a cycle out of reach does not, where the legacy sync refuses any cycle: whether it should is sync's migration to decide.
func (s Stack) Cascade(roots []string) ([]string, error) {
	var steps []string
	kept := map[string]bool{}
	for queue := slices.Clone(s.roots); len(queue) > 0; queue = queue[1:] {
		branch := queue[0]
		queue = append(queue, s.children[branch]...)
		parent, hasParent := s.parents[branch]
		if !slices.Contains(roots, branch) && !(hasParent && kept[parent]) {
			continue
		}
		kept[branch] = true

		if members, loops := s.cycles[branch]; loops {
			return nil, &CycleError{Branches: members}
		}
		if s.steps[branch] {
			steps = append(steps, branch)
		}
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
