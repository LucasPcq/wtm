package process

import (
	"os"
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
	reached := false
	for pgid := range started {
		groups[pgid] = run.Group{}
		// This process's own group is never a job's: a record naming it is read as gone, never reaped.
		reached = reached || pgid != os.Getpid() && pgid != syscall.Getpgrp() && reachable(pgid)
	}
	if !reached {
		return groups
	}

	members := groupMembers(time.Now())
	for pgid, at := range started {
		// A group this process may not signal is someone else's, which ps lists all the same: it reads as gone, never reaped.
		if pgid == os.Getpid() || pgid == syscall.Getpgrp() || !reachable(pgid) {
			continue
		}
		// A ps that failed identifies none: nothing is reaped, nothing reached is called gone.
		if members == nil {
			groups[pgid] = run.Group{Alive: true}
			continue
		}
		for _, member := range members[pgid] {
			group := groups[pgid]
			group.Alive = true
			group.Identified = group.Identified || member.Sub(at).Abs() <= startSkew
			groups[pgid] = group
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
			if groupAlive(pgid) {
				signalGroup(pgid, syscall.SIGKILL)
			}
		})
	}
	reaped.Wait()
}

// groupAlive reports whether a member of the group pgid is alive: one this
// process may signal (one it may not is someone else's, which reads as gone)
// that is no zombie. A member that exited is still reached by a signal until
// its parent reaps it, the init process for one its group's leader left, in
// its own time: counting it would read a group stopped as alive.
func groupAlive(pgid int) bool {
	if !reachable(pgid) {
		return false
	}
	members := groupMembers(time.Now())
	return members == nil || len(members[pgid]) > 0
}

// reachable reports whether this process may signal a member of the group
// pgid, a zombie included.
func reachable(pgid int) bool {
	return pgid > 1 && syscall.Kill(-pgid, 0) == nil
}

// groupMembers returns when each process of each group that is no zombie
// started, read from one ps, now being when it ran; nil when ps fails, never
// nil otherwise.
func groupMembers(now time.Time) map[int][]time.Time {
	out, err := exec.Command("ps", "-Ao", "pgid=,etime=,stat=").Output()
	if err != nil {
		return nil
	}

	starts := map[int][]time.Time{}
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) != 3 || strings.HasPrefix(fields[2], "Z") {
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
