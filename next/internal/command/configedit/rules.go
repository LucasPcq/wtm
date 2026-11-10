package configedit

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what config edit checks once the editor is closed: config.toml as written, and the branches the base branch can be.
type Facts struct {
	Saved    project.Saved
	Branches []string
}

// ----------------------------------------------------------------------------
// The detail

// subject is the one item of config edit: the file it opens.
const subject = "config.toml"

// A Detail is what config edit reports of config.toml: nothing more than its item's status and warnings say.
type Detail struct{}

// ----------------------------------------------------------------------------
// Report

// The codes of a config.toml edited into one wtm cannot use, and of one a newer wtm wrote, which this one reads but never rewrites.
const (
	CodeInvalid      = Name + ".invalid"
	CodeInvalidValue = Name + ".invalid_value"
	CodeConfigNewer  = Name + ".config_newer"
)

// Report is config.toml as the editor left it: done when every setting is one wtm can use, else failed with a warning per problem and the edit to fix them; the file is kept as edited either way.
func Report(facts Facts) Outcome {
	questions := settings.NewQuestions(settings.QuestionsParams{Command: Name, Facts: settings.Facts{Saved: &facts.Saved, Branches: facts.Branches}})
	item := Item{Subject: subject, Status: domain.StatusDone, Detail: &Detail{}}
	problems := questions.Problems()
	for _, problem := range problems {
		params := domain.Params{"path": problem.Path, "problem": problem.Code}
		for key, value := range problem.Params {
			params[key] = value
		}
		item.Warnings = append(item.Warnings, domain.Warning{Code: CodeInvalidValue, Params: params, Message: problem.Message(keyOf(problem.Path))})
	}
	if len(problems) > 0 {
		item.Status = domain.StatusFailed
		item.Err = &domain.Error{Kind: domain.KindPrecondition, Code: CodeInvalid, Message: "config.toml holds " + count(len(problems)) + " wtm can't use: kept as edited", Params: domain.Params{"count": strconv.Itoa(len(problems))}, FollowUp: editAgain()}
	}

	if facts.Saved.Newer() {
		item.Warnings = append(item.Warnings, domain.Warning{Code: CodeConfigNewer, Message: "a newer wtm wrote config.toml: this wtm reads it but never rewrites it"})
	}
	return Outcome{Items: []Item{item}}
}

// editAgain is config edit again, to fix what the last edit left.
func editAgain() *domain.FollowUp {
	return &domain.FollowUp{Command: Name, Code: Name + ".again", Message: "fix it"}
}

func count(problems int) string {
	if problems == 1 {
		return "a setting"
	}
	return strconv.Itoa(problems) + " settings"
}

// keys are the settings by the TOML table or key config.toml holds them under, which an edit's problems name.
var keys = map[string]string{
	"worktrees.base_path":   settings.PathBasePath,
	"worktrees.base_branch": settings.PathBaseBranch,
	"env.strategy":          settings.PathEnvStrategy,
	"env.file":              settings.PathEnvFiles,
	"hooks.on_create":       settings.PathOnCreate,
	"hooks.on_clean":        settings.PathOnClean,
	"wizard.remembered":     settings.PathRemembered,
}

// keyOf is the TOML key config.toml holds the setting at path under, an entry's index or a remembered question kept: env.file[1] of env_files[1].
func keyOf(path string) string {
	for key, setting := range keys {
		if rest, ok := strings.CutPrefix(path, setting); ok && (rest == "" || rest[0] == '[' || rest[0] == '.') {
			return key + rest
		}
	}

	return path
}
