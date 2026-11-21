# Slice 13a: Gate-Aware Height-First Lexicographic Saddle Clamping

Status: Planned 2026-11-20

This slice implements **Slice 13a** from [DESIGN.md](DESIGN.md): adding the BRP and HandGrip fit-gate threshold constants in `internal/reboot/assembly` and replacing Euclidean box clamping in `fitSaddle` with gate-aware, height-first lexicographic clamping over the bounded `[EffectiveMinInsertion, MaxAllowedInsertion] × [−railLimit, +railLimit]` box.

> **Stop-and-escalate rule:** Implement strictly as specified below. Do not make unapproved design changes or deviate without a real blocker. If you encounter an unspec'd design fork, stop, record `Status: Blocked <date> on <question>` with an `## Implementation handoff` section, and ask Matt to switch to a Frontier model.

## Confirmed rulings this slice rests on

- **Q59a:** Fit-gate thresholds (`R ≤ 1.0`, `|ΔY| ≤ 2.0 mm`, `|ΔX| ≤ 10.0 mm`) are `assembly` constants, added in Slice 13a.
- **Q60:** When BRP is reachable, `fitSaddle` hits exact `(X, Y)`. When rails hit `±railLimit`, `fitSaddle` solves `Seatpost.InsertionMm` to preserve exact `target.Y` (clamped to `[EffectiveMinInsertion, MaxAllowedInsertion]`) so rail-limit shortfall lands purely on `ΔX`, and `|ΔY| > 0` occurs only when insertion limits bind (or when a small `|ΔY| ≤ 2.0 mm` trade-off is needed to pass the `|ΔX| ≤ 10.0 mm` gate per Q74).
- **Q68 & Q74:** When BRP is unreachable within `[EffectiveMinInsertion, MaxAllowedInsertion] × [−railLimit, +railLimit]`, prefer points passing the BRP fit gate (`|ΔY| ≤ 2.0 mm` and `|ΔX| ≤ 10.0 mm`) first — minimizing `(|ΔY|, |ΔX|)` lexicographically over the gate-passing intersection on the box edges — and fall back to lexicographic `(|ΔY|, |ΔX|)` over the full box only when no point in the box passes the gate.
- **Q84:** `fitSaddle` operates on `SeatpostSetup.InsertionMm` within `SeatpostInsertionBounds(frame, spec)` (`[EffectiveMinInsertionMm, MaxAllowedInsertionMm]`), with no unbounded sentinel.

## Implementation mechanics (proposed defaults for review)

- **Exported fit-gate constants** in `internal/reboot/assembly/fitting.go`: `FitGateGripR = 1.0`, `FitGateBrpYMm = 2.0`, `FitGateBrpXMm = 10.0`.
- **Floating-point tolerances:** reuse `fitBoundTolMm = 1e-9` for gate-membership (`|ΔY| ≤ FitGateBrpYMm + fitBoundTolMm && |ΔX| ≤ FitGateBrpXMm + fitBoundTolMm`), lexicographic comparison tie tolerance (`1e-9`), and bound detection for `LimitedBy`; treat direction components with `|s| < 1e-12` as zero (matching `pose.DecomposeAlong`'s `1e-12` threshold) so `sin(180°) ≈ 1.22e-16` on level rails never turns a horizontal rail edge into a spurious vertical solve.
- **Scope of clamping change:** only `fitSaddle` switches to `fitSaddleBox`. `fitCockpit` and `FitPostSetback` keep `fitBox` (Euclidean minimum) unchanged.

## Exact changes by file

### 1. `internal/reboot/assembly/fitting.go`

Add the exported Q59a fit-gate constants near the top of the file (right after `FitTarget` or `FitOutcome`):

```go
// Fit-gate thresholds (Q59a). A build passes the HandGrip gate when
// GripR <= FitGateGripR and passes the BRP gate when |ΔY| <= FitGateBrpYMm
// and |ΔX| <= FitGateBrpXMm.
const (
	FitGateGripR  = 1.0
	FitGateBrpYMm = 2.0
	FitGateBrpXMm = 10.0
)
```

Update `fitSaddle` to call a new unexported helper `fitSaddleBox(delta pose.Point, a, b fitAxis)` instead of `fitBox(delta, a, b)`. Keep `fitSaddle`'s signature, probe convention (`e = MaxAllowedInsertionMm - InsertionMm ∈ [0, MaxAllowedInsertionMm - EffectiveMinInsertionMm]`, `r = RailOffsetMm ∈ [-railLimit, +railLimit]`), and parallel-axes error unchanged.

Add `fitSaddleBox`:

```go
// fitSaddleBox solves delta ≈ da·dir(a.heading) + db·dir(b.heading) with finite
// bounds da in [a.lo, a.hi] and db in [b.lo, b.hi]. Reachable targets return
// the exact solution (snapped within fitBoundTolMm) with nil limited. When the
// unconstrained solution lies outside the box, the optimum lies on the box's
// boundary; fitSaddleBox prefers points passing the BRP fit gate
// (|ΔY| <= FitGateBrpYMm and |ΔX| <= FitGateBrpXMm) first, minimizing
// (|ΔY|, |ΔX|) lexicographically, and falls back to lexicographic (|ΔY|, |ΔX|)
// over the full box when no point in the box passes the gate (Q60, Q68, Q74).
func fitSaddleBox(delta pose.Point, a, b fitAxis) (da, db float64, limited []InputField, ok bool)
```

Algorithm for `fitSaddleBox` (zero raw trig; use only `pose.DecomposeAlong` and `pose.Along`):

1. Unconstrained check:
   - `da, db, ok = pose.DecomposeAlong(delta, a.heading, b.heading)`. If `!ok`, return `0, 0, nil, false`.
   - If `da >= a.lo-fitBoundTolMm && da <= a.hi+fitBoundTolMm && db >= b.lo-fitBoundTolMm && db <= b.hi+fitBoundTolMm`, return `clampFit(da, a.lo, a.hi), clampFit(db, b.lo, b.hi), nil, true`.
2. Direction vectors:
   - `dirA := pose.Along(a.heading, 1)`
   - `dirB := pose.Along(b.heading, 1)`
   - Define `miss(x, y float64) pose.Point`: `delta.Sub(pose.Point{X: x*dirA.X + y*dirB.X, Y: x*dirA.Y + y*dirB.Y})`.
3. 1D segment helpers along an edge `t ∈ [lo, hi]` with residual `rx(t) = bx - t·sx`, `ry(t) = by - t·sy`:
   - `clipLinearBound(lo, hi, c, s, limit float64) (float64, float64, bool)`: intersects `[lo, hi]` with `{t : |c - t·s| <= limit}`.
     - If `math.Abs(s) < 1e-12`: if `math.Abs(c) <= limit+fitBoundTolMm`, return `lo, hi, true`; else return `0, 0, false`.
     - Otherwise compute `t1 := (c - limit) / s` and `t2 := (c + limit) / s`; sort so `tLo <= tHi`; intersect `newLo := max(lo, tLo)`, `newHi := min(hi, tHi)`.
     - If `newLo > newHi+fitBoundTolMm`, return `0, 0, false`.
     - If `newLo > newHi` (tiny round-off overlap), snap both to `clampFit(0.5*(newLo+newHi), lo, hi)`.
     - Return `newLo, newHi, true`.
   - `bestOnSegment(lo, hi, bx, by, sx, sy float64) float64`: returns `t ∈ [lo, hi]` minimizing `(|by - t·sy|, |bx - t·sx|)` lexicographically:
     - If `math.Abs(sy) >= 1e-12`: return `clampFit(by/sy, lo, hi)`.
     - If `math.Abs(sx) >= 1e-12`: return `clampFit(bx/sx, lo, hi)`.
     - Otherwise return `lo`.
4. Evaluate all 4 edges in fixed deterministic order (`a = a.lo`, `a = a.hi`, `b = b.lo`, `b = b.hi`):
   - For each edge, append up to 2 candidates:
     1. Full-edge lexicographic minimizer (`bestOnSegment(lo, hi, bx, by, sx, sy)`).
     2. Gate-clipped lexicographic minimizer: clip `[lo, hi]` by `|ry(t)| <= FitGateBrpYMm` and `|rx(t)| <= FitGateBrpXMm`; if non-empty, evaluate `bestOnSegment(gLo, gHi, bx, by, sx, sy)` clamped to `[lo, hi]`.
   - Each candidate records `x, y`, `absY = math.Abs(m.Y)`, `absX = math.Abs(m.X)`, and `inGate = absY <= FitGateBrpYMm+fitBoundTolMm && absX <= FitGateBrpXMm+fitBoundTolMm`.
5. Pick the winning candidate using strict better-than comparison (ties keep the earlier edge):
   - `c.inGate != best.inGate`: `c.inGate` wins.
   - `math.Abs(c.absY-best.absY) > fitBoundTolMm`: smaller `c.absY` wins.
   - `c.absX < best.absX - fitBoundTolMm`: smaller `c.absX` wins.
6. Report `limited` in `a.field` then `b.field` order:
   - If `math.Abs(best.x-a.lo) <= fitBoundTolMm || math.Abs(best.x-a.hi) <= fitBoundTolMm`, append `a.field`.
   - If `math.Abs(best.y-b.lo) <= fitBoundTolMm || math.Abs(best.y-b.hi) <= fitBoundTolMm`, append `b.field`.
   - Return `clampFit(best.x, a.lo, a.hi), clampFit(best.y, b.lo, b.hi), limited, true`.

### 2. `internal/reboot/assembly/fitting_test.go`

Update existing clamped-limit expectations (all values below measured by independent Python probe against Fenrir L + Lithic + SQlab 610):

- `TestApplyFittingFitReportBRPLimitedByRailOffset`:
  - `generic 0 mm` post, target `fitReportBRP (-219, 713.1)`:
    - `LimitedBy`: `[InputSaddleRailOffset]`, `rail = 15` (`tol 0`)
    - `insertion`: `197.387571` (`tol 1e-6`), `extension`: `400 - 197.387571` (`tol 1e-6`)
    - `miss.X`: `-53.368222` (`tol 1e-6`), `miss.Y`: `0` (`tol 1e-9`), `|miss|`: `53.368222` (`tol 1e-6`)
  - `setback = 27 mm` post, same target (update comment: setback lowers `RailCradle.Y` along local `+Y` at `195.29°`, so holding `ΔY = 0` pulls the post out to `190.006853 mm` insertion):
    - `LimitedBy`: `[InputSaddleRailOffset]`, `rail = 15` (`tol 0`)
    - `insertion`: `190.006853` (`tol 1e-6`), `miss.Y`: `0` (`tol 1e-9`), `|miss|`: `25.377598` (`tol 1e-6`)
- `TestApplyFittingIsMinimalOverGrid`:
  - Update the grid check to verify **gate-aware lexicographic `(|ΔY|, |ΔX|)` minimality** (no legal grid point `(ext, rail)` may beat `fit` on `(inGate, |ΔY|, |ΔX|)` by more than `1e-6`), and run it on `fitReportBRP` (outside gate), the gate-trade-off target below (inside gate), and the pitched-rails insertion-bound target below.
- `TestFitPostSetbackAtRailLimit` and `TestFitPostSetbackForwardTarget`:
  - Remove the Euclidean cross-checks that assumed `fitSaddle`'s `|miss|` equals `FitPostSetback`'s perpendicular setback (lines 474–476 and 529–532). `FitPostSetback` itself is unchanged and its own assertions remain intact.
- `TestApplyFittingReferencePosts`:
  - `generic 0 mm`: `insertion = 197.387571`, `miss = 53.368222` (`tol 1e-6`), `miss.Y = 0` (`tol 1e-9`)
  - `generic with synthetic 27 mm setback`: `insertion = 190.006853`, `miss = 25.377598` (`tol 1e-6`), `miss.Y = 0` (`tol 1e-9`)
  - `Redshift`: `insertion = 144.107252`, `miss = 40.927945` (`tol 1e-6`), `miss.Y = 0` (`tol 1e-9`)
- `TestFitPostSetbackRedshiftForwardLimit`:
  - `fit := mustFit(t, setup, pose.Point{X: -150, Y: 758.1})` on Redshift: `rail = -15` (`tol 1e-9`), `insertion = 100` (`tol 1e-9`), `LimitedBy = [InputSeatpostInsertion, InputSaddleRailOffset]`, `miss.X = 9.702493` (`tol 1e-6`), `miss.Y = 2.453759` (`tol 1e-6`). (`FitPostSetback` assertions on lines 626–638 stay unchanged.)

Add new unit tests in `internal/reboot/assembly/fitting_test.go`:

1. `TestApplyFittingGateAwareTradeOff` (Q74):
   - Setup: `fenrirFittingSetup()` (generic 0 mm post, level rails).
   - Reference edge point: `ins = 200`, `rail = +15` → BRP `(-164.942919, 710.580028)`.
   - **Gate-passing target** `(-175.242919, 710.580028)` (`10.3 mm` rearward of edge at same `Y`):
     - Exact `Y` would miss by `ΔX = -10.3 mm` (outside `±10 mm` gate).
     - `fitSaddle` trades height within `|ΔY| ≤ 2.0 mm` to enter the gate:
       - `Outcome = FitLimited`, `LimitedBy = [InputSaddleRailOffset]`
       - `insertion = 198.862281` (`tol 1e-5`), `rail = 15` (`tol 0`)
       - `miss.X = -10.0` (`tol 1e-6`), `miss.Y = -1.097454` (`tol 1e-5`)
       - Passes BRP gate: `|miss.Y| <= FitGateBrpYMm` and `|miss.X| <= FitGateBrpXMm + 1e-9`.
   - **Just-outside-gate target** `(-175.542919, 710.580028)` (`10.6 mm` rearward of edge at same `Y`):
     - Reaching `|ΔX| = 10.0 mm` would require `|ΔY| = 2.194908 mm > 2.0 mm`, so no point in the box passes the gate.
     - Falls back to exact `Y`:
       - `Outcome = FitLimited`, `LimitedBy = [InputSaddleRailOffset]`
       - `insertion = 200.0` (`tol 1e-5`), `rail = 15` (`tol 0`)
       - `miss.X = -10.6` (`tol 1e-5`), `miss.Y = 0` (`tol 1e-9`).
2. `TestApplyFittingPitchedRailsInsertionBound` (Q68):
   - Setup: `fenrirFittingSetup()` with `Saddle.RailPitchDeg = 5`.
   - Target `(-180.355237, 819.658647)` (`10 mm` above the `ins = 100, rail = 0` BRP, outside gate).
   - Sliding rails forward (`rail = -15`) raises BRP by `15·sin(5°) = 1.307336 mm`, minimizing `|ΔY|` first even though `|ΔX|` grows:
     - `Outcome = FitLimited`, `LimitedBy = [InputSeatpostInsertion, InputSaddleRailOffset]`
     - `insertion = 100` (`tol 0`), `rail = -15` (`tol 0`)
     - `miss.X = -14.942920` (`tol 1e-5`), `miss.Y = 8.692664` (`tol 1e-5`).
3. `TestApplyFittingDegenerateBox`:
   - Single-value insertion (`MaxPostInsertionMm = 100` so `EffectiveMinInsertionMm == MaxAllowedInsertionMm == 100`) and zero rail travel (`RailLengthMm = 30`, `RailClampLengthMm = 30` so `railLimit == 0`).
   - Verify an unreachable target returns `Outcome = FitLimited`, `LimitedBy = [InputSeatpostInsertion, InputSaddleRailOffset]`, `insertion = 100`, `rail = 0`, finite miss, and no NaN/error.
   - Verify the exact reachable point `(ins = 100, rail = 0)` returns `Outcome = FitReached`, `LimitedBy = nil`, `|miss| < 1e-9`.
4. `TestFitGateConstants` (Q59a):
   - Assert `FitGateGripR == 1.0`, `FitGateBrpYMm == 2.0`, `FitGateBrpXMm == 10.0`.

### 3. `internal/reboot/assembly/stock_fitting_test.go`

Update clamped BRP miss, insertion, and score expectations (all values independently probed; non-BRP penalties and `GripR` are unchanged):

- `TestStockFittingRefitsSaddlePerFork`:
  - `wantMiss`: `{53.368222, 53.311080, 56.197798, 59.087687, 41.221376, 44.669901, 48.122963}` (`tol 1e-6`)
  - `wantInsertion`: `{197.387571, 197.372502, 198.128305, 198.873690, 194.085981, 195.043299, 195.986030}` (`tol 1e-6`)
  - Top-5 `requireStockForkRanks` scores:
    - `{2, stockStem{-17, 130, 0}, 1.019761, -24.230340}`
    - `{0, stockStem{-17, 130, 0}, 1.358546, -26.024462}`
    - `{5, stockStem{-17, 130, 0}, 2.401938, -31.582448}`
    - `{2, stockStem{-17, 130, 5}, 1.412418, -32.868795}`
    - `{0, stockStem{-17, 130, 5}, 1.755355, -34.754251}`
- `TestStockFittingPenalizesBrpMissForSubmittedFork`:
  - `BRP miss points`: `104.736445` (`tol 1e-6`)
  - `score`: `-26.024462` (`tol 1e-6`)

### 4. `internal/reboot/web/web_test.go`

Update clamped BRP expectations in the 6 affected web tests (all values independently probed):

- `TestBikeWorkbenchFitsFitReportBrpAtRailLimit`:
  - `fit-insertion`: `"197.4 mm"`
  - `apply post_insert`: `197.387571` (`tol 1e-6`)
- `TestBikeWorkbenchBothFitTargets`:
  - `apply post_insert`: `197.387571` (`tol 1e-6`)
- `TestBikeWorkbenchStockFitKeepsSaddleFitAndTargets`:
  - `row 1 score`: `"-26.0"`
  - `stock apply post_insert`: `197.387571` (`tol 1e-6`)
- `TestBikeWorkbenchSetbackNotesForwardTarget`:
  - `fit-saddle-outcome`: `"limited by seatpost insertion, saddle rail offset"` (holding `ΔY = 0` on Redshift at `rail = -15` hits `MinInsertion = 100 mm`)
  - All `fit-setback-*` assertions stay unchanged (`FitPostSetback` is unchanged).
- `TestBikeWorkbenchStockForksWithSaddle`:
  - Top-5 rows `{"fork", "stem", "spacers", "score", "brp-miss"}` + `"BRP miss"` penalty column:
    - `{stockForkSID20, "130 mm, -17°", "0 mm", "-24.2", "56.2 mm", "110.4"}`
    - `{stockForkLithic, "130 mm, -17°", "0 mm", "-26.0", "53.4 mm", "104.7"}`
    - `{stockForkFox20, "130 mm, -17°", "0 mm", "-31.6", "44.7 mm", "87.3"}`
    - `{stockForkSID20, "130 mm, -17°", "5 mm", "-32.9", "56.2 mm", "110.4"}`
    - `{stockForkLithic, "130 mm, -17°", "5 mm", "-34.8", "53.4 mm", "104.7"}`
  - `row 1 Apply post_insert`: `198.128305` (`tol 1e-6`)
- `TestBikeWorkbenchStockForksAndBars`:
  - Top-5 scores (same 5 Fox 20 rows): `"-6.7"`, `"-7.8"`, `"-8.7"`, `"-8.7"`, `"-8.7"`

## Commit split

One atomic commit (the code change shifts clamped-limit test expectations across `assembly` and `web`, so they land together to keep `make pre-commit` green):

1. **`Use gate-aware height-first saddle clamping`**
   - Files: `internal/reboot/assembly/fitting.go`, `internal/reboot/assembly/fitting_test.go`, `internal/reboot/assembly/stock_fitting_test.go`, `internal/reboot/web/web_test.go`.

## Out of scope

- Replacing `PenaltyBand` with `0–100` weighted scoring or 4-tier ranking (Slice 13b).
- `stems.yaml` catalog port (Slice 13b).
- Changing `fitCockpit` or `FitPostSetback` (they keep Euclidean `fitBox`).
- Any changes to `internal/reboot/web` production code, templates, or URL parameters.
- Radial BB-to-saddle height fitting (deferred follow-up per Q60).

## Rules that bite

- **Plain-language commit messages:** no Conventional Commits prefixes (`feat:`, `fix:`, etc.).
- **No raw trig outside `internal/reboot/pose`:** `fitting.go` uses `pose.DecomposeAlong` and `pose.Along`; `math.Abs` is fine.
- **No banned words (`make boundary`):** never write `chassis`, `price`, `pricing`, `cost`, `cheap`, or `ledger` in any reboot file (including comments and tests).
- **No legacy imports:** reboot packages never import legacy `bikegeo/...` packages.
- **Zero runtime dependencies:** `go list -deps` stays stdlib + `bikegeo/...`.
- **Verification:** run `make pre-commit` and ensure it exits 0 cleanly (re-running if `go fix` / `go fmt` changes anything). No live browser check is required for Slice 13a because no HTML/SVG/template code changes (note this in the handoff).
- **Implementation handoff (required):** Before finishing, update this file's `Status:` line to `Implemented <date>; awaiting Matt's acceptance` (or `Partly implemented` / `Blocked`), add an `## Implementation handoff` section right after the Status paragraph following [DESIGN.md](DESIGN.md) ("Implementation handoff"), and update the Slice 13a Milestones line in [DESIGN.md](DESIGN.md) with the same status words (touching nothing else in `DESIGN.md`).
