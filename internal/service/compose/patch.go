package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type VerifyAllParams struct {
	ProjectDir  string
	ByFile      map[string][]domain.ComposePortBinding
	NamesByFile map[string][]domain.ComposeAbsoluteName
}

// VerifyAll reports, per file, why its recorded positions can no longer be
// trusted — the file changed since it was scanned, or it became unreadable. It
// writes nothing, so the caller can withdraw those files from the plan before
// anything is committed to disk rather than discovering it half way through.
func VerifyAll(params VerifyAllParams) map[string]string {
	failures := map[string]string{}
	for _, file := range editedFiles(params.ByFile, params.NamesByFile) {
		if _, err := renderPatched(renderPatchedParams{
			ProjectDir: params.ProjectDir,
			File:       file,
			Bindings:   params.ByFile[file],
			Names:      params.NamesByFile[file],
		}); err != nil {
			failures[file] = err.Error()
		}
	}
	return failures
}

// editedFiles is every file either family of edits touches, in one stable
// order: a file carrying only a name rewrite is patched like any other.
func editedFiles(byFile map[string][]domain.ComposePortBinding, namesByFile map[string][]domain.ComposeAbsoluteName) []string {
	merged := make(map[string]struct{}, len(byFile)+len(namesByFile))
	for file := range byFile {
		merged[file] = struct{}{}
	}
	for file := range namesByFile {
		merged[file] = struct{}{}
	}
	return rules.SortedComposeFiles(merged)
}

type PatchAllParams struct {
	ProjectDir  string
	ByFile      map[string][]domain.ComposePortBinding
	NamesByFile map[string][]domain.ComposeAbsoluteName
}

// PatchAll rewrites every file's host ports. It renders all of them first and
// only then writes, so a position that no longer matches aborts before any file
// on disk has been touched.
func PatchAll(params PatchAllParams) error {
	files := editedFiles(params.ByFile, params.NamesByFile)

	rendered := make([]patchedFile, 0, len(files))
	for _, file := range files {
		out, err := renderPatched(renderPatchedParams{
			ProjectDir: params.ProjectDir,
			File:       file,
			Bindings:   params.ByFile[file],
			Names:      params.NamesByFile[file],
		})
		if err != nil {
			return err
		}
		if out.changed {
			rendered = append(rendered, out)
		}
	}

	var written []string
	for _, out := range rendered {
		if err := writeFile(out); err != nil {
			if len(written) == 0 {
				return err
			}
			return fmt.Errorf("%w — %s", err, fmt.Sprintf(domain.ComposePatchPartialFmt, strings.Join(written, ", ")))
		}
		written = append(written, out.file)
	}
	return nil
}

type patchedFile struct {
	file    string
	path    string
	content string
	mode    os.FileMode
	changed bool
}

type renderPatchedParams struct {
	ProjectDir string
	File       string
	Bindings   []domain.ComposePortBinding
	Names      []domain.ComposeAbsoluteName
}

func renderPatched(params renderPatchedParams) (patchedFile, error) {
	projectDir, file, bindings := params.ProjectDir, params.File, params.Bindings
	path := filepath.Join(projectDir, file)

	// The rewrite lands on what the path points at: replacing a symlink with a
	// regular file would fork the compose into two copies drifting apart.
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}

	info, err := os.Stat(path)
	if err != nil {
		return patchedFile{}, fmt.Errorf(domain.ComposeReadFileFmt, file, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return patchedFile{}, fmt.Errorf(domain.ComposeReadFileFmt, file, err)
	}

	patched, err := rules.ApplyComposeEdits(rules.ApplyComposeEditsParams{
		Content: string(content),
		Edits:   append(rules.ComposePortEdits(bindings), rules.ComposeNameEdits(params.Names)...),
	})
	if err != nil {
		return patchedFile{}, err
	}

	return patchedFile{
		file:    file,
		path:    path,
		content: patched,
		mode:    info.Mode().Perm(),
		changed: patched != string(content),
	}, nil
}

// writeFile replaces the file through a temporary neighbour: a compose file
// truncated by an interrupted write would take the project's stack down with it.
func writeFile(out patchedFile) error {
	temp, err := os.CreateTemp(filepath.Dir(out.path), filepath.Base(out.path)+".wtm-*")
	if err != nil {
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	tempPath := temp.Name()

	if _, err := temp.WriteString(out.content); err != nil {
		temp.Close()
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := os.Chmod(tempPath, out.mode); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	if err := os.Rename(tempPath, out.path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf(domain.ComposeWriteFileFmt, out.file, err)
	}
	return nil
}
