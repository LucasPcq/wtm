package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run init's form.
func words(q runinit.Questions, field domain.Field) form.Words {
	d := q.Decision
	switch field.Spec().Path {
	case runinit.PathJobs:
		return form.Words{Flag: "--job", Label: "What runs", Help: "The jobs to write, each a compose service, an app or a package script (repeatable; default: as run.toml holds them, else every one found)", Empty: "Pick at least one job", Options: candidateWords(d)}
	case runinit.PathTasks:
		return form.Words{Flag: "--task", Label: "Run once", Help: "The jobs that run to their end, the others keep running (repeatable)", Empty: "No task"}
	case runinit.PathRunner:
		return form.Words{Flag: "--runner", Label: "Root dev script", Help: "How the root script that starts the apps runs: one_by_one, each app its own job, or through, the script one job that starts them", Options: map[string]form.OptionWords{
			string(runinit.OneByOne): {Label: "One by one: each app its own port, logs and restart", Short: "one by one"},
			string(runinit.Through):  {Label: "Through the root script, as one job", Short: "through the root script"},
		}}
	case runinit.PathShared:
		return form.Words{Flag: "--shared", Label: "Shared", Help: "The compose services one copy serves for every worktree, the others one each (repeatable)", Empty: "None shared"}
	case runinit.PathTouches:
		return form.Words{Flag: "--touches", Label: "Tasks that change data", Help: "The tasks that change a shared service's data: they run on each worktree's own space (repeatable)", Empty: "None"}
	case runinit.PathPorts:
		return form.Words{Flag: "--port", Label: "Ports", Help: "Each job's port, job.VARIABLE=base, or job= for a job that binds none (repeatable)"}
	case runinit.PathCommands:
		return form.Words{Flag: "--command", Label: "Commands", Help: "A job's command changed, job=command, so it reads its port's variable (repeatable)"}
	case runinit.PathAddressing:
		return form.Words{Flag: "--addressing", Label: "Addresses", Help: "How .env values spell an address: names, each worktree its own host through the proxy, or ports", Options: map[string]form.OptionWords{
			"names": {Label: "Named URLs: <job>.<worktree>.<repository>.localhost, each worktree its own cookies, through the proxy", Short: "named URLs"},
			"ports": {Label: "Port URLs: localhost and the worktree's port", Short: "port URLs"},
		}}
	case runinit.PathURLs:
		return form.Words{Flag: "--url", Label: "Named jobs", Help: "The jobs answering under their own name through the proxy (repeatable)", Empty: "None named"}
	case runinit.PathEnvCopy:
		return form.Words{Flag: "--env-copy", Label: "Copy into worktrees", Help: "The .env files config.toml copies into a new worktree besides its own (repeatable)", Empty: "None"}
	case runinit.PathLinks:
		return form.Words{Flag: "--link", Label: ".env keys", Help: "The .env keys that follow a job, file#KEY (repeatable; default: every key holding a job's port)", Empty: "None linked", Options: linkWords(d)}
	case runinit.PathProfiles:
		return form.Words{Flag: "--profile", Label: "Profiles", Help: "The profiles run up starts, name=job,job (repeatable)"}
	case runinit.PathDefaultProfile:
		return form.Words{Flag: "--default-profile", Label: "Default profile", Help: "The profile run up starts when none is named"}
	case runinit.PathPatchCompose:
		return form.Words{Flag: "--patch-compose", Label: "docker-compose.yml", Help: "Rewrite docker-compose.yml's literal host ports and pinned names so each worktree gets its own (default: true)", Options: map[string]form.OptionWords{
			"true":  {Label: "Rewrite it", Short: "rewritten"},
			"false": {Label: "Leave it as it is", Short: "left as it is"},
		}}
	case runinit.PathPreview:
		return form.Words{Flag: "--preview", Label: "Preview", Help: "The worktree the plan previews (default: the first that is not the main one)"}
	case runinit.PathSpaces:
		return form.Words{Flag: "--space", Label: "Each worktree's space", Help: "What each worktree gets of a shared service: service.name=its space ({worktree} filled in), service.create= and service.remove=the commands that make and drop it, its name in $WTM_NAMESPACE (repeatable; a service with no name gets nothing of its own)"}
	}

	return form.Words{}
}

func candidateWords(d runinit.Decision) map[string]form.OptionWords {
	said := map[string]form.OptionWords{}
	for _, c := range d.Candidates {
		where := c.Place + " · " + c.Cmd
		if c.Compose {
			where = c.Image + " · docker compose"
		}
		said[c.Name] = form.OptionWords{Label: c.Name + "  " + where}
	}
	return said
}

func linkWords(d runinit.Decision) map[string]form.OptionWords {
	said := map[string]form.OptionWords{}
	for _, link := range d.Linkable {
		said[link.Spelled()] = form.OptionWords{Label: link.File + " · " + link.Key + " follows " + link.Job + " (" + string(link.Kind) + ")"}
	}
	return said
}
