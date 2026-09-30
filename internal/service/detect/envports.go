package detect

import (
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type EnvPortCandidatesParams struct {
	// ProjectDir is the main worktree, whose .env files hold the base ports —
	// any other worktree's copy already carries an offset.
	ProjectDir string
	Files      []domain.EnvFile
	Bases      map[domain.PortRef]int
	Existing   []domain.EnvPortLink
	JobsByDir  map[string]string
}

// EnvLines reads the project's configured env value files once, so a surface
// can match candidates against a config that is still being composed without
// touching the filesystem again. A file that cannot be read is skipped: the
// detection is a convenience, and refusing `wtm run init` over an unreadable
// .env would be out of proportion.
func EnvLines(params EnvPortCandidatesParams) map[string][]domain.EnvLine {
	lines := map[string][]domain.EnvLine{}
	for _, file := range params.Files {
		data, err := os.ReadFile(filepath.Join(params.ProjectDir, file.Target))
		if err != nil {
			continue
		}
		lines[file.Target] = rules.ParseEnv(string(data))
	}
	return lines
}

// EnvPortCandidates reads the project's configured env value files and reports
// the keys whose value holds a declared base port. A file that cannot be read is
// skipped: the detection is a convenience, and refusing `wtm run init` over an
// unreadable .env would be out of proportion.
func EnvPortCandidates(params EnvPortCandidatesParams) []domain.EnvPortLink {
	if len(params.Bases) == 0 {
		return nil
	}

	return rules.EnvPortCandidates(rules.EnvPortCandidatesParams{
		Lines:     EnvLines(params),
		Bases:     params.Bases,
		Existing:  params.Existing,
		JobsByDir: params.JobsByDir,
	})
}
