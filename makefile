# -----------------------
# Configuration
# -----------------------
DC_PROD := docker compose -f infrastructure/docker/docker-compose.yml
DC_DEV := docker compose -f infrastructure/docker/docker-compose.yml -f infrastructure/docker/docker-compose.override.yml

PROTO_DIR := proto
GEN_DIR := shared/proto

# list your services directories under services/ (used by go-build-all etc.)
SERVICES := auth-service api-gateway user-service order-service notification-service media-service review-service chat-service

# Path to Go tools (adjust if needed)
PROTOC := protoc

# -----------------------
# Helpers
# -----------------------

.PHONY: help
help: ## Show this help.
	@echo "Usage: make <target> [VARIABLE=value]"
	@echo
	@echo "Common targets:"
	@grep -E '^[a-zA-Z0-9._-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-25s %s\n", $$1, $$2}'
	@echo

# -----------------------
# Protobuf generation
# -----------------------
.PHONY: proto-gen
proto-gen: ## Generate protobuf Go code into $(GEN_DIR). Requires protoc and protoc-gen-go/protoc-gen-go-grpc in PATH.
	@echo "Generating protobuf files from '$(PROTO_DIR)' → '$(GEN_DIR)' ..."
	@rm -rf $(GEN_DIR)
	@mkdir -p $(GEN_DIR)

	@for file in $(PROTO_DIR)/*.proto; do \
		base=$$(basename $$file .proto); \
		outdir=$(GEN_DIR)/$$base; \
		mkdir -p $$outdir; \
		protoc --proto_path=$(PROTO_DIR) \
			--go_out=$$outdir --go_opt=paths=source_relative \
			--go-grpc_out=$$outdir --go-grpc_opt=paths=source_relative \
			$$file; \
		echo "Generated $$file → $$outdir"; \
	done

	@echo "Protobuf generation completed."

.PHONY: proto-clean
proto-clean: ## Remove generated protobuf code.
	@rm -rf $(GEN_DIR)
	@echo "Removed $(GEN_DIR)."

.PHONY: proto-install-tools
proto-install-tools: ## Install protoc-gen-go and protoc-gen-go-grpc (local user go/bin must be in PATH)
	@echo "Installing protoc plugins (protoc-gen-go, protoc-gen-go-grpc)..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Make sure $$GOPATH/bin or $$GOBIN is in your PATH."

# -----------------------
# Docker / Compose targets
# -----------------------

# Production commands
.PHONY: up-prod
up-prod: ## Build and start all services in production mode
	$(DC_PROD) up -d --build

.PHONY: down-prod
down-prod: ## Stop production services
	$(DC_PROD) down

# Development commands (traditional)
.PHONY: up-dev
up-dev: ## Build and start all services in development mode
	$(DC_DEV) up -d --build

.PHONY: down-dev
down-dev: ## Stop development services
	$(DC_DEV) down

# Watch commands (new - for hot reload)
.PHONY: watch
watch: ## Start services in watch mode (hot reload)
	$(DC_DEV) watch

.PHONY: watch-build
watch-build: ## Build and start services in watch mode
	$(DC_DEV) build
	$(DC_DEV) watch

.PHONY: watch-logs
watch-logs: ## Watch services with logs output
	$(DC_DEV) watch --no-up

.PHONY: down
down: ## Stop and remove containers
	$(DC_DEV) down

.PHONY: restart
restart: ## Restart all containers
	$(DC_DEV) restart

.PHONY: build
build: ## Build images defined in docker-compose
	$(DC_DEV) build

.PHONY: build-no-cache
build-no-cache: ## Build images with no cache
	$(DC_DEV) build --no-cache

.PHONY: up-% 
up-%: ## Build & start a single service (usage: make up-% where % is a service name as in docker-compose)
	$(DC_DEV) up -d --build $*

.PHONY: build-%
build-%: ## Build a specific service (usage: make build-<service>)
	$(DC_DEV) build $*

.PHONY: logs
logs: ## Follow logs for all services
	$(DC_DEV) logs -f

.PHONY: logs-%
logs-%: ## Follow logs for a specific service, usage: make logs-% (e.g. make logs-api-gateway)
	$(DC_DEV) logs -f $*

.PHONY: exec
exec: ## Exec into a running service, usage: make exec SERVICE=api-gateway CMD="sh"
	@if [ -z "$(SERVICE)" ]; then echo "Please set SERVICE and CMD, e.g.: make exec SERVICE=api-gateway CMD='sh'"; exit 1; fi
	$(DC_DEV) exec $(SERVICE) sh -c "$(CMD)"

.PHONY: shell
shell: ## Open interactive shell in a running container, usage: make shell SERVICE=api-gateway
	@if [ -z "$(SERVICE)" ]; then echo "Please set SERVICE, e.g.: make shell SERVICE=api-gateway"; exit 1; fi
	$(DC_DEV) exec -it $(SERVICE) sh

.PHONY: clean
clean: ## Stop containers and remove volumes (DANGEROUS: deletes data)
	docker system prune -a

# -----------------------
# Local go helpers (per-service)
# -----------------------
.PHONY: go-build-all
go-build-all: ## Build Go binaries for all services locally (runs `go build` inside each service dir)
	@for s in $(SERVICES); do \
		if [ -d "services/$$s" ]; then \
			echo "=> Building $$s ..."; \
			( cd services/$$s && go build ./... ) || exit 1; \
		fi \
	done
	@echo "All services built."

.PHONY: go-test-all
go-test-all: ## Run tests for all services
	@for s in $(SERVICES); do \
		if [ -d "services/$$s" ]; then \
			echo "=> Testing $$s ..."; \
			( cd services/$$s && go test ./... ) || exit 1; \
		fi \
	done
	@echo "All tests passed."

.PHONY: fmt
fmt: ## gofmt all services
	@for s in $(SERVICES); do \
		if [ -d "services/$$s" ]; then \
			( cd services/$$s && gofmt -w . ) ; \
		fi \
	done
	@echo "gofmt done."

.PHONY: vet
vet: ## go vet all services
	@for s in $(SERVICES); do \
		if [ -d "services/$$s" ]; then \
			( cd services/$$s && go vet ./... ) ; \
		fi \
	done
	@echo "go vet done."

.PHONY: lint
lint: ## Run golangci-lint if available (install from https://golangci-lint.run/)
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not found, install it: https://golangci-lint.run/"; exit 1; \
	fi
	@for s in $(SERVICES); do \
		if [ -d "services/$$s" ]; then \
			echo "=> Linting $$s ..."; \
			( cd services/$$s && golangci-lint run ) || exit 1; \
		fi \
	done
	@echo "Linting done."

# -----------------------
# Misc utilities
# -----------------------
.PHONY: prune-images
prune-images: ## Remove dangling docker images
	docker image prune -f

.PHONY: system-prune
system-prune: ## Dangerous: prune docker system (images, containers, volumes). USE WITH CAUTION.
	docker system prune -a --volumes

.PHONY: diagnose
diagnose: ## Diagnose project structure
	@echo "=== Project Structure Diagnosis ==="
	@echo "Current directory: $$(pwd)"
	@echo ""
	@echo "Services directory contents:"
	@ls -la services/ 2>/dev/null || echo "services/ directory not found"
	@echo ""
	@echo "Checking specific paths:"
	@test -f services/auth/main.go && echo "✓ services/auth/main.go exists" || echo "✗ services/auth/main.go not found"
	@test -f services/api-gateway/main.go && echo "✓ services/api-gateway/main.go exists" || echo "✗ services/api-gateway/main.go not found"
	@echo ""
	@echo "All main.go files:"
	@find . -name "main.go" -type f | grep -v vendor | grep -v tmp