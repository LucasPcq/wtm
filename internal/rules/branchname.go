package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BranchNameProblem applies `git check-ref-format --branch` without asking git:
// a name git refuses used to be accepted by the wizard and fail only once the
// worktree was being created.
func BranchNameProblem(name string) error {
	if reason := branchNameReason(name); reason != "" {
		return flagValueError{text: fmt.Sprintf(domain.BranchNameInvalidFmt, name, reason)}
	}
	return nil
}

func branchNameReason(name string) string {
	switch {
	case name == "@", name == "HEAD":
		return domain.BranchNameReserved
	case strings.HasPrefix(name, "-"):
		return domain.BranchNameLeadingDash
	case strings.HasPrefix(name, "/"), strings.HasSuffix(name, "/"), strings.Contains(name, "//"):
		return domain.BranchNameBadSlash
	case strings.HasSuffix(name, "."), strings.Contains(name, ".."):
		return domain.BranchNameBadDot
	case strings.Contains(name, "@{"):
		return domain.BranchNameAtBrace
	case strings.ContainsFunc(name, forbiddenInRef):
		return domain.BranchNameBadChar
	}
	for _, component := range strings.Split(name, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return domain.BranchNameBadComponent
		}
	}
	return ""
}

func forbiddenInRef(r rune) bool {
	return r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r)
}
