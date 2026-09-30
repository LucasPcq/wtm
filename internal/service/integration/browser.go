// Package integration adapts wtm to the third-party tools a worktree is used
// with — for now, whatever the desktop uses to open a URL.
package integration

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/LucasPcq/wtm/internal/domain"
)

// OpenURL hands a URL to the desktop's own opener. It returns rather than
// swallows the failure: a browser that never opened has to be visible, or the
// reader waits for a window that is not coming.
func OpenURL(url string) error {
	spec := openerFor(runtime.GOOS)
	out, err := exec.Command(spec.Name, append(spec.Args, url)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", spec.Name, url, err, out)
	}
	return nil
}

func openerFor(goos string) domain.ExecSpec {
	switch goos {
	case domain.GOOSDarwin:
		return domain.ExecSpec{Name: domain.OpenerDarwin}
	case domain.GOOSWindows:
		return domain.ExecSpec{Name: domain.OpenerWindows, Args: []string{domain.OpenerWindowsArg}}
	default:
		return domain.ExecSpec{Name: domain.OpenerUnix}
	}
}
