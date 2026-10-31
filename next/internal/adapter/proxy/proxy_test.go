package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARequestReachesTheJobItsHostNamesWithItsHostAndForwardedHeaders(t *testing.T) {
	port := backend(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host+" "+r.Header.Get("X-Forwarded-Host")+" "+r.Header.Get("X-Forwarded-Proto"))
	})
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: port, Job: "web"})

	resp, body := get(t, server, "Web.Feat.Acme.localhost:11080")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Web.Feat.Acme.localhost:11080 Web.Feat.Acme.localhost:11080 http", body, "the host as the browser asked, case aside")
}

func TestAnAddressAddedAgainReplacesItsPortAndOneRemovedIsNoLongerServed(t *testing.T) {
	old := backend(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "old") })
	current := backend(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "new") })
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: old, Job: "web"})

	server.Add(run.Address{Host: "web.feat.acme.localhost", Port: current, Job: "web"})
	_, body := get(t, server, "web.feat.acme.localhost")
	server.Remove(run.Address{Host: "WEB.feat.acme.localhost", Port: current, Job: "web"})
	gone, _ := get(t, server, "web.feat.acme.localhost")

	assert.Equal(t, "new", body)
	assert.Equal(t, http.StatusNotFound, gone.StatusCode)
}

func TestAnUnknownHostIsAnsweredWithTheNamesServed(t *testing.T) {
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: 3100, Job: "web", Worktree: "feat", Project: "acme"}, run.Address{Host: "api.feat.acme.localhost", Port: 3200, Job: "api", Worktree: "feat", Project: "acme"})

	resp, body := get(t, server, "docs.feat.acme.localhost")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "wtm: no job is published under docs.feat.acme.localhost\n\nRoutes wtm is currently serving:\n"+
		"  api.feat.acme.localhost  ->  localhost:3200 (job api, worktree feat of acme)\n"+
		"  web.feat.acme.localhost  ->  localhost:3100 (job web, worktree feat of acme)\n", body)
	_, none := get(t, serve(t), "docs.feat.acme.localhost")
	assert.Contains(t, none, "(none — start a job that publishes a named URL)")
}

func TestAJobThatDoesNotAnswerIsABadGateway(t *testing.T) {
	silent := closed(t)
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: silent, Job: "web"})

	resp, body := get(t, server, "web.feat.acme.localhost")

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, "wtm: job web is published under web.feat.acme.localhost but nothing answers on localhost:"+strconv.Itoa(silent)+"\n", body)
}

func TestAWebSocketIsUpgradedThroughTheProxy(t *testing.T) {
	port := backend(t, func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok || r.Header.Get("Upgrade") != "websocket" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
	})
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: port, Job: "web"})
	conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(server.Port()))
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: web.feat.acme.localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))
	require.NoError(t, err)
	status, err := bufio.NewReader(conn).ReadString('\n')

	require.NoError(t, err)
	assert.Contains(t, status, "101")
}

func TestAJobBoundToIPv6AloneIsReached(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	v6 := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "v6") })}}
	v6.Start()
	t.Cleanup(v6.Close)
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: portOf(t, listener.Addr()), Job: "web"})

	resp, body := get(t, server, "web.feat.acme.localhost")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "v6", body)
}

func TestAPortTakenMovesTheProxyToTheNextAndGivesUpPastItsSpan(t *testing.T) {
	busy := taken(t)

	moved := New()
	require.NoError(t, moved.Listen(busy))
	defer moved.Close()
	refused := New().listen(busy, 1)

	assert.NotEqual(t, busy, moved.Port())
	assert.Positive(t, moved.Port())
	assert.Error(t, refused)
}

func TestAPortNoBrowserOpensIsSteppedOver(t *testing.T) {
	err := New().listen(6000, 1)

	assert.ErrorContains(t, err, "no port a browser opens from 6000 to 6000")
}

func TestTheProbeTellsItsOwnProxyAnsweringOnAPortFromAnythingElse(t *testing.T) {
	server := serve(t)
	spoof := backend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Wtm-Proxy", strconv.Itoa(server.Port()))
		w.WriteHeader(http.StatusNoContent)
	})
	other := serve(t)

	assert.Equal(t, server.Port(), server.Probe(server.Port()))
	assert.Zero(t, server.Probe(spoof), "a process echoing its port")
	assert.Zero(t, server.Probe(other.Port()), "another wtm proxy")
	assert.Zero(t, server.Probe(closed(t)), "nothing")
}

func TestAHostOfAnotherSiteIsNeverToldTheNamesServed(t *testing.T) {
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: 3100, Job: "web", Worktree: "feat", Project: "acme"})

	resp, foreign := get(t, server, "rebound.example.com")
	_, loopback := get(t, server, "127.0.0.1:11080")
	_, v6 := get(t, server, "[::1]:11080")

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "wtm: no job is published under this host\n", foreign, "a page of a site rebound to the loopback reads nothing")
	assert.Contains(t, loopback, "web.feat.acme.localhost")
	assert.Contains(t, v6, "web.feat.acme.localhost")
}

func TestAProxyServesNoPortBeforeItListensNorOnceClosed(t *testing.T) {
	server := New()
	server.Add(run.Address{Host: "web.feat.acme.localhost", Port: 3100, Job: "web"})
	before := server.Port()
	require.NoError(t, server.Close(), "nothing to close")
	require.NoError(t, server.Listen(0))
	listening := server.Port()
	require.NoError(t, server.Close())

	assert.Zero(t, before)
	assert.Positive(t, listening)
	assert.Zero(t, server.Port())
}

func TestAJobRemovesItsAddressNeverOneAnotherJobTookItsHostOver(t *testing.T) {
	port := backend(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "b") })
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: 3100, Job: "a"})
	server.Add(run.Address{Host: "web.feat.acme.localhost", Port: port, Job: "b"})

	server.Remove(run.Address{Host: "web.feat.acme.localhost", Port: 3100, Job: "a"})

	resp, body := get(t, server, "web.feat.acme.localhost")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "b", body)
}

func TestWhatAClientSaysOfWhereItCameFromIsNeverForwarded(t *testing.T) {
	port := backend(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Header.Get("X-Forwarded-For")+"|"+r.Header.Get("Forwarded"))
	})
	server := serve(t, run.Address{Host: "web.feat.acme.localhost", Port: port, Job: "web"})
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(server.Port())+"/", nil)
	require.NoError(t, err)
	req.Host = "web.feat.acme.localhost"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("Forwarded", "for=203.0.113.7")

	resp, err := http.DefaultTransport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1|", string(body))
}
