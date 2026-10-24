---
name: ingest-frame
description: Standard workflow and recipe for ingesting new bike frames and geometry charts into bikegeo without throwaway scripts.
---

# Frame Ingestion Workflow

When ingesting a new bicycle frame geometry chart and spec into the `bikegeo` database:

## Core Principles

1. **Strict Chart Immutability (Ground Truth)**:
   - Published manufacturer geometry numbers (Stack, Reach, HTA, Wheelbase, etc.) are sacred ground truth. **NEVER modify or nudge published numbers** to absorb rounding residuals or force validation to pass.
   - Size geometry overrides are strictly forbidden unless resolving a proven, catastrophic manufacturer typo (>5mm or digit transposition) documented in `notes`.
2. **Headset Standard Verification**:
   - Check the frame material and headset specification before declaring ingestion complete.
   - Carbon frames and frames with integrated headset bearings use `IS42`/`IS52` (0.0mm lower cup stack).
   - Steel/aluminum frames with external cups (`EC34`/`EC44`) physically add 10–14mm stack below the headtube; `geo_headset_spacer_mm` explicitly sets the CAD spacer (typically 10–12mm if drawn with cups, or 0.0mm if drawn to the bare headtube).
3. **`geo_slop_mm` is an Absolute Last Resort**:
   - `geo_slop_mm` must **never** be used to mask modeling errors (such as using the bike's sold rigid fork rather than the suspension fork the CAD chart was drawn around, ignoring CAD sag, or assuming the wrong headset standard).
   - Wheelbase validation automatically propagates input rounding uncertainty (HTA, Reach, Chainstay Length) with a 2.0mm physical precision floor, eliminating the need for custom wheelbase slop from normal rounding noise.
   - If a custom `geo_slop_mm` override is genuinely necessary, it requires an explicit `"reason"` field in the JSON documenting the manufacturer discrepancy.
4. **Use `bikegeo fit` as Primary Tool**:
   - Always use `bikegeo fit` and `bikegeo fit --try` as your primary diagnostic for solving CAD assumptions (A2C, spacers, slop, sag, and residuals).
   - **Do not write throwaway scripts for routine CAD math** that `fit` already solves.

**Acceptable uses of scratch code:**
- Parsing, cleaning, or converting raw manufacturer data (e.g., unit conversions, stripping HTML/PDF tables).
- Investigating non-standard frame features not yet modeled in `bikegeo` (e.g., flip-chip geometries or eccentric bottom brackets).

If a calculation turns out to be common across frames rather than a one-off quirk, consider promoting it to `bikegeo fit`.

## 5-Step Ingestion Recipe

### Step 1: Transcribe Geometry into `data/bikes/<id>.json`
Create or edit `data/bikes/<id>.json` directly:
- Follow standard `FrameModel` schema (`headset_cup_type`, `geo_headset_spacer_mm`, `supported_travel_mm`, `default_fork`, `configurations`, `geometries`).
- Enter literal, exact published numbers without altering them.
- Storage does not reject invalid frames on load, so iterating in place in `data/bikes/` is completely safe.

### Step 2: Run `bikegeo fit` to Diagnose Geometry & Review Automated Recommendations
Run:
```bash
bikegeo fit <frame-id>
```
Review the output:
- **Per-size table**: check implied structural A2C, `Δ vs Best`, FC calculated vs published, WB calculated vs published.
- **Minimax structural A2C**: the optimal midpoint minimizing worst-case deviation.
- **Implied spacer**: the exact spacer height implied by the declared fork A2C.
- **Implied static fork A2C**: the static fork A2C implied if the declared spacer is held fixed.
- **Minimum passing slop**: minimum `geo_slop_mm` required for all sizes to pass.
- **Candidate Physical Solutions**: If the frame is invalid or requires A2C slop > 1.0mm, `bikegeo fit` automatically solves for candidate fork travels, standard sags (0%, 15%, 20%, 25%, 30%), static A2Cs, rakes (including 44, 51, and 55mm), and spacers, presenting suggested `bikegeo set ...` commands.

### Step 3: Sweep Unknowns with `--try` (if needed)
If the configuration requires exploring specific parameters beyond standard candidates, sweep candidate combinations:
```bash
bikegeo fit <frame-id> --try fork.a2c=470,472 --try fork.rake=44,51,55 --try spacer=10,12
```
For sagged charts:
```bash
bikegeo fit <frame-id> --try sag=0,15,20,25
```
Use `--detail` to print the full per-size residual table for each combination.

### Step 4: Apply Chosen Values with `bikegeo set`
Use `bikegeo set` to apply the determined physical parameters to the frame JSON:
```bash
# Frame & hardware attributes:
bikegeo set <frame-id> cup_type=IS52 spacer=0

# Baseline fork attributes:
bikegeo set <frame-id> fork.a2c=506 fork.rake=51 sag=25
```

### Step 5: Validate and Verify Test Suite
Verify that CAD validation passes with default tolerances:
```bash
bikegeo validate <frame-id>
```
Then run the fast feedback verification loop:
```bash
go vet ./... && go test ./...
```
`TestAllFrameData` in `pkg/validator/validator_test.go` and `TestLoadRealBikesDataDir` in `pkg/storage/storage_test.go` automatically validate all frames in `data/bikes/` and dynamically verify storage counts. You do **not** need to add boilerplate frame tests or edit hard-coded counts.
