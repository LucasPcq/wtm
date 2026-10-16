package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

func overCeiling(migrating migratingList, deadcode []string) []string {
	var over []string
	for _, entry := range migrating.entries {
		key := entry.rule + " " + entry.pattern.String()
		ceiling, listed := migratingCeiling[key]
		switch {
		case !listed:
			over = append(over, fmt.Sprintf(".archlint-migrating: %q is a new entry — the list only shrinks; an entry taken knowingly goes into migratingCeiling (tools/archlint/ceiling.go), in review", entry.line))
		case entry.budget > ceiling:
			over = append(over, fmt.Sprintf(".archlint-migrating: %q raises its count above %d — the list only shrinks", entry.line, ceiling))
		}
	}
	for _, pattern := range deadcode {
		if !slices.Contains(deadcodeCeiling, pattern) {
			over = append(over, fmt.Sprintf(".deadcode-ignore: %q is a new pattern — the list only shrinks; one taken knowingly goes into deadcodeCeiling (tools/archlint/ceiling.go), in review", pattern))
		}
	}
	return over
}

func loadDeadcodeIgnore() ([]string, error) {
	data, err := os.ReadFile(".deadcode-ignore")
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseDeadcodeIgnore(string(data)), nil
}

func parseDeadcodeIgnore(data string) []string {
	var patterns []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}

var migratingCeiling = map[string]int{
	`portbuild internal/service/env/`:                    8,
	`portbuild internal/service/memory/`:                 1,
	`portbuild internal/service/worktree/`:               11,
	`statewrite internal/service/execsvc/execsvc\.go`:    1,
	`statewrite internal/service/process/daemonlock\.go`: 1,
	`statewrite internal/service/worktree/relocate\.go`:  1,
	`statewrite internal/service/worktree/extract\.go`:   2,
	`stateowner internal/config/write\.go`:               4,
	`stateowner internal/config/pendingremovals\.go`:     1,
	`stateowner internal/infra/registry\.go`:             1,
	`stateowner internal/service/compose/patch\.go`:      1,
	`stateowner internal/service/selfupdate/state\.go`:   1,
	`gitexec internal/infra/branch\.go`:                  10,
	`gitexec internal/infra/git_paths\.go`:               3,
	`gitexec internal/infra/history\.go`:                 3,
	`gitexec internal/infra/patch\.go`:                   5,
	`gitexec internal/infra/rebase\.go`:                  12,
	`gitexec internal/infra/status\.go`:                  4,
	`gitexec internal/infra/tools\.go`:                   1,
	`gitexec internal/infra/upstream\.go`:                4,
	`gitexec internal/infra/worktree_ops\.go`:            5,
	`gitexec internal/infra/worktree_query\.go`:          3,
	`rootfreeze internal/domain/agents\.go`:              7,
	`rootfreeze internal/domain/branch\.go`:              25,
	`rootfreeze internal/domain/compose\.go`:             19,
	`rootfreeze internal/domain/config\.go`:              16,
	`rootfreeze internal/domain/constants\.go`:           2506,
	`rootfreeze internal/domain/dashboard\.go`:           7,
	`rootfreeze internal/domain/detail\.go`:              38,
	`rootfreeze internal/domain/env\.go`:                 8,
	`rootfreeze internal/domain/envdiff\.go`:             24,
	`rootfreeze internal/domain/envports\.go`:            24,
	`rootfreeze internal/domain/envsync\.go`:             6,
	`rootfreeze internal/domain/errors\.go`:              92,
	`rootfreeze internal/domain/events\.go`:              42,
	`rootfreeze internal/domain/exec\.go`:                10,
	`rootfreeze internal/domain/extract\.go`:             10,
	`rootfreeze internal/domain/github\.go`:              11,
	`rootfreeze internal/domain/hooks\.go`:               9,
	`rootfreeze internal/domain/init\.go`:                17,
	`rootfreeze internal/domain/input\.go`:               2,
	`rootfreeze internal/domain/jobs\.go`:                92,
	`rootfreeze internal/domain/portprobe\.go`:           6,
	`rootfreeze internal/domain/proxy\.go`:               6,
	`rootfreeze internal/domain/prune\.go`:               7,
	`rootfreeze internal/domain/relocate\.go`:            18,
	`rootfreeze internal/domain/runview\.go`:             15,
	`rootfreeze internal/domain/shield\.go`:              2,
	`rootfreeze internal/domain/status\.go`:              2,
	`rootfreeze internal/domain/sync\.go`:                23,
	`rootfreeze internal/domain/tone\.go`:                6,
	`rootfreeze internal/domain/tree\.go`:                16,
	`rootfreeze internal/domain/upgrade\.go`:             15,
	`rootfreeze internal/domain/version\.go`:             2,
	`rootfreeze internal/domain/worktree\.go`:            28,
	`rootfreeze internal/domain/worktreestatus\.go`:      12,
	`rootfreeze internal/rules/addressing\.go`:           7,
	`rootfreeze internal/rules/age\.go`:                  5,
	`rootfreeze internal/rules/baseports\.go`:            4,
	`rootfreeze internal/rules/batch\.go`:                5,
	`rootfreeze internal/rules/branch\.go`:               8,
	`rootfreeze internal/rules/branchlist\.go`:           3,
	`rootfreeze internal/rules/branchname\.go`:           2,
	`rootfreeze internal/rules/browserports\.go`:         2,
	`rootfreeze internal/rules/changes\.go`:              2,
	`rootfreeze internal/rules/compose\.go`:              12,
	`rootfreeze internal/rules/composelines\.go`:         16,
	`rootfreeze internal/rules/composenames\.go`:         11,
	`rootfreeze internal/rules/composeplan\.go`:          13,
	`rootfreeze internal/rules/composestack\.go`:         3,
	`rootfreeze internal/rules/composestop\.go`:          4,
	`rootfreeze internal/rules/concurrency\.go`:          5,
	`rootfreeze internal/rules/correlation\.go`:          2,
	`rootfreeze internal/rules/daemon\.go`:               9,
	`rootfreeze internal/rules/dashboard\.go`:            24,
	`rootfreeze internal/rules/deselected\.go`:           4,
	`rootfreeze internal/rules/detach\.go`:               8,
	`rootfreeze internal/rules/detailpanel\.go`:          10,
	`rootfreeze internal/rules/detectedports\.go`:        4,
	`rootfreeze internal/rules/devorigins\.go`:           4,
	`rootfreeze internal/rules/elapsed\.go`:              2,
	`rootfreeze internal/rules/env_detect\.go`:           7,
	`rootfreeze internal/rules/env_parse\.go`:            4,
	`rootfreeze internal/rules/envaccess\.go`:            9,
	`rootfreeze internal/rules/envaddressing\.go`:        10,
	`rootfreeze internal/rules/envflags\.go`:             3,
	`rootfreeze internal/rules/envmerge\.go`:             6,
	`rootfreeze internal/rules/envmergejob\.go`:          5,
	`rootfreeze internal/rules/envorigins\.go`:           10,
	`rootfreeze internal/rules/envowned\.go`:             12,
	`rootfreeze internal/rules/envportdetect\.go`:        9,
	`rootfreeze internal/rules/envportlines\.go`:         16,
	`rootfreeze internal/rules/envports\.go`:             22,
	`rootfreeze internal/rules/envportsides\.go`:         3,
	`rootfreeze internal/rules/envredact\.go`:            3,
	`rootfreeze internal/rules/envreport\.go`:            9,
	`rootfreeze internal/rules/envresolve\.go`:           7,
	`rootfreeze internal/rules/envrestore\.go`:           7,
	`rootfreeze internal/rules/envsummary\.go`:           4,
	`rootfreeze internal/rules/envvalue\.go`:             9,
	`rootfreeze internal/rules/envvalueapply\.go`:        5,
	`rootfreeze internal/rules/envvaluefields\.go`:       5,
	`rootfreeze internal/rules/eol\.go`:                  5,
	`rootfreeze internal/rules/exec\.go`:                 19,
	`rootfreeze internal/rules/exitcode\.go`:             6,
	`rootfreeze internal/rules/extract\.go`:              8,
	`rootfreeze internal/rules/fastforward\.go`:          7,
	`rootfreeze internal/rules/flagvalue\.go`:            7,
	`rootfreeze internal/rules/focusexit\.go`:            3,
	`rootfreeze internal/rules/foreigndata\.go`:          7,
	`rootfreeze internal/rules/github\.go`:               7,
	`rootfreeze internal/rules/help\.go`:                 4,
	`rootfreeze internal/rules/hookbeat\.go`:             9,
	`rootfreeze internal/rules/hooks\.go`:                5,
	`rootfreeze internal/rules/hostlabel\.go`:            3,
	`rootfreeze internal/rules/hyperlink\.go`:            4,
	`rootfreeze internal/rules/init_answers\.go`:         11,
	`rootfreeze internal/rules/initrecap\.go`:            3,
	`rootfreeze internal/rules/initsummary\.go`:          3,
	`rootfreeze internal/rules/isolation\.go`:            23,
	`rootfreeze internal/rules/jobaddress\.go`:           9,
	`rootfreeze internal/rules/jobdir\.go`:               3,
	`rootfreeze internal/rules/jobedit\.go`:              9,
	`rootfreeze internal/rules/jobenv\.go`:               10,
	`rootfreeze internal/rules/jobindex\.go`:             7,
	`rootfreeze internal/rules/joblog\.go`:               13,
	`rootfreeze internal/rules/joborder\.go`:             3,
	`rootfreeze internal/rules/jobports\.go`:             18,
	`rootfreeze internal/rules/jobroutes\.go`:            4,
	`rootfreeze internal/rules/jobs_builder\.go`:         9,
	`rootfreeze internal/rules/jobs\.go`:                 24,
	`rootfreeze internal/rules/jobstate\.go`:             7,
	`rootfreeze internal/rules/joburl\.go`:               9,
	`rootfreeze internal/rules/jobvisibility\.go`:        15,
	`rootfreeze internal/rules/jsonneedsyes\.go`:         2,
	`rootfreeze internal/rules/keystroke\.go`:            2,
	`rootfreeze internal/rules/logsjobs\.go`:             6,
	`rootfreeze internal/rules/nameclash\.go`:            3,
	`rootfreeze internal/rules/namespace\.go`:            25,
	`rootfreeze internal/rules/occupiedpath\.go`:         3,
	`rootfreeze internal/rules/ordinal\.go`:              6,
	`rootfreeze internal/rules/output\.go`:               3,
	`rootfreeze internal/rules/package_manager\.go`:      2,
	`rootfreeze internal/rules/plural\.go`:               3,
	`rootfreeze internal/rules/porcelain\.go`:            2,
	`rootfreeze internal/rules/portclash\.go`:            7,
	`rootfreeze internal/rules/portkeys\.go`:             19,
	`rootfreeze internal/rules/portprobe\.go`:            19,
	`rootfreeze internal/rules/portwiring\.go`:           17,
	`rootfreeze internal/rules/prefill\.go`:              3,
	`rootfreeze internal/rules/prnumber\.go`:             2,
	`rootfreeze internal/rules/process\.go`:              6,
	`rootfreeze internal/rules/profileedit\.go`:          5,
	`rootfreeze internal/rules/profileplan\.go`:          9,
	`rootfreeze internal/rules/proxyhint\.go`:            3,
	`rootfreeze internal/rules/proxyroute\.go`:           5,
	`rootfreeze internal/rules/prune\.go`:                13,
	`rootfreeze internal/rules/publicport\.go`:           3,
	`rootfreeze internal/rules/publishedurl\.go`:         3,
	`rootfreeze internal/rules/reach\.go`:                9,
	`rootfreeze internal/rules/reconcile\.go`:            6,
	`rootfreeze internal/rules/relocate_plan\.go`:        11,
	`rootfreeze internal/rules/relocate_recap\.go`:       5,
	`rootfreeze internal/rules/relocate_steps\.go`:       5,
	`rootfreeze internal/rules/remember\.go`:             5,
	`rootfreeze internal/rules/removejob\.go`:            4,
	`rootfreeze internal/rules/reparent\.go`:             11,
	`rootfreeze internal/rules/runboard\.go`:             6,
	`rootfreeze internal/rules/runnerchoices\.go`:        11,
	`rootfreeze internal/rules/runners\.go`:              9,
	`rootfreeze internal/rules/runsurface\.go`:           3,
	`rootfreeze internal/rules/runview\.go`:              11,
	`rootfreeze internal/rules/runworktrees\.go`:         11,
	`rootfreeze internal/rules/scandirs\.go`:             2,
	`rootfreeze internal/rules/scopeproposal\.go`:        17,
	`rootfreeze internal/rules/scriptkind\.go`:           3,
	`rootfreeze internal/rules/scriptkinds\.go`:          5,
	`rootfreeze internal/rules/scripts\.go`:              5,
	`rootfreeze internal/rules/sharedapply\.go`:          8,
	`rootfreeze internal/rules/shell\.go`:                2,
	`rootfreeze internal/rules/shellcmd\.go`:             3,
	`rootfreeze internal/rules/sync_ancestry\.go`:        3,
	`rootfreeze internal/rules/sync_plan\.go`:            11,
	`rootfreeze internal/rules/sync\.go`:                 31,
	`rootfreeze internal/rules/tally\.go`:                2,
	`rootfreeze internal/rules/touchchoices\.go`:         8,
	`rootfreeze internal/rules/tree\.go`:                 9,
	`rootfreeze internal/rules/upgrade\.go`:              13,
	`rootfreeze internal/rules/urlcandidates\.go`:        7,
	`rootfreeze internal/rules/usage\.go`:                4,
	`rootfreeze internal/rules/validate\.go`:             15,
	`rootfreeze internal/rules/workspace\.go`:            5,
	`rootfreeze internal/rules/worktree\.go`:             18,
	`rootfreeze internal/rules/worktreestatus\.go`:       13,
}

var deadcodeCeiling = []string{
	`TextInputModel\.helpRowless`,
	`TextListModel\.helpRowless`,
}

var commentCeiling = map[string]int{
	`dispatch`:         13,
	`domain/operation`: 1,
	`infra/fs`:         40,
	`infra/git`:        19,
	`infra/proc`:       36,
	`rules/branch`:     2,
	`rules/envcopy`:    4,
	`rules/runenv`:     2,
	`rules/statefile`:  4,
	`rules/worktree`:   6,
	`service/gitwt`:    5,
	`service/hooks`:    42,
	`service/identity`: 9,
	`service/runenv`:   11,
	`store`:            20,
	`surface/tui/form`: 9,
}
