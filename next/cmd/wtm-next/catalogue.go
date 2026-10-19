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
		api.Bind(create.Name, openCreate, create.UseCase.Questions, create.UseCase.Run),
		api.Bind(extract.Name, openExtract, extract.UseCase.Questions, extract.UseCase.Run),
		api.Bind(list.Name, openList, list.UseCase.Questions, list.UseCase.Run),
	}
}
