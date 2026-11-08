package run

import "slices"

// A Shutdown is what a daemon's shutdown does to the jobs up it holds: the
// processes it runs itself stop (Cut, a shared job's own run among them), its
// stacks run on and are taken over by the next daemon, as are the claims on a
// shared stack (Kept), and the claims on a shared job it runs itself go with
// that job (Lost): the next daemon drops a claim whose job is down.
type Shutdown struct {
	Cut  []Job
	Kept []Job
	Lost []Job
}

// ShutdownOf decides the shutdown of a daemon holding jobs, those up alone.
func ShutdownOf(jobs []Job) Shutdown {
	var s Shutdown
	for _, job := range jobs {
		switch {
		case !job.Up:
		case job.Hold == HoldOwn:
			s.Cut = append(s.Cut, job)
		case job.Hold == HoldClaim && slices.ContainsFunc(jobs, func(served Job) bool {
			return served.Up && served.Hold == HoldOwn && served.Name == job.Name && served.WorkDir == job.SharedDir
		}):
			s.Lost = append(s.Lost, job)
		default:
			s.Kept = append(s.Kept, job)
		}
	}
	return s
}
