.DEFAULT_GOAL := help

ifneq (,$(wildcard ./.env))
  include .env
  export
endif

TIMEOUT ?= 120m
GOMAXPROCS ?= 5
TESTARGS ?= ./...

.PHONY: build
build: ## build program
	go build cmd/azdo/azdo.go

.PHONY: dist
dist: ## create new release
	goreleaser release --clean --skip publish

.PHONY: clean
clean: ## clean repositorty
	rm -f azdo
	rm -rf dist

.PHONY: lint
lint: ## lint source
	@echo "Check for golangci-lint"; [ -e "$(shell which golangci-lint)" ]
	@echo "Executing golangci-lint"; golangci-lint run -v --timeout $(TIMEOUT)

.PHONY: help
tidy: ## call go mod tidy on all existing go.mod files
	find . -name go.mod -execdir go mod tidy \;

.PHONY: docs
docs: ## create documentation
	go run cmd/gen-docs/gen-docs.go --doc-path ./docs --website

.PHONY: generate-mocks
generate-mocks: ## regenerate Go mocks (requires mockgen)
	bash scripts/generate_mocks.sh

.PHONY: help
help:
	@grep '^[^#.][A-Za-z._/]\+:\s\+.*#' Makefile | \
	sed "s/\(.\+\):\s*\(.*\) #\s*\(.*\)/`printf "\033[93m"`\1`printf "\033[0m"`	\3 [\2]/" | \
	expand -t30
