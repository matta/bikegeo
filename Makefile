.PHONY: all fix fmt vet exhaustive deadcode test check pre-commit build run serve watch watch-serve clean

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

fix:
	go fix ./...

fmt:
	go fmt ./...

vet:
	go fmt ./...
	go vet ./...

# Every switch on an enum-like type must list all of its constants, even when it
# has a default case. The linter is a `tool` dependency in go.mod, so it is
# built with the pinned Go version and never linked into the bikegeo binary.
exhaustive:
	go tool exhaustive ./...

# Fails when a function is unreachable from main or from any test. -test makes
# tests roots, so code that only tests exercise is kept; delete a function that
# nothing needs, or move it into the test that does. Like exhaustive, deadcode
# is a `tool` dependency in go.mod and is never linked into the bikegeo binary.
deadcode:
	@out="$$(go tool deadcode -test ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; echo "deadcode: unreachable functions found"; exit 1; fi

test:
	go test -v ./...

check: vet exhaustive deadcode test

# Required pre-commit workflow: modernize with go fix, then run full check.
pre-commit: fix check

watch:
	watchexec -c -f '*.go' -f 'go.*' -f '*.json' -f '*.s' -f '*.[ch]' "go vet ./... && go test ./..."

build:
	go build -o bin/bikegeo ./cmd/bikegeo

run: build
	./bin/bikegeo compare

serve: build
	./bin/bikegeo serve --open

watch-serve:
	watchexec -r -e go,html,css -- go run ./cmd/bikegeo serve

clean:
	rm -rf bin/
