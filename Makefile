# Copyright 2026 Marc-Antoine Ruel. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# Format, verify and test Go sources and the agent index.

.DEFAULT_GOAL := help
.PHONY: fix git-hooks help test verify

verify:
	@go mod tidy -diff
	@go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	@go tool golangci-lint run --show-stats=false ./...
	@go test -run '^$$' ./...
	@go tool addlicense -ignore pyproject.toml -ignore '.venv/**' -ignore '.ruff_cache/**' -check .
	@uv run --locked python scripts/lint_binaries.py
	@uv run --locked python scripts/update_agents_file_index.py --check
	@uv run --locked ruff check --quiet scripts
	@uv run --locked ruff format --check --quiet scripts

fix:
	@go tool golangci-lint fmt
	@uv run --locked ruff format scripts >/dev/null
	@uv run --locked python scripts/update_agents_file_index.py

test:
	@go test -timeout=600s -v -covermode=count -coverprofile coverage.txt -bench=. -benchtime=1x ./...

help:
	@printf '  %-14s - %s\n' 'make help' 'Show available targets'
	@printf '  %-14s - %s\n' 'make fix' 'Format sources and refresh the agent index'
	@printf '  %-14s - %s\n' 'make verify' 'Run static checks without tests'
	@printf '  %-14s - %s\n' 'make test' 'Run tests with coverage and benchmark smoke tests'
	@printf '  %-14s - %s\n' 'make git-hooks' 'Install Git hooks'

git-hooks:
	@./scripts/install-git-hooks.sh
