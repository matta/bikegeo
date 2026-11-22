# Slice 19: Promote the reboot to `bikegeo`

Status: Planned 2026-11-22

**Goal:** the reboot becomes the only app under its real name. `internal/reboot/<pkg>` becomes `internal/<pkg>`, `cmd/bikegeo-reboot` becomes `cmd/bikegeo`, every user-visible `bikegeo-reboot` becomes `bikegeo`, the `make` targets lose their `-reboot` suffix, the default port becomes 8080, and CI runs `make check`. No behavior changes.

**Rulings:** Q147 (port 8080, and the rename table for pre-Slice-19 prose), Q144 (`make check` leaves out `deadcode`), Q115 and Slice 18a (legacy is gone, so import isolation has nothing left to isolate).

**Size:** larger (84 files move, 14 more change, plus Makefile, CI, the README, and docs). The mechanical work is fully specified below, so a lower-tier model can do it without design choices. Everything here was dry-run on 2026-11-22 in a throwaway worktree: both code commits passed `make pre-commit`, the rewritten `boundary` target was negative-tested, and the built binary printed the new name and port.

**Planner defaults (veto on review):**

- **Import isolation check goes away.** The old `boundary` check 1 ("reboot files import no legacy `bikegeo/...` package") has no legacy left. The zero-dependency check stays, repointed to `./cmd/bikegeo`, and the `chassis`, `price/pricing/cost/cheap/ledger`, and `cad` bans now scan `cmd` and `internal` (everything Go-side).
- **`make` targets:** `build-reboot` → `build`, `serve-reboot` → `serve` (the README already documents `make build` and `make serve`). The binary is `bin/bikegeo`.
- **`.claude/launch.json`:** the configuration is named `bikegeo` (it was `reboot`), on port 8080.
- **Docs get path fixes only, except the README.** `AGENTS.md`, `WORKFLOW.md`, and the ingest-frame skill get mechanical path and command fixes so no command breaks. The README is rewritten in full, because it describes only the deleted legacy app and its commands become real in this slice (Matt asked for it 2026-11-22; it moves here from Slice 20). The rest of the prose rewrite (the word "reboot", the isolation and precedence rules, the legacy sections) is Slice 20. DESIGN.md's rulings and milestones keep their wording as history (Q147's rename table); only its active layout sections change.
- **CI:** one `make check` step plus a formatting-clean check replaces the separate vet, exhaustive, and test steps. This is the only change that can't be run locally (see Verification).
- **Web tests' placeholder host** `127.0.0.1:8081` becomes `127.0.0.1:8080` so every mention of the port agrees (`req.Host` isn't read by any handler).
- **No file contents change in the move commit** except import paths and the Makefile and launch config paths, so git detects every file as a rename.

## Commit 1: `Move the reboot packages to internal and cmd/bikegeo`

Run from the repo root, in this order.

1. Move the packages and the command:

   ```bash
   git mv internal/reboot/assembly internal/assembly
   git mv internal/reboot/catalog internal/catalog
   git mv internal/reboot/compare internal/compare
   git mv internal/reboot/pose internal/pose
   git mv internal/reboot/query internal/query
   git mv internal/reboot/web internal/web
   git mv cmd/bikegeo-reboot cmd/bikegeo
   rmdir internal/reboot
   ```

   `rmdir` removes the now-empty directory and fails harmlessly if something (an ignored scratch file) is left in it; look at what remains before deleting anything.

2. Rewrite the import paths (111 import lines in 65 files):

   ```bash
   find cmd internal -name '*.go' -exec sed -i '' 's#bikegeo/internal/reboot/#bikegeo/internal/#g' {} +
   ```

   Afterwards `grep -rn 'internal/reboot' cmd internal --include='*.go'` finds exactly one line: a comment in `internal/assembly/fitting_test.go` (line 14) that names the deleted `web/fixture.go`. Commit 2 rewrites it; leave it here. Import order inside each block doesn't change because every path shares the same prefix.

3. **`Makefile`**, `boundary` target (the only part that must change for this commit to pass):
   - Replace the comment above it with `# Enforce the zero-dependency rule and the banned words.`
   - Delete the first `@bad=` block (the `find cmd/bikegeo-reboot … internal/reboot … grep -vE '"bikegeo/(internal/reboot|cmd/bikegeo-reboot)(/|")'` import-isolation check, 9 lines through its `fi`).
   - In the dependency check: `./cmd/bikegeo-reboot` → `./cmd/bikegeo`; the filter `grep -vE '^bikegeo/(cmd/bikegeo-reboot|internal/reboot)(/|$$)'` → `grep -vE '^bikegeo/(cmd/bikegeo|internal)(/|$$)'`; the message → `boundary: cmd/bikegeo must depend only on bikegeo packages and the standard library`.
   - In the three ban checks: every `cmd/bikegeo-reboot internal/reboot` → `cmd internal`, and every message prefix `boundary: reboot files must not` → `boundary: files under cmd and internal must not`. The ban patterns themselves don't change.
   - `build-reboot` target: `go build -o bin/bikegeo-reboot ./cmd/bikegeo-reboot` → `go build -o bin/bikegeo-reboot ./cmd/bikegeo` (path only; the name changes in commit 2).
4. **`.claude/launch.json`:** `./cmd/bikegeo-reboot` → `./cmd/bikegeo` in `runtimeArgs` (nothing else yet).
5. Run `make pre-commit` (repeat until `go fix` leaves nothing changed). `git status` must show only renames plus `Makefile` and `.claude/launch.json` modified, and `git diff -M --stat HEAD` must show no unrelated edits.

**Negative tests for the rewritten `boundary` (run once, then remove what you created):**

| Check | Input | Expected |
| --- | --- | --- |
| Banned words (`cost`) | create `internal/pose/neg.txt` containing `the cost of a frame` | `make boundary` exits non-zero, prints `internal/pose/neg.txt:1:the cost of a frame` and the `price, pricing, cost, cheap, or ledger` message |
| Zero dependencies | create `internal/pose/neg.go` containing `package pose` and `import _ "github.com/PuerkitoBio/goquery"` | `make boundary` exits non-zero, prints `github.com/PuerkitoBio/goquery` and the `cmd/bikegeo must depend only on bikegeo packages and the standard library` message |

With both files removed, `make boundary` exits 0. (The `chassis` and `cad` checks use the same shape as the `cost` one.)

## Commit 2: `Rename bikegeo-reboot to bikegeo`

1. Bulk string rename in Go and templates:

   ```bash
   find cmd internal \( -name '*.go' -o -name '*.html' \) -exec sed -i '' -e 's/bikegeo-reboot/bikegeo/g' -e 's/127\.0\.0\.1:8081/127.0.0.1:8080/g' {} +
   ```

2. Hand edits (exact text):
   - `cmd/bikegeo/main.go`: `fs.Int("port", 8081,` → `fs.Int("port", 8080,`.
   - `internal/web/web.go`: `// Package web serves the reboot HTML/SVG workbench.` → `// Package web serves the HTML/SVG workbench.`; `HTTP handler for the reboot workbench.` → `HTTP handler for the workbench.`
   - `internal/catalog/catalog.go`: `holds the reboot's frames,` → `holds the frames,`.
   - `internal/pose/pose.go`: `degree-valued angles for reboot` + next line `// component assembly.` → `degree-valued angles for` + `// component assembly.`
   - `internal/assembly/fitting_test.go`: `Numbers copied from internal/reboot/web/fixture.go; assembly tests` + `// must not import the web fixture.` → `Numbers copied from the web fixture it replaced; assembly tests` + `// must not import web code.`
   - `Makefile`: `.PHONY` drops `build-reboot serve-reboot` and gains `build serve`; `all: check build-reboot` → `all: check build`; `build-reboot:` / `go build -o bin/bikegeo-reboot ./cmd/bikegeo` → `build:` / `go build -o bin/bikegeo ./cmd/bikegeo`; `serve-reboot: build-reboot` / `./bin/bikegeo-reboot serve --port 8081 --open` → `serve: build` / `./bin/bikegeo serve --port 8080 --open`.
   - `.claude/launch.json`: `"name": "reboot"` → `"name": "bikegeo"`, and both `8081` → `8080`.
3. **Left alone on purpose:** `internal/catalog/stems.go` lines 52 and 132 name `docs/reboot/stems.yaml` (docs move in Slice 20), and the Markdown source records under `internal/catalog/` keep their wording.
4. `git diff --stat` must show 14 files changed, 58 insertions and 58 deletions (`.claude/launch.json` 6, `Makefile` 12, `cmd/bikegeo/main.go` 28, `main_test.go` 8, `internal/web/web_test.go` 28, `compare_page.go` 6, `compare_test.go` 6, `query/bike.go` 4, `bike.html` 4, `compare.html` 2, `web.go` 4, `fitting_test.go` 4, `catalog.go` 2, `pose.go` 2). A different count means a stray match; look before continuing.
5. `grep -rn -i 'reboot\|8081' cmd internal Makefile .claude/launch.json --include='*' | grep -v '\.md:'` finds only the two `stems.go` lines.
6. `make pre-commit`.

**Probe values** (what every changed readout must now read; the existing tests assert them after the rename):

| Readout | New value |
| --- | --- |
| `/` `<title>` | `Compare 18 Frames — bikegeo` (custom selections: `Compare <n> <noun> — bikegeo`) |
| `/` 400 page `<title>` | `Invalid Comparison — bikegeo` |
| `/bike` `<title>` | `bikegeo — Bike Workbench` |
| `<h1>` on both pages | `bikegeo` |
| `bikegeo` with no args (usage) | first lines `Usage:` then `  bikegeo serve [--port N] [--open]`, `  bikegeo compare [query]`, `  bikegeo bike [query]` |
| `bikegeo serve -h` | `-port int … (default 8080)` |
| serve banner | `bikegeo serving on http://127.0.0.1:<port>/ (Ctrl-C to stop)` |
| `compare` / `bike` wrong-arity error | `Usage: bikegeo compare [query]` / `Usage: bikegeo bike [query]` |

## Commit 3: `Run make check in CI, rewrite the README, and fix paths in the docs` (Markdown and YAML)

0. **`README.md`: replace the whole file with the text below** (added to this slice on Matt's request 2026-11-22; it was Slice 20's job, and it moves here because the README now describes `bin/bikegeo` and `make serve`, which are real after commit 2). The old README describes only the deleted legacy app (`list`, `validate`, `fit`, `+spec` columns, `--score-detail`, `docs/WEB.md`, `internal/frames`). Every command below was run against the current binary on 2026-11-22 (under its old name) and works; the counts the README avoids stating (frames, sizes, forks) are the ones Q124 keeps out of prose.

   ````markdown
   # bikegeo 🚲

   A zero-dependency Go application that compares bicycle frames the way a fitter does. It assembles each candidate bike physically (frame, fork, headset, stem, spacers, handlebar, seatpost, saddle), finds the best-fitting build for a rider's saddle and hand-position targets, and scores it. Two interfaces share one engine: a **local web app** (`bikegeo serve`) for people, and a **JSON command line** (`bikegeo compare`, `bikegeo bike`) for agents and scripts.

   Every frame, fork, handlebar, stem, seatpost, and saddle is compiled into Go structs under [`internal/catalog/`](internal/catalog), with no data or configuration files. Each frame has a source record (`frame_<id>.md`) that lists every value gathered and where it came from.

   ---

   ## What it does

   1. **Physical assembly.** A bike is built by walking its parts in order: frame, fork (with sag), headset cup, spacers, stem, handlebar, and seatpost and saddle. The result is each landmark's position, so every readout comes from one placement rather than from separate formulas.
   2. **Fitting to two targets.** The rider's targets are the saddle's **BRP** and the **HandGrip** point, both measured from the bottom bracket. For each frame, size, fork, and sag, the engine finds the best stem, spacers, and saddle position it can. Builds fall into four fit tiers: `fits_all`, `fits_brp`, `fits_grip`, and `fits_neither`.
   3. **0–100 scoring.** Each build gets a score from 12 weighted attributes: three fit attributes, plus ranges such as trail, fork sag, spacer stack, and seatpost insertion. Builds are ranked by tier first, then by each tier's own tie-breaks and the score (DESIGN.md, Q61, Q69, Q90).
   4. **Open fork choice.** Any catalog fork that is compatible with a frame can be tried, in every variant and at the sags its travel allows. A frame's listed forks come first.
   5. **Charts are checked.** Each frame's published geometry chart is checked against its chart basis fork in `go test`, so a transcription error fails the build instead of reaching the app.

   ---

   ## Quick start

   ```bash
   make build                           # builds bin/bikegeo
   make serve                           # build, serve on http://127.0.0.1:8080, open the browser
   ./bin/bikegeo serve --port 0 --open  # or choose a free port
   ```

   ### Web (for people)

   - `/` is the multi-frame comparison matrix: the best configuration of every frame (or a custom selection), ranked, with a leveled-geometry table and a collapsible score breakdown.
   - `/bike` is the single-bike inspector: the diagram, the fitting readouts, the submitted build's score, and the ranked builds.

   Everything is a plain `GET` URL that you can bookmark or share, for example `/?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20` and `/bike?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20`. The URL query is the only state, there is no JavaScript, and the server listens on `127.0.0.1` only.

   ### CLI (for agents and scripts)

   `compare` and `bike` take the same URL query as the web pages and print indented JSON. `--help` lists every query key.

   ```bash
   ./bin/bikegeo --help
   ./bin/bikegeo compare --help

   # Rank every frame (best row each), keep the top 3:
   ./bin/bikegeo compare 'top_per_frame=1&top=3'

   # One selection, then two frames across every handlebar:
   ./bin/bikegeo compare 'frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20'
   ./bin/bikegeo compare 'frame=surly-krampus&frame=otso-fenrir-stainless&bar_options=all&top=5'

   # Evaluate one bike, with or without fit targets:
   ./bin/bikegeo bike
   ./bin/bikegeo bike 'frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20'
   ./bin/bikegeo bike 'frame=binary-havok&size=L&brp=off&grip=off'

   # Pick fields out with jq:
   ./bin/bikegeo compare 'top_per_frame=1&top=3' | jq '[.Rows[] | {frame: .FrameID, size: .Size, fork: .ForkID, tier: .Build.Score.Tier, total: .Build.Score.Total}]'
   ```

   ---

   ## Adding a frame

   Follow the [ingest-frame skill](.agents/skills/ingest-frame/SKILL.md): record every value and its source first, transcribe the published chart, find its basis fork with a temporary Go test, and let the catalog's `go test` confirm the chart is consistent.

   ## Running verification and tests

   ```bash
   make check       # format, vet, banned-term and dependency checks, exhaustive enum check, and tests
   make pre-commit  # required before committing: runs go fix, then make check
   make deadcode    # unreachable-function report; run by hand, never a gate
   ```

   The design and its rulings are in [`docs/reboot/DESIGN.md`](docs/reboot/DESIGN.md), the workflow in [`docs/reboot/WORKFLOW.md`](docs/reboot/WORKFLOW.md), and the working rules for agents in [`AGENTS.md`](AGENTS.md).
   ````

   After writing it, check: `grep -n -i 'validate\|--score-detail\|internal/frames\|docs/WEB\|internal/ideal\|compare all' README.md` finds nothing; every relative link resolves (`ls` each path); `make build` then run each command in the README's CLI block and confirm each exits 0 (the `jq` line prints a 3-element array with `surly-krampus`, `sklar-pbj`, `seven-custom` first, tiers `fits_all`, `fits_all`, `fits_brp`).

1. **`.github/workflows/test.yml`:** keep checkout, Set up Go, and `go mod verify`. Replace the `Vet`, `Exhaustive enum switches`, and `Test` steps with two steps, in this order:

   ```yaml
       - name: Check
         run: make check

       - name: Formatting is clean
         run: git diff --exit-code
   ```

   (`make check` runs `go fmt`, `go vet`, `boundary`, `exhaustive`, and `go test -v`, and not `deadcode`, Q144. The `git diff` step fails if `go fmt` changed anything.)
2. **`AGENTS.md`, `docs/reboot/WORKFLOW.md`, `.agents/skills/ingest-frame/SKILL.md`:** replace `cmd/bikegeo-reboot` → `cmd/bikegeo`, then `internal/reboot/` → `internal/` and the bare `internal/reboot` → `internal`, so `./internal/reboot/catalog` becomes `./internal/catalog`, the skill's import example becomes `"bikegeo/internal/assembly"`, and the AGENTS heading becomes ``## Reboot Tree (`cmd/bikegeo`, `internal/...`)``. In the skill's live-check bullet, ``In the `reboot` preview`` → ``In the `bikegeo` preview``. Change nothing else in them (the word "reboot", the isolation and precedence bullets, and the legacy sections stay for Slice 20).
3. **`docs/reboot/DESIGN.md`**, active sections only (rulings and milestones stay as history, Q147):
   - Line 3: `A clean-slate reboot built beside the existing application under `cmd/bikegeo-reboot` and `internal/reboot/...`.` → `A clean-slate reboot, now the only app, under `cmd/bikegeo` and `internal/...` (promoted in Slice 19).`
   - "Run and inspect": `make serve-reboot` → `make serve`, `:8081` → `:8080`.
   - "Layout and boundaries": the layout block's paths and port (`cmd/bikegeo/`, `internal/pose/`, … `internal/web/`, port 8080), and `- `make boundary` enforces the import isolation and the banned-term rule described in AGENTS.md.` → `- `make boundary` enforces the zero-dependency rule and the banned-term rule described in AGENTS.md.`
   - The active rule "Catalog data is application input in `internal/reboot/catalog`." → `internal/catalog`.
4. Markdown-only edits need no `make pre-commit`, but run `go vet ./... && go test ./...` once at the end as a sanity check.

## Verification

1. `make pre-commit` after commits 1 and 2, each passing on its own; `go vet ./... && go test ./...` after commit 3.
2. `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` lists only `bikegeo/...` packages.
3. **Live check** (`preview_start` `bikegeo`, port 8080; if 8080 is already in use on the machine, say so and use `--port`): `/` is titled `Compare 18 Frames — bikegeo` and says `Showing 18 of 2889 configurations (2163861 builds evaluated)`; `/bike` is titled `bikegeo — Bike Workbench`; both `<h1>` read `bikegeo`; `/bike?frame=nope` is HTTP 400. Then `make build && ./bin/bikegeo compare | jq '.RowCount, (.Rows | length)'` prints `2889` and `18`, and `./bin/bikegeo` with no arguments prints the usage with `bikegeo` in it.
4. **CI can't be run locally.** `make check` is what the new step runs, and it passes locally. State that the workflow file itself is unverified until its first run after the push.

## Out of scope

- Slice 20: AGENTS.md prose consolidation (the word "reboot", the isolation, precedence, and legacy bullets), moving `DESIGN.md`, `WORKFLOW.md`, and `stems.yaml` out of `docs/reboot/` (and updating `stems.go`'s two comments), and the Reboot Tree heading's name.
- Any behavior, scoring, or URL change; `deadcode` cleanup (Q144); the Milestone 5 UI overhauls (Q116).
- Renaming the Go module (`bikegeo` already).

## Rules that bite

- Commit 1 changes no file contents except import paths, the `boundary` and build-path lines in `Makefile`, and one path in `.claude/launch.json`. If `git diff -M --stat` shows other edits, undo them.
- If a test fails after a commit, other than a mistake in the steps above, stop. The rename shouldn't change any expected value; mark the plan Blocked and report it. Don't adjust a test to make it pass.
- Don't use `rm -rf` or delete anything not named here. `rmdir internal/reboot` is the only removal, and the negative-test files you create are the only others.
- No Conventional Commits prefix. End each commit message with the attribution line.
- Finish with the **Implementation handoff** (DESIGN.md "Slice planning"): update this Status line, add the `## Implementation handoff` section, and set DESIGN.md's Slice 19 Milestones line to the same status words.
