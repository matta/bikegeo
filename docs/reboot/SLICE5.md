# M2 Slice 5, handlebars in stock fitting

**Status: Planned 2026-11-19 from Matt's rulings Q22–Q25 ([DESIGN.md](DESIGN.md), "Handlebars in stock fitting"). Implemented 2026-11-19; awaiting Matt's acceptance.** After review, `StockCandidate` gained a `StockHandlebar` flag in place of the `HandlebarIndex == -1` sentinel (A2), and the workbench checks each candidate's bar index against the bar list it offered. Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. Slices 1–4 are implemented; Slice 4 (handlebar choice in the workbench) is awaiting Matt's acceptance.

This slice has two commits and touches 6 code and test files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Today stock cockpit fitting (`assembly.ApplyStockFitting`) always keeps the submitted handlebar. After this slice:

- `assembly.CockpitStock` gains an optional list of handlebars. When it is non-empty, stock fitting enumerates every bar in it, alongside the stem angles, stem lengths, and spacer steps it already enumerates, and installs each one at the **submitted** bar roll (Q24). Each candidate records which bar it installed.
- `/bike` gets a **Stock fitting handlebars** select, `stock_bars=selected|all` (Q22). `selected`, the default, keeps today's behavior exactly. `all` hands the 15 fixture bars to stock fitting.
- Bars cost nothing (Q23). The ledger keeps its 4 lines: HandGrip oval, stem length, stem angle, and spacers.
- The result is still the 5 cheapest builds overall (Q25 = Q18). Each row names its bar, and its Apply link sets `bar` to that row's bar.
- The continuous fitting block ("needs a 124.5 mm stem") is unchanged and always uses the submitted bar.

Rulings:

- **Q22:** opt-in control `stock_bars=selected|all`; `selected` is the default and today's behavior.
- **Q23:** no bar price. Don't add a `LedgerBand`.
- **Q24:** every installed bar gets the submitted `bar_roll`. Roll is never enumerated, never changed, and never reset.
- **Q25:** top 5 overall across all bars; several rows may share a bar.

## 2. Terminology

- **Handlebar** (**bar** in short labels and URL keys). The new URL key is `stock_bars`; the form control is `#ctrl-stock-bars`, labelled "Stock fitting handlebars".
- **Stock fitting** / **fit** / **fitting** (Q8). Never write "search" for it.
- Don't write the term AGENTS.md bans for the frameset (`make boundary` checks it).

## 3. Commit 1: assembly (`internal/reboot/assembly`)

Files: `stock_fitting.go`, `stock_fitting_test.go`. Nothing else changes in `assembly` or `pose`.

Commit subject: `Let reboot stock cockpit fitting choose among handlebars`.

### A1. `CockpitStock` (`stock_fitting.go`)

Add one field after `SpacerStepMm`, and extend the type's doc comment:

```go
// CockpitStock lists the stock cockpit parts stock fitting may choose from.
// Stem lengths start at Stem.MinLengthMm, and spacer stacks start at zero.
// Both advance in fixed steps up to their hardware limits. Handlebars, when
// non-empty, lists the bars stock fitting may install; each is installed at
// the submitted setup's BarRollDeg (Q24). Empty keeps the submitted bar (Q22).
type CockpitStock struct {
	StemLengthStepMm float64
	StemAnglesDeg    []pose.Deg
	SpacerStepMm     float64
	Handlebars       []HandlebarSetup
}
```

At the end of `Validate()`, before `return nil`, add:

```go
for i, bar := range s.Handlebars {
	if bar.BarRollDeg != 0 {
		return fmt.Errorf("stock handlebar %d must not set BarRollDeg; stock fitting installs every bar at the submitted roll (Q24)", i)
	}
	if _, err := PlaceHandlebar(pose.NewPose(pose.Point{}, 0, nil), bar); err != nil {
		return fmt.Errorf("stock handlebar %d: %w", i, err)
	}
}
```

This reuses `PlaceHandlebar`'s own checks. Don't restate them.

### A2. `StockCandidate`

Add one field after `Target`:

```go
HandlebarIndex int // index into CockpitStock.Handlebars of the installed bar; -1 when Handlebars is empty and the submitted bar was kept
```

### A3. `ApplyStockFitting`

Replace the doc comment with:

```go
// ApplyStockFitting places every stock combination of handlebar, stem angle,
// stem length, and spacer stack with PlaceBike and returns the cheapest limit
// candidates. When stock.Handlebars is empty, the submitted bar is the only
// one. Every bar is installed at setup.Handlebar.BarRollDeg (Q24) and costs
// nothing (Q23). Ties favor smaller oval R, then enumeration order (bars in
// list order, then angles, lengths, and spacers). When target.BRP is set,
// ApplyFitting fits the saddle first and every candidate carries those saddle
// inputs (Q15).
```

In the body, right after the `lengths` and `stacks` lines, build the bar choices:

```go
type stockBar struct {
	index int
	setup HandlebarSetup
}
bars := []stockBar{{index: -1, setup: base.Handlebar}}
if len(stock.Handlebars) > 0 {
	bars = make([]stockBar, len(stock.Handlebars))
	for i, bar := range stock.Handlebars {
		bar.BarRollDeg = base.Handlebar.BarRollDeg // Q24
		bars[i] = stockBar{index: i, setup: bar}
	}
}
```

Then wrap the existing three loops in an outermost `for _, bar := range bars {`. Inside, set `candidate.Handlebar = bar.setup` next to the stem assignments, and set `HandlebarIndex: bar.index` in the `StockCandidate` literal. Don't change the sort, `stockSteps`, the saddle-first block, or the `PlaceBike(base, nil)` check. `Evaluated` is `len(candidates)` as before, so it becomes `bars × angles × lengths × stacks`.

### A4. Tests (`stock_fitting_test.go`, package `assembly_test`)

Add a helper beside `fenrirCockpitStock` (catalog values, same as the web fixture's `sqlab311_flx_12_15`):

```go
func sqlab311Flx12x15Handlebar() assembly.HandlebarSetup {
	return assembly.HandlebarSetup{
		StockWidthMm: 740, CurrentWidthMm: 740, BacksweepDeg: 12, UpsweepDeg: 4,
		StatedRiseMm: 15, RiseMode: assembly.RiseModeStandardKnuckle, BendStartMm: 68.5,
		GripInsetMm: 70, BarRollDeg: 0,
	}
}
```

All new tests use `fenrirFittingSetup()`, `fitReportGrip`, `illustrativeCockpitPreferences()`, and a stock of `fenrirCockpitStock()` with `Handlebars = []assembly.HandlebarSetup{persuader800DefaultHandlebar(), sqlab311Flx12x15Handlebar()}` (call it `twoBarStock`). Call `assembly.ApplyStockFitting` directly, since `mustStockFit` always uses `fenrirCockpitStock()`.

1. **`TestStockFittingKeepsSubmittedBarByDefault`.** `mustStockFit(t, fenrirFittingSetup(), FitTarget{HandGrip: &fitReportGrip}, 450)`: every candidate has `HandlebarIndex == -1` and `Placement.Setup.Handlebar == fenrirFittingSetup().Handlebar`.
2. **`TestStockFittingChoosesAmongHandlebars`.** `twoBarStock`, no BRP, limit 900. `Evaluated == 900`, and exactly 450 candidates have each index. Every candidate's `Placement.Setup.Handlebar == twoBarStock.Handlebars[c.HandlebarIndex]`. Then limit 5, and check this table (angle, length, spacers via `requireStockStem`; R within 1e-5; total within 1e-9):

   | Rank | Index | Stem | R | Total |
   | --- | --- | --- | --- | --- |
   | 1 | 1 | −17°, 110, 0 | 0.806233 | 90.6 |
   | 2 | 1 | −17°, 110, 5 | 0.969920 | 90.6 |
   | 3 | 1 | −6°, 130, 0 | 0.919776 | 88.8 |
   | 4 | 1 | −17°, 120, 5 | 0.253407 | 88.6 |
   | 5 | 1 | −17°, 120, 10 | 0.396539 | 88.6 |

   Ranks 1 and 2 tie on points, as do ranks 4 and 5, so this also checks the R tie-break.
   Cross-check: rank 1 must equal rank 1 of a single-bar run (empty `Handlebars`, `setup.Handlebar = sqlab311Flx12x15Handlebar()`): same stem and the same `Total()` to 1e-12.
3. **`TestStockFittingInstallsEveryBarAtSubmittedRoll`.** `setup.Handlebar.BarRollDeg = 5`, `twoBarStock`, limit 900: every candidate's `Placement.Setup.Handlebar.BarRollDeg == 5`. With limit 5: rank 1 is index 1, −17°, 110, 0, R 0.573433, total 90.6. Rank 3 is index 1, −17°, 120, 5, R 0.055006, total 88.6.
4. **`TestStockFittingCarriesSaddleAcrossBars`.** `twoBarStock`, target with `BRP: &fitReportBRP` and the grip, limit 900. Compute `saddle, _ := assembly.ApplyFitting(setup, FitTarget{BRP: &fitReportBRP}, nil)`. Every candidate's `Seatpost.ExtensionMm` and `Saddle.RailOffsetMm` equal `saddle.Placement.Setup`'s exactly.
5. **`TestCockpitStockValidation`** (extend it): a stock bar with `BarRollDeg = 5` is rejected, with an error containing `BarRollDeg`. A stock bar with `GripInsetMm = 0` is rejected (`PlaceHandlebar`'s error). `twoBarStock.Validate()` is nil.

Every existing test must pass unchanged.

## 4. Commit 2: workbench (`internal/reboot/web`)

Files: `fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`. No CSS or JS change: `workbench.js` listens for `change` on the whole form, so a new select submits like the others.

Commit subject: `Offer every fixture handlebar to stock fitting in the reboot workbench`.

### W1. Fixture (`fixture.go`)

```go
// handlebarSetups returns every fixture handlebar's setup, in list order.
func (f bikeFixture) handlebarSetups() []assembly.HandlebarSetup
```

Don't fill `CockpitStock.Handlebars` in `otsoFenrirFixture()`. The handler fills it per request (W5).

### W2. State, URL, form (`web.go`)

- Add near `stockCandidateLimit`:

  ```go
  // stock_bars values (Q22).
  const (
      stockBarsSelected = "selected"
      stockBarsAll      = "all"
  )
  ```

- `workbenchState` gains `AllStockBars bool // stock fitting offers every fixture handlebar (Q22)` after `Targets`. The zero value is the default, so `defaultWorkbenchState` and `workbenchStateFromSetup` stay unchanged.
- Add a method below `workbenchStateFromSetup` and use it in place of the "from setup, then copy Targets" pattern:

  ```go
  // withSetup returns s with every setup-derived field replaced by setup's,
  // keeping the submitted view, fork, bar, fit targets, and stock bar option.
  func (s workbenchState) withSetup(setup assembly.BikeSetup) workbenchState {
      next := workbenchStateFromSetup(s.View, s.ForkID, s.BarID, setup)
      next.Targets = s.Targets
      next.AllStockBars = s.AllStockBars
      return next
  }
  ```

  Replace these three call sites. **This is the step that keeps `stock_bars` on every link.** A missed one silently drops the option, and the W7 tests catch it.
  - `buildFittingView`: `fv.ApplyURL = state.withSetup(fitted).URL()`. Delete the two `applied` lines.
  - `buildStockFittingView`: see W4.
  - `withFitTargets`: `state := submitted.withSetup(setup)`. Delete the `state.Targets = ...` line.

  `buildBikeView` keeps calling `workbenchStateFromSetup`. W5 covers it.
- `URL()`: after the grip target block, add `if s.AllStockBars { q.Set("stock_bars", stockBarsAll) }`. The default is **omitted**, so every existing canonical URL and link stays byte-for-byte the same.
- `bikeFormValues` gains `StockBars string` after `GripY`. `formValues()` sets it to `stockBarsAll` when `AllStockBars`, else `stockBarsSelected`.

### W3. Decoding (`decodeWorkbenchState`)

Right after the `grip_x`/`grip_y` block, before the `view` block:

```go
form.StockBars = stockBarsSelected
switch raw := strings.TrimSpace(q.Get("stock_bars")); raw {
case "", stockBarsSelected:
case stockBarsAll:
	state.AllStockBars = true
	form.StockBars = stockBarsAll
default:
	form.StockBars = raw
	p.fail("stock_bars", fmt.Errorf("unknown stock fitting handlebar option %q", raw))
}
```

### W4. Stock view (`web.go`)

- `stockFittingView` gains `AllBars bool`. `stockCandidateView` gains `BarName string`.
- Change the signature to `buildStockFittingView(state workbenchState, fix bikeFixture, stock assembly.StockFitting)`. Read the preferences from `fix.CockpitPreferences` and set `AllBars: state.AllStockBars`. For each candidate:

  ```go
  applied := state.withSetup(c.Placement.Setup)
  bar, _ := fix.resolveHandlebar(state.BarID)
  if c.HandlebarIndex >= 0 {
      bar = fix.Handlebars[c.HandlebarIndex]
      applied.BarID = bar.ID
  }
  ```

  Set `BarName: bar.Name` and `ApplyURL: applied.URL()`. The index is valid because W5 builds `stock.Handlebars` from `fix.Handlebars` in the same order.

### W5. Handler (`handleBikeWorkbench`)

- Rename `withFitTargets` to `withSubmittedState`. Its comment becomes: "carries the submitted options that are not part of the placed setup (fit targets, stock bar option) into the view's form echo, canonical URL, and links. The diagram itself still shows the submitted build." Call it **unconditionally**, right after `buildBikeView(...)`, and delete the call inside `if resolved.State.Targets.any()`. Without targets it reproduces what `buildBikeView` set, plus `stock_bars`. This is what keeps `stock_bars=all` on a page that has no HandGrip target yet.
- Replace the stock call:

  ```go
  stock := fix.CockpitStock
  if resolved.State.AllStockBars {
      stock.Handlebars = fix.handlebarSetups()
  }
  result, err := assembly.ApplyStockFitting(resolved.Setup, target, stock, fix.CockpitPreferences, stockCandidateLimit)
  if err != nil { /* unchanged error handling */ }
  view.Fitting.Stock = buildStockFittingView(resolved.State, fix, result)
  ```

- `renderBikeError`: after the existing `cadBase.BarID` lines, add `cadBase.AllStockBars = resolved.State.AllStockBars`.

### W6. Template (`bike.html`)

- In the **HandGrip fit target** fieldset, add a third field inside `group-fields`, after HandGrip Y:

  ```html
  <div class="field field-full">
    <label for="ctrl-stock-bars">Stock fitting handlebars</label>
    <select id="ctrl-stock-bars" name="stock_bars"{{if and $.ErrorMessage (eq $.InvalidField "stock_bars")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
      <option value="selected"{{if ne .Form.StockBars "all"}} selected{{end}}>Selected handlebar only</option>
      <option value="all"{{if eq .Form.StockBars "all"}} selected{{end}}>All {{len .Handlebars}} handlebars</option>
    </select>
  </div>
  ```

- Stock note: replace `stock stem and spacer combinations placed` with `{{if .AllBars}}stock handlebar, stem, and spacer combinations{{else}}stock stem and spacer combinations{{end}} placed`.
- Stock table: add `<th>Handlebar</th>` as the **last** header, and `<td data-stock-cell="bar">{{.BarName}}</td>` as the **last** cell after the `{{range .Lines}}` cells. Existing tests find headers and cells by position, so it must go last.

### W7. Tests (`web_test.go`, browserless `httptest` + `goquery`)

**Update existing tests:**

- The form label count goes from 17 to 18.
- `TestBikeWorkbenchRejectsInvalidQuery`: add `{"/bike?stock_bars=some", "stock_bars", ""}`.
- `TestWorkbenchStateCodecRoundTrip` `okCases`: add `{name: "all-stock-bars", path: "/bike?stock_bars=all", want: <defaultWorkbenchState(fix) with AllStockBars = true>}`. Build `want` in a small closure or a variable before the table.
- `TestBikeWorkbenchFormRoundTripPreservesEveryBuild`: append `&stock_bars=all` to `customURL`. The idempotent-submit check then covers the select. **Don't** add `stock_bars` to the `mutations` list: that list requires each control to change the rendered build, and this one doesn't.
- Every other existing test must pass unchanged. The default is `selected` and the URL omits it, so every existing URL, number, and link holds.

**New tests.** Reference numbers are in §5. The stem cell format is `%.0f mm, %+.0f°` (for example `110 mm, -17°`), oval R is `%.2f`, and the total is `%.1f`. Use the existing helpers `loadBikeDoc`, `loadBikeDocStatus`, `stockCell`, `hrefQuery`, `metricText`, and `submitForm`.

1. **`TestBikeWorkbenchStockBarsDefaultKeepsSubmittedBar`.** `/bike?grip_x=481&grip_y=726`: `select[name="stock_bars"]` has 2 options and `selected` is the selected one. Evaluated is `450`. Every row's `bar` cell is `Tumbleweed Persuader 20° (800 mm)`. Every row's apply href has `bar=persuader20_800` and no `stock_bars` key. The canonical URL has no `stock_bars` key.
2. **`TestBikeWorkbenchStockFittingAcrossAllBars`.** `/bike?stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem`: evaluated is `6750`, the note contains `stock handlebar, stem, and spacer combinations`, and the 5 rows match §5 table A (bar cell, `stem`, `spacers`, `oval-r`, `total`). Row 2's apply href has `bar=sqlab311_flx_12_30`, `stock_bars=all`, `stem_length=110.0`, `stem_angle=-17.0`, `spacers=0.0`, `bar_roll=0.0`, `rail_offset=15.0`, `brp_x=-219.0`, and `grip_x=481.0`. `#apply-fitting`'s href has `stock_bars=all` and `bar=persuader20_800`.
3. **Following an all-bars apply link.** Load row 2's apply href from test 2. Selected `bar` is `sqlab311_flx_12_30`, selected `stock_bars` is `all`, and row 1 is still `SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm)` at `110 mm, -17°`, `0 mm`, total `90.6`. The all-bars ranking doesn't depend on the submitted bar.
4. **The submitted bar doesn't change the all-bars ranking.** `/bike?stock_bars=all&bar=sqlab3ox_12_15&grip_x=481&grip_y=726`: rows 1–5 have the same bar, stem, spacers, and total cells as table A.
5. **Roll is kept for every bar (Q24).** `/bike?stock_bars=all&bar_roll=5&grip_x=481&grip_y=726`: rows match §5 table B, and every row's apply href has `bar_roll=5.0`.
6. **The option survives without a target.** `/bike?stock_bars=all&view=seat`: there is no `[data-stock-fitting]` block, and `all` is the selected `stock_bars` option. The canonical URL, every `a[data-diagram-view]` href, and the CAD Baseline Fork preset href have `stock_bars=all`. The Reset Defaults href has no `stock_bars` key.
7. **Submitting the select.** Load `/bike?grip_x=481&grip_y=726`, then `submitForm` with `stock_bars` = `all`: evaluated becomes `6750` and row 1's bar cell is the FL-X 12° 15 mm bar.
8. **Error page.** `loadBikeDocStatus(t, "/bike?stock_bars=some", http.StatusBadRequest)`: `select[name="stock_bars"]` has `aria-invalid="true"`, and `#form-error` names `stock_bars`' message (`unknown stock fitting handlebar option "some"`).

## 5. Reference numbers (measured 2026-11-19)

The probe called the **existing** single-bar `ApplyStockFitting` once per fixture bar, with the submitted roll copied onto each bar. It merged the results and sorted them with the same comparator (points lost, then R, then bar order). It is independent of the new enumeration code. Default Fenrir build: Lithic fork, EC44 12 mm cup, spacers 20, stem 60 mm at −6°, post extension 263.5, rail offset 0. All 15 bars evaluated 6750 combinations in about 25 ms.

**Table A:** `stock_bars=all`, HandGrip (481, 726), roll 0. The same ranking holds with and without the BRP target (−219, 713.1) and for any submitted bar. With BRP, every row carries post extension 260.092 and rail offset 15.0 (the rail limit, Q10).

| Rank | Bar | Stem | Spacers | R | Total |
| --- | --- | --- | --- | --- | --- |
| 1 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 110 mm, −17° | 0 | 0.806233 | 90.6 |
| 2 | SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm) | 110 mm, −17° | 0 | 0.863228 | 90.6 |
| 3 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 110 mm, −17° | 5 | 0.969920 | 90.6 |
| 4 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 130 mm, −6° | 0 | 0.919776 | 88.8 |
| 5 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 120 mm, −17° | 5 | 0.253407 | 88.6 |

The ledger lines for ranks 1–3 are oval 0, length 6, angle 3.4, spacers 0. For rank 4 they are 0, 10, 1.2, 0, and for rank 5, 0, 8, 3.4, 0.

**Table B:** `stock_bars=all`, `bar_roll=5`, HandGrip (481, 726), no BRP.

| Rank | Bar | Stem | Spacers | R | Total |
| --- | --- | --- | --- | --- | --- |
| 1 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 110 mm, −17° | 0 | 0.573433 | 90.6 |
| 2 | SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm) | 110 mm, −17° | 0 | 0.744984 | 90.6 |
| 3 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 110 mm, −17° | 5 | 0.814177 | 90.6 |
| 4 | SQlab 3OX 12°, 15 mm rise (780 mm) | 110 mm, −17° | 0 | 0.935625 | 90.6 |
| 5 | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 120 mm, −17° | 5 | 0.055006 | 88.6 |

Rank 4 differs from table A, so table B shows that roll reaches every bar.

**Selected mode** (default) is unchanged: Persuader 800 rank 1 is −17°, 130, 0, total 78.711983 (Slice 3). `bar=sqlab3ox_12_15` rank 1 is −17°, 120, 0, total 88.6 (Slice 4).

Findings worth noting (not defects). With every bar free, the 12° SQlab bars win: they bring HandGrip inside the oval with a 110 mm stem, where the Persuader 800 needs 130 mm and still misses (R 1.36). The top rows tie on points and are separated only by R. That's expected while bars and short stems are priced the same, and it's the case Q23's "priced bands later" would address. If your numbers differ beyond the stated tolerances, find out why before changing a test.

## 6. Not in this slice

Priced bar preferences (Q23, later); roll enumeration (Q24, not planned); bar shortlists or subsets other than `selected`/`all`; continuous fitting across bars; cutting a bar or per-bar grip inset; a handlebar outline in the diagram; the `/` stem preview page; CLI/MCP access; other frames; any change to `pose` or to `assembly` outside `stock_fitting.go`.

## 7. Rules that bite here

- **Forward assembly is the evaluator (Q16).** Every candidate comes from `PlaceBike`. Don't compute HandGrip for a bar any other way, and don't skip placing a bar because it "can't win".
- **Q24:** a stock bar's own `BarRollDeg` must be 0 (`Validate`), and stock fitting installs each bar at `setup.Handlebar.BarRollDeg`. Never change `bar_roll` in the workbench because of a stock result.
- **Q23:** no new `LedgerBand` and no bar line in the ledger. The ledger stays 4 lines in today's order.
- **Q15:** the saddle fit is computed once, before enumeration, and every candidate carries it.
- **The web never recomputes geometry.** The bar name and ID come from `HandlebarIndex` and the fixture list. Readouts come from the candidate's placement.
- **The URL is the only state**, and the page must still work as a plain `GET` form without JS. `stock_bars=all` must round-trip through the form, the canonical URL, and every link the page builds. The default is omitted from URLs.
- `boundary`: nothing in `internal/reboot` imports a legacy package (no parity test in this slice).
- `deadcode`: `handlebarSetups` and `withSetup` must be reached from production code. Test helpers live in `_test.go` files.
- `exhaustive`: this plan adds no switch on an enum-like type (the `stock_bars` switch is on a plain string).
- Placements must be identical with tracing on or off. Stock fitting passes a nil trace, as today.
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 5 done at the end, and only after Matt accepts it.

## 8. Verification

After the code compiles, run `go vet ./... && go test ./...` after every change. Before each commit, run `make pre-commit` and re-run it until `go fix` is clean. Commit 1 must pass on its own before Commit 2 starts.

Then check it live: `make serve-reboot`, then open `http://127.0.0.1:8081/bike?stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=stem`. Confirm that:

- The note says 6750 combinations, and row 1 is the FL-X 12° 15 mm bar at 110 mm, −17°, total 90.6.
- Clicking row 2's Apply selects the FL-X 12° 30 mm bar and keeps "All 15 handlebars".
- Switching the select to "Selected handlebar only" drops back to 450 combinations with the selected bar on every row.

Take a screenshot for Matt.
