package configshow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDocumentIsEachSettingAsConfigTomlHoldsIt(t *testing.T) {
	read, err := usecase(t, acme).Run(t.Context(), configshow.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Items, 1)
	assert.Empty(t, read.Warnings)
	detail := read.Items[0].Detail
	require.NotNil(t, detail)
	answers := map[string][]string{}
	for _, field := range detail.Settings.Fields {
		answers[field.Spec().Path] = field.Answer()
		assert.Equal(t, domain.OriginConfig, field.Spec().Origin, field.Spec().Path)
	}
	assert.Equal(t, []string{"../.trees"}, answers[settings.PathBasePath])
	assert.Equal(t, []string{"apps/api/.env", "apps/web/.env"}, answers[settings.PathEnvFiles])
	assert.Len(t, answers[settings.PathOnCreate], 2)
	assert.Equal(t, []configshow.Default{
		{Path: settings.PathBasePath, Value: "../.trees", InUse: true},
		{Path: settings.PathBaseBranch, Value: "main", InUse: true},
		{Path: settings.PathEnvStrategy, Value: "example", InUse: true},
	}, detail.Defaults)
	assert.Equal(t, []configshow.Remembered{{Question: "env_strategy", Value: "parent", Entry: "env_strategy=parent"}}, detail.Remembered)
}

func TestAProblemConfigTomlHoldsIsAWarningOnItsSetting(t *testing.T) {
	read, err := usecase(t, strings.Replace(acme, `base_branch = "main"`, `base_branch = "gone"`, 1)).Run(t.Context(), configshow.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Warnings, 1)
	assert.Equal(t, configshow.CodeInvalidValue, read.Warnings[0].Code)
	assert.Equal(t, settings.PathBaseBranch, read.Warnings[0].Params["path"])
	assert.Equal(t, configshow.Name+"."+settings.CodeBaseBranchUnknown, read.Warnings[0].Params["problem"])
	assert.Nil(t, read.Failure())
}

func TestNoConfigOrOneThatDoesNotParseIsRefused(t *testing.T) {
	cases := map[string]struct {
		content string
		code    string
	}{
		"none":     {"", configshow.CodeConfigNotFound},
		"no_parse": {acme + "base_path = [\n", configshow.CodeConfigInvalid},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := usecase(t, c.content).Run(t.Context(), configshow.Request{}, domain.RunOptions{})

			refusal, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, "%v", err)
			assert.Equal(t, c.code, refusal.Code)
		})
	}
}

func TestAConfigThatDoesNotParseSaysWhereAndOffersTheEdit(t *testing.T) {
	u := usecase(t, strings.Replace(acme, `base_branch = "main"`, `base_branch =`, 1))
	u.Edit = &domain.FollowUp{Command: domain.CommandConfigEdit}

	_, err := u.Run(t.Context(), configshow.Request{}, domain.RunOptions{})

	refusal, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, configshow.CodeConfigInvalid, refusal.Code)
	assert.Equal(t, "5", refusal.Params["line"])
	assert.Equal(t, settings.PathBaseBranch, refusal.Params["setting"])
	require.NotNil(t, refusal.FollowUp)
	assert.Equal(t, domain.CommandConfigEdit, refusal.FollowUp.Command)
}

func TestAValueWtmCannotUseIsShownWithItsProblem(t *testing.T) {
	read, err := usecase(t, strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1)).Run(t.Context(), configshow.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, read.Warnings, 1)
	assert.Equal(t, settings.PathEnvStrategy, read.Warnings[0].Params["path"])
	assert.Equal(t, domain.CodeOneOf, read.Warnings[0].Params["problem"])
}

func TestAConfigANewerWtmWroteIsReadAndSaidSo(t *testing.T) {
	read, err := usecase(t, strings.Replace(acme, "schema = 1", "schema = 9", 1)).Run(t.Context(), configshow.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.True(t, read.Items[0].Detail.Newer)
	require.Len(t, read.Warnings, 1)
	assert.Equal(t, configshow.CodeConfigNewer, read.Warnings[0].Code)
}
