package shared

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

// WarnSchemaNewer says, before the command runs, that a file it read was
// written by a newer wtm: it is read as far as this one understands it, and a
// write to it will be refused. On stderr, so it never reaches a caller reading
// the command's output.
func WarnSchemaNewer(cmd *cobra.Command, schema statefile.Schema) {
	format, _ := cmd.Flags().GetString(domain.FlagOutput)
	if !rules.IsHumanFormat(format) || !statefile.Newer(schema) {
		return
	}
	render.Frame(cmd.ErrOrStderr(), func(w io.Writer) {
		render.SchemaNewer(w, schema)
	})
}
