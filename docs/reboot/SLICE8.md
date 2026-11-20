# M2 Slice 8, fork choice in stock fitting

**Status: Planned 2026-11-19 from Matt's rulings Q31–Q34 ([DESIGN.md](DESIGN.md), "Seatposts, bar sweep, and fork choice"). Not started.** Q34 (ruled 2026-11-19): the BRP miss line always counts. Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. This plan **builds on Slices 6 and 7** ([SLICE6.md](SLICE6.md), [SLICE7.md](SLICE7.md)). Don't start it until both are committed: it extends Slice 7's 5-line ledger and keeps Slice 6's `post` on every link.

This slice has two commits and touches 6 code, test, and template files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Today stock fitting always keeps the submitted fork, sag, and lower cup. After this slice:

- `assembly.CockpitStock` gains an optional list of fork configurations (a fork at a sag, with its lower cup). When it is non-empty, stock fitting enumerates each one, outermost. For each fork it refits the saddle when there is a BRP target (Q15: the saddle is fitted first, now per fork), and then enumerates bars, stem angles, lengths, and spacers as today.
- `assembly.CockpitPreferences` gains two bands. **ForkSag** is free at exactly 20% and penalizes 2 points per percent away from it, for suspension forks only (Q32). **BrpMiss** is free up to 1 mm of remaining saddle BRP miss and penalizes 2 points per mm beyond that, whenever there is a BRP target (Q32, Q34, illustrative).
- The ledger becomes 7 lines: oval, stem length, stem angle, spacers, bar sweep, **fork sag**, **BRP miss**. Both new lines are always present. Fork sag is 0 for a rigid fork. BRP miss is 0 without a BRP target.
- **Q34:** the BRP miss line scores the saddle fit itself, not the fork choice, so it counts on every ledger with a BRP target, including `stock_forks=selected`. There every row has the same miss: with the 0 mm post and the fit report BRP, every selected-mode total drops by 77.227222 points, and the ranking doesn't change. The existing-test updates marked **(Q34)** below follow from this.
- `/bike` gets a **Stock fitting forks** select, `stock_forks=selected|all` (Q33). `all` offers the 7 fixture fork configurations (Q31): Lithic (rigid), and RockShox SID and Fox 34, each at 15, 20, and 25% sag. Each stock row names its fork, and its Apply link sets `fork`, `fork_a2c`, `cup`, and `sag`.

Rulings:

- **Q31:** fixture forks at catalog A2C and default cup. Rigid at 0% sag, suspension at 15, 20, and 25%. No CAD Baseline Fork.
- **Q32:** forks free except for sag (free at 20%, penalized 2 points per percent, suspension only). BRP miss free to 1 mm, then penalized 2 points per mm.
- **Q34:** the BRP miss line counts whenever there is a BRP target, whatever the cause of the miss.
- **Q33:** `stock_forks=selected|all`, default `selected`, omitted from URLs.
- **Q15, Q16:** saddle first, now per fork. Every candidate comes from PlaceBike.

## 2. Terminology

- **Fork configuration**: a fork at a sag with its lower cup (`StockFork`). The URL key is `stock_forks`; the control is `#ctrl-stock-forks`, labelled "Stock fitting forks".
- **Frameset** is the leveled frame + fork + lower cup + wheels. Don't write the term AGENTS.md bans for it.
- **Stock fitting** / **fit** (Q8). Never write "search".

## 3. Commit 1: assembly (`internal/reboot/assembly`)

Files: `stock_fitting.go`, `stock_fitting_test.go`.

Commit subject: `Let reboot stock fitting choose the fork and sag`.

### A1. Types (`stock_fitting.go`)

```go
// StockFork is one fork configuration stock fitting may install: a fork at a
// sag with its lower headset cup (Q31).
type StockFork struct {
	Fork     ForkSetup
	LowerCup LowerCupSetup
}
```

`CockpitStock` gains `Forks []StockFork` after `Handlebars`. Append to its doc comment: "Forks, when non-empty, lists the fork configurations stock fitting may install. Empty keeps the submitted fork and lower cup (Q33)." `Validate()` adds, after the bar loop:

```go
for i, f := range s.Forks {
	if err := f.Fork.Validate(); err != nil {
		return fmt.Errorf("stock fork %d: %w", i, err)
	}
	if err := f.LowerCup.Validate(); err != nil {
		return fmt.Errorf("stock fork %d lower cup: %w", i, err)
	}
}
```

`CockpitPreferences` gains, after `BarSweep`:

```go
ForkSag PricedBand // percent sag; penalizes suspension forks only (Q32)
BrpMiss PricedBand // mm of remaining saddle BRP miss, for any cause; counts whenever there is a BRP target (Q32, Q34)
```

`Validate()` appends `{string(BandForkSag), p.ForkSag}` and `{string(BandBrpMiss), p.BrpMiss}`.

Add the constants `BandForkSag LedgerBand = "fork sag"` and `BandBrpMiss LedgerBand = "BRP miss"` after `BandBarSweep`.

`StockCandidate` gains, after `HandlebarIndex`:

```go
StockFork bool        // the fork came from CockpitStock.Forks; false when the submitted fork was kept
ForkIndex int         // index into CockpitStock.Forks; meaningful only when StockFork
BrpTarget *pose.Point // saddle BRP target; nil without one
```

Its `Ledger` comment becomes `// oval, stem length, stem angle, spacers, bar sweep, fork sag, BRP miss`. Add:

```go
// BrpMiss returns BrpTarget − placed BRP (world mm), and false without a BRP target.
func (c StockCandidate) BrpMiss() (pose.Point, bool)
```

### A2. `ApplyStockFitting`

Doc comment, replacing the current one:

```go
// ApplyStockFitting places every stock combination of fork configuration,
// handlebar, stem angle, stem length, and spacer stack with PlaceBike and
// returns the best-scoring limit candidates. Empty stock.Forks keeps the submitted
// fork and lower cup; empty stock.Handlebars keeps the submitted bar. Every bar
// is installed at setup.Handlebar.BarRollDeg (Q24). When target.BRP is set,
// ApplyFitting fits the saddle first for each fork configuration, and every
// candidate on that fork carries those saddle inputs (Q15). Ties favor smaller
// oval R, then enumeration order (forks, bars, angles, lengths, spacers).
```

Body changes, keeping the existing validation at the top:

1. Delete the single `base` / saddle-fit / `PlaceBike(base, nil)` block. Build the fork choices:

   ```go
   type stockForkChoice struct {
       stock    bool
       index    int
       fork     ForkSetup
       lowerCup LowerCupSetup
   }
   forks := []stockForkChoice{{fork: setup.Fork, lowerCup: setup.LowerCup}}
   if len(stock.Forks) > 0 {
       forks = make([]stockForkChoice, len(stock.Forks))
       for i, f := range stock.Forks {
           forks[i] = stockForkChoice{stock: true, index: i, fork: f.Fork, lowerCup: f.LowerCup}
       }
   }
   ```

2. `lengths` and `stacks` are computed once from `setup.Stem`, because the fork never changes the stem. The bar list's roll comes from `setup.Handlebar.BarRollDeg`.
3. Wrap the existing bar/angle/length/spacer loops in `for _, fork := range forks {`. At the top of that loop:

   ```go
   base := setup
   base.Fork, base.LowerCup = fork.fork, fork.lowerCup
   if target.BRP != nil {
       saddle, err := ApplyFitting(base, FitTarget{BRP: target.BRP}, nil)
       if err != nil {
           return StockFitting{}, err
       }
       base = saddle.Placement.Setup
   }
   if _, err := PlaceBike(base, nil); err != nil {
       return StockFitting{}, err
   }
   ```

4. In the candidate literal, set `StockFork: fork.stock` and `ForkIndex: fork.index`. Set `BrpTarget` to a copy of `*target.BRP` when it is set (`p := *target.BRP; &p`, taken once before the loops). Append two ledger lines after bar sweep:

   ```go
   {Band: BandForkSag, PointsLost: forkSagPoints(prefs.ForkSag, candidate.Fork)},
   {Band: BandBrpMiss, PointsLost: brpMissPoints(prefs.BrpMiss, target.BRP, placement)},
   ```

   with these helpers below `ApplyStockFitting`:

   ```go
   // forkSagPoints penalizes a suspension fork's sag; a rigid fork has no sag penalty (Q32).
   func forkSagPoints(band PricedBand, fork ForkSetup) float64 {
       if fork.Spec.TravelMm == 0 {
           return 0
       }
       return band.PointsLost(fork.SagPct)
   }

   // brpMissPoints penalizes the placed BRP's distance from the BRP target; 0 without one (Q32, Q34).
   func brpMissPoints(band PricedBand, brp *pose.Point, placement BikePlacement) float64 {
       if brp == nil {
           return 0
       }
       return band.PointsLost(brp.Sub(placement.Saddle.BRP.Origin()).Norm())
   }
   ```

`Evaluated` is `len(candidates)`, so it is forks × bars × angles × lengths × stacks. Don't change the sort or `stockSteps`.

### A3. Tests (`stock_fitting_test.go`)

Add helpers:

```go
// fenrirStockForks returns the Q31 configurations in fixture order: Lithic
// rigid; SID at 15, 20, 25% sag; Fox 34 at 15, 20, 25% sag; all on the EC44 12 mm cup.
func fenrirStockForks() []assembly.StockFork
```

The specs are Lithic {495, 0, 54}, SID {506, 100, 44}, and Fox 34 {530, 120, 44} (StaticA2cMm, TravelMm, RakeMm), all with `fenrirEc44Cup()`. The indices are Lithic 0, SID15 1, SID20 2, SID25 3, Fox15 4, Fox20 5, and Fox25 6. `illustrativeCockpitPreferences()` gains `ForkSag: {FreeMin: 20, FreeMax: 20, CostBelow: 2, CostAbove: 2}` and `BrpMiss: {FreeMax: 1, CostAbove: 2}`. `forkStock` is `fenrirCockpitStock()` with `Forks = fenrirStockForks()`.

**Existing tests that change:**

- `wantBands` in `TestStockFittingForwardAndRankOrder` gains `BandForkSag`, `BandBrpMiss`. The fit-report ledger check gains two more expected 0 lines (that test has no BRP target).
- **(Q34)** Any test with both a BRP and a HandGrip target that asserts a total: subtract 77.227222 (2 × (39.613611 − 1)). `TestStockFittingFitsSaddleFirst` and `TestStockFittingCarriesSaddleAcrossBars` compare stems and saddle inputs, not totals, so they don't change.

**New tests:**

1. **`TestStockFittingKeepsSubmittedForkByDefault`.** `mustStockFit` with grip only: every candidate has `StockFork == false`, its `Fork` and `LowerCup` equal the setup's, and the fork sag and BRP miss lines are 0.
2. **`TestStockFittingChoosesAmongForks`.** `forkStock`, grip only, limit 3150. `Evaluated == 3150`, with exactly 450 candidates per `ForkIndex`. Every candidate's `Placement.Setup.Fork`/`LowerCup` equal `forkStock.Forks[c.ForkIndex]`. The fork sag line is 10 for indices 1, 3, 4, and 6, and 0 for 0, 2, and 5. Then limit 5 against §5 table A (index, stem via `requireStockStem`, R to 1e-5, total to 1e-6).
3. **`TestStockFittingRefitsSaddlePerFork`.** `forkStock`, BRP + grip, limit 3150. For each index *i*, `ApplyFitting` on the setup with fork *i* (BRP only) gives the extension and rail offset that every candidate with `ForkIndex == i` carries exactly. `BrpMiss().Norm()` matches §5's per-fork list to 1e-6, and the BRP miss line equals 2 × (miss − 1) to 1e-9. Then limit 5 against §5 table B.
4. **(Q34) `TestStockFittingPenalizesBrpMissForSubmittedFork`.** `mustStockFit` with BRP + grip, limit 5: every candidate's BRP miss line is 77.227222 (1e-6), and rank 1 is −17°, 130, 0 with total 1.484762 (1e-6).
5. **`TestCockpitStockValidation`** (extend it): a stock fork with `SagPct = 150` is rejected, and so is a stock fork whose lower cup fails `Validate`. `forkStock.Validate()` is nil. A `ForkSag` band with `FreeMin > FreeMax` is rejected with an error containing `fork sag`.

## 4. Commit 2: workbench (`internal/reboot/web`)

Files: `fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`. No CSS or JS change.

Commit subject: `Offer every fixture fork and sag to stock fitting in the reboot workbench`.

### W1. Fixture (`fixture.go`)

- `forkOption` gains `Name string`, set by `makeForkOption` from its `name` argument. `cadForkOption()` sets `Name: "CAD Baseline Fork"`.
- `bikeFixture` gains `StockSagPcts []float64` after `CockpitPreferences`. `otsoFenrirFixture()` sets `StockSagPcts: []float64{15, 20, 25}, // Q31`.
- `CockpitPreferences` gains:

  ```go
  ForkSag: assembly.PricedBand{FreeMin: 20, FreeMax: 20, CostBelow: 2, CostAbove: 2}, // Matt's preference (Q32)
  BrpMiss: assembly.PricedBand{FreeMax: 1, CostAbove: 2},                             // illustrative (Q32)
  ```

- Add:

  ```go
  // stockForkOption is one fork configuration offered to stock fitting (Q31).
  type stockForkOption struct {
      Option forkOption
      SagPct float64
  }

  // stockForks returns the fork configurations stock fitting offers, in fixture
  // order: a rigid fork once at 0% sag, a suspension fork at each StockSagPcts.
  func (f bikeFixture) stockForks() []stockForkOption

  // stockForkSetups returns each offered configuration as an assembly.StockFork,
  // in the same order: the option's catalog Spec, the sag, and its DefaultLowerCup.
  func stockForkSetups(offered []stockForkOption) []assembly.StockFork

  // forkCellName names a fork configuration in a stock row: the fork name, plus
  // ", N% sag" for a suspension fork.
  func forkCellName(opt forkOption, sagPct float64) string
  ```

  `forkCellName` returns `opt.Name` when `opt.Spec.TravelMm == 0`, and otherwise `fmt.Sprintf("%s, %.0f%% sag", opt.Name, sagPct)`.
- `validate()`: `StockSagPcts` is non-empty, and every `stockForks()` entry passes `assembly.ForkSetup{Spec: o.Option.Spec, SagPct: o.SagPct}.Validate()`.

### W2. State, URL, form, decoding (`web.go`)

Mirror Slice 5's `stock_bars` exactly:

- Constants `stockForksSelected = "selected"` and `stockForksAll = "all"` (Q33).
- `workbenchState.AllStockForks bool`, placed after `AllStockBars`. `withSetup` copies it, and its comment names "stock bar and fork options".
- `URL()`: `if s.AllStockForks { q.Set("stock_forks", stockForksAll) }`. The default is omitted.
- `bikeFormValues.StockForks string`, placed after `StockBars`. `formValues()` sets it.
- Decoding: right after the `stock_bars` switch, an identical switch on `stock_forks`. The error message is `unknown stock fitting fork option %q`.
- `renderBikeError`: `cadBase.AllStockForks = resolved.State.AllStockForks`.
- `bikeView` gains `StockForkCount int`, set to `len(fix.stockForks())` in `buildBikeView` and `renderBikeError`.

### W3. Handler and stock view

- In `handleBikeWorkbench`, next to `offered` (bars):

  ```go
  var offeredForks []stockForkOption
  if resolved.State.AllStockForks {
      offeredForks = fix.stockForks()
  }
  stock.Forks = stockForkSetups(offeredForks)
  ```

  Pass `offeredForks` to `buildStockFittingView` as a new parameter after `offered`.
- `stockFittingView` gains `AllForks bool` and `PartsText string`. `stockCandidateView` gains `ForkName string`, `HasBrp bool`, and `BrpMissMm float64`.
- `PartsText` comes from a helper:

  ```go
  // stockPartsText names the enumerated parts: "stem and spacer",
  // "handlebar, stem, and spacer", "fork, stem, and spacer", or
  // "fork, handlebar, stem, and spacer".
  func stockPartsText(allForks, allBars bool) string
  ```

- For each candidate: `applied := state.withSetup(c.Placement.Setup)`, then the existing bar block. Then:

  ```go
  fork, _ := fix.resolveForkOption(state.ForkID)
  if c.StockFork {
      if c.ForkIndex < 0 || c.ForkIndex >= len(offeredForks) {
          return nil, fmt.Errorf("stock candidate %d names fork %d of %d offered", i+1, c.ForkIndex, len(offeredForks))
      }
      fork = offeredForks[c.ForkIndex].Option
      applied.ForkID = fork.ID
  }
  ```

  Set `ForkName: forkCellName(fork, c.Placement.Setup.Fork.SagPct)`. When `c.BrpMiss()` reports a target, set `HasBrp: true` and `BrpMissMm` to its norm. `withSetup` already carries `ForkA2cMm`, `LowerCupMm`, and `SagPct` from the candidate's setup. The URL carries no `prev_fork`, so decoding reads them as submitted.
- Preference note:

  ```go
  "Free: HandGrip inside the ±%.0f × ±%.0f mm oval, stem length %s, stem angle %s, spacers %s, bar sweep %s, fork sag %s (suspension forks), BRP miss %s. Penalties are illustrative except bar sweep and fork sag (Q30, Q32)."
  ```

  The last two arguments are `freeRangeText(prefs.ForkSag, "%")` and `freeRangeText(prefs.BrpMiss, " mm")`, which give `20%` and `0–1 mm`.

### W4. Template (`bike.html`)

- In the HandGrip fit target fieldset, after the stock bars field:

  ```html
  <div class="field field-full">
    <label for="ctrl-stock-forks">Stock fitting forks</label>
    <select id="ctrl-stock-forks" name="stock_forks"{{if and $.ErrorMessage (eq $.InvalidField "stock_forks")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
      <option value="selected"{{if ne .Form.StockForks "all"}} selected{{end}}>Selected fork only</option>
      <option value="all"{{if eq .Form.StockForks "all"}} selected{{end}}>All {{.StockForkCount}} fork setups</option>
    </select>
  </div>
  ```

- Stock note: replace the `{{if .AllBars}}...{{end}}` phrase with `stock {{.PartsText}} combinations`. Replace "The saddle fit is kept." with `{{if .AllForks}}The saddle is refitted for each fork.{{else}}The saddle fit is kept.{{end}}`.
- Stock table headers: insert `<th>Sag pts</th><th>BRP pts</th>` after `<th>Sweep pts</th>`. After `<th>Handlebar</th>`, append `<th>Fork</th><th>BRP miss</th>`. That makes 17 headers. Row cells after the bar cell:

  ```html
  <td data-stock-cell="fork">{{.ForkName}}</td>
  <td class="mono" data-stock-cell="brp-miss">{{if .HasBrp}}{{printf "%.1f mm" .BrpMissMm}}{{else}}—{{end}}</td>
  ```

### W5. Tests (`web_test.go`)

**Update existing tests:**

- The form label count goes up by 1 (to 19 after Slices 6 and 7).
- `TestBikeWorkbenchRejectsInvalidQuery`: add `{"/bike?stock_forks=some", "stock_forks", ""}`.
- `TestWorkbenchStateCodecRoundTrip`: add an `all-stock-forks` case (`/bike?stock_forks=all`, default state with `AllStockForks = true`).
- `TestBikeWorkbenchFormRoundTripPreservesEveryBuild`: append `&stock_forks=all` to `customURL`. Don't add it to the `mutations` list.
- `TestBikeWorkbenchRanksStockFitting`: points cells 5 → 7 and headers 13 → 17. Add `{"fork sag", "0.0", "Sag pts"}` and `{"BRP miss", "0.0", "BRP pts"}` to the ledger table (header indices 12 and 13). The last header is now `BRP miss`. The preference note is `Free: HandGrip inside the ±10 × ±20 mm oval, stem length 70–80 mm, stem angle 0°, spacers 0–20 mm, bar sweep 16–20°, fork sag 20% (suspension forks), BRP miss 0–1 mm. Penalties are illustrative except bar sweep and fork sag (Q30, Q32).`. Row 1's fork cell is `Lithic Carbon Mountain`, and its brp-miss cell is `—`.
- Every test that asserts "the last header is Handlebar" or reads cells by position after Spacer pts: update the positions to the new layout.
- **(Q34)** Tests with `brp_x` and `grip_x` that assert totals: subtract 77.227222 and reformat with `%.1f`. Slice 7's table A totals of 86.6 become `9.4` on the BRP pages of `TestBikeWorkbenchStockFittingAcrossAllBars` and its "apply row two" subtest. Its "different submitted bar" subtest has no BRP target, so it keeps 86.6: give it its own expected table instead of sharing `want`. Persuader 78.7 becomes `1.5`.

**New tests** (reference numbers in §5):

1. **`TestBikeWorkbenchStockForksDefaultKeepsSubmittedFork`.** `/bike?grip_x=481&grip_y=726`: `select[name="stock_forks"]` has 2 options, with `selected` selected and the second option's text `All 7 fork setups`. Evaluated is `450`. Every row's fork cell is `Lithic Carbon Mountain`, and no Apply href has `stock_forks`. `/bike?fork=sid_100&sag=25&grip_x=481&grip_y=726`: every fork cell is `RockShox SID, 25% sag`, and every `fork sag` points cell is `10.0`.
2. **`TestBikeWorkbenchStockFittingAcrossAllForks`.** `/bike?stock_forks=all&grip_x=481&grip_y=726&view=stem`: evaluated `3150`. The note contains `stock fork, stem, and spacer combinations` and `The saddle is refitted for each fork.`. The rows match table A (fork cell, stem, spacers, oval-r, total, fork sag points). Row 1's Apply href has `fork=sid_100`, `fork_a2c=506.0`, `sag=20.0`, `cup=12.0`, `stem_length=130.0`, `stem_angle=-17.0`, `spacers=0.0`, `stock_forks=all`, and `post=setback_0`.
3. **`TestBikeWorkbenchStockForksWithSaddle`.** `/bike?stock_forks=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`: the rows match table B (fork, stem, spacers, total, brp-miss cell, BRP points). Row 1's Apply href has `fork=sid_100`, `sag=20.0`, and `rail_offset=15.0`. Following it selects `sid_100`, keeps `all` selected for `stock_forks`, and row 1 is unchanged (the all-forks ranking doesn't depend on the submitted fork).
4. **`TestBikeWorkbenchStockForksAndBars`.** `/bike?stock_forks=all&stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`: evaluated `47250`. The note contains `stock fork, handlebar, stem, and spacer combinations`. Rows 1–5 match table C (bar, fork, stem, spacers, total).
5. **The option survives without a target.** `/bike?stock_forks=all&view=seat`: no `[data-stock-fitting]`, and `all` is selected. The canonical URL, the view links, and the CAD preset have `stock_forks=all`. Reset Defaults doesn't.
6. **Error page.** `/bike?stock_forks=some` returns 400, with `aria-invalid="true"` on the select and `unknown stock fitting fork option "some"` in `#form-error`.

## 5. Reference numbers (measured 2026-11-19)

The probe decoded each fork configuration's workbench build (`fork`, `sag`, `bar` in the query, all other fields default, 0 mm post). It called the **existing** single-fork `ApplyStockFitting` (limit 450) for each configuration and bar, then added Slice 7's sweep penalty, the sag penalty (2 × |sag − 20| for suspension forks), and the BRP miss penalty (2 × max(0, miss − 1), with miss measured from each candidate's placed BRP). It merged and sorted by points, then R. It is independent of the new enumeration code. Totals are 100 − points; tolerances are R to 1e-5 and totals to 1e-6.

**Saddle fitting per fork** (fit report BRP, 0 mm post; every fork stops at rail +15):

| Index | Fork | Extension | BRP miss |
| --- | --- | --- | --- |
| 0 | Lithic Carbon Mountain | 260.092242 | 39.613611 |
| 1 | RockShox SID, 15% sag | 260.095 | 39.553918 |
| 2 | RockShox SID, 20% sag | 259.930 | 42.572417 |
| 3 | RockShox SID, 25% sag | 259.752 | 45.600102 |
| 4 | Fox 34, 15% sag | 260.653 | 26.978855 |
| 5 | Fox 34, 20% sag | 260.516 | 30.554575 |
| 6 | Fox 34, 25% sag | 260.361 | 34.144103 |

(Extensions other than index 0 are to 1e-3; the tests compare them with `ApplyFitting`, not with this table.)

**Table A:** `stock_forks=all`, Persuader 800, HandGrip (481, 726), no BRP, 3150 evaluated.

| Rank | Index | Fork | Stem | Spacers | R | Sag pts | Total |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 2 | RockShox SID, 20% sag | 130 mm, −17° | 0 | 1.019761 | 0 | 86.165257 |
| 2 | 0 | Lithic Carbon Mountain | 130 mm, −17° | 0 | 1.358546 | 0 | 78.711983 |
| 3 | 2 | RockShox SID, 20% sag | 130 mm, −17° | 5 | 1.412418 | 0 | 77.526802 |
| 4 | 3 | RockShox SID, 25% sag | 130 mm, −17° | 0 | 0.676521 | 10 | 76.600000 |
| 5 | 3 | RockShox SID, 25% sag | 130 mm, −17° | 5 | 1.061626 | 10 | 75.244228 |

**Table B:** `stock_forks=all`, Persuader 800, BRP (−219, 713.1) + HandGrip (481, 726), 3150 evaluated.

| Rank | Index | Fork | Stem | Spacers | R | BRP miss | BRP pts | Total |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | 2 | RockShox SID, 20% sag | 130 mm, −17° | 0 | 1.019761 | 42.572417 | 83.144834 | 3.020423 |
| 2 | 0 | Lithic Carbon Mountain | 130 mm, −17° | 0 | 1.358546 | 39.613611 | 77.227222 | 1.484762 |
| 3 | 5 | Fox 34, 20% sag | 130 mm, −17° | 0 | 2.401938 | 30.554575 | 59.109150 | −3.351795 |
| 4 | 2 | RockShox SID, 20% sag | 130 mm, −17° | 5 | 1.412418 | 42.572417 | 83.144834 | −5.618032 |
| 5 | 0 | Lithic Carbon Mountain | 130 mm, −17° | 5 | 1.755355 | 39.613611 | 77.227222 | −7.245027 |

Web cells: totals `3.0`, `1.5`, `-3.4`, `-5.6`, `-7.2`; brp-miss `42.6 mm`, `39.6 mm`, `30.6 mm`, `42.6 mm`, `39.6 mm`.

**Table C:** `stock_forks=all`, `stock_bars=all`, BRP + HandGrip, 47250 evaluated.

| Rank | Fork | Bar | Stem | Spacers | R | Total |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | Fox 34, 20% sag | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 120 mm, −17° | 0 | 0.715673 | 21.490851 |
| 2 | Fox 34, 20% sag | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 120 mm, −17° | 5 | 1.046532 | 20.467154 |
| 3 | Fox 34, 20% sag | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 130 mm, −17° | 0 | 0.184746 | 19.490851 |
| 4 | Fox 34, 20% sag | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 130 mm, −17° | 5 | 0.243928 | 19.490851 |
| 5 | Fox 34, 20% sag | SQlab 3OX 12°, 15 mm rise (780 mm) | 130 mm, −17° | 0 | 0.274618 | 19.490851 |

Web totals: `21.5`, `20.5`, `19.5`, `19.5`, `19.5`. The probe took about 0.4 s for table C, so the page with both `all` options takes roughly that long.

**Selected mode (Q34)**, Lithic, Persuader, BRP + grip: rank 1 is 130 mm, −17°, 0, BRP points 77.227222, total 1.484762.

Findings worth noting (not defects). With a 0 mm post, no fork lets the saddle reach the fit report BRP. The 20% Fox 34 leaves the smallest miss (30.6 mm) and wins once the bars are free to choose. With the Persuader, the 20% SID wins even though its saddle misses about 3 mm more than the Lithic's (5.9 extra points), because its cockpit loses 7.5 fewer points. That's the 2 points/mm BRP miss penalty at work. If Matt wants the saddle to dominate more strongly (Q15), the BRP miss penalty is the lever. Slice 6's setback notes show the post this needs. If your numbers differ beyond the stated tolerances, find out why before changing a test.

## 6. Not in this slice

The CAD Baseline Fork in stock fitting (Q31); sag steps other than 15/20/25 (Q31); fork A2C or cup choice apart from each fork's catalog values; continuous fork fitting; seatpost choice (Q27); roll enumeration (Q24); renaming `CockpitStock` or `CockpitPreferences` now that they hold forks and the BRP miss penalty; the scoring-word rename (DESIGN.md, Milestone 2, **Later**); CLI/MCP access; other frames; any change to `pose` or to `assembly` outside `stock_fitting.go`.

## 7. Rules that bite here

- **Forward assembly is the evaluator (Q16).** Each fork configuration's frameset comes from PlaceBike. Don't adjust a placement for pitch by hand.
- **Q15, per fork:** the saddle is fitted once per fork configuration, before that fork's cockpit enumeration, and every candidate on that fork carries it.
- **Ledger order is fixed:** oval, stem length, stem angle, spacers, bar sweep, fork sag, BRP miss. Every line is always present.
- **Q32:** a rigid fork has no sag penalty. Don't score its 0% against the 20% band.
- **Q34:** the BRP miss line counts in both `stock_forks` modes. Don't skip it when the fork is the submitted one.
- **The web never recomputes geometry.** Fork names come from `ForkIndex` and the offered list; BRP miss comes from `StockCandidate.BrpMiss()`.
- **The URL is the only state.** `stock_forks=all` round-trips through the form, the canonical URL, and every link. Apply links also keep Slice 6's `post` and Slice 5's `stock_bars`.
- `deadcode`: `stockForks`, `stockForkSetups`, `forkCellName`, `stockPartsText`, `forkSagPoints`, `brpMissPoints`, and `StockCandidate.BrpMiss` must be reached from production code. `exhaustive`: no new enum switch (`stock_forks` switches on a plain string). `boundary`: no legacy imports.
- Placements must be identical with tracing on or off. Stock fitting passes a nil trace.
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Finish with the implementation handoff (DESIGN.md, "Slice planning"): the Status line, an `## Implementation handoff` section in this file, and the Slice 8 Milestones line. Don't touch any other part of DESIGN.md. Only Matt's acceptance marks the slice done.

## 8. Verification

Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit, re-running it until `go fix` is clean. Commit 1 must pass on its own before Commit 2 starts.

Then `make serve-reboot` and open `http://127.0.0.1:8081/bike?stock_forks=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=bike`. Confirm that:

- The note says 3150 stock fork, stem, and spacer combinations, and row 1 is RockShox SID, 20% sag, at 130 mm, −17°, BRP miss 42.6 mm, total 3.0.
- Clicking row 1's Apply selects RockShox SID with sag 20 and keeps "All 7 fork setups".
- Switching "Stock fitting handlebars" to all gives 47250 combinations, with row 1 being Fox 34, 20% sag, with the FL-X 12° 15 mm bar, total 21.5.

Take a screenshot for Matt.
