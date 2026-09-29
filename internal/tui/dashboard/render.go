package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// borderWidth and paddingWidth are what lipgloss adds around a panel's text; the
// rect a panel is given must account for both. buttonPadding is what the button
// style adds around its label.
const (
	borderWidth   = 2
	paddingWidth  = 2
	buttonPadding = 4
	// panelChromeRows is what renderPanel prepends to Body before drawing it:
	// the title row and the blank row under it.
	panelChromeRows = 2
)

// panelBodyHeight is the row budget renderPanel leaves for Body once its own
// border and chrome rows are accounted for.
func panelBodyHeight(rect domain.Rect) int {
	return max(rect.Height-borderWidth-panelChromeRows, 0)
}

// tabbedPanelBodyHeight is panelBodyHeight for a panel that heads itself: it
// passes no Title, so renderPanel prepends nothing, and what its own head costs
// is the tab bar rather than the title row.
func tabbedPanelBodyHeight(rect domain.Rect) int {
	return max(rect.Height-borderWidth-domain.DashboardPanelTabsChrome, 0)
}

type panelParams struct {
	Rect  domain.Rect
	Title string
	// TitleZone, when set, makes the whole title row its own clickable zone.
	TitleZone string
	// TitleRight is rendered flush right on the title row, under its own zone. It
	// is dropped whole when the panel is too narrow for it: trimming it would cut
	// through its marker and take the zone with it.
	TitleRight     string
	TitleRightZone string
	Body           []string
	Zone           string
}

// renderPanel draws a titled, bordered box filling Rect exactly and registers it
// as one mouse zone. Body is clipped to the rows the layout allotted.
func (m Model) renderPanel(params panelParams) string {
	textWidth := params.Rect.Width - borderWidth - paddingWidth
	contentHeight := params.Rect.Height - borderWidth
	if textWidth <= 0 || contentHeight <= 0 {
		return ""
	}

	// A panel whose head is its own first body line — the detail's tab bar —
	// passes no title, and no empty row is drawn in its place. It must not carry
	// title decorations either: there is no title row to hang them on, and
	// dropping them silently is how a zone goes missing without an error.
	if params.Title == "" && params.TitleRight == "" && params.TitleZone == "" {
		lines := clipRenderedLines(params.Body, contentHeight)
		box := styles.DashboardPanel.
			Width(params.Rect.Width - borderWidth).
			Height(contentHeight).
			Render(strings.Join(lines, "\n"))
		return m.marks().Mark(params.Zone, box)
	}

	title := styles.DashboardPanelTitle.Render(pad(truncate(params.Title, textWidth), textWidth))
	if rightWidth := lipgloss.Width(params.TitleRight); params.TitleRight != "" && rightWidth+1 < textWidth {
		left := styles.DashboardPanelTitle.Render(truncate(params.Title, textWidth-rightWidth-1))
		gap := textWidth - lipgloss.Width(left) - rightWidth
		title = left + strings.Repeat(" ", max(gap, 0)) + m.marks().Mark(params.TitleRightZone, params.TitleRight)
	}
	if params.TitleZone != "" {
		title = m.marks().Mark(params.TitleZone, title)
	}

	lines := clipRenderedLines(append([]string{title, ""}, params.Body...), contentHeight)

	box := styles.DashboardPanel.
		Width(params.Rect.Width - borderWidth).
		Height(contentHeight).
		Render(strings.Join(lines, "\n"))

	return m.marks().Mark(params.Zone, box)
}

// renderHeader dispatches to whichever header ComputeDashboardLayout chose
// for this frame — the six-row signature block, or the compact three-row
// fallback below domain.DashboardHeaderTallThreshold rows. It re-derives
// nothing: layout.HeaderTall is the one place that decision is made.
func (m Model) renderHeader(layout domain.DashboardLayout) string {
	// A terminal too short for any header is degenerate — nothing else fits
	// either — but it must not overflow. ComputeDashboardLayout already
	// shrinks Tabs.Height to what actually fits.
	if layout.Tabs.Height <= 0 {
		return ""
	}
	if layout.HeaderTall {
		return m.renderTallHeader(layout)
	}
	return m.renderCompactHeader(layout)
}

// No rule between the two lines: the tab rule underneath already separates the
// header from the body. Whole lines drop from the bottom under a tight budget,
// rather than emitting a fixed count that would push the frame past the screen.
func (m Model) renderCompactHeader(layout domain.DashboardLayout) string {
	context := m.renderContextLine(layout.Tabs.Width)
	if layout.Tabs.Height == 1 {
		return context
	}

	// The wordmark already opens the context line above; the bar starts
	// directly with the tabs so it is not drawn twice.
	bar, activeStart, activeWidth := m.renderTabBar(layout.Tabs.Width, m.headerRight)
	if layout.Tabs.Height == 2 {
		return lipgloss.JoinVertical(lipgloss.Left, context, bar)
	}

	return lipgloss.JoinVertical(lipgloss.Left, context, bar, tabRule(tabRuleParams{
		Width:       layout.Tabs.Width,
		ActiveStart: m.tabRuleStart(activeStart),
		ActiveWidth: activeWidth,
	}))
}

// The wordmark's three rows hold the context the compact header packs onto one
// line, so the block costs height it also uses. The blank line before the tabs
// is not optional — it is what makes this read as a header. No partial variant:
// the layout only selects this when all six rows fit.
func (m Model) renderTallHeader(layout domain.DashboardLayout) string {
	width := layout.Tabs.Width
	room := max(width-lipgloss.Width(styles.DashboardWordmark.Render(domain.DashboardWordmarkLines[0]))-lipgloss.Width(domain.DashboardWordmarkGap), 0)
	context := [3]string{
		m.headerRepoLine(),
		m.headerBaseActiveLine(),
		m.headerCountLine(room),
	}

	lines := make([]string, 0, domain.DashboardHeaderTallHeight)
	for index, art := range domain.DashboardWordmarkLines {
		lines = append(lines, m.wordmarkRow(art, context[index], width))
	}
	lines = append(lines, "")

	bar, activeStart, activeWidth := m.renderTabBar(width, m.headerButtons)
	lines = append(lines, bar, tabRule(tabRuleParams{
		Width:       width,
		ActiveStart: m.tabRuleStart(activeStart),
		ActiveWidth: activeWidth,
	}))
	return strings.Join(lines, "\n")
}

// Hard-truncated rather than dropped in segments: each row already carries a
// single fact, so there is nothing to drop before the wordmark itself.
func (m Model) wordmarkRow(art, styledContext string, width int) string {
	left := styles.DashboardWordmark.Render(art)
	if styledContext == "" {
		return truncateRendered(left, width)
	}
	return truncateRendered(left+domain.DashboardWordmarkGap+styledContext, width)
}

// headerRepoLine is the tall header's first row: the repository name alone,
// styled like the rest of the context text.
func (m Model) headerRepoLine() string {
	if m.repoName == "" {
		return ""
	}
	return styles.DashboardContext.Render(m.repoName)
}

// Built directly rather than through contextLeft, whose leading space belongs to
// the compact header's inline wordmark and would indent this row out of
// alignment with the repository name above it.
func (m Model) headerBaseActiveLine() string {
	segments := make([]string, 0, 2)
	if m.baseBranch() != "" {
		segments = append(segments, fmt.Sprintf(domain.DashboardBaseFmt, m.baseBranch()))
	}
	if m.activeBranch != "" {
		segments = append(segments, domain.DashboardActiveGlyph+" "+m.activeBranch)
	}
	if len(segments) == 0 {
		return ""
	}
	return styles.DashboardContext.Render(strings.Join(segments, domain.DashboardContextSep))
}

// Built from plain text rather than countLabel/fetchedLabel: those carry the
// compact header's own padding and styling, which would break this row's
// alignment with the two above it.
// The row is hard-truncated by wordmarkRow, so it degrades itself instead of
// being cut mid-word: fetched goes first (it comes back on `r`), then the count,
// leaving the version and its call to action — the one thing here the reader has
// to act on — standing longest.
func (m Model) headerCountLine(room int) string {
	version, action := rules.DashboardVersionSegments(rules.NewerVersionParams{
		Current: m.params.Version,
		Latest:  m.params.UpgradeLatest,
	})

	for _, variant := range []struct{ count, fetched, action bool }{
		{true, true, true},
		{true, false, true},
		{false, false, true},
		{true, true, false},
		{true, false, false},
		{false, false, false},
	} {
		line := m.countLineVariant(countLineParams{
			Count:      variant.count,
			Fetched:    variant.fetched,
			Version:    version,
			Action:     action,
			WithAction: variant.action,
		})
		if line == "" || lipgloss.Width(line) <= room {
			return line
		}
	}

	return ""
}

type countLineParams struct {
	Count      bool
	Fetched    bool
	Version    string
	Action     string
	WithAction bool
}

// The muted segments and the call to action are styled apart on purpose: an
// available upgrade has to read as actionable, not as one more context crumb —
// hence the signature accent it shares with the wordmark and the add button.
func (m Model) countLineVariant(params countLineParams) string {
	muted := make([]string, 0, 3)
	if params.Count {
		if count := m.countText(); count != "" {
			muted = append(muted, count)
		}
	}
	if params.Fetched {
		if fetched := m.fetchedText(); fetched != "" {
			muted = append(muted, fetched)
		}
	}
	if params.Version != "" {
		muted = append(muted, params.Version)
	}

	line := ""
	if len(muted) > 0 {
		line = styles.DashboardContext.Render(strings.Join(muted, domain.DashboardContextSep))
	}

	if !params.WithAction || params.Action == "" {
		return line
	}

	action := styles.DashboardUpgrade.Render(params.Action)
	if line == "" {
		return action
	}

	return line + styles.DashboardContext.Render(domain.DashboardContextSep) + action
}

// renderTabBar draws the tab row shared by both header variants: each tab
// gets its own zone, and right fills whatever room is left — headerRight
// (compact header, count included) or headerButtons (tall header, whose
// count already sits in the signature block).
func (m Model) renderTabBar(width int, right func(room int) string) (bar string, activeStart, activeWidth int) {
	rendered := make([]string, 0, len(tabs))
	used := 0
	for _, index := range m.shownTabs() {
		title := tabs[index]
		style := styles.DashboardTabInactive
		if index == m.tab {
			style = styles.DashboardTabActive
		}
		tab := style.Render(title)
		if used+lipgloss.Width(tab) > width {
			break
		}
		if index == m.tab {
			activeStart, activeWidth = used, lipgloss.Width(tab)
		}
		used += lipgloss.Width(tab)
		rendered = append(rendered, m.marks().Mark(tabZone(index), tab))
	}

	bar = lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if r := right(width - used); r != "" {
		bar += strings.Repeat(" ", width-used-lipgloss.Width(r)) + r
	}
	return bar, activeStart, activeWidth
}

// Segments drop whole, right to left, on the same variant-list mechanic
// headerRight uses: a half-drawn label reads as a wrong one, so a segment that
// does not fit is dropped rather than cut.
func (m Model) renderContextLine(width int) string {
	wordmark := styles.DashboardWordmark.Render(domain.DashboardWordmark)
	version, action := rules.DashboardVersionSegments(rules.NewerVersionParams{
		Current: m.params.Version,
		Latest:  m.params.UpgradeLatest,
	})

	// Same drop order as the tall header's third row, so the two variants of the
	// header agree on what matters: fetched, then the count-side context, and the
	// call to action outlives both.
	for _, variant := range []struct{ repo, base, active, fetched, action, version bool }{
		{true, true, true, true, true, true},
		{true, true, true, false, true, true},
		{true, true, false, false, true, true},
		{true, false, false, false, true, true},
		{true, true, true, true, false, true},
		{true, true, true, false, false, true},
		{true, true, false, false, false, true},
		{true, false, false, false, false, true},
		{true, false, false, false, false, false},
		{false, false, false, false, false, false},
	} {
		left := wordmark + m.contextLeft(contextLeftParams{Repo: variant.repo, Base: variant.base, Active: variant.active})
		right := m.countLineVariant(countLineParams{
			Fetched:    variant.fetched,
			Version:    versionIf(version, variant.version),
			Action:     action,
			WithAction: variant.action,
		})
		if line, ok := fitContextLine(left, right, width); ok {
			return line
		}
	}
	// The last variant above already tried the bare wordmark alone; if that
	// did not fit either, nothing will — the line goes empty rather than
	// overflow the width it was given.
	return ""
}

func versionIf(version string, keep bool) string {
	if !keep {
		return ""
	}
	return version
}

type contextLeftParams struct {
	Repo   bool
	Base   bool
	Active bool
}

// contextLeft builds the segments after the wordmark, joined by
// DashboardContextSep. A segment with no data to show (no repo name resolved,
// no base configured, cwd outside every known worktree) is skipped rather than
// printed empty.
func (m Model) contextLeft(params contextLeftParams) string {
	segments := make([]string, 0, 3)
	if params.Repo && m.repoName != "" {
		segments = append(segments, m.repoName)
	}
	if params.Base && m.baseBranch() != "" {
		segments = append(segments, fmt.Sprintf(domain.DashboardBaseFmt, m.baseBranch()))
	}
	if params.Active && m.activeBranch != "" {
		segments = append(segments, domain.DashboardActiveGlyph+" "+m.activeBranch)
	}
	if len(segments) == 0 {
		return ""
	}
	return styles.DashboardContext.Render(" " + strings.Join(segments, domain.DashboardContextSep))
}

// fetchedText is the header's only non-permanent element, in plain text: it
// appears only once the origin refs are stale enough to matter, and it is
// the view's property, not the selected worktree's — every origin badge in
// the list is equally old.
func (m Model) fetchedText() string {
	now := time.Now()
	if !rules.FetchIsStale(rules.FetchStalenessParams{FetchedAt: m.fetchedAt, Now: now}) {
		return ""
	}
	if m.fetchedAt.IsZero() {
		return domain.DashboardNeverFetched
	}
	age := rules.RelativeAge(rules.RelativeAgeParams{At: m.fetchedAt, Now: now})
	return fmt.Sprintf(domain.DashboardFetchedFmt, age)
}

// fitContextLine lays the left cluster and the right notice on one row of the
// given width, keeping both whole. It reports whether they fit at all rather
// than clipping either one.
func fitContextLine(left, right string, width int) (string, bool) {
	if right == "" {
		if lipgloss.Width(left) <= width {
			return left, true
		}
		return "", false
	}
	if lipgloss.Width(left)+1+lipgloss.Width(right) > width {
		return "", false
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	return left + strings.Repeat(" ", gap) + right, true
}

// headerRight is the bar's right cluster: the two global actions, then the count
// of what the active tab lists. Whole segments are dropped rather than the bar
// trimmed — a hard trim would cut through a zone marker and take that button's
// hit-testing with it.
func (m Model) headerRight(room int) string {
	count := m.countLabel()
	for _, variant := range []struct{ add, actions, count string }{
		{domain.DashboardAddLabelLong, domain.DashboardActionsLabel, count},
		{domain.DashboardAddLabel, domain.DashboardActionsLabel, count},
		{domain.DashboardAddLabel, domain.DashboardActionsShort, count},
		{domain.DashboardAddLabel, domain.DashboardActionsShort, ""},
		{"", domain.DashboardActionsShort, ""},
	} {
		add, actions := "", ""
		if variant.add != "" {
			add = m.marks().Mark(zoneAdd, styles.DashboardAddButton.Render(variant.add))
		}
		if variant.actions != "" {
			actions = m.marks().Mark(zoneActions, styles.DashboardHeaderButton.Render(variant.actions))
		}
		cluster := joinHeader(add, actions, variant.count)
		if lipgloss.Width(cluster)+1 <= room {
			return cluster
		}
	}
	return ""
}

// headerButtons is the tall header's tab-bar right cluster: the two global
// actions, no count — the signature block's own third row already carries
// it. Whole segments are dropped rather than the bar trimmed, the same
// reasoning headerRight follows.
func (m Model) headerButtons(room int) string {
	for _, variant := range []struct{ add, actions string }{
		{domain.DashboardAddLabelLong, domain.DashboardActionsLabel},
		{domain.DashboardAddLabel, domain.DashboardActionsLabel},
		{domain.DashboardAddLabel, domain.DashboardActionsShort},
		{"", domain.DashboardActionsShort},
	} {
		add, actions := "", ""
		if variant.add != "" {
			add = m.marks().Mark(zoneAdd, styles.DashboardAddButton.Render(variant.add))
		}
		if variant.actions != "" {
			actions = m.marks().Mark(zoneActions, styles.DashboardHeaderButton.Render(variant.actions))
		}
		cluster := joinHeader(add, actions)
		if lipgloss.Width(cluster)+1 <= room {
			return cluster
		}
	}
	return ""
}

func joinHeader(segments ...string) string {
	parts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			parts = append(parts, segment)
		}
	}
	return strings.Join(parts, " ")
}

// countText counts what the active tab lists, in plain text: worktrees, or
// the nodes of the forest — which includes the parents that have none.
func (m Model) countText() string {
	if m.tab == tabServices {
		up := 0
		for _, block := range m.servicesBlocks() {
			up += block.Up
		}
		return fmt.Sprintf(domain.DashboardServicesCountFmt, up)
	}
	if m.tab == tabTree {
		if !m.treeLoaded {
			return ""
		}
		format := domain.DashboardTreeCountFmt
		if len(m.treeRows) == 1 {
			format = domain.DashboardTreeCountOneFmt
		}
		return fmt.Sprintf(format, len(m.treeRows))
	}
	if !m.loaded {
		return ""
	}
	format := domain.DashboardCountFmt
	if len(m.statuses) == 1 {
		format = domain.DashboardCountOneFmt
	}
	return fmt.Sprintf(format, len(m.statuses))
}

// countLabel is countText styled for the compact header's tab-bar right
// cluster.
func (m Model) countLabel() string {
	text := m.countText()
	if text == "" {
		return ""
	}
	return styles.DashboardCount.Render(text)
}

// Both tab styles share the same padding, so the column does not depend on which
// tab is active — which is what lets the slide animation measure from and to it.
func (m Model) tabStart(width, index int) int {
	used := 0
	for _, i := range m.shownTabs() {
		w := lipgloss.Width(styles.DashboardTabInactive.Render(tabs[i]))
		if used+w > width {
			break
		}
		if i == index {
			return used
		}
		used += w
	}
	return used
}

// tabRuleStart interpolates the rule's start column while a slide is in
// progress (rules.TabSlideStart reports target outright once it either has
// not started or has finished): the caller always gets somewhere to draw,
// animating or not.
func (m Model) tabRuleStart(target int) int {
	return rules.TabSlideStart(rules.TabSlideParams{
		From:     m.tabSlideFrom,
		To:       target,
		Since:    m.tabSlideSince,
		Now:      time.Now(),
		Duration: domain.DashboardTabSlide,
	})
}

type tabRuleParams struct {
	Width       int
	ActiveStart int
	ActiveWidth int
}

// tabRule underlines the active tab and carries a quieter rule across the rest,
// which is what separates the header from the panels below it.
func tabRule(params tabRuleParams) string {
	if params.Width <= 0 {
		return ""
	}
	if params.ActiveWidth <= 0 {
		return styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.Width))
	}

	before := styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.ActiveStart))
	under := styles.DashboardTabRule.Render(strings.Repeat(domain.DashboardActiveRuleGlyph, params.ActiveWidth))
	rest := max(params.Width-params.ActiveStart-params.ActiveWidth, 0)
	return before + under + styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, rest))
}

func (m Model) renderHelpBar(layout domain.DashboardLayout) string {
	if layout.Help.Height <= 0 {
		return ""
	}
	hint := domain.DashboardHelpWide
	switch {
	case m.loadErr != nil:
		hint = m.loadErr.Error()
	case m.logsOpen():
		hint = domain.DashboardLogsHint
	case m.tab == tabTree:
		hint = domain.DashboardHelpTree
	case m.tab == tabServices:
		hint = domain.DashboardHelpServices
	case layout.Narrow && m.detailOpen:
		hint = domain.DashboardHelpDetail
	case layout.Narrow:
		hint = domain.DashboardHelpNarrow
	}
	return styles.DashboardHelp.Render(truncate(hint, max(layout.Help.Width-paddingWidth, 0)))
}

// A body is assembled by counting entries as rows, and an entry carrying its own
// newlines renders as several — which is how a hook or git failure pasted in
// verbatim pushes the frame past the terminal and scrolls the alt screen. This
// clips by rendered rows instead, whatever a caller puts in Body. Entries are
// kept or dropped whole so a styled one is never cut mid-escape; only an entry
// that alone overflows is split.
func clipRenderedLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	kept := make([]string, 0, len(lines))
	used := 0
	for _, entry := range lines {
		rows := strings.Count(entry, "\n") + 1
		if used+rows <= height {
			kept = append(kept, entry)
			used += rows
			continue
		}
		if used == 0 {
			split := strings.SplitN(entry, "\n", height+1)
			return append(kept, strings.Join(split[:height], "\n"))
		}
		break
	}
	return kept
}

// truncate clips plain text to a display width, marking the cut with an ellipsis.
func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// truncateRendered hard-trims styled text. It must never see marked content: a
// cut through a zone marker silently breaks that zone's bounds.
func truncateRendered(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	return styles.DashboardClip.MaxWidth(width).Render(text)
}

func pad(text string, width int) string {
	if gap := width - lipgloss.Width(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

// spread lays a left and a right segment on one row of the given width, keeping
// the right one whole and clipping the left when they collide. A width leaving
// the left segment no room at all drops it rather than letting it through:
// truncateRendered(left, 0) returns the text whole, and a line wider than its
// panel is re-wrapped by lipgloss and breaks the frame.
func spread(left, right string, width int) string {
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return truncateRendered(right, width)
	}
	budget := width - rightWidth - 1
	if budget <= 0 {
		return pad(truncateRendered(right, width), width)
	}
	left = truncateRendered(left, budget)
	gap := width - lipgloss.Width(left) - rightWidth
	return left + strings.Repeat(" ", max(gap, 0)) + right
}
