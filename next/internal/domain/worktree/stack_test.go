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
		"linear_stack": {
			worktrees: live("a", "b", "c"),
			records:   map[string]Meta{"a": from("main"), "b": from("a"), "c": from("b")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}, "a": {"b"}, "b": {"c"}, "c": nil},
			parents:   map[string]string{"a": "main", "b": "a", "c": "b"},
		},
		"fan_out_ordered": {
			worktrees: live("late", "early", "tie-b", "tie-a"),
			records:   map[string]Meta{"late": at("main", "2026-03-01T00:00:00Z"), "early": at("main", "2026-01-01T00:00:00+02:00"), "tie-b": at("main", "2026-02-01T00:00:00Z"), "tie-a": at("main", "2026-02-01T00:00:00Z")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"early", "tie-a", "tie-b", "late"}},
		},
		"several_roots_main_first": {
			worktrees: live("newer", "older", "unrecorded"),
			records:   map[string]Meta{"newer": at("", "2026-02-01T00:00:00Z"), "older": at("", "2026-01-01T00:00:00Z")},
			roots:     []string{"main", "unrecorded", "older", "newer"},
			children:  map[string][]string{"main": nil},
			parents:   map[string]string{"newer": "", "older": "", "unrecorded": ""},
		},
		"main_always_root": {
			worktrees: live("a"),
			records:   map[string]Meta{"main": from("a"), "a": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
			parents:   map[string]string{"main": "", "a": "main"},
		},
		"base_branch_keeps_parent": {
			worktrees: live("develop", "feat"),
			records:   map[string]Meta{"develop": from("main"), "feat": from("develop")},
			base:      "develop",
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"develop"}, "develop": {"feat"}},
			parents:   map[string]string{"develop": "main"},
		},
		"detached_main_virtual_root": {
			worktrees: []Worktree{{Path: "/repo", Main: true, State: Detached{}}, {Path: "/trees/a", State: OnBranch{Branch: "a"}}},
			records:   map[string]Meta{"a": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
			parents:   map[string]string{"a": "main"},
		},
		"deleted_parent_virtual_root": {
			worktrees: live("a", "b"),
			records:   map[string]Meta{"a": from("gone"), "b": from("gone"), "gone": from("main")},
			roots:     []string{"main", "gone"},
			children:  map[string][]string{"gone": {"a", "b"}, "main": nil},
			parents:   map[string]string{"a": "gone", "gone": ""},
		},
		"origin_branch_virtual_root": {
			worktrees: live("a", "b"),
			records:   map[string]Meta{"a": from("origin/x"), "b": from("unmanaged")},
			roots:     []string{"main", "origin/x", "unmanaged"},
			children:  map[string][]string{"origin/x": {"a"}, "unmanaged": {"b"}},
		},
		"cycle_members_promoted": {
			worktrees: live("a", "b", "c"),
			records:   map[string]Meta{"a": at("b", "2026-01-01T00:00:00Z"), "b": at("a", "2026-02-01T00:00:00Z"), "c": from("a")},
			roots:     []string{"main", "a", "b"},
			children:  map[string][]string{"a": {"c"}, "b": nil},
			parents:   map[string]string{"a": "", "b": "", "c": "a"},
		},
		"self_source_root": {
			worktrees: live("a"),
			records:   map[string]Meta{"a": from("a")},
			roots:     []string{"main", "a"},
			children:  map[string][]string{"a": nil},
			parents:   map[string]string{"a": ""},
		},
		"detached_not_node_rebasing_is": {
			worktrees: append(live(), Worktree{Path: "/trees/detached", State: Detached{}}, Worktree{Path: "/trees/a", State: Rebasing{Branch: "a"}}),
			records:   map[string]Meta{"a": from("main"), "": from("a")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}, "a": nil},
		},
		"record_without_worktree": {
			worktrees: live("a"),
			records:   map[string]Meta{"a": from("main"), "removed": from("main")},
			roots:     []string{"main"},
			children:  map[string][]string{"main": {"a"}},
		},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			stack := StackOf(c.worktrees, c.records, c.base)

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
	stack := StackOf(
		live("develop", "a", "a1", "b", "d", "e", "x", "y", "z"),
		map[string]Meta{
			"develop": from("main"), "a": at("main", "2026-01-01T00:00:00Z"), "a1": from("a"), "b": at("main", "2026-02-01T00:00:00Z"),
			"d": from("develop"), "e": from("gone"), "x": from("y"), "y": from("x"), "z": from("x"),
		},
		"develop",
	)
	cases := map[string]struct {
		roots []string
		steps []string
	}{
		"level_by_level_main_skipped":       {roots: []string{"main"}, steps: []string{"a", "b", "d", "a1"}},
		"base_branch_skipped_children_kept": {roots: []string{"develop"}, steps: []string{"d"}},
		"worktree_root_is_step":             {roots: []string{"a"}, steps: []string{"a", "a1"}},
		"virtual_root_skipped":              {roots: []string{"gone"}, steps: []string{"e"}},
		"each_branch_once":                  {roots: []string{"main", "a", "a1"}, steps: []string{"a", "b", "d", "a1"}},
		"parent_before_child":               {roots: []string{"a1", "a"}, steps: []string{"a", "a1"}},
		"child_beside_ancestor":             {roots: []string{"main", "a1"}, steps: []string{"a", "b", "d", "a1"}},
		"unknown_branch":                    {roots: []string{"nope"}},
		"branch_under_cycle":                {roots: []string{"z"}, steps: []string{"z"}},
		"cycle_out_of_reach":                {roots: []string{"main", "develop", "gone"}, steps: []string{"a", "b", "e", "d", "a1"}},
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
		"mutual_pair":         {records: map[string]Meta{"a": from("b"), "b": from("a")}, roots: []string{"b"}, cycle: []string{"a", "b"}},
		"self_source":         {records: map[string]Meta{"a": from("a"), "b": from("main")}, roots: []string{"b", "a"}, cycle: []string{"a"}},
		"member_above_branch": {records: map[string]Meta{"a": from("b"), "b": from("a"), "c": from("a")}, roots: []string{"c", "a"}, cycle: []string{"a", "b"}},
		"longer_loop":         {records: map[string]Meta{"a": from("c"), "b": from("a"), "c": from("b")}, roots: []string{"main", "c"}, cycle: []string{"a", "b", "c"}},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			steps, err := StackOf(live("a", "b", "c"), c.records, "").Cascade(c.roots)

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

func TestAStackSaysWhichRootsAreVirtualAndWhichBranchesLoop(t *testing.T) {
	stack := StackOf(live("a", "b", "x", "y"), map[string]Meta{"a": from("gone"), "b": from("a"), "x": from("y"), "y": from("x")}, "")

	assert.True(t, stack.Virtual("gone"))
	assert.False(t, stack.Virtual("a"))
	assert.False(t, stack.Virtual("main"))
	assert.True(t, stack.InCycle("x"))
	assert.True(t, stack.InCycle("y"))
	assert.False(t, stack.InCycle("b"))
}

func TestWithinSaysWhichMovedBranchATargetSitsUnder(t *testing.T) {
	worktrees := append(live("a", "b", "c", "x", "p", "q"), Worktree{Path: "/trees/d", State: Detached{}})
	records := map[string]Meta{"a": from("main"), "b": from("a"), "c": from("b"), "x": from("main"), "p": from("q"), "q": from("p")}
	stack := StackOf(worktrees, records, "main")
	cases := map[string]struct {
		branch string
		moving []string
		under  string
	}{
		"itself":          {branch: "b", moving: []string{"b"}, under: "b"},
		"grandchild":      {branch: "c", moving: []string{"a"}, under: "a"},
		"one_of_several":  {branch: "c", moving: []string{"x", "b"}, under: "b"},
		"sibling":         {branch: "x", moving: []string{"a"}},
		"above":           {branch: "main", moving: []string{"a"}},
		"no_worktree":     {branch: "topic", moving: []string{"a"}},
		"cycle_elsewhere": {branch: "p", moving: []string{"a"}},
		"into_its_cycle":  {branch: "q", moving: []string{"p"}, under: "p"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.under, stack.Within(c.branch, c.moving))
		})
	}
}

func TestWithinStopsAtTheBaseBranch(t *testing.T) {
	stack := StackOf(live("develop", "feat/x"), map[string]Meta{"develop": from("feat/x"), "feat/x": from("main")}, "develop")

	assert.Empty(t, stack.Within("develop", []string{"feat/x"}))
}

func TestAncestorsClimbToTheRoot(t *testing.T) {
	stack := StackOf(live("a", "b", "c"), map[string]Meta{"a": from("main"), "b": from("a"), "c": from("b")}, "main")
	assert.Equal(t, []string{"b", "a", "main"}, stack.Ancestors("c"))
	assert.Empty(t, stack.Ancestors("main"))
}
