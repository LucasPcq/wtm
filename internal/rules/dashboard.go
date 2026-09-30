package rules

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type DashboardLayoutParams struct {
	Width          int
	Height         int
	OutputExpanded bool
	// DetailOpen only matters under DashboardNarrowWidth, where the detail takes
	// the list's place instead of sitting beside it.
	DetailOpen bool
	// FullBody is asked by a tab whose content needs the width — the Running
	// tab, whose whole point is that the addresses are readable. The detail
	// then does not show: there is nothing beside the main panel.
	FullBody bool
}

// minDashboardBody keeps the list readable when the terminal is short: the
// output panel gives up its rows before the body does.
const minDashboardBody = 3

// DashboardHeaderHeight is the header's row budget for a terminal this tall:
// the six-row signature block (domain.DashboardHeaderTallHeight) above
// domain.DashboardHeaderTallThreshold rows, the compact three-row header
// (domain.DashboardHeaderCompactHeight) below it — six rows of chrome on a
// short terminal is a quarter of the screen. This is the one place that
// makes the call; ComputeDashboardLayout reads it, the renderer reads the
// layout's own HeaderTall rather than re-deriving the choice.
func DashboardHeaderHeight(height int) int {
	if height >= domain.DashboardHeaderTallThreshold {
		return domain.DashboardHeaderTallHeight
	}
	return domain.DashboardHeaderCompactHeight
}

// ComputeDashboardLayout places every dashboard panel for one frame. It is the
// single reference the renderer draws from and the mouse zones are marked
// against, so a panel cannot drift from the region it is clickable in.
func ComputeDashboardLayout(params DashboardLayoutParams) domain.DashboardLayout {
	width, height := max(params.Width, 0), max(params.Height, 0)

	// helpHeight is reserved first: on a terminal too short for the header
	// alone, the header gives up rows to it rather than the two overlapping —
	// which is what let the header silently overflow before it deferred to
	// this budget.
	helpHeight := min(1, max(height-1, 0))
	headerHeight := DashboardHeaderHeight(height)
	tabsHeight := min(headerHeight, max(height-helpHeight, 0))
	tabs := domain.Rect{X: 0, Y: 0, Width: width, Height: tabsHeight}
	help := domain.Rect{X: 0, Y: max(height-1, 0), Width: width, Height: helpHeight}

	outputHeight := domain.DashboardChromeHeight
	if params.OutputExpanded {
		outputHeight += domain.DashboardTitleGap + domain.DashboardOutputBodyHeight
	}
	available := height - tabs.Height - help.Height
	if available-outputHeight < minDashboardBody {
		outputHeight = available - minDashboardBody
	}
	outputHeight = max(outputHeight, 0)
	bodyHeight := max(available-outputHeight, 0)

	layout := domain.DashboardLayout{
		Narrow:      width < domain.DashboardNarrowWidth,
		HeaderTall:  headerHeight == domain.DashboardHeaderTallHeight,
		Tabs:        tabs,
		Help:        help,
		Output:      domain.Rect{X: 0, Y: tabs.Height + bodyHeight, Width: width, Height: outputHeight},
		OutputLines: max(outputHeight-domain.DashboardChromeHeight-domain.DashboardTitleGap, 0),
	}

	body := domain.Rect{X: 0, Y: tabs.Height, Width: width, Height: bodyHeight}
	if params.FullBody {
		layout.List, layout.ListVisible = body, true
		layout.ListRows = dashboardListRows(body.Height)
		layout.TreeRows = dashboardTreeRows(body.Height)
		layout.ServicesRows = dashboardServicesRows(body.Height)
		return layout
	}
	if layout.Narrow {
		if params.DetailOpen {
			layout.Detail, layout.DetailVisible = body, true
			return layout
		}
		layout.List, layout.ListVisible = body, true
		layout.ListRows = dashboardListRows(body.Height)
		layout.TreeRows = dashboardTreeRows(body.Height)
		layout.ServicesRows = dashboardServicesRows(body.Height)
		return layout
	}

	listWidth := dashboardListWidth(width)
	layout.List = domain.Rect{X: 0, Y: body.Y, Width: listWidth, Height: body.Height}
	layout.Detail = domain.Rect{X: listWidth, Y: body.Y, Width: width - listWidth, Height: body.Height}
	layout.ListVisible, layout.DetailVisible = true, true
	layout.ListRows = dashboardListRows(body.Height)
	layout.TreeRows = dashboardTreeRows(body.Height)
	layout.ServicesRows = dashboardServicesRows(body.Height)
	return layout
}

// dashboardListRows is how many worktrees fit in a list body, each taking its
// own lines plus the gap that separates it from the next one.
func dashboardListRows(bodyHeight int) int {
	available := bodyHeight - domain.DashboardChromeHeight - domain.DashboardTitleGap
	if available < domain.DashboardRowHeight {
		return 0
	}
	return (available + domain.DashboardRowGap) / (domain.DashboardRowHeight + domain.DashboardRowGap)
}

// dashboardTreeRows is how many tree nodes fit in the same body: one line each,
// with no gap, so the connector gutters line up down the panel.
func dashboardTreeRows(bodyHeight int) int {
	return max(bodyHeight-domain.DashboardChromeHeight-domain.DashboardTitleGap, 0) / domain.DashboardTreeRowHeight
}

// dashboardServicesRows is how many Services lines fit: one line each, unlike a
// tree node, which carries the spacer under it.
func dashboardServicesRows(bodyHeight int) int {
	return max(bodyHeight-domain.DashboardChromeHeight-domain.DashboardTitleGap, 0)
}

func dashboardListWidth(width int) int {
	if width < domain.DashboardMinListWidth+domain.DashboardMinDetailWidth {
		return width / 2
	}
	listWidth := width * domain.DashboardListWidthPercent / 100
	listWidth = max(listWidth, domain.DashboardMinListWidth)
	return min(listWidth, width-domain.DashboardMinDetailWidth)
}

type ModalRectParams struct {
	ScreenWidth  int
	ScreenHeight int
	// ContentHeight is how many body lines the modal wants to show; the rect adds
	// the border rows and clamps the whole box to the screen.
	ContentHeight int
}

// ComputeModalRect places a modal box over the dashboard. Like
// ComputeDashboardLayout it is the single reference: the renderer draws the box
// there and its rows are clickable there.
func ComputeModalRect(params ModalRectParams) domain.Rect {
	screenWidth, screenHeight := max(params.ScreenWidth, 0), max(params.ScreenHeight, 0)
	return CenterRect(CenterRectParams{
		Width:        ModalWidth(screenWidth),
		Height:       max(params.ContentHeight, 1) + domain.DashboardModalChrome,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	})
}

type CenterRectParams struct {
	Width        int
	Height       int
	ScreenWidth  int
	ScreenHeight int
}

// CenterRect places a box of a known size in the middle of the screen, clamped
// to it — for an overlay that sizes itself on its content.
func CenterRect(params CenterRectParams) domain.Rect {
	width := max(min(params.Width, params.ScreenWidth), 0)
	height := max(min(params.Height, params.ScreenHeight), 0)
	return domain.Rect{
		X:      max((params.ScreenWidth-width)/2, 0),
		Y:      max((params.ScreenHeight-height)/2, 0),
		Width:  width,
		Height: height,
	}
}

type MenuRectParams struct {
	// AnchorX and AnchorY are the cell the menu was opened from: the click, or
	// the row the keyboard is on.
	AnchorX int
	AnchorY int
	// Width and Height are the box's outer size, borders included.
	Width        int
	Height       int
	ScreenWidth  int
	ScreenHeight int
}

// ComputeMenuRect places a context menu next to the cell it was opened from. It
// hangs below the anchor and flips above it rather than running off the bottom,
// which is what every other context menu does — and, like the panels, it is the
// one reference the renderer draws from and the entries are clickable in.
func ComputeMenuRect(params MenuRectParams) domain.Rect {
	width := min(params.Width, params.ScreenWidth)
	height := min(params.Height, params.ScreenHeight)

	x := max(min(params.AnchorX, params.ScreenWidth-width), 0)

	y := params.AnchorY + 1
	if y+height > params.ScreenHeight {
		y = params.AnchorY - height
	}
	y = max(min(y, params.ScreenHeight-height), 0)

	return domain.Rect{X: x, Y: y, Width: max(width, 0), Height: max(height, 0)}
}

// ModalWidth is the width a modal gets on a screen, before its content is known:
// its widgets need it to lay themselves out.
func ModalWidth(screenWidth int) int {
	width := screenWidth * domain.DashboardModalWidthPercent / 100
	width = max(width, min(domain.DashboardModalMinWidth, screenWidth))
	return min(width, domain.DashboardModalMaxWidth, screenWidth)
}

type DashboardScrollParams struct {
	Cursor  int
	Total   int
	Visible int
	Offset  int
}

// DashboardScrollOffset returns the first visible row keeping Cursor on screen.
func DashboardScrollOffset(params DashboardScrollParams) int {
	if params.Visible <= 0 || params.Total <= params.Visible {
		return 0
	}
	cursor := ClampIndex(params.Cursor, params.Total)
	offset := DashboardClampOffset(DashboardOffsetParams{
		Offset:  params.Offset,
		Total:   params.Total,
		Visible: params.Visible,
	})
	return max(min(offset, cursor), cursor-params.Visible+1)
}

type DashboardOffsetParams struct {
	Offset  int
	Total   int
	Visible int
}

// DashboardClampOffset keeps a free scroll offset — one with no cursor to follow,
// as in the output panel — inside the scrollable range.
func DashboardClampOffset(params DashboardOffsetParams) int {
	if params.Visible <= 0 || params.Total <= params.Visible {
		return 0
	}
	return min(max(params.Offset, 0), params.Total-params.Visible)
}

// ClampIndex keeps an index inside [0, count-1], and returns 0 for an empty set.
func ClampIndex(index, count int) int {
	if count <= 0 {
		return 0
	}
	return min(max(index, 0), count-1)
}

// SplitRecapField reads a recap line written as "Label:" followed by spaces and a
// value, and reports its two halves so a renderer can weight them differently.
// Anything else is left whole: a heading ("Will delete:"), prose that happens to
// contain a colon, and "Label:value" with no gap — the run of spaces is what
// makes a line a field rather than a sentence.
func SplitRecapField(line string) (label, value string, ok bool) {
	colon := strings.Index(line, ":")
	if colon <= 0 || colon == len(line)-1 {
		return "", "", false
	}
	rest := line[colon+1:]
	trimmed := strings.TrimLeft(rest, " ")
	if trimmed == "" || len(trimmed) == len(rest) {
		return "", "", false
	}
	return line[:colon+1] + rest[:len(rest)-len(trimmed)], trimmed, true
}

type ActiveWorktreeParams struct {
	Cwd      string
	Statuses []domain.WorktreeStatus
}

// ActiveWorktree nomme la branche du worktree qui contient Cwd. Quand deux
// worktrees s'emboîtent, le plus profond l'emporte : c'est celui dans lequel on
// travaille réellement.
func ActiveWorktree(params ActiveWorktreeParams) string {
	if params.Cwd == "" {
		return ""
	}

	branch, deepest := "", 0
	for _, status := range params.Statuses {
		if status.Path == "" || !underPath(params.Cwd, status.Path) {
			continue
		}
		if length := len(status.Path); length > deepest {
			branch, deepest = status.Branch, length
		}
	}
	return branch
}

// underPath tests containment on a segment boundary, so that /a/bc is not taken
// for a child of /a/b.
func underPath(cwd, root string) bool {
	if cwd == root {
		return true
	}
	return strings.HasPrefix(cwd, root+string(filepath.Separator))
}

// AnimationsEnabled: an absent key means on. Only an explicit false in
// ui.animations turns every dashboard animation off at once.
func AnimationsEnabled(cfg domain.Config) bool {
	if cfg.Global.UI.Animations == nil {
		return true
	}
	return *cfg.Global.UI.Animations
}

type TabSlideParams struct {
	From, To int
	Since    time.Time
	Now      time.Time
	Duration time.Duration
}

// TabSlideStart interpolates the tab rule's start column between From and To
// over Duration. Before the slide began (Since is zero) or once Duration has
// elapsed, it reports To outright — the animation is over either way.
func TabSlideStart(params TabSlideParams) int {
	if params.Duration <= 0 || params.Since.IsZero() {
		return params.To
	}
	elapsed := params.Now.Sub(params.Since)
	if elapsed >= params.Duration {
		return params.To
	}
	if elapsed <= 0 {
		return params.From
	}
	fraction := float64(elapsed) / float64(params.Duration)
	return params.From + int(float64(params.To-params.From)*fraction)
}

type FlashParams struct {
	Since    time.Time
	Now      time.Time
	Duration time.Duration
}

// FlashLit reports whether a just-created row is still in the lit half of its
// one-shot flash — the bright opening beat before it fades back into the
// ordinary selected look.
func FlashLit(params FlashParams) bool {
	if params.Duration <= 0 || params.Since.IsZero() {
		return false
	}
	elapsed := params.Now.Sub(params.Since)
	return elapsed >= 0 && elapsed < params.Duration/2
}
