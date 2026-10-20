package output

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
)

func TestAnOriginBadgeSaysOnlyADivergence(t *testing.T) {
	cases := map[string]struct {
		origin *worktree.Origin
		badge  string
	}{
		"ahead":      {origin: &worktree.Origin{Ahead: 2, State: worktree.Ahead}, badge: "origin ↑2"},
		"behind":     {origin: &worktree.Origin{Behind: 1, State: worktree.Behind}, badge: "origin ↓1"},
		"diverged":   {origin: &worktree.Origin{Ahead: 2, Behind: 1, State: worktree.Diverged}, badge: "origin ↑2 ↓1"},
		"up_to_date": {origin: &worktree.Origin{State: worktree.UpToDate}},
		"no_origin":  {},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.badge, OriginBadge(c.origin))
			assert.Equal(t, c.badge, plainStyles.Origin(c.origin))
		})
	}
}
