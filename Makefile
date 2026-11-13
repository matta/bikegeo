.PHONY: all fmt vet exhaustive test check build run serve clean watch

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

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

test:
	go test -v ./...

check: vet exhaustive test

watch:
	watchexec -c -f '*.go' -f 'go.*' -f '*.json' -f '*.s' -f '*.[ch]' "go vet ./... && go test ./..."

build:
	go build -o bin/bikegeo ./cmd/bikegeo

run: build
	./bin/bikegeo compare

serve: build
	./bin/bikegeo serve --open

clean:
	rm -rf bin/
