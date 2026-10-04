package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ClassifyExtractStatus maps a `git status --porcelain` XY code to the extraction
// status used for display and apply strategy. It accepts the raw two-character
// field as well as a trimmed one. Untracked files ("??") are copied verbatim;
// renames and copies carry both their paths into the patch; deleted files (status
// "D" in either column) carry a deletion; everything else is a content
// modification. Renames are matched before deletions so that "RD" and "RM" stay
// renames.
func ClassifyExtractStatus(status string) domain.ExtractFileStatus {
	code := strings.TrimSpace(status)
	if code == domain.PorcelainUntracked {
		return domain.ExtractStatusUntracked
	}
	if strings.Contains(code, domain.PorcelainRename) || strings.Contains(code, domain.PorcelainCopy) {
		return domain.ExtractStatusRenamed
	}
	if strings.Contains(code, domain.PorcelainDeleted) {
		return domain.ExtractStatusDeleted
	}
	return domain.ExtractStatusModified
}

// SelectExtractFilesParams holds the enumerated candidates and the paths asked for.
type SelectExtractFilesParams struct {
	Available []domain.ExtractFile
	Paths     []string
}

// SelectExtractFiles resolves the requested paths against the uncommitted changes
// of the source worktree, erroring on any path that matches nothing. A path that
// names no file on its own is retried as a directory prefix, so `--files pkg/`
// takes every change below it — untracked directories are enumerated per file, so
// this is the only way to ask for one wholesale. Duplicates are dropped, and the
// result follows enumeration order within each requested path.
func SelectExtractFiles(params SelectExtractFilesParams) ([]domain.ExtractFile, error) {
	byPath := make(map[string]domain.ExtractFile, len(params.Available))
	for _, f := range params.Available {
		byPath[f.Path] = f
	}

	seen := make(map[string]struct{}, len(params.Paths))
	out := make([]domain.ExtractFile, 0, len(params.Paths))
	for _, p := range params.Paths {
		matches := matchExtractPath(params.Available, byPath, p)
		if len(matches) == 0 {
			return nil, fmt.Errorf("%w: %s is not an uncommitted change", domain.ErrNoFilesSelected, p)
		}
		for _, f := range matches {
			if _, dup := seen[f.Path]; dup {
				continue
			}
			seen[f.Path] = struct{}{}
			out = append(out, f)
		}
	}

	if len(out) == 0 {
		return nil, domain.ErrNoFilesSelected
	}
	return out, nil
}

// matchExtractPath resolves one requested path: an exact file first, then every
// change below it read as a directory.
func matchExtractPath(available []domain.ExtractFile, byPath map[string]domain.ExtractFile, path string) []domain.ExtractFile {
	if f, ok := byPath[path]; ok {
		return []domain.ExtractFile{f}
	}

	prefix := strings.TrimSuffix(path, "/") + "/"
	var matches []domain.ExtractFile
	for _, f := range available {
		if strings.HasPrefix(f.Path, prefix) {
			matches = append(matches, f)
		}
	}
	return matches
}

// ExtractStatusLabel returns the short, human-readable tag shown next to a file
// in the interactive selector. The labels share the same width so the file names
// line up.
func ExtractStatusLabel(status domain.ExtractFileStatus) string {
	switch status {
	case domain.ExtractStatusUntracked:
		return "new"
	case domain.ExtractStatusDeleted:
		return "del"
	case domain.ExtractStatusRenamed:
		return "ren"
	default:
		return "mod"
	}
}

// ExtractStatusTone colours a file's tag in the selector: a new file, a
// deletion and a modification read apart at a glance.
func ExtractStatusTone(status domain.ExtractFileStatus) domain.Tone {
	switch status {
	case domain.ExtractStatusUntracked:
		return domain.ToneSuccess
	case domain.ExtractStatusDeleted:
		return domain.ToneDanger
	case domain.ExtractStatusRenamed:
		return domain.ToneNeutral
	default:
		return domain.ToneWarning
	}
}

// ExtractFileLabel names both paths of a rename, since both take part in the
// extraction.
func ExtractFileLabel(file domain.ExtractFile) string {
	if file.OrigPath == "" {
		return file.Path
	}
	return file.OrigPath + " → " + file.Path
}

func FileCount(count int) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}
