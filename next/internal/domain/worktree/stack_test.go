package worktree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStackHangsEachWorktreeUnderTheBranchItWasCreatedFrom(t *testing.T) {
	cases := map[string]struct {
		worktrees []Worktree
		records   map[string]Meta
		base      string
		roots     []string
		children  map[string][]string
		parents   map[string]string
	}{
		"a linear stack": {
			worktrees: live("a", "b", "c"),
			records:   map[string]Meta{"a": from("main"), "b": from("a"), "c": from("b")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}, "a": {"b"}, "b": {"c"}, "c": nil},
			parents:   map[string]string{"a": "main", "b": "a", "c": "b"},
		},
		"a fan-out ordered by creation time, then by name": {
			worktrees: live("late", "early", "tie-b", "tie-a"),
			records:   map[string]Meta{"late": at("main", "2026-03-01T00:00:00Z"), "early": at("main", "2026-01-01T00:00:00+02:00"), "tie-b": at("main", "2026-02-01T00:00:00Z"), "tie-a": at("main", "2026-02-01T00:00:00Z")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"early", "tie-a", "tie-b", "late"}},
		},
		"several roots, main first, a worktree with no record or no source among them": {
			worktrees: live("newer", "older", "unrecorded"),
			records:   map[string]Meta{"newer": at("", "2026-02-01T00:00:00Z"), "older": at("", "2026-01-01T00:00:00Z")},
			roots:     []string{"main", "unrecorded", "older", "newer"},
			children:  map[string][]string{"main": nil},
			parents:   map[string]string{"newer": "", "older": "", "unrecorded": ""},
		},
		"the main worktree is a root whatever its record says": {
			worktrees: live("a"),
			records:   map[string]Meta{"main": from("a"), "a": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
			parents:   map[string]string{"main": "", "a": "main"},
		},
		"the base branch keeps its recorded parent": {
			worktrees: live("develop", "feat"),
			records:   map[string]Meta{"develop": from("main"), "feat": from("develop")},
			base:      "develop",
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"develop"}, "develop": {"feat"}},
			parents:   map[string]string{"develop": "main"},
		},
		"a main worktree on no branch is no node, its children under a virtual root": {
			worktrees: []Worktree{{Path: "/repo", Main: true, State: Detached{}}, {Path: "/trees/a", State: OnBranch{Branch: "a"}}},
			records:   map[string]Meta{"a": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
			parents:   map[string]string{"a": "main"},
		},
		"a deleted parent is a virtual root holding its children": {
			worktrees: live("a", "b"),
			records:   map[string]Meta{"a": from("gone"), "b": from("gone"), "gone": from("main")},
			roots:     []string{"main", "gone"},
			children:  map[string][]string{"gone": {"a", "b"}, "main": nil},
			parents:   map[string]string{"a": "gone", "gone": ""},
		},
		"a branch of origin, as create records --from origin/x, is a virtual root": {
			worktrees: live("a", "b"),
			records:   map[string]Meta{"a": from("origin/x"), "b": from("unmanaged")},
			roots:     []string{"main", "origin/x", "unmanaged"},
			children:  map[string][]string{"origin/x": {"a"}, "unmanaged": {"b"}},
		},
		"the members of a cycle are promoted to roots, what hangs from them stays": {
			worktrees: live("a", "b", "c"),
			records:   map[string]Meta{"a": at("b", "2026-01-01T00:00:00Z"), "b": at("a", "2026-02-01T00:00:00Z"), "c": from("a")},
			roots:     []string{"main", "a", "b"},
			children:  map[string][]string{"a": {"c"}, "b": nil},
			parents:   map[string]string{"a": "", "b": "", "c": "a"},
		},
		"a worktree created from itself is a root": {
			worktrees: live("a"),
			records:   map[string]Meta{"a": from("a")},
			roots:     []string{"main", "a"},
			children:  map[string][]string{"a": nil},
			parents:   map[string]string{"a": ""},
		},
		"a detached worktree is not a node, a rebasing one is its branch's": {
			worktrees: append(live(), Worktree{Path: "/trees/detached", State: Detached{}}, Worktree{Path: "/trees/a", State: Rebasing{Branch: "a"}}),
			records:   map[string]Meta{"a": from("main"), "": from("a")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}, "a": nil},
		},
		"a record with no live worktree is not a node": {
			worktrees: live("a"),
			records:   map[string]Meta{"a": from("main"), "removed": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
		},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			stack := StackOf(StackOfParams{Worktrees: c.worktrees, Records: c.records, Base: c.base})

			assert.Equal(t, c.roots, stack.Roots())
			for branch, children := range c.children {
				assert.Equal(t, children, stack.Children(branch), "children of %s", branch)
			}
			for branch, want := range c.parents {
				parent, ok := stack.Parent(branch)
				assert.Equal(t, want, parent, "parent of %s", branch)
				assert.Equal(t, want != "", ok, "parent of %s", branch)
			}
		})
	}
}

func TestACascadeRebasesEachParentBeforeItsChildren(t *testing.T) {
	stack := StackOf(StackOfParams{
		Worktrees: live("develop", "a", "a1", "b", "d", "e", "x", "y", "z"),
		Records: map[string]Meta{
			"develop": from("main"), "a": at("main", "2026-01-01T00:00:00Z"), "a1": from("a"), "b": at("main", "2026-02-01T00:00:00Z"),
			"d": from("develop"), "e": from("gone"), "x": from("y"), "y": from("x"), "z": from("x"),
		},
		Base: "develop",
	})
	cases := map[string]struct {
		roots []string
		steps []string
	}{
		"level by level, main never a step":          {roots: []string{"main"}, steps: []string{"a", "b", "d", "a1"}},
		"the base branch never a step, its children": {roots: []string{"develop"}, steps: []string{"d"}},
		"a root that is a worktree is a step":        {roots: []string{"a"}, steps: []string{"a", "a1"}},
		"a virtual root never a step":                {roots: []string{"gone"}, steps: []string{"e"}},
		"each branch once":                           {roots: []string{"main", "a", "a1"}, steps: []string{"a", "b", "d", "a1"}},
		"a parent before a child given first":        {roots: []string{"a1", "a"}, steps: []string{"a", "a1"}},
		"a child given beside its ancestor at depth": {roots: []string{"main", "a1"}, steps: []string{"a", "b", "d", "a1"}},
		"an unknown branch nothing":                  {roots: []string{"nope"}},
		"a branch under a cycle, without the cycle":  {roots: []string{"z"}, steps: []string{"z"}},
		"a cycle out of reach does not refuse":       {roots: []string{"main", "develop", "gone"}, steps: []string{"a", "b", "e", "d", "a1"}},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			steps, err := stack.Cascade(c.roots)

			assert.NoError(t, err)
			assert.Equal(t, c.steps, steps)
		})
	}
}

func TestACascadeReachingACycleIsRefusedNamingIt(t *testing.T) {
	cases := map[string]struct {
		records map[string]Meta
		roots   []string
		cycle   []string
	}{
		"two worktrees created from each other": {records: map[string]Meta{"a": from("b"), "b": from("a")}, roots: []string{"b"}, cycle: []string{"a", "b"}},
		"a worktree created from itself":        {records: map[string]Meta{"a": from("a"), "b": from("main")}, roots: []string{"b", "a"}, cycle: []string{"a"}},
		"a member above a branch under it":      {records: map[string]Meta{"a": from("b"), "b": from("a"), "c": from("a")}, roots: []string{"c", "a"}, cycle: []string{"a", "b"}},
		"a longer loop":                         {records: map[string]Meta{"a": from("c"), "b": from("a"), "c": from("b")}, roots: []string{"main", "c"}, cycle: []string{"a", "b", "c"}},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			steps, err := StackOf(StackOfParams{Worktrees: live("a", "b", "c"), Records: c.records}).Cascade(c.roots)

			var cycle *CycleError
			require.ErrorAs(t, err, &cycle)
			assert.Equal(t, c.cycle, cycle.Branches)
			assert.Nil(t, steps)
		})
	}
}

func TestACycleErrorNamesItsBranches(t *testing.T) {
	err := &CycleError{Branches: []string{"a", "b"}}

	assert.EqualError(t, err, "the parents of these worktrees form a cycle: a, b")
}
