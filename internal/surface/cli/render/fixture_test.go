package render_test

import (
	"encoding/json"

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

func (pathDetail) Item(item kernel.Item[json.RawMessage]) render.ItemView {
	return render.ItemView{Rows: []styles.PlanRow{{Label: item.Subject, Values: []string{string(item.Detail)}}}, Next: []render.NextStepParams{{Command: "wtm go " + item.Subject}}}
}

type planAndPath struct{ pathDetail }

func (planAndPath) Plan(detail json.RawMessage) []styles.PlanRow {
	return render.GenericDetail{}.Plan(detail)
}

func done(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Detail: json.RawMessage(`"/wt/` + subject + `"`)}
}

func unchanged(subject string) kernel.Item[json.RawMessage] {
	return kernel.Item[json.RawMessage]{Subject: subject, Status: kernel.StatusDone, Reason: kernel.ReasonUnchanged}
}
