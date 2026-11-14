---
name: ingest-frame
description: Standard workflow and recipe for ingesting new bike frames and geometry charts into bikegeo without throwaway scripts.
---

# Frame Ingestion Workflow

When ingesting a new bicycle frame geometry chart and spec into the `bikegeo` codebase:

## Core Principles

1. **Strict Chart Immutability (Ground Truth)**:
   - Published manufacturer geometry numbers (Stack, Reach, HTA, Wheelbase, etc.) are sacred ground truth. **NEVER modify or nudge published numbers** to absorb rounding residuals or force validation to pass.
   - Size geometry overrides are strictly forbidden unless resolving a proven, catastrophic manufacturer typo (>5mm or digit transposition) documented in `Notes`.
2. **Headset Standard Verification**:
   - Check the frame material and headset specification before declaring ingestion complete.
   - Carbon frames and frames with integrated headset bearings use `IS42`/`IS52` (0.0mm lower cup stack).
   - Steel/aluminum frames with external cups (`EC34`/`EC44`) physically add 10–14mm stack below the headtube; `GeoHeadsetSpacerMm` explicitly sets the CAD spacer (typically 10–12mm if drawn with cups, or 0.0mm if drawn to the bare headtube).
3. **`GeoSlopMm` is an Absolute Last Resort**:
   - `GeoSlopMm` must **never** be used to mask modeling errors (such as using the bike's sold rigid fork rather than the suspension fork the CAD chart was drawn around, ignoring CAD sag, or assuming the wrong headset standard).
   - Validation needs no slop for ordinary rounding. Each closure check (Stack ↔ fork A2C, Front Center, Wheelbase) propagates the rounding half-width of every published input through its equation (`geometry.A2cBudget`, `FrontCenterBudget`, `WheelbaseBudget`) and fails only when the residual exceeds that worst case. Precision is inferred per chart column: a head-angle column of 68, 68.5, 69 is on a half-degree grid (±0.25°). A pass that holds only because the head angle is assumed rounded is reported as an `*_needs_angle_rounding` notice.
   - `GeoSlopMm` is an allowance **added** to the rounding budget, for chart error that rounding cannot explain. It must be needed (some size fails without it) and no larger than `bikegeo fit`'s "Allowance needed beyond rounding" rounded up to 0.5mm; `TestDeclaredAllowancesAreNecessary` enforces both.
   - If a custom `GeoSlopMm` override is genuinely necessary, it requires an explicit `Reason` field on `model.GeoSlopMm` documenting the manufacturer discrepancy.
4. **Use `bikegeo fit` as Primary Tool**:
   - Always use `bikegeo fit` and `bikegeo fit --try` as your primary diagnostic for solving CAD assumptions (A2C, spacers, slop, sag, and residuals).
   - **Do not write throwaway scripts for routine CAD math** that `fit` already solves.

**Acceptable uses of scratch code:**
- Parsing, cleaning, or converting raw manufacturer data (e.g., unit conversions, stripping HTML/PDF tables).
- Investigating non-standard frame features not yet modeled in `bikegeo` (e.g., flip-chip geometries or eccentric bottom brackets).

If a calculation turns out to be common across frames rather than a one-off quirk, consider promoting it to `bikegeo fit`.

## 5-Step Ingestion Recipe

### Step 1: Transcribe Geometry into `pkg/frames/<id>.go` and Register in `pkg/frames/frames.go`
Create `pkg/frames/<id>.go`:
- Define a `var <FrameVar> = model.FrameModel{...}` following the standard `model.FrameModel` struct (`HeadsetCupType`, `GeoHeadsetSpacerMm`, `CadFork`, `Forks`, `MaxSpacerMm`, `CanonicalStem`, `Stems`, `Sizes`, `Geometries`) and register it in `allFrames` in `pkg/frames/frames.go`.
- Enter literal, exact published numbers without altering them.

### Step 2: Run `bikegeo fit` to Diagnose Geometry & Review Automated Recommendations
Output is Markdown (tables plus indented text). Run:
```bash
go run ./cmd/bikegeo fit <frame-id>
```
Review the output:
- **Per-size table**: check implied structural A2C, `Δ vs Best`, FC calculated vs published, WB calculated vs published.
- **Minimax structural A2C**: the optimal midpoint minimizing worst-case deviation.
- **Implied spacer**: the exact spacer height implied by the declared fork A2C.
- **Implied static fork A2C**: the static fork A2C implied if the declared spacer is held fixed.
- **Budget used (a2c/fc/wb)**: per size, the share of each check's rounding budget the residual uses. `*` means it fits only if the head angle is rounded, `+` means it needs the declared allowance, `!` means it fails.
- **Allowance needed beyond rounding**: the `GeoSlopMm` each check needs for all sizes to pass; `+0.0` means rounding explains everything and no override belongs in the frame file.
- **Candidate Physical Solutions**: If the frame is invalid or any size needs more than length rounding to pass, `bikegeo fit` automatically solves for candidate fork travels, standard sags (0%, 20%, 25%, 30%), static A2Cs, rakes (including 44, 51, and 55mm), and spacers, presenting the solved Go struct fields (`CadFork`, `GeoHeadsetSpacerMm`, etc.).

### Step 3: Sweep Unknowns with `--try` (if needed)
If the configuration requires exploring specific parameters beyond standard candidates, sweep candidate combinations:
```bash
go run ./cmd/bikegeo fit <frame-id> --try fork.a2c=470,472 --try fork.rake=44,51,55 --try spacer=10,12
```
For sagged charts:
```bash
go run ./cmd/bikegeo fit <frame-id> --try sag=0,20,25,30
```
Use `--detail` to print the full per-size residual table for each combination.

### Step 4: Apply Solved Values in `pkg/frames/<id>.go`
Update `HeadsetCupType`, `GeoHeadsetSpacerMm`, `CadFork`, and `Forks` in `pkg/frames/<id>.go` with the solved physical parameters.

### Step 5: Validate and Verify Test Suite
Verify that CAD validation passes within the rounding budgets, without a `GeoSlopMm` override unless `fit` showed one is needed:
```bash
go run ./cmd/bikegeo validate <frame-id>
```
Then run the required pre-commit workflow (which runs `go fix`, `go fmt`, `go vet`, `exhaustive`, and the full test suite; review any `go fix` diff per `AGENTS.md` to ensure no published geometry values changed):
```bash
make pre-commit
```
`TestAllFrameData` in `pkg/validator/validator_test.go` automatically validates all frames in `frames.AllFrames()`. You do **not** need to add boilerplate frame tests or edit hard-coded counts.
