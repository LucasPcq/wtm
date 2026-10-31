package process

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	// startSkew is how far a group member's start may sit from the one its record says and still be the job's: ps gives starts to the second, and a record is written once the child started.
	startSkew = 90 * time.Second
	reapGrace = 2 * time.Second
	groupPoll = 50 * time.Millisecond
)

// Orphans finds and stops the process groups a daemon that died left behind.
type Orphans struct{}

// Probe returns how each group of started stands (a group id, mapped to when
// the record that names it says its job started): alive, and identified when
// a member of it started then. A ps that fails identifies none, which reaps
// nothing: the probe never keeps a daemon from starting.
func (Orphans) Probe(started map[int]time.Time) map[int]run.Group {
	groups := make(map[int]run.Group, len(started))
	alive := false
	for pgid := range started {
		groups[pgid] = run.Group{Alive: groupAlive(pgid)}
		alive = alive || groups[pgid].Alive
	}
	if !alive {
		return groups
	}

	starts := groupStarts(time.Now())
	for pgid, at := range started {
		if !groups[pgid].Alive {
			continue
		}
		for _, member := range starts[pgid] {
			if member.Sub(at).Abs() <= startSkew {
				groups[pgid] = run.Group{Alive: true, Identified: true}
				break
			}
		}
	}

	return groups
}

// Reap stops the groups pgids together, each with SIGTERM then, past a grace,
// SIGKILL, so the grace is paid once however many there are.
func (Orphans) Reap(pgids []int) {
	var reaped sync.WaitGroup
	for _, pgid := range pgids {
		reaped.Go(func() {
			signalGroup(pgid, syscall.SIGTERM)
			deadline := time.Now().Add(reapGrace)
			for groupAlive(pgid) && time.Now().Before(deadline) {
				time.Sleep(groupPoll)
			}
			signalGroup(pgid, syscall.SIGKILL)
		})
	}
	reaped.Wait()
}

// groupAlive reports whether this process may signal a member of the group
// pgid: one it may not is someone else's, which reads as gone.
func groupAlive(pgid int) bool {
	return pgid > 1 && syscall.Kill(-pgid, 0) == nil
}

// groupStarts returns when each process of each group started, read from one
// ps, now being when it ran; nil when ps fails.
func groupStarts(now time.Time) map[int][]time.Time {
	out, err := exec.Command("ps", "-Ao", "pgid=,etime=").Output()
	if err != nil {
		return nil
	}

	starts := map[int][]time.Time{}
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pgid, err := strconv.Atoi(fields[0])
		elapsed, ok := parseElapsed(fields[1])
		if err != nil || !ok {
			continue
		}
		starts[pgid] = append(starts[pgid], now.Add(-elapsed))
	}

	return starts
}

// parseElapsed reads ps's etime, [[dd-]hh:]mm:ss.
func parseElapsed(etime string) (time.Duration, bool) {
	days := 0
	if day, rest, found := strings.Cut(etime, "-"); found {
		n, err := strconv.Atoi(day)
		if err != nil {
			return 0, false
		}
		days, etime = n, rest
	}

	parts := strings.Split(etime, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	seconds := 0
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		seconds = seconds*60 + n
	}

	return time.Duration(days)*24*time.Hour + time.Duration(seconds)*time.Second, true
}
