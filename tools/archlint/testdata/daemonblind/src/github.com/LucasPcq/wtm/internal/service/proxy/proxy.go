package proxy

import (
	"github.com/LucasPcq/wtm/internal/config" // want `the daemon must not import "github.com/LucasPcq/wtm/internal/config"`
	"github.com/LucasPcq/wtm/internal/infra"
)

var _ = config.Load

func Route() error {
	_, err := infra.GlobalDir()
	return err
}

func root() (string, error) {
	return infra.Toplevel("") // want `the daemon calls infra\.Toplevel`
}
