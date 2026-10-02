package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// ExecView is progress, not a result: it is repainted on each beat and erased
// by Close, so it is neither framed nor barred.
type ExecView struct {
	w       io.Writer
	rows    []string
	painted int
}

type ExecViewParams struct {
	W        io.Writer
	Branches []string
}

func NewExecView(params ExecViewParams) *ExecView {
	rows := make([]string, len(params.Branches))
	for i, branch := range params.Branches {
		rows[i] = progressRow(branch, domain.ExecQueuedLabel)
	}
	view := &ExecView{w: params.W, rows: rows}
	view.repaint()
	return view
}

func (v *ExecView) OnBeat(beat domain.ExecBeat) {
	if beat.Index < 0 || beat.Index >= len(v.rows) {
		return
	}
	v.rows[beat.Index] = rowFor(beat)
	v.repaint()
}

func (v *ExecView) Close() { v.clear() }

func rowFor(beat domain.ExecBeat) string {
	if beat.Started {
		return progressRow(beat.Result.Branch, domain.ExecRunningLabel)
	}
	label := rules.ExecResultLabel(beat.Result)
	if beat.Result.Status == domain.ExecStatusPassed {
		return styles.Success.Render(domain.GlyphSuccess) + " " + label
	}
	return styles.DangerText.Render(domain.GlyphFailure) + " " + label
}

func progressRow(branch, state string) string {
	return styles.Muted.Render(domain.GlyphProgress + " " + fmt.Sprintf(domain.ExecStateLabelFmt, branch, state))
}

func (v *ExecView) repaint() {
	v.clear()
	width := TerminalWidthOf(v.w)
	for _, row := range v.rows {
		if width > 0 {
			row = styles.Truncate(styles.TruncateParams{Value: row, Width: width - len([]rune(Indent))})
		}
		fmt.Fprintf(v.w, "%s%s\n", Indent, row)
		v.painted++
	}
}

func (v *ExecView) clear() {
	if v.painted == 0 {
		return
	}
	fmt.Fprintf(v.w, domain.AnsiCursorUpFmt+domain.AnsiClearBelow, v.painted)
	v.painted = 0
}
