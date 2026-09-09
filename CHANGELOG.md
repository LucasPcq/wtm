# Changelog

## v0.27.1 — Un worktree enfant ne pousse plus sur la branche de son parent

### Bug fixes

- **Un worktree créé depuis un parent qui n'existe que sur `origin` poussait sur la branche du parent.** wtm passait la ref distante telle quelle à `git worktree add -b`, et git — via son défaut `branch.autoSetupMerge` — configurait alors `origin/<parent>` comme upstream de la branche enfant. Un `git push` avec `push.default = upstream` écrasait directement la branche du parent sur le remote ; avec le défaut `simple`, git refusait mais suggérait `git push origin HEAD:<parent>`, qui fait exactement la même chose. La création passe désormais `--no-track` : la branche enfant n'a plus d'upstream tant qu'elle n'a pas été poussée, et `git push` propose la bonne branche. Le flag prime sur `branch.autoSetupMerge` quelle que soit sa valeur, donc aucune configuration git ne peut ramener le problème. Au passage, `wtm prune --gone` ne proposera plus de supprimer un worktree enfant jamais poussé parce que la branche du parent a disparu du remote.

  **Le correctif n'est pas rétroactif.** Les worktrees enfants créés par une version antérieure gardent leur upstream erroné. Pour vérifier un worktree suspect, depuis son répertoire : `git rev-parse --abbrev-ref @{upstream}` — s'il répond le nom d'une *autre* branche que la sienne, corrigez avec `git branch --unset-upstream`, puis publiez la branche normalement avec `git push -u origin HEAD`.

## v0.27.0 — `wtm upgrade`, `wtm fast-forward` et une aide qui se lit

Deux nouvelles commandes : `wtm upgrade` met le CLI à jour tout seul, `wtm fast-forward` avance une branche sur son homologue distant sans rien rejouer. L'overlay d'aide du dashboard est repris de zéro pour redevenir une référence consultable.

### New features

- **`wtm upgrade` — mettre wtm à jour depuis wtm.** La commande fait ce qu'il faut selon l'installation : un binaire autonome est remplacé sur place après vérification de son SHA256 contre les checksums de la release, un binaire Homebrew ou `go install` est confié à son gestionnaire plutôt que désynchronisé, et un binaire compilé depuis les sources est refusé. `--check` dit ce qui est disponible sans rien toucher, `--version` épingle une release précise.
- **Une notification passive de mise à jour.** wtm signale en fin de commande qu'une version plus récente existe, sans jamais bloquer ni ralentir la commande en cours. Le header de `wtm ui` affiche la version installée et l'appel à la mise à jour.
- **`wtm fast-forward` — avancer une branche sur `origin/<branche>`.** Rien d'autre : pas de rebase sur le parent, pas de merge. Une branche qui a divergé est refusée — c'est `wtm sync` qui rejoue les commits locaux, et `--force` ne lève pas ce refus. Disponible au clavier depuis le dashboard comme en CLI (`--all`, ou sélection interactive).

### Improvements

- **L'overlay d'aide du dashboard se lit comme un document.** Les 17 lignes à plat deviennent quatre sections (NAV, ACT, MOUSE, VIEW), appariées sur un écran large et empilées sur un écran étroit ; la souris devient une section à part au lieu d'un suffixe sur chaque touche. L'overlay se dimensionne sur l'écran et fait défiler ce qui dépasse — sur un écran court, sa bordure basse était coupée sans un mot — et il répond enfin à la molette et au clic, lui qui était la seule surface à documenter la souris en l'ignorant. `h`/`l`, que le dashboard bindait sans les lister, y figurent désormais.

### Bug fixes

- **`wtm sync` interactif affichait un picker vide au lieu de son récapitulatif.** L'étape de confirmation montrait « No matches » et le plan n'était jamais chargé.
- **Le placeholder de chargement n'affiche plus « No matches ».** Ce message répond à un filtre, pas à un chargement en cours (le plan de `sync`, la vérification des worktrees de `clean`).

## v0.26.1 — Le panneau détail ne clignote plus

### Bug fixes

- **Le panneau détail de `wtm ui` se rechargeait tout seul toutes les trois secondes** — le
  poll qui rafraîchit la liste rechargeait aussi le détail du worktree sélectionné, si bien
  qu'un panneau qu'on était en train de lire passait en gris derrière un marqueur
  `refreshing` en continu. Le poll ne touche plus au détail : il se recharge quand la
  sélection change, quand une opération vient de toucher sa branche, et sur la touche `r`
  — jamais sur une horloge. La liste, elle, garde son poll court.

## v0.26.0 — `wtm ui` : un dashboard pour piloter ses worktrees

Cette version apporte `wtm ui`, un dashboard plein écran d'où se pilotent les commandes
mutantes, et la couche `internal/flow` qui le rend possible : chaque commande décrit son
déroulé une seule fois, indépendamment de la surface, et le CLI comme le dashboard le
rejouent. Aucune commande, aucun flag et aucune charge `--output json` existants n'ont
changé.

### New features

- **`wtm ui` — le dashboard plein écran.** `create`, `clean`, `reparent`, `prune` et
  `sync` s'y lancent avec leurs questions, leurs refus de sécurité et leur sortie
  streamée dans un panneau dédié. Navigation clavier et souris, aide complète sur `?`.
- **Un onglet Tree** pour voir la forêt de branches empilées, et le reparentage directement
  depuis le dashboard.
- **Un panneau détail qui décrit le travail d'un worktree**, pas seulement son
  emplacement : dernier commit et activité récente, état réel du working tree
  (`4 modified · 2 untracked · 1 staged` avec son volume de diff), enfants, drift `.env`,
  ce qui empêche une suppression, et la pull request dépliée avec ses checks CI et sa
  décision de review. Une section n'apparaît que si elle a quelque chose à dire — le
  panneau ne récite pas `none` ni `up to date`.
  Il distingue aussi trois absences qui se ressemblent : une donnée en cours de
  rechargement, une absence légitime (`not configured`) et une panne
  (`⚠ unavailable — <raison>`). Un `gh` cassé ne se lit pas « pas de PR ».
- **La pull request s'ouvre dans le navigateur** depuis le panneau, à la touche `p` ou au
  clic sur sa ligne.
- **`wtm list` marque le worktree courant.** Son tag `● active` existait dans le code mais
  n'était jamais renseigné : il fonctionne désormais, dans la sortie texte comme dans le
  sélecteur interactif, et `wtm resolve` le montre aussi. Le dashboard le signale de son
  côté dans son header, sa liste, son arbre et son détail.
- **`ui.animations`** — nouvelle clé de config globale (`~/.config/wtm/config.toml`). Les
  animations du dashboard sont actives par défaut ; `false` les éteint toutes d'un coup,
  utile sur une liaison lente.

### Improvements

- **Nouvelle palette de couleurs**, appliquée à la sortie humaine de **toutes** les
  commandes, pas seulement au dashboard : accents retravaillés et rôles clarifiés
  (navigation, identité, état, structure). Les couleurs restent adaptatives clair/sombre
  et `NO_COLOR` est toujours respecté.

### Bug fixes

- **`wtm sync`** : l'état du parent est affiné et l'étape parent reste visible ; les
  parents que la cascade ne couvre pas sont désormais rafraîchis.
- **La configuration globale documentée était invalide.** Le `README` montrait une clé
  `agent` sous `~/.config/wtm/config.toml`, absente du schéma — et le décodage refusant
  les clés inconnues, copier le bloc documenté faisait échouer **toutes** les commandes
  avec `unknown keys in config.toml: agent`.

## v0.25.0 — Réutiliser une branche locale existante (`create`, `checkout`, `extract`)

### New features

- **`checkout <PR>` réutilise une branche locale existante** — refusait auparavant la PR
  dès que la branche locale existait, en demandant un `wtm clean` (donc de supprimer des
  commits non poussés). Elle réutilise désormais la branche telle quelle et propose un
  fast-forward interactif quand elle est en retard sur origin — jamais sous `--yes`/JSON,
  qui ne touchent aucune ref. Le recap annonce la réutilisation avant même le fetch, pas
  après.
- **`create <branche-existante>` et `extract --to <branche-existante>` réutilisent
  proprement** — la source devient un simple parent de sync optionnel côté wizard,
  `--ff` change de sujet pour cibler la branche réutilisée plutôt qu'une source hors
  sujet, et le recap comme la sortie finale annoncent la réutilisation
  (`Branch: x (existing local branch — reused)` + `Parent:`) au lieu du `from:` trompeur
  d'avant.
- **Une branche déjà checked out ailleurs est signalée proprement** — remontait une
  erreur git brute en exit `1` ; lève désormais `ErrWorktreeExists` (exit `10`) avec un
  hint `wtm go <branche>`. `--if-not-exists` renvoie le worktree existant, y compris
  quand c'est le worktree main qui détient la branche.
- **Nouveaux champs JSON** — `existing_branch`, `origin_state`, `origin_ahead` et
  `origin_behind` sur `create` et `checkout` signalent la réutilisation et sa
  divergence avec origin (`up-to-date`/`behind`/`ahead`/`diverged`).

### Breaking changes

- **`wtm create <branche-existante> --yes` exige désormais `--from`** — `extract --to
  <branche-existante> --yes` aussi. Le parent d'une branche créée hors de wtm ne peut
  pas être deviné sans risquer que `sync`, `tree` et `reparent` traitent la supposition
  comme un fait ; la commande échoue en nommant le flag plutôt que de dégrader
  silencieusement vers `base_branch`. Une branche neuve garde son défaut `base_branch`,
  et `checkout <PR>` garde le sien (la base de la PR est un fait GitHub, pas une
  supposition).

### Improvements

- **L'état de la branche cible n'est inspecté qu'une fois par run** — `branch.Target`
  (nouveau `service/branch/target.go`) est désormais mémoïsé par nom de branche pour la
  durée d'un wizard interactif, évitant des dizaines d'appels git redondants par run
  (`create` en tournait ~20-25 auparavant).
- **`wtm checkout` a maintenant des tests** — jusqu'ici sans aucune couverture, il en
  gagne deux (réutilisation de branche, non-mutation des refs sous `--yes`), en testant
  la logique de création directement, sans dépendance réseau à GitHub.
- Nouveaux tests de bout en bout : `--yes` qui ne fast-forward jamais une branche en
  retard, `--ff` qui retargete la branche réutilisée et pas la source, `origin_state`
  vérifié en JSON, garde-fou `--from` sur `extract`.
- Chaînes utilisateur dupliquées ("Fast-forward %s to origin", "Updating %s from
  origin…", la ligne de recap "Update: fast-forward…") centralisées dans
  `domain/constants.go`.

## v0.24.1 — `wtm extract` : les fichiers non trackés enfin gérés pour de bon

### Bug fixes

- **Les dossiers entièrement neufs ne sont plus tout-ou-rien** — `wtm extract` énumérait ses
  candidats avec `git status --porcelain`, qui replie un dossier non tracké en une seule entrée
  `newmod/`. Le picker ne montrait que le dossier et `--files newmod/x.go` échouait sur
  « is not an uncommitted change ». L'énumération passe à `--untracked-files=all` : chaque fichier
  est listé et sélectionnable individuellement.
- **Les chemins avec espaces ou caractères non-ASCII étaient inextractibles** — git les quote
  (`?? "a b.txt"`, `?? "caf\303\251.txt"`) et le parser conservait guillemets et échappements, si
  bien qu'aucun `--files` ne matchait et qu'une sélection dans le picker échouait à la copie. Le
  passage à `git status --porcelain -z` supprime le quoting à la source ; les chemins sont rendus
  verbatim. Le défaut touchait aussi les fichiers trackés.
- **Les renommages indexés sont enfin extractibles** — un record `R` était lu comme un chemin bidon
  `old -> new`. Le chemin d'origine est désormais lu dans son champ dédié et les deux chemins
  partent ensemble dans le patch (sans quoi la cible recevait l'ajout sans la suppression).
  Nouveau statut `renamed` et champ `orig_path` dans `--output json`, tag `ren` dans le picker.
- **`--on-conflict resolve` couvre les fichiers non trackés** — un fichier non tracké déjà présent
  dans la cible provoquait un abort inconditionnel, avant même que le mode de conflit soit
  consulté ; il n'existait aucune échappatoire. Il rejoint l'axe `--on-conflict` : `abort` reste
  le défaut sûr (y compris sous `--yes`), `resolve` écrit des marqueurs de conflit via un merge
  à base vide. Un contenu identique des deux côtés n'est plus compté comme un conflit. Un fichier
  binaire, qui ne peut pas porter de marqueurs, continue d'aborter avec un message explicite.
  Le code de sortie `15` est inchangé.

### Improvements

- **`--files` accepte un dossier** — `--files newmod/` prend tous les changements en dessous.
  Les entrées « dossier » ayant disparu de l'énumération, c'est ce qui garde fonctionnels les
  scripts existants qui passaient un dossier.
- **Les dossiers vidés sont purgés côté source** — déplacer tous les fichiers d'un dossier neuf ne
  laisse plus une arborescence vide derrière lui.
- Les fichiers ignorés par `.gitignore` restent volontairement hors de `extract` : `.env` et
  consorts relèvent de `wtm env`.

## v0.24.0 - Module `env` : Détection & gestion des conflits d'un worktree à un autre

### New features 

- **wtm env [branch]** - Détecter et de gérer les conflits d'un worktree en fonction de la méthode choisi lors de la création (main, parent, exemple). Sans argument : Affiche un picker interactif.

### Improvements

- Mise à jour du skill et de la documentation

## v0.23.0 — Hooks `on_clean`, détection `.env` généralisée & sorties harmonisées

### Breaking changes

- **La config `.env` passe d'un `copy_files` plat à un modèle structuré `[[env.file]]`** — chaque entrée décrit une **cible** (`target`, le fichier de valeurs à provisionner, ex. `.env`, `apps/api/.env`) et son **template committé** (`template`, ex. `.env.example`), avec `.env.local` détecté et marqué `local = true`. `copy_files` est abandonné **sans migration douce** : un `wtm.toml` qui l'utilise doit être régénéré (`wtm init --only env`) ou édité à la main. La détection reconnaît désormais `.env.example` / `.env.dist` / `.env.sample` / `.env.template` / `.env.tmpl` comme templates (priorité dans cet ordre) et distingue templates (committés) et fichiers de valeurs (gitignorés) (LUC-89).

### New features

- **Hooks `on_clean` — teardown avant suppression** — une nouvelle liste `[hooks] on_clean` s'exécute **dans le worktree, juste avant** que `clean`/`prune` ne le retirent (ex. `docker compose down` pour libérer des ressources externes). Un hook qui sort non-zéro **abandonne la suppression** sauf si son entrée pose `continue_on_error`. Interpolation `{{worktree}}` / `{{branch}}` / `{{root}}` (le `{{from_branch}}` reste réservé à `on_create`). `wtm init` gagne `--clean-command` et `--skip-clean` pour configurer la section en non-interactif (#45).
- **Fallback `sudo rm -rf` sur suppression bloquée** — si `git worktree remove` échoue sur des fichiers non-supprimables par l'utilisateur courant (typiquement des fichiers root créés par Docker), un run **interactif** propose un `sudo rm -rf` en dernier recours, puis prune la métadonnée git obsolète et supprime la branche. Ne se déclenche **jamais** en mode `--output json` / `--yes`, où l'échec est remonté comme une erreur. Une garde de sécurité refuse d'escalader sur un chemin manifestement dangereux (racine du système de fichiers, `$HOME`, racine du dépôt ou un de ses ancêtres) (#45).

### Improvements

- **Recap `init` encadré & sorties de commandes harmonisées** — le récapitulatif final de `wtm init` est désormais cadré comme les autres sorties, et les micro-conventions d'affichage (icônes, lignes vides, décomptes) sont uniformisées entre commandes pour une lecture homogène (LUC-125).
- **Provisioning `.env` fidèle au template détecté** — la stratégie `example` copie le template **résolu par la détection** (`.env.dist`, `.env.sample`, …) ou, à défaut, sonde les candidats connus, au lieu de coder en dur `.env.example`. Un projet dont le template committé n'est pas `.env.example` reçoit maintenant bien son `.env` au lieu d'être silencieusement ignoré (LUC-89).

### Bug fixes

- **`wtm sync` : sortie de push et liste de conflits** — le push affiche le spinner « Pushing to origin… » au lieu de paraître figé ; plus de double ligne vide au-dessus du récap sur le chemin picker-confirmé ; les fichiers en conflit sont rendus en **liste verticale** plafonnée à 5 (`…+N more`), dans le footer `--keep-conflict` comme dans le récap d'abandon automatique (#44).

## v0.22.0 — Module `run` opt-in, bypass `--yes`/`--force` unifié & wizards harmonisés

### Breaking changes

- **`--output json` exige désormais `--yes` sur toutes les commandes qui mutent** (`create`, `clean`, `sync`, `prune`, `relocate`, `reparent`, `extract`, `checkout`) — auparavant seuls `clean`/`prune` le demandaient. En JSON, une sélection requise sans défaut (`extract` source/`--files`/`--to`, `sync` branches ou `--all`, `reparent` worktrees + `--to`) **erreure en nommant le flag** au lieu d'ouvrir un picker. `--force` reste un axe séparé : il ne lève que les refus de sécurité et n'implique jamais `--yes` (LUC-119).
- **Le module `run` devient opt-in** — `wtm init` ne détecte/configure plus les services (suppression de `--skip-services` et `--only services`) ; le module s'initialise via **`wtm run init`**. Toute commande `run` sur un module non initialisé sort en **code 16** avec un message pédagogique (LUC-101).

### New features

- **`wtm run init`** — met en place `run.toml` depuis la détection (docker-compose + scripts de package). Wizard interactif (pré-rempli en ré-exécution) ou auto-génération non-interactive ; les deux fusionnent additivement sans écraser les jobs existants. Seul point d'entrée fonctionnant avant l'existence du module (`run job/profile add` et `run import` en sont aussi exemptés) (LUC-101).
- **Badges de divergence `origin` dans les listes de worktrees** — `list`/`tree`/pickers/JSON affichent l'avance/retard vis-à-vis d'`origin` à côté du compteur vs base, étiquetés `base ↑N` et `origin ↑a ↓b` (lus depuis les refs remote-tracking en cache, sans fetch ; touche `r` pour rafraîchir). Le contrôle de fast-forward des sources périmées s'étend aux chemins explicites `create --from` / `extract`, avec un flag opt-in `--ff` pour fast-forwarder une source en simple retard en non-interactif (LUC-109).
- **`wtm extract [source]`** — la source n'est plus le worktree courant mais un **picker en première étape** (worktrees avec changements uniquement), donc on extrait depuis n'importe où. Argument `[source]` requis en non-interactif / `--output json` ; `-y/--yes` saute le récap final (LUC-118).
- **`wtm reparent` multi-sélection** — reparente plusieurs worktrees sur un même nouveau parent en un seul passage ; le récap liste le parent actuel de chacun (`branche (from parent)`) (LUC-108).

### Improvements

- **Taxonomie `--yes`/`--force` unifiée sur les 8 commandes mutantes** — deux axes orthogonaux (confirmation vs sécurité) alignés sur `gcloud --quiet` / `terraform -input=false` / clig.dev : `--yes` résout chaque décision par son flag ou un **défaut sûr** (sync no-push, extract on-conflict abort, clean/prune orphelins) sans jamais retomber sur un picker ; `clean` converge sur `prune` (LUC-119).
- **Wizards harmonisés — fil d'Ariane partout** — chaque confirmation vit désormais **dans** son wizard (breadcrumb + retour arrière), plus aucun prompt orphelin où `Esc` annulait tout le run. Forme unique `[saisies] → [sélections optionnelles] → récap`, raisons de skip visibles, `No, cancel` constant, chargements async par étape (`OnEnter`). `clean` est réécrit en un seul wizard (picker → suppression → reparent), et `relocate` unifie l'édition de `base_path` dans le même wizard (LUC-115, LUC-116, LUC-108).
- **`wtm sync` : la prévisualisation du plan devient l'étape finale du wizard** — `Esc` revient à l'étape précédente au lieu d'abandonner le run ; recall compact (compteur) sur les étapes intermédiaires, liste détaillée sur la confirmation (LUC-110).
- **`wtm agents install` met à jour une skill déjà installée** au lieu de la laisser intacte — résultat par destination : `created` / `updated` / `unchanged` (aucune écriture) / `skipped` (erreur réelle), identique en interactif et JSON (#34).
- **Guidage post-`init`** — les worktrees pré-existants sont signalés après un `wtm init`, avec un pointeur vers `wtm relocate` pour les adopter/réaligner (LUC-108).

### Bug fixes

- **`wtm prune` détecte mergé/fermé depuis l'état PR GitHub, pas les commits locaux** — l'ancienne heuristique (`git rev-list base..branch == 0`) taguait à tort tout worktree jamais divergé et manquait les squash/rebase-merges. Désormais `--merged` = PR mergée, `--closed` = PR fermée sans merge, `--gone` = distante supprimée (seul filtre hors-ligne) ; une branche sans PR n'est jamais taguée. Alerte sur stderr si `gh` manque ; valeurs JSON `pr_merged`/`pr_closed`/`gone` (LUC-111).
- **`clean`/`relocate` en environnement non-TTY** — un run piped/CI en format humain sans `--yes` erreure proprement au lieu de lancer un wizard sur un stdin non-interactif ; `relocate` refuse de muter sans `--yes` quand il ne peut pas confirmer.
- **`clean --reparent-children` ignoré sur le chemin wizard interactif** — le flag est désormais respecté (LUC-119).

## v0.21.0 — `wtm prune`, `sync --keep-conflict` & parité JSON

### New features

- **`wtm prune [filters]`** — supprime en un passage les worktrees dont le travail est terminé, en reparentant les enfants survivants sur leur grand-parent (comme `clean --reparent-children`). Sans filtre, prune considère tout worktree fini ; les filtres restreignent par catégorie : `--merged` (aucun commit d'avance sur la base — ne capture pas les squash-merges), `--closed` (PR mergée/fermée, nécessite `gh`), `--gone` (branche distante supprimée, lance `git fetch --prune` d'abord sauf `--no-fetch`). Sur TTY : les matches sont présentés pour revue (les worktrees **unsafe** décochés), puis une confirmation de prune, puis — comme `clean` — une confirmation dédiée pour reparenter les enfants. Le worktree principal et la branche de base sont toujours protégés ; le worktree courant est supprimé et le shell redirigé vers le dépôt de base. **Comme `clean`, un worktree dirty, avec commits non-pushés, ou avec une PR ouverte est unsafe et nécessite `--force`** — en mode `--yes`/`--output json` ces worktrees sont reportés sous `skipped` (raison `dirty`/`unpushed`/`open_pr`) plutôt que supprimés, pour ne jamais perdre de travail commité silencieusement. `--dry-run` prévisualise sans rien changer ; `--yes` saute les prompts (requis avec `--output json`) ; non-interactivement les enfants restent orphelins sauf `--reparent-children`.
- **`wtm sync --keep-conflict`** — laisse un rebase en conflit **en cours dans son worktree** pour résolution manuelle, au lieu de l'abandonner. Les fichiers en conflit sont capturés avant tout abandon, la branche d'un worktree en plein rebase est correctement récupérée, et un second `sync` détecte le rebase en cours (nouveau statut `rebase_in_progress`) pour bloquer ses descendants sans re-tenter. La sortie JSON gagne `conflict_files`, `kept_in_progress` et `path` (#32).

### Improvements

- **Parité `--output json` sur l'ensemble des commandes** — les commandes qui ne l'exposaient pas encore émettent désormais un payload JSON stable et agent-friendly (slices normalisées en `[]` plutôt que `null`, jamais encadré par le framing humain), pour piloter wtm depuis un agent/script de façon homogène.
- **`--help` groupé, docs générées & README allégé** — l'aide racine range les commandes en sections (`Worktrees:`, `Navigate:`, `Stacked branches:`, `Dev jobs:`, `GitHub:`, `Setup:`). La référence complète des commandes sous `docs/` est désormais **générée** depuis l'arbre Cobra par `tools/gendocs` (`make docs`), le README redevient un guide concis (concepts + tableau d'aperçu groupé pointant vers `docs/`), et la skill agent `using-wtm` est resserrée tout en documentant `prune` et `sync --keep-conflict`.

## v0.20.0 — Pickers de branches : divergence, branches distantes & filtrage

### New features

- **Brancher depuis une branche distante** — `wtm create --from origin/x` crée un worktree à partir d'une branche remote-tracking que vous n'avez pas checkout localement, et `wtm reparent --to origin/x` reparente sur une branche d'intégration distante. Les pickers de branches (create, checkout, reparent, relocate, init) listent désormais les locales **et** les distantes d'`origin` (groupées après un séparateur, taguées `remote`), avec les distantes dont le nom existe déjà en local masquées au profit de la locale. La validation `reparent` accepte une branche locale **ou** un ref `origin/x` (LUC-98).
- **Badges de divergence dans les pickers de branches** — une branche locale qui a dérivé de sa contrepartie `origin/` est taguée avec son avance/retard (`↓5`, `↑2`, `↑2 ↓5`), pour ne pas brancher sur une base périmée sans le savoir. À l'ouverture, wtm fetch `origin` en arrière-plan (callout `Fetching branches…` non-bloquant) et les badges se rafraîchissent ; la touche **`r`** relance un fetch à la demande. Hors-ligne, le picker reste utilisable avec les derniers compteurs connus (LUC-98).
- **Fast-forward d'une source périmée avant création** — si la branche source choisie est strictement en retard sur `origin/` (`↓N`), wtm propose de la fast-forward vers origin avant de créer le worktree (la source reste une branche locale, préservant stratégie env, métadonnée parent et cohérence `wtm sync`). Le fast-forward est sauté si le worktree de cette branche a des changements non commités — wtm demande alors s'il faut créer depuis la branche locale telle quelle (défaut non). Une branche **divergée** (`↑N ↓M`) ne peut pas être fast-forwardée : wtm affiche un heads-up explicite et, sur confirmation, crée depuis la locale en préservant les commits locaux (LUC-98).
- **Filtre inline sur les listes multi-sélection** — dans le picker interactif de `wtm sync` (et `extract`, wizard `init`), `/` entre en mode filtre : la liste se réduit en live aux worktrees dont le nom contient le terme (sous-chaîne, insensible à la casse). `a` coche/décoche tous les worktrees **filtrés**, les cases cochées sont conservées d'un filtre à l'autre, `échap` efface le filtre s'il est actif sinon annule. Le filtrage vit dans le composant `MultiSelect` partagé (LUC-95).

### Improvements

- **Harmonisation & redesign des listes de worktree (LUC-97)** — espacement de ligne désormais uniforme, avec ou sans badge. Les badges deviennent du **texte coloré compact** aligné en colonne (au lieu de chips volumineux) ; la ligne sélectionnée porte un fond teinté + un marqueur de bord gauche `▌▸` ; le statut est une **pastille droite alignée** avec glyphe (`✓ clean`, `⚠ dirty`) qui scanne d'un coup d'œil. Les exemples `wtm list` / `wtm tree` du README reflètent les nouveaux glyphes.
- **Transparence de la stratégie env `parent`** — avant de créer un worktree (create, extract, checkout), si la stratégie `parent` allait silencieusement copier le `.env` depuis le worktree **principal** parce que la branche source n'a pas de worktree local, wtm le signale et demande confirmation au lieu de copier depuis un emplacement inattendu (LUC-98).
- **Nettoyage de conformité interne** — extraction des helpers de filtre partagés (`SelectList`/`MultiSelect`), factory `branchrefresh.Handler` unique pour les 5 pickers de branches, fonctions pures déplacées dans `rules/` (`BranchCandidateExists`, `IsRemoteBranch`), et centralisation des constantes (`RemoteBranchPrefix`, glyphes de statut) — aucun changement de comportement.

## v0.19.0 — Workflow de branches empilées : visualiser, reparenter, sync ciblé

### Breaking changes

- **`wtm sync` ne cascade plus tout par défaut en mode non-interactif** — `sync` rebase désormais un **sous-ensemble** choisi de worktrees : passez des noms de branches, utilisez le **picker multi-select** (sans argument, en TTY), ou `--all` pour toute la cascade. La base est toujours rafraîchie en premier (sauf worktree principal dirty) ; sélectionner la base se contente d'un fetch + fast-forward. **En mode `--output json`, `sync` ne sélectionne plus toutes les branches par défaut** — des noms de branches ou `--all` sont maintenant requis. Un argument de branche inconnu sort en code `11` (LUC-86).

### New features

- **`wtm tree`** — affiche la **forêt** des worktrees (liens `parent → enfant` issus du `source_branch`), pensée pour les workflows de branches empilées. Arbre ASCII coloré avec connecteurs (`├─ └─`), annotations par nœud : `↑N` (commits d'avance), `● dirty`, et surtout **`⚠ needs sync`** — le signal d'orchestration clé, levé quand le parent a avancé et que l'enfant doit être rebasé. Un parent sans worktree (ex. `dev`) apparaît en **racine virtuelle** grisée `(no worktree)` ; un cycle de `source_branch` est rendu sans planter et annoté `⚠ cycle`. Flags : `--with-prs` (numéros de PR + marquage mergée/fermée, fetch réseau opt-in comme `wtm list`) et `--output text|json|mermaid` — `json` pour les agents/scripts, `mermaid` pour un `flowchart TD` collable en PR/Notion (LUC-82).
- **`wtm reparent <branch> --to <parent>`** — change le parent enregistré (`source_branch`) d'un worktree après sa création (métadonnée seulement ; le rebase se fait au prochain `wtm sync`). Utile pour les branches empilées une fois qu'une branche intermédiaire est mergée. Wizard multi-étapes (breadcrumb + back-nav) affichant le parent actuel ; pilotable par arguments directs et `--output json`. La validation cycle / self-parent réutilise le check topologique de `sync` (LUC-88).
- **`wtm clean` reparente les orphelins** — `clean` détecte désormais les enfants orphelins et propose de les reparenter sur le grand-parent : récap interactif + confirmation (`Esc` annule tout le `clean`), ou l'opt-in `--reparent-children` en mode non-interactif (LUC-88).

### Bug fixes

- **Breadcrumb du wizard `relocate` toujours visible** — sur des listes longues ou après plusieurs étapes, le breadcrumb (qui nomme le worktree concerné) ne scrolle plus hors écran. `View()` est découpé en fragments mesurables (head / list / tail) pour dimensionner la liste sur le chrome réel, et les résumés d'étapes terminées sont bornés (les plus anciens se replient en une ligne discrète `… (N earlier steps)`). Le correctif vit dans le composant wizard partagé, donc `reparent` et les autres wizards en profitent aussi (LUC-85).
- **Sortie de task rejouée au streamer (flaky CI)** — une task rapide pouvait écrire son premier chunk de sortie dans l'historique avant que la goroutine de streaming ne s'abonne, laissant la sortie streamée vide. L'historique est désormais rejoué avant de boucler sur le canal live, sous un seul verrou — plus de gap ni de chevauchement.

### Improvements

- **Espacement vertical harmonisé derrière `output.Frame` (LUC-87)** — chaque commande encadre sa sortie humaine **exactement une fois** (`output.Frame` ou `FrameStart`/`FrameEnd`) ; les helpers et formatters de tables renvoient des corps **bruts** sans lignes vides externes. JSON (`--output json`) et sortie machine (`resolve`, `shell-init`) restent strictement à ras. Le routage se fait sur `rules.IsHumanFormat`.
- **Spinners unifiés en boîte `RunLoading`** — `shared.StartSpinner` (spinner braille brut, sans padding, écrivant du garbage `\r` dans les pipes) est remplacé par `components.RunLoading` : un loader bordé avec StatusBox + MiniDot identique au chargement du wizard. ~19 sites migrés (`clean`, `create`, `list`, `sync`, `relocate`, `checkout`, `init`, `resolve`, `run/*`) ; en non-TTY / JSON, le travail s'exécute directement sans boîte.

## v0.18.0 — `wtm checkout` au top-level (suppression du groupe `pr`)

### Breaking changes

- **Suppression du groupe `pr`** — `wtm pr checkout` devient `wtm checkout` (promu sous `Core Commands:`). `wtm pr list` est retiré : la liste des PRs vit déjà dans les pickers de worktrees (`wtm list --with-prs`) et dans le nouveau wizard de `checkout`. Mettez à jour vos scripts et alias. La sortie `--output json` de `checkout` reste `{number, branch, path}` (inchangée vs `pr checkout`).

### New features

- **`wtm checkout [number]`** — crée un worktree depuis une pull request. Sans argument : wizard interactif multi-étapes **PR → branche parente → stratégie env** qui s'affiche instantanément et streame les PRs ouvertes en arrière-plan (PRs déjà checkout ou issues d'un fork désactivées). Avec un numéro : checkout direct. Flags `--review` / `--mine` (filtre des PRs), `--from <branche>` (parent de sync, défaut = base de la PR), `--env-from example|main|parent` (override de la stratégie env). Chaque étape du wizard est sautée si elle est déjà résolue par un flag.

### Improvements

- **Nettoyage de la tuyauterie PR inutilisée** — retrait de l'affichage PR dédié (`output/pr.go`), des champs jamais consommés (CI status, reviews) et fusion des field-sets `gh pr` en une seule constante. Le fetch GitHub ne récupère plus que ce que `checkout` utilise réellement.

## v0.17.0 — commandes worktree au top-level (suppression du groupe `wt`)

### Breaking changes

- **Suppression du groupe `wt`** — les 8 commandes worktree sont promues au top-level sous `Core Commands:`. `wtm wt list` devient `wtm list`, et de même pour `create`, `clean`, `sync`, `relocate`, `go`, `switch`, `extract`. Aucun alias `wt` n'est conservé : mettez à jour vos scripts et alias. Régénérez l'intégration shell (`eval "$(wtm shell-init)"`) — le wrapper intercepte désormais `wtm go`/`wtm switch` (et non plus `wtm wt go`/`wtm wt switch`). Les groupes `run` et `pr` sont inchangés.

### Improvements

- **PTY drainé jusqu'à l'EOF naturel avant fermeture** dans les jobs détachés — évite une troncature de sortie en fin de process (LUC-84).

## v0.16.0 — `wt relocate` : rassembler les worktrees + grand nettoyage de surface inutilisée

### New features

- **`wtm wt relocate`** — réorganise les worktrees pour qu'ils vivent tous sous `base_path`. Déplace les worktrees éparpillés (créés ailleurs) vers l'emplacement configuré et **adopte** les worktrees externes existants dans la gestion wtm. Plan/preview des déplacements avant exécution, wizard interactif, et pilotable sans TTY via `--to`, `--force`, `--output json` (statuts par worktree : `moved`, `moved_adopted`, `adopted`, `skipped`, …).

### Breaking changes

- **`wtm pr create` retiré** — la création de PR revient à `gh pr create`, qui gère déjà templates, push de branche et détection de la base. C'était un simple wrapper qui ne touchait jamais au modèle worktree. `pr list` et `pr checkout` (les vrais ponts PR↔worktree) sont inchangés.
- **Décodage strict de la config** — les sections/clés désormais supprimées font échouer le chargement : `[agents]`, `[integrations]` et `[github]` dans le `config.toml` projet, ainsi que `agent` dans le config global (`~/.config/wtm/config.toml`). Retirer ces lignes ou relancer `wtm init`.

### Improvements

- **Nettoyage de surface inutilisée** — retrait du scaffolding `project_manager` (intégration VS Code/Cursor jamais livrée), de la config « agent par défaut » jamais consommée (clé `agent`, flag `--agent`, étape du wizard d'`init`, schéma), de la machinerie morte `context.md` / `Detail()` (vue détail jamais branchée), et de la config `[github] auto_draft` (orpheline après le retrait de `pr create`).

## v0.15.0 — `wt sync` : rebase en cascade de toute la chaîne de worktrees

### New features

- **`wtm wt sync`** — remet à jour **toute la chaîne de worktrees** en une commande, dans l'ordre topologique (parents avant enfants), en s'appuyant sur le parent déjà tracké (`source_branch`). Pour la base puis chaque branche (`main → feat → dev1/dev2`) : fetch + fast-forward de la branche depuis son propre `origin/<branche>` (récupère une PR mergée dans le parent), puis rebase `--onto` sur le parent rafraîchi — seuls les commits propres de l'enfant sont rejoués. La cascade est **100 % locale** (les worktrees partagent `.git`).
- **Push groupé découplé** — après une cascade réussie, un récap détaillé (parent, commit cible, avant→après, commits rejoués) s'affiche **avant** de proposer le push, puis un seul prompt pousse les branches rebasées en `--force-with-lease`. Flags : `--dry-run` (preview 100 % offline), `--base <branche>`, `--push` (seul moyen de push en mode `--output json`), `--no-push`, `-y`/`--yes`.
- **Statuts par branche (`--output json`)** — `synced`, `up_to_date`, `skipped_dirty` (+ descendants `skipped_ancestor`), `diverged` (local **et** `origin/<branche>` ont divergé → laissé pour réconciliation manuelle), `conflict` (rebase auto-aborté, working tree propre), `error`, `unknown_parent`. Sortie non-zéro si au moins un `conflict`/`error`.

### Improvements

- **Erreurs git remontées au lieu d'être avalées** dans la cascade : un `rev-list`/`git status` en échec produit désormais un statut `error` bloquant plutôt qu'un faux `up_to_date`, et le push n'est tenté que si `origin/<branche>` est réellement absent (évite un force-push sur erreur transitoire).

## v0.14.0 — Affichage instantané des worktrees + streaming des PRs

### New features

- **Liste des worktrees instantanée** — `wt list`, `wt go` et `wt switch` affichent les worktrees immédiatement et navigables, sans attendre GitHub. Les PRs sont fetchées en arrière-plan et remplissent les badges (`PR #x`) dès qu'elles arrivent ; une bannière de statut montre la progression (spinner animé) puis, si `gh` est indisponible, le hint d'installation/connexion.
- **`wt list --with-prs`** — en non-interactif (pipe/JSON), les PRs ne sont plus fetchées par défaut (liste instantanée) ; `--with-prs` les inclut explicitement. Comportement **identique** entre texte et JSON.

### Improvements

- **Fetch GitHub allégé** — les pickers de worktrees ne récupèrent plus le champ `body` (lourd) des PRs, inutile pour les badges ; `pr list` garde le détail complet.
- Action « Open PR » disponible immédiatement, l'URL étant résolue à la volée pendant le chargement.

## v0.13.0 — Refonte de `wtm init` : gates de sections, re-init `--only`, éditeur de hooks `on_create`

### New features

- **Gates de sections à l'`init`** — chaque section optionnelle (`env`, `hooks`, `services`) démarre par une étape **Configurer / Passer** avec une intro explicative. Passer une section l'écrit en config **commentée** (template prêt à activer) plutôt qu'en valeurs vides. En non-interactif : `--skip-env`, `--skip-hooks`, `--skip-services`.
- **`wtm init --only <section>`** — re-initialise proprement une ou plusieurs sections (`worktrees`, `env`, `hooks`, `services`) sans toucher aux autres. Accepte le CSV ou la répétition (`--only env,hooks`). Le wizard est **pré-rempli depuis la config existante** (base branch, stratégie env, fichiers copiés, hooks, et jobs détectés déjà configurés). `config.toml` préserve chaque section non ciblée ; `run.toml` régénère les jobs en **conservant les profiles**.
- **Éditeur de liste de hooks `on_create`** — l'étape hooks devient un vrai éditeur : ajout / édition / suppression / réordonnancement (`shift+↑/↓`) des entrées, chacune avec `cmd`, `cwd` optionnel et toggle `continue_on_error`. Remplace l'ancien duo « install command + packages monorepo ». Pré-rempli depuis la détection à l'`init`, depuis le `on_create` courant en re-init.

### Improvements

- Quand une config existe déjà, `wtm init` guide vers `--only` pour re-initialiser une section ciblée au lieu de tout réécrire.

## v0.12.0 — `wt extract` : déplacer les changements non-commités entre worktrees

### New features

- **`wtm wt extract`** — déplace un sous-ensemble des changements non-commités du worktree courant vers un autre worktree (nouveau ou existant), pour découper une PR trop grosse ou isoler du travail sans rapport. Wizard interactif **Files → Target → Mode** (Move/Copy), chaque étape étant sautée si résolue par un flag. Pilotable sans TTY via `--files`, `--to`, `--from`, `--keep`, `--on-conflict`, `--output json`.
- **Move vs copy** — les fichiers sont déplacés par défaut (retirés de la source une fois posés) ; `--keep` les copie.
- **Règle de sécurité transactionnelle** — la source n'est nettoyée que si toute l'extraction s'applique proprement. Au moindre conflit, la source est laissée **totalement intacte et récupérable** — jamais d'état à moitié déplacé.
- **Mode résolution de conflits** — `--on-conflict abort` (défaut) ne change rien et sort en code `15` ; `--on-conflict resolve` applique les changements dans la cible avec des marqueurs de conflit git (via `git merge-file`) pour résoudre comme un rebase, en gardant la source intacte.

## v0.11.0 — CLI pilotable par agents LLM + logs des services détachés en live

### New features

- **`wtm pr create --yes`** — mode non-interactif : push automatiquement une branche non poussée et saute les prompts (implicite avec `--output json`). Corrige un cas où `pr create --output json` sur une branche non poussée s'arrêtait silencieusement.
- **`wtm init` pilotable par flags** — `--non-interactive`, `--agent`, `--shell`, `--base-path`, `--base-branch`, `--env-strategy`, `--install-command`. Résolution `flags > détection > défauts`, échoue proprement si la base branch est introuvable en non-interactif. Un agent peut bootstrapper un projet sans wizard.
- **`wtm wt create --if-not-exists`** — succès idempotent (`already_exists: true`) si le worktree existe déjà, au lieu d'échouer.
- **Codes de sortie granulaires** — `10` worktree existe, `11` branch introuvable, `12` config introuvable, `13` PR existe déjà, `14` job non déclaré. Agents et scripts peuvent brancher sur la cause d'échec.
- **Logs des launchers détachés en live** — un service détaché (`docker compose up -d`…) streame sa sortie de démarrage en direct pendant `wtm run up` (réseau/conteneurs créés et démarrés), comme une task — au lieu d'un simple spinner puis « started ».

### Breaking changes

- **`config not found` sort en code 12 (au lieu de 0)** — toute commande hors d'un repo initialisé échoue désormais avec un code non nul ; un script comptant sur un exit 0 silencieux doit être ajusté.
- **`wtm pr create` sort en code 13 si une PR existe déjà** (au lieu de 0) ; en mode JSON la PR existante est imprimée sur stdout.

### Improvements

- **`wtm wt clean` idempotent** — supprimer un worktree déjà absent réussit en no-op (`already_absent: true`).
- **`run stop` / `run down` rejouables** — stopper un job déjà arrêté est un no-op ; un job non déclaré renvoie le code 14.
- **Forks documentés** — `wtm pr checkout` d'une PR de fork reste refusé (par design), message clair vers `gh pr checkout` ; README + skill `using-wtm` à jour, référence morte supprimée.

## v0.10.0 — Run unifié (start + tail, tasks streamées) + ordre des jobs de profil

### New features

- **`wtm run up` / `run start` : lancement unifié avec tail intégré** — les jobs démarrent et leur sortie est streamée dans la foulée (services en arrière-plan, tasks one-shot en direct) ; plus besoin d'enchaîner `start` puis `logs` à la main.
- **Ordonnancement des jobs dans le wizard de profil** — nouveau step **Order** après la sélection des jobs (`run profile add` / `edit`) : on réordonne l'ordre d'exécution avec `shift+↑/↓` (ou `J`/`K`). L'ordre choisi est persisté dans `run.toml` et respecté à l'exécution (ex. une task `build` avant le `server`).

### Improvements

- **Gestion homogène des échecs service/task** — un échec de task abort proprement le reste du profil, les logs d'échec sont remontés, et le rapport d'abort est nettoyé quand une task échoue.
- **Spinners de chargement uniformes** — spinners cohérents sur l'ensemble des commandes (`run`, `wt`, `pr`, `resolve`…) et ellipses normalisées (`…`).
- **Picker `wt go` / `wt switch` enrichi** — spinner de chargement, fetch parallèle des worktrees et callout GitHub CLI quand `gh` est absent.

## v0.9.0 — `wt list` interactif, run export/import + CRUD jobs/profiles

### New features

- **`wtm wt list` enrichie** — spinner de chargement pendant la récupération des worktrees, PRs et services ; bandeau d'avertissement quand la GitHub CLI est absente (non installée / non authentifiée) ; nouvelle action **Open PR** pour ouvrir directement la PR liée à une branche.
- **Retour automatique au repo de base après suppression** — quand on supprime le worktree dans lequel on se trouve (via `wt list` → Clean ou `wtm wt clean`), le shell est ramené dans le repo de base au lieu de rester bloqué dans un dossier fantôme. S'appuie sur le pont `WTM_GO_FILE` existant.
- **`wtm init` détecte les scripts `package.json`** — après les docker-compose files, une étape MultiSelect propose les scripts du `package.json` racine et, si `pnpm-workspace.yaml` est présent, ceux de chaque workspace. Les scripts `dev`/`start`/`serve`/`watch` (ou leurs formes préfixées `dev:*` / `*:dev`) sont pré-sélectionnés comme `kind="service"` ; les autres (`build`, `test`, `lint`…) comme `kind="task"`.
- **`wtm run export [--profile <name>]`** — émet `.wtm/run.toml` comme JSON sur stdout. Compatible avec `--profile` pour exporter un seul profil et ses jobs.
- **`wtm run import [file|-] [--replace --force]`** — ingère un payload JSON et le fusionne dans `.wtm/run.toml`. Par défaut, les nouveaux jobs/profils sont appendés, les doublons sont ignorés avec un avertissement. `--replace --force` écrase le fichier entièrement.
- **`wtm run job add|rm|edit|list` et `wtm run profile add|rm|edit|list`** — CRUD CLI sur les jobs et profiles déclarés dans `run.toml` (qui vit désormais dans `<git-common-dir>/wtm/`). `add` : mode wizard si lancé sans flags obligatoires, mode flag pour pilotage LLM (`--cmd`, `--kind`, `--stop`, `--cwd` côté job ; `--jobs`, `--default` côté profile). `rm` : sans argument, picker interactif sur les jobs/profiles existants ; `--force` côté `job rm` retire aussi les références d'un job dans les profiles. `edit` : sans argument, picker puis wizard pré-rempli ; avec `<name>`, va directement au wizard. Le rename est autorisé, `ValidateRun` détecte les références orphelines. `list` : picker interactif → menu Edit/Remove en TTY, simple émission de la slice avec `--output json` ou en pipe.
- **Default profile auto-override** — quand on définit un nouveau profile comme `--default` (ou via le wizard), l'ancien default est automatiquement désactivé au lieu d'erreur "two defaults". Le wizard prévient l'utilisateur (description du step Confirm) avant de basculer.
- **TextInput re-valide à chaque touche** — les erreurs de validation (ex. "profile already exists") restent visibles tant que la valeur entrée est invalide, et disparaissent dès qu'elle redeviendrait valide. Avant, l'erreur s'effaçait à la première touche, illisible.

### Breaking changes

- **`wtm run list --output json` : champs JSON en minuscules** — les clés passent de PascalCase (`Jobs`, `Name`, `Kind`) à lowercase (`job`, `name`, `kind`), alignées sur `run.schema.json`. Impacte tout script ou outil qui parsait la sortie JSON de `run list`.

### Improvements

- Suppression de la branche dédiée `wt clean` du wrapper shell et de son heuristique fragile `$PWD` ; le mécanisme générique `WTM_GO_FILE` gère désormais toutes les voies d'entrée.

### Migration

- **Re-`eval` de `wtm shell-init` requis** — le template du wrapper shell a changé. Les utilisateurs existants doivent re-`eval "$(wtm shell-init)"` (re-source `.zshrc`/`.bashrc`/config fish) pour bénéficier du retour automatique au repo de base.

## v0.8.0 — Strict TOML decoding + JSON Schema autocomplete

### New features

- **JSON Schema bundled pour les 3 fichiers de config** — `.wtm/run.toml`, `.wtm/config.toml`, `~/.config/wtm/config.toml`. Fichiers embarqués dans le binaire et écrits dans `.wtm/schemas/` (ou `~/.config/wtm/schemas/`) au moment du `wtm init`. Chaque TOML généré est préfixé par `#:schema ./schemas/...json`.
- **`wtm schema dump`** — extrait les schémas embarqués vers le disque pour les régénérer après upgrade. `--global` cible le schéma global.
- **Autocomplete + validation IDE via Taplo** — l'extension "Even Better TOML" (VS Code / Cursor / JetBrains) lit la directive `#:schema` et fournit autocomplete sur les champs et enums (kind, env.strategy, agent, shell), hover docs, erreurs en live.

### Bug fixes

- **Decode TOML strict** — les clés inconnues (typos comme `[[profiles]]` au lieu de `[[profile]]`) sont maintenant rejetées avec un message clair `unknown keys in /path: profiles` au lieu d'être silencieusement ignorées.

## v0.7.2 — Reset terminal modes après détach de `wtm run logs`

### Bug fixes

- **Le terminal n'est plus corrompu après détach d'un service TUI** — Sortir d'un `wtm run logs <service>` sur un service à TUI interactif (turbo, vite, vim, dev servers à HMR souris) laissait le terminal coincé en mouse-tracking + alt-screen + curseur masqué : taper produisait du garbage, et chaque mouvement de souris écho `<button>;<col>;<row>M` au prompt. La séquence "soft detach" (désactivation des cinq modes mouse, sortie d'alt-screen, ré-affichage du curseur, reset SGR) est désormais émise via `defer` à la sortie de `attachSingleJob` et `multiplexAllJobs`, donc sur Ctrl+C, EOF, ou erreur de connexion. Re-réintègre le fix `4ac6219` perdu au merge du gros refactor `9e8ac08` (PR #7).

## v0.7.1 — Stop tue toute l'arborescence de processus

### Bug fixes

- **`wtm run stop` / `run down` n'orphelinent plus les processus enfants** — `stopWithSignal` envoyait SIGTERM uniquement au PID direct (npm/pnpm), laissant les enfants (node, vite, turbo, nest, tsc, esbuild, …) tourner détachés en arrière-plan. Le signal cible désormais l'intégralité du process group via `kill -PGID`, attend la sortie effective avant de marquer le job *stopped*, et escalade vers SIGKILL après 5 secondes si le groupe ignore SIGTERM. Validé end-to-end sur un projet pnpm + turbo (web + api + tsc watch + esbuild) — toute l'arborescence est nettoyée.

## v0.7.0 — Run config refactor (services + tasks unifiés en jobs)

### Breaking changes

- **`.wtm/services.toml` → `.wtm/run.toml`** — le fichier de config est renommé. Sections refactorées : plus de `[[services]]` / `[[profiles]]`, on a maintenant `[[job]]` (avec `kind = "service"` ou `"task"`) et `[[profile]]` (avec `jobs = [...]` au lieu de `services = [...]`). Aucune migration auto — les anciens fichiers ne sont plus lus.
- **CLI `wtm svc *` → `wtm run *`** — toutes les sous-commandes basculent (`run up`, `run down`, `run ps`, `run logs`, `run start`, `run stop`, `run list`).
- **Helpers exportés / shell wrapper** — le wrapper `wt switch` appelle maintenant `wtm run up` (regénéré via `wtm shell init`).

### New features

- **Type `task` pour les scripts one-shot** — `kind = "task"` modélise les commandes qui doivent terminer avec succès avant que le profil continue (migrations, seeds, formatters). Le daemon stream l'output live au CLI, le job disparaît de `run ps` après exit, et un échec abort le reste du profil.
- **Streaming NDJSON pour les tasks** — le protocole daemon/CLI gère plusieurs `Response` par requête (`StatusOutput` pour les chunks, `StatusDone` pour la fin). Le CLI forward le contenu sur stdout pour suivre l'exécution en direct.
- **Colonne `KIND` dans `run ps`** — distingue services et tasks dans la table et le picker.
- **Validation stricte du format** — `kind` requis, `task` ne peut pas avoir de `stop`, profils référencent uniquement des jobs déclarés. Les erreurs sont remontées avant l'exécution.

### Improvements

- **Init docker-compose génère du `[[job]]` directement** — la détection à `wtm init` écrit dans `.wtm/run.toml` avec `kind = "service"` et `stop` configuré (donc détaché).
- **Skill `using-wtm` mis à jour** — la doc agent reflète le nouveau vocabulaire (jobs, kinds, run.toml, `wtm run *`).

## v0.6.2 — Style polish (suite)

### Bug fixes

- **Couleurs du picker `wt go` / `wt switch` (vrai fix)** — La release v0.6.1 utilisait `SetDefaultRenderer`, qui remplace le pointeur global mais laisse les styles existants (déclarés au package init) référencer l'ancien renderer. Remplacé par `SetColorProfile` qui mute le renderer partagé en place. Le picker retrouve maintenant effectivement sa barre bleue et ses badges colorés quand invoqué via le shell wrapper.
- **Padding entre le prompt de confirmation et le résultat du stop** — `svc up` ajoute une ligne vide entre "Stop other services before starting?" et le premier "✓ Stopped services in X", pour qu'on distingue clairement la réponse au prompt du résultat.

## v0.6.1 — Style polish

### Improvements

- **Padding cohérent autour des warnings** — `svc up` (services sur un autre worktree), `pr create` (PR déjà ouverte) et `svc ps` (aucun service running) respectent maintenant un padding top/bottom uniforme.
- **Padding `wtm init`** — Ajout d'un style dédié `Intro` avec accent primary pour le message "No .wtm/config.toml found", nettoyage du padding autour des messages de succès.
- **Picker `wt go` / `wt switch` stylisé** — Quand le picker est invoqué depuis le shell wrapper (qui capture stdout via `$()`), lipgloss détectait stdout non-TTY et désactivait les couleurs. La détection bascule maintenant sur stderr, le picker conserve son highlight bleu et ses badges.
- **Padding bottom après refus d'ouvrir un PR existant** — `pr create` ajoute une ligne vide après le prompt "Open in browser?" quel que soit le choix de l'utilisateur.

## v0.6.0 — Ready for LLM agents

### New features

- **`--output json` sur les commandes data** — `wt list`, `wt create`, `wt clean` (avec `--force`), `pr list`, `pr create`, `pr checkout`, `svc list`, `svc ps`, `svc up`, `svc down`, `svc start`, `svc stop` retournent maintenant du JSON machine-readable sur stdout. Le texte humain reste sur stderr. Permet aux agents (Claude Code, Cursor, scripts) de piloter wtm sans TUI.
- **`wtm svc list`** — Liste les services et profils déclarés dans `.wtm/services.toml`. En TTY, picker interactif avec actions `up`/`down` sur un profil ou `start`/`stop`/`logs` sur un service, pour découvrir le cycle de vie svc sans mémoriser chaque commande.
- **`wtm svc ps`** — Liste les services gérés en ce moment par le daemon (name, status, pid, worktree). Picker avec actions `stop`/`logs`/`restart` + une entrée "Stop all running services" qui dispatche `svc down --all`.
- **`wtm agents install`** — Détecte les destinations skill existantes (`.claude/` / `.cursor/` projet ou global) et installe un skill compact `using-wtm` que Claude Code / Cursor consultent automatiquement quand l'utilisateur parle worktrees, services ou PRs.
- **Détection `docker-compose` dans `wtm init`** — Si des fichiers `docker-compose*.yml/yaml` sont trouvés, étape wizard MultiSelect pour scaffolder des services correspondants dans `.wtm/services.toml` avec `up -d` / `down --remove-orphans` et détection automatique de la commande (`docker compose` v2 ou `docker-compose` v1).
- **`wtm svc down --all`** — Stoppe tous les services de tous les worktrees (le comportement par défaut de `svc down` reste scoped au worktree courant).

### Bug fixes

- **Échecs silencieux de `docker compose up -d`** (LUC-56) — Les services launcher-style (ceux avec un `Stop`) affichaient `✓ started` même quand docker échouait (port conflit, image manquante, compose invalide). Le manager attend maintenant la sortie du launcher et remonte l'erreur avec la sortie capturée, nettoyée des ANSI et des redraw `\r` de compose.
- **`svc down` traversant les worktrees** — `svc down` (et indirectement `wt clean`, `svc up --exclusive`) pouvait stopper des services d'autres worktrees parce que `handleStopAll` ignorait `Request.WorkDir`. Ajout de `StopAllInWorkDir` et respect du workdir côté daemon.

### Improvements

- **Picker `wt switch` aligné sur `wt list`** — Même styling (breadcrumb, badges parent / PR / services / dirty) quand l'utilisateur appelle `wt switch` sans argument.
- **Spinners sur les opérations svc** — `svc up`, `svc down`, `svc start` affichent maintenant un spinner pendant l'aller-retour daemon (utile quand `docker pull` prend plusieurs secondes).
- **`output.Error` multi-ligne** — Les erreurs avec sortie capturée (typiquement docker compose) sont formatées en bloc indenté au lieu d'une ligne illisible.
- **`wtm svc down` scoping par défaut** — Sans `--all`, ne touche que le worktree courant. Help text mis à jour.

## v0.5.1 — Fix TUI et navigation shell

### Corrections

- **TUI invisible dans `wt go` / `wt switch`** — Le picker de worktree ne s'affichait pas quand appelé via le shell wrapper. Le TUI Bubbletea rendait sur stdout, qui était capturé par la substitution `$()`. Le rendu passe maintenant sur stderr.
- **"Go to worktree" depuis `pr list` et `wt list`** — L'action affichait "requires shell integration" au lieu de naviguer. Les commandes résolvaient le path via un sous-processus `wtm wt go` qui tombait sur le fallback. Remplacé par une résolution directe et écriture dans `WTM_GO_FILE`.
- **Shell wrapper étendu** — La clause `else` du wrapper (bash/zsh/fish) passe maintenant `WTM_GO_FILE` à toutes les commandes, permettant à n'importe quelle sous-commande de déclencher un `cd`.

## v0.5.0 — New TUI Components, Focus Removal & Unified Output

### Breaking changes

- **`wtm wt focus` removed** — The focus command, `on_focus`/`on_blur` hooks, and active worktree state tracking have been removed. Services are now managed exclusively through `svc up`/`svc down`.
- **`on_focus` / `on_blur` hooks removed from config** — Only `on_create` hooks remain. Docker lifecycle is handled by the service manager.
- **Dashboard hidden** — The interactive dashboard is disabled behind a feature flag while being reworked. `wtm` without arguments shows help instead.

### New features

- **`wtm wt switch [branch]`** — New command that combines `wt go` + `svc up` in one step. Supports `--exclusive`, `--parallel`, and `--profile` flags.
- **Smart `svc up`** — Detects services running on other worktrees and prompts to stop them before starting. Use `--exclusive` to auto-stop or `--parallel` to skip the prompt.
- **Auto-stop on clean** — `wt clean` automatically stops running services before deleting a worktree.
- **Reusable TUI components** — New Bubbletea component library (`internal/tui/components/`) with SelectList, TextInput, MultiSelect, Confirm, and Wizard. Full-row highlight, inline filtering (`/`), step breadcrumb, and Esc back navigation.
- **Contextual PR actions** — `pr list` picker shows "Go to worktree" if a worktree exists for the PR branch, "Checkout into worktree" otherwise.
- **Worktree badges** — `wt list` picker shows colored badge chips (parent, PR, services, dirty/clean) aligned to the right.

### Improvements

- **Unified output styling** — All CLI messages use standardized helpers (`output.Success`, `output.Error`, `output.Warning`, `output.Loading`, `output.Message`) with consistent `"  "` indent.
- **Uniform spacing** — Every command has blank line padding top and bottom. Help text is indented to match.
- **Centralized error display** — All errors go through a styled `✗` handler with proper padding.
- **PR detail view** — Rewritten with output helpers, no more lipgloss box.
- **Separator support** — Action lists use visual separators to group navigation, services, and danger actions.
- **Detached service fix** — Services using `docker compose up -d` (detached mode) are now correctly tracked as running and properly stopped.
- **Config resolution fix** — `svc up`, `svc start`, `svc down` now correctly read `services.toml` from the main worktree when run from a secondary worktree.

### Removed

- `charmbracelet/huh` dependency — Fully replaced by custom Bubbletea components.
- `state.json` active worktree tracking — No longer written to.
- Docker hooks from `wtm init` wizard — The docker-compose file selection and hook confirmation steps are removed.

### Tests

- Added 35+ new tests across commands, output, config, and infra layers.
- Commands: `branchInList`, `buildWorktreeLabel`, `joinTags`, `truncate`, `joinServiceNames`.
- Output: all block helpers (Success, Error, Warning, Loading, Message, etc.).
- Config: corrupted TOML, merge precedence, default application.
- Infra: IsDirty, CurrentBranch, CommitsAhead.

---

## v0.4.1 — Migrate GitHub integration to gh CLI

### Breaking changes

- **`wtm auth` supprimé** — Les commandes `wtm auth login/status/logout` n'existent plus.
  L'authentification GitHub est désormais gérée par le `gh` CLI.
  Installez-le et connectez-vous avec `gh auth login` : [cli.github.com](https://cli.github.com).
- **`WTM_GITHUB_TOKEN` non supporté** — Utilisez `GH_TOKEN` à la place (nativement supporté par `gh`).

### Improvements

- Suppression de toute la couche auth custom (OAuth Device Flow, token storage, auto-refresh) au profit du `gh` CLI.
- `wtm pr list`, `wtm pr create`, `wtm pr checkout` et le dashboard PR passent par `gh` en subprocess.
- Le README documente désormais les dépendances (`git` requis, `gh` recommandé).

---

## v0.4.0 — GitHub Integration & PR Management

### New features

- **GitHub authentication** — `wtm auth login` lance un flux OAuth Device Flow.
  `wtm auth status` affiche l'état du token, `wtm auth logout` le révoque.
  Support de `WTM_GITHUB_TOKEN` comme PAT alternatif. Token auto-refreshé en arrière-plan.
- **`wtm pr list`** — Liste les pull requests du dépôt avec filtres `--mine` et `--review`.
  Intégré dans le dashboard (panneau droit, touche `p`).
- **`wtm pr create`** — Assistant interactif pour créer une PR depuis la branche courante :
  titre, body, draft, reviewers.
- **`wtm pr checkout`** — Crée un worktree directement depuis une branche de PR existante.

### Breaking changes

- **Commandes regroupées sous `wtm wt` et `wtm svc`** — Les commandes worktree passent
  sous `wtm wt` (ex. `wtm wt new`, `wtm wt ls`, `wtm wt go`). Les commandes service
  passent sous `wtm svc` (ex. `wtm svc up`, `wtm svc down`, `wtm svc start`, `wtm svc stop`).
- **`wtm svc start/stop`** — `start`/`stop` ciblent des services individuels,
  `up`/`down` gèrent les profils complets. Les deux sont sous `wtm svc`.

### Improvements

- Migration de `gh` CLI vers `go-github` pour la détection de PR ouverte lors du `clean`.
- Dashboard : logs multiplexés, focus corrigé, split panel worktrees/PRs 50/50.

---

## v0.3.0 — Services & PTY

### New features
