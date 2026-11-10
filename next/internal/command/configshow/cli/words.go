package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of config show's form: it has none.
func words(configshow.Questions, domain.Field) form.Words {
	return form.Words{}
}

// sections group the settings as the document reads them.
var sections = []struct {
	Title string
	Paths []string
}{
	{"Worktrees", []string{domain.PathBasePath, domain.PathBaseBranch}},
	{".env files", []string{domain.PathEnvStrategy, domain.PathEnvFiles}},
	{"Hooks", []string{domain.PathOnCreate, domain.PathOnClean}},
}

// rememberedTitle titles the remembered answers.
const rememberedTitle = "Remembered answers"

// remembered word each question an answer is remembered for.
var remembered = map[string]string{
	project.RememberEnvStrategy:  "Env strategy",
	project.RememberIsolation:    "Isolation",
	project.RememberSourceUpdate: "Source update",
}
