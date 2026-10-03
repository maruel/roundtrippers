# Copyright 2026 Marc-Antoine Ruel. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

.DEFAULT_GOAL := help
.PHONY: build fix help test verify

verify:
	@go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	@go tool golangci-lint run --show-stats=false ./...
	@go vet ./...
	@go test -run '^$$' ./...
	@go tool addlicense -check .
	@files=$$(git ls-files --stage | awk '$$1 == "100755" && $$4 !~ /\.(sh|py)$$/ { print $$4 }'); \
	if [ -n "$$files" ]; then echo 'Do not commit executables beside shell scripts:'; echo "$$files"; exit 1; fi

fix:
	@go tool golangci-lint run --show-stats=false ./... --fix
	@go tool golangci-lint fmt

build:
	@go build ./...

test:
	@go test ./...

help:
	@printf '  %-14s - %s\n' 'make build' 'Build all Go packages'
	@printf '  %-14s - %s\n' 'make fix' 'Apply lint and formatting fixes'
	@printf '  %-14s - %s\n' 'make test' 'Run Go tests'
	@printf '  %-14s - %s\n' 'make verify' 'Run the static checks'
