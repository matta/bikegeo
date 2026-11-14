# Architecture, Correctness & Simplification Blueprint (`bikegeo`)

> **Scope of this assessment**: Read-only technical review of `bikegeo` at commit `db9e574` (**18,060 non-test Go lines** across 76 files—of which **2,024 lines** across 20 files live in [pkg/frames](pkg/frames/frames.go) and **16,036 lines** across 56 files are non-frame code—plus **9,193 test lines** across 22 files, 19 compiled frame models, 4 CLI commands, and 10 Web 1.0 HTTP routes). No source code was modified. `go vet ./... && go test ./...` passes on the current tree.
>
> **Revision 2026-11-14**: cross-reviewed against [SIMPLIFY_C.md](SIMPLIFY_C.md) and the code at `db9e574` (no Go source has changed since). C1 now opens with the decision it forces; the comparison baseline is stated as an explicit rule (C6, Part II §1); the worked fit-solve example is recomputed; Part V records what was verified and what was not.

---

## Executive Summary & Synthesis Overview

### The Two Questions `bikegeo` Answers

1. **Chart Auditing (`validate`, `fit`)**: Is a manufacturer's published geometry chart geometrically self-consistent across Stack, Reach, HTA, HTL, BB Drop, Fork A2C, Fork Rake, and Wheelbase—and what hidden CAD assumptions (fork A2C, sag, lower headset cup stack) was it drawn around?
2. **Rider Fit & Handling Comparison (`compare`, `serve`)**: If a rider puts their saddle, seatpost, stem, spacers, handlebar, and suspension fork onto this frame, where do the axles, bottom bracket, sit bones, and hand grips land in 2D space, and can standard hardware achieve the fit cleanly?

Question 1 is implemented with genuine rigor. Question 2 has evolved into a multi-pass pipeline of scalar trigonometry formulas, cross-saddle target translations, and composite scoring heuristics. Almost every bug, duplication, and opacity in the codebase lives in that second pipeline.

### What `bikegeo` Gets Right Today (Keep & Protect)

1. **Worst-case rounding-budget propagation** ([budget.go](pkg/geometry/budget.go)): Perturbing each published chart column by its rounding half-width through `A2cBudget`, `FrontCenterBudget`, and `WheelbaseBudget`—combined with chart immutability and `TestDeclaredAllowancesAreNecessary` in [validator_test.go](pkg/validator/validator_test.go#L421-L526)—beats arbitrary tolerances cleanly.
2. **Compiled pure-Go datasets with one registry-wide suite**: Keeping frames ([pkg/frames](pkg/frames/frames.go)) and components ([pkg/catalog](pkg/catalog/catalog.go)) as Go structs verified by `TestAllFrameData` ([validator_test.go](pkg/validator/validator_test.go#L18-L419)) avoids per-frame test boilerplate and runtime config drift.
3. **Continuous, flat-free curve shapes** ([curve.go](pkg/curve/curve.go)): `Falloff` and `Saturating` guarantee monotonicity with no flat plateaus or step jumps.
4. **Web 1.0 discipline & presentation-neutral reports**: URL query as sole state, `GET` forms, zero JavaScript, zero cookies ([docs/WEB.md](docs/WEB.md)), in-process `goquery` tests, and zero non-stdlib runtime dependencies.
5. **Exact constant-rake rear-axle pitch math**: When fork rake is unchanged, `CalculateDynamicGeometry` matches an independent 4-point sagittal construction to within `0.01 mm`.

### Root Cause & Core Recommendation

Every metric `bikegeo` computes is **2D rigid-body kinematics in one sagittal plane**: roughly a dozen named `(X, Y)` points per build. Today that geometry is fragmented across `BikeGeometry`, `BaselineMetrics`, `DynamicGeometryResult`, `BikeFitAnalysis`, `BRPPoint`, `HBPoint`, `CockpitFitResult`, and `CanonicalComparisonRow` in [bike.go](pkg/model/bike.go) (**1,611** lines), [geometry.go](pkg/geometry/geometry.go) (**1,305** lines), [canonicalizer.go](pkg/canonicalizer/canonicalizer.go) (**767** lines), and [scoring.go](pkg/scoring/scoring.go) (**682** lines).

**The unified blueprint**:
1. **Settle C1, then fix the active correctness & naming bugs**: decide which physical point the `700 mm` fit target names (the answer moves either every cockpit score or the `seven-custom` cockpit by ~70 mm), eliminate the ambiguous `HB`/`HX`/`HY` terminology across code, fit targets, and docs (distinguishing **`StemClamp`** from **`HandGrip`** explicitly), add the omitted upper headset top cover stack (`C2`), level both axles across fork rake swaps (`C3`), and apply physical lower headset cups consistently (`C4`).
2. **Replace the scalar pipeline with one explicit 2D sagittal `BikeScene` (`Layout`)** where every geometry metric, validator residual, posture/CoG ratio, fit solve, and SVG drawing reads directly off the same named points.
3. **Normalize biomechanics once at the physical rider interfaces** (`SitBones`, `StemClamp`, and `HandGrip`, declaring whether a fit target was measured at the stem clamp or grips) instead of translating `BRP` targets across packages on every evaluation.
4. **Expose transparency directly in the Web 1.0 UI**: add an inline `<svg>` side-view workbench (`/bike` + comparison overlays + zoomed cockpit/rail boxes + zero-JS `−10 −5 +5 +10` nudge links) and a 5-stage arithmetic computation ledger, backed by a **closed-form 2×2 linear fit solve** (`StemLengthMm`, `SpacerMm`, `RailOffsetMm`) that can either accompany or replace most of the composite scoring machinery.

### Upfront Summary of Correctness Findings (Ranked by Output Impact)

| ID | Finding | Where | Output / Behavioral Impact |
| --- | --- | --- | --- |
| **C1** | **Open decision worth ~70 mm of reach**: the `700 mm` cockpit target and the tool name different points, so either every cockpit score or the `seven-custom` cockpit (CAD preview 2026-11-11) is off by ~70 mm. Root cause is ambiguous `HB` / `HX` / `HY` reference naming: canonical idmatch & Seven Cycles use **`HB` = stem clamp center**, whereas this session's straight-bar idmatch fit & `bikegeo` measure **`HB` = hand grip center** | [seven_custom.go](pkg/frames/seven_custom.go#L25), [readme.md](pkg/ideal/readme.md#L15), [bike.go](pkg/model/bike.go#L893-L905), [handlebar.go](pkg/geometry/handlebar.go#L22-L134) | `seven-custom` (`70 mm` stem) matches `Fenrir L` (`100 mm` stem) within `~6 mm` on seated cockpit reach (`AETT +24–26 mm` offsets `30 mm` shorter stem), yet **both** score poorly (`23` / `-9` to `-15`) because Persuader 20° backsweep puts `HandGrip` `~53–69 mm` behind `(481, 726)`; overloaded `HB` naming hid the clamp-vs-grip ambiguity |
| **C2** | Missing upper headset cup / top cover stack (~11 mm) between `HeadTubeTop` and stem clamp | [handlebar.go](pkg/geometry/handlebar.go#L106) | Clamp shifts `~3.8 mm` forward and `~10.3 mm` low across all frames; adding `11 mm` puts the `seven-custom` clamp at `(481.3, 677.1)`, within ~1 mm of Seven's stem-centre target `(482, 676)` (a one-frame calibration; treat `11 mm` as a stated assumption) |
| **C3** | Fork swaps pitch only for `ΔA2C` and ignore `ΔRake` | [geometry.go](pkg/geometry/geometry.go#L714-L767), [#L1080-L1158](pkg/geometry/geometry.go#L1080-L1158) | Front axle sits **2.1–3.6 mm off level**; HTA off by **0.11°–0.18°**, Reach by **1.2–2.0 mm**, Stack by **0.8–1.4 mm** |
| **C4** | Physical lower `HeadsetCupType.CupStackMm()` promised by validator but ignored in `compare` | [validator.go](pkg/validator/validator.go#L208-L210), [geometry.go](pkg/geometry/geometry.go#L1064-L1069) | `otso-fenrir` & `salsa-fargo` (`EC44` 12 mm, `GeoHeadsetSpacerMm: 0`) sit **~12 mm** lower and **~0.55°** steeper than physical build |
| **C5** | "Phantom Fenrir" posture & weight distribution (`CalculateBaselinePosture`) | [geometry.go](pkg/geometry/geometry.go#L818-L991), [#L1132-L1137](pkg/geometry/geometry.go#L1132-L1137) | Hardcoded Otso Fenrir L with `SaddleHeight = 780` vs `746`; every bike prints **53.6° back angle** and **SystemCgX = −13.60 mm** |
| **C6** | Closure equation mismatch between `validator/residuals.go` and `geometry/budget.go` | [residuals.go](pkg/validator/residuals.go#L89-L92), [budget.go](pkg/geometry/budget.go#L190-L257) | Sweeping `fork.a2c` in `bikegeo fit` moves `ΔFC`/`ΔWB` table columns while `FrontCenterBudget`/`WheelbaseBudget` remain invariant |
| **C7** | Duplicated 3-pass cockpit selection & scattered fit literals (`746.0`, `219.0`, `292.0`, `700.0`, `3.0`, `11.0`) | [canonicalizer.go](pkg/canonicalizer/canonicalizer.go#L680-L767), [scoring.go](pkg/scoring/scoring.go#L115-L189) | ~75-line copy-pasted selection block mutates a throwaway copy on every `ScoreRow`; cross-saddle translations hardcode reference constants |
| **C8** | Silent fallback defaults mask missing inputs (~20 sites) | [geometry.go](pkg/geometry/geometry.go), [saddle_swap.go](pkg/geometry/saddle_swap.go#L106-L120), [geom2d.go](pkg/geom2d/geom2d.go#L369-L375) | Inconsistent defaults (`chainstay = 430` vs `440`, `wheelbase = 1150`); `if hbDropMm == 0 { hbDropMm = 13.0 }` makes a level `0 mm` bar impossible |
| **C9** | Circular validation of derived wheelbases & float-inferred `ChartPrecision` | [validator.go](pkg/validator/validator.go#L33), [budget.go](pkg/geometry/budget.go#L63-L82), [moots_mountaineer.go](pkg/frames/moots_mountaineer.go#L23) | `Moots Mountaineer`, `Seven 926d`, `Seven 926f` store computed wheelbases validated against the same formula; `seven-custom` gets `±0.5°` HTA budget instead of `±0.05°` |
| **C10** | Typed domain facts re-derived by splitting display strings & fork IDs | [bike.go](pkg/model/bike.go#L1007-L1023), [fork_format.go](pkg/canonicalizer/fork_format.go#L12-L104), [registry.go](pkg/ideal/registry.go#L396-L434) | `ForkTravel()` parses `"_"` tokens clamped to `[60, 200]`; `IsRigidFork` inspects label strings; `LookupAttribute` carries ~55 aliases |
| **C11** | Mathematical, scoring & reliability papercuts (`StemPenalty` slope, `Oval Fits`, `Seatpost Ext`, map order, sag pruning, dead code) | [curve.go](pkg/curve/curve.go#L147-L160), [eval.go](pkg/ideal/eval.go#L51-L86), [prune.go](pkg/compare/prune.go#L14-L100), [bike.go](pkg/model/bike.go#L1455-L1476) | `StemPenalty` derivative halves at `u = 1` (`30 → 15`); `Oval Fits` mixes integer count & float sub-score; non-deterministic `ErrorSummary()`; `PruneRows` picks sag by fit score |

### Verification Status of Each Finding

Part V gives the commands and the limits behind this table.

| ID | Status | Still needs a person or a document |
| --- | --- | --- |
| **C1** | Arithmetic and live output re-run 2026-11-14 | Which point the idmatch `700 mm` names. The fit-session and Seven fit-sheet facts cited in C1 are not recorded in the repository |
| **C2** | Code confirmed; `11 mm` is a one-frame calibration | Stack height of the upper headset actually specified |
| **C3** | Code confirmed (the pitch takes no rake). Table values are [SIMPLIFY_C.md](SIMPLIFY_C.md)'s, from probe programs outside the repository | — |
| **C4** | Code and test confirmed | Whether the Fenrir and Fargo charts are drawn to a bare head tube; which behaviour is intended |
| **C5** | `53.6°` header confirmed on live output; `−13.60 mm` is SIMPLIFY_C's probe value | — |
| **C6** | Code confirmed; both definitions are deliberate per their comments | Which definition governs (recommended rule in C6 and Part II §1) |
| **C7**, **C8** | From code reading; line citations spot-checked only | — |
| **C9** | Frame notes confirmed for all three frames | — |
| **C10** | Code confirmed; the travel mis-parse is latent with today's catalog | — |
| **C11** | `StemPenalty` formula and `compare all` output confirmed; dead-code list is from `grep` | Run `deadcode` before deleting anything |

---

## Part I: Complete Correctness Audit (11 Concrete Findings)

### C1. Ambiguous `HB` / `HX` / `HY` Reference Naming & the ~70 mm Backsweep Discrepancy (Highest Output Impact)

**The decision this finding forces (settle it first).** The idmatch target is `700 mm` from the BRP and `+13 mm` above it, the point `(481, 726)`. Depending on which physical point that names, either the tool or the `seven-custom` cockpit is off by about 70 mm. The frame notes record the order on 2026-10-30 and the CAD preview on 2026-11-11, so readings 2 and 3 should go to the fitter and to Seven now, while the design can still change.

| Reading of the `(481, 726)` target | Required `StemClamp` with the Persuader 20° | Hardware on `seven-custom` | Consequence |
| --- | --- | --- | --- |
| **1. Clamp X, hand height.** Seven's reading: subtract the `50 mm` bar rise, do not compensate sweep | `(481, 676)` | `70 mm -6°` stem, `0 mm` spacers, as drawn | The frame is as intended. `bikegeo` reads ~70 mm short on every frame because it scores `HandGrip`. Hands sit `631 mm` from the BRP, within `6–16 mm` of the current Fenrir |
| **2. Hands.** The marker was at the grips of an effectively straight bar, and the hand position is what must be reproduced | `(553.7, 680.6)` | `-6°`: `140 mm` stem and `-14 mm` of spacers (not buildable). `-17°`: `145 mm` stem and `+14 mm` spacers | `bikegeo` is right. The frame is ~70 mm short at the hands with this bar, and no catalog stem closes the gap |
| **3. Clamp of the fit bike's own bar** (canonical idmatch `HB`), with the hands reproduced through that bar's offset | `(553.7, 680.6) + BarOffset(ReferenceBar)` | Between rows 1 and 2, depending on the test bar's sweep and rise | Neither is right as it stands. `RiderFit.ReferenceBar` must carry the test bar's geometry |

Required-clamp coordinates and stem solves are computed by hand from the Part II §3 equations (inputs listed there), not by code.

What settles it:
- The idmatch report's own definition of the handlebar point, where the marker sat on the fit bike, and which bar that bike carried.
- A tape measure on the current Fenrir, BRP to grip centre. The tool predicts about `637 mm` with a `100 mm` stem and `647 mm` with the `110 mm` baseline stem. A match confirms the tool's hand-point model on a real bike and shows how far today's position is from `700 mm`, which is what choosing reading 2 would mean in practice.

Provenance: the fit-session details and Seven's fit-sheet numbers used below are not recorded anywhere in the repository, and [seven_custom.go](pkg/frames/seven_custom.go#L25) still presents `HX 481mm, HY 727mm` as CAD-preview cockpit coordinates. Record the source of each in the frame `Notes` and in [readme.md](pkg/ideal/readme.md) once confirmed.

- **Where**:
  - [readme.md](pkg/ideal/readme.md#L15) & [#L64](pkg/ideal/readme.md#L64) (formally defines `HB` / `P_HB` as the **stem clamp center**), vs. [#L80-L87](pkg/ideal/readme.md#L80-L87) (defines `HbX/HbY` as the hand position)
  - [bike.go](pkg/model/bike.go#L893-L905) (`HBPoint`: `ClampX/ClampY` holds stem clamp center, while `HbX/HbY`, `SaddleHbMm`, and `SaddleHbDropMm` store **hand grip center**)
  - [loader.go](pkg/ideal/loader.go#L11-L35) (`IdmatchProfile`: `saddle_hb`, `saddle_hb_drop`, `handlebar_clamp_fit`)
  - [handlebar.go](pkg/geometry/handlebar.go#L22-L134) (`GetHandlebarOffset`, `CalculateHBPointWithHandlebar`) & [handlebars.go](pkg/catalog/handlebars.go#L11-L21) (`persuader20_800`)
  - [seven_custom.go](pkg/frames/seven_custom.go#L25) (`Notes` recording AI-derived `HX 481mm, HY 727mm`) & [otso_fenrir_stainless.go](pkg/frames/otso_fenrir_stainless.go#L64-L75) + [profiles.go](pkg/catalog/profiles.go#L14) (`110 mm` Fenrir L baseline)
- **What actually happened (the idmatch convention conflict)**:
  1. **Canonical idmatch vs. what this fit session measured**:
     - Canonical idmatch documentation (and [readme.md](pkg/ideal/readme.md#L15)) defines `HB` as the intersection of the handlebar and the **stem clamp center** (`StemClamp`).
     - In this rider's session, however, the idmatch test bike ran a **flat, straight handlebar** (minimal backsweep), and the fitter placed the tracking marker near the **hand grip position** (`HandGrip`) on the rider's physical bike (`Tumbleweed Persuader 20°`). That session produced `BRP = (-219.0, 713.1)` (`746 mm` height, `219 mm` setback) and **`(X = 481, Y = 726)`** (`700 mm` diagonal distance from BRP, `+3 mm` above Sit Bones / `+13 mm` above BRP).
  2. **How Seven Cycles designed `seven-custom` around `(481, 726)`**:
     - Seven's fit targets specify: `74.6 cm` saddle height, `7.6 cm` saddle-nose setback (`X_nose = -76 mm`), `24.5 cm` saddle length (`X_tail = -76 - 245 = -321 mm`), `3.7 cm` saddle-top-to-stem-center drop (`713 - 37 = 676 mm`), and `80.3 cm` saddle-tail-to-bar reach (`-321 + 803 = +482 mm`).
     - Following canonical idmatch convention, Seven treated `X = 481–482 mm, Y = 726 mm` as the unswept bar top (`726 - 50 mm rise = 676 mm` **stem clamp height**), and designed `seven-custom` (`Reach = 424 mm`, `Stack = 631 mm`, `HTA = 70.0°`) so a **`70 mm -6°` stem** places the **stem clamp center** at **`(481.3, 677.1)`** (after `~11 mm` upper headset cover stack; note that `"HX 481mm, HY 727mm"` in [seven_custom.go](pkg/frames/seven_custom.go#L25) was added during AI coding from the idmatch `(481, 726)` coordinate rather than supplied by Seven).
     - Vertically, Seven subtracted the `50 mm` handlebar rise (`726 → 676 mm`). Horizontally, Seven left the **stem clamp** at `X = 481–482 mm` without adding forward reach for the Persuader bar's `20°` backsweep.
  3. **What `bikegeo` computes—and why it reveals a real frame design discrepancy**:
     - Because `bikegeo` models `(481, 726)` as the **`HandGrip` position** (matching the straight test bar / physical hand marker), evaluating `seven-custom` with its `70 mm -6°` stem and `persuader20_800` adds `GetHandlebarOffset`:
       - `ClampPoint = (485.1, 666.7)` *(missing ~11 mm upper headset cover; see C2)*
       - `persuader20_800` offset (`800 mm` width, `DefaultGripInsetMm = 65 mm`, `EffectiveBendStartMm = 135 mm` → `lz = 200 mm`, `20°` backsweep, `4°` upsweep, `50 mm` bar-end rise): `dx0 = -200 × tan(20°) = -72.8 mm`, `dy0 = 50.0 - 65 × tan(4°) = +45.5 mm`
       - `HbPoint (HandGrip) = (412.3, 712.2)` → `631.3 mm` from BRP (**68.8 mm short** of `700.1 mm`!).
     - Even using the shorter legacy Persuader sweep estimate in [readme.md](pkg/ideal/readme.md#L80-L83) (`-47.9 mm`), grips on the `70 mm` stem build still sit **~48 mm behind `(481, 726)`**.
     - Cross-checking by **seated cockpit length (`ETT` / `AETT` / `BRP Setback`)** rather than `Reach` alone gives the principled comparison against **Otso Fenrir L**:
       - Comparing `Reach` alone (`Seven Custom = 424 mm` vs. `Fenrir L = 428 mm` rigid / `422 mm` @ 120mm) misleads because it ignores `Seven`'s `2.25°` slacker `STA` (`73.0°` vs. `Fenrir L` `75.25°` rigid / `74.7°` @ 120mm).
       - Across full seated geometry in `bikegeo compare`: `Seven Custom` adds **`+19 to +20 mm` of `ETT`** (`617 mm` vs. `597–598 mm`), **`+24 to +26 mm` of `AETT`** (`612 mm` vs. `586–588 mm`), and puts **`BRP Setback` dead-on at `219 mm ✓`** (`rail +13 mm`), whereas `Fenrir L` maxes out the saddle rails at `+15 mm` and still lands **`20–26 mm` too far forward** (`193–199 mm ✗`).
       - Because that `+24–26 mm` gain in `AETT` / setback compensates for most of a shorter stem: **`Seven Custom` with a `70 mm -6°` stem (`BRP → HandGrip = 631.3 mm`, `BRP → Clamp+50 = 704.1 mm`, or `700.5 mm` w/ C2 11 mm top cover) lands within `5.9 mm` of `Fenrir L` with a `100 mm -6°` stem (`BRP → HandGrip = 637.2 mm`, `BRP → Clamp+50 = 710.0 mm`)** and `15.5 mm` of `Fenrir L` with its baseline `110 mm -6°` stem (`646.8 mm` in `DefaultRiderProfile`).
       - What this reveals: **`Seven Custom` is NOT ~5–7 cm shorter than `Fenrir L`**—in seated `AETT` + stem terms (`612 + 70 ≈ 682 mm` vs. `588 + 100 ≈ 688 mm`) the two bikes put the Persuader grips in almost the same place (`631 mm` vs. `637–647 mm`), while `Seven` fixes the `20–26 mm` saddle-setback deficit. **Both** bikes read `53–69 mm` short of `700 mm` in `bikegeo` strictly because the `700 mm` / `(481, 726)` target aligns with `(StemClamp + BarRise)` while `bikegeo` evaluates `HandGrip` after `-72.8 mm` of Persuader 20° backsweep. Whether `631–647 mm` (matching your existing Fenrir setup w/ Persuader sweep) or `700 mm` (`StemClamp` before sweep vs. straight-bar hands) was the physical intent is the exact question the clamp-vs-grip distinction clarifies.

- **Exact decomposition of `bikegeo`'s `(-69.0 mm, -14.8 mm)` delta vs. `(481, 727)` on `seven-custom` (`70 mm -6°` stem)**:

| Component | ΔX (mm) | ΔY (mm) | Physical Cause |
| --- | --- | --- | --- |
| Backsweep of `persuader20_800` (`-lz × tan(20°)`) applied to `HandGrip`, absent at `StemClamp` | -72.8 | — | `GetHandlebarOffset` ([handlebar.go](pkg/geometry/handlebar.go#L40)) |
| Inboard grip rise (`50 - 65×tan(4°) = 45.5 mm`) vs stated `50 mm` bar-end rise | — | -4.5 | `BaseRiseAtBend` ([bike.go](pkg/model/bike.go#L878-L885)) |
| ~11 mm upper headset top cover omitted between `HeadTubeTop` and stem (C2) | +3.8 | -10.3 | `CalculateHBPointWithHandlebar` ([handlebar.go](pkg/geometry/handlebar.go#L106)) |
| **Total delta** | **-69.0** | **-14.8** | `bikegeo` `HandGrip (412.3, 712.2)` vs `(481, 727)` |

- **Observed live output** (`go run ./cmd/bikegeo compare fenrir:L@rigid fenrir:L@120mm@20% seven-custom --score-detail` and `compare all`):
  - `seven-custom` maxes out at the `100 mm -6° +0mm` stem ceiling, lands at `BRP–HB = 660 ✗ 35 sht`, `Oval Fits = 0 ✗` (`-68 (-66)`), and scores **23 (#1)** standalone (and **26 (#11)** in `compare all`, where `PruneRows` picks `110mm @ 30% sag` over its `492mm` CAD rigid fork), while `Fenrir L` itself scores **-9 (#2)** / **-15 (#3)** (`BRP–HB = 637 ✗ 58 sht`, `0 ✗` oval fits).
  - Across `compare all`, **11 of 19** frames peg the `100 mm -6°` ceiling and **11 of 19** score `0 ✗` in the oval, ranking XL frames (`Krampus XL #1`, `Sklar PBJ L #2`, `Esker Hayduke S4 #3`, `Ritchey Ultra XL #4`, `Moots Mountaineer XL #5`) highest because longer-reach frames offset the Persuader's `-72.8 mm` pullback without exceeding `100 mm` stems.
- **The actual code & documentation bug — overloaded `HB` / `HX` / `HY` naming**:
  - Regardless of which point matches your intended posture (`~631–647 mm` Persuader `HandGrip` vs. `700 mm` `StemClamp`), **the codebase's `HB`/`HX`/`HY` terminology actively conceals what is being measured**:
    - [readme.md](pkg/ideal/readme.md#L15) defines `HB` as the **stem clamp center** (canonical idmatch terminology).
    - `HBPoint` ([bike.go](pkg/model/bike.go#L893-L905)), `SaddleHbMm`, `BRP–HB`, `HB Rise`, `PossibleHBs`, and `SelectBestHB*` instead store and score the swept **`HandGrip`** point (`HbX/HbY`), while naming the scored attribute **`handlebar_clamp_fit`** (literally calling a grip check a *clamp* fit!).
- **Required code cleanup (eliminate `HB` / `HX` / `HY` everywhere and require explicit point types on every value)**:
  1. **Every cockpit value in code must explicitly state what physical point it represents—retire ambiguous `HB`, `HX`, `HY` names.** Rename what the user reads now (CLI/Web table headers, footers, attribute display names, and [readme.md](pkg/ideal/readme.md)) and require all new and surviving code (`BikeScene`, `RiderFit`, the fit solve, `/bike`) to name the exact physical reference point on every type, field, constructor, and literal:
     - **`StemClamp`** (`StemClampPoint`, `StemClampX`, `StemClampY`, `BrpToStemClampMm`, `StemClampDropMm`) — intersection of the handlebar and stem clamp center (canonical idmatch `HB` / bike-CAD stem center).
     - **`UnsweptBarTop`** (`UnsweptBarTopPoint`, `UnsweptBarTopX`, `UnsweptBarTopY`) — `StemClamp + (0, Bar.StatedRiseMm)` (the bar top at the stem clamp X coordinate before backsweep, as used in Seven's reading of `(481, 726)`).
     - **`HandGrip`** (`GripPoint`, `GripX`, `GripY`, `BrpToGripMm`, `GripRiseAboveSitBonesMm`, `grip_fit`) — rider's hand contact center after backsweep, upsweep, grip inset, and bar roll (`StemClamp + BarOffset(Bar, GripInset, Roll)`).
     - Do not mass-rename internal identifiers that later steps delete (`HBPoint`, `PossibleHBs`, `SelectBestHB*`, `handlebar_clamp_fit`); they go away with their structs.
  2. **Require an explicit `TargetRef` on every cockpit fit target (no default or implicit interpretation)**: Every cockpit target coordinate or distance/drop pair in `RiderFit` must declare its `TargetRef` explicitly:
     - `TargetHandGrip` *(reading 2: target is the physical hand grip position; `C* = TargetPoint - BarOffset(Bar, Roll)`)*,
     - `TargetUnsweptBarTop` *(reading 1: target is at the stem clamp X with stated bar rise included; `C* = TargetPoint - (0, Bar.StatedRiseMm)`)*,
     - `TargetStemClamp` *(target is the stem clamp center directly; `C* = TargetPoint`)*, or
     - `TargetReferenceBarClamp` *(reading 3: target is the stem clamp of the fit bike's bar `ReferenceBar`; `C* = TargetPoint + BarOffset(ReferenceBar) - BarOffset(Bar, Roll)`)*.
  3. **Display `TargetPoint` (with its `TargetRef`), `StemClamp`, and `HandGrip` side-by-side** in both the SVG blueprint and arithmetic ledger so a 50–73 mm clamp-vs-grip discrepancy cannot hide behind an overloaded label.

---

### C2. Missing Upper Headset Top Cover Stack (`~11 mm`) Between Head Tube Top and Stem

- **Where**: [handlebar.go](pkg/geometry/handlebar.go#L106) (`dSteerer := spacerMm + model.StemThicknessMm/2.0`)
- **What happens**: With `0 mm` spacers and `StemThicknessMm = 40.0`, `CalculateHBPointWithHandlebar` centers the stem only `20.0 mm` along the upward steerer axis from `HeadTubeTop`—treating the stem as sitting directly on bare titanium/steel frame metal with no upper headset cup, bearing cover, or compression ring.
- **Calibration from Seven's fit sheet (`3.7 cm` saddle-to-stem drop, `80.3 cm` saddle-tail-to-bar reach)**: Adding `HeadsetTopStackMm = 11.0 mm` (typical `EC44`/`ZS44` upper bearing + top cover) along `(-cos 70°, sin 70°)` shifts `StemClamp` on `seven-custom` (`70 mm -6°`, `0 mm` spacer) by `(-3.76 mm, +10.34 mm)` from `(485.1, 666.7)` to `(481.3, 677.1)`—matching Seven's stem-center target `(482, 676)` (`X = -321 + 803`, `Y = 713 - 37`) within ~1 mm, and `(481.3, 677.1 + 50)` = `(481.3, 727.1)`.
- **How far to trust `11 mm`**: It is a one-frame calibration. `seven-custom`'s Reach and Stack were themselves read off a 1:5 scale CAD preview ([seven_custom.go](pkg/frames/seven_custom.go#L25)), and the fit-sheet targets are whole millimetres. Agreement within ~1 mm supports a value near `11 mm`, not `11.0`. Treat it as a stated, editable assumption and replace it with the stack height of the headset actually specified. It is also a different quantity from `GeoHeadsetSpacerMm: 11` on the same frame, which is the lower cup.
- **Impact**: Shifts every frame's cockpit by ~4 mm X and ~10 mm Y (critical since `saddle_hb_drop` ideal window `-2 to +8` is only `±5 mm` wide). Model `HeadsetTopStackMm` explicitly (per headset standard or frame, just like `CupStackMm()`) and state the assumption in the comparison header.

---

### C3. Fork Swaps Ignore Rake Changes (`CalculateDynamicGeometry` Leaves Axles Un-Leveled)

- **Where**: [geometry.go](pkg/geometry/geometry.go#L714-L767) (`CalculateDynamicGeometry`) and [geometry.go](pkg/geometry/geometry.go#L1080-L1158) (`AnalyzeBikeFitWithSaddle`)
- **What happens**: `CalculateDynamicGeometry` computes `deltaA2c = newStructuralA2c - baseStructuralA2c` and pitches the frame around the rear axle using only `deltaA2c`. It accepts neither `baseRake` nor `targetRake`. Yet the Stack closure equation:
  ```text
  Stack = BB_drop - Rake × cos(HTA) + (A2C + CupOrSpacer + HTL) × sin(HTA)
  ```
  depends directly on `Rake`. Many frames in the registry use rigid CAD forks with `51–55 mm` rake while comparison suspension forks use `44 mm` rake. Dropping rake by `10 mm` at `68.5°` HTA shifts the front axle downward relative to the steerer by `-10 × cos(68.5°) = -3.66 mm`, leaving `FrontAxle.Y != RearAxle.Y`!
- **Quantified verification across registry frames** (rebuilding `FrontAxle.Y` and `RearAxle.Y` from `bikegeo`'s own output Reach, Stack, HTA, and BB Drop against an exact leveled 4-point solve):

| Setup | CAD Fork (A2C/Rake) | Target Fork (A2C/Rake) | Front Axle Off Level | Tool HTA vs. Leveled HTA | Tool Reach/Stack vs. Leveled |
| --- | --- | --- | --- | --- | --- |
| Otso Fenrir L | 495 / 54 | 495 / 54 | +0.2 mm *(rounding)* | 68.25° vs. 68.26° | 428.0 / 642.0 vs. 428.1 / 641.9 |
| Otso Fenrir L | 495 / 54 | 506 / 44 | **-3.6 mm** | 67.75° vs. 67.57° | 422.3 / 645.7 vs. 420.3 / 647.0 |
| Seven Custom | 492 / 52 | 506 / 44 | **-2.5 mm** | 69.34° vs. 69.21° | 416.7 / 635.8 vs. 415.3 / 636.8 |
| Seven 926f S 60cm | 472 / 55 | 506 / 44 | **-3.5 mm** | 68.39° vs. 68.21° | 390.8 / 651.2 vs. 388.8 / 652.5 |
| Salsa Fargo L | 483 / 51 | 486 / 44 | **-2.9 mm** | 68.86° vs. 68.71° | 386.4 / 662.0 vs. 384.7 / 663.0 |
| Surly Krampus L | 483 / 47 | 506 / 44 | **-2.1 mm** | 67.93° vs. 67.82° | 436.5 / 620.3 vs. 435.3 / 621.1 |

- **Why this matters**: `2.1–3.6 mm` of un-leveled axle height is the same magnitude as a `5%` sag step on a `100 mm` fork (`5 mm` A2C). Fixing this by pitching the actual 2D `(RearAxle, FrontAxle)` vector guarantees axles stay level to machine precision for every fork swap.

---

### C4. Physical Lower Headset Cup (`HeadsetCupType.CupStackMm()`) Is Promised by Validator But Ignored in `compare`

- **Where**: [validator.go](pkg/validator/validator.go#L208-L210) vs. [geometry.go](pkg/geometry/geometry.go#L1064-L1069) & [geometry_test.go](pkg/geometry/geometry_test.go#L1077-L1104)
- **What happens**: When a frame combines `HeadsetCupType: CupEC44` (`12.0 mm`) with `GeoHeadsetSpacerMm: 0` (`otso-fenrir-stainless`, `salsa-fargo`), `validator.go` emits a warning stating:
  > *"Validation uses geo_headset_spacer_mm to verify the published chart, while comparisons pitch the frame using the physical headset_cup_type stack."*
  Yet `AnalyzeBikeFitWithSaddle` adds `geoSpacer` (`0`) to *both* `baselineStructuralA2c` and `totalNewA2c`—so `HeadsetCupType.CupStackMm()` is never used in comparison pitching (only displayed), and `TestAnalyzeBikeFit_NormalizesCadBaseline` locks in the contradiction by asserting `DeltaAngleDeg == 0` on Fenrir XS at `495 mm`.
- **Impact on Otso Fenrir L (rigid 495/54)** if `otso-fenrir-stainless` truly omits the `12 mm` `EC44` cup from its CAD chart:

| Metric | Current Tool (`0 mm` cup) | Physical Build (`12 mm` EC44 cup) | Delta |
| --- | --- | --- | --- |
| Reach | 428.0 mm | 421.8 mm | **-6.2 mm** |
| Stack | 642.0 mm | 646.1 mm | **+4.1 mm** |
| HTA | 68.25° | 67.70° | **-0.55°** |
| BB Drop | 72.0 mm | 67.7 mm | **-4.3 mm** |
| Trail | 90.3 mm | 94.2 mm | **+3.9 mm** |

- **Fix**: Record per frame (with manufacturer CAD verification) what the published Stack was drawn with (`CadLowerCupMm`, today's `GeoHeadsetSpacerMm`) and which cup the built bike uses (`PhysicalLowerCupMm`, today's `HeadsetCupType.CupStackMm()`), and pitch comparisons by the difference.
- **Recommended default**: apply the physical cup. The page is titled "Apples-to-Apples", the validator message already promises it, and the two affected frames fill 8 of the 20 rows in the default comparison (Fenrir, the rider's baseline bike, four times; Fargo four times). Adopting it keeps the validator message and changes `TestAnalyzeBikeFit_NormalizesCadBaseline` to expect the pitch from the cup difference instead of zero. If the opposite is chosen, the message is what changes. Either way the code, the test and the message change in one commit.

---

### C5. "Phantom Fenrir" Posture & Weight Distribution (`CalculateBaselinePosture`)

- **Where**: [profiles.go](pkg/catalog/profiles.go#L7-L19), [run.go](pkg/compare/run.go#L218-L220), [geometry.go](pkg/geometry/geometry.go#L818-L991) & [#L1132-L1158](pkg/geometry/geometry.go#L1132-L1158)
- **What happens**:
  1. `DefaultRiderProfile()` defines `SaddleHeightMm = 780.0` (hardcoded around an Otso Fenrir L with `110 mm -6°` stem and `30 mm` setback offset), while `compare.Run` overwrites `SaddleHeightMm = 746.0`.
  2. `AnalyzeBikeFitWithSaddle` ignores the frame being analyzed and calls `CalculateBaselinePosture(GetDefaultBaselineBike(), profile)` (approx. 120 lines of torso/arm anthropometry with 58/10/32% mass splits), which computes `BackAngleDeg = 53.6°` and `SystemCgX = -13.60 mm` behind the BB—identical for every bike in the catalog.
  3. `CalculateWeightDistribution` then computes `FrontWt% = (RearCenterHoriz + (-13.60)) / Wheelbase` (`weight = 4` in `IdmatchProfile`).
- **Impact**: `Modeled Back Angle: 53.6°` is printed on every comparison header regardless of bike, and changing stems, spacers, bars, stack, reach, or STA has zero effect on `Front Wt`.
- **Fix**: Delete `GetDefaultBaselineBike` and `CalculateBaselinePosture`, and report an honest, labelled per-bike ratio (`RearCenter / Wheelbase`, or BB position as a fraction of wheelbase) as a display-only value. A per-bike rider CoG from the solved contact points (a weighted mean of `SitBones.X` and `HandGrip.X`) becomes possible once `BikeScene` exists, but its mass-split weights would be new unsourced constants, which is the kind of thing this finding removes. Add it only with a cited source and with the weights shown on the page.

---

### C6. Closure Equation Mismatch Between `validator/residuals.go` and `geometry/budget.go`

- **Where**: [residuals.go](pkg/validator/residuals.go#L87-L123) vs. [budget.go](pkg/geometry/budget.go#L187-L257) & [validator.go](pkg/validator/validator.go#L294-L320)
- **What happens**: `FrontCenterBudget`, `WheelbaseBudget`, and `ValidateBikeGeometry` eliminate fork A2C by using **Stack-implied** structural A2C (`(Stack - BbDrop + Rake × cos(HTA)) / sin(HTA)`), making horizontal front-center checks completely independent of `Fork.A2cMm` and `GeoHeadsetSpacerMm`. Meanwhile, `SizeResiduals` computes `FC calc` / `ΔFC` and `WB calc` / `ΔWB` using `evalStructuralA2c = expectedForkA2c + geoSpacer` (the **declared** fork A2C).
- **Impact**: In `bikegeo fit`, `--try fork.a2c=...` or `--try spacer=...` shifts `ΔFC` and `ΔWB` in the table while `FrontCenterBudget` and `WheelbaseBudget` report zero change. Both choices are deliberate. [budget.go](pkg/geometry/budget.go#L187-L189) says "the front end is as long as the published Stack says it is", so a wrong fork hypothesis fails only the A2C check. [residuals.go](pkg/validator/residuals.go#L87-L88) evaluates at the declared fork "so residuals reflect full frame+fork discrepancy". One shared closure function therefore needs a rule, not just one code path.
- **Recommended rule**: pass/fail keeps today's `budget.go` semantics. Front Center and Wheelbase are checked at the chart-implied front axle, and the declared fork is tested only by the A2C check (Part II §1). This leaves every frame's current validation result and `GeoSlopMm` allowance unchanged. `bikegeo fit` may still print declared-fork `ΔFC` / `ΔWB` as a hypothesis diagnostic, in columns labelled as such.

---

### C7. Duplicated 3-Pass Cockpit Selection & Scattered Fit Literals

- **Where**: [canonicalizer.go](pkg/canonicalizer/canonicalizer.go#L680-L767) (`ApplyIdealCockpitTargets`), [scoring.go](pkg/scoring/scoring.go#L115-L189) (`ScoreRow`), [geometry.go](pkg/geometry/geometry.go#L1021-L1201), [saddle.go](pkg/geometry/saddle.go#L78-L83)
- **What happens**:
  - `CanonicalizeBikes` runs `AnalyzeBikeFitWithSaddle` (populating `PossibleHBs` around `DeadCenterBRP`). `ApplyIdealCockpitTargets` then optimizes `SelectedBRP`, re-runs `MapPossibleHBsWithHandlebar`, and runs `ApplyCockpitFit`. Then `ScoreRow` repeats the exact ~75-line BRP → `MapPossibleHBsWithHandlebar` → `ApplyCockpitFit` block on a **value copy** of `row.Canonical` and discards the mutations! Both `PruneRows` and `BuildDocument` call this cascade repeatedly.
  - Hardcoded fit numbers (`746.0`, `219.0`, `292.0`, `700.0`, `13.0`, `3.0`, and `DefaultSeatpostSetbackMm = 11.0`) appear as bare literals across 5+ packages. Worse, saddle-translation calls pass literal `746.0` instead of the profile's field.
- **Fix**: One explicit `RiderFit` struct (no literals inside geometry/scoring), one single-pass constructor `BuildScene`, and read-only scoring.
- **Severity today, and why the order matters**: `compare` has no way to change saddle height, so the literal `746.0` is latent, not active. The `/bike` workbench in Part III adds a `saddle_h` URL parameter, which makes it live. `RiderFit` therefore has to arrive with `BikeScene` (step 3), not in the later input-tightening step.

---

### C8. Silent Fallback Defaults Masking Missing Inputs (~20 Sites)

About two dozen sites silently invent values when inputs are `<= 0` rather than returning an error:

| Quantity | Invented Fallback(s) | Locations |
| --- | --- | --- |
| Wheelbase | `1150.0` | [geometry.go](pkg/geometry/geometry.go#L719), [#L976](pkg/geometry/geometry.go#L976), [#L1073](pkg/geometry/geometry.go#L1073) |
| Chainstay | **`430.0`** in 2 places vs. **`440.0`** in 2 others | [geometry.go](pkg/geometry/geometry.go#L722), [#L1077](pkg/geometry/geometry.go#L1077) vs. [#L967](pkg/geometry/geometry.go#L967), [#L1141](pkg/geometry/geometry.go#L1141) |
| Head tube length | `120.0` | [geometry.go](pkg/geometry/geometry.go#L1151) |
| Fork rake | `44.0` | [geometry.go](pkg/geometry/geometry.go#L1267), [canonicalizer.go](pkg/canonicalizer/canonicalizer.go#L91), [#L315](pkg/canonicalizer/canonicalizer.go#L315), [#L333](pkg/canonicalizer/canonicalizer.go#L333), [#L502](pkg/canonicalizer/canonicalizer.go#L502) |
| STA / HTA / Reach / Stack | `74.71 / 67.71 / 421.9 / 646.0` | [geometry.go](pkg/geometry/geometry.go#L854-L876) |
| Fit targets (`hbDropMm == 0`) | `746.0 / 292.0 / 700.0 / 13.0` | [saddle_swap.go](pkg/geometry/saddle_swap.go#L106-L120) |
| Ellipse semi-axes | `10.0 / 20.0` | [geom2d.go](pkg/geom2d/geom2d.go#L369-L375) |

Notably, `if hbDropMm == 0 { hbDropMm = 13.0 }` in [saddle_swap.go](pkg/geometry/saddle_swap.go#L118-L120) makes a `0 mm` (level bar) input impossible to enter. Constructors should return explicit errors on missing required geometry; downstream code can then trust its inputs unconditionally.

The validator already rejects registry frames that lack these essentials, so most of the fallbacks cannot fire for real data. They exist to keep partially filled, hand-built test structs working. Converting them to errors is therefore mostly test-fixture work (one shared, complete fixture builder) and should be sized as such.

---

### C9. Circular Validation of Derived Wheelbases & Float-Inferred `ChartPrecision`

- **Where**: [validator.go](pkg/validator/validator.go#L33), [moots_mountaineer.go](pkg/frames/moots_mountaineer.go#L23), [seven_expedition_926d.go](pkg/frames/seven_expedition_926d.go#L25), [seven_expedition_926f.go](pkg/frames/seven_expedition_926f.go#L25), [budget.go](pkg/geometry/budget.go#L63-L82)
- **What happens**:
  1. `validator.go` requires `Wheelbase > 0` for every frame. When a maker omits wheelbase, the frame notes candidly record *"Wheelbases are derived from horizontal rear center plus front center"*—meaning the ingested Go struct stores a pre-calculated wheelbase that `WheelbaseBudget` then circularly confirms matches the identical formula.
  2. `InferChartPrecision` infers column precision from runtime `float64` values across sizes (`70` and `70.0` are indistinguishable). On a single-size frame like `seven-custom` (`HtaDeg: 70`), it assumes `±0.5°` degree rounding (`±2.4 mm` in A2C budget) even though the CAD notes specify `70.0°` (`±0.05°`).
- **Fix**: Make `Wheelbase` optional (like `FrontCenter`—skipping the check when omitted instead of fabricating ground truth), and allow an optional declared `ChartPrecision` field on `FrameModel` with inference only as fallback.

---

### C10. Typed Domain Facts Re-Derived from Strings

- `ForkRef.ForkTravel()` ([bike.go](pkg/model/bike.go#L1007-L1023)) splits `Fork` IDs on `"_"` and parses numbers in `[60, 200]`, which would misclassify any 40–50 mm gravel suspension fork as `0 mm` rigid (latent: the catalog today holds only rigid forks and 100–150 mm travel), even though [catalog.go](pkg/catalog/catalog.go) already stores `TravelMm`.
- `IsRigidFork` and `FormatForkType` ([fork_format.go](pkg/canonicalizer/fork_format.go#L12-L104)) inspect display strings (`"rigid"`, `"@"`) instead of booleans/fields.
- `ForkRef.A2cMm` means sagged A2C when `SagPct > 0` and static A2C otherwise, while `DefaultComparisonState()` ([specs.go](pkg/compare/specs.go#L377)) hand-types derived `A2cMm` and `DisplayLabel` fields that the catalog already determines.
- Attribute IDs are matched as lowercased strings across ~18 sites in 5 files, backed by ~55 aliases in `ideal.LookupAttribute` ([registry.go](pkg/ideal/registry.go#L396-L434)) despite all profiles being compiled into Go structs. Enforce typed enums (checked by `exhaustive`) and catalog `Fork` refs instead of string parsing.

---

### C11. Smaller Mathematical, Scoring & Implementation Defects

- **`curve.StemPenalty` slope halves at the boundary** ([curve.go](pkg/curve/curve.go#L145-L160)): The docstring states *"Beyond the boundaries it keeps growing, more steeply."* However, with `MaxStemPenalty = 15.0`, the interior curve `15 × u²` has derivative `dp/du = 30.0` as `u → 1-`, whereas the exterior curve `15 × (1 + 2×over² + over)` (`over = u - 1`) has derivative `dp/du = 15.0` at `u → 1+`—the penalty slope drops by **half** right at `u = 1`! Replacing `+ over` with `+ 2.0*over` restores C1 smoothness (`dp/du = 30.0`), and `+ 2.5*over` or `+ 3.0*over` makes it genuinely steeper outside `[minLen, maxLen]`.
- **`Oval Fits` cell displays count while graded on continuous sub-score** ([eval.go](pkg/ideal/eval.go#L51-L86), [document.go](pkg/compare/document.go#L495-L506)): The main table prints integer `CountInside` (e.g., `2 ✗` vs `2 ~` in `compare all`, or `0 ✗`) alongside the `≥100` / `≥75` headers, obscuring why setups with the same count score differently. Showing `"90mm -6° +0mm · R=0.57 (18in)"` or `"R=0.57 · 18in ✓"` makes the cell self-explanatory.
- **`Seatpost Ext` mixes radial height with maker-dependent tube conventions**: `seatHeight - seatTubeLength` ([geometry.go](pkg/geometry/geometry.go#L657-L662)) subtracts a nominal `C-C` (`seven-custom`) or `C-T` tube length from radial `BB-to-BRP` distance (`746 mm`), yet is weighted `4` in `IdmatchProfile`.
- **`HasBentSeatTube()` inconsistency**: `BikeGeometry.HasBentSeatTube()` ([bike.go](pkg/model/bike.go#L287-L289)) requires `ActualStaDeg > 0 && |ActualStaDeg - StaDeg| > 0.001`, while `FrameModel.HasBentSeatTube()` ([bike.go](pkg/model/bike.go#L1137-L1139)) checks only `ActualStaDeg > 0`. Effective-STA logic is implemented three times across packages.
- **Non-deterministic validation summary**: `FrameValidationReport.ErrorSummary()` ([bike.go](pkg/model/bike.go#L1465-L1475)) ranges over `r.SizeReports` (`map[string]ValidationReport`) and returns `msgs[0]`, producing non-deterministic error strings across runs.
- **`PruneRows` selects suspension sag by fit score** ([prune.go](pkg/compare/prune.go#L14-L100)): With `all:all@all`, sag (`20%`, `25%`, `30%`) acts as a free score optimizer—for example picking `110mm @ 30% sag (488mm)` for `seven-custom` (`Score 26`) instead of its `492mm` CAD rigid fork (`Score 23`) because `4 mm` less A2C moves grips fractionally closer to the oval.
- **Wheel radius normalization**: Hardcoded `372 mm` for 29ers (matching unloaded `311 + 60.96` despite comments calling it loaded). Trail shifts ~0.4 mm per mm of radius against an 8 mm ideal window—acceptable as a stated normalization if documented explicitly alongside seatpost setback and headset stack.
- **Dead exported code**: ~25 exported symbols appear unreferenced outside tests or definitions (`geom2d.Transform2`, `RotationForLift`, `AngleFromLift`, `HBToGrip`, `GripToHB`, `CalculateHandlebarOffset`, `ClampOffsetFromBrp`, `ForkSpec`, `geometry.CalculateEffectiveSta`, `MapPossibleBRPs`). The list comes from `grep`, not a call graph, so confirm each symbol with `deadcode` before deleting it. `BaselineMetrics`, for example, looks unused but is populated in [canonicalizer.go](pkg/canonicalizer/canonicalizer.go#L633); it goes only with the Part II restructure. Adding `go tool deadcode ./...` to `make check` (as a `tool` dependency, like `exhaustive`) prevents future accumulation.

---

## Part II: The "Obviously Correct" Architecture — Unified 2D `BikeScene` & Fit Solve

### 1. One Sagittal 2D Point Model (`BikeScene` in `pkg/geometry/scene.go`, ~250 Lines)

Instead of passing loose floats between `DynamicGeometryResult`, `BikeFitAnalysis`, `BRPPoint`, and `HBPoint`, compute **one immutable `BikeScene`** per build. Every metric, validator check, closed-form fit solve, and SVG primitive reads directly from the same points:

```text
                       GR (HandGrip) o<-- bar offset --o CL (StemClamp)
                                                      /
                SB (SitBones) o                      / stem
             BRP o             \ saddle             o SC (SteererClamp)
                  \                                 |  spacers + stemH/2 + headsetTop
                   \                                o HT (Reach, Stack)
                    \  seat tube                   /
                     \                            /  steerer (HTL + cup + A2C_active)
                      \                          /
        RA o-----------o BB (0,0)               /
         ( )                                   o--o FA   (rake, perpendicular)
          |                                       ( )
   -------o----------------------------------------o--------- ground (Y = -WheelRadius + BbDrop)
         RC                                       FC
```

**Typed inputs (no hidden defaults inside builders)**:
- `FrameSize`: `Reach`, `Stack`, `Hta`, `Sta`, `Htl`, `BbDrop`, `Chainstay`, optional `ActualSta` + `SeatTubeOffsetX`, optional `SeatTubeLength`, optional published `Wheelbase`, `FrontCenter`, `CadLowerCupMm`, `Precision`.
- `BuildSpec`: `Fork{ID, StaticA2c, Rake, Travel}`, `SagPct`, `PhysicalLowerCupMm`, `WheelRadiusFront`, `WheelRadiusRear`.
- `CockpitSpec`: `HeadsetTopStackMm`, `SpacerMm`, `Stem{Length, Angle, ClampHeight}`, `Handlebar`, `BarRoll`, `GripInset`.
- `SeatSpec`: `PostSetbackMm`, `Saddle`, `RailOffsetMm`.
- `RiderFit`: `LegExtensionOrBrpHeight`, `SitBonesTargetX` (or `TargetBrpSetback`), `TargetPoint` + explicit required `TargetRef` (`TargetHandGrip`, `TargetUnsweptBarTop`, `TargetStemClamp`, or `TargetReferenceBarClamp`; see C1—no bare/implicit `HB` target is permitted), `ReferenceSaddle`, `ReferenceBar` (the bar on the fit bike; required when `TargetRef == TargetReferenceBarClamp`).
- Lower-cup names: `CadLowerCupMm` is today's `GeoHeadsetSpacerMm`, and `PhysicalLowerCupMm` is today's `HeadsetCupType.CupStackMm()`. Only those two names are used below.

**Exact 2D Construction (BB initially `(0,0)`, `+X` forward, `+Y` up)**:

```text
u = (-cos HTA,  sin HTA)           // up the steerer
n = ( sin HTA,  cos HTA)           // forward perpendicular to steerer (rake direction)
s = (-cos STA,  sin STA)           // up the seat tube (uses EffectiveSta(H, Stack, Sta) if ActualSta present)
r = (-sin STA, -cos STA)           // rearward perpendicular to seat tube

// 1. Chart attitude landmarks (with active fork, sag, and lower headset cup)
BB_0  = (0, 0)
RA_0  = (-sqrt(Chainstay^2 - BbDrop^2), BbDrop)
HT_0  = (Reach, Stack)
HTB_0 = HT_0 - HTL * u
A2C_eff = Fork.StaticA2c - Fork.Travel * SagPct

// 1a. Chart-implied front end. The published Reach/Stack/HTA/BbDrop win, so the CAD
//     configuration is level by definition and reproduces the chart exactly.
L_chart  = (Stack - BbDrop + CadFork.Rake * cos HTA) / sin HTA   // HT -> axle, along the steerer
                                                                 // (frontEndLength in budget.go)
FA_chart = HT_0 - L_chart * u + CadFork.Rake * n                 // FA_chart.Y == RA_0.Y == BbDrop

// 1b. The setup contributes only its differences from the CAD configuration.
//     (If CadFork.A2C_eff is unset in synthetic tests, fall back to ForkA2c + CadLowerCupMm
//     when within A2cBudget, else L_chart - HTL.)
dL    = (A2C_eff + PhysicalLowerCupMm) - (CadFork.A2C_eff + CadLowerCupMm)
FA_0  = HT_0 - (L_chart + dL) * u + Fork.Rake * n
// With CadFork and CadLowerCupMm: dL = 0, FA_0 == FA_chart, and theta == 0 below.
// Validation only: the declared-fork axle is
// FA_decl = HTB_0 - (CadLowerCupMm + CadFork.A2C_eff) * u + CadFork.Rake * n

// 2. Exact rear-axle pitch leveling (handles A2C, Sag, Cup, AND Rake changes together)
theta = -atan2(FA_0.Y - RA_0.Y, FA_0.X - RA_0.X)   // positive pitches nose up (slackens HTA/STA)
Rotate {RA_0, BB_0, HT_0, HTB_0, Crown_0, FA_0, SeatTube} by theta around RA_0,
then translate by -BB_rotated so BB == (0, 0).

// 3. Front cockpit on leveled frame (HTA_act = HTA - theta)
SC = HT + (HeadsetTopStackMm + SpacerMm + StemClampHeight/2) * u_act
CL = SC + Stem.Length * rotate(n_act, Stem.Angle)
GR = CL + BarOffset(Handlebar, GripInset, BarRoll)

// 4. Rear cockpit on leveled frame (STA_act = STA_eff - theta)
SA  = H_frame * s_frame_act                  // frame-only seat-axis point at radial H_frame (746 mm)
p   = PostSetbackMm + RailOffsetMm - Saddle.RailCenterToBRP
BRP = sqrt(H^2 - p^2) * s_act + p * r_act   // exact intersection at target BB-to-BRP height H (or SitBones Y)
SB  = BRP + Saddle.BRPToSitBones
```

**Baseline rule: the chart wins.** A published chart rarely closes exactly on its declared fork. Fenrir L is `0.2 mm` off, which is rounding. Placing the front axle from the declared fork and then leveling would pitch the CAD configuration itself: the first row of the C3 table shows Reach/Stack/HTA of `428.1 / 641.9 / 68.26°` against the published `428 / 642 / 68.25°`, and the shift grows with a chart's A2C residual. Steps 1a and 1b instead keep today's behaviour ([geometry.go](pkg/geometry/geometry.go#L1059-L1069), pinned by `TestAnalyzeBikeFit_NormalizesCadBaseline`): the CAD configuration reproduces the chart exactly, and a setup moves the front axle only by what it changes (A2C, sag, cup, and now rake). The declared fork is tested against the chart in one place only, the A2C check below.

**Every output metric becomes a 1-line readout**:

| Metric | 1-Line Definition on `BikeScene` | Replaces |
| --- | --- | --- |
| `Reach`, `Stack` | `HT.X`, `HT.Y` | `DynamicGeometryResult.NewReach/NewStack` |
| `Hta`, `Sta`, `BbDrop` | `Hta_cad - theta`, `Sta_cad - theta`, `RA.Y` *(verified `== FA.Y`)* | `NewHta`, `NewSta`, `NewBbDrop` |
| `FrontCenter`, `RearCenter`, `Wheelbase` | `FA.X`, `-RA.X`, `FA.X - RA.X` | `CalculateFrontCenter`, `CalculateWeightDistribution` splits |
| `Trail` | `(WheelRadius * cos(HTA_act) - Rake) / sin(HTA_act)` | `CalculateTrail` |
| `ETT`, `AETT` | `HT.X - NominalSeatRay.AtY(HT.Y).X`, `SteererRay.AtY(SA.Y).X - SA.X` *(frame-only at radial `H_frame = 746 mm`, independent of saddle/rail)* | `CalculateEtt`, `CalculateAett` |
| `SitBoneSetback`, `BRPSetback` | `-SB.X`, `-BRP.X` | `CalculateBRPPoint`, `TranslateBrp*` |
| `GripDistFromBRP`, `GripRiseAboveSB` | `GR.DistanceTo(BRP)`, `GR.Y - SB.Y` | `HBPoint.SaddleHbMm`, `SaddleHbDropMm` |
| `RailMarginMm` | `Saddle.HalfRailWindow - abs(RailOffsetMm)` | Hidden inside boolean checks |

**Unified validation**: With `CadFork`, `CadLowerCupMm`, and `theta = 0`, validator closure is three readings off the same construction. Each names the axle it uses, which preserves today's `budget.go` pass/fail semantics (C6):
- `ResidualA2c`: the declared fork against the chart, `(CadFork.A2C_eff + CadLowerCupMm + HTL) - L_chart` (equivalently `(RA_0.Y - FA_decl.Y) / sin(HTA)`). This is the only check that depends on the fork hypothesis.
- `ResidualFc`: `hypot(FA_chart.X, FA_chart.Y) - PublishedFrontCenter`
- `ResidualWb`: `(FA_chart.X - RA_0.X) - PublishedWheelbase`
`pkg/geometry/budget.go` then perturbs inputs straight through this single constructor, and `residuals.go` reads the same three values instead of keeping its own closed forms. If `bikegeo fit` keeps declared-fork `ΔFC` / `ΔWB` columns as a diagnostic, they read `FA_decl` and are labelled as such.

### 2. Sit-Bone-First & Explicit-Point Cockpit Fit Normalization

Convert idmatch's `sqlab-610-ergolux` BRP coordinates (`BRP = (-219, 713.1)`, `SitBones = (-292, 723.1)`) and cockpit target (`TargetPoint` with an explicit required `TargetRef` of `TargetHandGrip`, `TargetUnsweptBarTop`, `TargetStemClamp`, or `TargetReferenceBarClamp`, per C1) **once** at profile load into physical coordinates:

1. For any candidate saddle `S`, solve `TargetRailCenter = TargetSitBones - (S.RailCenterToSitBones)` and solve `RequiredRailOffsetMm` against the active seat tube + post offset line in one step. Clamp to `[-HalfWindow, +HalfWindow]` only to display physical limits; surface `RailMarginMm = HalfWindow - |RequiredRailOffsetMm|` directly.
2. Compute `BRP` and `BB-to-BRP Saddle Height` as pure workshop outputs for the mechanic's calipers.
3. **Delete** `TranslateSaddleHeightFromRef`, `TranslateHbDistanceFromRef`, `TranslateBrpFromRef`, `TargetBrpForPelvicSetback`, `TranslateClampOffset`, and `translateAttributeSpecForSaddle` (~200 lines) along with every `746.0` / `219.0` / `292.0` / `3.0` literal outside `IdmatchProfile()`.

### 3. Closed-Form 2×2 Linear Algebra Fit Solve vs. Continuous Curve Scoring

Notice a striking mathematical fact about front-cockpit fitting: once `TargetPoint` (with its explicit `TargetRef`) and handlebar (`Bar`, `Roll`) are known, the required `StemClamp` point `C*` is fixed:
```text
TargetHandGrip:           C* = TargetPoint - BarOffset(Bar, Roll)
TargetUnsweptBarTop:      C* = TargetPoint - (0, Bar.StatedRiseMm)
TargetStemClamp:          C* = TargetPoint
TargetReferenceBarClamp:  C* = TargetPoint + BarOffset(ReferenceBar) - BarOffset(Bar, Roll)
```
For any stem angle `a` (e.g., `-6°`, `0°`, `+6°`, `+17°`), finding the exact continuous **stem length `L`** and **steerer distance `t`** (`t = HeadsetTopStack + SpacerMm + StemClampHeight/2`) that puts `StemClamp` at `C*` is just **two linear equations in two unknowns**:
```text
HT + t · u_steer + L · u_stem(a) = C*
```
Solving that 2×2 system (`det(u_steer, u_stem(a)) != 0` whenever stem isn't parallel to steerer) takes **5 lines of exact arithmetic** and outputs what the rider actually buys (making any clamp-vs-grip discrepancy unmistakable):

```text
Seven Custom (TargetRef = TargetUnsweptBarTop) -6°:  70 mm stem,   0 mm spacers | rail +13 mm | within hardware range (exact solve: 69 mm, -1 mm)
Seven Custom (TargetRef = TargetHandGrip)      -6°: 140 mm stem, -14 mm spacers | rail +13 mm | stem > 100 mm (+40 mm over), spacers < 0 (not buildable)
Seven Custom (TargetRef = TargetHandGrip)     -17°: 145 mm stem, +14 mm spacers | rail +13 mm | stem > 100 mm (+45 mm over)
```
*(Computed by hand from the 2×2 system above, not yet by code. Inputs: `HT = (424, 631)`, `HTA = 70°`, `HeadsetTopStackMm = 11`, `StemClampHeight = 40`, `TargetPoint = (480.9, 726.1)`, `Bar.StatedRiseMm = 50` giving `C* = (480.9, 676.1)` under `TargetUnsweptBarTop`, Persuader offset `(-72.8, +45.5)` giving `C* = (553.7, 680.6)` under `TargetHandGrip`. The rail offset is today's tool output. Exact stock rounding adds ±2.5 mm.)*

This gives two clean architectural options for grading bikes:
- **Radical Fit Solve (Recommended)**: Replace the Fit Oval, 3-pass stem grid search, `VersatilityBonus`, `StemPenalty`, and composite weighted score with the closed-form 2×2 solve + nearest stock `(Stem, Spacer)` hardware match + millimeter excesses outside hardware limits (`0 mm` = exact hardware fit; `+40 mm stem` = transparent gap). Cuts ~2,200 LOC while eliminating arbitrary weights and score-vs-display mismatches.
- **Conservative Simplified Curve Scoring (If a 0–100 score is retained)**: Feed the closed-form solved `(StemLength, SpacerMm, RailOffset)` directly into `pkg/curve` (`StemPenalty` fixed for C1 monotonicity at `u = 1` + `Falloff` on clamp/grip error), keep non-frame metrics (`FrontWt`, `SeatpostExt`, `BB Drop`, `Trail`) clearly separated from cockpit fit, delete the duplicate `ApplyIdealCockpitTargets` block in `ScoreRow`, and print the real fit metric (`"70mm -6° +0mm · R=0.12"`) instead of `"6 ✓"`.

### 4. Four Registry-Wide Invariant Property Tests

Add one test function looping over **every frame, size, fork, and sag in `frames.AllFrames()`** (`pkg/frames` + `TestAllFrameData`):
1. `AxlesStayLevel`: `|Scene.FrontAxle.Y - Scene.RearAxle.Y| < 1e-6` for every frame × fork × sag (immediately catches C3).
2. `RigidDistancesInvariantUnderPitch`: Euclidean distances `|HT - BB|`, `|HTB - BB|`, `|BB - RA|`, `|HT - HTB|`, and `HTL` never change under any fork swap or cup swap. The included angle `HTA - STA` (nominal) is likewise identical for every setup of a frame size.
3. `CADForkReproducesPublishedChart`: `BuildScene(frame, size, CadFork, CadLowerCupMm)` recovers published `Reach`, `Stack`, `Hta`, `Sta`, `BbDrop` to `< 1e-6` (true under the baseline rule in §1, where `dL = 0` gives `theta = 0`), recovers published `FrontCenter` and `Wheelbase` within `FrontCenterBudget` / `WheelbaseBudget`, and finds the declared fork within `A2cBudget` of `L_chart`.
4. `CupAndA2cAreInterchangeable`: At equal rake, adding `x mm` of lower cup and adding `x mm` of A2C produce identical scenes to `< 1e-9`, and a longer front end never steepens `Hta`. An A → B → A round trip is not used: `BuildScene` is a pure function, so that test cannot fail.

---

## Part III: Computational & Visual Transparency — Server-Rendered SVG Side View, Interactive Tweaker & Arithmetic Ledger

Because `bikegeo serve` uses pure Go `html/template` and Web 1.0 `GET` forms with zero JS, **inline `<svg>` driven straight from `BikeScene`'s millimeter coordinates** requires zero new dependencies and is testable in-process with `goquery` (`svg [data-point="GR"]`). Using `<g transform="scale(1,-1)">` emits `BikeScene` `(X, Y)` coordinates directly while keeping text annotations upright in an un-flipped overlay group.

**Build order and size**: Panel A alone, with `TargetPoint`, `StemClamp` and `HandGrip` labelled, is what would have exposed C1 and C2, and [SIMPLIFY_C.md](SIMPLIFY_C.md) estimates that single view at 250–350 lines of Go and template. Ship it first. Panels B and C, the `BB` / `SitBones` alignment toggle, the `/validate` overlay and the CLI `--trace` are follow-ons, each to be justified and sized on its own. Together they are new surface in a plan whose aim is less of it.

### 1. Three Coordinated Inline SVG Panels

1. **Panel A: Full-Bike Blueprint & Ghost Overlay (`/bike` + `/` `/`compare` cards + `/validate`)**:
   - **Draws**: Ground line (`Y = BbDrop - WheelRadius`), wheel circles, chainstay, seatstay, down tube, top tube, seat tube (with any bent kink shown), head tube, lower cup stack (`PhysicalLowerCupMm`), fork crown/blades + perpendicular rake offset, steering axis extended to ground + Trail callout, seatpost + setback head, saddle rail window + solved clamp + `BRP` + `SitBones`, upper headset cover (`HeadsetTopStackMm`), spacer stack, stem, handlebar arrow (`StemClamp → HandGrip`), and rider `SitBones ↔ HandGrip` line.
   - **Multi-bike overlay**: Render primary frame in solid stroke (`#2563eb`) and comparison setups (`b.frame=...`, `b.stem=...`) as dashed semi-transparent overlays, with a URL toggle to align either at **`BB (0,0)`** (comparing frame kinematics) or **`SitBones`** (comparing cockpit/wheelbase position for identical pelvis placement).
   - **Chart validation mode on `/validate`**: Overlay published chart geometry (dashed gray) against reconstructed closure from `CadFork + CadLowerCupMm` so `ΔA2C`, `ΔFC`, and `ΔWB` appear visually right beside the residual table.

2. **Panel B: Front Cockpit Zoom Box (`~260 × 260 mm` around `HeadTubeTop`)**:
   - **Reachable Stem/Spacer Polygon**: Draw the exact boundary polygon traced by `[MinStem..MaxStem] × [0..MaxSpacerMm]` for each catalog stem angle (replacing cryptic `"N in oval"` counts with an immediately legible envelope), plus discrete stock-stem grid dots (`PossibleStemClamps` / `PossibleHandGrips`), bold selected stem/spacer vector, handlebar offset arrow (`StemClamp → HandGrip`), and the 30°-rotated `FitOval` around `TargetPoint`.
   - Had this zoom box existed from day one—explicitly labeling **`TargetPoint`**, **`StemClamp`**, and **`HandGrip`**—both the ~70 mm Persuader backsweep discrepancy (`C1`) and the ~11 mm missing headset top cover (`C2`) would have jumped off the screen in 1 second.

3. **Panel C: Rear Saddle & Seatpost Rail Zoom Box (`~220 × 160 mm` around `SeatpostClamp`)**:
   - Draws `EffectiveSTA` ray, perpendicular `SeatpostSetbackMm` offset, usable straight rail window `[-HalfWindow, +HalfWindow]` shaded green, solved clamp pin, `RailCenter`, `SitBones` elevation/setback step, and `70 mm BRP` caliper mark.

### 2. Zero-JS Web 1.0 Interactive Workbench ("What-If Tweaker")

- One `<form method="GET">` with bookmarkable query overrides: `frame`, `size`, `fork`, `sag`, `stem_len`, `stem_ang`, `spacer`, `bar`, `bar_roll`, `bar_width`, `saddle`, `post_sb`, `rail_off`, `saddle_h`, plus optional ghost `b.*` params and `Auto (Best Fit)` toggle.
- Beside numeric fields, render server-generated `<a>` nudge links (`−10`, `−5`, `+5`, `+10`, `Flip ±6°`) built from `url.Values`. Every click reloads a bookmarkable state updating all three SVG panels, contact deltas (`ΔHands X/Y`, `ΔSitBones`, `ΔRailMargin`), and the ledger—subsuming the separate `/handlebars` and `/saddle-swap` comparison pages cleanly.
- Every cell in the `/` comparison table links straight to `/bike?...` for that setup.

### 3. 5-Stage "Show Your Work" Arithmetic Computation Ledger (`<details>` + CLI `--trace`)

Extending `<details><summary>` in [compare.html](pkg/web/templates/compare.html#L54), emit one compact derivation table printing **(Quantity | Plain Formula | Substituted Inputs | Result)** for every intermediate value:
1. **Chart & Closure**: `Published Reach, Stack, HTA, HTL, BbDrop, CS, WB` → `Implied Structural A2C` vs. `CadFork + CadLowerCupMm`.
2. **Fork, Cup & Exact Pitch**: `A2C_eff`, `PhysicalLowerCupMm`, `Rake`, `FA_0.Y - RA_0.Y`, `Δθ = -atan2(...)` → Leveled `(Reach, Stack, HTA, STA, BbDrop, FC, WB, Trail)`.
3. **Rear Cockpit**: `EffectiveSTA`, `PostSetback`, solved `RailOffset (margin)` → `SitBones (X, Y)`, Workshop `BRP`, `BB-to-BRP SaddleHeight`.
4. **Front Cockpit**: `HeadsetTopStack + Spacer + StemH/2` → `SteererClamp (X, Y)` → `StemClamp (X, Y)` → `BarOffset (ΔX, ΔY @ Roll)` → `HandGrip (X, Y)`.
5. **Fit Target / Hardware Solve**: Exact 2×2 solved `(StemLength, SpacerMm)` + nearest stock fit (and if retained, `FitOval R`, `StemPenalty`, `FinalSubScore`, and `PointsLost`).

---

## Part IV: Feature Reduction, Codebase Diet & Quality Principles

### 1. Conservative Consolidation vs. Radical Fit-Solve Diet

Every figure below is measured from `find/wc -l` on current non-test Go sources (**18,060 total = 2,024 in `pkg/frames` + 16,036 non-frame**):

| Area | Current Non-Test LOC | Conservative Consolidation (~40% Non-Frame Cut) | Radical Fit-Solve Diet (~65–70% Non-Frame Cut) |
| --- | --- | --- | --- |
| **`pkg/model` (`bike.go`, `opt.go`, `units.go`)** | **1,674** | Split into clean input types + `BikeScene`; drop `BaselineMetrics`, `ForkSpec`, `CanonicalConfig`, duplicate `BikeGeometry`/`FrameModel` getters (~700 LOC) | Same minimal input structs + `BikeScene` (`~400 LOC`) |
| **`pkg/geometry` (`geometry.go`, `budget.go`, `saddle*.go`, `handlebar.go`)** | **2,348** | Unified `BikeScene` (`~250`), `budget.go` (`~250`), consolidated saddle math (`~120`), bar offset (`~80`), fold `saddle_swap` into `BikeScene` (`~750 LOC`) | Unified `BikeScene`, shared closure/budget, bar offset, closed-form rail & 2×2 cockpit solve (`~450 LOC`) |
| **`pkg/canonicalizer`** | **871** | Typed `Fork` + `Setup`, drop `ScoreRow` duplicate pass, drop string-parsed labels/travel (`~220 LOC`) | Merge into `BuildScene` + `compare`; keep typed catalog lookup (`~150 LOC`) |
| **Scoring + Ideal + Curve (`scoring`, `ideal`, `curve`)** | **1,995** (`682` + `1,149` + `164`) | Keep `pkg/curve` (fix `StemPenalty` slope) + clean read-only `ScoreRow`; drop deprecated metrics (`seat_to_rc`, `setback`) & alias soup (`~950 LOC`) | Replace oval/weights/floors/versatility/breakdown/pruning with 2×2 hardware fit solve + range flags (`~200 LOC`, or drop `scoring` & `curve`) |
| *Subset of `pkg/geometry` above, not added to the total:* **Posture model (`geometry.go` L795–991)** | **~200** | Report a labelled `RearCenter / Wheelbase` ratio (`~10 LOC`); see C5 | Same (`~10 LOC`) |
| **`pkg/fit`** | **1,193** | Keep `fit` CLI; share unified closure with `validator` + replace 330-line string alias `ApplyAssignment` with typed `--try` keys (`~650 LOC`) | Per-size implied A2C + explicit candidate sweep (`~250 LOC`; edit compiled frame literal + run `validate`). Keep the "Allowance needed beyond rounding" line that the `GeoSlopMm` rule cites, and rewrite the ingestion workflow first (§4) |
| **`pkg/compare` + `pkg/cli`** | **2,426** (`1,752` + `674`) | Replace `-2` / `-fenrir` flag reordering hack (`reorderFlagsForStdlib`) with `drop:` syntax; unify 1-pass scene build (`~1,500 LOC`) | Drop composite breakdown/head-to-head/sag pruning; one table shape (collapse the `Transposed` / `Flat` / `Detail` layout variants in `compare/document.go`, each of which multiplies test surface); explicit-key specs; evaluate `--format=json` of `BikeScene` for CLI agents (`~700 LOC`) |
| **`pkg/web` (10 routes)** | **2,817** | Keep routes, fold `/saddles` + `/saddle-swap` (~809 LOC) into one Workbench + add `/bike` SVG + tweaker (`~2,200 LOC`) | Fold catalog/swap/bar sub-pages (`/saddles`, `/saddle-swap`, `/handlebars`, `/stems`, `/forks`, `/ideals`) into `/bike` selectors + ghost overlay; keep `/`, `/bike`, `/validate`, `/frames`, `/help` (`~1,000–1,200 LOC` incl. SVG) |
| **Other (`pkg/validator`, `pkg/catalog`, `pkg/geom2d`, `pkg/report`, `cmd/`)** | **2,712** (`1,455` + `717` + `379` + `150` + `11`) | Deduplicate `residuals.go` vs `budget.go`, remove unused `geom2d` helpers (`~2,200 LOC`) | Optional `Wheelbase`, declared `ChartPrecision`, deduped `validator` + `budget` (`~1,600–1,800 LOC`) |
| **Total Non-Frame Go** | **16,036** | **~8,500–9,500 LOC** *(-40–47%)* | **~4,800–5,500 LOC** *(-65–70%)* |

**What to explicitly keep unchanged or intact**:
- Pure-Go frame files in `pkg/frames` (**2,024 LOC**) + `TestAllFrameData` + `GeoSlopMm` discipline.
- Rounding-budget validation & `TestDeclaredAllowancesAreNecessary`.
- Bent seat-tube geometry, adjustable chainstays, and per-column `frame + size + fork + sag` specs.
- Web 1.0 discipline (URL query state, `GET` forms, no JS, `goquery` tests) + localhost/host guard + zero non-stdlib runtime dependencies.

*(One strategic CLI question worth deciding: since [docs/WEB.md](docs/WEB.md) and `AGENTS.md` designate the CLI for agents and scripts, emitting structured JSON of `BikeScene` + the hardware fit solve makes agent consumption and golden tests dramatically simpler than Markdown tables, and could let `report.Document` simplify to just the web presentation layer.)*

### 2. Nine Implementation Quality Principles ("Obviously Correct" Code)

1. **Errors, never silent fallbacks**: Remove every invented fallback (`1150`, `430/440`, `120`, `44`, `hbDropMm == 0 → 13.0`); constructors validate up front so downstream geometry functions operate only on verified inputs.
2. **One home for each domain fact**: `RiderFit` owns fit targets; `Catalog` owns fork `TravelMm` and `StaticA2c`; `BuildScene` owns kinematics; `EffectiveSta` exists in one function; sizes live in one ordered slice rather than a `Sizes` slice + `Geometries` map that can drift.
3. **Types over string parsing**: Replace attribute alias matching (~55 strings in `LookupAttribute`), `ForkTravel()` substring splitting, and `IsRigidFork` display parsing with typed fields and `exhaustive`-checked enums.
4. **Declare precision and provenance, don't infer from floats**: Add optional declared `ChartPrecision` on `FrameModel` and treat `Wheelbase` as optional when manufacturers omit it.
5. **State every normalization on the page**: Extend the comparison header's seatpost disclosure to state `HeadsetTopStackMm`, lower cup behavior (`CadLowerCupMm` vs `PhysicalLowerCupMm`), wheel radius (`372 mm`), grip inset (`65 mm`), and cockpit target convention (`TargetRef`: `TargetHandGrip`, `TargetUnsweptBarTop`, `TargetStemClamp`, or `TargetReferenceBarClamp`).
6. **Registry-wide invariants before formatted-string tests**: Keep the 4 property tests on `AllFrames()` as permanent guardrails against C3, C4, and C6 regressions.
7. **Deterministic output everywhere**: Sort map keys before iterating (`FrameValidationReport.ErrorSummary()`, `fit` candidate tie-breaking).
8. **Literal names matching physics—retire `HB` / `HX` / `HY` and require explicit point types on every value**: Never overload `HB`/`HX`/`HY` across stem clamps, unswept bar tops, and grips. Every cockpit coordinate, distance, drop, or target in code must explicitly declare what physical point it is (`StemClamp`, `UnsweptBarTop`, `HandGrip`, or `ReferenceBarClamp` via `TargetRef`), rename `handlebar_clamp_fit` to `grip_fit` (or remove via fit solve), rename `"Seatpost Ext"` to `BB-to-BRP minus SeatTube`, and make `"Oval Fits"` report what is graded.
9. **Enforce dead-code cleanliness**: Delete unreferenced exports and add `go tool deadcode ./...` to `make check`.

### 3. Granular Commit-by-Commit Execution Plan (23 Atomic Commits Across 5 Phases)

Every commit below has a single responsibility, is independently reviewable, and passes `make pre-commit` (which runs `go fix`, `go fmt`, `go vet`, `exhaustive`, and `go test`) before committing. Refactors are strictly separated from behavior-changing fixes so test diffs only appear on commits that intentionally change an equation or label. Any commit that changes a rule in [AGENTS.md](AGENTS.md) or [docs/WEB.md](docs/WEB.md) amends that document in the same commit (§4).

**Resolved pre-implementation decisions**:
- **C1 / Explicit Cockpit Point Typing**: Every cockpit coordinate, distance, drop, or target in code must explicitly state what physical point it represents (`StemClamp`, `UnsweptBarTop`, `HandGrip`, or `ReferenceBarClamp` via `TargetRef`)—no bare or implicit `HB` / `HX` / `HY` values are permitted anywhere. All four `TargetRef` modes remain supported so readings 1–3 can be evaluated and compared explicitly while C1 is verified with Seven and the fitter.
- **C4 (Lower Headset Cup Policy)**: Apply `PhysicalLowerCupMm - CadLowerCupMm` (`HeadsetCupType.CupStackMm() - GeoHeadsetSpacerMm`) in `compare`, keeping the validator warning accurate and updating `TestAnalyzeBikeFit_NormalizesCadBaseline` in the same commit.
- **`AETT` Definition**: Keep `AETT` as a pure frame-only metric at radial `seatHeight` (`746 mm`) along the seat-tube axis (`SeatAxisPoint`), independent of saddle model, seatpost setback, and rail offset (preserving zero diff when `BikeScene` lands).
- **Scoring & Diet Strategy**: Execute Conservative Consolidation through Phase 4 (keeping the 0–100 continuous curve score alongside the closed-form 2×2 hardware solve), then decide in Phase 5 whether to retire composite scoring.

#### Phase 1: Provenance Docs, User-Facing Disambiguation & Guardrail Tests
1. **`docs: record C1 target convention and C4 lower-cup provenance`**
   - Document the four explicit `TargetRef` conventions (`TargetHandGrip`, `TargetUnsweptBarTop`, `TargetStemClamp`, `TargetReferenceBarClamp`) and C4 lower-cup verification notes in [pkg/ideal/readme.md](pkg/ideal/readme.md) and frame `Notes` ([seven_custom.go](pkg/frames/seven_custom.go), [otso_fenrir_stainless.go](pkg/frames/otso_fenrir_stainless.go), [salsa_fargo.go](pkg/frames/salsa_fargo.go)).
2. **`refactor(compare,web,ideal): clarify StemClamp vs HandGrip in user-facing labels`**
   - Rename CLI/Web table headers, footers, and attribute display labels from ambiguous `HB` / `HX` / `HY` to explicit `StemClamp` or `HandGrip` (updating golden/DOM assertions only). Zero math changes.
3. **`test(validator): add registry-wide geometric invariant property tests`**
   - Add `RigidDistancesInvariantUnderPitch`, `CADForkReproducesPublishedChart`, `CupAndA2cAreInterchangeable`, and `AxlesStayLevel` (with C3's known rake-swap delta temporarily asserted or skipped until Commit 6).

#### Phase 2: `BikeScene` Foundation & Isolated Correctness Fixes (One Fix per Commit)
4. **`feat(geometry): introduce explicit RiderFit/TargetRef and pure-2D BikeScene constructor`**
   - Add `pkg/geometry/scene.go` (`BuildScene`, `RiderFit` with required `TargetRef`, `FrameSize`, `BuildSpec`, `CockpitSpec`, `SeatSpec`) alongside existing code, plus unit tests verifying exact parity with `CalculateDynamicGeometry` on constant-rake setups. Zero existing callers changed yet.
5. **`refactor(geometry): route CalculateDynamicGeometry and cockpit points through BikeScene`**
   - Wire `AnalyzeBikeFitWithSaddle` to read `Reach`, `Stack`, `HTA`, `STA`, `BbDrop`, `BRP`, `StemClamp`, and `HandGrip` from `BikeScene` under existing baseline assumptions (`HeadsetTopStackMm = 0`, constant-rake pitch, `CadLowerCupMm` baseline). **Pure refactor: zero output change across all tests.**
6. **`fix(geometry): level both axles across fork rake changes (C3)`**
   - Switch `BikeScene` pitch leveling to include `Fork.Rake` (`theta = -atan2(FA_0.Y - RA_0.Y, FA_0.X - RA_0.X)`), enable strict `< 1e-6` in `AxlesStayLevel` across all frames × forks × sags, and update affected test expectations (`1.2–2.0 mm` Reach, `0.8–1.4 mm` Stack, `0.11°–0.18°` HTA).
7. **`fix(geometry): model upper HeadsetTopStackMm between head tube top and stem (C2)`**
   - Add `HeadsetTopStackMm` (`11 mm` default, disclosed in the comparison header) to `CockpitSpec` / `BikeScene` and update cockpit test expectations (`~3.8 mm` X, `~10.3 mm` Y).
8. **`fix(geometry): apply physical lower headset cup stack in comparisons (C4)`**
   - Pitch by `PhysicalLowerCupMm - CadLowerCupMm` (`HeadsetCupType.CupStackMm() - GeoHeadsetSpacerMm`) so `otso-fenrir-stainless` and `salsa-fargo` match the validator disclosure, and update `TestAnalyzeBikeFit_NormalizesCadBaseline` in the same commit.
9. **`feat(ideal,compare): wire explicit TargetRef modes into cockpit target evaluation (C1)`**
   - Require every cockpit target in `RiderFit` / `IdmatchProfile` to state its `TargetRef` (`TargetHandGrip`, `TargetUnsweptBarTop`, `TargetStemClamp`, `TargetReferenceBarClamp`) and disclose the active `TargetRef` in the comparison header.
10. **`refactor(validator,geometry): unify closure residuals on BikeScene (C6)`**
    - Route `validator.ValidateBikeGeometry`, `geometry.A2cBudget`/`FrontCenterBudget`/`WheelbaseBudget`, and `validator.SizeResiduals` through `BikeScene`'s `FA_chart` and `FA_decl` points. Zero change to frame pass/fail or `GeoSlopMm` allowances.

#### Phase 3: Deduplication, Metric Migration & Papercut Fixes
11. **`refactor(scoring,canonicalizer): remove duplicate 3-pass cockpit selection in ScoreRow (C7)`**
    - Make `ScoreRow` read the already-computed `CockpitFit` from `CanonicalComparisonRow` instead of mutating a throwaway copy. **Pure refactor: zero output change.**
12. **`refactor(geometry): normalize saddle fit once via SitBones and delete Translate*FromRef (C7)`**
    - Compute rail offset directly from `SitBones` in `BikeScene`, removing `TranslateSaddleHeightFromRef`, `TranslateHbDistanceFromRef`, `TranslateBrpFromRef`, `TargetBrpForPelvicSetback`, `TranslateClampOffset`, and scattered `746.0` / `219.0` / `292.0` literals.
13. **`fix(geometry,compare): replace phantom Fenrir posture model with RearCenter/Wheelbase ratio (C5)`**
    - Delete `GetDefaultBaselineBike` and `CalculateBaselinePosture` (~200 LOC), remove `Modeled Back Angle: 53.6°` from headers, and replace `Front Wt` with the labelled `RearCenter / Wheelbase` ratio.
14. **`refactor(geometry,model): unify HasBentSeatTube, ETT, AETT, and Trail on BikeScene`**
    - Consolidate the three `EffectiveSta` / `HasBentSeatTube` implementations and route `ETT`, `AETT` (at radial `H_frame = 746 mm`), `Trail`, and `Seatpost Ext` through `BikeScene` readouts. **Pure refactor: zero output change.**
15. **`fix(curve,model): fix StemPenalty C1 slope at u=1 and sort ErrorSummary keys (C11)`**
    - Fix `curve.StemPenalty` exterior derivative at `u = 1` ([curve.go](pkg/curve/curve.go#L145-L160)) and sort size keys in `FrameValidationReport.ErrorSummary()` ([bike.go](pkg/model/bike.go#L1465-L1475)).

#### Phase 4: Closed-Form 2×2 Fit Solve & Visual Workbench
16. **`feat(geometry,compare): add closed-form 2x2 linear fit solve alongside curve score`**
    - Implement the 5-line 2×2 solver `(StemLengthMm, SpacerMm)` for each explicit `TargetRef` and display the solved hardware alongside the 0–100 curve score on real comparisons (`seven-custom`, `fenrir`, `compare all`).
17. **`feat(web): add /bike route with Panel A full-bike inline SVG and zero-JS nudge links`**
    - Ship the ~300-line single-bike SVG view (`TargetPoint` with `TargetRef`, `StemClamp`, `HandGrip`, frame/fork/cockpit/saddle) and `GET` query nudges (`−10 −5 +5 +10`).
18. **`feat(web): add 5-stage arithmetic computation ledger to /bike and compare details`**
    - Render the 5-stage intermediate calculation table in `<details>`.
19. **`feat(web): add cockpit/saddle zoom panels and multi-bike ghost overlay (optional follow-ons)`**
    - Add Panels B and C and the `BB` / `SitBones` overlay toggle if still wanted after Panel A lands.

#### Phase 5: Input Tightening & Surface Diet (Each Amending `AGENTS.md` / `docs/WEB.md` in the Same Commit)
20. **`refactor(model,canonicalizer,ideal): replace string-parsed fork travel and attribute aliases with typed fields (C10)`**
    - Use `catalog.Fork` `TravelMm` / booleans instead of parsing `"_"` or `"rigid"` strings, and replace the ~55 string aliases in `LookupAttribute` with typed `MetricID` constants checked by `exhaustive`.
21. **`feat(model,validator,frames): support optional Wheelbase and declared ChartPrecision (C9)`**
    - Allow omitted `Wheelbase` on frames that do not publish it (`moots-mountaineer`, `seven-expedition-926d`, `seven-expedition-926f`), add optional declared `ChartPrecision` (`seven-custom`), and update [AGENTS.md](AGENTS.md).
22. **`refactor(geometry,geom2d): replace silent <=0 fallback defaults with explicit constructor errors (C8)`**
    - Add a shared complete test-fixture builder in `pkg/*/*_test.go` and remove the ~20 silent fallback defaults (`1150`, `430`/`440`, `120`, `44`, `hbDropMm == 0 → 13.0`).
23. **`refactor(web,compare,fit): execute chosen surface diet and add deadcode check to make check`**
    - Decide whether to retain or retire the composite 0–100 curve score after evaluating Phase 4, fold redundant web sub-pages into `/bike`, update [AGENTS.md](AGENTS.md) and [docs/WEB.md](docs/WEB.md), delete unreferenced exports, and add `go tool deadcode ./...` to `make check`.

### 4. Repository Rules This Plan Changes

Several proposals contradict rules that [AGENTS.md](AGENTS.md) states today. Each rule is deliberate, so the step that breaks one amends it in the same change.

| Proposal | Rule it contradicts | Document to amend |
| --- | --- | --- |
| `--format=json` (or a JSON `BikeScene` dump) and CLI `--trace` | "There is no `--format`: Markdown is the only output" | AGENTS.md |
| Folding `/saddles`, `/saddle-swap`, `/handlebars`, `/stems`, `/forks`, `/ideals` into `/bike` | The CLI/Web capability table | AGENTS.md, [docs/WEB.md](docs/WEB.md) |
| Radical path removes scoring, and possibly `pkg/curve` | "Score Curves Live in `pkg/curve`" | AGENTS.md |
| Shrinking `pkg/fit` to a per-size sweep | "Use `bikegeo fit` as Primary Tool & Automated Solver", and the `ingest-frame` skill, both of which rely on the automatic candidate solver | AGENTS.md, `.agents/skills/ingest-frame/SKILL.md` |
| Declared `ChartPrecision`; optional `Wheelbase` | "Precision is inferred per chart column"; Wheelbase listed among the published ground-truth numbers | AGENTS.md |
| Explicit-key specs without `+spec` / `-spec` / `-col_num` | "Default Comparison Baseline" | AGENTS.md |
| `StemClamp` / `HandGrip` naming and `TargetRef` | [readme.md](pkg/ideal/readme.md#L15) defines `HB` as the stem clamp, then uses `HbX/HbY` for the hands at [L80–L87](pkg/ideal/readme.md#L80-L87) | pkg/ideal/readme.md |

---

## Part V: Provenance, Reproduction & Limits

### Where the Numbers Come From

- **[SIMPLIFY_C.md](SIMPLIFY_C.md)'s probe programs** (two throwaway Go programs kept outside the repository; see its §8): the C3 table, the C4 table, `SystemCgX = −13.60 mm`, and the statement that constant-rake pitch matches an independent 4-point construction to `0.01 mm`. They are reproduced here unchanged and were not re-run for this document.
- **Re-run on 2026-11-14 at `db9e574`** with the commands below:
  - the three-bike comparison: scores `23 / −9 / −15`, `BRP–HB` of `660 / 637 / 637`, and `Modeled Back Angle: 53.6°` in the header;
  - `compare all`: 11 of 19 frames at the `100 mm` stem, 11 of 19 with `0` in the oval, `seven-custom` at `26 (#11)`;
  - the default comparison: 20 rows, of which 4 are Fenrir and 4 are Fargo;
  - per-package line counts in Part IV;
  - the code citations for C2, C4, C6, C10 and the `StemPenalty` formula.
- **Computed by hand, not by code**: the fit-solve example in Part II §3 and the required-clamp coordinates in the C1 decision table.
- **Not in the repository**: the idmatch session details (straight test bar, marker at the grips), Seven's fit-sheet values (`74.6 cm`, `7.6 cm`, `24.5 cm`, `3.7 cm`, `80.3 cm`), and the statement that `HX 481mm, HY 727mm` was added during AI coding. A search of the tree finds none of them, and the frame `Notes` still present `HX`/`HY` as CAD-preview coordinates.

### Commands

```bash
go run ./cmd/bikegeo compare fenrir:L@rigid 'fenrir:L@120mm@20%' seven-custom
```

```bash
go run ./cmd/bikegeo compare all
```

```bash
go run ./cmd/bikegeo compare
```

```bash
go run ./cmd/bikegeo fit seven-custom
```

```bash
go run ./cmd/bikegeo fit otso-fenrir-stainless
```

### Not Verified

- Which handlebar point the idmatch report names (C1), and the geometry of the bar on the fit bike (needed for reading 3).
- Whether the Fenrir and Fargo charts are drawn to a bare head tube (C4).
- The actual upper headset stack on `seven-custom` (C2).
- Catalog values (fork A2C figures, saddle and bar dimensions) against manufacturer data.
- The web pages in a browser.
- The dead-code list, which is from `grep` and not a call-graph tool.
- The literal-occurrence counts and fallback line numbers in C7 and C8, beyond a spot check.
- The line-count targets in Part IV, which are estimates.
