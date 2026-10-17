// Command wtm-next is wtm rebuilt as use-case slices over a light hexagon.
package main

import (
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	createcli "github.com/LucasPcq/wtm/next/internal/command/create/cli"
	extractcli "github.com/LucasPcq/wtm/next/internal/command/extract/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	os.Exit(cli.Main(cli.Params{
		Version:       version,
		Args:          os.Args[1:],
		Commands:      []*cobra.Command{createcli.New(openCreate), extractcli.New(openExtract)},
		KillCancelled: shell.KillCancelled,
		AwaitShields:  shell.AwaitShields,
	}))
}
