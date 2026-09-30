package rules_test

import (
	"fmt"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func profileNames(profiles []domain.ProfileConfig) []string {
	out := make([]string, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, p.Name)
	}
	return out
}

func findProfile(t *testing.T, profiles []domain.ProfileConfig, name string) domain.ProfileConfig {
	t.Helper()
	for _, p := range profiles {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no profile %q in %v", name, profileNames(profiles))
	return domain.ProfileConfig{}
}

var (
	composeJob = domain.JobConfig{Name: "docker-compose", Kind: domain.JobKindService, Cwd: ".", Stop: "docker compose down"}
	apiJob     = domain.JobConfig{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api"}
	webJob     = domain.JobConfig{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"}
	lintJob    = domain.JobConfig{Name: "lint", Kind: domain.JobKindTask, Cwd: "."}
	fanOutJob  = domain.JobConfig{Name: "dev:all", Kind: domain.JobKindService, Cwd: "."}
)

func TestIsSharedJob(t *testing.T) {
	if !rules.IsSharedJob(composeJob) {
		t.Error("une stack détachée à la racine est de l'infra partagée")
	}
	if !rules.IsSharedJob(lintJob) {
		t.Error("une task à la racine est une étape que tout profil doit franchir")
	}
	if rules.IsSharedJob(apiJob) {
		t.Error("un job dans un package n'est pas partagé")
	}
	if rules.IsSharedJob(fanOutJob) {
		t.Error("un service au premier plan à la racine démarre les packages lui-même")
	}
}

// Un `turbo run dev` à la racine démarre déjà tous les packages : l'ajouter au
// profil d'un package démarre tout le dépôt à côté de ce package, deux fois sur
// les mêmes ports. Il a son propre profil.
func TestProposeProfilesGivesAFanOutItsOwnProfile(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, fanOutJob, apiJob, webJob}},
	})

	api := findProfile(t, profiles, "api")
	for _, job := range api.Jobs {
		if job == fanOutJob.Name {
			t.Errorf("api = %v, want the fan-out left out", api.Jobs)
		}
	}

	own := findProfile(t, profiles, fanOutJob.Name)
	if len(own.Jobs) != 2 || own.Jobs[1] != fanOutJob.Name {
		t.Errorf("%s = %v, want the shared infra then the fan-out", fanOutJob.Name, own.Jobs)
	}
}

func TestProposeProfilesGroupsByPackage(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
	})

	if got := profileNames(profiles); len(got) != 3 {
		t.Fatalf("expected 3 profiles (api, web, all), got %v", got)
	}

	api := findProfile(t, profiles, "api")
	if len(api.Jobs) != 2 || api.Jobs[0] != "docker-compose" || api.Jobs[1] != "api-dev" {
		t.Errorf("api = %v, want [docker-compose api-dev]", api.Jobs)
	}

	all := findProfile(t, profiles, domain.ProfileAllName)
	if len(all.Jobs) != 3 {
		t.Errorf("all = %v, want the three jobs", all.Jobs)
	}
	if !all.Default {
		t.Error("le profil global est le default pré-sélectionné dans le picker")
	}
}

func TestProposeProfilesCollapsesInASinglePackageRepo(t *testing.T) {
	// Un profil par package plus un global se confondent quand il n'y a qu'un
	// package : la règle doit dégrader d'elle-même, sans cas particulier.
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, {
			Name: "dev", Kind: domain.JobKindService, Cwd: ".",
		}}},
	})

	if len(profiles) != 1 {
		t.Fatalf("expected a single profile, got %v", profileNames(profiles))
	}
	if !profiles[0].Default {
		t.Error("le profil unique doit être le default")
	}
}

// Une task entre dans le profil, et avant les services : `run up` démarrait
// sinon un serveur sur une base qu'aucune migration n'avait touchée (LUC-208).
func TestProposeProfilesPutsTasksBeforeServices(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, lintJob}},
	})

	all := findProfile(t, profiles, domain.ProfileAllName)
	if len(all.Jobs) != 3 {
		t.Fatalf("all = %v, want the three jobs", all.Jobs)
	}
	if all.Jobs[0] != "lint" {
		t.Errorf("all = %v; la task doit précéder les services qui en dépendent", all.Jobs)
	}
}

func TestProposeProfilesKeepsTheExistingSplit(t *testing.T) {
	// Sur un init relancé, la proposition est la configuration en place : on
	// montre ce qu'il y a, on n'infère pas par-dessus une composition faite à
	// la main.
	existing := []domain.ProfileConfig{
		{Name: "app1", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
	}

	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob, webJob}},
		Existing: existing,
	})

	if len(profiles) != 1 || profiles[0].Name != "app1" {
		t.Fatalf("expected the existing split, got %v", profileNames(profiles))
	}
}

func TestApplyInitAnswersCorrectsAPort(t *testing.T) {
	cfg := rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "web-dev", Kind: domain.JobKindService, Ports: map[string]int{"WEB_PORT": 5173}},
		}},
		Ports: []domain.PortEntry{{Job: "web-dev", Name: "WEB_PORT", Base: 4000}},
	})

	if got := cfg.Jobs[0].Ports["WEB_PORT"]; got != 4000 {
		t.Errorf("port = %d, want the corrected 4000", got)
	}
}

func TestApplyInitAnswersNeverInventsAJob(t *testing.T) {
	// Une entrée qui ne correspond à aucun job ne doit rien créer : l'étape
	// revoit les jobs retenus, elle n'en ajoute pas.
	cfg := rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web-dev", Kind: domain.JobKindService}}},
		Ports:  []domain.PortEntry{{Job: "ghost", Name: "WEB_PORT", Base: 4000}},
	})

	if len(cfg.Jobs) != 1 || len(cfg.Jobs[0].Ports) != 0 {
		t.Errorf("config = %+v, want it untouched", cfg.Jobs)
	}
}

func TestApplyInitAnswersWritesTheComposedSplit(t *testing.T) {
	cfg := rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
		Config:   domain.RunConfig{Jobs: []domain.JobConfig{composeJob, apiJob}},
		Profiles: []domain.ProfileConfig{{Name: "app1", Jobs: []string{"docker-compose", "api-dev"}, Default: true}},
	})

	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Name != "app1" {
		t.Errorf("profiles = %+v", cfg.Profiles)
	}
}

// filepath.Base fusionnait silencieusement apps/app-1/back et apps/app-2/back
// dans un seul profil portant les jobs des deux applications (LUC-208).
func TestProposeProfilesDisambiguatesCollidingBaseNames(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "one-dev", Kind: domain.JobKindService, Cwd: "apps/app-1/back"},
			{Name: "two-dev", Kind: domain.JobKindService, Cwd: "apps/app-2/back"},
		}},
	})

	names := profileNames(profiles)
	if len(profiles) != 3 {
		t.Fatalf("expected one profile per package plus the global one, got %v", names)
	}
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Fatalf("%q apparaît deux fois: %v", name, names)
		}
		seen[name] = true
	}
	for _, profile := range profiles {
		if profile.Name != domain.ProfileAllName && len(profile.Jobs) != 1 {
			t.Errorf("%s = %v; deux packages distincts ne partagent pas un profil", profile.Name, profile.Jobs)
		}
	}
}

// Au-delà du seuil, un profil par package n'est plus une proposition qu'on
// relit, c'est une liste qu'on fait défiler.
func TestProposeProfilesStopsSplittingPastTheThreshold(t *testing.T) {
	var jobs []domain.JobConfig
	for i := 0; i <= domain.ProfileProposalMaxPackages; i++ {
		jobs = append(jobs, domain.JobConfig{
			Name: fmt.Sprintf("pkg-%d-dev", i),
			Kind: domain.JobKindService,
			Cwd:  fmt.Sprintf("packages/pkg-%d", i),
		})
	}

	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{Config: domain.RunConfig{Jobs: jobs}})
	if len(profiles) != 1 || profiles[0].Name != domain.ProfileAllName {
		t.Fatalf("expected only the global profile, got %v", profileNames(profiles))
	}
}

// Le profil d'un package porte aussi les tasks partagées, avant ses services.
func TestProposeProfilesCarriesSharedTasksIntoEachPackage(t *testing.T) {
	profiles := rules.ProposeProfiles(rules.ProposeProfilesParams{
		Config: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "api-dev", Kind: domain.JobKindService, Cwd: "apps/api"},
			{Name: "web-dev", Kind: domain.JobKindService, Cwd: "apps/web"},
			{Name: "migrate", Kind: domain.JobKindTask, Cwd: "."},
		}},
	})

	api := findProfile(t, profiles, "api")
	if len(api.Jobs) != 2 || api.Jobs[0] != "migrate" || api.Jobs[1] != "api-dev" {
		t.Errorf("api = %v, want [migrate api-dev]", api.Jobs)
	}
}

// Tout supprimer dans la step est une réponse : la réinstaller effacerait le
// geste de l'utilisateur au moment même de l'écriture.
func TestApplyInitAnswersEcritUneListeDeProfilsVideQuiAEteDemandee(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"web"}}},
	}

	got := rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{Config: cfg, ProfilesAsked: true})

	if len(got.Profiles) != 0 {
		t.Errorf("profils = %v, want aucun", got.Profiles)
	}
}

// Une step jamais affichée ne retire rien : un init non interactif garde la
// proposition, sans quoi `run up` n'aurait plus rien à démarrer.
func TestApplyInitAnswersGardeLesProfilsQuandLaStepNaPasTourne(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}},
		Profiles: []domain.ProfileConfig{{Name: "dev", Jobs: []string{"web"}}},
	}

	got := rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{Config: cfg})

	if len(got.Profiles) != 1 {
		t.Errorf("profils = %v, want la proposition conservée", got.Profiles)
	}
}
