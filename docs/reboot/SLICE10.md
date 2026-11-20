# M3 Slice 10, chart closure check

**Status: Implemented 2026-11-19; awaiting Matt's acceptance.** Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. Slice numbers keep counting across milestones, so this is the first M3 slice. It builds on Slice 1's frameset recipe (`PlaceCadBaselineFork`) and on nothing from Slices 2–9.

## Implementation handoff

- **Who and when:** 2026-11-19, Gemini 4 Hydrogen (Medium).
- **Commits:**
  - `8b91fb1` — `Check the reboot fixture chart closure against its rounding budget at startup`
  - `2626d1f` — `Drop the reboot closure residual readout`
- **Done and not done:** Done: A1–A6, R1–R5, startup probe, live check, and handoff. Not done: none.
- **Deviations:** None.
- **Verification:**
  - `go vet ./... && go test ./...` and `make pre-commit` passed cleanly after both commits (all Go tests and 21/21 JS tests passed; `go fix` made no changes).
  - `grep -rn "Residuals\|ResidualAxle\|ResidualWheelbase\|Closure Residuals" internal/reboot` returned only `chart_closure.go` (`residuals`) and the negative assertion in `web_test.go`.
  - Startup probe (`StackMm = 644.0`) panicked as expected: `panic: fixture "Otso Fenrir Stainless (2024, Size L)" chart: chart closure: axle height residual +2.22 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)`. Reverted and confirmed `git diff internal/reboot/web/fixture.go` is empty.
  - Live check (`make serve-reboot`, `http://127.0.0.1:8081/bike`): confirmed `CAD Baseline Fork (495 mm / 54 mm / 0 mm cup): 68.26° HTA, 1157.7 mm WB (Published 1158.0 mm)` and `ΔPitch vs CAD: +0.55°`, and confirmed no `"0° Closure Residuals"` text appears on the page. Screenshot saved at `/Users/matt/.gemini/antigravity/brain/4e14beb9-16ab-4f95-ad70-6c0d5f45ff22/scratch/slice10_summary.png`.
- **Open issues:** None.

This slice has two commits and touches 8 code, test, and template files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Published chart numbers are rounded, so the Fenrir's 0° chart closure doesn't balance exactly: with the CAD Baseline Fork, the front axle sits +0.22 mm above the rear axle, and the axle span is −0.34 mm off the published wheelbase. Today `/bike` prints those two numbers and nothing judges them. After this slice:

- `FrameChart` declares the rounding half-width of each column the closure reads (`ChartPrecision`, Q43).
- `assembly.CheckChartClosure(chart)` returns an error when either residual exceeds what rounding of the published columns can explain (Q44, Q45).
- `web.NewHandler` runs it once on the fixture chart and panics on failure. The residual readout leaves `/bike`, and `ChartBaseline.Residuals` is removed (Q47).

The Fenrir passes easily: axle height +0.216 mm against a ±2.089 mm budget, and wheelbase −0.338 mm against ±3.017 mm.

Rulings: **Q43–Q47** as recorded in DESIGN.md. AGENTS.md's "Trig lives in `pose`" rule applies: this slice adds no trig, because each term comes from rerunning the forward recipe.

## 2. Terminology

- **Residual:** the 0°-pitch mismatch of a closure check. **Axle height** residual = `FrontAxle.Y − RearAxle.Y`. **Wheelbase** residual = `(FrontAxle.X − RearAxle.X) − PublishedWheelbaseMm`. These are today's `ChartResiduals` expressions, unchanged.
- **Half-width:** the most a published column can differ from the true value through rounding (a length printed as 642 is 642 ± 0.5).
- **Term:** for one column, the larger change in a residual when that column moves by +half-width or −half-width. **Rounding budget:** the sum of a check's terms.
- Don't use the scoring words banned by `make boundary` (DESIGN.md "Scoring words"). "Budget" and "term" are fine.

## 3. Commit 1: check the chart closure at startup

Subject: `Check the reboot fixture chart closure against its rounding budget at startup`

Files: `internal/reboot/assembly/chart_closure.go` (new), `internal/reboot/assembly/chart_closure_test.go` (new), `internal/reboot/assembly/frameset.go`, `internal/reboot/web/fixture.go`, `internal/reboot/web/web.go`, `internal/reboot/web/web_test.go`.

The readout and `ChartBaseline.Residuals` stay in this commit. Commit 2 removes them.

### A1. `frameset.go`

Add a field to `FrameChart`, after `PublishedWheelbaseMm`:

```go
	Precision            ChartPrecision // rounding half-widths of the published columns (Q43)
```

In `PlaceCadBaselineFork`, compute the residuals through the new helper instead of inline. Replace the `unpitched, pitchDeg := …` line and the `residuals := ChartResiduals{…}` literal with:

```go
	_, pitchDeg := measureUnpitchedFrameset(chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork)
	r := chartResiduals(chart)
	residuals := ChartResiduals{AxleHeightDiffMm: r[0], WheelbaseDiffMm: r[1]}
```

Nothing else in `frameset.go` changes in this commit.

### A2. `chart_closure.go` (new, `package assembly`)

Write this file exactly, apart from `gofmt` alignment:

```go
package assembly

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"bikegeo/internal/reboot/pose"
)

// ChartPrecision holds the rounding half-width of each published chart column
// the closure checks read: a length printed as 642 could be anything in
// 642 ± 0.5 before the manufacturer rounded it. Zero means the column is exact
// and gets no term. The CAD fork and CAD lower cup are hypotheses, not rounded
// chart numbers, so they have no half-width (Q44).
type ChartPrecision struct {
	ReachMm          float64
	StackMm          float64
	HeadTubeAngleDeg pose.Deg
	HeadTubeLengthMm float64
	BbDropMm         float64
	ChainstayMm      float64
	WheelbaseMm      float64
}

// closureEpsilonMm absorbs floating-point noise: a term no larger than this is
// dropped, and a residual may exceed its budget by this much and still pass.
const closureEpsilonMm = 1e-9

// closureCheckNames names the two residuals chartResiduals returns, in order.
var closureCheckNames = [2]string{"axle height", "wheelbase"}

// chartColumns lists the published columns the closure checks read, in chart
// order, with how to read each column's half-width and how to move it.
var chartColumns = []struct {
	name      string
	halfWidth func(ChartPrecision) float64
	nudge     func(*FrameChart, float64)
}{
	{"Reach", func(p ChartPrecision) float64 { return p.ReachMm }, func(c *FrameChart, d float64) { c.Frame.ReachMm += d }},
	{"Stack", func(p ChartPrecision) float64 { return p.StackMm }, func(c *FrameChart, d float64) { c.Frame.StackMm += d }},
	{"HTA", func(p ChartPrecision) float64 { return float64(p.HeadTubeAngleDeg) }, func(c *FrameChart, d float64) { c.Frame.HeadTubeAngleDeg += pose.Deg(d) }},
	{"HTL", func(p ChartPrecision) float64 { return p.HeadTubeLengthMm }, func(c *FrameChart, d float64) { c.Frame.HeadTubeLengthMm += d }},
	{"BB drop", func(p ChartPrecision) float64 { return p.BbDropMm }, func(c *FrameChart, d float64) { c.Frame.BbDropMm += d }},
	{"Chainstay", func(p ChartPrecision) float64 { return p.ChainstayMm }, func(c *FrameChart, d float64) { c.Frame.ChainstayMm += d }},
	{"Wheelbase", func(p ChartPrecision) float64 { return p.WheelbaseMm }, func(c *FrameChart, d float64) { c.PublishedWheelbaseMm += d }},
}

// CheckChartClosure checks that the chart's 0° closure residuals with its CAD
// baseline fork are no larger than rounding of the published columns can
// explain (Q45). Each column's term is the larger change in a residual when the
// column moves by ± its half-width and the unpitched frameset recipe is rerun;
// a check's rounding budget is the sum of its terms. It returns one error per
// failing check, joined, axle height first.
func CheckChartClosure(chart FrameChart) error {
	if err := validateFrameGeometry(chart.Frame); err != nil {
		return err
	}
	if err := validateForkGeometry(chart.CadFork); err != nil {
		return err
	}
	if err := chart.CadLowerCup.Validate(); err != nil {
		return err
	}
	if !isFinite(chart.PublishedWheelbaseMm) || chart.PublishedWheelbaseMm <= 0 {
		return fmt.Errorf("published wheelbase must be finite and positive, got %v mm", chart.PublishedWheelbaseMm)
	}
	for _, col := range chartColumns {
		if hw := col.halfWidth(chart.Precision); !isFinite(hw) || hw < 0 {
			return fmt.Errorf("chart precision: %s half-width must be finite and non-negative, got %v", col.name, hw)
		}
	}

	residuals := chartResiduals(chart)
	var budgets [2]float64
	var terms [2][]string
	for _, col := range chartColumns {
		hw := col.halfWidth(chart.Precision)
		if hw == 0 {
			continue
		}
		up, down := chart, chart
		col.nudge(&up, hw)
		col.nudge(&down, -hw)
		for _, moved := range []FrameChart{up, down} {
			if err := validateFrameGeometry(moved.Frame); err != nil {
				return fmt.Errorf("chart precision: %s ±%v moves the chart out of range: %w", col.name, hw, err)
			}
		}
		ru, rd := chartResiduals(up), chartResiduals(down)
		for i := range residuals {
			term := math.Max(math.Abs(ru[i]-residuals[i]), math.Abs(rd[i]-residuals[i]))
			if term <= closureEpsilonMm {
				continue
			}
			budgets[i] += term
			terms[i] = append(terms[i], fmt.Sprintf("%s %.2f mm", col.name, term))
		}
	}

	var errs []error
	for i, name := range closureCheckNames {
		if math.Abs(residuals[i]) <= budgets[i]+closureEpsilonMm {
			continue
		}
		listed := "no rounded columns"
		if len(terms[i]) > 0 {
			listed = strings.Join(terms[i], ", ")
		}
		errs = append(errs, fmt.Errorf("chart closure: %s residual %+.2f mm exceeds its ±%.2f mm rounding budget (%s)", name, residuals[i], budgets[i], listed))
	}
	return errors.Join(errs...)
}

// chartResiduals returns the chart's 0°-pitch closure residuals with its CAD
// baseline fork: FrontAxle.Y − RearAxle.Y, then the axle X span minus the
// published wheelbase.
func chartResiduals(chart FrameChart) [2]float64 {
	unpitched, _ := measureUnpitchedFrameset(chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork)
	return [2]float64{
		unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle),
		(unpitched.FrontAxle.Origin().X - unpitched.RearAxle.Origin().X) - chart.PublishedWheelbaseMm,
	}
}
```

Terms are listed in chart order (the `chartColumns` order), not sorted. `isFinite`, `validateFrameGeometry`, and `validateForkGeometry` already exist in the package.

### A3. `fixture.go`

In `otsoFenrirFixture`, add to the `assembly.FrameChart` literal, after `PublishedWheelbaseMm: 1158.0,`:

```go
			// illustrative: inferred from the published grid (whole mm; HTA in quarter degrees), Q43
			Precision: assembly.ChartPrecision{
				ReachMm:          0.5,
				StackMm:          0.5,
				HeadTubeAngleDeg: 0.125,
				HeadTubeLengthMm: 0.5,
				BbDropMm:         0.5,
				ChainstayMm:      0.5,
				WheelbaseMm:      0.5,
			},
```

And put this comment on the line above `CadLowerCup:` in the same literal:

```go
			// hypotheses, not rounded chart numbers: CadLowerCup and CadFork get no rounding terms (Q44)
```

### A4. `web.go`

Replace `NewHandler` and add `checkFixtureChart` right after it:

```go
// NewHandler returns the HTTP handler for the reboot workbench. It panics if
// the fixture chart fails its closure check (Q47).
func NewHandler() http.Handler {
	checkFixtureChart(otsoFenrirFixture())
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleStemWorkbench)
	mux.HandleFunc("/bike", handleBikeWorkbench)
	mux.HandleFunc("/static/workbench.js", handleWorkbenchJS)
	return mux
}

// checkFixtureChart panics unless the fixture chart closes within its rounding budget.
func checkFixtureChart(fix bikeFixture) {
	if err := assembly.CheckChartClosure(fix.Chart); err != nil {
		panic(fmt.Sprintf("fixture %q chart: %v", fix.Name, err))
	}
}
```

Leave the per-request `fix.validate()` calls alone (out of scope).

### A5. `chart_closure_test.go` (new, `package assembly_test`)

Helper, using the existing `fenrirSizeLFrame()` and `fenrirCadCup()` from `frameset_test.go`:

```go
func fenrirChart() assembly.FrameChart {
	return assembly.FrameChart{
		Frame:                fenrirSizeLFrame(),
		CadLowerCup:          fenrirCadCup(),
		CadFork:              assembly.ForkGeometry{A2cMm: 495.0, RakeMm: 54.0},
		PublishedWheelbaseMm: 1158.0,
		Precision: assembly.ChartPrecision{
			ReachMm: 0.5, StackMm: 0.5, HeadTubeAngleDeg: 0.125, HeadTubeLengthMm: 0.5,
			BbDropMm: 0.5, ChainstayMm: 0.5, WheelbaseMm: 0.5,
		},
	}
}
```

Expected values come from an independent closed-form probe (Python, written from the leveling geometry, not from this code): at 0° pitch the axle height residual is `Stack − (HTL + A2C)·sin HTA + rake·cos HTA − BbDrop` = +0.216036 mm, and the wheelbase residual is `Reach + (HTL + A2C)·cos HTA + rake·sin HTA + √(CS² − BbDrop²) − WB` = −0.337655 mm. The probe's terms:

| Column | Axle height term (mm) | Wheelbase term (mm) |
| --- | --- | --- |
| Reach | 0 | 0.500000 |
| Stack | 0.500000 | 0 |
| HTA | 0.624132 | 1.243754 |
| HTL | 0.464405 | 0.185279 |
| BB drop | 0.500000 | 0.081333 |
| Chainstay | 0 | 0.506533 |
| Wheelbase | 0 | 0.500000 |
| **Budget** | **2.088537** | **3.016899** |

Moving Stack moves only the axle height residual and leaves every term as it is. Moving the published wheelbase moves only the wheelbase residual and leaves every term as it is. The edge cases below use that.

Tests (compare error strings exactly unless a test says otherwise):

1. **`TestCheckChartClosureFenrirPasses`:** `CheckChartClosure(fenrirChart())` returns nil.
2. **`TestCheckChartClosureBudgetEdges`** (table-driven):

   | Case | Change from `fenrirChart()` | Residual | Want |
   | --- | --- | --- | --- |
   | axle height inside | `Frame.StackMm = 643.8` | +2.0160 | nil |
   | axle height outside | `Frame.StackMm = 643.95` | +2.1660 | `chart closure: axle height residual +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)` |
   | wheelbase inside | `PublishedWheelbaseMm = 1160.6` | −2.9377 | nil |
   | wheelbase outside | `PublishedWheelbaseMm = 1160.8` | −3.1377 | `chart closure: wheelbase residual -3.14 mm exceeds its ±3.02 mm rounding budget (Reach 0.50 mm, HTA 1.24 mm, HTL 0.19 mm, BB drop 0.08 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)` |

   The minus sign in `-3.14` is ASCII, from `%+.2f`. The two failing lists also pin Q44: no A2C, rake, or cup term appears.
3. **`TestCheckChartClosureWithoutPrecision`:** `fenrirChart()` with `Precision = assembly.ChartPrecision{}`. Want exactly `"chart closure: axle height residual +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)\nchart closure: wheelbase residual -0.34 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"`.
4. **`TestCheckChartClosureInvalidPrecision`** (table-driven):
   - `Precision.StackMm = math.NaN()` → `chart precision: Stack half-width must be finite and non-negative, got NaN`
   - `Precision.HeadTubeAngleDeg = -0.1` → `chart precision: HTA half-width must be finite and non-negative, got -0.1`
5. **`TestCheckChartClosureNudgeOutOfRange`:** `fenrirChart()` with `Frame.ChainstayMm = 72.4` and `Precision = assembly.ChartPrecision{ChainstayMm: 0.5}`. The chart itself is valid (72.4 > 72), but 71.9 mm is not. Want an error whose text starts with `chart precision: Chainstay ±0.5 moves the chart out of range: ` (prefix check; the wrapped validation text follows).
6. **`TestCheckChartClosureInvalidWheelbase`:** `PublishedWheelbaseMm` of `0` and of `math.NaN()` each give an error containing `published wheelbase must be finite and positive`.

### A6. `web_test.go`

Add **`TestCheckFixtureChart`**:

- `checkFixtureChart(otsoFenrirFixture())` does not panic.
- With `fix := otsoFenrirFixture(); fix.Chart.Precision = assembly.ChartPrecision{}`, `checkFixtureChart(fix)` panics. Recover it, and check that the recovered value is a `string` containing `fixture "Otso Fenrir Stainless (2024, Size L)" chart: chart closure: axle height residual +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)`.

No other web test changes in this commit.

## 4. Commit 2: drop the residual readout

Subject: `Drop the reboot closure residual readout`

Files: `internal/reboot/assembly/frameset.go`, `internal/reboot/assembly/frameset_test.go`, `internal/reboot/web/web.go`, `internal/reboot/web/templates/bike.html`, `internal/reboot/web/web_test.go`.

### R1. `frameset.go`

- Delete the `ChartResiduals` type and its comment.
- `ChartBaseline` keeps only `Placement FramesetPlacement`. Its comment becomes `// ChartBaseline holds the leveled CAD baseline fork frameset placement.`
- `PlaceCadBaselineFork`: delete the published wheelbase check (it lives in `CheckChartClosure` now), the `r := chartResiduals(chart)` and `residuals := …` lines, and the `Residuals:` field in the returned literal. Its comment becomes `// PlaceCadBaselineFork runs the unpitched (0°) frameset recipe once to measure the leveling pitch, then builds the leveled CAD baseline fork frameset.` (wrapped like the other comments).

### R2. `frameset_test.go` (`TestPlaceCadBaselineFork`)

- Delete the two `baselineFork.Residuals` checks ("want within 1.5 mm rounding"). `TestCheckChartClosureFenrirPasses` replaces them.
- Delete the zero and NaN `PublishedWheelbaseMm` cases. `TestCheckChartClosureInvalidWheelbase` replaces them.
- Every other case stays.

### R3. `web.go`

- Delete the `ResidualAxleYMm` and `ResidualWheelbaseMm` fields from the bike view struct and their two assignments in `buildBikeView` (`cadBaselineFork.Residuals.…`).

### R4. `bike.html`

Delete the whole `<span>0° Closure Residuals: …</span>` line in the CAD Baseline `summary` div. The CAD Baseline Fork span and the ΔPitch span stay.

### R5. `web_test.go`

In the test that checks `delta-pitch` (around L294–306 at `ef91e10`), replace the `residual-axle-y` and `residual-wheelbase` assertions with:

```go
	if n := doc.Find(`[data-metric^="residual-"]`).Length(); n != 0 {
		t.Errorf("found %d closure residual readouts, want none (Q47)", n)
	}
	if strings.Contains(doc.Text(), "Closure Residuals") {
		t.Error("page still says Closure Residuals, want no readout (Q47)")
	}
```

Then do the implementation handoff (section 7).

## 5. Rules that bite

- **Each commit passes `make pre-commit` alone.** Commit 1 must call `CheckChartClosure` from `main` (through `NewHandler`), or `deadcode` rejects it.
- **No trig outside `pose`.** Moving HTA is `+= pose.Deg(d)`, plain arithmetic. Don't add a closed-form residual to non-test code; the forward recipe is the source of truth.
- **The residual expressions don't change.** `chartResiduals` uses exactly today's `VerticalOffsetTo` and X-span expressions, so the CAD Baseline numbers on `/bike` (HTA, WB, ΔPitch) are unchanged. If an existing web test's number moves, stop.
- **Never nudge chart numbers.** If the Fenrir fails the check, stop and report it. Don't change a published value or a half-width to make it pass.
- **Don't touch legacy code.** Read `internal/geometry/budget.go` for background if you like, but port nothing else from it: no grid inference, tiers, allowances, or sorting (Q43, Q45).
- `make boundary`: no banned scoring words in the new files.
- **Finish with the implementation handoff** in DESIGN.md "Slice planning" → "Implementation handoff", whether you finished, stopped early, or got blocked.

## 6. Verification

1. After every edit: `go vet ./... && go test ./...`. Before each commit: `make pre-commit`, rerun until `go fix` is clean.
2. After commit 2: `grep -rn "Residuals\|ResidualAxle\|ResidualWheelbase\|Closure Residuals" internal/reboot` prints nothing.
3. **Startup probe** (don't commit it): temporarily set the fixture's `StackMm` to `644.0`, run `go run ./cmd/bikegeo-reboot serve --port 0`, and confirm it panics with `fixture "Otso Fenrir Stainless (2024, Size L)" chart: chart closure: axle height residual +2.22 mm exceeds its ±2.09 mm rounding budget (…)`. Revert and confirm `git diff` is clean for `fixture.go`.
4. **Live check:** `make serve-reboot`, open `http://127.0.0.1:8081/bike`. Confirm the CAD Baseline Fork span (its HTA, its WB, and `Published 1158.0 mm`) and the ΔPitch span still render with the same numbers as before commit 2, and that no "0° Closure Residuals" text appears. Save a screenshot of that summary line to the session scratchpad and name its full path in the handoff.

## 7. Implementation handoff

Required. Follow DESIGN.md "Slice planning" → "Implementation handoff": set this file's Status line, add the `## Implementation handoff` section right after the Status paragraph, and set the Slice 10 Milestones line in DESIGN.md to the same status words.

## 8. Out of scope

- Showing budgets, terms, or residuals anywhere in the workbench (Q47).
- Front center or A2C closure checks, GeoSlopMm-style allowances, tiers, and precision inference (Q43, Q45).
- Other frames, other sizes, and catalog storage (Q4).
- Moving the per-request `fix.validate()` calls to startup.
- Evaluating fork, sag, or cup hypotheses (later M3 work).
