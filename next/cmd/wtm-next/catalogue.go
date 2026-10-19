package main

import (
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
)

// catalogue lists every command for a surface other than the command line, one line per command.
func catalogue() api.Catalogue {
	return api.Catalogue{
		api.Ask(api.Bind(create.Name, create.Questions{}.Form(), openCreate, create.UseCase.Run), openCreate, create.UseCase.Questions),
		api.Ask(api.Bind(extract.Name, extract.Questions{}.Form(), openExtract, extract.UseCase.Run), openExtract, extract.UseCase.Questions),
		api.Bind(list.Name, list.Questions{}.Form(), openList, list.UseCase.Run),
	}
}
