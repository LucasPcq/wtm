package runenv

import "github.com/LucasPcq/wtm/next/internal/wtm/run"

type PortsNeedOrdinalParams struct {
	Case Case
	Run  run.Config
}

func PortsNeedOrdinal(params PortsNeedOrdinalParams) bool {
	if _, linked := params.Case.(Linked); !linked {
		return false
	}
	return len(params.Run.EnvPorts) > 0 || len(params.Run.EnvValues) > 0
}

type HooksReadRunParams struct {
	Run       run.Config
	Isolation run.Isolation
}

func HooksReadRun(params HooksReadRunParams) bool {
	return params.Isolation != "" && len(composeDirs(params.Run)) > 0
}
