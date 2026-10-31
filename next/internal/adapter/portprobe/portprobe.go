// Package portprobe tells which ports of this machine something answers on: the check a start makes that each port its jobs declare was bound.
package portprobe

import (
	"context"
	"net"
	"strconv"
	"time"
)

// every is how often a port nothing answers on yet is dialled again.
const every = 200 * time.Millisecond

// dialTimeout bounds one dial: a port on the loopback answers at once or not at all.
const dialTimeout = 100 * time.Millisecond

// Listening dials each of ports on the loopback until something answers on every one, within, or ctx ends, and reports which answered.
func Listening(ctx context.Context, ports []int, within time.Duration) map[int]bool {
	answered := make(map[int]bool, len(ports))
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	for {
		for _, port := range ports {
			if !answered[port] && answers(ctx, port) {
				answered[port] = true
			}
		}
		if len(answered) == len(ports) {
			return answered
		}
		select {
		case <-ctx.Done():
			return answered
		case <-time.After(every):
		}
	}
}

func answers(ctx context.Context, port int) bool {
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
