package configedit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAValidEditIsDone(t *testing.T) {
	read, err := usecase(t, acme, strings.Replace(acme, "../.trees", "../wt", 1), nil).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
}

func TestAnEditLeavingAValueWtmCannotUseFailsWithItsProblemsAndTheEditAgain(t *testing.T) {
	read, err := usecase(t, acme, strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1), nil).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	failure := read.Failure()
	require.NotNil(t, failure)
	assert.Equal(t, configedit.CodeInvalid, failure.Code)
	require.NotNil(t, failure.FollowUp)
	assert.Equal(t, configedit.Name, failure.FollowUp.Command)
	require.Len(t, read.Items[0].Warnings, 1)
	assert.Equal(t, settings.PathEnvStrategy, read.Items[0].Warnings[0].Params["path"])
}

func TestAnEditThatNoLongerParsesIsRefusedAndKept(t *testing.T) {
	_, err := usecase(t, acme, acme+"x = [\n", nil).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configedit.CodeConfigInvalid, refusal.Code)
	require.NotNil(t, refusal.FollowUp)
	assert.Equal(t, configedit.Name, refusal.FollowUp.Command)
}

// Legacy said "Config file is missing after edit." and exited 0.
func TestAnEditThatRemovesTheFileIsNoConfig(t *testing.T) {
	_, err := usecase(t, acme, "", nil).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configedit.CodeConfigNotFound, refusal.Code)
}

// Legacy warned and exited 0.
func TestNoConfigIsRefusedBeforeTheEditor(t *testing.T) {
	u := usecase(t, "", acme, nil)
	u.Init = &domain.FollowUp{Command: domain.CommandInit}

	_, err := u.Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configedit.CodeConfigNotFound, refusal.Code)
	assert.Equal(t, domain.CommandInit, refusal.FollowUp.Command)
}

func TestAConfigThatDoesNotParseIsOpenedToBeFixed(t *testing.T) {
	read, err := usecase(t, acme+"x = [\n", acme, nil).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Nil(t, read.Failure())
}

func TestAnEditorThatFailsIsRefused(t *testing.T) {
	_, err := usecase(t, acme, acme, errors.New("exit status 3")).Run(t.Context(), configedit.Request{}, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configedit.CodeEditorFailed, refusal.Code)
}
