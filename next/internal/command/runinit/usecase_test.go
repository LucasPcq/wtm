package runinit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/compose"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFirstRunInitWritesRunTomlAndRewritesCompose(t *testing.T) {
	w := newWorld(t)

	outcome := w.apply(t, runinit.Request{})

	formtest.Golden(t, "first.run.toml", w.runToml(t))
	formtest.Golden(t, "first.docker-compose.yml", readFile(t, filepath.Join(w.main, "docker-compose.yml")))
	assert.Equal(t, []string{"docker-compose.yml", "run.toml"}, subjects(outcome.Result.Items))
	assert.Equal(t, []string{runinit.CodeUp}, codes(outcome.Result.FollowUps))
}

func TestEveryPortAComposeServicePublishesIsDeclaredMailpits1025Included(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	mailpit := w.job(t, "mailpit")
	assert.Equal(t, map[string]int{"MAILPIT_PORT": 8025, "MAILPIT_PORT_1025": 1025}, mailpit.Ports)
	assert.Contains(t, readFile(t, filepath.Join(w.main, "docker-compose.yml")), `"${MAILPIT_PORT_1025:-1025}:1025"`)
}

func TestAPortAJobOnlyDialsIsNeitherItsOwnNorLinkedToASharedService(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	worker := w.job(t, "worker")
	assert.Empty(t, worker.Ports, "SMTP_PORT is mailpit's, which worker dials")
	assert.True(t, worker.BindsNoPort)
	assert.Nil(t, worker.URL)
	run := w.run(t)
	assert.NotContains(t, linkedKeys(run), "apps/worker/.env#SMTP_PORT", "mailpit is shared: its port never moves")
}

func TestOnlyAJobThatSpeaksHTTPIsOfferedAName(t *testing.T) {
	w := newWorld(t)

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	assert.Equal(t, []string{"mailpit", "admin", "api", "web"}, questions.Decision.Addressable)
	assert.Equal(t, []string{"admin", "api", "web"}, questions.Decision.URLs, "a compose service is named only when asked")
}

func TestOnlyACommandThatBindsItsPortAsALiteralIsToCheckWithTheCommandThatReadsIt(t *testing.T) {
	w := newWorld(t)

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	require.Len(t, questions.ToCheck, 2)
	ignored := questions.ToCheck[0]
	assert.Equal(t, runinit.CodePortIgnored, ignored.Code)
	assert.Equal(t, domain.Params{"job": "admin", "variable": "ADMIN_PORT", "port": "3001", "before": "pnpm dev", "after": "pnpm exec next dev -p ${ADMIN_PORT}"}, ignored.Params)
	assert.Empty(t, questions.Errors, "a problem to check never refuses the request")
	ports := sectionOf(t, questions.Form(), runinit.SectionPorts)
	assert.Equal(t, domain.SectionToCheck, ports.State)
	assert.Equal(t, runinit.CodePortIgnored, ports.Reason)
}

func TestTheCommandProposedIsWrittenOnceAnsweredAndNothingIsLeftToCheck(t *testing.T) {
	w := newWorld(t)

	outcome := w.apply(t, runinit.Request{Commands: []string{"admin=pnpm exec next dev -p ${ADMIN_PORT}"}})

	assert.Equal(t, "pnpm exec next dev -p ${ADMIN_PORT}", w.job(t, "admin").Cmd)
	assert.NotContains(t, warningCodes(outcome.Warnings), runinit.CodePortIgnored)
}

func TestAnEnvFileNoWorktreeGetsIsToCheckAndCopyingItWritesConfigToml(t *testing.T) {
	w := newWorld(t)
	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})
	require.NoError(t, err)
	require.Contains(t, toCheckCodes(questions.ToCheck), runinit.CodeEnvNotCopied)

	outcome := w.apply(t, runinit.Request{EnvCopy: []string{"apps/web/.env.local"}})

	assert.Contains(t, subjects(outcome.Result.Items), "config.toml")
	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `target = "apps/web/.env.local"`)
	assert.NotContains(t, warningCodes(outcome.Warnings), runinit.CodeEnvNotCopied)
}

func TestADatabaseURLFollowsItsSharedServiceOnceAsData(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	run := w.run(t)
	assert.Contains(t, run.EnvValues, project.EnvValueLink{File: "apps/api/.env", Key: "DATABASE_URL", Job: "postgres", Value: "postgres://postgres:postgres@localhost:{port.POSTGRES_PORT}/{namespace}"})
	assert.NotContains(t, linkedKeys(run), "apps/api/.env#DATABASE_URL", "never an env_port as well")
}

func TestAComposeServicesJobIsNamedAfterItAndRunsThroughDockerComposeV2(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	redis := w.job(t, "redis")
	assert.Equal(t, "docker compose up -d redis", redis.Cmd)
	assert.Equal(t, "docker compose stop redis", redis.Stop)
	assert.Equal(t, project.PerWorktree, redis.Scope)
	assert.Contains(t, readFile(t, filepath.Join(w.main, "docker-compose.yml")), `container_name: "${COMPOSE_PROJECT_NAME:-acme}-redis"`)
}

func TestADatabaseIsSharedWithASpaceEachAndAMailCatcherWithNothingOfItsOwn(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	postgres, mailpit := w.job(t, "postgres"), w.job(t, "mailpit")
	assert.Equal(t, project.Shared, postgres.Scope)
	require.NotNil(t, postgres.Namespace)
	assert.Equal(t, "acme_{worktree}", postgres.Namespace.Name)
	assert.Equal(t, project.Shared, mailpit.Scope)
	assert.Nil(t, mailpit.Namespace)
	assert.Equal(t, []string{"postgres"}, w.job(t, "migrate").Touches)
}

func TestTheRecapCountsWhatRunTomlDeclares(t *testing.T) {
	w := newWorld(t)

	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})

	require.NoError(t, err)
	var declared int
	for _, job := range plan.Run.Jobs {
		declared += len(job.Ports)
	}
	assert.Len(t, recapRow(t, plan.Recap, "ports").Values, declared)
	assert.Len(t, plan.Preview.Ports, declared)
}

func TestProfilesAreAnsweredWholeTheirJobsAndTheDefault(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{Profiles: []string{"web=postgres,api,web", "api=postgres,api"}, DefaultProfile: "web"})

	assert.Equal(t, []project.Profile{{Name: "web", Jobs: []string{"postgres", "api", "web"}, Default: true}, {Name: "api", Jobs: []string{"postgres", "api"}}}, w.run(t).Profiles)
}

func TestTheRootScriptThatStartsTheAppsRunsThemThroughIt(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{Runner: runinit.Through})

	dev := w.job(t, "dev")
	assert.Equal(t, "pnpm dev", dev.Cmd)
	assert.Equal(t, []string{"admin", "api", "web", "worker"}, dev.Runs)
	assert.Equal(t, []string{"postgres", "redis", "mailpit", "migrate", "seed", "dev"}, w.run(t).Profiles[0].Jobs)
}

func TestARunnerLeftForOneByOneGivesItsPlaceInTheProfilesBackToTheApps(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{Runner: runinit.Through})

	w.apply(t, runinit.Request{Runner: runinit.OneByOne})

	run := w.run(t)
	assert.NotContains(t, jobNames(run), "dev")
	assert.Equal(t, []string{"postgres", "redis", "mailpit", "migrate", "seed", "admin", "api", "web", "worker"}, run.Profiles[0].Jobs)
}

func TestAReInitLeavingASharedServiceOutDropsItFromTheTasksThatTouchedIt(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})
	require.Equal(t, []string{"postgres"}, w.job(t, "migrate").Touches)

	w.apply(t, runinit.Request{Jobs: []string{"redis", "mailpit", "admin", "api", "web", "worker", "migrate", "seed"}})

	assert.Empty(t, w.job(t, "migrate").Touches)
	assert.Empty(t, w.job(t, "seed").Touches)
}

func TestADefaultProfileLeftOutWithItsJobFallsBackToTheFirst(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{DefaultProfile: "worker"})

	w.apply(t, runinit.Request{Jobs: []string{"postgres", "redis", "mailpit", "admin", "api", "web", "migrate", "seed"}})

	run := w.run(t)
	assert.Equal(t, "all", run.Profiles[0].Name)
	assert.True(t, run.Profiles[0].Default)
}

func TestEveryPortSyntaxIsRewrittenIntoAFileThatStillReadsTheSamePorts(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.main, "docker-compose.yml"), readFile(t, filepath.Join("testdata", "syntaxes.docker-compose.yml")))

	w.apply(t, runinit.Request{})

	formtest.Golden(t, "syntaxes.docker-compose.yml", readFile(t, filepath.Join(w.main, "docker-compose.yml")))
	rewritten, err := compose.Repository{Dir: w.main}.Compose()
	require.NoError(t, err)
	require.Empty(t, rewritten.Err)
	var hosts []string
	for _, service := range rewritten.Services {
		for _, port := range service.Ports {
			hosts = append(hosts, port.IP+" "+port.Host+":"+port.Container)
		}
	}
	assert.Equal(t, []string{" ${POSTGRES_PORT:-5432}:5432", "127.0.0.1 ${REDIS_PORT:-6379}:6379", " ${MAILPIT_PORT:-8025}:8025", "127.0.0.1 ${MAILPIT_PORT_1025:-1025}:1025"}, hosts)
}

func TestAPortVariableRenamedOnAReInitIsTheOneDockerComposeReads(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})

	w.apply(t, runinit.Request{Ports: []string{"postgres.POSTGRES_PORT=5432", "redis.CACHE_PORT=6379", "mailpit.MAILPIT_PORT=8025", "mailpit.MAILPIT_PORT_1025=1025", "admin.ADMIN_PORT=3001", "api.PORT=4000", "web.PORT=3000", "worker="}})

	file := readFile(t, filepath.Join(w.main, "docker-compose.yml"))
	assert.Contains(t, file, `"${CACHE_PORT:-6379}:6379"`)
	assert.NotContains(t, file, "REDIS_PORT")
}

func TestAProfileNamedWrongIsRefusedBeforeAnyFileIsWritten(t *testing.T) {
	cases := map[string]struct {
		profiles []string
		code     string
	}{
		"space": {profiles: []string{"my web=postgres,api"}, code: runinit.CodeProfileName},
		"empty": {profiles: []string{"=postgres"}, code: runinit.CodeProfileName},
		"twice": {profiles: []string{"web=postgres", "web=api"}, code: runinit.CodeProfileTwice},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			compose := readFile(t, filepath.Join(w.main, "docker-compose.yml"))

			_, err := w.usecase.Run(t.Context(), runinit.Request{Profiles: c.profiles}, domain.RunOptions{DryRun: true})
			assert.Equal(t, []string{c.code}, fieldCodes(err))
			_, err = w.usecase.Run(t.Context(), runinit.Request{Profiles: c.profiles}, domain.RunOptions{})

			require.Error(t, err)
			assert.Equal(t, compose, readFile(t, filepath.Join(w.main, "docker-compose.yml")))
			assert.NoFileExists(t, filepath.Join(w.stateDir, "run.toml"))
		})
	}
}

func TestApplyChecksRunTomlBeforeRewritingCompose(t *testing.T) {
	w := newWorld(t)
	compose := readFile(t, filepath.Join(w.main, "docker-compose.yml"))
	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})
	require.NoError(t, err)
	plan.Run.Profiles = append(plan.Run.Profiles, project.Profile{Name: "ghost", Jobs: []string{"nobody"}})

	result := w.usecase.Apply(t.Context(), plan, func(domain.Event) {})

	require.NotNil(t, result.Failure())
	assert.Equal(t, compose, readFile(t, filepath.Join(w.main, "docker-compose.yml")))
	assert.NoFileExists(t, filepath.Join(w.stateDir, "run.toml"))
}

func TestATaskKeepsTouchingTheRunnerItRunsThrough(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{Runner: runinit.Through})
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), strings.Replace(w.runToml(t), `touches = ["postgres"]`, `touches = ["postgres", "dev"]`, 1))

	w.apply(t, runinit.Request{Ports: []string{"postgres.POSTGRES_PORT=5432", "redis.REDIS_PORT=6380", "mailpit.MAILPIT_PORT=8025", "mailpit.MAILPIT_PORT_1025=1025", "admin.ADMIN_PORT=3001", "api.PORT=4000", "web.PORT=3000", "worker="}})

	assert.Equal(t, []string{"postgres", "dev"}, w.job(t, "migrate").Touches)
}

func TestAReInitOfARunTomlWithNoDefaultProfileMakesNone(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), strings.Replace(w.runToml(t), "  default = true\n", "", 1))

	w.apply(t, runinit.Request{Ports: []string{"postgres.POSTGRES_PORT=5432", "redis.REDIS_PORT=6380", "mailpit.MAILPIT_PORT=8025", "mailpit.MAILPIT_PORT_1025=1025", "admin.ADMIN_PORT=3001", "api.PORT=4000", "web.PORT=3000", "worker="}})

	for _, profile := range w.run(t).Profiles {
		assert.False(t, profile.Default, profile.Name)
	}
}

func TestADetachedWorktreesRecordKeepsItsNumberOutOfThePreview(t *testing.T) {
	w := newWorld(t, worktree.Worktree{Path: "detached", State: worktree.Detached{}})
	w.usecase.Meta = store.Meta{StateDir: t.TempDir()}
	_, err := w.usecase.Meta.(store.Meta).Update(t.Context(), worktree.MetaUpdate{Branch: "feat/old", Change: func(stored *worktree.Meta) error {
		stored.Ordinal, stored.SourceBranch = 1, "main"
		return nil
	}})
	require.NoError(t, err)

	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})

	require.NoError(t, err)
	assert.Equal(t, 2, plan.Preview.Ordinal)
}

func TestAProfileNamingTheRunnerIsRefusedWhenNothingRunsThroughIt(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.main, "package.json"), `{"name": "acme", "packageManager": "pnpm@9.12.0", "scripts": {"build": "turbo run build"}}`)

	_, err := w.usecase.Run(t.Context(), runinit.Request{Profiles: []string{"all=api,dev"}}, domain.RunOptions{DryRun: true})

	assert.Equal(t, []string{runinit.CodeUnknownJob}, fieldCodes(err))
}

func TestDockerComposeIsLeftAloneWhenAsked(t *testing.T) {
	w := newWorld(t)
	before := readFile(t, filepath.Join(w.main, "docker-compose.yml"))
	no := false

	outcome := w.apply(t, runinit.Request{PatchCompose: &no})

	assert.Equal(t, before, readFile(t, filepath.Join(w.main, "docker-compose.yml")))
	assert.Equal(t, []string{"run.toml"}, subjects(outcome.Result.Items))
	assert.Empty(t, w.job(t, "redis").Ports, "a literal port the file keeps binds the same in every worktree: none is declared")
	assert.False(t, w.job(t, "redis").BindsNoPort, "it binds one all the same")
}

func TestAReInitThatChangesNothingWritesNothing(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})
	before := w.runToml(t)

	outcome := w.apply(t, runinit.Request{})

	assert.Empty(t, outcome.Result.Items)
	assert.True(t, outcome.Plan.Empty)
	assert.Equal(t, before, w.runToml(t))
}

func TestAReInitKeepsOutWhatWasLeftOutAndAddsNothingFoundSince(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{Jobs: []string{"postgres", "redis", "mailpit", "admin", "api", "web", "migrate", "seed"}})
	writeFile(t, filepath.Join(w.main, "apps", "docs", "package.json"), `{"name": "docs", "scripts": {"dev": "astro dev"}}`)

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	assert.NotContains(t, questions.Decision.Picked, "worker")
	assert.NotContains(t, questions.Decision.Picked, "docs")
	assert.NotContains(t, profileNames(questions.Decision.Profiles), "worker", "a profile goes with the job it is named after")
	assert.Contains(t, candidates(questions.Decision), "docs", "the whole list is shown")
	assert.Equal(t, domain.OriginConfig, questions.Decision.Origins[runinit.PathJobs])
}

func TestAReInitKeepsAJobRunInitDoesNotProposeAsItIs(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), w.runToml(t)+"\n[[job]]\n  name = \"tunnel\"\n  kind = \"service\"\n  cmd = \"ngrok http 4000\"\n  binds_no_port = true\n")

	w.apply(t, runinit.Request{Addressing: project.AddressPorts})

	assert.Equal(t, "ngrok http 4000", w.job(t, "tunnel").Cmd)
	assert.Equal(t, project.AddressPorts, w.run(t).Addressing)
}

func TestAReInitOverALegacyRunTomlTakesItsJobsUnderTheirNames(t *testing.T) {
	w := newWorld(t)
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), "[[job]]\n  name = \"api-dev\"\n  kind = \"service\"\n  cmd = \"pnpm run dev\"\n  cwd = \"apps/api\"\n  [job.ports]\n    PORT = 4000\n")

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	assert.Equal(t, []string{"api-dev"}, questions.Decision.Picked)
}

func TestAFileChangedSinceThePlanWritesNoFile(t *testing.T) {
	w := newWorld(t)
	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})
	require.NoError(t, err)
	changed := readFile(t, filepath.Join(w.main, "docker-compose.yml")) + "# edited\n"
	writeFile(t, filepath.Join(w.main, "docker-compose.yml"), changed)

	result := w.usecase.Apply(t.Context(), plan, func(domain.Event) {})

	require.NotNil(t, result.Failure())
	assert.Equal(t, runinit.CodeFileChanged, result.Failure().Code)
	assert.NoFileExists(t, filepath.Join(w.stateDir, "run.toml"))
	assert.Equal(t, changed, readFile(t, filepath.Join(w.main, "docker-compose.yml")))
}

func TestARepositoryWithNoComposeFileSkipsTheSharedPart(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, os.Remove(filepath.Join(w.main, "docker-compose.yml")))

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	shared := sectionOf(t, questions.Form(), runinit.SectionShared)
	assert.Equal(t, domain.SectionSkipped, shared.State)
	assert.Equal(t, string(domain.SkipNoCompose), shared.Reason)
}

func TestTheDryRunPreviewsAWorktreeAtItsOrdinal(t *testing.T) {
	w := newWorld(t)

	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})

	require.NoError(t, err)
	preview := plan.Preview
	assert.Equal(t, billing, preview.Worktree)
	assert.Equal(t, 20, preview.Offset)
	assert.Contains(t, preview.Ports, runinit.PreviewPort{Job: "api", Variable: "PORT", Base: 4000, Port: 4020})
	assert.Contains(t, preview.Ports, runinit.PreviewPort{Job: "postgres", Variable: "POSTGRES_PORT", Base: 5432, Port: 5432, Shared: true})
	assert.Contains(t, preview.Data, runinit.PreviewData{Service: "postgres", Name: "acme_feat-billing", Template: "acme_{worktree}"})
	assert.Contains(t, preview.Addresses, runinit.PreviewAddress{Job: "api", URL: "http://api.feat-billing.acme.localhost:11080"})
}

func TestAWorktreeNeverNumberedIsPreviewedAtTheNumberItsFirstRunUpGivesIt(t *testing.T) {
	w := newWorld(t)
	w.usecase.Meta = store.Meta{StateDir: t.TempDir()}

	plan, _, err := w.usecase.Plan(t.Context(), runinit.Request{})

	require.NoError(t, err)
	assert.Equal(t, 1, plan.Preview.Ordinal)
	assert.False(t, plan.Preview.Numbered)
	assert.Contains(t, plan.Preview.Ports, runinit.PreviewPort{Job: "api", Variable: "PORT", Base: 4000, Port: 4010})
}

func TestAPortVariableRenamedIsTheOneDockerComposeReads(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{Ports: []string{"postgres.POSTGRES_PORT=5432", "redis.CACHE_PORT=6379", "mailpit.MAILPIT_PORT=8025", "mailpit.MAILPIT_PORT_1025=1025", "admin.ADMIN_PORT=3001", "api.PORT=4000", "web.PORT=3000", "worker="}})

	assert.Equal(t, map[string]int{"CACHE_PORT": 6379}, w.job(t, "redis").Ports)
	assert.Contains(t, readFile(t, filepath.Join(w.main, "docker-compose.yml")), `"${CACHE_PORT:-6379}:6379"`)
}

func TestAReInitKeepsWhatRunTomlSaysOfAJobRunInitDoesNotAsk(t *testing.T) {
	w := newWorld(t)
	w.apply(t, runinit.Request{})
	edited := strings.Replace(w.runToml(t), "  cwd = \"apps/api\"\n", "  cwd = \"apps/api\"\n  stop = \"pkill -f tsx\"\n", 1)
	writeFile(t, filepath.Join(w.stateDir, "run.toml"), edited)

	w.apply(t, runinit.Request{Addressing: project.AddressPorts})

	assert.Equal(t, "pkill -f tsx", w.job(t, "api").Stop)
}
