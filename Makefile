.PHONY: all fix fmt vet boundary exhaustive deadcode test check pre-commit bench profile build serve watch clean

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

fix:
	go fix ./...

fmt:
	go fmt ./...

vet:
	go fmt ./...
	go vet ./...

# Enforce the zero-dependency rule and the banned words.
boundary:
	@bad="$$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo \
	  | grep -vE '^github\.com/matta/bikegeo/(cmd/bikegeo|internal)(/|$$)' || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: cmd/bikegeo must depend only on bikegeo packages and the standard library"; \
	  exit 1; \
	fi
	@bad="$$({ \
	  find cmd internal -iname '*chassis*'; \
	  find cmd internal -type f -exec grep -Hin 'chassis' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: files under cmd and internal must not use the banned term 'chassis' (use frame, frameset, or bike)"; \
	  exit 1; \
	fi
	@bad="$$({ \
	  find cmd internal \( -iname '*price*' -o -iname '*pricing*' -o -iname '*cost*' -o -iname '*cheap*' -o -iname '*ledger*' \) -print; \
	  find cmd internal -type f -exec grep -HinIE 'price|pricing|cost|cheap|ledger' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: files under cmd and internal must not contain the words price, pricing, cost, cheap, or ledger, in any case or inside a longer word (PricedBand, CostPerR, cheapest); say score, penalty, points per unit, or best-scoring"; \
	  exit 1; \
	fi
	@bad="$$({ \
	  find cmd internal -iname '*cad*'; \
	  find cmd internal -type f -exec grep -HinI 'cad' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: files under cmd and internal must not use 'CAD'; say basis (Q121)"; \
	  exit 1; \
	fi

# Every switch on an enum-like type must list all of its constants, even when it
# has a default case. The linter is a `tool` dependency in go.mod, so it is
# built with the pinned Go version and never linked into the bikegeo binary.
exhaustive:
	go tool exhaustive ./...

# Fails when a function is unreachable from main (plain deadcode) or from main
# and tests (-test). Running both ensures test-only helpers stay in *_test.go
# files and neither production code nor test helpers rot. Like exhaustive,
# deadcode is a `tool` dependency in go.mod and is never linked into the binary.
deadcode:
	@out="$$(go tool deadcode ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; echo "deadcode: unreachable functions from main found"; exit 1; fi
	@out="$$(go tool deadcode -test ./...)"; \
	if [ -n "$$out" ]; then echo "$$out"; echo "deadcode -test: unreachable functions found"; exit 1; fi

test:
	go test -v ./...

# `deadcode` is deliberately not part of `check` (Q144): run `make deadcode` by
# hand, periodically. Its findings don't block a commit.
check: vet boundary exhaustive test

# Required pre-commit workflow: modernize with go fix, then run full check.
pre-commit: fix check

watch:
	watchexec -c -f '*.go' -f 'go.*' -f '*.json' -f '*.s' -f '*.[ch]' "go vet ./... && go test ./..."

# Benchmarks are not part of `check`. `make bench` runs every benchmark;
# `make profile` runs one benchmark (BENCH, a -bench pattern) with CPU and
# memory profiles under tmp/profile/ (gitignored) for `go tool pprof`.
BENCH ?= BenchmarkComparePage/^selected_bar$$
PROFILE_PKG ?= ./internal/web
PROFILE_DIR := tmp/profile

bench:
	go test -run '^$$' -bench . -benchmem ./...

profile:
	mkdir -p $(PROFILE_DIR)
	go test -run '^$$' -bench '$(BENCH)' -benchtime=1x -benchmem \
	  -cpuprofile $(PROFILE_DIR)/cpu.pprof -memprofile $(PROFILE_DIR)/mem.pprof \
	  -o $(PROFILE_DIR)/bench.test $(PROFILE_PKG)
	@echo "go tool pprof -http=: $(PROFILE_DIR)/bench.test $(PROFILE_DIR)/cpu.pprof"
	@echo "go tool pprof -http=: -sample_index=alloc_space $(PROFILE_DIR)/bench.test $(PROFILE_DIR)/mem.pprof"

build:
	go build -o bin/bikegeo ./cmd/bikegeo

serve: build
	./bin/bikegeo serve --port 8080 --open

clean:
	rm -rf bin/
