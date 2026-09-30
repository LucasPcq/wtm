package proxy

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TargetFunc names the port the run proxy is really on. It is a function and
// not a number because the daemon may fall back to another port, or restart on
// one: baking a port into the LaunchAgent would make every such move break the
// privileged port until the agent was reinstalled.
type TargetFunc func() (int, error)

type ForwardParams struct {
	Listeners []net.Listener
	// Target resolves where to relay. The forwarder speaks no HTTP: the Host
	// header it would need to read is the proxy's business, and a byte relay
	// keeps websockets and streaming working for free.
	Target TargetFunc
}

// Forward relays every connection the privileged listeners accept to the run
// proxy, and blocks until they all stop accepting.
func Forward(params ForwardParams) error {
	if len(params.Listeners) == 0 {
		return domain.ErrProxyNoListeners
	}
	if params.Target == nil {
		return domain.ErrProxyNoTarget
	}

	var wg sync.WaitGroup
	for _, listener := range params.Listeners {
		wg.Add(1)
		go func(l net.Listener) {
			defer wg.Done()
			acceptLoop(l, params.Target)
		}(listener)
	}
	wg.Wait()
	return nil
}

func acceptLoop(listener net.Listener, target TargetFunc) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go relay(conn, target)
	}
}

func relay(from net.Conn, target TargetFunc) {
	defer from.Close()

	port, err := target()
	if err != nil {
		return
	}
	to, err := net.Dial("tcp", fmt.Sprintf(domain.ProxyLoopbackFmt, port))
	if err != nil {
		return
	}
	defer to.Close()

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(to, from)
		closeWrite(to)
		close(done)
	}()
	_, _ = io.Copy(from, to)
	closeWrite(from)
	<-done
}

// closeWrite half-closes so the peer sees EOF on a response that ends with the
// connection, instead of hanging until a timeout.
func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}

type cachedTarget struct {
	resolve TargetFunc
	ttl     time.Duration

	mu   sync.Mutex
	port int
	at   time.Time
}

// Cached spares the daemon a round-trip per connection without letting the
// forwarder keep aiming at a port the daemon has left.
func Cached(resolve TargetFunc, ttl time.Duration) TargetFunc {
	c := &cachedTarget{resolve: resolve, ttl: ttl}
	return c.get
}

func (c *cachedTarget) get() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.port != 0 && time.Since(c.at) < c.ttl {
		return c.port, nil
	}
	port, err := c.resolve()
	if err != nil {
		return 0, err
	}
	c.port, c.at = port, time.Now()
	return port, nil
}
