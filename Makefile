# KubeBench build entry points. `make` (or `make help`) lists the targets.
#
# Build metadata comes from git: VERSION from scripts/version.sh, COMMIT and
# DATE from the checked-out commit. Override any of them on the command line,
# e.g. `make build VERSION=0.8.0`.

SHELL := bash
.SHELLFLAGS := -Eeuo pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
MAKEFLAGS += --no-builtin-rules

VERSION ?= $(shell scripts/version.sh)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
# Commit time rather than wall-clock time keeps builds reproducible.
DATE    ?= $(shell TZ=UTC git log -1 --format=%cd --date=format-local:%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
# Expand the shell calls above once instead of on every use; VERSION=v1.2.3
# on the command line means 1.2.3, as in scripts/version.sh.
override VERSION := $(patsubst v%,%,$(VERSION))
COMMIT := $(COMMIT)
DATE   := $(DATE)

IMAGE      ?= ghcr.io/shavell/kubebench
# Container image tarballs to put into the offline bundle (space separated).
IMAGE_TARS ?=

BIN       := bin/kubebench
EMBED_DIR := backend/internal/web/dist
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# CHANGELOG.md is bundled for What's new (src/core/updates/changelog.ts).
FRONTEND_SRC  := index.html vite.config.ts package.json CHANGELOG.md $(wildcard tsconfig*.json) \
                 $(shell find src public -type f 2>/dev/null)
SHELL_SCRIPTS := install.sh $(wildcard scripts/*.sh packaging/scripts/*.sh)

.PHONY: help version deps frontend embed build run test lint chart-lint manifests \
        image snapshot deb offline-bundle release-check clean

##@ General

help: ## Show this help
	@awk 'BEGIN { FS = ":.*## " } \
	  /^##@/ { printf "\n%s\n", substr($$0, 5) } \
	  /^[a-z0-9-]+:.*## / { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo

version: ## Print the build version
	@echo '$(VERSION)'

deps: node_modules/.package-lock.json ## Install npm packages and download Go modules
	go -C backend mod download

# npm writes node_modules/.package-lock.json on every install, which makes it
# a stamp for "node_modules matches package-lock.json".
node_modules/.package-lock.json: package.json package-lock.json
	npm ci --no-audit --no-fund

##@ Build

frontend: dist/index.html ## Build the frontend into dist/ (npm ci + vite build)

dist/index.html: node_modules/.package-lock.json $(FRONTEND_SRC)
	npm run build

embed: $(EMBED_DIR)/index.html ## Copy dist/ into the Go embed directory (keeps .gitkeep)

$(EMBED_DIR)/index.html: dist/index.html
	find $(EMBED_DIR) -mindepth 1 ! -name .gitkeep -delete
	cp -R dist/. $(EMBED_DIR)/

build: embed ## Build bin/kubebench with the frontend embedded
	CGO_ENABLED=0 go -C backend build -trimpath -ldflags '$(LDFLAGS)' -o $(CURDIR)/$(BIN) .

run: build ## Build and run the server on :9876 (state in build/run/; honours KUBECONFIG)
	mkdir -p build/run
	KUBEBENCH_DATA_DIR=$(CURDIR)/build/run KUBEBENCH_DB=$(CURDIR)/build/run/kubebench.db $(BIN)

##@ Quality

test: node_modules/.package-lock.json ## Run Go tests and the frontend checks (tsc, oxlint, i18n, unit tests)
	npx tsc -b
	npm run lint
	npm run i18n:check
	npm test
	go -C backend test ./...

lint: node_modules/.package-lock.json ## Static analysis: gofmt, go vet, oxlint, shellcheck, actionlint
	@unformatted="$$(gofmt -l backend)"; \
	  if [[ -n "$$unformatted" ]]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go -C backend vet ./...
	npm run lint
	shellcheck $(SHELL_SCRIPTS)
	actionlint

chart-lint: ## Lint the Helm chart and validate its manifests with kubeconform
	scripts/lint-chart.sh

##@ Distribution

manifests: ## Render deploy/{kubernetes,openshift}/kubebench.yaml from the chart
	scripts/render-manifests.sh

image: ## Build the container image for the local platform, tagged IMAGE:VERSION
	docker build --build-arg VERSION='$(VERSION)' --build-arg COMMIT='$(COMMIT)' \
	  --build-arg DATE='$(DATE)' -t '$(IMAGE):$(VERSION)' .

snapshot: ## Build every archive and package with GoReleaser into build/release/ (no publish)
	goreleaser release --snapshot --clean --skip=sign,sbom

deb: ## Build snapshot .deb and .rpm packages into build/release/
	goreleaser release --snapshot --clean --skip=archive,sign,sbom

offline-bundle: ## Assemble the offline bundle from build/release/ (IMAGE_TARS="a.tar b.tar")
	scripts/offline-bundle.sh $(foreach tar,$(IMAGE_TARS),--image-tar $(tar))

release-check: ## Check versions, CHANGELOG and manifests for a release (make release-check VERSION=0.8.0)
	scripts/check-release.sh '$(VERSION)'

clean: ## Remove bin/, build/, dist/ and the embedded frontend copy
	rm -rf bin build dist
	find $(EMBED_DIR) -mindepth 1 ! -name .gitkeep -delete
