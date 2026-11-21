# Slice 13c: 0–100 weighted scoring and 4-tier fit gate

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-21, Claude Sonnet 5.5.
- **Commits:** `Add 0-100 weighted scoring and 4-tier fit ranking` (`bf99568`); `Add default scoring weights and stock stems to catalog` (`d203497`); `Switch stock fitting and /bike stock table to 0-100 scoring` (`796c306`).
- **Done and not done:** A1, A2, C1, C2, C3, S1, S2, W1, W2, W3, W4 all done. Nothing left undone.
- **Deviations:** None in behavior. Every probed expected value in the plan (attribute curves, reference builds, stock fitting ranks, web table rows) matched on the first run within tolerance. Small test-structure choices: `scoring_test.go` and `stock_fitting_test.go` share one `testDefaultScoreWeights()` instead of defining a second `defaultScoreWeights()`; `FitOval` moved into `scoring.go` as the plan says; `TestBikeWorkbenchStockSweepColumn` checks only row 1 for `stock_bars=all` because later rows may install other bars.
- **Verification:** `make pre-commit` passed on each of the three commits and on the final state. Live check on `make serve-reboot`'s server (port 8081): `/bike?grip_x=481&grip_y=726` showed 749 combinations, total weight 23, 4-tier legend, row 1 `Fits neither` `56.9` `120 mm, -30°` `0 mm`, and `—` in the 4 saddle attribute cells; `/bike?post=setback_25&brp_x=-189&brp_y=713.1&grip_x=481&grip_y=726&stock_bars=all` showed 11235 combinations, weight 36, row 1 `Fits all` `80.0` `120 mm, -30°` `0 mm` `SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)` with all 12 attribute cells populated and 23 headers. I did not re-open bare `/bike` live; `TestBikeWorkbenchStockTiersAndLegend` asserts it shows no stock table, legend, or score readout. Screenshot was viewed in the in-app browser only; no file was saved.
- **Open issues:** None.

**Rulings:** Q16, Q17, Q40 (`make boundary` scoring words), Q58, Q59, Q59a, Q61, Q63, Q69, Q75e, Q76, Q82, Q84, Q86, Q89, Q90, Q96, Q97, Q98, Q99; AGENTS.md trig and `exhaustive` rules.

**Size:** larger (10 code/test/template files, 3 commits). Written for a lower-tier implementer. **Don't make design choices.** If something below doesn't match committed code or a probed expected value disagrees beyond its tolerance, stop and record `Blocked <date> on <question>` (see "Rules that bite").

*(Sub-slice decision: no further split beyond Q96's 13b/13c split. Commits 1 and 2 are strictly additive, and Commit 3 cuts over `ApplyStockFitting` and `/bike`'s stock table together so every commit builds and passes `make pre-commit`.)*

## Goal

1. Add the 12-attribute `0–100` weighted scoring model, 4-tier fit gate (`FitsAll`, `FitsBRP`, `FitsGrip`, `FitsNeither`), `ScoreBuild`, and shared `CompareFitRank` comparator in `internal/reboot/assembly/`.
2. Add `catalog.DefaultScoreWeights()` (Q59a weights, total `36`; `23` without BRP) and `catalog.StockStems()` (107 stems from `catalog.Stems()`) in `internal/reboot/catalog/`.
3. Replace `PenaltyBand`, `CockpitPreferences`, `BandName`, `Penalty`, `StemLengthStepMm`, and `StemAnglesDeg` in `assembly/stock_fitting.go` with `CockpitStock.Stems []StockStem`, `BuildScore`, and `CompareStockCandidates` (with Q69 deterministic cockpit tie-breaks), keeping strict forward `PlaceBike` per candidate (Q16, Q86), supporting cleared BRP targets (Q63, Q76), and scoring `fork=cad` as rigid (Q89).
4. Update `/bike`'s stock table (`web/workbench.go`, `web/web.go`, `web/templates/bike.html`, `web/web_test.go`) to display a Tier badge column, `0–100` Score column, 12 attribute score columns with weights in headers (`—` for the 4 saddle attributes when BRP is cleared, Q99), updated summary, and a readable on-page definition of all four tiers (Q61, Q90). No submitted-build score readout (Q98).

---

## Commit 1: `Add 0-100 weighted scoring and 4-tier fit ranking`

*(Strictly additive: existing `stock_fitting.go` and callers continue compiling unchanged until Commit 3.)*

### A1. `internal/reboot/assembly/scoring.go` (new file)

Define the 4-tier fit gate, 12 attributes, weights, stock stem input type, scoring curves, `ScoreBuild`, and `CompareFitRank`:

```go
package assembly

// FitTier classifies a build against the active fit gates (Q61, Q63, Q76, Q90).
type FitTier string

const (
	FitsAll     FitTier = "fits_all"
	FitsBRP     FitTier = "fits_brp"
	FitsGrip    FitTier = "fits_grip"
	FitsNeither FitTier = "fits_neither"
)

// Rank returns the sort priority of t (1 = best, 4 = worst).
func (t FitTier) Rank() int {
	switch t {
	case FitsAll:
		return 1
	case FitsBRP:
		return 2
	case FitsGrip:
		return 3
	case FitsNeither:
		return 4
	default:
		return 5
	}
}

// AttributeName names one of the 12 Q59a scoring attributes.
type AttributeName string

const (
	AttrHandGrip          AttributeName = "HandGrip"
	AttrSaddleBrpY        AttributeName = "Saddle BRP Y"
	AttrSaddleBrpX        AttributeName = "Saddle BRP X"
	AttrStem              AttributeName = "Stem"
	AttrHandlebarSweep    AttributeName = "Handlebar sweep"
	AttrForkSag           AttributeName = "Fork sag"
	AttrSpacers           AttributeName = "Spacers"
	AttrSaddleRailOffset  AttributeName = "Saddle rail offset"
	AttrRcWbRatio         AttributeName = "RC / WB ratio"
	AttrSeatpostInsertion AttributeName = "Seatpost insertion"
	AttrLeveledBbDrop     AttributeName = "Leveled BB drop"
	AttrTrail             AttributeName = "Trail"
)

// AttributeScore holds one attribute's 0–100 score and weight for a build.
// Active is false for the four saddle-dependent attributes when no BRP target is active (Q63, Q76).
type AttributeScore struct {
	Attribute AttributeName
	Weight    float64
	Score     float64
	Active    bool
}

// ScoreWeights holds the non-negative weights for the 12 Q59a attributes.
type ScoreWeights struct {
	HandGrip          float64
	SaddleBrpY        float64
	SaddleBrpX        float64
	Stem              float64
	HandlebarSweep    float64
	ForkSag           float64
	Spacers           float64
	SaddleRailOffset  float64
	RcWbRatio         float64
	SeatpostInsertion float64
	LeveledBbDrop     float64
	Trail             float64
}

// StockStem is one discrete stock stem (length, angle, and 0–100 composite score, Q58, Q97).
type StockStem struct {
	LengthMm float64
	AngleDeg pose.Deg
	Score    float64
}

// BuildScore holds a placed build's fit tier, weighted 0–100 overall score,
// gate misses, and per-attribute breakdown.
type BuildScore struct {
	Tier       FitTier
	Total      float64
	GripR      float64
	HasBrp     bool
	BrpMissXMm float64 // BrpTarget.X - placed BRP.X; 0 when !HasBrp
	BrpMissYMm float64 // BrpTarget.Y - placed BRP.Y; 0 when !HasBrp
	Attributes []AttributeScore
}
```

Validation and helpers in `scoring.go`:

1. `func DefaultGripOval() FitOval` — returns `FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60}` (Q17). *(Note: `FitOval` still lives in `stock_fitting.go` during Commit 1 and moves into `scoring.go` in Commit 3.)*
2. `func (s StockStem) Validate() error`:
   - `LengthMm`: finite and `> 0`.
   - `AngleDeg`: `isFinite(float64(s.AngleDeg))`.
   - `Score`: finite and `0 <= s.Score && s.Score <= 100`.
3. `func (w ScoreWeights) Validate() error`:
   - Every one of the 12 weights must satisfy `isFinite(v) && v >= 0`.
   - Sum of all 12 weights must be `> 0`.
4. `func (w ScoreWeights) ActiveTotal(hasBrp bool) float64`:
   - Sum of the 8 non-saddle weights (`HandGrip + Stem + HandlebarSweep + ForkSag + Spacers + RcWbRatio + LeveledBbDrop + Trail`), plus `SaddleBrpY + SaddleBrpX + SaddleRailOffset + SeatpostInsertion` when `hasBrp`.
5. Curve helpers (unexported, using `math.Abs`, `math.IsNaN`, `math.IsInf` only — no trig):
   - `clamp01(x float64) float64`: clamps to `[0, 1]`.
   - `scoreThreePoint(errMag, idealMax, gateMax, gateScore, worstMax float64) float64`:
     - `errMag <= idealMax` → `100`
     - `errMag <= gateMax` → `100 - (100 - gateScore)*(errMag - idealMax)/(gateMax - idealMax)`
     - `errMag < worstMax` → `gateScore * (worstMax - errMag) / (worstMax - gateMax)`
     - otherwise → `0`
   - `scoreRange(v, idealMin, idealMax, worstMin, worstMax float64) float64`:
     - If `v < idealMin`: if `math.IsInf(worstMin, -1)` return `100`; return `100 * clamp01((v - worstMin) / (idealMin - worstMin))`.
     - If `v > idealMax`: if `math.IsInf(worstMax, 1)` return `100`; return `100 * clamp01((worstMax - v) / (worstMax - idealMax))`.
     - Otherwise return `100`.
   - `scoreInsertionMargin(insertionMm float64, bounds InsertionBounds) float64` (Q75e, Q84):
     - `margin := min(insertionMm-bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm-insertionMm)`
     - `return 100.0 * clamp01(margin/30.0)`
   - **Attribute parameters (Q59a, Q75e, Q82, Q84, Q89):**
     - `AttrHandGrip`: `scoreThreePoint(gripR, 0.0, FitGateGripNormalizedRadius, 90.0, 2.0)`
     - `AttrSaddleBrpY`: when `hasBrp`, `scoreThreePoint(math.Abs(brpMiss.Y), 0.5, FitGateBrpYMm, 50.0, 4.0)`
     - `AttrSaddleBrpX`: when `hasBrp`, `scoreThreePoint(math.Abs(brpMiss.X), 1.0, FitGateBrpXMm, 50.0, 20.0)`
     - `AttrStem`: `stemScore`
     - `AttrHandlebarSweep`: `scoreRange(float64(setup.Handlebar.BacksweepDeg), 16.0, 20.0, 10.0, 26.0)`
     - `AttrForkSag`: `100.0` when `setup.Fork.Spec.TravelMm == 0` (rigid forks and `SteererCAD`, Q59, Q89); otherwise `scoreRange(setup.Fork.SagPct, 20.0, 20.0, 10.0, 30.0)`
     - `AttrSpacers`: `scoreRange(setup.Stem.SpacersMm, 0.0, 20.0, math.Inf(-1), DefaultMaxSpacersMm)` (`30.0` mm, Q82)
     - `AttrSaddleRailOffset`: when `hasBrp`, `scoreRange(setup.Saddle.RailOffsetMm, -5.0, 5.0, -15.0, 15.0)`
     - `AttrRcWbRatio`: `scoreRange(placement.Frameset.RcWbRatioPct(), 37.0, 39.0, 34.0, 42.0)`
     - `AttrSeatpostInsertion`: when `hasBrp`, `scoreInsertionMargin(setup.Seatpost.InsertionMm, bounds)`
     - `AttrLeveledBbDrop`: `scoreRange(placement.Frameset.LeveledBbDropMm(), 65.0, 75.0, 45.0, 95.0)`
     - `AttrTrail`: `scoreRange(placement.Frameset.TrailMm(), 98.0, 106.0, 78.0, 126.0)`
6. `func ScoreBuild(placement BikePlacement, target FitTarget, stemScore float64, weights ScoreWeights) (BuildScore, error)`:
   - Validate in order:
     1. `target.HandGrip == nil` → error `"build scoring needs a HandGrip target"`
     2. `!isFinitePoint(*target.HandGrip)` → `invalidInput(InputFitTargetGrip, ...)`
     3. `target.BRP != nil && !isFinitePoint(*target.BRP)` → `invalidInput(InputFitTargetBRP, ...)`
     4. `!isFinite(stemScore) || stemScore < 0 || stemScore > 100` → error
     5. `weights.Validate()` → error
     6. `activeWeight := weights.ActiveTotal(target.BRP != nil); activeWeight <= 0` → error (for example saddle-only weights when `target.BRP == nil`)
     7. When `target.BRP != nil`: call `SeatpostInsertionBounds(placement.Setup.Frame, placement.Setup.Seatpost.Spec)` (error on invalid bounds)
     8. Compute placed metrics and check finiteness of `gripR`, `rcWb`, `bbDrop`, `trail` (and `brpMiss` when `target.BRP != nil`).
   - Fit-gate classification using `fitBoundTolMm` (`1e-9`, matching `fitSaddle` in `fitting.go`):
     - `passGrip := gripR <= FitGateGripNormalizedRadius+fitBoundTolMm`
     - If `target.BRP == nil`: `tier = FitsGrip` if `passGrip`, else `FitsNeither` (Q63, Q76).
     - If `target.BRP != nil`: `passBrp := math.Abs(brpMiss.Y) <= FitGateBrpYMm+fitBoundTolMm && math.Abs(brpMiss.X) <= FitGateBrpXMm+fitBoundTolMm`; choose `FitsAll` / `FitsBRP` / `FitsGrip` / `FitsNeither` (Q61, Q90).
   - Build the 12-element `Attributes` slice in canonical Q59a order (the 4 saddle attributes have `Active: false, Score: 0` when `target.BRP == nil`), and compute `Total = weightedSum / activeWeight`.
7. `func CompareFitRank(a, b BuildScore) int`:
   - First compare `a.Tier.Rank()` vs `b.Tier.Rank()` ascending.
   - Then `switch a.Tier` (listing every `FitTier` constant for `exhaustive`):
     - `FitsAll`, `FitsGrip`:
       1. `cmp.Compare(b.Total, a.Total)` (descending score)
       2. For `FitsAll`: `cmp.Compare(a.GripR, b.GripR)` → `cmp.Compare(math.Abs(a.BrpMissYMm), math.Abs(b.BrpMissYMm))` → `cmp.Compare(math.Abs(a.BrpMissXMm), math.Abs(b.BrpMissXMm))`
       3. For `FitsGrip`: `cmp.Compare(math.Abs(a.BrpMissYMm), math.Abs(b.BrpMissYMm))` → `cmp.Compare(math.Abs(a.BrpMissXMm), math.Abs(b.BrpMissXMm))` → `cmp.Compare(a.GripR, b.GripR)`
     - `FitsBRP`, `FitsNeither`:
       1. `cmp.Compare(a.GripR, b.GripR)` (ascending `GripR`)
       2. `cmp.Compare(b.Total, a.Total)` (descending score)
       3. `cmp.Compare(math.Abs(a.BrpMissYMm), math.Abs(b.BrpMissYMm))`
       4. `cmp.Compare(math.Abs(a.BrpMissXMm), math.Abs(b.BrpMissXMm))`
     - `default`: `return 0`

### A2. `internal/reboot/assembly/scoring_test.go` (new file)

Add thorough unit tests (no `catalog` import; define `testDefaultScoreWeights()` locally):

1. `TestScoreWeightsValidation`:
   - Q59a weights pass `Validate()`, `ActiveTotal(true) == 36`, `ActiveTotal(false) == 23`.
   - Negative tests (must fail `Validate()`): negative weight (`-1`), `NaN`, `+Inf`, all 12 weights `0`.
2. `TestStockStemValidation`:
   - `{LengthMm: 70, AngleDeg: 0, Score: 100}` and `{LengthMm: 120, AngleDeg: -30, Score: 0}` pass `Validate()`.
   - Negative tests (must fail `Validate()`): `LengthMm` = `0`, `-10`, `NaN`, `+Inf`; `AngleDeg` = `NaN`, `-Inf`; `Score` = `-0.1`, `100.1`, `NaN`, `+Inf`.
3. `TestAttributeCurves`:
   - Test every breakpoint (ideal, midpoint, gate edge, worst anchor, beyond-worst clamp to `0`) for all 12 attributes via `ScoreBuild`, including:
     - `HandGrip`: `R = 0 → 100`, `0.5 → 95`, `1.0 → 90`, `1.5 → 45`, `2.0 → 0`, `3.0 → 0`.
     - `Saddle BRP Y`: `|ΔY| = 0, 0.5 → 100`, `1.25 → 75`, `2.0 → 50`, `3.0 → 25`, `4.0, 5.0 → 0`.
     - `Saddle BRP X`: `|ΔX| = 0, 1.0 → 100`, `5.5 → 75`, `10.0 → 50`, `15.0 → 25`, `20.0, 30.0 → 0`.
     - `Handlebar sweep`: `10° → 0`, `13° → 50`, `16°..20° → 100`, `23° → 50`, `26° → 0`.
     - `Fork sag`: rigid (`TravelMm: 0`) → `100`; `SteererCAD` (`TravelMm: 0`) → `100` (Q89); suspension at `10/15/20/25/30%` → `0/50/100/50/0`.
     - `Spacers`: `0/10/20 mm → 100`, `25 mm → 50`, `30 mm → 0` (Q82).
     - `Saddle rail offset`: `±15 → 0`, `±10 → 50`, `−5..+5 → 100`.
     - `Seatpost insertion` (Fenrir L + generic 400/365/100 post → `[100, 290]`): `100 → 0`, `115 → 50`, `130..260 → 100`, `275 → 50`, `290 → 0`; also degenerate equal bounds (`MaxPostInsertionMm = 100`, `InsertionMm = 100`, `margin = 0`) → `0` (no NaN/divide-by-zero).
     - `RC / WB ratio`, `Leveled BB drop`, `Trail` using `fenrirFittingSetup()` and Fenrir forks (independent probe values, tolerance `1e-5`):
       - Lithic: `RC/WB = 38.278721% → 100.0`, `BB drop = 67.819258 mm → 100.0`, `Trail = 94.123289 mm → 80.616445`.
       - Fox 34 @ 20% sag: `RC/WB = 38.511960% → 100.0`, `BB drop = 62.443349 mm → 87.216745`, `Trail = 109.963183 mm → 80.184085`.
       - CAD baseline: `RC/WB = 38.369496% → 100.0`, `BB drop = 72.082893 mm → 100.0`, `Trail = 90.197878 mm → 60.989388`.
4. `TestScoreBuildReferenceBuilds` (tolerance `1e-5`, independent probe):
   - Fenrir L + SID 100 @ 20% sag + `70 mm, 0°, 10 mm spacers` (stem score `100`) + exact placed BRP & HandGrip: `FitsAll`, `Total = 100.000000`, all 12 attributes `100.0`; with `BRP: nil`: `FitsGrip`, `Total = 100.000000`, 4 saddle attributes `Active == false`.
   - Fenrir L + Lithic + `70 mm, 0°, 10 mm spacers` (stem score `100`) + exact placed BRP & HandGrip: `FitsAll`, `Total = 99.461568` (`Trail = 80.616445`); with `BRP: nil`: `FitsGrip`, `Total = 99.157237`.
   - Fenrir L + CAD baseline (`SteererCAD`, `LowerCupBareHeadTube`) + `120 mm, -20°, 0 mm spacers` (stem score `0`) + `fitReportGrip` (`BRP: nil`): `FitsNeither`, `Total = 69.082428`, `GripR = 1.207581`, `AttrHandGrip = 71.317744`, `AttrForkSag = 100.0` (Q89), `AttrTrail = 60.989388`.
   - Gate-boundary BRP (`gateTradeOffBRP = (-175.242919, 710.580028)`, fitted via `ApplyFitting`) + exact `(70, 0, 10)` grip on Lithic: `FitsAll`, `Total = 85.031263`, `BrpMissXMm = -10.000000`, `BrpMissYMm = -1.097455`, `AttrSaddleBrpY = 80.084835`, `AttrSaddleBrpX = 50.0`, `AttrSaddleRailOffset = 0.0`.
   - Just outside BRP gate (`(-175.542919, 710.580028)`, fitted via `ApplyFitting`) + exact `(70, 0, 10)` grip on Lithic: `FitsGrip`, `Total = 88.017123`, `BrpMissXMm = -10.600000`, `BrpMissYMm = 0.0`, `AttrSaddleBrpY = 100.0`, `AttrSaddleBrpX = 46.999999`.
   - Non-default `ScoreWeights` (only `Trail: 1`): returns `80.616445` on Lithic.
5. `TestScoreBuildErrors`:
   - Negative tests: nil `HandGrip`, `NaN` `HandGrip` (`*InputError` with `InputFitTargetGrip`), `+Inf` `BRP` (`*InputError` with `InputFitTargetBRP`), invalid `stemScore` (`-1`, `101`, `NaN`), invalid `ScoreWeights`, saddle-only weights when `target.BRP == nil`, impossible `SeatpostInsertionBounds` (`MaxPostInsertionMm < EffectiveMinInsertionMm`) when `target.BRP != nil`.
6. `TestFitTierRankAndCompareFitRank`:
   - Tier order `FitsAll < FitsBRP < FitsGrip < FitsNeither`.
   - Intra-tier primary and all tie-break levels for each of the 4 tiers (verifying `FitsBRP` and `FitsNeither` sort by `GripR` ascending first even when the closer-`R` candidate has a lower `Total`), `HasBrp == false` ignoring BRP tie-breaks, antisymmetry, and transitivity.

---

## Commit 2: `Add default scoring weights and stock stems to catalog`

### C1. `internal/reboot/catalog/scoring.go` (new file)

```go
package catalog

import "bikegeo/internal/reboot/assembly"

// DefaultScoreWeights returns the Q59a 12-attribute scoring weights (total 36;
// 23 when no BRP target is set).
func DefaultScoreWeights() assembly.ScoreWeights {
	return assembly.ScoreWeights{
		HandGrip:          6,
		SaddleBrpY:        6,
		SaddleBrpX:        4,
		Stem:              5,
		HandlebarSweep:    3,
		ForkSag:           3,
		Spacers:           2,
		SaddleRailOffset:  2,
		RcWbRatio:         2,
		SeatpostInsertion: 1,
		LeveledBbDrop:     1,
		Trail:             1,
	}
}
```

### C2. `internal/reboot/catalog/stems.go`

Add `"bikegeo/internal/reboot/assembly"` to imports and add:

```go
// StockStems returns the 107 catalog stems in assembly.StockStem form for
// stock cockpit fitting (Q97).
func StockStems() []assembly.StockStem {
	stems := Stems()
	out := make([]assembly.StockStem, len(stems))
	for i, s := range stems {
		out[i] = assembly.StockStem{
			LengthMm: s.LengthMm,
			AngleDeg: s.AngleDeg,
			Score:    s.Score,
		}
	}
	return out
}
```

### C3. `internal/reboot/catalog/scoring_test.go` (new file) & `internal/reboot/catalog/stems_test.go`

1. `scoring_test.go` (`TestDefaultScoreWeights`): assert `Validate() == nil`, `ActiveTotal(true) == 36`, `ActiveTotal(false) == 23`, and every one of the 12 weights matches Q59a.
2. `stems_test.go` (`TestStockStems`): assert `len(StockStems()) == 107`, matches `Stems()` element-for-element (`LengthMm`, `AngleDeg`, `Score`), and `s.Validate() == nil` for every entry.

---

## Commit 3: `Switch stock fitting and /bike stock table to 0-100 scoring`

### S1. `internal/reboot/assembly/scoring.go` & `internal/reboot/assembly/stock_fitting.go`

1. Move `FitOval` into `scoring.go`, delete `PointsPerR` and `PointsLost(r float64)`, and add `func (o FitOval) Validate() error` (rejects non-finite/non-positive `SemiMinorMm`, `SemiMajorMm`, or non-finite `MajorAxisHeadingDeg`). Keep `NormalizedRadius(center, p pose.Point) float64`.
2. Delete `PenaltyBand`, `CockpitPreferences`, `BandName` (+ `Band*` constants), `Penalty`, `forkSagPoints`, and `brpMissPoints` from `stock_fitting.go`.
3. Update `CockpitStock` per Q97:
   ```go
   type CockpitStock struct {
       Stems        []StockStem
       SpacerStepMm float64
       Handlebars   []HandlebarSetup
       Forks        []StockFork
   }
   ```
4. Update `CockpitStock.Validate()`:
   - Require `len(s.Stems) > 0`.
   - Call `stem.Validate()` on every entry and reject duplicate `(LengthMm, AngleDeg)` pairs.
   - Keep `SpacerStepMm`, `Handlebars`, and `Forks` validation unchanged.
5. Update `StockCandidate`:
   ```go
   type StockCandidate struct {
       Placement      BikePlacement
       Target         pose.Point  // HandGrip target
       StockHandlebar bool        // true when installed from CockpitStock.Handlebars
       HandlebarIndex int         // index into CockpitStock.Handlebars; meaningful only when StockHandlebar
       StockFork      bool        // true when installed from CockpitStock.Forks
       ForkIndex      int         // index into CockpitStock.Forks; meaningful only when StockFork
       BrpTarget      *pose.Point // saddle BRP target; nil without one
       Score          BuildScore  // 4-tier fit gate, 0–100 weighted total, misses, and 12 attribute scores
   }
   ```
   Remove `GripR`, `Penalties`, `PointsLost()`, and `Score()`. Keep `GripMiss()` and `BrpMiss()`.
6. Add `func CompareStockCandidates(a, b StockCandidate) int`:
   - First `if order := CompareFitRank(a.Score, b.Score); order != 0 { return order }` (Q61, Q90).
   - Then Q69 within-row deterministic cockpit tie-breaks:
     1. `cmp.Compare(sa.SpacersMm, sb.SpacersMm)` (fewer spacers)
     2. `cmp.Compare(sa.LengthMm, sb.LengthMm)` (shorter stem)
     3. `cmp.Compare(math.Abs(float64(sa.AngleDeg)), math.Abs(float64(sb.AngleDeg)))` (smaller `|angle|`)
     4. `cmp.Compare(float64(sb.AngleDeg), float64(sa.AngleDeg))` (prefer `+` over `−` on exact `|angle|` tie)
     5. `cmp.Compare(a.HandlebarIndex, b.HandlebarIndex)` (handlebar catalog order)
     6. `cmp.Compare(a.ForkIndex, b.ForkIndex)` (fork order)
7. Update `ApplyStockFitting`:
   ```go
   func ApplyStockFitting(setup BikeSetup, target FitTarget, stock CockpitStock, weights ScoreWeights, limit int) (StockFitting, error)
   ```
   - Validate `target.HandGrip`, `target.BRP` (when non-nil), `limit >= 1`, `stock.Validate()`, `weights.Validate()`, and `weights.ActiveTotal(target.BRP != nil) > 0`.
   - Do **not** call `validateStemLengthRange(setup.Stem)` (Q97: `MinLengthMm`/`MaxLengthMm` belong to continuous `ApplyFitting` only).
   - Enumerate `forks × bars × stock.Stems × stacks`, building `candidate` with `stem.AngleDeg`, `stem.LengthMm`, `spacers`, `bar.setup`, placing each candidate forward via `PlaceBike(candidate)` (Q16, Q86), scoring via `ScoreBuild(placement, candidateTarget, stem.Score, weights)` (where `candidateTarget` has `HandGrip: &grip` and `BRP: brp`), sorting via `slices.SortFunc(candidates, CompareStockCandidates)`, and returning up to `limit` candidates and `Evaluated: len(candidates)`.

### S2. `internal/reboot/assembly/stock_fitting_test.go`

Rewrite `stock_fitting_test.go` against the new types (no `catalog` import). Use a fixed 10-stem fixture `fenrirStockStems()` (`70` spacer/stem combos per fork/bar) and `defaultScoreWeights()` (`6,6,4,5,3,3,2,2,2,1,1,1`):

```go
func fenrirStockStems() []assembly.StockStem {
	return []assembly.StockStem{
		{LengthMm: 60, AngleDeg: 0, Score: 95.5},
		{LengthMm: 70, AngleDeg: -6, Score: 100.0},
		{LengthMm: 70, AngleDeg: 0, Score: 100.0},
		{LengthMm: 70, AngleDeg: 6, Score: 100.0},
		{LengthMm: 90, AngleDeg: -17, Score: 62.0},
		{LengthMm: 110, AngleDeg: -17, Score: 36.25},
		{LengthMm: 120, AngleDeg: -30, Score: 0.0},
		{LengthMm: 120, AngleDeg: -20, Score: 0.0},
		{LengthMm: 120, AngleDeg: -17, Score: 24.0},
		{LengthMm: 120, AngleDeg: -10, Score: 34.5},
	}
}
```

Probe values for `stock_fitting_test.go` (tolerance `1e-5` on scores/radii, `1e-3` on `GripMiss().Norm()`, `1e-9` on reachable/tie-break invariants; independent probe `/Users/matt/.gemini/antigravity/brain/8a6a5b46-0239-41db-82cd-ef8ffbb3868f/scratch/probe13c.py`):

| Test / Case | `Evaluated` | Rank 1 (and checked ranks) |
| --- | ---: | --- |
| `TestStockFittingFreeBuild`: SID 20 + `(70, 0, 10)` + reachable BRP | `70` | `#1 FitsAll Total=100.000000 stem=(70,+0°,10) GripR<1e-9` |
| `TestStockFittingFreeBuild`: Lithic + `(70, 0, 10)` no BRP | `70` | `#1 FitsGrip Total=99.157237 stem=(70,+0°,10) GripR<1e-9` (and 4 saddle attrs `Active=false`) |
| `TestStockFittingReferenceBuilds`: default grip `(60,-6,20)`, no BRP | `70` | `#1 FitsGrip 97.929120 (60,+0°,15) R=0.095778`, `#2 FitsGrip 97.595491 (70,+6°,10) R=0.598669`, `#3 FitsGrip 97.551593 (70,+0°,20) R=0.615497`, `#4 FitsGrip 97.435741 (70,+6°,15) R=0.659907` |
| `TestStockFittingReferenceBuilds`: `fitReportGrip`, no BRP | `70` | `#1 FitsNeither 56.854583 (120,-30°,0) R=1.764743 gmiss=31.956`, `#2 FitsNeither 54.312823 (120,-30°,5) R=1.873003`, `#3 FitsNeither 53.138838 (120,-20°,0) R=1.923006` |
| `TestStockFittingForwardAndRankOrder`: `fitReportGrip`, limit `70` | `70` | Every candidate matches `PlaceBike`, keeps fixed inputs, has 12 `Attributes` in Q59a order (`Active=false` on the 4 saddle attrs), and `CompareStockCandidates(prev, curr) <= 0` for all adjacent ranks |
| `TestStockFittingIndependentOracle`: both `fitReportGrip` and `gripOf(base)` | `70` | Independent Go oracle computing all 70 builds agrees on rank 1 stem and top-5 `Total` within `1e-9` |
| `TestStockFittingChoosesAmongHandlebars`: `twoBarStock()`, `fitReportGrip` | `140` | `#1 FitsGrip 74.823105 (120,-10°,0) bar=1 R=0.536417`, `#2 FitsGrip 74.499673 (110,-17°,0) bar=1 R=0.806233`, `#3 FitsGrip 74.072663 (110,-17°,5) bar=1 R=0.969920`, `#4 FitsGrip 73.717781 (120,-10°,5) bar=1 R=0.960125`, `#5 FitsGrip 73.278784 (120,-17°,5) bar=1 R=0.253407` |
| `TestStockFittingScoresBarSweep`: `twoBarStock()`, `fitReportGrip` | `140` | `Attributes[4]` (`AttrHandlebarSweep`) is `100.0` for `bar=0` (`20°`) and `33.333333` for `bar=1` (`12°`) |
| `TestStockFittingInstallsEveryBarAtSubmittedRoll`: `twoBarStock()`, `BarRollDeg=5` | `140` | `#1 FitsGrip 75.106976 (110,-17°,0) bar=1 R=0.573433`, `#2 FitsGrip 74.586581 (120,-10°,0) bar=1 R=0.627085`, `#3 FitsGrip 74.478949 (110,-17°,5) bar=1 R=0.814177`, `#4 FitsGrip 73.796352 (120,-17°,5) bar=1 R=0.055006`, `#5 FitsGrip 72.965927 (120,-17°,10) bar=1 R=0.373335` |
| `TestStockFittingChoosesAmongForks`: `fenrirForkStock()`, `fitReportGrip` | `490` | `#1 FitsNeither 61.643767 fork=3 (SID 25%) (120,-20°,0) R=1.318877`, `#2 FitsNeither 61.669799 fork=3 (120,-17°,0) R=1.539990`, `#3 FitsNeither 56.130389 fork=3 (120,-30°,5) R=1.553706`, `#4 FitsNeither 54.965173 fork=3 (120,-30°,0) R=1.603335`, `#5 FitsNeither 61.179441 fork=2 (SID 20%) (120,-20°,0) R=1.616431` |
| `TestStockFittingRefitsSaddlePerFork`: `fenrirForkStock()`, `fitReportBRP+fitReportGrip` | `490` | `#1 FitsNeither 58.827962 fork=3 (SID 25%) (120,-20°,0) R=1.318877` (`miss=59.087687`), `#2 FitsNeither 58.844594 fork=3 (120,-17°,0) R=1.539990`, `#3 FitsNeither 55.305526 fork=3 (120,-30°,5) R=1.553706`, `#4 FitsNeither 54.561083 fork=3 (120,-30°,0) R=1.603335`, `#5 FitsNeither 58.531310 fork=2 (120,-20°,0) R=1.616431` |
| `TestStockFittingScoresBrpMissForSubmittedFork`: Lithic, `fitReportBRP+fitReportGrip` | `70` | `#1 FitsNeither 55.768206 (120,-30°,0)` (`AttrSaddleBrpY=100`, `AttrSaddleBrpX=0`, `AttrSaddleRailOffset=0`, `AttrSeatpostInsertion=100`) |
| `TestStockFittingFourTiersAndCadFork`: `setback_25`, `BRP=(-189,713.1)`, `fitReportGrip` | `70` / `140` | 1 bar: `#1 FitsBRP 68.295440 (120,-30°,0) R=1.764743` (`AttrSaddleRailOffset=25.490222`); 2 bars: `#1 FitsAll 79.775329 (120,-10°,0) bar=1 R=0.536417`; CAD fork no BRP: `#1 FitsNeither 69.082428 (120,-20°,0) R=1.207581` (`AttrForkSag=100`, `AttrTrail=60.989388`) |

Also test in `stock_fitting_test.go`:
- `TestCompareStockCandidatesTieBreaks`: exact tie on `BuildScore` broken step-by-step by spacers → stem length → `|stem angle|` → `+angle` over `−angle` → `HandlebarIndex` → `ForkIndex`; plus reversing `stock.Stems` produces the identical sorted candidate sequence.
- `TestStockFittingZeroSpacers` (`Evaluated == 10`), `TestStockFittingStepsAtHardwareLimits` (`Evaluated == 70`), `TestStockFittingIgnoresContinuousStemLengthRange` (`setup.Stem.MinLengthMm = 0` succeeds in `ApplyStockFitting` per Q97).
- `TestStockFittingErrors` and `TestCockpitStockValidation`: missing/NaN `HandGrip`, NaN `BRP`, `limit < 1`, bad `MaxSpacersMm`, empty `Stems`, duplicate `(LengthMm, AngleDeg)` in `Stems`, invalid `StockStem`, invalid `SpacerStepMm`, non-zero bar `BarRollDeg`, invalid bar, invalid fork/cup, invalid `ScoreWeights`, saddle-only `ScoreWeights` without BRP target.

### W1. `internal/reboot/web/workbench.go`

1. Replace `CockpitPreferences assembly.CockpitPreferences` on `bikeWorkbench` with `ScoreWeights assembly.ScoreWeights`.
2. In `bikeWorkbench.validate()`: validate `w.ScoreWeights.Validate()` and require `w.ScoreWeights.ActiveTotal(false) > 0` (`fmt.Errorf("workbench %q score weights have zero active weight without BRP", w.Name)`).
3. In `fenrirWorkbench()`:
   ```go
   CockpitStock: assembly.CockpitStock{
       Stems:        catalog.StockStems(),
       SpacerStepMm: 5,
   },
   ScoreWeights: catalog.DefaultScoreWeights(),
   ```

### W2. `internal/reboot/web/web.go`

1. In `handleBikeWorkbench`: pass `wb.ScoreWeights` to `assembly.ApplyStockFitting`.
2. Delete `freeRangeText`.
3. Update `stockFittingView`, `stockCandidateView`, and `buildStockFittingView`:

```go
type stockAttributeHeader struct {
	Attribute assembly.AttributeName
	Label     string
}

type fitTierLegendItem struct {
	Tier       assembly.FitTier
	Class      string
	Label      string
	Definition string
}

type stockFittingView struct {
	AllForks         bool
	HasBrp           bool
	ActiveWeight     float64
	PartsText        string
	Evaluated        int
	AttributeHeaders []stockAttributeHeader
	TierLegend       []fitTierLegendItem
	Rows             []stockCandidateView
}

type stockCandidateView struct {
	Rank                    int
	Tier                    assembly.FitTier
	TierClass               string
	TierLabel               string
	BarName                 string
	ForkName                string
	HasBrp                  bool
	BrpMissXMm              float64
	BrpMissYMm              float64
	StemLengthMm, SpacersMm float64
	StemAngleDeg            pose.Deg
	GripMissMm, GripR       float64
	Score                   float64
	Attributes              []assembly.AttributeScore
	ApplyURL                string
}
```

- Helper `fitTierPresentation(t assembly.FitTier) (class, label string)` using an **exhaustive `switch`** over `FitsAll` (`"fits-all"`, `"Fits all"`), `FitsBRP` (`"fits-brp"`, `"Fits BRP"`), `FitsGrip` (`"fits-grip"`, `"Fits grip"`), `FitsNeither` (`"fits-neither"`, `"Fits neither"`).
- `TierLegend` lists all 4 tiers in rank order, built using `assembly.DefaultGripOval()`, `assembly.FitGateGripNormalizedRadius`, `assembly.FitGateBrpYMm`, and `assembly.FitGateBrpXMm`:
  - `Fits all`: `"HandGrip inside the ±10 × ±20 mm oval (R ≤ 1.0) and saddle BRP within |ΔY| ≤ 2.0 mm, |ΔX| ≤ 10.0 mm; ranked by descending score."`
  - `Fits BRP`: `"Saddle BRP within |ΔY| ≤ 2.0 mm, |ΔX| ≤ 10.0 mm, but HandGrip outside the oval (R > 1.0); ranked by closest HandGrip R, then descending score."`
  - `Fits grip`: `"HandGrip inside the oval (R ≤ 1.0), but misses saddle BRP (or BRP target is unset); ranked by descending score."`
  - `Fits neither`: `"HandGrip outside the oval (R > 1.0) and misses saddle BRP (or BRP target is unset); ranked by closest HandGrip R, then descending score."`
- `AttributeHeaders` lists all 12 Q59a attributes in order with formatted weight from `wb.ScoreWeights` via `strconv.FormatFloat(w, 'f', -1, 64)`:
  `Grip (6)`, `BRP Y (6)`, `BRP X (4)`, `Stem (5)`, `Sweep (3)`, `Sag (3)`, `Spacers (2)`, `Rail (2)`, `RC/WB (2)`, `Post (1)`, `BB drop (1)`, `Trail (1)`.
- Set `HasBrp: state.Targets.HasBrp` and `ActiveWeight: wb.ScoreWeights.ActiveTotal(state.Targets.HasBrp)` on `stockFittingView`.
- Populate each `stockCandidateView` from `c.Score` (`Tier`, `TierClass`, `TierLabel`, `Score: c.Score.Total`, `GripR: c.Score.GripR`, `HasBrp: c.Score.HasBrp`, `BrpMissXMm: c.Score.BrpMissXMm`, `BrpMissYMm: c.Score.BrpMissYMm`, `Attributes: c.Score.Attributes`).

### W3. `internal/reboot/web/templates/bike.html`

1. Add CSS classes for tier badges and legend:
   - `.tier-badge` (inline-block pill badge, `font-size: 0.78rem; font-weight: 600; padding: 0.12rem 0.45rem; border-radius: 999px;`)
   - `.tier-fits-all { background: #dafbe1; color: #116329; }`
   - `.tier-fits-brp { background: #ddf4ff; color: #0550ae; }`
   - `.tier-fits-grip { background: #fff8c5; color: #7d4e00; }`
   - `.tier-fits-neither { background: #ffebe9; color: #82071e; }`
   - `.tier-legend` (`display: grid; grid-template-columns: auto 1fr; gap: 0.25rem 0.65rem; margin: 0 0 0.55rem; font-size: 0.8rem; color: var(--muted);`)
2. Replace the stock fitting header notes and table inside `{{with .Stock}}`:
   - `<p class="fit-note" data-stock-summary><span data-stock-evaluated>{{.Evaluated}}</span> stock {{.PartsText}} combinations placed; top {{len .Rows}} by fit tier and 0–100 weighted score (total weight {{printf "%.0f" .ActiveWeight}}{{if not .HasBrp}}; 4 saddle attributes omitted without BRP target{{end}}). {{if .AllForks}}The saddle is refitted for each fork.{{else}}The saddle fit is kept.{{end}}</p>`
   - `<dl class="tier-legend" data-fit-tier-legend>{{range .TierLegend}}<dt><span class="tier-badge tier-{{.Class}}">{{.Label}}</span></dt><dd>{{.Definition}}</dd>{{end}}</dl>`
   - Table headers (23 columns total, inside `.stock-scroll` per Q99):
     `<th>#</th><th></th><th>Tier</th><th>Score</th><th>Stem</th><th>Spacers</th><th>HandGrip miss</th><th>Oval R</th><th>BRP miss</th><th>Handlebar</th><th>Fork</th>{{range .AttributeHeaders}}<th>{{.Label}}</th>{{end}}`
   - Table cells per row:
     - `<td>{{.Rank}}</td>`
     - `<td><a href="{{.ApplyURL}}" data-preset="apply-stock" data-stock-apply>Apply</a></td>`
     - `<td data-stock-cell="tier"><span class="tier-badge tier-{{.TierClass}}" data-fit-tier="{{.Tier}}">{{.TierLabel}}</span></td>`
     - `<td class="mono" data-stock-cell="score"><strong>{{printf "%.1f" .Score}}</strong></td>`
     - `<td class="mono" data-stock-cell="stem">{{printf "%.0f mm, %+.0f°" .StemLengthMm .StemAngleDeg}}</td>`
     - `<td class="mono" data-stock-cell="spacers">{{printf "%.0f mm" .SpacersMm}}</td>`
     - `<td class="mono" data-stock-cell="grip-miss">{{printf "%.1f mm" .GripMissMm}}</td>`
     - `<td class="mono" data-stock-cell="oval-r">{{printf "%.2f" .GripR}}</td>`
     - `<td class="mono" data-stock-cell="brp-miss">{{if .HasBrp}}{{printf "(%+.1f, %+.1f) mm" .BrpMissXMm .BrpMissYMm}}{{else}}—{{end}}</td>`
     - `<td data-stock-cell="bar">{{.BarName}}</td>`
     - `<td data-stock-cell="fork">{{.ForkName}}</td>`
     - `{{range .Attributes}}<td class="mono" data-stock-attr="{{.Attribute}}">{{if .Active}}{{printf "%.1f" .Score}}{{else}}—{{end}}</td>{{end}}`
   - Do **not** add any submitted-build score readout (Q98).

### W4. `internal/reboot/web/web_test.go`

Update all affected stock-table assertions to the 107-stem catalog, 4-tier ranking, and 23-column `0–100` attribute table (all values verified by `/Users/matt/.gemini/antigravity/brain/8a6a5b46-0239-41db-82cd-ef8ffbb3868f/scratch/probe13c.py`):

| Test & Query | `Evaluated` | Expected top rows & key cells |
| --- | ---: | --- |
| `TestBikeWorkbenchRanksStockFitting` (`/bike?grip_x=481&grip_y=726&view=stem`) | `749` | `23` headers (`Tier` col 2, `Score` col 3, `Grip (6)`..`Trail (1)` cols 11–22); `[data-fit-tier-legend]` has 4 items; summary `749 stock stem and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 23; 4 saddle attributes omitted without BRP target). The saddle fit is kept.`; row 1 `tier="Fits neither"`, `score="56.9"`, `stem="120 mm, -30°"`, `spacers="0 mm"`, `grip-miss="32.0 mm"`, `oval-r="1.76"`, `brp-miss="—"`, `fork="Lithic Carbon Mountain"`; row 1 attrs `[21.2, —, —, 0.0, 100.0, 100.0, 100.0, —, 100.0, —, 100.0, 80.6]`; rows 2–3 `score` = `"54.3"`, `"53.1"`; apply query `stem_angle=-30.0`, `stem_length=120.0`, `spacers=0.0` |
| `TestBikeWorkbenchStockBarsDefaultKeepsSubmittedBar` (`/bike?grip_x=481&grip_y=726`) | `749` | Summary matches weight `23` without BRP; 5 rows all keep `persuader20_800` |
| `TestBikeWorkbenchStockFittingAcrossAllBars` (`?stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem`) | `11235` | Row 1 `sqlab311_20_16_50` `120 mm, -30°` `0 mm` `0.84` `67.5` (`Fits grip`), row 2 `sqlab311_flx_12_15` `120 mm, -10°` `0 mm` `0.54` `67.2` (apply sets `bar=sqlab311_flx_12_15`, `stem_length=120.0`, `stem_angle=-10.0`, `spacers=0.0`), row 3 `sqlab311_flx_12_15` `110 mm, -17°` `0 mm` `0.81` `67.0`, row 4 `sqlab311_flx_12_30` `110 mm, -17°` `0 mm` `0.86` `66.9`, row 5 `sqlab311_flx_12_15` `110 mm, -17°` `5 mm` `0.97` `66.8`; `"different submitted bar"` (`stock_bars=all&bar=sqlab3ox_12_15&grip_x=481&grip_y=726`): `Fits grip`, scores `75.2`, `74.8`, `74.5`, `74.4`, `74.1` |
| `TestBikeWorkbenchStockFittingKeepsSubmittedBarRoll` (`?stock_bars=all&bar_roll=5&grip_x=481&grip_y=726`) | `11235` | Row 1 `sqlab311_20_16_50` `120 mm, -30°` `0 mm` `0.53` `76.0`, row 2 `sqlab311_flx_12_15` `110 mm, -17°` `0 mm` `0.57` `75.1`, row 3 `sqlab3ox_16_45` `120 mm, -30°` `0 mm` `0.92` `75.0`, row 4 `sqlab311_flx_16_30` `120 mm, -30°` `5 mm` `0.94` `75.0`, row 5 `sqlab311_20_16_50` `120 mm, -30°` `5 mm` `0.95` `74.9` |
| `TestBikeWorkbenchStockSweepColumn` | — | `stock_bars=all` row 1 `Handlebar sweep` attr cell = `"100.0"`; `bar=sqlab311_flx_12_15` all rows `Handlebar sweep` attr cell = `"33.3"` |
| `TestBikeWorkbenchSubmitsStockBarsSelect` | `11235` | Row 1 `bar` = `SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)` |
| `TestBikeWorkbenchAppliesFreeStockBuild` (`stem 0° / 70 / 10` grip) | `749` | Row 1 `tier="Fits grip"`, `stem="70 mm, +0°"`, `spacers="10 mm"`, `oval-r="0.00"`, `score="99.2"` (`Trail=80.6` on Lithic); applied outcome `reached`, miss `0.0 mm` |
| `TestBikeWorkbenchStockFitKeepsSaddleFitAndTargets` (`?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`) | `749` | Row 1 `tier="Fits neither"`, `score="55.8"`, `brp-miss="(-53.4, +0.0) mm"`, `stem_angle=-30.0`, `stem_length=120.0`, `spacers=0.0`, `rail_offset=15.0`, `post_insert≈197.387571` |
| `TestBikeWorkbenchValidatesStockAndPreferences` | — | Test `wb.validate()` rejects `SpacerStepMm = 0`, `Stems = nil`, negative `ScoreWeights.HandGrip`, and saddle-only `ScoreWeights` (zero active weight without BRP) |
| `TestBikeWorkbenchFitsAroundChosenHandlebar` (`?bar=sqlab3ox_12_15&grip_x=481&grip_y=726&view=stem`) | `749` | Row 1 `tier="Fits grip"`, `stem="120 mm, -10°"`, `spacers="0 mm"`, `oval-r="0.96"`, `score="73.7"`, apply sets `stem_length=120.0`, `stem_angle=-10.0`, `spacers=0.0` |
| `TestBikeWorkbenchStockForksDefaultKeepsSubmittedFork` | `749` | Rigid Lithic: `Fork sag` attr = `"100.0"`; `fork=sid_100&sag=25`: `Fork sag` attr = `"50.0"`; `fork=cad`: `Fork sag` attr = `"100.0"` (Q89), `score="69.1"`, `stem="120 mm, -20°"` |
| `TestBikeWorkbenchStockFittingAcrossAllForks` (`?stock_forks=all&grip_x=481&grip_y=726&view=stem`) | `5243` | Row 1 `SID 25%` `120 mm, -20°` `0 mm` `1.32` `61.6` (`Sag=50.0`), row 2 `SID 25%` `120 mm, -17°` `0 mm` `1.54` `61.7`, row 3 `SID 25%` `120 mm, -30°` `5 mm` `1.55` `56.1`, row 4 `SID 25%` `120 mm, -30°` `0 mm` `1.60` `55.0`, row 5 `SID 20%` `120 mm, -20°` `0 mm` `1.62` `61.2`; row 1 apply sets `fork=sid_100`, `sag=25.0`, `stem_length=120.0`, `stem_angle=-20.0`, `spacers=0.0` |
| `TestBikeWorkbenchStockForksWithSaddle` (`?stock_forks=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`) | `5243` | Row 1 `SID 25%` `120 mm, -20°` `0 mm` `58.8` `(-59.1, +0.0) mm` `BRP X=0.0`, row 2 `SID 25%` `120 mm, -17°` `0 mm` `58.8`, row 3 `SID 25%` `120 mm, -30°` `5 mm` `55.3`, row 4 `SID 25%` `120 mm, -30°` `0 mm` `54.6`, row 5 `SID 20%` `120 mm, -20°` `0 mm` `58.5`; apply sets `fork=sid_100`, `sag=25.0`, `rail_offset=15.0`, `post_insert≈198.873690` |
| `TestBikeWorkbenchStockForksAndBars` (`?stock_forks=all&stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`) | `78645` | Summary `78645 stock fork, handlebar, stem, and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 36). The saddle is refitted for each fork.`; row 1 `SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm)` `SID 20%` `120 mm, -7°` `0 mm` `69.0`, row 2 `FL-X 12°, 15` `SID 20%` `110 mm, -10°` `0 mm` `68.8`, row 3 `FL-X 12°, 15` `SID 20%` `120 mm, -6°` `0 mm` `68.7`, row 4 `Alloy 16°, 50` `SID 20%` `120 mm, -30°` `0 mm` `68.5`, row 5 `FL-X 12°, 15` `SID 20%` `120 mm, -10°` `0 mm` `68.3` |
| `TestBikeWorkbenchStockTiersAndLegend` (new) | — | `?post=setback_25&brp_x=-189&brp_y=713.1&grip_x=481&grip_y=726`: row 1 `Fits BRP` (`fits_brp`) `68.3` `brp-miss="(+0.0, +0.0) mm"` `Rail=25.5`; with `&stock_bars=all`: row 1 `Fits all` (`fits_all`) `80.0` `Alloy 16°, 50` `120 mm, -30°` `0 mm` `0.84` `Rail=25.5`; verify all 4 `[data-fit-tier-legend]` items and no score readout on `/bike` without targets (Q98) |

---

## Mechanics chosen by the planner (veto on review)

1. `BuildScore` and `CompareFitRank(a, b BuildScore)` are separated from `CompareStockCandidates(a, b StockCandidate)` so Slice 13d's `compare` shares the exact Q61/Q90 4-tier comparator and adds its own Q69 cross-row tie-breaks (`FrameID` → size → fork → sag).
2. `FitTier` uses snake_case string constants (`"fits_all"`, `"fits_brp"`, `"fits_grip"`, `"fits_neither"`) with a `Rank() int` method, matching existing `assembly` enums and giving self-describing JSON in Slice 13d.
3. Fit-gate classification applies `fitBoundTolMm` (`1e-9`) on both BRP (`|ΔY| ≤ 2.0 + 1e-9`, `|ΔX| ≤ 10.0 + 1e-9`) and HandGrip (`R ≤ 1.0 + 1e-9`), matching `fitSaddle`'s gate check so a gate-boundary saddle fit never flips tier from floating-point rounding.
4. `BuildScore.Attributes` always contains all 12 Q59a entries in canonical order, marking the 4 saddle attributes `Active: false, Score: 0` when `target.BRP == nil` so table columns and JSON layout stay fixed.
5. `CockpitStock.Validate()` rejects duplicate `(LengthMm, AngleDeg)` entries in `Stems`. `ApplyStockFitting` ignores `setup.Stem.MinLengthMm`/`MaxLengthMm` (retained for continuous `ApplyFitting` per Q97).
6. `/bike`'s stock table places build identity and misses (`#`, Apply, `Tier`, `Score`, `Stem`, `Spacers`, `HandGrip miss`, `Oval R`, `BRP miss`, `Handlebar`, `Fork`) before the 12 attribute score columns, formatting `BRP miss` as `(%+.1f, %+.1f) mm` (matching the Saddle fitting block above it) so both gated axes are visible.
7. `ScoreBuild` scores `Saddle rail offset` against Q59a's fixed `−5..+5` ideal / `±15 mm` worst anchor, and scores `Fork sag` as `100` whenever `Fork.Spec.TravelMm == 0` (rigid forks and `SteererCAD`, Q89).

## Rules that bite

- `make boundary`: never use the words `price`, `pricing`, `cost`, `cheap`, `ledger`, or `chassis` in reboot file names, identifiers, comments, templates, or tests (in any case or inside longer words).
- `exhaustive`: every `switch` on `FitTier`, `AttributeName`, `InputField`, etc. must name every constant even when it has a `default`.
- Trig stays in `internal/reboot/pose`: outside `pose`, non-test reboot code may only use `math.Abs`, `math.IsNaN`, `math.IsInf` from `math`.
- Import boundaries: `assembly` and `assembly_test` never import `catalog`.
- Strict forward `PlaceBike` per candidate: keep the nested enumeration loop in `ApplyStockFitting` placing every candidate via `PlaceBike` (Q16, Q86).
- Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Commit subjects are plain language, matching the three commit titles above.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 13c Milestones line.

## Verification

1. `make pre-commit` passes on each of the 3 commits individually.
2. Live check (`make serve-reboot`):
   - Open `http://127.0.0.1:8081/bike?grip_x=481&grip_y=726`: confirm `749` combinations, weight `23`, 4-tier legend, row 1 `Fits neither` `56.9` `120 mm, -30°` `0 mm`, and `—` in the 4 saddle attribute columns.
   - Open `http://127.0.0.1:8081/bike?post=setback_25&brp_x=-189&brp_y=713.1&grip_x=481&grip_y=726&stock_bars=all`: confirm `11235` combinations, weight `36`, row 1 `Fits all` `80.0` `120 mm, -30°` `0 mm` `SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)`, and all 12 attribute cells populated.
   - Open `http://127.0.0.1:8081/bike`: confirm no stock table and no submitted-build score readout.
   - Record screenshot path(s) in the handoff, or explain why skipped.

## Out of scope

- Multi-frame `compare` engine, `LeveledReachMm`/`LeveledStackMm`/`EttMm`, `internal/reboot/query`, and CLI JSON output (Slice 13d).
- Changing `/bike`'s default seatpost to `redshift_shockstop_pro`, defaulting `/bike` to idmatch targets, durable `brp=off`/`grip=off` URL state, and multi-frame `/bike` selector (Slice 14).
- Submitted-build score readout on `/bike` (Q98, Slice 14).
- URL-tunable attribute weights (Q59a).
- Any stock-fitting loop optimization or hoisting (Q86).
