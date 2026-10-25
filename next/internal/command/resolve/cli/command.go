// Package cli is resolve's command line: its argument, its worktree picker, and the path of the worktree resolved, bare on stdout for a shell to enter, or its JSON, over resolve's use case.
package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	resolveShort = "Resolve a branch to its worktree path"
	resolveLong  = "Print the path of a worktree, named by its branch, its path or part of its\n" +
		"branch, alone on stdout so that a shell can enter it, --quiet or not. `wtm go`\n" +
		"is the shell function (`wtm shell-init`) that runs `wtm resolve` and enters the\n" +
		"path it prints.\n\n" +
		"On a terminal, a part of a branch several worktrees hold opens the picker over\n" +
		"them, and no [worktree] the picker over every worktree, the cursor on the one\n" +
		"you are in. Anywhere else (no terminal, --output json, --quiet) an ambiguous\n" +
		"name is refused and no [worktree] resolves the one you are in. In text, nothing\n" +
		"is printed on stdout unless a worktree is resolved."
	resolveExample = `  wtm resolve feat/login

  # Use it in a script
  cd "$(wtm resolve feat/login)"

  wtm resolve login --output json`
)

// query is resolve's form: a read whose text a shell captures.
type query = form.Query[resolve.Request, resolve.Questions, resolve.Outcome]

// resolveFlags spells the failures: the worktree is positional.
var resolveFlags = (&query{Words: words}).Flags()

// New returns the resolve command: its argument becomes a resolve.Request, the worktree picked first when none is named, or when the name is ambiguous, and the user can see and answer the picker; it prints the path resolved on streams, or the v2 JSON envelope with --output json.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (resolve.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	run := &uicli.Run{Command: resolve.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: resolveFlags}
	q := &query{
		Words: words,
		Open: func(ctx context.Context, dir string) (form.Reader[resolve.Request, resolve.Questions, resolve.Outcome], error) {
			return open(ctx, dir)
		},
		Draw:     drawResolve,
		Captured: true,
	}
	cmd := &cobra.Command{Use: resolve.Name + " [worktree]", Short: resolveShort, Long: resolveLong, Example: resolveExample, Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
