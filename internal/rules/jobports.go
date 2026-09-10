package rules

import (
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsEnvVarName reports whether key can be exported to a child process.
func IsEnvVarName(key string) bool { return envVarNamePattern.MatchString(key) }

// EffectivePortOffsetBlock is the spacing a config actually runs with. A block
// left unset, or set to zero, is the default rather than a block of nothing —
// which would collapse every worktree onto the main checkout's ports.
func EffectivePortOffsetBlock(cfg domain.RunConfig) int {
	if cfg.PortOffsetBlock <= 0 {
		return domain.PortOffsetBlock
	}
	return cfg.PortOffsetBlock
}

type JobPortsParams struct {
	Ports      map[string]int
	PortOffset int
	// Scope zeroes the offset for a shared job: it runs in the main checkout,
	// so the port it declares is the port it binds in every worktree's reading.
	// That stability is what lets a namespace's env write its URL literally.
	Scope domain.JobScope
}

// JobPorts resolves a job's declared base ports for the worktree it is about to
// run in. The main checkout has offset 0, so its ports stay exactly as declared.
func JobPorts(params JobPortsParams) map[string]int {
	if len(params.Ports) == 0 {
		return nil
	}
	offset := params.PortOffset
	if params.Scope == domain.JobScopeShared {
		offset = 0
	}
	resolved := make(map[string]int, len(params.Ports))
	for name, base := range params.Ports {
		resolved[name] = base + offset
	}
	return resolved
}

// PortDeclaration is one `NAME = base` entry, carrying the job it came from so
// a collision can name both sides.
type PortDeclaration struct {
	Job  string
	Name string
	Base int
	// Scope is the declaring job's. A shared job binds its declared port in
	// every worktree, so its declaration never moves.
	Scope domain.JobScope
}

// PortCollision is a pair of declarations that end up on the same port.
// Worktrees is how far apart the two worktrees reaching it are — zero meaning
// the pair already collides inside a single one.
type PortCollision struct {
	A         PortDeclaration
	B         PortDeclaration
	Worktrees int
}

// PortDeclarations flattens a config's ports in a stable order: jobs as
// declared, names sorted within each job.
func PortDeclarations(cfg domain.RunConfig) []PortDeclaration {
	var decls []PortDeclaration
	for _, job := range cfg.Jobs {
		for _, name := range sortedPortNames(job.Ports) {
			decls = append(decls, PortDeclaration{Job: job.Name, Name: name, Base: job.Ports[name], Scope: job.Scope})
		}
	}
	return decls
}

// PortCollisions finds the base ports that cannot coexist. Two of them meet
// whenever their difference is a multiple of the block: worktree n binds
// base+n×block, so a gap of k×block is exactly what k worktrees of offset
// close. The horizon bounds it — a pair that only meets on the 508th worktree
// is arithmetic, not a conflict.
func PortCollisions(cfg domain.RunConfig) []PortCollision {
	block := EffectivePortOffsetBlock(cfg)
	decls := PortDeclarations(cfg)

	var collisions []PortCollision
	for i := range decls {
		for j := i + 1; j < len(decls); j++ {
			gap := decls[j].Base - decls[i].Base
			if gap < 0 {
				gap = -gap
			}
			// Two shared declarations never move, so they meet only where they
			// are already equal. A shared one against a per-worktree one is the
			// ordinary arithmetic: the moving side still walks onto the fixed
			// one every block.
			if decls[i].Scope == domain.JobScopeShared && decls[j].Scope == domain.JobScopeShared {
				if gap != 0 {
					continue
				}
			} else if gap%block != 0 {
				continue
			}
			if apart := gap / block; apart <= domain.PortCollisionHorizon {
				collisions = append(collisions, PortCollision{A: decls[i], B: decls[j], Worktrees: apart})
			}
		}
	}
	return collisions
}

type ProxyPortCollisionsParams struct {
	Config    domain.RunConfig
	ProxyPort int
}

// ProxyPortCollision is a declaration whose resolved port reaches the proxy's,
// and how many worktrees away that happens.
type ProxyPortCollision struct {
	Declaration PortDeclaration
	Worktrees   int
}

// ProxyPortCollisions finds the bases that land on the proxy's port. The daemon
// binds before any job does, so the job is the one that fails — with the
// port's own error, which names nothing about wtm. Unlike PortCollisions the
// gap is one-way: an offset is always positive, so a base above the proxy's
// port never reaches it.
func ProxyPortCollisions(params ProxyPortCollisionsParams) []ProxyPortCollision {
	if params.ProxyPort <= 0 {
		return nil
	}

	block := EffectivePortOffsetBlock(params.Config)
	var collisions []ProxyPortCollision
	for _, decl := range PortDeclarations(params.Config) {
		gap := params.ProxyPort - decl.Base
		if gap < 0 || gap%block != 0 {
			continue
		}
		if apart := gap / block; apart <= domain.PortCollisionHorizon {
			collisions = append(collisions, ProxyPortCollision{Declaration: decl, Worktrees: apart})
		}
	}
	return collisions
}

// ProxyPortCollisionLines names each base that will meet the proxy, and the two
// ways out: move the base, or move the proxy.
func ProxyPortCollisionLines(collisions []ProxyPortCollision, proxyPort int) []string {
	lines := make([]string, 0, len(collisions))
	for _, c := range collisions {
		lines = append(lines, fmt.Sprintf(domain.ProxyPortCollisionFmt,
			c.Declaration.Name, c.Declaration.Job, c.Declaration.Base, proxyPort, c.Worktrees))
	}
	return lines
}

func sortedPortNames(ports map[string]int) []string {
	names := make([]string, 0, len(ports))
	for name := range ports {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type LabelWithPortsParams struct {
	Label string
	Ports map[string]int
}

// LabelWithPorts appends the ports a job bound to the line naming it, so the
// user reading it knows where to reach that worktree's copy. A job declaring
// none reads exactly as it did before.
func LabelWithPorts(params LabelWithPortsParams) string {
	if len(params.Ports) == 0 {
		return params.Label
	}

	return fmt.Sprintf(domain.RunPortsSuffixFmt, params.Label, strings.Join(PortEntries(params.Ports), " "))
}

// PortEntries writes ports back in the NAME=PORT form ParsePorts reads, sorted
// so a rewritten declaration keeps a stable order.
func PortEntries(ports map[string]int) []string {
	entries := make([]string, 0, len(ports))
	for _, name := range sortedPortNames(ports) {
		entries = append(entries, fmt.Sprintf(domain.RunPortEntryFmt, name, ports[name]))
	}
	return entries
}

// ParsePorts reads the NAME=PORT entries a --port flag repeats and the wizard
// puts on one line. It rejects what ValidateRunPorts would reject anyway, but
// here the user is still typing and can be told which entry is wrong.
func ParsePorts(entries []string) (map[string]int, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	ports := make(map[string]int, len(entries))
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("port %q: expected NAME=PORT", entry)
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		if !IsEnvVarName(name) {
			return nil, fmt.Errorf("port %q: %q is not a valid environment variable name", entry, name)
		}
		if _, duplicate := ports[name]; duplicate {
			return nil, fmt.Errorf("port %s is declared twice", name)
		}

		base, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("port %s: %q is not a number", name, value)
		}
		if base < domain.PortMin || base > domain.PortMax {
			return nil, fmt.Errorf("port %s is %d, outside %d-%d", name, base, domain.PortMin, domain.PortMax)
		}
		ports[name] = base
	}
	return ports, nil
}

type LifecyclePortsParams struct {
	Config     domain.RunConfig
	PortOffset int
}

// LifecyclePorts resolves the ports a lifecycle hook can be given: those whose
// variable name a single job declares. A hook is not a job, so it has no
// declaration of its own; two jobs naming PORT with different bases give the
// name no answer, and it is left unresolved rather than guessed.
func LifecyclePorts(params LifecyclePortsParams) map[string]int {
	decls := PortDeclarations(params.Config)

	jobs := make(map[string]int, len(decls))
	for _, decl := range decls {
		jobs[decl.Name]++
	}

	ports := map[string]int{}
	for _, decl := range decls {
		if jobs[decl.Name] > 1 {
			continue
		}
		// A shared job binds its declared port in every worktree, so the value a
		// hook and a .env read must be that port and not this worktree's shift —
		// or the whole worktree would address a service that answers elsewhere.
		if decl.Scope == domain.JobScopeShared {
			ports[decl.Name] = decl.Base
			continue
		}
		ports[decl.Name] = decl.Base + params.PortOffset
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

// WithPortEnv layers resolved ports onto an environment. A declaration in
// run.toml is an explicit instruction, so it wins over whatever the caller
// inherited for that name — without that, the isolation the ports exist for
// would not happen.
func WithPortEnv(env map[string]string, ports map[string]int) map[string]string {
	if len(ports) == 0 {
		return env
	}

	merged := make(map[string]string, len(env)+len(ports))
	maps.Copy(merged, env)
	for name, port := range ports {
		merged[name] = strconv.Itoa(port)
	}
	return merged
}
