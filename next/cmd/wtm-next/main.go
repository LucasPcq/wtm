// Command wtm-next is wtm rebuilt as use-case slices over a light hexagon.
package main

import (
	"context"
	"os"

	"github.com/LucasPcq/wtm/next/internal/surface/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(context.Background(), cli.NewRoot(version)))
}
