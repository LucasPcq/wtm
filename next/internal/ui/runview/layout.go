package runview

const (
	sidebarWidth       = 26
	sidebarMinPaneCols = 40 // the narrowest pane the list leaves beside it
	minBodyRows        = 3
	marginCols         = 2
	gutterCols         = 1
	gapRows            = 2
	minAiredCols       = 60 // below it the frame drops its margin
	minPanelCols       = 3
	minPanelRows       = 3
	borderWidth        = 2
	paneChrome         = 4 // a border, the title and the blank row under it
	sidebarChrome      = 4
	titleIndent        = 1
)

// A rect is a region of the frame.
type rect struct {
	x, y, width, height int
}

// A layout places the frame's regions at one size, and sizes the emulator the
// pane holds: what the view draws and the job's terminal is sized to.
type layout struct {
	notice   rect
	sidebar  rect
	pane     rect
	help     rect
	listRows int
	paneCols int
	paneRows int
	margin   int
	gutter   int
	gap      int
}

// layoutAt lays a width × height frame out with noticeLines rows of notices:
// the help row at the bottom, the notices served before the air around the
// body, the list beside the pane while the pane stays wide enough to read.
func layoutAt(width, height, noticeLines int) layout {
	width, height = max(width, 0), max(height, 0)

	helpHeight := min(1, height)
	available := max(height-helpHeight, 0)
	noticeHeight := min(max(noticeLines, 0), max(available-minBodyRows, 0))
	rest := available - noticeHeight

	gaps := 0
	if rest-gapRows >= minBodyRows {
		gaps = gapRows
		rest -= gaps
	}

	margin, gutter := marginCols, gutterCols
	if width < minAiredCols || (fitsSidebar(width, 0, 0) && !fitsSidebar(width, margin, gutter)) {
		margin, gutter = 0, 0
	}
	inner := max(width-2*margin, 0)
	listWidth := 0
	if fitsSidebar(width, margin, gutter) {
		listWidth = sidebarWidth
	} else {
		gutter = 0
	}
	paneWidth := max(inner-listWidth-gutter, 0)
	bodyY := gaps/2 + noticeHeight

	return layout{
		notice:   rect{x: margin, y: gaps / 2, width: inner, height: noticeHeight},
		sidebar:  rect{x: margin, y: bodyY, width: listWidth, height: rest},
		pane:     rect{x: margin + listWidth + gutter, y: bodyY, width: paneWidth, height: rest},
		help:     rect{x: margin, y: max(height-1, 0), width: inner, height: helpHeight},
		listRows: max(rest-sidebarChrome, 0),
		paneCols: max(paneWidth-borderWidth, 0),
		paneRows: max(rest-paneChrome, 0),
		margin:   margin,
		gutter:   gutter,
		gap:      gaps / 2,
	}
}

func fitsSidebar(width, margin, gutter int) bool {
	return max(width-2*margin, 0)-sidebarWidth-gutter >= sidebarMinPaneCols
}

// scrollOffset is the first row of the list on screen: the one before, moved
// only as far as keeps the cursor in the visible rows.
func scrollOffset(cursor, total, visible, offset int) int {
	if visible <= 0 || total <= visible {
		return 0
	}
	cursor = min(max(cursor, 0), total-1)
	offset = min(max(offset, 0), total-visible)
	return max(min(offset, cursor), cursor-visible+1)
}
