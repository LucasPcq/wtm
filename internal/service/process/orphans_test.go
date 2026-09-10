package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

const orphanPGID = 4242

type fakeOrphans struct {
	states map[int]GroupState
	asked  []GroupQuery
	reaped []int
}

func (f *fakeOrphans) Probe(groups []GroupQuery) map[int]GroupState {
	f.asked = append(f.asked, groups...)
	return f.states
}

func (f *fakeOrphans) Reap(pgids []int) {
	f.reaped = append(f.reaped, pgids...)
}

func foregroundRecord(t *testing.T, workDir string) domain.JobRecord {
	t.Helper()
	record := detachedRecord(t, workDir)
	record.Name = "dev"
	record.Config.Name = "dev"
	record.Config.Cmd = "pnpm dev"
	record.Config.Stop = ""
	record.PID = orphanPGID
	record.PGID = orphanPGID
	record.StartedAt = time.Now().Add(-12 * 24 * time.Hour)
	return record
}

func aliveAndOurs() map[int]GroupState {
	return map[int]GroupState{orphanPGID: {Alive: true, IdentityConfirmed: true}}
}

func TestAdoptReapsAForegroundServiceThatOutlivedItsDaemon(t *testing.T) {
	dir := t.TempDir()
	orphans := &fakeOrphans{states: aliveAndOurs()}
	index := &recordingIndex{}

	manager := NewManagerWith(ManagerParams{Index: index, Orphans: orphans})
	manager.Adopt([]domain.JobRecord{foregroundRecord(t, dir)})

	if len(orphans.reaped) != 1 || orphans.reaped[0] != orphanPGID {
		t.Fatalf("reaped %v, want the recorded group: a service whose reader is dead serves nobody", orphans.reaped)
	}
	jobs := manager.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusReaped {
		t.Fatalf("jobs = %+v, want one reported %q so run ps names the reap", jobs, domain.JobStatusReaped)
	}
	if last := index.saved[len(index.saved)-1]; len(last) != 0 {
		t.Fatalf("index still holds %d records: a reaped job is not up", len(last))
	}
}

func TestAdoptLeavesAGroupItCouldNotIdentifyAlone(t *testing.T) {
	dir := t.TempDir()
	orphans := &fakeOrphans{states: map[int]GroupState{orphanPGID: {Alive: true}}}

	manager := NewManagerWith(ManagerParams{Orphans: orphans})
	manager.Adopt([]domain.JobRecord{foregroundRecord(t, dir)})

	if len(orphans.reaped) != 0 {
		t.Fatal("a recycled group id belongs to a stranger: killing it is the worst outcome available")
	}
	if len(manager.List()) != 0 {
		t.Fatal("an entry aiming at a stranger says nothing worth reporting")
	}
}

func TestAdoptReapsEvenWhenTheWorktreeIsGone(t *testing.T) {
	dir := t.TempDir()
	record := foregroundRecord(t, dir)
	os.RemoveAll(dir)
	orphans := &fakeOrphans{states: aliveAndOurs()}

	manager := NewManagerWith(ManagerParams{Orphans: orphans})
	manager.Adopt([]domain.JobRecord{record})

	if len(orphans.reaped) != 1 {
		t.Fatal("a deleted worktree is the worst case: no run down can name the process any more")
	}
}

func TestAdoptAsksNothingAboutADetachedStack(t *testing.T) {
	dir := t.TempDir()
	record := detachedRecord(t, dir)
	record.PGID = orphanPGID
	orphans := &fakeOrphans{}

	manager := NewManagerWith(ManagerParams{Orphans: orphans})
	manager.Adopt([]domain.JobRecord{record})

	if len(orphans.asked) != 0 {
		t.Fatalf("probed %v: a detached stack belongs to Docker, wtm owns no process of it", orphans.asked)
	}
}

func TestAdoptAsksNothingAboutARecordWithNoFingerprint(t *testing.T) {
	dir := t.TempDir()
	record := foregroundRecord(t, dir)
	record.PID, record.PGID = 0, 0
	orphans := &fakeOrphans{}

	manager := NewManagerWith(ManagerParams{Orphans: orphans})
	manager.Adopt([]domain.JobRecord{record})

	if len(orphans.asked) != 0 {
		t.Fatal("a record written before the fingerprint existed names no group to aim at")
	}
	jobs := manager.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusCrashed {
		t.Fatalf("jobs = %+v, want the pre-fingerprint path unchanged", jobs)
	}
}

func TestAdoptDropsTheClaimsOfAServiceItReaped(t *testing.T) {
	mainDir, tenantDir := t.TempDir(), t.TempDir()
	service := foregroundRecord(t, mainDir)
	service.SharedDir = mainDir
	claim := foregroundRecord(t, tenantDir)
	claim.SharedDir = mainDir
	claim.Attached = true
	claim.PID, claim.PGID = 0, 0

	orphans := &fakeOrphans{states: aliveAndOurs()}
	manager := NewManagerWith(ManagerParams{Orphans: orphans})
	manager.Adopt([]domain.JobRecord{service, claim})

	for _, job := range manager.List() {
		if job.Status == domain.JobStatusAttached {
			t.Fatal("a claim on a reaped service is a reference count on nothing: it would answer \"already running\" to the next worktree")
		}
	}
}

// startOrphanGroup reproduces the shape found on the machine: a `sh -c` leader
// that is already dead while the child it spawned holds the group open.
func startOrphanGroup(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("sh", "-c", "sleep 300 & exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	pgid := cmd.Process.Pid
	_ = cmd.Wait()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if groupAlive(pgid) {
			return pgid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Skip("could not observe an orphan group on this machine")
	return 0
}

func TestReapGroupKillsAGroupWhoseLeaderIsAlreadyGone(t *testing.T) {
	pgid := startOrphanGroup(t)
	defer syscall.Kill(-pgid, syscall.SIGKILL)

	systemOrphans{}.Reap([]int{pgid})

	if groupAlive(pgid) {
		t.Fatal("the group survived the reap: signalling the leader alone is what leaves twelve-day orphans behind")
	}
}

func TestProbeRefusesTheGroupsThatWouldSignalEverything(t *testing.T) {
	states := systemOrphans{}.Probe([]GroupQuery{{PGID: 0}, {PGID: 1}, {PGID: -1}})

	for pgid, state := range states {
		if state.Alive {
			t.Fatalf("group %d reported alive: kill(-1, …) would signal everything the user owns", pgid)
		}
	}
}

func TestProbeIdentifiesAGroupItJustStarted(t *testing.T) {
	pgid := startOrphanGroup(t)
	defer syscall.Kill(-pgid, syscall.SIGKILL)

	states := systemOrphans{}.Probe([]GroupQuery{{PGID: pgid, StartedAt: time.Now()}})

	if !states[pgid].Alive || !states[pgid].IdentityConfirmed {
		t.Fatalf("state = %+v, want a group started just now to be recognised", states[pgid])
	}
}

func TestProbeRefusesAGroupThatDoesNotMatchTheRecordedStart(t *testing.T) {
	pgid := startOrphanGroup(t)
	defer syscall.Kill(-pgid, syscall.SIGKILL)

	states := systemOrphans{}.Probe([]GroupQuery{{PGID: pgid, StartedAt: time.Now().Add(-12 * 24 * time.Hour)}})

	if !states[pgid].Alive {
		t.Fatal("the group is alive; the probe must say so")
	}
	if states[pgid].IdentityConfirmed {
		t.Fatal("a group id handed to a process started twelve days after the record is not the record's process")
	}
}

// The whole reconciliation rests on this: a service the daemon starts must leave
// the index a group to aim at, or the next daemon has nothing to reconcile with.
func TestStartRecordsTheGroupTheIndexWillHaveToSignal(t *testing.T) {
	dir := t.TempDir()
	index := &recordingIndex{}
	manager := NewManagerWith(ManagerParams{Index: index})

	job := domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := manager.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = manager.StopAll() })

	records := index.saved[len(index.saved)-1]
	if len(records) != 1 {
		t.Fatalf("indexed %d records, want the running service", len(records))
	}
	if records[0].PID <= 1 || records[0].PGID <= 1 {
		t.Fatalf("record = %+v, want a fingerprint a signal can aim at", records[0])
	}
	if records[0].PGID != processGroupOf(records[0].PID) {
		t.Fatalf("PGID = %d, want the group the kernel reports for %d", records[0].PGID, records[0].PID)
	}
}

// The whole chain, with nothing faked: a daemon starts a foreground service and
// is killed without running a handler, and the next one finds the group, proves
// it is the recorded one, and takes it down.
func TestADaemonKilledWithoutAHandlerLeavesNothingBehindTheNextOneCannotReap(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(t.TempDir(), domain.DaemonStateFileName)

	killed := NewManagerWith(ManagerParams{Index: NewStateStore(statePath)})
	job := domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 300"}
	if err := killed.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start: %v", err)
	}

	records := NewStateStore(statePath).Load()
	if len(records) != 1 || records[0].PGID <= 1 {
		t.Fatalf("index = %+v, want the service and its group", records)
	}
	pgid := records[0].PGID
	defer syscall.Kill(-pgid, syscall.SIGKILL)

	// No StopForeground: this is the SIGKILL case, where no teardown runs at all.
	next := NewManagerWith(ManagerParams{Index: &recordingIndex{}})
	next.Adopt(records)

	if groupAlive(pgid) {
		t.Fatal("the orphan survived a full reconciliation: this is the twelve-day bug")
	}
	jobs := next.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusReaped {
		t.Fatalf("jobs = %+v, want the reap named so run ps can report it", jobs)
	}
}
