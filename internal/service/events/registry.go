package events

import (
	"encoding/json"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type RegisterParams struct {
	// Root is the main checkout, and StateDir the wtm directory inside the git
	// common dir: both are already resolved by the caller, so registering costs
	// no git call.
	Root     string
	StateDir string
	// SocketPath is the daemon's; empty is the one every command talks to.
	SocketPath    string
	CorrelationID string
}

// Register enrolls the repository a command runs in, for a global stream to
// follow. The common case — already listed — reads the file and nothing else.
func Register(params RegisterParams) error {
	commonDir := filepath.Dir(params.StateDir)
	if resolved, err := filepath.EvalSymlinks(commonDir); err == nil {
		commonDir = resolved
	}
	repo := domain.EventRepo{Root: params.Root, CommonDir: commonDir}
	if listed, err := infra.ReadRegistry(); err == nil && containsRepo(listed, repo.CommonDir) {
		return nil
	}
	added := false
	err := infra.UpdateRegistry(func(repos []domain.RegisteredRepo) []domain.RegisteredRepo {
		if containsRepo(repos, repo.CommonDir) {
			return repos
		}
		added = true
		return append(repos, domain.RegisteredRepo{CommonDir: repo.CommonDir, Root: repo.Root, AddedAt: now()})
	})
	if err != nil || !added {
		return err
	}
	bus := busParams{SocketPath: params.SocketPath, CorrelationID: params.CorrelationID}
	announce(announceParams{Bus: bus, Type: domain.EventRepoAdded, Repo: repo})
	_, err = Prune(PruneParams(bus))
	return err
}

type PruneParams struct {
	SocketPath    string
	CorrelationID string
}

// Prune drops the repositories that are gone or no longer initialized, and
// returns the ones left.
func Prune(params PruneParams) ([]domain.RegisteredRepo, error) {
	var kept, dropped []domain.RegisteredRepo
	err := infra.UpdateRegistry(func(repos []domain.RegisteredRepo) []domain.RegisteredRepo {
		kept, dropped = nil, nil
		for _, repo := range repos {
			if initialized(repo) {
				kept = append(kept, repo)
				continue
			}
			dropped = append(dropped, repo)
		}
		return kept
	})
	if err != nil {
		return nil, err
	}
	for _, repo := range dropped {
		announce(announceParams{Bus: busParams(params), Type: domain.EventRepoRemoved, Repo: eventRepoOf(repo)})
	}
	return kept, nil
}

func initialized(repo domain.RegisteredRepo) bool {
	return infra.FileExists(filepath.Join(repo.CommonDir, domain.StateDirName, domain.ConfigFileName))
}

func containsRepo(repos []domain.RegisteredRepo, commonDir string) bool {
	return slices.ContainsFunc(repos, func(repo domain.RegisteredRepo) bool { return repo.CommonDir == commonDir })
}

func eventRepoOf(repo domain.RegisteredRepo) domain.EventRepo {
	return domain.EventRepo{Root: repo.Root, CommonDir: repo.CommonDir}
}

type busParams struct {
	SocketPath    string
	CorrelationID string
}

type announceParams struct {
	Bus  busParams
	Type domain.EventType
	Repo domain.EventRepo
}

// announce is as opportunistic as Publish: with no daemon nobody listens, and a
// global stream reads the registry itself when it starts.
func announce(params announceParams) {
	socket := params.Bus.SocketPath
	if socket == "" {
		socket = process.SocketPath()
	}
	event := stamp(stampParams{Event: domain.Event{Type: params.Type, CorrelationID: params.Bus.CorrelationID}, Repo: params.Repo})
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_ = process.Publish(process.PublishParams{SocketPath: socket, Repo: params.Repo.CommonDir, Payload: payload})
}
