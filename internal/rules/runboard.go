package rules

import (
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type RunBoardParams struct {
	Config domain.RunConfig
	Jobs   []domain.JobInfo
	// Addresses is branch → job → where it answers, and Notes what has to be
	// said about them, by branch.
	Addresses map[string]map[string]domain.JobAddress
	Notes     map[string]string
	// Expanded keys the runners whose held addresses are unfolded, by job name.
	Expanded map[string]bool
	Statuses []domain.WorktreeStatus
	Now      time.Time
}

type RunWorktreeBlock struct {
	Branch string
	Path   string
	Up     int
	Rows   []domain.DetailRow
	// Note is what has to be said about this worktree's addresses, empty when
	// its .env answers on what its jobs publish.
	Note string
}

// RunBoard is what the daemon holds up, worktree by worktree. This board
// answers "what is running" and nothing else: a worktree with nothing up has no
// block, and a job that is not up has no row. It is the one surface that is
// purely about the present — the detail panel keeps this session's stopped and
// crashed jobs, and the logs view is where an archive is read.
func RunBoard(params RunBoardParams) []RunWorktreeBlock {
	blocks := make([]RunWorktreeBlock, 0, len(params.Statuses))
	for _, status := range params.Statuses {
		indexed := IndexedJobsByName(params.Jobs, status.Path)
		addresses := params.Addresses[status.Branch]

		rows := make([]domain.DetailRow, 0, len(indexed))
		for _, job := range params.Config.Jobs {
			info, held := indexed[job.Name]
			if !held || !IsJobUp(info.Status) {
				continue
			}
			rows = append(rows, jobRow(jobRowParams{
				Visible:  VisibleJob{Job: job, State: JobStateUp, Info: info},
				Address:  addresses[job.Name],
				Expanded: params.Expanded[job.Name],
				Now:      params.Now,
			}))
			if params.Expanded[job.Name] {
				rows = append(rows, heldRows(job.Name, addresses[job.Name])...)
			}
		}
		if len(rows) == 0 {
			continue
		}
		blocks = append(blocks, RunWorktreeBlock{
			Branch: status.Branch, Path: status.Path, Up: countUpRows(rows), Rows: rows,
			Note: params.Notes[status.Branch],
		})
	}
	return blocks
}

// countUpRows counts the jobs, not the rows: a runner's unfolded children are
// addresses of one process, and counting them would inflate what is running.
func countUpRows(rows []domain.DetailRow) int {
	up := 0
	for _, row := range rows {
		if row.Up {
			up++
		}
	}
	return up
}

// ServicesRows flattens the board into the lines the Services tab draws. Every
// row carries its worktree, header or not: the menu acts on the worktree of the
// job under the cursor, and a job row that could not name it would send the
// action somewhere else.
func ServicesRows(blocks []RunWorktreeBlock) []domain.ServicesRow {
	rows := make([]domain.ServicesRow, 0, len(blocks)*3)
	for index, block := range blocks {
		if index > 0 {
			rows = append(rows, domain.ServicesRow{Kind: domain.ServicesRowGap})
		}
		rows = append(rows, domain.ServicesRow{
			Kind: domain.ServicesRowHeader, Branch: block.Branch, Path: block.Path, Up: block.Up,
		})
		for _, job := range block.Rows {
			kind := domain.ServicesRowJob
			if job.Depth > 0 {
				kind = domain.ServicesRowHeld
			}
			rows = append(rows, domain.ServicesRow{
				Kind: kind, Branch: block.Branch, Path: block.Path, Job: job,
			})
		}
		if block.Note != "" {
			rows = append(rows,
				domain.ServicesRow{Kind: domain.ServicesRowGap},
				domain.ServicesRow{
					Kind: domain.ServicesRowNote, Branch: block.Branch, Path: block.Path, Note: block.Note,
				},
			)
		}
	}
	return rows
}

// RunningWorktreeDirs are the worktrees a board holds something up in, as git
// spells them. It is what a stop or a view over several arrives ticked with:
// those gestures are about what is standing, not about where you are.
func RunningWorktreeDirs(blocks []RunWorktreeBlock) []string {
	dirs := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Path == "" {
			continue
		}
		dirs = append(dirs, block.Path)
	}
	return dirs
}
