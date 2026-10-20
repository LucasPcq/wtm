package cli

import (
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfTree(t *testing.T) {
	got := runTree(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "tree_help", got.Stdout)
}

func TestTreeDrawsEachNodeUnderItsParentItsBadgesAligned(t *testing.T) {
	got := runTree(t, world{})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "tree_text", got.Stdout)
}

func TestWithPRsANodeShowsItsPullRequestAndItsState(t *testing.T) {
	got := runTree(t, world{}, "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "tree_with_prs", got.Stdout)
}

func TestWhatCannotBeAskedIsAWarningUnderTheTree(t *testing.T) {
	got := runTree(t, world{PRsErr: fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable), JobsErr: run.ErrUnknown}, "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "tree_warnings", got.Stdout)
}

func TestTheJSONOfTreeIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runTree(t, world{}, "--with-prs", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_tree", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Equal(t, tree.Name, doc.Command)
	assert.Empty(t, doc.Op)
	assert.Empty(t, doc.Fingerprint)
}

func TestTheMermaidOfTreeIsAFlowchartUnframedEvenQuiet(t *testing.T) {
	got := runTree(t, world{}, "--with-prs", "--output", "mermaid", "--quiet")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "tree_mermaid", got.Stdout)
}

func TestMermaidGivesEachBranchItsOwnNode(t *testing.T) {
	branches := []string{"feat/a", "feat-a", "feat.a", "end", `say"hi"`}
	w := world{Meta: map[string]worktree.Meta{}}
	for _, branch := range branches {
		w.Worktrees = append(w.Worktrees, worktree.Worktree{Path: "/repo/.trees/" + worktree.PathSegment(branch) + "-" + branch, State: worktree.OnBranch{Branch: branch}})
		w.Meta[branch] = worktree.Meta{SourceBranch: "main"}
	}

	got := runTree(t, w, "--output", "mermaid")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "tree_mermaid_ids", got.Stdout)
}

func TestARefusalWithJSONIsTheEnvelopeOnStdout(t *testing.T) {
	got := runTree(t, world{ConfigErr: project.ErrConfigNotFound}, "--output", "json")

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "envelope_config_not_found", got.Stdout)
}

func TestARefusalIsTextOnStderrInTextAndMermaid(t *testing.T) {
	for _, format := range []string{"text", "mermaid"} {
		t.Run(format, func(t *testing.T) {
			got := runTree(t, world{ConfigErr: project.ErrConfigNotFound}, "--output", format)

			assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
			assert.Empty(t, got.Stdout)
			formtest.Golden(t, "tree_config_not_found", got.Stderr)
		})
	}
}

func TestTreeTakesNoArgument(t *testing.T) {
	got := runTree(t, world{}, "feat/a")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `unknown command "feat/a"`)
}

func TestAnUnknownFormatIsRefusedNamingTheThree(t *testing.T) {
	got := runTree(t, world{}, "--output", "dot")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `invalid --output "dot": expected one of text, json, mermaid`)
}
