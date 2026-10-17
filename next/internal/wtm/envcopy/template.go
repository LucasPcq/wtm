package envcopy

import "path/filepath"

var templateSuffixes = [...]string{".example", ".dist", ".sample", ".template", ".tmpl"}

func templateCandidates(params requestFile) []string {
	dir, file := params.Request.MainPath, params.File
	if file.Template != "" {
		return []string{filepath.Join(dir, file.Template)}
	}
	candidates := make([]string, 0, len(templateSuffixes))
	for _, suffix := range templateSuffixes {
		candidates = append(candidates, filepath.Join(dir, file.Target+suffix))
	}
	return candidates
}

type templateParams struct {
	Candidates []string
	Present    map[string]bool
}

func template(params templateParams) string {
	for _, candidate := range params.Candidates {
		if params.Present[candidate] {
			return candidate
		}
	}
	return ""
}
