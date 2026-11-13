# Agent Instructions & Verification Loop

This repository is a zero-dependency **Go** command-line application (`bikegeo`).

## Fast Feedback Command Loop

After **every** code modification, always execute the following command to verify type safety and test suite pass rate:

```bash
go vet ./... && go test ./...
```

## Final Verification & Cleanup

Formatting and modernization are not part of the fast feedback loop, but are part of the final cleanup stages of code modification. Before completing a task or submitting changes, apply the toolchain's automatic modernizations, format all Go source files, and verify the full check suite:

```bash
go fix ./... && go fmt ./... && make check
```

- `go fix` rewrites code to current language and library idioms (for example `new(expr)` for pointer literals, `strings.FieldsSeq`, `maps.Copy`). Run it with the Go version pinned in `go.mod`, and re-run it until it reports no further changes.
- Review its diff before committing. Its rewrites must be semantically neutral: if a change touches `pkg/frames/*.go`, confirm only idioms changed and no published geometry value did (see the chart-immutability rule below). Commit `go fix` output separately from feature work so it stays reviewable.
- If a rewrite produces an awkward result (such as an aliased import like `maps0` caused by a shadowing variable), fix the cause (rename the variable) rather than keeping the workaround.

## Architectural Guidelines

- **Zero Runtime Dependencies & Pure-Go Data**:
  - The shipped binary uses only the Go standard library (`flag`, `encoding/csv`, `net/http`, `html/template`, `embed`) with custom Unicode box-drawing, Markdown, and CSV table formatting in `pkg/cli/render.go`.
  - The single exception is `github.com/PuerkitoBio/goquery`, allowed **only** in `_test.go` files for DOM assertions. Never import it (or any third-party package) from non-test code; verify with `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo`, which must list only `bikegeo/...`.
  - `github.com/nishanths/exhaustive` is a `tool` dependency in `go.mod`, used only by `make check` (`go tool exhaustive ./...`). It is never linked into the binary. A `switch` on an enum-like type (`model.HeadsetCupType`, `model.PublishMode`, `ideal.ZoneType`, ...) must name every constant even when it has a `default`; when adding a constant, add its case everywhere the linter reports.
  - All bike frames (`pkg/frames/*.go`), fork and stem catalogs (`pkg/catalog/catalog.go`), ideal fit profiles (`pkg/ideal/loader.go`), and the default comparison baseline (`compare.DefaultComparisonState()` in `pkg/compare/specs.go`, also reachable as `compare.DefaultComparisonState()`) are compiled directly as Go structs. There are no JSON data files or runtime persistence files.
- **Each Interface Lives Where It Is Used** (see [docs/WEB.md](docs/WEB.md)):
  - The web (`bikegeo serve`) is for people; the CLI is for agents and scripts. A capability gets a surface only for a real audience, never by default.

    | Capability | CLI (agents) | Web (people) |
    | --- | --- | --- |
    | `compare`, `validate`, `list` | yes | yes (`/`, `/validate`, `/frames`) |
    | `fit` | yes | no |
    | forks, stems, ideals, saddles, saddle-swap, handlebars | no | yes |

  - Presentation lives in a **builder** that returns a `report.Document`: `compare.BuildDocument`, `validator.BuildValidationDocument`, `compare.BuildFrameListDocument`, `fit.BuildSweepDocument` / `fit.BuildAnalysisDocument`, and the web-only builders in `pkg/web` (`docb`). `pkg/report` holds the presentation-neutral `Document` (ordered `Sections` of tables and text), `Table` and `Cell` (cells carry `Zone` and `MetricID`). Builders set zones explicitly; never infer them from strings.
  - The CLI (`pkg/cli`) parses flags, calls a builder and prints Markdown (`App.printDocument`). There is no `--format`: Markdown is the only output. `compare` keeps a many-bike comparison as one wide table (`DocOptions.Flat`) so programs can parse it.
  - The web (`pkg/web`) turns the URL query into a builder call and renders HTML. `pkg/web` never imports `pkg/cli`; `pkg/cli` imports `pkg/web` only to dispatch `bikegeo serve`.
  - Do not add a CLI subcommand for a human-browsing feature, or a web page for an agent-only tool. If both audiences need it, share a builder.
  - The web is strictly Web 1.0: `GET` forms, URL query as the only state, full-page responses, no JavaScript, no cookies. Controls must be populated from the parsed query.
  - `bikegeo fit` can list tied candidate solutions in a varying order between runs; that is pre-existing.
- **Score Curves Live in `pkg/curve`**:
  - Every scoring shape (attribute falloffs, cockpit proximity, stem penalty, versatility bonus) is built from `pkg/curve`, which cannot express flat stretches or jumps: curves are continuous, strictly decreasing and approach their floor asymptotically, so two different failures never tie.
  - Never clamp, round, or add stepped (count-based or fixed) bonuses and penalties to a score in scoring code. If a new curve shape is needed, add it to `pkg/curve` with property tests.
- **Per-Column Comparison Model (`ComparisonSetup`)**:
  - Every comparison column specifies `FrameID + Size + Fork (travel in mm or "rigid") + Sag %`.
  - Allows comparing the same frame across multiple sag points or fork capacities alongside different frames.
- **Frame-Level Fork Constraints**:
  - Frames define `CadFork` and `Forks` (supported fork refs with `ID`, `A2cMm`, `RakeMm`, `SagPct`) in `pkg/frames/*.go`.
  - `pkg/canonicalizer` strictly enforces these constraints, rejecting unsupported travel sizes, rigid forks on suspension-only frames, or non-zero sag on rigid forks.
- **Default Comparison Baseline**:
  - Running `bikegeo compare` with no arguments evaluates `compare.DefaultComparisonState()`. Incremental `+spec` and `-spec` / `-col_num` flags add or remove columns against that baseline in memory.

## Adding or Modifying Bike Frame Datasets

When adding or editing frame files in `pkg/frames/*.go`:
1. **Strict Ground Truth & Chart Immutability**:
   - Published manufacturer geometry numbers (Stack, Reach, HTA, Wheelbase, etc.) are sacred ground truth. **NEVER modify or nudge published chart numbers** to absorb rounding residuals or force validation to pass.
   - Size geometry overrides are strictly forbidden unless resolving a proven, catastrophic manufacturer typo (>5mm or digit transposition) documented in `Notes`.
2. **Headset Standard & Cup Stack Verification**:
   - Verify the frame material and headset specification before finishing ingestion.
   - Carbon frames and frames with integrated headset bearings use `IS42`/`IS52` (0.0mm lower cup stack).
   - Steel/aluminum frames with external cups (`EC34`/`EC44`) physically add 10–14mm stack below the headtube; `GeoHeadsetSpacerMm` explicitly sets the CAD spacer (typically 10–12mm if drawn with cups, or 0.0mm if drawn to the bare headtube).
3. **`GeoSlopMm` is an Absolute Last Resort**:
   - `GeoSlopMm` must **never** be used to mask modeling errors (such as using the bike's sold rigid fork rather than the suspension fork the CAD chart was drawn around, ignoring CAD sag, or assuming the wrong headset standard).
   - Validation needs no slop for ordinary rounding. Each closure check (Stack ↔ fork A2C, Front Center, Wheelbase) propagates the rounding half-width of every published input through its equation (`geometry.A2cBudget`, `FrontCenterBudget`, `WheelbaseBudget`) and fails only when the residual exceeds that worst case. Precision is inferred per chart column: a head-angle column of 68, 68.5, 69 is on a half-degree grid (±0.25°). A pass that holds only because the head angle is assumed rounded is reported as an `*_needs_angle_rounding` notice.
   - `GeoSlopMm` is an allowance **added** to the rounding budget, for chart error that rounding cannot explain. It must be needed (some size fails without it) and no larger than `bikegeo fit`'s "Allowance needed beyond rounding" rounded up to 0.5mm; `TestDeclaredAllowancesAreNecessary` enforces both.
   - If a custom `GeoSlopMm` override is genuinely necessary, it requires an explicit `Reason` field on `model.GeoSlopMm` documenting the manufacturer discrepancy.
4. **Use `bikegeo fit` as Primary Tool & Automated Solver**:
   - Run `bikegeo fit <frame>` to diagnose geometry. If validation fails or any size needs more than rounding of the published lengths to pass, `bikegeo fit` automatically runs the physical solver exploring candidate fork travels, standard sag points (0%, 20%, 25%, 30%), static A2C values, rakes (including 44, 51, and 55mm), and headset spacers, presenting the solved Go struct fields.
   - Use `bikegeo fit <frame> --try ...` to sweep candidate parameters manually when exploring non-standard options.
   - Ad-hoc scripts are acceptable for source data wrangling (unit conversions, table parsing) or investigating unmodeled frame quirks, but routine CAD back-solving belongs in `fit`.
   - See the workspace skill [ingest-frame](file:///.agents/skills/ingest-frame/SKILL.md) for the complete 5-step recipe.
5. **Register New Frames in `pkg/frames/frames.go`**:
   - Create `pkg/frames/<frame-id>.go` and add the frame to the `allFrames` slice in `pkg/frames/frames.go`.
6. **Automated Test Coverage**:
   - `pkg/validator/validator_test.go` (`TestAllFrameData`) automatically validates all frames in `frames.AllFrames()`, verifies every size materializes and passes validation, and verifies supported fork capabilities.
   - You do **not** need to add boilerplate per-frame unit tests or update test counts when adding frames.

## Testing Strategy & Conventions

- **Unit & Integration Tests**:
  - Domain math, validation, canonicalization, ideal profiles, scoring, fit solver, and CLI subcommands (`compare`, `validate`, `list`, `fit`) are tested in `pkg/*/*_test.go`. CLI tests assert on cell values (`runCLIRows` flattens Markdown tables), not on padding.
  - HTTP handlers and templates are tested browserlessly in `pkg/web/web_test.go`: `httptest.NewRequest` (set `req.Host = "127.0.0.1:8080"`) → `web.New().ServeHTTP` → `goquery` selectors such as `tr[data-frame-id="..."] td[data-metric="..."]`. No Playwright, Selenium, or other browser automation.
- **Command**:
  ```bash
  go vet ./... && go test -v ./...
  ```

## Output Formatting & Markdown Guidelines

- **No LaTeX / MathJax**: The Antigravity IDE chat UI does not render LaTeX or MathJax math syntax (e.g., `$...$`, `$$...$$`, `\text{...}`). Never emit raw LaTeX math markup.
- **Plain Text / Standard Markdown**: Format mathematical relationships, formulas, and dimensions using plain text or inline code blocks (e.g., `472 mm (Fork A2C) + 10 mm (CAD Spacer) = 482 mm (Structural A2C)` or `Stack = BB_Drop - Rake * cos(HTA) + (A2C + Spacer + HTL) * sin(HTA)`).
- **Unicode Symbols**: Use standard Unicode characters for mathematical symbols and units (e.g., `±1.0 mm`, `70.0°`).
