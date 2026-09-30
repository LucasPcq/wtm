package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func service(name string, ports map[string]int) domain.JobConfig {
	return domain.JobConfig{Name: name, Kind: domain.JobKindService, Ports: ports}
}

// Le premier service reçoit PORT, les suivants <NOM>_PORT : c'est ce que
// freePortName écrit, et donc les deux seules formes sous lesquelles un job
// déclare le port qu'il écoute.
func TestURLCandidatesForReadsBothListeningPortNames(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("web", map[string]int{domain.PortNameDefault: 3000}),
		service("api", map[string]int{"API_PORT": 3100}),
	}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg, NewJobs: []string{"web", "api"}})
	want := []domain.JobURLChoice{
		{Job: "web", Port: domain.PortNameDefault, Publish: true},
		{Job: "api", Port: "API_PORT", Publish: true},
	}
	assertChoices(t, got, want)
}

// Un port qu'un job compose n'est pas un port qu'il écoute : le publier
// annoncerait un nom que rien ne sert.
func TestURLCandidatesForIgnoresDialledPorts(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("api", map[string]int{"DB_PORT": 5432, "REDIS_PORT": 6379}),
		service("worker", nil),
		{Name: "migrate", Kind: domain.JobKindTask, Ports: map[string]int{domain.PortNameDefault: 3000}},
	}}

	if got := URLCandidatesFor(URLCandidatesForParams{Config: cfg}); len(got) != 0 {
		t.Errorf("candidats = %v, want aucun", got)
	}
}

func TestURLCandidatesForPrefersPortOverTheDerivedName(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("api", map[string]int{"API_PORT": 3100, domain.PortNameDefault: 3000}),
	}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg, NewJobs: []string{"api"}})
	assertChoices(t, got, []domain.JobURLChoice{{Job: "api", Port: domain.PortNameDefault, Publish: true}})
}

// Une url déjà écrite est reprise telle quelle : l'étape la propose pour
// pouvoir la retirer, jamais pour désigner un autre port à la place.
func TestURLCandidatesForKeepsTheDeclaredPort(t *testing.T) {
	api := service("api", map[string]int{domain.PortNameDefault: 3000, "DB_PORT": 5432})
	api.URL = &domain.JobURLConfig{Port: "DB_PORT"}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{api}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg})
	assertChoices(t, got, []domain.JobURLChoice{{Job: "api", Port: "DB_PORT", Publish: true}})
}

// Un job qui ne serait pas candidat mais porte déjà une url reste listé :
// autrement, elle ne pourrait plus être décochée.
func TestURLCandidatesForListsAPublishedJobWithNoListeningPort(t *testing.T) {
	job := service("legacy", map[string]int{"DB_PORT": 5432})
	job.URL = &domain.JobURLConfig{Port: "DB_PORT"}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{job}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg})
	assertChoices(t, got, []domain.JobURLChoice{{Job: "legacy", Port: "DB_PORT", Publish: true}})
}

func assertChoices(t *testing.T, got, want []domain.JobURLChoice) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("candidats = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidat[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestApplyInitAnswersPublishesTheChosenPort(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{service("web", map[string]int{domain.PortNameDefault: 3000})}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:    cfg,
		URLs:      []string{"web"},
		URLsAsked: true,
	})

	if got.Jobs[0].URL == nil || got.Jobs[0].URL.Port != domain.PortNameDefault {
		t.Fatalf("url = %v, want le port PORT publié", got.Jobs[0].URL)
	}
	if cfg.Jobs[0].URL != nil {
		t.Error("la config d'entrée a été modifiée")
	}
}

// Le host est le seul champ que l'étape ne sait pas proposer : elle ne doit pas
// pouvoir l'effacer non plus.
func TestApplyInitAnswersKeepsAHandWrittenHost(t *testing.T) {
	job := service("web", map[string]int{domain.PortNameDefault: 3000})
	job.URL = &domain.JobURLConfig{Port: domain.PortNameDefault, Host: "app.acme"}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{job}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:    cfg,
		URLs:      []string{"web"},
		URLsAsked: true,
	})

	if got.Jobs[0].URL.Host != "app.acme" {
		t.Errorf("host = %q, want app.acme", got.Jobs[0].URL.Host)
	}
}

// Décocher est une réponse, comme une base à zéro dans l'étape des ports.
func TestApplyInitAnswersWithdrawsADeselectedURL(t *testing.T) {
	job := service("mailhog", map[string]int{domain.PortNameDefault: 8025})
	job.URL = &domain.JobURLConfig{Port: domain.PortNameDefault}
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{job}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:    cfg,
		URLsAsked: true,
	})

	if got.Jobs[0].URL != nil {
		t.Errorf("url = %v, want retirée", got.Jobs[0].URL)
	}
	if cfg.Jobs[0].URL == nil {
		t.Error("la config d'entrée a été modifiée")
	}
}

func TestApplyInitAnswersIgnoresAChoiceNamingNoJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{service("web", map[string]int{domain.PortNameDefault: 3000})}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:    cfg,
		URLs:      []string{"absent"},
		URLsAsked: true,
	})

	if got.Jobs[0].URL != nil {
		t.Errorf("url = %v, want aucune", got.Jobs[0].URL)
	}
}

// Rien n'a été demandé : la proposition vaut réponse, comme pour les profils.
func TestResolveURLChoicesPublishesEveryCandidateWhenNothingWasAsked(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("web", map[string]int{domain.PortNameDefault: 3000}),
		service("api", map[string]int{"API_PORT": 3100}),
	}}

	got := ResolveURLChoices(ResolveURLChoicesParams{Config: cfg, NewJobs: []string{"web", "api"}})
	assertChoices(t, got, []domain.JobURLChoice{
		{Job: "web", Port: domain.PortNameDefault, Publish: true},
		{Job: "api", Port: "API_PORT", Publish: true},
	})
}

func TestResolveURLChoicesFollowsWhatWasChecked(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("web", map[string]int{domain.PortNameDefault: 3000}),
		service("api", map[string]int{"API_PORT": 3100}),
	}}

	got := ResolveURLChoices(ResolveURLChoicesParams{Config: cfg, Asked: true, Published: []string{"web"}})
	assertChoices(t, got, []domain.JobURLChoice{
		{Job: "web", Port: domain.PortNameDefault, Publish: true},
		{Job: "api", Port: "API_PORT", Publish: false},
	})
}

// Tout décocher est une réponse, pas une absence de réponse : sans le drapeau
// Asked, elle serait relue comme « on n'a rien demandé » et tout republierait.
func TestResolveURLChoicesHonoursAnEmptySelection(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{service("web", map[string]int{domain.PortNameDefault: 3000})}}

	got := ResolveURLChoices(ResolveURLChoicesParams{Config: cfg, Asked: true})
	assertChoices(t, got, []domain.JobURLChoice{{Job: "web", Port: domain.PortNameDefault, Publish: false}})
}

// Le port déclaré dans l'étape des ports arrive par le même appel que l'url :
// juger la candidature avant de l'appliquer laissait sans nom le job qui vient
// tout juste d'en recevoir un.
func TestApplyInitAnswersPublishesAJobPortedInTheSameCall(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{service("web", nil)}}

	got := ApplyInitAnswers(ApplyInitAnswersParams{
		Config:    cfg,
		Ports:     []domain.PortEntry{{Job: "web", Name: domain.PortNameDefault, Base: 3000}},
		URLs:      []string{"web"},
		URLsAsked: true,
	})

	if got.Jobs[0].URL == nil {
		t.Fatal("url = nil : le port posé par le même appel doit rendre le job publiable")
	}
	if got.Jobs[0].URL.Port != domain.PortNameDefault {
		t.Errorf("url.port = %q, want %q", got.Jobs[0].URL.Port, domain.PortNameDefault)
	}
}

// Un job déjà dans run.toml sans url a vu la question et y a répondu non.
// Le recocher recréerait l'url que l'utilisateur venait de retirer.
func TestURLCandidatesForLaisseDecocheUnJobDepublie(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("web", map[string]int{domain.PortNameDefault: 3000}),
	}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg})

	assertChoices(t, got, []domain.JobURLChoice{
		{Job: "web", Port: domain.PortNameDefault, Publish: false},
	})
}

// Un job que cette passe vient d'ajouter n'a jamais vu la question : la
// proposition tient, comme au premier init.
func TestURLCandidatesForCocheUnJobNeuf(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		service("web", map[string]int{domain.PortNameDefault: 3000}),
	}}

	got := URLCandidatesFor(URLCandidatesForParams{Config: cfg, NewJobs: []string{"web"}})

	assertChoices(t, got, []domain.JobURLChoice{
		{Job: "web", Port: domain.PortNameDefault, Publish: true},
	})
}

// A Vite app declares VITE_PORT and nothing else. Reading only PORT and
// <JOB>_PORT left every front-end — the jobs a human opens in a browser —
// without a name, which is what the whole named-URL layer is for.
func TestPublishablePortNameTakesASingleDeclaredPort(t *testing.T) {
	got := PublishablePortName(domain.JobConfig{
		Name:  "crm-web-dev",
		Kind:  domain.JobKindService,
		Ports: map[string]int{"VITE_PORT": 5175},
	})

	if got != "VITE_PORT" {
		t.Errorf("got %q, want VITE_PORT", got)
	}
}

// Several ports are several meanings, and only a name settles which one is
// listened on: a compose stack publishing seven of them publishes no url.
func TestPublishablePortNameRefusesToGuessAmongSeveral(t *testing.T) {
	got := PublishablePortName(domain.JobConfig{
		Name:  "docker-compose",
		Kind:  domain.JobKindService,
		Ports: map[string]int{"POSTGRES_PORT": 5432, "REDIS_PORT": 6379},
	})

	if got != "" {
		t.Errorf("got %q, want nothing published", got)
	}
}

// The wiring question stays strict: a job whose only port is one it dials is
// still offered a PORT of its own.
func TestListeningPortNameStaysStrict(t *testing.T) {
	got := ListeningPortName(domain.JobConfig{
		Name:  "api-dev",
		Kind:  domain.JobKindService,
		Ports: map[string]int{"DB_PORT": 5432},
	})

	if got != "" {
		t.Errorf("got %q, want nothing: a DB_PORT is dialled, not listened on", got)
	}
}
