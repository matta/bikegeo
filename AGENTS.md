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
- **Use descriptive names in math and specs too:** prefer `brpToHbOffset`, `coupledHbTarget`, `HB_dynamic`, and `forkSagPitchDelta` over single-letter placeholders (`V`) or cryptic abbreviations (`HB_dyn`, `pitch_source`).
- **Distinguish `world`, `direction`, `orientation`, `local coordinates`, and `axis`:**
  - **World:** a bike-wide coordinate system with BB at `(0, 0)`. Worlds differ only in the bike's state (as charted, static, or dynamic). Name a world for that state (`StaticWorld`, `DynamicWorld`), and convert between coordinate systems with a transform rather than mixing their values.
  - **Direction and orientation (`Dir`):** use both words as plain English does. A *direction* is which way something points or moves, and its sign matters: up the steerer and down the steerer are different directions. An *orientation* is how a part is turned. In this 2D model a part's orientation is set by the direction its local +X points, so code uses one `Dir` type for both. Name a `Dir` for that direction (`SteererDownDir`, `PlumbDownDir`). An **axis** is the unsigned line a direction runs along (see below), and a **heading** is a direction given as an angle in degrees.
  - **Local coordinates:** a part's own coordinate system: its orientation's +X and +Y, measured from a landmark on that part. Local offsets such as seatpost setback are given in local coordinates. Say **coordinate frame** in full if you need a noun; bare "frame" means the bicycle frame.
  - **Axis:** a line through a particular point, such as the seat axis through BB or an oval's major axis through its center. When no position is involved, say direction or orientation instead.
- **Document normalization and origin semantics explicitly:** state in comments, types, or docstrings whether a direction vector is unit-length (for example, `pose.Along(h, 1)` vs. a millimeter displacement `Vec`) and what origin a coordinate system, `Pose`, `Transform`, or axis uses (`BB = (0, 0)`, `p.Origin()`, or target center), rather than relying on naming alone.

## Geometry & Technical Exposition

- **Name the coordinate system before combining vectors:** a component displacement (insertion, setback, seatpost sag) lives in that component's invariant local coordinate system, never the bike's world coordinates (`BB = (0, 0)`, `+X` forward, `+Y` up). A component's local coordinates never rotate relative to the component; state the rigid transformation between the component's local coordinates and the world before combining points or vectors.
- **Specify transforms with `pose.Pose` (turtle graphics):** express 2D planar placements and transforms in specs and code as turtle steps (`Turn`, `Advance`, `Face`) rather than ad-hoc trig (`90° − STA`), turning back when preserving the parent orientation (e.g., seatpost setback). Reserve `Translate(Vec[Dir])` for offsets that don't reduce to a planar turn and advance: coupled 2D local displacements (e.g., `(ReachMm, StackMm)`) and 3D-to-2D projections (e.g., `PlaceHandlebar`).
- **Check signs and state transitions:** if a part sags down and back (`static → dynamic`, `Translate(-6, +5)`), unloading (`dynamic → static`) or pre-positioning it moves up and forward (`Translate(+6, -5)`). Don't confuse fitting a candidate in the dynamic world (where `Face` levels the saddle and orients the bar after pitching the frame) with unloading one already-clamped build to the static state.
- **Define variables in plain English first:** state what each variable represents, its load state (`static` vs. `dynamic`), and its coordinate frame before writing formulas. Drop tables and ceremony when direct prose and turtle steps say it better.

## Verification

- After **every** code change: `go vet ./... && go test ./...`
- Before committing or finishing: `make pre-commit`. It runs `fix` (`go fix`, `go fmt`) and then `check` (vet, `boundary`, `exhaustive`, and Go tests; not `deadcode`, which runs only by hand). If `go fix` changes anything, re-run until it's clean.
- Markdown-only changes don't need `make pre-commit`.

## Workflow

Work uses a Rolling-Wave workflow described in [docs/WORKFLOW.md](docs/WORKFLOW.md): [docs/DESIGN.md](docs/DESIGN.md) holds direction, invariants, and confirmed `Q#` rulings; [docs/fit-audit/README.md](docs/fit-audit/README.md) tracks the fit audit and physical Fenrir verification stages; `docs/SLICE<n>.md` is the just-in-time plan for the next slice only. When asked what's next, check `git status`, `docs/SLICE*.md`, `docs/fit-audit/README.md`, and the `DESIGN.md` Milestones per `WORKFLOW.md`, state the current phase, and recommend the model tier (Frontier for direction, audit stages, slice planning, and review; lower-tier for planned slice implementation).

## Architecture

- **Zero runtime dependencies:** the binary uses only the standard library. `goquery` is allowed only in `_test.go` files. `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` must list only `github.com/matta/bikegeo/...` (`make boundary`). `exhaustive` and `deadcode` are `go.mod` tool dependencies and are never linked into the binary.
- **Web:** server-rendered HTML/SVG driven by plain `GET` forms and the URL query, with no JavaScript, framework, `package.json`, npm, or build step, no cookies, and controls filled from the parsed query. `/` is the multi-frame comparison matrix and `/bike` is the single-bike inspector. Web tests are browserless Go (`httptest` + `goquery`).
- **CLI:** the web is for people and the CLI is for agents. `bikegeo compare` and `bikegeo bike` take URL-query arguments, parse them through `internal/query`, and print JSON. The CLI never depends on `internal/web`; both call `internal/query`, `internal/compare`, and `internal/assembly` directly.
- **Data is Go structs:** frames, forks, handlebars, seatposts, stems, the SQlab 610 saddle, and default fit targets are Go values in `internal/catalog`, validated by its go test (chart consistency, fork-list placement, seatpost bounds). There are no data or persistence files. Published chart values are never nudged.
- **Angles use `pose.Deg`; lengths are `float64`.** Trig lives in `internal/pose`: outside `pose`, non-test code uses `pose`'s semantic operations for all trig, radian conversion, and geometric math. Inside `pose`, raw trig stays unexported. `Abs`, `IsNaN`, `IsInf`, and independent `math` in tests are fine. Enforced by code review.
- **Terminology:** **Frame** (`FrameSetup`) is the bare frame. **Frameset** (`FramesetPlacement`, `PlaceFrameset`) is the leveled frame + fork + lower cup + wheel contacts. **Bike** (`BikePlacement`, `PlaceBike`, `/bike`) is the full assembly. **Build** (`ScoreBuild`, `BuildScore`, `FitBuilds`, `BuildRanking`, `ScoredBuild`) is a complete realized/scored bike; **Part / Option** (`PartOptions`, `StemOption`, `ForkOption`) names discrete component inputs; **Catalog** belongs only to the `internal/catalog` data-provider layer; **`stock`** keeps its cycling meaning (factory-installed or uncut, e.g. `HandlebarSetup.StockWidthMm`) and is not used for discrete fitting options (Q100–Q102, Slice 13d). URL query keys are nouns (`bar_options`, `fork_options`, `post_insert`, `rail_offset`), never verb+noun commands. `chassis` is banned in code and docs (`make boundary`). Scoring uses score words: the words `price`, `pricing`, `cost`, `cheap`, and `ledger` are banned in any case and inside longer words such as `PricedBand` or `cheapest` (`make boundary`; DESIGN.md "Scoring words").
- **`exhaustive`:** every `switch` on an enum-like type names every constant, even when it has a `default`.
- **`deadcode`:** not part of `make check` or `make pre-commit` (Q144). Run `make deadcode` by hand, periodically; its findings don't block a commit. When you choose to clean one up, delete the unreachable function or move a test-only helper into a `*_test.go` file.

## Frame Datasets

New frames go into `internal/catalog`, following the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill (DESIGN.md Q114, Q119). Every value gathered for a frame, published or not, goes with its source into the frame's source record, `internal/catalog/frame_<id>.md`. Published chart numbers are never nudged, and a chart allowance must be necessary and give its reason.

## One-off investigations

When no existing test, `/bike`, or `compare` answers a question about geometry, a frame chart, or fitting, answer it with a temporary Go test in the package that owns the data. For frame charts, that package is `internal/catalog`. Never write a Python or standalone Go reimplementation.

- Name the file `scratch_<topic>_test.go` and its tests `TestScratch…`. The file pattern is gitignored, so it can't be committed.
- Print results with `t.Logf`.
- Run it with `go test -run TestScratch -v ./internal/catalog`.
- Delete the file when you're done.
- Call the real placement and chart-consistency functions, and never restate geometry formulas. Published chart values stay untouched in catalog files; a temporary test may only change copies.
- If you write the same scratch helper twice, propose promoting it into a shared test helper.
- For a new frame chart, follow the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill; its basis-finding test is the template for chart questions.

## Rider Fit Provenance

Before analyzing Matt's personal bike fit, read [docs/RIDER_FIT.md](docs/RIDER_FIT.md). It records his confirmed saddle and seatpost, riding observations, and the idmatch transfer from his bike with an **unsagged Redshift post** to the test jig using the **same physical saddle**. Treat these owner-confirmed facts as established; do not ask him to repeat them. Keep reported measurements, model-derived sit-bone coordinates, and untested fit preferences distinct.

## Testing

- Web tests are browserless: `httptest` (set `req.Host = "127.0.0.1:8080"`) → `web.New().ServeHTTP` → `goquery` selectors. No Playwright or Selenium.
- CLI tests assert on the JSON the `compare` and `bike` subcommands print, not on formatting.

## Output Formatting

No LaTeX or MathJax; the chat UI doesn't render it. Write formulas in plain text or inline code, with Unicode symbols (`±1.0 mm`, `70.0°`).
