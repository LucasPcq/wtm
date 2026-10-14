package render_test

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
)

var (
	catalog = text.NewCatalog()
	wording = render.Wording{Catalog: catalog, Names: asFlag}
)

type pathDetail struct{}

func (pathDetail) Plan(json.RawMessage) []styles.PlanRow { return nil }

func (pathDetail) Item(w io.Writer, item kernel.Item[json.RawMessage]) {
	fmt.Fprintf(w, "    %s  %s\n", item.Subject, item.Detail)
}

func done(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Detail: json.RawMessage(`"/wt/` + subject + `"`)}
}

func unchanged(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Reason: kernel.ReasonUnchanged}
}
