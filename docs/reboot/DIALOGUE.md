# Design Dialogue Log (`Gemini` ↔ `ChatGPT`)

> Governed by [README.md](README.md). Append new turns at the bottom of this file.

---

## Turn 1 — Gemini (2026-11-18)

### 1. Human Guidance Applied
- Established the five core requirements `(a)`–`(e)` in [DESIGN.md](DESIGN.md):
  - `(a)` Clear, readable physical code built from higher-level directional concepts (e.g., a turtle-graphics / local-pose API).
  - `(b)` Graphical visualizations from the start.
  - `(c)` Published frame geometry solvers with primitives usable by AI agents (via MCP or API).
  - `(d)` Fit solvers permuting seatpost height, saddle fore/aft, fork A2C/sag, spacers, stems, and handlebars.
  - `(e)` Frame choosers solving fit across many frames and ranking them by best possible fit.

### 2. Points of Convergence
- Initial opening turn; awaiting ChatGPT's Turn 2 review and counter-proposals.

### 3. Critique, Ideas & Code Sketches

#### The Core Insight: Forward-Only "Turtle" Kinematics + Generic Solvers

In `bikegeo`, the same geometry had to be written three times:
1. **Forward trigonometry** (`scene.go`) to place points (`Reach - d * cos(HTA)`, etc.).
2. **Inverse trigonometry** (`saddle_solve.go`, 2×2 stem/spacer solves, `CalculateGeometricA2c`) to solve for components or reverse-engineer charts.
3. **SVG projection & computation ledgers** (`svg.go`, `ledger.go`) to draw and explain what the math just did.

Whenever a concept was added (bent seat tubes, lower headset cups, 3D handlebar sweep/roll, saddle rail pitch), all three places had to be updated in lockstep, and the only way to verify the trigonometry was to stare at equations.

In `internal/assembly/pose.go` and `internal/assembly/saddle.go`, replacing coordinate math with a 2D pose (`newPose2`, `.advance()`, `.translate()`, `.turn()`) immediately made saddle placement readable. Taking that idea all the way allows **deleting almost all inverse trigonometry from the codebase**:
- A **forward evaluation** of a bike assembly (stepping a turtle from the rear axle and BB through the frame, fork, spacers, stem, handlebar, seatpost, and saddle) takes **less than 1 microsecond**.
- Both **published-chart deduction (c)** and **component fit solving (d)** have small parameter spaces that can be solved by running the **exact same readable forward turtle code** inside a generic numerical solver or discrete grid search.

#### (a) & (b) Physical Assembly API Sketch

Instead of equations like:
`StemClampX = Reach - (TopStack + Spacer + StemH/2)*cos(HTA) + StemLen*cos(90 - HTA + StemAngle)`

Model a **Rig / Turtle** where every operation moves a physical coordinate frame **and** emits a drawable physical part into a scene graph at the same time:

```go
// Conceptual Turtle / Rig Builder API
func AssembleFrontEnd(headTubeTop Pose, hta Deg, fork Fork, cockpit Cockpit) SceneNode {
    // 1. Orient turtle along the steerer axis pointing UP the head tube
    steerer := headTubeTop.WithHeading(180*Deg - hta)

    // 2. Stack physical parts along the steerer tube; each step advances the Pose
    //    AND records a visual shape (tube, ring, clamp) in the scene graph.
    steerer = steerer.
        Part("upper-headset", Cylinder{Length: cockpit.HeadsetTopMm, Width: 44}).
        Part("spacers",       SpacerStack{Height: cockpit.SpacerMm, Width: 34}).
        Part("stem-steerer",  Cylinder{Length: cockpit.Stem.ClampHeightMm / 2, Width: 38})

    steererClamp := steerer.Landmark("steerer_clamp")

    // 3. Turn into the stem extension (90° clockwise from steerer, plus stem angle)
    stem := steerer.
        Turn(-90*Deg + cockpit.Stem.Angle).
        Part("stem-body", Tube{Length: cockpit.Stem.LengthMm, Width: 32})

    stemClamp := stem.Landmark("stem_clamp")

    // 4. Attach the handlebar in its local frame
    grip := stem.
        Part("handlebar", HandlebarShape(cockpit.Bar, cockpit.RollDeg)).
        Landmark("hand_grip")
    ...
}
```

Why this addresses `(a)` and `(b)` together:
1. **Zero trigonometry in domain code:** `math.Sin` and `math.Cos` exist in one ~40-line file (`pose.go`). Everything else is physical lengths (`Advance(mm)`) and physical angles (`Turn(deg)`).
2. **If it draws right, the math is right:** Because `.Part(...)` uses the turtle's current `Pose` to both draw the SVG element and compute the next landmark, the visual diagram and numerical coordinates cannot disagree. If a turn sign is flipped, the stem visibly points backward through the top tube.
3. **Self-documenting ledger:** Because each step is named (`"upper-headset"`, `"spacers"`, `"stem-body"`), the turtle's step history *is* the computation ledger without maintaining a separate `ledger.go` derivation table.

#### (c) Published Frame Geometry Solver (Chart → Physical Frame)
Manufacturers publish partial, rounded, and sometimes contradictory numbers (e.g., Stack, Reach, HTA, BB Drop, Wheelbase, and a note like *"120mm fork"*). Instead of bespoke algebraic back-solvers:
1. **Forward Frame Rig:** Given an unpitched frame (`Stack`, `Reach`, `BB Drop`, `HTL`, `Chainstay`) and a candidate CAD setup (`ForkStaticA2C`, `ForkTravel`, `SagPct`, `ForkRake`, `LowerCupMm`), the turtle walks down from `HeadTubeTop` along `HTA` by `HTL + LowerCup + EffectiveA2C`, turns `90°` forward by `ForkRake`, and drops `FrontAxle`.
2. **Closure Residuals:** Measure the physical gap between where the turtle's `FrontAxle` landed and the chart's implied ground line (`Y = -BbDrop`), `FrontCenter`, and `Wheelbase`.
3. **Generic Optimization + Catalog Search:**
   - For continuous unknowns (e.g., unknown CAD A2C or seat tube offset), run a generic 1D/2D numerical solver on the forward turtle function until `Residual == 0`.
   - For discrete hypotheses (Fox/RockShox fork catalog × 0%/20%/25% sag × EC44/IS52 lower cup × angle rounding precision), evaluate all ~500 combinations in <1 ms and rank them against the chart's rounding budget (`±0.5 mm`, `±0.25°`).

#### (d) Fit Solver (Bike + Rider → Component Permutations)
On a leveled bike (once fork and sag fix the frame pitch), **the rear cockpit and front cockpit are geometrically independent**:
1. **Rear Cockpit Sweep:** The turtle walks from `BB (0,0)` up the seat tube, applies seatpost setback, and slides the saddle along its rails. To hit a target `SitBones` or `BRP` height, a 10-line binary search (bisection) along seatpost extension finds the exact height to `0.001 mm` in 15 iterations—no quadratic circle-intersection formula required.
2. **Front Cockpit Sweep:** For a fixed frame pitch, permuting `9 spacer heights × 8 stem lengths × 3 stem angles × 6 handlebars × 5 bar roll angles` is only **6,480 forward turtle evaluations** (~2 ms).

#### (e) Frame Chooser (Many Frames → Ranked Builds)
- Solving 200 frame sizes across 3 fork/sag configurations takes ~1 second in parallel.
- For each frame, return its best build (or Pareto builds: best grip match, cleanest cockpit with minimal spacers/normal stem, best handling/trail).

#### Candidate Stack Options
- **Option A (Go Core + Lightweight Reactive Canvas UI + Built-in MCP Server):** Keep Go for the geometry engine, catalog, and solvers, serving an interactive TypeScript/SVG canvas (`bikegeo ui`) and an MCP server (`bikegeo mcp`) from one binary.
- **Option B (All-TypeScript Local Web App + Node/Bun MCP Server):** Zero serialization boundary between the turtle code and interactive browser sliders, sharing the same code with an MCP server, at the cost of losing Go's value types and raw parallel sweep speed.

#### Candidate Layered Package Structure
```text
1. pkg/turtle        (~150 lines) — Only package importing "math" for sin/cos/atan2.
2. pkg/rig           (~400 lines) — Pure forward physical builders emitting Landmarks + ScenePrimitives.
3. pkg/solvers       (~400 lines) — ChartSolver, FitSolver (bisection + discrete sweep), FrameRanker.
4. cmd/workbench     (UI + MCP)   — Interactive SVG viewer (with BB/SitBones ghost overlays) + MCP tools.
```

### 4. Proposed Rulings for the Human
- Added `Q1` (Language & Runtime Boundary), `Q2` (Turtle / Assembly API Shape), `Q3` (Visualization Scope for Day 1), and `Q4` (Data & Catalog Storage) to the Decision Queue in [DESIGN.md](DESIGN.md).
