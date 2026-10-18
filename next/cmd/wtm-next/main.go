// Command wtm-next manages the git worktrees of a repository: it is wtm rebuilt as use-case slices over a light hexagon, working on the same repositories and state files.
//
// main.go registers each command's command line on the root; wire.go builds each adapter once and injects it; project.go finds the repository a command runs on from the directory the command line names.
package main

import (
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	createcli "github.com/LucasPcq/wtm/next/internal/command/create/cli"
	extractcli "github.com/LucasPcq/wtm/next/internal/command/extract/cli"
	listcli "github.com/LucasPcq/wtm/next/internal/command/list/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	streams := output.System()
	env := cli.ReadEnv()
	os.Exit(cli.Main(cli.Params{
		Version:       version,
		Streams:       streams,
		Args:          os.Args[1:],
		Commands:      []*cobra.Command{createcli.New(streams, env, openCreate), extractcli.New(streams, env, openExtract), listcli.New(streams, env, openList)},
		KillCancelled: shell.KillCancelled,
		AwaitShields:  shell.AwaitShields,
	}))
}
