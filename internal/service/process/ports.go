package process

import (
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// jobPorts resolves the ports a job binds in the worktree its request describes.
// The offset is read back from the environment the client resolved rather than
// carried as its own field, so the number the daemon adds and the one the job
// reads can never drift apart.
func jobPorts(job domain.JobConfig, env map[string]string) map[string]int {
	offset, err := strconv.Atoi(env[domain.EnvPortOffset])
	if err != nil {
		offset = 0
	}
	return rules.JobPorts(rules.JobPortsParams{Ports: job.Ports, PortOffset: offset, Scope: job.Scope})
}

func withJobPorts(job domain.JobConfig, env map[string]string) map[string]string {
	return rules.WithPortEnv(env, jobPorts(job, env))
}
