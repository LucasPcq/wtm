package rules

import "github.com/LucasPcq/wtm/internal/domain"

type URLCandidatesForParams struct {
	Config domain.RunConfig
	// NewJobs are the jobs this pass just added. They alone have never been
	// through the URLs step, so a nil URL on them is a question never asked
	// rather than an answer of no.
	NewJobs []string
}

// URLCandidatesFor is the jobs `run init` offers to publish under their own
// name. A job qualifies by declaring the port it listens on, or by already
// carrying a url — the latter so the step can withdraw one, which it could not
// do if the job were left off the list.
func URLCandidatesFor(params URLCandidatesForParams) []domain.JobURLChoice {
	fresh := make(map[string]bool, len(params.NewJobs))
	for _, name := range params.NewJobs {
		fresh[name] = true
	}

	var choices []domain.JobURLChoice
	for _, job := range params.Config.Jobs {
		port := publishablePortOf(job)
		if port == "" {
			continue
		}
		choices = append(choices, domain.JobURLChoice{
			Job:     job.Name,
			Port:    port,
			Publish: job.URL != nil || fresh[job.Name],
		})
	}
	return choices
}

// publishablePortOf answers which port a job would be published on, empty for
// one that has nothing to publish. A port already named by a url outranks the
// derived one: the step proposes a port to a job that has none, it never
// redirects a job to another port than the one it was given.
func publishablePortOf(job domain.JobConfig) string {
	if job.URL != nil && job.URL.Port != "" {
		return job.URL.Port
	}
	if job.Kind != domain.JobKindService {
		return ""
	}
	return PublishablePortName(job)
}

// ListeningPortName is the port a job binds, as opposed to the ones it dials.
//
// A name settles it when there is one: PORT for the first service to claim it,
// <JOB>_PORT for the ones after — the two freePortName writes. Otherwise a job
// declaring exactly one port binds that port: a DB_PORT and a REDIS_PORT on the
// same job are addresses it connects to, but a single declaration is what the
// job answers on. Without that last reading a Vite app declaring VITE_PORT
// published nothing, and the front-ends — the ones a human opens in a browser —
// were the only jobs left without a name.
func ListeningPortName(job domain.JobConfig) string {
	if _, ok := job.Ports[domain.PortNameDefault]; ok {
		return domain.PortNameDefault
	}
	derived := EnvVarNameFor(job.Name) + "_" + domain.PortNameDefault
	if _, ok := job.Ports[derived]; ok {
		return derived
	}
	return ""
}

// PublishablePortName is which port a job would be published under. It answers
// the URL question, not the wiring one: a job declaring exactly one port is
// published on it, because a single declaration is what the job answers on.
//
// Being strict here is what left every Vite front-end, declaring VITE_PORT and
// nothing else, with no name at all. The ports step reads it the same way: a
// job holding a single port is not asked for a second one.
func PublishablePortName(job domain.JobConfig) string {
	if name := ListeningPortName(job); name != "" {
		return name
	}
	if len(job.Ports) != 1 {
		return ""
	}
	for name := range job.Ports {
		return name
	}
	return ""
}

type ResolveURLChoicesParams struct {
	Config domain.RunConfig
	// Published names the jobs the reader left checked, and Asked says the
	// question was put at all — an empty selection that was answered withdraws
	// every url, where one that was never asked leaves the proposal standing.
	Published []string
	Asked     bool
	NewJobs   []string
}

// ResolveURLChoices is the one reading of the URLs step, shared by the wizard's
// recap and by the run that never showed it. Computing it twice let a
// non-interactive init write what the wizard would not have.
func ResolveURLChoices(params ResolveURLChoicesParams) []domain.JobURLChoice {
	choices := URLCandidatesFor(URLCandidatesForParams{Config: params.Config, NewJobs: params.NewJobs})
	if !params.Asked {
		return choices
	}

	published := make(map[string]bool, len(params.Published))
	for _, job := range params.Published {
		published[job] = true
	}
	for i := range choices {
		choices[i].Publish = published[choices[i].Job]
	}
	return choices
}
