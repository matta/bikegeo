# BikeGeo Analyst CLI 🚲

A zero-dependency Go application for **bicycle geometry validation and apples-to-apples frame comparisons**, with two interfaces, each used for what it suits: a **local web app** (`bikegeo serve`) for people, and a **Markdown command line** for agents and scripts.

All bike frame definitions ([`pkg/frames/`](pkg/frames)), fork and stem catalogs ([`pkg/catalog/catalog.go`](pkg/catalog/catalog.go)), ideal fit profiles ([`pkg/ideal/loader.go`](pkg/ideal/loader.go)), and the baseline comparison state ([`pkg/compare/specs.go`](pkg/compare/specs.go)) are compiled directly into Go structs with zero external dependencies or JSON configuration files.

---

## Key Features

1. **Unified Per-Column Comparison Model (`Frame + Size + Fork + Sag`)**:
   - Every column in `bikegeo compare` is an independent setup (`frame[@size][@fork][@sag%]`).
   - Compare the **exact same frame** across different sag points (`havok:L@110mm@20% havok:L@110mm@25%`) or across its supported forks (`havok:L@110mm@20% havok:L@rigid`).
   - Compare **different frames** at different sag points and fork capacities (`havok:L@110mm@20% hayduke:S3@110mm@20% fenrir:L@rigid`).

2. **Strict Frame-Level Fork & Sag Constraint Enforcement**:
   - Frames specify their CAD fork (`CadFork`) and supported forks (`Forks`) in [`pkg/frames/`](pkg/frames).
   - Automatically prevents invalid combinations:
     - Rejects unsupported suspension travel capacities for a frame.
     - Rejects `rigid` forks on suspension-only frames.
     - Rejects non-zero sag (`@20%`) on rigid forks.
     - Enforces fixed-fork frame constraints seamlessly.

3. **Compiled Baseline Comparison & Incremental Tweaks**:
   - Running bare `bikegeo compare` evaluates the compiled default comparison baseline (`compare.DefaultComparisonState()`).
   - Use `+spec` / `-spec` / `-col_num` to incrementally add or remove columns against the baseline in memory.

4. **Master Stack Equation & CAD Reverse-Engineering (`bikegeo validate` & `bikegeo fit`)**:
   - Proves whether published manufacturer charts match physical front-end geometry:
     `Stack = BB_drop - Rake * cos(HTA) + (A2C + cup_stack + HTL) * sin(HTA)`
   - Reverse-engineers geometric Axle-to-Crown:
     `A2C_geo = (Stack - BB_drop + Rake * cos(HTA)) / sin(HTA) - HTL - cup_stack`
   - Detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.
   - `bikegeo fit <frame>` automatically solves for candidate physical fork/spacer/sag combinations when ingesting new frames.

5. **Ideal Frame Profiles & Sweet-Spot Scoring (`pkg/ideal`, `pkg/scoring`)**:
   - Frames are evaluated against parameterized sweet-spot ranges defined in [`pkg/ideal/loader.go`](pkg/ideal/loader.go) (`idmatch` biomechanical fit profile).
   - Evaluates criteria across nested ranges:
     - **Ideal (`✓`)**: Scores 100.0 (e.g. BRP Setback 219mm, Handlebar Clamp Fit ≥100 pts, BB Drop 70mm, Trail 98–106mm).
     - **Acceptable (`~`)**: Scores `EdgeScore` (typically 75–80) to 100 with linear ramp towards ideal. Cell displays offset to ideal (e.g. `221 ~ 2 aft`).
     - **Outside (`✗`)**: Soft falloff beyond acceptable to 0 (or floor). Cell displays offset to ideal (e.g. `204 ✗ 15 fwd`).
   - Tables are automatically sorted by score descending (with original `[#]` identifiers preserved).
   - Generates plain-English footer explanations (`Biggest Losses vs. Ideal` and `Head-to-Head Deciding Factors`).
   - Disable scoring with `--no-score`, or inspect sub-scores with `--score-detail`.

6. **Catalogs and Analysis in the Web UI**:
   - Ideal profiles, forks, stems, saddles, saddle swap and handlebars are browsed and tuned in the web app only (see [docs/WEB.md](docs/WEB.md)).

7. **Agent-Friendly CLI Output**:
   - Every command prints Markdown (no `--format` flag). `compare` keeps a many-bike comparison as one wide table.

---

## Quick Start

### Build & Run
```bash
make build
./bin/bikegeo help
```

### CLI (for agents and scripts)

`compare`, `validate`, `list` and `fit` print Markdown.

```bash
./bin/bikegeo help

# List compiled frames, sizes, and supported fork capacities:
./bin/bikegeo list
./bin/bikegeo list --all-sizes

# Compare setups (auto-transposed when >4 columns):
./bin/bikegeo compare havok@110mm@20% hayduke:S3@110mm@20% fenrir:L@rigid
./bin/bikegeo compare fenrir:all@120mm
./bin/bikegeo compare                      # the default baseline
./bin/bikegeo compare +surly-ecr:XL        # add to / remove from the baseline
./bin/bikegeo compare -salsa-fargo:all
./bin/bikegeo compare --score-detail       # sub-score loss breakdown

# Validate frame CAD math across all sizes and configurations:
./bin/bikegeo validate binary-havok
./bin/bikegeo validate --all

# Diagnose or sweep CAD assumptions on a frame:
./bin/bikegeo fit surly-ecr
./bin/bikegeo fit seven-expedition-926f --try fork.a2c=511,531 --try sag=0,20,25
```

### Web (for people)

```bash
make serve                           # build, serve on http://127.0.0.1:8080, open the browser
./bin/bikegeo serve --port 0 --open  # or choose a free port
```

Compare matrices with zone-coloured cells, sticky headers and linked metric definitions; frame and validation pages; and catalog and analysis pages for ideals, forks, stems, saddles, saddle swap and handlebars, each with a form for every input. Everything is a plain `GET` URL (`/?col=fenrir:L@110mm@20%25&col=hayduke:S3@110mm@20%25`) that you can bookmark or share. It listens on `127.0.0.1` only and uses no JavaScript. See [docs/WEB.md](docs/WEB.md).

---

## Running Verification & Tests

```bash
go vet ./... && go test -v ./...
```
