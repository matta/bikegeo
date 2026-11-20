.PHONY: all fix fmt vet boundary exhaustive deadcode test check pre-commit build build-reboot run serve serve-reboot watch watch-serve clean

export PATH := $(HOME)/go/bin:$(PATH)

all: check build

fix:
	go fix ./...

fmt:
	go fmt ./...

vet:
	go fmt ./...
	go vet ./...

# Enforce strict import isolation between the legacy tree and internal/reboot/.
# Only *_parity_test.go files under internal/reboot/ may import legacy packages.
boundary:
	@bad="$$({ \
	  find cmd/bikegeo-reboot -type f -name '*.go' -exec grep -HnE '"bikegeo/' {} +; \
	  find internal/reboot -type f -name '*.go' ! -name '*_parity_test.go' -exec grep -HnE '"bikegeo/' {} +; \
	} | grep -vE '"bikegeo/(internal/reboot|cmd/bikegeo-reboot)(/|")' || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: reboot non-parity files must not import legacy bikegeo packages"; \
	  exit 1; \
	fi
	@bad="$$(find cmd/bikegeo internal -path internal/reboot -prune -o -type f -name '*.go' \
	  -exec grep -HnE '"bikegeo/(internal/reboot|cmd/bikegeo-reboot)(/|")' {} + || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: legacy packages must not import reboot packages"; \
	  exit 1; \
	fi
	@bad="$$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo-reboot \
	  | grep -vE '^bikegeo/(cmd/bikegeo-reboot|internal/reboot)(/|$$)' || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: cmd/bikegeo-reboot must not depend on legacy or third-party packages"; \
	  exit 1; \
	fi
	@bad="$$(go list -deps ./cmd/bikegeo | grep -E '^bikegeo/(cmd/bikegeo-reboot|internal/reboot)(/|$$)' || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: cmd/bikegeo must not depend on reboot packages"; \
	  exit 1; \
	fi
	@bad="$$({ \
	  find cmd/bikegeo-reboot internal/reboot -iname '*chassis*'; \
	  find cmd/bikegeo-reboot internal/reboot -type f -exec grep -Hin 'chassis' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: reboot files must not use the banned term 'chassis' (use frame, frameset, or bike)"; \
	  exit 1; \
	fi
	@bad="$$({ \
	  find cmd/bikegeo-reboot internal/reboot \( -iname '*price*' -o -iname '*pricing*' -o -iname '*cost*' -o -iname '*cheap*' -o -iname '*ledger*' \) -print; \
	  find cmd/bikegeo-reboot internal/reboot -type f -exec grep -HinIE 'price|pricing|cost|cheap|ledger' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: reboot files must not contain the words price, pricing, cost, cheap, or ledger, in any case or inside a longer word (PricedBand, CostPerR, cheapest); say score, penalty, points per unit, or best-scoring"; \
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

# Temporarily omit `deadcode` during the reboot so slices can land foundational
# functions before every caller exists; re-enable in `check` at cutover.
check: vet boundary exhaustive test

# Required pre-commit workflow: modernize with go fix, then run full check.
pre-commit: fix check

watch:
	watchexec -c -f '*.go' -f 'go.*' -f '*.json' -f '*.s' -f '*.[ch]' "go vet ./... && go test ./..."

build:
	go build -o bin/bikegeo ./cmd/bikegeo

build-reboot:
	go build -o bin/bikegeo-reboot ./cmd/bikegeo-reboot

run: build
	./bin/bikegeo compare

serve: build
	./bin/bikegeo serve --open

serve-reboot: build-reboot
	./bin/bikegeo-reboot serve --port 8081 --open

watch-serve:
	watchexec -r -e go,html,css -- go run ./cmd/bikegeo serve

clean:
	rm -rf bin/
