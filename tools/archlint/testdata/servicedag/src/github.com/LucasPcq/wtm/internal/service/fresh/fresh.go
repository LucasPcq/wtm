package fresh

import "github.com/LucasPcq/wtm/internal/service/branch" // want `internal/service/fresh must not import .* undeclared service edge`

var _ = branch.List
