# Next slice: M2 Slice 4, handlebar choice in the workbench

**Status: Implemented and verified 2026-11-19 from Matt's rulings Q19–Q21 ([DESIGN.md](DESIGN.md), "Handlebar choice"). Awaiting Matt's acceptance.** Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. Slices 1–3 (saddle fitting, front-cockpit fitting, stock cockpit fitting) are done. The implementation is in `internal/reboot/web`.

## 1. Goal

The `/bike` workbench always installs one handlebar today: the fixture's Tumbleweed Persuader 20° (800 mm). Only `bar_roll` is in the URL. After this slice:

- The fixture carries a list of **15 handlebars**, ported from the legacy catalog (Q20).
- `/bike` has a **Handlebar** select (`bar=<id>` in the URL). The submitted bar is installed with the submitted `bar_roll`.
- Every link the page builds keeps `bar`: view links, Reset Defaults (which resets to the default bar), CAD Baseline Fork, "Apply fitted build", and every stock row's Apply link.
- Fitting (`ApplyFitting`) and stock fitting (`ApplyStockFitting`) already take the submitted `BikeSetup`, so they fit around the selected bar with no code change (Q19).

Rulings:

- **Q19, scope:** handlebar choice in the workbench only. Stock fitting keeps the submitted bar. Bars join stock fitting in a later slice.
- **Q20, bar list:** all 15 legacy bars, values ported exactly, labelled as legacy catalog values with sources not recorded.
- **Q21, roll:** `BarRollDeg` stays a submitted fixed input. Changing the bar keeps the submitted roll. There is no `prev_bar` and no reset.

## 2. Terminology

- **Handlebar** (or **bar** in short labels and the URL key). The URL key is `bar`; the form control is `#ctrl-bar`, labelled "Handlebar".
- The landmark is still **HandGrip**. Fitting is still **fitting** / **fit** (Q8). Never write "search" for it.
- Don't write the term AGENTS.md bans for the frameset (`make boundary` checks it).

## 3. The bar list (Q20)

Port every bar in `internal/catalog/handlebars.go` (`CommonHandlebarCatalog`), in that order. Field mapping from legacy `model.HandlebarSpec` to `assembly.HandlebarSetup`:

| Legacy | Reboot | Note |
| --- | --- | --- |
| `StockWidthMm` | `StockWidthMm` | |
| `CurrentWidthMm` | `CurrentWidthMm` | equals stock width for every legacy bar |
| `StatedRiseMm` | `StatedRiseMm` | |
| `BacksweepDeg`, `UpsweepDeg` | same, as `pose.Deg` | |
| `EffectiveBendStartMm` | `BendStartMm` | SQlab values come from legacy `SQlabEffectiveBendStartMm(rise) = 55 + 0.9·rise`; write the **literal** result, don't port the formula |
| `RiseMode` | `RiseMode` | `assembly.RiseModeBarEndCenter` / `RiseModeStandardKnuckle` (same string values) |
| — | `GripInsetMm` | 70 mm on every bar: the rider's grips, unchanged from today's fixture (legacy `WingGripInsetMm`) |
| — | `BarRollDeg` | always 0 in the fixture; the roll comes from the URL (Q21) |

IDs are the legacy preset keys **lowercased** (legacy `GetHandlebarPreset` matches case-insensitively, which the parity test relies on). Names are the display text in the select.

| # | ID | Name | Width | Rise | Mode | Backsweep | Upsweep | Bend start |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | `persuader20_800` | Tumbleweed Persuader 20° (800 mm) | 800 | 50 | bar end center | 20° | 4° | 135 |
| 2 | `persuader20_840` | Tumbleweed Persuader 20° (840 mm) | 840 | 50 | bar end center | 20° | 4° | 135 |
| 3 | `sqlab3ox_16_15` | SQlab 3OX 16°, 15 mm rise (780 mm) | 780 | 15 | knuckle | 16° | 4° | 68.5 |
| 4 | `sqlab3ox_16_30` | SQlab 3OX 16°, 30 mm rise (780 mm) | 780 | 30 | knuckle | 16° | 4° | 82.0 |
| 5 | `sqlab3ox_16_45` | SQlab 3OX 16°, 45 mm rise (780 mm) | 780 | 45 | knuckle | 16° | 4° | 95.5 |
| 6 | `sqlab3ox_12_15` | SQlab 3OX 12°, 15 mm rise (780 mm) | 780 | 15 | knuckle | 12° | 4° | 68.5 |
| 7 | `sqlab3ox_12_30` | SQlab 3OX 12°, 30 mm rise (780 mm) | 780 | 30 | knuckle | 12° | 4° | 82.0 |
| 8 | `sqlab3ox_12_45` | SQlab 3OX 12°, 45 mm rise (780 mm) | 780 | 45 | knuckle | 12° | 4° | 95.5 |
| 9 | `sqlab311_20_16_15` | SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm) | 740 | 15 | knuckle | 16° | 5° | 68.5 |
| 10 | `sqlab311_20_16_25` | SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm) | 740 | 25 | knuckle | 16° | 5° | 77.5 |
| 11 | `sqlab311_20_16_50` | SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm) | 740 | 50 | knuckle | 16° | 5° | 100.0 |
| 12 | `sqlab311_flx_16_15` | SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm) | 740 | 15 | knuckle | 16° | 4° | 68.5 |
| 13 | `sqlab311_flx_16_30` | SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm) | 740 | 30 | knuckle | 16° | 4° | 82.0 |
| 14 | `sqlab311_flx_12_15` | SQlab 311 FL-X Carbon 12°, 15 mm rise (740 mm) | 740 | 15 | knuckle | 12° | 4° | 68.5 |
| 15 | `sqlab311_flx_12_30` | SQlab 311 FL-X Carbon 12°, 30 mm rise (740 mm) | 740 | 30 | knuckle | 12° | 4° | 82.0 |

The default is `persuader20_800`, the same bar the workbench installs today, so every existing number stays the same.

**Provenance labels in `fixture.go`** (AGENTS.md: label values as measured, catalog, or illustrative; never present an invented value as manufacturer truth). One comment above the list: `// catalog: ported from legacy internal/catalog/handlebars.go; sources not recorded (Q20)`. One comment on the SQlab bend starts: `// illustrative: legacy SQlabEffectiveBendStartMm = 55 + 0.9·rise`. One on the grip inset constant: `// illustrative: the rider's grips (legacy WingGripInsetMm), the same on every bar`.

## 4. Code changes: one commit

Everything is in `internal/reboot/web`: `fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`, and a new `handlebar_parity_test.go`. `make deadcode` fails on unexported functions nothing reaches, so the fixture helpers and their callers land together.

Commit subject: `Choose the handlebar from a legacy-catalog list in the reboot workbench`.

### C1. Fixture (`fixture.go`)

Add:

```go
// handlebarOption is one handlebar the workbench can install. Setup.BarRollDeg
// is always 0: the roll comes from the workbench state (Q21).
type handlebarOption struct {
	ID    string
	Name  string
	Setup assembly.HandlebarSetup
}

// resolveHandlebar returns the handlebar with requestedID, or the default
// handlebar when requestedID is empty.
func (f bikeFixture) resolveHandlebar(requestedID string) (handlebarOption, bool)
```

In `bikeFixture`, **remove** `HandlebarName` and `DefaultHandlebar` and add:

```go
DefaultHandlebarID string
Handlebars         []handlebarOption
```

Build the list in `otsoFenrirFixture()` with one small constructor so each row is one line:

```go
// legacyHandlebar builds an uncut handlebar with the rider's grips and no roll.
func legacyHandlebar(id, name string, widthMm, riseMm float64, mode assembly.RiseMeasurementMode, backsweep, upsweep pose.Deg, bendStartMm float64) handlebarOption
```

It sets `StockWidthMm` and `CurrentWidthMm` to `widthMm`, `GripInsetMm` to a `const gripInsetMm = 70.0` declared beside it, and `BarRollDeg` to 0. Use the §3 table values, in its order.

In `bikeFixture.validate()`, add after the fork checks:

- `DefaultHandlebarID` is non-empty and matches a bar: `fixture %q DefaultHandlebarID %q not found in Handlebars`.
- Every bar ID is non-empty and unique: `fixture %q has handlebar option with empty ID` / `fixture %q has duplicate handlebar ID %q`.
- Every bar has `Setup.BarRollDeg == 0`: `fixture %q handlebar %q must not set BarRollDeg`.
- Every bar places: `assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0, nil), opt.Setup)` returns no error, else `fixture %q handlebar %q invalid: %w`. This reuses assembly's own checks; don't restate them in `web`.

### C2. Workbench state (`web.go`)

- `workbenchState` gains `BarID string`, right after `ForkID`.
- `defaultWorkbenchState`: `BarID` is the default bar's ID, and `BarRollDeg` comes from the default bar's `Setup.BarRollDeg` (0) in place of `fix.DefaultHandlebar.BarRollDeg`.
- `workbenchStateFromSetup(view diagramView, forkID, barID string, setup assembly.BikeSetup)` sets `BarID: barID`. Update all four callers: `buildFittingView` and `buildStockFittingView` pass `state.BarID`; `withFitTargets` passes `submitted.BarID`; `buildBikeView` passes its new `barID` argument. **This is the step that keeps `bar` on the apply links.** A missed caller silently drops back to the default bar, and the C6 tests catch it.
- `URL()`: `q.Set("bar", s.BarID)` right after `fork`.
- `bikeFormValues` gains `Bar string` (after `Fork`); `formValues()` sets it from `BarID`.

### C3. Decoding (`decodeWorkbenchState`)

Right after the fork block (before `spacers`), resolve the bar the way the fork is resolved:

```go
selectedBar, barOK := fix.resolveHandlebar(q.Get("bar"))
if barOK {
	state.BarID = selectedBar.ID
	form.Bar = selectedBar.ID
} else {
	state.BarID = ""
	p.fail("bar", fmt.Errorf("unknown handlebar %q", q.Get("bar")))
	// Build with the default bar so the error page can echo the other controls.
	selectedBar, _ = fix.resolveHandlebar("")
}
```

Replace `bar := fix.DefaultHandlebar` with `bar := selectedBar.Setup`, then keep `bar.BarRollDeg = state.BarRollDeg` (Q21). An empty or missing `bar` resolves to the default, as an empty `fork` does.

### C4. View (`web.go`)

- `bikeView` gains `Handlebars []handlebarOption` and `SelectedBarID string`, beside `Forks` and `SelectedForkID`.
- `buildBikeView(fix, view, forkID, barID, bike, cadBaselineFork, steps)`: the new `barID` parameter follows `forkID`. Set `Handlebars: fix.Handlebars`, `SelectedBarID: barID`, and `HandlebarName` from `fix.resolveHandlebar(barID)`'s `Name`. The handler passes `resolved.State.BarID`.
- `renderBikeError`: set `Handlebars: fix.Handlebars` and `SelectedBarID: resolved.State.BarID`, so the select renders on error pages (with no option selected for an unknown bar, as for an unknown fork).
- No other web code changes. The diagram's handlebar segment, the HandGrip landmark, and the readouts already come from the placed setup.

### C5. Template (`bike.html`)

In the **Front Cockpit** fieldset, add the select as the **first** field, before Spacers:

```html
<div class="field field-full">
  <label for="ctrl-bar">Handlebar</label>
  <select id="ctrl-bar" name="bar"{{if and $.ErrorMessage (eq $.InvalidField "bar")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
    {{range .Handlebars}}
    <option value="{{.ID}}"{{if eq .ID $.SelectedBarID}} selected{{end}}>{{.Name}}</option>
    {{end}}
  </select>
</div>
```

No CSS or JS change. `workbench.js` listens for `change` on the whole form, so the select submits like the fork select. If you find otherwise, add a `workbench_test.js` case.

### C6. Tests

**Update existing tests** (`web_test.go`):

- Replace each `fix.DefaultHandlebar` (5 places) with a test helper `defaultHandlebarSetup(t, fix)` that returns `fix.resolveHandlebar("")`'s `Setup` and fails the test when it isn't found.
- Update the two `buildBikeView(...)` calls for the new `barID` argument (pass `"persuader20_800"` or `resolved.State.BarID`).
- The form label count goes from 16 to 17.
- `TestBikeWorkbenchRejectsInvalidQuery`: add `{"/bike?bar=unknown", "bar", ""}`.
- `TestBikeWorkbenchFormRoundTripPreservesEveryBuild`: add `&bar=sqlab3ox_16_45` to the custom build URL, and add `{field: "bar", val: "sqlab311_flx_12_30", isSelect: true}` to the mutations.
- Every other existing test must pass unchanged. The default bar is the old fixed bar, so every existing number holds.

**New tests** (`web_test.go`, browserless `httptest` + `goquery` per AGENTS.md):

1. **Fixture bars place at the reference HandGrips.** For each `fix.Handlebars` entry, decode the default build with `bar=<id>`, `PlaceBike` it, and check `Handlebar.HandGrip.Origin()` against the §5 table within 1e-3, plus `HandLateralMm` exactly. Check that `len(fix.Handlebars) == 15` and the IDs match the §3 order.
2. **Default page.** `/bike` has `select[name="bar"]` with 15 options, the selected one is `persuader20_800`, and `main[data-canonical-url]` has `bar=persuader20_800`. The `brp-grip-dist` readout still says `556.6 mm`.
3. **A chosen bar changes the build.** `/bike?bar=sqlab3ox_12_15&view=stem`: the selected option is `sqlab3ox_12_15`, `hand-lateral` is `320.0 mm`, `brp-grip-dist` is `573.6 mm`, and `sitbones-grip-rise` is `+5.8 mm`. The `brp-grip-dist` label text contains `SQlab 3OX 12°, 15 mm rise (780 mm)`.
4. **Roll is kept across a bar change (Q21).** Load `/bike?bar_roll=5`, then submit the form with `bar` changed to `sqlab3ox_16_30` using the existing `submitForm` helper. The new page's `bar_roll` input is `5.0` and its selected bar is `sqlab3ox_16_30`.
5. **Links keep the bar.** On `/bike?bar=sqlab3ox_16_30&view=seat`, every `a[data-diagram-view]` href and the CAD Baseline Fork preset href have `bar=sqlab3ox_16_30`. The Reset Defaults href has `bar=persuader20_800`.
6. **Fitting uses the chosen bar and keeps it on apply links.** `/bike?bar=sqlab3ox_12_15&grip_x=481&grip_y=726&view=stem`:
   - `fit-cockpit-outcome` is `limited by stem spacers` (stem length is *not* limited with this bar), `fit-spacers` is `0.0 mm`, and `fit-stem-length` is `124.5 mm`.
   - `#apply-fitting`'s href has `bar=sqlab3ox_12_15`.
   - Stock row 1: stem `120 mm, -17°`, spacers `0 mm`, oval-r `0.45`, total `88.6`. Its apply href has `bar=sqlab3ox_12_15`, `stem_length=120.0`, `stem_angle=-17.0`, and `spacers=0.0`.
   - Following row 1's apply href renders with `sqlab3ox_12_15` selected and row 1 still at `120 mm, -17°`.
7. **Fixture validation.** Unit-test `bikeFixture.validate()` on broken copies of `otsoFenrirFixture()`: unknown `DefaultHandlebarID`, an empty bar ID, a duplicate bar ID, a bar with `BarRollDeg = 5`, and a bar with `GripInsetMm = 0` (PlaceHandlebar's error). Each returns an error that names the problem.

**Parity test** (new file `internal/reboot/web/handlebar_parity_test.go`, package `web`). Only `*_parity_test.go` files may import legacy packages (`make boundary`). For every fixture bar, `catalog.GetHandlebarPreset(opt.ID)` must be found, and these must be equal: stock width, current width, stated rise, backsweep, upsweep, bend start (`EffectiveBendStartMm`; exact `==`, since 55 + 0.9·rise gives exactly the literal doubles for rises 15, 25, 30, 45, and 50), and `string(RiseMode)`. Also check that the fixture's ID order matches `catalog.CommonHandlebarCatalog` lowercased. Per DESIGN.md, a parity test explains differences rather than assuming the legacy value is right. Here the claim is only "ported exactly", so equality is the right check. `GripInsetMm` and `BarRollDeg` have no legacy counterpart; don't check them.

## 5. Reference numbers (default Fenrir build, measured 2026-11-19 with an independent probe)

Default build: Lithic fork, EC44 12 mm cup, spacers 20, stem 60 mm at −6°, bar roll 0°, post extension 263.5, rail offset 0, grip inset 70. The probe installed each bar with the §3 values. The Persuader 800 row reproduces today's numbers (HandGrip (389.218, 755.120) and Slice 3's stock total 78.711983), which cross-checks the probe.

| ID | HandGrip | BRP→HandGrip | Rise vs SitBones | Lateral |
| --- | --- | --- | --- | --- |
| `persuader20_800` | (389.218, 755.120) | 556.624 | +18.287 | 330.0 |
| `persuader20_840` | (381.938, 755.120) | 549.354 | +18.287 | 350.0 |
| `sqlab3ox_16_15` | (388.075, 742.602) | 554.986 | +5.769 | 320.0 |
| `sqlab3ox_16_30` | (391.946, 756.658) | 559.429 | +19.825 | 320.0 |
| `sqlab3ox_16_45` | (395.817, 770.714) | 564.213 | +33.881 | 320.0 |
| `sqlab3ox_12_15` | (406.734, 742.602) | 573.638 | +5.769 | 320.0 |
| `sqlab3ox_12_30` | (409.603, 756.658) | 577.062 | +19.825 | 320.0 |
| `sqlab3ox_12_45` | (412.473, 770.714) | 580.820 | +33.881 | 320.0 |
| `sqlab311_20_16_15` | (393.810, 745.269) | 560.800 | +8.436 | 300.0 |
| `sqlab311_20_16_25` | (396.391, 754.481) | 563.756 | +17.649 | 300.0 |
| `sqlab311_20_16_50` | (402.843, 777.513) | 571.780 | +40.680 | 300.0 |
| `sqlab311_flx_16_15` | (393.810, 741.203) | 560.681 | +4.370 | 300.0 |
| `sqlab311_flx_16_30` | (397.681, 755.259) | 565.084 | +18.426 | 300.0 |
| `sqlab311_flx_12_15` | (410.985, 741.203) | 577.851 | +4.370 | 300.0 |
| `sqlab311_flx_12_30` | (413.854, 755.259) | 581.237 | +18.426 | 300.0 |

Fitting to the fit report's HandGrip (481, 726), with stock fitting also given the fit report's BRP (−219, 713.1). Stock rank 1 is (angle, length, spacers):

| ID | Continuous fit (spacers, length, limited by) | Stock rank 1 | Stock total | Oval R |
| --- | --- | --- | --- | --- |
| `persuader20_800` | 0, 130, spacers + length | −17°, 130, 0 | 78.711983 | 1.358546 |
| `sqlab3ox_12_15` | 0, 124.538, spacers | −17°, 120, 0 | 88.6 | 0.452807 |
| `sqlab311_flx_12_15` | 0, 120.850, spacers | −17°, 110, 0 | 90.6 | 0.806233 |
| `sqlab3ox_16_45` | 0, 127.131, spacers | −17°, 130, 0 | 70.108813 | 1.749599 |

Findings worth noting (not defects). With the 12° SQlab bars, the fit report HandGrip lands inside the oval with a stock stem (R < 1), which the Persuader 800 can't do. The higher-rise bars score worse, because the target is low. The 840 mm Persuader puts HandGrip 7.3 mm further back than the 800 mm one: the extra 20 mm per side is all outboard of the bend, so it's swept back by 20°. If your numbers differ by more than the stated tolerances, find out why before changing a test.

## 6. Not in this slice

Bars or roll in stock fitting (the next slice); roll choice; cutting a bar (a `CurrentWidthMm` control); grip choice or a per-bar grip inset; a handlebar outline in the diagram; the `/` stem preview page; CLI/MCP access; other frames; any change to `assembly` or `pose`.

## 7. Rules that bite here

- **No assembly change.** If you think you need one, stop and ask Matt.
- **The web never recomputes geometry.** The bar's effect reaches the page only through `PlaceBike` on the decoded setup. Fixture validation calls `assembly.PlaceHandlebar` and doesn't restate its checks.
- **The URL is the only state**, and the page must still work as a plain `GET` form without JS (DESIGN.md). `bar` must round-trip through the form, the canonical URL, and every link the page builds.
- **Q21:** never reset or change `bar_roll` because the bar changed.
- `boundary`: only `handlebar_parity_test.go` imports `bikegeo/internal/catalog`. Nothing else in `internal/reboot` may.
- `deadcode`: `legacyHandlebar` and `resolveHandlebar` must be reached from production code (the fixture and `decodeWorkbenchState`). `defaultHandlebarSetup` lives in `web_test.go`.
- `exhaustive`: if you switch on `RiseMeasurementMode`, name both constants. (The plan doesn't need such a switch.)
- Placements must be identical with tracing on or off. (Nothing here touches tracing.)
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 4 done at the end, and only after Matt accepts it.

## 8. Verification

After the code compiles, run `go vet ./... && go test ./...` after every change. At the end, run `make pre-commit` and re-run it until `go fix` is clean. Then check it live: `make serve-reboot`, then open `http://127.0.0.1:8081/bike?bar=sqlab3ox_12_15&grip_x=481&grip_y=726&view=stem`. Confirm the Handlebar select shows the SQlab 3OX 12° 15 mm bar, the continuous block says "limited by stem spacers" at 124.5 mm, and stock row 1 is 120 mm at −17° with total 88.6. Change the select to Persuader 840 and confirm the page updates with `bar_roll` unchanged. Click stock row 1's Apply and confirm the bar stays selected. Take a screenshot for Matt.
