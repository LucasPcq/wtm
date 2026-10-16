package branch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestABranchClashesWithALiveOneSharingADerivedName(t *testing.T) {
	cases := []struct {
		name   string
		branch string
		live   []string
		want   Clash
		found  bool
	}{
		{name: "a dot and a slash share the slug", branch: "feat.x", live: []string{"main", "feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "case is folded", branch: "Feat/X", live: []string{"feat/x"}, want: Clash{Branch: "feat/x", Name: "feat-x"}, found: true},
		{name: "an underscore survives the slug, not the host label", branch: "feat/a_b", live: []string{"feat/a-b"}, want: Clash{Branch: "feat/a-b", Name: "feat-a-b"}, found: true},
		{name: "the branch itself is no clash", branch: "feat/x", live: []string{"feat/x"}},
		{name: "distinct names", branch: "feat/y", live: []string{"main", "feat/x"}},
		{name: "a detached worktree has no name", branch: "feat/x", live: []string{""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clash, found := ClashWith(ClashParams{Branch: tc.branch, Live: tc.live})

			assert.Equal(t, tc.found, found)
			assert.Equal(t, tc.want, clash)
		})
	}
}
