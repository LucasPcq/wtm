package rules

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HookLabel names the hook a beat is about, with the directory it runs in when
// that is not the worktree root.
func HookLabel(beat domain.HookBeat) string {
	if beat.Cwd == "" {
		return beat.Cmd
	}
	return fmt.Sprintf(domain.HookCwdLabelFmt, beat.Cmd, beat.Cwd)
}

// HookResultLabel is the same name plus how long the hook took — what a surface
// leaves behind once the stream it replaced is gone.
func HookResultLabel(beat domain.HookBeat) string {
	return fmt.Sprintf(domain.HookResultLabelFmt, HookLabel(beat), HookDuration(beat.Duration))
}

// HookBeatLine is a beat as one plain line, glyph included, for a surface with
// no styling of its own to give it.
func HookBeatLine(beat domain.HookBeat) string {
	if beat.Started {
		return domain.HookGlyphStart + " " + HookLabel(beat)
	}
	if beat.Err != "" {
		return domain.HookGlyphFailed + " " + HookResultLabel(beat)
	}
	return domain.HookGlyphDone + " " + HookResultLabel(beat)
}

func HookDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf(domain.HookDurationMsFmt, d.Milliseconds())
	}
	return fmt.Sprintf(domain.HookDurationSecFmt, d.Seconds())
}

type HooksLogPathParams struct {
	StateDir string
	// Phase is domain.HookOnCreate or domain.HookOnClean, and Branch the worktree
	// the phase ran on — empty for one that ran on several (prune).
	Phase  domain.HookEvent
	Branch string
}

// HooksLogPath is where a hook phase keeps its whole output, whatever the
// surface running it chose to show. Phase and branch are in the name because one
// file for the whole project would have `clean` overwrite what `create` just
// wrote, and the line pointing a reader at it would point at another run.
func HooksLogPath(params HooksLogPathParams) string {
	if params.StateDir == "" {
		return ""
	}
	name := string(params.Phase)
	if params.Branch != "" {
		name += domain.HooksLogNameSeparator + EncodeBranchSegment(params.Branch)
	}
	return filepath.Join(params.StateDir, domain.HooksLogDirName, name+domain.HooksLogFileExt)
}
