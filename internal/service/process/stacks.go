package process

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// stackProbeTimeout bounds one compose call. Docker Desktop starting up answers
// slowly or not at all, and a daemon start-up that hangs on it costs the user the
// command they actually ran. A probe that times out is a probe that could not
// tell, which changes nothing about the entry.
const stackProbeTimeout = 3 * time.Second

// StackQuery is one detached entry as the prober needs it: keyed so the answer
// can be matched back, and carrying the environment the launcher ran with —
// COMPOSE_PROJECT_NAME above all, since it is what names the project to ask
// about.
type StackQuery struct {
	Key string
	Job domain.JobConfig
	Dir string
	Env map[string]string
}

// StackState is what a probe found. Known false is the ordinary answer and the
// safe one: an unrecognized launcher, a docker that is not installed, a call that
// failed or timed out.
type StackState struct {
	Known bool
	Up    bool
}

// Stacks answers whether the work a detached launcher started is still running.
// Separate from Orphans because it is a different question about a different kind
// of thing — a process group wtm owns, versus containers Docker owns.
type Stacks interface {
	Probe(queries []StackQuery) map[string]StackState
}

type systemStacks struct{}

func (systemStacks) Probe(queries []StackQuery) map[string]StackState {
	states := make(map[string]StackState, len(queries))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, query := range queries {
		probe := rules.ComposeProbeFor(query.Job)
		if !probe.Recognized {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, ok := composeStackUp(query, probe)
			if !ok {
				return
			}
			mu.Lock()
			states[query.Key] = StackState{Known: true, Up: state}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return states
}

func composeStackUp(query StackQuery, probe rules.ComposeProbe) (up bool, known bool) {
	ctx, cancel := context.WithTimeout(context.Background(), stackProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, domain.DockerBin, probe.Args...)
	cmd.Dir = query.Dir
	cmd.Env = rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: query.Env,
	})

	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	return strings.TrimSpace(string(out)) != "", true
}

// stackQueriesOf builds the probe's questions from the records worth asking
// about. A claim owns no launcher, and a foreground service is the other probe's
// subject.
func stackQueriesOf(records []domain.JobRecord) []StackQuery {
	queries := make([]StackQuery, 0, len(records))
	for _, record := range records {
		if record.Attached || record.Config.Kind != domain.JobKindService || !rules.IsDetached(record.Config) {
			continue
		}
		queries = append(queries, StackQuery{
			Key: jobKey(record.Name, record.WorkDir),
			Job: record.Config,
			Dir: rules.JobDir(rules.JobDirParams{WorkDir: record.WorkDir, Cwd: record.Config.Cwd}),
			Env: record.Env,
		})
	}
	return queries
}
