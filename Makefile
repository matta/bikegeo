.PHONY: all fmt vet test check build run clean

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test -v ./...

check: fmt vet test

build:
	go build -o bin/bikegeo ./cmd/bikegeo

run: build
	./bin/bikegeo compare

clean:
	rm -rf bin/
