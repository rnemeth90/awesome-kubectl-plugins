SHELL := /bin/bash
.DEFAULT_GOAL := help

PLUGINS_DIR  := plugins
TEMPLATE_DIR := template
BIN_DIR      := bin
INSTALL_DIR  ?= $(HOME)/.local/bin

PLUGINS := $(patsubst $(PLUGINS_DIR)/%/,%,$(wildcard $(PLUGINS_DIR)/*/))
PLUGIN  ?=
# Extra names installed as symlinks. kubectl maps "kubectl aks-versions" to a
# binary named kubectl-aks_versions (dashes become underscores).
ALIAS_nodepool       := np
ALIAS_aksversions    := aks_versions
ALIAS_aksmaintenance := aks_maintenance
ALIAS_settags        := set_tags
ARGS    ?=
GOTESTFLAGS ?=

# With PLUGIN set, commands act on that plugin only; otherwise on all plugins.
TARGETS := $(if $(PLUGIN),$(PLUGIN),$(PLUGINS))
PKGS    := $(if $(PLUGIN),./$(PLUGINS_DIR)/$(PLUGIN)/...,./...)

.PHONY: help list build install uninstall run test cover vet fmt tidy check new clean docs docs-serve lens _check-plugin _require-plugin

help: ## Show this help
	@echo "Usage: make <target> [PLUGIN=name] [ARGS=\"...\"]"
	@echo
	@echo "Targets:"
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'
	@echo
	@echo "Variables:"
	@echo "  PLUGIN       Limit a target to one plugin (default: all)"
	@echo "  ARGS         Arguments passed to the plugin for 'make run'"
	@echo "  INSTALL_DIR  Install destination (default: $(INSTALL_DIR))"
	@echo "  GOTESTFLAGS  Extra flags for go test, e.g. GOTESTFLAGS=-v"
	@echo
	@echo "Examples:"
	@echo "  make build"
	@echo "  make test PLUGIN=count GOTESTFLAGS=-v"
	@echo "  make run PLUGIN=count ARGS=\"pods -n kube-system\""
	@echo "  make new PLUGIN=nodecount"
	@echo "  make install"

list: ## List plugins and whether they are installed
	@for p in $(PLUGINS); do \
		if [ -x "$(INSTALL_DIR)/kubectl-$$p" ]; then s="installed"; else s="-"; fi; \
		printf "  %-16s %s\n" "kubectl-$$p" "$$s"; \
	done

build: _check-plugin ## Build plugin binaries into ./bin
	@mkdir -p $(BIN_DIR)
	@for p in $(TARGETS); do \
		echo "building $(BIN_DIR)/kubectl-$$p"; \
		go build -trimpath -ldflags "-s -w" -o $(BIN_DIR)/kubectl-$$p ./$(PLUGINS_DIR)/$$p || exit 1; \
	done

install: build ## Build and install plugins into INSTALL_DIR (must be on PATH)
	@mkdir -p $(INSTALL_DIR)
	@for p in $(TARGETS); do \
		install -m 0755 $(BIN_DIR)/kubectl-$$p $(INSTALL_DIR)/kubectl-$$p && \
		echo "installed $(INSTALL_DIR)/kubectl-$$p"; \
	done
	@$(foreach p,$(filter $(patsubst ALIAS_%,%,$(filter ALIAS_%,$(.VARIABLES))),$(TARGETS)), \
		ln -sf kubectl-$(p) $(INSTALL_DIR)/kubectl-$(ALIAS_$(p)) && echo "aliased $(INSTALL_DIR)/kubectl-$(ALIAS_$(p)) -> kubectl-$(p)";)
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; \
		*) echo "warning: $(INSTALL_DIR) is not on your PATH";; esac

uninstall: _check-plugin ## Remove installed plugins from INSTALL_DIR
	@for p in $(TARGETS); do \
		rm -f $(INSTALL_DIR)/kubectl-$$p && echo "removed $(INSTALL_DIR)/kubectl-$$p"; \
	done
	@$(foreach p,$(filter $(patsubst ALIAS_%,%,$(filter ALIAS_%,$(.VARIABLES))),$(TARGETS)), \
		rm -f $(INSTALL_DIR)/kubectl-$(ALIAS_$(p)) && echo "removed $(INSTALL_DIR)/kubectl-$(ALIAS_$(p))";)

run: _require-plugin ## Run a plugin without installing: make run PLUGIN=x ARGS="..."
	@go run ./$(PLUGINS_DIR)/$(PLUGIN) $(ARGS)

test: _check-plugin ## Run unit tests (all plugins, or PLUGIN=name)
	go test $(GOTESTFLAGS) $(PKGS)

cover: _check-plugin ## Run tests with a coverage summary
	@mkdir -p $(BIN_DIR)
	go test -coverprofile=$(BIN_DIR)/coverage.out $(PKGS)
	go tool cover -func=$(BIN_DIR)/coverage.out | tail -n 1

vet: ## Run go vet
	go vet ./...

fmt: ## Format Go code with gofmt
	gofmt -l -w $(PLUGINS_DIR) internal

tidy: ## Tidy go.mod / go.sum
	go mod tidy

check: ## Verify formatting, run vet and tests (CI-style)
	@test -z "$$(gofmt -l $(PLUGINS_DIR) internal)" || { echo "gofmt needed on:"; gofmt -l $(PLUGINS_DIR) internal; exit 1; }
	$(MAKE) vet
	$(MAKE) test

new: ## Scaffold a plugin from template/: make new PLUGIN=name
	@if [ -z "$(PLUGIN)" ]; then echo "PLUGIN is required, e.g. make new PLUGIN=nodecount"; exit 1; fi
	@if ! echo "$(PLUGIN)" | grep -qE '^[a-z][a-z0-9]*$$'; then \
		echo "invalid plugin name '$(PLUGIN)': use lowercase letters and digits, starting with a letter"; exit 1; fi
	@if [ -e "$(PLUGINS_DIR)/$(PLUGIN)" ]; then echo "$(PLUGINS_DIR)/$(PLUGIN) already exists"; exit 1; fi
	@mkdir -p $(PLUGINS_DIR)/$(PLUGIN)
	@sed 's/__NAME__/$(PLUGIN)/g' $(TEMPLATE_DIR)/main.go.tmpl > $(PLUGINS_DIR)/$(PLUGIN)/main.go
	@sed 's/__NAME__/$(PLUGIN)/g' $(TEMPLATE_DIR)/main_test.go.tmpl > $(PLUGINS_DIR)/$(PLUGIN)/main_test.go
	@echo "created $(PLUGINS_DIR)/$(PLUGIN)/{main.go,main_test.go}"
	@echo "next: make test PLUGIN=$(PLUGIN) && make install PLUGIN=$(PLUGIN)"

docs: ## Build the Hugo documentation site into site/public
	hugo --source site --minify

lens: ## Test and build the Freelens extension (lens/dist)
	cd lens && npm install && npm test && npm run type:check && npm run build

docs-serve: ## Serve the documentation site locally with live reload
	hugo server --source site

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) site/public site/resources

_check-plugin:
	@if [ -n "$(PLUGIN)" ] && [ ! -d "$(PLUGINS_DIR)/$(PLUGIN)" ]; then \
		echo "unknown plugin '$(PLUGIN)'. available: $(PLUGINS)"; exit 1; fi

_require-plugin: _check-plugin
	@if [ -z "$(PLUGIN)" ]; then echo "PLUGIN is required, e.g. make run PLUGIN=count"; exit 1; fi
