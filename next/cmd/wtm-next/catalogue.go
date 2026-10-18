package main

import (
	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
)

// catalogue lists every command for a surface other than the command line, one line per command.
func catalogue() api.Catalogue {
	return api.Catalogue{
		api.Bind(create.Name, create.Questions{}.Form(), openCreate, create.UseCase.Run),
		api.Bind(extract.Name, extract.Questions{}.Form(), openExtract, extract.UseCase.Run),
	}
}
