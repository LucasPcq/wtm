package events

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeCreatedIsPublishedAsTheDaemonRelaysIt(t *testing.T) {
	daemon := listen(t)

	publisherOn(t, daemon).Publish(domain.WorktreeCreated{Worktree: identity()})

	request := daemon.next(t)
	assert.JSONEq(t, `{"action":"publish","repo":"`+daemon.commonDir+`","payload":{}}`, withoutPayload(t, request))
	assert.JSONEq(t, `{"v":2,"type":"worktree.created","ts":"2026-10-16T12:00:00Z","correlation_id":"popup-1","repo":{"root":"/repo","common_dir":"`+daemon.commonDir+`"},"worktree":{"branch":"feat/a","path":"/repo/.trees/feat-a","parent":"main","ordinal":null,"isolation":"isolated","is_main":false,"created_at":"2026-10-16T11:00:00Z"},"op_id":"op-1"}`, payloadOf(t, request))
}

func TestAWorktreeUpdatedNamesTheFieldsThatChanged(t *testing.T) {
	daemon := listen(t)
	numbered := identity()
	numbered.Ordinal = new(3)

	publisherOn(t, daemon).Publish(domain.WorktreeUpdated{Worktree: numbered, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}})

	payload := decoded(t, payloadOf(t, daemon.next(t)))
	assert.Equal(t, "worktree.updated", payload["type"])
	assert.Equal(t, []any{"ordinal"}, payload["changed"])
	assert.Equal(t, float64(3), payload["worktree"].(map[string]any)["ordinal"])
}

func TestAWorktreeProvisionedSaysWhetherItWorkedAndWhichHookFailed(t *testing.T) {
	daemon := listen(t)
	exit := 3
	publisher := publisherOn(t, daemon)

	publisher.Publish(domain.WorktreeProvisioned{Worktree: identity(), OK: true})
	publisher.Publish(domain.WorktreeProvisioned{Worktree: identity(), Hook: "exit 3", ExitCode: &exit})

	ok := decoded(t, payloadOf(t, daemon.next(t)))
	failed := decoded(t, payloadOf(t, daemon.next(t)))
	assert.Equal(t, "worktree.provisioned", ok["type"])
	assert.Equal(t, true, ok["ok"])
	assert.NotContains(t, ok, "hook")
	assert.NotContains(t, ok, "exit_code")
	assert.Equal(t, false, failed["ok"])
	assert.Equal(t, "exit 3", failed["hook"])
	assert.Equal(t, float64(3), failed["exit_code"])
}

func TestAnOperationIsAnnouncedStartedUnitByUnitAndFinished(t *testing.T) {
	daemon := listen(t)
	publisher := publisherOn(t, daemon)

	publisher.Publish(domain.OperationStarted{Command: "create", Surface: "cli"})
	publisher.Publish(domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
	publisher.Publish(domain.OperationFinished{Command: "create", Surface: "cli", Code: "lock.held"})

	assert.JSONEq(t, `{"v":2,"type":"operation.started","ts":"2026-10-16T12:00:00Z","correlation_id":"popup-1","repo":{"root":"/repo","common_dir":"`+daemon.commonDir+`"},"op_id":"op-1","command":"create","surface":"cli"}`, payloadOf(t, daemon.next(t)))
	assert.JSONEq(t, `{"v":2,"type":"unit.finished","ts":"2026-10-16T12:00:00Z","correlation_id":"popup-1","repo":{"root":"/repo","common_dir":"`+daemon.commonDir+`"},"op_id":"op-1","subject":"feat/a","status":"done"}`, payloadOf(t, daemon.next(t)))
	assert.JSONEq(t, `{"v":2,"type":"operation.finished","ts":"2026-10-16T12:00:00Z","correlation_id":"popup-1","repo":{"root":"/repo","common_dir":"`+daemon.commonDir+`"},"op_id":"op-1","command":"create","surface":"cli","ok":false,"code":"lock.held"}`, payloadOf(t, daemon.next(t)))
}

func TestAnOperationThatEndedWellCarriesNoCode(t *testing.T) {
	daemon := listen(t)

	publisherOn(t, daemon).Publish(domain.OperationFinished{Command: "create", Surface: "cli", OK: true})

	payload := decoded(t, payloadOf(t, daemon.next(t)))
	assert.Equal(t, true, payload["ok"])
	assert.NotContains(t, payload, "code")
}

func TestAnUncorrelatedEventCarriesNoCorrelation(t *testing.T) {
	daemon := listen(t)
	publisher := publisherOn(t, daemon)
	publisher.correlationID = ""

	publisher.Publish(domain.WorktreeCreated{Worktree: identity()})

	assert.NotContains(t, decoded(t, payloadOf(t, daemon.next(t))), "correlation_id")
}

func TestAUnitStartedIsPublishedSoAnotherSurfaceKnowsTheWorktreeIsUnderWay(t *testing.T) {
	daemon := listen(t)
	publisher := publisherOn(t, daemon)

	publisher.Publish(domain.UnitStarted{Subject: "feat/a"})

	line := decoded(t, payloadOf(t, daemon.next(t)))
	assert.Equal(t, "unit.started", line["type"])
	assert.Equal(t, "feat/a", line["subject"])
}

func TestProgressAndTheStreamsOwnLinesAreNeverPublished(t *testing.T) {
	daemon := listen(t)
	publisher := publisherOn(t, daemon)

	publisher.Publish(domain.PhaseStarted{Subject: "feat/a", Phase: "on_create"})
	publisher.Publish(domain.Output{Subject: "feat/a", Line: "installing"})
	publisher.Publish(domain.PhaseFinished{Subject: "feat/a", Phase: "on_create"})
	publisher.Publish(domain.Snapshot{})
	publisher.Publish(domain.Ready{})
	publisher.Publish(domain.WorktreeCreated{Worktree: identity()})

	assert.Equal(t, "worktree.created", decoded(t, payloadOf(t, daemon.next(t)))["type"])
}

func TestPublishingWithNoDaemonListeningReturnsQuietlyAndFast(t *testing.T) {
	publisher := NewPublisher(PublisherParams{Root: "/repo", CommonDir: t.TempDir(), SocketPath: filepath.Join(t.TempDir(), "wtm.sock")})
	begin := time.Now()

	publisher.Publish(domain.WorktreeCreated{Worktree: identity()})

	assert.Less(t, time.Since(begin), publishTimeout)
}

func TestAPublisherWithNoSocketPublishesNothing(t *testing.T) {
	assert.NotPanics(t, func() {
		NewPublisher(PublisherParams{Root: "/repo", CommonDir: t.TempDir()}).Publish(domain.WorktreeCreated{Worktree: identity()})
	})
}

func TestADaemonThatRefusesTheEventDoesNotStopThePublisher(t *testing.T) {
	daemon := listenAnswering(t, `{"status":"error","message":"no"}`)

	publisherOn(t, daemon).Publish(domain.WorktreeCreated{Worktree: identity()})

	require.NotEmpty(t, daemon.next(t))
}

func TestTheSocketIsTheOneWtmsDaemonListensOn(t *testing.T) {
	path := SocketPath()

	assert.Equal(t, "wtm.sock", filepath.Base(path))
	assert.Equal(t, "wtm", filepath.Base(filepath.Dir(path)))
}

func TestTheCommonDirIsResolvedThroughSymlinks(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, symlink(target, link))
	resolved, err := filepath.EvalSymlinks(target)
	require.NoError(t, err)

	publisher := NewPublisher(PublisherParams{Root: "/repo", CommonDir: link})

	assert.Equal(t, resolved, publisher.repo.CommonDir)
}

func TestARepositoryJoiningIsAnnouncedWithItsEnvelopeAlone(t *testing.T) {
	daemon := listen(t)
	publisher := publisherOn(t, daemon)
	publisher.opID, publisher.correlationID = "", ""

	publisher.Publish(domain.RepoAdded{})

	assert.JSONEq(t, `{"v":2,"type":"repo.added","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"`+daemon.commonDir+`"}}`, payloadOf(t, daemon.next(t)))
}
