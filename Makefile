# dside events — run everything through make. `make help` lists the targets.

BIN      := bin/dside-events
ADDR     ?= 127.0.0.1:8080
DB_PATH  ?= events.db
BASE_URL ?= http://$(ADDR)

# Production (docker-compose.prod.yml, image from GHCR, Watchtower restarts it).
PROD     := docker compose -f docker-compose.prod.yml
CURATORS := "Katerina Spatharou" "George Spanos"

# Overridable curator details for `make poster`.
NAME ?=
SLUG ?=

.PHONY: help build run poster poster-links test e2e check fmt vet docker-up docker-down docker-poster docker-poster-links prod-up prod-curators prod-poster-links clean

help: ## show this list
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

build: ## compile the binary into bin/
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o $(BIN) .

run: build ## start the dev server (DB in events.db)
	ADDR=$(ADDR) DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) serve

poster: build ## make a curator, prints their secret link: make poster NAME="Maria P." [SLUG=maria]
	@test -n "$(NAME)" || { echo 'usage: make poster NAME="Maria P." [SLUG=maria]'; exit 2; }
	DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) add-poster -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")

poster-links: build ## list every curator with their secret link (rotate one with: $(BIN) poster-link -slug x)
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

docker-poster: ## curator inside docker: make docker-poster NAME="Maria P." [SLUG=maria]
	@test -n "$(NAME)" || { echo 'usage: make docker-poster NAME="Maria P." [SLUG=maria]'; exit 2; }
	docker compose exec events /dside-events add-poster -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")

docker-poster-links: ## list every curator with their secret link inside docker
	docker compose exec events /dside-events poster-links

prod-up: ## start production from the published image (set BASE_URL to the public address)
	$(PROD) up -d

prod-curators: ## create the production curators (idempotent), print their secret links and save them to curators.txt (gitignored)
	@for name in $(CURATORS); do $(PROD) exec -T events /dside-events add-poster -name "$$name"; done | tee -a curators.txt

prod-poster-links: ## list every production curator with their secret link, saved to curators.txt (gitignored)
	$(PROD) exec -T events /dside-events poster-links | tee curators.txt

clean: ## remove the binary and the dev database
	rm -rf bin $(DB_PATH) $(DB_PATH)-wal $(DB_PATH)-shm
