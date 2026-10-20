// Package cli is tree's command line: its flags, the forest drawn as text, its JSON and its Mermaid flowchart, over tree's use case.
package cli

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/tree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

const (
	treeShort = "Show the worktree forest (parent → child)"
	treeLong  = "Draw the worktrees as the forest of their stacks, each under the branch it was\n" +
		"created from, with what a stacked-branch workflow needs: the commits a branch has\n" +
		"past its parent (↑N), where it stands against origin, uncommitted changes, the jobs\n" +
		"up in it, and \"needs sync\" when its parent has commits it lacks. A parent with no\n" +
		"worktree is a muted root. It never fetches and changes nothing.\n\n" +
		"--with-prs adds each branch's newest pull request, open, merged or closed.\n" +
		"--output json gives each node with what it needs next; --output mermaid a\n" +
		"flowchart to paste into a pull request."
	treeExample = `  wtm tree

  # With each branch's pull request, open, merged or closed
  wtm tree --with-prs

  # A flowchart to paste into a pull request
  wtm tree --output mermaid`
	formatMermaid = "mermaid"
)

// query is tree's form: a read.
type query = form.Query[tree.Request, tree.Questions, tree.Outcome]

// treeFlags spells the failures: tree's only field is a flag.
var treeFlags = (&query{}).Flags()

// New returns the tree command: its flags become a tree.Request, read and drawn on streams, as the v2 JSON envelope with --output json or a Mermaid flowchart with --output mermaid, its follow-ups spelled by commandLine.
func New(streams *output.Streams, env uicli.Env, open func(ctx context.Context, dir string) (tree.UseCase, error), commandLine output.CommandLine) *cobra.Command {
	// exits are the exit codes of tree's own codes, ahead of those of their kind.
	exits := map[string]int{tree.CodeConfigNotFound: uicli.ExitConfigNotFound, tree.CodeConfigInvalid: uicli.ExitError}
	run := &uicli.Run{Command: tree.Name, Streams: streams, Env: env, CommandLine: commandLine, Flags: treeFlags, Exits: exits}
	q := &query{
		Open: func(ctx context.Context, dir string) (form.Reader[tree.Request, tree.Outcome], error) {
			return open(ctx, dir)
		},
		Draw:    drawTree,
		Formats: map[string]func(w io.Writer, read tree.Outcome){formatMermaid: drawMermaid},
	}
	cmd := &cobra.Command{Use: tree.Name, Short: treeShort, Long: treeLong, Example: treeExample, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return q.Launch(cmd, args)
	}}
	q.Bind(cmd, run)

	return cmd
}
