# Canonical Design Intent (`bikegeo` Reboot)

> Governed by [README.md](README.md). Only the Human Operator approves decisions; agents may record explicit approval in **Human-Confirmed Decisions**.

---

## 1. Human-Confirmed Decisions

### Core Requirements (Established by Human Operator)
- **(a) Clear, readable physical code:** Physical structures must be built up from higher-level directional concepts (e.g., a turtle-graphics / local-pose API with higher-level component builders such as seatpost, saddle, fork, spacers, stem, and handlebars) rather than raw trigonometric equations scattered across files.
- **(b) Visualizations from the start:** Resulting physical structures must be rendered graphically from day one.
- **(c) Published frame geometry solvers + AI primitives:** Deduce physical geometry from incomplete/rounded manufacturer or custom-builder charts, providing solver primitives that AI agents can invoke via MCP or structured API endpoints.
- **(d) Fit solvers:** Given a fit target, permute seatpost height, saddle fore/aft, fork A2C/sag, headset spacers, stem lengths/angles, and handlebar choices to find good fits.
- **(e) Frame choosers:** Solve fit across many frames and rank them by best possible fit.

### Go implementation core (Turn 3, resolves Q1)

**Decision:** Use Go for the reboot's physical model, component assembly, solvers, and shared application operations. Browser and agent interfaces should use that core rather than duplicate its physical calculations.

**Rationale:**
- Matt already knows Go. His ability to read, review, and debug the physical construction code directly serves requirement `(a)`.
- Go's value types, familiar testing workflow, native execution, and concurrency facilities suit geometric modeling and computational searches. This is an architectural choice without comparative benchmarks, not a claim of a measured speed advantage or a commitment to brute force for every problem.
- Visualization from the start does not require browser-side geometry. Selecting components, inspecting construction steps, toggling overlays, and panning/zooming can operate on geometry already supplied by Go. Changed physical settings can request a new evaluation.
- At language selection, the desired richness of browser interaction was still open. The Go choice avoided assuming a need for continuous direct manipulation of geometry in the browser; the later Q3 decision selects a control-driven inspector.
- A browser/core boundary is an accepted trade-off for keeping the model in a language Matt knows and retaining straightforward Go tooling and distribution. The earlier all-TypeScript proposal favored direct browser evaluation and one language across the application; those benefits do not outweigh the Go rationale for this choice.

**Scope:** This settles the core language. The subsequent browser/frontend choice and Milestone 1 scope are recorded below under Q3. Existing code is useful reference material, not a requirement to preserve the current architecture.

### Pose-based component assembly with optional embedded tracing (Turn 4, resolves Q2)

**Decision:** Express physical component assembly using immutable, value-style `Pose` operations inside named component functions (`PlaceStem`, `PlaceSaddle`, `PlaceHandlebar`, etc.), returning `Pose`-valued landmarks alongside component setup dimensions, with an optional embedded `*Trace` sink on `Pose` to record construction steps for inspection.

**Key Design Conditions:**
1. **Value-style `Pose` movements:** Movement methods (`Advance`, `Translate`, `Turn`) return new `Pose` values so branching from a common attachment (such as a saddle's `RailCenter` or a frame's `HeadTubeTop`) is natural and cannot mutate prior poses.
2. **Pose-valued landmarks with explicit coordinate-frame meanings:** Placement structs retain `Pose`s for attachments and physical landmarks rather than bare points, preventing callers and renderers from reconstructing orientations via scattered trigonometry. Each landmark's local coordinate axes must have a documented physical meaning (e.g., `BRP` and `SitBones` carry the saddle-rail construction frame, not a curved saddle-surface normal). Raw points remain internal to low-level geometry helpers and final scalar readouts (`X()`, `Y()`, `DistanceTo`).
3. **Optional `*Trace` sink with explicit step parentage:** A `Pose` carries an optional `*Trace` pointer and `stepID int`. When tracing is `nil` (such as during solver searches), movement methods skip record collection and string formatting; when non-nil (such as when inspecting a build in the workbench), they record sink-free geometry snapshots (`From`, `To`), operation labels, inputs, and explicit `ParentID -> ID` provenance using the exact same calculation path.
4. **Freeze/detach trace before rendering:** Each traced evaluation uses its own `Trace` instance and freezes or detaches recording when assembly completes, ensuring that downstream rendering helpers (e.g., offsetting from a landmark `Pose` to draw a tick mark or dimension callout) never append to the assembly history.
5. **Separate placement from outline projection:** Moving along a construction path (such as advancing half a stem clamp height to reach its center) is distinct from a part's full physical outline. Component functions return placement geometry and setup dimensions; the renderer projects physical shapes in those local `Pose` frames without recalculating angles.
6. **Verification:** Identical inputs must produce identical placed geometry whether tracing is enabled or disabled.

### Go-rendered HTML/SVG workbench & 4-Milestone Sequence (Turn 5, resolves Q3)

**Frontend & Browser Boundary Decision:**
Go generates the HTML and SVG from placement geometry and frozen construction traces. Use a small, dependency-free vanilla JavaScript layer for control-driven updates, trace selection/highlighting, and diagram navigation (pan/zoom). Embed the web assets with Go; no frontend framework, npm dependencies, or frontend build step is required.

**Milestone 1 Scope (One-Bike Visual Workbench) & Acceptance Criteria:**
1. **Full single-bike traceable assembly (100% solver-free, no hardcoded pre-leveled frame endpoints):**
   - Assemble one known bike fixture (**Otso Fenrir Stainless**) starting from published chart dimensions (`Stack`, `Reach`, `HTA`, `STA`, `HTL`, `BB Drop`, `Chainstay`) and a declared fork (`StaticA2C`, `Travel`, `SagPct`, `Rake`, `LowerCup`) using traceable `Pose` steps:
     1. Build the chassis and fork at `0°` pitch using the single forward `Pose` recipe (`placeChassisAtPitch`), measure the rear-to-front-axle angle (`PitchDeg = -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)`), and rebuild the leveled chassis with the exact same `Pose` recipe rooted at `bb.Turn("frame pitch to level axles", PitchDeg)` (see **Frame Pitch & Leveling as a Rooted `Pose` Tree** below).
     2. Mount the rear cockpit (`PlaceSaddle`: direct forward `BBToClampAlongSeatAxisMm`, `PostSetbackMm`, `RailOffsetMm`, `RailPitchDeg` → `SeatpostClamp`, `RailCenter`, `BRP`, `SitBones`) and front cockpit (`PlaceStem` + `PlaceHandlebar`: upper cover, spacers, stem length/angle, bar sweep/rise/roll → `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HandGrip`).
   - Milestone 1 is **strictly solver-free**: the seatpost height control edits the forward physical input `BBToClampAlongSeatAxisMm` directly, while radial saddle height (`|BRP - BB|`) and estimated seatpost extension (`BBToClampAlongSeatAxisMm - SeatTubeLengthMm`) are displayed as measured dimension readouts.
2. **Side-view visual inspector:**
   - Render physical outlines in local `Pose` frames, named landmarks, local coordinate axes (`+X`, `+Y`), key dimensions, declared wheel sizes/orientations, editable component controls (fork choice/length/sag, lower cup, spacers, stem length/angle, handlebar roll, seatpost clamp distance/setback, saddle rail offset/pitch), and a selectable construction trace table where clicking/hovering/focusing a step highlights its movement vector and frame on the SVG.
   - Multi-bike ghost overlays, fit solvers, chart-hypothesis solvers, and catalog file persistence are explicitly outside Milestone 1.
3. **Verification:**
   - Matt can read each component/chassis recipe, predict the effect of a setting change, and inspect the result visually and in the trace.
   - Automated Go tests verify identical placements with tracing on/off, independent geometric fixtures (lengths, angles, leveling invariants, contact positions), and HTML/SVG step-ID correspondence, paired with a browser interaction check for control updates, highlighting, history, and pan/zoom.

**Confirmed 4-Milestone Sequence:**
1. **Milestone 1 — One-bike visual workbench `(a, b)`:** Single-bike traceable assembly (chassis/fork leveling + rear/front cockpits, solver-free) and interactive Go-rendered HTML/SVG workbench.
2. **Milestone 2 — One-bike fit search `(d)`:** Add explicit rider fit targets, hardware limits, and preference costs; search component permutations, verify returned builds through forward assembly, and open candidate builds directly in the workbench. Verify against a small exhaustive fixture and measure representative search responsiveness.
3. **Milestone 3 — Chart reconstruction, catalog persistence & agent access `(c)`:** Preserve original chart observations and rounding budgets; evaluate candidate fork/sag/cup hypotheses, show residuals and ambiguous explanations, select structured catalog file persistence (`Q4`), and expose structured solver operations (MCP/CLI) for AI agents.
4. **Milestone 4 — Frame comparison / chooser `(e)`:** Reuse fit search across a catalog of frames and sizes, return best builds and their explanations, support visual comparison in the workbench, and report search coverage and sensitivity to assumptions.

### Side-by-side `internal/reboot/` isolation, `*_parity_test.go` import rule & Otso Fenrir fixture (Turn 6, resolves Q5)

**Decision:** Adopt the side-by-side implementation arrangement, five vertical Milestone 1 slices, and transition plan in [IMPLEMENTATION.md](IMPLEMENTATION.md):
1. **Package & Binary Isolation:** Leave existing `cmd/bikegeo` and `internal/*` packages in place. Build the reboot under `cmd/bikegeo-reboot` (temporary binary `bin/bikegeo-reboot`, default port `8081`) and `internal/reboot/{pose,assembly,web}`.
2. **Strict Self-Containment + `*_parity_test.go` Exception:**
   - Production `.go` files and ordinary `*_test.go` files under `cmd/bikegeo-reboot` and `internal/reboot/...` must be 100% self-contained and may **never** import any legacy `bikegeo/...` package outside `cmd/bikegeo-reboot` and `internal/reboot/...`.
   - **Only files named `*_parity_test.go`** under `internal/reboot/...` may import legacy `bikegeo/internal/...` packages for transitional side-by-side comparisons, so retiring the old codebase later only requires deleting `*_parity_test.go` alongside the old packages.
   - Legacy packages may never import `bikegeo/internal/reboot/...` or `bikegeo/cmd/bikegeo-reboot/...`.
   - Enforce these boundaries in `make check` / `make pre-commit` using shell + regex source checks and `go list -deps` (see [IMPLEMENTATION.md §4](IMPLEMENTATION.md)).
3. **Initial Milestone 1 Bike Fixture:** Use the **Otso Fenrir Stainless** (`otso-fenrir-stainless`, default size `L`) as the Milestone 1 bike fixture (see [IMPLEMENTATION.md §2](IMPLEMENTATION.md) for the exact default dimensions).

### Frame Pitch & Leveling as a Rooted `Pose` Tree, Plus Coordinate Conventions (Turn 7)

**1. Frame Pitch & Leveling Model (Two-Pass `Pose` Recipe, Zero Closed-Form Trig):**
In 2D rigid-body geometry, pitching a rigid frame around `RearAxle` by `PitchDeg` and translating it so `BB` returns to `(0, 0)` is algebraically identical to rotating the frame around `BB (0, 0)` by `PitchDeg`:
`P_leveled = (RA + Rotate(PitchDeg, P - RA)) - (RA - Rotate(PitchDeg, RA)) = Rotate(PitchDeg, P)`

Rather than computing `PitchDeg` with separate closed-form trigonometry (`L_chart`, `ΔL`, `ΔRake`) outside the `Pose` recipe, both the pitch measurement and the leveled frame construction use **one single `placeChassisAtPitch(frameRoot Pose, chart FrameChart, fork ForkBuild)` recipe**:
1. **The Single Chassis & Fork `Pose` Recipe (`placeChassisAtPitch`):**
   Given a `frameRoot` pose at `BB (0, 0)` (at heading `0°` when unpitched, or heading `PitchDeg` when leveled):
   - `rearAxle := frameRoot.Translate("chart rear axle", -sqrt(Chainstay² - BbDrop²), chart.BbDropMm)`
   - `headTubeTop := frameRoot.Translate("chart head tube top (reach, stack)", chart.ReachMm, chart.StackMm)`
   - `steererUp := headTubeTop.Turn("steerer axis (180° - HTA)", 180 - chart.HtaDeg)` (points up the steerer for `PlaceStem`).
   - Walk down the steerer (`steererDown := steererUp.Turn("down steerer", 180)`) using actual physical component dimensions:
     - `headTubeBottom := steererDown.Advance("head tube length", chart.HtlMm)`
     - `lowerCupBottom := headTubeBottom.Advance("lower headset cup", fork.LowerCupMm)`
     - `forkSteererBase := lowerCupBottom.Advance("fork effective A2C", fork.EffectiveA2cMm)`
     - `frontAxle := forkSteererBase.Turn("fork rake direction", +90).Advance("fork rake", fork.RakeMm)` (counter-clockwise `+90°` from the downward steerer `-HTA`, pointing forward and up at `90° - HTA`).
   - `seatAxis := frameRoot.Turn("seat tube axis (180° - STA)", 180 - chart.StaDeg)` (points up the seat axis for `PlaceSaddle`).
2. **Two-Pass Leveling Using That Same Recipe:**
   - **Pass 1 (Measure `PitchDeg` at `0°` pitch):** Evaluate `unpitched := placeChassisAtPitch(OriginPose(nil), chart, installedFork)` at `0°` pitch and measure the rear-to-front-axle angle (`PitchDeg = -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)`). *(Running this same recipe at `0°` pitch with the chart's `cadFork` also exposes the raw unpitched chart-closure residuals `FrontAxle.Y - RearAxle.Y` and `FrontAxle.X - RearAxle.X - Wheelbase` without separate trig formulas.)*
   - **Pass 2 (Build the Leveled Chassis at `PitchDeg`):** Evaluate `leveled := placeChassisAtPitch(OriginPose(&trace).Turn("frame pitch to level axles", PitchDeg), chart, installedFork)`. Because Pass 2 rotates `frameRoot` by `PitchDeg = -axleAngle`, `leveled.FrontAxle` and `leveled.RearAxle` automatically sit at equal height (`FrontAxle.Y == RearAxle.Y`), and every step in the trace uses the actual physical dimensions (`HTL`, `LowerCup`, `EffectiveA2C`, `Rake`) in the exact leveled coordinates rendered on the SVG.

**2. Self-Contained Coordinate & Sign Conventions:**

| Area | Convention |
| --- | --- |
| **World coordinates** | Planar sagittal 2D, `BB` fixed at `(0, 0)`, `+X` forward, `+Y` up; positive `Turn` angles are counter-clockwise (`CCW`). |
| **Units** | Lengths are `float64` millimeters (`mm`); angles use the reboot's named `pose.Deg` type (`0°` along `+X`, `90°` along `+Y`). |
| **Wheel & ground line** | With `BB` at `(0, 0)` and equal wheel radii `R_wheel`, leveled front and rear axle centers sit at `Y = +BbDrop_leveled`, and the ground line sits at `Y = +BbDrop_leveled - R_wheel`. |
| **Steerer & stem (`PlaceStem`)** | Starting at `SteererUp` (`180° - HTA_leveled`, pointing up and rearward along the steerer): advance by `UpperCoverMm`, `SpacersMm`, and `ClampHeightMm / 2` to `SteererClamp`; turn `-90° + StemAngleDeg` (clockwise from steerer by `90°`, plus signed stem angle) and advance by `StemLengthMm` to `BarClamp`. |
| **Handlebar (`PlaceHandlebar`)** | Takes `BarClamp` pose and applies the 3D piecewise handlebar offset (lateral grip station `Width/2 - GripInset`, knuckle vs. bar-end rise mode, backsweep, upsweep) rotated by `BarRollDeg` **relative to the leveled world horizontal** (positive `BarRollDeg` rotates clockwise in the `+X/+Y` plane) to place `HandGrip`. |
| **Seatpost & saddle (`PlaceSaddle`)** | Starting at `SeatAxis` (`180° - STA_leveled`, pointing up and rearward along the effective seat axis; local `+Y` points `90°` CCW, i.e., perpendicular rearward/downward): advance by `BBToClampAlongSeatAxisMm` and translate along local `+Y` by `PostSetbackMm` to `SeatpostClamp`. From `SeatpostClamp`, orient a world-horizontal rail frame (`0°`) turned by `RailPitchDeg` (positive raises the saddle nose CCW), advance by `-RailOffsetMm` (so **positive `RailOffsetMm` moves the saddle body rearward** relative to the clamp) to `RailCenter`, then locate `BRP` at `(+RailCenterToBrpMm, 0)` and `SitBones` at `(RailCenterToSitBonesMm, +SitBonesElevationMm)` in the rail frame. |
| **Chart immutability** | Published manufacturer chart values are sacred ground truth and are never nudged to hide closure residuals. |

---

## 2. Converged Proposals (Supporting Notes for Later Milestones)

- **Browser implementation hygiene:** Progressive enhancement over standard Go-rendered forms/links; replace diagram and trace together while ignoring superseded in-flight requests (`AbortController`); preserve input focus and URL history; support keyboard focus as well as hover/click for trace step highlighting.
- **Forward evaluation supports search (Milestones 2–4):** Enumerate component choices and evaluate candidate assemblies for chart reconstruction and rider fit, using forward assembly as the shared evaluator and final check.
- **Catalogs loaded as data (Milestone 3):** Use structured data definitions that can be ingested without recompilation (`Q4`).

---

## 3. Active Design Options (Deferred to Future Milestones)

### Solver policy details (Revisit before Milestone 2 & Milestone 3)
- **Gemini:** Prefer forward assembly with generic bisection/grid sweeps wherever practical to minimize bespoke inverse trigonometry.
- **ChatGPT:** Use forward assembly as the final check; choose bounded numerical search, enumeration, or a small direct geometric solve per problem. Keep observations separate from inferred models, permit multiple chart explanations, and assess the combined fit objective before independently selecting front/rear builds. Preserve explicit preference costs and distinguish infeasible hardware from imperfect fit.
- **Trigger:** Review and settle concrete solver algorithms when starting Milestone 2 (fit search) and Milestone 3 (chart reconstruction).

---

## 4. Decision Queue (For the Human Operator)

*(No active decisions blocking Milestone 1 — `Q1`, `Q2`, `Q3`, and `Q5` are resolved.)*

### Deferred

- **Q4 (Data & Catalog Storage):**
  - *Question:* Which structured file format and persistence mechanism (e.g., TOML/YAML/JSON) should store frames and component catalogs so the AI MCP solver can ingest, test, and persist new frames without recompiling?
  - *Recommendation (Both agents):* Agree on versioned structured data documents with sources and assumptions; Milestone 1 uses the explicit **Otso Fenrir Stainless** Go fixture.
  - *Revisit when:* Designing the first import/save operation in Milestone 3.
