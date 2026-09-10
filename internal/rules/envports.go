package rules

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// EnvPortBases is every port a run config declares, keyed by the job that
// carries it and its name. It is deliberately not keyed by name alone: two jobs
// may each declare a PORT, and flattening them made a link follow whichever base
// happened to be written last.
func EnvPortBases(cfg domain.RunConfig) map[domain.PortRef]int {
	bases := map[domain.PortRef]int{}
	for _, job := range cfg.Jobs {
		for name, base := range job.Ports {
			bases[domain.PortRef{Job: job.Name, Name: name}] = base
		}
	}
	return bases
}

// EnvPortBaseFor resolves the base one link follows. A link naming no job names
// no base either: the port name alone can belong to two jobs, and picking one is
// how a key came to follow the wrong port.
func EnvPortBaseFor(bases map[domain.PortRef]int, link domain.EnvPortLink) (base int, found bool) {
	if link.Job == "" {
		return 0, false
	}
	base, found = bases[domain.PortRef{Job: link.Job, Name: link.Port}]
	return base, found
}

// OriginContext is everything a plan needs to write an address rather than a
// port: which addressing the project asked for, the jobs a link may name, and
// where this worktree is published. A zero value plans ports, which is what a
// caller that knows nothing about the proxy gets.
type OriginContext struct {
	Addressing domain.Addressing
	Jobs       map[string]domain.JobConfig
	Worktree   string
	Project    string
	// PublicPort is what a named URL announces, zero when nothing serves names.
	PublicPort int
}

// Names reports whether this context can write an address at all.
func (c OriginContext) Names() bool {
	return c.Addressing == domain.AddressingNames && c.PublicPort > 0
}

// JobLabel is the host segment the job a link names publishes under, empty for
// a job that publishes nothing.
func (c OriginContext) JobLabel(job string) string {
	return JobHostLabel(c.Jobs[job])
}

type PlanEnvPortsParams struct {
	Links   []domain.EnvPortLink
	Bases   map[domain.PortRef]int
	Origins OriginContext
	// Block is the spacing between two worktrees' ports, needed to recognize
	// another worktree's spelling of a value copied from it.
	Block int
	// Offset is the worktree's port offset — zero for the main checkout, where
	// every substitution is the identity.
	Offset int
	// Lines holds the parsed .env of each file a link names, keyed by the same
	// path the link spells. A file absent from the map contributes missing keys
	// rather than nothing, so a link never disappears silently from the report.
	Lines map[string][]domain.EnvLine
	// Shared names the jobs that run once for the repository. Their ports never
	// take this worktree's offset — the service binds what it declares — so a
	// .env shifted for them would address something that answers elsewhere.
	Shared map[string]bool
}

// PlanEnvPorts resolves every link against the value its .env currently holds,
// without any I/O. A value is only ever rewritten when the declared base sits in
// it exactly once: two occurrences, or none, are reported instead — picking one
// number out of a URL by guesswork can corrupt it.
func PlanEnvPorts(params PlanEnvPortsParams) domain.EnvPortPlan {
	plan := domain.EnvPortPlan{
		Offset:     params.Offset,
		Addressing: params.Origins.Addressing,
		PublicPort: params.Origins.PublicPort,
	}

	// Links are grouped by the value they act on: a key listing two origins is
	// one line of one file, rewritten once, and two entries for it would have the
	// second overwrite the first.
	for _, group := range groupLinksByKey(params.Links, params.Bases) {
		plan.Entries = append(plan.Entries, planEnvPortKey(planEnvPortKeyParams{
			Group:   group,
			Offset:  params.Offset,
			Block:   params.Block,
			Lines:   params.Lines[group.File],
			Origins: params.Origins,
			Shared:  params.Shared,
		}))
	}

	return plan
}

// linkGroup is every link one .env key follows, in declaration order.
type linkGroup struct {
	File  string
	Key   string
	Links []domain.EnvPortLink
	Bases []int
}

func groupLinksByKey(links []domain.EnvPortLink, bases map[domain.PortRef]int) []linkGroup {
	var order []string
	byKey := map[string]*linkGroup{}
	for _, link := range links {
		base, declared := EnvPortBaseFor(bases, link)
		if !declared {
			continue
		}
		id := link.File + "\x00" + link.Key
		group, seen := byKey[id]
		if !seen {
			group = &linkGroup{File: link.File, Key: link.Key}
			byKey[id] = group
			order = append(order, id)
		}
		group.Links = append(group.Links, link)
		group.Bases = append(group.Bases, base)
	}

	groups := make([]linkGroup, 0, len(order))
	for _, id := range order {
		groups = append(groups, *byKey[id])
	}
	return groups
}

type planEnvPortKeyParams struct {
	Group   linkGroup
	Offset  int
	Block   int
	Lines   []domain.EnvLine
	Origins OriginContext
	Shared  map[string]bool
}

// planEnvPortKey folds every link a key follows over the same value, each one
// moving its own port and leaving the rest of the value — an origin no job
// declares, a port belonging to nothing here — exactly as it found it.
func planEnvPortKey(params planEnvPortKeyParams) domain.EnvPortEntry {
	merged := planEnvPortEntry(planEnvPortEntryParams{
		Link:     params.Group.Links[0],
		Base:     params.Group.Bases[0],
		Resolved: resolvedPort(params, 0),
		Block:    params.Block,
		Lines:    params.Lines,
		Origins:  params.Origins,
	})
	merged.Moves = []domain.EnvPortMove{moveOf(params.Group.Links[0], params.Group.Bases[0], resolvedPort(params, 0))}
	if len(params.Group.Links) == 1 {
		return merged
	}

	lines := params.Lines
	for i := 1; i < len(params.Group.Links); i++ {
		// Each link reads the value the one before it left, so two ports in one
		// value both move.
		if merged.Status == domain.EnvPortStatusRewrite {
			lines = ApplyEnvPorts(lines, []domain.EnvPortEntry{merged})
		}
		next := planEnvPortEntry(planEnvPortEntryParams{
			Link:     params.Group.Links[i],
			Base:     params.Group.Bases[i],
			Resolved: resolvedPort(params, i),
			Block:    params.Block,
			Lines:    lines,
			Origins:  params.Origins,
		})
		merged = foldEnvPortEntry(merged, next)
		merged.Moves = append(merged.Moves, moveOf(params.Group.Links[i], params.Group.Bases[i], resolvedPort(params, i)))
	}
	return merged
}

// resolvedPort is what one link's port becomes in this worktree, and it is the
// only place that answers it. A shared job takes no shift: it binds its declared
// port in every worktree, so a .env shifted for it addresses something that
// answers elsewhere.
//
// Everything downstream reads the result rather than recomputing it. Two sites
// deriving it independently is exactly how the report came to say 5432 while
// the file was written 5452.
func resolvedPort(params planEnvPortKeyParams, index int) int {
	return ResolvedPort(ResolvedPortParams{
		Base:   params.Group.Bases[index],
		Offset: params.Offset,
		Shared: params.Shared[params.Group.Links[index].Job],
	})
}

type ResolvedPortParams struct {
	Base   int
	Offset int
	Shared bool
}

// ResolvedPort is what a declared port becomes in one worktree. Exported so the
// [[env]] templates read the same answer rather than deriving their own: two
// sites computing it independently is exactly how the report came to say 5432
// while the file was written 5452.
func ResolvedPort(params ResolvedPortParams) int {
	if params.Shared {
		return params.Base
	}
	return params.Base + params.Offset
}

func moveOf(link domain.EnvPortLink, base, resolved int) domain.EnvPortMove {
	return domain.EnvPortMove{Port: link.Port, Job: link.Job, Base: base, Resolved: resolved}
}

// foldEnvPortEntry keeps what the run has done so far and what the next link
// adds to it. A key is rewritten as soon as one of its ports moves, and its
// value is whatever the last rewrite left; a link that found nothing to do
// never withdraws the work of the one before it.
func foldEnvPortEntry(into, next domain.EnvPortEntry) domain.EnvPortEntry {
	if next.Status == domain.EnvPortStatusRewrite {
		into.NewValue = next.NewValue
		into.Status = domain.EnvPortStatusRewrite
		// One key may take an origin from one job and a bare port from another.
		// The value is addressed by name as soon as either link writes one, and
		// reading the last link alone hid the origin behind the port.
		if next.Addressing == domain.AddressingNames {
			into.Addressing = next.Addressing
		}
		return into
	}
	if into.Status == domain.EnvPortStatusRewrite {
		return into
	}
	// Neither moved: the more specific reading wins, so a key nobody could place
	// still says why.
	if into.Status == domain.EnvPortStatusUnchanged || into.Status == domain.EnvPortStatusNotFound {
		into.Status = next.Status
	}
	return into
}

type planEnvPortEntryParams struct {
	Link domain.EnvPortLink
	Base int
	// Resolved is the port this link takes in this worktree, decided by
	// resolvedPort. It is passed in rather than derived here so that the answer
	// exists once: a second derivation is a second thing to keep in step.
	Resolved int
	Block    int
	Lines    []domain.EnvLine
	Origins  OriginContext
}

func planEnvPortEntry(params planEnvPortEntryParams) domain.EnvPortEntry {
	entry := domain.EnvPortEntry{
		File:       params.Link.File,
		Key:        params.Link.Key,
		Port:       params.Link.Port,
		Base:       params.Base,
		Resolved:   params.Resolved,
		Addressing: domain.AddressingPorts,
	}

	line, found := envPairByKey(params.Lines, params.Link.Key)
	if !found {
		entry.Status = domain.EnvPortStatusMissingKey
		return entry
	}
	entry.CurrentValue = line.Value

	if origin, planned := planOriginEntry(entry, line.Value, params); planned {
		return origin
	}

	// Rewound to the base before anything is shifted, because the value may not
	// be spelled on the base at all: a .env provisioned from a parent worktree
	// carries that worktree's port, and one a previous pass addressed carries a
	// route. Both are this setting, written by another worktree.
	value := ReduceEnvPortValue(ReduceEnvPortParams{
		Value:    line.Value,
		Base:     entry.Base,
		Block:    params.Block,
		JobLabel: params.Origins.JobLabel(params.Link.Job),
		Project:  params.Origins.Project,
	})

	switch at := portOffsets(value, entry.Base); {
	case len(at) > 1:
		entry.Status = domain.EnvPortStatusAmbiguous
	case len(at) == 1:
		entry.NewValue = replaceAt(value, at[0], len(strconv.Itoa(entry.Base)), entry.Resolved)
		entry.Status = domain.EnvPortStatusRewrite
		if entry.NewValue == entry.CurrentValue {
			entry.Status = domain.EnvPortStatusUnchanged
			entry.NewValue = ""
		}
	default:
		entry.Status = alreadyResolvedStatus(value, entry)
	}

	return entry
}

// planOriginEntry resolves one link as an address, reporting false when it must
// stay a port — the project asked for ports, the job publishes no name, or the
// value holds no URL for an authority to be swapped in.
func planOriginEntry(entry domain.EnvPortEntry, value string, params planEnvPortEntryParams) (domain.EnvPortEntry, bool) {
	if !params.Origins.Names() {
		return entry, false
	}

	origin := LinkOrigin(LinkOriginParams{
		Job:        params.Origins.Jobs[params.Link.Job],
		PortName:   params.Link.Port,
		Worktree:   params.Origins.Worktree,
		Project:    params.Origins.Project,
		PublicPort: params.Origins.PublicPort,
	})
	if origin == "" {
		return entry, false
	}

	rewrite := RewriteOrigin(RewriteOriginParams{
		Value:    value,
		Origin:   origin,
		JobLabel: params.Origins.JobLabel(params.Link.Job),
		Project:  params.Origins.Project,
		Base:     entry.Base,
		Resolved: entry.Resolved,
	})
	if rewrite.Fallback {
		return entry, false
	}

	entry.Addressing = domain.AddressingNames
	entry.Status = rewrite.Status
	entry.NewValue = rewrite.Value
	entry.ForeignHost = rewrite.ForeignHost
	return entry, true
}

// alreadyResolvedStatus classifies a value the base does not appear in. Finding
// the resolved port instead means a previous run already applied the offset —
// the common case when `wtm env` runs twice — and is not an anomaly.
func alreadyResolvedStatus(value string, entry domain.EnvPortEntry) domain.EnvPortStatus {
	if entry.Resolved == entry.Base {
		return domain.EnvPortStatusNotFound
	}
	switch len(portOffsets(value, entry.Resolved)) {
	case 0:
		return domain.EnvPortStatusNotFound
	case 1:
		return domain.EnvPortStatusUnchanged
	default:
		return domain.EnvPortStatusAmbiguous
	}
}

// ApplyEnvPorts writes the planned rewrites into the lines of one file. Raw is
// cleared on every mutated pair, which is what makes RenderEnv re-emit the line
// from Key/Value instead of reproducing the stale original.
func ApplyEnvPorts(lines []domain.EnvLine, entries []domain.EnvPortEntry) []domain.EnvLine {
	byKey := map[string]string{}
	for _, e := range entries {
		if e.Status == domain.EnvPortStatusRewrite {
			byKey[e.Key] = e.NewValue
		}
	}
	if len(byKey) == 0 {
		return lines
	}

	out := make([]domain.EnvLine, len(lines))
	copy(out, lines)
	for i, line := range out {
		value, rewritten := byKey[line.Key]
		if line.Kind != domain.EnvLinePair || !rewritten {
			continue
		}
		out[i].Value = value
		out[i].Raw = ""
	}
	return out
}

// EnvValueRef is what the reconciliation diff needs to recognize any worktree's
// spelling of one linked key: the declared base, and the route the value may
// carry instead of a port.
type EnvValueRef struct {
	Base     int
	JobLabel string
	Project  string
}

// EnvValueRefsByKey indexes those by key. A key whose job publishes no name gets
// an empty label, and only the port reduction ever applies to it.
func EnvValueRefsByKey(links []domain.EnvPortLink, bases map[domain.PortRef]int, origins OriginContext) map[string]EnvValueRef {
	byKey := map[string]EnvValueRef{}
	for _, link := range links {
		base, declared := EnvPortBaseFor(bases, link)
		if !declared {
			continue
		}
		// The label comes from the declaration, never from LinkOrigin: a .env
		// written while the proxy was up still holds a route once it is down,
		// and the diff has to recognize it either way.
		ref := EnvValueRef{Base: base, Project: origins.Project}
		if PublishesPort(PublishesPortParams{Job: origins.Jobs[link.Job], PortName: link.Port}) {
			ref.JobLabel = origins.JobLabel(link.Job)
		}
		byKey[link.Key] = ref
	}
	return byKey
}

type ReduceEnvPortParams struct {
	Value string
	Base  int
	Block int
	// JobLabel and Project name the route this key may already hold. Empty for a
	// key no published job backs, where only the port reduction below applies.
	JobLabel string
	Project  string
}

// ReduceEnvPortValue rewinds whichever worktree's port a value holds back to the
// declared base, so two worktrees' spellings of the same setting compare equal.
//
// It cannot simply undo the reader's own offset: under the `parent` strategy the
// source value comes from another worktree, bound on that worktree's offset,
// which the reader does not know. What every worktree's port does share is the
// arithmetic — it is the base plus some multiple of the block — so the reduction
// looks for a number of that shape instead of for one known value.
//
// A value holding no such number, or more than one, is returned untouched:
// reducing on a guess would hide a real conflict.
func ReduceEnvPortValue(params ReduceEnvPortParams) string {
	value := ReduceOriginValue(ReduceOriginParams{
		Value: params.Value, JobLabel: params.JobLabel, Project: params.Project, Base: params.Base,
	})
	if value != params.Value {
		// A route just rewound onto the base is already canonical: applying the
		// port reduction to it would look for a ladder step that is not there.
		return value
	}

	block := params.Block
	if block <= 0 {
		block = domain.PortOffsetBlock
	}

	var at []int
	for _, n := range standaloneNumbers(params.Value) {
		gap := n.value - params.Base
		if gap < 0 || gap%block != 0 || gap/block > domain.PortCollisionHorizon {
			continue
		}
		at = append(at, n.at)
	}
	if len(at) != 1 {
		return params.Value
	}

	number := params.Value[at[0]:]
	return replaceAt(params.Value, at[0], numberLengthAt(number), params.Base)
}

type standaloneNumber struct {
	at    int
	value int
}

// standaloneNumbers finds every run of digits in value that is not part of a
// longer one, which is the same boundary rule the substitution uses.
func standaloneNumbers(value string) []standaloneNumber {
	var out []standaloneNumber
	for i := 0; i < len(value); {
		if !isDigitAt(value, i) {
			i++
			continue
		}
		end := i
		for isDigitAt(value, end) {
			end++
		}
		if n, err := strconv.Atoi(value[i:end]); err == nil {
			out = append(out, standaloneNumber{at: i, value: n})
		}
		i = end
	}
	return out
}

func numberLengthAt(value string) int {
	end := 0
	for isDigitAt(value, end) {
		end++
	}
	return end
}

// portOffsets locates port in value as a standalone number. The digit boundaries
// are the whole point: without them base 5432 matches inside 54321 and the
// substitution silently corrupts the value.
func portOffsets(value string, port int) []int {
	needle := strconv.Itoa(port)

	var offsets []int
	for from := 0; ; {
		i := strings.Index(value[from:], needle)
		if i < 0 {
			return offsets
		}
		at := from + i
		if !isDigitAt(value, at-1) && !isDigitAt(value, at+len(needle)) {
			offsets = append(offsets, at)
		}
		from = at + len(needle)
	}
}

func isDigitAt(value string, i int) bool {
	return i >= 0 && i < len(value) && value[i] >= '0' && value[i] <= '9'
}

// replaceAt swaps the width-character number at offset at for to.
func replaceAt(value string, at int, width int, to int) string {
	return value[:at] + strconv.Itoa(to) + value[at+width:]
}

func envPairByKey(lines []domain.EnvLine, key string) (domain.EnvLine, bool) {
	for _, line := range lines {
		if line.Kind == domain.EnvLinePair && line.Key == key {
			return line, true
		}
	}
	return domain.EnvLine{}, false
}

type ElideEnvValueParams struct {
	Value string
	// Width is the room the value has. Zero is a surface that could not measure
	// itself — a pipe, a test — and falls back to the default.
	Width int
}

// ElideEnvValue shortens a value for display. Cutting at the credentials
// separator is not only about width: a DATABASE_URL printed whole puts a
// password on screen, and the part worth reading is the host and the port.
//
// The width is an input because a named origin is long by nature —
// `http://admin.feat-x.monorepo.localhost:11080` is past fifty characters — and
// a value cut to a fixed budget on a wide terminal is unreadable for no reason.
func ElideEnvValue(params ElideEnvValueParams) string {
	width := params.Width
	if width <= 0 {
		width = domain.EnvValueDisplayWidth
	}

	value := params.Value
	if at := strings.LastIndex(value, domain.EnvCredentialsSeparator); at >= 0 {
		value = domain.Ellipsis + value[at:]
	}
	if len(value) <= width {
		return value
	}
	return value[:max(width-len(domain.Ellipsis), 0)] + domain.Ellipsis
}

type EnvPortCandidatesParams struct {
	// Lines is each configured env target's parsed content, keyed by target.
	Lines map[string][]domain.EnvLine
	Bases map[domain.PortRef]int
	// Existing are the links run.toml already declares, which are never offered
	// again — a re-run of the detection is additive, like the compose one.
	Existing []domain.EnvPortLink
	// JobsByDir is the job running in each directory. It answers the case an
	// already-shifted .env creates, where no value anchors a declared base.
	JobsByDir map[string]string
}

// EnvPortCandidates finds the .env keys whose value holds a declared base port,
// one link per port found. A CORS_ORIGIN listing two front-ends follows both:
// the key belongs to no single service, and asking wtm to pick one would leave
// the other pinned to the main checkout in every worktree.
func EnvPortCandidates(params EnvPortCandidatesParams) []domain.EnvPortLink {
	// Keyed by the port too: a key may follow several, and a re-run must offer
	// the ones run.toml does not already hold rather than none of them. A link
	// that names no job covers every job declaring that port name — it is how a
	// config written when only one did still reads.
	declared := func(link domain.EnvPortLink) bool {
		for _, existing := range params.Existing {
			if existing.File != link.File || existing.Key != link.Key || existing.Port != link.Port {
				continue
			}
			if existing.Job == "" || existing.Job == link.Job {
				return true
			}
		}
		return false
	}

	var candidates []domain.EnvPortLink
	for _, file := range sortedKeys(params.Lines) {
		for _, line := range params.Lines[file] {
			if line.Kind != domain.EnvLinePair {
				continue
			}
			if refs := portsIn(line.Value, params.Bases); len(refs) > 0 {
				for _, ref := range refs {
					link := domain.EnvPortLink{File: file, Key: line.Key, Job: ref.Job, Port: ref.Name}
					if declared(link) {
						continue
					}
					candidates = append(candidates, link)
				}
				continue
			}
			link, found := linkByDir(linkByDirParams{
				File: file, Line: line, Bases: params.Bases, JobsByDir: params.JobsByDir,
			})
			if found && !declared(link) {
				candidates = append(candidates, link)
			}
		}
	}
	return candidates
}

type linkByDirParams struct {
	File      string
	Line      domain.EnvLine
	Bases     map[domain.PortRef]int
	JobsByDir map[string]string
}

// linkByDir attaches a port key to the job running in the directory holding the
// file. It is the second rank on purpose: a value carrying a declared base says
// which port it follows, and only when none does is the directory the next best
// evidence. The key must name a port and hold one, so a URL or a placeholder is
// never guessed at.
func linkByDir(params linkByDirParams) (domain.EnvPortLink, bool) {
	if !IsPortKey(params.Line.Key) {
		return domain.EnvPortLink{}, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(params.Line.Value))
	if err != nil || value < domain.PortMin || value > domain.PortMax {
		return domain.EnvPortLink{}, false
	}

	job, found := params.JobsByDir[filepath.Dir(params.File)]
	if !found {
		return domain.EnvPortLink{}, false
	}
	for _, ref := range sortedPortRefs(params.Bases) {
		if ref.Job == job && ref.Name == params.Line.Key {
			return domain.EnvPortLink{
				File: params.File, Key: params.Line.Key, Job: job, Port: ref.Name, ByDir: true,
			}, true
		}
	}
	return domain.EnvPortLink{}, false
}

// solePortIn names the one declared port whose base sits exactly once in value.
// Two ports sharing a base cannot both be it, so neither is offered.
// portsIn names every declaration the value spells out exactly once. A port
// appearing twice in one value is left alone: two occurrences of one base are
// two different things the key means by it, and moving both together would be a
// guess.
func portsIn(value string, bases map[domain.PortRef]int) []domain.PortRef {
	var refs []domain.PortRef
	for _, candidate := range sortedPortRefs(bases) {
		if len(portOffsets(value, bases[candidate])) == 1 {
			refs = append(refs, candidate)
		}
	}
	return refs
}

// sortedPortRefs orders the declarations so a match is picked the same way twice.
func sortedPortRefs(bases map[domain.PortRef]int) []domain.PortRef {
	refs := make([]domain.PortRef, 0, len(bases))
	for ref := range bases {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Job != refs[j].Job {
			return refs[i].Job < refs[j].Job
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

// EnvPortPlanTouches reports whether a plan rewrites anything in one env target.
func EnvPortPlanTouches(plan domain.EnvPortPlan, target string) bool {
	for _, e := range EnvPortRewrites(plan) {
		if e.File == target {
			return true
		}
	}
	return false
}
