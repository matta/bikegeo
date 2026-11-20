# bikegeo Reboot: Design and Plan

A clean-slate reboot built beside the existing application under `cmd/bikegeo-reboot` and `internal/reboot/...`. This file describes the current design; git history holds past discussion.

## Working rules

- Matt makes the architectural decisions. Agents propose, critique, and record his explicit rulings here. Agreement between agents is not approval, and a plan is not permission to change code.
- When a choice is open, sketch the options briefly and recommend one. Don't write a deep spec that assumes an answer. Number open questions `Q#` and never reuse a number.
- Prefer fewer types and clearer physical steps over heavier machinery. Show how a real bike assembly step reads in code.
- Back technical claims with arithmetic, existing behavior, or a measurement. Two agents agreeing does not prove an unmeasured claim.
- **Scoring words:** a build's **score** (0–100) is the weighted average of its 0–100 attribute scores (Q59, superseding Q35–Q37). Don't describe scoring as pricing or cost in prose; call each criterion's value its **attribute score** and the top builds **best-scoring**. `make boundary` bans the words `price`, `pricing`, `cost`, `cheap`, and `ledger` in reboot files (Q40).

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

- **(a) Readable physical code:** build structures from directional pose steps and component builders (seatpost, saddle, fork, spacers, stem, handlebar), not scattered trig. Code clarity replaces web trace introspection (Q48).
- **(b) Focused Web 1.0 interface (parity with legacy `compare`):** Go-rendered HTML/SVG with plain `GET` forms, URL query state, and zero JS (Q48, Q54), serving `/` (multi-frame comparison matrix) and `/bike` (single-bike inspector with schematic SVG; open separate browser tabs to compare builds visually). No Construction Trace UI, no side-by-side or overlayed views, and no legacy secondary pages (Q48, Q52).
- **(c) Validated Go catalogs + JSON agent/debug output:** compiled Go structs validated by `assembly.CheckChartClosure` in `go test` (Q50, Q57), plus thin CLI JSON serialization of Go result structs for agents and debugging (Q49a). No MCP server, no file-based catalog persistence, and no chart-reconstruction solver UI (Q49, Q50).
- **(d) Fitting and `0–100` weighted scoring:** saddle fitting (height-first when rails clamp, Q60) and discrete stock cockpit fitting over the `docs/reboot/stems.yaml` stem catalog (Q58), ranked by a 3-criterion fit gate and tunable `0–100` weighted attribute scores (Q59, Q59a).
- **(e) Multi-frame comparison:** rank all 19 catalog frames (or custom frame/size/fork/sag selections) by best achievable fit and score (Q50, Q52, Q56).

## Confirmed decisions

### Precedence over legacy specs

For reboot code (`cmd/bikegeo-reboot` and `internal/reboot/...`), this document takes precedence over conflicting legacy specifications in `AGENTS.md` (Architecture and Frame Datasets), `docs/WEB.md`, and `internal/ideal/readme.md`:

- `0–100` weighted attribute scoring (`Q59`, `Q59a`) supersedes legacy's unweighted `100 − Σ points lost` model.
- `docs/reboot/stems.yaml` (107 stems and composite `0–100` stem score, `Q58`) supersedes legacy's `mtb`/`gravel` stem catalogs.
- The 3-criterion fit gate (`R ≤ 1.0`, `|ΔY| ≤ 2.0 mm`, `|ΔX| ≤ 10.0 mm`), unified 3-tier ranking (`Q61`), and height-first rail clamping (`Q60`) supersede legacy's 2-criterion gate and Euclidean/rail-scan solvers.
- Direct `encoding/json` CLI output (`Q49a`) replaces legacy's shared `report.Document` Markdown CLI layer.
- Single-row `/` web compare without `+spec`/`-spec` query grammar or legacy secondary routes (`Q52`), defaulting to `redshift_shockstop_pro` (`Q62`) instead of legacy's hardcoded `0 mm` dropper.

### Go core, pure Web 1.0 server-rendered workbench

- Go owns the physical model, assembly, catalogs, comparison, and HTML/SVG rendering. Browser and CLI surfaces call it directly rather than duplicating geometry or scoring.
- Pure Web 1.0 (Q48, Q54): Go renders HTML and SVG directly from placements (`html/template`, embedded assets). Forms submit with plain `GET`, state lives in the URL query, and there is zero JavaScript (no bespoke JS, no vendored HTMX for now, no npm, no build step).
- `/` is the multi-frame comparison matrix and `/bike` is the single-bike inspector with three fixed server-rendered diagram views (`view=bike|seat|stem`) instead of free pan/zoom. Open separate browser tabs to compare builds visually.
- Thin CLI JSON output (`encoding/json` directly over internal Go result structs) serves as the agent-friendly and debug interface, with no duplicated Markdown CLI renderer or `report.Document` abstraction (Q49a).

### Pose-based assembly

- `Pose` is an immutable value. `Advance`, `Translate`, `Turn`, and `Face` return new poses, so branching from a shared attachment is safe. `Face(worldHeadingDeg)` sets an absolute world heading. It is used for parts leveled against the world, not against their mounting tube (saddle `RailPitchDeg`, handlebar `BarRollDeg`). (`pose.Trace`, `pose.Step`, `PoseSnapshot`, and unused `label` parameters on `Pose` methods are removed in Slice 11 per Q48.)
- Component functions (`PlaceStem`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceHandlebar`, ...) return placements whose landmarks are `Pose`s. Each landmark's local axes have a documented physical meaning. For example, `BRP` and `SitBones` carry the rail frame, not a saddle-surface normal.
- Placement (moving along a construction path) is separate from outline projection. The renderer draws outlines in landmark frames without recomputing angles.
- Trig lives in `pose` (rule in AGENTS.md).

### `assembly` builds and measures; `web` only renders

Test: if fitting or comparison would need the code without HTTP, it belongs in `assembly` (or `compare`).

- Composition (frameset → seatpost → saddle, stem → handlebar) happens in one `assembly` entry point (`PlaceBike`).
- Any scalar derived from placed poses is a method on a placement type. Callers, including tests, call it rather than restating the expression.
- Outlines and readouts use the setup that was actually placed, never catalog defaults or a second computation.
- Catalog data is application input in `internal/reboot/catalog` (replacing `internal/reboot/web/fixture.go` in Slice 12). `assembly` never imports it.

### Frameset leveling: one recipe, two passes

`placeFramesetAtPitch(frameRoot, frame, lowerCupMm, fork)` walks from `BB`: rear axle, then head tube top (reach, stack), then down the steerer by HTL, lower cup, and fork A2C, then rake at `+90°` to the front axle; separately, up the seat axis to `SeatTubeTop`.

1. Run it at 0° and measure `PitchDeg = -RearAxle.AngleTo(FrontAxle)`. The same run on `FrameChart` values gives the raw chart-closure residuals, which the startup closure check holds to the chart's rounding budget (Q47).
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
- **Q6, solver policy for saddle fitting:** a direct solve is acceptable as long as forward assembly (PlaceBike) remains the final check of every fitted build. (Clamped rail fallback updated by Q60.)
- **Q9, rail range:** `PlaceSaddle` rejects rail offsets outside `±(RailLengthMm − ClampLengthMm)/2`, the same range fitting respects. This lands as its own commit after saddle fitting.
- **Q10, fixture rail and clamp lengths:** the Fenrir fixture's 60 mm rail length and 30 mm clamp length are measured and authoritative. A fitting that is limited by the ±15 mm rail range is a real finding, not a fixture artifact.
- **Q11, cockpit target:** the front-cockpit fit target is a **HandGrip** point in world X/Y (BB origin), like the BRP target. With the frameset fixed, the saddle and cockpit fits are independent.
- **Q12, solver policy for front-cockpit fitting:** M2 Slice 2 fits `SpacersMm` and stem `LengthMm` as continuous values with a direct 2×2 solve at the submitted stem angle and bar roll, reusing Slice 1's box logic. Discrete stock parts and preference penalties are M2 Slice 3. **Forward pose-based assembly stays the source of truth for geometry:** a direct solve reads each fitted input's direction from the probe placement's poses and never restates the assembly formulas, and PlaceBike checks every fitted build.
- **Q13, cockpit limits:** `StemSetup` carries `MaxSpacersMm` (the steerer allowance; `PlaceStem` rejects more, like Q9) and `MinLengthMm`/`MaxLengthMm` (the stem range fitting may choose; `PlaceStem` ignores it).
- **Q14, one entry point:** `ApplyFitting` takes one `FitTarget` with optional BRP and HandGrip landmarks and returns one fitted build, so the workbench has one apply link.
- **Q15, saddle first:** saddle fit is the primary fit criterion and must be right. Cockpit fit is secondary. Stock cockpit fitting never changes the saddle fit; when a BRP target is set, every stock cockpit build carries the saddle fitting's values.
- **Q16, solver policy for discrete part choice:** enumerate every stock combination, place each one forward with PlaceBike, score it, and rank. No direct solve and no rounding of a continuous answer. (Stem catalog and scoring model superseded by Q58, Q59, Q59a.)
- **Q17, HandGrip miss penalty:** port the legacy oval ("Area of Reasonable Fit"): semi-axes 10 mm and 20 mm, major axis tilted 30° clockwise from vertical (world heading 60°), centered on the HandGrip target, free inside, 22 points per unit of normalized radius R beyond R = 1. (Scoring model superseded by Q59 and Q59a: 0–100 weighted scores with a light inside-oval slope.)
- **Q18, stock fitting result:** a ranked list of the 5 best-scoring stock cockpit builds, each with its penalty lines and its own apply link. Slice 2's continuous fitting stays as the explanation (for example, "needs a 134 mm stem"). (Scoring model superseded by Q59, Q59a.)

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
- **Q30, bar sweep band:** bars are scored by backsweep only. 16°–20° is free (every bar in that range scores the same), and each degree outside it is penalized 2 points. The line appears in every stock fitting result, including `stock_bars=selected`. This supersedes Q23's "bars are free". (Rate superseded by Q59/Q59a 0–100 weighted bar sweep score.)
- **Q31, fork set:** fork choice enumerates the fixture forks at their catalog A2C and default lower cup. Rigid forks run at 0% sag. Suspension forks run at 15, 20, and 25% sag. The CAD Baseline Fork is excluded.
- **Q32, fork and saddle penalties:** forks are free except for sag. A fork sag band is free at exactly 20% and penalizes 2 points per percent away from it, so 15% and 25% each lose 10 points. It applies to suspension forks only. A BRP miss line penalizes the saddle's remaining miss: free up to 1 mm, then 2 points per mm (illustrative). (Rates superseded by Q59/Q59a 0–100 weighted fork sag and BRP scores.)
- **Q33, fork control:** `stock_forks=selected|all`, mirroring Q22. `selected` (the default) keeps the submitted fork. With `all`, each row names its fork and sag, and its Apply link sets them.
- **Q34, BRP miss always counts:** the BRP miss line scores the saddle fit itself, not the fork choice. It penalizes any failure to reach the BRP target, whatever the cause (frame, seatpost setback, fork, saddle). So it appears in every stock fitting result that has a BRP target, including `stock_forks=selected`. There, every row has the same miss: with the 0 mm post and the fit report BRP, every score drops by 77.2 points (78.7 → 1.5) and the ranking doesn't change. (Rate superseded by Q59/Q59a 0–100 weighted BRP scores.)

### Scoring words in code (Q35–Q40, ruled 2026-11-19)

- **Q35, band type:** the band type is `PenaltyBand{FreeMin, FreeMax, PointsPerUnitBelow, PointsPerUnitAbove}`, and the HandGrip oval's rate is `FitOval.PointsPerR`. The fields are rates (points per unit), so **penalty** keeps meaning an amount of points. (Superseded by Q59 0–100 weighted attribute scores.)
- **Q36, penalty lines:** each scored line of a stock build is a `Penalty{Band, PointsLost}`, held in `StockCandidate.Penalties`. The band-name type is `BandName`; its constants keep their `Band…` names and string values. Prose says "penalty lines". (Superseded by Q59 0–100 weighted attribute scores.)
- **Q37, score:** a build's 100 − points lost is its **score**: `StockCandidate.Score()`, a **Score** column in the stock table, `data-stock-cell="score"`, and the note says "score = 100 − points lost". The best builds are "best-scoring". (Superseded by Q59 0–100 weighted attribute scores.)
- **Q38, free stays:** `FreeMin`/`FreeMax`, the "Free: …" preference note, and "free inside its range" keep the word *free*, meaning no penalty. (Superseded by Q59 0–100 weighted attribute scores.)
- **Q39, scope:** reboot code, tests, templates, and all of this file. Rulings are reworded without changing their substance. AGENTS.md's legacy Architecture bullet on scores stays as it is, because it documents legacy code that still uses those words.
- **Q40, enforcement:** `make boundary` bans the words `price`, `pricing`, `cost`, `cheap`, and `ledger` in the file names and contents of `cmd/bikegeo-reboot` and `internal/reboot`, like `chassis`. It matches in any case and inside longer words, so `prices`, `priced`, `PricedBand`, `CostPerR`, and `cheapest` are caught too. The list names whole words, not stems, so it reads plainly (revised 2026-11-19).

### Chart closure check (Q41–Q47, ruled 2026-11-19)

- **Q41, next slice:** M3 starts with a chart closure check for the Fenrir fixture chart (Slice 10).
- **Q42, M2 closed:** Milestone 2 is done and accepted. Roll enumeration stays not planned (Q24).
- **Q43, declared precision:** each `FrameChart` declares the rounding half-width of every published column the closure checks read, as a `ChartPrecision`, labelled in the fixture like other values. There is no inference code. The Fenrir uses ±0.5 mm for lengths and ±0.125° for HTA (a quarter-degree grid), marked illustrative.
- **Q44, fork and cup are hypotheses:** the CAD fork's A2C and rake and the CAD lower cup get no rounding terms. Only frame chart columns do: reach, stack, HTA, HTL, BB drop, chainstay, and wheelbase.
- **Q45, method:** port the legacy budget. Move each column by ± its half-width, rerun the unpitched frameset recipe forward (no closure equations restated), and take the larger change in the residual as that column's term. A check's rounding budget is the sum of its terms, and it passes when |residual| ≤ budget. There are no allowances or tiers. (Extended by Q57 for unpublished wheelbases and documented allowances.)
- **Q46:** superseded by Q47 (it put a budget table on `/bike`).
- **Q47, a data check, not a readout:** the closure budget validates fixture data and is not shown to users. `web.NewHandler` runs `assembly.CheckChartClosure` on the fixture chart once and panics if a check fails. The `/bike` "0° Closure Residuals" readout and `ChartBaseline.Residuals` are removed.

### Focus redirect: web compare parity, Go catalogs, and 0–100 weighted scoring (Q48–Q70, ruled 2026-11-20)

- **Q48, drop trace introspection, stem preview, and JS:** remove `pose.Trace`, `pose.Step`, `PoseSnapshot`, and unused `label` parameters from `pose` and `assembly`. Short inline comments must replace dropped `Pose` `label` strings wherever a turn or advance is not self-evident, preserving Requirement (a) code readability. Delete the `/` stem preview (`stem.html`), `trace_steps.html`, the Construction Trace table, step inspector, SVG trace/local-axis overlays, `workbench.js`, `workbench_test.js`, and `make test-js`. Keep `/bike` as a pure Web 1.0 page with form controls, numerical readouts, fitting/stock-fitting tables, and a clean schematic SVG (`view=bike|seat|stem`).
- **Q49, drop MCP:** remove MCP server work permanently from the reboot scope.
- **Q49a, agent/debug CLI via JSON:** do not build a duplicated Markdown CLI table renderer, `report.Document` abstraction, or `+spec`/`-spec` CLI grammar. Expose thin CLI JSON output (`encoding/json` directly over internal Go result structs) as an agent-friendly and debug surface, not a versioned external schema with bespoke JSON adapters.
- **Q50, Go-struct catalogs, cut persistence and chart-solver UI:** cut file-based catalog persistence (old Q4) and the chart-reconstruction solver UI. Port the 19 legacy frames (119 sizes), 22 forks, 15 handlebars, 4 seatposts, and SQlab 610 saddle as Go structs in `internal/reboot/catalog/`, validated by `assembly.CheckChartClosure` in `go test`. Keep the legacy `bikegeo fit` CLI for occasional new-frame ingestion until cutover.
- **Q51, multi-frame scoring scope:** include frame-handling attributes (`RC / WB` ratio, leveled `BB drop`, `Trail`) and rear-cockpit attributes (`rail offset`, `seatpost extension`) alongside fit, stem, spacers, handlebar sweep, and fork sag, plus a fit gate ranking fitting builds strictly above non-fitting builds in both `ApplyStockFitting` and `compare`.
- **Q52, `/` compare feature set:**
  - Default `/` view is **"All 19 frames, best configuration each"**, with web form controls to pick custom `(frame, size, fork, sag)` setups, adjust `top-per-frame` / `top`, and edit global seatpost, handlebar, and target settings.
  - Keep a single saddle (SQlab 610) for now.
  - Sliding dropouts (7 of 19 frames) evaluate at **max chainstay** in `compare` (flagged `*`) and at published chainstay in `assembly.CheckChartClosure`. (See Q67: both `/` and `/bike` use max chainstay so compare links reproduce the ranked build exactly.)
  - Always **one row per bike** (no auto-transpose or bike-as-column layout).
  - Use web form controls only (drop CLI `+spec`/`-spec` query grammar from the web URL surface).
  - Cut legacy secondary web pages (`/frames`, `/saddles`, `/saddle-swap`, `/handlebars`, `/validate`, `/help`) and the 5-stage arithmetic breakdown.
  - Each row links to `/bike?...` so visual comparison uses separate browser tabs rather than side-by-side or overlayed SVG views.
- **Q53, default fit targets:** Matt confirmed (2026-11-20; see `docs/RIDER_FIT.md` and Q64) that the idmatch report's `(481, 726)` handlebar coordinate represents the physical **HandGrip** position. Default `/` and `/bike` to `BRP (-219, 713.1)` and `HandGrip (481, 726)`, editable in the form and URL (with a clear-targets option on `/bike` for manual mode), and carried in every compare row's `/bike` link.
- **Q54, pure Web 1.0 (no HTMX now):** zero JavaScript and no vendored HTMX for now. Revisit vendoring single-file HTMX only if full-page `GET` forms feel clunky in daily use; the full-page `GET` route structure leaves that option open without architectural churn.
- **Q55:** superseded by Q59 and Q59a (the multi-frame attribute set and ideal ranges moved into Q59a's 0–100 weighted table).
- **Q56, handlebar and fork search on `/`:** default `/` to the selected handlebar (`persuader20_800`) with a `Selected handlebar only` / `All 15 handlebars` toggle. Each frame expands its declared sizes × supported forks (rigid at `0%` sag; suspension at `15`, `20`, and `25%` sag).
- **Q57, 19-frame chart closure:** skip the wheelbase check in `assembly.CheckChartClosure` when `PublishedWheelbaseMm == 0` (3 legacy frames omit wheelbase), and allow an optional documented A2C closure allowance with a mandatory `Reason` (verified strictly necessary by test) for `sklar-pbj` and `ritchey-ultra` (`0.5 mm` each), never nudging published chart values (extends Q45). (Extended by Q65 to cover Esker Hayduke S4's wheelbase residual via documented `AxleHeightMm` / `WheelbaseMm` allowances.)
- **Q58, stem catalog and composite `0–100` stem score:** populate candidate stems from `docs/reboot/stems.yaml` converted to Go structs in `internal/reboot/catalog/stems.go` (107 concrete `(length_mm, angle_deg)` stems across `40–120 mm`; `"+/-X"` expands to `−X°` and `+X°`, deduplicating overlapping signed angles at the same length by keeping the higher-availability tier). Each stem receives a single composite `0–100` weighted score from 4 sub-attributes (weights summing to `100`):
  - **Length availability (weight 35):** `abundant = 100`, `common = 80`, `limited = 35`, `sparse = 0`.
  - **Angle availability (weight 30):** `dominant = 100`, `common = 80`, `niche = 0`.
  - **Length breathing room (weight 20):** `100 × clamp(min(L − 40, 100 − L) / 20, 0, 1)` against mainstream lengths `40–100 mm` (`60–80 mm = 100`, `50/90 = 50`, `40/100+ = 0`).
  - **Angle breathing room (weight 15):** `100 × clamp(min(α − minMs(L), maxMs(L) − α) / 10, 0, 1)` against the mainstream (`dominant`/`common`) angle span `[minMs(L), maxMs(L)]` at length `L`.
  - Confirmed intentional (2026-11-20) that at `40–50 mm` the mainstream angle span is `±6°`, so `±6°` stems score `0` on the 15%-weighted angle-room sub-score (`40 mm ±6° = 59.0`, `50 mm ±6° = 69.0`) and `0°` scores `60` (`40 mm 0° = 74.0`, `50 mm 0° = 84.0`).
- **Q59, `0–100` weighted attribute scoring (supersedes Q35–Q38, Q55):** every attribute scores `0–100` on its own natural scale, and a build's overall score (`0–100`) is `Σ(weight × score) / Σ(weight)`:
  - The **3 fit-gate attributes** (`HandGrip`, `Saddle BRP Y`, `Saddle BRP X`) use **two-segment piecewise linear curves** (`100` ideal zone → gate-edge score at the fit gate → `0` worst anchor, clamped to `[0, 100]`).
  - The **8 non-fit range attributes** use a **single linear ramp** (`100` inside the ideal range → `0` at the worst anchor, clamped to `[0, 100]`).
  - **Stem** uses the Q58 composite `0–100` score.
  - Rigid forks score `100` on `Fork sag` so `Σ weight` stays constant across builds and a rigid fork matches `20%` sag.
- **Q59a, 12 scoring attributes and fit-gate thresholds (total weight 36):** both `compare` and `ApplyStockFitting` rank candidates using Q61's unified 3-tier order (`FitsAll` → `FitsGrip` → `R > 1.0`).

| Attribute | Fit Gate | Weight | `100` (Ideal) | Gate-edge score | `0` (Worst anchor) |
| --- | --- | ---: | --- | --- | --- |
| HandGrip (`R`) | `R ≤ 1.0` | 6 | `R = 0` | `90` at `R = 1.0` *(light inside-oval slope)* | `0` at `R ≥ 2.0` *(steep outside slope)* |
| Saddle BRP Y (height) | `|ΔY| ≤ 2.0 mm` | 6 | `|ΔY| ≤ 0.5 mm` | `50` at `|ΔY| = 2.0 mm` | `0` at `|ΔY| ≥ 4.0 mm` |
| Saddle BRP X (setback) | `|ΔX| ≤ 10.0 mm` | 4 | `|ΔX| ≤ 1.0 mm` | `50` at `|ΔX| = 10.0 mm` | `0` at `|ΔX| ≥ 20.0 mm` |
| Stem (Q58 composite) | No | 5 | `70 mm 0°/±6°/±7°, 80 mm ±6°/±7°` (`100`) | — | `120 mm −20° / ±30°` (`0`) |
| Handlebar sweep | No | 3 | `16°–20°` | — | `≤ 10°` or `≥ 26°` |
| Fork sag | No | 3 | `20%` (or rigid = `100`) | — | `≤ 10%` or `≥ 30%` |
| Spacers | No | 2 | `0–20 mm` | — | `≥ 40 mm` |
| Saddle rail offset | No | 2 | `−5 to +5 mm` | — | `±15 mm` |
| RC / WB ratio | No | 2 | `37–39%` | — | `≤ 34%` or `≥ 42%` |
| Seatpost extension | No | 1 | `250–310 mm` | — | `≤ 150 mm` or `≥ 410 mm` |
| Leveled BB drop | No | 1 | `65–75 mm` | — | `≤ 45 mm` or `≥ 95 mm` |
| Trail | No | 1 | `98–106 mm` | — | `≤ 78 mm` or `≥ 126 mm` |

- **Q60, height-first rail clamping (and deferred radial-height follow-up):** when BRP is reachable, `fitSaddle` hits exact `(X, Y)`. When rails hit `±railLimit`, `fitSaddle` solves `Seatpost.ExtensionMm` to preserve exact `target.Y` (clamped to `[MinExtensionMm, MaxExtensionMm]`) so rail-limit shortfall lands purely on `ΔX`, and `|ΔY| > 0` occurs only when extension limits bind. (Extended to all box edges, including extension-bound pitched rails, by Q68.) **Recorded follow-up (deferred):** preserving `target.Y` approximates the true biomechanical invariant — matching radial BB-to-saddle distance (`|BRP − BB|`) within ~`1 mm` while allowing a small `X` setback range. Revisit after `compare` parity if radial-distance-constrained saddle fitting is needed.
- **Q61, unified 3-tier ranking comparator and page tier legend:** both `ApplyStockFitting` and `compare` use one shared 3-tier total order so a frame's champion selection and cross-frame ranking never disagree:
  1. **Tier 1 (`FitsAll`):** passes all active fit gates (`R ≤ 1.0`, `|ΔY| ≤ 2.0 mm`, `|ΔX| ≤ 10.0 mm`) → sort by descending `Score` (tie-break smaller `GripR`, smaller `|ΔY|`, smaller `|ΔX|`).
  2. **Tier 2 (`FitsGrip`):** passes `R ≤ 1.0` but misses BRP → sort by descending `Score` (tie-break smaller `|ΔY|`, smaller `|ΔX|`, smaller `GripR`).
  3. **Tier 3 (`R > 1.0`):** full sort by ascending `GripR` (closest to the oval), tie-break descending `Score`, smaller `|ΔY|`, smaller `|ΔX|`.
  Both `/` compare and `/bike`'s stock table must clearly indicate each row's fit tier in the table and include a readable definition of all three tiers on the page. (Exact tie-breaks and row identity defined in Q69.)
- **Q62, default seatpost:** `/` compare and `/bike` default to `redshift_shockstop_pro` (`12 mm` setback when sagged; see Q28 and `docs/RIDER_FIT.md`, noting extension limits and clamp length remain illustrative until measured). Users can switch to `setback_0`, `setback_15`, or `setback_25` via the form and URL.
- **Q63, cleared or partial fit targets on `/bike`:**
  - Both targets cleared: manual inspection only (no fitting, no score table).
  - HandGrip only (no BRP target): omit `Saddle BRP Y` (weight `6`) and `Saddle BRP X` (weight `4`) from both the fit gate and the weighted average (denominator becomes `26` over the remaining 10 attributes).
  - BRP only (no HandGrip target): continuous saddle fitting runs; stock cockpit fitting and scoring do not run.
  (Durable URL encoding for cleared targets defined in Q70.)
- **Q64, confirmed HandGrip reference and Q58 40–50 mm behavior:** Matt confirmed (2026-11-20) that the idmatch report's `(481, 726)` handlebar coordinate represents the physical **HandGrip** location (recorded in `Q53` and `docs/RIDER_FIT.md`), and confirmed `Q58`'s `40–50 mm` mainstream `±6°` angle-room scoring behavior.
- **Q65, forward-recipe closure allowances including Hayduke S4:** keep reboot's forward-only unpitched `assembly.CheckChartClosure` recipe (Q45; no separate chart-only wheelbase closure equation restated). `FrameChart` carries an optional documented `ClosureAllowance` with `AxleHeightMm`, `WheelbaseMm`, and mandatory `Reason`, with a test verifying every declared allowance is strictly necessary. Across all 19 legacy frames (119 sizes), 115 pass strict rounding budgets; 4 require documented allowances: `sklar-pbj` L (`AxleHeight`: `−3.06` vs `±2.68 mm`), `ritchey-ultra` M/XL (`AxleHeight`: `+2.64` vs `±2.63 mm` and `+2.68` vs `±2.65 mm`), and `esker-hayduke` S4 (`Wheelbase`: `+5.42` vs `±4.31 mm`, allowance `1.2 mm`). Use `esker-hayduke` S4 as an explicit acceptance test in Slice 12 before enabling startup closure checks across all 19 frames.
- **Q66, physical head tube / steerer headset derivation, per-frame fork variants, and seatpost insertion limits:**
  - **Head tube & headset derivation:** each frame specifies its physical head tube bore (`UpperBore` and `LowerBore` — diameter and pressed/integrated standard, inferred from legacy `HeadsetCupType` when porting: `EC34 → 34 mm pressed`, `EC44 → 44 mm pressed`, `ZS56 → 56 mm pressed`, `IS52 → 52 mm integrated`), and each fork specifies its steerer (`Straight 1-1/8"` vs `Tapered 1-1/8"–1.5"`). The installed `LowerCupSetup` (and fork-to-frame compatibility, for example rejecting a tapered steerer in a `34 mm` head tube) is derived from mating `(headTube, steerer)` (with `CadLowerCup` kept separate on `FrameChart`, and an explicit reducer-cup note where a frame's straight-steerer build uses an `EC44` external cup such as `sklar-pbj` / `tumbleweed-sunliner`). Each frame's supported fork list specifies its exact `ForkSpec` variant (`StaticA2cMm`, `TravelMm`, `RakeMm`, `Steerer`), so shared models carry frame-specific variants (for example `SID 100` at `44 mm` rake on Fenrir vs `51 mm` rake on Cutthroat).
  - **Seatpost extension limits:** `MaxExtensionMm` is a property of the seatpost (`PostLengthMm − MinInsertionMm`, e.g. `400 − 100 = 300 mm` for illustrative posts). `MinExtensionMm` is `max(0, PostLengthMm − FrameInsertionDepthMm)` when a frame size records a measured insertion depth (Fenrir L: `290 mm` → `110 mm`), and `0 mm` (exposed extension `≥ 0`) when insertion depth is unmeasured.
- **Q67, sliding dropouts fixed at max chainstay on both `/` and `/bike`:** for the 7 frames with sliding dropouts (`ChainstayRangeMm`), both `/` compare and `/bike` evaluate at **max chainstay** (fixed at max for now; no `chainstay` input on `/bike`), while `assembly.CheckChartClosure` validates `FrameChart`'s published chainstay. Following any compare row's `/bike` link must reproduce the ranked build's exact geometry, fitted parts, score, and fit tier (verified by test in Slice 14).
- **Q68, lexicographic height-first saddle fitting across the full box:** when BRP is unreachable within `[MinExtensionMm, MaxExtensionMm] × [−railLimit, +railLimit]`, `fitSaddle` minimizes `(|ΔY|, |ΔX|)` **lexicographically** (`|ΔY|` first, `|ΔX|` second) over all bounded edges and corners, preserving `MaxExtensionMm == 0` as unbounded (`+Inf`). Reachable targets still hit `(0, 0)`; level-rail extension-bound targets solve exact `X` (clamped); pitched-rail extension-bound targets prioritize `|ΔY|` first, then `|ΔX|`.
- **Q69, distinct comparison rows and deterministic tie-breaks:**
  - One comparison row represents one distinct `(Frame, Size, Fork, Sag)`, scored by its single winning cockpit build `(Bar, Stem, Spacers)` under Q61. `top-per-frame` and `top` prune across distinct `(Frame, Size, Fork, Sag)` rows and never return multiple cockpit builds for the same `(Frame, Size, Fork, Sag)`.
  - Deterministic final tie-break when candidates tie on all Q61 keys (`Tier`, `Score`, `GripR`, `|ΔY|`, `|ΔX|`):
    - **Within a row (cockpit choice):** fewer spacers → shorter stem → smaller `|stem angle|` (preferring `+` over `−` on exact tie) → handlebar catalog order.
    - **Across rows (`compare`):** `FrameID` ascending → declared frame size order → frame fork list order → smaller `|sag − 20%|` (then lower sag).
- **Q70, durable URL encoding for cleared targets:** because missing coordinates default to the idmatch targets (Q53), URL parsing distinguishes 4 states without JS:
  - Both coordinates non-empty → active target (takes precedence even if `=off` is present).
  - Exactly one coordinate non-empty → `400` validation error.
  - Both coordinate keys present and empty (`brp_x=&brp_y=` from clearing plain-`GET` form boxes) **or** `brp=off` → deliberately cleared; canonical URLs and Apply/view links emit `brp=off` (and `grip=off` for HandGrip) so cleared state survives Apply, view switches, and reload.
  - Neither coordinate key nor `=off` present → default idmatch target (Q53).
  - On `/` compare: HandGrip is required to rank cockpits, so clearing HandGrip returns `400` with a Reset Defaults link; `brp=off` is allowed on `/` just like `/bike` (Q63, scoring on the remaining 10 attributes with total weight `26`).

## Run and inspect

- `make serve-reboot` serves the pure Web 1.0 app on `:8081`. `/` is the multi-frame comparison matrix (once Slice 14 lands; redirects to `/bike` after Slice 11), and `/bike` is the single-bike inspector (`view=bike|seat|stem`, `fork=cad`). State lives in the URL query with zero JS.
- Catalog data lives in `internal/reboot/web/fixture.go` until Slice 12 moves it into `internal/reboot/catalog/` (19 legacy frames, 22 forks, 107 stems from `docs/reboot/stems.yaml`, 15 handlebars, 4 seatposts, SQlab 610 saddle). Values remain labelled measured, catalog, or illustrative.

## Layout and boundaries

```text
cmd/bikegeo-reboot/       entrypoint; `serve` on port 8081 (and JSON CLI in Slice 13c)
internal/reboot/pose/     poses, units, primitives
internal/reboot/assembly/ component setups, placements, fitting, scoring, chart closure
internal/reboot/catalog/  (Slice 12; stems.go in Slice 13b) 19 frames, 22 forks, 107 stems, 15 bars, 4 seatposts, SQlab 610, default targets/weights
internal/reboot/compare/  (Slice 13c) multi-frame expansion, fitting, ranking, pruning
internal/reboot/web/      pure Web 1.0 handlers, HTML/SVG views, embedded templates (no JS)
```

- Dependencies flow `cmd → {web, compare, catalog}`, `web → {compare, catalog, assembly, pose}`, `compare → {assembly, pose}`, `catalog → {assembly, pose}`, `assembly → pose`. Neither `assembly` nor `compare` imports `catalog`; callers pass catalog slices as explicit arguments so both packages stay unit-testable with small fixtures.
- Add packages only when a slice needs them. Keep each component's setup and placement types with its assembly code; don't rebuild a catch-all `model` package.
- `make boundary` enforces the import isolation and the banned-term rule described in AGENTS.md.
- To reuse legacy code, read, audit, and port only what's needed. Never wrap an old implementation under a new name. Small duplicated primitives are acceptable; a shared package created just to avoid duplication is not. A parity test must explain differences, not assume the old result is right.
- Label catalog and fixture values as measured, catalog, or illustrative. Never present an invented outline as manufacturer truth.

## Milestones

1. **One-bike visual workbench (a, b): done.** Accepted by Matt; tests cover trace on/off equivalence, independent geometric fixtures, and HTML/SVG step-ID correspondence in every view. Solver-free: the seatpost control edits `ExtensionMm` directly.
2. **One-bike fitting (d): done and accepted 2026-11-19 (Q42).** Explicit fit targets, hardware limits, and attribute scoring. Produce candidate builds, check each with forward assembly, and open them in the workbench.
   - **Slice 1, saddle fitting (done 2026-11-19):** fit `SeatpostSetup.ExtensionMm` and `SaddleSetup.RailOffsetMm` to a BRP target with a direct 2×2 solve (Q6). Rail pitch and post setback stay fixed inputs. Limits are the post's min/max extension and the rail clamp range from `RailLengthMm`. The result reports the fitted setups, the BRP actually reached, the miss in mm, and any limit hit, and keeps *reached*, *clamped at a limit*, and *invalid input* distinct. PlaceBike on the fitted setup must land BRP on the target (to 1e-9 mm when no limit binds). `pose` gains one operation that decomposes a vector along two headings. `/bike` gets a BRP target field and a plain-`GET` link to the fitted build. Tests: fit → place round trip, out-of-range target clamps to the right limit, trace on/off equivalence, the fit report's BRP (−219, 713.1). Out of scope: post setback choice, the front cockpit, preference scores, other frames.
   - **Slice 2, front-cockpit fitting (done 2026-11-19):** fit `StemSetup.SpacersMm` and `LengthMm` to a HandGrip target with a continuous direct 2×2 solve (Q11–Q14). Stem angle and handlebar stay fixed inputs. Limits are `MaxSpacersMm` and the stem length range on `StemSetup`. The same `ApplyFitting` fits the saddle, the cockpit, or both, and `/bike` gets a HandGrip target and one apply link.
   - **Slice 3, stock cockpit fitting (done and accepted 2026-11-19):** choose stock stem lengths, stem angles, and spacer steps for a HandGrip target by enumerating every combination with PlaceBike, scoring each as 100 − points lost (Q17 oval plus stem length, stem angle, and spacer bands), and listing the 5 best-scoring with apply links (Q15–Q18). The saddle fit is unchanged and carried into every build. The handlebar stays fixed.
   - **Slice 4, handlebar choice in the workbench (done and accepted 2026-11-19; commits `d197572`, `6717b39`):** the fixture carries the 15 legacy catalog bars, and `/bike` gets a `bar` select that every link keeps (Q19–Q21). Fitting and stock fitting fit around the submitted bar. Web and fixture only; no assembly change.
   - **Slice 5, handlebars in stock fitting (done and accepted 2026-11-19; commits `599fcf4`, `bf3b2a4`, `fe0a707`):** `CockpitStock` gains an optional bar list, and stock fitting enumerates it at the submitted roll with no bar penalty. `/bike` gets a `stock_bars=selected|all` control, and each stock row names its bar and its apply link sets `bar` (Q22–Q25).
   - **Slice 6, seatpost choice and setback notes (done and accepted 2026-11-19; commits `9ae0e97`, `89f6a7d`):** a `post` select of fixture seatposts replaces the free `post_setback` field (Q27, Q28). `assembly.FitPostSetback` explains the post setback a BRP target needs: with the rails centered, always, and with the rails held at the saddle fitting's offset when that fitting stops at a limit (Q29). Fitting never chooses the post.
   - **Slice 7, bar sweep band (done and accepted 2026-11-19; commit `6ea0295`):** stock fitting penalizes every bar's backsweep outside 16°–20° by 2 points per degree, as a fifth penalty line in both `stock_bars` modes (Q30).
   - **Slice 8, fork choice in stock fitting (done and accepted 2026-11-19; commits `e29da70`, `6ea4d65`):** a `stock_forks=selected|all` control. `all` enumerates the fixture forks, with suspension forks at 15, 20, and 25% sag, and refits the saddle for each fork. Each build gains a fork sag penalty line and a BRP miss penalty line, and the BRP miss line counts in both modes (Q31–Q34).
   - **Slice 9, score-centered scoring words (done and accepted 2026-11-19; commits `bb704db`, `35b818d`, `b830a51`, `7b41ac4`):** rename the stock fitting identifiers to `PenaltyBand`, `PointsPerUnitBelow`/`PointsPerUnitAbove`, `PointsPerR`, `Penalty`, `Penalties`, `BandName`, and `Score()`; the stock table says Score and "best-scoring"; `make boundary` bans the old words in reboot files (Q35–Q40). No scoring values change.
   - **Later:** roll enumeration is not planned (Q24).
3. **Web `compare` parity and `0–100` weighted scoring (b, c, d, e):**
   - **Slice 10, chart closure check (done and accepted 2026-11-19; commits `8b91fb1`, `2626d1f`):** `FrameChart` declares a `ChartPrecision`, and `assembly.CheckChartClosure` holds the axle height and wheelbase residuals to their rounding budgets (Q43–Q45). `web.NewHandler` runs it once and panics on failure, and the `/bike` residual readout goes away (Q47).
   - **Slice 11, strip trace introspection, stem preview, and JS (planned next; Q48, Q54):** remove `pose.Trace`/`Step`/`PoseSnapshot` and unused `label` args (adding short inline comments where a turn or advance isn't self-evident); delete `stem.html`, `trace_steps.html`, Construction Trace table, step inspector, SVG trace/local-axis overlays, `workbench.js`, `workbench_test.js`, `.node-version`, `test-js` from `Makefile` (including removing `test-js` from `check:`), and the `Set up Node` / `Test JS` steps from `.github/workflows/test.yml`; redirect `/` to `/bike` until Slice 14; update `AGENTS.md`'s **Verification** and **Reboot Tree → Web** bullets.
   - **Slice 12, Go catalogs and 19-frame chart closure (Q50, Q57, Q65, Q66, Q67):** extend `assembly.CheckChartClosure` for unpublished wheelbases (`0`) and documented `ClosureAllowance` (`AxleHeightMm`, `WheelbaseMm`, `Reason` required, verified strictly necessary by test, using `esker-hayduke` S4 as an explicit acceptance test before enabling startup checks across all 119 sizes); add `internal/reboot/catalog/` porting all 19 legacy frames (119 sizes, physical head tube bores with steerer-derived `LowerCupSetup`, per-frame `ForkSpec` variants, per-size seatpost insertion limits per Q66, sliding dropouts fixed at max chainstay on `/` and `/bike` per Q67, and chart precisions), 22 forks, 15 handlebars, 4 seatposts, SQlab 610 saddle, and default idmatch targets (only adding catalog functions called from `main`, with `web.NewHandler` running `assembly.CheckChartClosure` across `catalog.Frames()` at startup so every commit passes `make deadcode`); wire `/bike` to `catalog`, delete `internal/reboot/web/fixture.go`, and update `AGENTS.md`'s **Fixture** bullet.
   - **Slice 13a, height-first lexicographic saddle clamping (Q60, Q68):** update `fitSaddle` to minimize `(|ΔY|, |ΔX|)` lexicographically across all bounded edges and corners (preserving `MaxExtensionMm == 0` as unbounded, and updating clamped-limit tests). No uncalled placement metrics are added here so `make deadcode` stays clean.
   - **Slice 13b, `stems.yaml` catalog, `0–100` weighted scoring, and 3-tier fit gate (Q58, Q59, Q59a, Q61, Q63, Q69):** add `TrailMm`, `RearCenterMm`, and `RcWbRatioPct` alongside `internal/reboot/catalog/stems.go` (107 stems from `docs/reboot/stems.yaml` + composite `0–100` stem score) and default attribute weights; replace `PenaltyBand` with the 12-attribute `0–100` weighted scoring model (two-segment piecewise curves on the 3 fit attributes, single linear ramps on the 8 range attributes) and shared 3-tier comparator + deterministic tie-breaks (Q61, Q69) in `assembly`; hoist frameset + saddle placement outside the inner cockpit loop; support cleared/partial targets (Q63); update `/bike`'s stock table to display `0–100` scores, clear tier badges/column, and a readable on-page definition of the three tiers.
   - **Slice 13c, multi-frame `compare` engine and JSON CLI output (Q49a, Q51, Q52, Q56, Q61, Q69):** add `LeveledReachMm`, `LeveledStackMm`, and `EttMm` alongside `internal/reboot/compare/` (accepting catalog slices as arguments and never importing `catalog`): expand all-frames or custom `(frame, size, fork, sag)` selections using max chainstay on sliding dropouts, fit with selected bar or all 15 bars, keep one winning build per distinct `(Frame, Size, Fork, Sag)`, rank with the shared 3-tier comparator + deterministic tie-breaks, and prune (`top-per-frame`, `top`); add thin CLI subcommands (`bikegeo-reboot compare --json`, plus `bike --json` if included, with `MarshalJSON` on `pose.Pose` if needed) serializing the internal Go result structs directly via `encoding/json`, giving both `compare` and the new metrics an immediate `main` caller so `make deadcode` passes cleanly.
   - **Slice 14, web `/` compare page and multi-frame `/bike` (Q52, Q53, Q54, Q61, Q62, Q63, Q67, Q70):** make `/bike` accept `frame` and `size` from `catalog`, default both `/` and `/bike` to Matt's idmatch targets (Q53) and `redshift_shockstop_pro` (Q62), and support durable `brp=off`/`grip=off` URL state for cleared targets (Q63, Q70, tested through Apply, view changes, and reload); make `/` the pure Web 1.0 comparison page (default "All 19 frames, best config each", `400` with Reset Defaults link if HandGrip is cleared on `/`, custom frame/size/fork/sag form controls, seatpost/handlebar/target controls, one-row-per-bike ranked table with clear 3-tier badges and on-page tier legend per Q61, plus attribute score breakdown, every row linking to `/bike?...` and tested to reproduce exact geometry, fitted parts, score, and fit tier on `/bike` per Q67).

## Deferred follow-ups

- **Radial BB-to-saddle height fitting (Q60 follow-up):** match `|BRP − BB|` within ~`1 mm` with a small allowed `X` range instead of holding `target.Y` when rails clamp.
- **HTMX (Q54):** revisit vendoring single-file HTMX only if full-page `GET` forms feel clunky in daily use.
- **Additional saddles (Q52):** SQlab 610 only for now; port Berthoud Aspin Open and Selle Italia SLR Boost with sit-bone translation only if needed later.

## Cutover (a separate decision, later)

Until cutover, both applications coexist and reboot work doesn't refactor the old one. At cutover:

1. List the old workflows Matt still uses and replace or retire each one.
2. Migrate datasets and provenance; never adjust published values to make the new engine agree.
3. Tag a known-good old revision.
4. Delete the `*_parity_test.go` files and the old packages, move `internal/reboot/*` to `internal/*`, and promote the entrypoint to `cmd/bikegeo`.
5. Run full verification.
