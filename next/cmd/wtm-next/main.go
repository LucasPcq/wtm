// Command wtm-next is wtm rebuilt as use-case slices over a light hexagon.
package main

import (
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/surface/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Main(cli.Params{
		Version:       version,
		Args:          os.Args[1:],
		Open:          open,
		KillCancelled: shell.KillCancelled,
		AwaitShields:  shell.AwaitShields,
	}))
}
