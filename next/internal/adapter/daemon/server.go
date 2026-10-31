package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// writeTimeout bounds a write to a client: one that stopped reading is let go rather than holding a job's output.
const writeTimeout = time.Second

// errClientGone is the error of a write to a client already let go.
var errClientGone = errors.New("the client stopped reading")

// Jobs is what the server asks of the run daemon it serves.
type Jobs interface {
	List(workDir string) []run.Job
	Start(spec run.Spec, out func(run.Line)) (run.Job, error)
	Stop(req run.Stop) (run.Outcome, error)
	StopAll(req run.StopAll) ([]run.StoppedJob, error)
	Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error)
	Attach(req run.Window) (run.Session, error)
	Resize(req run.Window) error
}

// A Server answers the requests of the run daemon's protocol for its Jobs,
// stamping every answer with its build's Version; a shutdown is answered, then
// handed to Stop.
type Server struct {
	Jobs    Jobs
	Version string
	Stop    func()
}

// Serve answers the request line on conn, the connection it came from, read
// no further than that line: an attached client types into conn once it read
// its answer. A client that stops reading is let go: a write it does not take
// within a second fails, and every write after it is dropped, so a job's
// output never waits on it.
func (s Server) Serve(conn net.Conn, line []byte) {
	encoder := json.NewEncoder(conn)
	var writing sync.Mutex
	gone := false
	reply := func(a answer) error {
		a.Protocol, a.Version, a.PID = run.ProtocolVersion, s.Version, os.Getpid()
		writing.Lock()
		defer writing.Unlock()
		if gone {
			return errClientGone
		}
		_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err := encoder.Encode(a); err != nil {
			gone = true
			return err
		}
		return nil
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
	case actionLines:
		s.lines(conn, req, reply)
	case actionAttach:
		s.attach(conn, req, reply)
	case actionResize:
		err := s.Jobs.Resize(run.Window{Name: req.Name, WorkDir: req.WorkDir, Cols: req.Cols, Rows: req.Rows})
		_ = reply(answerOf(err, answer{Status: statusOK}))
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
		_ = reply(answer{Status: statusLine, At: stamp(line.At), Text: line.Text})
	})
	done := answer{Status: statusOK}
	// A start refused before any job ran has no job to report.
	if started.Name != "" {
		wire := jobOf(started)
		done.Job = &wire
	}
	_ = reply(answerOf(err, done))
}

// lines sends the lines of the job req names, then how it stands; a follow
// ends when the job does, or once the client leaves.
func (s Server) lines(conn net.Conn, req request, reply func(answer) error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A quiet job writes nothing a client gone would fail: its leaving is read on the connection.
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		cancel()
	}()

	tail := run.Tail{Name: req.Name, WorkDir: req.WorkDir, Lines: req.Tail, Follow: req.Follow}
	stood, err := s.Jobs.Lines(ctx, tail, func(line run.Line) {
		if reply(answer{Status: statusLine, At: stamp(line.At), Text: line.Text}) != nil {
			cancel()
		}
	})
	if ctx.Err() != nil {
		return
	}
	done := answer{Status: statusOK}
	if stood.Name != "" {
		wire := jobOf(stood)
		done.Job = &wire
	}
	_ = reply(answerOf(err, done))
}

// attach answers, then relays the job's terminal both ways on conn, raw:
// what it printed last, what it prints, and what the client types, until the
// job ends or the client leaves.
func (s Server) attach(conn net.Conn, req request, reply func(answer) error) {
	session, err := s.Jobs.Attach(run.Window{Name: req.Name, WorkDir: req.WorkDir, Cols: req.Cols, Rows: req.Rows})
	if reply(answerOf(err, answer{Status: statusOK})) != nil || err != nil {
		return
	}
	defer session.Release()

	left := make(chan struct{})
	go func() {
		defer close(left)
		input := session.Input
		if input == nil {
			input = io.Discard
		}
		_, _ = io.Copy(input, conn)
	}()
	// Each raw write gets its own deadline: the answer's would cut the terminal off a second after it opened.
	write := func(p []byte) error {
		_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		_, err := conn.Write(p)
		return err
	}
	if write(session.History) != nil {
		return
	}
	for {
		select {
		case chunk, open := <-session.Stream:
			if !open {
				return
			}
			if write(chunk) != nil {
				return
			}
		case <-left:
			return
		}
	}
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
