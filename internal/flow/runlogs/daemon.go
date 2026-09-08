package runlogs

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type ServiceParams struct {
	SocketPath string
}

// NewService binds this seam to the running daemon. The caller is the one that
// made sure it is up — this package never starts it.
func NewService(params ServiceParams) Service {
	return &daemonService{client: process.NewClient(params.SocketPath)}
}

type daemonService struct {
	client *process.Client
}

func (s *daemonService) Start(ctx context.Context, req StartRequest) (StartResult, error) {
	resp, err := s.client.SendStreamContext(ctx, process.Request{
		Action:  process.ActionStart,
		Job:     &req.Job,
		WorkDir: req.WorkDir,
		LogDir:  req.LogDir,
		Env:     req.Env,
		Routes:  req.Routes,
	}, req.OnOutput)
	if err != nil {
		return StartResult{}, err
	}
	if resp.Status == process.StatusError {
		return StartResult{Refused: true, Message: resp.Message, ExitCode: resp.ExitCode}, nil
	}
	return StartResult{ExitCode: resp.ExitCode, Ports: resp.Ports, ProxyPort: resp.ProxyPort, PublicPort: resp.ProxyPublicPort}, nil
}

func (s *daemonService) List(workDir string) ([]domain.JobInfo, error) {
	resp, err := s.client.Send(process.Request{Action: process.ActionList, WorkDir: workDir})
	if err != nil {
		return nil, err
	}
	if resp.Status == process.StatusError {
		return nil, fmt.Errorf("list jobs: %s", resp.Message)
	}
	return resp.Jobs, nil
}

func (s *daemonService) Attach(req AttachRequest) (Stream, error) {
	conn, err := s.client.Attach(process.AttachParams{
		Name:    req.Name,
		WorkDir: req.WorkDir,
		Cols:    req.Size.Cols,
		Rows:    req.Size.Rows,
	})
	if err != nil {
		return nil, err
	}
	return newConnStream(connStreamParams{
		Conn:    conn,
		Client:  s.client,
		Name:    req.Name,
		WorkDir: req.WorkDir,
	}), nil
}

func (s *daemonService) Tail(req TailRequest) ([]string, error) {
	return process.TailJobLog(process.TailParams{LogDir: req.LogDir, Job: req.Job, Lines: req.Lines})
}

type connStreamParams struct {
	Conn    net.Conn
	Client  *process.Client
	Name    string
	WorkDir string
}

// connStream carries one attached job: the connection is a raw byte stream in
// both directions once the daemon accepted it, so resizing takes its own
// connection rather than sending JSON down this one.
type connStream struct {
	conn    net.Conn
	client  *process.Client
	name    string
	workDir string

	chunks chan []byte
	done   chan struct{}
	once   sync.Once
}

func newConnStream(params connStreamParams) *connStream {
	stream := &connStream{
		conn:    params.Conn,
		client:  params.Client,
		name:    params.Name,
		workDir: params.WorkDir,
		chunks:  make(chan []byte, domain.JobStreamQueueChunks),
		done:    make(chan struct{}),
	}
	go stream.read()
	return stream
}

func (s *connStream) Chunks() <-chan []byte { return s.chunks }

func (s *connStream) Write(p []byte) error {
	if err := s.subscribed(); err != nil {
		return err
	}
	_, err := s.conn.Write(p)
	return err
}

func (s *connStream) Resize(size Size) error {
	if err := s.subscribed(); err != nil {
		return err
	}
	if s.client == nil {
		return fmt.Errorf("resize %s: stream has no daemon client", s.name)
	}
	return s.client.Resize(process.ResizeParams{
		Name:    s.name,
		WorkDir: s.workDir,
		Cols:    size.Cols,
		Rows:    size.Rows,
	})
}

// subscribed makes Close a barrier for everything this stream can ask of the
// job. Resize in particular reaches the daemon on its own connection, so a
// closed stream would still be obeyed: a surface that destroys a pane and then
// runs the resize command it had already scheduled would size the job's PTY to
// a pane nobody is looking at, under the eyes of the panes that are left.
func (s *connStream) subscribed() error {
	select {
	case <-s.done:
		return fmt.Errorf("%w: %s", domain.ErrJobStreamClosed, s.name)
	default:
		return nil
	}
}

func (s *connStream) Close() error {
	var err error
	s.once.Do(func() {
		close(s.done)
		err = s.conn.Close()
	})
	return err
}

func (s *connStream) read() {
	defer close(s.chunks)

	buf := make([]byte, domain.JobStreamChunkBytes)
	for {
		n, err := s.conn.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case s.chunks <- chunk:
			case <-s.done:
				return
			}
		}
		if err != nil {
			return
		}
	}
}
