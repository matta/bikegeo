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
   - Proves whether published manufacturer charts match physical front-end geometry:
     $$\text{Stack} = \text{BB\_drop} - \text{Rake} \cdot \cos(\text{HTA}) + (\text{A2C} + \text{cup\_stack} + \text{HTL}) \cdot \sin(\text{HTA})$$
   - Reverse-engineers geometric Axle-to-Crown:
     $$\text{A2C}_{\text{geo}} = \frac{\text{Stack} - \text{BB\_drop} + \text{Rake} \cdot \cos(\text{HTA})}{\sin(\text{HTA})} - \text{HTL} - \text{cup\_stack}$$
   - Detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.
   - Verifies **Cross-Configuration CAD Coherence** when a frame publishes charts for multiple forks.

5. **Multiple Output Formats**:
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

# Compare the same frame across 20% sag, 30% sag, and its rigid fork:
./bin/bikegeo compare havok@110mm@20% havok@110mm@30% havok@rigid

# Compare different frames across sizes, sag points, and fork capacities:
./bin/bikegeo compare havok@L@rigid fenrir@L@rigid fenrir@L@110mm@20%

# Replay the most recent comparison (from data/last_compare.json):
./bin/bikegeo compare

# Incrementally add a column and switch to transposed/compact view:
./bin/bikegeo compare +havok@ML@100mm@20% --transpose

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

