.PHONY: all fmt vet test check build run serve clean watch

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

fmt:
	go fmt ./...

vet:
	go fmt ./...
	go vet ./...

test:
	go test -v ./...

check: vet test

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
