package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ParseProcessLinks reads `ps -Ao pid=,ppid=,pgid=`. A line it cannot read is left out.
func ParseProcessLinks(table string) []domain.ProcessLink {
	var links []domain.ProcessLink
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		numbers := make([]int, 0, len(fields))
		for _, field := range fields {
			number, err := strconv.Atoi(field)
			if err != nil {
				break
			}
			numbers = append(numbers, number)
		}
		if len(numbers) != len(fields) {
			continue
		}
		links = append(links, domain.ProcessLink{PID: numbers[0], PPID: numbers[1], PGID: numbers[2]})
	}
	return links
}

type DescendantsParams struct {
	Links []domain.ProcessLink
	Root  int
	// Group is the process group the descendants must still be in. One that
	// left it — a credential cache, an fsmonitor, an ssh ControlPersist master —
	// daemonized on purpose to outlive its parent, and is never one of them.
	Group int
}

// Descendants are the processes below Root that stayed in Group, children
// before grandchildren.
func Descendants(params DescendantsParams) []int {
	children := map[int][]domain.ProcessLink{}
	for _, link := range params.Links {
		children[link.PPID] = append(children[link.PPID], link)
	}
	var found []int
	queue := children[params.Root]
	seen := map[int]bool{params.Root: true}
	for len(queue) > 0 {
		link := queue[0]
		queue = queue[1:]
		if seen[link.PID] || link.PGID != params.Group {
			continue
		}
		seen[link.PID] = true
		found = append(found, link.PID)
		queue = append(queue, children[link.PID]...)
	}
	return found
}

type StillInGroupParams struct {
	Links []domain.ProcessLink
	PIDs  []int
	Group int
}

// StillInGroup keeps the processes that are still alive and still in Group:
// one that daemonized since it was first seen is left alone.
func StillInGroup(params StillInGroupParams) []int {
	var kept []int
	for _, link := range params.Links {
		if link.PGID == params.Group && slices.Contains(params.PIDs, link.PID) {
			kept = append(kept, link.PID)
		}
	}
	return kept
}
