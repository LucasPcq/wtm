package process

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/rules"
)

// orphanStartSkew is how far a group member's start may sit from the StartedAt
// its record holds and still be the same process. The two are taken
// milliseconds apart — StartedAt just before the spawn, etime by the reaping
// daemon — so this only absorbs a loaded machine and the one-second resolution
// of the ps field. It is nowhere near wide enough to accept a group id recycled
// hours or days later, which is the whole point of measuring it.
const orphanStartSkew = 90 * time.Second

// orphanGracePeriod is shorter than stopGracePeriod on purpose: a foreground
// service whose reader died has nothing left to flush, and this wait is paid at
// daemon start-up — which means it is paid by the user's next `run up`.
const orphanGracePeriod = 2 * time.Second

const reapPollInterval = 50 * time.Millisecond

// GroupQuery is one indexed job's fingerprint, as the prober reads it.
type GroupQuery struct {
	PGID      int
	StartedAt time.Time
}

// GroupState is what a probe found. Alive without IdentityConfirmed is the case
// the caution exists for: something holds that group id, and it is not ours.
type GroupState struct {
	Alive             bool
	IdentityConfirmed bool
}

// Orphans is how a daemon deals with the process groups the previous one left
// behind: what it can still find of them, and what it takes down. One seam for
// both, so Adopt can be tested without spawning or signalling anything.
type Orphans interface {
	Probe(groups []GroupQuery) map[int]GroupState
	Reap(pgids []int)
}

type systemOrphans struct{}

func (systemOrphans) Probe(groups []GroupQuery) map[int]GroupState {
	states := make(map[int]GroupState, len(groups))
	live := make([]GroupQuery, 0, len(groups))
	for _, query := range groups {
		states[query.PGID] = GroupState{}
		if !groupAlive(query.PGID) {
			continue
		}
		states[query.PGID] = GroupState{Alive: true}
		live = append(live, query)
	}
	if len(live) == 0 {
		return states
	}

	// One ps for the whole pass: a per-group call would fork as many times as
	// the index has entries, at the moment the user is waiting on a daemon.
	members := groupStartTimes()
	for _, query := range live {
		for _, started := range members[query.PGID] {
			if withinSkew(started, query.StartedAt) {
				states[query.PGID] = GroupState{Alive: true, IdentityConfirmed: true}
				break
			}
		}
	}
	return states
}

// groupAlive reports whether this process may signal that group. A group id of
// 0, 1 or below is refused before any syscall: kill(-1, …) would signal
// everything the user owns. EPERM is alive but somebody else's, which reads the
// same as dead here — never a group to reap.
func groupAlive(pgid int) bool {
	if pgid <= 1 {
		return false
	}
	return syscall.Kill(-pgid, syscall.Signal(0)) == nil
}

// groupStartTimes reads every process's group and elapsed time in one call.
// A ps that is missing, refused or unreadable yields nothing, which reconciles
// as "cannot confirm" and therefore kills nothing: the probe must never be what
// stops a daemon from starting.
func groupStartTimes() map[int][]time.Time {
	out, err := exec.Command("ps", "-Ao", "pid=,pgid=,etime=").Output()
	if err != nil {
		return nil
	}

	now := time.Now()
	starts := make(map[int][]time.Time)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pgid, convErr := strconv.Atoi(fields[1])
		if convErr != nil {
			continue
		}
		elapsed, ok := rules.ParseElapsed(fields[2])
		if !ok {
			continue
		}
		starts[pgid] = append(starts[pgid], now.Add(-elapsed))
	}
	return starts
}

func withinSkew(observed time.Time, recorded time.Time) bool {
	gap := observed.Sub(recorded)
	if gap < 0 {
		gap = -gap
	}
	return gap <= orphanStartSkew
}

type reapParams struct {
	PGID  int
	Grace time.Duration
}

// reapGroup signals the group rather than the leader, which is what makes it
// work at all: the leader is the `sh -c` rules.ShellCommand produces, and it
// often dies before the watcher it spawned. The escalation is not optional
// either — `tsx watch` intercepts SIGTERM to wait on a child it no longer has,
// and only SIGKILL ended the two that survived twelve days.
func reapGroup(params reapParams) {
	if params.PGID <= 1 {
		return
	}
	if err := syscall.Kill(-params.PGID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return
	}

	deadline := time.Now().Add(params.Grace)
	for time.Now().Before(deadline) {
		if !groupAlive(params.PGID) {
			return
		}
		time.Sleep(reapPollInterval)
	}
	_ = syscall.Kill(-params.PGID, syscall.SIGKILL)
}

// Reap takes the groups down together, so the grace period above is paid once
// however many orphans a pass finds rather than once each.
func (systemOrphans) Reap(pgids []int) {
	var wg sync.WaitGroup
	for _, pgid := range pgids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reapGroup(reapParams{PGID: pgid, Grace: orphanGracePeriod})
		}()
	}
	wg.Wait()
}

// processGroupOf asks the kernel rather than assuming the group equals the PID.
// It does today — pty.Start forces Setsid and a task gets Setpgid — but a
// record signalled on that assumption would aim at nothing the day it changes.
func processGroupOf(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return pid
	}
	return pgid
}
