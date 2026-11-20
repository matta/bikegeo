# M2 Slice 3, stock cockpit fitting

**Status: Implemented in `8e4bf0f` and accepted by Matt on 2026-11-19.** This plan records the implementation of rulings Q15–Q18 from the [reboot design](DESIGN.md). A later cleanup dropped the unused `LedgerLine.Value` and `stockCandidateView.PointsLost` fields and moved the Apply and Total columns next to the rank, so the snippets below differ from the code there.

## 1. Goal

Given a BikeSetup and a **HandGrip** target (world mm: BB = (0,0), +X forward, +Y up), choose a **stock** cockpit: a stem length, a stem angle, and a spacer stack from a short stock list. Place every stock combination forward with `PlaceBike`, price each on the **points-lost ledger**, and return the 5 cheapest builds. The `/bike` workbench shows them in a table, and each row has its own apply link.

- **Saddle first (Q15).** The saddle fit is primary and must be right. Stock cockpit fitting never changes it. When a BRP target is also set, every stock build carries the values from `ApplyFitting`'s saddle fit (an exact continuous fit, unchanged from Slice 1).
- **Enumerate and place (Q16).** No direct solve and no rounding of the continuous answer. Forward assembly is the evaluator, and every listed build is a `PlaceBike` result.
- **Oval price (Q17).** The HandGrip miss is priced with the legacy oval, as described in §3.
- **Ranked list (Q18).** List the 5 cheapest builds. Slice 2's continuous cockpit fitting and its "Apply fitted build" link stay exactly as they are.

Fixed inputs: frame, fork, cup, wheels, `UpperCoverMm`, `ClampHeightMm`, the stem length range and spacer allowance on `StemSetup` (Q13), and the whole handlebar, including roll. Handlebar choice is a later slice.

## 2. Terminology (Q8, mandatory)

- The operation is **stock fitting** (noun) and **fit** (verb). The entry point is **`ApplyStockFitting`** and the result type is **`StockFitting`**. Never write "search", "solver search", "optimize", or "resolve" for it in code, comments, test names, or docs.
- The target landmark is **HandGrip**. The priced region around it is the **oval** (in the UI, "HandGrip oval").
- The score is the **points-lost ledger**: each priced band contributes points lost, and `Total = 100 − Σ points lost`. There are no weights, caps, clamps, or rounding, and totals may go below 0.

## 3. The model

### Stock list (`CockpitStock`)

- **Stem lengths:** `Stem.MinLengthMm + i·StemLengthStepMm` for `i = 0, 1, …`, keeping each value `≤ Stem.MaxLengthMm + 1e-9`. A value in the `1e-9` slack above the max is replaced by `MaxLengthMm`. The fixture gives 40, 50, …, 130 (10 values).
- **Stem angles:** `StemAnglesDeg`, in the listed order. The fixture gives −17°, −6°, 0°, +6°, +17° (5 values).
- **Spacers:** `0 + i·SpacerStepMm` up to `Stem.MaxSpacersMm`, with the same slack rule. The fixture gives 0, 5, …, 40 (9 values). If `MaxSpacersMm` is 0, the only stack is 0.
- Compute every value as `lo + float64(i)*step`. **Never accumulate** (`v += step`), so 130 and 40 come out exact.
- **Enumeration order:** angles in list order (outer loop), then lengths ascending, then spacers ascending (inner loop). The fixture has 5 × 10 × 9 = **450** combinations.

### Prices (`CockpitPreferences`)

Each band is free inside its range and linear outside it:

```text
PricedBand.PointsLost(v) = CostBelow·(FreeMin − v)   if v < FreeMin
                         = CostAbove·(v − FreeMax)   if v > FreeMax
                         = 0                         otherwise
```

The oval (Q17) has semi-axes `SemiMinorMm` and `SemiMajorMm`, and its major axis points along world heading `MajorAxisHeadingDeg`:

```text
d     = HandGrip − target
minor = ProjectOnto(d, MajorAxisHeadingDeg − 90°) / SemiMinorMm
major = ProjectOnto(d, MajorAxisHeadingDeg)       / SemiMajorMm
R     = |(minor, major)|
oval points lost = CostPerR · max(0, R − 1)
```

**Legacy check:** legacy `geom2d.Ellipse2.Evaluate` tilts the major axis 30° clockwise from vertical. That axis is the unit vector `(sin 30°, cos 30°)`, at world heading 60°. Its minor axis is `(cos 30°, −sin 30°)`, at heading −30° = 60° − 90°. So the fixture's `MajorAxisHeadingDeg = 60` reproduces the legacy `pMajor`/`pMinor` exactly.

Fixture values. Only the oval comes from a ruling; the other three bands are illustrative values Matt may retune:

| Band | Value priced | Free range | Price outside |
| --- | --- | --- | --- |
| HandGrip oval | R | R ≤ 1 (semi-axes 10 mm × 20 mm, major axis at 60°) | 22 points per unit R |
| stem length | mm | 70–80 mm | 0.2 points/mm on both sides |
| stem angle | degrees | 0° | 0.2 points/° on both sides |
| spacers | mm | 0–20 mm | 0.4 points/mm above (0 below) |

### Ranking

Sort candidates by points lost, ascending. Break ties by oval R, ascending, then by enumeration order (use a **stable** sort with exact `cmp.Compare` on both keys; no tolerance comparator). Keep the first `limit` candidates (5 in the workbench). `Evaluated` reports how many combinations were placed (450).

### Why there's no direct solve here

Slice 2's 2×2 solve explains the continuous answer. Q16 rules that the stock choice comes from forward placement of every combination. One `PlaceBike` takes about 2.6 µs, so all 450 take about 1.3 ms, which is fine for a page request. Don't add a cheaper partial placement (stem and handlebar only): that would be a second assembly path.

## 4. Code changes: one commit, two stages

`make deadcode` fails on an exported assembly function that nothing in `cmd/bikegeo-reboot` reaches. So the assembly code and its workbench caller land in **one commit**. Work in two stages. After Stage A, `go vet ./... && go test ./...` must pass; `make deadcode` is expected to fail until Stage B is done. Run `make pre-commit` only at the end.

Commit subject: `Rank stock cockpit builds for a HandGrip target in the reboot workbench`.

### Stage A: `assembly`

New files: `internal/reboot/assembly/stock_fitting.go` and `stock_fitting_test.go`. Edit `fitting.go` for the small refactor in A1.

**A1. Shared stem length check (refactor, no behavior change).** In `fitting.go`, move the stem length range check out of `ApplyFitting` into:

```go
// validateStemLengthRange checks the stem length range fitting may choose.
func validateStemLengthRange(stem StemSetup) error
```

Keep the exact message, `"stem length range must be finite with 0 < min <= max, got [%v, %v] mm"`. `ApplyFitting` calls it where the inline check was. Every existing test must pass unchanged.

**A2. Types and methods** in `stock_fitting.go`:

```go
// CockpitStock lists the stock cockpit parts stock fitting may choose from.
// Stem lengths run from Stem.MinLengthMm to Stem.MaxLengthMm in StemLengthStepMm
// steps; spacer stacks run from 0 to Stem.MaxSpacersMm in SpacerStepMm steps.
type CockpitStock struct {
	StemLengthStepMm float64
	StemAnglesDeg    []pose.Deg
	SpacerStepMm     float64
}

// Validate rejects non-finite or non-positive steps and an empty or non-finite angle list.
func (s CockpitStock) Validate() error

// PricedBand prices one value on the points-lost ledger: free inside
// [FreeMin, FreeMax], then CostBelow points per unit below FreeMin and
// CostAbove points per unit above FreeMax, linear with no cap.
type PricedBand struct {
	FreeMin, FreeMax     float64
	CostBelow, CostAbove float64
}

// PointsLost returns the points lost by v under the band.
func (b PricedBand) PointsLost(v float64) float64

// FitOval is the legacy Area of Reasonable Fit around a HandGrip target (Q17):
// an ellipse with semi-axes SemiMinorMm and SemiMajorMm whose major axis points
// along world heading MajorAxisHeadingDeg. It is free inside (R ≤ 1) and costs
// CostPerR points per unit of normalized radius R beyond the edge.
type FitOval struct {
	SemiMinorMm, SemiMajorMm float64
	MajorAxisHeadingDeg      pose.Deg
	CostPerR                 float64
}

// NormalizedRadius returns R for p around center (R = 1 on the oval's edge).
func (o FitOval) NormalizedRadius(center, p pose.Point) float64 {
	d := p.Sub(center)
	minor := pose.ProjectOnto(d, o.MajorAxisHeadingDeg-90) / o.SemiMinorMm
	major := pose.ProjectOnto(d, o.MajorAxisHeadingDeg) / o.SemiMajorMm
	return pose.Point{X: minor, Y: major}.Norm() // length of the normalized components
}

// PointsLost returns CostPerR · max(0, r − 1).
func (o FitOval) PointsLost(r float64) float64

// CockpitPreferences prices a stock cockpit build on the points-lost ledger.
type CockpitPreferences struct {
	GripOval   FitOval
	StemLength PricedBand // mm
	StemAngle  PricedBand // degrees
	Spacers    PricedBand // mm
}

// Validate rejects non-finite values, negative costs, FreeMin > FreeMax, and
// non-positive oval semi-axes. Messages name the band ("stem length band: …").
func (p CockpitPreferences) Validate() error

// LedgerBand names one priced line of a stock cockpit ledger.
type LedgerBand string

const (
	BandGripOval   LedgerBand = "HandGrip oval"
	BandStemLength LedgerBand = "stem length"
	BandStemAngle  LedgerBand = "stem angle"
	BandSpacers    LedgerBand = "spacers"
)

// LedgerLine is one priced value: R for the oval, mm or degrees otherwise.
type LedgerLine struct {
	Band       LedgerBand
	Value      float64
	PointsLost float64
}

// StockCandidate is one stock cockpit build, placed forward and priced.
type StockCandidate struct {
	Placement BikePlacement // PlaceBike(candidate setup); Placement.Setup is the candidate setup
	Target    pose.Point    // the HandGrip target it was priced against
	GripR     float64       // oval normalized radius of the placed HandGrip
	Ledger    []LedgerLine  // always BandGripOval, BandStemLength, BandStemAngle, BandSpacers
}

// PointsLost returns the sum of the ledger's points lost, in ledger order.
func (c StockCandidate) PointsLost() float64

// Total returns 100 − PointsLost().
func (c StockCandidate) Total() float64

// GripMiss returns target − placed HandGrip (world mm).
func (c StockCandidate) GripMiss() pose.Point

// StockFitting is the result of ApplyStockFitting.
type StockFitting struct {
	Candidates []StockCandidate // cheapest first; at most the requested limit
	Evaluated  int              // stock combinations placed and priced
}
```

Use `pose.ProjectOnto`, `Sub`, and `Norm`. Don't use `math.Hypot`, trig, or radians in `assembly` (AGENTS.md trig rule). `math.Max`, `math.Min`, `math.IsNaN`, and `math.IsInf` are fine. `PricedBand` gets an unexported `validate(name string) error` that `CockpitPreferences.Validate` calls for each band.

**A3. Entry point:**

```go
// ApplyStockFitting chooses stock cockpit parts (stem length, stem angle, and
// spacer stack from stock) for target.HandGrip. It places every combination
// with PlaceBike, prices each on the points-lost ledger with prefs, and returns
// the cheapest limit candidates (ties: smaller oval R, then enumeration order).
// The saddle fit comes first (Q15): when target.BRP is set, the saddle inputs
// are fitted with ApplyFitting and kept in every candidate. All other setup
// fields are kept.
func ApplyStockFitting(setup BikeSetup, target FitTarget, stock CockpitStock, prefs CockpitPreferences, limit int) (StockFitting, error)
```

Steps, in this order:

1. If `target.HandGrip == nil`, return `fmt.Errorf("stock fitting needs a HandGrip target")`. If it isn't finite, return `invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", …)`, the same message `ApplyFitting` uses.
2. If `limit < 1`, return `fmt.Errorf("stock fitting needs a positive candidate limit, got %d", limit)`.
3. Return the first error from `stock.Validate()`, `prefs.Validate()`, and `validateStemLengthRange(setup.Stem)`.
4. `base := setup`. If `target.BRP != nil`, call `saddle, err := ApplyFitting(setup, FitTarget{BRP: target.BRP}, nil)`, return `err` unchanged, and set `base = saddle.Placement.Setup`. This step also validates the BRP target.
5. Call `PlaceBike(base, nil)` once and return its error unchanged. This rejects a bad base setup (for example, `MaxSpacersMm < 0`) before the enumeration.
6. `grip := *target.HandGrip`. Enumerate in the §3 order. For each combination, set `cand := base`, then set `cand.Stem.AngleDeg`, `cand.Stem.LengthMm`, and `cand.Stem.SpacersMm`. Call `p, err := PlaceBike(cand, nil)` and return any error unchanged. Then:
   - `r := prefs.GripOval.NormalizedRadius(grip, p.Handlebar.HandGrip.Origin())`
   - Ledger lines in this order: oval `{BandGripOval, r, prefs.GripOval.PointsLost(r)}`; `{BandStemLength, LengthMm, prefs.StemLength.PointsLost(LengthMm)}`; `{BandStemAngle, float64(AngleDeg), prefs.StemAngle.PointsLost(float64(AngleDeg))}`; `{BandSpacers, SpacersMm, prefs.Spacers.PointsLost(SpacersMm)}`.
7. Sort with `slices.SortStableFunc` (`cmp.Compare` on `PointsLost()`, then on `GripR`), set `Evaluated` to the full count, and truncate to `limit`. If `limit` is greater than the count, return them all.

Keep the enumeration helper unexported, for example `stockSteps(lo, hi, step float64) []float64`.

**A4. Tests (`stock_fitting_test.go`, package `assembly_test`).** Add helpers that copy the fixture values, because assembly tests must not import the web fixture: `fenrirCockpitStock()` and `illustrativeCockpitPreferences()` (§3 table), plus `mustStockFit(t, setup, target, limit)`. Reuse `fenrirFittingSetup`, `gripOf`, `fitReportBRP`, and `fitReportGrip` from `fitting_test.go`. Independent math in tests may use `math.Sin`, `math.Cos`, and `math.Hypot` directly. Use the §5 numbers.

- **PricedBand:** with `{70, 80, 0.2, 0.2}`, 60 → 2.0, 70 → 0, 75 → 0, 80 → 0, and 130 → 10.0, each within 1e-12. With `{0, 20, 0, 0.4}`, 40 → 8.0.
- **FitOval:** at the center, R = 0. A point 20 mm along heading 60° gives R = 1, and 10 mm along heading −30° gives R = 1, both within 1e-12. 30 mm along −30° gives R = 3, and `PointsLost(3) = 44`. **Legacy formula:** for a few offsets `(dx, dy)`, R equals `hypot((dx·cos30° − dy·sin30°)/10, (dx·sin30° + dy·cos30°)/20)` within 1e-12.
- **Free stock build wins exactly:** the target is `gripOf` the setup with stem 0°, 70 mm, and 10 mm spacers. Rank 1 is (0°, 70, 10), `GripR < 1e-9`, `PointsLost() == 0`, and `Total() == 100`. `Evaluated == 450` and `len(Candidates) == 5`. Don't assert the order of ranks 2 and 3: (0°, 70, 5) and (0°, 70, 15) tie in exact arithmetic.
- **Default build's grip:** the target is `gripOf(fenrirFittingSetup())`, about (389.218, 755.120). Ranks 1–4 are (0°, 70, 20), (0°, 70, 15), (0°, 70, 10), and (+6°, 70, 10). Their R values are ≈ 0.615497, 0.694523, 0.972410, and 0.598669 (1e-5). Points lost are 0, 0, 0, and 1.2 (1e-9).
- **Fit report grip (481, 726):** rank 1 is (−17°, 130, 0), with R ≈ 1.358546, ledger points ≈ (7.888017, 10.0, 3.4, 0.0) (1e-5), `Total` ≈ 78.711983 (1e-5), and |GripMiss| ≈ 13.624 (1e-3). That matches Slice 2's limited −17° continuous fit. Rank 2 is (−17°, 130, 5) with `Total` ≈ 69.982194. Rank 3 is (−17°, 120, 0) with `Total` ≈ 62.174803.
- **Order invariant:** with `limit = 450`, all 450 come back. `PointsLost` is non-decreasing, and within equal `PointsLost`, `GripR` is non-decreasing.
- **Forward check:** for every candidate in the 450, `reflect.DeepEqual(c.Placement, PlaceBike(c.Placement.Setup, nil))`. Copy the candidate's setup, put the base stem's `SpacersMm`, `LengthMm`, and `AngleDeg` back, and the result `reflect.DeepEqual`s the base setup. Each candidate's ledger bands are in the A2 order.
- **Oracle:** brute-force all 450 in the test with `PlaceBike` and independent pricing math, sort the points-lost values, and check that the five smallest equal the returned candidates' `PointsLost()` values within 1e-9. Also check that the oracle's cheapest combination is rank 1. Use the fit-report grip and the default grip.
- **Saddle first:** with `FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}`, every candidate's `Seatpost.ExtensionMm` and `Saddle.RailOffsetMm` `==` the values from `ApplyFitting(setup, FitTarget{BRP: &fitReportBRP}, nil)`. The candidates' stem values, in order, `==` those from the HandGrip-only run (the frameset doesn't depend on the saddle).
- **Fixed inputs kept:** every candidate keeps `UpperCoverMm`, `ClampHeightMm`, `MaxSpacersMm`, `MinLengthMm`, `MaxLengthMm`, and the whole `Handlebar` from the input.
- **MaxSpacersMm = 0** (with base spacers 0): `Evaluated == 50` and every candidate has 0 spacers.
- **Errors:** no HandGrip → error. A NaN grip → `*InputError` with `InputFitTargetGrip`. A NaN BRP → `*InputError` with `InputFitTargetBRP`. `limit = 0` → error. Stock steps of 0, −5, or NaN → error. An empty angle list → error. A NaN angle → error. Prefs with `SemiMinorMm = 0`, a negative cost, `FreeMin > FreeMax`, or a NaN field → error. `MinLengthMm = 0` → error. `MaxSpacersMm = −1` → `PlaceBike`'s error, unchanged (compare `err.Error()` with `PlaceBike`'s own message).

### Stage B: `/bike` workbench

Files: `internal/reboot/web/fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`.

**B1. Fixture.** Add two fields to `bikeFixture` and fill them in `otsoFenrirFixture()`:

```go
CockpitStock: assembly.CockpitStock{
	StemLengthStepMm: 10.0,                          // illustrative: stock stems every 10 mm over DefaultStem's length range
	StemAnglesDeg:    []pose.Deg{-17, -6, 0, 6, 17}, // illustrative: common stock angles
	SpacerStepMm:     5.0,                           // illustrative: 5 mm spacers
},
CockpitPreferences: assembly.CockpitPreferences{
	GripOval:   assembly.FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60, CostPerR: 22}, // legacy idmatch oval (Q17)
	StemLength: assembly.PricedBand{FreeMin: 70, FreeMax: 80, CostBelow: 0.2, CostAbove: 0.2},             // illustrative
	StemAngle:  assembly.PricedBand{FreeMin: 0, FreeMax: 0, CostBelow: 0.2, CostAbove: 0.2},               // illustrative
	Spacers:    assembly.PricedBand{FreeMin: 0, FreeMax: 20, CostBelow: 0, CostAbove: 0.4},                // illustrative
},
```

In `bikeFixture.validate()`, return `fmt.Errorf("fixture %q cockpit stock invalid: %w", …)` and `"… cockpit preferences invalid: %w"` when the `Validate()` methods fail.

**B2. Handler.** Add `const stockCandidateLimit = 5 // Q18`. In `handleBikeWorkbench`, inside the existing `if resolved.State.Targets.any()` block, after `view.Fitting = buildFittingView(...)`:

```go
if resolved.State.Targets.HasGrip {
	stock, err := assembly.ApplyStockFitting(resolved.Setup, resolved.State.Targets.fitTarget(), fix.CockpitStock, fix.CockpitPreferences, stockCandidateLimit)
	if err != nil {
		renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
		return
	}
	view.Fitting.Stock = buildStockFittingView(resolved.State, fix.CockpitPreferences, stock)
}
```

Pass `resolved.Setup` (the submitted build), as `ApplyFitting` does. The diagram, trace, and form still show the submitted build.

**B3. View data.** `bikeFittingView` gains `Stock *stockFittingView // nil unless a HandGrip target is set`. Add:

```go
type stockFittingView struct {
	Evaluated      int
	PreferenceNote string
	Rows           []stockCandidateView
}

type stockCandidateView struct {
	Rank                    int
	StemLengthMm, SpacersMm float64
	StemAngleDeg            pose.Deg
	GripMissMm, GripR       float64
	Lines                   []assembly.LedgerLine // assembly's ledger order
	PointsLost, Total       float64
	ApplyURL                string // the candidate build, with the submitted targets kept
}
```

`buildStockFittingView` reads every value from the candidate: `Placement.Setup.Stem` for the stem and spacers, `GripMiss().Norm()`, `GripR`, `Ledger`, `PointsLost()`, and `Total()`. Never recompute a price or a position in `web`. `Rank` is the 1-based index. `ApplyURL` follows the existing pattern: `applied := workbenchStateFromSetup(state.View, state.ForkID, c.Placement.Setup); applied.Targets = state.Targets; applied.URL()`. The URL already carries `stem_angle`, so a stock row's angle travels with it.

`PreferenceNote` comes from `prefs`, through a helper `freeRangeText(b assembly.PricedBand, unit string) string`. It returns `"%.0f%s"` of `FreeMin` when `FreeMin == FreeMax`, else `"%.0f–%.0f%s"`. Pass the units `" mm"` and `"°"`. Format:

```go
fmt.Sprintf("Free: HandGrip inside the ±%.0f × ±%.0f mm oval, stem length %s, stem angle %s, spacers %s. Prices are illustrative.",
	o.SemiMinorMm, o.SemiMajorMm, freeRangeText(prefs.StemLength, " mm"), freeRangeText(prefs.StemAngle, "°"), freeRangeText(prefs.Spacers, " mm"))
```

With the fixture, the text is exactly: `Free: HandGrip inside the ±10 × ±20 mm oval, stem length 70–80 mm, stem angle 0°, spacers 0–20 mm. Prices are illustrative.`

**B4. Template.** Inside `<div data-fitting>`, after the `<div class="summary">` that holds `#apply-fitting`, add:

```html
{{with .Stock}}
<div class="stock-fitting" data-stock-fitting>
  <strong>Stock cockpit builds</strong>
  <p class="fit-note"><span data-stock-evaluated>{{.Evaluated}}</span> stock stem and spacer combinations placed; the cheapest {{len .Rows}} on the points-lost ledger (total = 100 − points lost). The saddle fit is kept.</p>
  <p class="fit-note" data-stock-preferences>{{.PreferenceNote}}</p>
  <div class="stock-scroll">
    <table class="stock-table">
      <thead>
        <tr><th>#</th><th>Stem</th><th>Spacers</th><th>HandGrip miss</th><th>Oval R</th><th>Oval pts</th><th>Length pts</th><th>Angle pts</th><th>Spacer pts</th><th>Total</th><th></th></tr>
      </thead>
      <tbody>
        {{range .Rows}}
        <tr data-stock-rank="{{.Rank}}">
          <td>{{.Rank}}</td>
          <td class="mono" data-stock-cell="stem">{{printf "%.0f mm, %+.0f°" .StemLengthMm .StemAngleDeg}}</td>
          <td class="mono" data-stock-cell="spacers">{{printf "%.0f mm" .SpacersMm}}</td>
          <td class="mono" data-stock-cell="grip-miss">{{printf "%.1f mm" .GripMissMm}}</td>
          <td class="mono" data-stock-cell="oval-r">{{printf "%.2f" .GripR}}</td>
          {{range .Lines}}<td class="mono" data-stock-points="{{.Band}}">{{printf "%.1f" .PointsLost}}</td>{{end}}
          <td class="mono" data-stock-cell="total"><strong>{{printf "%.1f" .Total}}</strong></td>
          <td><a href="{{.ApplyURL}}" data-preset="apply-stock" data-stock-apply>Apply</a></td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>
</div>
{{end}}
```

- The four "pts" headers follow assembly's ledger order (oval, stem length, stem angle, spacers). A web test pins that order.
- **Don't** use `data-metric` in this block. `collectMetrics` keys metrics by name, and the repeated rows would collide in the submit-idempotence tests.
- CSS: the page already styles generic `table`, `th`, and `td`. Add only `.stock-fitting { margin-bottom: 0.75rem; font-size: 0.85rem; color: var(--muted); }`, `.stock-fitting > strong { color: var(--text); }`, `.stock-scroll { overflow-x: auto; }`, and `table.stock-table td { white-space: nowrap; }`.
- No JS change: `a[data-preset]` navigation is generic, and the trace code only reads `table.trace-table`. If you find otherwise, add a `workbench_test.js` case.
- No SVG markers for candidates in this slice.

**B5. Tests** (`httptest` + `goquery`, per AGENTS.md). Add a helper `stockCell(row *goquery.Selection, name string) string` that trims the text of `[data-stock-cell=name]`.

- `/bike` with no targets → no `[data-stock-fitting]`. With a BRP target only (`brp_x=-219&brp_y=713.1`) → no `[data-stock-fitting]`.
- `grip_x=481&grip_y=726&view=stem` → 200, with exactly 5 `tr[data-stock-rank]` rows and `[data-stock-evaluated]` = "450". Row 1: stem "130 mm, -17°", spacers "0 mm", grip-miss "13.6 mm", oval-r "1.36", points in order "7.9", "10.0", "3.4", "0.0" (read the four `[data-stock-points]` cells in document order and check their `data-stock-points` attributes are "HandGrip oval", "stem length", "stem angle", "spacers"), total "78.7". Row 2 total "70.0" and row 3 total "62.2". Row 1's apply href has `stem_angle=-17.0`, `stem_length=130.0`, `spacers=0.0`, `grip_x=481.0`, `grip_y=726.0`, and no `brp_x`. The existing continuous block still says "limited by stem spacers, stem length".
- Exact text of `[data-stock-preferences]` (B3).
- Every row link has `data-preset="apply-stock"`, and there are 5 `a[data-stock-apply]`.
- **Free round trip:** target from `gripTestTarget(t, url.Values{"stem_angle": {"0"}, "stem_length": {"70"}, "spacers": {"10"}})` → row 1 stem "70 mm, +0°", spacers "10 mm", oval-r "0.00", total "100.0". Following row 1's apply href gives `fit-cockpit-outcome` "reached", `fit-grip-miss` "0.0 mm", and row 1 stem "70 mm, +0°" again.
- **Both targets** (`brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`) → row 1's apply href has `rail_offset=15.0`, `post_ext` ≈ 260.092 (0.01), `stem_angle=-17.0`, `stem_length=130.0`, `spacers=0.0`, and all four target keys. `#apply-fitting` is still the only element with that id.
- A fixture whose cockpit stock or preferences fail `Validate()` makes `fix.validate()` return an error (unit test on `bikeFixture.validate()` with a broken copy; no HTTP needed).
- Every existing Slice 1 and Slice 2 web test passes unchanged.

## 5. Reference numbers (default Fenrir build, measured 2026-11-19 with an independent probe)

Default build: Lithic fork, EC44 12 mm cup, spacers 20, stem 60 mm at −6°, Persuader 800 at 0° roll. Stock: lengths 40–130 by 10, angles −17/−6/0/+6/+17, spacers 0–40 by 5 (450 combinations). Prices are the §3 table.

| Target | Rank | Stem (angle, length, spacers) | HandGrip | \|miss\| | R | Points lost (oval, len, angle, spacers) | Total |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Fit report (481, 726) | 1 | −17°, 130, 0 | (468.658, 731.769) | 13.624 | 1.358546 | 7.888017, 10, 3.4, 0 | 78.711983 |
| | 2 | −17°, 130, 5 | (466.762, 736.395) | 17.629 | 1.755355 | 16.617806, 10, 3.4, 0 | 69.982194 |
| | 3 | −17°, 120, 0 | (458.701, 730.847) | 22.820 | 2.201145 | 26.425197, 8, 3.4, 0 | 62.174803 |
| Default grip (389.218, 755.120) | 1 | 0°, 70, 20 | (396.396, 764.841) | 12.084 | 0.615497 | 0, 0, 0, 0 | 100 |
| | 2 | 0°, 70, 15 | (398.292, 760.214) | 10.407 | 0.694523 | 0, 0, 0, 0 | 100 |
| | 3 | 0°, 70, 10 | (400.189, 755.588) | 10.981 | 0.972410 | 0, 0, 0, 0 | 100 |
| | 4 | +6°, 70, 10 | (397.059, 762.213) | 10.573 | 0.598669 | 0, 0, 1.2, 0 | 98.8 |
| Free build 0°/70/10 (400.189, 755.588) | 1 | 0°, 70, 10 | same | 0 | 0 | 0, 0, 0, 0 | 100 |

Two findings worth noting (not defects). On the default build, the default stem (60 mm at −6°) costs 3.2 points against its own HandGrip, because a free 70 mm 0° stem lands inside the oval. For the fit report grip, the best stock build is still 13.6 mm short: it's the same corner (0 spacers, 130 mm) that limits Slice 2's continuous fit at −17°. If your numbers differ by more than the stated tolerances, find out why before changing a test.

## 6. Not in this slice

Handlebar choice or roll choice; preference costs on the saddle (Q15 keeps the saddle fit exact); a target relative to the saddle; LimitPoints-style bands priced against hardware limits; SVG markers for candidates or the oval; other frames; CLI/MCP access; changing `ApplyFitting`.

## 7. Rules that bite here

- **Q16:** every listed build is a `PlaceBike` result. Don't add a second, partial assembly path for speed.
- **Trig rule:** no trig, radians, or `math.Hypot` in `assembly` or `web`. The oval uses `pose.ProjectOnto` and `Point.Norm`. If you think you need another vector operation, add it to `pose` with a test (none should be needed).
- The web never recomputes a price, a position, or a limit. Every number comes from the `StockCandidate` or `bike.Setup`.
- `exhaustive`: if you switch on `LedgerBand`, `InputField`, or `FitOutcome`, name every constant. (The plan doesn't need such a switch.)
- `deadcode`: every new export must be reached from `cmd/bikegeo-reboot` once Stage B is done (`Validate` methods via the fixture and `ApplyStockFitting`; `PointsLost`, `Total`, and `GripMiss` via the view builder; `NormalizedRadius` and the band `PointsLost` via `ApplyStockFitting`). Test-only helpers go in `_test.go` files.
- `boundary`: no legacy imports, and never write the banned term listed in AGENTS.md.
- Placements must be identical with tracing on or off. (Stock fitting places untraced; the workbench trace still comes from the submitted build.)
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 3 done at the end, and only after Matt accepts it.

## 8. Verification

After Stage A, run `go vet ./... && go test ./...`. After Stage B, run `make pre-commit` and re-run it until `go fix` is clean. Then check it live: `make serve-reboot`, open `http://127.0.0.1:8081/bike?grip_x=481&grip_y=726&view=stem` and `…&brp_x=-219&brp_y=713.1&view=bike`. Confirm the stock table (5 rows, row 1 at −17°/130 mm/0 mm, total 78.7) and the unchanged continuous blocks. Click row 1's Apply and confirm the stem view shows a 130 mm stem at −17° with the saddle unchanged. Take a screenshot for Matt.
