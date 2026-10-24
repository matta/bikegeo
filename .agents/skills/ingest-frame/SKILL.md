---
name: ingest-frame
description: Standard workflow and recipe for ingesting new bike frames and geometry charts into bikegeo without throwaway scripts.
---

# Frame Ingestion Workflow

When ingesting a new bicycle frame geometry chart and spec into the `bikegeo` database:

## Core Principle: Use `bikegeo fit` as Primary Tool
Always use `bikegeo fit` and `bikegeo fit --try` as your primary diagnostic for solving CAD assumptions (A2C, spacers, slop, sag, and residuals).

**Do not write throwaway scripts for routine CAD math** that `fit` already solves.

**Acceptable uses of scratch code:**
- Parsing, cleaning, or converting raw manufacturer data (e.g., unit conversions, stripping HTML/PDF tables).
- Investigating non-standard frame features not yet modeled in `bikegeo` (e.g., flip-chip geometries or eccentric bottom brackets).

If a calculation turns out to be common across frames rather than a one-off quirk, consider promoting it to `bikegeo fit`.

## 5-Step Ingestion Recipe

### Step 1: Transcribe Geometry into `data/bikes/<id>.json`
Create or edit `data/bikes/<id>.json` directly:
- Follow standard `FrameModel` schema (e.g. `headset_cup_type`, `geo_headset_spacer_mm`, `supported_travel_mm`, `default_fork`, `configurations`, `geometries`).
- Storage does not reject invalid frames on load, so iterating in place in `data/bikes/` is completely safe.

### Step 2: Run `bikegeo fit` to Diagnose Geometry
Run:
```bash
bikegeo fit <frame-id>
```
Review the output:
- **Per-size table**: check implied structural A2C, `Δ vs Best`, FC calculated vs published, WB calculated vs published.
- **Minimax structural A2C**: the optimal midpoint minimizing worst-case deviation.
- **Implied spacer**: the exact spacer height implied by the declared fork A2C.
- **Implied static fork A2C**: the static fork A2C implied if the declared spacer is held fixed.
- **Minimum passing slop**: minimum `geo_slop_mm` (a2c, fc, wb) required for all sizes to pass.

### Step 3: Sweep Unknowns with `--try` (if needed)
If the manufacturer's fork offset (rake), CAD spacer, sag percentage, or fork A2C are ambiguous or unstated, sweep candidate combinations:
```bash
bikegeo fit <frame-id> --try fork.a2c=470,472 --try fork.rake=44,52 --try spacer=10,12
```
For sagged charts:
```bash
bikegeo fit <frame-id> --try sag=0,15,20,25
```
For checking whether a single size has a typo in published stack or reach:
```bash
bikegeo fit <frame-id> --try XL:stack=685,690
```
Use `--detail` to print the full per-size residual table for each combination.

### Step 4: Apply Chosen Values with `bikegeo set`
Use `bikegeo set` to apply the determined parameters to the frame JSON:
```bash
# Frame & hardware attributes:
bikegeo set <frame-id> spacer=11 slop.a2c=1.5

# Baseline fork attributes:
bikegeo set <frame-id> fork.a2c=472 fork.rake=52 sag=20

# Specific size geometry overrides:
bikegeo set <frame-id> XL:stack=690
```

### Step 5: Validate and Verify Test Suite
Verify that CAD validation passes:
```bash
bikegeo validate <frame-id>
```
Then run the fast feedback verification loop:
```bash
go vet ./... && go test ./...
```
`TestAllFrameData` in `pkg/validator/validator_test.go` and `TestLoadRealBikesDataDir` in `pkg/storage/storage_test.go` automatically validate all frames in `data/bikes/` and dynamically verify storage counts. You do **not** need to add boilerplate frame tests or edit hard-coded counts.
