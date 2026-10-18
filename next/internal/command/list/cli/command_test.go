package cli

import (
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfList(t *testing.T) {
	got := runList(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "list_help", got.Stdout)
}

func TestListDrawsEachWorktreeAlignedTheActiveOneMarked(t *testing.T) {
	got := runList(t, world{Cwd: "/repo/.trees/feat-a/src"})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "list_text", got.Stdout)
}

func TestWithPRsAWorktreeShowsItsPullRequest(t *testing.T) {
	got := runList(t, world{}, "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "list_with_prs", got.Stdout)
}

func TestWhatCannotBeAskedIsAWarningUnderTheList(t *testing.T) {
	got := runList(t, world{PRsErr: fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable), JobsErr: run.ErrUnknown}, "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "list_warnings", got.Stdout)
}

func TestTheJSONOfListIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runList(t, world{}, "--with-prs", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_list", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Empty(t, doc.Op)
	assert.Empty(t, doc.Fingerprint)
}

func TestARefusalWithJSONIsTheEnvelopeOnStdout(t *testing.T) {
	got := runList(t, world{ConfigErr: project.ErrConfigNotFound}, "--output", "json")

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "envelope_config_not_found", got.Stdout)
}

func TestListTakesNoArgument(t *testing.T) {
	got := runList(t, world{}, "feat/a")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `unknown command "feat/a"`)
}

func TestAWorktreeOnNoBranchIsNamedByItsPath(t *testing.T) {
	got := runList(t, world{}, "--output", "json")

	doc := formtest.Decoded[struct{}](t, got.Stdout)
	var subjects []string
	for _, item := range doc.Items {
		subjects = append(subjects, item.Subject)
	}
	assert.Equal(t, []string{"main", "feat/a", "feat/b", "/repo/.trees/detached", "feat/gone"}, subjects)
	assert.Equal(t, list.Name, doc.Command)
}
