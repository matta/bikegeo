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

**Scope:** This settles the core language. The subsequent browser/frontend choice is recorded below under Q3. The core choice does not approve a specific milestone, require a terminal-first interface, or authorize implementation. Existing code is useful reference material, not a requirement to preserve the current architecture.

### Pose-based component assembly with optional embedded tracing (Turn 4, resolves Q2)

**Decision:** Express physical component assembly using immutable, value-style `Pose` operations inside named component functions (`PlaceStem`, `PlaceSaddle`, `PlaceHandlebar`, etc.), returning `Pose`-valued landmarks alongside component setup dimensions, with an optional embedded `*Trace` sink on `Pose` to record construction steps for inspection.

**Key Design Conditions:**
1. **Value-style `Pose` movements:** Movement methods (`Advance`, `Translate`, `Turn`) return new `Pose` values so branching from a common attachment (such as a saddle's `RailCenter` or a frame's `HeadTubeTop`) is natural and cannot mutate prior poses.
2. **Pose-valued landmarks with explicit coordinate-frame meanings:** Placement structs retain `Pose`s for attachments and physical landmarks rather than bare points, preventing callers and renderers from reconstructing orientations via scattered trigonometry. Each landmark's local coordinate axes must have a documented physical meaning (e.g., `BRP` and `SitBones` carry the saddle-rail construction frame, not a curved saddle-surface normal). Raw points remain internal to low-level geometry helpers and final scalar readouts (`X()`, `Y()`, `DistanceTo`).
3. **Optional `*Trace` sink with explicit step parentage:** A `Pose` carries an optional `*Trace` pointer and `stepID int`. When tracing is `nil` (such as during solver searches), movement methods skip record collection and string formatting; when non-nil (such as when inspecting a build in the workbench), they record sink-free geometry snapshots (`From`, `To`), operation labels, inputs, and explicit `ParentID -> ID` provenance using the exact same calculation path.
4. **Freeze/detach trace before rendering:** Each traced evaluation uses its own `Trace` instance and freezes or detaches recording when assembly completes, ensuring that downstream rendering helpers (e.g., offsetting from a landmark `Pose` to draw a tick mark or dimension callout) never append to the assembly history.
5. **Separate placement from outline projection:** Moving along a construction path (such as advancing half a stem clamp height to reach its center) is distinct from a part's full physical outline. Component functions return placement geometry and setup dimensions; the renderer projects physical shapes in those local `Pose` frames without recalculating angles.
6. **Verification:** Identical inputs must produce identical placed geometry whether tracing is enabled or disabled.

### Go-rendered HTML/SVG with plain JavaScript (Q3 frontend discussion, 2026-11-18)

**Decision:** Go generates the HTML and SVG from placement geometry and frozen construction traces. Use a small, dependency-free vanilla JavaScript layer for control-driven updates, trace selection/highlighting, and diagram navigation. Embed the web assets with Go; no frontend framework, npm dependencies, or frontend build step is required.

**Rationale:** Matt agreed with Gemini's recommendation for dropdowns, number fields, tabs, and an inspectable SVG diagram. This keeps physical drawing and geometry in the familiar Go codebase while allowing useful browser interactions. Shared HTML attributes can associate trace rows with diagram elements without mirroring every domain struct in TypeScript. The old zero-JavaScript restriction is not part of this reboot decision.

**Scope:** This resolves the frontend portion of Q3. It selects a control-driven inspector rather than a direct manipulation CAD interface. The exact first-milestone feature list remains in Q3. The suggestion of approximately 60 lines of JavaScript, fixed code percentages, and localhost response times are estimates, not acceptance criteria. Prefer understandable browser code over an arbitrary line limit.

---

## 2. Converged Proposals (Pending Human Sign-off)

Common ground from Turns 1–4 in [DIALOGUE.md](DIALOGUE.md), limited to the shared substance below:

- **Single-bike visual workbench for Milestone 1 (Endorsed by both agents in Turn 2 & Turn 4):** Start Milestone 1 with a single known bike in side view showing physical outlines, named landmarks, local coordinate axes, dimensions, editable component settings, and selectable construction trace steps. Defer multi-bike ghost overlays and chart-residual visualizers to later milestones.
- **Forward evaluation supports search:** Enumerate component choices and evaluate candidate assemblies for chart reconstruction and rider fit. The frame chooser reuses fit evaluation across frames and sizes. This agreement does not require eliminating all direct inverse solutions or assume the objectives separate.
- **Catalogs can be loaded as data:** Both agents favor structured definitions that can be ingested without recompilation (Gemini's Q4 recommendation; ChatGPT Turn 2). Format and persistence details can wait until ingestion is implemented.

---

## 3. Active Design Options (Under Discussion)

### Browser implementation notes (frontend choice confirmed; details proposed)

- Start from ordinary Go-rendered forms and links; enhance them with fetched HTML/SVG updates. Keep assembly and drawing in Go rather than duplicating physical calculations in browser code.
- Update the diagram and trace together; ignore superseded responses so rapid edits cannot display an older build. Preserve input focus, handle incomplete/invalid input, and keep refresh and Back/Forward consistent with the displayed controls and result.
- Associate trace rows and diagram elements through shared attributes. Support keyboard focus/selection as well as hover/click. Treat step IDs as belonging to the current build when replacing results; do not accidentally carry selection to an unrelated new step.
- Go tests verify geometry, HTTP behavior, and rendered HTML/SVG structure. They do not establish that client-side highlighting, fetch updates, history, or pan/zoom work in an actual browser. Include a small browser interaction check, which can initially be manual; no browser test framework is selected here.
- These are implementation notes, not reasons to reopen the chosen stack. Frameworks are not inherently restricted to large applications, and duplicated frontend types are not inevitable; this project's concrete needs justify the simpler choice.

### Spatial scope
- **Planar 2D pose core + 3D handlebar offset helper:** Both agents agree the frame/cockpit assembly and Milestone 1 view are sagittal 2D (`+X` forward, `+Y` up, CCW positive angle), while preserving the existing 3D handlebar sweep/upsweep/roll offset calculation (`HandlebarOffset3D`) so `HandGrip` landmarks reflect lateral grip position and world-relative bar roll.

### Solver policy (For Milestones 2–3)
- **Gemini:** Prefer forward assembly with generic bisection/grid sweeps wherever practical to minimize bespoke inverse trigonometry.
- **ChatGPT:** Use forward assembly as the final check; use bounded numerical search, enumeration, or a small direct geometric solve according to the problem. Keep observations separate from inferred models, permit multiple chart explanations, and assess the combined fit objective before independently selecting front/rear builds. Preserve explicit preference costs and distinguish infeasible hardware from imperfect fit.
- **Timing:** Solver details need review before Milestone 2, not before Milestone 1.

### Proposed milestone sequence (Pending human approval)
1. **One-bike visual workbench:** Assemble one known rigid-frame bike with configurable fork compression, spacers/stem, and saddle settings; show a side view with named landmarks, axes, dimensions, and selectable construction steps. Declare wheel sizes and orientation conventions. Matt can read a component recipe, predict an adjustment, and inspect the result; independent dimensions and known fixtures verify it. Full charts, catalog persistence, overlays, and solvers are outside this milestone.
2. **One-bike fit search:** Add explicit targets, limits, and preference costs; verify returned builds through assembly and open them directly in the workbench. Use a small exhaustive fixture to check candidate selection, then measure representative search time and responsiveness.
3. **Chart reconstruction and agent access:** Preserve original observations and rounding information; show candidate hypotheses, residuals, and unresolved quantities. Verify with a known reconstruction and an intentionally ambiguous case. Expose structured operations over the same application functions and choose catalog persistence here.
4. **Frame comparison:** Reuse fit search across a small catalog, return best builds and their explanations, and report search coverage and sensitivity to assumptions before expanding the catalog.

---

## 4. Decision Queue (For the Human Operator)

- **Q3 (Remaining Milestone 1 Visualization Scope):**
  - *Resolved frontend portion:* Go-rendered HTML/SVG with dependency-free vanilla JavaScript, as recorded above. No further language/framework choice is needed.
  - *Remaining question:* Approve the converged **one-bike workbench scope** for Milestone 1 (side view, editable settings, landmarks/axes/dimensions, selectable construction trace steps; deferring multi-bike overlays and chart diagnostics to later milestones)?
  - *Recommendation (Both agents):* The focused one-bike scope. Matt's approval of the pasted frontend proposal establishes the implementation approach and control-driven interaction model, not approval of the entire proposed milestone sequence.
  - *Blocks:* Finalizing Milestone 1's feature list and visual acceptance criterion.

### Deferred

- **Q4 (Data & Catalog Storage):**
  - *Question:* Should frames and component catalogs remain compiled-in structs/code, or move to structured data files (e.g., TOML/YAML/JSON) so an AI MCP solver can ingest, test, and persist new frames without recompiling?
  - *Recommendation (Both agents):* Agree on structured data ingestion without recompilation; defer the exact file format and persistence mechanism until Milestone 3 (chart reconstruction & agent access). Milestone 1 can use an explicit Go fixture.
  - *Revisit when:* Designing the first import/save operation in Milestone 3.
