package rules

import "github.com/LucasPcq/wtm/internal/domain"

// RemoveJobEffect is what a removal touched beyond the job itself, so both
// callers report the same thing.
type RemoveJobEffect struct {
	Removed         bool
	Profiles        []string
	EmptiedProfiles []string
	EnvPorts        []string
	// Runners are the jobs whose `runs` named this one. A runner left pointing
	// at a job that no longer exists fails validation, so the removal that
	// leaves it behind cannot be written at all.
	Runners []string
	// Touchers are the jobs whose `touches` named this one, for the same reason.
	Touchers []string
}

// RemoveJob takes a job out of a config along with everything that named it. A
// profile left with no job goes too: one that starts nothing is not a choice
// `run up` can offer.
func RemoveJob(cfg domain.RunConfig, name string) (domain.RunConfig, RemoveJobEffect) {
	var effect RemoveJobEffect

	out := cfg
	out.Jobs = make([]domain.JobConfig, 0, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		if job.Name == name {
			effect.Removed = true
			continue
		}
		out.Jobs = append(out.Jobs, job)
	}
	if !effect.Removed {
		return cfg, effect
	}

	out.Profiles = make([]domain.ProfileConfig, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		jobs := make([]string, 0, len(profile.Jobs))
		referenced := false
		for _, job := range profile.Jobs {
			if job == name {
				referenced = true
				continue
			}
			jobs = append(jobs, job)
		}
		if !referenced {
			out.Profiles = append(out.Profiles, profile)
			continue
		}
		effect.Profiles = append(effect.Profiles, profile.Name)
		if len(jobs) == 0 {
			effect.EmptiedProfiles = append(effect.EmptiedProfiles, profile.Name)
			continue
		}
		profile.Jobs = jobs
		out.Profiles = append(out.Profiles, profile)
	}

	// A job is named in three places, not two: the profiles that start it, the
	// env_port links that follow its ports, and the `runs` list of a runner that
	// starts it itself. Missing the third made such a job unremovable — the
	// removal was refused by the validator it had just invalidated.
	for i, job := range out.Jobs {
		kept := make([]string, 0, len(job.Runs))
		named := false
		for _, child := range job.Runs {
			if child == name {
				named = true
				continue
			}
			kept = append(kept, child)
		}
		if !named {
			continue
		}
		effect.Runners = append(effect.Runners, job.Name)
		out.Jobs[i].Runs = kept
		if len(kept) == 0 {
			out.Jobs[i].Runs = nil
		}
	}

	for i, job := range out.Jobs {
		kept := withoutName(job.Touches, name)
		if len(kept) == len(job.Touches) {
			continue
		}
		effect.Touchers = append(effect.Touchers, job.Name)
		out.Jobs[i].Touches = kept
	}

	out.EnvPorts = make([]domain.EnvPortLink, 0, len(cfg.EnvPorts))
	for _, link := range cfg.EnvPorts {
		if link.Job == name {
			effect.EnvPorts = append(effect.EnvPorts, link.Key)
			continue
		}
		out.EnvPorts = append(out.EnvPorts, link)
	}

	return out, effect
}

// withoutName is names without one of them, nil when nothing is left.
func withoutName(names []string, name string) []string {
	kept := make([]string, 0, len(names))
	for _, candidate := range names {
		if candidate != name {
			kept = append(kept, candidate)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}
