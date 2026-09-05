# dside events — run everything through make. `make help` lists the targets.

BIN      := bin/dside-events
ADDR     ?= 127.0.0.1:8080
DB_PATH  ?= events.db
BASE_URL ?= http://$(ADDR)

# Production (docker-compose.prod.yml, image from GHCR, Watchtower restarts it).
PROD     := docker compose -f docker-compose.prod.yml
CURATORS := "Katerina Spatharou" "George Spanos"

# Overridable curator details for `make poster` / `make poster-link`.
NAME ?=
SLUG ?=

.PHONY: help build run poster poster-link test e2e check fmt vet docker-up docker-down docker-poster docker-poster-link prod-up prod-curators prod-poster-link clean

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

poster-link: build ## new secret link for a curator (the old one stops working): make poster-link SLUG=maria
	@test -n "$(SLUG)" || { echo 'usage: make poster-link SLUG=maria'; exit 2; }
	DB_PATH=$(DB_PATH) BASE_URL=$(BASE_URL) $(BIN) poster-link -slug "$(SLUG)"

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

docker-poster-link: ## new curator link inside docker: make docker-poster-link SLUG=maria
	@test -n "$(SLUG)" || { echo 'usage: make docker-poster-link SLUG=maria'; exit 2; }
	docker compose exec events /dside-events poster-link -slug "$(SLUG)"

prod-up: ## start production from the published image (set BASE_URL to the public address)
	$(PROD) up -d

prod-curators: ## create the production curators (idempotent) and print their secret links
	@for name in $(CURATORS); do $(PROD) exec -T events /dside-events add-poster -name "$$name"; done

prod-poster-link: ## new secret link for a production curator: make prod-poster-link SLUG=george-spanos
	@test -n "$(SLUG)" || { echo 'usage: make prod-poster-link SLUG=george-spanos'; exit 2; }
	$(PROD) exec -T events /dside-events poster-link -slug "$(SLUG)"

clean: ## remove the binary and the dev database
	rm -rf bin $(DB_PATH) $(DB_PATH)-wal $(DB_PATH)-shm
