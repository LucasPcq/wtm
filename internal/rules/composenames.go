package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ComposeIsolatedNameParams struct {
	// Name is the absolute name the file pins, quotes stripped.
	Name string
	// Project names the repository, the same value COMPOSE_PROJECT_NAME is
	// built from. It becomes the template's default, so a checkout wtm is not
	// driving keeps the name the file used to pin.
	Project string
}

// ComposeIsolatedName is what an absolute name becomes: the compose project
// fronts it. The result is a quoted YAML scalar, ready to splice in place of
// the token the scan recorded.
func ComposeIsolatedName(params ComposeIsolatedNameParams) string {
	project := WorktreeSlug(params.Project)

	// The file spells the prefix as the repository is actually named, which the
	// slug may have rewritten; trimming both keeps a suffix out of a name that
	// already fronts itself with the project under either spelling.
	suffix := params.Name
	for _, prefix := range []string{project + "-", params.Project + "-"} {
		if trimmed := strings.TrimPrefix(suffix, prefix); trimmed != suffix {
			suffix = trimmed
			break
		}
	}

	if suffix == project || suffix == params.Project || suffix == "" {
		return strconv.Quote(fmt.Sprintf(domain.ComposeIsolatedBareNameFmt, project))
	}
	return strconv.Quote(fmt.Sprintf(domain.ComposeIsolatedNameFmt, project, suffix))
}

// ComposeNamePlan is what a run of the detection decided about the names a
// compose file pins, before anything is written. Withheld is what keeps the
// feature honest: a name wtm does not rewrite still collides, and saying so
// beats leaving the user to discover it on the second `run up`.
type ComposeNamePlan struct {
	Patches  map[string][]domain.ComposeAbsoluteName
	Withheld []domain.ComposeAbsoluteName
}

type PlanComposeNamesParams struct {
	Scans map[string]domain.ComposeScan
	Files []string
	// Patch says the rewrite was authorized — by the wizard step or by the
	// flag. Without it an absolute name is withheld rather than rewritten.
	Patch bool
}

// PlanComposeNames writes nothing and reads no disk.
func PlanComposeNames(params PlanComposeNamesParams) ComposeNamePlan {
	plan := ComposeNamePlan{Patches: map[string][]domain.ComposeAbsoluteName{}}

	for _, file := range params.Files {
		scan, found := params.Scans[file]
		if !found || scan.Err != "" {
			continue
		}

		for _, name := range scan.Names {
			switch {
			case name.Status == domain.ComposeNameTemplated:
			case name.Status == domain.ComposeNameAbsolute && params.Patch:
				plan.Patches[file] = append(plan.Patches[file], name)
			default:
				plan.Withheld = append(plan.Withheld, name)
			}
		}
	}

	return plan
}

// ComposeNamePatchLines describes the rewrites a patch would perform, one line
// each. Both surfaces read it: the wizard step asking for authorization and the
// recap reporting what was done.
func ComposeNamePatchLines(byFile map[string][]domain.ComposeAbsoluteName) []string {
	var lines []string
	for _, file := range SortedComposeFiles(byFile) {
		for _, n := range byFile[file] {
			lines = append(lines, fmt.Sprintf(domain.ComposeNameLineFmt, file, n.Kind, n.Owner, n.Token, n.Replacement))
		}
	}
	return lines
}

// ComposeNameWithheldLine names an absolute name left as it is, with the reason
// when there is one — a name withheld only for want of authorization has none.
func ComposeNameWithheldLine(n domain.ComposeAbsoluteName) string {
	detail := n.Reason
	if detail == "" {
		detail = fmt.Sprintf(domain.ComposeNameCollidesFmt, n.Name)
	}
	return fmt.Sprintf(domain.ComposeNameWithheldFmt, n.File, n.Kind, n.Owner, detail)
}

// ComposeNamesRenameAVolume reports whether a plan renames a volume. A
// container is recreated from nothing, but a volume's data stays behind under
// the name it used to carry, and a reader not told that reads the empty volume
// as data loss.
func ComposeNamesRenameAVolume(byFile map[string][]domain.ComposeAbsoluteName) bool {
	for _, names := range byFile {
		for _, n := range names {
			if n.Kind == domain.ComposeNameVolume {
				return true
			}
		}
	}
	return false
}

// ComposeNameEdits reduces the names to the splice the patcher performs.
func ComposeNameEdits(names []domain.ComposeAbsoluteName) []domain.ComposeEdit {
	edits := make([]domain.ComposeEdit, 0, len(names))
	for _, n := range names {
		if n.Replacement == "" {
			continue
		}
		edits = append(edits, domain.ComposeEdit{
			File: n.File, Line: n.Line, Column: n.Column,
			Token: n.Token, Replacement: n.Replacement,
		})
	}
	return edits
}

// ComposeNamesWithoutFiles withdraws the files whose recorded positions can no
// longer be trusted. A stale scan would splice a name over whatever took its
// place, so such a file contributes nothing rather than being rewritten blind.
func ComposeNamesWithoutFiles(byFile map[string][]domain.ComposeAbsoluteName, files []string) map[string][]domain.ComposeAbsoluteName {
	if len(files) == 0 {
		return byFile
	}

	excluded := make(map[string]bool, len(files))
	for _, f := range files {
		excluded[f] = true
	}

	kept := map[string][]domain.ComposeAbsoluteName{}
	for file, names := range byFile {
		if !excluded[file] {
			kept[file] = names
		}
	}
	return kept
}
