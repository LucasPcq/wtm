package settings

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// Store reads config.toml as written and writes it whole: Saved fails with project.ErrConfigNotFound when there is none and a *project.InvalidConfigError when it does not parse or validate; Save refuses a file that is no longer the one it replaces with project.ErrConfigChanged, and one a newer wtm wrote with a *domain.SchemaNewer.
type Store interface {
	Saved() (project.Saved, error)
	Save(ctx context.Context, rewrite project.Rewrite) (domain.Written, error)
}
