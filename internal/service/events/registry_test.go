package events

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

func initializedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	state := filepath.Join(dir, ".git", domain.StateDirName)
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, domain.ConfigFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func subscribeAll(t *testing.T, socket string) <-chan process.Delivery {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	deliveries, err := process.Subscribe(ctx, process.SubscribeParams{SocketPath: socket})
	if err != nil {
		t.Fatal(err)
	}
	return deliveries
}

func nextDelivered(t *testing.T, deliveries <-chan process.Delivery) domain.Event {
	t.Helper()
	select {
	case d := <-deliveries:
		var event domain.Event
		if err := json.Unmarshal(d.Payload, &event); err != nil {
			t.Fatal(err)
		}
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("nothing delivered")
		return domain.Event{}
	}
}

func TestRegisterIsIdempotent(t *testing.T) {
	processtest.Home(t)
	dir := initializedRepo(t)

	for range 2 {
		if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir)}); err != nil {
			t.Fatal(err)
		}
	}

	repos, err := infra.ReadRegistry()
	if err != nil || len(repos) != 1 || repos[0].Root != dir || repos[0].AddedAt == "" {
		t.Fatalf("repos = %+v, err = %v", repos, err)
	}
}

func TestAnEmptyRegistryIsNoError(t *testing.T) {
	processtest.Home(t)

	repos, err := infra.ReadRegistry()
	if err != nil || len(repos) != 0 {
		t.Fatalf("repos = %+v, err = %v", repos, err)
	}
}

func TestConcurrentRegistrationsKeepEveryRepo(t *testing.T) {
	processtest.Home(t)
	dirs := make([]string, 8)
	for i := range dirs {
		dirs[i] = initializedRepo(t)
	}

	var wg sync.WaitGroup
	for _, dir := range dirs {
		wg.Go(func() { _ = Register(RegisterParams{Root: dir, StateDir: stateOf(dir)}) })
	}
	wg.Wait()

	if repos, _ := infra.ReadRegistry(); len(repos) != len(dirs) {
		t.Fatalf("registered %d, want %d", len(repos), len(dirs))
	}
}

func TestRegisterPublishesRepoAddedOnce(t *testing.T) {
	processtest.Home(t)
	socket := socktest.Path(t)
	processtest.RealDaemon(t, socket)
	deliveries := subscribeAll(t, socket)
	dir := initializedRepo(t)

	for range 2 {
		if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir), SocketPath: socket, CorrelationID: "c"}); err != nil {
			t.Fatal(err)
		}
	}

	got := nextDelivered(t, deliveries)
	if got.Type != domain.EventRepoAdded || got.Repo == nil || got.Repo.Root != dir || got.V != domain.EventsSchemaVersion || got.CorrelationID != "c" {
		t.Fatalf("event = %+v", got)
	}
	select {
	case d := <-deliveries:
		t.Fatalf("a second delivery: %s", d.Payload)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestPruneDropsARepoThatIsGoneAndPublishesIt(t *testing.T) {
	processtest.Home(t)
	socket := socktest.Path(t)
	processtest.RealDaemon(t, socket)
	kept, gone := initializedRepo(t), initializedRepo(t)
	for _, dir := range []string{kept, gone} {
		if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir)}); err != nil {
			t.Fatal(err)
		}
	}
	deliveries := subscribeAll(t, socket)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	repos, err := Prune(PruneParams{SocketPath: socket})

	if err != nil || len(repos) != 1 || repos[0].Root != kept {
		t.Fatalf("repos = %+v, err = %v", repos, err)
	}
	if got := nextDelivered(t, deliveries); got.Type != domain.EventRepoRemoved || got.Repo == nil || got.Repo.Root != gone {
		t.Fatalf("event = %+v", got)
	}
}

func TestPruneDropsARepoNoLongerInitialized(t *testing.T) {
	processtest.Home(t)
	dir := initializedRepo(t)
	if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir)}); err != nil {
		t.Fatal(err)
	}
	removeConfig(t, dir)

	if repos, err := Prune(PruneParams{}); err != nil || len(repos) != 0 {
		t.Fatalf("repos = %+v, err = %v", repos, err)
	}
}

func removeConfig(t *testing.T, dir string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, ".git", domain.StateDirName, domain.ConfigFileName)); err != nil {
		t.Fatal(err)
	}
}

func stateOf(dir string) string {
	return filepath.Join(dir, ".git", domain.StateDirName)
}

// A registry nobody can parse would fail every Register and keep a global
// stream retrying forever: it is rebuilt instead, from the repositories used.
func TestACorruptRegistryIsRebuilt(t *testing.T) {
	processtest.Home(t)
	path, err := infra.RegistryPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := initializedRepo(t)

	if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir)}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repos, err := Prune(PruneParams{}); err != nil || len(repos) != 1 {
		t.Fatalf("repos = %+v, err = %v", repos, err)
	}
}
