package process

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

var daemonStartTimeout = time.Duration(domain.DaemonStartTimeoutSeconds) * time.Second

// Client communicates with the daemon over a Unix socket.
type Client struct {
	socketPath string
	// skipVersionCheck is set for the length of one SendUnchecked call. A Client
	// is a per-command value dialing one connection at a time, never a shared
	// pool, so the flag has no other conversation to leak into.
	skipVersionCheck bool
	// versionChecked keeps the preflight to once per client: a `run up` sends a
	// request per job, and asking again between two of them would answer the
	// same thing at the cost of a round-trip each.
	versionChecked bool
}

// NewClient creates a client for the given socket path.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

// Send sends a request to the daemon and returns the first terminal response
// (StatusOK, StatusDone, or StatusError). Intermediate StatusOutput chunks
// are discarded — use SendStream when the caller wants to forward them.
func (c *Client) Send(ctx context.Context, req Request) (Response, error) {
	return c.SendStream(ctx, req, nil)
}

// SendUnchecked is Send without the version guard, for the two commands whose
// job is that divergence: reporting it (`run daemon status`) and ending it
// (`run daemon stop`/`restart`). Every other caller wants Send — talking to a
// daemon of another build is what this whole guard exists to prevent.
func (c *Client) SendUnchecked(ctx context.Context, req Request) (Response, error) {
	c.skipVersionCheck = true
	defer func() { c.skipVersionCheck = false }()
	return c.SendStream(ctx, req, nil)
}

// SendStream sends the request and iterates over every response. `onOutput`
// is invoked with each StatusOutput chunk's Data (useful for streaming task
// stdout/stderr to the user). Returns the first terminal response
// (StatusOK, StatusDone, or StatusError) received.
//
// It is given up on when ctx is done: the connection is closed, which unblocks
// the read, and the call returns the context's error. The daemon is not told
// anything — the job it is running is untouched, and only this conversation
// about it ends.
func (c *Client) SendStream(ctx context.Context, req Request, onOutput func([]byte)) (Response, error) {
	if err := c.preflight(ctx, req); err != nil {
		return Response{}, err
	}
	return c.send(ctx, req, onOutput)
}

// preflight settles the version question before a request that changes
// something is sent. Reading the version off the answer is too late: the daemon
// acts on a request as it decodes it, so a `run down` served by a daemon of
// another build has already torn the stack down by the time the client refuses
// to believe its answer. A listing costs one round-trip on a unix socket and
// changes nothing, which is what makes it safe to ask first.
func (c *Client) preflight(ctx context.Context, req Request) error {
	if c.skipVersionCheck || c.versionChecked || !mutates(req.Action) || versionTolerant(req.Action) {
		return nil
	}
	c.versionChecked = true

	resp, err := c.send(ctx, Request{Action: ActionList}, nil)
	if err != nil {
		return err
	}
	return checkVersion(resp)
}

// versionTolerant names the requests every build has answered the same way since
// the first daemon: listing, stopping and shutting down. An older daemon must
// always be listable and stoppable — it is the way out of the mismatch itself.
func versionTolerant(action RequestAction) bool {
	switch action {
	case ActionList, ActionStop, ActionStopAll, ActionShutdown:
		return true
	default:
		return false
	}
}

// mutates says whether the daemon does something irreversible on decoding this
// request. A listing does not, and neither does an attach — it subscribes to
// output a job is producing either way.
func mutates(action RequestAction) bool {
	switch action {
	case ActionStart, ActionStop, ActionStopAll, ActionResize, ActionShutdown:
		return true
	default:
		return false
	}
}

func (c *Client) send(ctx context.Context, req Request, onOutput func([]byte)) (Response, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	defer context.AfterFunc(ctx, func() { conn.Close() })()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return Response{}, fmt.Errorf("send request: %w", err)
	}

	for {
		var resp Response
		if err := decoder.Decode(&resp); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return Response{}, ctxErr
			}
			return Response{}, fmt.Errorf("read response: %w", err)
		}
		if !c.skipVersionCheck && !versionTolerant(req.Action) {
			if err := checkVersion(resp); err != nil {
				return Response{}, err
			}
		}
		if resp.Status == StatusOutput {
			if onOutput != nil && len(resp.Data) > 0 {
				onOutput(resp.Data)
			}
			continue
		}
		resp.Jobs = rules.CurrentJobInfos(resp.Jobs)
		return resp, nil
	}
}

// checkVersion refuses a daemon built from another version of wtm. It is the
// daemon that runs the jobs, so an older one silently serves its own behaviour —
// the fix that made a client print the right port never reaches the process
// binding it. Refusing beats warning: the user is told which two versions are in
// play and how to get out, once, instead of being handed an outcome that looks
// right.
func checkVersion(resp Response) error {
	if resp.Version == domain.Version {
		return nil
	}
	return fmt.Errorf("%w: %s", domain.ErrDaemonVersionMismatch, rules.DaemonVersionMismatch(rules.DaemonVersionMismatchParams{
		Client: domain.Version,
		Daemon: resp.Version,
	}))
}

// AttachParams holds inputs for attaching to a service.
type AttachParams struct {
	Name    string
	WorkDir string
	Cols    int
	Rows    int
}

// Attach sends an attach request and returns the raw connection on success.
// The caller is responsible for closing the connection.
func (c *Client) Attach(params AttachParams) (net.Conn, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{
		Action:  ActionAttach,
		Name:    params.Name,
		WorkDir: params.WorkDir,
		Cols:    params.Cols,
		Rows:    params.Rows,
	}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send attach request: %w", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read attach response: %w", err)
	}

	if !c.skipVersionCheck {
		if err := checkVersion(resp); err != nil {
			conn.Close()
			return nil, err
		}
	}

	if resp.Status != StatusOK {
		conn.Close()
		return nil, fmt.Errorf("attach failed: %s", resp.Message)
	}

	held, err := io.ReadAll(decoder.Buffered())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read attach response: %w", err)
	}

	return prefixedConn{
		Conn:   conn,
		reader: io.MultiReader(bytes.NewReader(afterResponseDelimiter(held)), conn),
	}, nil
}

// afterResponseDelimiter drops the newline the encoder writes to close the
// attach response, which the decoder leaves in its buffer. Everything past it
// is the job's own output.
func afterResponseDelimiter(held []byte) []byte {
	held = bytes.TrimPrefix(held, []byte("\r"))
	return bytes.TrimPrefix(held, []byte("\n"))
}

// prefixedConn replays what the decoder read past the create command response before
// the rest of the connection. The daemon writes the job's buffered history
// right behind that response, so a single read commonly carries both, and
// whatever the decoder kept would otherwise be dropped with it.
type prefixedConn struct {
	net.Conn
	reader io.Reader
}

func (c prefixedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

// Resize asks the daemon to size a job's PTY to the pane rendering it. It
// dials its own connection rather than reusing the one Attach returned, which
// is a raw byte stream feeding the job's stdin from the moment it is accepted.
func (c *Client) Resize(ctx context.Context, params ResizeParams) error {
	resp, err := c.Send(ctx, Request{
		Action:  ActionResize,
		Name:    params.Name,
		WorkDir: params.WorkDir,
		Cols:    params.Cols,
		Rows:    params.Rows,
	})
	if err != nil {
		return err
	}
	if resp.Status != StatusOK {
		return fmt.Errorf("resize failed: %s", resp.Message)
	}
	return nil
}

// IsDaemonRunning checks if a daemon is listening on the socket.
func IsDaemonRunning(socketPath string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// spawnDaemon is StartDaemon, a variable so a test never forks its own binary.
var spawnDaemon = StartDaemon

// EnsureDaemon checks if the daemon is running; if not, starts it and waits.
func EnsureDaemon(ctx context.Context, params DaemonParams) error {
	if IsDaemonRunning(params.SocketPath) {
		return nil
	}
	if err := awaitDaemonGone(ctx, params.SocketPath); err != nil {
		return err
	}
	if IsDaemonRunning(params.SocketPath) {
		return nil
	}

	if err := spawnDaemon(params); err != nil {
		return err
	}

	return poll(ctx, pollParams{
		Timeout: daemonStartTimeout,
		Done:    func() bool { return IsDaemonRunning(params.SocketPath) },
		Expired: fmt.Errorf("daemon did not start within %v", daemonStartTimeout),
	})
}

// AwaitDaemonStopped waits for the socket to stop answering, so a restart never
// races the exit it just asked for.
func AwaitDaemonStopped(ctx context.Context, socketPath string) error {
	return poll(ctx, pollParams{
		Timeout: daemonStartTimeout,
		Done:    func() bool { return !IsDaemonRunning(socketPath) },
		Expired: fmt.Errorf("daemon did not stop within %v", daemonStartTimeout),
	})
}

type pollParams struct {
	Timeout time.Duration
	Done    func() bool
	Expired error
}

// poll asks Done every DaemonPollInterval until it holds, the timeout runs out,
// or ctx is done — an interrupted command stops waiting on the daemon at once.
func poll(ctx context.Context, params pollParams) error {
	ctx, cancel := context.WithTimeout(ctx, params.Timeout)
	defer cancel()
	ticker := time.NewTicker(domain.DaemonPollInterval)
	defer ticker.Stop()
	for !params.Done() {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return params.Expired
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

// StartDaemon forks "wtm daemon" as a detached background process. The proxy
// port travels on the command line because only a client can read the user's
// global config — the daemon is global and outlives any one of them.
func StartDaemon(params DaemonParams) error {
	// Under go test the executable is the test binary: forked as "daemon" it
	// runs the whole suite again, which forks again, and so on until the
	// machine gives out. No test can be served by it either way.
	if testing.Testing() {
		return domain.ErrDaemonForkInTest
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}

	args := []string{domain.CmdDaemon}
	if params.ProxyPort > 0 {
		args = append(args, "--"+domain.FlagProxyPort, strconv.Itoa(params.ProxyPort))
	}
	cmd := exec.Command(exePath, args...)
	cmd.Env = daemonEnv(os.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	return startDetached(cmd)
}

// daemonEnv drops what belongs to the one command that started the daemon: its
// jobs run long after, for other callers.
func daemonEnv(environ []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool {
		return strings.HasPrefix(entry, domain.EnvCorrelationID+"=")
	})
}

// startDetached never waits on the daemon, but reaps it: a client that lives
// as long as the daemon — `wtm events`, `wtm ui` — stays its parent, and a
// daemon stopped under it would otherwise linger as a zombie that every
// "has it exited yet" check still finds.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
