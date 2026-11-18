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

---

## Turn 2 — ChatGPT (2026-11-18)

### 1. Human Guidance Applied

- Matt asked ChatGPT to take its turn under the revised protocol and explicitly bring its earlier, substantially similar design proposal into the comparison.
- That earlier proposal recommended a visual workbench, component assembly over a small geometry vocabulary, a strict TypeScript core shared by browser and Node, SVG/Svelte, and separate chart reconstruction, fit search, and frame ranking. These were recommendations, not human-approved choices.
- No architectural question has been answered this turn. This is initial queue triage, with detail concentrated on assembly/inspection and solver policy. The checkout was clean; no existing edits needed incorporation.

### 2. Points of Convergence

- Agree with Turn 1 on readable physical construction through poses and named components, a visual workbench from the start, and avoiding a general CAD framework.
- Agree that drawing and measurement should use the same placed geometry, and that forward evaluation should support both searches and the frame chooser. I do not endorse the stronger requirements to emit rendering output on every search evaluation or eliminate every inverse solution.
- Agree with the Q4 recommendation to support structured catalog data without recompilation. The format can wait until the import/save workflow exists.
- Q1 remains a choice: I recommend all-TypeScript for this clean slate. Q2 now has comparable sketches below. For Q3, I recommend a smaller first milestone than a workbench with overlays and chart diagnostics.

### 3. Critique, Ideas & Code Sketches

#### Assembly: share physical results; keep inspection inexpensive to omit

My earlier proposal and Gemini's agree that a stem should be constructed once, then measured and drawn from that construction. The difference is what a builder must produce on each evaluation.

- **Gemini:** A physical step moves the pose and emits a drawable primitive plus trace.
- **ChatGPT:** A component function returns its placed geometry and landmarks. The renderer projects that result; optional construction recording explains the same steps when requested. A search need not construct SVG, labels, decorative shapes, or a full trace for every rejected candidate.

This is already partly present in [PlaceStem](../../internal/assembly/stem.go) and [buildBikeSVG](../../internal/web/bike_diagram.go): the latter consumes scene landmarks rather than independently recalculating the stem. Turn 1's account of three copies of all geometry overstates the current duplication. We can improve construction explanations without replacing an existing shared-placement pattern with obligatory drawing side effects.

These are schematic API alternatives for the **same stem**, using TypeScript notation only to make comparison compact. They are not runnable implementations or a language decision. Both start at a head-tube-top pose pointing up the steerer; positive turns are counterclockwise, and stem angle is measured from perpendicular to that steerer.

**A. Immutable poses inside a component function:**

```ts
function placeStem(headTubeTop: Pose, s: StemSetup) {
  const coverTop = headTubeTop.advance(s.upperCoverMm);
  const spacerTop = coverTop.advance(s.spacersMm);
  const steererClamp = spacerTop.advance(s.clampHeightMm / 2);
  const barClamp = steererClamp
    .turn(-90 + s.angleDeg)
    .advance(s.lengthMm);

  return {
    headTubeTop, coverTop, spacerTop,
    steererClamp, barClamp,
  };
}
```

**B. A scoped cursor that also records construction:**

```ts
function placeStem(headTubeTop: Pose, s: StemSetup) {
  const rig = new ScopedRig(headTubeTop);
  rig.mark("headTubeTop");
  rig.advance("upper cover", s.upperCoverMm);
  rig.mark("coverTop");
  rig.advance("spacers", s.spacersMm);
  rig.mark("spacerTop");
  rig.advance("half clamp height", s.clampHeightMm / 2);
  rig.mark("steererClamp");
  rig.turn("stem angle from perpendicular", -90 + s.angleDeg);
  rig.advance("stem length", s.lengthMm);
  rig.mark("barClamp");
  return rig.finish();
}
```

I prefer A as the default: named local values make branching and attachment clear, and the current [pose implementation](../../internal/assembly/pose.go) already demonstrates the approach in Go. B makes step recording explicit and may read better to Matt. A can also record steps through an optional recorder without making poses mutable or copying a scene on every movement. The essential test is that recording on/off produces the same landmarks; there must not be separate physical implementations for search and inspection.

At the bicycle level, either style belongs behind calls such as `placeStem(headset.steeringAxis, settings)` and `placeSaddle(seatpost.clamp, settings)`. Keep mathematics in small geometry helpers and component definitions where needed, without promising that every component can be reduced to a fixed-size trigonometry-free recipe.

Two concrete modeling boundaries matter:

- A construction path is not always the physical outline. Advancing half a clamp height reaches its center; it does not describe the full clamp body. Retain component dimensions for drawing, and distinguish construction guides from physical shapes.
- Attachment does not automatically mean inheriting every orientation. [HandlebarSetup](../../internal/assembly/handlebar.go) already defines roll relative to the leveled world, not the stem extension. Preserve that explicit convention and the lateral grip coordinate, even if Milestone 1 shows only a side view.

#### Solvers: forward verification, without a blanket ban on direct solutions

I agree with forward assembly as the shared evaluator. I would choose the search method per problem: enumerate catalog parts, use bracketed scalar search where applicable, and retain small direct geometric solutions when they simplify the work. The current [saddle solvers](../../internal/geometry/saddle_solve.go) already forward-check their solved settings. This is an alternative to the forward-only policy, not a proposed expansion into a general optimization framework.

Corrections and qualifications to Turn 1:

| Claim | Consequence for the design |
| --- | --- |
| Bisection reaches 0.001 mm in 15 iterations. | The bound depends on the initial interval. A 200 mm interval has width `200 / 2^15 ≈ 0.0061 mm`; 18 halvings yield about `0.00076 mm`. This is a parameter-interval bound, not automatically a landmark-error bound. Require a continuous residual and a sign-changing bracket, handle unreachable targets, and verify the final measurement. A unique branch or monotonicity must be established before claiming a unique answer. |
| Front and rear cockpits are independent once the frame is leveled. | Their placements can be calculated separately, but their fit objectives can be coupled. A hand target relative to the saddle moves when the saddle moves. Independent best choices are justified only for separable objectives and constraints; otherwise retain alternatives and assess combined builds. |
| Solve chart residuals to zero. | Published values are rounded and may be inconsistent or insufficient. Preserve original observations and precision, evaluate agreement within their budgets, and return multiple hypotheses or unresolved quantities when warranted. Do not force all chart centers to coincide. |
| The chart's implied ground line is `Y = -BbDrop`. | In bikegeo's BB-relative, positive-up coordinates, equal-radius wheel axle centers are at `Y = +BbDrop`; the ground is lower by wheel radius. [EvaluateChartClosure](../../internal/geometry/scene.go) and [buildBikeSVG](../../internal/web/bike_diagram.go) distinguish these. State coordinate and measurement conventions in the new model too. |
| The searches take microseconds, milliseconds, or about one second. | No benchmark accompanies these figures. The stated cockpit count is correct: `9 × 8 × 3 × 6 × 5 = 6,480`. Across 200 sizes and 3 fork states that is `3,888,000` cockpit evaluations before additional rear-cockpit combinations, reconstruction, scoring, or tracing. Timings remain unknown. |

The coupling is visible in [ApplyIdealCockpitTargets](../../internal/canonicalizer/canonicalizer.go), which selects a saddle and then remaps cockpit distances. That documents the current staged policy; it does not prove that policy minimizes every possible combined objective. For the reboot I recommend explicit feasible limits and the existing explainable points-lost approach, with selection verified against a small exhaustive example before optimizing search speed.

Chart reconstruction and rider fit should have separate inputs and explanations even if they reuse numerical primitives. A chart solver explains observations under hypotheses; a fit solver selects usable hardware and settings against a stated target. Frame ranking reuses the latter and reports its search coverage. A numerical best result must not be reported as a uniquely identified frame or a proven global optimum without the corresponding evidence.

Preserve measurement state as well as landmark names. [RIDER_FIT.md](../RIDER_FIT.md) establishes that Matt's saddle transfer was measured with the source post unsagged, and that SIT is a saddle support landmark rather than an independently measured pelvis position. My earlier conversational use of "sit bones" was too loose for that distinction. Do not silently reinterpret either fact when introducing compression scenarios or target overlays.

For the solver milestone, I propose three small checks rather than a performance promise: a scalar solve with reachable and unreachable targets; a coupled saddle/hand example checked against exhaustive enumeration; and a chart with two indistinguishable physical hypotheses. Benchmark a declared representative workload with tracing off and on after implementation is authorized. No experiment was executed in this turn.

#### Initial queue triage and first milestone

- **Q1:** My clean-slate recommendation remains strict TypeScript, an SVG workbench with Svelte, and the same core functions used from Node for agent access. This prioritizes one language and direct browser evaluation. Go with a browser frontend remains viable if reuse and distribution outweigh that preference. Neither choice removes all boundaries: [Web Workers](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API/Using_web_workers) run outside the UI thread and communicate through messages. Sharing source code is the advantage; "zero serialization boundary" is too broad a promise. [Strict TypeScript](https://www.typescriptlang.org/tsconfig/strict.html) and [Svelte](https://svelte.dev/docs/svelte/overview) support the proposed implementation approach, but do not establish its performance or make external input validation unnecessary.
- **Q2:** Prefer immutable poses within named component functions, with optional recording; use the two stem sketches above to judge readability. The recording/output boundary still needs Gemini's review.
- **Q3:** Start with one known bike: side view, named landmarks, axes, dimensions, editable fork compression/spacers/stem/saddle settings, and selectable construction steps. Declare its wheel sizes and orientation conventions. Defer comparison overlays and chart diagnostics. Matt should be able to predict and inspect an adjustment; independent fixtures must check lengths, directions, and contact placement. The UI should not claim a full 3D bike or manufacture unknown dimensions.
- **Q4:** Agree on data ingestion; defer the file format and persistence design until the first import/save workflow. Use one explicit fixture for Milestone 1.

I have outlined a proposed milestone sequence in DESIGN.md: one-bike inspection, one-bike fit search, chart reconstruction with agent access, then frame comparison. This covers `(a)`–`(e)` while keeping the first build independent of unresolved solver internals. It awaits review and human approval.

### 4. Proposed Rulings for the Human

- **Q1:** Recommend all-TypeScript for the reboot; the Go alternative favors preserving its implementation and distribution. This choice unblocks the concrete runtime boundary and implementation prototypes.
- **Q2:** Recommend the immutable-pose recipe above; Matt's readability judgment is the relevant decision. Gemini can first review optional tracing and component outputs without needing a language ruling.
- **Q3:** Recommend the smaller one-bike milestone above. Approval fixes the initial visual acceptance scope; it does not authorize implementation by itself.
- **Q4:** Deferred to the import/save milestone; no answer is needed for the first workbench.

No new queue questions were added. The remaining solver disagreements are technical review topics for Gemini, not requests for Matt to decide numerical correctness.
