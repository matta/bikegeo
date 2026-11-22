# Slice 16c: the fork model

Status: Planned 2026-11-22 (Q128, Q129, Q131–Q134; ready for implementation)

## Goal

Move published axle-to-crown and rake variants onto `catalog.Fork` models, close suspension travel in `assembly` to a typed table (`0, 100, 120, 130 mm`) with per-travel sag ranges and Q132 sag scoring, add the official FOX 29" `15QRx110` User Specs (`fork_fox_34.md`, `fork_fox_34_sl.md`, `fork_fox_36_sl.md`), remove unsupported/unused forks, and wire Q131 variant identifiers and Q134 default sags through `compare`, `query`, `web`, and `cmd/bikegeo-reboot`.

Frames still offer only their listed `Frame.Forks` (`[]Fork`); cross-frame fork classes, typical lower-cup selection, and open fork choice stay in Slice 16d ([DESIGN.md](DESIGN.md) Q129, Q130).

## Confirmed Rulings

- **Q128:** Catalog `Fork` models own their `Variants []ForkVariant`; a frame lists compatible `Fork` models (`[]Fork`) and offers all variants of each listed fork. Frame `BasisFork` fields stay exact (untouched for chart consistency).
- **Q129:** Travel is a closed named enum type (`assembly.ForkTravel`), not a free `float64`: `TravelRigid (0)`, `Travel100 (100)`, `Travel120 (120)`, `Travel130 (130)`, each mapped by exhaustive `switch` to its sag steps (`100 -> [15, 20]`, `120 -> [15, 20, 25]`, `130 -> [20, 25]`). Delete `sid_110`, `pike_140`, and `fox36_150`.
- **Q131:** Variant query IDs use bare `Fork.ID` on single-variant models (`fox34_sl_120`, `surly_krampus_483`) and `<Fork.ID>-<StaticA2cMm>x<RakeMm>` on multi-variant models (`lithic_carbon_mountain-495x44`, `lithic_carbon_mountain-498x51.5`, `sid_100-506x44`, `fox34_120-534x44`) formatted without trailing `.0`. On `/`, a bare multi-variant model ID selects all variants of that model; on `/bike`, `fork=` requires a concrete geometry (`basis`, bare single-variant ID, or `<model>-<a2c>x<rake>`) and rejects a bare multi-variant model ID with HTTP `400`.
- **Q132:** `AttrForkSag` keeps weight `3` (`36` total with BRP, `23` without); rigid and basis forks score `100`. Suspension forks score `100` at `maxSag`, falling linearly to `90` at `minSag` (`[15, 20]` for `100 mm`; `[15, 25]` for `120 mm`; `[20, 25]` for `130 mm`), and linearly to `0` over a `10%` span below `minSag` (`minSag - 10%`) and above `maxSag` (`maxSag + 10%`), clamped to `[0, 100]`.
- **Q133:** Update `fox34_120` (`534/44`, `534/51`) and `fox34_130` (`544/44`, `544/51`) to MY25 FOX 34 specs; add `fox34_sl_120` (`530.5/44`), `fox34_sl_130` (`540.5/44`), `fox36_sl_120` (`536/44`), and `fox36_sl_130` (`546/44`); trim `enve_mountain_boost` to S5 (`490/44`, `492/52`) and `lithic_carbon_mountain` to S6 (`495/44`, `498/51.5`); delete `enve_mountain_472` and `rigid_495`; set both `seven-expedition-926d` and `seven-expedition-926f` to `[enve_mountain_boost, lithic_carbon_mountain, fox34_120]`; replace `rigid_495` on `esker-hayduke` with `enve_mountain_boost` and `lithic_carbon_mountain`; drop `pike_140` from `turner-venn-ti`.
- **Q134:** On `/bike`, selecting a suspension fork without `sag=` defaults `sag` to the travel's `MaxSagPct` (`20` for `100 mm`, `25` for `120 mm` and `130 mm`), removing `catalog.DefaultSuspensionSagPct`.

## Planner Decisions (`D1`–`D9`)

- **D1 (`assembly.ForkTravel` enum and methods):** Define `type ForkTravel int` in `internal/reboot/assembly/frameset.go` with constants `TravelRigid = 0`, `Travel100 = 100`, `Travel120 = 120`, `Travel130 = 130`. Replace `ForkSpec.TravelMm float64` with `ForkSpec.Travel ForkTravel`. Implement exhaustive `switch t` methods on `ForkTravel`:
  - `Valid() bool`: `true` for the four constants, `false` in `default`.
  - `Mm() float64`: returns `0, 100, 120, 130` (and `0` in `default`).
  - `IsSuspension() bool`: `false` for `TravelRigid` (and `default`), `true` for `Travel100/120/130`.
  - `SagRange() (minPct, maxPct float64, ok bool)`: `Travel100 -> (15, 20, true)`, `Travel120 -> (15, 25, true)`, `Travel130 -> (20, 25, true)`, `TravelRigid -> (0, 0, false)`.
  - `SagPcts() []float64`: returns a fresh slice copy (`nil` for `TravelRigid`, `{15, 20}`, `{15, 20, 25}`, `{20, 25}`).
  - `DefaultSagPct() float64`: returns `0, 20, 25, 25` (`Q134`).
  - `String() string`: `"rigid"`, `"100mm"`, `"120mm"`, `"130mm"`.
- **D2 (`ForkSetup.EffectiveA2cMm` & validation):** `EffectiveA2cMm()` computes `f.Spec.StaticA2cMm - f.Spec.Travel.Mm()*(f.SagPct/100.0)`. `ForkSetup.Validate()` checks `!f.Spec.Travel.Valid()` returning a plain error (`fmt.Errorf("unsupported fork travel %d mm", f.Spec.Travel)` so `InputError.Field` stays `""` matching existing `frameset_test.go`), `f.Spec.Steerer == SteererBasis && f.Spec.Travel != TravelRigid` (plain error), and `f.Spec.Travel == TravelRigid && f.SagPct != 0` (`InputForkSag`). Negative test cases in `frameset_test.go`: `Travel: -10`, `Travel: 110`, `Travel: 140`, `Travel: 150`.
- **D3 (`catalog` fork & variant API):** In `internal/reboot/catalog/forks.go`:
  - `type ForkVariant struct { StaticA2cMm float64; RakeMm float64 }`
  - `type Fork struct { ID string; Name string; Travel assembly.ForkTravel; Steerer assembly.Steerer; Variants []ForkVariant }`
  - `(f Fork) IsMultiVariant() bool { return len(f.Variants) > 1 }`
  - `(f Fork) VariantID(v ForkVariant) string`: `strconv.FormatFloat(x, 'f', -1, 64)` formatting `StaticA2cMm` and `RakeMm`; returns `f.ID` when `len(f.Variants) == 1`, else `fmt.Sprintf("%s-%sx%s", f.ID, fmtF(v.StaticA2cMm), fmtF(v.RakeMm))`.
  - `type ForkOption struct { Fork Fork; Variant ForkVariant }` with `(o ForkOption) ID() string`, `(o ForkOption) Name() string`, and `(o ForkOption) Spec() assembly.ForkSpec`.
  - Rename `enve_mountain_boost` display `Name` from `"ENVE Mountain Carbon Boost 492mm"` to `"ENVE Mountain Carbon Boost"`.
  - Add `ForkByID(id string) (Fork, bool)` in `catalog`.
  - Replace `Frame.Forks []FrameFork` with `Frame.Forks []Fork`, and remove `FrameFork`, `SuspensionSagPcts()`, and `DefaultSuspensionSagPct`.
  - Add `(f Frame) ForkOptions() []ForkOption` expanding `f.Forks` in declared order, each fork's `Variants` in canonical `(StaticA2cMm, RakeMm)` ascending order; and `(f Frame) LookupFork(id string) ([]ForkOption, bool)` returning the single matching `ForkOption` for any concrete variant ID or all variants of `fk` when `id == fk.ID` for a multi-variant fork in `f.Forks`.
- **D4 (`catalog` validation & legacy parity):**
  - `TestForks` validates all 21 `catalog.Forks()`: non-empty unique IDs/names, valid non-basis `Steerer`, `Travel.Valid()`, non-empty `Variants` strictly ascending by `(StaticA2cMm, RakeMm)` with finite `StaticA2cMm > 0` and finite `RakeMm > 0`, and unique `VariantID`s.
  - `TestFrameProblems` checks every frame has non-empty unique `Forks` with valid forks (`frames_test.go`), and `TestForkListsPlace` places every `(frame, size, forkOption, travel sag)` asserting `placements == 1100` and `bases == 119`.
  - Per the settled `SLICE16c` rule, delete `TestForksMatchLegacy` from `parts_parity_test.go` (keep stem, handlebar, seatpost, and saddle parity) and delete fork assertions from `TestFramesMatchLegacy` in `frames_parity_test.go` (keep chart, headset, basis, insertion, and `MaxChainstayMm` parity).
- **D5 (`compare` package):**
  - Add `ModelID string` to `compare.Fork` (`ModelID` defaults to `ID` when empty in unit fixtures; `query.CatalogFrames()` sets `ID: o.ID()`, `ModelID: o.Fork.ID`).
  - Remove `Request.SuspensionSagPcts` (`compare.go`); replace open sag expansion with `fk.Spec.Travel.SagPcts()` (or `[0]` when rigid). Validate every frame fork via `fk.Spec.Travel.Valid()` (plus `Validate` when placing).
  - In `expandSelections` (`compare.go`): match concrete `fk.ID == sel.ForkID` first; otherwise match all forks with `fk.ModelID == sel.ForkID` (checking `SagPct` compatibility on every matched variant). Update `Validation` error case in `compare_test.go` from `"no sag steps"` to an unsupported `Travel` (`ForkTravel(110)` / `ForkTravel(140)`).
  - Update `compare_test.go` `fixtureFrames()` to Slice 16c variants (`fox34_120-534x44` & `534x51`, `lithic_carbon_mountain-495x44` & `498x51.5`, `sid_100-506x44` & `506x51`, `surly_krampus_483`) so `TestRankFullFixture` and `TestRankSelections` verify multi-variant expansion, `ModelID` selection (`fork = "fox34_120"` -> 6 rows, `fork = "fox34_120"` with `sag = 20` -> 2 rows), concrete selection (`fork = "fox34_120-534x44"` with `sag = 22`), and `RowCount = 26, Evaluated = 1820`.
- **D6 (`query` package):**
  - Replace `FrameFork` field on `Bike` and `BikeForkOption` with `ForkOption catalog.ForkOption`.
  - `CatalogFrames()` expands each frame's `ForkOptions()` into `compare.Fork{ID: o.ID(), ModelID: o.Fork.ID, Spec: o.Spec()}`.
  - `DefaultBike()` picks `otso-fenrir-stainless` size `L`'s first `ForkOption`: `ForkID: "lithic_carbon_mountain-495x44"` (`StaticA2cMm: 495`, `RakeMm: 44`, `TravelRigid`, `SteererTapered`).
  - `parseBikeFork` (`query/bike.go`): `rawFork == "basis"` keeps `BasisFork`; `rawFork == ""` selects `frame.ForkOptions()[0]`; non-empty calls `frame.LookupFork(rawFork)`. If `len(matches) == 0`, return unknown/incompatible error; if `len(matches) > 1`, reject bare multi-variant model ID with `InputError{Field: "fork"}` naming e.g. `matches[0].ID()`. When setting default `sag` (`sag=` omitted or `prev_fork` changed), use `opt.Fork.Travel.DefaultSagPct()` (`Q134`: `20` for 100 mm, `25` for 120 & 130 mm, `0` for rigid).
  - `ForkOptions(frame, cup)` (`query/bike.go`) iterates `o` in `frame.ForkOptions()`, emitting `BikeForkOption{ForkOption: o, LowerCup: cup}` once if rigid or once per sag in `o.Fork.Travel.SagPcts()`.
  - `ParseCompare` (`query/compare.go`): accepts any `fork` value supported by `frame.LookupFork(raw)` (single-variant bare ID, concrete `<model>-<a2c>x<rake>`, or bare multi-variant model ID on `/`). `BikeForRow` / `RowBikeURL` resolves `row.ForkID` (always concrete `o.ID()`).
- **D7 (`web` compare & bike UI):**
  - On `/` (`compare_page.go`): `<select name="fork">` renders `<option value="<ID>"><Name></option>` for single-variant forks and `<option value="<ID>"><Name> (all variants)</option>` followed by `<option value="<variantID>"><Name> (<a2c>/<rake>)</option>` for multi-variant forks. `<select name="sag">` uses `selectedFork.Travel.SagPcts()` when a suspension fork (concrete variant or bare multi-variant model) is selected (`[15, 20]` for 100, `[15, 20, 25]` for 120, `[20, 25]` for 130), keeping an explicit out-of-step valid value such as `18` if present; when no fork is selected, offer union `{15, 20, 25}`. Update `forkCellName` on `/` so multi-variant forks include `(<a2c>/<rake>)` in their cell text (e.g. `"Fox Float 34 120mm (534/44), 25% sag"` vs single-variant `"RockShox Pike 120mm, 25% sag"`).
  - On `/bike` (`bike_page.go`): `<select name="fork">` lists each `catalog.ForkOption` from `frame.ForkOptions()` with `value = o.ID()` and `query.FormatFloat` A2C/Rake labels (showing `530.5`, `51.5`, etc. cleanly without truncating `.5`), plus `basis`. Update `forkNameAndTravel` to resolve `o.ID()` and return `Fork.Name, Fork.Travel, Fork.IsMultiVariant(), ForkVariant`.
  - Update `web/compare_test.go` & `query/compare_test.go`: note `fox36_150` is deleted, so test "frame has no fork" with an existing catalog fork the Krampus doesn't list (such as `pike_120`), plus test deleted/unknown `fox36_150` as invalid fork.
- **D8 (`prev_frame` switch tests in `query/bike_test.go` & `web/web_test.go`):**
  With Q133, `esker-hayduke` now carries `lithic_carbon_mountain`. Therefore switching from default `otso-fenrir-stainless` (`lithic_carbon_mountain-495x44`) → `esker-hayduke` *preserves* `lithic_carbon_mountain-495x44`, whereas switching from `sid_100-506x44` on Fenrir → `esker-hayduke` falls back to `pike_120` (`531/44`, `sag=25`). Update tests to cover both behaviors cleanly!
- **D9 (`cmd/bikegeo-reboot`):** Update CLI help examples and tests from bare `fox34_120&sag=20` to `fox34_120-534x44&sag=20` (and `sag=25` / exact probed `post_insert`), and add CLI coverage for bare multi-variant model ID on `compare` (`fox34_120` returning 2 rows) and HTTP/CLI rejection on `bike`.

---

## Complete Catalog & Frame Data Reference

### 1. Catalog Fork Inventory (21 forks after Q129 & Q133)

Deleted from previous 22-fork catalog (5): `sid_110`, `pike_140`, `fox36_150`, `enve_mountain_472`, `rigid_495`.
Added from FOX User Specs (4): `fox34_sl_120`, `fox34_sl_130`, `fox36_sl_120`, `fox36_sl_130`.

| ID | Display Name | Travel | Steerer | Variants (`StaticA2cMm / RakeMm`) | Concrete Variant Query IDs (`Q131`) | Source |
| --- | --- | --- | --- | --- | --- | --- |
| `sunliner_440` | `Tumbleweed Chromoly Steel Unicrown 440mm` | `TravelRigid` | `SteererStraight118` | `440 / 55` | `sunliner_440` | catalog |
| `surly_ecr_468` | `Surly ECR 4130 Chromoly 468mm` | `TravelRigid` | `SteererStraight118` | `468 / 43` | `surly_ecr_468` | catalog |
| `firestarter_deluxe_483` | `Salsa Firestarter / Cutthroat Carbon Deluxe 483mm` | `TravelRigid` | `SteererTapered` | `483 / 51` | `firestarter_deluxe_483` | catalog |
| `surly_krampus_483` | `Surly Krampus 4130 Chromoly 483mm` | `TravelRigid` | `SteererStraight118` | `483 / 47` | `surly_krampus_483` | catalog |
| `rigid_483` | `Rigid Bikepacking 483mm` | `TravelRigid` | `SteererTapered` | `483 / 44`, `483 / 51` | `rigid_483-483x44`, `rigid_483-483x51` | catalog |
| `rigid_485` | `Rigid Bikepacking 485mm` | `TravelRigid` | `SteererTapered` | `485 / 45`, `485 / 52` | `rigid_485-485x45`, `rigid_485-485x52` | catalog |
| `sklar_steel_485` | `Sklar Chromoly Steel 485mm` | `TravelRigid` | `SteererStraight118` | `485 / 52` | `sklar_steel_485` | catalog |
| `rigid_490` | `Rigid Bikepacking 490mm` | `TravelRigid` | `SteererTapered` | `490 / 44` | `rigid_490` | catalog |
| `enve_mountain_boost` | `ENVE Mountain Carbon Boost` | `TravelRigid` | `SteererTapered` | `490 / 44`, `492 / 52` | `enve_mountain_boost-490x44`, `enve_mountain_boost-492x52` | published (`S5`) |
| `lithic_carbon_mountain` | `Lithic Carbon Mountain` | `TravelRigid` | `SteererTapered` | `495 / 44`, `498 / 51.5` | `lithic_carbon_mountain-495x44`, `lithic_carbon_mountain-498x51.5` | published (`S6`) |
| `rigid_500` | `Rigid Bikepacking 500mm` | `TravelRigid` | `SteererTapered` | `500 / 44`, `500 / 51` | `rigid_500-500x44`, `rigid_500-500x51` | catalog |
| `rigid_510` | `Rigid Bikepacking 510mm` | `TravelRigid` | `SteererTapered` | `510 / 44` | `rigid_510` | catalog |
| `sid_100` | `RockShox SID 100mm` | `Travel100` | `SteererTapered` | `506 / 44`, `506 / 51` | `sid_100-506x44`, `sid_100-506x51` | catalog |
| `fox34_sl_120` | `Fox 34 SL 120mm` | `Travel120` | `SteererTapered` | `530.5 / 44` | `fox34_sl_120` | published (`fork_fox_34_sl.md`) |
| `sid_120` | `RockShox SID 120mm` | `Travel120` | `SteererTapered` | `530 / 44`, `534 / 44` | `sid_120-530x44`, `sid_120-534x44` | catalog |
| `pike_120` | `RockShox Pike 120mm` | `Travel120` | `SteererTapered` | `531 / 44` | `pike_120` | catalog |
| `fox34_120` | `Fox Float 34 120mm` | `Travel120` | `SteererTapered` | `534 / 44`, `534 / 51` | `fox34_120-534x44`, `fox34_120-534x51` | published (`fork_fox_34.md`) |
| `fox36_sl_120` | `Fox 36 SL 120mm` | `Travel120` | `SteererTapered` | `536 / 44` | `fox36_sl_120` | published (`fork_fox_36_sl.md`) |
| `fox34_sl_130` | `Fox 34 SL 130mm` | `Travel130` | `SteererTapered` | `540.5 / 44` | `fox34_sl_130` | published (`fork_fox_34_sl.md`) |
| `fox34_130` | `Fox Float 34 130mm` | `Travel130` | `SteererTapered` | `544 / 44`, `544 / 51` | `fox34_130-544x44`, `fox34_130-544x51` | published (`fork_fox_34.md`) |
| `fox36_sl_130` | `Fox 36 SL 130mm` | `Travel130` | `SteererTapered` | `546 / 44` | `fox36_sl_130` | published (`fork_fox_36_sl.md`) |

### 2. Per-Frame Fork Lists & Placements (`19 frames`, `119 bases`, `1100 placements`)

`BasisFork` and `BasisLowerCup` stay untouched on every frame (`Q128`).

| Frame ID | `Sizes` | `Frame.Forks` (`[]Fork`) | Rows / Size | Total Placements | Delta vs Pre-16c |
| --- | ---: | --- | ---: | ---: | --- |
| `binary-havok` | 5 | `rigid_485`, `sid_100`, `fox34_120` | 12 | 60 | `rigid_485` (+`52`), `sid_100` (+`51`, 15/20%), `fox34_120` (`534/44`, `534/51`) |
| `binary-kinetik` | 6 | `fox34_120`, `sid_100`, `rigid_500` | 12 | 72 | `fox34_120` (`534/44`, `534/51`), `sid_100` (+`51`, 15/20%), `rigid_500` (+`51`) |
| `esker-hayduke` | 4 | `pike_120`, `fox34_130`, `enve_mountain_boost`, `lithic_carbon_mountain` | 11 | 44 | Drops `rigid_495`; adds `enve_mountain_boost` & `lithic_carbon_mountain`; `fox34_130` (`544/44`, `544/51`, 20/25%) |
| `fairlight-holt-2` | 4 | `pike_120`, `sid_100` | 7 | 28 | `sid_100` (+`51`, 15/20%) |
| `moots-mountaineer` | 4 | `pike_120` | 3 | 12 | Unchanged (`531/44`, 15/20/25%) |
| `otso-fenrir-stainless` | 5 | `lithic_carbon_mountain`, `sid_100`, `fox34_120` | 12 | 60 | `lithic` -> `495/44`, `498/51.5` (`BasisFork` stays `495/54`); `sid_100` (+`51`, 15/20%); `fox34_120` (`534/44`, `534/51`) |
| `revel-el-jefe` | 4 | `sid_120`, `sid_100`, `fox34_130`, `rigid_510` | 15 | 60 | `sid_120` (+`534/44`), `sid_100` (+`51`, 15/20%), `fox34_130` (`544/44`, `544/51`, 20/25%) |
| `ritchey-ultra` | 4 | `fox34_120`, `rigid_500` | 8 | 32 | `fox34_120` (`534/44`, `534/51`), `rigid_500` (+`51`) |
| `salsa-cutthroat` | 5 | `firestarter_deluxe_483`, `sid_100` | 5 | 25 | `sid_100` (+`44`, 15/20%) |
| `salsa-fargo` | 5 | `firestarter_deluxe_483`, `sid_100` | 5 | 25 | `sid_100` (+`44`, 15/20%) |
| `seven-custom` | 1 | `enve_mountain_boost`, `sid_100`, `fox34_120` | 12 | 12 | `enve_mountain_boost` (+`490/44`), `sid_100` (+`51`, 15/20%), `fox34_120` (`534/44`, `534/51`) |
| `seven-expedition-926d` | 24 | `enve_mountain_boost`, `lithic_carbon_mountain`, `fox34_120` | 10 | 240 | Drops `enve_mountain_472` & `sid_100`; adds `lithic_carbon_mountain` & `fox34_120` (`BasisFork` stays `472/55`) |
| `seven-expedition-926f` | 24 | `enve_mountain_boost`, `lithic_carbon_mountain`, `fox34_120` | 10 | 240 | Drops `enve_mountain_472` & `sid_100`; adds `enve_mountain_boost` & `lithic_carbon_mountain` (`BasisFork` stays `472/55`) |
| `sklar-pbj` | 5 | `sklar_steel_485`, `sid_100`, `fox34_120` | 11 | 55 | `sid_100` (+`51`, 15/20%), `fox34_120` (`534/44`, `534/51`) |
| `sour-pasta-party` | 4 | `fox34_120`, `sid_100`, `rigid_490` | 11 | 44 | `fox34_120` (`534/44`, `534/51`), `sid_100` (+`51`, 15/20%) |
| `surly-ecr` | 3 | `surly_ecr_468` | 1 | 3 | Unchanged (`468/43`) |
| `surly-krampus` | 4 | `surly_krampus_483`, `sid_100`, `fox34_120` | 11 | 44 | `sid_100` (+`51`, 15/20%), `fox34_120` (`534/44`, `534/51`) |
| `tumbleweed-sunliner` | 4 | `sunliner_440` | 1 | 4 | Unchanged (`440/55`) |
| `turner-venn-ti` | 4 | `fox34_130`, `fox34_120` | 10 | 40 | Drops `pike_140`; `fox34_130` (`544/44`, `544/51`, 20/25%), `fox34_120` (`534/44`, `534/51`) (`BasisFork` stays `541/44`) |
| **Total** | **119** | | | **1100** | |

---

## Commit Split (2 Atomic Green Commits)

Every commit builds and passes `make pre-commit`.

### Commit 1: Closed `ForkTravel` table and Q132 sag scoring (`assembly` + callers)

1. **`internal/reboot/assembly/frameset.go`:**
   - Add `ForkTravel` enum (`TravelRigid = 0`, `Travel100 = 100`, `Travel120 = 120`, `Travel130 = 130`) and exhaustive methods `Valid`, `Mm`, `IsSuspension`, `SagRange`, `SagPcts`, `DefaultSagPct`, `String`.
   - Replace `ForkSpec.TravelMm float64` with `ForkSpec.Travel ForkTravel`.
   - Update `ForkSetup.EffectiveA2cMm()` (`StaticA2cMm - Travel.Mm()*(SagPct/100.0)`) and `ForkSetup.Validate()` (reject `!Spec.Travel.Valid()` with plain error; reject `SteererBasis` with non-rigid travel; keep `InputForkSag` for non-zero sag on `TravelRigid` and out-of-`[0, 100)` sag on suspension).
2. **`internal/reboot/assembly/scoring.go`:**
   - Replace `scoreForkSag` with the Q132 curve using `setup.Fork.Spec.Travel.SagRange()`.
3. **`internal/reboot/catalog/{forks,catalog,frame}.go` & deleted Q129 forks:**
   - Replace `Fork.TravelMm float64` with `Fork.Travel assembly.ForkTravel`.
   - Delete `sid_110`, `pike_140`, and `fox36_150` from `forks.go`, drop `pike_140` from `frame_turner_venn_ti.go`, remove `catalog.SuspensionSagPcts()` and `catalog.DefaultSuspensionSagPct` from `catalog.go`, and remove the fork assertions from `TestFramesMatchLegacy` / delete `TestForksMatchLegacy` so Commit 1 stays green. Keep MY25/S5/S6 A2C and `FrameFork -> ForkOption` variant moves for Commit 2.
4. **`internal/reboot/compare/compare.go` & `internal/reboot/query/{bike,compare}.go` & `internal/reboot/web/{bike_page,compare_page}.go`:**
   - Update `.TravelMm` references to `.Travel` (`Travel == assembly.TravelRigid`, `.Travel.IsSuspension()`, `.Travel.Mm()`, `.Travel.SagPcts()`, `.Travel.DefaultSagPct()`).
   - Remove `compare.Request.SuspensionSagPcts` and its `len == 0` validation error; validate every `fk.Spec.Travel.Valid()` in `compare.Request` validation instead.
   - Replace `catalog.DefaultSuspensionSagPct` in `query/bike.go` with `ff.Fork.Travel.DefaultSagPct()`.
   - Update `assembly`, `catalog`, `compare`, `query`, `web`, and `cmd/bikegeo-reboot` tests for Commit 1 (`ForkTravel` literals, Q132 sag curve, `[15, 20]` on 100 mm, `[20, 25]` on 130 mm, Q134 `25%` default sag on 120/130 mm).

#### Commit 1 & Shared `assembly` Negative Validation + Probed Expectations

- **Negative validation test cases (`assembly/frameset_test.go`):**
  - `Travel: -10` -> plain error (`""` field)
  - `Travel: 110`, `Travel: 140`, `Travel: 150` -> plain error (`""` field), confirming closed travel table rejection
  - `Steerer: SteererBasis, Travel: Travel100` -> plain error
  - `Travel: TravelRigid, SagPct: 20` -> `InputForkSag`
  - `compare/compare_test.go`: reject frame containing fork with `Travel: assembly.ForkTravel(110)` or `assembly.ForkTravel(-5)` (replacing removed `"no sag steps"` test).
- **Q132 Sag Curve Unit Test (`assembly/scoring_test.go`), verified by `TestScratchSlice16cAssembly`:**

| `Travel` | `0%` | `5%` | `10%` | `15%` | `17.5%` | `18%` | `20%` | `22%` | `22.5%` | `25%` | `30%` | `35%` | `50%` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `TravelRigid` | `100` | — | — | — | — | — | — | — | — | — | — | — | — |
| `Travel100` | `0` | `0` | `45` | `90` | `95` | `96` | `100` | `80` | `75` | `50` | `0` | `0` | `0` |
| `Travel120` | `0` | `0` | `45` | `90` | `92.5` | `93` | `95` | `97` | `97.5` | `100` | `50` | `0` | `0` |
| `Travel130` | `0` | `0` | `0` | `45` | `67.5` | `72` | `90` | `94` | `95` | `100` | `50` | `0` | `0` |

- **`assembly/build_fitting_test.go` (`TestFitBuildsChoosesAmongForks` & `TestFitBuildsRefitsSaddlePerFork`):**
  When updating `fenrirForkOptions()` for the 100 mm travel table (`sid` at `{15, 20}` -> 6 total fork options, `420` builds evaluated with `70` builds per fork, and `fox34` updated to `534/44` in Commit 2 or tested at `534/44`):
  - `TestFitBuildsChoosesAmongForks` (`6` fork configs, `420` evaluated, `HandGrip` target only):
    - Rank 1 (`sid 20%`, `forkIdx=2`): `tier=fits_neither`, `total=61.179441`, `stem=-20/120`, `spacers=0`, `gripR=1.616431`
    - Rank 4 (`lithic 0%`, `forkIdx=0`): `tier=fits_neither`, `total=56.854583`, `stem=-30/120`, `spacers=0`, `gripR=1.764743`
    - Rank 5 (`sid 15%`, `forkIdx=1`): `tier=fits_neither`, `total=56.326779`, `stem=-30/120`, `spacers=0`, `gripR=1.767563`
  - `TestFitBuildsRefitsSaddlePerFork` (`6` fork configs, `420` evaluated, both targets):
    - Rank 1 (`sid 20%`, `forkIdx=2`): `tier=fits_neither`, `total=58.531310`, `stem=-20/120`, `spacers=0`, `gripR=1.616431`, `post=198.128`, `rail=15.0`
    - Rank 4 (`lithic 0%`, `forkIdx=0`): `tier=fits_neither`, `total=55.768206`, `stem=-30/120`, `spacers=0`, `gripR=1.764743`, `post=197.388`, `rail=15.0`
    - Note `TestFitBuildsReferenceBuilds` on rigid `lithic` (`495/54`) stays unchanged (`total=54.312823` etc. because `TravelRigid` keeps sag score `100`).

---

### Commit 2: Fork variants, FOX/S5/S6 specs, Q131 IDs, and frame fork-list updates (`catalog`, `compare`, `query`, `web`, `cmd/bikegeo-reboot`)

1. **`internal/reboot/catalog/`:**
   - Replace `FrameFork` and `Frame.Forks []FrameFork` with `ForkVariant`, `Fork.Variants []ForkVariant`, `(f Fork) IsMultiVariant()`, `(f Fork) VariantID(v ForkVariant)`, `ForkOption`, `Frame.Forks []Fork`, `(f Frame) ForkOptions() []ForkOption`, `(f Frame) LookupFork(id string) ([]ForkOption, bool)`, and `catalog.ForkByID(id string) (Fork, bool)`.
   - Apply all Q133 catalog fork specs, rename `enve_mountain_boost`, delete `enve_mountain_472` & `rigid_495`, and update `frame_seven_expedition_926d.go`, `frame_seven_expedition_926f.go`, `frame_esker_hayduke.go`, and the 15 other `frame_*.go` files to `[]Fork`. Note in `parts_test.go` (`len(catalog.Forks()) == 21`) and `frames_test.go` (`placements == 1100`, `bases == 119`).
2. **`internal/reboot/compare/`:**
   - Add `ModelID string` to `compare.Fork` (when `ModelID == ""`, treat `ID` as the model ID).
   - Expand bare multi-variant `Selection.ForkID` matching `fk.ModelID` across all variants of that model (both in open-sag expansion and with explicit `Selection.SagPct`), while always keeping concrete variant IDs on `Row.ForkID`.
   - Update `compare_test.go` fixture (`26` rows, `1820` evaluated; `AllBarOptions` `3640` evaluated; bare-model selection tests; explicit `sag=22` on `fox34_120-534x44`).
3. **`internal/reboot/query/`:**
   - Update `CatalogFrames()` (`Fork{ID: o.ID(), ModelID: o.Fork.ID, Spec: o.Spec()}`), `DefaultBike()` (`lithic_carbon_mountain-495x44`), `ParseCompare` (uses `frame.LookupFork`), `ParseBike` (requires `len == 1` match or `"basis"`; bare multi-variant model ID returns `InputError{Field: "fork"}`), and `ForkOptions` (`[]BikeForkOption` over `frame.ForkOptions()` × `Travel.SagPcts()`).
   - Update `query/compare_test.go` and `query/bike_test.go` with probed values below (including updating unknown/absent fork negative tests since `fox36_150` is deleted).
4. **`internal/reboot/web/` & `cmd/bikegeo-reboot/`:**
   - Update `compare_page.go`: `<select name="fork">` renders variant options + `(all variants)` for multi-variant forks; `<select name="sag">` uses selected fork travel's `SagPcts()`; `forkCellName` displays `(<a2c>/<rake>)` when multi-variant.
   - Update `bike_page.go`: `<select name="fork">` options over `frame.ForkOptions()` using `query.FormatFloat` so fractional dims (`530.5`, `51.5`) never round to `.0`; `forkNameAndTravel` resolves concrete variant IDs cleanly.
   - Update `cmd/bikegeo-reboot/main.go` help examples and `main_test.go` + `web/{compare,web}_test.go` assertions to the exact probed values below.

#### Commit 2 Probed Test Expectations (verified by temporary probes, 2026-11-22)

1. **`internal/reboot/compare/compare_test.go` (Scenario B multi-variant fixture: `RowCount = 26`, `Evaluated = 1820`):**
   - Frames: `surly-krampus` (`L`, `XL` with `surly_krampus_483`, `fox34_120-534x44`, `fox34_120-534x51`) and `otso-fenrir-stainless` (`L`, `XL` with `lithic_carbon_mountain-495x44`, `lithic_carbon_mountain-498x51.5`, `sid_100-506x44`, `sid_100-506x51`).
   - Top 12 ranked rows (`t1WantRows`, 26 total; verify full list in test from `CompareProbe` Scenario B):
     - `#1`: `surly-krampus`, `XL`, `fox34_120-534x44`, `15%`, `fits_all`, `total=82.940355`, `gripR=0.309902`, `110/-17/5`
     - `#2`: `surly-krampus`, `XL`, `fox34_120-534x44`, `20%`, `fits_all`, `total=82.603558`, `gripR=0.227208`, `110/-17/10`
     - `#3`: `surly-krampus`, `XL`, `fox34_120-534x51`, `15%`, `fits_all`, `total=82.036517`, `gripR=0.241906`, `110/-17/5`
     - `#4`: `surly-krampus`, `XL`, `fox34_120-534x51`, `20%`, `fits_all`, `total=81.324723`, `gripR=0.121191`, `110/-17/10`
     - `#5`: `surly-krampus`, `XL`, `fox34_120-534x44`, `25%`, `fits_all`, `total=81.281365`, `gripR=0.186563`, `110/-17/15`
     - `#6`: `surly-krampus`, `XL`, `fox34_120-534x51`, `25%`, `fits_all`, `total=79.863568`, `gripR=0.007809`, `110/-17/15`
     - `#10`: `surly-krampus`, `L`, `fox34_120-534x44`, `20%`, `fits_brp`, `total=78.131937`, `gripR=1.015032`, `120/-17/5`
     - `#13`: `surly-krampus`, `XL`, `surly_krampus_483`, `0%`, `fits_grip`, `total=76.852325`, `gripR=0.741170`, `90/-17/15`
     - `#15`: `otso-fenrir-stainless`, `XL`, `sid_100-506x44`, `20%`, `fits_grip`, `total=68.479876`, `gripR=0.578741`, `120/-30/0`
     - `#22`: `otso-fenrir-stainless`, `L`, `sid_100-506x44`, `20%`, `fits_neither`, `total=58.531310`, `gripR=1.616431`, `120/-20/0`
     - `#26`: `otso-fenrir-stainless`, `L`, `lithic_carbon_mountain-495x44`, `0%`, `fits_neither`, `total=54.271845`, `gripR=1.897639`, `120/-30/0`
   - `TestRankAllHandlebars` (`AllBarOptions`): `RowCount = 26`, `Evaluated = 3640`; `#1 = {surly-krampus XL fox34_120-534x44 25 fits_all 84.089299 0.647636 90/-17/0 barIdx=1}`.
   - Explicit `Selection` with `krampus L`, `fox34_120-534x44`, `sag=22`: `total=79.359645`, `gripR=0.934747`, `120/-10/0`.

2. **Full 19-frame Default Compare (`query/compare_test.go`, `web/compare_test.go`):**
   `RowCount = 1100`, `Evaluated = 823900`, `Shown = 19`:
   - `#1`: `surly-krampus`, `XL`, `fox34_120-534x44`, `sag=25`, `fits_all`, `total=85.120463740`, `gripR=0.945603917`, `sagAttr=100.0`, `100/-7/0`, `rail=15.00`
   - `#2`: `seven-custom`, `Custom`, `fox34_120-534x44`, `sag=25`, `fits_brp`, `total=53.126792179`, `gripR=2.092042057`, `120/-20/0`
   - `#3`: `seven-expedition-926f`, `S 63cm`, `enve_mountain_boost-492x52`, `sag=0`, `fits_brp`, `total=55.726426456`, `gripR=2.525570555`, `120/-30/0`
   - `#4`: `salsa-fargo`, `XL`, `sid_100-506x51`, `sag=20`, `fits_brp`, `total=54.488981918`, `gripR=4.332846698`, `120/-30/0`
   - `#5`: `seven-expedition-926d`, `S 62cm`, `enve_mountain_boost-492x52`, `sag=0`, `fits_brp`, `total=55.791926019`, `gripR=4.363209681`, `120/-30/0`
   - `#6`: `esker-hayduke`, `S3`, `enve_mountain_boost-492x52`, `sag=0`, `fits_grip`, `total=81.320323095`, `gripR=0.952602612`, `80/-7/0`
   - `#7`: `sour-pasta-party`, `L`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=80.881655493`, `gripR=0.899510156`, `80/-7/10`
   - `#8`: `fairlight-holt-2`, `L`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=80.661119747`, `gripR=0.644100460`, `80/-6/5`
   - `#9`: `revel-el-jefe`, `L`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=79.856548523`, `gripR=0.696045832`, `80/-7/15`
   - `#10`: `ritchey-ultra`, `XL`, `rigid_500-500x44`, `sag=0`, `fits_grip`, `total=78.742795187`, `gripR=0.872697532`, `90/-7/0`
   - `#11`: `sklar-pbj`, `L`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=78.097626656`, `gripR=0.849757340`, `90/-10/0`
   - `#12`: `binary-kinetik`, `L`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=76.854019372`, `gripR=0.720921710`, `90/-17/0`
   - `#13`: `moots-mountaineer`, `XL`, `pike_120`, `sag=25`, `fits_grip`, `total=76.837585720`, `gripR=0.478035986`, `90/-17/0`
   - `#14`: `turner-venn-ti`, `L`, `fox34_120-534x44`, `sag=25`, `fits_grip`, `total=76.528181131`, `gripR=0.916424655`, `90/-17/0`
   - `#15`: `tumbleweed-sunliner`, `L`, `sunliner_440`, `sag=0`, `fits_grip`, `total=72.632039185`, `gripR=0.509269123`, `120/-7/0`
   - `#16`: `otso-fenrir-stainless`, `XL`, `sid_100-506x44`, `sag=20`, `fits_grip`, `total=68.479876012`, `gripR=0.578741059`, `120/-30/0`
   - `#17`: `surly-ecr`, `XL`, `surly_ecr_468`, `sag=0`, `fits_neither`, `total=69.859198518`, `gripR=1.006941473`, `120/-30/0`
   - `#18`: `binary-havok`, `XL`, `rigid_485-485x52`, `sag=0`, `fits_neither`, `total=50.150557529`, `gripR=3.227810500`, `120/-30/0`
   - `#19`: `salsa-cutthroat`, `60cm`, `firestarter_deluxe_483`, `sag=0`, `fits_neither`, `total=50.197172824`, `gripR=3.390669575`, `120/-30/0`

3. **Single-Row Compare & CLI Probes (`query/compare_test.go`, `web/compare_test.go`, `cmd/bikegeo-reboot/main_test.go`):**
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=20`: `RowCount=1, Evaluated=749, fits_all, total=84.690417237, gripR=0.725092414, sagScore=95.0, stem=110/-7/0, post_insert=163.13014276565696, rail=15.0`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=25`: `RowCount=1, Evaluated=749, fits_all, total=85.120463740, gripR=0.945603917, sagScore=100.0, stem=100/-7/0, post_insert=164.29156757597764, rail=15.0`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&bar_options=all`: `RowCount=1, Evaluated=11235, fits_all, total=87.120779384, gripR=0.579375126, bar=sqlab311_20_16_15, stem=100/-7/0`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=25&bar_options=all`: `RowCount=1, Evaluated=11235, fits_all, total=86.509786164, gripR=0.945343796, bar=sqlab311_flx_16_30, stem=90/-7/0`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=18`: `RowCount=1, fits_all, total=84.564040849, sagScore=93.0, stem=110/-7/0`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2`: `Evaluated=749, fits_all, total=91.139658843, gripR=0.284643315, stem=80/-30/0, rail=4.29`
   - `surly-krampus&size=XL&fork=fox34_120-534x44&sag=20&brp_x=-210&brp_y=720&grip_x=470&grip_y=730`: `Evaluated=749, fits_all, total=93.656414150, gripR=0.949174019, stem=90/-7/0, rail=5.48`
   - `binary-havok&size=L&fork=rigid_485-485x45`: `Evaluated=749, fits_neither, total=49.866243596, gripR=3.875593862, stem=120/-30/0, rail=15.0`
   - `binary-havok&size=L&fork=rigid_485-485x52`: `Evaluated=749, fits_neither, total=49.155334622, gripR=3.718610833, stem=120/-30/0, rail=15.0`

4. **Default `/bike` (`otso-fenrir-stainless` `L`, `lithic_carbon_mountain-495x44` — `query/bike_test.go`, `web/web_test.go`, `cmd/bikegeo-reboot/main_test.go`):**
   - **Geometry on default `/bike` (`495/44`):**
     `WB=1152.9138`, `RC=445.0808`, `LeveledReach=419.8159`, `LeveledStack=647.3814`, `HTA=67.5227°`, `STA=74.5227°`, `BbDrop=66.3554`, `Trail=106.2977`, `Pitch=0.7273°`, `BRP=(-157.0249, 682.6822)`, `HandGrip=(386.8794, 756.6302)`.
   - **SVG `viewBox` & `UnitsPerPx` on default `/bike` (`495/44`):**
     - `bike`: `"-912.08 -851.63 2086.91 1257.27"` (`unitsPerPx=2.89849144`)
     - `seat`: `"-265.60 -703.99 137.14 82.62"` (`unitsPerPx=0.19047619`)
     - `stem`: `"298.49 -769.98 240.24 144.74"` (`unitsPerPx=0.33367051`)
   - **All 15 reference handlebar `HandGrip`s on default `/bike` (`495/44`):**
     - `persuader20_800`: `(386.879379, 756.630227)`
     - `persuader20_840`: `(379.599974, 756.630227)`
     - `sqlab3ox_16_15`: `(385.737110, 744.111697)`
     - `sqlab3ox_16_30`: `(389.608172, 758.167685)`
     - `sqlab3ox_16_45`: `(393.479235, 772.223673)`
     - `sqlab3ox_12_15`: `(404.395599, 744.111697)`
     - `sqlab3ox_12_30`: `(407.265113, 758.167685)`
     - `sqlab3ox_12_45`: `(410.134626, 772.223673)`
     - `sqlab311_20_16_15`: `(391.472017, 746.778729)`
     - `sqlab311_20_16_25`: `(394.052726, 755.991331)`
     - `sqlab311_20_16_50`: `(400.504497, 779.022836)`
     - `sqlab311_flx_16_15`: `(391.472017, 742.713160)`
     - `sqlab311_flx_16_30`: `(395.343080, 756.769148)`
     - `sqlab311_flx_12_15`: `(408.646730, 742.713160)`
     - `sqlab311_flx_12_30`: `(411.516244, 756.769148)`
   - **Fitting & Setback (`495/44`):**
     - `ApplyFitting` (default BRP+Grip targets): `outcome=limited`, `limited=[saddle rail offset, stem spacers, stem length]`, `insert=143.437621`, `rail=15.0`, `spacers=0.0`, `stemLength=130.0`, `brpMiss=(-38.5524, 0.0)`, `gripMiss=(19.3490, -32.0041)`
     - `FitPostSetback` default post + default BRP: `outcome=reached`, `insert=129.1468`
     - `FitPostSetback` `setback_25` + default BRP: `outcome=reached`, `insert=179.1468`
     - `FitPostSetback` default post + `brp=(-150, 713.1)`: `outcome=limited` (`seatpost setback`), `insert=147.5599`, `brp=(-152.7827, 712.3295)`
     - High BRP (`brp_y=945`): `outcome=limited` (`seatpost insertion, ...`), `insert=100.0`, `rail=15.0`
   - **Build Ranking (`495/44`):**
     - Default (`brp=on`, `749` evaluated): `#1` `120/-30/0`, `r=1.8976`, `total=54.271845` (`54.3`); `#2` `120/-30/5`, `r=2.0551`, `total=52.736437` (`52.7`); `#3` `120/-20/0`, `r=2.1859`, `total=52.736437` (`52.7`).
     - `brp=off` (`749` evaluated): `#1` `120/-30/0`, `r=1.8976`, `total=54.512454` (`54.5`); `#2` `120/-30/5`, `r=2.0551`, `total=52.109206` (`52.1`); `#3` `120/-20/0`, `r=2.1859`, `total=52.109206` (`52.1`).
     - `bar_options=all` (`brp=on`, `11235` evaluated): `#1` `sqlab311_flx_12_15`, `120/-10/0`, `r=0.8173` (`0.82`), `total=67.277031` (`67.3`).
     - `bar_options=all` (`brp=off`, `11235` evaluated): `#1` `sqlab311_flx_12_15`, `120/-10/0`, `r=0.8173`, `total=74.8684` (`74.9`).
     - `fork_options=all`: **`8988` evaluated** (`12` fork×variant×sag options × `749`).
       - `brp=on` `#1`: `sid_100-506x51` at `sag=20`, `120/-20/0`, `r=1.4461`, `total=60.642370` (`60.6`); `#3`: `sid_100-506x44` at `sag=20`, `120/-20/0`, `r=1.6164`, `total=58.531310` (`58.5`).
       - `brp=off` `#1`: `sid_100-506x51` at `sag=20`, `120/-20/0`, `r=1.4461`, `total=64.483710` (`64.5`).
   - **Specific Fork Queries (`495/44` base):**
     - `bar=sqlab3ox_12_15&bar_roll=5`: `total=49.9587` (`50.0`), `r=7.5518`.
     - `sid_100-506x44&sag=20` (default build / ranking): placed `total=55.5556` (`55.6`), `r=8.8315`; ranking `#1` `120/-20/0`, `total=58.5313` (`58.5`), `r=1.6164`.
     - `sid_100-506x44&sag=25` (explicit sag `25`): `Fork sag` score = `50.0`; `120/-17/0` placed (`brp=off` -> `total=61.7`, `r=1.54`; `brp=on` -> `total=58.3`, `r=1.54`).
     - `fox34_120-534x44&sag=20`: `Fork sag` = `95.0`; placed `total=53.8516`; ranking `#1` `120/-30/0`, `total=51.0738`, `r=2.5819`.
     - `fox34_120-534x44&sag=25`: `Fork sag` = `100.0`; placed `total=54.8411`; ranking `#1` `120/-30/0`, `total=52.0633`, `r=2.2804`.
     - `esker-hayduke` size `S3`, `pike_120` at default `sag=25` (`Q134`): `wb=1202.2`, `rc=449.5`, `reach=468.4` (`sag=20`: `wb=1204.5`, `rc=449.8`, `reach=465.5`).
   - Note: `TestBikeSlidingDropoutsAndUnpublishedWheelbase` (`binary-havok` `L`) defaults to `rigid_485-485x45` (the first variant, matching pre-16c `485/45`!), so `wheelbase=1126.5 mm`, `rear-center=454.8 mm`, `leveled-reach=396.9 mm`, and `basis` numbers stay identical.

---

## Rules That Bite

1. **`make boundary` banned substrings:** Check every new/edited `.go`, `.md`, `.html`, and binary file under `internal/reboot/` and docs against `chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, and `cad` (outside `*_parity_test.go`) — even as substrings inside English words.
2. **`make exhaustive` on `ForkTravel`:** Every `switch` over `ForkTravel` (even with `default`) must explicitly name all 4 constants (`TravelRigid`, `Travel100`, `Travel120`, `Travel130`).
3. **`query.FormatFloat` on `.5` dimensions:** Always format variant A2C and Rake with `strconv.FormatFloat(..., 'f', -1, 64)` / `query.FormatFloat` so `498/51.5` and `530.5/44` round-trip without `.0` or rounded `.5` truncation in both IDs and UI labels.
4. **Never nudge published data:** FOX User Specs, S5 ENVE, S6 Lithic, and all frame `BasisFork` fields must remain verbatim as published/documented.
5. **Implementation handoff:** After `SLICE16c.md` is reviewed and implemented across Commits 1 and 2, run `make pre-commit` until clean with zero diffs.

---

## Verification

- `make pre-commit` (after every edit batch and before committing Commit 1 and Commit 2).
- `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo-reboot` (zero external runtime dependencies).
- Smoke-test CLI after Commit 2:
  - `go run ./cmd/bikegeo-reboot compare 'frame=surly-krampus&size=XL&fork=fox34_120'` (returns 6 rows covering both `534/44` and `534/51` at `15, 20, 25%`)
  - `go run ./cmd/bikegeo-reboot bike 'frame=otso-fenrir-stainless&size=L&fork=fox34_120-534x44'` (defaults `sag` to `25`)
  - Verify `bike 'fork=fox34_120'` exits non-zero / rejects bare multi-variant ID (`Q131`).

---

## Out of Scope

- **Slice 16d:** Fork compatibility classes, rigid-in-suspension-space mapping, typical lower-cup catalog, steerer/bore matching, and open fork choice across unlisted frames (plus `fox34_sl_*` / `fox36_sl_*` frame availability).
- **Slice 16e:** Re-ingesting `otso-fenrir-stainless` from Otso's published chart and promoting the basis search into a shared test helper.
