.PHONY: all generate vet test check build run clean

# Default PATH setup to include linuxbrew and user go bin if present
export PATH := /home/linuxbrew/.linuxbrew/bin:$(HOME)/go/bin:$(PATH)

all: check build

generate:
	templ generate

vet:
	go vet ./...

test:
	go test -v ./...

check: generate vet test

build: generate
	go build -o bin/bikegeo ./cmd/server

run: build
	./bin/bikegeo

clean:
	rm -rf bin/
