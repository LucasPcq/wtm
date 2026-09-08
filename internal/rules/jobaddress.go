package rules

import (
	"fmt"
	"sort"

	"github.com/LucasPcq/wtm/internal/domain"
)

type WorktreeJobAddressesParams struct {
	Config domain.RunConfig
	// PortOffset is the worktree's own, what makes the same job bind a different
	// port in each one.
	PortOffset int
	// Worktree and Project are the labels the proxy routes a published job on.
	Worktree   string
	Project    string
	PublicPort int
}

// WorktreeJobAddresses is where every declared job answers in one worktree —
// what `run url` computes job by job, gathered in one reading for a surface
// listing them all.
//
// A runner answers for its children: it is given their ports and it publishes
// their names, so its address is theirs. Reading its own declaration instead
// showed the one job that is actually up as the one with nothing to show.
func WorktreeJobAddresses(params WorktreeJobAddressesParams) map[string]domain.JobAddress {
	if len(params.Config.Jobs) == 0 {
		return nil
	}
	addresses := make(map[string]domain.JobAddress, len(params.Config.Jobs))
	for _, job := range params.Config.Jobs {
		ports := JobPorts(JobPortsParams{
			Ports: EffectiveJobPorts(params.Config, job), PortOffset: params.PortOffset,
		})
		addresses[job.Name] = domain.JobAddress{
			Ports: sortedPortValues(ports),
			URL:   jobAddressURL(params, job, ports),
			Held:  heldAddresses(params, job),
		}
	}
	return addresses
}

func jobAddressURL(params WorktreeJobAddressesParams, job domain.JobConfig, ports map[string]int) string {
	return JobURL(JobURLParams{
		Job:        job,
		Ports:      ports,
		Host:       RouteHost(RouteHostParams{Job: job, Worktree: params.Worktree, Project: params.Project}),
		PublicPort: params.PublicPort,
	})
}

// heldAddresses is where each published job a runner starts answers. The ports
// come from the child's own declaration shifted by this worktree's offset —
// the very numbers the runner was given — so the two readings can never
// disagree about which port a name sits on.
func heldAddresses(params WorktreeJobAddressesParams, job domain.JobConfig) []domain.JobURLEntry {
	children := RunnerChildren(params.Config, job.Name)
	if len(children) == 0 {
		return nil
	}

	byName := jobsByName(params.Config)
	var held []domain.JobURLEntry
	for _, name := range children {
		child := byName[name]
		ports := JobPorts(JobPortsParams{Ports: child.Ports, PortOffset: params.PortOffset})
		if url := jobAddressURL(params, child, ports); url != "" {
			held = append(held, domain.JobURLEntry{Job: name, URL: url})
		}
	}
	return held
}

// A map range would permute the ports between two reads of the same config.
func sortedPortValues(ports map[string]int) []int {
	values := make([]int, 0, len(ports))
	for _, port := range ports {
		values = append(values, port)
	}
	sort.Ints(values)
	return values
}

// HeldBy names the jobs a runner already shows the address of, so a surface
// listing every declared job does not list them a second time on their own. It
// answers only for a runner the reader can see is up: a child nothing holds is
// a job like any other, and hiding it would lose the only place its address is
// written.
func HeldBy(cfg domain.RunConfig, up map[string]bool) map[string]bool {
	held := map[string]bool{}
	for _, job := range cfg.Jobs {
		if !up[job.Name] {
			continue
		}
		for _, child := range RunnerChildren(cfg, job.Name) {
			if !up[child] {
				held[child] = true
			}
		}
	}
	return held
}

// HeldAddressLines names each job a runner started and where it answers, one
// line each, aligned. A runner announces itself in one line and its apps
// underneath: six jobs' worth of addresses on the line of the single process
// holding them is unreadable, and leaving them out loses the only place they
// are written.
func HeldAddressLines(held []domain.JobURLEntry) []string {
	if len(held) == 0 {
		return nil
	}

	width := 0
	for _, entry := range held {
		width = max(width, len([]rune(entry.Job)))
	}

	lines := make([]string, 0, len(held))
	for _, entry := range held {
		lines = append(lines, fmt.Sprintf(domain.RunStreamHeldFmt, pad(entry.Job, width), entry.URL))
	}
	return lines
}
