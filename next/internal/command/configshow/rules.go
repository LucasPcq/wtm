package configshow

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what config show reads: config.toml as written, and the branches the base branch can be.
type Facts struct {
	Saved    project.Saved
	Branches []string
}

// ----------------------------------------------------------------------------
// The detail

// subject is the one item of config show: the file it reads.
const subject = "config.toml"

// A Detail is config.toml as a document: the digest of its bytes, which a change made on it names (config set's replaces); whether a newer wtm wrote it, which this one then reads but never rewrites; each setting as the field of a form, as config.toml holds it, with the problems it holds; the default of each setting that has one; the answers it remembers.
type Detail struct {
	Digest     string       `json:"digest"`
	Newer      bool         `json:"newer"`
	Settings   domain.Form  `json:"settings"`
	Defaults   []Default    `json:"defaults"`
	Remembered []Remembered `json:"remembered"`
}

// A Default is a setting's value when config.toml names none, and whether its value is that one.
type Default struct {
	Path  string `json:"path"`
	Value string `json:"value"`
	InUse bool   `json:"in_use"`
}

// A Remembered is an answer config.toml remembers: its question and its value, the entry config set's remembered field spells it as, and the problem it holds, if wtm cannot keep it.
type Remembered struct {
	Question string             `json:"question"`
	Value    string             `json:"value"`
	Entry    string             `json:"entry"`
	Problem  *domain.FieldError `json:"problem,omitempty"`
}

// ----------------------------------------------------------------------------
// Report

// The codes of what a reader of config.toml should know: a setting it holds that wtm cannot use, which the commands that read it refuse until it is changed; a newer wtm wrote it, so this one never rewrites it.
const (
	CodeInvalidValue = Name + ".invalid_value"
	CodeConfigNewer  = Name + ".config_newer"
)

// Report is the read of config.toml: its one item, the document, and a warning per problem it holds, a remembered answer's followed by edit, the one fix of an answer no question takes.
func Report(facts Facts, edit *domain.FollowUp) Outcome {
	questions := settings.NewQuestions(settings.QuestionsParams{Command: Name, Facts: settings.Facts{Saved: &facts.Saved, Branches: facts.Branches}})
	fields, problems := questions.Fields(), questions.Problems()
	detail := &Detail{Digest: facts.Saved.Digest, Newer: facts.Saved.Newer(), Settings: domain.Form{Fields: fields, Problems: problems, Sections: domain.SectionsOf(settings.Sections(), fields, problems)}, Remembered: []Remembered{}}
	detail.Defaults = []Default{
		{Path: settings.PathBasePath, Value: project.DefaultBasePath, InUse: questions.BasePath.Value == project.DefaultBasePath},
		{Path: settings.PathBaseBranch, Value: project.DefaultBaseBranch, InUse: questions.BaseBranch.Value == project.DefaultBaseBranch},
		{Path: settings.PathEnvStrategy, Value: string(project.DefaultEnvStrategy), InUse: questions.EnvStrategy.Value == string(project.DefaultEnvStrategy)},
	}
	remembered := facts.Saved.Config.Remembered
	for _, question := range slices.Sorted(maps.Keys(remembered)) {
		value := remembered[question]
		answer := Remembered{Question: question, Value: value, Entry: question + "=" + value}
		if at := slices.IndexFunc(problems, func(problem domain.FieldError) bool { return problem.Path == settings.PathRemembered+"."+question }); at >= 0 {
			answer.Problem = &problems[at]
		}
		detail.Remembered = append(detail.Remembered, answer)
	}

	var warnings []domain.Warning
	if detail.Newer {
		newer := &domain.SchemaNewer{Path: subject, Found: facts.Saved.Schema, Supported: project.ConfigSchema}
		warnings = append(warnings, domain.Warning{Code: CodeConfigNewer, Params: domain.Params{"found": strconv.Itoa(newer.Found), "supported": strconv.Itoa(newer.Supported)}, Message: newer.Error()})
	}
	for _, problem := range detail.Settings.Problems {
		params := domain.Params{"path": problem.Path, "problem": problem.Code}
		for key, value := range problem.Params {
			params[key] = value
		}
		warning := domain.Warning{Code: CodeInvalidValue, Params: params, Message: problem.Message(keyOf(problem.Path))}
		if strings.HasPrefix(problem.Path, settings.PathRemembered+".") {
			warning.FollowUp = edit
		}
		warnings = append(warnings, warning)
	}
	return Outcome{Items: []Item{{Subject: subject, Status: domain.StatusDone, Detail: detail}}, Warnings: warnings}
}

// ----------------------------------------------------------------------------
// The keys

// keys are the settings by the TOML table or key config.toml holds them under.
var keys = map[string]string{
	"worktrees.base_path":   settings.PathBasePath,
	"worktrees.base_branch": settings.PathBaseBranch,
	"env.strategy":          settings.PathEnvStrategy,
	"env.file":              settings.PathEnvFiles,
	"hooks.on_create":       settings.PathOnCreate,
	"hooks.on_clean":        settings.PathOnClean,
	"wizard.remembered":     settings.PathRemembered,
}

// settingOf is the setting the TOML key is, or holds, "" when it is none.
func settingOf(key string) string {
	for prefix, setting := range keys {
		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return setting
		}
	}

	return ""
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
