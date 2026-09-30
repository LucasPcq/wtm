package proxy

import "github.com/LucasPcq/wtm/internal/domain"

type RedirectorParams struct {
	// Root is what every path is joined onto, "/" in production. It exists so
	// the tests never touch a system file.
	Root string
}

// Plan is what Apply would write and run, rendered without writing anything —
// the recap the user confirms is this value.
type Plan struct {
	Files  []domain.ProxyPlannedFile
	Script string
}

type Redirector interface {
	Plan() (Plan, error)
	Apply() error
	Remove() error
	Inspect() domain.ProxyStatus
}
