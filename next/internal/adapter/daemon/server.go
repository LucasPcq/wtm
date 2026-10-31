package daemon

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Jobs is what the server asks of the run daemon it serves.
type Jobs interface {
	List(workDir string) []run.Job
	Start(spec run.Spec, out func(run.Line)) (run.Job, error)
	Stop(req run.Stop) (run.Outcome, error)
	StopAll(req run.StopAll) ([]run.StoppedJob, error)
}

// A Server answers the requests of the run daemon's protocol for its Jobs,
// stamping every answer with its build's Version; a shutdown is answered, then
// handed to Stop.
type Server struct {
	Jobs    Jobs
	Version string
	Stop    func()
}

// Serve answers the request line on conn, the connection it came from.
func (s Server) Serve(conn io.Writer, line []byte) {
	encoder := json.NewEncoder(conn)
	var writing sync.Mutex
	reply := func(a answer) error {
		a.Protocol, a.Version, a.PID = run.ProtocolVersion, s.Version, os.Getpid()
		writing.Lock()
		defer writing.Unlock()
		return encoder.Encode(a)
	}

	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		_ = reply(answer{Status: statusError, Code: run.CodeInvalidRequest, Message: "invalid request: " + err.Error()})
		return
	}

	switch req.Action {
	case actionList:
		_ = reply(answer{Status: statusOK, Jobs: jobsOf(s.Jobs.List(req.WorkDir))})
	case actionStart:
		s.start(req, reply)
	case actionStop:
		outcome, err := s.Jobs.Stop(run.Stop{Name: req.Name, WorkDir: req.WorkDir, CorrelationID: req.CorrelationID})
		_ = reply(answerOf(err, answer{Status: statusOK, Outcome: string(outcome)}))
	case actionStopAll:
		stopped, err := s.Jobs.StopAll(run.StopAll{WorkDir: req.WorkDir, CorrelationID: req.CorrelationID})
		jobs := make([]job, 0, len(stopped))
		for _, each := range stopped {
			wire := jobOf(each.Job)
			wire.Outcome = string(each.Outcome)
			jobs = append(jobs, wire)
		}
		_ = reply(answerOf(err, answer{Status: statusOK, Jobs: jobs}))
	case actionShutdown:
		_ = reply(answer{Status: statusOK, Message: "daemon stopping"})
		s.Stop()
	default:
		_ = reply(answer{Status: statusError, Code: run.CodeUnknownAction, Message: "unknown action: " + req.Action})
	}
}

// start starts the job req asks for and sends each line it prints, then how
// the start ended.
func (s Server) start(req request, reply func(answer) error) {
	if req.Spec == nil || req.Spec.Job.Name == "" {
		_ = reply(answer{Status: statusError, Code: run.CodeInvalidRequest, Message: "a start names its job"})
		return
	}

	// A client that left stops reading, not the job: its lines are dropped.
	started, err := s.Jobs.Start(req.Spec.runSpec(), func(line run.Line) {
		_ = reply(answer{Status: statusLine, At: line.At, Text: line.Text})
	})
	done := answer{Status: statusOK}
	// A start refused before any job ran has no job to report.
	if started.Name != "" {
		wire := jobOf(started)
		done.Job = &wire
	}
	_ = reply(answerOf(err, done))
}

// answerOf is the answer of a request that failed with err, ok otherwise.
func answerOf(err error, ok answer) answer {
	if err == nil {
		return ok
	}
	failure := &run.Error{Code: run.CodeLaunchFailed, Message: err.Error()}
	errors.As(err, &failure)
	ok.Status, ok.Code, ok.Message, ok.ExitCode = statusError, failure.Code, failure.Message, failure.ExitCode

	return ok
}

func jobsOf(jobs []run.Job) []job {
	wire := make([]job, 0, len(jobs))
	for _, j := range jobs {
		wire = append(wire, jobOf(j))
	}

	return wire
}
