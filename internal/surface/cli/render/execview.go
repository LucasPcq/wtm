package render

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

type execRowState int

const (
	execQueued execRowState = iota
	execRunning
	execDone
)

// ExecView is progress, not a result: it is repainted on each beat and erased
// by Close, so it is neither framed nor barred.
type ExecView struct {
	w        io.Writer
	branches []string
	states   []execRowState
	rows     []string
	maxRows  int
	painted  int
}

type ExecViewParams struct {
	W        io.Writer
	Branches []string
	// Height is the terminal's row count, 0 to measure it. The region must fit:
	// the cursor cannot climb back over rows that scrolled off the top.
	Height int
}

func NewExecView(params ExecViewParams) *ExecView {
	rows := make([]string, len(params.Branches))
	for i, branch := range params.Branches {
		rows[i] = progressRow(branch, domain.ExecQueuedLabel)
	}
	height := params.Height
	if height == 0 {
		height = terminalHeightOf(params.W)
	}
	view := &ExecView{
		w:        params.W,
		branches: params.Branches,
		states:   make([]execRowState, len(params.Branches)),
		rows:     rows,
		maxRows:  height - domain.ExecViewMargin,
	}
	view.repaint()
	return view
}

func (v *ExecView) OnBeat(beat domain.ExecBeat) {
	if beat.Index < 0 || beat.Index >= len(v.rows) {
		return
	}
	v.rows[beat.Index] = rowFor(beat)
	v.states[beat.Index] = execDone
	if beat.Started {
		v.states[beat.Index] = execRunning
	}
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

// visible is every row when they fit; otherwise a count of each state and the
// running rows, which are the ones still changing.
func (v *ExecView) visible() []string {
	if v.maxRows <= 0 || len(v.rows) <= v.maxRows {
		return v.rows
	}
	var counts [3]int
	var running []string
	for i, state := range v.states {
		counts[state]++
		if state == execRunning {
			running = append(running, v.rows[i])
		}
	}
	summary := styles.Muted.Render(domain.GlyphProgress + " " + fmt.Sprintf(domain.ExecViewSummaryFmt, counts[execDone], counts[execRunning], counts[execQueued]))
	shown := append([]string{summary}, running...)
	return shown[:min(len(shown), v.maxRows)]
}

func (v *ExecView) repaint() {
	v.clear()
	width := TerminalWidthOf(v.w)
	for _, row := range v.visible() {
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
	fmt.Fprintf(v.w, domain.AnsiPrevLineFmt+domain.AnsiClearBelow, v.painted)
	v.painted = 0
}

func terminalHeightOf(w io.Writer) int {
	stream, _ := unwrapStream(w)
	file, ok := stream.(*os.File)
	if !ok {
		return 0
	}
	_, rows, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0
	}
	return rows
}
