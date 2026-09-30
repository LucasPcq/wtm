package proxy

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ProbeParams struct {
	Port int
	// Timeout takes the default when zero.
	Timeout time.Duration
}

// Probe is which bind port answers behind Port, zero when what answers is not
// this proxy. The bind port in the reply is what separates a live redirection
// from one still pointing at a port the daemon no longer holds.
func Probe(params ProbeParams) int {
	timeout := params.Timeout
	if timeout <= 0 {
		timeout = time.Duration(domain.ProxyProbeTimeoutMs) * time.Millisecond
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://"+domain.ProxyLoopbackFmt+"/", params.Port), nil)
	if err != nil {
		return 0
	}
	req.Host = domain.ProxyProbeHost

	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	port, err := strconv.Atoi(resp.Header.Get(domain.ProxyProbeHeader))
	if err != nil {
		return 0
	}
	return port
}
