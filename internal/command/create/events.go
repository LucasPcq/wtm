package create

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/service/hooks"
)

func Created(_ Facts, m kernel.Committed[provision.Detail]) []domain.Event {
	return []domain.Event{{Type: domain.EventWorktreeCreated, Worktree: identityOf(m.Subject, m.Detail)}}
}

func Updated(_ Facts, m kernel.PhaseDone[provision.Detail]) []domain.Event {
	if m.Phase != provision.PhaseOrdinal || !m.Detail.Allocated {
		return nil
	}
	return []domain.Event{{Type: domain.EventWorktreeUpdated, Worktree: identityOf(m.Subject, m.Detail), Changed: []domain.IdentityField{domain.IdentityOrdinal}}}
}

func Provisioned(_ Facts, item kernel.Item[provision.Detail]) []domain.Event {
	if item.Detail.Path == "" {
		return nil
	}
	ok := item.Status == kernel.StatusDone
	failure := hookFailure(item)
	return []domain.Event{{Type: domain.EventWorktreeProvisioned, Worktree: identityOf(item.Subject, item.Detail), OK: &ok, Hook: failure.Cmd, ExitCode: failure.ExitCode}}
}

func hookFailure(item kernel.Item[provision.Detail]) hooks.Failure {
	var failure hooks.Failure
	if item.Error == nil || !errors.As(item.Error, &failure) {
		return hooks.Failure{}
	}
	return failure
}

func identityOf(branch string, detail provision.Detail) *domain.WorktreeIdentity {
	return &domain.WorktreeIdentity{
		Branch:    branch,
		Path:      detail.Path,
		Parent:    detail.Metadata.SourceBranch,
		Ordinal:   detail.Ordinal,
		Isolation: detail.Metadata.Isolation,
		CreatedAt: detail.Metadata.CreatedAt,
	}
}
