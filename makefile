# -----------------------
# Configuration
# -----------------------
DC := docker compose -f infrastructure/docker/docker-compose.yml

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
	@FILES="$$(find $(PROTO_DIR) -name '*.proto' -print)"; \
	if [ -z "$${FILES}" ]; then \
		echo "No .proto files found in $(PROTO_DIR)"; \
		exit 1; \
	fi; \
	$(PROTOC) --proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$${FILES}
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
.PHONY: up
up: ## Build and start all services with docker-compose
	$(DC) up -d --build

.PHONY: up-no-build
up-no-build: ## Start containers without rebuild
	$(DC) up -d

.PHONY: down
down: ## Stop and remove containers
	$(DC) down

.PHONY: restart
restart: ## Restart all containers
	$(DC) restart

.PHONY: build
build: ## Build images defined in docker-compose
	$(DC) build

.PHONY: build-no-cache
build-no-cache: ## Build images with no cache
	$(DC) build --no-cache

.PHONY: up-% 
up-%: ## Build & start a single service (usage: make up-% where % is a service name as in docker-compose)
	$(DC) up -d --build $*

.PHONY: build-%
build-%: ## Build a specific service (usage: make build-<service>)
	$(DC) build $*

.PHONY: logs
logs: ## Follow logs for all services
	$(DC) logs -f

.PHONY: logs-%
logs-%: ## Follow logs for a specific service, usage: make logs-% (e.g. make logs-api-gateway)
	$(DC) logs -f $*

.PHONY: exec
exec: ## Exec into a running service, usage: make exec SERVICE=api-gateway CMD="sh"
	@if [ -z "$(SERVICE)" ]; then echo "Please set SERVICE and CMD, e.g.: make exec SERVICE=api-gateway CMD='sh'"; exit 1; fi
	$(DC) exec $(SERVICE) sh -c "$(CMD)"

.PHONY: shell
shell: ## Open interactive shell in a running container, usage: make shell SERVICE=api-gateway
	@if [ -z "$(SERVICE)" ]; then echo "Please set SERVICE, e.g.: make shell SERVICE=api-gateway"; exit 1; fi
	$(DC) exec -it $(SERVICE) sh

.PHONY: clean
clean: ## Stop containers and remove volumes (DANGEROUS: deletes data)
	$(DC) down -v

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