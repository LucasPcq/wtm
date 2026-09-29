package rules

import (
	"slices"
	"unicode/utf8"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HelpSections is the dashboard's key and mouse reference, in the order the
// overlay lays it out (row-major). The RUN section is left out of a project that
// runs no jobs, whose dashboard draws none of what those keys reach.
func HelpSections(runModule bool) []domain.HelpSection {
	sections := []domain.HelpSection{
		{Title: domain.DashboardHelpSectionNav, Entries: []domain.HelpEntry{
			{Keys: domain.DashboardHelpKeysSelect, Text: domain.DashboardHelpTextSelect},
			{Keys: domain.DashboardHelpKeysEnds, Text: domain.DashboardHelpTextEnds},
			{Keys: domain.DashboardHelpKeysPage, Text: domain.DashboardHelpTextPage},
			{Keys: domain.DashboardHelpKeysTab, Text: domain.DashboardHelpTextTab},
			{Keys: domain.DashboardHelpKeysOpenDetail, Text: domain.DashboardHelpTextOpenDetail},
			{Keys: domain.DashboardHelpKeysCloseDetail, Text: domain.DashboardHelpTextCloseDetail},
		}},
		{Title: domain.DashboardHelpSectionAct, Entries: []domain.HelpEntry{
			{Keys: domain.KeyNew, Text: domain.DashboardHelpTextNew},
			{Keys: domain.KeyMenu, Text: domain.DashboardHelpTextMenu},
			{Keys: domain.KeyActions, Text: domain.DashboardHelpTextActions},
			{Keys: domain.KeyFastForward, Text: domain.DashboardHelpTextFastForward},
			{Keys: domain.KeyOpenPR, Text: domain.DashboardHelpTextOpenPR},
		}},
		{Title: domain.DashboardHelpSectionMouse, Entries: []domain.HelpEntry{
			{Keys: domain.DashboardHelpKeysClick, Text: domain.DashboardHelpTextClick},
			{Keys: domain.DashboardHelpKeysRightClick, Text: domain.DashboardHelpTextRightClick},
			{Keys: domain.DashboardHelpKeysWheel, Text: domain.DashboardHelpTextWheel},
		}},
		{Title: domain.DashboardHelpSectionView, Entries: []domain.HelpEntry{
			{Keys: domain.KeyToggleOutput, Text: domain.DashboardHelpTextOutput},
			{Keys: domain.DashboardHelpKeysOutputMove, Text: domain.DashboardHelpTextOutputMove},
			{Keys: domain.KeyRefresh, Text: domain.DashboardHelpTextRefresh},
		}},
	}
	if !runModule {
		return sections
	}
	run := domain.HelpSection{Title: domain.DashboardHelpSectionRun, Entries: []domain.HelpEntry{
		{Keys: domain.KeyRunLogs, Text: domain.DashboardHelpTextRunLogs},
		{Keys: domain.DashboardHelpKeysJobSwitch, Text: domain.DashboardHelpTextJobSwitch},
		{Keys: domain.KeyOpenAddress, Text: domain.DashboardHelpTextOpenAddress},
	}}
	return slices.Insert(sections, runSectionAt, run)
}

// runSectionAt keeps RUN where it always sat, right after ACT.
const runSectionAt = 2

type HelpLayoutParams struct {
	Sections     []domain.HelpSection
	ScreenWidth  int
	ScreenHeight int
}

// ComputeHelpLayout resolves the reference overlay's geometry. Two columns are
// taken only when both fit at their natural width: a column squeezed into place
// would truncate the very text it exists to make readable.
func ComputeHelpLayout(params HelpLayoutParams) domain.HelpLayout {
	available := params.ScreenWidth - domain.DashboardHelpFrame
	if available <= 0 || len(params.Sections) == 0 {
		return domain.HelpLayout{}
	}

	layout := helpArrangement(params.Sections, 2)
	if layout.Inner > available {
		layout = helpArrangement(params.Sections, 1)
	}
	fitHelpWidth(&layout, available)

	budget := max(params.ScreenHeight-domain.DashboardHelpChrome-domain.DashboardModalChrome, 0)
	layout.BodyRows = min(layout.ContentRows, budget)
	layout.Scrollable = layout.BodyRows < layout.ContentRows
	return layout
}

// helpArrangement bands the sections into the given number of columns and sizes
// each column on what that column carries.
func helpArrangement(sections []domain.HelpSection, columns int) domain.HelpLayout {
	layout := domain.HelpLayout{
		Bands:     helpBands(sections, columns),
		KeyWidth:  make([]int, columns),
		TextWidth: make([]int, columns),
	}

	for _, band := range layout.Bands {
		for column, section := range band {
			for _, entry := range section.Entries {
				layout.KeyWidth[column] = max(layout.KeyWidth[column], textWidth(entry.Keys))
				layout.TextWidth[column] = max(layout.TextWidth[column], textWidth(entry.Text))
			}
		}
	}

	layout.ColumnWidth = make([]int, columns)
	for column := range columns {
		layout.KeyWidth[column] += domain.DashboardHelpKeyGap
		layout.ColumnWidth[column] = helpColumnWidth(layout, column)
		layout.Inner += layout.ColumnWidth[column]
	}
	layout.Inner += domain.DashboardHelpColumnGap * (columns - 1)
	layout.ContentRows = helpContentRows(layout.Bands)
	return layout
}

func helpColumnWidth(layout domain.HelpLayout, column int) int {
	width := layout.KeyWidth[column] + layout.TextWidth[column]
	for _, band := range layout.Bands {
		if column < len(band) {
			width = max(width, textWidth(band[column].Title))
		}
	}
	return width
}

// fitHelpWidth settles the box on a width the screen holds and the two lines it
// always draws fill, then gives the difference to the last column: every other
// column keeps the width its own content asked for.
func fitHelpWidth(layout *domain.HelpLayout, available int) {
	layout.Inner = min(max(layout.Inner, helpFloorWidth()), available)

	last := len(layout.ColumnWidth) - 1
	gaps := domain.DashboardHelpColumnGap * last
	for column := range last {
		gaps += layout.ColumnWidth[column]
	}
	layout.ColumnWidth[last] = max(layout.Inner-gaps, 0)
}

func helpBands(sections []domain.HelpSection, columns int) []domain.HelpBand {
	bands := make([]domain.HelpBand, 0, (len(sections)+columns-1)/columns)
	for start := 0; start < len(sections); start += columns {
		bands = append(bands, domain.HelpBand(sections[start:min(start+columns, len(sections))]))
	}
	return bands
}

// helpContentRows counts the body lines the bands render to: a title and its
// rule per band, the tallest section's entries, and a blank between bands.
func helpContentRows(bands []domain.HelpBand) int {
	rows := 0
	for index, band := range bands {
		tallest := 0
		for _, section := range band {
			tallest = max(tallest, len(section.Entries))
		}
		rows += helpSectionChrome + tallest
		if index < len(bands)-1 {
			rows++
		}
	}
	return rows
}

// helpSectionChrome is what a band spends above its entries: the section title
// and the rule cutting it off from them.
const helpSectionChrome = 2

// helpFloorWidth keeps the box wide enough for the two lines it always draws,
// however narrow its entries are.
func helpFloorWidth() int {
	return max(textWidth(domain.DashboardHelpTitle), textWidth(domain.DashboardHelpHintScroll))
}

func textWidth(text string) int { return utf8.RuneCountInString(text) }
