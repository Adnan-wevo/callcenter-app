# callcenter-service
#
# Modelled on wevetel-bastion's Makefile (github/local: ~/wevetel-bastion) —
# same shape: .DEFAULT_GOAL/help, .PHONY up front, one comment-header per
# section, `target: ## description` doc comments picked up by `help`'s own
# grep+awk one-liner below.
#
# The actual local dev topology (see README.md + .env.example) is a hybrid:
# mysql + the two mocks + the Go server itself all run via `docker compose`
# (docker-compose.yml), while the Angular dev server runs in its OWN
# container (`cc-web`, --network host, web/ bind-mounted in) that is NOT
# part of docker-compose.yml — nothing here invents a new topology, this
# just gives the existing one names.

.DEFAULT_GOAL := help

BIN_DIR := bin
WEB_CONTAINER := cc-web
WEB_IMAGE := node:24-alpine
WEB_PORT := 4200

.PHONY: help \
	build-server test \
	up down status \
	restart restart-api restart-web \
	web-up web-down \
	logs-api logs-web logs-mysql \
	db-shell db-tables \
	hmacsign \
	clean

## ---------------------------------------------------------------- build ----

build-server: ## Build the Go server into bin/callcenter-service
	go build -o $(BIN_DIR)/callcenter-service ./cmd/server

test: ## Run all Go tests
	go test ./...

## ------------------------------------------------------------------ run ----
#
# `up`/`down` own the docker-compose services (mysql, mock-pbx-worker,
# mock-laravel, callcenter-service). `web-up`/`web-down` own the ONE
# container docker-compose.yml does not define. `restart` is the two-line
# dev loop: rebuild the API image and recreate it, then restart the web
# container so its esbuild watcher gets a clean re-scan — the same pair of
# commands used by hand to get a code change (Go or Angular) actually
# showing up, named instead of re-typed.

up: ## Start mysql + both mocks + the API via docker compose, then the web dev server
	docker compose up -d --build
	@$(MAKE) --no-print-directory web-up

down: ## Stop everything (docker compose services + the web dev container)
	docker compose down
	@$(MAKE) --no-print-directory web-down

restart: restart-api restart-web ## Rebuild+restart the API AND restart the web dev server — the one command for "I changed code, show me"

restart-api: ## Rebuild the API image and recreate just that container (picks up Go changes)
	docker compose build callcenter-service
	docker compose up -d callcenter-service
	@sleep 2
	@curl -sf http://localhost:8080/healthz >/dev/null \
		&& echo "API up: http://localhost:8080" \
		|| (echo "API FAILED to come up - check: make logs-api" && exit 1)

restart-web: ## Restart the web dev container (picks up Angular/TS changes; bind-mounted, so usually not even needed — the esbuild watcher hot-reloads on its own)
	@docker restart $(WEB_CONTAINER) >/dev/null 2>&1 || $(MAKE) --no-print-directory web-up
	@echo "Web dev server restarting: http://localhost:$(WEB_PORT) (tail with 'make logs-web' for when it's ready)"

web-up: ## Start the Angular dev server container fresh (idempotent — replaces it if already running)
	@docker rm -f $(WEB_CONTAINER) >/dev/null 2>&1 || true
	docker run -d --name $(WEB_CONTAINER) \
		--network host \
		-v $(PWD)/web:/app \
		-w /app \
		$(WEB_IMAGE) \
		npx ng serve --host 0.0.0.0 --port $(WEB_PORT)
	@echo "Web dev server: http://localhost:$(WEB_PORT) (tail with 'make logs-web' for when it's ready)"

web-down: ## Stop and remove the web dev container
	docker rm -f $(WEB_CONTAINER) >/dev/null 2>&1 || true

status: ## Show container status for the whole local stack
	docker compose ps
	@docker ps --filter name=$(WEB_CONTAINER) --format 'table {{.Names}}\t{{.Status}}'

## ----------------------------------------------------------------- logs ----

logs-api: ## Follow the API's logs
	docker compose logs -f callcenter-service

logs-web: ## Follow the Angular dev server's logs (compile errors, rebuild timing)
	docker logs -f $(WEB_CONTAINER)

logs-mysql: ## Follow mysql's logs
	docker compose logs -f mysql

## ------------------------------------------------------------------- db ----
#
# Migrations here are plain numbered .sql files (migrations/callcenter/),
# mounted into docker-entrypoint-initdb.d — no atlas/golang-migrate. That
# means a NEW migration file only runs automatically against a FRESH
# volume; against the qstats-data volume this stack already has, apply it
# by hand once with `make db-shell` (or pipe the file straight in:
# `docker compose exec -T mysql mysql -uroot -prootpass callcenter < migrations/callcenter/00N_x.sql`).

db-shell: ## Open a mysql shell on the callcenter database
	docker compose exec mysql mysql -uroot -prootpass callcenter

db-tables: ## List tables in the callcenter database (quick "did my migration apply" check)
	docker compose exec mysql mysql -uroot -prootpass callcenter -e "SHOW TABLES;"

## ---------------------------------------------------------------- tools ----

hmacsign: ## Print signed HMAC headers for a manual curl test (method=GET uri=/api/v1/... [body='{"k":"v"}'])
	@test -n "$(method)" || (echo "usage: make hmacsign method=GET uri=/api/v1/call-center/queues" && exit 1)
	@test -n "$(uri)" || (echo "usage: make hmacsign method=GET uri=/api/v1/call-center/queues" && exit 1)
	go run ./cmd/hmacsign -key local-dev-key -secret local-dev-secret \
		-method $(method) -uri "$(uri)" $(if $(body),-body '$(body)',)

## ---------------------------------------------------------------- clean ----

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)/

## ----------------------------------------------------------------- help ----

help: ## List all available targets
	@echo "callcenter-service available targets:"
	@echo ""
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
