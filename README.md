# BikeGeo Analyst CLI 🚲

A zero-dependency Go command-line application for **bicycle geometry validation and apples-to-apples frame comparisons**.

All bike frame definitions ([`pkg/frames/`](pkg/frames)), fork and stem catalogs ([`pkg/catalog/catalog.go`](pkg/catalog/catalog.go)), ideal fit profiles ([`pkg/ideal/loader.go`](pkg/ideal/loader.go)), and the baseline comparison state ([`pkg/cli/compare.go`](pkg/cli/compare.go)) are compiled directly into Go structs with zero external dependencies or JSON configuration files.

---

## Key Features

1. **Unified Per-Column Comparison Model (`Frame + Size + Fork + Sag`)**:
   - Every column in `bikegeo compare` is an independent setup (`frame[@size][@fork][@sag%]`).
   - Compare the **exact same frame** across different sag points (`havok:L@110mm@15% havok:L@110mm@25%`) or across its supported forks (`havok:L@110mm@20% havok:L@rigid`).
   - Compare **different frames** at different sag points and fork capacities (`havok:L@110mm@20% hayduke:S3@110mm@20% fenrir:L@rigid`).

2. **Strict Frame-Level Fork & Sag Constraint Enforcement**:
   - Frames specify their CAD fork (`CadFork`) and supported forks (`Forks`) in [`pkg/frames/`](pkg/frames).
   - Automatically prevents invalid combinations:
     - Rejects unsupported suspension travel capacities for a frame.
     - Rejects `rigid` forks on suspension-only frames.
     - Rejects non-zero sag (`@20%`) on rigid forks.
     - Enforces fixed-fork frame constraints seamlessly.

3. **Compiled Baseline Comparison & Incremental Tweaks**:
   - Running bare `bikegeo compare` evaluates the compiled default comparison baseline (`cli.DefaultComparisonState()`).
   - Use `+spec` / `-spec` / `-col_num` to incrementally add or remove columns against the baseline in memory.

4. **Master Stack Equation & CAD Reverse-Engineering (`bikegeo validate` & `bikegeo fit`)**:
   - Proves whether published manufacturer charts match physical front-end geometry:
     `Stack = BB_drop - Rake * cos(HTA) + (A2C + cup_stack + HTL) * sin(HTA)`
   - Reverse-engineers geometric Axle-to-Crown:
     `A2C_geo = (Stack - BB_drop + Rake * cos(HTA)) / sin(HTA) - HTL - cup_stack`
   - Detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.
   - `bikegeo fit <frame>` automatically solves for candidate physical fork/spacer/sag combinations when ingesting new frames.

5. **Ideal Frame Profiles & Sweet-Spot Scoring (`pkg/ideal`, `pkg/scoring`)**:
   - Frames are evaluated against parameterized sweet-spot ranges defined in [`pkg/ideal/loader.go`](pkg/ideal/loader.go) (`default` and `idmatch` profiles).
   - Evaluates criteria across nested ranges:
     - **Ideal (`✓`)**: Scores 100.0 (e.g. Setback 230mm, AETT 625–636mm, Spare Vert 70–100mm, Seat-RC ≥235mm).
     - **Acceptable (`~`)**: Scores `EdgeScore` (typically 75–80) to 100 with linear ramp towards ideal. Cell displays offset to ideal (e.g. `225 ~ −5`).
     - **Outside (`✗`)**: Soft falloff beyond acceptable to 0 (or floor). Cell displays offset to ideal (e.g. `213 ✗ −17`).
   - Tables are automatically sorted by score descending (with original `[#]` identifiers preserved).
   - Generates plain-English footer explanations (`Biggest Losses vs. Ideal` and `Head-to-Head Deciding Factors`).
   - Switch profiles using `--ideal <name>`, disable scoring with `--no-score`, or inspect sub-scores with `--score-detail`.

6. **Ideal Profile Inspection (`bikegeo ideals`)**:
   - `bikegeo ideals list`: List compiled ideal profiles.
   - `bikegeo ideals show <name>`: Display profile attribute weights, sweet-spot ranges, falloff configuration, and sample evaluation points.

7. **Multiple Output Formats**:
   - Supports `--format table` (Unicode box tables, default), `--format md` (Markdown), and `--format csv`.

---

## Quick Start

### Build & Run
```bash
make build
./bin/bikegeo help
```

### Usage Examples

```bash
# List all compiled frames, sizes, and supported fork capacities:
./bin/bikegeo list
./bin/bikegeo list --all-sizes

# List available ideal frame profiles:
./bin/bikegeo ideals list
./bin/bikegeo ideals show default

# Compare frames against an ideal profile (auto-transposed when >4 columns):
./bin/bikegeo compare havok@110mm@20% hayduke:S3@110mm@20% fenrir:L@rigid

# Compare using a specific ideal profile:
./bin/bikegeo compare --ideal idmatch fenrir:all@120mm

# Run the default baseline comparison:
./bin/bikegeo compare

# Incrementally add or remove columns from the default baseline:
./bin/bikegeo compare +surly-ecr:XL
./bin/bikegeo compare -salsa-fargo:all

# View detailed sub-score loss breakdown:
./bin/bikegeo compare --score-detail

# Validate frame CAD math across all sizes and configurations:
./bin/bikegeo validate binary-havok
./bin/bikegeo validate --all

# Diagnose or sweep CAD assumptions on a frame:
./bin/bikegeo fit surly-ecr
./bin/bikegeo fit seven-expedition-926f --try fork.a2c=511,531 --try sag=0,20,25
```

---

## Running Verification & Tests

```bash
go vet ./... && go test -v ./...
```
