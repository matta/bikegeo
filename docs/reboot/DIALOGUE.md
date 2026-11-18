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

---

## Turn 3 — ChatGPT (2026-11-18)

### 1. Human Guidance Applied

- Matt explicitly chose Go: "Okay, record the choice of Go as an implementation language. include rationale."
- Recorded the Go core decision in DESIGN.md and closed Q1. The discussion did not settle the browser language/framework, interaction richness, or exact transport/rendering boundary; these remain part of Q3.
- Matt also clarified that choosing a language cannot depend on comparative measurements from implementations we have not built. This decision uses architectural trade-offs rather than a benchmark prerequisite.

### 2. Points of Convergence

- ChatGPT accepts the Go choice and revises its earlier TypeScript recommendation in light of the UX discussion. Gemini's case for Go emphasized Matt's familiarity, value semantics, computation, and tooling.
- Visual-first development is compatible with a Go core. Inspecting existing geometry in a browser does not require reimplementing the physical model there.

### 3. Critique, Ideas & Code Sketches

The key clarification was that "rich browser UX" covers different activities. Highlighting components, displaying construction explanations, aligning existing overlays, and pan/zoom can use already-loaded geometry. Editing a physical setting can request a new Go evaluation. Continuous dragging with constraint solving would strengthen the case for browser-side computation, but that experience has not been selected.

Go therefore gives greater weight to Matt's ability to understand the implementation while providing suitable value types, testing tools, native execution, and concurrency for solver work. The cost is a browser/core boundary, which we accept without promising specific localhost latency or solver throughput. Neither a terminal-first design nor retention of the existing architecture follows from the language choice.

The decision does not endorse all numerical claims in the chat discussion. Separately precomputing front/rear placements does not establish that a coupled fit objective can select them independently, and estimated runtime figures are not measurements. Solver method remains an open technical review topic.

No new API sketches or implementation changes are needed to record this ruling. DESIGN.md contains the rationale and scope; earlier turns remain intact as the history of the alternatives.

### 4. Proposed Rulings for the Human

- **Q1:** Resolved: Go for the implementation core.
- **Q2:** Still open: component/pose API style.
- **Q3:** Still open: required browser interactions and initial visual scope, followed by the frontend and browser/core boundary choices they require.
- **Q4:** Still deferred to catalog import/save design.

No further ruling is needed to complete this decision record.

---

## Discussion — Q2 tracing — ChatGPT recording (2026-11-18)

This note captures the chat exchange following Turn 3. Gemini's response was pasted by Matt; it was not a new filesystem turn authored by Gemini. The summary preserves the main points without importing the full transcript.

- **Gemini's revised recommendation:** Prefer value-style poses inside named component functions over a mutable cursor, particularly for branching from a common attachment. Return intermediate poses and setup dimensions so the renderer can use supplied orientations rather than reconstruct component placement. Accept ChatGPT's distinction between construction paths and physical outlines, and keep drawing out of the placement operation.
- **Matt's requirement and suggestion:** Retain traceable geometry computations. Optional/debug tracing is a suggested way to balance inspection with search efficiency; Matt has not yet approved the entire Q2 API.
- **ChatGPT's response:** Agree on value-style poses, placement results, and separate rendering. Intermediate poses show where geometry ended up; an optional recorder should also explain the operations that produced it. Record structured inputs/results and labels during the actual calculation, rather than separately reconstructing explanations afterward.
- **Proposed usage:** Search without collecting traces, then re-evaluate an inspected candidate using its exact component choices and settings with tracing enabled. The same inputs must produce the same geometry in both modes. Tracing should be available in the ordinary workbench, without requiring a special developer build. No promise of zero allocations is established.
- **Remaining refinement:** Gemini recommends pose-valued landmarks throughout; ChatGPT recommends retaining orientation where meaningful, with points for position-only landmarks. Optional recording does not require a shared mutable movement cursor. The API details and tracing mechanism remain open under Q2; Q3's browser interaction scope is also open.

DESIGN.md now reflects this exchange. The earlier proposal requiring every movement to emit a drawing primitive is no longer Gemini's current recommendation.

---

## Discussion — Q2 embedded trace proposal — ChatGPT recording (2026-11-18)

- **Matt's clarification:** The design is not settled; review Gemini's latest response as a proposal. Q2 stays open. The confirmed desire for traceability does not approve a particular representation or tracing mechanism.
- **Gemini's proposal, relayed by Matt:** Every placed landmark retains a Pose. Labeled movement methods return new poses while propagating an optional `*Trace`; when enabled, they record the operation and its source/result. Render physical shapes in the supplied local frames. Gemini argues that this preserves both readable branching and automatic construction explanations.
- **ChatGPT's position:** Favor poses for component attachments and physical landmarks, revising the earlier casual suggestion to discard orientation where only position is immediately needed. A BRP inherits a specified construction frame, however, not knowledge of the saddle surface: a rail-normal direction and a surface normal are not interchangeable. Keep raw points available internally to geometry helpers; callers should not reconstruct component orientations through scattered trigonometry.
- **Embedded sink:** This is a practical candidate that preserves a short recipe. Geometry remains value-style while tracing has shared mutable state. Scope recording to one evaluation, with a separate sink for each concurrently traced build. Finish/detach it before drawing so constructing a tick mark does not modify the assembly history. The drawing must not change the physical model or its explanation.
- **Trace contents:** Record geometry snapshots without the live sink. To promise an exact construction tree, retain source/result identities rather than infer parentage from equal poses; distinct construction paths can reach the same coordinates and orientation. A small recording-only reference is enough; no general graph framework is proposed.
- **Efficiency wording:** Nil tracing can skip record collection and formatting. It is not literally zero overhead, and Go does not guarantee stack allocation for a struct solely from its shape; allocation depends on escape analysis and usage ([Go FAQ](https://go.dev/doc/faq#stack_or_heap)). This qualification does not require benchmarks before accepting or rejecting the API.

ChatGPT supports the direction with these implementation conditions. No additional human-confirmed design decision was added; DESIGN.md now explicitly labels Q2 unresolved and preserves these notes for the next review.

---

## Turn 4 — Gemini (2026-11-18)

### 1. Human Guidance Applied
- Matt reviewed the converged Q2 notes and approved resolving Q2 (`"lgtm"`).
- Recorded the confirmed Q2 decision in [DESIGN.md](DESIGN.md) (`(Turn 4, resolves Q2)`) and closed Q2 in the Decision Queue.

### 2. Points of Convergence
- **Q2 (Turtle / Assembly API Shape):** Full agreement between Gemini and ChatGPT, confirmed by Matt:
  1. **Value-style `Pose` methods** (`Advance`, `Translate`, `Turn`) inside named component functions (`PlaceStem`, `PlaceSaddle`, `PlaceHandlebar`, etc.).
  2. **Pose-valued landmarks and attachments** on placement structs, with explicit coordinate-frame meanings for each landmark (e.g., `BRP` and `SitBones` carry the saddle rail construction frame, not an implicit curved saddle-surface normal). Raw point coordinates remain internal to geometry helpers and final scalar readouts (`X()`, `Y()`, `DistanceTo`), preventing callers and renderers from writing scattered trigonometry.
  3. **Optional `*Trace` sink + `stepID` on `Pose`:** Propagated across derived poses during assembly, recording sink-free geometry snapshots and explicit `ParentID -> ID` branch identity when non-nil, and skipping record collection when `nil`.
  4. **Freeze/detach trace before rendering:** Assembly finishes/freezes its trace before returning (or detaches the sink) so downstream rendering helpers (such as drawing tick marks or dimension callouts from a landmark `Pose`) cannot append to the model's construction history.
  5. **Separate placement from physical outline projection:** Component functions return placement structs (`Pose`s + setup dimensions); rendering uses those local `Pose` frames and dimensions to project physical shapes without recalculating orientations.

### 3. Critique, Ideas & Code Sketches

#### Compact Go Reference Sketch for Q2 (`Pose` + `Trace`)

To capture ChatGPT's branching-identity (`stepID`) and trace-freezing conditions without heavy machinery:

```go
// Snapshot is a sink-free 2D coordinate frame stored inside Trace steps.
type Snapshot struct {
    Origin Point2
    AxisX  Vec2
}

type Step struct {
    ID       int
    ParentID int
    Label    string
    Op       string // e.g. "advance", "translate", "turn"
    Amount   float64
    AmountY  float64
    From     Snapshot
    To       Snapshot
}

type Trace struct {
    Steps  []Step
    frozen bool
}

func (t *Trace) Freeze() {
    if t != nil {
        t.frozen = true
    }
}

type Pose struct {
    origin Point2
    axisX  Vec2
    trace  *Trace
    stepID int // 0 when untraced; 1..N index into trace.Steps when traced
}

func (p Pose) Advance(label string, distMm float64) Pose {
    next := Pose{
        origin: p.origin.Add(p.axisX.Scale(distMm)),
        axisX:  p.axisX,
        trace:  p.trace,
        stepID: p.stepID,
    }
    if p.trace != nil && !p.trace.frozen {
        id := len(p.trace.Steps) + 1
        p.trace.Steps = append(p.trace.Steps, Step{
            ID:       id,
            ParentID: p.stepID,
            Label:    label,
            Op:       "advance",
            Amount:   distMm,
            From:     p.Snapshot(),
            To:       next.Snapshot(),
        })
        next.stepID = id
    }
    return next
}
```

#### Next Topic Up: Q3 (Visualization & Browser Interaction Scope for Milestone 1)
With `Q1` (Go core) and `Q2` (`Pose` + optional `Trace` assembly) settled, `Q3` is the sole remaining decision blocking Milestone 1:
- **Where we already agree on Q3:** Milestone 1 should render a side-view visual workbench of a known bike from Go-supplied geometry, showing physical outlines, named landmarks, local coordinate axes, dimensions, and selectable construction trace steps, with controls to adjust component settings (fork compression, spacers, stem, saddle) and inspect the resulting build.
- **What remains to decide in Q3:**
  1. **Initial scope:** Keep Milestone 1 strictly to a single-bike workbench (ChatGPT's recommendation, deferring multi-bike ghost overlays and chart-residual views to later milestones) vs. including a two-bike ghost overlay in Milestone 1. *Refinement:* I am happy to endorse ChatGPT's narrower single-bike scope for Milestone 1 so the first build stays small and focused; ghost overlays fit naturally into Milestone 2/4 once single-bike rendering is solid.
  2. **Browser interaction & frontend stack:** Form/control-driven settings (sliders/inputs that request a fresh Go evaluation and update the SVG + trace inspector) vs. direct canvas manipulation, and how light to keep the frontend (e.g., server-rendered/fetched SVG + minimal vanilla JS/TS vs. a reactive frontend framework like Svelte).

### 4. Proposed Rulings for the Human
- **Q2:** Resolved (`Turn 4`).
- **Q3:** Now the only active item in the Decision Queue blocking Milestone 1. Recommend confirming the **single-bike workbench scope** for Milestone 1 and choosing the preferred **browser interaction model / frontend weight**.

---

## Discussion — Q3 Go-rendered browser interface — ChatGPT recording (2026-11-18)

- **Incoming context:** Turn 4 already records Matt's approval of Q2. This discussion addresses Q3; it does not reopen the Pose/Trace decision.
- **Gemini's recommendation, relayed by Matt:** For dropdowns, number fields, tabs, and an inspectable SVG, render HTML/SVG in Go and use a small dependency-free JavaScript layer for fetched updates, trace highlighting, and pan/zoom. Embed assets with Go and avoid a frontend framework/build pipeline.
- **Matt's ruling:** "I'm liking what Gemini had to say and I'm agreeing with it." Recorded approval of this frontend approach and control-driven interaction model. The pasted proposal did not explicitly settle the complete single-bike milestone feature list, so the scope portion of Q3 remains open.
- **ChatGPT's agreement:** This fits the stated interface and Matt's familiarity with Go. Shared attributes can link SVG elements and trace rows without exporting the whole domain model into frontend types. Preserve ordinary forms/links and progressively enhance them; keep the geometry and physical drawing implementation in Go.
- **Implementation notes:** Do not impose the proposed 60–100-line count or timing estimates as requirements. Handle superseded requests, invalid/incomplete input, focus, and URL history; replace diagram and trace consistently. Keyboard selection should accompany hover/click, and step IDs must be interpreted within the current build. Browser cancellation and history events support this approach ([AbortController](https://developer.mozilla.org/en-US/docs/Web/API/AbortController), [popstate](https://developer.mozilla.org/en-US/docs/Web/API/Window/popstate_event)).
- **Testing correction:** Go's HTTP tests and HTML/SVG assertions verify server behavior and output structure; they do not execute browser JavaScript ([httptest](https://pkg.go.dev/net/http/httptest)). A short actual-browser interaction check is still needed when implementing the frontend and may initially be manual. No new testing framework is required by this decision.
- **Comparison qualification:** Frameworks can serve small applications too, and mirroring every Go type is not inevitable. Those broad claims are unnecessary to justify a small plain-JavaScript interface here. This is agreement with the architectural recommendation, not endorsement of its exact size/performance predictions.

DESIGN.md records the confirmed frontend choice and keeps the remaining Q3 milestone-scope question separate. No implementation was started.

---

## Turn 5 — Gemini (2026-11-18)

### 1. Human Guidance Applied
- Matt approved (`"yes"`) the remaining portion of **Q3 (Milestone 1 single-bike visual workbench scope)** and the **4-Milestone Sequence** (`M1: One-Bike Visual Workbench` → `M2: One-Bike Fit Search` → `M3: Chart Reconstruction, Catalog Persistence & Agent Access` → `M4: Multi-Frame Chooser & Comparison`), including the explicit requirement that Milestone 1 construct the frame and fork from published chart numbers + declared fork (and level the frame when fork/sag settings change) as traceable `Pose` steps rather than hardcoding pre-leveled frame coordinates.
- Recorded this confirmed decision in [DESIGN.md](DESIGN.md) (`(Turn 5, resolves Q3)`) and closed `Q3`, emptying the active Decision Queue for Milestone 1.

### 2. Points of Convergence
- **Q3 (Milestone 1 Visual Scope & Frontend Boundary):** Both agents and Matt are aligned on the single-bike visual workbench rendered as HTML/SVG in Go with a small dependency-free vanilla JS layer for control updates, trace step selection/highlighting, and pan/zoom.
- **Milestone 1 Chassis & Cockpit Scope:** Milestone 1 covers the full single-bike physical assembly in four traceable stages:
  1. **Unpitched chart frame:** Place `BB`, `RearAxle`, and `HeadTubeTop` from published chart dimensions (`Stack`, `Reach`, `HTA`, `HTL`, `BB Drop`, `Chainstay`), plus the seat-tube axis (`STA` / bent-seat-tube offset if present).
  2. **Fork & lower cup placement:** Walk down the steerer axis from `HeadTubeTop` through `HeadTubeBottom`, `LowerCupBottom`, and fork `EffectiveA2C` (`StaticA2C - Travel * SagPct`), then turn `90°` forward perpendicular to the steerer by `ForkRake` to place `FrontAxle` (keeping chart closure vs. declared CAD fork explicit).
  3. **Frame leveling:** Pitch the rigid frame around `RearAxle` when fork length, lower cup, sag, or rake changes so both wheel axles sit at their leveled heights, and return `BB` to `(0, 0)`.
  4. **Rear & front cockpits:** Place the seatpost and saddle (`PlaceSaddle` → `SeatpostClamp`, `RailCenter`, `BRP`, `SitBones`) and the steerer stack, stem, and handlebar (`PlaceStem`, `PlaceHandlebar` → `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HandGrip`).
- **4-Milestone Sequence:** Confirmed (`M1` → `M2` → `M3` → `M4`), with `Q4` (catalog file format & persistence) and solver internal details deferred to their respective milestones (`M3` and `M2`).

### 3. Critique, Ideas & Code Sketches
- No new objections. All active questions blocking Milestone 1 (`Q1`, `Q2`, `Q3`) are now resolved in [DESIGN.md](DESIGN.md).
- One minor point for ChatGPT if Matt wants a quick final review before implementation handoff: confirm there are no objections to including the traceable chassis/fork placement and frame-leveling steps inside Milestone 1's single-bike assembly (preserving the existing chart-baseline vs. declared-fork distinction from `docs/kinematic/README.md`).

### 4. Proposed Rulings for the Human
- **Active Decision Queue:** Empty (`Q1`, `Q2`, and `Q3` resolved; `Q4` deferred to Milestone 3).
- Ready for final sign-off / implementation handoff for Milestone 1 (or a quick Turn 6 check from ChatGPT if desired).

---

## Discussion — Q5 implementation isolation and transition — ChatGPT recording (2026-11-18)

- **Matt's concern:** The product design is settled, but implementation organization is unspecified: greenfield versus reuse, old/new package isolation, executable naming, and eventual coexistence or replacement. The prior milestone list did not answer these operational questions.
- **ChatGPT's recommendation:** Keep old packages and `cmd/bikegeo` in place. Create an independent production tree under `internal/reboot/` and a temporary `cmd/bikegeo-reboot` / `bin/bikegeo-reboot`. Use the same Go module, with no production imports in either direction between the implementations. Do not relocate everything into `internal/legacy/` or link both engines into one command.
- **Reuse policy:** Carry forward source observations, provenance, physical conventions, and independent tests. Audit and port useful small primitives into the new packages as needed; do not depend on legacy types through runtime adapters. Test-only comparisons can use both implementations. Copy one sourced input fixture for Milestone 1 and derive its geometry; defer full catalog migration until Q4 is settled.
- **Concrete execution proposal:** Added IMPLEMENTATION.md covering package ownership, separate run/build commands and ports, import checks, existing verification/dead-code requirements, and five runnable slices within the confirmed Milestone 1 scope. The first slice draws a stem with a trace; subsequent slices build the complete chassis, leveling, cockpits, and interactive inspector. This is an incremental route to the approved milestone, not a smaller replacement scope.
- **Cutover proposal:** After Matt decides the reboot meets his needs, preserve the old revision in Git, explicitly retain/migrate useful data and workflows, retire old production packages, and promote the reboot to the canonical `bikegeo` executable and package paths. No automatic retirement follows from completing Milestone 1.
- **Status:** Q5 is a new implementation-organization decision, not a reopening of Q1–Q3 or the four approved milestones. The arrangement remains proposed, with Matt's naming refinement below incorporated. No code was moved, no build wiring changed, and no implementation was started.
- **Matt's naming refinement during this exchange:** Use `bikegeo-reboot` to match `internal/reboot/`, replacing ChatGPT's initial suggestion of `bikegeo-next`. The proposal now uses `cmd/bikegeo-reboot`, `bin/bikegeo-reboot`, `build-reboot`, and `serve-reboot` consistently. The rest of the implementation arrangement remains under discussion.

---

## Turn 6 — Gemini (2026-11-18)

### 1. Human Guidance Applied
- Matt approved **Q5** ([IMPLEMENTATION.md](IMPLEMENTATION.md)) with three specific requirements:
  1. **Initial bike fixture:** Use the **Otso Fenrir Stainless** (`otso-fenrir-stainless`) as the Milestone 1 bike fixture.
  2. **Strict self-containment with `*_parity_test.go` exception:** Under `cmd/bikegeo-reboot` and `internal/reboot/...`, **only files named `*_parity_test.go`** may import legacy `bikegeo/internal/...` packages. All production `.go` files and ordinary `*_test.go` files must be 100% self-contained so deleting `*_parity_test.go` and the old codebase at cutover requires zero edits to the permanent reboot test suite.
  3. **Pre-commit enforcement:** Add shell + regex boundary checks (alongside `go list -deps`) to `make check` / `make pre-commit` in Slice 1 to enforce this import rule mechanically.
- Recorded this confirmed decision in [DESIGN.md](DESIGN.md) (`(Turn 6, resolves Q5)`) and updated [IMPLEMENTATION.md](IMPLEMENTATION.md).

### 2. Points of Convergence
- Full agreement between Matt, ChatGPT, and Gemini on **Q5** ([IMPLEMENTATION.md](IMPLEMENTATION.md)).
- The **Otso Fenrir Stainless** fixture is an ideal Milestone 1 testbed because it exercises the CAD rigid fork baseline (`495 mm` A2C, `54 mm` rake, `0 mm` CAD lower cup), the physical `12 mm` EC44 lower cup delta, and both rigid and `100 mm`/`120 mm` suspension forks (`506 mm` / `530 mm` static A2C, `44 mm` rake, sag) on a single frame.

### 3. Critique, Ideas & Code Sketches
- Added the concrete `find` + `grep -HnE` shell/regex boundary checks to [IMPLEMENTATION.md §4](IMPLEMENTATION.md) so Slice 1 can wire them directly into `Makefile` (`make boundary` inside `make check`).

### 4. Proposed Rulings for the Human
- **Active Decision Queue:** Empty (`Q1`, `Q2`, `Q3`, and `Q5` are all confirmed; `Q4` is deferred to Milestone 3).
- Design and implementation transition planning are complete and ready for Milestone 1 Slice 1 whenever authorized.

---

## Turn 7 — Gemini (2026-11-18)

### 1. Human Guidance Applied
- Matt approved four final specification details before starting Milestone 1 implementation:
  1. **Rooted `Pose` tree for frame pitch & leveling:** Confirmed the algebraic identity that rotating a rigid frame around `RearAxle` by `PitchDeg` and recentering `BB` to `(0, 0)` is identical to rotating the chart frame around `BB (0, 0)` by `PitchDeg` (`frameRoot := bb.Turn("frame pitch from fork/cup/sag change", PitchDeg)`). Specified this construction in [DESIGN.md](DESIGN.md) so the entire leveled bike (chassis, fork, seatpost, stem, handlebar) is built as a single unbroken `Pose` tree in the exact leveled world coordinate system rendered by the SVG.
  2. **100% solver-free Milestone 1:** Confirmed that Milestone 1 uses the direct forward physical input `BBToClampAlongSeatAxisMm` (default `753.5 mm`, yielding `|BRP - BB| ≈ 746.0 mm` on the Otso Fenrir L with the SQlab 610 saddle) rather than embedding a saddle-height solver in Milestone 1.
  3. **Explicit Otso Fenrir Stainless default numbers:** Added the exact published chart numbers, CAD fork baseline (`495/54` rigid, `0 mm` CAD lower cup), physical `12 mm` EC44 lower cup, 3 selectable forks, and front/rear cockpit defaults to [IMPLEMENTATION.md §2](IMPLEMENTATION.md).
  4. **Self-contained Coordinate & Sign Conventions table:** Added the full coordinate, angle, stem, handlebar, and saddle sign-convention table to [DESIGN.md](DESIGN.md) so `docs/reboot/` requires no cross-referencing to legacy docs.

### 2. Points of Convergence
- The Milestone 1 specification in [DESIGN.md](DESIGN.md) and [IMPLEMENTATION.md](IMPLEMENTATION.md) is now self-contained and unambiguous across all five slices.

### 3. Critique, Ideas & Code Sketches
- None needed; all four pre-implementation gaps are specified.

### 4. Proposed Rulings for the Human
- **Active Decision Queue:** Empty. Ready for Milestone 1, Slice 1 implementation whenever Matt authorizes it.

---

## Discussion — Fork-rake turn sign correction — Gemini recording (2026-11-18)

- **ChatGPT's catch (relayed by Matt):** In Turn 7's leveled chassis description ([DESIGN.md](DESIGN.md) §1.2), walking down the steerer (`steererDown`, heading `-HTA ≈ -68.25°`) and turning `-90°` would point to `-158.25°` (rearward and down). Because positive turns are counter-clockwise, turning forward perpendicular from the downward steerer requires **`+90°`** (`-HTA + 90° = 90° - HTA ≈ +21.75°`, or local `+Y`), whereas `-90°` applies when facing *up* the steerer (`180° - HTA - 90° = 90° - HTA`, as in `PlaceStem`).
- **Fix applied:** Corrected `-90°` to `+90°` in [DESIGN.md](DESIGN.md) §1.2.

---

## Discussion — Two-pass `Pose` recipe for `PitchDeg` — Gemini recording (2026-11-18)

- **ChatGPT's feedback (endorsed by Matt):** Computing `PitchDeg` via closed-form formulas (`L_chart`, `ΔL`, `ΔRake`, `atan2`) outside the `Pose` tree reintroduces the scattered trigonometry that requirement `(a)` seeks to eliminate, and hides the fork step behind an adjusted length (`L_chart - HTL - PhysicalLowerCup + ΔL`) instead of the actual physical `EffectiveA2C`. Instead, build the chassis and fork at `0°` pitch using the `Pose` recipe (`placeChassisAtPitch`), measure the rear-to-front-axle angle (`PitchDeg = -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)`), and rebuild the leveled chassis with the exact same `Pose` recipe rooted at `bb.Turn("frame pitch to level axles", PitchDeg)`.
- **Gemini's agreement & update:** Completely agree. This eliminates `L_chart`, `ΔL`, and `ΔRake` from chassis assembly, ensures every steerer step in the trace advances by its true physical dimension (`HTL`, `LowerCup`, `EffectiveA2C`, `Rake`), and uses one inspectable `Pose` recipe for both pitch measurement and leveled construction. Updated [DESIGN.md](DESIGN.md) accordingly.

---

## Discussion — World-relative `Pose.Face` for saddle rail pitch and handlebar roll — Gemini recording (2026-11-18)

- **Matt's confirmation:** Confirmed that saddle `RailPitchDeg` and handlebar `BarRollDeg` are relative to the leveled world horizontal (`0°`), not the pitched frame—meaning when a fork change pitches the frame, the rider re-levels the saddle and handlebar to their world-relative pitch/roll.
- **ChatGPT's API catch (endorsed by Matt):** With only relative `Turn`, setting a world-relative heading at `SeatpostClamp` or `BarClamp` would either require subtracting the accumulated seat-tube/stem heading by hand (fragile angle arithmetic) or constructing a fresh root pose at the clamp point (which severs the `ParentID -> ID` trace link). Adding `Pose.Face(label string, worldHeading Deg) Pose` keeps the current pose's position (`Origin`), sets its heading to an absolute world angle, and records a normal traced step linked to its parent (`seatpostClamp.Face("saddle rail pitch vs. world horizontal", saddle.RailPitchDeg)` and `barClamp.Face("bar roll vs. world horizontal", -setup.BarRollDeg)`).
- **Gemini's agreement & update:** Completely agree; added `Face(label, worldHeadingDeg)` to the Q2 `Pose` movement vocabulary and the handlebar/saddle conventions in [DESIGN.md](DESIGN.md).



