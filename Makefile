GO ?= go
NODE ?= node
GOFLAGS ?=

.PHONY: help test test-cases test-reference build

help:
	@echo make test           - build and test the Go interpreter
	@echo make test-cases     - validate the checked-in test fixtures
	@echo make test-reference - compare fixtures with the original L0.js
	@echo make build          - build main.exe from ./cmd/sm

test:
	$(GO) test $(GOFLAGS) -count=1 -timeout=120s ./...

test-cases:
	$(GO) test $(GOFLAGS) -count=1 ./tests -run "^TestFixtures$$"

test-reference:
	$(NODE) scripts/reference.cjs --check
	$(NODE) scripts/fixture-files.cjs --check

build:
	$(GO) build $(GOFLAGS) -o main.exe ./cmd/sm
