package runview

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// sidebarRow is one line of the job list. Above several worktrees the list is
// two levels deep — a heading, then that worktree's jobs — rather than flat
// with a worktree column: the sidebar is narrow, and a column would be the
// first thing truncated. A Spacer sets one group off from the next; otherwise
// exactly one of Header and View is set. Shared marks the heading of the
// shared services, listed once above every worktree.
type sidebarRow struct {
	Spacer bool
	Header string
	Shared bool
	View   runlogs.JobView
}

// rows is the job list as the sidebar draws it: the shared services first, one
// row each however many worktrees hold them, then every worktree's own jobs.
// Headings are not selectable, and a single worktree gets one too: the list says
// which worktree it is about whatever its arity, and a shape that changes
// between one and two is a shape the reader has to learn twice.
func (m Model) rows() []sidebarRow {
	visible := m.visible()
	rows := make([]sidebarRow, 0, len(visible)+4)
	current, opened := "", false
	open := func(header sidebarRow) {
		// A blank line before every group but the first: two headings with only
		// their jobs between them read as one list, which is what the second
		// level exists to stop.
		if opened {
			rows = append(rows, sidebarRow{Spacer: true})
		}
		opened = true
		rows = append(rows, header)
	}
	for _, view := range visible {
		if host := sharedHostOf(view); host != "" {
			if current != sharedGroup+host {
				current = sharedGroup + host
				open(sidebarRow{Header: fmt.Sprintf(domain.RunViewSharedRowFmt, host), Shared: true})
			}
			rows = append(rows, sidebarRow{View: view})
			continue
		}
		heading := headingOf(view)
		// A worktree git cannot name gets no heading rather than a blank one: an
		// empty row says less than the job it pushed off the panel.
		if heading != "" && view.WorkDir != current {
			current = view.WorkDir
			open(sidebarRow{Header: heading})
		}
		rows = append(rows, sidebarRow{View: view})
	}
	return rows
}

// sharedGroup keeps the shared group's key apart from any work dir.
const sharedGroup = "\x00shared:"

// sharedHostOf is the worktree a shared service runs in, empty for a job that
// is not one or is not up: a stopped shared job is only this worktree's history.
func sharedHostOf(view runlogs.JobView) string {
	if !rules.IsJobUp(view.Status) {
		return ""
	}
	if view.SharedIn != "" {
		return view.SharedIn
	}
	if view.Shared {
		return headingOf(view)
	}
	return ""
}

// headingOf falls back to the path for a worktree git could not name, and to
// nothing at all for a board that names neither.
func headingOf(view runlogs.JobView) string {
	if view.Worktree != "" {
		return view.Worktree
	}
	return view.WorkDir
}

// listViewport is how many rows the list itself may fill. A grouped list keeps
// one back for the heading it pins when it scrolls: counting that row as
// available is what let the cursor slide out from under the panel it was
// measured to fit in.
func (m Model) listViewport() int {
	if !m.grouped() {
		return m.layout().SidebarRows
	}
	return max(m.layout().SidebarRows-1, 1)
}

// grouped reports that the list carries headings at all, which a board naming
// no worktree does not. It is what the indent and the pinned heading hang on —
// both cost a row or a column that an ungrouped list has no use for.
func (m Model) grouped() bool {
	for _, view := range m.jobs {
		if headingOf(view) != "" || sharedHostOf(view) != "" {
			return true
		}
	}
	return false
}

// stickyHeader is the heading of the group the list opens in the middle of. A
// long enough list scrolls its headings away, and the rows left at the top then
// belong to a worktree nothing on screen names — the very ambiguity the second
// level exists to remove. Not found when the group's own heading is already
// visible.
func stickyHeader(rows []sidebarRow, offset int) (sidebarRow, bool) {
	if offset <= 0 || offset >= len(rows) {
		return sidebarRow{}, false
	}
	if rows[offset].Spacer || rows[offset].Header != "" {
		return sidebarRow{}, false
	}
	for index := offset - 1; index >= 0; index-- {
		if rows[index].Header != "" {
			return rows[index], true
		}
	}
	return sidebarRow{}, false
}

// selectedRowIndex places the cursor among the rows, headings included, which
// is what the sidebar's scroll offset is measured against.
func selectedRowIndex(rows []sidebarRow, selected jobKey) int {
	for index, row := range rows {
		if row.Spacer || row.Header != "" {
			continue
		}
		if viewKey(row.View) == selected {
			return index
		}
	}
	return 0
}

func (m Model) indent() string {
	if !m.grouped() {
		return ""
	}
	return " "
}
