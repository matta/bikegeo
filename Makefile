.PHONY: all generate vet test check build run clean test-e2e test-all

# Default PATH setup to include user go bin if present
export PATH := $(HOME)/go/bin:$(PATH)

all: check build

generate:
	templ generate

vet:
	go vet ./...

test:
	go test -v ./...

check: generate vet test

test-e2e:
	npx playwright test

test-all: check test-e2e

build: generate
	go build -o bin/bikegeo ./cmd/server

run: build
	./bin/bikegeo

clean:
	rm -rf bin/ tmp/ playwright-report/ test-results/
