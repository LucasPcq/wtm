package env

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
)

func TestAHookLogIsNamedAfterItsEscapedBranch(t *testing.T) {
	assert.Equal(t, filepath.Join("/state", "hooks", "on_create-feat%2Fx.log"), HookLogPath(HookLogPathParams{StateDir: "/state", Branch: "feat/x"}))
}

func TestNoStateDirMeansNoLog(t *testing.T) {
	assert.Empty(t, HookLogPath(HookLogPathParams{Branch: "feat/x"}))
}

func TestAPlainStringTakesEveryVariableAsIs(t *testing.T) {
	vars := HookVars{Worktree: "/path/to/worktree", Branch: "feat/my-branch", Root: "/path/to/root", FromBranch: "main"}
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty_string", input: "", want: ""},
		{name: "no_placeholders", input: "echo hello", want: "echo hello"},
		{name: "worktree_placeholder", input: "cd {{worktree}}", want: "cd /path/to/worktree"},
		{name: "branch_placeholder", input: "git checkout {{branch}}", want: "git checkout feat/my-branch"},
		{name: "root_placeholder", input: "ls {{root}}", want: "ls /path/to/root"},
		{name: "from_branch_placeholder", input: "git merge {{from_branch}}", want: "git merge main"},
		{name: "multiple_placeholders", input: "cp {{root}}/.env {{worktree}}/.env", want: "cp /path/to/root/.env /path/to/worktree/.env"},
		{name: "all_placeholders", input: "{{worktree}} {{branch}} {{root}} {{from_branch}}", want: "/path/to/worktree feat/my-branch /path/to/root main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vars.plain(tc.input))
		})
	}
}

func TestAHookTakesItsVariablesInItsCommandAndItsDirectory(t *testing.T) {
	vars := HookVars{Worktree: "/wt", Branch: "feature", Root: "/root", FromBranch: "main"}
	cases := []struct {
		name string
		hook project.Hook
		want project.Hook
	}{
		{name: "no_placeholders", hook: project.Hook{Cmd: "npm install", Cwd: "/app"}, want: project.Hook{Cmd: "npm install", Cwd: "/app"}},
		{name: "cmd_worktree_placeholder", hook: project.Hook{Cmd: "ls {{worktree}}"}, want: project.Hook{Cmd: "ls /wt"}},
		{name: "cwd_root_placeholder", hook: project.Hook{Cmd: "make build", Cwd: "{{root}}/scripts"}, want: project.Hook{Cmd: "make build", Cwd: "/root/scripts"}},
		{name: "cmd_and_cwd_placeholders", hook: project.Hook{Cmd: "cp {{root}}/.env {{worktree}}/.env", Cwd: "{{worktree}}"}, want: project.Hook{Cmd: "cp /root/.env /wt/.env", Cwd: "/wt"}},
		{name: "continue_on_error_kept", hook: project.Hook{Cmd: "ls {{worktree}}", ContinueOnError: true}, want: project.Hook{Cmd: "ls /wt", ContinueOnError: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vars.Hook(tc.hook))
		})
	}
}

func TestAShellLineQuotesEachValueForItsSpot(t *testing.T) {
	vars := HookVars{Worktree: "/t/it's $HOME", Branch: "feat/x", Root: "/repo"}
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain_stays_bare", input: "cd {{root}} && git log {{branch}}", want: "cd /repo && git log feat/x"},
		{name: "unquoted", input: "cd {{worktree}}", want: `cd '/t/it'\''s $HOME'`},
		{name: "inside_double_quotes", input: `cd "{{worktree}}/sub"`, want: `cd "/t/it's \$HOME/sub"`},
		{name: "inside_single_quotes", input: "cd '{{worktree}}'", want: `cd '/t/it'\''s $HOME'`},
		{name: "escaped_quote_opens_nothing", input: `echo \" {{worktree}}`, want: `echo \" '/t/it'\''s $HOME'`},
		{name: "double_inside_single_opens_nothing", input: `echo '"' {{worktree}}`, want: `echo '"' '/t/it'\''s $HOME'`},
		{name: "empty_value_empty_word", input: "git merge {{from_branch}}", want: "git merge ''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vars.shell(tc.input))
		})
	}
}
