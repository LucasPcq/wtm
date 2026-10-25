package cli

import (
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	textReady     = "watching for changes"
	ordinalNone   = "none"
	changedSep    = ", "
	exitCodeLabel = " (exit "
)

// drawLine draws the line received for a person: its worktree's or repository's change, nothing for an operation's progress or a type this wtm does not know. A stream of every repository names the repository of each worktree's change.
func drawLine(w io.Writer, received events.Received) {
	line := received.Line
	prefix := ""
	if received.Every && line.Repo != nil {
		prefix = filepath.Base(line.Repo.Root) + output.Dot
	}

	switch e := line.Event.(type) {
	case domain.Snapshot:
		output.UnchangedLine(w, worktrees(len(e.Worktrees))+output.Dot+root(line))
	case domain.Ready:
		output.UnchangedLine(w, textReady)
	case domain.WorktreeCreated:
		output.SuccessLine(w, prefix+"created "+e.Worktree.Branch+"  "+e.Worktree.Path)
	case domain.WorktreeUpdated:
		output.SuccessLine(w, prefix+"updated "+e.Worktree.Branch+"  "+changed(e))
	case domain.WorktreeChanged:
		output.SuccessLine(w, prefix+"changed "+e.Worktree.Branch+"  "+polled(e.Changed))
	case domain.WorktreeProvisioned:
		drawProvisioned(w, e, prefix)
	case domain.WorktreeRemoved:
		output.SuccessLine(w, prefix+"removed "+e.Worktree.Branch+"  "+e.Worktree.Path)
	case domain.RepoAdded:
		output.SuccessLine(w, "watching "+root(line))
	case domain.RepoRemoved:
		output.WarningLine(w, "no longer watching "+root(line))
	case domain.OperationStarted, domain.UnitFinished, domain.OperationFinished, domain.UnitStarted, domain.PhaseStarted, domain.PhaseFinished, domain.Output, nil:
	}
}

func drawProvisioned(w io.Writer, e domain.WorktreeProvisioned, prefix string) {
	if e.OK {
		output.SuccessLine(w, prefix+"provisioned "+e.Worktree.Branch)
		return
	}

	failed := prefix + "on_create failed for " + e.Worktree.Branch
	if e.Hook != "" {
		failed += "  " + e.Hook
	}
	if e.ExitCode != nil {
		failed += exitCodeLabel + strconv.Itoa(*e.ExitCode) + ")"
	}
	output.ErrorLine(w, failed)
}

func worktrees(count int) string {
	if count == 1 {
		return "1 worktree"
	}
	return strconv.Itoa(count) + " worktrees"
}

func root(line domain.Line) string {
	if line.Repo == nil {
		return ""
	}
	return line.Repo.Root
}

// polled is each field of the worktree a poll found changed.
func polled(fields []worktree.PolledField) string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, string(field))
	}
	return strings.Join(names, changedSep)
}

// changed is each field of the worktree an update changed, with its new value.
func changed(e domain.WorktreeUpdated) string {
	fields := make([]string, 0, len(e.Changed))
	for _, field := range e.Changed {
		fields = append(fields, string(field)+"="+value(e.Worktree, field))
	}
	return strings.Join(fields, changedSep)
}

func value(identity worktree.Identity, field worktree.IdentityField) string {
	switch field {
	case worktree.IdentityIsolation:
		return string(identity.Isolation)
	case worktree.IdentityParent:
		return identity.Parent
	case worktree.IdentityCreatedAt:
		return identity.CreatedAt
	case worktree.IdentityOrdinal:
		if identity.Ordinal == nil {
			return ordinalNone
		}
		return strconv.Itoa(*identity.Ordinal)
	}
	return ""
}
