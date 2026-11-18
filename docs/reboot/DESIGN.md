# Canonical Design Intent (`bikegeo` Reboot)

> Governed by [README.md](README.md). Only the Human Operator may move items into **Human-Confirmed Decisions**.

---

## 1. Human-Confirmed Decisions

### Core Requirements (Established by Human Operator)
- **(a) Clear, readable physical code:** Physical structures must be built up from higher-level directional concepts (e.g., a turtle-graphics / local-pose API with higher-level component builders such as seatpost, saddle, fork, spacers, stem, and handlebars) rather than raw trigonometric equations scattered across files.
- **(b) Visualizations from the start:** Resulting physical structures must be rendered graphically from day one.
- **(c) Published frame geometry solvers + AI primitives:** Deduce physical geometry from incomplete/rounded manufacturer or custom-builder charts, providing solver primitives that AI agents can invoke via MCP or structured API endpoints.
- **(d) Fit solvers:** Given a fit target, permute seatpost height, saddle fore/aft, fork A2C/sag, headset spacers, stem lengths/angles, and handlebar choices to find good fits.
- **(e) Frame choosers:** Solve fit across many frames and rank them by best possible fit.

*(Additional architectural decisions will be moved here from the Decision Queue as the Human Operator rules on them.)*

---

## 2. Converged Proposals (Pending Human Sign-off)

*(None yet — awaiting Turn 2 review from ChatGPT in [DIALOGUE.md](DIALOGUE.md).)*

---

## 3. Active Design Options (Under Discussion)

From **Turn 1 — Gemini** in [DIALOGUE.md](DIALOGUE.md):
- **Forward-only turtle kinematics + generic solvers:** Replace bespoke inverse trigonometry with fast forward assembly + generic bisection/grid sweeps for both chart deduction `(c)` and fit solving `(d)`.
- **Unified Assembly + Scene Graph:** Every physical builder step advances a local 2D `Pose` *and* emits a drawable primitive + trace entry, ensuring the code that computes coordinates is identical to the code that draws the bike.
- **Language & UI Stack:**
  - *Option A:* Go core engine + embedded reactive TypeScript/SVG canvas + built-in MCP server.
  - *Option B:* All-TypeScript (browser UI + Node/Bun MCP server sharing the exact same assembly code).

---

## 4. Decision Queue (For the Human Operator)

- **Q1 (Language & Runtime Boundary):** Do you prefer **Option A** (stay in Go for the geometry/solver core and MCP server, with a lightweight TypeScript/SVG frontend for interactive visuals) or **Option B** (all-TypeScript across browser UI and MCP server), or another stack?
- **Q2 (Turtle / Assembly API Shape):** Should the physical builder API be **pure immutable value chaining** (returning a new `Pose` + appending to an explicit `Scene` value) or a **scoped builder/cursor** (`rig.Step(...)`) that records both landmarks and drawable shapes?
- **Q3 (Visualization Scope for Day 1):** For the initial visual workbench, what views are essential from day one (e.g., single-bike interactive turtle inspector, multi-bike ghost overlay anchored at BB vs. SitBones, chart closure residual visualizer)?
- **Q4 (Data & Catalog Storage):** Should frames and component catalogs remain compiled-in structs/code, or move to structured files (e.g., TOML/YAML/JSON/SQLite) now that an AI MCP solver will be ingesting and proposing new frames interactively?
