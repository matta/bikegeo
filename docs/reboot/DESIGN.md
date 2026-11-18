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
1. **Full single-bike traceable assembly (no hardcoded pre-leveled frame endpoints):**
   - Assemble one known bike fixture starting from published chart dimensions (`Stack`, `Reach`, `HTA`, `STA`, `HTL`, `BB Drop`, `Chainstay`) and a declared fork (`StaticA2C`, `Travel`, `SagPct`, `Rake`, `LowerCup`) using traceable `Pose` steps:
     1. Place unpitched chart landmarks (`BB`, `RearAxle`, `HeadTubeTop`, seat axis) and walk down the steerer axis through `HeadTubeBottom`, `LowerCupBottom`, and fork `EffectiveA2C`, turning perpendicular by `ForkRake` to locate `FrontAxle` (preserving the chart-baseline vs. declared-fork distinction).
     2. Level the frame around `RearAxle` when fork length, lower cup, sag, or rake changes, returning `BB` to `(0, 0)`.
     3. Mount the rear cockpit (`PlaceSaddle`: seatpost extension, setback, rail offset, rail pitch → `SeatpostClamp`, `RailCenter`, `BRP`, `SitBones`) and front cockpit (`PlaceStem` + `PlaceHandlebar`: upper cover, spacers, stem length/angle, bar sweep/rise/roll → `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HandGrip`).
2. **Side-view visual inspector:**
   - Render physical outlines in local `Pose` frames, named landmarks, local coordinate axes (`+X`, `+Y`), key dimensions, declared wheel sizes/orientations, editable component controls (fork length/sag, spacers, stem length/angle, handlebar, seatpost/saddle settings), and a selectable construction trace table where clicking/hovering/focusing a step highlights its movement vector and frame on the SVG.
   - Multi-bike ghost overlays, fit solvers, chart-hypothesis solvers, and catalog file persistence are explicitly outside Milestone 1.
3. **Verification:**
   - Matt can read each component/chassis recipe, predict the effect of a setting change, and inspect the result visually and in the trace.
   - Automated Go tests verify identical placements with tracing on/off, independent geometric fixtures (lengths, angles, leveling invariants, contact positions), and HTML/SVG step-ID correspondence, paired with a browser interaction check for control updates, highlighting, history, and pan/zoom.

**Confirmed 4-Milestone Sequence:**
1. **Milestone 1 — One-bike visual workbench `(a, b)`:** Single-bike traceable assembly (chassis/fork leveling + rear/front cockpits) and interactive Go-rendered HTML/SVG workbench.
2. **Milestone 2 — One-bike fit search `(d)`:** Add explicit rider fit targets, hardware limits, and preference costs; search component permutations, verify returned builds through forward assembly, and open candidate builds directly in the workbench. Verify against a small exhaustive fixture and measure representative search responsiveness.
3. **Milestone 3 — Chart reconstruction, catalog persistence & agent access `(c)`:** Preserve original chart observations and rounding budgets; evaluate candidate fork/sag/cup hypotheses, show residuals and ambiguous explanations, select structured catalog file persistence (`Q4`), and expose structured solver operations (MCP/CLI) for AI agents.
4. **Milestone 4 — Frame comparison / chooser `(e)`:** Reuse fit search across a catalog of frames and sizes, return best builds and their explanations, support visual comparison in the workbench, and report search coverage and sensitivity to assumptions.

---

## 2. Converged Proposals (Supporting Notes for Later Milestones)

- **Spatial scope:** Sagittal 2D pose core (`+X` forward, `+Y` up, CCW positive rotation) for frame and cockpit assembly, while retaining lateral 3D handlebar sweep/upsweep/grip-inset and world-relative bar roll when computing `HandGrip`.
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

*(No active decisions blocking Milestone 1 — `Q1`, `Q2`, and `Q3` are resolved.)*

### Deferred

- **Q4 (Data & Catalog Storage):**
  - *Question:* Which structured file format and persistence mechanism (e.g., TOML/YAML/JSON) should store frames and component catalogs so the AI MCP solver can ingest, test, and persist new frames without recompiling?
  - *Recommendation (Both agents):* Agree on versioned structured data documents with sources and assumptions; Milestone 1 uses an explicit Go fixture.
  - *Revisit when:* Designing the first import/save operation in Milestone 3.
