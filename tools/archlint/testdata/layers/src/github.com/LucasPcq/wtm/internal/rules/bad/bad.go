package bad

import (
	"strings"

	_ "github.com/LucasPcq/wtm/internal/domain"
	in "github.com/LucasPcq/wtm/internal/infra" // want `internal/rules must not import "github.com/LucasPcq/wtm/internal/infra" — pure functions`
)

var _ = strings.TrimSpace

var _ = in.GlobalDir
