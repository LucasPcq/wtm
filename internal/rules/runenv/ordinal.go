package runenv

import "github.com/LucasPcq/wtm/internal/domain"

type PortsNeedOrdinalParams struct {
	Case Case
	Run  domain.RunConfig
}

func PortsNeedOrdinal(params PortsNeedOrdinalParams) bool {
	if _, linked := params.Case.(Linked); !linked {
		return false
	}
	return len(params.Run.EnvPorts) > 0 || len(params.Run.EnvValues) > 0
}
