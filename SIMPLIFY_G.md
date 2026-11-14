# Architecture, Simplification & Transparency Assessment (`bikegeo`)

> **Scope of this report**: Read-only technical assessment of the `bikegeo` codebase (98 Go files totaling 27,253 lines, of which 2,412 lines are compiled frame datasets in `pkg/frames/*.go`, plus 4 HTML templates and 1 CSS stylesheet). No source code was modified.

---

## Executive Summary & Architectural Diagnosis

### What `bikegeo` Gets Right Today

`bikegeo` has unusually disciplined architectural foundations that make simplification very achievable:

1. **Zero external runtime dependencies**: The shipped binary uses only the Go standard library (`flag`, `encoding/csv`, `net/http`, `html/template`, `embed`), compiled frame/component structs, and two narrow dev/test dependencies (`goquery` in tests, `exhaustive` in `make check`).
2. **Chart immutability & honest rounding budgets**: [budget.go](file:///Users/matt/src/bikegeo/pkg/geometry/budget.go) propagates per-column rounding half-widths coherently across the Stack, Front Center, and Wheelbase closure equations rather than using ad-hoc fudge factors.
3. **Continuous, flat-free scoring shapes**: [curve.go](file:///Users/matt/src/bikegeo/pkg/curve/curve.go) enforces strictly decreasing piecewise-linear + hyperbolic falloff curves (`Falloff`, `Saturating`, `StemPenalty`), eliminating score ties and discontinuous steps.
4. **Presentation-neutral intermediate document**: [report.go](file:///Users/matt/src/bikegeo/pkg/report/report.go) cleanly decouples computation builders from both the CLI Markdown renderer and the zero-JS Web 1.0 HTML templates ([views.go](file:///Users/matt/src/bikegeo/pkg/web/views.go), [compare.html](file:///Users/matt/src/bikegeo/pkg/web/templates/compare.html), [page.html](file:///Users/matt/src/bikegeo/pkg/web/templates/page.html)).

### Where Complexity & Subtle Bugs Have Accumulated

The core domain of `bikegeo` is **2D rigid-body kinematics in the sagittal plane**: roughly 10 named points `(X, Y)` per bike build (Rear Axle, Bottom Bracket, Front Axle, Head Tube Bottom/Top, Seatpost Clamp, Saddle Rail Center, Sit Bones, BRP, Stem Clamp, Hand Grip). On paper, all of that geometry takes ~250 lines of straightforward vector math.

In the current codebase (~24,841 non-frame Go lines), however, that core math has spread across ~7,500 lines in [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go) (1,611 lines), [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go) (1,305 lines), [canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go) (767 lines), [validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go) (958 lines), [residuals.go](file:///Users/matt/src/bikegeo/pkg/validator/residuals.go) (153 lines), [budget.go](file:///Users/matt/src/bikegeo/pkg/geometry/budget.go) (263 lines), [saddle.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle.go) (306 lines), [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle_swap.go) (340 lines), [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go) (682 lines), and [registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go) (441 lines).

Three structural habits drive most of the accidental complexity and several correctness bugs:

1. **Scalar coordinate projections instead of one 2D scene**: Functions re-derive individual scalar projections (`ActiveReach`, `ActiveStack`, `Aett`, `ActualSetback`, `SpareVerticalStack`, `BrpSetbackMm`, `HbX`, `HbY`) piecemeal across disconnected call sites instead of building one immutable 2D `BikeScene` once per setup.
2. **Coordinate-system indirection through BRP instead of anatomical invariants**: Targets are authored in the 70 mm Biomechanical Reference Point (`BRP`) frame of one reference saddle (`sqlab-610-ergolux`) and then repeatedly translated back and forth across packages whenever another saddle is chosen.
3. **Multi-pass mutation**: `CanonicalizeBikes` computes dead-center BRP/HB and an unused `BaselineMetrics` struct, `ApplyIdealCockpitTargets` overwrites `SelectedBRP`, `PossibleHBs`, `CockpitFitResult`, and `SelectedHB`, and `ScoreRow` duplicates that exact overwrite block a third time.

The rest of this report documents **6 concrete correctness defects** observed in the current code, proposes an **"obviously correct" unified 2D model**, identifies **~40% of non-frame code to cut or merge**, and specifies a **zero-JS inline SVG side-view workbench and step-by-step computation ledger** so every calculation is visually self-verifying.

---

## Part I: Concrete Correctness Bugs & "Not Obviously Correct" Inconsistencies

### Bug 1: The "Phantom Fenrir" Posture & Weight Distribution Bug

- **Where**:
  - [profiles.go](file:///Users/matt/src/bikegeo/pkg/catalog/profiles.go#L7-L19) (`DefaultRiderProfile`)
  - [loader.go](file:///Users/matt/src/bikegeo/pkg/ideal/loader.go#L11-L16) (`IdmatchProfile`) & [run.go](file:///Users/matt/src/bikegeo/pkg/compare/run.go#L218-L220)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L818-L991) (`GetDefaultBaselineBike`, `CalculateBaselinePosture`, `CalculateWeightDistribution`)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1132-L1168) (`AnalyzeBikeFitWithSaddle`)
- **What happens**:
  1. `DefaultRiderProfile()` sets `SaddleHeightMm = 780.0` (an Otso Fenrir L reference build with a 110 mm `-6°` stem and 30 mm saddle setback offset), while `compare.Run` overwrites `state.SaddleHeightMm = 746.0` from `IdmatchProfile()` ([run.go](file:///Users/matt/src/bikegeo/pkg/compare/run.go#L218-L220)).
  2. Inside `AnalyzeBikeFitWithSaddle`, lines 1133–1134 ignore the bike being analyzed completely and compute rider posture on a hardcoded phantom bike:
     ```go
     baselineBike := GetDefaultBaselineBike() // hardcoded Otso Fenrir L (Reach 428, Stack 642, HTA 68.25°)
     posture, err := CalculateBaselinePosture(baselineBike, profile)
     ```
  3. `CalculateBaselinePosture` computes `BackAngleDeg` and `SystemCgX` on that phantom Fenrir using `profile.SaddleHeightMm` (now `746.0`, paired with Fenrir's old `780 mm` baseline setback/stem assumptions), and `CalculateWeightDistribution` uses that fixed `posture.SystemCgX` and `posture.BackAngleDeg` for *every* bike in the comparison.
- **Impact**:
  - Every row in a comparison reports the **exact same `Modeled Back Angle`** (`backangle`) regardless of frame stack, reach, STA, stem length, spacers, or handlebar rise/sweep.
  - `WeightDistFrontPct` (`frontwt`, weight 4 in `IdmatchProfile`) claims to model combined rider + bike weight distribution, yet **changing a bike's stem, spacers, handlebar, STA, or saddle has zero effect on `SystemCgX`**—`frontwt` only varies with `(RearCenterCalc + constant) / WheelbaseCalc`.

### Bug 2: `HeadsetCupType` Is Promised in Validation Warnings but Never Used in Dynamic Pitching

- **Where**:
  - [validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L200-L213) (`geo_spacer_cup_mismatch` warning)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1024-L1091) (`AnalyzeBikeFitWithSaddle`)
  - [otso_fenrir_stainless.go](file:///Users/matt/src/bikegeo/pkg/frames/otso_fenrir_stainless.go#L12-L20) & [salsa_fargo.go](file:///Users/matt/src/bikegeo/pkg/frames/salsa_fargo.go#L12-L20) (`HeadsetCupType: CupEC44`, `GeoHeadsetSpacerMm: 0`)
  - [geometry_test.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry_test.go#L1077-L1104) (`TestAnalyzeBikeFit_NormalizesCadBaseline`)
- **What happens**:
  - When a frame declares `HeadsetCupType: CupEC44` (`CupStackMm() == 12.0` in [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go#L25-L36)) alongside `GeoHeadsetSpacerMm: 0` (such as `otso-fenrir-stainless` or `salsa-fargo`), [validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L209) emits a warning asserting:
    > *"Validation uses geo_headset_spacer_mm to verify the published chart, while comparisons pitch the frame using the physical headset_cup_type stack."*
  - **The comparison code does the exact opposite.** In `AnalyzeBikeFitWithSaddle` ([geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1061-L1069)):
    ```go
    geoSpacer := bike.GeoHeadsetSpacerMm
    ...
    baselineStructuralA2c = bike.CadFork.A2cMm + geoSpacer
    totalNewA2c := tA2c + geoSpacer
    ```
    `geoSpacer` is added to *both* `baselineStructuralA2c` and `totalNewA2c`, so it cancels out identically (`deltaA2c = tA2c - bike.CadFork.A2cMm`), and `bike.HeadsetCupType.CupStackMm()` is never called anywhere in `pkg/geometry` or `pkg/canonicalizer`.
  - Worse, `TestAnalyzeBikeFit_NormalizesCadBaseline` explicitly asserts `fit.DeltaAngleDeg == 0` when evaluating Fenrir XS (`CupEC44`, `GeoHeadsetSpacerMm: 0`) at its `495 mm` CAD fork—locking in the contradiction with `validator.go`!
- **Impact**: If a manufacturer's Stack chart genuinely omitted an external 12 mm lower cup (`GeoHeadsetSpacerMm: 0`, `CupEC44`), `compare` silently omits that 12 mm cup too—leaving the front end 12 mm lower and ~0.5° steeper than the physical build. Conversely, if the manufacturer's 495 mm fork figure already baked in the lower cup interface, `validator.go` emits a misleading promise that `compare` does not keep. The data model needs to distinguish explicitly between "cup stack included in published CAD Stack" and "physical cup stack on the built bike" rather than contradicting itself across packages.

### Bug 3: Fork Rake Changes Are Ignored When Pitching the Frame (`CalculateDynamicGeometry`)

- **Where**:
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L714-L767) (`CalculateDynamicGeometry`)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L770-L775) (`CalculateFrontCenter`)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1080-L1168) (`AnalyzeBikeFitWithSaddle`)
- **What happens**:
  1. `CalculateDynamicGeometry` accepts `baseStructuralA2c` and `newStructuralA2c` (`deltaA2c`), but **does not accept `baseRake` or `targetRake` at all**.
  2. When comparing a frame with a non-CAD fork rake—for example, `otso-fenrir-stainless` moving from its CAD rigid fork (`495 mm` A2C, `54 mm` rake) to `fox34_120` (`530 mm` static, `44 mm` rake)—`ActiveReach`, `ActiveStack`, `ActiveHta`, `ActiveSta`, and `ActiveBbDrop` are computed as if fork rake were still `54 mm`.
  3. Only later at line 1158 does `AnalyzeBikeFitWithSaddle` pass `rake` (`44 mm`) to `CalculateWeightDistribution` → `CalculateFrontCenter(activeReach, activeHtl, activeHta, rake, activeStructuralA2c)`.
- **Impact**:
  - Because the rake vector `u_rake = (sin(HTA), cos(HTA))` has a vertical component `cos(HTA)`, reducing rake from `54 mm` to `44 mm` at `68.25°` HTA drops the front axle relative to the steerer by `-10 × cos(68.25°) = -3.71 mm`, which pitches the frame forward by ~0.18°, slightly steepening HTA/STA, increasing Reach, lowering Stack, and increasing BB drop.
  - Because `AnalyzeBikeFitWithSaddle` never pitches the frame for `ΔRake`, `ActiveStack`, `ActiveHta`, `ActiveBbDrop`, `activeStructuralA2c`, and `rake` **no longer satisfy the Stack closure equation** (off by `ΔRake × cot(HTA) ≈ 4.0 mm` for Fenrir).

### Bug 4: Closure Equation Mismatch Between `validator/residuals.go` and `geometry/budget.go`

- **Where**:
  - [residuals.go](file:///Users/matt/src/bikegeo/pkg/validator/residuals.go#L87-L123) (`SizeResiduals`)
  - [budget.go](file:///Users/matt/src/bikegeo/pkg/geometry/budget.go#L187-L257) (`impliedFrontCenterHoriz`, `A2cBudget`, `FrontCenterBudget`, `WheelbaseBudget`)
  - [validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L294-L320) (`ValidateBikeGeometry`)
  - [fit.go](file:///Users/matt/src/bikegeo/pkg/fit/fit.go) & [document.go](file:///Users/matt/src/bikegeo/pkg/fit/document.go)
- **What happens**:
  - `FrontCenterBudget`, `WheelbaseBudget` ([budget.go](file:///Users/matt/src/bikegeo/pkg/geometry/budget.go#L190-L196)), and `ValidateBikeGeometry` ([validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L294-L311)) compute horizontal front-center from **Stack-implied structural A2C** (`frontEnd = frontEndLength(Stack, BbDrop, HtaDeg, Rake)`), where `HTL`, `CadFork.A2cMm`, and `GeoHeadsetSpacerMm` cancel out completely.
  - Meanwhile, `SizeResiduals` ([residuals.go](file:///Users/matt/src/bikegeo/pkg/validator/residuals.go#L89-L92)) deliberately swaps `structuralA2c` for `evalStructuralA2c = expectedForkA2c + geoSpacer` (the *declared* fork A2C + spacer):
    ```go
    evalStructuralA2c := structuralA2c
    if expectedForkA2c > 0 {
        evalStructuralA2c = expectedForkA2c + geoSpacer
    }
    ```
- **Impact**: In `bikegeo fit`, the table displays `FC calc` / `ΔFC` and `WB calc` / `ΔWB` evaluated with `SizeResiduals` (using the declared fork A2C), right alongside the pass/fail budget report (`FrontCenterBudget`, `WheelbaseBudget`) evaluated using the Stack-implied A2C. Sweeping `--try fork.a2c=505` changes `ΔFC` and `ΔWB` in the table while `FrontCenterBudget` and `WheelbaseBudget` remain completely invariant to `fork.a2c` and `spacer`. Two functions reporting residuals for the same frame should never use two different equations.

### Bug 5: Duplicated, State-Mutating Cockpit Selection & Hardcoded Fit Constants

- **Where**:
  - [canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L651-L767) (`CanonicalizeBikes`, `ApplyIdealCockpitTargets`)
  - [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L70-L190) (`translateAttributeSpecForSaddle`, `ScoreRow`)
  - [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1021-L1201) (`AnalyzeBikeFitWithSaddle`)
- **What happens**:
  1. `CanonicalizeBikes` runs `AnalyzeBikeFitWithSaddle` (populating `PossibleHBs` around `DeadCenterBRP` and selecting `SelectedHB` at dead center).
  2. Next, `ApplyIdealCockpitTargets` optimizes `SelectedBRP` against the `brp_setback` target, recomputes `PossibleHBs` around `SelectedBRP`, and overwrites `SelectedHB` and `CockpitFitResult`.
  3. Next, `ScoreRow` contains an **~75-line copy-pasted duplicate** of `ApplyIdealCockpitTargets` ([scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L115-L189)) that mutates a value copy of `row.Canonical` a third time before calling `ideal.Evaluate` (whose `translateAttributeSpecForSaddle` repeats the saddle translation a *fourth* time!).
  4. Hardcoded constants from one rider's idmatch PDF (`746.0` saddle height, `219.0` BRP setback, `292.0` sit-bone setback, `3.0` default rise) are baked directly into core geometry and scoring functions ([geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1022), [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1201), [canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L721-L734), [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L87), [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L149-L162)).
  5. Meanwhile, `IdealStaDeg` (`ideal_sta`) is computed inside `AnalyzeBikeFitWithSaddle` using `DefaultSeatpostSetbackMm` (`11.0`) and `292.0`, regardless of what ideal profile or seatpost is active.
  6. Finally, [eval.go](file:///Users/matt/src/bikegeo/pkg/ideal/eval.go#L51-L86) special-cases `handlebar_clamp_fit` so the table prints an integer count (`CountInside`, e.g. `6 ✓`) while grading and ranking on a continuous float (`FinalSubScore = BaseScore + VersatilityBonus - StemPenalty`). Two bikes can both display `4 ✓` or `0 ✗` in the main table yet have substantially different scores.

### Bug 6: Guessing Fork Travel by Splitting Fork ID Strings

- **Where**: [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go#L1007-L1023) (`ForkRef.ForkTravel`, `ForkRef.StaticA2c`)
- **What happens**:
  ```go
  func (r ForkRef) ForkTravel() float64 {
      parts := strings.SplitSeq(r.Fork, "_")
      for p := range parts {
          if v, err := strconv.ParseFloat(strings.TrimSuffix(p, "mm"), 64); err == nil && v >= 60 && v <= 200 {
              return v
          }
      }
      return 0.0
  }
  ```
- **Impact**: Suspension travel is a physical property, not a substring convention. Any gravel suspension fork with 40 mm or 50 mm travel (`< 60`) is misclassified as a rigid fork (`0 mm` travel, `0%` sag), and any fork whose model name contains an underscored number in `[60, 200]` (such as a tire clearance or stanchion/year code) has its travel and `StaticA2c()` silently miscalculated. Fork entries in [catalog.go](file:///Users/matt/src/bikegeo/pkg/catalog/catalog.go) already carry `TravelMm`; `ForkRef` should either store `TravelMm` explicitly or resolve it from the catalog.

---

## Part II: Radical Simplification of the Math & Coordinate Model ("Obviously Correct" by Construction)

### 1. The Unified 2D `BikeScene` Rigid-Body Model (~250 Lines Total)

Almost every bug and redundancy above vanishes if `pkg/geometry` builds **one explicit 2D sagittal scene** (`BikeScene`) per setup in a single pure pass. Every geometry metric then becomes a 1-line vector difference, distance, or angle between two named `geom2d.Point2` landmarks—impossible to desynchronize.

Using standard bicycle coordinates with the **Bottom Bracket at `(0, 0)`**, `+X` forward toward the front wheel, `+Y` upward:

#### Step 1: Frame Landmarks in Published CAD Attitude

- `BB = (0, 0)`
- `RearAxle = (-√(Chainstay² - BbDrop²), BbDrop)` *(positive `BbDrop` means axles sit above the BB)*
- `HeadTubeTop = (Reach, Stack)`
- Steerer unit vector pointing **down-and-forward** (`u_steer_down`) and rake unit vector pointing **forward-perpendicular** (`u_rake`):
  - `u_steer_down = (cos(HTA), -sin(HTA))`
  - `u_rake = (sin(HTA), cos(HTA))`
- `HeadTubeBottom = HeadTubeTop + HTL × u_steer_down`
- **Seat tube axis**:
  - Straight seat tube: ray from `BB` in direction `(-cos(STA), sin(STA))`.
  - Bent seat tube (`SeatTubeOffsetXMm` or `ActualStaDeg`): offset start `(SeatTubeOffsetXMm, 0)` with physical direction `(-cos(ActualStaDeg), sin(ActualStaDeg))`, intersected cleanly instead of approximate Law-of-Sines rewrites, plus exact `SeatTubeTop` at `SeatTubeLength`.

#### Step 2: Exact Rigid-Body Pitching for Any Fork `(A2C_active, Rake)` & Lower Cup `CupStack`

No small-angle approximations, no omitted rake coupling, and no two-pivot drift (`CalculateDynamicGeometry`):

1. Using the CAD frame's rigid coordinates, attach the lower cup (`CupStack`) and active fork (`A2C_active = A2C_static - Travel × Sag`, `Rake`):
   ```text
   ForkCrown_raw  = HeadTubeBottom + CupStack × u_steer_down
   FrontAxle_raw  = ForkCrown_raw + A2C_active × u_steer_down + Rake × u_rake
   ```
   *(When comparing at the published CAD baseline `A2C_active + CupStack == StructuralA2c_CAD` and `Rake == Rake_CAD`, `FrontAxle_raw.Y` equals `BbDrop` up to rounding, so `Δθ == 0` automatically.)*
2. Equal wheel radii mean the ground plane is parallel to the axle line `v_wb = FrontAxle_raw - RearAxle`. Bring the axle line back to horizontal by pitching the entire rigid assembly around `RearAxle`:
   ```text
   Δθ = -atan2(v_wb.Y, v_wb.X)   // positive pitches nose up (slackens HTA & STA)
   ```
   Rotate `RearAxle`, `BB`, `HeadTubeBottom`, `HeadTubeTop`, `ForkCrown`, `FrontAxle`, `SeatTubeTop`, and the seat-tube axis around `RearAxle` by `Δθ`, then translate all points by `-BB_rotated` so `BB` sits back at `(0, 0)`.

That 12-line rigid transform gives **exact** active `Reach`, `Stack`, `HTA = HTA_cad - Δθ` *(note sign convention: pitching nose up decreases acute HTA measured from horizontal)*, `STA`, `BbDrop = RearAxle.Y`, `FrontCenter = FrontAxle.X`, and `Wheelbase = FrontAxle.X - RearAxle.X`—and **automatically preserves exact geometric closure for any fork A2C, sag, headset cup, or fork rake change**.

#### Step 3: Rear Cockpit (Seatpost + Saddle) in `BikeScene`

- Solve the exact intersection of the active seat-tube ray + perpendicular `SeatpostSetbackMm` offset with the saddle rail clamp line so that `SitBones` sits at the rider's anatomical vertical height (or radial leg extension).
- Move fore-aft along the saddle rail window `[-HalfWindow, +HalfWindow]` by `RailOffsetMm` to obtain:
  - `SeatpostClamp`
  - `RailCenter`
  - `SitBones = RailCenter + (RailCenterToSitBonesMm, SitBonesElevationMm)`
  - `BRP = RailCenter + (RailCenterToBrpMm, 0)`

#### Step 4: Front Cockpit (Spacers + Stem + Handlebar) in `BikeScene`

- `SteererClamp = HeadTubeTop - (SpacerMm + StemThicknessMm/2) × u_steer_down` *(upward along steerer)*
- `StemClamp = SteererClamp + StemLengthMm × u_stem(HTA_active, StemAngleDeg)`
- `HandGrip = StemClamp + HandlebarOffsetXY(BarSpec, GripInsetMm, RollDeg)`

#### Step 5: Every Metric Is a 1-Line Readout from `BikeScene`

| Metric | 1-Line Definition on `BikeScene` | Replaces |
| --- | --- | --- |
| `Reach`, `Stack` | `HeadTubeTop.X`, `HeadTubeTop.Y` | `dyn.NewReach`, `dyn.NewStack` |
| `BbDrop` | `RearAxle.Y` *(verified `== FrontAxle.Y`)* | `dyn.NewBbDrop` |
| `FrontCenter`, `RearCenter`, `Wheelbase` | `FrontAxle.X`, `-RearAxle.X`, `FrontAxle.X - RearAxle.X` | `CalculateFrontCenter`, `RearCenterCalc`, `WheelbaseCalc` |
| `Trail` | `(WheelRadius × cos(HTA) - Rake) / sin(HTA)` | `CalculateTrail` |
| `ETT` | `HeadTubeTop.X - SeatAxis.AtY(HeadTubeTop.Y).X` | `CalculateEtt` |
| `AETT` | `SteererAxis.AtY(SitBones.Y).X - SeatAxis.AtY(SitBones.Y).X` | `CalculateAett` |
| `SitBoneSetback`, `BRPSetback` | `-SitBones.X`, `-BRP.X` | `CalculateBRPPoint`, `TranslateBrp*` |
| `SaddleHeightBRP`, `LegExtension` | `hypot(BRP.X, BRP.Y)`, `hypot(SitBones.X, SitBones.Y)` | `TranslateSaddleHeightFromRef` |
| `BrpToGripDist`, `GripDrop` | `HandGrip.DistanceTo(BRP)`, `HandGrip.Y - SitBones.Y` | `HBPoint.SaddleHbMm`, `SaddleHbDropMm` |
| `RailMarginMm` | `HalfWindow - abs(RailOffsetMm)` | Hidden inside feasibility booleans |

---

### 2. Sit-Bone-First & Grip-First Biomechanics (Eliminating the BRP Translation Labyrinth)

Today, `ideal.IdmatchProfile()` stores cockpit targets in the 70 mm BRP coordinate system of `sqlab-610-ergolux` (`SaddleHeightMm = 746.0`, `brp_setback = 219.0`, `saddle_hb = 695–705`, `saddle_hb_drop = -2 to 8`). Whenever another saddle (such as `berthoud-aspin-open` or `selle-italia-slr-boost-superflow-l3`) is evaluated, [canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L713-L735), [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L70-L95), [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L141-L163), [saddle.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle.go#L78-L83), and [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1021-L1022) translate those target numbers back to `sqlab-610` using hardcoded `746.0`, `219.0`, and `3.0` constants.

**The simpler invariant**: The rider's bones never touch the 70 mm BRP plastic underside caliper mark—their pelvis rests on the **Sit Bones** and their hands hold the **Hand Grips**. The idmatch session on `sqlab-610` is simply *one measurement* of those two anatomical coordinates in space:

1. **Convert idmatch → Anatomical Coordinates ONCE at profile load**:
   - From `(idmatch.BrpSetback = 219, idmatch.SaddleHeight = 746, sqlab-610)` →
     - `TargetSitBones = (-292.0, +723.1)` *(exact `SitBonePointFromBrp` including `sqlab`'s `+10 mm` Ergolux step!)*
   - From `(idmatch.SaddleHb = 700, idmatch.HbDrop = +3, ReferenceHandlebar)` →
     - `TargetHandGrip = (Brp_ref.X + √(700² - (TargetSitBones.Y + 3 - Brp_ref.Y)²), TargetSitBones.Y + 3)`
   *(Wait: note that `IdmatchProfile` line 26 specifies `Between(695, 705)` while `711.5` in early notes became `700` midpoint here—computing directly from `prof`'s fields guarantees zero drift between profile targets and geometry!)*
2. **Solve any saddle `S` directly from `TargetSitBones`**:
   - Place `SitBones` at `TargetSitBones`:
     ```text
     TargetRailCenter = TargetSitBones - (S.RailCenterToSitBonesMm, S.SitBonesElevationMm)
     ```
   - Intersect the frame's seat-tube + seatpost offset line at `Y = TargetRailCenter.Y` → `NominalClampX`.
   - Then `RequiredRailOffset = NominalClampX - TargetRailCenter.X` *(with sign convention matching positive clamp offset moving clamp forward on rails)*, clamped into `[-HalfWindow, +HalfWindow]`.
   - **Rail Margin** (`HalfWindow - |RequiredRailOffset|`) directly measures whether the bike can physically put the rider's pelvis in the right place!
3. **Measure Workshop BRP Numbers as Pure Outputs**:
   - Once the saddle sits at its solved rail offset, compute `BRP = RailCenter + (S.RailCenterToBrpMm, 0)` and report `BRP Setback = -BRP.X` and `BB-to-BRP Saddle Height = hypot(BRP.X, BRP.Y)` purely for the mechanic's tape measure and calipers.

**What this deletes**:
- Completely removes `TranslateSaddleHeightFromRef`, `TranslateHbDistanceFromRef`, `TranslateBrpFromRef`, `TargetBrpForPelvicSetback`, `TranslateClampOffset`, and `translateAttributeSpecForSaddle` (~200 lines across [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go), [saddle.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle.go), and [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go)).
- Completely eliminates hardcoded `746.0`, `219.0`, `292.0`, `3.0` from `pkg/geometry`, `pkg/canonicalizer`, and `pkg/scoring`.
- Guarantees by construction that two setups with different saddles are always compared at the **exact same pelvic and hand coordinates**.

---

### 3. Unifying the Three Closure Equations (`validator` & `budget`)

Extract the three chart closure equations into three pure functions shared identically by `pkg/validator` and `pkg/geometry/budget.go`:

1. **Vertical / Front-End Stack Closure (`A2C`)**:
   ```text
   ImpliedStructuralA2C = (Stack - BbDrop + Rake × cos(HTA)) / sin(HTA) - HTL
   ResidualA2c          = (DeclaredForkA2c + GeoHeadsetSpacerMm) - ImpliedStructuralA2C
   ```
2. **Horizontal Front-Center Closure (`FC`)**:
   - Use **one** canonical choice across both `SizeResiduals` and `FrontCenterBudget`—ideally **Declared Structural A2C** (`DeclaredForkA2c + GeoHeadsetSpacerMm`) or explicitly show *both* Stack-implied FC and Fork-implied FC—so that `--try fork.a2c=...` in `bikegeo fit` moves both `ΔFC` and `FrontCenterBudget` identically.
3. **Wheelbase Closure (`WB`)**:
   ```text
   WheelbaseCalc = RearCenterHoriz(Chainstay, BbDrop) + FrontCenterHoriz
   ResidualWb    = WheelbaseCalc - PublishedWheelbase
   ```

Making `residuals.go`, `budget.go`, and `validator.go` call the exact same three functions guarantees that `bikegeo validate` and `bikegeo fit` can never disagree.

---

### 4. Replacing the 150-Line Stick-Figure Posture Model with Explicit CoG

Delete `BiomechanicalPosture`, `GetDefaultBaselineBike`, `CalculateBaselinePosture`, and the torso/arm/SSS-triangle solver in [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L795-L959).

Two far simpler, honest options exist for `frontwt`:
- **Option A (Axle-Relative Balance from Actual Contact Points)**: Estimate rider CoG from the *actual* bike's solved `SitBones` and `HandGrip` points on `BikeScene`:
  ```text
  RiderCgX = 0.65 × SitBones.X + 0.35 × HandGrip.X   // transparent weighted centroid of contact points
  SystemCgX = (RiderMass × RiderCgX + BikeMass × 0) / (RiderMass + BikeMass)
  FrontWtPct = ((SystemCgX - RearAxle.X) / Wheelbase) × 100
  ```
  Now stem, spacer, handlebar, and saddle choices actually move `SystemCgX` on the bike being evaluated! And **modeled torso angle** (if desired at all) can be computed honestly from *that bike's* `SitBones` and `HandGrip`.
- **Option B (Simplest — Geometry-Only Axle Split)**: Drop biomechanical mass heuristics altogether and report `RearCenter / Wheelbase` (BB weight split %) directly, removing `RiderWeightLbs`, `BikeWeightLbs`, `HeightIn`, `InseamIn` anthropometric assumptions from `RiderProfile`.

---

## Part III: Feature Reduction & Codebase Diet (~40% Non-Frame LOC Reduction)

Cutting redundant abstractions, legacy compatibility paths, and peripheral sub-tools can shrink the ~24.8k non-frame Go lines by roughly **3,500–4,500 lines (~15–20% total, ~35–40% of core logic)** while making every remaining feature sharper:

| Area | Current State (Verified Files & LOC) | Proposed Reduction | Est. Savings |
| --- | --- | --- | --- |
| **1. `pkg/model/bike.go` & `pkg/geometry/geometry.go` bloat** | [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go) (**1,611** lines): duplicated accessors on `FrameModel` and `BikeGeometry` (`HasBentSeatTube`, `CalculateEffectiveSta`, `HasAdjustableChainstay`, `GetMaxSpacerMm`, `GetSpacersMm`, `GetCanonicalStem`), unused `ForkSpec`, dead `BaselineMetrics` (13 of 15 fields unused outside tests), string-parsed `ForkTravel()` | Split cleanly into `FrameModel` (chart + fork refs) + `BikeScene` (2D scene). Remove `BaselineMetrics`, `ForkSpec`, `CalculateBaselinePosture`, `GetDefaultBaselineBike`, and redundant `BikeGeometry` getter clones. | **~900–1,100 LOC** |
| **2. Three-pass canonicalize + score duplication** | [canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go) (**767** lines) + [scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go) (**682** lines) | Make `BuildScene(frame, size, setup, profile, overrides)` a single pure constructor that solves saddle rail offset + cockpit fit once; `ScoreRow` becomes read-only (no `can.SelectedBRP` / `PossibleHBs` copy-paste mutation!). Remove legacy 6-tier `SelectBestHBWithDrop` ([geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L321-L415)) so only `EvaluateCockpitFit` remains. | **~700–900 LOC** |
| **3. Saddle & Saddle-Swap subsystem** | [saddle.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle.go) (**306**), [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle_swap.go) (**340**), [saddles.go](file:///Users/matt/src/bikegeo/pkg/web/saddles.go) (**362**), [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/web/saddle_swap.go) (**447**) — **1,455 LOC total for 3 catalog saddles** ([saddles.go](file:///Users/matt/src/bikegeo/pkg/catalog/saddles.go))! | Replace with one compact **Saddle & Seatpost Workbench** powered directly by `BikeScene` (showing for each saddle: required rail offset, margin, workshop BRP height/setback, and ideal STA). Drop the 64-iteration minimax `"Universal Compromise STA"` solver and the 45-field `SaddleSwapReport`. | **~950–1,100 LOC** |
| **4. Ideal Registry & `handlebar_clamp_fit` opacity** | [registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go) (**441** lines): **26** entries (including deprecated `setback`, `seat_to_rc`, plus redundant `sparevert`/`spareaxial`), and **~40** string alias branches in `LookupAttribute` | Trim to canonical IDs (drop `setback`, `seat_to_rc` from both `registry.go` and `IdmatchProfile()`, drop `spareaxial` and multi-alias soup). Separate **Cockpit Error** (or `BestR` / `ΔReach, ΔStack`) cleanly from **Stem Versatility** so cell text matches its sub-score! | **~200–250 LOC** |
| **5. CLI spec grammar & flag reordering hack** | [specs.go](file:///Users/matt/src/bikegeo/pkg/compare/specs.go) (**412** lines), [prune.go](file:///Users/matt/src/bikegeo/pkg/compare/prune.go) (**183**), `reorderFlagsForStdlib` ([cli.go](file:///Users/matt/src/bikegeo/pkg/cli/cli.go#L155-L194)) | Replace single-dash `-2` / `-fenrir` removal tokens (which force `cli.go` to intercept single-dash flags before Go's `flag.FlagSet` runs) with explicit `drop:2` or `--drop=2`. Simplify modifier parsing so numbers `< 60` vs `>= 60` don't silently overload sag and fork travel. | **~200–300 LOC** |

### Two Specific Transparency Fixes in Scoring & CLI

1. **Make `handlebar_clamp_fit` match its own cell**: Currently [eval.go](file:///Users/matt/src/bikegeo/pkg/ideal/eval.go#L77) displays `"6 ✓"` (`CountInside`) while [registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go#L367-L371) scores `FinalSubScore` (`BaseScore + VersatilityBonus - StemPenalty`). That forces users to open the verbose loss text to learn *why* two frames with `"4 ✓"` differ in points, or why a setup with `CountInside = 0` scored higher than another. Displaying `"R=0.32 · 6in ✓"` or `"90mm -6° +15mm · R=0.32 (6in)"` makes the primary cell self-explanatory.
2. **Stop printing deprecated metrics by default**: `IdmatchProfile()` ([loader.go](file:///Users/matt/src/bikegeo/pkg/ideal/loader.go#L81)) still includes `DisplayOnly("seat_to_rc")` even though [registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go#L99-L100) explicitly marks it `Deprecated`. Removing deprecated metrics from default output declutters every comparison table immediately.

---

## Part IV: Computational Transparency & Interactive SVG Side-View Workbench

Because `bikegeo serve` renders static HTML + CSS directly from Go `html/template` with zero JavaScript ([docs/WEB.md](file:///Users/matt/src/bikegeo/docs/WEB.md)), **inline `<svg>` is a native zero-dependency visual layer**. Generating SVG straight from `BikeScene`'s millimeter `(X, Y)` coordinates makes every geometric equation *literally visible*—any sign error in HTA, fork rake, stem angle, bar roll, or saddle rail offset shows up instantaneously as a misaligned line on screen.

### 1. Zero-Dependency Server-Rendered `<svg>` Side-View Diagrams

SVG uses `+Y` downward while bicycle kinematics uses `+Y` upward. Wrapping the geometry group in `<g transform="scale(1,-1)">` lets Go templates emit `BikeScene` millimeter coordinates verbatim (`x1="{{printf "%.1f" .BB.X}}" y1="{{printf "%.1f" .BB.Y}}"`), while keeping crisp upright text annotations in a separate un-flipped overlay group.

Three coordinated SVG panels fit cleanly into [compare.html](file:///Users/matt/src/bikegeo/pkg/web/templates/compare.html) and [page.html](file:///Users/matt/src/bikegeo/pkg/web/templates/page.html):

1. **Panel A: Full-Bike Side View & Multi-Bike Ghost Overlay**
   - Draws ground plane (`Y = BbDrop - WheelRadius`), wheel circles, chainstay, seatstay, down tube, top tube, seat tube (showing any kinked segment explicitly), head tube, lower headset cup stack, fork blades (with crown-to-axle line + perpendicular rake offset visible), seatpost + setback head, saddle rail window + Sit Bones + BRP, spacer stack, stem, handlebar vector, and the `SitBones ↔ HandGrip` rider reach triangle.
   - **Multi-bike overlay on `/`**: Render the primary reference bike in solid stroke (`#2563eb`) and 1–3 selected comparison setups as semi-transparent color-coded overlays (`#dc2626`, `#059669`, `#d97706`), with a simple `GET` toggle to align either at **Bottom Bracket `(0, 0)`** (comparing frame geometry) or at **Sit Bones** (comparing pure rider posture and wheel base shift).
   - **On `/validate`**: Draw the single frame showing **both** the published CAD triangle (dashed gray) and the reconstructed closure lines from `(Reach, Stack, HTA, HTL, Rake, A2C)` so any chart discrepancy (`ΔA2C`, `ΔFC`, `ΔWB`) is drawn to scale right next to the residual table.

2. **Panel B: Front Cockpit Zoom Box (`~260 mm × 260 mm` around `HeadTubeTop`)**
   - Shows the steerer axis (`0` to `MaxSpacerMm`), **every candidate stem/spacer grip dot** evaluated in `PossibleHBs`, the selected stem/spacer polyline in bold, the handlebar vector (`StemClamp → HandGrip`), and the **30°-rotated `FitOval`** ([geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L418-L426), semi-axes `10 mm × 20 mm`) rendered as an SVG `<ellipse transform="rotate(30 ...)">`!
   - Immediately demystifies `handlebar_clamp_fit`: users see the exact stem/spacer grid falling inside or outside the green target oval and see why extreme stems trigger `StemPenalty`.

3. **Panel C: Rear Saddle & Seatpost Zoom Box (`~220 mm × 160 mm` around `SeatpostClamp`)**
   - Shows the active seat tube axis (`EffectiveSTA`), perpendicular `SeatpostSetbackMm` offset, usable straight rail window `[-HalfWindow, +HalfWindow]` highlighted in green, the solved clamp position, `RailCenter`, the step vector to `SitBones`, and the 70 mm `BRP` caliper mark.
   - Makes steep seat-tube vs. rail-margin feasibility (`rail_margin`) obvious at a glance.

---

### 2. Interactive Component Tweaker ("What-If Workbench")

Keeping strictly within `bikegeo`'s Web 1.0 architecture (`GET` forms, URL query state, zero JS, zero cookies):

- **URL Query Override Parameters**:
  - Front cockpit: `stem_len` (`auto` or `50..120`), `stem_ang` (`auto` or `-17..+35`), `spacer` (`auto` or `0..MaxSpacerMm`), `handlebar` (catalog ID), `bar_roll` (`-10..+15`), `bar_width` (`640..820`)
  - Rear cockpit: `saddle` (catalog ID), `post_sb` (`0, 7, 11, 15, 20, 25`), `rail_off` (`auto` or `-30..+30`)
  - Fork/sag: existing `fork` & `sag` controls
- **Auto vs. Pinned Comparison in SVG & Table**:
  - When any component is set to `auto`, the optimizer picks the best stem/spacer/rail position as today.
  - When the user tweaks a control to test real parts they own (e.g., `"What if I run my 80 mm +7° stem, 25 mm spacers, and a 20 mm setback post?"`), the SVG renders the **auto-optimal build as a dashed ghost** and the **tweaked build as solid**, and reports exact `ΔReach`, `ΔStack`, `ΔGripDist`, `ΔGripDrop`, `ΔRailMargin`, and `ΔScore`.
- **Zero-JS Quick-Step Links**:
  - Right beside each form control in the workbench HTML, render pure `<a>` step links built on the server from the current `url.Values`:
    - Spacers: `[-10 mm]` `[-5 mm]` **15 mm** `[+5 mm]` `[+10 mm]`
    - Stem: `[70]` `[80]` **90 mm** `[100]` · `[Flip ±6°]`
    - Bar roll: `[-5°]` **0°** `[+5°]` `[+10°]`
    - Seatpost setback: `[0 mm]` `[11 mm]` `[20 mm]` `[25 mm]`
  - Every click is a bookmarkable HTTP `GET` that updates the SVG diagrams, fit zones, and arithmetic ledger with zero client-side JavaScript.

---

### 3. "Show Your Work" Step-by-Step Computation Ledger

`bikegeo`'s `compare` page already uses native HTML `<details><summary>Scoring breakdown</summary>` ([compare.html](file:///Users/matt/src/bikegeo/pkg/web/templates/compare.html#L54)). Extending that pattern with a collapsible **`<details><summary>Inspect computation ledger</summary>`** (and a `--trace` CLI flag if desired for agent debugging) exposes the exact arithmetic chain for any setup in 5 auditable stages:

1. **Stage 1 — Chart Inputs & Closure Verification**:
   - `Published`: `Reach`, `Stack`, `HTA`, `HTL`, `BbDrop`, `Chainstay`, `Wheelbase`, `FrontCenter`
   - `Implied Structural A2C = (Stack - BbDrop + Rake × cos(HTA)) / sin(HTA) - HTL` vs. `Declared Fork A2C + GeoSpacer` (`Residual` vs. `RoundingTol`)
2. **Stage 2 — Active Fork, Cup Stack & Exact Pitch**:
   - `Active A2C = StaticA2C - Travel × SagPct + PhysicalCupStack`
   - `Δθ = -atan2(v_wb.Y, v_wb.X)` → Active `(Reach, Stack, HTA, STA, BbDrop, FrontCenter, RearCenter, Wheelbase, Trail)`
3. **Stage 3 — Rear Cockpit (Pelvis / Sit Bones)**:
   - `Effective STA` (showing bent-tube offset if applicable) + `SeatpostSetback` → `SeatpostClamp (X, Y)`
   - `Solved RailOffset (mm)` within `[-HalfWindow, +HalfWindow]` → `SitBones (X, Y)` & Workshop `BRP (X, Y)`, `SaddleHeight (BRP)`
4. **Stage 4 — Front Cockpit (Steerer → Stem → Bar → Grip)**:
   - `SteererClamp` (`HeadTubeTop` + `Spacer` + `StemThickness/2`) → `StemClamp (X, Y)` (`Length`, `Angle`) → `HandGrip (X, Y)` (`Handlebar ΔX, ΔY @ Roll`)
5. **Stage 5 — Fit Deltas & Score Curves**:
   - `Grip Error (ΔX, ΔY)` vs. `TargetHandGrip` → `Rotated Oval Ratio R` → `BaseScore + VersatilityBonus - StemPenalty = FinalSubScore`
   - Weighted points contribution and `PointsLost` per criterion

Every number displayed in the top comparison table becomes traceable in 3 seconds without reading Go source code.

---

## Part V: Prioritized Implementation Roadmap

Each phase is self-contained, preserves strict manufacturer chart immutability, and keeps `go vet ./... && go test ./...` green at every step:

1. **Phase 1 — Fix the 6 Correctness Bugs & Unify Closure Math**
   - Store/resolve fork travel explicitly instead of parsing `ForkRef.Fork` strings.
   - Fix `SizeResiduals` vs. `FrontCenterBudget` / `WheelbaseBudget` so `fit` and `validate` evaluate identical closure equations.
   - Replace `CalculateDynamicGeometry` + post-hoc `deltaRake` addition with exact 2D rear-axle rigid-body rotation over `(A2C_active, CupStack, Rake)`.
   - Resolve the `HeadsetCupType.CupStackMm()` vs. `GeoHeadsetSpacerMm` contradiction between [validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L209) and [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L1064-L1069).
   - Remove the hardcoded Otso Fenrir L call inside `AnalyzeBikeFitWithSaddle` (`GetDefaultBaselineBike`).

2. **Phase 2 — Introduce `BikeScene` & Sit-Bone-First Coordinates**
   - Implement the single-pass `BuildScene` pipeline returning explicit 2D coordinates for frame, fork, rear cockpit, and front cockpit.
   - Convert `IdmatchProfile()` BRP targets once at load time into `TargetSitBones` and `TargetHandGrip`; delete all cross-saddle BRP target translation helpers and hardcoded `746.0` / `219.0` / `292.0` / `3.0` constants.
   - Delete `ApplyIdealCockpitTargets` mutation from `scoring.ScoreRow` and remove `BaselineMetrics`, `ForkSpec`, and legacy `SelectBestHBWithDrop`.

3. **Phase 3 — Server-Rendered SVG Side-Views & Computation Ledger**
   - Add `<svg>` builders producing Full-Bike Overlay, Cockpit Zoom (`FitOval` + candidate stem/spacer dots), and Saddle/Seatpost Rail Zoom diagrams directly from `BikeScene`.
   - Embed the 5-stage `<details>` Computation Ledger on `/` and `/validate`.
   - Collapse `/saddles` and `/saddle-swap` (~809 web lines + 646 geometry lines) into one concise Saddle & Seatpost workbench tab powered by `BikeScene`.

4. **Phase 4 — Interactive Workbench & CLI/Registry Polish**
   - Add URL-driven component override parameters (`stem_len`, `stem_ang`, `spacer`, `bar_roll`, `bar_width`, `post_sb`, `rail_off`) plus zero-JS quick-step `<a>` controls with dashed-auto vs. solid-tweaked SVG overlays.
   - Clean up [registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go) (remove deprecated metrics and multi-alias clutter), make `handlebar_clamp_fit` cells transparent, and replace `-2` CLI removal tokens with an explicit `drop:` syntax that removes `reorderFlagsForStdlib`.
