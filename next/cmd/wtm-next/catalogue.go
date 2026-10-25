package main

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
)

// catalogue lists every command for a surface other than the command line, one line per command.
func catalogue() api.Catalogue {
	return api.Catalogue{
		api.Bind(create.Name, openCreate, create.UseCase.Questions, create.UseCase.Run),
		api.Bind(extract.Name, openExtract, extract.UseCase.Questions, extract.UseCase.Run),
		api.Bind(list.Name, openList, list.UseCase.Questions, list.UseCase.Run),
		api.Bind(resolve.Name, openResolve, resolve.UseCase.Questions, resolve.UseCase.Run),
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

// watchEvents is the watch of the event stream over stream for a surface: each line goes out as its JSON, each warning as it is.
func watchEvents(stream events.UseCase) func(ctx context.Context, req events.Request, sink api.Sink) error {
	return func(ctx context.Context, req events.Request, sink api.Sink) error {
		return stream.Watch(ctx, req, events.Out{Emit: func(received events.Received) error { return sink.Send(received.JSON) }, Warn: sink.Warn})
	}
}
