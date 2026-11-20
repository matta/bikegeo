# bikegeo Reboot: Design and Plan

A clean-slate reboot built beside the existing application under `cmd/bikegeo-reboot` and `internal/reboot/...`. This file describes the current design; git history holds past discussion.

## Working rules

- Matt makes the architectural decisions. Agents propose, critique, and record his explicit rulings here. Agreement between agents is not approval, and a plan is not permission to change code.
- When a choice is open, sketch the options briefly and recommend one. Don't write a deep spec that assumes an answer. Number open questions `Q#` and never reuse a number.
- Prefer fewer types and clearer physical steps over heavier machinery. Show how a real bike assembly step reads in code.
- Back technical claims with arithmetic, existing behavior, or a measurement. Two agents agreeing does not prove an unmeasured claim.
- **Scoring words:** a build's **score** is 100 − points lost. A band is free inside its range and **penalizes** values outside it; the points it takes off are its **penalty**. Don't describe scoring as pricing or cost in prose. Call a band's rate its points per unit, the list of a build's penalties its penalty lines, and the best builds best-scoring. `make boundary` bans the words `price`, `pricing`, `cost`, `cheap`, and `ledger` in reboot files (Q40).

### Slice planning

Each slice plan is its own file, `SLICE<n>.md` beside this one (for example `SLICE7.md`), and it lives only until Matt accepts the slice. Several slices may be planned ahead together, as one batch in one planning session; each plan names the slices it builds on.

**Planning gate:** don't start a new batch while any `SLICE*.md` file remains beside this one, whatever its status (planned, partly implemented, blocked, or awaiting acceptance). Every slice in the current batch must be implemented and accepted first. Instead of planning, list each remaining plan with its Status line and ask Matt to review and accept it (see "After acceptance"). Hold to this even when Matt asks for a new plan. Planning resumes only when no `SLICE*.md` file remains.

Before writing a plan, get Matt's rulings on the slice's open choices (numbered `Q#`) and record them under **Confirmed decisions** here. The plan cites them and doesn't decide them.

Size the plan by the work. Count the code, test, template, and asset files the slice touches. The status edits to the slice plan and DESIGN.md don't count.

- **Small: one commit and no more than 3 files.** Keep the plan sparse: the goal, the rulings it rests on, the files and the change in each, the tests and their expected values, and the verification steps. The planning agent offers to implement it itself.
- **Anything larger.** Write the plan in enough detail that a less capable model (for example Sonnet) can implement it reliably without making design choices. It gives exact type, field, and function signatures; every caller to update; URL and form keys; the commit split with subjects; each test with its inputs, its expected values (measured by an independent probe), and their tolerances; the rules that bite; the verification steps; and an out-of-scope list. The planning agent suggests delegating the implementation to that model.

Each planned commit must pass `make pre-commit` on its own. In particular, don't add a function in a commit before the one that first calls it from `main`: `make deadcode` rejects it. Every plan's "Rules that bite" section ends by requiring the implementation handoff below.

#### Implementation handoff

The implementing agent's last step is to record what it did and the state it left. It does this whether it finished, stopped early, or got blocked. The record goes in the same change as the work, or stays uncommitted alongside the work if the work isn't committed. A reviewer adds its findings under the same section and doesn't rewrite the implementer's record.

1. **The slice plan's Status line** says one of: `Implemented <date>; awaiting Matt's acceptance`, `Partly implemented <date>; stopped after <plan step>`, or `Blocked <date> on <question>`.
2. **An `## Implementation handoff` section** sits right after the Status paragraph in the slice plan. It contains:
   - **Who and when:** the date and the model that did the work.
   - **Commits:** each planned commit's subject with its hash, or "not committed". For uncommitted work, list the files that hold it and which planned commit each change belongs to.
   - **Done and not done:** by the plan's step IDs (for example A1, W3, N4).
   - **Deviations:** every place where the code or tests differ from the plan, and why. Write "None" if there are none. A deviation needs a reason. "Seemed cleaner" isn't one; stop and ask instead (see the plan's header).
   - **Verification:** the result of `make pre-commit` on the final state, and the plan's live check: what you confirmed, with numbers, and where the screenshot is. If you skipped it, say so and why.
   - **Open issues:** findings, questions for Matt, and anything you noticed but left alone.
3. **DESIGN.md's Milestones line** for the slice uses the same status words as the Status line. Don't touch any other part of DESIGN.md. Only Matt's acceptance makes a slice "done" (see "After acceptance").

#### After acceptance

When Matt accepts a slice, delete its plan file in the same commit that marks the slice accepted. After that, the code, the tests, and git history are the record. That commit also does three things:

1. **The Milestones line** says `done and accepted <date>` and lists the slice's commit hashes in place of the plan link.
2. **Links to the plan:** remove or rewrite every link to the deleted file, in DESIGN.md and in other slice plans. A plan that builds on the slice names it by number and lists its commits.
3. **Anything that must outlive the plan** moves into DESIGN.md first. That means a ruling not yet under **Confirmed decisions**, or an open issue Matt wants kept. Acceptance closes the handoff's other open issues, and the rest of the plan is dropped.

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
- **Q12, solver policy for front-cockpit fitting:** M2 Slice 2 fits `SpacersMm` and stem `LengthMm` as continuous values with a direct 2×2 solve at the submitted stem angle and bar roll, reusing Slice 1's box logic. Discrete stock parts and preference penalties are M2 Slice 3. **Forward pose-based assembly stays the source of truth for geometry:** a direct solve reads each fitted input's direction from the probe placement's poses and never restates the assembly formulas, and PlaceBike checks every fitted build.
- **Q13, cockpit limits:** `StemSetup` carries `MaxSpacersMm` (the steerer allowance; `PlaceStem` rejects more, like Q9) and `MinLengthMm`/`MaxLengthMm` (the stem range fitting may choose; `PlaceStem` ignores it).
- **Q14, one entry point:** `ApplyFitting` takes one `FitTarget` with optional BRP and HandGrip landmarks and returns one fitted build, so the workbench has one apply link.
- **Q15, saddle first:** saddle fit is the primary fit criterion and must be right. Cockpit fit is secondary. Stock cockpit fitting never changes the saddle fit; when a BRP target is set, every stock cockpit build carries the saddle fitting's values.
- **Q16, solver policy for discrete part choice:** enumerate every stock combination, place each one forward with PlaceBike, score it, and rank. No direct solve and no rounding of a continuous answer.
- **Q17, HandGrip miss penalty:** port the legacy oval ("Area of Reasonable Fit"): semi-axes 10 mm and 20 mm, major axis tilted 30° clockwise from vertical (world heading 60°), centered on the HandGrip target, free inside, 22 points per unit of normalized radius R beyond R = 1.
- **Q18, stock fitting result:** a ranked list of the 5 best-scoring stock cockpit builds, each with its penalty lines and its own apply link. Slice 2's continuous fitting stays as the explanation (for example, "needs a 134 mm stem").

### Handlebar choice (Q19–Q21, ruled 2026-11-19)

- **Q19, workbench first:** M2 Slice 4 adds a handlebar list to the fixture and a `bar` select to `/bike`. Fitting and stock fitting use the submitted bar unchanged. Bars join stock fitting in a later slice.
- **Q20, bar list:** port all 15 bars of the legacy catalog (`internal/catalog/handlebars.go`) exactly, labelled as legacy catalog values with sources not recorded. SQlab bend starts are illustrative (legacy formula 55 + 0.9·rise).
- **Q21, roll:** `BarRollDeg` stays a submitted fixed input. Changing the bar keeps the submitted roll.

### Handlebars in stock fitting (Q22–Q25, ruled 2026-11-19)

- **Q22, opt-in:** a `stock_bars=selected|all` workbench control. `selected` (the default) keeps today's behavior: stock fitting uses the submitted bar. `all` makes stock fitting enumerate every fixture bar.
- **Q23, bar penalty:** bars are free. There is no bar band; bars compete only through the HandGrip oval and the stem and spacer bands. Bar preferences may become penalty bands later. (Superseded by Q30.)
- **Q24, roll locked:** every bar stock fitting installs gets the submitted `bar_roll`. Roll enumeration is not planned.
- **Q25, result:** Q18 is unchanged: the 5 best-scoring builds across all bars, each row naming its bar. Several rows may share one bar.

### Seatposts, bar sweep, and fork choice (Q26–Q34, ruled 2026-11-19)

- **Q26, slice order:** Slice 6 is the seatpost select with setback notes, Slice 7 the bar sweep band, and Slice 8 fork choice in stock fitting. Each has its own plan file.
- **Q27, the seatpost is fixed:** fitting and stock fitting never choose the seatpost or its setback. The workbench gets a `post` select of fixture seatposts. It replaces the free `post_setback` field: each post carries its setback, extension limits, and clamp length, and the page shows the setback read-only.
- **Q28, fixture seatposts:** generic posts with 0, 15, and 25 mm setback (illustrative, with today's Ritchey-based extension limits), and the Redshift ShockStop PRO at **12 mm setback when sagged** (owner-reported 2026-11-19). Its extension limits and clamp length have not been supplied, so they use the generic illustrative values until Matt measures them. The FAIR Drop Best is neither modeled nor offered.
- **Q29, setback notes:** with a BRP target, the saddle fitting block always reports the post setback that puts BRP on the target with the saddle centered on its rails. When saddle fitting stops at a limit, it also reports the setback that would reach the target with the rails held at the fitted offset. These notes are explanations. They are never applied or linked, and they keep the current post's extension limits.
- **Q30, bar sweep band:** bars are scored by backsweep only. 16°–20° is free (every bar in that range scores the same), and each degree outside it is penalized 2 points. The line appears in every stock fitting result, including `stock_bars=selected`. This supersedes Q23's "bars are free".
- **Q31, fork set:** fork choice enumerates the fixture forks at their catalog A2C and default lower cup. Rigid forks run at 0% sag. Suspension forks run at 15, 20, and 25% sag. The CAD Baseline Fork is excluded.
- **Q32, fork and saddle penalties:** forks are free except for sag. A fork sag band is free at exactly 20% and penalizes 2 points per percent away from it, so 15% and 25% each lose 10 points. It applies to suspension forks only. A BRP miss line penalizes the saddle's remaining miss: free up to 1 mm, then 2 points per mm (illustrative).
- **Q33, fork control:** `stock_forks=selected|all`, mirroring Q22. `selected` (the default) keeps the submitted fork. With `all`, each row names its fork and sag, and its Apply link sets them.
- **Q34, BRP miss always counts:** the BRP miss line scores the saddle fit itself, not the fork choice. It penalizes any failure to reach the BRP target, whatever the cause (frame, seatpost setback, fork, saddle). So it appears in every stock fitting result that has a BRP target, including `stock_forks=selected`. There, every row has the same miss: with the 0 mm post and the fit report BRP, every score drops by 77.2 points (78.7 → 1.5) and the ranking doesn't change.

### Scoring words in code (Q35–Q40, ruled 2026-11-19)

- **Q35, band type:** the band type is `PenaltyBand{FreeMin, FreeMax, PointsPerUnitBelow, PointsPerUnitAbove}`, and the HandGrip oval's rate is `FitOval.PointsPerR`. The fields are rates (points per unit), so **penalty** keeps meaning an amount of points.
- **Q36, penalty lines:** each scored line of a stock build is a `Penalty{Band, PointsLost}`, held in `StockCandidate.Penalties`. The band-name type is `BandName`; its constants keep their `Band…` names and string values. Prose says "penalty lines".
- **Q37, score:** a build's 100 − points lost is its **score**: `StockCandidate.Score()`, a **Score** column in the stock table, `data-stock-cell="score"`, and the note says "score = 100 − points lost". The best builds are "best-scoring".
- **Q38, free stays:** `FreeMin`/`FreeMax`, the "Free: …" preference note, and "free inside its range" keep the word *free*, meaning no penalty.
- **Q39, scope:** reboot code, tests, templates, and all of this file. Rulings are reworded without changing their substance. AGENTS.md's legacy Architecture bullet on scores stays as it is, because it documents legacy code that still uses those words.
- **Q40, enforcement:** `make boundary` bans the words `price`, `pricing`, `cost`, `cheap`, and `ledger` in the file names and contents of `cmd/bikegeo-reboot` and `internal/reboot`, like `chassis`. It matches in any case and inside longer words, so `prices`, `priced`, `PricedBand`, `CostPerR`, and `cheapest` are caught too. The list names whole words, not stems, so it reads plainly (revised 2026-11-19).

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
2. **One-bike fitting (d):** explicit fit targets, hardware limits, and preference penalties. Produce candidate builds, check each with forward assembly, and open them in the workbench.
   - **Slice 1, saddle fitting (done 2026-11-19):** fit `SeatpostSetup.ExtensionMm` and `SaddleSetup.RailOffsetMm` to a BRP target with a direct 2×2 solve (Q6). Rail pitch and post setback stay fixed inputs. Limits are the post's min/max extension and the rail clamp range from `RailLengthMm`. The result reports the fitted setups, the BRP actually reached, the miss in mm, and any limit hit, and keeps *reached*, *clamped at a limit*, and *invalid input* distinct. PlaceBike on the fitted setup must land BRP on the target (to 1e-9 mm when no limit binds). `pose` gains one operation that decomposes a vector along two headings. `/bike` gets a BRP target field and a plain-`GET` link to the fitted build. Tests: fit → place round trip, out-of-range target clamps to the right limit, trace on/off equivalence, the fit report's BRP (−219, 713.1). Out of scope: post setback choice, the front cockpit, preference penalties, other frames.
   - **Slice 2, front-cockpit fitting (done 2026-11-19):** fit `StemSetup.SpacersMm` and `LengthMm` to a HandGrip target with a continuous direct 2×2 solve (Q11–Q14). Stem angle and handlebar stay fixed inputs. Limits are `MaxSpacersMm` and the stem length range on `StemSetup`. The same `ApplyFitting` fits the saddle, the cockpit, or both, and `/bike` gets a HandGrip target and one apply link.
   - **Slice 3, stock cockpit fitting (done and accepted 2026-11-19):** choose stock stem lengths, stem angles, and spacer steps for a HandGrip target by enumerating every combination with PlaceBike, scoring each as 100 − points lost (Q17 oval plus stem length, stem angle, and spacer bands), and listing the 5 best-scoring with apply links (Q15–Q18). The saddle fit is unchanged and carried into every build. The handlebar stays fixed.
   - **Slice 4, handlebar choice in the workbench (done and accepted 2026-11-19; commits `d197572`, `6717b39`):** the fixture carries the 15 legacy catalog bars, and `/bike` gets a `bar` select that every link keeps (Q19–Q21). Fitting and stock fitting fit around the submitted bar. Web and fixture only; no assembly change.
   - **Slice 5, handlebars in stock fitting (done and accepted 2026-11-19; commits `599fcf4`, `bf3b2a4`, `fe0a707`):** `CockpitStock` gains an optional bar list, and stock fitting enumerates it at the submitted roll with no bar penalty. `/bike` gets a `stock_bars=selected|all` control, and each stock row names its bar and its apply link sets `bar` (Q22–Q25).
   - **Slice 6, seatpost choice and setback notes (done and accepted 2026-11-19; commits `9ae0e97`, `89f6a7d`):** a `post` select of fixture seatposts replaces the free `post_setback` field (Q27, Q28). `assembly.FitPostSetback` explains the post setback a BRP target needs: with the rails centered, always, and with the rails held at the saddle fitting's offset when that fitting stops at a limit (Q29). Fitting never chooses the post.
   - **Slice 7, bar sweep band (done and accepted 2026-11-19; commit `6ea0295`):** stock fitting penalizes every bar's backsweep outside 16°–20° by 2 points per degree, as a fifth penalty line in both `stock_bars` modes (Q30).
   - **Slice 8, fork choice in stock fitting (done and accepted 2026-11-19; commits `e29da70`, `6ea4d65`):** a `stock_forks=selected|all` control. `all` enumerates the fixture forks, with suspension forks at 15, 20, and 25% sag, and refits the saddle for each fork. Each build gains a fork sag penalty line and a BRP miss penalty line, and the BRP miss line counts in both modes (Q31–Q34).
   - **Slice 9, score-centered scoring words (done and accepted 2026-11-19; commits `bb704db`, `35b818d`, `b830a51`, `7b41ac4`):** rename the stock fitting identifiers to `PenaltyBand`, `PointsPerUnitBelow`/`PointsPerUnitAbove`, `PointsPerR`, `Penalty`, `Penalties`, `BandName`, and `Score()`; the stock table says Score and "best-scoring"; `make boundary` bans the old words in reboot files (Q35–Q40). No scoring values change.
   - **Later:** roll enumeration is not planned (Q24).
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
