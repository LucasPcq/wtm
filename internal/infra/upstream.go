package infra

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type UpstreamsParams struct {
	ProjectDir string
	Branches   []string
}

const upstreamFieldSep = "\x00"

// Upstreams reads what each branch tracks in one `git for-each-ref`, however
// many branches are asked about.
func Upstreams(params UpstreamsParams) (map[string]domain.Upstream, error) {
	upstreams := map[string]domain.Upstream{}
	if len(params.Branches) == 0 {
		return upstreams, nil
	}
	args := []string{"for-each-ref", "--format=" + strings.Join([]string{
		"%(refname)", "%(upstream:remotename)", "%(upstream:remoteref)", "%(upstream)", "%(upstream:track)",
	}, "%00")}
	for _, branch := range params.Branches {
		args = append(args, domain.LocalRefPrefix+branch)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git for-each-ref: %w", err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		fields := strings.Split(line, upstreamFieldSep)
		if len(fields) != 5 {
			continue
		}
		upstreams[strings.TrimPrefix(fields[0], domain.LocalRefPrefix)] = domain.Upstream{
			Remote:      fields[1],
			RemoteRef:   fields[2],
			TrackingRef: fields[3],
			Gone:        fields[4] == "[gone]",
		}
	}
	return upstreams, nil
}

type RemoteRefsParams struct {
	ProjectDir string
	Remote     string
	Refs       []string
}

// ExistingRemoteRefs asks the remote which of Refs it has, without fetching
// anything: the remote filters its advertisement down to the refs named.
func ExistingRemoteRefs(params RemoteRefsParams) (map[string]bool, error) {
	existing := map[string]bool{}
	if len(params.Refs) == 0 {
		return existing, nil
	}
	cmd := exec.Command("git", append([]string{"ls-remote", params.Remote}, params.Refs...)...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote %s: %w", params.Remote, err)
	}
	asked := make(map[string]bool, len(params.Refs))
	for _, ref := range params.Refs {
		asked[ref] = true
	}
	// ls-remote matches a pattern on its tail, so refs/heads/a also answers for
	// refs/heads/x/refs/heads/a: keep exact names only.
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_, ref, found := strings.Cut(line, "\t")
		if found && asked[ref] {
			existing[ref] = true
		}
	}
	return existing, nil
}

// FetchRemoteRefs fetches the named refs of a remote, each landing on its
// remote-tracking ref through the remote's configured refspec, as a full fetch
// would land it. Every ref must exist on the remote.
func FetchRemoteRefs(params RemoteRefsParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	cmd := exec.Command("git", append([]string{"fetch", "--quiet", params.Remote}, params.Refs...)...)
	cmd.Dir = params.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch %s: %s", params.Remote, strings.TrimSpace(string(out)))
	}
	return nil
}

type DeleteRefsParams struct {
	ProjectDir string
	Refs       []string
}

// DeleteRefs deletes the refs in one transaction; a ref already absent is not
// an error.
func DeleteRefs(params DeleteRefsParams) error {
	if len(params.Refs) == 0 {
		return nil
	}
	var stdin strings.Builder
	for _, ref := range params.Refs {
		fmt.Fprintf(&stdin, "delete %s\n", ref)
	}
	cmd := exec.Command("git", "update-ref", "--stdin")
	cmd.Dir = params.ProjectDir
	cmd.Stdin = strings.NewReader(stdin.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git update-ref: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
