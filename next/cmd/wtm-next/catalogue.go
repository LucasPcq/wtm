package main

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
)

// catalogue lists every command for a surface other than the command line, one line per command.
func catalogue() api.Catalogue {
	return api.Catalogue{
		api.Bind(create.Name, openCreate, create.UseCase.Questions, create.UseCase.Run),
		api.Bind(extract.Name, openExtract, extract.UseCase.Questions, extract.UseCase.Run),
		api.Bind(list.Name, openList, list.UseCase.Questions, list.UseCase.Run),
		api.Bind(status.Name, openStatus, status.UseCase.Questions, status.UseCase.Run),
		api.Bind(tree.Name, openTree, tree.UseCase.Questions, tree.UseCase.Run),
	}
}

// streams lists every command that streams rather than returns, for a surface other than the command line, one line per command.
func streams() []api.Stream {
	return []api.Stream{
		api.BindStream[events.Request, events.Questions](events.Name, watchEvents(systemStream())),
	}
}

// watchEvents is the watch of the event stream over stream for a surface: the call's directory is named, never a fallback to every repository, a relative repo is read from it, never from the server's own directory, and each line goes out as its JSON.
func watchEvents(stream events.UseCase) func(ctx context.Context, dir string, req events.Request, send func(json.RawMessage) error) error {
	return func(ctx context.Context, dir string, req events.Request, send func(json.RawMessage) error) error {
		req.Dir, req.Named = dir, true
		if req.Repo != "" && !filepath.IsAbs(req.Repo) {
			req.Repo = filepath.Join(dir, req.Repo)
		}
		return stream.Watch(ctx, req, events.Out{Emit: func(received events.Received) error { return send(received.JSON) }})
	}
}
