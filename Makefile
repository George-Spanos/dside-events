# dside events — run everything through make. `make help` lists the targets.

BIN      := bin/dside-events
ADDR     ?= 127.0.0.1:8080
DB_PATH  ?= events.db
BASE_URL ?= http://$(ADDR)

# Production (docker-compose.prod.yml, image from GHCR, Watchtower restarts it).
PROD     := docker compose -f docker-compose.prod.yml

# Who the curators are lives in curators.txt (gitignored), not here: the roster
# is private, and a name in a tracked file would outlive the person's
# involvement.
#
# NAME and SLUG are plain `=`, not `?=`, so a stray NAME in the environment
# cannot name a curator. `make prod-curator` writes to the production database,
# and `?=` yields to the environment: an exported NAME would be used silently.
# A command-line `make prod-curator NAME="..."` still wins, which is the only
# way these should ever be set. CURATOR keeps `?=` because it only picks an
# existing link out of curators.txt, and a wrong value fails loudly.
NAME =
SLUG =
CURATOR ?= george-spanos

.PHONY: help build run curator curator-links test e2e check fmt vet docker-up docker-down docker-curator docker-curator-links prod-up prod-curator prod-curator-links prod-seed reset-events prod-reset-events prod-backup prod-restore clean

help: ## show this list
	@grep -E '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-21s %s\n", $$1, $$2}'

build: ## compile the binary into bin/
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o $(BIN) .

run: build ## start the dev server (DB in events.db)
	ADDR=$(ADDR) DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) serve

curator: build ## make a curator, prints their secret link: make curator NAME="Maria P." [SLUG=maria]
	@test -n "$(NAME)" || { echo 'usage: make curator NAME="Maria P." [SLUG=maria]'; exit 2; }
	DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) add-poster -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")

curator-links: build ## list every curator with their secret link (rotate one with: $(BIN) poster-link -slug x)
	DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) poster-links

test: ## unit tests (store, slug)
	go test . ./internal/...

e2e: ## end-to-end suite against the real binary
	go test ./e2e/... -count=1

check: fmt vet test e2e ## everything CI would run

fmt: ## fail if any file is not gofmt-clean
	@out="$$(gofmt -l .)"; test -z "$$out" || { echo "$$out"; exit 1; }

vet: ## go vet
	go vet ./...

docker-up: ## build the image and start it with a data volume
	docker compose up --build -d

docker-down: ## stop the container (keeps the volume)
	docker compose down

docker-curator: ## curator inside docker: make docker-curator NAME="Maria P." [SLUG=maria]
	@test -n "$(NAME)" || { echo 'usage: make docker-curator NAME="Maria P." [SLUG=maria]'; exit 2; }
	docker compose exec events /dside-events add-poster -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")

docker-curator-links: ## list every curator with their secret link inside docker
	docker compose exec events /dside-events poster-links

prod-up: ## start production from the published image (set BASE_URL to the public address)
	$(PROD) up -d

prod-curator: ## create one production curator and append their secret link to curators.txt (gitignored): make prod-curator NAME="Maria P." [SLUG=maria]
	@test -n "$(NAME)" || { echo 'usage: make prod-curator NAME="Maria P." [SLUG=maria]'; exit 2; }
	@out=$$($(PROD) exec -T events /dside-events add-poster -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")) || exit $$?; \
	echo "$$out"; \
	if echo "$$out" | grep -q '^link http'; then \
	  echo "$$out" >> curators.txt; \
	  echo "saved to curators.txt"; \
	else \
	  echo 'that curator already exists and their link is unchanged, so curators.txt was left alone;' >&2; \
	  echo 'run make prod-curator-links to recover it, or rotate it with poster-link -slug <slug>' >&2; \
	fi

prod-curator-links: ## list every production curator with their secret link, saved to curators.txt (gitignored, overwritten)
	$(PROD) exec -T events /dside-events poster-links | tee curators.txt

prod-seed: ## publish every seed/*.tsv on production as a curator from curators.txt (FILE=seed/one.tsv for a single file, CURATOR=george-spanos); safe to re-run, events that exist are skipped
	@files="$(if $(FILE),$(FILE),$(sort $(wildcard seed/*.tsv)))"; \
	test -n "$$files" || { echo 'no seed files: expected seed/*.tsv'; exit 2; }; \
	for f in $$files; do test -f "$$f" || { echo "no such file: $$f"; exit 2; }; done; \
	link=$$(awk -v p="poster $(CURATOR)" 'index($$0, p" ")==1 || $$0==p {getline; print $$2}' curators.txt | grep '^http' | tail -1); \
	test -n "$$link" || { echo "no secret link for $(CURATOR) in curators.txt; run make prod-curator-links first"; exit 2; }; \
	failed=0; \
	for f in $$files; do \
	  echo "== $$f"; \
	  seed/seed.sh "$$link" "$$f" || failed=$$((failed+1)); \
	done; \
	test "$$failed" = 0 || { echo "$$failed file(s) had failures" >&2; exit 1; }

reset-events: build ## delete every local event so a seed file can be published again (keeps curators)
	DB_PATH=$(DB_PATH) $(BIN) reset-events -yes

prod-reset-events: ## delete every production event so the seed files can be published again: takes a backup first, keeps curators and their links
	$(MAKE) prod-backup
	$(PROD) exec -T events /dside-events reset-events -yes
	@echo 'now republish: make prod-seed FILE=seed/<events>.tsv [CURATOR=george-spanos]'

prod-backup: ## snapshot the production database into backups/events-<UTC time>.db (safe while running)
	@mkdir -p backups
	$(PROD) exec -T events /dside-events backup -to /data/backup.db
	$(PROD) cp events:/data/backup.db backups/events-$$(date -u +%Y%m%dT%H%M%SZ).db
	@ls -1t backups | head -1

prod-restore: ## replace the production database with a backup: make prod-restore FILE=backups/events-....db (stops the app briefly)
	@test -f "$(FILE)" || { echo 'usage: make prod-restore FILE=backups/events-<time>.db'; exit 2; }
	$(PROD) stop events
	docker run --rm -v $$($(PROD) config --format json | python3 -c 'import json,sys; print(json.load(sys.stdin)["volumes"]["events-data"]["name"])'):/data alpine sh -c 'rm -f /data/events.db /data/events.db-wal /data/events.db-shm'
	$(PROD) cp "$(FILE)" events:/data/events.db
	$(PROD) start events

clean: ## remove the binary and the dev database
	rm -rf bin $(DB_PATH) $(DB_PATH)-wal $(DB_PATH)-shm
