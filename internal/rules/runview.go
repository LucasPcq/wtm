package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RunViewLayoutParams struct {
	Width  int
	Height int
	// NoticeLines is how many rows a report wants under the header. It is served
	// only out of what the body can spare.
	NoticeLines int
}

type PreviewLayoutParams struct {
	Width  int
	Height int
}

// PreviewLayout is the run view hosted inside a panel: the whole rect is the
// pane. It shares the pane's chrome arithmetic with the full view, which is what
// keeps a job's output the same size in both.
func PreviewLayout(params PreviewLayoutParams) domain.RunViewLayout {
	width, height := max(params.Width, 0), max(params.Height, 0)
	return domain.RunViewLayout{
		Pane:     domain.Rect{Width: width, Height: height},
		PaneCols: max(width-domain.RunViewBorderWidth, 0),
		PaneRows: max(height-domain.RunViewPanelChrome, 0),
	}
}

// fitsSidebar says the list can be shown beside a pane still wide enough to be
// worth reading, once the frame has paid for its margin and gutter.
func fitsSidebar(width, margin, gutter int) bool {
	inner := max(width-2*margin, 0)
	return inner-domain.RunViewSidebarWidth-gutter >= domain.RunViewSidebarMinPaneCols
}

// ComputeRunViewLayout places the run view's regions for one frame, and sizes
// the emulator the pane box will hold. It is the single reference the renderer
// draws from and the job's PTY is resized against.
func ComputeRunViewLayout(params RunViewLayoutParams) domain.RunViewLayout {
	width, height := max(params.Width, 0), max(params.Height, 0)

	// No header row: what the view has to say about itself rides the help row,
	// and the row it used to spend is a row of output.
	helpHeight := min(1, height)
	headerHeight := 0
	available := max(height-helpHeight, 0)

	// The notice is served before the air: a report the reader has to act on
	// outranks a blank row, and a frame with room for neither keeps the report.
	noticeHeight := min(max(params.NoticeLines, 0), max(available-domain.RunViewMinBodyRows, 0))
	rest := available - noticeHeight

	gaps := 0
	if rest-domain.RunViewGapRows >= domain.RunViewMinBodyRows {
		gaps = domain.RunViewGapRows
		rest -= gaps
	}
	bodyHeight := rest

	// Columns follow the same order: the list outranks the margin, so the air is
	// dropped whenever taking it would be what pushes the list off the frame.
	margin, gutter := domain.RunViewMarginCols, domain.RunViewGutterCols
	if width < domain.RunViewMinAiredCols {
		margin, gutter = 0, 0
	}
	if fitsSidebar(width, 0, 0) && !fitsSidebar(width, margin, gutter) {
		margin, gutter = 0, 0
	}

	inner := max(width-2*margin, 0)
	sidebarWidth := 0
	if fitsSidebar(width, margin, gutter) {
		sidebarWidth = domain.RunViewSidebarWidth
	}
	paneGutter := gutter
	if sidebarWidth == 0 {
		paneGutter = 0
	}
	paneWidth := max(inner-sidebarWidth-paneGutter, 0)
	bodyY := headerHeight + gaps/2 + noticeHeight

	return domain.RunViewLayout{
		Header:         domain.Rect{X: margin, Y: 0, Width: inner, Height: headerHeight},
		Notice:         domain.Rect{X: margin, Y: headerHeight + gaps/2, Width: inner, Height: noticeHeight},
		Sidebar:        domain.Rect{X: margin, Y: bodyY, Width: sidebarWidth, Height: bodyHeight},
		Pane:           domain.Rect{X: margin + sidebarWidth + paneGutter, Y: bodyY, Width: paneWidth, Height: bodyHeight},
		Help:           domain.Rect{X: margin, Y: max(height-1, 0), Width: inner, Height: helpHeight},
		SidebarVisible: sidebarWidth > 0,
		SidebarRows:    max(bodyHeight-domain.RunViewSidebarChrome, 0),
		PaneCols:       max(paneWidth-domain.RunViewBorderWidth, 0),
		PaneRows:       max(bodyHeight-domain.RunViewPanelChrome, 0),
		MarginCols:     margin,
		GutterCols:     paneGutter,
		GapRows:        gaps / 2,
	}
}

type JobMarkParams struct {
	Status domain.JobStatus
	Step   domain.JobStep
	// Tracked is false for a job no start sequence has said anything about, and
	// for every job of a view that is only reading what already runs.
	Tracked bool
}

// JobMark reconciles what the start sequence knows of a job with what the
// daemon answers about it. The sequence wins while it has something to say: a
// job it has just reached is starting before any list call mentions it. Once it
// has said the job started, the daemon has the newer answer — it may have
// crashed since.
func JobMark(params JobMarkParams) domain.JobMark {
	if !params.Tracked {
		return statusMark(params.Status)
	}
	switch params.Step {
	case domain.JobStepStarting:
		return domain.JobMarkStarting
	case domain.JobStepDone:
		return domain.JobMarkDone
	case domain.JobStepFailed:
		return domain.JobMarkCrashed
	}
	return statusMark(params.Status)
}

func statusMark(status domain.JobStatus) domain.JobMark {
	switch status {
	case domain.JobStatusRunning:
		return domain.JobMarkRunning
	case domain.JobStatusDetached:
		return domain.JobMarkDetached
	case domain.JobStatusAttached:
		return domain.JobMarkShared
	case domain.JobStatusCrashed:
		return domain.JobMarkCrashed
	default:
		return domain.JobMarkStopped
	}
}

// ClipReport fits an abort report into the rows the frame can spare. Its head
// is what the run has to say, but its last line is how the band is taken off
// the screen: cutting from the bottom leaves a reader on a short terminal with
// a report they cannot remove and a pane they cannot widen. Below two rows
// there is only room for what happened.
func ClipReport(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if height == 1 {
		return lines[:1]
	}
	return append(lines[:height-1:height-1], lines[len(lines)-1])
}

// MatchesJobFilter reads the run view's filter box against a job's name. An
// empty filter matches everything: the box is open but says nothing yet.
func MatchesJobFilter(name, filter string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(filter)))
}

type ClipSegmentsParams struct {
	Text  string
	Sep   string
	Width int
}

// ClipSegments cuts a separated list at the last whole segment that fits. A
// hint bar sharing its row with the run's state is clipped often, and a cut
// through a word — "…enter focus · o open · r refresh · q" — reads as a bug in
// the hint rather than as a hint that did not fit.
func ClipSegments(params ClipSegmentsParams) string {
	if params.Width <= 0 || params.Sep == "" {
		return ""
	}
	if len([]rune(params.Text)) <= params.Width {
		return params.Text
	}

	segments := strings.Split(params.Text, params.Sep)
	kept := ""
	for _, segment := range segments {
		next := segment
		if kept != "" {
			next = kept + params.Sep + segment
		}
		if len([]rune(next)) > params.Width {
			break
		}
		kept = next
	}
	return kept
}
