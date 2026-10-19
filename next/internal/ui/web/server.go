// Package web is a throwaway spike: the catalogue served over local HTTP to one embedded page, to test whether a GUI needs any logic the commands do not already give.
package web

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

//go:embed index.html
var page []byte

// Params are what the server serves: the catalogue, the repository directory every call runs on, and the token a call must carry.
type Params struct {
	Catalogue api.Catalogue
	Dir       string
	Token     string
}

type callBody struct {
	Answers           []domain.Answered `json:"answers"`
	DryRun            bool              `json:"dry_run"`
	ExpectFingerprint string            `json:"expect_fingerprint"`
}

// Handler serves the page and the catalogue's commands under /api, each call guarded by the token and the page's own origin.
func Handler(p Params) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})

	mux.HandleFunc("GET /api/info", p.guard(func(w http.ResponseWriter, _ *http.Request) {
		names := make([]string, 0, len(p.Catalogue))
		for _, c := range p.Catalogue {
			names = append(names, c.Name)
		}
		writeJSON(w, http.StatusOK, map[string]any{"dir": p.Dir, "commands": names})
	}))

	mux.HandleFunc("POST /api/commands/{name}/questions", p.guard(func(w http.ResponseWriter, r *http.Request) {
		command, body, ok := p.read(w, r)
		if !ok {
			return
		}
		if command.Questions == nil {
			writeJSON(w, http.StatusOK, command.Form)
			return
		}

		form, err := command.Questions(r.Context(), api.Call{Dir: p.Dir, Answers: answers(body)})
		if err != nil {
			e := domain.AsError(domain.AsErrorParams{Err: err})
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": e})
			return
		}
		writeJSON(w, http.StatusOK, form)
	}))

	mux.HandleFunc("POST /api/commands/{name}", p.guard(func(w http.ResponseWriter, r *http.Request) {
		command, body, ok := p.read(w, r)
		if !ok {
			return
		}
		call := api.Call{Dir: p.Dir, Answers: answers(body), Options: domain.RunOptions{Surface: "web", DryRun: body.DryRun, ExpectFingerprint: body.ExpectFingerprint}}

		if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			response := command.Execute(r.Context(), call)
			writeJSON(w, response.Status, response.Envelope)
			return
		}

		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		var mu sync.Mutex
		send := func(event string, data []byte) {
			mu.Lock()
			defer mu.Unlock()
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			if flusher != nil {
				flusher.Flush()
			}
		}

		call.Options.Report = func(e domain.Event) {
			data, err := domain.MarshalEvent(e)
			if err == nil {
				send("progress", data)
			}
		}
		response := command.Execute(r.Context(), call)
		data, _ := json.Marshal(map[string]any{"status": response.Status, "envelope": response.Envelope})
		send("done", data)
	}))

	return mux
}

// guard refuses a call without the token, or from a page of another origin: a server that creates worktrees is reachable by any page the browser opens.
func (p Params) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Wtm-Token")), []byte(p.Token)) != 1 {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (p Params) read(w http.ResponseWriter, r *http.Request) (api.Command, callBody, bool) {
	command, found := p.Catalogue.Find(r.PathValue("name"))
	if !found {
		http.Error(w, "unknown command", http.StatusNotFound)
		return api.Command{}, callBody{}, false
	}

	var body callBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return api.Command{}, callBody{}, false
	}
	return command, body, true
}

// answers keeps an empty list non-nil, so the call is folded from answers rather than decoded from a JSON request.
func answers(body callBody) []domain.Answered {
	if body.Answers == nil {
		return []domain.Answered{}
	}
	return body.Answers
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
