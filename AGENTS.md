# Agent Instructions & Verification Loop

This repository is a zero-dependency **Go** command-line application (`bikegeo`).

## Fast Feedback Command Loop

After **every** code modification, always execute the following command to verify type safety and test suite pass rate:

```bash
go vet ./... && go test ./...
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
1. **Use `bikegeo fit` as Primary Tool**:
   - Use the built-in primitive `bikegeo fit <frame>` and `bikegeo fit <frame> --try ...` to diagnose geometry and sweep candidate parameters instead of writing scratch CAD math scripts.
   - Ad-hoc scripts are acceptable for source data wrangling (unit conversions, table parsing) or investigating unmodeled frame quirks, but routine CAD back-solving belongs in `fit`.
   - See the workspace skill [ingest-frame](file:///.agents/skills/ingest-frame/SKILL.md) for the complete 5-step recipe.
2. **Apply Parameters with `bikegeo set`**:
   - Use `bikegeo set <frame> spacer=11 slop.a2c=1.5 fork.a2c=472 ...` to update frame or configuration properties.
3. **Automated Test Coverage**:
   - `pkg/validator/validator_test.go` (`TestAllFrameData`) automatically validates all files in `data/bikes/*.json`, verifies every size materializes and passes validation, checks ID/filename parity, and verifies supported fork capabilities.
   - `pkg/storage/storage_test.go` (`TestLoadRealBikesDataDir`) dynamically validates storage loading and size counts without hard-coded numbers.
   - You do **not** need to add boilerplate per-frame unit tests or update test counts when adding frames.
4. **State Cleanliness**:
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

