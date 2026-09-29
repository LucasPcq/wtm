package rules

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PortClaimsParams struct {
	Jobs    []domain.JobConfig
	WorkDir string
	Offset  int
}

// PortClaims are the ports these jobs bind in one worktree. A shared job is
// left out: it runs once for the repository, and every worktree attaching to it
// is the point rather than a clash.
func PortClaims(params PortClaimsParams) []domain.PortClaim {
	var claims []domain.PortClaim
	for _, job := range params.Jobs {
		if IsShared(job) {
			continue
		}
		for _, port := range JobPorts(JobPortsParams{Ports: job.Ports, PortOffset: params.Offset, Scope: job.Scope}) {
			claims = append(claims, domain.PortClaim{Port: port, Job: job.Name, WorkDir: params.WorkDir})
		}
	}
	return claims
}

type PortClashesParams struct {
	Starting []domain.PortClaim
	// Held are the ports jobs already up bind, in worktrees this run leaves
	// alone.
	Held []domain.PortClaim
}

// PortClashes finds the ports this run would bind that a job in another
// worktree already holds — which only happens where two worktrees share an
// offset, a verbatim one and its source above all. Two claims in the same
// worktree are not a clash here: a runner and its children declare the same
// port, and StartConflicts already refuses the ones that do collide.
func PortClashes(params PortClashesParams) []domain.PortClash {
	held := make(map[int]domain.PortClaim, len(params.Held))
	for _, claim := range params.Held {
		held[claim.Port] = claim
	}

	seen := map[int]bool{}
	var clashes []domain.PortClash
	for _, claim := range params.Starting {
		if seen[claim.Port] {
			continue
		}
		holder, taken := held[claim.Port]
		if !taken || holder.WorkDir == claim.WorkDir {
			continue
		}
		seen[claim.Port] = true
		clashes = append(clashes, domain.PortClash{Port: claim.Port, Want: claim, HeldBy: holder})
	}
	sort.Slice(clashes, func(i, j int) bool { return clashes[i].Port < clashes[j].Port })
	return clashes
}

// SelfPortClashes finds the ports two worktrees of the same run would both
// bind. Nothing can be stopped to make room: the run itself is the conflict.
func SelfPortClashes(starting []domain.PortClaim) []domain.PortClash {
	byPort := map[int]domain.PortClaim{}
	var clashes []domain.PortClash
	for _, claim := range starting {
		first, taken := byPort[claim.Port]
		if !taken {
			byPort[claim.Port] = claim
			continue
		}
		if first.WorkDir == claim.WorkDir {
			continue
		}
		clashes = append(clashes, domain.PortClash{Port: claim.Port, Want: claim, HeldBy: first})
	}
	return clashes
}

// PortClashLines names each clash: the port, the job that wants it here, and
// the job and worktree already holding it.
func PortClashLines(clashes []domain.PortClash) []string {
	lines := make([]string, 0, len(clashes))
	for _, clash := range clashes {
		lines = append(lines, fmt.Sprintf(domain.RunPortClashLineFmt,
			clash.Port, clash.Want.Job, filepath.Base(clash.Want.WorkDir),
			clash.HeldBy.Job, filepath.Base(clash.HeldBy.WorkDir)))
	}
	return lines
}

// ClashingWorktrees are the worktrees holding the ports, which is what stopping
// them first would free.
func ClashingWorktrees(clashes []domain.PortClash) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, clash := range clashes {
		if seen[clash.HeldBy.WorkDir] {
			continue
		}
		seen[clash.HeldBy.WorkDir] = true
		dirs = append(dirs, clash.HeldBy.WorkDir)
	}
	sort.Strings(dirs)
	return dirs
}
