# Agent Instructions & Verification Loop

This repository is a zero-dependency **Go** command-line application (`bikegeo`).

## Python

**Never run bare `python` or `python3`.** Run all Python through `uv`, including one-off scripts, inline heredocs, and file-editing helpers: `uv run python script.py` or `uv run python - <<'EOF' ... EOF`. Only if `command -v uv` finds nothing may you fall back to `python3`, and say so.

## Commit Messages

Write plain-language subjects (for example `Fix fork sag validation`). Conventional Commits prefixes (`feat:`, `fix(web):`, `chore!:`, ...) are banned.

## Naming & Comments

- **Define semantics first:** before proposing names for a non-obvious concept, state in plain English what it is and does, and discuss conversationally before locking in names.
- **Use real English and domain meanings:** never repurpose an existing word with a non-standard meaning (for example, calling discrete part options "stock" in cycling).
- **Don't force one prefix across every layer:** name each role for what it represents, and omit adjectives when plain prose is already clear.
- **Avoid vague or architecture-leaking names:** confine generic terms (`Catalog`, `Candidate`) to their natural roles and don't bake incidental implementation choices into names.
- **Match part of speech to role:** functions are accurate verbs; URL query keys, state keys, and data fields are nouns (never verb+noun commands like `try_bars`).
- **Use descriptive names in math and specs too:** prefer `brpToHbOffset`, `coupledHbTarget`, `HB_dynamic`, and `forkSagPitchDelta` over single-letter placeholders (`V`) or cryptic abbreviations (`HB_dyn`, `pitch_source`).
- **Explain rationale in plain English, never cite `Q#` or `Slice #`:** code comments and docstrings should state physical or engineering reasons directly so a file is readable in isolation without cross-referencing decision logs.
- **Distinguish `world`, `coordinate system`, `direction`, `orientation`, and `axis`:**
  - **World:** a bike-wide coordinate system with BB at `(0, 0)`, `+X` forward, and `+Y` up. Worlds differ only in the bike's state (`UnpitchedChart`, `StaticWorld`, `DynamicWorld`). Convert between coordinate systems with a `pose.Transform` rather than mixing coordinates across systems.
  - **Coordinate system (`pose.CoordinateSystem`):** a specific 2D coordinate system with both a fixed datum origin `(0, 0)` and `(+X, +Y)` axes (for example `FrameLocal`, `SeatpostLocal`, `SaddleLocal`, `DynamicWorld`). Reserve `Frame` and `FrameID` for the bicycle frame.
  - **Direction and orientation:** a *direction* is which way something points or moves, and its sign matters (up the steerer vs. down the steerer). An *orientation* is how a part is turned (set by its local `+X` direction). A **heading** (`pose.Heading`) is a direction given as an angle in degrees in a specific coordinate system.
  - **Axis:** a line through a particular point, such as the seat axis through BB or an oval's major axis through its center. When no position is involved, say direction or orientation instead.
- **Document normalization and origin semantics explicitly:** state in comments, types, or docstrings whether a vector is unit-length (for example `pose.Along(h, 1)` vs. a millimeter displacement `pose.Vector`) and what datum origin each `CoordinateSystem`, `Pose`, `Transform`, or axis uses (`BB = (0, 0)`, shaft bottom, `RailCenter`, `p.Origin()`, or target center).

## Geometry & Technical Exposition

- **Name the coordinate system before combining vectors:** a component displacement (insertion, setback, seatpost sag) lives in that component's invariant local coordinate system (e.g., `SeatpostLocal`), never world coordinates. State the rigid `pose.Transform` between a component's local coordinate system and the world before combining points or vectors.
- **Use `pose` primitives (`Point`, `Vector`, `Heading`, `Pose`, `Transform`):** `pose` uses concrete structs carrying deterministic runtime `pose.CoordinateSystem` checks (no phantom generic type parameters). Express 2D planar placements as fluent turtle steps (`p.Turn(...).Advance(...)`) and rigid `pose.Compose` transforms rather than ad-hoc trig (`90° − STA`). Reserve `TranslateLocal(dx, dy)` or local `pose.Point` offsets for coupled 2D displacements (such as `(ReachMm, StackMm)`) and 3D-to-2D projections (such as handlebar grip offsets).
- **Keep clamped joints rigid across load states:** a built bike stores clamped physical joint angles (relative turns between mating parts) so placing it in `StaticWorld` and `DynamicWorld` runs the same forward `pose.Compose` chain. Setting saddle rail pitch or bar roll against world horizontal is a clamping/fitting step in `DynamicWorld`, never a gyroscope that re-levels on unload.
- **Define variables in plain English first:** state what each variable represents, its load state (`Static` vs. `Dynamic`), and its coordinate system before writing formulas. Drop tables and ceremony when direct prose and turtle steps say it better.

## Idiomatic Go & Encapsulation

Follow the generics, interface, and encapsulation guidelines in [docs/WRITE_CODE.md](docs/WRITE_CODE.md):

- **Packages are the true sealing boundary — respect constructors even within a package:** in Go, unexported fields are private to the *package*, not the *file*. Align package boundaries with real invariant layers (`pose` → `bounds` → `parts` → `assembly` → `scoring`) so outer layers cannot scrape inner private fields. Even inside the same package, construct and query collaborators through their constructors and methods rather than literal-initializing structs or reading unexported fields across files.
- **Consumer-side thin interfaces by default, with the standard-library exception for polymorphic part families:**
  - *Single concrete role (`Frame`, `Saddle`, `Stem`, `Handlebar`, `Frameset`, `SeatedFrameset`, `Build`):* export the concrete struct with zero producer interface. Consumers that abstract over orthogonal behaviors (such as `placedNode` in `web/diagram.go`) define **1–3 method** interfaces right next to the caller.
  - *Polymorphic part family (`RigidPost` / `LinkagePost`, `RigidFork` / `SuspensionFork` / `BasisFork`):* when callers across multiple packages must store and pass interchangeable implementations of a physical part role, export one minimal **2–3 method** domain interface (`parts.Seatpost`, `parts.Fork`) from `parts` (like `io.Reader` or `fs.File`) rather than duplicating identical interface definitions across `catalog`, `compare`, and `assembly`. Keep non-physical concerns (scoring weights, SVG styling) out of that core interface.
- **Delegate, don't type-switch ("Tell, Don't Ask"):** never accept an interface and then `switch x := v.(type)` on its concrete implementations to extract fields. Either delegate to a behavioral method on the implementations, or move the concern to a consumer-side interface in the layer that owns it. Let joints own their local movement vectors (`SlideJoint`) and parts own their local millimeter sketches (`Sketch`) so solvers and renderers project them without re-deriving part math externally.
- **Avoid phantom generics:** reserve Go type parameters for language containers, general-purpose data structures, or eliminating identical duplicated functions. Prefer concrete structs with deterministic runtime `CoordinateSystem` assertions over type-level phantom tags.
- **"Write code" means clear domain objects — neither Rust-in-Go nor flat procedural bags:** avoid phantom type parameters, sum-type visitor boilerplate, and untyped string-keyed scene-graph frameworks when concrete structs and methods express the physics directly. Conversely, never collapse sealed objects back into shadow `*Setup` property bags (`BikeSetup`). Invariant-carrying domain objects (`pose.Point`/`Pose`/`Transform`, `bounds.BoundedRange`/`SlideJoint`, `parts.*`, `Frameset`, `SeatedFrameset`, `Build`) use **unexported fields** constructible only through validating constructors and mating functions; readout snapshots (`Placement`, `FittedSettings`, `BoundStatus`, `BuildScore`, `Fitting`, CLI result structs) use **exported fields** carrying placed results, bound statuses, and compact query settings.

## Verification

- After **every** code change: `go vet ./... && go test ./...`
- Before committing or finishing: `make pre-commit`. It runs `fix` (`go fix`, `go fmt`) and then `check` (vet, `boundary`, `exhaustive`, and Go tests; not `deadcode`, which runs only by hand). If `go fix` changes anything, re-run until it's clean.
- Markdown-only changes don't need `make pre-commit`.

## Workflow

Work uses a Rolling-Wave workflow described in [docs/WORKFLOW.md](docs/WORKFLOW.md): [docs/DESIGN.md](docs/DESIGN.md) holds the present-tense normative specification and Active Roadmap; [docs/fit-audit/README.md](docs/fit-audit/README.md) tracks the fit audit and physical Fenrir verification stages; `docs/SLICE<n>.md` (or the active stage in `docs/implementation_plan.md`, whose final stage deletes `docs/implementation_plan.md`) is the just-in-time plan for the next slice only. When asked what's next, check `git status`, `docs/SLICE*.md`, `docs/implementation_plan.md`, `docs/fit-audit/README.md`, and the `DESIGN.md` Active Roadmap per `WORKFLOW.md`, state the current phase, and recommend the model tier (Frontier for direction, audit stages, slice planning, and review; lower-tier for planned slice implementation).

## Architecture

- **Zero runtime dependencies:** the binary uses only the standard library. `goquery` is allowed only in `_test.go` files. `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` must list only `github.com/matta/bikegeo/...` (`make boundary`). `exhaustive` and `deadcode` are `go.mod` tool dependencies and are never linked into the binary.
- **Web:** server-rendered HTML/SVG driven by plain `GET` forms and the URL query, with no JavaScript, framework, `package.json`, npm, or build step, no cookies, and controls filled from the parsed query. `/` is the multi-frame comparison matrix and `/bike` is the single-bike inspector. Web tests are browserless Go (`httptest` + `goquery`).
- **CLI:** the web is for people and the CLI is for agents. `bikegeo compare` and `bikegeo bike` take URL-query arguments, parse them through `internal/query`, and print JSON. The CLI never depends on `internal/web`; both call `internal/query`, `internal/compare`, `internal/scoring`, and `internal/assembly` directly.
- **Data is Go structs:** frames, forks, handlebars, seatposts, stems, the SQlab 610 saddle, and default fit targets are Go values in `internal/catalog`, validated by its `go test` (chart consistency, fork-list placement, seatpost bounds). There are no data or persistence files. Published chart values are never nudged.
- **Angles use `pose.Deg`; lengths are `float64`.** Trig lives in `internal/pose`: outside `pose`, non-test code uses `pose`'s semantic operations for all trig, radian conversion, and geometric math. Inside `pose`, raw trig stays unexported. `Abs`, `IsNaN`, `IsInf`, and independent `math` in tests are fine. Enforced by code review.
- **Terminology:** **Frame** is the bare frame. **Frameset** is the mated frame + lower cup + fork + wheels. **SeatedFrameset** is a frameset with mated seatpost and saddle. **Build** is a complete mated, clamped bike (`Place(Static)` / `Place(Dynamic)` produce `Placement` readouts; `ScoreBuild`, `BuildScore`, `FitBuilds`, `BuildRanking`, `ScoredBuild` score builds). **Part / Option** (`PartOptions`, `StemOption`, `ForkOption`) names discrete component inputs; **Catalog** belongs only to the `internal/catalog` data-provider layer; **`stock`** keeps its cycling meaning (factory-installed or uncut, e.g. `StockWidthMm`) and is not used for discrete fitting options. URL query keys are nouns (`bar_options`, `fork_options`, `post_insert`, `rail_offset`), never verb+noun commands. `chassis` and `cad` are banned in code and docs (`make boundary`). Scoring uses score words: the words `price`, `pricing`, `cost`, `cheap`, and `ledger` are banned in any case and inside longer words such as `PricedBand` or `cheapest` (`make boundary`).
- **`exhaustive`:** every `switch` on an enum-like type names every constant, even when it has a `default`.
- **`deadcode`:** not part of `make check` or `make pre-commit`. Run `make deadcode` by hand, periodically; its findings don't block a commit. When you choose to clean one up, delete the unreachable function or move a test-only helper into a `*_test.go` file.

## Frame Datasets

New frames go into `internal/catalog`, following the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill. Every value gathered for a frame, published or not, goes with its source into the frame's source record, `internal/catalog/frame_<id>.md`. Published chart numbers are never nudged, and a chart allowance must be necessary and give its reason.

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
