SHELL := /bin/bash

.DEFAULT_GOAL := help

.PHONY: help check install test build deploy-only deploy clean run

help:
	@echo "Available targets:"
	@echo "  check          Validate Shell syntax, ShellCheck (when installed), and whitespace"
	@echo "  install        Install Node.js dependencies in src/"
	@echo "  test           Install dependencies and run the Node.js tests"
	@echo "  build          Run all checks/tests and prepare the SCF entry point"
	@echo "  deploy-only    Deploy the current src/ directory"
	@echo "  deploy         Build and deploy"
	@echo "  clean          Remove installed Node.js dependencies"

check:
	/bin/bash -n src/shell/*.sh
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck --severity=warning --shell=bash src/shell/*.sh; \
	else \
		echo "shellcheck not installed; skipping ShellCheck"; \
	fi
	#git diff --check

install:
	cd src && npm ci

test: install
	cd src && npm test

build: check test
	chmod +x src/scf_bootstrap

deploy-only:
	@command -v scf >/dev/null 2>&1 || { echo "scf CLI is required" >&2; exit 1; }
	@test -f src/index.js || { echo "src/index.js is missing" >&2; exit 1; }
	@test -d src/node_modules || { echo "dependencies are missing; run 'make build' first" >&2; exit 1; }
	scf deploy

deploy: build deploy-only

clean:
	rm -rf src/node_modules

run:
	@node src/index.js
