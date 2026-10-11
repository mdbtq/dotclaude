BINARY      := dotclaude
PKG         := ./cmd/dotclaude
BUILD_DIR   := bin
DIST_DIR    := dist
WEB_DIR     := web
PREFIX      ?= $(HOME)/.local
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)

# Platforms built by `make build/release`.
PLATFORMS := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64

.DEFAULT_GOAL := build/binary

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} \
		/^##@ / {printf "  \033[1m%s\033[0m\n", substr($$0, 5); next} \
		/^[a-zA-Z0-9_\/.-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' \
		$(MAKEFILE_LIST)

##@ Build

$(WEB_DIR)/node_modules: $(WEB_DIR)/package-lock.json
	cd $(WEB_DIR) && npm ci
	@touch $@

.PHONY: build/web
build/web: $(WEB_DIR)/node_modules ## Build the web UI into web/dist
	cd $(WEB_DIR) && npm run build

.PHONY: build/binary
build/binary: build/web ## Build the binary, UI included, into bin/
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) $(PKG)
	@echo "built $(BUILD_DIR)/$(BINARY) ($(VERSION))"

.PHONY: build/install
build/install: build/web ## Install the binary into $(PREFIX)/bin
	@mkdir -p $(PREFIX)/bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(PREFIX)/bin/$(BINARY) $(PKG)
	@echo "installed $(PREFIX)/bin/$(BINARY) ($(VERSION))"

.PHONY: build/uninstall
build/uninstall: ## Remove the installed binary
	rm -f $(PREFIX)/bin/$(BINARY)

.PHONY: build/release
build/release: build/web ## Cross-compile release binaries into dist/
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=$(DIST_DIR)/$(BINARY)-$(VERSION)-$$os-$$arch; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags '$(LDFLAGS)' -o $$out $(PKG) || exit 1; \
	done
	@echo "release binaries in $(DIST_DIR)/"

.PHONY: build/clean
build/clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out
	find $(WEB_DIR)/dist -mindepth 1 ! -name .gitkeep -delete

##@ Develop

.PHONY: dev/serve
dev/serve: ## Run the API on :7878 (pair with dev/web for the UI)
	go run $(PKG) serve

.PHONY: dev/web
dev/web: $(WEB_DIR)/node_modules ## Run the UI with hot reload on :5173, proxying /api to :7878
	cd $(WEB_DIR) && npm run dev

##@ Test

.PHONY: test/all
test/all: ## Run the test suite
	go test ./...

.PHONY: test/race
test/race: ## Run the test suite with the race detector
	go test -race ./...

.PHONY: test/cover
test/cover: ## Run tests and open the coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

##@ Lint

.PHONY: lint/check
lint/check: ## Run go vet, and golangci-lint when available
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| echo "golangci-lint not installed, ran go vet only"

.PHONY: lint/web
lint/web: $(WEB_DIR)/node_modules ## Type-check the web UI
	cd $(WEB_DIR) && npm run typecheck

.PHONY: lint/fmt
lint/fmt: ## Format the Go source tree
	gofmt -s -w .

.PHONY: lint/fmt-check
lint/fmt-check: ## Fail when the Go source tree is not formatted
	@out=$$(gofmt -s -l .); \
	if [ -n "$$out" ]; then echo "not gofmt-clean:"; echo "$$out"; exit 1; fi

.PHONY: lint/tidy
lint/tidy: ## Tidy and verify module dependencies
	go mod tidy
	go mod verify

.PHONY: qa/check
qa/check: lint/fmt-check lint/check lint/web test/all ## Run every check (formatting, lint, types, tests)
