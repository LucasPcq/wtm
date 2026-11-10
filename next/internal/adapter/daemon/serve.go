package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	// lockWait is how long a daemon starting waits for the lock: a client probing whether a daemon holds it takes it for an instant.
	lockWait  = 250 * time.Millisecond
	lockRetry = 10 * time.Millisecond
	// acceptRetry is the first wait after an accept that failed, doubling up to acceptRetryMax.
	acceptRetry    = 5 * time.Millisecond
	acceptRetryMax = time.Second

	actionPublish   = "publish"
	actionSubscribe = "subscribe"
)

// ErrRunning is the error of [Serve] when another daemon holds the lock beside the socket.
var ErrRunning = errors.New("another daemon holds the lock beside its socket")

// ServeParams is where [Serve] listens, SocketPath; how long it waits with no
// request in flight and nothing Busy before it stops, Idle; Ready, run once the
// lock is held and before it listens; Bus, which answers a request of the
// event bus (publish, subscribe), and Answer, which answers any other request
// line read from conn (stop stops the daemon); Busy, which keeps it from
// idling out while true; and Stopping, run once it stopped listening, before
// the requests in flight end. A nil hook does nothing.
type ServeParams struct {
	SocketPath string
	Idle       time.Duration
	Ready      func()
	Bus        func(ctx context.Context, conn net.Conn, line []byte)
	Answer     func(conn net.Conn, line []byte, stop func())
	Busy       func() bool
	Stopping   func()
}

// Serve runs wtm's daemon on params.SocketPath until ctx is done or it idles
// out, one request per connection. Only one daemon serves a socket, holding
// the lock beside it until its process exits; Serve returns [ErrRunning] when
// another one holds it.
func Serve(ctx context.Context, params ServeParams) error {
	lock, err := statefile.TryLock(lockPath(params.SocketPath), domain.LockExclusive)
	for waited := time.Duration(0); errors.Is(err, statefile.ErrBusy) && waited < lockWait; waited += lockRetry {
		time.Sleep(lockRetry)
		lock, err = statefile.TryLock(lockPath(params.SocketPath), domain.LockExclusive)
	}
	if errors.Is(err, statefile.ErrBusy) {
		return ErrRunning
	}
	if err != nil {
		return err
	}
	defer lock.Close()

	if params.Ready != nil {
		params.Ready()
	}
	// The lock proves no daemon listens on a socket file left behind: one that died without removing it.
	_ = os.Remove(params.SocketPath)
	listener, err := listenOwned(params.SocketPath)
	if err != nil {
		return err
	}
	defer os.Remove(params.SocketPath)

	ctx, stop := context.WithCancel(ctx)
	defer stop()
	s := &server{params: params, last: time.Now(), stop: stop}
	go s.idleOut(ctx)
	context.AfterFunc(ctx, func() { _ = listener.Close() })

	var served sync.WaitGroup
	defer served.Wait()
	backoff := acceptRetry
	for {
		conn, err := listener.Accept()
		if err == nil {
			backoff = acceptRetry
			served.Go(func() { s.serve(ctx, conn) })
			continue
		}
		if ctx.Err() != nil {
			if params.Stopping != nil {
				params.Stopping()
			}
			return nil
		}
		// A failed accept (too many open files) is no reason to stop the jobs a daemon holds: wait, then accept again, as net/http does.
		time.Sleep(backoff)
		backoff = min(2*backoff, acceptRetryMax)
	}
}

// listenOwned listens on the unix socket at path, which only this user may connect
// to: a request on it runs a command as this user. The umask is narrowed
// around its creation, so the socket is never open to others, even for an
// instant; nothing else of this process creates a file meanwhile, its
// requests not being served yet.
func listenOwned(path string) (net.Listener, error) {
	umask := syscall.Umask(0o077)
	listener, err := net.Listen("unix", path)
	syscall.Umask(umask)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}

	return listener, nil
}

func lockPath(socketPath string) string {
	return filepath.Join(filepath.Dir(socketPath), lockName)
}

// A server is a daemon serving: what it answers with, the requests in flight
// and when the last one ended.
type server struct {
	params ServeParams
	stop   func()

	mu     sync.Mutex
	active int
	last   time.Time
}

// idleOut stops the daemon once it has had no request in flight and nothing
// busy for its Idle.
func (s *server) idleOut(ctx context.Context) {
	ticker := time.NewTicker(max(s.params.Idle/4, time.Millisecond))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		s.mu.Lock()
		idle := s.active == 0 && time.Since(s.last) >= s.params.Idle
		s.mu.Unlock()
		if idle && (s.params.Busy == nil || !s.params.Busy()) {
			s.stop()
			return
		}
	}
}

// busy records a request starting (by 1) or ending (by -1).
func (s *server) busy(by int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active += by
	s.last = time.Now()
}

// serve answers the one request conn carries; a subscription is one request
// for as long as it lasts.
func (s *server) serve(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return
	}
	s.busy(1)
	defer s.busy(-1)

	var req struct {
		Action string `json:"action"`
	}
	// A line that is no request is Answer's to refuse.
	_ = json.Unmarshal(line, &req)
	switch {
	case (req.Action == actionPublish || req.Action == actionSubscribe) && s.params.Bus != nil:
		s.params.Bus(ctx, conn, line)
	case s.params.Answer != nil:
		// What the client sent past its request line is in the reader: an attached terminal's first keys.
		s.params.Answer(buffered{Conn: conn, reader: reader}, line, s.stop)
	}
}

// buffered is a connection read through the reader that read its request.
type buffered struct {
	net.Conn
	reader *bufio.Reader
}

func (b buffered) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}
