# Slice 16a: chart data ownership, the unpitched placement, and "basis"

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-21, Gemini 4 Neon (Medium).
- **Commits:**
  - C1 `Place the basis frameset with PlaceFrameset`: `0532e01`
  - C2 `Move the chart-consistency check to catalog tests`: `1db1eb8`
  - C3 `Move chart data to catalog and delete FrameChart`: `7e444ee`
  - C4 `Rename CAD to basis`: `c66571d`
- **Done and not done:** C0, C1 (A1–A3, T1–T4), C2 (A4–A6, T5–T7), C3 (A7–A11, T8–T10), C4 (R1–R5, A12), and all Verification steps 1–6 are done. Nothing is left undone.
- **Deviations:** None.
- **Verification:**
  - `make pre-commit` passed on each of C1–C4 and on the final state.
  - Snapshot comparison (`bike.json`, `bike_basis.json`, `compare.json`, and 5 rendered HTML pages) was byte-identical after C1, C2, and C3, and empty after the 5 planned substitutions on C4 (no extra substitutions needed).
  - `go test -run 'ChartConsistency' -v ./internal/reboot/catalog` listed and passed every T5 test plus `TestCheckChartConsistencyRejectsMismatchedBasisCup` and `TestMeasureChartConsistencyExposesChecks`. `go test -v ./internal/reboot/assembly` listed zero `ChartConsistency` tests.
  - R5 negative check: temporarily appending `// CAD` to `internal/reboot/pose/pose.go` made `make boundary` fail with `boundary: reboot files must not use 'CAD'; say basis (Q121)`; removing it restored a clean `make boundary`.
  - Grep gate after C4 found zero remaining matches for `FrameChart`, `PlaceCadBaselineFork`, `ChartBaseline`, `assembly.CheckChartConsistency`, `DeltaPitchDeg(`, `measureUnpitchedFrameset`, `placeLeveledFrameset`, `validateForkGeometry`, or `.Chart` (only the `TestCheckChartConsistency…` test names kept per D5 matched the unanchored `CheckChartConsistency` substring).
  - `go tool deadcode ./...` reported only the two expected reboot entries: `FitOval.Validate` (`internal/reboot/assembly/scoring.go:107:18`) and `catalog.Forks` (`internal/reboot/catalog/forks.go:60:6`).
  - Live check against `http://127.0.0.1:8081`:
    - `/bike?fork=basis`: `Chart Basis Fork (495 mm / 54 mm / 0 mm cup): 68.26° HTA, 1157.7 mm WB (Published 1158.0 mm)`, `ΔPitch vs Basis: +0.00°`, Chart Basis Fork note visible (exact match to C0 snapshot). Screenshot: `/Users/matt/.gemini/antigravity/brain/dc6d5843-ee7c-4715-ad27-aec8943bc1dc/.system_generated/steps/11/media_0.png`.
    - `/bike`: `Chart Basis Fork (495 mm / 54 mm / 0 mm cup): 68.26° HTA, 1157.7 mm WB (Published 1158.0 mm)`, `ΔPitch vs Basis: +0.55°` (exact match to C0 snapshot).
    - `/bike?fork=cad`: confirmed HTTP `400` with banner `Error: frame "otso-fenrir-stainless" has no fork "cad"`.
- **Open issues:** None.

Implement this plan exactly. If the code disagrees with it, or a design choice comes up that the plan doesn't cover, stop. Record `Status: Blocked <date> on <question>` and the Implementation handoff, then tell Matt to switch to a Frontier model. "Seemed cleaner" is not a reason to deviate.

## Goal

Make the package split and its vocabulary coherent before the ingestion workflow (Slice 16b) builds on them. `assembly` keeps the physics. `catalog` owns the published chart data and the chart-consistency curation policy, which lives in catalog test code. The only seam between them is one exported placement pass. "CAD" gives way to **basis**: the front end a chart's numbers are computed on.

- **C1–C3 change no behavior.** The JSON CLI output and every rendered `/bike` and `/` page stay byte-identical, and every moved test keeps its expected values.
- **C4 is the rename.** Its only user-visible changes are the listed labels, ids, and the `fork=basis` URL value.

## Rulings this rests on

- **Q114:** no ingestion subcommand. The building blocks are an exported unpitched placement and, in catalog test code, the check with a result type that exposes each check's mismatch, rounding budget, per-column terms, allowance, and excess.
- **Q118:** the curation policy and its data (`ChartPrecision`, `ChartAllowance`, `PublishedWheelbaseMm`) move to `catalog`, and `assembly.FrameChart` is deleted.
- **Q120:** the vocabulary is chart/published, basis, installed, and unpitched/leveled. The basis is `catalog.Frame.BasisFork` (an `assembly.ForkSpec` with `SteererBasis`) plus `catalog.Frame.BasisLowerCup`, and `PlaceFrameset` places it. `PlaceCadBaselineFork` and `ChartBaseline` are deleted. `catalog.FrameSize` is `{Name, ChartFrame, PublishedWheelbaseMm, Allowance}`, and `ChartPrecision` sits on `catalog.Frame`.
- **Q121:** "basis" replaces "CAD" in every reboot identifier, comment, UI label, URL value, and active DESIGN.md rule, with no URL alias. The label is "Chart Basis Fork". `make boundary` bans `cad` in reboot files outside `*_parity_test.go`.
- **Unchanged:** Q91 (the check runs only in `go test`), Q44/Q81 (basis values are hypotheses), Q45/Q57/Q93 (budget, unpublished-wheelbase, and allowance rules, ported verbatim), Q67 (`InstalledFrame` uses max chainstay, and the basis readouts use the published chainstay), and Q117 (wording).

## Planner defaults (Matt may veto on review)

- **D1.** `PlaceUnpitchedFrameset` has the same parameters as `PlaceFrameset`, wheels included. Its tire contacts sit one wheel radius straight below each axle, so no field of the returned placement is a zero-value trap.
- **D2.** Two scalar readouts become `FramesetPlacement` methods: `LevelingPitchDeg()` (the rotation that would level the axles) and `PitchDeltaDeg(reference)`, which replaces `ChartBaseline.DeltaPitchDeg`.
- **D3.** `query.Bike.BasisFrameset()` is the one entry point for the basis readouts. This moves the placement call out of `web`, which only renders.
- **D4.** The check's identifiers are unexported in package `catalog_test`: `measureChartConsistency`, `chartConsistency`, `chartCheck`, `roundingTerm`, `chartConsistency.Err`, and the wrapper `checkChartConsistency`.
- **D5.** The moved tests run on the real catalog sizes (Fenrir L, Hayduke S4, Sklar L) instead of copied fixtures. A planning probe confirmed those sizes equal the assembly fixtures field by field and reproduce every expected string. Test names keep their `TestCheckChartConsistency…` form.
- **D6.** Four commits. C2 leaves `ChartPrecision` and `ChartAllowance` in `assembly` for one commit, so that each move is verified on its own. Identifiers that C1–C3 create or re-sign get their basis names right away; C4 renames everything older. So C1–C3 mix the two words for a few commits.
- **D7.** `ForkGeometry` and `ForkSetup.Geometry()` stay exported. They're still the recipe's input, and `web_test` uses them. `validateForkGeometry` is deleted, and an invalid basis fork now reports `ForkSetup.Validate`'s wording ("fork static A2C …"). No test asserts on the old wording.
- **D8.** `frameProblems` gains two rules: a basis fork must use the basis steerer sentinel, and it must have zero travel.
- **D9.** In C1–C3 the JSON CLI output is unchanged: `BikeResult` and `CompareResult` carry IDs, not catalog structs. In C4, `"cad"` becomes `"basis"` in the `fork=basis` output (Q49a: not a versioned schema).
- **D10.** The C4 user-visible strings, in title case like today's label:
  - "Chart Basis Fork" for the option group, preset link, notes, and readout line;
  - "ΔPitch vs Basis";
  - `fork=basis` in URLs;
  - ids and data attributes `preset-basis-fork` and `basis-fork`.
  Old `fork=cad` URLs get today's unknown-fork 400, as Q101 did with its renamed keys.
- **D11.** Comments where "CAD" meant a real drawing are reworded to keep their meaning without the word: the Seven custom's "CAD preview" becomes "design-drawing preview", and its "1:5 CAD drawing" becomes "1:5 design drawing". The ban leaves no exceptions.

## C0: baseline snapshot (no commit)

Before C1, capture the outputs that must not change:

1. Run `go run ./cmd/bikegeo-reboot bike`, `go run ./cmd/bikegeo-reboot bike 'fork=cad'`, and `go run ./cmd/bikegeo-reboot compare`. Save each to the scratchpad as `bike.json`, `bike_basis.json`, and `compare.json` under `before/`.
2. Write `internal/reboot/web/scratch_snapshot_test.go` (gitignored). It renders `/`, `/bike`, `/bike?fork=cad`, `/bike?frame=esker-hayduke&size=S4`, and `/bike?frame=ritchey-ultra&size=L&fork=cad` through `web.NewHandler().ServeHTTP` with `req.Host = "127.0.0.1:8080"`. It writes each body to a fixed filename in the directory named by the `SNAPSHOT_DIR` environment variable. Run it with `SNAPSHOT_DIR=<scratchpad>/before`.

After C1, C2, and C3, rerun both into `after/` and `diff -r before after`. Any difference is a blocker. C4 has its own comparison (see C4). Delete the scratch test at the end.

## C1: Place the basis frameset with PlaceFrameset

Subject: `Place the basis frameset with PlaceFrameset`

**A1. `internal/reboot/assembly/frameset.go`**

- Add the exported pass. Its doc says it builds the frameset in the chart's own orientation: BB at the origin, frame reference axis horizontal (0° pitch). The axles are level only when the fork and cup reproduce the chart's basis, and `LevelingPitchDeg` measures how far off they are. Tire contacts sit one wheel radius straight below each axle.
  ```go
  func PlaceUnpitchedFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error) {
  	if err := validateFramesetInputs(frame, lowerCup, fork, wheels); err != nil {
  		return FramesetPlacement{}, err
  	}
  	return placeFramesetAtPitch(0, frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
  }
  ```
- `PlaceFrameset` becomes:
  ```go
  unpitched, err := PlaceUnpitchedFrameset(frame, lowerCup, fork, wheels)
  if err != nil {
  	return FramesetPlacement{}, err
  }
  return placeFramesetAtPitch(unpitched.LevelingPitchDeg(), frame, lowerCup.StackMm, fork.Geometry(), wheels), nil
  ```
  Keep its doc's two-pass description, pointing at `PlaceUnpitchedFrameset` for pass 1.
- Merge `measureUnpitchedFrameset`, `placeLeveledFrameset`, and the old `placeFramesetAtPitch` into one `placeFramesetAtPitch(pitchDeg pose.Deg, frame FrameSetup, lowerCupMm float64, fork ForkGeometry, wheels WheelSetup) FramesetPlacement`:
  - root at `pose.NewPose(pose.Point{}, 0).Turn(pitchDeg)`;
  - run the existing walk unchanged;
  - set both tire contacts as `placeLeveledFrameset` does today (keep its comment).
- Add methods next to `PitchDeg`:
  ```go
  // LevelingPitchDeg returns the counter-clockwise rotation about BB that
  // would put the axles level: nonzero on an unpitched placement whose fork
  // and cup differ from the chart's basis, and 0 (to rounding) on a
  // placement PlaceFrameset returned.
  func (f FramesetPlacement) LevelingPitchDeg() pose.Deg { return -f.RearAxle.AngleTo(f.FrontAxle) }

  // PitchDeltaDeg returns f's leveling pitch minus reference's: positive when
  // f's front end sits higher, as an installed fork does against the basis fork.
  func (f FramesetPlacement) PitchDeltaDeg(reference FramesetPlacement) pose.Deg { return f.PitchDeg() - reference.PitchDeg() }
  ```
- `FramesetPlacement`'s doc currently says the placement is leveled. Reword it to say `PlaceFrameset` returns it leveled (horizontal axle line, contacts on one ground line) and `PlaceUnpitchedFrameset` returns the same landmarks at 0° pitch. The landmark-axis bullets are unchanged.
- Delete `ChartBaseline`, its `DeltaPitchDeg`, and `PlaceCadBaselineFork`.
- `chartMismatches` in `chart_consistency.go` now calls `placeFramesetAtPitch(0, chart.Frame, chart.CadLowerCup.StackMm, chart.CadFork, WheelSetup{})`. Add a comment that the tire contacts are unused. C2 deletes it.

**A2. `internal/reboot/query/bike.go`:** add

```go
// BasisFrameset places b's size on its published chart frame with the chart's
// basis fork and lower cup (Q81, Q121), for the basis readouts and ΔPitch.
func (b Bike) BasisFrameset() (assembly.FramesetPlacement, error) {
	return assembly.PlaceFrameset(b.Size.Chart.Frame, b.Size.Chart.CadLowerCup, cadForkSetup(b.Size), b.Setup.Wheels)
}
```

**A3. `internal/reboot/web`**

- `bike_page.go`: replace `assembly.PlaceCadBaselineFork(parsed.Size.Chart, parsed.Setup.Wheels)` with `parsed.BasisFrameset()`, and rename the local `cadBaselineFork` to `basis`.
- `web.go`: `buildBikeView`'s third parameter becomes `basis assembly.FramesetPlacement`. The three readouts become `basis.LeveledHeadTubeAngleDeg()`, `basis.WheelbaseMm()`, and `placed.PitchDeltaDeg(basis)`. The view field names change in C4.

**T1. `internal/reboot/assembly/frameset_test.go`:** replace `TestPlaceCadBaselineFork` with `TestPlaceUnpitchedFrameset`. Use `fenrirSizeLFrame()`, `fenrirCadCup()`, `basisFork := ForkSetup{Spec: ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: SteererCAD}}`, and `fenrirWheels()` (372 mm). Expected values come from the planning probe (catalog Fenrir L, which equals this fixture):

| Assertion | Expected | Tolerance |
| --- | --- | --- |
| `unpitched.PitchDeg()` | 0 | exact |
| `unpitched.LevelingPitchDeg()` | −0.010692172° | 1e-9 |
| `leveled := PlaceFrameset(same)`; `leveled.PitchDeg() − unpitched.LevelingPitchDeg()` | 0 | 1e-12 |
| `leveled.LevelingPitchDeg()` | 0 | 1e-12 |
| `unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle)` (axle rise) | +0.22 mm | 0.005 |
| `unpitched.FrontAxle.Origin().X − unpitched.RearAxle.Origin().X − 1158` | −0.34 mm | 0.005 |
| Each unpitched tire contact vs. its axle: same X, Y lower by 372 | — | 1e-9 |
| `BB.DistanceTo(FrontAxle)` unpitched vs. leveled (a rotation about BB) | equal | 1e-9 |
| `leveled.FrontAxle.HeadingDeg() − unpitched.FrontAxle.HeadingDeg()` | −0.010692172° | 1e-9 |
| Installed fox34: `{530, 120, 44, Tapered}` at 20% sag on `fenrirEc44Cup()`, then `installed.PitchDeltaDeg(leveled)` | +1.241315762° | 1e-9 |
| `leveled.PitchDeltaDeg(leveled)` | 0 | exact |

Negative cases are ported from the old test. Each must make **both** `PlaceUnpitchedFrameset` and `PlaceFrameset` return an error:

- basis fork A2C 0
- rake NaN
- `LowerCupSetup{}`
- bare head tube with a 12 mm stack
- EC44 with a 0 mm stack

In `TestMismatchedPhysicalCupFailsBothChartEntryPoints`, the `PlaceCadBaselineFork` assertion becomes `PlaceUnpitchedFrameset(chart.Frame, chart.CadLowerCup, basisFork, fenrirWheels())`, still expecting `InputLowerCup`. The `CheckChartConsistency` half stays until C2.

**T2. `internal/reboot/web/web_test.go`:** four `PlaceCadBaselineFork` sites, around lines 214, 587, 990, and 1414.

- Each becomes `<bike>.BasisFrameset()`, using the `parsed` or `decoded` value already in scope. At line 587, `parsed.Setup.Wheels` is already `customWheels`.
- `cadBaselineFork.DeltaPitchDeg(placed)` becomes `placed.PitchDeltaDeg(basis)`.
- The `fork=cad` test (around line 998) asserts `placed.PitchDeltaDeg(basis)` within 1e-12 of 0.

**T3. `internal/reboot/catalog/frames_test.go`:** in `TestForkListsPlace`, replace `PlaceCadBaselineFork(s.Chart, wheels)` with `assembly.PlaceFrameset(s.Chart.Frame, s.Chart.CadLowerCup, basisForkOf(s.Chart), wheels)`. Add the test helper `basisForkOf(chart assembly.FrameChart) assembly.ForkSetup`, which returns `{Spec: {StaticA2cMm: chart.CadFork.A2cMm, RakeMm: chart.CadFork.RakeMm, Steerer: assembly.SteererCAD}}`; C3 deletes it. Rename the test's local `baselines` counter to `bases`. The counts stay 701 and 119.

**T4. `internal/reboot/query/bike_test.go`:** add `TestBasisFrameset`. `DefaultBike().BasisFrameset()` has no error, and its `PitchDeg()` is −0.010692172° within 1e-9 (the default is Fenrir L at 372 mm).

## C2: Move the chart-consistency check to catalog tests

Subject: `Move the chart-consistency check to catalog tests`

**A4. `internal/reboot/assembly`**

- Delete `chart_consistency.go` and `chart_consistency_test.go`.
- Move `ChartPrecision` and `ChartAllowance` into `frameset.go` beside `FrameChart`. Reword their docs so they don't name a function: "catalog's chart-consistency test reads these". `ChartAllowance` keeps its rule text (necessity, 0.5 mm slack, required reason) and attributes it to that test.
- Delete `validateForkGeometry` and any imports left unused.
- In `frameset_test.go`, rename `TestMismatchedPhysicalCupFailsBothChartEntryPoints` to `TestMismatchedBasisCupFailsUnpitchedPlacement` and drop its `CheckChartConsistency` assertions; T6 covers them.

**A5. New `internal/reboot/catalog/chart_consistency_test.go`** (package `catalog_test`). Port the logic verbatim from the deleted file. Keep every error string and the validation order identical.

```go
// chartAllowanceSlackMm and consistencyEpsilonMm keep their values (0.5, 1e-9) and comments.

// roundingTerm is one published column's contribution to a check's rounding budget.
type roundingTerm struct {
	Column string  // "Reach", "Stack", "HTA", "HTL", "BB drop", "Chainstay", or "Wheelbase"
	Mm     float64
}

// chartCheck is one chart-consistency check of one size (Q45, Q57, Q93).
type chartCheck struct {
	Name        string         // "axle height" or "wheelbase"
	MismatchMm  float64        // axle height: front axle rise above the rear axle; wheelbase: axle span minus the published wheelbase
	Terms       []roundingTerm // columns whose term exceeds consistencyEpsilonMm, in chart column order
	BudgetMm    float64        // sum of Terms
	AllowanceMm float64        // the size's documented allowance for this check (0 = none)
	ExcessMm    float64        // |MismatchMm| − BudgetMm; ≤ 0 when rounding explains the mismatch
}

// chartConsistency is one size's measurement: the axle height check, then the
// wheelbase check unless the wheelbase is unpublished (Q57).
type chartConsistency struct{ Checks []chartCheck }

// measureChartConsistency places the chart's unpitched basis frameset and each
// ±half-width column nudge, and returns every check. Its error reports input
// the check cannot measure.
func measureChartConsistency(chart assembly.FrameChart) (chartConsistency, error)

// Err applies the pass/fail and allowance rules to the measured checks, in
// order, joined, using today's messages verbatim.
func (c chartConsistency) Err() error

// checkChartConsistency returns measureChartConsistency's error, else Err().
func checkChartConsistency(chart assembly.FrameChart) error
```

The order inside `measureChartConsistency` is:

1. `assembly.PlaceUnpitchedFrameset(chart.Frame, chart.CadLowerCup, basisForkOf(chart), wheels)`; return its error unwrapped.
2. Published wheelbase finite and non-negative.
3. Allowance values finite and non-negative.
4. Reason present if and only if an allowance is set.
5. No wheelbase allowance when the wheelbase is unpublished.
6. Precision half-widths finite and non-negative.
7. For each budget column with a nonzero half-width, place the up and down nudges with `PlaceUnpitchedFrameset`, wrapping any error as `chart precision: %s ±%v moves the chart out of range: %w`.

The mismatches come from an unexported helper `chartMismatches(unpitched assembly.FramesetPlacement, publishedWheelbaseMm float64) [2]float64`, which returns `unpitched.RearAxle.VerticalOffsetTo(unpitched.FrontAxle)` and `unpitched.FrontAxle.Origin().X − unpitched.RearAxle.Origin().X − publishedWheelbaseMm`. Copy those expressions exactly. The column table's `nudge` takes `*assembly.FrameChart`, as today.

**T5. Move every test from the deleted file** into `chart_consistency_test.go` with the same names, cases, and expected strings. Only the chart source and the call change:

- `fenrirChart()` → `mustSize(t, mustFrame(t, "otso-fenrir-stainless"), "L").Chart`
- `haydukeS4Chart()` → the `esker-hayduke` S4 chart
- `sklarLChart()` → the `sklar-pbj` L chart
- `legacyChartPrecision` is deleted, because the catalog sizes carry it
- `assembly.CheckChartConsistency` and `requireChartConsistency` → `checkChartConsistency` and a local `requireChartConsistency(t, chart, want)`

Each table case overwrites `Allowance`, so the catalog's own Hayduke and Sklar allowances don't leak in. The moved tests are: `FenrirPasses`, `BudgetEdges` (4 cases), `WithoutPrecision`, `InvalidPrecision` (2), `NudgeOutOfRange`, `InvalidWheelbase`, `HaydukeS4Allowance` (5), `SklarLAllowance` (4), `UnnecessaryAllowance`, `InvalidAllowance` (5), and `UnpublishedWheelbase` (3).

**T6. New tests in the same file**

- `TestCheckChartConsistencyRejectsMismatchedBasisCup`: Fenrir L with `CadLowerCup = {IS52, 0}` makes `checkChartConsistency` return an error where `errors.As` gives an `*assembly.InputError` with `Field == assembly.InputLowerCup`. With `{EC44, 12}`, any error it returns is not an `InputLowerCup` error.
- `TestMeasureChartConsistencyExposesChecks` (Q114):
  - Hayduke S4 as cataloged: `len(Checks) == 2`. `Checks[1]` is `"wheelbase"` with `MismatchMm` +5.42, `BudgetMm` 4.31, `ExcessMm` 1.11 (each ±0.005) and `AllowanceMm` exactly 1.2. Its `Terms` columns are `[Reach HTA HTL "BB drop" Chainstay Wheelbase]` with mm `[0.50 2.54 0.19 0.07 0.51 0.50]` (each ±0.005), and `Err() == nil`.
  - Fenrir L with `PublishedWheelbaseMm = 0`: `len(Checks) == 1`, with `Name` `"axle height"`, `MismatchMm` +0.22, and `BudgetMm` 2.09 (±0.005). `Terms` are `[Stack HTA HTL "BB drop"]` with `[0.50 0.62 0.46 0.50]` (±0.005), `AllowanceMm` is 0, and `Err() == nil`.

**T7. `frames_test.go`:** `TestEveryChartIsConsistent` and `TestChartAllowances` call `checkChartConsistency`. Their expected prefixes are unchanged.

**A6. `AGENTS.md` "One-off investigations":** replace the interim bullet with: "Interim, until Slice 16b rewrites the ingest-frame skill: `measureChartConsistency` in `internal/reboot/catalog/chart_consistency_test.go` exposes each check's mismatch, rounding budget, per-column terms, allowance, and excess; call it from a temporary test in the same package."

## C3: Move chart data to catalog

Subject: `Move chart data to catalog and delete FrameChart`

**A7. `internal/reboot/assembly/frameset.go`:** delete `FrameChart`, `ChartPrecision`, and `ChartAllowance`. `ForkGeometry`'s doc drops the `FrameChart.CadFork` mention and keeps "used by the frameset Pose recipe".

**A8. `internal/reboot/catalog/frame.go`**

```go
type Frame struct {
	ID, Manufacturer, Model, Year, DefaultSize, Sizes, Forks, MaxChainstayMm // unchanged, same order
	// BasisLowerCup and BasisFork are the chart's basis: the lower cup and
	// fork its numbers are computed on (Q44, Q81, Q121), whether stated by
	// the manufacturer or inferred from the chart; not physical parts.
	// BasisFork uses assembly.SteererCAD and has no travel; its StaticA2cMm
	// is the basis axle-to-crown, sag included.
	BasisLowerCup assembly.LowerCupSetup
	BasisFork     assembly.ForkSpec
	// ChartPrecision is the rounding half-width of each published column (Q43, Q80).
	ChartPrecision ChartPrecision
}

// FrameSize is one size's published chart.
type FrameSize struct {
	Name string
	// ChartFrame is the bare frame at its published dimensions (published
	// chainstay); Frame.InstalledFrame gives the frame / and /bike build (Q67).
	ChartFrame           assembly.FrameSetup
	PublishedWheelbaseMm float64        // 0 = unpublished; skips the wheelbase check (Q57)
	Allowance            ChartAllowance // documented widening beyond rounding (Q65, Q71, Q93)
}
```

- `ChartPrecision` and `ChartAllowance` move here unchanged in their fields. Their docs refer to "the chart-consistency test" and keep the Q43/Q44/Q65/Q71/Q93 text; any "CAD" in them becomes "basis".
- `frameFamily` keeps only `LowerBore` and `DefaultLowerCup`, with its doc trimmed to "headset values every size shares".
- `sizes()` builds the new `FrameSize`, and `InstalledFrame` reads `size.ChartFrame`.

**A9. The 19 `frame_*.go` files.** In each one:

1. Cut the `CadLowerCup:`, `CadFork:`, and `Precision:` entries, with the comment line above each, from the `family := frameFamily{…}` literal.
2. Paste them into the returned `Frame{…}` literal right after `MaxChainstayMm` (or after `DefaultSize` where there is no `MaxChainstayMm`), before `Forks`.
3. Rename `CadLowerCup:` to `BasisLowerCup:`, `Precision:` to `ChartPrecision:`, and `assembly.ChartPrecision{` to `ChartPrecision{`.
4. Rewrite `CadFork: assembly.ForkGeometry{A2cMm: X, RakeMm: Y}` as `BasisFork: assembly.ForkSpec{StaticA2cMm: X, RakeMm: Y, Steerer: assembly.SteererCAD}`, keeping X and Y character for character.

In `frame_esker_hayduke.go` and `frame_sklar_pbj.go`, `assembly.ChartAllowance{` becomes `ChartAllowance{`. No number, reason, or other comment text changes in this commit; C4 rewords the "CAD" prose. The parity tests (T9) prove the values.

**A10. `internal/reboot/query/bike.go`**

- Replace `cadForkSetup(size)` with `basisForkSetup(frame catalog.Frame) assembly.ForkSetup`, which returns `{Spec: frame.BasisFork}`.
- The `isCad` branch returns `size.ChartFrame, frame.BasisLowerCup, basisForkSetup(frame), nil`.
- `prevCupStd = frame.BasisLowerCup.Standard`.
- `WithCadBaselineFork` uses `b.Size.ChartFrame`, `b.Frame.BasisLowerCup`, and `basisForkSetup(b.Frame)`.
- `BasisFrameset` uses `b.Size.ChartFrame`, `b.Frame.BasisLowerCup`, and `basisForkSetup(b.Frame)`.

**A11. `internal/reboot/web`**

- `makeCadForkOption(size)` becomes `makeBasisForkOption(frame catalog.Frame)` (update its caller). It labels with `frame.BasisFork.StaticA2cMm`, `frame.BasisFork.RakeMm`, and `frame.BasisLowerCup.StackMm`.
- The view builder sets `BasisFork: b.Frame.BasisFork`, `CadLowerCupMm: b.Frame.BasisLowerCup.StackMm`, and `PublishedWheelbaseMm: b.Size.PublishedWheelbaseMm`.
- `cadBaselineForkNote` reads `b.Frame.BasisLowerCup`, and `slidingDropoutsNote` reads `b.Size.ChartFrame.ChainstayMm`.
- `web.go`: the view field `CadFork assembly.ForkGeometry` becomes `BasisFork assembly.ForkSpec`.
- `templates/bike.html`, around line 619: `.CadFork.A2cMm` becomes `.BasisFork.StaticA2cMm` and `.CadFork.RakeMm` becomes `.BasisFork.RakeMm`. The rendered text is unchanged because a basis fork has no travel.

**T8. Test updates.** The compiler finds every site.

- In `catalog/chart_consistency_test.go`, `measureChartConsistency(f catalog.Frame, s catalog.FrameSize)` and `checkChartConsistency(f, s)` replace the chart parameter. The basis comes from `f` and the precision from `f.ChartPrecision`. `nudge` takes `*catalog.FrameSize`: the six frame columns edit `s.ChartFrame`, and Wheelbase edits `s.PublishedWheelbaseMm`. `basisForkOf` is deleted, so place with `assembly.ForkSetup{Spec: f.BasisFork}`. Test mutations move to the matching field (`c.Frame.StackMm` → `s.ChartFrame.StackMm`, `c.Precision` → `f.ChartPrecision`, `c.Allowance` → `s.Allowance`, `c.CadLowerCup` → `f.BasisLowerCup`). Every expected value stays the same.
- `catalog/frames_test.go`:
  - `TestForkListsPlace` places with `s.ChartFrame`, `f.BasisLowerCup`, and `assembly.ForkSetup{Spec: f.BasisFork}`.
  - `TestChartAllowances` uses `s.Allowance` and `catalog.ChartAllowance`.
  - `frameProblems` reads `s.ChartFrame.ChainstayMm`.
  - `TestInstalledFrameUsesMaxChainstay` uses `s.ChartFrame`.
- `query/compare_test.go` lines 92–93 use `sz.ChartFrame`.
- `web/web_test.go`:
  - Replace `chart := …Size.Chart` with direct reads: `decoded.Size.ChartFrame`, `decoded.Frame.BasisLowerCup`, and `parsed.Frame.BasisFork.StaticA2cMm`.
  - `decoded.Setup.Fork.Geometry() != chart.CadFork` becomes `decoded.Setup.Fork.Spec != decoded.Frame.BasisFork`.

**T9. `catalog/frames_parity_test.go`**

- In `TestFramesMatchLegacy`, check `f.BasisLowerCup` and `f.BasisFork` once per frame, outside the size loop. The expected basis fork is `assembly.ForkSpec{StaticA2cMm: l.CadFork.A2cMm, RakeMm: l.CadFork.RakeMm, Steerer: assembly.SteererCAD}`, with the ritchey 508.5 override kept. `l.CadFork` is legacy's field and keeps its name.
- The size columns read `s.ChartFrame` and `s.PublishedWheelbaseMm`, and the allowance check uses `s.Allowance != (catalog.ChartAllowance{})`.
- In `TestPrecisionMatchesLegacyInference`, `want` is a `catalog.ChartPrecision`, compared once per frame against `f.ChartPrecision`.
- Do not relax `legacyFramesByID`; that change is Q119's, in Slice 16b.

**T10. D8 rules in `frameProblems`.** Write the messages with `%q` of the constant, so C4's rename needs no edit here.

- `f.BasisFork.Steerer != assembly.SteererCAD` → `fmt.Sprintf("basis fork steerer is %q, want %q", f.BasisFork.Steerer, assembly.SteererCAD)`
- `f.BasisFork.TravelMm != 0` → `fmt.Sprintf("basis fork has %v mm travel, want 0", f.BasisFork.TravelMm)`

Add both as `TestFrameProblemsCatchMistakes` cases: `BasisFork.Steerer = assembly.SteererTapered` should give exactly one problem containing `"basis fork steerer"`, and `BasisFork.TravelMm = 120` exactly one containing `"basis fork has"`.

## C4: Rename CAD to basis

Subject: `Rename CAD to basis`

Apply Q121 to every remaining use in `cmd/bikegeo-reboot` and `internal/reboot` (`*_parity_test.go` keeps legacy's own `CadFork` field). The compiler and the new `make boundary` rule find every site.

**R1. Exported and package-level identifiers**

| Old | New |
| --- | --- |
| `assembly.SteererCAD` (value `"cad"`) | `assembly.SteererBasis` (value `"basis"`) |
| `query.CadForkID` (value `"cad"`) | `query.BasisForkID` (value `"basis"`) |
| `query.Bike.WithCadBaselineFork` | `WithBasisFork` |
| web view fields `CadForkOption`, `CadBaselineNote`, `CadBaselineForkURL`, `CadLowerCupMm`, `CadHtaDeg`, `CadWheelbaseMm` | `BasisForkOption`, `BasisForkNote`, `BasisForkURL`, `BasisLowerCupMm`, `BasisHtaDeg`, `BasisWheelbaseMm` |
| web `cadForkName` (`"CAD Baseline Fork"`), `cadBaselineForkNote` | `basisForkName` (`"Chart Basis Fork"`), `basisForkNote` |

**R2. Local and test identifiers.** Rename by this rule: `CADBaselineFork` and `CadBaselineFork` become `BasisFork`; `cadBaselineFork` and `cadBaseline` become `basis`; otherwise `Cad`/`CAD` become `Basis` and `cad` becomes `basis` inside an identifier. Examples:

- `isCad` → `isBasis`, `cadDoc` → `basisDoc`, `customCadHref` → `customBasisHref`
- `fenrirCadCup` → `fenrirBasisCup`
- `TestPlaceFramesetCADBaselineForkAndInvariants` → `TestPlaceFramesetBasisForkAndInvariants`
- `TestBikeCadBaselineForkURLDecodesToChartFork` → `TestBikeBasisForkURLDecodesToChartFork`

**R3. User-visible strings (D10)**

- `templates/bike.html`:
  - `<optgroup label="Chart Basis Fork">`;
  - preset `id="preset-basis-fork"`, `data-preset="basis-fork"`, link text "Chart Basis Fork (… mm cup)";
  - `data-note="basis-fork"`;
  - readout "Chart Basis Fork (… mm / … mm / … mm cup)";
  - "ΔPitch vs Basis".
- `bike_page.go` notes: the "CAD Baseline Fork:" prefixes and "CAD Baseline Fork uses published chainstay" become "Chart Basis Fork". The rest of each sentence is unchanged.
- `assembly` error: `CAD baseline fork cannot have travel` → `basis fork cannot have travel`.
- `compare` error: `is the CAD baseline fork; compare installs physical forks only` → `is the chart basis fork; compare installs physical forks only`.
- `cmd/bikegeo-reboot/main.go` usage: `fork ID ("basis" selects the chart basis fork)`.
- Update test assertions on these strings, selectors, and `fork=cad` URLs to the new forms.

**R4. Comments.** Every "CAD" in reboot comments becomes "basis", for example "Its CAD fork is …" → "Its basis fork is …" and "EC44 CAD cup" → "EC44 basis cup". Where "CAD" named a real drawing, follow D11. In `frame_otso_fenrir_stainless.go`, "whether the CAD chart omitted it" → "whether the chart omitted it". Numbers and provenance tags such as "hypothesis (Q44, Q81)" stay.

**R5. `Makefile` `boundary`.** Add a rule shaped like the `chassis` one:

- Fail when any file under `cmd/bikegeo-reboot` or `internal/reboot`, other than a `*_parity_test.go` file, has `cad` in its name or contents (case-insensitive, inside longer words too).
- The message is: `boundary: reboot files must not use 'CAD'; say basis (Q121)`.

A planning grep found no other English word containing `cad` in these trees. **Negative check:** temporarily add `// CAD` to any reboot file, confirm `make boundary` fails with that message, then remove it.

**A12. `docs/reboot/DESIGN.md` active rules.** Leave the Q-rulings and Milestones history alone.

- Requirement (c): "validated by `assembly.CheckChartConsistency` in `go test`" becomes "validated by the chart-consistency check in `catalog`'s `go test` (Q91, Q118)".
- In **Frameset leveling**, name `PlaceUnpitchedFrameset` as pass 1. Write step 1 as: "measure `LevelingPitchDeg()`; on the basis fork and cup, the same pass gives the chart-consistency mismatches that catalog's `go test` holds to the chart's rounding budget (Q91, Q118)". Name `PlaceFrameset` as pass 2.
- Coordinate table row "CAD chart vs. installed fork" → "Chart basis vs. installed fork": `catalog.Frame.BasisFork` (a `ForkSpec` with `SteererBasis`) and `BasisLowerCup` hold the chart's basis (Q120, Q121), keeping bare `FrameSetup` free of chart quirks.
- Section **CAD Baseline Fork in the workbench** → **Chart Basis Fork in the workbench**:
  - "`fork=basis` resolves A2C, cup, and sag (0%) from the frame's `BasisFork` and `BasisLowerCup` and the size's `ChartFrame`";
  - "Every visible label says 'Chart Basis Fork'";
  - "reproduces the basis 0 mm + 495 mm".

**C4 snapshot check.** Regenerate the C0 outputs with `fork=basis` in place of `fork=cad`, into `after/`, under the same filenames. Then apply these substitutions to a copy of `before/`:

- `CAD Baseline Fork` → `Chart Basis Fork`
- `ΔPitch vs CAD` → `ΔPitch vs Basis`
- `cad-baseline-fork` → `basis-fork`
- `fork=cad` → `fork=basis`
- `"cad"` → `"basis"`

`diff -r` against `after/` must be empty. Record any substitution you had to add, with the reason, as a deviation.

## Rules that bite

- Published values never change. Moves are cut and paste, and parity tests T9 are the proof.
- Error strings in the moved check are copied character for character. A changed expected string is a blocker. C4's renamed strings in R3 are the only exceptions.
- Trig stays in `pose`. Catalog test code uses `VerticalOffsetTo`, `Origin()`, and the existing exported methods, with no `math` trig.
- `assembly` never imports `catalog`, and `make boundary` must pass. No banned scoring words (the check says "budget", never "cost").
- `exhaustive` must pass: the `Steerer` switches pick up `SteererBasis` through the rename.
- Every commit passes `make pre-commit` on its own, and the snapshot comparison for that commit is empty.
- Don't touch the ingest-frame skill, `legacyFramesByID`, `docs/RIDER_FIT.md`, or anything else in Slice 16b's scope or outside the reboot tree.
- Finish with the Implementation handoff (DESIGN.md "Implementation handoff"): set this file's Status line, add the handoff section below it, and set the Slice 16a Milestones line in DESIGN.md to the same status words.

## Verification

1. Run `make pre-commit` after each commit.
2. Compare snapshots after each commit: byte-identical after C1–C3, and empty after the substitutions after C4.
3. Run `go test -run 'ChartConsistency' -v ./internal/reboot/catalog`: every T5 test name plus the T6 tests is listed and passes. `go test ./internal/reboot/assembly` no longer lists any `TestCheckChartConsistency…`.
4. Grep gate after C4: `grep -rnE 'FrameChart|PlaceCadBaselineFork|ChartBaseline|CheckChartConsistency|DeltaPitchDeg\(|measureUnpitchedFrameset|placeLeveledFrameset|validateForkGeometry|\.Chart\b' cmd/bikegeo-reboot internal/reboot` prints nothing, and `make boundary` passes, which proves no `cad` remains.
5. Run `go tool deadcode ./...` and record the reboot entries. The ones expected to remain are `FitOval.Validate` and `catalog.Forks` (Slice 17).
6. Live check: start the `reboot` preview. Open `/bike?fork=basis` and `/bike`. Confirm the "Chart Basis Fork (…)" readout line and "ΔPitch vs Basis" show the same numbers as the C0 snapshot, and that `/bike?fork=cad` returns the unknown-fork 400. Record the numbers and a screenshot path in the handoff.

## Out of scope

- The parity relaxation (Q119), the ingest-frame skill rewrite, the AGENTS.md final wording, and the re-ingestion proof (Slice 16b).
- Unexporting `ForkGeometry`, other `deadcode` work (Slice 17), and any other `/bike` UX change.
- Performance: `PlaceFrameset` still runs two passes, and validation runs once.
