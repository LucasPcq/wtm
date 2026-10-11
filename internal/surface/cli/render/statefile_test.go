package render

import (
	"bytes"
	"strings"
	"testing"

	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

func TestSchemaNewerNamesTheFileItsSchemaAndWhatToDo(t *testing.T) {
	var out bytes.Buffer
	SchemaNewer(&out, statefile.Schema{Path: "run.toml", Found: 3, Supported: 1})

	for _, want := range []string{
		domainstatefile.SchemaNewerTitle,
		"run.toml is at schema 3, newer than the 1 this wtm knows",
		domainstatefile.SchemaNewerCost,
		domainstatefile.SchemaNewerFix,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("callout misses %q:\n%s", want, out.String())
		}
	}
}
