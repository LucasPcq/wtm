// Package portprobe answers one question about a port: is anything listening
// on it. It is the only place in wtm that opens a socket.
//
// It deliberately proves nothing about *who* listens. Ownership by pid is not
// available for the most common case — a compose job's listener is dockerd,
// never a descendant, and `docker compose up -d` leaves no process to ask — so
// a dial is what there is, and saying only what a dial can support is what
// keeps the report honest.
package portprobe

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type PollParams struct {
	Ports  []int
	Budget time.Duration
	// Settled ends the poll early. A healthy stack answers on the first round,
	// so only a failure ever spends the whole budget — which is what makes a
	// generous default cost nothing.
	Settled func(listening map[int]bool) bool
}

// Poll dials every port until they have settled or the budget runs out, and
// returns what answered. A port that never answers is absent from the map
// rather than false: the caller reads it either way.
func Poll(ctx context.Context, params PollParams) map[int]bool {
	listening := map[int]bool{}
	if len(params.Ports) == 0 || params.Budget <= 0 {
		return listening
	}

	deadline := time.Now().Add(params.Budget)
	ticker := time.NewTicker(domain.PortProbeInterval)
	defer ticker.Stop()

	for {
		for _, port := range params.Ports {
			if listening[port] {
				continue
			}
			if dial(port) {
				listening[port] = true
			}
		}

		if params.Settled(listening) || time.Now().After(deadline) {
			return listening
		}

		select {
		case <-ctx.Done():
			return listening
		case <-ticker.C:
		}
	}
}

// dial tries loopback on both families: a service bound to ::1 only would
// otherwise read as silent.
func dial(port int) bool {
	for _, host := range []string{domain.PortProbeHostV4, domain.PortProbeHostV6} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), domain.PortProbeDialTimeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}
