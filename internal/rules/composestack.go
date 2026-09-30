package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ComposeProbe is the command that answers whether a detached launcher's work is
// still running. Recognized false is the honest majority case and the one every
// caller has to handle: the entry stays `detached`, exactly as R1 left it.
type ComposeProbe struct {
	Recognized bool
	// Args are passed to the docker binary, carrying the same -f files the
	// launcher used. Without them `ps` resolves whatever compose file happens to
	// sit in the directory, which is a different project's answer.
	Args []string
}

// ComposeProbeFor recognizes a compose launcher in a job's command and builds the
// query that verifies it. The two spellings it accepts are the two
// infra.DockerComposeCommand writes into a generated job, which is why both ends
// read the same constants. It is deliberately one tool with one stable subcommand,
// not a framework of launchers: everything unrecognized keeps saying what wtm
// actually knows, which is that it started the job and has not seen it since.
//
// Only -f/--file is carried over. The service names an `up` line ends with would
// narrow the answer usefully, but telling them from `up`'s own flags means
// tracking those flags — and a probe that misreads its own arguments is worse
// than a project-wide one.
func ComposeProbeFor(job domain.JobConfig) ComposeProbe {
	words := strings.Fields(job.Cmd)
	rest, ok := afterComposeBinary(words)
	if !ok {
		return ComposeProbe{}
	}

	args := []string{domain.ComposeSubcommand}
	args = append(args, composeFileFlags(rest)...)
	return ComposeProbe{Recognized: true, Args: append(args, domain.ComposePSArgs...)}
}

// afterComposeBinary accepts both spellings — `docker compose` and the legacy
// `docker-compose` — and returns what follows, which is where the file flags sit.
func afterComposeBinary(words []string) (rest []string, ok bool) {
	if len(words) >= 2 && words[0] == domain.DockerBin && words[1] == domain.ComposeSubcommand {
		return words[2:], true
	}
	if len(words) >= 1 && words[0] == domain.ComposeLegacyBin {
		return words[1:], true
	}
	return nil, false
}

func composeFileFlags(words []string) []string {
	var flags []string
	for i := 0; i < len(words); i++ {
		switch {
		case words[i] == domain.ComposeFileFlag || words[i] == domain.ComposeFileFlagLong:
			if i+1 >= len(words) {
				return flags
			}
			flags = append(flags, domain.ComposeFileFlag, words[i+1])
			i++
		case strings.HasPrefix(words[i], domain.ComposeFileFlagLong+"="):
			flags = append(flags, domain.ComposeFileFlag, strings.TrimPrefix(words[i], domain.ComposeFileFlagLong+"="))
		}
	}
	return flags
}
