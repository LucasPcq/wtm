package output

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// WriteEventJSONLine is one JSON Lines record, written as it was received:
// compact, so a consumer can split the stream on newlines, and never decoded
// on the way, so a field this build does not know still reaches it.
func WriteEventJSONLine(w io.Writer, raw json.RawMessage) error {
	_, err := fmt.Fprintf(w, "%s\n", raw)
	return err
}

// WriteEventLine renders what a person watching needs: a type it does not know
// is left out, as the contract asks of every consumer.
func WriteEventLine(w io.Writer, event domain.Event) error {
	writeEventLine(eventLineParams{W: w, Event: event})
	return nil
}

// WriteGlobalEventLine is WriteEventLine for a stream over several
// repositories: a worktree line names the repository it belongs to.
func WriteGlobalEventLine(w io.Writer, event domain.Event) error {
	prefix := ""
	if aboutAWorktree(event.Type) && event.Repo != nil {
		prefix = fmt.Sprintf(domain.EventRepoPrefixFmt, filepath.Base(event.Repo.Root))
	}
	writeEventLine(eventLineParams{W: w, Event: event, Prefix: prefix})
	return nil
}

func aboutAWorktree(eventType domain.EventType) bool {
	return strings.HasPrefix(string(eventType), domain.EventWorktreePrefix) || strings.HasPrefix(string(eventType), domain.EventJobPrefix)
}

type eventLineParams struct {
	W      io.Writer
	Event  domain.Event
	Prefix string
}

func writeEventLine(params eventLineParams) {
	w, event, prefix := params.W, params.Event, params.Prefix
	identity := event.Worktree
	if identity == nil {
		identity = &domain.WorktreeIdentity{}
	}
	switch event.Type {
	case domain.EventSnapshot:
		Unchanged(w, fmt.Sprintf(domain.EventSnapshotFmt, rules.WorktreeCountLabel(len(event.Worktrees)), repoRoot(event.Repo)))
	case domain.EventReady:
		Unchanged(w, domain.EventReadyMessage)
	case domain.EventWorktreeCreated:
		Success(w, prefix+fmt.Sprintf(domain.EventCreatedFmt, identity.Branch, identity.Path))
	case domain.EventWorktreeProvisioned:
		if hookPassed(event) {
			Success(w, prefix+fmt.Sprintf(domain.EventProvisionedFmt, identity.Branch))
			return
		}
		Error(w, prefix+fmt.Sprintf(domain.EventProvisionFailedFmt, identity.Branch)+hookDetail(event))
	case domain.EventWorktreeDeprovisioned:
		if hookPassed(event) {
			return
		}
		Error(w, prefix+fmt.Sprintf(domain.EventDeprovisionFailedFmt, identity.Branch)+hookDetail(event))
	case domain.EventRepoAdded:
		Update(w, fmt.Sprintf(domain.EventRepoAddedFmt, repoRoot(event.Repo)))
	case domain.EventRepoRemoved:
		Update(w, fmt.Sprintf(domain.EventRepoRemovedFmt, repoRoot(event.Repo)))
	case domain.EventWorktreeRemoved:
		Success(w, prefix+fmt.Sprintf(domain.EventRemovedFmt, identity.Branch))
	case domain.EventWorktreeRelocated:
		Update(w, prefix+fmt.Sprintf(domain.EventRelocatedFmt, identity.Branch, event.FromPath, identity.Path))
	case domain.EventWorktreeReparented:
		Update(w, prefix+fmt.Sprintf(domain.EventReparentedFmt, identity.Branch, event.FromParent, identity.Parent))
	case domain.EventWorktreeUpdated:
		Update(w, prefix+fmt.Sprintf(domain.EventUpdatedFmt, identity.Branch, changedFields(changedFieldsParams{Identity: *identity, Changed: event.Changed})))
	case domain.EventJobStarted, domain.EventJobCrashed, domain.EventJobExited, domain.EventJobStopped:
		writeJobEventLine(jobEventLineParams{W: w, Event: event, Prefix: prefix, Branch: identity.Branch})
	}
}

type jobEventLineParams struct {
	W      io.Writer
	Event  domain.Event
	Prefix string
	Branch string
}

func writeJobEventLine(params jobEventLineParams) {
	job := params.Event.Job
	if job == nil {
		return
	}
	w, prefix, held := params.W, params.Prefix, heldBy(params.Event.HeldBy)
	switch params.Event.Type {
	case domain.EventJobStarted:
		url := ""
		if job.URL != "" {
			url = fmt.Sprintf(domain.EventJobURLFmt, job.URL)
		}
		Success(w, prefix+fmt.Sprintf(domain.EventJobStartedFmt, job.Name, params.Branch)+held+url)
	case domain.EventJobExited:
		Success(w, prefix+fmt.Sprintf(domain.EventJobExitedFmt, job.Name, params.Branch)+held)
	case domain.EventJobStopped:
		Update(w, prefix+fmt.Sprintf(domain.EventJobStoppedFmt, job.Name, params.Branch)+held)
	case domain.EventJobCrashed:
		line := prefix + fmt.Sprintf(domain.EventJobCrashedFmt, job.Name, params.Branch) + held
		if params.Event.ExitCode != nil {
			line += fmt.Sprintf(domain.EventExitCodeFmt, *params.Event.ExitCode)
		}
		Error(w, strings.Join(append([]string{line}, params.Event.LastLines...), "\n"))
	}
}

func heldBy(refs []domain.WorktreeRef) string {
	if len(refs) == 0 {
		return ""
	}
	branches := make([]string, 0, len(refs))
	for _, ref := range refs {
		branches = append(branches, ref.Branch)
	}
	return fmt.Sprintf(domain.EventJobHeldByFmt, strings.Join(branches, domain.EventFieldSep))
}

func hookPassed(event domain.Event) bool {
	return event.OK == nil || *event.OK
}

func hookDetail(event domain.Event) string {
	if event.Hook == "" {
		return ""
	}
	detail := fmt.Sprintf(domain.EventHookFmt, event.Hook)
	if event.ExitCode != nil {
		detail += fmt.Sprintf(domain.EventExitCodeFmt, *event.ExitCode)
	}
	return detail
}

func repoRoot(repo *domain.EventRepo) string {
	if repo == nil {
		return ""
	}
	return repo.Root
}

type changedFieldsParams struct {
	Identity domain.WorktreeIdentity
	Changed  []domain.IdentityField
}

func changedFields(params changedFieldsParams) string {
	fields := make([]string, 0, len(params.Changed))
	for _, field := range params.Changed {
		fields = append(fields, fmt.Sprintf(domain.EventFieldFmt, field, fieldValue(fieldValueParams{Identity: params.Identity, Field: field})))
	}
	return strings.Join(fields, domain.EventFieldSep)
}

type fieldValueParams struct {
	Identity domain.WorktreeIdentity
	Field    domain.IdentityField
}

func fieldValue(params fieldValueParams) string {
	switch params.Field {
	case domain.IdentityIsolation:
		return string(params.Identity.Isolation)
	case domain.IdentityParent:
		return params.Identity.Parent
	case domain.IdentityCreatedAt:
		return params.Identity.CreatedAt
	case domain.IdentityOrdinal:
		if params.Identity.Ordinal == nil {
			return domain.EventOrdinalNone
		}
		return strconv.Itoa(*params.Identity.Ordinal)
	default:
		return ""
	}
}
