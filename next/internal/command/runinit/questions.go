package runinit

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller answers of run init, part by part; a list left nil takes what run.toml holds, else what the repository suggests, and a list given empty is none.
//
// What runs: the jobs started (Jobs, each a candidate's name), those of them that run once (Tasks), and how a root script that starts the apps runs (Runner).
// Shared or per worktree: the services one copy serves for every worktree (Shared), and what each worktree gets of each (Spaces, each service.name=, service.create= or service.remove=, its namespace's name and the commands that make and drop it, a service with no name getting nothing of its own); the tasks that change their data (Touches).
// Ports: each job's port, spelled job.VARIABLE=base, job= for one that binds none (Ports); the commands changed to read their port (Commands, job=command).
// Addresses: how the .env files spell an address (Addressing) and the jobs that answer under their own name (URLs).
// .env files: the files config.toml copies into a worktree besides its own (EnvCopy) and the keys linked to a job (Links, file#KEY).
// Profiles: each spelled name=job,job (Profiles) and the one run up starts by default (DefaultProfile).
// Then whether docker-compose.yml is rewritten to give each worktree its own ports and names (PatchCompose), and the worktree the plan previews (Preview, none: the first worktree that is not the main one).
type Request struct {
	Jobs           []string           `json:"jobs"`
	Tasks          []string           `json:"tasks"`
	Runner         Runner             `json:"runner"`
	Shared         []string           `json:"shared"`
	Spaces         []string           `json:"spaces"`
	Touches        []string           `json:"touches"`
	Ports          []string           `json:"ports"`
	Commands       []string           `json:"commands"`
	Addressing     project.Addressing `json:"addressing"`
	URLs           []string           `json:"urls"`
	EnvCopy        []string           `json:"env_copy"`
	Links          []string           `json:"links"`
	Profiles       []string           `json:"profiles"`
	DefaultProfile string             `json:"default_profile"`
	PatchCompose   *bool              `json:"patch_compose"`
	Preview        string             `json:"preview"`
}

// Runner is how a root script that starts the apps (turbo run dev) runs: each app its own job, or the script as one job that starts them.
type Runner string

// The runners.
const (
	OneByOne Runner = "one_by_one"
	Through  Runner = "through"
)

// Runners returns every runner, in the order a choice offers them.
func Runners() []Runner {
	return []Runner{OneByOne, Through}
}

// The paths of run init's fields, by which each surface words them.
const (
	PathJobs           = "jobs"
	PathTasks          = "tasks"
	PathRunner         = "runner"
	PathShared         = "shared"
	PathTouches        = "touches"
	PathPorts          = "ports"
	PathCommands       = "commands"
	PathAddressing     = "addressing"
	PathURLs           = "urls"
	PathEnvCopy        = "env_copy"
	PathLinks          = "links"
	PathProfiles       = "profiles"
	PathDefaultProfile = "default_profile"
	PathPatchCompose   = "patch_compose"
	PathPreview        = "preview"
	PathSpaces         = "spaces"
)

// The parts of a space, after its service and a dot.
const (
	SpaceName   = "name"
	SpaceCreate = "create"
	SpaceRemove = "remove"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case PathJobs:
		next.Jobs = filled(values)
	case PathTasks:
		next.Tasks = filled(values)
	case PathRunner:
		next.Runner, err = domain.One[Runner](path, values)
	case PathShared:
		next.Shared = filled(values)
	case PathTouches:
		next.Touches = filled(values)
	case PathPorts:
		next.Ports = filled(values)
	case PathCommands:
		next.Commands = filled(values)
	case PathAddressing:
		next.Addressing, err = domain.One[project.Addressing](path, values)
	case PathURLs:
		next.URLs = filled(values)
	case PathEnvCopy:
		next.EnvCopy = filled(values)
	case PathLinks:
		next.Links = filled(values)
	case PathProfiles:
		next.Profiles = filled(values)
	case PathDefaultProfile:
		next.DefaultProfile, err = domain.One[string](path, values)
	case PathPatchCompose:
		var yes bool
		yes, err = domain.Yes(path, values)
		next.PatchCompose = &yes
	case PathPreview:
		next.Preview, err = domain.One[string](path, values)
	case PathSpaces:
		next.Spaces = filled(values)
	default:
		return r, domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}
	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to run.toml or the detection.
func (r Request) Values(path string) []string {
	switch path {
	case PathJobs:
		return r.Jobs
	case PathTasks:
		return r.Tasks
	case PathRunner:
		return domain.Spelled(r.Runner)
	case PathShared:
		return r.Shared
	case PathTouches:
		return r.Touches
	case PathPorts:
		return r.Ports
	case PathCommands:
		return r.Commands
	case PathAddressing:
		return domain.Spelled(r.Addressing)
	case PathURLs:
		return r.URLs
	case PathEnvCopy:
		return r.EnvCopy
	case PathLinks:
		return r.Links
	case PathProfiles:
		return r.Profiles
	case PathDefaultProfile:
		return domain.Spelled(r.DefaultProfile)
	case PathPatchCompose:
		return domain.Said(r.PatchCompose)
	case PathPreview:
		return domain.Spelled(r.Preview)
	case PathSpaces:
		return r.Spaces
	}
	return nil
}

// filled are the values that are not blank, an empty list rather than nil, since a list given empty is an answer.
func filled(values []string) []string {
	kept := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}
	return kept
}

// ----------------------------------------------------------------------------
// The form

// Questions are the request settled on the facts, part by part, each answer as given, else as run.toml holds it, else as the repository suggests: the decision every surface shows and the plan is made from, and the request's problems.
type Questions struct {
	Decision Decision
	Request  Request
	Errors   []domain.FieldError
	ToCheck  []domain.FieldError
}

// NewQuestions settles the request on the facts.
func NewQuestions(f Facts, req Request) Questions {
	decision := f.decide(req)
	problems := decision.problems(f)
	return Questions{Decision: decision, Request: req, Errors: slices.DeleteFunc(slices.Clone(problems), toCheck), ToCheck: slices.DeleteFunc(problems, func(p domain.FieldError) bool { return !toCheck(p) })}
}

// Form is the questions as the schema a surface renders, part by part: what runs, shared or per worktree, ports, addresses, .env files, profiles; then the rewrite of docker-compose.yml and the worktree previewed. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	d := q.Decision
	fields := []domain.Field{
		domain.MultiSelectField{FieldSpec: d.spec(PathJobs), Picked: d.Picked, Options: options(names(d.Candidates))},
		domain.MultiSelectField{FieldSpec: d.spec(PathTasks), Picked: d.Tasks, Options: options(d.Picked)},
		domain.SelectField{FieldSpec: d.spec(PathRunner), Value: string(d.Runner), Options: domain.OptionsOf(Runners()), Closed: true},
		domain.MultiSelectField{FieldSpec: d.spec(PathShared), Picked: d.Shared, Options: options(d.Shareable)},
	}
	fields = append(fields,
		domain.TextListField{FieldSpec: d.spec(PathSpaces), Entries: spelledSpaces(d.Shared, d.Spaces), Options: options(spelledSpaces(d.Shared, d.ProposedSpaces))},
		domain.MultiSelectField{FieldSpec: d.spec(PathTouches), Picked: d.Touches, Options: options(d.TaskNames())},
		domain.TextListField{FieldSpec: d.spec(PathPorts), Entries: spelledPorts(d.Ports), Options: options(spelledPorts(d.ProposedPorts))},
		domain.TextListField{FieldSpec: d.spec(PathCommands), Entries: spelledCommands(d.Commands), Options: options(spelledCommands(d.Rewrites))},
		domain.SelectField{FieldSpec: d.spec(PathAddressing), Value: string(d.Addressing), Options: domain.OptionsOf([]project.Addressing{project.AddressNames, project.AddressPorts}), Closed: true},
		domain.MultiSelectField{FieldSpec: d.spec(PathURLs), Picked: d.URLs, Options: options(d.Addressable)},
		domain.MultiSelectField{FieldSpec: d.spec(PathEnvCopy), Picked: d.EnvCopy, Options: options(d.Uncopied)},
		domain.MultiSelectField{FieldSpec: d.spec(PathLinks), Picked: spelledLinks(d.Links), Options: options(spelledLinks(d.Linkable))},
		domain.TextListField{FieldSpec: d.spec(PathProfiles), Entries: spelledProfiles(d.Profiles), Options: options(spelledProfiles(d.ProposedProfiles))},
		domain.SelectField{FieldSpec: d.spec(PathDefaultProfile), Value: d.DefaultProfile, Options: options(profileNames(d.Profiles))},
		domain.ConfirmField{FieldSpec: d.spec(PathPatchCompose), Value: d.PatchCompose},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathPreview, Origin: d.Origins[PathPreview], Skip: domain.SkipFlagOnly}, Value: d.Preview, Options: options(d.Previewable)},
	)
	return domain.Form{Fields: fields, Problems: q.Errors, Sections: d.sections(sectionsParams{Fields: fields, Problems: q.Errors, ToCheck: q.ToCheck})}
}

// The sections of run init's form, in the order a surface shows them, each needing those before it.
const (
	SectionRuns      = "runs"
	SectionShared    = "shared"
	SectionPorts     = "ports"
	SectionAddresses = "addresses"
	SectionEnv       = "env"
	SectionProfiles  = "profiles"
)

// Sections are run init's sections, in order.
func Sections() []string {
	return []string{SectionRuns, SectionShared, SectionPorts, SectionAddresses, SectionEnv, SectionProfiles}
}

// sectionOf is the section of each field path.
var sectionOf = map[string]string{
	PathJobs: SectionRuns, PathTasks: SectionRuns, PathRunner: SectionRuns,
	PathShared: SectionShared, PathSpaces: SectionShared, PathTouches: SectionShared,
	PathPorts: SectionPorts, PathCommands: SectionPorts, PathPatchCompose: SectionPorts,
	PathAddressing: SectionAddresses, PathURLs: SectionAddresses,
	PathEnvCopy: SectionEnv, PathLinks: SectionEnv,
	PathProfiles: SectionProfiles, PathDefaultProfile: SectionProfiles,
}

// spec is the spec of the field at path, as the decision settled it.
func (d Decision) spec(path string) domain.FieldSpec {
	return domain.FieldSpec{Path: path, Origin: d.Origins[path], Skip: d.Skips[path], Section: sectionOf[path]}
}

type sectionsParams struct {
	Fields   []domain.Field
	Problems []domain.FieldError
	ToCheck  []domain.FieldError
}

// sections are the form's sections as its fields and problems stand, each waiting for the sections before it: a skipped one says why, by the skip of its first field (Shared or per worktree with no docker-compose.yml: no_compose); one with a problem to check, which never refuses the request and so is no problem of the form, is to check, its reason the problem's code and its params the problem's, the field's entry it names among them (path).
func (d Decision) sections(params sectionsParams) []domain.Section {
	sections := domain.SectionsOf(Sections(), params.Fields, params.Problems)
	for i := range sections {
		sections[i].WaitsFor = Sections()[:i]
		if sections[i].State == domain.SectionSkipped {
			for _, field := range params.Fields {
				if spec := field.Spec(); spec.Section == sections[i].ID && spec.Skip != "" {
					sections[i].Reason = string(spec.Skip)
					break
				}
			}
			continue
		}
		for _, problem := range append(slices.Clone(params.Problems), params.ToCheck...) {
			if sectionOf[pathOf(problem.Path)] == sections[i].ID {
				sections[i].State, sections[i].Reason = domain.SectionToCheck, problem.Code
				sections[i].Params = domain.Params{"path": problem.Path}
				maps.Copy(sections[i].Params, problem.Params)
				break
			}
		}
	}
	return sections
}

// pathOf is the field path a problem names, without the entry it points to: ports[2] is ports.
func pathOf(at string) string {
	path, _, _ := strings.Cut(at, "[")
	return path
}

func options(values []string) []domain.Option {
	found := make([]domain.Option, 0, len(values))
	for _, value := range values {
		found = append(found, domain.Option{Value: value})
	}
	return found
}

// spelledSpaces are the spaces of the services, each part service.part=value, a service with no name spelling none.
func spelledSpaces(services []string, spaces map[string]project.Namespace) []string {
	spelled := []string{}
	for _, service := range services {
		space := spaces[service]
		if space.Name == "" {
			continue
		}
		spelled = append(spelled, service+"."+SpaceName+"="+space.Name, service+"."+SpaceCreate+"="+space.Create)
		if space.Remove != "" {
			spelled = append(spelled, service+"."+SpaceRemove+"="+space.Remove)
		}
	}
	return spelled
}

func spelledPorts(ports []Port) []string {
	spelled := make([]string, 0, len(ports))
	for _, port := range ports {
		spelled = append(spelled, port.Spelled())
	}
	return spelled
}

// Spelled is the port as an answer spells it: job.VARIABLE=base, job= for a job that binds none.
func (p Port) Spelled() string {
	if p.Variable == "" {
		return p.Job + "="
	}
	return p.Job + "." + p.Variable + "=" + strconv.Itoa(p.Base)
}

func spelledCommands(commands map[string]string) []string {
	spelled := make([]string, 0, len(commands))
	for _, job := range sortedKeys(commands) {
		spelled = append(spelled, job+"="+commands[job])
	}
	return spelled
}

func spelledLinks(links []Link) []string {
	spelled := make([]string, 0, len(links))
	for _, link := range links {
		spelled = append(spelled, link.Spelled())
	}
	return spelled
}

// Spelled is the link as an answer spells it: file#KEY.
func (l Link) Spelled() string {
	return l.File + "#" + l.Key
}

func spelledProfiles(profiles []project.Profile) []string {
	spelled := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		spelled = append(spelled, profile.Name+"="+strings.Join(profile.Jobs, ","))
	}
	return spelled
}

func profileNames(profiles []project.Profile) []string {
	found := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		found = append(found, profile.Name)
	}
	return found
}

// ----------------------------------------------------------------------------
// Resolve

// Resolve settles the request on the facts, every field answered as NewQuestions answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Decision, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Decision{}, domain.Invalid(questions.Errors)
	}
	return questions.Decision, nil
}
