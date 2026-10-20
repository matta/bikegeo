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
1. **Frame Attributes & Hardware Assumptions**:
   - Specify `headset_cup_type` (e.g. `EC44`, `ZS44`, `IS52`) and the explicit CAD spacer `geo_headset_spacer_mm` (typically 10-12mm for external cups, 0mm for integrated).
   - Declare `supported_travel_mm` (slice of mm), `rigid_a2c_mm` (if supported), `default_fork`, and `chainstay_range_mm` (for adjustable/rocker dropouts).
   - Ensure baseline configuration specifies published fork specs (`travel_mm`, `a2c`, `rake`, `standard_sag_percent`).
   - If published geometry rounding exceeds default $\pm 1.0\text{mm}$, declare appropriate `geo_slop_mm` (e.g. `a2c`, `wheelbase`).
2. **Synchronize Storage Test Counts**:
   - `pkg/storage/storage_test.go` (`TestLoadRealBikesDataDir`) enforces exact counts for real frames and materialized bikes. Update these assertions when adding or removing frames.
3. **Add Validation Unit Tests**:
   - Add a test case in `pkg/validator/validator_test.go` verifying `validator.ValidateFrame(frame)` and frame capabilities (`SupportsRigid()`, `SupportsTravel()`, `HasAdjustableChainstay()`).
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
