package rules

import (
	"slices"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
)

type TouchChoicesParams struct {
	// Config is the configuration the wizard is building, which carries the
	// names the jobs will be written under.
	Config   domain.RunConfig
	Existing domain.RunConfig
}

// TouchChoices lists every task against the services that hold data — the
// shared ones and the compose stacks. A task run.toml already gives touches
// keeps them; one it says nothing about gets what its name proposes.
func TouchChoices(params TouchChoicesParams) []domain.JobTouchChoice {
	services := DataServices(params.Config)
	if len(services) == 0 {
		return nil
	}
	options := append([]string{""}, services...)

	existing := make(map[string]domain.JobConfig, len(params.Existing.Jobs))
	for _, job := range params.Existing.Jobs {
		existing[job.Name] = job
	}

	var choices []domain.JobTouchChoice
	for _, job := range params.Config.Jobs {
		if job.Kind != domain.JobKindTask {
			continue
		}
		touches := job.Touches
		if held, found := existing[job.Name]; found && len(held.Touches) > 0 {
			touches = held.Touches
		}
		if len(touches) == 0 {
			touches = ProposedTouches(ProposedTouchesParams{Task: job.Name, Services: services})
		}
		choices = append(choices, domain.JobTouchChoice{
			Job:     job.Name,
			Label:   job.Name,
			Touches: slices.Clone(touches),
			Options: options,
		})
	}
	return choices
}

// DataServices are the jobs whose data a task can change: every shared
// service, and every service a compose stack runs.
func DataServices(cfg domain.RunConfig) []string {
	var names []string
	for _, job := range cfg.Jobs {
		if job.Kind != domain.JobKindService {
			continue
		}
		if IsShared(job) || runsCompose(job) {
			names = append(names, job.Name)
		}
	}
	return names
}

type ProposedTouchesParams struct {
	Task     string
	Services []string
}

// ProposedTouches reads a task's name: it proposes a service only when the name
// carries a data verb and shares a word with exactly one service —
// `orm:billing:reset` and `postgres-billing`. Anything less certain proposes nothing.
func ProposedTouches(params ProposedTouchesParams) []string {
	words := nameWords(params.Task)
	subjects := make([]string, 0, len(words))
	hasVerb := false
	for _, word := range words {
		if slices.Contains(domain.TouchDataVerbs, word) {
			hasVerb = true
			continue
		}
		subjects = append(subjects, word)
	}
	if !hasVerb || len(subjects) == 0 {
		return nil
	}

	var matches []string
	for _, service := range params.Services {
		if sharesWord(subjects, nameWords(service)) {
			matches = append(matches, service)
		}
	}
	if len(matches) != 1 {
		return nil
	}
	return matches
}

func nameWords(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func sharesWord(words, others []string) bool {
	for _, word := range words {
		if slices.Contains(others, word) {
			return true
		}
	}
	return false
}

type ApplyTouchChoicesParams struct {
	Config  domain.RunConfig
	Choices []domain.JobTouchChoice
}

// ApplyTouchChoices writes the step's answers onto the tasks they name. A row
// left on none clears the task's touches: the step asked, and none is an answer.
func ApplyTouchChoices(params ApplyTouchChoicesParams) domain.RunConfig {
	cfg, choices := params.Config, params.Choices
	if len(choices) == 0 {
		return cfg
	}
	declared := make(map[string]bool, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		declared[job.Name] = true
	}
	// A service the wizard showed but the final configuration no longer holds
	// is dropped rather than written: a touches naming nothing refuses the file.
	byJob := make(map[string][]string, len(choices))
	for _, choice := range choices {
		var kept []string
		for _, name := range choice.Touches {
			if declared[name] {
				kept = append(kept, name)
			}
		}
		byJob[choice.Job] = kept
	}

	out := cfg
	out.Jobs = make([]domain.JobConfig, len(cfg.Jobs))
	copy(out.Jobs, cfg.Jobs)
	for i, job := range out.Jobs {
		touches, answered := byJob[job.Name]
		if !answered {
			continue
		}
		out.Jobs[i].Touches = nil
		if len(touches) > 0 {
			out.Jobs[i].Touches = slices.Clone(touches)
		}
	}
	return out
}
