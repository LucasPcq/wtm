package rules

import "github.com/LucasPcq/wtm/internal/domain"

const bashZshTemplate = `wtm() {
  local wtm_status dir
  if [ "$1" = "go" ]; then
    dir="$(command wtm resolve "${@:2}")"
    wtm_status=$?
    if [ -n "$dir" ]; then
      cd "$dir" || return 1
    fi
    return $wtm_status
  fi
  local tmpfile
  tmpfile="$(mktemp /tmp/wtm-go.XXXXXX)"
  WTM_GO_FILE="$tmpfile" command wtm "$@"
  wtm_status=$?
  dir="$(cat "$tmpfile" 2>/dev/null)"
  rm -f "$tmpfile"
  if [ -n "$dir" ] && [ -d "$dir" ]; then
    cd "$dir" || return 1
  fi
  return $wtm_status
}
`

const fishTemplate = `function wtm
  if test "$argv[1]" = "go"
    set -l dir (command wtm resolve $argv[2..])
    set -l wtm_status $status
    if test -n "$dir"
      cd "$dir"; or return 1
    end
    return $wtm_status
  end
  set -l tmpfile (mktemp /tmp/wtm-go.XXXXXX)
  WTM_GO_FILE="$tmpfile" command wtm $argv
  set -l wtm_status $status
  set -l dir (cat "$tmpfile" 2>/dev/null)
  rm -f "$tmpfile"
  if test -n "$dir" -a -d "$dir"
    cd "$dir"; or return 1
  end
  return $wtm_status
end
`

// GenerateShellInit returns the shell function wrapper for the given shell type.
func GenerateShellInit(shell domain.ShellType) string {
	switch shell {
	case domain.ShellFish:
		return fishTemplate
	default:
		return bashZshTemplate
	}
}
