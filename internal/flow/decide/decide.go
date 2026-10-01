// Package decide holds the branch and env decisions the create-like flows share.
package decide

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func BranchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}

// MemoizedTarget caches branch classification for one run: it costs ~3 git
// subprocesses, and a step, a recap and a warning all classify the same name.
func MemoizedTarget(projectDir string) func(string) domain.BranchTarget {
	cache := make(map[string]domain.BranchTarget)
	return func(name string) domain.BranchTarget {
		if name == "" {
			return domain.BranchTarget{}
		}
		if target, ok := cache[name]; ok {
			return target
		}
		target := branch.Target(branch.BranchParams{ProjectDir: projectDir, Branch: name})
		cache[name] = target
		return target
	}
}

type SourceUpdatePrompt struct {
	Branch      string
	Show        bool
	Title       string
	Description string
	Warning     string
	// AbortOnDecline distinguishes a diverged branch (declining cancels) from a
	// fast-forward offer (declining proceeds as-is).
	AbortOnDecline bool
	SkipReason     string
}

type SourceUpdateParams struct {
	ProjectDir string
	Target     func(string) domain.BranchTarget
	Branch     string
	Source     string
	// Many is a source several new worktrees start from, which the prose says.
	Many bool
}

// SourceUpdate classifies the divergence from origin of whichever branch the
// worktree starts from: the target branch when it already exists locally (its
// commits are what gets checked out), the source branch otherwise.
func SourceUpdate(params SourceUpdateParams) SourceUpdatePrompt {
	subject := params.Source
	if params.Target != nil && params.Branch != "" &&
		params.Target(params.Branch).State == domain.BranchTargetExisting {
		subject = params.Branch
	}
	if subject == "" {
		return SourceUpdatePrompt{SkipReason: domain.SourceUpdateSkipNoSource}
	}
	if rules.IsRemoteBranch(subject) {
		return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipRemote}
	}

	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: params.ProjectDir, Branch: subject})
	if rules.ShouldOfferFastForward(state) {
		return SourceUpdatePrompt{
			Branch:      subject,
			Show:        true,
			Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, subject, ab.Behind),
			Description: Pick(PickParams{Many: params.Many, One: domain.SourceFastForwardDescription, Several: domain.SourceFastForwardDescriptionMany}),
		}
	}
	if state == domain.DivergenceDiverged {
		return SourceUpdatePrompt{
			Branch:         subject,
			Show:           true,
			Title:          fmt.Sprintf(domain.SourceDivergedPrompt, subject, ab.Ahead, ab.Behind),
			Warning:        Pick(PickParams{Many: params.Many, One: domain.SourceDivergedWarning, Several: domain.SourceDivergedWarningMany}),
			AbortOnDecline: true,
			SkipReason:     domain.SourceUpdateSkipDiverged,
		}
	}
	return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipUpToDate}
}

type FastForwardSubjectParams struct {
	Target     domain.BranchTarget
	FromBranch string
	Branch     string
}

// FastForwardSubject names the branch a fast-forward updates: the source for a
// branch git is about to create, the worktree's own branch when it already exists.
func FastForwardSubject(params FastForwardSubjectParams) string {
	if rules.SourceIsStartPoint(params.Target.State) {
		return params.FromBranch
	}
	return params.Branch
}

type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// EnvParentFallback: the "parent" strategy silently sources .env from the main
// worktree when the source branch has no local one.
func EnvParentFallback(params EnvFallbackParams) (bool, string) {
	if params.Source == "" {
		return false, ""
	}
	applies := worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	})
	if !applies {
		return false, ""
	}
	return true, domain.EnvParentFallbackWarning
}

type PickParams struct {
	Many    bool
	One     string
	Several string
}

// Pick is the wording for one new worktree or for several.
func Pick(params PickParams) string {
	if params.Many {
		return params.Several
	}
	return params.One
}
