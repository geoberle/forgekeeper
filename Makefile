SHELL := /bin/bash
PYTHON ?= python3
TOOLS := .local/dev-venv/bin
SHELL_SOURCES := $(wildcard scripts/*.sh)

.PHONY: install-tools fmt lint lint-fix test verify verify-fmt

install-tools: $(TOOLS)/.installed

$(TOOLS)/.installed: requirements-dev.txt
	$(PYTHON) -m venv .local/dev-venv
	$(TOOLS)/python -m pip install -r requirements-dev.txt
	touch $@

fmt: install-tools
	$(TOOLS)/ruff format scripts tests
	$(TOOLS)/shfmt -w -i 4 $(SHELL_SOURCES)

verify-fmt: install-tools
	$(TOOLS)/ruff format --check scripts tests
	$(TOOLS)/shfmt -d -i 4 $(SHELL_SOURCES)

lint: install-tools
	$(TOOLS)/ruff check scripts tests
	$(TOOLS)/shellcheck $(SHELL_SOURCES)
	$(TOOLS)/yamllint .github projects .yamllint.yml
	helm lint charts/factory -f charts/factory/example-values.yaml
	$(TOOLS)/yamllint charts/factory/Chart.yaml charts/factory/values.yaml charts/factory/example-values.yaml

lint-fix: install-tools
	$(TOOLS)/ruff check --fix scripts tests
	$(MAKE) fmt

test: install-tools
	$(TOOLS)/python -m unittest discover -s tests -v

verify: verify-fmt lint test

GO_TOOLCHAIN := go1.25.5
GOLANGCI_LINT := .local/bin/golangci-lint

$(GOLANGCI_LINT):
	mkdir -p .local/bin
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOBIN=$(CURDIR)/.local/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0

.PHONY: build go-fmt go-verify-fmt go-lint go-test
build:
	go build -o .local/bin/orchestrator ./cmd/orchestrator

go-fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

go-verify-fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff > .local/go-format.diff
	@cat .local/go-format.diff
	@test ! -s .local/go-format.diff

go-lint: $(GOLANGCI_LINT)
	GOTOOLCHAIN=$(GO_TOOLCHAIN) $(GOLANGCI_LINT) run

go-test:
	go test -race ./...

fmt: go-fmt
verify-fmt: go-verify-fmt
lint: go-lint
test: go-test
