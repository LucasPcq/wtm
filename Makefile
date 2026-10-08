BINARY   := wtm
BUILD_DIR := bin

.PHONY: build test vet fmt lint arch dead dead-strict dupl tidy docs site site-dev demos release release-notes install clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

test:
	go test ./... -race -count=1

vet:
	go vet ./...

# fmt fails rather than rewrites: a formatting fix belongs in the commit that
# caused it, not in the one that ran the linter.
fmt:
	@test -z "$$(gofmt -l cmd internal tools *.go)" || \
		{ echo "gofmt needed:"; gofmt -l cmd internal tools *.go; exit 1; }

# arch checks the rules of CLAUDE.md section 9 that no general-purpose linter
# knows about: layers and service edges, the styles monopoly, comma-ok
# assertions, the confirmation axis, mutators reached outside flow/. See
# tools/archlint.
arch:
	go run ./tools/archlint

# dead finds what no path reaches, tests included. staticcheck reports the
# unused *within* a package and cannot see this class at all. The exceptions —
# code reachable by a route the analysis cannot follow — are listed with their
# reason in .deadcode-ignore; everything else fails.
dead:
	@go tool deadcode -test ./... | grep -v -E -f .deadcode-ignore > /tmp/wtm-deadcode || true
	@test ! -s /tmp/wtm-deadcode || { echo "unreachable code:"; cat /tmp/wtm-deadcode; exit 1; }

# dead-strict is dead without -test: what only a test still calls. Informative,
# not part of lint — a helper built for tests (testutil/, processtest/) lives
# there by design, so those packages are left out of the report.
dead-strict:
	@go tool deadcode ./... | grep -v -E -f .deadcode-ignore | grep -v -E '/(testutil|processtest)/' || true

lint: fmt vet arch dead
	go tool staticcheck ./...

# dupl reports token-level clones. It is not part of `lint`: a clone is a
# judgement call — two parallel families over unrelated types read better
# duplicated than behind a generic — so it informs a review rather than gating
# one. Raise the threshold to see only the large ones.
DUPL_THRESHOLD ?= 75
dupl:
	@find cmd internal -name '*.go' ! -name '*_test.go' > /tmp/wtm-dupl-files
	@go tool dupl -t $(DUPL_THRESHOLD) -files < /tmp/wtm-dupl-files

tidy:
	go mod tidy

docs:
	go run ./tools/gendocs

# site builds the documentation site (site/, Starlight) from docs/, README.md and
# CHANGELOG.md into site/dist; site-dev serves it with live reload. Both need Node 22.
site:
	cd site && npm ci && npm test && npm run build

site-dev:
	cd site && npm install && npm run dev

# Re-records the README GIFs from docs/demos/*.tape (needs vhs).
demos:
	docs/demos/record.sh

release: docs
	goreleaser release --snapshot --clean

# release-notes prints the CHANGELOG section of VERSION (0.29.0, no v): the
# release workflow publishes it as the GitHub release notes, where a relative
# link would resolve under /releases/tag/, so docs links are pinned to the tag.
NOTES_REF = $(if $(filter Unreleased,$(VERSION)),main,v$(VERSION))

release-notes:
	@test -n "$(VERSION)" || { echo "usage: make release-notes VERSION=x.y.z"; exit 1; }
	@awk -v v="$(VERSION)" 'index($$0, "## [" v "]") == 1 { on = 1; next } on && /^## \[/ { exit } on && /^\[[^]]+\]: / { exit } on' CHANGELOG.md | \
		sed -e '/./,$$!d' -e 's|](docs/|](https://github.com/LucasPcq/wtm/blob/$(NOTES_REF)/docs/|g'
	@grep -q "^## \[$(VERSION)\]" CHANGELOG.md || { echo "no CHANGELOG section for $(VERSION)" >&2; exit 1; }

install:
	go install .

clean:
	rm -rf $(BUILD_DIR) dist
