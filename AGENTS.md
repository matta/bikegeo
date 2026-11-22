# Agent Instructions & Verification Loop

This repository is a zero-dependency **Go** command-line application (`bikegeo`).

## Python

**Never run bare `python` or `python3`.** Run all Python through `uv`, including one-off scripts, inline heredocs, and file-editing helpers: `uv run python script.py` or `uv run python - <<'EOF' ... EOF`. Only if `command -v uv` finds nothing may you fall back to `python3`, and say so.

## Commit Messages

Write plain-language subjects (for example `Fix fork sag validation`). Conventional Commits prefixes (`feat:`, `fix(web):`, `chore!:`, ...) are banned.

## Naming

- **Define semantics first:** before proposing names for a non-obvious concept, state in plain English what it is and does, and discuss conversationally before locking rulings.
- **Use real English and domain meanings:** never repurpose an existing word with a non-standard meaning (for example, calling discrete part options "stock" in cycling).
- **Don't force one prefix across every layer:** name each role for what it represents, and omit adjectives when plain prose is already clear.
- **Avoid vague or architecture-leaking names:** confine generic terms (`Catalog`, `Candidate`) to their natural roles and don't bake incidental implementation choices into names.
- **Match part of speech to role:** functions are accurate verbs; URL query keys, state keys, and data fields are nouns (never verb+noun commands like `try_bars`).

## Verification

- After **every** code change: `go vet ./... && go test ./...`
- Before committing or finishing: `make pre-commit`. It runs `fix` (`go fix`, `go fmt`) and then `check` (vet, `boundary`, `exhaustive`, and Go tests; `deadcode` is temporarily disabled during the reboot and will be re-enabled at cutover). If `go fix` changes anything, re-run until it's clean.
- Markdown-only changes don't need `make pre-commit`.

## Reboot Tree (`cmd/bikegeo-reboot`, `internal/reboot/...`)

A clean-slate reboot is being built beside the existing app; see [docs/reboot/DESIGN.md](docs/reboot/DESIGN.md) and the Rolling-Wave workflow/prompt playbook in [docs/reboot/WORKFLOW.md](docs/reboot/WORKFLOW.md). For reboot code, `DESIGN.md` overrides **Architecture** and **Frame Datasets** below wherever they differ. Don't change the existing app during reboot work unless asked.

- **Workflow (`docs/reboot/WORKFLOW.md`):** reboot work uses a Rolling-Wave workflow (`DESIGN.md` = direction & invariants; `SLICE<n>.md` = just-in-time plan for the next slice only). When asked what's next, check `git status`, `docs/reboot/SLICE*.md`, and `DESIGN.md` Milestones per `WORKFLOW.md`, state the current phase, and recommend the model tier (Frontier for direction, slice planning, and review; lower-tier for planned slice implementation).

- **Web:** server-rendered HTML/SVG driven by plain `GET` forms and the URL query, with no JavaScript, framework, `package.json`, npm, or build step. `/` is the multi-frame comparison matrix and `/bike` is the single-bike inspector. Web tests are browserless Go (`httptest` + `goquery`).
- **Isolation (`make boundary`):** reboot code never imports legacy `bikegeo/...` packages, except in files named `*_parity_test.go` under `internal/reboot/...`. Legacy code never imports reboot packages. Angles use `pose.Deg`, not `model.Deg`.
- **Trig lives in `internal/reboot/pose`:** outside `pose`, non-test reboot code uses `pose`'s semantic operations for all trig, radian conversion, and geometric math. Inside `pose`, raw trig stays unexported. `Abs`, `IsNaN`, `IsInf`, and independent `math` in tests are fine. Enforced by code review.
- **Catalog:** reboot frames, forks, handlebars, seatposts, the SQlab 610, and default fit targets are Go values in `internal/reboot/catalog` (not `internal/frames`), validated by its go test (chart consistency, fork-list placement, seatpost bounds). Published chart values are never nudged.
- **Terminology:** **Frame** (`FrameSetup`) is the bare frame. **Frameset** (`FramesetPlacement`, `PlaceFrameset`) is the leveled frame + fork + lower cup + wheel contacts. **Bike** (`BikePlacement`, `PlaceBike`, `/bike`) is the full assembly. **Build** (`ScoreBuild`, `BuildScore`, `FitBuilds`, `BuildRanking`, `ScoredBuild`) is a complete realized/scored bike; **Part / Option** (`PartOptions`, `StemOption`, `ForkOption`) names discrete component inputs; **Catalog** belongs only to the `internal/reboot/catalog` data-provider layer; **`stock`** keeps its cycling meaning (factory-installed or uncut, e.g. `HandlebarSetup.StockWidthMm`) and is not used for discrete fitting options (Q100–Q102, Slice 13d). URL query keys are nouns (`bar_options`, `fork_options`, `post_insert`, `rail_offset`), never verb+noun commands. `chassis` is banned in reboot code and docs (`make boundary`). Scoring uses score words: the words `price`, `pricing`, `cost`, `cheap`, and `ledger` are banned in reboot files too, in any case and inside longer words such as `PricedBand` or `cheapest` (`make boundary`; DESIGN.md "Scoring words").

## One-off investigations

When no existing test, `/bike`, or `compare` answers a question about geometry, a frame chart, or fitting, answer it with a temporary Go test in the package that owns the data. For frame charts, that package is `internal/reboot/catalog`. Never write a Python or standalone Go reimplementation.

- Name the file `scratch_<topic>_test.go` and its tests `TestScratch…`. The file pattern is gitignored, so it can't be committed.
- Print results with `t.Logf`.
- Run it with `go test -run TestScratch -v ./internal/reboot/catalog`.
- Delete the file when you're done.
- Call the real placement and chart-consistency functions, and never restate geometry formulas. Published chart values stay untouched in catalog files; a temporary test may only change copies.
- If you write the same scratch helper twice, propose promoting it into a shared test helper.
- For a new frame chart, follow the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill; its basis-finding test is the template for chart questions.

## Rider Fit Provenance

Before analyzing Matt's personal bike fit, read [docs/RIDER_FIT.md](docs/RIDER_FIT.md). It records his confirmed saddle and seatpost, riding observations, and the idmatch transfer from his bike with an **unsagged Redshift post** to the test jig using the **same physical saddle**. Treat these owner-confirmed facts as established; do not ask him to repeat them. Keep reported measurements, model-derived sit-bone coordinates, and untested fit preferences distinct.

## Architecture

- **Zero runtime dependencies:** the binary uses only the standard library. `goquery` is allowed only in `_test.go` files. `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` must list only `bikegeo/...`. `exhaustive` and `deadcode` are `go.mod` tool dependencies and are never linked into the binary.
- **`exhaustive`:** every `switch` on an enum-like type names every constant, even when it has a `default`.
- **`deadcode`:** temporarily disabled in `make check` during the reboot (re-enabled at cutover; still runnable manually via `make deadcode`). When active, delete unreachable functions or move test-only helpers into `*_test.go`.
- **Data is Go structs:** frames (`internal/frames`), fork/stem catalogs (`internal/catalog`), ideal profiles (`internal/ideal/loader.go`), and the default baseline (`compare.DefaultComparisonState()`). There are no data or persistence files.
- **Interfaces** (see [docs/WEB.md](docs/WEB.md)): the web is for people and the CLI is for agents. A capability gets a surface only for a real audience; if both need it, share a builder that returns a `report.Document`. Builders set cell zones explicitly; never infer them from strings. The CLI prints Markdown only (no `--format`). `internal/web` and `internal/cli` never import each other. The web is strictly Web 1.0: `GET` forms, the URL query as the only state, no JavaScript, no cookies, and controls filled from the parsed query.
- **Scores are a points-lost ledger:** `Total = 100 − Σ points lost`. Each band is free inside its ideal range and linear outside it (`Cost` per unit, or `LimitPoints` at the hardware limit). There are no weights, floors, curves, clamping, rounding, or stepped bonuses, and totals may go below 0. The selected build (saddle rail, stem, spacers) is the cheapest on those bands (`canonicalizer.ApplyIdealCockpitTargets`). Express preferences as priced bands, never as tie-break rules. The cockpit target is explicit on `ideal.Profile`. `saddle_hb` and `saddle_hb_drop` are display-only.
- **Comparison columns** are `FrameID + Size + Fork (travel mm or "rigid") + Sag %`. `bikegeo compare` with no arguments uses the default baseline; `+spec` and `-spec`/`-col_num` edit it. `internal/canonicalizer` enforces each frame's `CadFork`/`Forks` limits.
- **Angles are `model.Deg`; lengths are `float64`.** Trig goes through `Deg.Angle()` and `model.DegOf`, never a hand-written `math.Pi / 180`.
- `bikegeo fit` may list tied candidates in a different order between runs. That's a known, pre-existing behavior.

## Frame Datasets

Legacy `internal/frames` is frozen: new frames go into `internal/reboot/catalog` only, following the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill (DESIGN.md Q114, Q119). Every value gathered for a frame, published or not, goes with its source into the frame's source record, `internal/reboot/catalog/frame_<id>.md`. Published chart numbers are never nudged, and a chart allowance must be necessary and give its reason.

## Testing

- CLI tests assert on cell values (`runCLIRows`), not on padding.
- Web tests are browserless: `httptest` (set `req.Host = "127.0.0.1:8080"`) → `web.New().ServeHTTP` → `goquery` selectors. No Playwright or Selenium.

## Output Formatting

No LaTeX or MathJax; the chat UI doesn't render it. Write formulas in plain text or inline code, with Unicode symbols (`±1.0 mm`, `70.0°`).
