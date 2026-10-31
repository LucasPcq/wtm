package daemon

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// branchKey is the variable a job's environment names its branch with: the
// legacy index keeps no branch of its own.
const branchKey = "WTM_BRANCH"

// Index is the daemon's index of the jobs up, the file at Path, in the legacy
// wtm's format so a daemon of either takes over what the other left. Safe for
// concurrent use.
type Index struct {
	Path string
}

// state is the file: its version and the jobs up.
type state struct {
	Version int      `json:"version"`
	Jobs    []record `json:"jobs"`
}

// record is a job up, with the legacy wtm's keys; unset is this wtm's alone,
// which the legacy one drops.
type record struct {
	Name      string            `json:"name"`
	WorkDir   string            `json:"work_dir"`
	Config    config            `json:"config"`
	Profile   string            `json:"profile,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Unset     []string          `json:"unset,omitempty"`
	Routes    []route           `json:"routes,omitempty"`
	LogDir    string            `json:"log_dir,omitempty"`
	StartedAt time.Time         `json:"started_at,omitzero"`
	PID       int               `json:"pid,omitempty"`
	PGID      int               `json:"pgid,omitempty"`
	Joined    bool              `json:"joined,omitempty"`
	Attached  bool              `json:"attached,omitempty"`
	SharedDir string            `json:"shared_dir,omitempty"`
	MainHolds bool              `json:"main_holds,omitempty"`
	Origin    *eventOrigin      `json:"origin,omitempty"`
}

type route struct {
	Job  string `json:"job"`
	Host string `json:"host"`
	Port string `json:"port"`
}

func routesOf(routes []run.Route) []route {
	var wire []route
	for _, each := range routes {
		wire = append(wire, route{Job: each.Job, Host: each.Host, Port: each.Port})
	}
	return wire
}

func runRoutes(wire []route) []run.Route {
	var routes []run.Route
	for _, each := range wire {
		routes = append(routes, run.Route{Job: each.Job, Host: each.Host, Port: each.Port})
	}
	return routes
}

type eventOrigin struct {
	Repo struct {
		Root      string `json:"root"`
		CommonDir string `json:"common_dir"`
	} `json:"repo"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// Load returns the records of the index: none when it is absent, cannot be
// read (a daemon killed mid-write leaves the previous file, atomic writes
// aside) or is of another version, an older or a newer wtm's.
func (i *Index) Load() []run.Record {
	data, err := os.ReadFile(i.Path)
	if err != nil {
		return nil
	}
	var file state
	if json.Unmarshal(data, &file) != nil {
		return nil
	}
	if read, _ := run.IndexAccess(file.Version); !read {
		return nil
	}

	records := make([]run.Record, 0, len(file.Jobs))
	for _, r := range file.Jobs {
		records = append(records, r.runRecord())
	}

	return records
}

// Save replaces the index with records, atomically, unless a newer wtm wrote
// it: its record of what it started is left as it is.
func (i *Index) Save(records []run.Record) error {
	file := state{Version: run.IndexVersion, Jobs: make([]record, 0, len(records))}
	for _, r := range records {
		file.Jobs = append(file.Jobs, recordOf(r))
	}
	data, err := json.Marshal(file)
	if err != nil {
		return err
	}

	_, err = statefile.Update(context.Background(), statefile.UpdateParams{
		Path: i.Path,
		Perm: 0o600,
		Mutate: func(current []byte) (statefile.Change, error) {
			var held struct {
				Version int `json:"version"`
			}
			if json.Unmarshal(current, &held) == nil {
				if _, frozen := run.IndexAccess(held.Version); frozen {
					return statefile.Keep{}, nil
				}
			}
			return statefile.Write{Data: data}, nil
		},
	})

	return err
}

func recordOf(r run.Record) record {
	wire := record{Name: r.Spec.Job.Name, WorkDir: r.Spec.WorkDir, Config: configOf(r.Spec.Job), Profile: r.Spec.Profile, Env: r.Spec.Env, Unset: r.Spec.Unset, LogDir: r.Spec.LogDir, Routes: routesOf(r.Spec.Routes), StartedAt: r.StartedAt, PID: r.PID, PGID: r.PGID, Joined: r.Claim, SharedDir: r.SharedDir, MainHolds: r.MainHolds}
	if origin := r.Spec.Origin; origin != nil {
		wire.Origin = &eventOrigin{CorrelationID: origin.CorrelationID}
		wire.Origin.Repo.Root, wire.Origin.Repo.CommonDir = origin.Root, origin.CommonDir
	}

	return wire
}

func (r record) runRecord() run.Record {
	spec := run.Spec{Job: r.Config.job(), Profile: r.Profile, WorkDir: r.WorkDir, Env: r.Env, Unset: r.Unset, LogDir: r.LogDir, Routes: runRoutes(r.Routes)}
	if r.Origin != nil {
		spec.Origin = &run.Origin{Root: r.Origin.Repo.Root, CommonDir: r.Origin.Repo.CommonDir, Branch: r.Env[branchKey], CorrelationID: r.Origin.CorrelationID}
	}
	return run.Record{Spec: spec, StartedAt: r.StartedAt, PID: r.PID, PGID: r.PGID, Claim: r.Joined || r.Attached, SharedDir: r.SharedDir, MainHolds: r.MainHolds}
}
