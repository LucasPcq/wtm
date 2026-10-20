package output

import (
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// The glyphs of a count of commits: those a branch has past another, and those it lacks.
const (
	GlyphAhead  = "↑"
	GlyphBehind = "↓"
)

// OriginBadge is where a branch stands against origin as a badge reads it, `origin ↑a`, `origin ↓b` or both when they diverged; "" when it is up to date, not on origin or origin is nil.
func OriginBadge(origin *worktree.Origin) string {
	if origin == nil {
		return ""
	}

	ahead, behind := GlyphAhead+strconv.Itoa(origin.Ahead), GlyphBehind+strconv.Itoa(origin.Behind)
	switch origin.State {
	case worktree.Ahead:
		return "origin " + ahead
	case worktree.Behind:
		return "origin " + behind
	case worktree.Diverged:
		return "origin " + ahead + " " + behind
	case worktree.NoOrigin, worktree.UpToDate:
	}
	return ""
}

// Origin is OriginBadge coloured by how much it asks of the reader: muted ahead, a warning behind, danger diverged.
func (s *Styles) Origin(origin *worktree.Origin) string {
	badge := OriginBadge(origin)
	switch {
	case badge == "":
		return ""
	case origin.State == worktree.Behind:
		return s.Warning.Render(badge)
	case origin.State == worktree.Diverged:
		return s.Danger.Render(badge)
	}
	return s.Muted.Render(badge)
}
