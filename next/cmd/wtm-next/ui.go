package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	reposjson "github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/adapter/webserver"
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/web"
	"github.com/spf13/cobra"
)

const (
	uiCommand    = "ui"
	serveCommand = "serve"
	flagNoOpen   = "no-open"
	flagIdle     = "idle"

	// webIdle is how long the server waits with no client before it stops.
	webIdle = 30 * time.Minute
	// webPort is the port the server listens on when it is free, so that the page keeps its origin from one start to the next; another one otherwise.
	webPort = 4777
	// webStart bounds the wait for a server to answer once started, or to let go of its lock once asked to stop.
	webStart = 10 * time.Second
	webPoll  = 50 * time.Millisecond
	// webAsk bounds a question to a running server.
	webAsk     = time.Second
	tokenBytes = 32
)

// errServerInTest refuses to start a server from a test: the executable is the test binary.
var errServerInTest = errors.New("no web server is started from a test")

// uiCmd opens the web interface on the repository of the directory it runs in: it reuses the server of this machine when one of this version answers, asks one of another version to stop, starts one when none answers, then prints the page's address and opens it through a page only its user reads, so that the token never shows in the browser's command line.
func uiCmd(env cli.Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   uiCommand,
		Short: "Open the web interface, starting its server when none runs",
		Long:  "Open the web interface in the browser, on the repository of the current directory. One server serves every repository wtm knows on this machine: ui starts it when none runs, reuses it, and restarts one of another version. It stops after 30 minutes without a client.",
		Args:  cobra.NoArgs,
	}
	noOpen := cmd.Flags().Bool(flagNoOpen, false, "print the page's address without opening a browser")
	idle := cmd.Flags().Duration(flagIdle, webIdle, "how long a server this command starts waits without a client before it stops")
	_ = cmd.Flags().MarkHidden(flagIdle)

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := positive(*idle); err != nil {
			return err
		}
		record, err := ensureServer(cmd.Context(), *idle)
		if err != nil {
			return err
		}

		page := pageURL(record, repoID(cmd.Context(), env.ProjectDir))
		fmt.Fprintln(cmd.OutOrStdout(), page)
		if *noOpen {
			return nil
		}
		launch, err := webserver.Files{Dir: webserver.Dir()}.Launch(cmd.Context(), page)
		if err != nil {
			return err
		}
		return shell.Detached(shell.DetachedParams{Name: browser(), Args: []string{launch}, Env: os.Environ()})
	}
	cmd.AddCommand(serveCmd())
	return cmd
}

// serveCmd is the hidden command that runs the web server in a process of its own: ui starts it detached. A second one finds the lock held and leaves at once, saying so.
func serveCmd() *cobra.Command {
	cmd := &cobra.Command{Use: serveCommand, Short: "Run the web server ui opens", Hidden: true, Args: cobra.NoArgs}
	idle := cmd.Flags().Duration(flagIdle, webIdle, "how long the server waits without a client before it stops")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := positive(*idle); err != nil {
			return err
		}
		files := webserver.Files{Dir: webserver.Dir()}
		release, err := files.Lock()
		if errors.Is(err, webserver.ErrRunning) {
			return fmt.Errorf("a process holds web.lock in %s: %w", files.Dir, err)
		}
		if err != nil {
			return err
		}
		defer release()

		listener, err := listen()
		if err != nil {
			return err
		}
		addr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			_ = listener.Close()
			return fmt.Errorf("listening on %s: expected a TCP address, got %T", listener.Addr(), listener.Addr())
		}
		token := newToken()
		server := web.New(webParams(token, *idle))
		if err := files.Write(cmd.Context(), webserver.Record{PID: os.Getpid(), Port: addr.Port, Version: version, Token: token}); err != nil {
			_ = listener.Close()
			return err
		}
		return server.Serve(cmd.Context(), listener)
	}
	return cmd
}

// positive refuses an --idle that would stop a server before its first request.
func positive(idle time.Duration) error {
	if idle <= 0 {
		return domain.InvalidMessage("--" + flagIdle + " must be a positive duration, not " + idle.String())
	}
	return nil
}

// webParams are what the server of this process serves, with token, stopping after idle.
func webParams(token string, idle time.Duration) web.Params {
	return web.Params{
		Catalogue:   catalogue(),
		Events:      eventsStream(),
		Repos:       knownRepos(reposjson.Path(), bus.SocketPath()).Prune,
		Version:     version,
		Token:       token,
		Kill:        shell.KillCancelled,
		CommandLine: commandLine,
		Schema:      api.Schema,
		Idle:        idle,
	}
}

// eventsStream is the event stream of streams(), the one the server serves.
func eventsStream() api.Stream {
	for _, stream := range streams() {
		if stream.Name == events.Name {
			return stream
		}
	}
	panic("streams() lists no " + events.Name)
}

// ensureServer returns the record of a server of this version that answers: the one running, else one it starts, after asking one of another version to stop.
// A record is its server's only while that server holds the lock: a record left by a server that stopped names a port anything may listen on now, which must never get its token. No process is ever signalled by a pid read from the record.
func ensureServer(ctx context.Context, idle time.Duration) (webserver.Record, error) {
	files := webserver.Files{Dir: webserver.Dir()}
	last, _ := files.Read()
	// shortcut: free takes the lock for an instant, so a server another ui starts at that instant leaves at once; that ui then times out. Ask the lock without taking it if that bites.
	if !free(files) {
		running, answered := askVersion(ctx, last)
		if answered && running == version {
			return last, nil
		}
		if answered {
			busy, err := askStop(ctx, last)
			if err != nil {
				return webserver.Record{}, fmt.Errorf("the web server of wtm %s (pid %d) cannot be asked to stop: stop that process, then run wtm-next ui again: %w", running, last.PID, err)
			}
			if len(busy) > 0 {
				return webserver.Record{}, fmt.Errorf("the web server of wtm %s is finishing operation %s: run wtm-next ui again once it ends", running, strings.Join(busy, ", "))
			}
			if err := until(ctx, func() bool { return free(files) }); err != nil {
				return webserver.Record{}, fmt.Errorf("the web server of wtm %s did not stop: %w", running, err)
			}
		}
	}

	if err := startServer(idle); err != nil {
		return webserver.Record{}, err
	}
	var started webserver.Record
	err := until(ctx, func() bool {
		// the server started writes its record once it holds the lock, with a token of its own
		record, err := files.Read()
		if err != nil || record.Token == last.Token {
			return false
		}
		running, answered := askVersion(ctx, record)
		started = record
		return answered && running == version
	})
	if err != nil {
		return webserver.Record{}, fmt.Errorf("the web server did not answer once started: %w", err)
	}
	return started, nil
}

// askVersion returns the version of the server record names, and whether it answered with record's token.
func askVersion(ctx context.Context, record webserver.Record) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, webAsk)
	defer cancel()
	response, err := ask(ctx, askParams{Record: record, Method: http.MethodGet, Path: "/api/version"})
	if err != nil {
		return "", false
	}
	defer response.Body.Close()
	var answer struct {
		Version string `json:"version"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&answer) != nil {
		return "", false
	}
	return answer.Version, true
}

// askStop asks the server record names to stop, as its idle stop does, and returns the operations it refuses to stop for; it fails when the server cannot be asked, one older than POST /api/stop among them.
func askStop(ctx context.Context, record webserver.Record) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, webAsk)
	defer cancel()
	response, err := ask(ctx, askParams{Record: record, Method: http.MethodPost, Path: "/api/stop"})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusAccepted:
		return nil, nil
	case http.StatusConflict:
		var busy web.Busy
		if err := json.NewDecoder(response.Body).Decode(&busy); err != nil {
			return nil, err
		}
		return busy.Operations, nil
	default:
		return nil, fmt.Errorf("it answers %s", response.Status)
	}
}

type askParams struct {
	Record webserver.Record
	Method string
	Path   string
}

// ask sends a request to the server params.Record names, with its token.
func ask(ctx context.Context, params askParams) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, params.Method, "http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(params.Record.Port))+params.Path, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set(web.HeaderToken, params.Record.Token)
	return http.DefaultClient.Do(request)
}

// free reports whether no server holds the lock.
func free(files webserver.Files) bool {
	release, err := files.Lock()
	if err != nil {
		return false
	}
	release()
	return true
}

// until polls done until it reports true, for webStart at most.
func until(ctx context.Context, done func() bool) error {
	ctx, cancel := context.WithTimeout(ctx, webStart)
	defer cancel()
	ticker := time.NewTicker(webPoll)
	defer ticker.Stop()
	for !done() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

// startServer starts this binary's web server in a session of its own, with this process's environment but its correlation ID, which belongs to the one command that started it.
func startServer(idle time.Duration) error {
	if testing.Testing() {
		return errServerInTest
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return shell.Detached(shell.DetachedParams{Name: exe, Args: []string{uiCommand, serveCommand, "--" + flagIdle, idle.String()}, Env: shell.Environ([]string{cli.CorrelationIDEnv}, nil)})
}

// listen listens on the loopback at webPort, or at a port the system picks when it is taken.
func listen() (net.Listener, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(webPort)))
	if err == nil {
		return listener, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// pageURL is the address of the page of the repository repo on the server of record, the token in its fragment, which the browser never sends; repo is left out when empty.
func pageURL(record webserver.Record, repo string) string {
	fragment := url.Values{"token": {record.Token}}
	if repo != "" {
		fragment.Set("repo", repo)
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(record.Port)), Path: "/", Fragment: fragment.Encode()}).String()
}

// repoID is the id the server names the repository of dir by, registering it among those wtm knows; "" when dir is in no repository.
func repoID(ctx context.Context, dir string) string {
	params, err := locate(ctx, dir)
	if err != nil {
		return ""
	}

	register(ctx, params)
	return web.RepoID(params.repo())
}

// browser is the command that opens an address in the user's browser.
func browser() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

func newToken() string {
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
