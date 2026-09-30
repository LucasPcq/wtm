package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type NeedsDevOriginsParams struct {
	Job domain.JobConfig
	// ConfigSource is the job's next.config.* as service/detect read it. Empty
	// means there is none, so there is nothing to say: the presence of that file
	// is the signal, never a framework guessed from the command line.
	ConfigSource string
}

// NeedsDevOrigins reports whether a published Next job would refuse requests
// arriving under the name the proxy serves it under.
func NeedsDevOrigins(params NeedsDevOriginsParams) bool {
	if params.Job.URL == nil || params.ConfigSource == "" {
		return false
	}
	return !strings.Contains(params.ConfigSource, domain.DevOriginsKey)
}

// DevOriginsPattern is the allowedDevOrigins value a Next project needs, which
// loses its port for the same reason a named URL does.
func DevOriginsPattern(publicPort int) string {
	if publicPort == domain.ProxyPrivilegedPort {
		return fmt.Sprintf(domain.DevOriginsPatternNoPortFmt, domain.ProxyTLD)
	}
	return fmt.Sprintf(domain.DevOriginsPatternFmt, domain.ProxyTLD, publicPort)
}
