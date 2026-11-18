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
- The desired richness of browser interaction is still open. We should not choose the core language on an unconfirmed assumption that the product needs continuous direct manipulation of geometry in the browser.
- A browser/core boundary is an accepted trade-off for keeping the model in a language Matt knows and retaining straightforward Go tooling and distribution. The earlier all-TypeScript proposal favored direct browser evaluation and one language across the application; those benefits do not outweigh the Go rationale for this choice.

**Scope:** This settles the core language. Browser JavaScript versus TypeScript, UI framework, exact transport/rendering boundary, and interaction scope remain open under the UI discussion (Q3). It does not approve a specific milestone, require a terminal-first interface, or authorize implementation. Existing code is useful reference material, not a requirement to preserve the current architecture.

### Pose-based component assembly with optional embedded tracing (Turn 4, resolves Q2)

**Decision:** Express physical component assembly using immutable, value-style `Pose` operations inside named component functions (`PlaceStem`, `PlaceSaddle`, `PlaceHandlebar`, etc.), returning `Pose`-valued landmarks alongside component setup dimensions, with an optional embedded `*Trace` sink on `Pose` to record construction steps for inspection.

**Key Design Conditions:**
1. **Value-style `Pose` movements:** Movement methods (`Advance`, `Translate`, `Turn`) return new `Pose` values so branching from a common attachment (such as a saddle's `RailCenter` or a frame's `HeadTubeTop`) is natural and cannot mutate prior poses.
2. **Pose-valued landmarks with explicit coordinate-frame meanings:** Placement structs retain `Pose`s for attachments and physical landmarks rather than bare points, preventing callers and renderers from reconstructing orientations via scattered trigonometry. Each landmark's local coordinate axes must have a documented physical meaning (e.g., `BRP` and `SitBones` carry the saddle-rail construction frame, not a curved saddle-surface normal). Raw points remain internal to low-level geometry helpers and final scalar readouts (`X()`, `Y()`, `DistanceTo`).
3. **Optional `*Trace` sink with explicit step parentage:** A `Pose` carries an optional `*Trace` pointer and `stepID int`. When tracing is `nil` (such as during solver searches), movement methods skip record collection and string formatting; when non-nil (such as when inspecting a build in the workbench), they record sink-free geometry snapshots (`From`, `To`), operation labels, inputs, and explicit `ParentID -> ID` provenance using the exact same calculation path.
4. **Freeze/detach trace before rendering:** Each traced evaluation uses its own `Trace` instance and freezes or detaches recording when assembly completes, ensuring that downstream rendering helpers (e.g., offsetting from a landmark `Pose` to draw a tick mark or dimension callout) never append to the assembly history.
5. **Separate placement from outline projection:** Moving along a construction path (such as advancing half a stem clamp height to reach its center) is distinct from a part's full physical outline. Component functions return placement geometry and setup dimensions; the renderer projects physical shapes in those local `Pose` frames without recalculating angles.
6. **Verification:** Identical inputs must produce identical placed geometry whether tracing is enabled or disabled.

---

## 2. Converged Proposals (Pending Human Sign-off)

Common ground from Turns 1–4 in [DIALOGUE.md](DIALOGUE.md), limited to the shared substance below:

- **Single-bike visual workbench for Milestone 1 (Endorsed by both agents in Turn 2 & Turn 4):** Start Milestone 1 with a single known bike in side view showing physical outlines, named landmarks, local coordinate axes, dimensions, editable component settings, and selectable construction trace steps. Defer multi-bike ghost overlays and chart-residual visualizers to later milestones.
- **Forward evaluation supports search:** Enumerate component choices and evaluate candidate assemblies for chart reconstruction and rider fit. The frame chooser reuses fit evaluation across frames and sizes. This agreement does not require eliminating all direct inverse solutions or assume the objectives separate.
- **Catalogs can be loaded as data:** Both agents favor structured definitions that can be ingested without recompilation (Gemini's Q4 recommendation; ChatGPT Turn 2). Format and persistence details can wait until ingestion is implemented.

---

## 3. Active Design Options (Under Discussion)

### Browser interaction & frontend boundary (Q3)

| Aspect | Lightweight Server/Fetch UI | Richer Reactive Frontend (e.g. Svelte/TS) |
| --- | --- | --- |
| **Rendering boundary** | Go renders SVG (with `data-step-id` / `data-landmark` attributes) or returns JSON scene data to a small vanilla JS/TS script that updates the DOM on input/slider changes. | Go serves structured JSON (`Placement` + `Trace` + outlines); a TypeScript/Svelte frontend renders SVG components and manages selection/pan/zoom state client-side. |
| **Toolchain weight** | Zero `node_modules` or bundler required if using vanilla JS/HTML templates embedded in Go. | Requires a frontend build step (`fnm` / `npm` / Vite or esbuild), trading build tooling for richer client-side UI ergonomics. |

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

- **Q3 (Visualization Scope & Browser Boundary for Milestone 1):**
  - *Question:*
    1. **Scope:** Do you approve the converged **one-bike workbench scope** for Milestone 1 (side view, editable settings, landmarks/axes/dimensions, selectable construction trace steps; deferring multi-bike overlays and chart diagnostics to later milestones)?
    2. **Browser / Frontend Stack:** For the browser UI talking to the Go core, do you prefer a **minimal zero-bundler frontend** (Go HTTP server + embedded HTML/SVG/vanilla JS or single-file TS) or a **TypeScript + Svelte/reactive frontend** rendering Go's JSON scene/trace output?
  - *Recommendations:* Both agents now recommend the focused one-bike scope for Milestone 1. On frontend stack, choose based on whether you want zero Node/bundler tooling in the repo vs. a full client-side component framework.
  - *Blocks:* Finalizing Milestone 1's browser/core interface and visual acceptance criterion.

### Deferred

- **Q4 (Data & Catalog Storage):**
  - *Question:* Should frames and component catalogs remain compiled-in structs/code, or move to structured data files (e.g., TOML/YAML/JSON) so an AI MCP solver can ingest, test, and persist new frames without recompiling?
  - *Recommendation (Both agents):* Agree on structured data ingestion without recompilation; defer the exact file format and persistence mechanism until Milestone 3 (chart reconstruction & agent access). Milestone 1 can use an explicit Go fixture.
  - *Revisit when:* Designing the first import/save operation in Milestone 3.
