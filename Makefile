GO ?= go
NODE ?= node
DOCKER ?= docker
GOFLAGS ?=
REFERENCE ?= docker
REFERENCE_IMAGE ?= dboulytchev/pl-platforms@sha256:14acc7b02e71c0a9c69e4bfe9a0aed57e8e2e7e3c0bc6b75131cd489fd66e1dd
REFERENCE_PLATFORM ?= linux/amd64
REFERENCE_JS ?= .cache/L0.js

override SOURCE := $(value SOURCE)
override AST := $(value AST)
override CODE := $(value CODE)
override INPUT := $(value INPUT)
override OUTPUT := $(value OUTPUT)
export SOURCE AST CODE INPUT OUTPUT REFERENCE REFERENCE_IMAGE REFERENCE_PLATFORM REFERENCE_JS DOCKER

.PHONY: help test test-cases test-reference test-interpreter-reference test-sm-reference test-workflow build build-compiler build-interpreter ast run compile run-sm reference-js

help:
	@echo 'make ast SOURCE=program.l0 [AST=program.json] - source -> reference -> JSON AST'
	@echo 'make run SOURCE=program.l0 INPUT=input.txt [OUTPUT=result.txt] - source -> reference -> interpreter'
	@echo 'make run AST=program.json INPUT=input.txt [OUTPUT=result.txt] - execute an existing AST'
	@echo 'make compile AST=program.json [CODE=code.json] - AST -> our compiler -> SM instructions'
	@echo 'make run-sm AST=program.json INPUT=input.txt [OUTPUT=result.txt] - AST -> our compiler -> SM execution'
	@echo 'compile and run-sm also accept SOURCE=program.l0 instead of AST, using the reference parser'
	@echo 'REFERENCE=docker (default) or REFERENCE=web REFERENCE_JS=/path/to/L0.js'
	@echo 'make reference-js - download the website bundle to .cache/L0.js'
	@echo 'make build-interpreter - build interpreter.exe'
	@echo 'make build-compiler - build compiler.exe (JSON AST -> SM instructions)'
	@echo 'make build - build the independent SM interpreter main.exe'
	@echo 'make test - run Go tests; make test-workflow - test the source/AST workflow without Docker/network'
	@echo 'make test-cases - validate SM fixtures; make test-reference - compare legacy SM fixtures'
	@echo 'make test-interpreter-reference REFERENCE_JS=/path/to/pinned/L0.js - check saved eval.html fixtures'
	@echo 'make test-sm-reference REFERENCE_JS=/path/to/pinned/L0.js - check saved sm.html fixtures'

test:
	$(GO) test $(GOFLAGS) -count=1 -timeout=120s ./...

test-workflow: build-interpreter build-compiler build
	$(NODE) --test scripts/reference-ast.test.cjs

test-cases:
	$(GO) test $(GOFLAGS) -count=1 ./tests -run "^TestFixtures$$"

test-reference:
	$(NODE) scripts/reference.cjs --check
	$(NODE) scripts/fixture-files.cjs --check

build:
	@$(GO) build $(GOFLAGS) -o main.exe ./cmd/sm

build-compiler:
	@$(GO) build $(GOFLAGS) -o compiler.exe ./cmd/compiler

build-interpreter:
	@$(GO) build $(GOFLAGS) -o interpreter.exe ./cmd/interpreter

ast:
	@$(NODE) scripts/reference-ast.cjs ast

run: build-interpreter
	@$(NODE) scripts/reference-ast.cjs run

compile: build-compiler
	@$(NODE) scripts/reference-ast.cjs compile

run-sm: build-compiler build
	@$(NODE) scripts/reference-ast.cjs run-sm

reference-js:
	@mkdir -p .cache
	@temporary=$$(mktemp .cache/L0.js.XXXXXX); \
	trap 'rm -f "$$temporary"' EXIT HUP INT TERM; \
	curl -fLsS https://dboulytchev.github.io/L0.js -o "$$temporary" && mv "$$temporary" .cache/L0.js

test-interpreter-reference:
	$(NODE) scripts/interpreter-reference.cjs --check "$(REFERENCE_JS)"

test-sm-reference:
	$(NODE) scripts/sm-reference.cjs --check "$(REFERENCE_JS)"
