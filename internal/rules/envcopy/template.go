package envcopy

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateCandidates are in order: the one detection pinned, else each conventional spelling.
func TemplateCandidates(dir string, file domain.EnvFile) []string {
	if file.Template != "" {
		return []string{filepath.Join(dir, file.Template)}
	}
	candidates := make([]string, 0, len(domain.EnvTemplateSuffixes))
	for _, suffix := range domain.EnvTemplateSuffixes {
		candidates = append(candidates, filepath.Join(dir, file.Target+suffix))
	}
	return candidates
}

func Template(candidates []string, present map[string]bool) string {
	for _, candidate := range candidates {
		if present[candidate] {
			return candidate
		}
	}
	return ""
}
