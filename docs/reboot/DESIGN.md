# bikegeo Reboot: Design and Plan

A clean-slate reboot built beside the existing application under `cmd/bikegeo-reboot` and `internal/reboot/...`. This file describes the current design; git history holds past discussion.

## Working rules

- Matt makes the architectural decisions. Agents propose, critique, and record his explicit rulings here. Agreement between agents is not approval, and a plan is not permission to change code.
- When a choice is open, sketch the options briefly and recommend one. Don't write a deep spec that assumes an answer. Number open questions `Q#` and never reuse a number.
- Prefer fewer types and clearer physical steps over heavier machinery. Show how a real bike assembly step reads in code.
- Back technical claims with arithmetic, existing behavior, or a measurement. Two agents agreeing does not prove an unmeasured claim.

## Requirements

- **(a) Readable physical code:** build structures from directional pose steps and component builders (seatpost, saddle, fork, spacers, stem, handlebar), not scattered trig.
- **(b) Visualization from day one.**
- **(c) Chart solvers + agent primitives:** deduce geometry from incomplete or rounded charts; expose solver operations to AI agents (MCP or structured API).
- **(d) Fitting:** choose seatpost, saddle fore/aft, fork A2C/sag, spacers, stem, and handlebar settings that meet a fit target.
- **(e) Frame chooser:** rank many frames by best achievable fit.

## Confirmed decisions

### Go core, Go-rendered workbench

- Go owns the physical model, assembly, solvers, and shared operations. Browser and agent interfaces call it rather than duplicate geometry.
- Go renders HTML and SVG from placements and frozen traces. A small, dependency-free vanilla JS layer handles fetched updates (`fetch` + `AbortController`, stale responses ignored), trace/diagram highlighting (hover, click, keyboard), focus, and URL history. Pages must still work as plain `GET` forms without JS. There is no framework, npm, or build step. Assets are embedded.
- The diagram has three fixed server-rendered views (`view=bike|seat|stem`) instead of free pan/zoom.

### Pose-based assembly

- `Pose` is an immutable value. `Advance`, `Translate`, `Turn`, and `Face` return new poses, so branching from a shared attachment is safe. `Face(label, worldHeadingDeg)` sets an absolute world heading. It is used for parts leveled against the world, not against their mounting tube (saddle `RailPitchDeg`, handlebar `BarRollDeg`).
- Component functions (`PlaceStem`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceHandlebar`, ...) return placements whose landmarks are `Pose`s. Each landmark's local axes have a documented physical meaning. For example, `BRP` and `SitBones` carry the rail frame, not a saddle-surface normal.
- A pose can carry an optional `*Trace`. When it is nil (fitting and other solver loops), no records are made. When set, each move records `From`/`To`, label, inputs, and `ParentID → ID`. Each evaluation gets its own trace, frozen before rendering. **Placements must be identical with tracing on or off.**
- Placement (moving along a construction path) is separate from outline projection. The renderer draws outlines in landmark frames without recomputing angles.
- Trig lives in `pose` (rule in AGENTS.md).

### `assembly` builds and measures; `web` only renders

Test: if fitting would need the code without HTTP, it belongs in `assembly`.

- Composition (frameset → seatpost → saddle, stem → handlebar) and ending the trace happen in one `assembly` entry point.
- Any scalar derived from placed poses is a method on a placement type. Callers, including tests, call it rather than restating the expression.
- Outlines and readouts use the setup that was actually placed, never fixture defaults or a second computation.
- The fixture is application input. `assembly` never imports it.

### Frameset leveling: one recipe, two passes

`placeFramesetAtPitch(frameRoot, frame, lowerCupMm, fork)` walks from `BB`: rear axle, then head tube top (reach, stack), then down the steerer by HTL, lower cup, and fork A2C, then rake at `+90°` to the front axle; separately, up the seat axis to `SeatTubeTop`.

1. Run it at 0° and measure `PitchDeg = -RearAxle.AngleTo(FrontAxle)`. The same run on `FrameChart` values exposes the raw chart-closure residuals.
2. Run it again rooted at `bb.Turn("frame pitch to level axles", PitchDeg)`. The axles come out level and every traced step uses real dimensions.

Rotating about `RearAxle` and re-centering `BB` equals rotating about `BB`, so no closed-form pitch trig is needed.

### Coordinate and sign conventions

| Area | Convention |
| --- | --- |
| World | 2D side view, `BB = (0,0)`, `+X` forward, `+Y` up, positive turns CCW. Lengths are `float64` mm; angles are `pose.Deg`, with the heading convention owned by `Pose`. |
| Wheels | Equal radii. Leveled axles sit at `Y = BbDrop`; ground is at `Y = BbDrop − R_wheel`. |
| Stem | From `SteererUp` (`180° − HTA`): `UpperCoverMm` + `SpacersMm` + clamp/2 → `SteererClamp`; turn `−90° + StemAngleDeg`, advance `StemLengthMm` → `BarClamp`. (`UpperCoverMm` should move to a headset setup.) |
| Handlebar | `Face(−BarRollDeg)` from `BarClamp` (positive roll is clockwise); 3D grip offset projected to the side view → `HandGrip`. Backsweep is positive rearward; upsweep is positive upward. |
| Seatpost | From `SeatTubeTop` (`180° − STA`): advance `ExtensionMm`, translate local `+Y` by `PostSetbackMm` → `SeatpostClamp`. |
| Saddle | `Face(RailPitchDeg)` (positive raises the nose); advance `−RailOffsetMm` (**positive moves the saddle rearward**) → `RailCenter`; `BRP` and `SitBones` are offsets in the rail frame. |
| Readouts | `BrpToGripDistMm = |HandGrip − BRP|`; `SitBonesToGripRiseMm = HandGrip.Y − SitBones.Y` (measured from the sit bones so it carries across saddles). |
| CAD chart vs. installed fork | `FrameChart.CadFork` and `CadLowerCup` hold the manufacturer's CAD geometry, keeping bare `FrameSetup` free of chart quirks. Installed forks pair with a physical `LowerCupSetup`. |

### CAD Baseline Fork in the workbench

- `fork=cad` resolves A2C, cup, and sag (0%) from `FrameChart`, in one place, and ignores submitted overrides. Those fields render `readonly`. Sag is also read-only for rigid forks.
- Only the cup + A2C sum moves the front axle, so locking loses nothing. On the Fenrir, a 12 mm EC44 cup + 483 mm A2C reproduces the CAD 0 mm + 495 mm.
- This is a workbench rule only. `assembly` still accepts any explicit cup and fork.
- Every visible label says "CAD Baseline Fork". The explanatory note is built from fixture data.

### Fitting (Q6–Q18, ruled 2026-11-19)

- **Q8, terminology:** the operation of finding a build's settings for a fit target is **Fitting** (noun and result type), **fit** (verb), and **`ApplyFitting`** (entry point). Don't call it "search" in code or docs.
- **Q7, saddle target:** the saddle fit target is **BRP**, the landmark the fit report measured. SitBones stays a model-derived readout.
- **Q6, solver policy for saddle fitting:** a direct solve is acceptable as long as forward assembly (PlaceBike) remains the final check of every fitted build.
- **Q9, rail range:** `PlaceSaddle` rejects rail offsets outside `±(RailLengthMm − ClampLengthMm)/2`, the same range fitting respects. This lands as its own commit after saddle fitting.
- **Q10, fixture rail and clamp lengths:** the Fenrir fixture's 60 mm rail length and 30 mm clamp length are measured and authoritative. A fitting that is limited by the ±15 mm rail range is a real finding, not a fixture artifact.
- **Q11, cockpit target:** the front-cockpit fit target is a **HandGrip** point in world X/Y (BB origin), like the BRP target. With the frameset fixed, the saddle and cockpit fits are independent.
- **Q12, solver policy for front-cockpit fitting:** M2 Slice 2 fits `SpacersMm` and stem `LengthMm` as continuous values with a direct 2×2 solve at the submitted stem angle and bar roll, reusing Slice 1's box logic. Discrete stock parts and preference costs are M2 Slice 3. **Forward pose-based assembly stays the source of truth for geometry:** a direct solve reads each fitted input's direction from the probe placement's poses and never restates the assembly formulas, and PlaceBike checks every fitted build.
- **Q13, cockpit limits:** `StemSetup` carries `MaxSpacersMm` (the steerer allowance; `PlaceStem` rejects more, like Q9) and `MinLengthMm`/`MaxLengthMm` (the stem range fitting may choose; `PlaceStem` ignores it).
- **Q14, one entry point:** `ApplyFitting` takes one `FitTarget` with optional BRP and HandGrip landmarks and returns one fitted build, so the workbench has one apply link.
- **Q15, saddle first:** saddle fit is the primary fit criterion and must be right. Cockpit fit is secondary. Stock cockpit fitting never changes the saddle fit; when a BRP target is set, every stock cockpit build carries the saddle fitting's values.
- **Q16, solver policy for discrete part choice:** enumerate every stock combination, place each one forward with PlaceBike, price it, and rank. No direct solve and no rounding of a continuous answer.
- **Q17, HandGrip miss price:** port the legacy oval ("Area of Reasonable Fit"): semi-axes 10 mm and 20 mm, major axis tilted 30° clockwise from vertical (world heading 60°), centered on the HandGrip target, free inside, 22 points per unit of normalized radius R beyond R = 1.
- **Q18, stock fitting result:** a ranked list of the 5 cheapest stock cockpit builds, each with its ledger lines and its own apply link. Slice 2's continuous fitting stays as the explanation (for example, "needs a 134 mm stem").

## Run and inspect

- `make serve-reboot` serves on :8081. `/` is the stem preview and `/bike` is the full bike. State lives in the URL query.
- Each landmark and trace row shares a step ID. Hover, focus or click a row to highlight its vector and local +X/+Y axes.
- Read a recipe top-down: each row is one `advance`, `turn`, `face` or `translate` from its parent step.
- `view=bike|seat|stem` picks the diagram framing. `fork=cad` is the locked CAD Baseline Fork.
- Fixture data is in `internal/reboot/web/fixture.go`; its values are labelled measured, catalog or illustrative.

## Layout and boundaries

```text
cmd/bikegeo-reboot/       entrypoint; `serve` on port 8081 (make serve-reboot / build-reboot)
internal/reboot/pose/     poses, units, primitives, optional trace
internal/reboot/assembly/ component setups, placements, whole-bike construction
internal/reboot/web/      handlers, HTML/SVG, embedded CSS/JS, Otso Fenrir fixture
```

- Dependencies flow `cmd → web → assembly → pose`. Add packages only when a slice needs them. Keep each component's setup and placement types with its assembly code; don't rebuild a catch-all `model` package.
- `make boundary` enforces the import isolation and the banned-term rule described in AGENTS.md.
- To reuse legacy code, read, audit, and port only what's needed. Never wrap an old implementation under a new name. Small duplicated primitives are acceptable; a shared package created just to avoid duplication is not. A parity test must explain differences, not assume the old result is right.
- The fixture is `internal/reboot/web/fixture.go` (Otso Fenrir Stainless 2024, size L). Label values as measured, catalog, or illustrative. Never present an invented outline as manufacturer truth.

## Milestones

1. **One-bike visual workbench (a, b): done.** Accepted by Matt; tests cover trace on/off equivalence, independent geometric fixtures, and HTML/SVG step-ID correspondence in every view. Solver-free: the seatpost control edits `ExtensionMm` directly.
2. **One-bike fitting (d):** explicit fit targets, hardware limits, and preference costs. Produce candidate builds, check each with forward assembly, and open them in the workbench.
   - **Slice 1, saddle fitting (done 2026-11-19):** fit `SeatpostSetup.ExtensionMm` and `SaddleSetup.RailOffsetMm` to a BRP target with a direct 2×2 solve (Q6). Rail pitch and post setback stay fixed inputs. Limits are the post's min/max extension and the rail clamp range from `RailLengthMm`. The result reports the fitted setups, the BRP actually reached, the miss in mm, and any limit hit, and keeps *reached*, *clamped at a limit*, and *invalid input* distinct. PlaceBike on the fitted setup must land BRP on the target (to 1e-9 mm when no limit binds). `pose` gains one operation that decomposes a vector along two headings. `/bike` gets a BRP target field and a plain-`GET` link to the fitted build. Tests: fit → place round trip, out-of-range target clamps to the right limit, trace on/off equivalence, the fit report's BRP (−219, 713.1). Out of scope: post setback choice, the front cockpit, preference costs, other frames.
   - **Slice 2, front-cockpit fitting (done 2026-11-19):** fit `StemSetup.SpacersMm` and `LengthMm` to a HandGrip target with a continuous direct 2×2 solve (Q11–Q14). Stem angle and handlebar stay fixed inputs. Limits are `MaxSpacersMm` and the stem length range on `StemSetup`. The same `ApplyFitting` fits the saddle, the cockpit, or both, and `/bike` gets a HandGrip target and one apply link.
   - **Slice 3, stock cockpit fitting (done and accepted 2026-11-19, plan in [SLICE.md](SLICE.md)):** choose stock stem lengths, stem angles, and spacer steps for a HandGrip target by enumerating every combination with PlaceBike, pricing each on the points-lost ledger (Q17 oval plus stem length, stem angle, and spacer bands), and listing the 5 cheapest with apply links (Q15–Q18). The saddle fit is unchanged and carried into every build. The handlebar stays fixed.
   - **Later:** handlebar choice.
3. **Chart reconstruction, catalog persistence, agent access (c):** keep the original observations and rounding budgets, evaluate fork/sag/cup hypotheses, show residuals and ambiguous explanations, and expose operations over MCP or the CLI.
4. **Frame chooser (e):** run fitting across a catalog and report the best builds, coverage, and sensitivity.

## Open questions

- **Solver policy (M2 saddle decided in Q6, continuous front cockpit in Q12, discrete part choice in Q16; decide charts at the start of M3):** forward assembly is the evaluator and final check in every case. Options: generic bisection or grid sweeps, or a per-problem choice of bounded search, enumeration, or a small direct solve. Keep observations separate from inferred models, allow multiple chart explanations, and distinguish infeasible hardware from an imperfect fit.
- **Q4, catalog storage (decide at M3's first import/save):** versioned structured data files (TOML/YAML/JSON) with sources and assumptions, loadable without recompiling.

## Cutover (a separate decision, later)

Until cutover, both applications coexist and reboot work doesn't refactor the old one. At cutover:

1. List the old workflows Matt still uses and replace or retire each one.
2. Migrate datasets and provenance; never adjust published values to make the new engine agree.
3. Tag a known-good old revision.
4. Delete the `*_parity_test.go` files and the old packages, move `internal/reboot/*` to `internal/*`, and promote the entrypoint to `cmd/bikegeo`.
5. Run full verification.
