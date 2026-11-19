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

**Decision:** Express physical component assembly using immutable, value-style `Pose` operations inside named component functions (`PlaceStem`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceHandlebar`, etc.), returning `Pose`-valued landmarks alongside component setup dimensions, with an optional embedded `*Trace` sink on `Pose` to record construction steps for inspection.

**Key Design Conditions:**
1. **Value-style `Pose` movements:** Movement methods (`Advance`, `Translate`, `Turn`, and `Face`) return new `Pose` values so branching from a common attachment (such as a saddle's `RailCenter` or a frame's `HeadTubeTop`) is natural and cannot mutate prior poses. `Face(label, worldHeadingDeg)` keeps the current pose's position (`Origin`) and sets its heading directly to an absolute world angle while recording a traced step from the parent pose—used whenever a rider-adjusted part is leveled relative to the world rather than its mounting tube (such as saddle `RailPitchDeg` at `SeatpostClamp` and handlebar `BarRollDeg` at `BarClamp`).
2. **Pose-valued landmarks with explicit coordinate-frame meanings:** Placement structs retain `Pose`s for attachments and physical landmarks rather than bare points, preventing callers and renderers from reconstructing orientations via scattered trigonometry. Each landmark's local coordinate axes must have a documented physical meaning (e.g., `BRP` and `SitBones` carry the saddle-rail construction frame, not a curved saddle-surface normal). Raw points remain internal to low-level geometry helpers and final scalar readouts (`X()`, `Y()`, `DistanceTo`).
3. **Optional `*Trace` sink with explicit step parentage:** A `Pose` carries an optional `*Trace` pointer and `stepID int`. When tracing is `nil` (such as during solver searches), movement methods skip record collection and string formatting; when non-nil (such as when inspecting a build in the workbench), they record sink-free geometry snapshots (`From`, `To`), operation labels, inputs, and explicit `ParentID -> ID` provenance using the exact same calculation path.
4. **Freeze/detach trace before rendering:** Each traced evaluation uses its own `Trace` instance and freezes or detaches recording when assembly completes, ensuring that downstream rendering helpers (e.g., offsetting from a landmark `Pose` to draw a tick mark or dimension callout) never append to the assembly history.
5. **Separate placement from outline projection:** Moving along a construction path (such as advancing half a stem clamp height to reach its center) is distinct from a part's full physical outline. Component functions return placement geometry and setup dimensions; the renderer projects physical shapes in those local `Pose` frames without recalculating angles.
6. **Geometry & trigonometry live in `pose`:** Non-test Go code under `internal/reboot/...` (outside `pose`) delegates to semantic abstractions in `internal/reboot/pose` for all trigonometry, radian conversions, and geometric math, while `pose` keeps raw trig and radian conversions package-private. Plain numeric/validation checks and independent `math` usage in `*_test.go` remain allowed; codified in `AGENTS.md` and checked in code review.
7. **Verification:** Identical inputs must produce identical placed geometry whether tracing is enabled or disabled.

### Go-rendered HTML/SVG workbench & 4-Milestone Sequence (Turn 5, resolves Q3)

**Frontend & Browser Boundary Decision:**
Go generates the HTML and SVG from placement geometry and frozen construction traces. Use a small, dependency-free vanilla JavaScript layer for control-driven updates, trace selection/highlighting, and diagram navigation. For Milestone 1, diagram navigation is three fixed server-rendered views (whole bike, seat, stem) rather than free pan/zoom (Discussion, 2026-11-19). Embed the web assets with Go; no frontend framework, npm dependencies, or frontend build step is required.

**Milestone 1 Scope (One-Bike Visual Workbench) & Acceptance Criteria:**
1. **Full single-bike traceable assembly (100% solver-free, no hardcoded pre-leveled frame endpoints):**
   - Assemble one known bike fixture (**Otso Fenrir Stainless**) starting from bare published frame dimensions (`Stack`, `Reach`, `HTA`, `STA`, `HTL`, `BB Drop`, `Chainstay`), a typed lower headset cup (`LowerCupSetup{Standard, StackMm}`), and a declared fork (`StaticA2C`, `Travel`, `SagPct`, `Rake`) using traceable `Pose` steps:
     1. Build the frameset (frame, fork, and lower headset cup) at `0°` pitch using the single forward `Pose` recipe (`placeFramesetAtPitch`), measure the rear-to-front-axle angle (`PitchDeg = -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)`), and rebuild the leveled frameset with the exact same `Pose` recipe rooted at `bb.Turn("frame pitch to level axles", PitchDeg)` (see **Frame Pitch & Leveling as a Rooted `Pose` Tree** below).
     2. Mount the rear cockpit (`PlaceSeatpost`: `ExtensionMm` above `SeatTubeTop`, `PostSetbackMm` → `SeatpostClamp`; `PlaceSaddle`: `RailOffsetMm`, `RailPitchDeg` → `RailCenter`, `BRP`, `SitBones`) and front cockpit (`PlaceStem` + `PlaceHandlebar`: upper cover, spacers, stem length/angle, bar sweep/rise/roll → `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HandGrip`).
   - Milestone 1 is **strictly solver-free**: the seatpost control edits the forward physical input `ExtensionMm` (exposed seatpost extension from `SeatTubeTop` along the seat tube axis, subject to seatpost/frame extension and insertion limits) directly, while total axial distance from `BB` (`SeatTubeLengthMm + ExtensionMm`) and radial saddle height (`|BRP - BB|`) are displayed as measured dimension readouts.
2. **Side-view visual inspector:**
   - Render physical outlines in local `Pose` frames, named landmarks, local coordinate axes (`+X`, `+Y`), key dimensions, declared wheel sizes/orientations, editable component controls (fork choice/length/sag, lower cup, spacers, stem length/angle, handlebar roll, seatpost extension/setback, saddle rail offset/pitch), and a selectable construction trace table where clicking/hovering/focusing a step highlights its movement vector and frame on the SVG.
   - Multi-bike ghost overlays, fit solvers, chart-hypothesis solvers, and catalog file persistence are explicitly outside Milestone 1.
3. **Verification:**
   - Matt can read each component and frameset recipe, predict the effect of a setting change, and inspect the result visually and in the trace.
   - Automated Go tests verify identical placements with tracing on/off, independent geometric fixtures (lengths, angles, leveling invariants, contact positions), and HTML/SVG step-ID correspondence, paired with a browser interaction check for control updates, highlighting, history, and fixed diagram views.

**Confirmed 4-Milestone Sequence:**
1. **Milestone 1 — One-bike visual workbench `(a, b)`:** Single-bike traceable assembly (frameset leveling + rear/front cockpits, solver-free) and interactive Go-rendered HTML/SVG workbench.
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

Rather than computing `PitchDeg` with separate closed-form trigonometry (`L_chart`, `ΔL`, `ΔRake`) outside the `Pose` recipe, both the pitch measurement and the leveled frameset construction use **one single `placeFramesetAtPitch(frameRoot Pose, frame FrameSetup, lowerCupMm float64, fork ForkGeometry)` recipe** (called by `PlaceFrameset(frame, lowerCup, fork, wheels, trace)`, where `ForkSetup{Spec, SagPct}` resolves `EffectiveA2cMm()` into `ForkGeometry`, `LowerCupSetup{Standard, StackMm}` validates per-standard cup invariants, and `FrameChart{Frame, CadLowerCup, CadFork, PublishedWheelbaseMm}` keeps the bare frame separate from the CAD chart cup and fixed fork geometry):
1. **The Single Frameset `Pose` Recipe (`placeFramesetAtPitch`):**
   Given a `frameRoot` pose at `BB (0, 0)` (at heading `0°` when unpitched, or heading `PitchDeg` when leveled):
   - `rearAxle := frameRoot.Translate("rear axle", -pose.RightTriangleLeg(frame.ChainstayMm, frame.BbDropMm), frame.BbDropMm)`
   - `headTubeTop := frameRoot.Translate("head tube top (reach, stack)", frame.ReachMm, frame.StackMm)`
   - `steererUp := headTubeTop.Turn("steerer axis (180° - HTA)", 180 - frame.HeadTubeAngleDeg)` (points up the steerer for `PlaceStem`).
   - Walk down the steerer (`steererDown := steererUp.Turn("down steerer", 180)`) using actual physical component dimensions:
     - `headTubeBottom := steererDown.Advance("head tube length", frame.HeadTubeLengthMm)`
     - `lowerCupBottom := headTubeBottom.Advance("lower headset cup", lowerCupMm)`
     - `forkSteererBase := lowerCupBottom.Advance("fork A2C", fork.A2cMm)`
     - `frontAxle := forkSteererBase.Turn("fork rake direction", +90).Advance("fork rake", fork.RakeMm)` (counter-clockwise `+90°` from the downward steerer `-HTA`, pointing forward and up at `90° - HTA`).
   - `seatAxis := frameRoot.Turn("seat tube axis (180° - STA)", 180 - frame.SeatTubeAngleDeg)` (points up the seat axis, exposed at `BB` for full-axis reference and BB-to-clamp measurements).
   - `seatTubeTop := seatAxis.Advance("seat tube length", frame.SeatTubeLengthMm)` (points up the seat tube axis for `PlaceSeatpost`).
2. **Two-Pass Leveling Using That Same Recipe:**
   - **Pass 1 (Measure `PitchDeg` at `0°` pitch):** Evaluate `unpitched := placeFramesetAtPitch(OriginPose(nil), frame, lowerCup.StackMm, fork.Geometry())` at `0°` pitch and measure the rear-to-front-axle angle (`PitchDeg = -unpitched.RearAxle.AngleTo(unpitched.FrontAxle)`). *(Running this same recipe at `0°` pitch with `chart.Frame`, `chart.CadLowerCup.StackMm`, and fixed `chart.CadFork` also exposes the raw unpitched chart-closure residuals `FrontAxle.Y - RearAxle.Y` and `FrontAxle.X - RearAxle.X - Wheelbase` without separate trig formulas.)*
   - **Pass 2 (Build the Leveled Frameset at `PitchDeg`):** Evaluate `leveled := placeFramesetAtPitch(OriginPose(&trace).Turn("frame pitch to level axles", PitchDeg), frame, lowerCup.StackMm, fork.Geometry())`. Because Pass 2 rotates `frameRoot` by `PitchDeg = -axleAngle`, `leveled.FrontAxle` and `leveled.RearAxle` automatically sit at equal height (`FrontAxle.Y == RearAxle.Y`), and every step in the trace uses the actual physical dimensions (`HTL`, `LowerCupSetup.StackMm`, `EffectiveA2cMm()`, `RakeMm`) in the exact leveled coordinates rendered on the SVG.

**2. Self-Contained Coordinate & Sign Conventions:**

| Area | Convention |
| --- | --- |
| **World coordinates** | Planar sagittal 2D, `BB` fixed at `(0, 0)`, `+X` forward, `+Y` up; positive `Turn` angles are counter-clockwise (`CCW`). |
| **Units** | Lengths are `float64` millimeters (`mm`); angles use the reboot's named `pose.Deg` type, which is plane-agnostic with no implied coordinate system. The sagittal world heading convention (`0°` along `+X`, `90°` along `+Y`, positive CCW) belongs to `Pose` (`HeadingDeg`, `Turn`, `Face`), while component fields declare their own plane and sign conventions. |
| **Wheel & ground line** | With `BB` at `(0, 0)` and equal wheel radii `R_wheel`, leveled front and rear axle centers sit at `Y = +BbDrop_leveled`, and the ground line sits at `Y = +BbDrop_leveled - R_wheel`. |
| **Steerer & stem (`PlaceStem`)** | Starting at `SteererUp` (`180° - HTA_leveled`, pointing up and rearward along the steerer): advance by `UpperCoverMm` (currently on `StemSetup` with a `TODO` to move into a headset setup alongside `LowerCupSetup`), `SpacersMm`, and `ClampHeightMm / 2` to `SteererClamp`; turn `-90° + StemAngleDeg` (clockwise from steerer by `90°`, plus signed stem angle) and advance by `StemLengthMm` to `BarClamp`. |
| **Handlebar (`PlaceHandlebar`)** | Takes `BarClamp` pose, re-orients to world horizontal plus roll via `barClamp.Face("bar roll vs. world horizontal", -BarRollDeg)` (where positive `BarRollDeg` rotates clockwise in the leveled `+X/+Y` world plane), and translates by the 3D piecewise handlebar offset (lateral grip station `Width/2 - GripInset`, knuckle vs. bar-end rise mode, top-view `BacksweepDeg` positive rearward and front-view `UpsweepDeg` positive upward projected via `pose.DeflectionOverSpan`) to place `HandGrip`. |
| **Seatpost (`PlaceSeatpost`)** | Starting at `SeatTubeTop` (`180° - STA_leveled`, pointing up and rearward along the seat tube axis; local `+Y` points `90°` CCW, i.e., perpendicular rearward/downward): advance by `ExtensionMm` (exposed seatpost length above the seat tube collar, within seatpost/frame extension and insertion limits) and translate along local `+Y` by `PostSetbackMm` to `SeatpostClamp`. |
| **Saddle (`PlaceSaddle`)** | Starting at `SeatpostClamp`, set the world-relative rail orientation via `seatpostClamp.Face("saddle rail pitch vs. world horizontal", RailPitchDeg)` (positive raises the saddle nose CCW from world horizontal), advance by `-RailOffsetMm` (so **positive `RailOffsetMm` moves the saddle body rearward** relative to the clamp) to `RailCenter`, then locate `BRP` at `(+RailCenterToBrpMm, 0)` and `SitBones` at `(RailCenterToSitBonesMm, +SitBonesElevationMm)` in the rail frame. |
| **Cockpit readouts** | `BrpToGripDistMm` measures `|HandGrip - BRP|`, whereas `SitBonesToGripRiseMm` measures `HandGrip.Y - SitBones.Y` (from `SitBones` rather than `BRP`) so rider-fit cockpit elevation carries consistently across saddles through the sit-bone contact point. |
| **Chart immutability** | Published manufacturer chart values are sacred ground truth and are never nudged to hide closure residuals. |
| **CAD chart reference vs. installed forks** | `FrameChart.CadFork` (`ForkGeometry{A2cMm, RakeMm}`) and `FrameChart.CadLowerCup` (`LowerCupSetup`) capture the manufacturer's fixed CAD chart geometry, keeping bare `FrameSetup` free of any CAD cup omission (such as Otso Fenrir's `LowerCupBareHeadTube` `0 mm`). `LowerCupSetup.Validate()` enforces per-standard invariants (`EC34`/`EC44` require `StackMm > 0`, `BareHeadTube` requires `StackMm == 0`, `ZS44`/`ZS56`/`IS42`/`IS52` require `StackMm >= 0`, empty/unknown standard rejected). The workbench keeps the **CAD Baseline Fork** (`fork=cad`) distinct from real installable forks paired with their physical `LowerCupSetup` (`EC44`), and locks it to `FrameChart` values (see **Workbench CAD Baseline Fork** below). |
| **Terminology (banned term: `chassis`)** | Use **Frame** for bare frame geometry (`FrameSetup`), **Frameset** for leveled frame + fork + lower headset cup + wheel-ground contacts (`FramesetPlacement`, `PlaceFrameset`), and **Bike** for the complete assembly including cockpits (`BikePlacement`, `PlaceBike`, `/bike`). The term `chassis` is banned in reboot code, templates, tests, and living reboot docs (enforced in code by `make boundary`; legacy packages and historical `DIALOGUE.md` remain unchanged). |

### Workbench CAD Baseline Fork: locked chart values and one name (Discussion, 2026-11-19)

Matt adopted this as the plan of record (see the 2026-11-19 Claude discussion entries and ChatGPT Turn 8 in [DIALOGUE.md](DIALOGUE.md)). No new persisted mode or domain type is introduced, and the grouped fork selector and URL representation stay as they are.

1. **Locked to `FrameChart`, enforced in Go:** When `fork=cad`, the `/bike` handler resolves fork geometry from `FrameChart.CadFork`, the lower cup from `FrameChart.CadLowerCup`, and sag `0%`, and **ignores** submitted `fork_a2c`, `cup`, and `sag` (no error). It renders the resolved values. Resolve all three in one place (one `fork=cad` branch or one fixture method), not scattered `fork == cad` checks. Physical fork selections keep their existing input validation (for example, rejecting a `0 mm` EC44 cup).
   - *Basis:* `placeFramesetAtPitch` advances the lower cup and fork A2C consecutively along the steering axis, so only their sum moves the front axle. On Otso Fenrir, `lithic_carbon_mountain` with `12 mm` EC44 + `483 mm` A2C reproduces the CAD `0 mm` + `495 mm` front end, so locking loses no A2C exploration. The "Modified chart setup" indicator is therefore dropped.
   - *Layering:* The lock is a workbench selection rule. `assembly` keeps accepting explicit frame, cup, and fork inputs, including `BareHeadTube` at zero stack.
2. **Read-only fields without JavaScript:** Under `fork=cad`, the A2C, cup, and sag inputs render with `readonly` (not `disabled`, so values are still submitted and the URL round-trip is unchanged). Sag also renders read-only for any rigid fork (`TravelMm == 0`), including the default Lithic fork. This is safe without JavaScript because the existing fork-switch logic resets cup (on a lower-cup standard change), A2C, and sag when the selected fork changes.
3. **One name, "CAD Baseline Fork":** Every visible reference uses "CAD Baseline Fork": the selector option, its `<optgroup>`, the preset link, the CAD summary line, and the explanatory note. This replaces "CAD Chart Reference … chart only", "Chart Inspection", and the proposed "Fork / chart reference" / "Inspect published chart setup" labels.
4. **Explanatory note from fixture data:** When `FrameChart.CadLowerCup.Standard` is `BareHeadTube`, the CAD Baseline Fork view explains that the chart omits the lower cup and that the physical pairing defaults to the default fork's `DefaultLowerCup` (standard and stack). The note is built from fixture values, not hardcoded Otso Fenrir copy.
5. **Cup input labelled with its standard and rule:** Label the cup input with the selected pairing's `LowerCupStandard` and its stack rule (for example, "EC44 lower cup (mm, > 0)", or "No lower cup (bare head tube)" for `BareHeadTube`) via a small display-name mapping in `web`. Keep `min="0"`: HTML `min` is inclusive and cannot express EC34/EC44's strict `StackMm > 0`, and a fake minimum such as `min="0.5"` would wrongly reject valid stacks below it. The label and `LowerCupSetup.Validate()` carry the strict rule.
6. **Verification:** Move `/bike?fork=cad&cup=12` out of the invalid-input table and invert it (200, cup `0` read-only, no error), with matching `fork_a2c` and `sag` override cases. Add tests for switching both ways between `fork=cad` and physical forks, `readonly` rendering of the three fields under `fork=cad` and of sag on rigid forks, editable cockpit fields under `fork=cad`, the single "CAD Baseline Fork" name, the cup standard label, and the data-driven note. Keep the zero-stack EC44 rejection test.

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
