package create

import "github.com/LucasPcq/wtm/internal/domain"

const (
	PathBranches    = "branches"
	PathFrom        = "from"
	PathEnvFrom     = "env_from"
	PathIsolation   = "isolation"
	PathFastForward = "fast_forward"
	PathIfNotExists = "if_not_exists"
)

type Request struct {
	Branches    []string           `json:"branches"`
	From        string             `json:"from"`
	EnvFrom     domain.EnvStrategy `json:"env_from"`
	Isolation   domain.Isolation   `json:"isolation"`
	FastForward bool               `json:"fast_forward"`
	IfNotExists bool               `json:"if_not_exists"`
	Remember    []string           `json:"remember"`
}
