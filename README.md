# BikeGeo Analyst CLI 🚲

A zero-dependency Go command-line application for **bicycle geometry validation and apples-to-apples frame comparisons**.

Bike geometry is stored in **local JSON files** under [`data/bikes/`](data/bikes), and the most recent comparison setup is persisted in `data/last_compare.json`.

---

## Key Features

1. **Unified Per-Column Comparison Model (`Frame + Size + Fork + Sag`)**:
   - Every column in `bikegeo compare` is an independent setup (`frame[@size][@fork][@sag%]`).
   - Compare the **exact same frame** across different sag points (`sendero@20% sendero@30%`) or across its supported forks (`sendero@120mm@20% sendero@130mm@20% sendero@rigid`).
   - Compare **different frames** at different sag points and fork capacities (`sendero@120mm@20% chameleon@130mm@30% fenrir@rigid`).

2. **Strict Frame-Level Fork & Sag Constraint Enforcement**:
   - Frames specify legal suspension travel sizes in mm (`supported_travel_mm`, e.g. `[120, 130]`) and/or a rigid fork axle-to-crown (`rigid_a2c_mm`, e.g. `483`).
   - Automatically prevents invalid combinations:
     - Rejects unsupported suspension travel capacities for a frame.
     - Rejects `rigid` forks on suspension-only frames.
     - Rejects non-zero sag (`@20%`) on rigid forks.
     - Handles fixed-fork frames seamlessly without asking redundant questions.

3. **Persistent Comparison Memory & Interactive Builder (`-i`)**:
   - Automatically saves your most recent comparison to `data/last_compare.json`.
   - Running bare `bikegeo compare` immediately replays your last comparison.
   - Use `+spec` / `-col_num` to incrementally add or remove columns from the saved comparison.
   - Run `bikegeo compare -i` for a guided prompt/response column builder that only prompts for choices allowed by each frame's constraints.

4. **Master Stack Equation & CAD Reverse-Engineering (`bikegeo validate`)**:
   - Proves whether published manufacturer charts match physical front-end geometry:\
     $`\text{Stack} = \text{BB}_{\text{drop}} - \text{Rake} \cdot \cos(\text{HTA}) + (\text{A2C} + \text{cup}_{\text{stack}} + \text{HTL}) \cdot \sin(\text{HTA})`$
   - Reverse-engineers geometric Axle-to-Crown:\
     $`\displaystyle \text{A2C}_{\text{geo}} = \frac{\text{Stack} - \text{BB}_{\text{drop}} + \text{Rake} \cdot \cos(\text{HTA})}{\sin(\text{HTA})} - \text{HTL} - \text{cup}_{\text{stack}}`$
   - Detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.
   - Verifies **Cross-Configuration CAD Coherence** when a frame publishes charts for multiple forks.

5. **Ideal Frame Profiles & Sweet-Spot Scoring (`pkg/ideal`, `pkg/scoring`)**:
   - Frames are evaluated against parameterized sweet-spot ranges defined in editable JSON profiles in `data/ideals/` (default: [`data/ideals/default.json`](data/ideals/default.json)).
   - Evaluates criteria across nested ranges:
     - **Ideal (`✓`)**: Scores 100.0 (e.g. Setback 230mm, AETT 625–636mm, Spare Vert 70–100mm, Chainstay ≥470mm).
     - **Acceptable (`~`)**: Scores `edge_score` (typically 75–80) to 100 with linear ramp towards ideal (e.g. Setback 220–240mm, AETT 604–648mm). Cell displays offset to ideal (e.g. `225 ~ −5`).
     - **Outside (`✗`)**: Soft falloff beyond acceptable to 0 (or floor). Cell displays offset to ideal (e.g. `213 ✗ −17`).
   - Supports 15 raw geometry and derived fit metrics (`setback`, `aett`, `sparevert`, `chainstay`, `bbdrop`, `hta`, `frontwt`, `standover`, `reach`, `stack`, `sta`, `wheelbase`, `frontcenter`, `chainstay_range`, `bbheight`).
   - Tables are automatically sorted by score descending (with original `[#]` identifiers preserved).
   - Generates plain-English footer explanations (`Biggest Losses vs. Ideal` and `Head-to-Head Deciding Factors`).
   - Switch profiles using `--ideal <name>` or `p <name>` in interactive mode (`bikegeo compare -i`).
   - Use `--no-score` to disable scoring and restore input order, or `--score-detail` for the full sub-score breakdown table.

6. **Ideal Profile Management (`bikegeo ideals`)**:
   - `bikegeo ideals`: List available profile JSON files.
   - `bikegeo ideals show <name>`: Display profile attribute weights, sweet-spot ranges, and falloff configuration.

7. **Multiple Output Formats**:
   - Supports `--format table` (Unicode box tables, default), `--format json`, `--format md` (Markdown), and `--format csv`.

---

## Quick Start

### Build & Run
```bash
make build
./bin/bikegeo help
```

### Usage Examples

```bash
# List all stored frames, sizes, and supported fork capacities:
./bin/bikegeo list
./bin/bikegeo list --all-sizes

# List available ideal frame profiles:
./bin/bikegeo ideals
./bin/bikegeo ideals show default

# Compare frames against an ideal profile (auto-transposed when >4 columns):
./bin/bikegeo compare havok@110mm@20% hayduke:S3@110mm@20% fenrir:L@rigid

# Compare using a specific ideal profile:
./bin/bikegeo compare --ideal default fenrir:all@120mm

# Replay the most recent comparison (from data/last_compare.json):
./bin/bikegeo compare

# Incrementally add a column and switch to transposed/compact view:
./bin/bikegeo compare +havok@ML@100mm@20% --transpose

# View detailed sub-score loss breakdown:
./bin/bikegeo compare --score-detail

# Launch interactive prompt/response comparison builder:
./bin/bikegeo compare -i

# Validate frame CAD math across all sizes and configurations:
./bin/bikegeo validate binary-havok
./bin/bikegeo validate --all
```

---

## Running Verification & Tests

```bash
go vet ./... && go test -v ./...
```

