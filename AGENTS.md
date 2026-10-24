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

- **Zero External Dependencies**: Uses the Go standard library (`flag`, `encoding/json`, `encoding/csv`, `bufio`) with custom Unicode box-drawing, Markdown, CSV, and JSON table formatting in `pkg/cli/render.go`.
- **Per-Column Comparison Model (`ComparisonSetup`)**:
  - Every comparison column specifies `FrameID + Size + Fork (travel in mm or "rigid") + Sag %`.
  - Allows comparing the same frame across multiple sag points or fork capacities alongside different frames.
- **Frame-Level Fork Constraints**:
  - Frames define `supported_travel_mm` (in mm), `rigid_a2c_mm`, and `default_fork` in `data/bikes/*.json`.
  - `pkg/canonicalizer` strictly enforces these constraints, rejecting unsupported travel sizes, rigid forks on suspension-only frames, or non-zero sag on rigid forks.
- **Persistence**:
  - Bikes/frames are stored as individual JSON files in `data/bikes/*.json`.
  - The most recent comparison state is persisted to `data/last_compare.json` so running `bikegeo compare` with no arguments replays the last comparison.

## Adding or Modifying Bike Frame Datasets

When adding or editing frame files in `data/bikes/*.json`:
1. **Strict Ground Truth & Chart Immutability**:
   - Published manufacturer geometry numbers (Stack, Reach, HTA, Wheelbase, etc.) are sacred ground truth. **NEVER modify or nudge published chart numbers** to absorb rounding residuals or force validation to pass.
   - Size geometry overrides are strictly forbidden unless resolving a proven, catastrophic manufacturer typo (>5mm or digit transposition) documented in `notes`.
2. **Headset Standard & Cup Stack Verification**:
   - Verify the frame material and headset specification before finishing ingestion.
   - Carbon frames and frames with integrated headset bearings use `IS42`/`IS52` (0.0mm lower cup stack).
   - Steel/aluminum frames with external cups (`EC34`/`EC44`) physically add 10–14mm stack below the headtube; `geo_headset_spacer_mm` explicitly sets the CAD spacer (typically 10–12mm if drawn with cups, or 0.0mm if drawn to the bare headtube).
3. **`geo_slop_mm` is an Absolute Last Resort**:
   - `geo_slop_mm` must **never** be used to mask modeling errors (such as using the bike's sold rigid fork rather than the suspension fork the CAD chart was drawn around, ignoring CAD sag, or assuming the wrong headset standard).
   - Wheelbase validation incorporates dynamic propagation of input rounding uncertainty with a 2.0mm physical precision floor, eliminating the need for custom wheelbase slop from normal rounding noise.
   - If a custom `geo_slop_mm` override is genuinely necessary, it requires an explicit `"reason"` field in the JSON documenting the manufacturer discrepancy.
4. **Use `bikegeo fit` as Primary Tool & Automated Solver**:
   - Run `bikegeo fit <frame>` to diagnose geometry. If validation fails or A2C slop > 1.0mm, `bikegeo fit` automatically runs the physical solver exploring candidate fork travels, standard sag points (0%, 15%, 20%, 25%, 30%), static A2C values, rakes (including 44, 51, and 55mm), and headset spacers, presenting copy-paste `bikegeo set ...` solutions.
   - Use `bikegeo fit <frame> --try ...` to sweep candidate parameters manually when exploring non-standard options.
   - Ad-hoc scripts are acceptable for source data wrangling (unit conversions, table parsing) or investigating unmodeled frame quirks, but routine CAD back-solving belongs in `fit`.
   - See the workspace skill [ingest-frame](file:///.agents/skills/ingest-frame/SKILL.md) for the complete 5-step recipe.
5. **Apply Parameters with `bikegeo set`**:
   - Use `bikegeo set <frame> cup_type=IS52 spacer=0 fork.a2c=506 sag=25 ...` to update frame or configuration properties.
6. **Automated Test Coverage**:
   - `pkg/validator/validator_test.go` (`TestAllFrameData`) automatically validates all files in `data/bikes/*.json`, verifies every size materializes and passes validation, checks ID/filename parity, and verifies supported fork capabilities.
   - `pkg/storage/storage_test.go` (`TestLoadRealBikesDataDir`) dynamically validates storage loading and size counts without hard-coded numbers.
   - You do **not** need to add boilerplate per-frame unit tests or update test counts when adding frames.
7. **State Cleanliness**:
   - Running CLI compare commands mutates `data/last_compare.json`. Avoid leaving unintended comparison changes in git state.

## Testing Strategy & Conventions

- **Unit & Integration Tests**:
  - Domain math, validation, canonicalization, storage, and CLI subcommands (`compare`, `validate`, `list`, `forks`, `init`, `set`, `import`, `delete`) are tested in `pkg/*/*_test.go`.
  - Simulate interactive CLI flows (`-i`) by passing `strings.NewReader` to `cli.Run(args, stdin, stdout, stderr)`.
- **Command**:
  ```bash
  go vet ./... && go test -v ./...
  ```

## Output Formatting & Markdown Guidelines

- **No LaTeX / MathJax**: The Antigravity IDE chat UI does not render LaTeX or MathJax math syntax (e.g., `$...$`, `$$...$$`, `\text{...}`). Never emit raw LaTeX math markup.
- **Plain Text / Standard Markdown**: Format mathematical relationships, formulas, and dimensions using plain text or inline code blocks (e.g., `472 mm (Fork A2C) + 10 mm (CAD Spacer) = 482 mm (Structural A2C)` or `Stack = BB_Drop - Rake * cos(HTA) + (A2C + Spacer + HTL) * sin(HTA)`).
- **Unicode Symbols**: Use standard Unicode characters for mathematical symbols and units (e.g., `±1.0 mm`, `70.0°`).

