package render

import (
	"fmt"
	"io"

	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

// SchemaNewer is the callout for a file a newer wtm wrote: read as far as this
// one understands it, never written.
func SchemaNewer(w io.Writer, schema statefile.Schema) {
	Callout(w, domainstatefile.SchemaNewerTitle, []string{
		fmt.Sprintf(domainstatefile.SchemaNewerWhyFmt, schema.Path, schema.Found, schema.Supported),
		domainstatefile.SchemaNewerCost,
		domainstatefile.SchemaNewerFix,
	})
}
