# dside events — run everything through make. `make help` lists the targets.

BIN      := bin/dside-events
ADDR     ?= 127.0.0.1:8080
DB_PATH  ?= events.db
OTP_FILE ?= otp.log

# Overridable poster details for `make poster`.
EMAIL ?=
NAME  ?=
SLUG  ?=

.PHONY: help build run poster test e2e check fmt vet docker-up docker-down docker-poster clean

help: ## show this list
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

build: ## compile the binary into bin/
	@mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -o $(BIN) .

run: build ## start the dev server (codes go to otp.log, DB to events.db)
	ADDR=$(ADDR) DB_PATH=$(DB_PATH) DEV_OTP_FILE=$(OTP_FILE) $(BIN) serve

poster: build ## make someone a curator: make poster EMAIL=x@y.gr NAME="Maria P." [SLUG=maria]
	@test -n "$(EMAIL)" -a -n "$(NAME)" || { echo 'usage: make poster EMAIL=x@y.gr NAME="Maria P." [SLUG=maria]'; exit 2; }
	DB_PATH=$(DB_PATH) $(BIN) add-poster -email "$(EMAIL)" -name "$(NAME)" $(if $(SLUG),-slug "$(SLUG)")

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

docker-poster: ## curator inside docker: make docker-poster EMAIL=x@y.gr NAME="Maria P."
	@test -n "$(EMAIL)" -a -n "$(NAME)" || { echo 'usage: make docker-poster EMAIL=x@y.gr NAME="Maria P."'; exit 2; }
	docker compose exec events /dside-events add-poster -email "$(EMAIL)" -name "$(NAME)"

clean: ## remove the binary, the dev database and the otp log
	rm -rf bin $(DB_PATH) $(DB_PATH)-wal $(DB_PATH)-shm $(OTP_FILE)
