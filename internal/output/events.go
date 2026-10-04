package output

import (
	"encoding/json"
	"fmt"
	"io"
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
		Success(w, fmt.Sprintf(domain.EventCreatedFmt, identity.Branch, identity.Path))
	case domain.EventWorktreeRemoved:
		Success(w, fmt.Sprintf(domain.EventRemovedFmt, identity.Branch))
	case domain.EventWorktreeRelocated:
		Update(w, fmt.Sprintf(domain.EventRelocatedFmt, identity.Branch, event.FromPath, identity.Path))
	case domain.EventWorktreeReparented:
		Update(w, fmt.Sprintf(domain.EventReparentedFmt, identity.Branch, event.FromParent, identity.Parent))
	case domain.EventWorktreeProvisioned:
		if hookPassed(event) {
			Success(w, fmt.Sprintf(domain.EventProvisionedFmt, identity.Branch))
			return nil
		}
		Error(w, fmt.Sprintf(domain.EventProvisionFailedFmt, identity.Branch)+hookDetail(event))
	case domain.EventWorktreeDeprovisioned:
		if hookPassed(event) {
			return nil
		}
		Error(w, fmt.Sprintf(domain.EventDeprovisionFailedFmt, identity.Branch)+hookDetail(event))
	case domain.EventRepoAdded:
		Update(w, fmt.Sprintf(domain.EventRepoAddedFmt, repoRoot(event.Repo)))
	case domain.EventRepoRemoved:
		Update(w, fmt.Sprintf(domain.EventRepoRemovedFmt, repoRoot(event.Repo)))
	case domain.EventWorktreeUpdated:
		Update(w, fmt.Sprintf(domain.EventUpdatedFmt, identity.Branch, changedFields(changedFieldsParams{Identity: *identity, Changed: event.Changed})))
	}
	return nil
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
