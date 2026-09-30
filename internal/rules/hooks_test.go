package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestInterpolate(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree:   "/path/to/worktree",
		Branch:     "feat/my-branch",
		Root:       "/path/to/root",
		FromBranch: "main",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no placeholders",
			input: "echo hello",
			want:  "echo hello",
		},
		{
			name:  "worktree placeholder",
			input: "cd {{worktree}}",
			want:  "cd /path/to/worktree",
		},
		{
			name:  "branch placeholder",
			input: "git checkout {{branch}}",
			want:  "git checkout feat/my-branch",
		},
		{
			name:  "root placeholder",
			input: "ls {{root}}",
			want:  "ls /path/to/root",
		},
		{
			name:  "from_branch placeholder",
			input: "git merge {{from_branch}}",
			want:  "git merge main",
		},
		{
			name:  "multiple placeholders",
			input: "cp {{root}}/.env {{worktree}}/.env",
			want:  "cp /path/to/root/.env /path/to/worktree/.env",
		},
		{
			name:  "all placeholders",
			input: "{{worktree}} {{branch}} {{root}} {{from_branch}}",
			want:  "/path/to/worktree feat/my-branch /path/to/root main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.Interpolate(tt.input, vars)
			if got != tt.want {
				t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTemplateVars(t *testing.T) {
	vars := rules.TemplateVars{
		Worktree:   "/wt",
		Branch:     "feature",
		Root:       "/root",
		FromBranch: "main",
	}

	tests := []struct {
		name    string
		hook    domain.HookCommand
		wantCmd string
		wantCwd string
	}{
		{
			name:    "no placeholders",
			hook:    domain.HookCommand{Cmd: "npm install", Cwd: "/app"},
			wantCmd: "npm install",
			wantCwd: "/app",
		},
		{
			name:    "cmd with worktree placeholder",
			hook:    domain.HookCommand{Cmd: "ls {{worktree}}", Cwd: ""},
			wantCmd: "ls /wt",
			wantCwd: "",
		},
		{
			name:    "cwd with root placeholder",
			hook:    domain.HookCommand{Cmd: "make build", Cwd: "{{root}}/scripts"},
			wantCmd: "make build",
			wantCwd: "/root/scripts",
		},
		{
			name:    "both cmd and cwd with placeholders",
			hook:    domain.HookCommand{Cmd: "cp {{root}}/.env {{worktree}}/.env", Cwd: "{{worktree}}"},
			wantCmd: "cp /root/.env /wt/.env",
			wantCwd: "/wt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.ResolveTemplateVars(tt.hook, vars)
			if got.Cmd != tt.wantCmd {
				t.Errorf("ResolveTemplateVars().Cmd = %q, want %q", got.Cmd, tt.wantCmd)
			}
			if got.Cwd != tt.wantCwd {
				t.Errorf("ResolveTemplateVars().Cwd = %q, want %q", got.Cwd, tt.wantCwd)
			}
		})
	}
}

func TestInterpolateShellQuotesForItsSpot(t *testing.T) {
	vars := rules.TemplateVars{Worktree: "/t/it's $HOME", Branch: "feat/x", Root: "/repo"}

	tests := []struct {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rules.InterpolateShell(tt.input, vars); got != tt.want {
				t.Errorf("InterpolateShell(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
