package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestHookDir(t *testing.T) {
	tests := []struct {
		name    string
		cwd     string
		workDir string
		want    string
	}{
		{name: "empty cwd runs at the worktree root", cwd: "", workDir: "/trees/feat", want: "/trees/feat"},
		{name: "relative cwd is joined onto the worktree root", cwd: "apps/api", workDir: "/trees/feat", want: "/trees/feat/apps/api"},
		{name: "dotted relative cwd is cleaned", cwd: "./sub", workDir: "/trees/feat", want: "/trees/feat/sub"},
		{name: "absolute cwd is used as-is", cwd: "/elsewhere/dir", workDir: "/trees/feat", want: "/elsewhere/dir"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.HookDir(rules.HookDirParams{Cwd: tt.cwd, WorkDir: tt.workDir})
			if got != tt.want {
				t.Errorf("HookDir = %q, want %q", got, tt.want)
			}
		})
	}
}

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
