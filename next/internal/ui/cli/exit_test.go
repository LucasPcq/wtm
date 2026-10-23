package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestEachKindExitsOnItsCode(t *testing.T) {
	cases := map[domain.Kind]int{
		domain.KindInvalid:      ExitUsage,
		domain.KindCancelled:    ExitCancelled,
		domain.KindRefused:      ExitRefused,
		domain.KindConflict:     ExitConflict,
		domain.KindPrecondition: ExitPrecondition,
		domain.KindInternal:     ExitError,
	}
	for kind, code := range cases {
		assert.Equal(t, code, exitCode(&Failed{Err: &domain.Error{Kind: kind, Code: "some.code"}}, false), kind)
	}
}

func TestACodeWithAnExitOfItsOwnExitsOnIt(t *testing.T) {
	assert.Equal(t, 15, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindConflict, Code: codeConflict}, Exits: map[string]int{codeConflict: 15}}, false))
	assert.Equal(t, ExitNotGitRepo, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository}}, false))
}

func TestEveryCommandsConfigRefusalExitsOnItsSuffix(t *testing.T) {
	for _, command := range []string{"create", "extract", "list", "status", "tree", "events", "run up"} {
		t.Run(strings.ReplaceAll(command, " ", "_"), func(t *testing.T) {
			notFound := &domain.Error{Kind: domain.KindPrecondition, Code: command + domain.ConfigNotFound}
			invalid := &domain.Error{Kind: domain.KindPrecondition, Code: command + domain.ConfigInvalid}

			assert.Equal(t, ExitConfigNotFound, exitCode(&Failed{Err: notFound}, false))
			assert.Equal(t, ExitError, exitCode(&Failed{Err: invalid}, false))
		})
	}
}

func TestTheExitCodesKeepWtmsValues(t *testing.T) {
	// wtm's values (internal/domain/constants.go), which a script written for wtm branches on; 23 to 25 are wtm-next's own.
	legacy := map[string][2]int{
		"ok":               {ExitOK, 0},
		"error":            {ExitError, 1},
		"usage":            {ExitUsage, 2},
		"config_not_found": {ExitConfigNotFound, 12},
		"cancelled":        {ExitCancelled, 19},
		"stream_newer":     {ExitStreamNewer, 20},
		"not_git_repo":     {ExitNotGitRepo, 21},
		"state_newer":      {ExitStateSchemaNewer, 22},
	}
	for name, pair := range legacy {
		assert.Equal(t, pair[1], pair[0], name)
	}
}

func TestAStateFileOfANewerSchemaExitsOnItsCodeWhereverItSurfaced(t *testing.T) {
	newer := &domain.Error{Kind: domain.KindInternal, Code: "unit.step_failed", Cause: &domain.SchemaNewer{Path: "meta.json", Found: 3, Supported: 2}}

	assert.Equal(t, ExitStateSchemaNewer, exitCode(&Failed{Err: newer}, false))
}

func TestAFailureAfterAnInterruptIsTheInterrupt(t *testing.T) {
	assert.Equal(t, ExitCancelled, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindInternal}}, true))
}

func TestOnlyAPreconditionExitsOnTheSuffixOfAConfigRefusal(t *testing.T) {
	assert.Equal(t, ExitRefused, exitCode(&Failed{Err: &domain.Error{Kind: domain.KindRefused, Code: "sync.remote_config_not_found"}}, false))
}
