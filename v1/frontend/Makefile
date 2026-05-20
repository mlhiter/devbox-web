SERVICE_NAME=sealos-devbox
# Image URL to use all building/pushing image targets
IMG ?= $(SERVICE_NAME):latest
PNPM ?= pnpm
PNPM_INSTALL_FLAGS ?= --frozen-lockfile

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: install
install: ## Install frontend dependencies.
	$(PNPM) install $(PNPM_INSTALL_FLAGS)

.PHONY: lint
lint: ## Run frontend lint checks.
	$(PNPM) run lint

.PHONY: typecheck
typecheck: ## Run TypeScript type checks.
	$(PNPM) run ts-lint

.PHONY: verify
verify: ## Run the standard frontend verification pipeline.
	$(MAKE) install
	$(MAKE) lint
	$(MAKE) typecheck
	$(MAKE) build

##@ Build

.PHONY: build
build: ## Build desktop-frontend binary.
	$(PNPM) run build

.PHONY: dev
dev: ## Run the frontend in development mode.
	$(PNPM) run dev

.PHONY: run
run: ## Run a dev service from host.
	$(PNPM) run start

.PHONY: docker-build
docker-build: ## Build docker image with the desktop-frontend.
	docker build -t sealos-devbox:latest . --network host --build-arg DEVBOX_COMMIT_HASH=$$(git rev-parse --short=12 HEAD) --build-arg HTTP_PROXY=http://127.0.0.1:7890 --build-arg HTTPS_PROXY=http://127.0.0.1:7890
