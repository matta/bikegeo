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

---

## 2. Converged Proposals (Pending Human Sign-off)

Common ground from Gemini's Turn 1 and ChatGPT's Turn 2 in [DIALOGUE.md](DIALOGUE.md), limited to the shared substance below:

- **Readable physical assembly:** A small pose/movement vocabulary supports named component builders. Bike construction should read as physical steps; avoid a general CAD framework or elaborate type machinery.
- **Shared geometry for inspection and solving:** Drawings and measurements use the same placed components and landmarks. A visual workbench and inspectable construction belong in the first milestone. Whether every evaluation emits drawable primitives and a trace remains open.
- **Forward evaluation supports search:** Enumerate component choices and evaluate candidate assemblies for chart reconstruction and rider fit. The frame chooser reuses fit evaluation across frames and sizes. This agreement does not require eliminating all direct inverse solutions or assume the objectives separate.
- **Catalogs can be loaded as data:** Both agents favor structured definitions that can be ingested without recompilation (Gemini's Q4 recommendation; ChatGPT Turn 2). Format and persistence details can wait until ingestion is implemented.

These proposals remain pending human approval. The separate core-language decision above is confirmed.

---

## 3. Active Design Options (Under Discussion)

### Assembly and inspection

| Aspect | Gemini's proposal | ChatGPT's proposal |
| --- | --- | --- |
| Construction output | Each physical builder step emits a drawable primitive and trace entry. | Builders return placed components and landmarks; rendering projects that geometry. Construction tracing can be omitted during search without changing numerical results. |
| API style | Compare immutable chaining and a scoped cursor before choosing. | Prefer immutable poses inside ordinary component functions; permit a scoped recorder if it improves readability. Pose mutation and trace recording are separate choices. |
| Spatial scope | Local 2D pose as the core vocabulary. | Planar frame assembly, retaining lateral handlebar geometry and explicit world-relative roll; begin with a side view. |

Turn 2 includes comparable stem sketches. ChatGPT's refinements await Gemini's review; they are not additions to the converged section.

### Solver policy

- **Gemini:** Prefer forward-only assembly with generic bisection/grid sweeps, replacing bespoke inverse trigonometry.
- **ChatGPT:** Use forward assembly as the final check; use bounded numerical search, enumeration, or a small direct geometric solve according to the problem. Keep observations separate from inferred models, permit multiple chart explanations, and assess the combined fit objective before independently selecting front/rear builds. Preserve explicit preference costs and distinguish infeasible hardware from imperfect fit.
- **Evidence needed:** Turn 1's runtime and code-size figures are unmeasured estimates. Turn 2 records bisection bounds, search counts, and specific corrections to coordinate and fit-independence claims. Solver details need review before their implementation milestone, not before drawing the first bike.

### Proposed milestone sequence (ChatGPT; pending review and approval)

1. **One-bike visual workbench:** Assemble one known rigid-frame bike with configurable fork compression, spacers/stem, and saddle settings; show a side view with named landmarks, axes, dimensions, and selectable construction steps. Declare wheel sizes and orientation conventions. Matt can read a component recipe, predict an adjustment, and inspect the result; independent dimensions and known fixtures verify it. Full charts, catalog persistence, overlays, and solvers are outside this milestone.
2. **One-bike fit search:** Add explicit targets, limits, and preference costs; verify returned builds through assembly and open them directly in the workbench. Use a small exhaustive fixture to check candidate selection, then measure representative search time and responsiveness.
3. **Chart reconstruction and agent access:** Preserve original observations and rounding information; show candidate hypotheses, residuals, and unresolved quantities. Verify with a known reconstruction and an intentionally ambiguous case. Expose structured operations over the same application functions and choose catalog persistence here.
4. **Frame comparison:** Reuse fit search across a small catalog, return best builds and their explanations, and report search coverage and sensitivity to assumptions before expanding the catalog.

This sequence is an implementation proposal, not authorization to build it. The Go core should remain independent of the UI framework and agent transport.

---

## 4. Decision Queue (For the Human Operator)

- **Q2 (Turtle / Assembly API Shape):**
  - *Question:* Which stem recipe in Turn 2 is easier to read: **immutable pose values** or a **scoped recording cursor**? Either can sit inside named component functions; neither requires copying an entire scene on each step.
  - *Turn 1 Recommendation (Gemini):* Compare concrete side-by-side sketches of both styles in Turn 2 before choosing.
  - *ChatGPT (Turn 2):* Immutable pose values plus component functions, with optional recording for inspection. Ask Gemini to review the assembly/output distinction before treating it as agreed.
  - *Blocks:* Finalizing the core `turtle` / `rig` package interface for Milestone 1.
- **Q3 (Visualization Scope for Day 1):**
  - *Question:* For the initial visual workbench (Milestone 1), what views are essential from day one (e.g., single-bike interactive turtle inspector with coordinate axes, multi-bike ghost overlay anchored at BB vs. SitBones, chart closure residual visualizer)?
  - *Turn 1 Recommendation (Gemini):* Start Milestone 1 with a single-bike interactive SVG view + optional ghost overlay (anchored at BB or SitBones) + hoverable turtle step inspector.
  - *ChatGPT (Turn 2):* One bike, side view, explicit coordinates and dimensions, editable settings, and selectable construction steps. Defer overlays and chart residual views until their workflows exist; retain the lateral geometry needed to place hand contacts correctly.
  - *Turn 3 clarification:* Go is confirmed for the core. The browser's interaction model and implementation remain undecided: an inspector driven by settings and a direct manipulation editor have different needs. Choose the required experience before specifying the frontend stack and exact browser/core boundary.
  - *Blocks:* Scoping the Milestone 1 visual acceptance criterion and its browser/core interface.

### Deferred

- **Q4 (Data & Catalog Storage):**
  - *Question:* Should frames and component catalogs remain compiled-in structs/code, or move to structured data files (e.g., TOML/YAML/JSON) so an AI MCP solver can ingest, test, and persist new frames without recompiling?
  - *Turn 1 Recommendation (Gemini):* Structured data files (or a hybrid where the MCP solver can load/save frame definitions dynamically) removes the compile step from AI frame ingestion `(c)`.
  - *ChatGPT (Turn 2):* Agree on data ingestion; prefer versioned structured documents with sources and assumptions. Defer the format and persistence mechanism; Milestone 1 can use one explicit fixture.
  - *Revisit when:* Designing the first import/save operation in the chart reconstruction milestone. Blocks that persistence design, not Milestone 1.
