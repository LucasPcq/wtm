package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// InitGlobalRecapParams holds inputs for the framed end-of-init global recap.
type InitGlobalRecapParams struct {
	Fields    []domain.RecapField
	NextSteps []NextStepParams
}

// InitGlobalRecap prints the framed recap shown after the global config is written:
// a pill title, the aligned config summary, then the next-step bullets. It emits a
// raw box with no surrounding blank lines; the caller's frame owns the padding.
func InitGlobalRecap(w io.Writer, p InitGlobalRecapParams) {
	printInitRecap(w, initRecap{
		Title:     domain.InitRecapTitleGlobal,
		Fields:    p.Fields,
		NextSteps: p.NextSteps,
	})
}

// InitProjectRecapParams holds inputs for the framed end-of-init project recap.
type InitProjectRecapParams struct {
	ConfigPath string
	Fields     []domain.RecapField
	NextSteps  []NextStepParams
}

// InitProjectRecap prints the framed recap shown after the project config is
// written: a pill title, the "Created …" line, the aligned config summary, then the
// next-step bullets. It emits a raw box with no surrounding blank lines; the
// caller's frame owns the padding.
func InitProjectRecap(w io.Writer, p InitProjectRecapParams) {
	printInitRecap(w, initRecap{
		Title:     domain.InitRecapTitleProject,
		Lead:      fmt.Sprintf("Created %s", p.ConfigPath),
		Fields:    p.Fields,
		NextSteps: p.NextSteps,
	})
}

// initRecap is the shared body model for both init recaps, so the global and
// project screens render with the exact same structure and spacing.
type initRecap struct {
	Title     string
	Lead      string
	Fields    []domain.RecapField
	NextSteps []NextStepParams
}

func printInitRecap(w io.Writer, r initRecap) {
	var b strings.Builder
	if r.Lead != "" {
		b.WriteString(r.Lead)
		b.WriteString("\n\n")
	}
	b.WriteString(alignRecapFields(r.Fields))
	if len(r.NextSteps) > 0 {
		b.WriteString("\n\n")
		b.WriteString(styles.Bold.Render(domain.InitRecapNextSteps))
		for _, step := range r.NextSteps {
			b.WriteString("\n" + NextStepLine(step))
		}
	}

	fmt.Fprintln(w, styles.RenderRecap(styles.IntroParams{
		Width:   domain.RecapWidth,
		Title:   domain.GlyphSuccess + " " + r.Title,
		Body:    b.String(),
		InFrame: true,
	}))
}

// alignRecapFields renders label/value rows with the values aligned to a common
// column. Labels are plain ASCII, so byte length equals printable width.
func alignRecapFields(fields []domain.RecapField) string {
	width := 0
	for _, f := range fields {
		if l := len(f.Label); l > width {
			width = l
		}
	}

	var b strings.Builder
	for i, f := range fields {
		if i > 0 {
			b.WriteString("\n")
		}
		pad := strings.Repeat(" ", width-len(f.Label))
		b.WriteString(styles.Muted.Render(f.Label) + pad + "  " + f.Value)
	}
	return b.String()
}
