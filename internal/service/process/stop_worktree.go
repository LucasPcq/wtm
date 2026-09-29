package process

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type WorktreeJobsParams struct {
	SocketPath string
	WorkDir    string
}

// StopWorktreeJobs stops the jobs workDir runs itself and checks they are gone.
// Its claims on shared services stay standing: releasing the last one stops the
// service, which could then not take the worktree's namespace back.
// ReleaseWorktreeJobs lets them go once it has.
func StopWorktreeJobs(params WorktreeJobsParams) (stopped []string, err error) {
	client, reachable, err := daemonFor(params)
	if err != nil || !reachable {
		return nil, err
	}
	own, err := ownJobsUp(client, params.WorkDir)
	if err != nil || len(own) == 0 {
		return nil, err
	}

	var errs []error
	for _, name := range own {
		resp, sendErr := client.Send(Request{Action: ActionStop, Name: name, WorkDir: params.WorkDir})
		if sendErr != nil {
			errs = append(errs, sendErr)
			continue
		}
		if resp.Status == StatusError {
			errs = append(errs, fmt.Errorf("%s: %s", name, resp.Message))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	survivors, err := ownJobsUp(client, params.WorkDir)
	if err != nil {
		return nil, err
	}
	if len(survivors) > 0 {
		return nil, fmt.Errorf("%w: %s", domain.ErrWorktreeJobsRunning,
			fmt.Sprintf(domain.StopWorktreeSurvivorsFmt, strings.Join(survivors, ", ")))
	}
	return own, nil
}

// ReleaseWorktreeJobs lets go of everything workDir still holds — its claims on
// shared services, and whatever a forced removal left running.
func ReleaseWorktreeJobs(params WorktreeJobsParams) error {
	client, reachable, err := daemonFor(params)
	if err != nil || !reachable {
		return err
	}
	resp, err := client.Send(Request{Action: ActionStopAll, WorkDir: params.WorkDir})
	if err != nil {
		return err
	}
	if resp.Status == StatusError {
		return errors.New(resp.Message)
	}
	return nil
}

// daemonFor reaches the daemon holding workDir's jobs. None listening and none
// indexed means there is nothing to stop, and no daemon is started to learn it;
// a detached stack outliving its daemon does need one to run its stop command.
func daemonFor(params WorktreeJobsParams) (*Client, bool, error) {
	if !IsDaemonRunning(params.SocketPath) {
		if !HasIndexedJobs(params.WorkDir) {
			return nil, false, nil
		}
		if err := EnsureDaemon(DaemonParams{SocketPath: params.SocketPath}); err != nil {
			return nil, false, err
		}
	}
	return NewClient(params.SocketPath), true, nil
}

func ownJobsUp(client *Client, workDir string) ([]string, error) {
	resp, err := client.Send(Request{Action: ActionList})
	if err != nil {
		return nil, err
	}
	if resp.Status == StatusError {
		return nil, errors.New(resp.Message)
	}
	var own []string
	for _, info := range resp.Jobs {
		if info.WorkDir != workDir || !rules.IsJobUp(info.Status) || info.Status == domain.JobStatusAttached {
			continue
		}
		own = append(own, info.Name)
	}
	return own, nil
}
