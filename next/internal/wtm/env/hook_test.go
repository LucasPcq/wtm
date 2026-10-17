package env

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
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
		{name: "empty string", input: "", want: ""},
		{name: "no placeholders", input: "echo hello", want: "echo hello"},
		{name: "worktree placeholder", input: "cd {{worktree}}", want: "cd /path/to/worktree"},
		{name: "branch placeholder", input: "git checkout {{branch}}", want: "git checkout feat/my-branch"},
		{name: "root placeholder", input: "ls {{root}}", want: "ls /path/to/root"},
		{name: "from_branch placeholder", input: "git merge {{from_branch}}", want: "git merge main"},
		{name: "multiple placeholders", input: "cp {{root}}/.env {{worktree}}/.env", want: "cp /path/to/root/.env /path/to/worktree/.env"},
		{name: "all placeholders", input: "{{worktree}} {{branch}} {{root}} {{from_branch}}", want: "/path/to/worktree feat/my-branch /path/to/root main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vars.Plain(tc.input))
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
		{name: "no placeholders", hook: project.Hook{Cmd: "npm install", Cwd: "/app"}, want: project.Hook{Cmd: "npm install", Cwd: "/app"}},
		{name: "cmd with worktree placeholder", hook: project.Hook{Cmd: "ls {{worktree}}"}, want: project.Hook{Cmd: "ls /wt"}},
		{name: "cwd with root placeholder", hook: project.Hook{Cmd: "make build", Cwd: "{{root}}/scripts"}, want: project.Hook{Cmd: "make build", Cwd: "/root/scripts"}},
		{name: "both cmd and cwd with placeholders", hook: project.Hook{Cmd: "cp {{root}}/.env {{worktree}}/.env", Cwd: "{{worktree}}"}, want: project.Hook{Cmd: "cp /root/.env /wt/.env", Cwd: "/wt"}},
		{name: "continue_on_error is kept", hook: project.Hook{Cmd: "ls {{worktree}}", ContinueOnError: true}, want: project.Hook{Cmd: "ls /wt", ContinueOnError: true}},
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
		{name: "a plain value stays bare", input: "cd {{root}} && git log {{branch}}", want: "cd /repo && git log feat/x"},
		{name: "unquoted", input: "cd {{worktree}}", want: `cd '/t/it'\''s $HOME'`},
		{name: "inside double quotes", input: `cd "{{worktree}}/sub"`, want: `cd "/t/it's \$HOME/sub"`},
		{name: "inside single quotes", input: "cd '{{worktree}}'", want: `cd '/t/it'\''s $HOME'`},
		{name: "an escaped quote opens nothing", input: `echo \" {{worktree}}`, want: `echo \" '/t/it'\''s $HOME'`},
		{name: "a double quote inside single quotes opens nothing", input: `echo '"' {{worktree}}`, want: `echo '"' '/t/it'\''s $HOME'`},
		{name: "an empty value is an empty word", input: "git merge {{from_branch}}", want: "git merge ''"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vars.Shell(tc.input))
		})
	}
}
