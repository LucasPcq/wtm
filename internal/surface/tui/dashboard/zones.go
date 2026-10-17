package dashboard

import "strconv"

// Zone ids are the contract between what the renderer marks and what the mouse
// handler looks up; keeping them here is what makes a mis-mapped zone testable.
const (
	zoneTabPrefix       = "tab:"
	zoneRowPrefix       = "row:"
	zoneList            = "panel:list"
	zoneTree            = "panel:tree"
	zoneServices        = "panel:services"
	zoneServicesPfx     = "services:"
	zoneTreeRowPfx      = "tree:"
	zoneDetail          = "panel:detail"
	zoneDetailPR        = "detail:pr"
	zonePanelTabDtl     = "panel:tab:detail"
	zonePanelTabLogs    = "panel:tab:logs"
	zoneDetailRunPfx    = "detail:run:"
	zoneDetailURLPfx    = "detail:url:"
	zoneDetailFoldPfx   = "detail:fold:"
	zoneServicesFoldPfx = "services:fold:"
	zoneLogsHeldPfx     = "logs:held:"
	zoneLogsJobPfx      = "logs:job:"
	zoneLogsURL         = "logs:url"
	zoneServicesURL     = "services:url:"
	zoneOutput          = "panel:output"
	zoneOutputToggle    = "output:toggle"
	zoneAdd             = "header:add"
	zoneActions         = "header:actions"
	zoneMenuPrefix      = "menu:"
	zoneModalPrefix     = "modal:"
)

func menuZone(index int) string { return zoneMenuPrefix + strconv.Itoa(index) }

// modalRowZone keys a modal row by its index in the rows the modal drew, so a
// click answers the very row it landed on.
func modalRowZone(index int) string { return zoneModalPrefix + strconv.Itoa(index) }

func tabZone(index int) string { return zoneTabPrefix + strconv.Itoa(index) }

// treeRowZone keys a tree row by its index in the flattened forest, so a click
// resolves the same node at any scroll.
func treeRowZone(index int) string { return zoneTreeRowPfx + strconv.Itoa(index) }

// rowZone keys a row by its index in the full worktree slice, not by its
// position on screen, so a click resolves the same worktree at any scroll.
func rowZone(index int) string { return zoneRowPrefix + strconv.Itoa(index) }

// runRowZone keys a RUN row by its job name, not by its position: the row leads
// to that job whatever the section folded above it.
func runRowZone(job string) string { return zoneDetailRunPfx + job }

// servicesRowZone keys a block by its index in the board, so a click resolves the
// same worktree whatever started or stopped since the last frame.
func servicesRowZone(index int) string { return zoneServicesPfx + strconv.Itoa(index) }

// servicesFoldZone keys the marker that opens a runner's addresses on this tab.
// Its own zone, as in the detail panel: the rest of the row moves the cursor.
func servicesFoldZone(index int) string { return zoneServicesFoldPfx + strconv.Itoa(index) }

// runURLZone keys the address cell of a RUN row. It is a zone of its own inside
// the row's: clicking what you read opens what you read, and the rest of the row
// leads to the job's logs.
func runURLZone(job string) string { return zoneDetailURLPfx + job }

// runFoldZone keys the marker that opens and closes a runner's addresses. Its
// own zone rather than the whole row: the row already leads to the job's logs,
// and a runner is exactly the row one wants both gestures on.
func runFoldZone(job string) string { return zoneDetailFoldPfx + job }

// logsJobZone keys a line of the logs view's job column.
func logsJobZone(job string) string { return zoneLogsJobPfx + job }

// logsHeldZone keys one of the addresses a runner answers for, in the row that
// heads the logs view. Keyed by the child's name: the row is about the job on
// screen, so the runner is not in question.
func logsHeldZone(child string) string { return zoneLogsHeldPfx + child }

func servicesURLZone(index int) string { return zoneServicesURL + strconv.Itoa(index) }

// logsURLZone keys the address on the logs view's selection line. One at a
// time, so it needs no key of its own.
func logsURLZone() string { return zoneLogsURL }
