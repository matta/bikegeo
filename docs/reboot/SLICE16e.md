# Slice 16e: The Fenrir Correction and Promoted Basis Search

Status: Planned 2026-11-22 (Q126–Q130, Q140–Q143; ready for implementation)

## Goal

1. Promote the basis-finding test template out of [.agents/skills/ingest-frame/SKILL.md](../../.agents/skills/ingest-frame/SKILL.md) into committed catalog test code (`internal/reboot/catalog/basis_search_test.go`) so API changes can never silently break it (`Q126`), fixing the old template bug where `f.BasisFork.RakeMm` bled into the implied-A2C pass, and update `SKILL.md` to reference it plus `Q128` fork-list guidance.
2. Re-ingest `otso-fenrir-stainless` from Otso's published S1 chart (`Q126`, `Q127`, `Q128`, `Q140`–`Q143`), resolve every open question in `internal/reboot/catalog/frame_otso_fenrir_stainless.md`, skip Fenrir in the legacy parity tests (`Q126`), and update all pinned expectations across `catalog`, `query`, `cmd/bikegeo-reboot`, and `web`.

No production runtime packages outside `internal/reboot/catalog/frame_otso_fenrir_stainless.go` change, and no other catalog frame or legacy `internal/frames` file changes.

## Confirmed Rulings Governing This Slice

- **Q126 (`DESIGN.md`):** re-ingest `otso-fenrir-stainless` from Otso's chart (`ea9d423` derived `68.25°/75.25°/72/450` and `495/54` basis), and promote the skill's basis-finding template into a catalog test helper with a committed test, pointing the skill at that test.
- **Q127 (`DESIGN.md`):** update `internal/reboot/catalog/frame_otso_fenrir_stainless.md` so every catalog value, provenance label, dated chart-consistency readout, and resolved open question matches the Go catalog file.
- **Q128 & Q133 (`DESIGN.md`):** Fenrir's listed forks are `[lithicCarbonMountain, enveMountainBoost, sid100, fox34_120]` (`enve_mountain_boost` moves from `Other Compatible Forks` into `Listed Forks`, keeping `lithic_carbon_mountain-495x44` first/default).
- **Q136 & Q139 (`DESIGN.md`):** keep Cane Creek 40 typical `EC44 12 mm` `DefaultLowerCup` (with Wolf Tooth's `12.25 mm` documented in the source record), and keep `<optgroup label="Listed Forks">` / `<optgroup label="Other Compatible Forks">` grouping.
- **Q140 (`DESIGN.md`):** transcribe S1's nominal middle Tuning Chip geometry (`HeadTubeAngleDeg: 68`, `SeatTubeAngleDeg: 75`, `BbDropMm: 70`, `ChainstayMm: 440`) and set `MaxChainstayMm: 0` (removing the sliding-dropout `*` flag on the Fenrir).
- **Q141 (`DESIGN.md`):** set `PublishedWheelbaseMm: 0` on all five Fenrir sizes (printed 10-mm-rounded ranges stay in `frame_otso_fenrir_stainless.md`).
- **Q142 (`DESIGN.md`):** set `ChartPrecision.HeadTubeAngleDeg: 0.5` (from S1's integer `68°`), `BasisLowerCup: LowerCupBareHeadTube (0 mm)`, and `BasisFork: {StaticA2cMm: 495, RakeMm: 51, Steerer: SteererBasis}`.
- **Q143 (`DESIGN.md`):** set `MaxPostInsertionMm` across all five sizes from S1/S3: `XS 170`, `S 210`, `M 260`, `L 290`, `XL 330 mm`.

---

## Planner Defaults (`D1`–`D6`)

- **D1. Promoted basis search helper (`internal/reboot/catalog/basis_search_test.go`, package `catalog_test`):**
  Define exact types and helpers (zero `math` trig — only `math.Inf`, `math.IsNaN`, `math.Round`, `min`, `max`):
  - `type basisRange struct { From, To float64 }` (`NaN` when no value in the scanned window passes).
  - `type basisEval struct { A2cMm, RakeMm float64; Passes bool; AxleHeightExcess float64; HasWheelbase bool; WheelbaseExcess float64 }` (uses `HasWheelbase` instead of printing `-Inf` when every size has `PublishedWheelbaseMm == 0`).
  - `type sizeImpliedA2c struct { SizeName string; ImpliedA2cMm, AxleHeightBudgetMm float64 }`.
  - `type basisSearchResult struct { Sizes []sizeImpliedA2c; ImpliedMinMm, ImpliedMaxMm, MinimaxA2cMm, RoundedA2cMm float64; A2cRange, RakeRange basisRange; RakeComparisons []basisEval }`.
  - `func evalBasis(f catalog.Frame, cup assembly.LowerCupSetup, a2c, rake float64) (basisEval, error)`: checks `len(f.Sizes) > 0`, copies `f`, sets `BasisLowerCup = cup` and `BasisFork = assembly.ForkSpec{StaticA2cMm: a2c, RakeMm: rake, Steerer: assembly.SteererBasis}`, evaluates `measureChartConsistency` across all sizes.
  - `func searchBasis(f catalog.Frame, cup assembly.LowerCupSetup, rake float64, compareRakes []float64) (basisSearchResult, error)`: checks `len(f.Sizes) > 0`, sets `base.BasisLowerCup = cup` and `base.BasisFork = assembly.ForkSpec{StaticA2cMm: 480, RakeMm: rake, Steerer: assembly.SteererBasis}` (fixing the old `SKILL.md` bug where `f.BasisFork.RakeMm` stayed untouched during the implied-A2C pass), computes per-size implied A2C, minimax `(lowest+highest)/2`, rounded `a2c` (nearest `0.5 mm`), scans `a2c ± 10` (`step = 0.05`) and `rake ± 15` (`step = 0.05`) using `for v := from; v <= to+step/2; v += step`, and runs `evalBasis` for each `r` in `compareRakes`.
  - `func logBasisSearch(t *testing.T, label string, res basisSearchResult)`: logs per-size implied A2C and budget, implied range, minimax, rounded A2C, A2C/rake passing ranges, and `RakeComparisons` (printing `"unpublished"` when `!HasWheelbase`).
- **D2. Fenrir listed forks (`Q128`):** `Forks: []Fork{lithicCarbonMountain, enveMountainBoost, sid100, fox34_120}`. Keeps `lithic_carbon_mountain-495x44` first (so `/bike` default fork ID and `CompatibleForks()` count stay unchanged, `2,989` total fork placements across 19 frames) while moving `enve_mountain_boost` into `Listed Forks`.
- **D3. Fenrir lower cup & year:** keep `DefaultLowerCup` at Cane Creek 40 typical `EC44 12 mm` (required by `TestFrameStructure`, `Q136`), keeping Wolf Tooth's `12.25 mm` documented in `frame_otso_fenrir_stainless.md`. Keep `Year: 2024` (source record notes the chart dates to 2022).
- **D4. Legacy parity skip (`internal/reboot/catalog/frames_parity_test.go`):** skip `otso-fenrir-stainless` in `TestFramesMatchLegacy` (dropping now-dead `modelOverride` / `wantInsertion` special cases for Fenrir) and `TestPrecisionMatchesLegacyInference` citing `Q126`, while keeping `otso-fenrir-stainless` in `TestMissingLegacyFrames` so all 19 legacy IDs remain checked as present in `catalog.Frames()`.
- **D5. `chart_consistency_test.go` fixtures:** switch the tests that require a frame with a published wheelbase (`TestCheckChartConsistencyBudgetEdges`, `TestCheckChartConsistencyWithoutPrecision`, `TestCheckChartConsistencyInvalidAllowance`) from `fenrirL(t)` to `krampusL(t)` (`surly-krampus` size `L`); update `TestCheckChartConsistencyNudgeOutOfRange` on `fenrirL` (`ChainstayMm: 70.4` since `BbDropMm` is now `70`); and update `TestCheckChartConsistencyUnnecessaryAllowance`, `TestCheckChartConsistencyUnpublishedWheelbase`, and `TestMeasureChartConsistencyExposesChecks` for corrected Fenrir L (whose `PublishedWheelbaseMm` is already `0`).
- **D6. `frames_test.go` updates:** update `TestFrameProblemsCatchMistakes` (`MaxChainstayMm = 430` below `440` on Fenrir, and replace the hardcoded 3-fork slice in `"rigid fork without class"` with `append([]catalog.Fork{fk}, f.Forks[1:]...)`); add Fenrir `XS`, `S`, `M`, `XL` insertion-bound rows to `TestSeatpostBoundsEverySize`; update `TestInstalledFrameUsesMaxChainstay` (`otso-fenrir-stainless` L is now non-sliding `0, 0`; use `salsa-fargo` L for the matching-max-chainstay `462, 462` case); and add a check for Fenrir's 4 listed forks and `enve_mountain_boost` under `Listed Forks` in `TestCompatibleForksRules`.

---

## Commit 1 (`C1`): Promote basis search into catalog tests and update ingest-frame skill

**Subject:** `Promote basis search into catalog tests and update ingest-frame skill`

### `A1`. Create `internal/reboot/catalog/basis_search_test.go`

Implement the D1 structs and helpers (`basisRange`, `basisEval`, `sizeImpliedA2c`, `basisSearchResult`, `evalBasis`, `searchBasis`, `logBasisSearch`), plus:

1. `TestBasisSearchCatalogFrames(t *testing.T)`:
   - Verifies `evalBasis(f, f.BasisLowerCup, f.BasisFork.StaticA2cMm, f.BasisFork.RakeMm)` succeeds and returns `Passes == true` for every frame in `catalog.Frames()`.
   - Runs `searchBasis` and asserts measured targets (tolerance `±0.02 mm` on implied/minimax, exact `RoundedA2cMm`, tolerance `±0.10 mm` on passing scan bounds, `±0.02 mm` on excesses) on three representative frames:
     - **`otso-fenrir-stainless`** (`f.BasisLowerCup`, `rake = 54`, `compareRakes = []float64{44, 52, 54}`):
       - `ImpliedMinMm`: `494.6122`, `ImpliedMaxMm`: `495.3967`, `MinimaxA2cMm`: `495.0044`, `RoundedA2cMm`: `495.0`
       - `A2cRange`: `[493.20, 496.80]`, `RakeRange`: `[51.15, 56.90]`
       - Comparisons at `495.0`: `rake 44` (`Passes: false`, `AxleHeightExcess: +2.0213`, `HasWheelbase: true`, `WheelbaseExcess: +6.6008`); `rake 52` (`Passes: true`, `-0.9594`, `-0.8232`); `rake 54` (`Passes: true`, `-1.6877`, `-2.6792`).
     - **`surly-krampus`** (`f.BasisLowerCup`, `rake = 47`, `compareRakes = []float64{44, 47, 51}`):
       - `ImpliedMinMm`: `481.2465`, `ImpliedMaxMm`: `481.9579`, `MinimaxA2cMm`: `481.6022`, `RoundedA2cMm`: `481.5`
       - `A2cRange`: `[477.95, 485.25]`, `RakeRange`: `[40.40, 53.65]`
       - Comparisons at `481.5`: `rake 44` (`Passes: true`, `-2.3900`, `-3.3532`); `rake 47` (`Passes: true`, `-3.3303`, `-6.1446`); `rake 51` (`Passes: true`, `-1.9294`, `-2.5240`).
     - **`moots-mountaineer`** (`f.BasisLowerCup`, `rake = 44`, `compareRakes = []float64{44, 51}`):
       - `ImpliedMinMm`: `505.9346`, `ImpliedMaxMm`: `508.3849`, `MinimaxA2cMm`: `507.1598`, `RoundedA2cMm`: `507.0`
       - `A2cRange`: `[504.20, 510.15]`, `RakeRange`: `[36.35, 51.10]`
       - Comparisons at `507.0`: `rake 44` (`Passes: true`, `-2.6133`, `HasWheelbase: false`, `math.IsNaN(WheelbaseExcess)`); `rake 51` (`Passes: true`, `-0.0476`, `HasWheelbase: false`).
   - Calls `logBasisSearch` when verbose/subtesting or inside a dedicated `t.Log` helper so running `go test -run TestBasisSearch -v ./internal/reboot/catalog` shows clean formatted tables.
2. `TestBasisSearchErrorsAndRakeIndependence(t *testing.T)`:
   - **Negative case 1 (empty sizes):** `empty := mustFrame(t, "surly-krampus"); empty.Sizes = nil` — both `evalBasis` and `searchBasis` return non-nil errors containing `"has no sizes"`.
   - **Negative case 2 (incompatible cup):** `krampus := mustFrame(t, "surly-krampus")` with `assembly.LowerCupSetup{Standard: assembly.LowerCupIS52, StackMm: 1}` — both `evalBasis` and `searchBasis` return `*assembly.InputError` with `Field == assembly.InputLowerCup` (tapered/bare bore mismatch).
   - **Regression guard (implied A2C uses `rake` parameter, not `f.BasisFork.RakeMm`):** copy `otso-fenrir-stainless` (whose `BasisFork.RakeMm` is `54` in C1; in C2 use a copy with `BasisFork.RakeMm = 54` vs `44`), call `searchBasis(f, f.BasisLowerCup, 44, []float64{44})` and compare against `f` mutated to `f.BasisFork.RakeMm = 44`: the two `MinimaxA2cMm` values must be identical (`491.0149 mm`), and distinct from `rake = 54` (`495.0044 mm`, difference `≈ 3.99 mm`).

### `S1`. Update `.agents/skills/ingest-frame/SKILL.md`

- In `### 2. Write internal/reboot/catalog/frame_<id>.go`:
  - Update the `Forks` bullet for `Q128`: note that `Forks` lists the fork models the manufacturer sells or specifies (using catalog `Fork` vars, which own their A2C/rake variants; `CompatibleForks()` automatically opens other matching travel/rigid-class forks whose steerer fits the head tube). Add a `Fork` to `forks.go` only for a real fork model.
- In `### 3. Find the basis with a temporary test`:
  - Remove the ~80-line inline `searchBasis` Go template.
  - Instruct the agent to copy and adapt a short `TestScratchBasis` in `internal/reboot/catalog/scratch_basis_test.go` that calls the committed helpers `searchBasis`, `evalBasis`, and `logBasisSearch` in `internal/reboot/catalog/basis_search_test.go` (naming `f := mustFrame(t, "<frame-id>")`, testing candidate `(cup, rake, compareRakes)` tuples with `searchBasis`, logging with `logBasisSearch`, and calling `evalBasis` directly to check specific `(cup, a2c, rake)` hypotheses).
  - Keep all existing reading rules and step 3 verification/deletion rules intact.

---

## Commit 2 (`C2`): Re-ingest Otso Fenrir Stainless from Otso's published chart

**Subject:** `Re-ingest Otso Fenrir Stainless from Otso's published chart`

### `A2`. Update `internal/reboot/catalog/frame_otso_fenrir_stainless.go`

- Update the `otsoFenrirStainless` doc comment to summarize the S1 nominal transcription (`Q140`–`Q143`) and `495/51` bare-head-tube basis (`Q142`) with `12 mm` Cane Creek typical `EC44` physical cup (`Q136`).
- Remove `MaxChainstayMm: 450` (defaults to `0`, `Q140`).
- `BasisLowerCup: assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0}` (`hypothesis (Q44, Q81, Q142): bare head tube`).
- `BasisFork: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 51, Steerer: assembly.SteererBasis}` (`hypothesis (Q44, Q81, Q142): S1 chart printed 495 mm A2C / 51 mm offset`).
- `ChartPrecision.HeadTubeAngleDeg: 0.5` (`inferred from published S1 digits (68°), Q80, Q142`).
- `Forks: []Fork{lithicCarbonMountain, enveMountainBoost, sid100, fox34_120}` (`Q128`).
- Across all five `publishedSize` entries (`XS`, `S`, `M`, `L`, `XL`):
  - `HeadTubeAngleDeg: 68`
  - `SeatTubeAngleDeg: 75`
  - `BbDropMm: 70` (published S1 range midpoint `68–72 mm`)
  - `ChainstayMm: 440` (published S1 range midpoint `430–450 mm`)
  - Omit `WheelbaseMm` (`0` = unpublished, `Q57`, `Q141`)
  - Set `MaxPostInsertionMm`: `XS: 170`, `S: 210`, `M: 260`, `L: 290`, `XL: 330` (`Q143`, S1/S3).
  - Preserve `ReachMm`, `StackMm`, `HeadTubeLengthMm`, `SeatTubeLengthMm` unchanged.

### `A3`. Update `internal/reboot/catalog/frame_otso_fenrir_stainless.md` & `SKILL.md`

- In `internal/reboot/catalog/frame_otso_fenrir_stainless.md`:
  - Update the header summary to reflect that the catalog now transcribes S1's nominal middle chart (`Q140`–`Q143`).
  - Replace the `## Catalog values` table so every field, value, and provenance label matches `frame_otso_fenrir_stainless.go` verbatim:
    - `ReachMm`, `StackMm`, `HeadTubeLengthMm`, `SeatTubeLengthMm`: as S1 (`published`).
    - `HeadTubeAngleDeg`: `68` (`published`, S1 nominal middle).
    - `SeatTubeAngleDeg`: `75` (`published`, S1 nominal middle).
    - `BbDropMm`: `70` (`derived`, midpoint of S1 `68-72mm`).
    - `ChainstayMm`: `440` (`derived`, middle of S1 `430-450mm`).
    - `MaxChainstayMm`: `0` (`catalog`, Q140: Tuning Chip shifts drop/angles/chainstay together rather than sliding horizontally).
    - `WheelbaseMm`: `0` (`unpublished`, Q57/Q141; S1 prints only 10-mm ranges).
    - `MaxPostInsertionMm`: `170 / 210 / 260 / 290 / 330` (`published`, S1/S3).
    - `LowerBore`, `DefaultLowerCup`: `Bore44Pressed`, `EC44 12 mm` (`published` standard S1; `illustrative` typical stack Q136, vs Wolf Tooth `12.25 mm` in S8).
    - `BasisLowerCup`: `bare head tube, 0 mm` (`hypothesis`, Q142).
    - `BasisFork`: `495 mm A2C, 51 mm offset` (`hypothesis` matching S1 chart's printed rigid fork rows, Q142).
    - `Forks`: `lithic_carbon_mountain`, `enve_mountain_boost`, `sid_100`, `fox34_120` (`catalog`, Q128/Q133).
    - `ChartPrecision`: lengths `0.5 mm`, HTA `0.5°` (`derived` from S1 digits, Q80/Q142).
    - `Year`: `2024` (`catalog`; chart dates to 2022, S2/S4).
  - Under `## Chart consistency`, add a dated `**2026-11-22, Slice 16e.**` section recording the committed `measureChartConsistency` and `searchBasis` results (`495/51` bare head tube passes all 5 sizes: `XS +1.70 / S +2.43 / M +1.89 / L +2.34 / XL +1.80 mm` vs budgets `±3.81 / ±3.84 / ±3.91 / ±3.97 / ±4.04 mm`; `searchBasis` at `51 mm` offset: implied `496.84–497.62 mm`, minimax `497.23 mm`, rounded `497.0 mm`, passing A2C range `493.50–500.95 mm`).
  - Under `## Open questions`, mark items 1–6 resolved with `Q128`, `Q133`, `Q140`, `Q141`, `Q142`, `Q143`.
  - Check carefully that no `make boundary` banned substring (`chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, `cad`) appears anywhere in the file.
- In `.agents/skills/ingest-frame/SKILL.md` line 46: change `"Copy the shape of frame_otso_fenrir_stainless.go, but not its numbers: its record shows that its chart values were derived, not published."` to `"Copy the shape of frame_otso_fenrir_stainless.go and frame_otso_fenrir_stainless.md."`

### `A4`. Update catalog tests (`frames_parity_test.go`, `basis_search_test.go`, `chart_consistency_test.go`, `frames_test.go`)

1. **`internal/reboot/catalog/frames_parity_test.go`:**
   - Keep `otso-fenrir-stainless` in `missingLegacyFrames` / `TestMissingLegacyFrames`.
   - In `TestFramesMatchLegacy` and `TestPrecisionMatchesLegacyInference`, skip `f.ID == "otso-fenrir-stainless"` with `t.Skipf("re-ingested from published Otso chart (Q126)")` / `continue` (and remove `modelOverride` and the `otso-fenrir-stainless` `wantInsertion` override). Update the doc comments to mention Q126 alongside Q119.
2. **`internal/reboot/catalog/basis_search_test.go`:**
   - In `TestBasisSearchCatalogFrames`, update the `otso-fenrir-stainless` case to `rake = 51`, `compareRakes = []float64{44, 51, 51.5, 52}` with probed targets:
     - `ImpliedMinMm`: `496.8361`, `ImpliedMaxMm`: `497.6214`, `MinimaxA2cMm`: `497.2288`, `RoundedA2cMm`: `497.0`
     - `A2cRange`: `[493.50, 500.95]`, `RakeRange`: `[41.45, 59.90]`
     - Comparisons at `497.0`: all four `Passes: true`, `HasWheelbase: false`, `math.IsNaN(WheelbaseExcess)`, `AxleHeightExcess`: `-0.9844` (`44`), `-3.2720` (`51`), `-3.0888` (`51.5`), `-2.9055` (`52`).
     - Also assert `evalBasis(fenrir, fenrir.BasisLowerCup, 495, 51)` passes with `AxleHeightExcess ≈ -1.4110` and `HasWheelbase: false`.
   - In `TestBasisSearchErrorsAndRakeIndependence`: construct a temporary copy of Fenrir with `BasisFork.RakeMm = 54` vs `44` (since the catalog Fenrir now has `51`, pass `rake = 44` vs `rake = 54` on that copy so the test still exercises mismatched `BasisFork.RakeMm` independence).
3. **`internal/reboot/catalog/chart_consistency_test.go`:**
   - Add helper `krampusL(t *testing.T) (catalog.Frame, catalog.FrameSize)` returning `"surly-krampus"`, size `"L"`.
   - `TestCheckChartConsistencyBudgetEdges`: switch from `fenrirL(t)` to `krampusL(t)`:
     - `"axle height inside"`: `StackMm = 616.7` → `""`
     - `"axle height outside"`: `StackMm = 616.8` → `"chart consistency: axle height mismatch +3.83 mm exceeds its ±3.76 mm rounding budget (Stack 0.50 mm, HTA 2.30 mm, HTL 0.47 mm, BB drop 0.50 mm)"`
     - `"wheelbase inside"`: `PublishedWheelbaseMm = 1145.3` → `""`
     - `"wheelbase outside"`: `PublishedWheelbaseMm = 1145.4` → `"chart consistency: wheelbase mismatch -6.59 mm exceeds its ±6.55 mm rounding budget (Reach 0.50 mm, HTA 4.79 mm, HTL 0.18 mm, BB drop 0.08 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)"`
   - `TestCheckChartConsistencyWithoutPrecision`: switch from `fenrirL(t)` to `krampusL(t)` → `"chart consistency: axle height mismatch -0.97 mm exceeds its ±0.00 mm rounding budget (no rounded columns)\nchart consistency: wheelbase mismatch +0.81 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"`.
   - `TestCheckChartConsistencyNudgeOutOfRange`: keep `fenrirL(t)`, change `s.ChartFrame.ChainstayMm = 72.4` → `70.4` (since `BbDropMm` is now `70`).
   - `TestCheckChartConsistencyUnnecessaryAllowance`: keep `fenrirL(t)`, update want to `"chart allowance: axle height allowance 0.50 mm is unnecessary: mismatch +2.34 mm is within its ±3.97 mm rounding budget"`.
   - `TestCheckChartConsistencyInvalidAllowance`: switch `fenrirL(t)` to `krampusL(t)` (whose `PublishedWheelbaseMm` is `1138 > 0`, needed for the `NaN wheelbase` case).
   - `TestCheckChartConsistencyUnpublishedWheelbase`: keep `fenrirL(t)` (drop the now-redundant `s.PublishedWheelbaseMm = 0` line or assert it's already `0`), update `"axle height outside budget"` to `s.ChartFrame.StackMm = 644.0` → `"chart consistency: axle height mismatch +4.34 mm exceeds its ±3.97 mm rounding budget (Stack 0.50 mm, HTA 2.51 mm, HTL 0.46 mm, BB drop 0.50 mm)"`, and `"no rounding"` → `"chart consistency: axle height mismatch +2.34 mm exceeds its ±0.00 mm rounding budget (no rounded columns)"`.
   - `TestMeasureChartConsistencyExposesChecks`: in `t.Run("Fenrir L unpublished wheelbase", ...)` keep `fenrirL(t)` (without overriding `PublishedWheelbaseMm`): `len(m.Checks) == 1`, `ah.MismatchMm ≈ 2.34` (`±0.005`), `ah.BudgetMm ≈ 3.97` (`±0.005`), `ah.Terms` = `["Stack", "HTA", "HTL", "BB drop"]` with `[0.50, 2.51, 0.46, 0.50]` (`±0.005`).
4. **`internal/reboot/catalog/frames_test.go`:**
   - `TestFrameProblemsCatchMistakes`: change `"max chainstay below a size"` from `f.MaxChainstayMm = 440` to `f.MaxChainstayMm = 430` (since Fenrir's `ChainstayMm` is now `440`); change `"rigid fork without class"` from `f.Forks = []catalog.Fork{fk, f.Forks[1], f.Forks[2]}` to `f.Forks = append([]catalog.Fork{fk}, f.Forks[1:]...)`.
   - `TestSeatpostBoundsEverySize`: add Fenrir bounds cases for `XS` (`setback_0` `[100, 170]`, `redshift_shockstop_pro` `[100, 170]`), `S` (`[100, 210]`, `[100, 210]`), `M` (`[100, 260]`, `[100, 250]`), `XL` (`[100, 330]`, `[100, 250]`).
   - `TestInstalledFrameUsesMaxChainstay`: change `{"otso-fenrir-stainless", "L", 450, 450}` to `{"otso-fenrir-stainless", "L", 0, 0}` and add `{"salsa-fargo", "L", 462, 462}` so both fixed-chainstay (`MaxChainstayMm == 0`) and sliding-at-max (`MaxChainstayMm == chart`) cases stay covered.
   - `TestCompatibleForksRules`: verify Fenrir's `CompatibleForks()` starts with `["lithic_carbon_mountain", "enve_mountain_boost", "sid_100", "fox34_120"]` and `IsListedFork("enve_mountain_boost")` is `true` (keeping `TestForkListsPlace`'s `2989` and `119` pins unchanged).

### `A5`. Update `query`, `cmd/bikegeo-reboot`, and `web` test pins

Every expected value below was measured directly by probing the real code against the corrected Fenrir copy:

1. **`internal/reboot/query/bike_test.go`:**
   - `TestBikeEvaluate("")` (tolerances unchanged `1e-5`):
     - `LeveledReachMm`: `421.587330735`, `LeveledStackMm`: `646.229156386`, `EttMm`: `601.658169423`, `WheelbaseMm`: `1145.619462324`
     - `BRP`: `(-158.060600865, 682.374276229)`
     - `Score`: `FitsNeither`, `Total`: `53.545302341`, `GripR`: `9.539697085`, `BrpMissXMm`: `-60.939399135`, `BrpMissYMm`: `30.725723771`
     - `Fitting`: `InsertionMm`: `143.103723490`, `RailOffsetMm`: `15`, `SpacersMm`: `0`, `Stem.LengthMm`: `130`
     - `SetbackCentered`: `SetbackMm`: `62.455524116`, `InsertionMm`: `129.044363193`
     - `SetbackReach`: `SetbackMm`: `48.006005019`, `InsertionMm`: `133.070701214`
     - `Builds`: `Evaluated`: `749`; `Builds[0]`: `FitsNeither`, `persuader20_800`, `lithic_carbon_mountain-495x44`, `LengthMm`: `120`, `AngleDeg`: `-30`, `SpacersMm`: `0`, `Total`: `55.893998809`, `GripR`: `1.783235050`
   - `TestBikeEvaluate("brp=off")`: `Score.Total`: `70.766560186`, `Builds[0].Score.Total`: `57.051476397`
   - `TestBikeEvaluate("fork_options=all")`: `Evaluated`: `23968`; `Builds[0]`: `firestarter_deluxe_483`, `SagPct`: `0`, `Score.Total`: `65.713076690`, `Score.GripR`: `1.104821393`
   - `TestBasisFrameset`: `PitchDeg()`: `-0.116991618` (tol `1e-9`)

2. **`internal/reboot/query/compare_test.go`:**
   - `TestParseCompareDefaultAll19Frames`: `RowCount`: `2989`, `Evaluated`: `2238761`, `len(Rows)`: `19`; existing `want` rows `{0, 1, 2, 7, 18}` are all **unchanged** (`surly-krampus`, `sklar-pbj`, `seven-custom`, `esker-hayduke`, `salsa-cutthroat`); add `{16, "otso-fenrir-stainless", "XL", "firestarter_deluxe_483", 0, assembly.FitsGrip, 69.087518917, 0.956687823}` to pin Fenrir's new rank #17.

3. **`cmd/bikegeo-reboot/main_test.go`:**
   - `TestRunBikeDefaultAndWinningBuildJSON`: update `assertNear(t, "Total", r.Score.Total, 53.545302, 1e-5)`.

4. **`internal/reboot/web/compare_test.go`:**
   - `TestCompareDefaultAll19Frames`:
     - Rows #1, #2, #3, #6, #8, #19 are all **unchanged**.
     - Check rank #17 (`otso-fenrir-stainless` XL): `frame = "Otso Fenrir Stainless"`, zero `abbr.sliding-mark`, `chainstay = "440.0"`, `score = "69.1"`, `oval-r = "0.96"`, `tier = "Fits grip"`.
     - Remove `"Otso Fenrir Stainless"` from the `slidingNote` expected-present list and assert `!strings.Contains(slidingNote, "Otso Fenrir Stainless")` (`6` sliding-dropout frames remain in the note: `Binary Havok`, `Binary Kinetik`, `Esker Hayduke`, `Revel El Jefe`, `Salsa Fargo`, `Sklar PBJ`).
   - `TestCompareCustomSelectionsAndAutoTopPerFrame` (`/?frame=otso-fenrir-stainless&size=L`):
     - `32` rows, `23968` evaluated (unchanged)
     - Row 1 `fork`: `"Salsa Firestarter / Cutthroat Carbon Deluxe 483mm"`, `score`: `"65.7"`, `oval-r`: `"1.10"`
     - Geom 1: `reach`: `"429.3"`, `stack`: `"641.1"`, `ett`: `"599.7"`, `chainstay`: `"440.0"` (no `*`).

5. **`internal/reboot/web/web_test.go`:**
   - `TestBikeWorkbenchDefaultViewKeepsWholeBikeDrawing`:
     - `viewBox`: `"-902.07 -850.54 2079.62 1256.87"`
     - `style`: `fmt.Sprintf("--units-per-px: %.4f;", 2079.62/nominalDiagramWidthPx)`
   - `TestBikeWorkbenchViewsShareReadoutsAndDifferInFraming`:
     - `wheelbase`: `"1145.6 mm"`, `brp-grip-dist`: `"551.5 mm"`
     - `viewBox` cases: `"bike"` → `"-902.07 -850.54 2079.62 1256.87"`, `"seat"` → `"-266.63 -703.82 137.14 82.89"`, `"stem"` → `"300.54 -768.85 239.50 144.75"`
   - `TestBikeWorkbenchFitsFitReportBrpAtRailLimit`: `fit-insertion` `"196.4 mm"`, `post_insert` `196.447506` (`1e-6`)
   - `TestBikeWorkbenchFitsFitReportGripAtLimits`: `fit-grip-miss` `"35.7 mm"`
   - `TestBikeWorkbenchBothFitTargets`: `post_insert` `196.447506` (`1e-6`)
   - `TestBikeWorkbenchRanksBuilds`:
     - Row 1: `tier` `"Fits neither"`, `score` `"57.1"`, `stem` `"120 mm, -30°"`, `spacers` `"0 mm"`, `grip-miss` `"31.7 mm"`, `oval-r` `"1.78"`, `fork` `"Lithic Carbon Mountain (495/44)"`, `brp-miss` `"—"`
     - Top 3 scores: `[]string{"57.1", "54.1", "52.0"}`
     - Row 1 attributes (12): `[]string{"19.5", "—", "—", "0.0", "100.0", "100.0", "100.0", "—", "100.0", "—", "100.0", "95.1"}`
   - `TestBikeWorkbenchFitsBuildsAcrossAllBars`:
     - `want`:
       - `{barAlloy50, "120 mm, -30°", "0 mm", "0.90", "67.8"}`
       - `{barFLX12x15, "120 mm, -10°", "0 mm", "0.63", "67.5"}`
       - `{barFLX12x15, "110 mm, -17°", "0 mm", "0.84", "67.4"}`
       - `{barFLX12x30, "110 mm, -17°", "0 mm", "0.95", "67.2"}`
       - `{barFLX12x15, "120 mm, -17°", "5 mm", "0.25", "66.7"}`
     - `row 2 apply`: `stem_length = "120"`, `stem_angle = "-10"` (all other checked params including `bar = "sqlab311_flx_12_15"` unchanged)
     - `different submitted bar` (`/bike?bar_options=all&bar=sqlab3ox_12_15&brp=off`): scores `[]string{"75.7", "75.2", "75.0", "74.8", "73.9"}` (all `"Fits grip"`)
   - `TestBikeWorkbenchFitsBuildsKeepsSubmittedBarRoll`:
     - `{barAlloy50, "120 mm, -30°", "0 mm", "0.61", "76.5"}`
     - `{barFLX12x15, "110 mm, -17°", "0 mm", "0.62", "75.6"}`
     - `{barFLX16x30, "120 mm, -30°", "5 mm", "0.96", "75.5"}`
     - `{barFLX16x30, "120 mm, -30°", "0 mm", "0.97", "75.5"}`
     - `{bar3OX16x45, "120 mm, -30°", "0 mm", "0.99", "75.5"}`
   - `TestBikeWorkbenchBuildSweepColumn`: `"all bars"` sweep score `"100.0"` (`sqlab311_20_16_50` has `16°` backsweep, which sits in `16°–20°`); `"selected bar"` stays `"100.0"`.
   - `TestBikeWorkbenchSubmitsBarOptionsSelect`: row 1 `bar` is `barAlloy50`.
   - `TestBikeWorkbenchAppliesFreeScoredBuild`: `score` `"99.8"` (unchanged `stem`: `"70 mm, +0°"`, `spacers`: `"10 mm"`, `oval-r`: `"0.00"`, `tier`: `"Fits grip"`).
   - `TestBikeWorkbenchBuildRankingKeepsSaddleFitAndTargets`: `score`: `"55.9"`, `brp-miss`: `"(-49.8, +0.0) mm"`, `post_insert` `196.447506` (`1e-6`).
   - `TestWorkbenchHandlebarReferenceHandGrips`:
     - `{"persuader20_800", 388.546580, 755.539727, 330}`
     - `{"persuader20_840", 381.267176, 755.539727, 350}`
     - `{"sqlab3ox_16_15", 387.404312, 743.021197, 320}`
     - `{"sqlab3ox_16_30", 391.275374, 757.077185, 320}`
     - `{"sqlab3ox_16_45", 395.146437, 771.133174, 320}`
     - `{"sqlab3ox_12_15", 406.062801, 743.021197, 320}`
     - `{"sqlab3ox_12_30", 408.932314, 757.077185, 320}`
     - `{"sqlab3ox_12_45", 411.801828, 771.133174, 320}`
     - `{"sqlab311_20_16_15", 393.139219, 745.688230, 300}`
     - `{"sqlab311_20_16_25", 395.719928, 754.900832, 300}`
     - `{"sqlab311_20_16_50", 402.171699, 777.932337, 300}`
     - `{"sqlab311_flx_16_15", 393.139219, 741.622661, 300}`
     - `{"sqlab311_flx_16_30", 397.010282, 755.678649, 300}`
     - `{"sqlab311_flx_12_15", 410.313932, 741.622661, 300}`
     - `{"sqlab311_flx_12_30", 413.183446, 755.678649, 300}`
   - `TestBikeWorkbenchDefaultHandlebar`: `brp-grip-dist` `"551.5 mm"`.
   - `TestBikeWorkbenchChosenHandlebarChangesBuild`: `brp-grip-dist` `"564.7 mm"`, `sitbones-grip-rise` `"+4.6 mm"` (`hand-lateral` `"320.0 mm"`).
   - `TestBikeWorkbenchFitsAroundChosenHandlebar`: `fit-stem-length` `"124.9 mm"`, row 1 `oval-r` `"0.45"`, `score` `"73.4"` (`stem` `"120 mm, -17°"`, `spacers` `"0 mm"`, `tier` `"Fits grip"`).
   - `TestBikeWorkbenchSetbackNotesFitReport`: `fit-setback-centered` `"62.5 mm"`, `fit-setback-centered-insertion` `"179.0 mm"`, `fit-setback-reach` `"48.0 mm"`, `fit-setback-reach-insertion` `"183.1 mm"`.
   - `TestBikeWorkbenchSetbackNotesReachedFit`: `fit-rail-offset` `"8.9 mm"`, `fit-setback-centered` `"33.6 mm"`, `fit-setback-centered-insertion` `"187.1 mm"`.
   - `TestBikeWorkbenchSetbackNotesForwardTarget`: `centeredSpan` contains `"16.1 mm short"` (`fit-setback-reach-insertion` `"100.2 mm"`, `fit-setback-centered-insertion` `"104.2 mm"` stay unchanged).
   - `TestBikeWorkbenchSetbackNotesInsertionLimit`: `fit-setback-centered` `"0.2 mm"` (was `"1.7 mm"`), `reachSpan` contains `"141.0 mm short"` (`centeredSpan` stays `"144.3 mm short"`, insertions `"100.0 mm"`).
   - `TestBikeWorkbenchForkOptionsDefaultKeepsSubmittedFork`: `Basis` row 1 `score` `"72.4"` (`stem` `"120 mm, -20°"` unchanged).
   - `TestBikeWorkbenchFitsBuildsAcrossAllForks`:
     - `{forkFirestarter, "120 mm, -20°", "0 mm", "1.10", "72.4", "100.0"}`
     - `{forkSklar, "120 mm, -20°", "0 mm", "1.19", "70.3", "100.0"}`
     - `{forkKrampus483, "120 mm, -20°", "0 mm", "1.19", "71.2", "100.0"}`
     - `{forkSID51_20, "120 mm, -20°", "0 mm", "1.27", "68.8", "100.0"}`
     - `{forkFirestarter, "120 mm, -17°", "0 mm", "1.29", "73.2", "100.0"}`
   - `TestBikeWorkbenchForkOptionsWithSaddle`:
     - `{forkFirestarter, "120 mm, -20°", "0 mm", "65.7", "0.0"}`
     - `{forkSklar, "120 mm, -20°", "0 mm", "64.4", "0.0"}`
     - `{forkKrampus483, "120 mm, -20°", "0 mm", "64.9", "0.0"}`
     - `{forkSID51_20, "120 mm, -20°", "0 mm", "63.4", "0.0"}`
     - `{forkFirestarter, "120 mm, -17°", "0 mm", "66.2", "0.0"}`
     - Row 1 `brp-miss`: `"(-58.4, +0.0) mm"`, `post_insert`: `198.708763` (`1e-6`).
   - `TestBikeWorkbenchForkOptionsAndBarOptions`:
     - `{barFLX16x30, forkKrampus483, "120 mm, -17°", "0 mm", "71.7"}`
     - `{alloy16x25, forkKrampus483, "120 mm, -17°", "0 mm", "71.6"}`
     - `{barFLX16x30, forkFirestarter, "120 mm, -17°", "0 mm", "71.4"}`
     - `{alloy16x15, forkKrampus483, "120 mm, -17°", "0 mm", "71.4"}`
     - `{flx16x15, forkKrampus483, "120 mm, -17°", "0 mm", "71.3"}`
   - `TestBikeWorkbenchDefaultInsertionMidpointQ109`: `brp-height` `"700.4 mm"`, `BRP.X` `-158.061` (`0.001`), `BRP.Y` `682.374` (`0.001`).
   - `TestBikeWorkbenchFrameHandlingReadouts`:
     - `default Lithic`: `trail` `"107.0 mm"`, `rear-center` `"435.1 mm"`, `rc-wb` `"38.0%"`
     - `Fox 34 at 20%`: `trail` `"112.0 mm"`, `rear-center` `"435.8 mm"`, `rc-wb` `"37.9%"`
     - `basis`: `trail` `"94.5 mm"`, `rear-center` `"434.3 mm"`, `rc-wb` `"37.8%"`
   - `TestBikeWorkbenchBuildTiersAndLegend`:
     - Selected bar: `score` `"70.4"`, `Saddle rail offset` `"61.2"` (`tier` `"Fits BRP"`, `brp-miss` `"(+0.0, +0.0) mm"`)
     - All bars: `tier` `"Fits all"`, `score` `"82.3"`, `bar` `barAlloy50`, `stem` `"120 mm, -30°"`, `spacers` `"0 mm"`, `oval-r` `"0.90"`, `Saddle rail offset` `"61.2"`
   - `TestBikeSlidingDropoutsAndUnpublishedWheelbase`:
     - Keep `binary-havok` and `moots-mountaineer` checks unchanged.
     - On `fenrir := loadBikeDoc(t, "/bike")`: assert `!strings.Contains(fenrir.Text(), "(Published")` (`Q141`) and `fenrir.Find("[data-note=\"sliding-dropouts\"]").Length() == 0` (`Q140`).
     - Add `krampus := loadBikeDoc(t, "/bike?frame=surly-krampus&size=L")` asserting `strings.Contains(krampus.Text(), "(Published 1138.0 mm)")`.
   - `TestBikeLeveledReadoutsAndRedshiftNote`:
     - Default `/bike`: `leveled-reach` `"421.6 mm"`, `leveled-stack` `"646.2 mm"`, `ett` `"601.7 mm"`
     - `fork=basis`: `leveled-reach` `"429.3 mm"`, `leveled-stack` `"641.1 mm"`, `ett` `"599.7 mm"`
   - `TestBikeSubmittedBuildScoreQ110`:
     - Default `/bike`: `submitted-score` `"53.5"`, `submitted-oval-r` `"9.54"`, `submitted-brp-miss` `"(-60.9, +30.7) mm"` (`Stem` `"86.5"` unchanged)
     - `brp=off`: `submitted-score` `"70.8"`
   - `TestBikeForkSelectGroupsListedAndOtherForks`:
     - On `/bike`, `optgroups` stays `["Listed Forks", "Other Compatible Forks", "Chart Basis Fork"]`, and additionally assert `select[name="fork"] optgroup[label="Listed Forks"] option[value="enve_mountain_boost-490x44"]` has length `1` and `Other Compatible Forks` has `0` options for `enve_mountain_boost-490x44`.

---

## Rules That Bite

1. **`make boundary` scans every file under `internal/reboot` and `cmd/bikegeo-reboot` (outside `*_parity_test.go`), including `frame_otso_fenrir_stainless.md`!** Never write banned words (`chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, `cad`) in any case or inside longer words.
2. **Trig lives in `internal/reboot/pose`:** `basis_search_test.go` must not call `math.Sin`/`Cos`/`Tan`/`Atan2`/`Pi`; use only `math.Inf`, `math.IsNaN`, `math.Round`, `min`, `max` and call `measureChartConsistency`.
3. **No legacy or sibling-frame edits:** do not touch `internal/frames`, `internal/catalog`, or any catalog frame other than `frame_otso_fenrir_stainless.go` and `.md`.
4. **Commit messages (`AGENTS.md`):** plain-language subjects only (no Conventional Commits prefixes).
5. **Clean commit boundaries:** C1 makes zero catalog or runtime test changes; each commit passes `make pre-commit` on its own.
6. **Implementation handoff (`DESIGN.md`):** after C2 (or if stopped/blocked), record the `Status:` line and `## Implementation handoff` section in `docs/reboot/SLICE16e.md`, and update the Slice 16e line in `docs/reboot/DESIGN.md` Milestones to match.

---

## Verification Steps

1. After C1:
   - `make pre-commit` passes (re-run if `go fix`/`go fmt` touches anything).
   - `go test -run TestBasisSearch -v ./internal/reboot/catalog` passes and prints the formatted tables for `otso-fenrir-stainless`, `surly-krampus`, and `moots-mountaineer`.
2. After C2:
   - `make pre-commit` passes on the final state.
   - `go test -run 'TestBasisSearch|TestEveryChartIsConsistent|TestForkListsPlace|TestSeatpostBoundsEverySize|TestFramesMatchLegacy' -v ./internal/reboot/catalog` passes with `otso-fenrir-stainless` explicitly skipped in parity and `2989` fork-list / `119` basis placements passing.
   - Live HTTP/CLI check:
     - `go run ./cmd/bikegeo-reboot bike` outputs Fenrir L at `LeveledReachMm ≈ 421.59`, `LeveledStackMm ≈ 646.23`, `Score.Total ≈ 53.55`.
     - Start `make serve-reboot` (or targeted `httptest`/browser inspection): confirm `/bike` omits `[data-note="sliding-dropouts"]` and `(Published ...)`, lists `ENVE Mountain Carbon Boost` under `Listed Forks`, and prints `Chart Basis Fork (495 mm / 51 mm / 0 mm cup)` in the summary; confirm `/` shows `Otso Fenrir Stainless` without `*` (`440.0` chainstay) and lists only the 6 sliding-dropout frames in the footnote. Record screenshot path (if taken) or live-check numbers in the handoff.

---

## Out of Scope

- Slice 17 (`deadcode` cleanup for `FitOval.Validate` / `catalog.Forks` and legacy frame `Notes` parity audit).
- Slice 18a / 18b (deleting legacy packages and deleting `ritchey-ultra`).
- Updating `frame_<id>.md` or published seatpost insertion depths for the other 18 frames.
- Fractional basis formatting in `bike.html` (Fenrir's basis `495 / 51 / 0` uses integers).
