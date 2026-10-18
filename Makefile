GO ?= go
NODE ?= node
GOFLAGS ?=
REFERENCE_JS ?= .cache/L0.js

.PHONY: help test test-cases test-reference test-interpreter-reference build build-compiler build-interpreter

help:
	@echo make test           - build and test the Go interpreter
	@echo make test-cases     - validate the checked-in test fixtures
	@echo make test-reference - compare fixtures with the original L0.js
	@echo make build          - build main.exe from ./cmd/sm
	@echo make build-compiler - build compiler.exe from ./cmd/compiler
	@echo make build-interpreter - build interpreter.exe from ./cmd/interpreter
	@echo make test-interpreter-reference - check eval.html fixtures using REFERENCE_JS

test:
	$(GO) test $(GOFLAGS) -count=1 -timeout=120s ./...

test-cases:
	$(GO) test $(GOFLAGS) -count=1 ./tests -run "^TestFixtures$$"

test-reference:
	$(NODE) scripts/reference.cjs --check
	$(NODE) scripts/fixture-files.cjs --check

build:
	$(GO) build $(GOFLAGS) -o main.exe ./cmd/sm

build-compiler:
	$(GO) build $(GOFLAGS) -o compiler.exe ./cmd/compiler

build-interpreter:
	$(GO) build $(GOFLAGS) -o interpreter.exe ./cmd/interpreter

test-interpreter-reference:
	$(NODE) scripts/interpreter-reference.cjs --check "$(REFERENCE_JS)"
