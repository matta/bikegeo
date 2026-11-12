# Agent Instructions & Verification Loop

This repository is a zero-dependency **Go** command-line application (`bikegeo`).

## Fast Feedback Command Loop

After **every** code modification, always execute the following command to verify type safety and test suite pass rate:

```bash
go vet ./... && go test ./...
```

## Final Verification & Cleanup

Formatting is not part of the fast feedback loop, but is part of the final cleanup stages of code modification. Before completing a task or submitting changes, format all Go source files and verify the full check suite:

```bash
go fmt ./... && make check
```

## Architectural Guidelines

- **Zero Runtime Dependencies & Pure-Go Data**:
  - The shipped binary uses only the Go standard library (`flag`, `encoding/csv`, `net/http`, `html/template`, `embed`) with custom Unicode box-drawing, Markdown, and CSV table formatting in `pkg/cli/render.go`.
  - The single exception is `github.com/PuerkitoBio/goquery`, allowed **only** in `_test.go` files for DOM assertions. Never import it (or any third-party package) from non-test code; verify with `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo`, which must list only `bikegeo/...`.
  - All bike frames (`pkg/frames/*.go`), fork and stem catalogs (`pkg/catalog/catalog.go`), ideal fit profiles (`pkg/ideal/loader.go`), and the default comparison baseline (`compare.DefaultComparisonState()` in `pkg/compare/specs.go`, also reachable as `cli.DefaultComparisonState()`) are compiled directly as Go structs. There are no JSON data files or runtime persistence files.
- **CLI and Web Share One Engine** (see [docs/WEB.md](docs/WEB.md)):
  - `pkg/compare` holds spec parsing, `Run(Options)`, ranking/pruning and `BuildDocument`; `pkg/report` holds the presentation-neutral `Document`/`Table`/`Cell` types (cells carry `Zone` and `MetricID`); `pkg/cli` parses flags and prints; `pkg/web` serves HTML.
  - Command table output goes through `App.render` / `App.renderFooters` so it prints in the CLI and is collected by `cli.RunDocument` for the web. Do not call `RenderOutput*` directly from commands.
  - `bikegeo serve` is dispatched in `cmd/bikegeo/main.go` (`pkg/web` imports `pkg/cli`, never the reverse).
  - The web is strictly Web 1.0: `GET` forms, URL query as the only state, full-page responses, no JavaScript, no cookies. Controls must be populated from the parsed query.
  - CLI output must remain byte-identical when refactoring; `pkg/cli/cli_test.go` is the guard. `bikegeo fit` can list tied candidate solutions in a varying order between runs; that is pre-existing.
- **Per-Column Comparison Model (`ComparisonSetup`)**:
  - Every comparison column specifies `FrameID + Size + Fork (travel in mm or "rigid") + Sag %`.
  - Allows comparing the same frame across multiple sag points or fork capacities alongside different frames.
- **Frame-Level Fork Constraints**:
  - Frames define `CadFork` and `Forks` (supported fork refs with `ID`, `A2cMm`, `RakeMm`, `SagPct`) in `pkg/frames/*.go`.
  - `pkg/canonicalizer` strictly enforces these constraints, rejecting unsupported travel sizes, rigid forks on suspension-only frames, or non-zero sag on rigid forks.
- **Default Comparison Baseline**:
  - Running `bikegeo compare` with no arguments evaluates `cli.DefaultComparisonState()`. Incremental `+spec` and `-spec` / `-col_num` flags add or remove columns against that baseline in memory.

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
   - Wheelbase validation incorporates dynamic propagation of input rounding uncertainty with a 2.0mm physical precision floor, eliminating the need for custom wheelbase slop from normal rounding noise.
   - If a custom `GeoSlopMm` override is genuinely necessary, it requires an explicit `Reason` field on `model.GeoSlopMm` documenting the manufacturer discrepancy.
4. **Use `bikegeo fit` as Primary Tool & Automated Solver**:
   - Run `bikegeo fit <frame>` to diagnose geometry. If validation fails or A2C slop > 1.0mm, `bikegeo fit` automatically runs the physical solver exploring candidate fork travels, standard sag points (0%, 20%, 25%, 30%), static A2C values, rakes (including 44, 51, and 55mm), and headset spacers, presenting the solved Go struct fields.
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
  - Domain math, validation, canonicalization, ideal profiles, scoring, fit solver, and CLI subcommands (`compare`, `validate`, `list`, `forks`, `stems`, `ideals`, `fit`) are tested in `pkg/*/*_test.go`.
  - HTTP handlers and templates are tested browserlessly in `pkg/web/web_test.go`: `httptest.NewRequest` (set `req.Host = "127.0.0.1:8080"`) → `web.New().ServeHTTP` → `goquery` selectors such as `tr[data-frame-id="..."] td[data-metric="..."]`. No Playwright, Selenium, or other browser automation.
- **Command**:
  ```bash
  go vet ./... && go test -v ./...
  ```

## Output Formatting & Markdown Guidelines

- **No LaTeX / MathJax**: The Antigravity IDE chat UI does not render LaTeX or MathJax math syntax (e.g., `$...$`, `$$...$$`, `\text{...}`). Never emit raw LaTeX math markup.
- **Plain Text / Standard Markdown**: Format mathematical relationships, formulas, and dimensions using plain text or inline code blocks (e.g., `472 mm (Fork A2C) + 10 mm (CAD Spacer) = 482 mm (Structural A2C)` or `Stack = BB_Drop - Rake * cos(HTA) + (A2C + Spacer + HTL) * sin(HTA)`).
- **Unicode Symbols**: Use standard Unicode characters for mathematical symbols and units (e.g., `±1.0 mm`, `70.0°`).
