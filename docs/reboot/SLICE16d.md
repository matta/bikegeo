# Slice 16d: Open Fork Choice

Status: Planned 2026-11-22 (Q128–Q130, Q135–Q139; ready for implementation)

## Goal

Open `/`, `/bike`, and the CLI (`cmd/bikegeo-reboot`) from each frame's hand-listed forks to every compatible catalog fork (`Q128`–`Q130`, `Q135`–`Q139`):

1. Model a canonical lower-cup catalog (`lower_cups.go`, `lower_cups.md`) using Cane Creek 40-Series typical lower cups (`EC34: 12 mm`, `EC44: 12 mm`, `ZS44: 4 mm`, `ZS56: 4 mm`, `IS42: 1 mm`, `IS52: 1 mm`), update the four mismatched `DefaultLowerCup.StackMm` values (`turner-venn-ti`, `revel-el-jefe`, `ritchey-ultra`, `salsa-cutthroat`), relax `frames_parity_test.go` to headset bore + standard only, and resolve each compatible fork's cup per `(LowerBore, defaultStd, Steerer)`.
2. Classify the commercial rigid forks into four hand-assigned `RigidClass` values (`440`, `470`, `485`, `495`) with one-way suspension stand-ins (`100 mm → 485`, `120 mm → 495`, `130 mm → none`), record manufacturer provenance in `forks_rigid.md`, and drop all 5 illustrative `rigid_*` placeholder forks (`rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, `rigid_510`).
3. Expose compatible forks across `catalog.Frame`, `compare.Fork`/`Rank`, `query` (`CatalogFrames`, `parseBikeFork`, `Bike.ForkOptions`), the CLI, and web `<select>` controls (`<optgroup label="Listed Forks">` and `<optgroup label="Other Compatible Forks">`).

No geometry formulas, scoring weights, or legacy `internal/` behavior changes in this slice beyond the 4 lower-cup stack updates and removing placeholder `rigid_*` forks.

## Confirmed Rulings Governing This Slice

- **Q128–Q130 (`DESIGN.md`):** frame fork compatibility comes from fork class (`ForkTravel` or `RigidClass`), steerer-to-bore compatibility with resolved typical lower cup, and one-way rigid stand-ins; listed forks stay first; each frame's `BasisFork` and `BasisLowerCup` stay untouched.
- **Q135 (`DESIGN.md`):** 4 rigid classes:
  - `RigidClass440` (`"440"`): `sunliner_440` — stands in for none.
  - `RigidClass470` (`"470"`): `surly_ecr_468` — stands in for none.
  - `RigidClass485` (`"485"`): `firestarter_deluxe_483`, `surly_krampus_483`, `sklar_steel_485` — stands in for `Travel100` only.
  - `RigidClass495` (`"495"`): `enve_mountain_boost`, `lithic_carbon_mountain` — stands in for `Travel120` only (`RigidClass510` dropped with `rigid_510`; `Travel130` unlocks no rigid class).
  - Unidirectional: a listed rigid fork unlocks only its own `RigidClass` (never suspension travels); a listed suspension fork unlocks its own `ForkTravel` row plus `standInRigidClasses(travel)`.
- **Q136 (`DESIGN.md`):** Cane Creek 40-Series typical lower cups (`EC34: 12`, `EC44: 12`, `ZS44: 4`, `ZS56: 4`, `IS42: 1`, `IS52: 1`; `/30` and `/40` conversion rings share the same stack). Update `DefaultLowerCup.StackMm` on `turner-venn-ti` (`ZS56` `2 → 4 mm`) and `revel-el-jefe`, `ritchey-ultra`, `salsa-cutthroat` (`IS52` `0 → 1 mm`). Relax `frames_parity_test.go` to check `LowerBore` and `DefaultLowerCup.Standard` only. On `Bore44Pressed`, `SteererStraight118` preserves `ZS44` (`4 mm`) if the frame defaults to `ZS44` and `EC44` (`12 mm`) otherwise; `SteererTapered` uses `EC44` (`12 mm`). `Bore34Pressed` and `Bore42Integrated` reject `SteererTapered`.
- **Q137 (`DESIGN.md`):** drop `rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, `rigid_510` from `forks.go` and from `binary-havok`, `binary-kinetik`, `revel-el-jefe`, `ritchey-ultra`, `sour-pasta-party` (`16` commercial fork models remain: `7` rigid, `9` suspension). Keep `seven-expedition-926d` and `seven-expedition-926f` at `[enve_mountain_boost, lithic_carbon_mountain, fox34_120]`.
- **Q138 (`DESIGN.md`):** record provenance in `internal/reboot/catalog/lower_cups.md` and `internal/reboot/catalog/forks_rigid.md`.
- **Q139 (`DESIGN.md`):** `<select>` controls on `/` and `/bike` group options into `<optgroup label="Listed Forks">` and `<optgroup label="Other Compatible Forks">`, omitting `"Other Compatible Forks"` when empty. `/bike` keeps `<optgroup label="Chart Basis Fork">` last; `/` keeps leading `Invalid: ...` and `Any fork` above the groups.

---

## Planner Decisions (`D1`–`D8`)

### D1. Typical lower-cup catalog (`internal/reboot/catalog/lower_cups.go`)

Create `internal/reboot/catalog/lower_cups.go`:

- `TypicalLowerCup(std assembly.LowerCupStandard) (assembly.LowerCupSetup, bool)`:
  Exhaustive `switch std` over all `assembly.LowerCupStandard` constants (to pass `make exhaustive`):
  - `assembly.LowerCupEC34` → `{Standard: assembly.LowerCupEC34, StackMm: 12}, true`
  - `assembly.LowerCupEC44` → `{Standard: assembly.LowerCupEC44, StackMm: 12}, true`
  - `assembly.LowerCupZS44` → `{Standard: assembly.LowerCupZS44, StackMm: 4}, true`
  - `assembly.LowerCupZS56` → `{Standard: assembly.LowerCupZS56, StackMm: 4}, true`
  - `assembly.LowerCupIS42` → `{Standard: assembly.LowerCupIS42, StackMm: 1}, true`
  - `assembly.LowerCupIS52` → `{Standard: assembly.LowerCupIS52, StackMm: 1}, true`
  - `assembly.LowerCupBareHeadTube` → `{}, false`
  - `default` → `{}, false`
- `LowerCupFor(bore assembly.LowerBore, defaultStd assembly.LowerCupStandard, steerer assembly.Steerer) (assembly.LowerCupSetup, bool)`:
  1. Exhaustive `switch steerer`: `SteererStraight118` and `SteererTapered` proceed; `SteererBasis` and `default` return `{}, false`.
  2. Exhaustive `switch bore`:
     - `assembly.Bore34Pressed`: if `steerer == assembly.SteererStraight118`, return `TypicalLowerCup(assembly.LowerCupEC34)`; else `{}, false`.
     - `assembly.Bore42Integrated`: if `steerer == assembly.SteererStraight118`, return `TypicalLowerCup(assembly.LowerCupIS42)`; else `{}, false`.
     - `assembly.Bore44Pressed`:
       - if `steerer == assembly.SteererTapered`, return `TypicalLowerCup(assembly.LowerCupEC44)`.
       - if `steerer == assembly.SteererStraight118` and `defaultStd == assembly.LowerCupZS44`, return `TypicalLowerCup(assembly.LowerCupZS44)`.
       - otherwise (`SteererStraight118` with `EC44` default or other default), return `TypicalLowerCup(assembly.LowerCupEC44)`.
     - `assembly.Bore56Pressed`: return `TypicalLowerCup(assembly.LowerCupZS56)`.
     - `assembly.Bore52Integrated`: return `TypicalLowerCup(assembly.LowerCupIS52)`.
     - `default`: return `{}, false`.

**Tests + Negative Tests (`internal/reboot/catalog/parts_test.go`):**
- `TestTypicalLowerCup`: assert all 6 headset standards return their exact stacks (`12, 12, 4, 4, 1, 1`) and `ok == true`; negative checks on `assembly.LowerCupBareHeadTube` and `assembly.LowerCupStandard("invalid")` returning `ok == false`.
- `TestLowerCupFor`: positive checks for every bore/steerer/defaultStd pairing (including `Bore44Pressed` + `Straight118` keeping `ZS44 4` when `defaultStd == LowerCupZS44` vs `EC44 12` when `LowerCupEC44`, and `Bore44Pressed` + `Tapered` switching `ZS44` default to `EC44 12`); **negative tests** rejecting `Bore34Pressed` + `SteererTapered`, `Bore42Integrated` + `SteererTapered`, `SteererBasis` on every bore, and unknown `LowerBore` / `Steerer`.

### D2. Rigid classes, stand-in table, and placeholder deletion (`internal/reboot/catalog/forks.go`)

1. Add closed enum `RigidClass` and pair invariant check:
   ```go
   type RigidClass string

   const (
       RigidClassNone RigidClass = ""
       RigidClass440  RigidClass = "440"
       RigidClass470  RigidClass = "470"
       RigidClass485  RigidClass = "485"
       RigidClass495  RigidClass = "495"
   )
   ```
   - `(c RigidClass) ValidFor(travel assembly.ForkTravel) bool` (exhaustive switches on both `travel` and `c`):
     - When `travel == assembly.TravelRigid`: `true` iff `c` is one of `RigidClass440`, `RigidClass470`, `RigidClass485`, `RigidClass495`.
     - When `travel` is `Travel100`, `Travel120`, or `Travel130`: `true` iff `c == RigidClassNone`.
     - Otherwise (`default`): `false`.
   - `standInRigidClasses(travel assembly.ForkTravel) []RigidClass` (exhaustive switch on `travel`):
     - `Travel100` → `[]RigidClass{RigidClass485}`
     - `Travel120` → `[]RigidClass{RigidClass495}`
     - `TravelRigid`, `Travel130`, `default` → `nil`
2. Add fields:
   - `Fork.RigidClass RigidClass`
   - `ForkOption.LowerCup assembly.LowerCupSetup`
3. Assign `RigidClass` on the 7 remaining rigid forks (see Reference Table below) and delete `rigid483`, `rigid485`, `rigid490`, `rigid500`, `rigid510` from `forks.go` and `allForks` (`16` models remain).

**Tests + Negative Tests (`internal/reboot/catalog/parts_test.go`):**
- `TestRigidClassValidFor`: positive pairs (`TravelRigid` + all 4 `RigidClass` values; `Travel100/120/130` + `RigidClassNone`); **negative tests** rejecting `(TravelRigid, RigidClassNone)`, `(Travel100, RigidClass485)`, `(TravelRigid, RigidClass("510"))`, and `(assembly.ForkTravel(-1), RigidClassNone)`.
- `TestForks`: assert `len(Forks()) == 16`, every fork passes `fk.RigidClass.ValidFor(fk.Travel)`, and **negative checks** that `ForkByID` returns `false` for all 5 deleted IDs (`rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, `rigid_510`).

### D3. Open fork compatibility on `catalog.Frame` (`internal/reboot/catalog/frame.go`)

Add and update methods on `catalog.Frame`:

1. `(f Frame) IsListedFork(modelID string) bool` — reports whether `modelID` is in `f.Forks`.
2. `(f Frame) CompatibleForks() []Fork`:
   - Initialize `seen := make(map[string]bool, len(f.Forks))`, `allowedRigid := make(map[RigidClass]bool)`, `allowedTravel := make(map[assembly.ForkTravel]bool)`.
   - Start `out` with `f.Forks` in declared order (keeping listed forks first, Q69/Q139), recording each `fk.ID` in `seen`, and recording unlocked classes:
     - If `fk.Travel == assembly.TravelRigid`: `allowedRigid[fk.RigidClass] = true` (does NOT unlock any suspension travel).
     - Else (`Travel100`, `Travel120`, `Travel130`): `allowedTravel[fk.Travel] = true`, and for each `rc` in `standInRigidClasses(fk.Travel)`, `allowedRigid[rc] = true`.
   - Iterate `Forks()` in catalog order; for each `fk` not in `seen`:
     - If `_, ok := LowerCupFor(f.LowerBore, f.DefaultLowerCup.Standard, fk.Steerer); !ok`, skip.
     - Match class: if `fk.Travel == assembly.TravelRigid`, check `allowedRigid[fk.RigidClass]`; else check `allowedTravel[fk.Travel]`.
     - If matched, append `fk` and mark `seen[fk.ID] = true`.
3. Update `(f Frame) ForkOptions() []ForkOption`:
   - Iterate `f.CompatibleForks()`; resolve `cup, ok := LowerCupFor(f.LowerBore, f.DefaultLowerCup.Standard, fk.Steerer)` (skip if `!ok`, which `frameProblems` forbids on real frames); append `{Fork: fk, Variant: v, LowerCup: cup}` for each `v` in `fk.Variants`.
4. `(f Frame) LookupFork(id string) ([]ForkOption, bool)`:
   - Already filters `f.ForkOptions()`, so once `ForkOptions()` opens to `CompatibleForks()`, `LookupFork` automatically resolves any compatible fork ID or variant ID with its `LowerCup`! Leave `LookupFork`'s matching logic intact (exact variant ID `opt.ID() == id`, or model ID `opt.Fork.ID == id`).

**Validation & Tests (`internal/reboot/catalog/frames_test.go`):**
- Strengthen `frameProblems(f catalog.Frame) []string`:
  - Check every size in `f.Sizes` has `sz.Frame.LowerBore == f.LowerBore && sz.Frame.DefaultLowerCup == f.DefaultLowerCup` (so size switches never change cup expectations within a frame).
  - Check every listed fork in `f.Forks` satisfies `fk.RigidClass.ValidFor(fk.Travel)`, resolves `_, ok := LowerCupFor(f.LowerBore, f.DefaultLowerCup.Standard, fk.Steerer)` (`ok == true`), and appears in `f.CompatibleForks()`.
  - Check `f.DefaultLowerCup` equals `TypicalLowerCup(f.DefaultLowerCup.Standard)` (matching standard and typical stack).
- **Negative tests in `TestFrameProblemsCatchMistakes`:** test each newly enforced rule individually (size with mismatched `LowerBore` or `DefaultLowerCup`; `DefaultLowerCup` with non-typical stack `0`; rigid fork missing `RigidClass`; listed tapered fork on `Bore34Pressed`).
- `TestForkListsPlace`: iterate `f.ForkOptions()` using `opt.LowerCup` (instead of `s.ChartFrame.DefaultLowerCup`) when calling `assembly.PlaceFrameset`! Assert `placements == 2989` and `bases == 119`.
- `TestCompatibleForksRules`: explicitly verify Q135/Q136/Q139 rules:
  - `salsa-cutthroat` (`IS52`, `Travel100` via `sid_100` + `RigidClass485` via `firestarter_deluxe_483`): unlocks `Travel100` (`sid_100`) and all `RigidClass485` (`firestarter_deluxe_483`, `surly_krampus_483`, `sklar_steel_485`), and does NOT unlock `Travel120` (`fox34_120`), `Travel130`, or `RigidClass495`.
  - `seven-expedition-926d` (`EC44`, lists `RigidClass495` + `Travel120` via `fox34_120`): unlocks `Travel120` forks and `RigidClass495` forks only (`7` models, `11` variants).
  - `surly-ecr` (`Bore34Pressed`, `RigidClass470` only) and `tumbleweed-sunliner` (`EC44`, `RigidClass440` only): unlock only their single listed fork (`1` model each; rigid lists never unlock suspension).
  - Listed-first order (`f.CompatibleForks()` starts with `f.Forks` in exact declared order).
  - Synthetic `Bore44Pressed` + `ZS44` test frame listing a `Travel100` fork: `surly_krampus_483` (`Straight118`) gets `{LowerCupZS44, 4}` while `sid_100` (`Tapered`) gets `{LowerCupEC44, 12}`!

### D4. Frame dataset & parity updates (`internal/reboot/catalog/frame_*.go`, `frames_parity_test.go`)

1. Update `DefaultLowerCup.StackMm` to Q136 typical Cane Creek 40 stacks on 4 frames (leave `BasisLowerCup`, `BasisFork`, geometry, and chart targets untouched):
   - `frame_turner_venn_ti.go`: `LowerCupZS56` `2 → 4`
   - `frame_revel_el_jefe.go`: `LowerCupIS52` `0 → 1`
   - `frame_ritchey_ultra.go`: `LowerCupIS52` `0 → 1`
   - `frame_salsa_cutthroat.go`: `LowerCupIS52` `0 → 1`
2. Drop the deleted `rigid_*` entries from `Forks` slices (and update adjacent fork-list prose comments only where naming deleted `rigid_*` placeholders; leave `BasisFork` untouched):
   - `frame_binary_havok.go`: drop `rigid485` → leaves `[sid100, fox34_120]`
   - `frame_binary_kinetik.go`: drop `rigid485` → leaves `[fox34_120, sid100]`
   - `frame_revel_el_jefe.go`: drop `rigid490` → leaves `[sid120, sid100, fox34_130]`
   - `frame_ritchey_ultra.go`: drop `rigid500` → leaves `[fox34_120]`
   - `frame_sour_pasta_party.go`: drop `rigid485` → leaves `[fox34_120, sid100]`
   (Keep `seven-expedition-926d` and `seven-expedition-926f` at Q133's `[enveMountainBoost, lithicCarbonMountain, fox34_120]`.)
3. `internal/reboot/catalog/frames_parity_test.go` (around line 177):
   - Relax the headset assertion from `if fr.LowerBore != bore || fr.DefaultLowerCup != cup` to checking `fr.LowerBore != bore || fr.DefaultLowerCup.Standard != cup.Standard` (`Q136`), so the 4 typical stack updates pass legacy parity without changing legacy code.
   - Check `TestCatalogChartConsistency` (`go test ./internal/reboot/catalog`) still passes cleanly (chart consistency uses `BasisLowerCup`, which is untouched).

### D5. Provenance records (`internal/reboot/catalog/lower_cups.md`, `forks_rigid.md`)

Remember `make boundary` scans ALL files under `internal/reboot` (including `.md` files!) case-insensitively for `chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, and `cad` (even inside words!). Avoid substrings like "CAD", "cascade", "cadence", "cost", "pricing", etc.

- `internal/reboot/catalog/lower_cups.md`: document the Cane Creek 40-Series typical lower-cup table (`EC34/30: 12 mm`, `EC44/30 & EC44/40: 12 mm`, `ZS44/30: 4 mm`, `ZS56/30 & ZS56/40: 4 mm`, `IS42/30: 1 mm`, `IS52/30 & IS52/40: 1 mm`), SHIS bore pairings (`Q72`, `Q136`), the `/30` vs `/40` stack identity, and why `turner-venn-ti`, `revel-el-jefe`, `ritchey-ultra`, and `salsa-cutthroat` moved to `4 mm` / `1 mm`.
- `internal/reboot/catalog/forks_rigid.md`: document rigid-fork manufacturer suspension-correction specs, `RigidClass` assignments, and frame compatibility notes for all 7 commercial rigid models (`sunliner_440` non-suspension-corrected; `surly_ecr_468` 80 mm-corrected EC34 straight-steerer; `firestarter_deluxe_483` 100 mm-corrected; `surly_krampus_483` 100 mm class on a frame accepting up to 120 mm/550 mm; `sklar_steel_485` 100 mm-corrected on a 120 mm-tested frame; `enve_mountain_boost` 120 mm-corrected; `lithic_carbon_mountain` 100/120 mm-corrected mapped to `Travel120` in Q135) plus the frames listing them (`tumbleweed-sunliner`, `surly-ecr`, `salsa-cutthroat`, `salsa-fargo`, `surly-krampus`, `sklar-pbj`, `otso-fenrir-stainless`, `seven-expedition-926d`, `seven-expedition-926f`, `esker-hayduke`) and note the Q137 removal of `rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, and `rigid_510`.

### D6. Per-fork lower cup in `internal/reboot/compare` (`compare.go`, `compare_test.go`)

1. Add `LowerCup assembly.LowerCupSetup` to `compare.Fork`.
2. In `compare.Request.validate()` (when iterating `f.Forks`):
   - Validate `fk.LowerCup` when non-zero (`fk.LowerCup != (assembly.LowerCupSetup{})`):
     - Reject `fk.LowerCup.Standard == assembly.LowerCupBareHeadTube`, empty/unknown `Standard` (via exhaustive switch over `assembly.LowerCupStandard`), `math.IsNaN(fk.LowerCup.StackMm) || math.IsInf(fk.LowerCup.StackMm, 0) || fk.LowerCup.StackMm < 0`.
     - Check bore + steerer compatibility against each size's `sz.Frame.LowerBore`:
       - Exhaustive check: reject tapered steerer with `Bore34Pressed` / `Bore42Integrated`; reject incompatible `LowerCup.Standard` for that bore (e.g., `Bore34Pressed` requires `EC34`; `Bore42Integrated` requires `IS42`; `Bore44Pressed` with `Tapered` requires `EC44`, with `Straight118` requires `EC44` or `ZS44`; `Bore56Pressed` requires `ZS56`; `Bore52Integrated` requires `IS52`).
     - Keeping zero-value `fk.LowerCup == assembly.LowerCupSetup{}` allowed preserves existing synthetic fixtures in `compare_test.go`.
3. In `compare.Rank`: use `cup := item.size.Frame.DefaultLowerCup; if item.fork.LowerCup != (assembly.LowerCupSetup{}) { cup = item.fork.LowerCup }` when calling `assembly.PlaceFrameset`.
4. **Tests + Negative Tests (`internal/reboot/compare/compare_test.go`):**
   - **Negative validation tests:** reject invalid `LowerCup.Standard` (`LowerCupBareHeadTube`, unknown), negative/NaN/Inf `LowerCup.StackMm`, mismatched standard for bore (e.g. `EC34` on `Bore52Integrated`), and `ZS44` paired with `SteererTapered` on `Bore44Pressed`.
   - Mixed-cup test on a synthetic `Bore44Pressed` frame confirming a straight fork with `ZS44 4 mm` and a tapered fork with `EC44 12 mm` each place using their own `LowerCup`.

### D7. Query layer wiring & shifted test expectations (`internal/reboot/query/{compare,bike}.go`)

1. `internal/reboot/query/compare.go`:
   - In `CatalogFrames()`: set `LowerCup: opt.LowerCup` on each `compare.Fork` constructed from `f.ForkOptions()`.
2. `internal/reboot/query/bike.go`:
   - In `parseBikeFork`:
     - When selecting a non-basis fork (`matched := matches[0]`), use `matched.LowerCup.Standard` and `matched.LowerCup.StackMm` (instead of `frameSetup.DefaultLowerCup`) as the default lower cup!
     - When `forkSwitched` resets cup stack (`rawCup == ""` or standard changed), compare `matched.LowerCup.Standard != prevMatches[0].LowerCup.Standard` (using `frameSetup.DefaultLowerCup.Standard` only when the previous fork was `"basis"` or failed lookup) and reset `cupMm` to `matched.LowerCup.StackMm`.
     - Basis fork keeps `frameSetup.DefaultLowerCup` / `BasisLowerCup` behavior unchanged.
   - In `(b Bike) ForkOptions() []BikeForkOption`: set `LowerCup: opt.LowerCup` instead of `b.Frame.InstalledFrame(b.Size).DefaultLowerCup`.
3. **Query test updates & new tests (`query/compare_test.go`, `query/bike_test.go`):**
   - `query/compare_test.go:68-82`: verify `CatalogFrames()` matches `len(f.ForkOptions())` and `cf.Forks[j].LowerCup == opt.LowerCup` (and total `compare.Fork` count across all 19 frames is `294`, i.e., sum of per-frame variants `16+16+15+16+11+16+20+11+5+5+16+11+11+16+16+1+16+1+15`, giving `2,989` size×fork×sag rows across all 19 frames).
   - `query/compare_test.go:277` (negative fork validation on `surly-krampus`): `pike_120` is NOW compatible on `surly-krampus` (`Travel120` via `fox34_120`)! Replace `pike_120` in that negative test with `fox34_130` (compatible catalog fork outside `surly-krampus`'s classes → rejected) AND add a test rejecting `surly-ecr` + `firestarter_deluxe_483` (`SteererTapered` rejected on `Bore34Pressed`), while keeping `fox36_150` as deleted/unknown catalog fork. Also test `surly-krampus` + `pike_120` succeeds (`fits`, row count `4`).
   - `query/compare_test.go:401, 611`: replace deleted `rigid_485-485x45` on `binary-havok` L with `sklar_steel_485` (using measured values from Probe 4 & Probe 4b below!).
   - `query/compare_test.go:474–494` (`TestCompareDefaultAll19Frames`): update `RowCount=2989`, `Evaluated=2238761`, `len(Rows)=19`, and shifted ranks/scores per Probe 2 below (`#1 surly-krampus` unchanged; `#2` is now `sklar-pbj` L `fox36_sl_120`; `#3` is `seven-custom`; `#8` is `esker-hayduke`; `#19 salsa-cutthroat` updates from `IS52 0→1`).
   - `query/bike_test.go:75` (`TestParseBikeAllFramesAndSlidingDropouts`) & sliding-dropout wheelbase test (`binary-havok` L): default fork is now `sid_100-506x44` at `20%` sag (since `rigid_485` was deleted); update `binary-havok` expected metrics to Probe 5 below (`wb=1125.9`, `rc=454.8`, `reach=396.1`, `stack=657.1`, `ett=586.6`, `deltaPitch=+0.16°`; basis stays unchanged).
   - `query/bike_test.go:118–127` (`TestParseBikeSwitchesQ109`):
     - Switching from Fenrir (`lithic_carbon_mountain-495x44`) to `surly-krampus` now PRESERVES `lithic_carbon_mountain-495x44` (since `surly-krampus` unlocks `RigidClass495` via `fox34_120`)! Verify that preservation AND verify fallback to default fork (`surly_ecr_468`) when switching to `surly-ecr` (or `firestarter_deluxe_483` when switching to `salsa-cutthroat`).
     - Switching to `salsa-cutthroat`: expected cup changes from `"IS52 0"` to `"IS52 1"`.
   - New test in `query/bike_test.go`: opening an unlisted compatible fork directly on `/bike` (e.g. `frame=surly-krampus&size=L&fork=pike_120` → succeeds with default `25%` sag and `EC44 12`; `frame=binary-havok&size=L&fork=sklar_steel_485` → succeeds with `0%` sag and `EC44 12`); **negative test** `frame=surly-ecr&size=L&fork=firestarter_deluxe_483` rejects incompatible steerer, and deleted `rigid_485` fails with unknown/incompatible fork error.

### D8. Grouped fork `<select>` controls on `/` and `/bike` (`internal/reboot/web/`)

1. `/bike` (`web.go`, `bike_page.go`, `templates/bike.html`):
   - Replace `bikeView.Forks []forkOption` with `ListedForks []forkOption` and `OtherForks []forkOption`.
   - Partition `frameForkOptions` by `b.Frame.IsListedFork(opt.ForkOption.Fork.ID)` (order is preserved cleanly because `CompatibleForks()` puts listed forks first).
   - In `templates/bike.html`: replace `<optgroup label="Installed Forks">` with `<optgroup label="Listed Forks">`, add `{{if .OtherForks}}<optgroup label="Other Compatible Forks">...</optgroup>{{end}}`, and keep `<optgroup label="Chart Basis Fork">` last!
2. `/` (`compare_page.go`, `templates/compare.html`):
   - Replace `compareSelectionRowView.Forks []selectOption` with `ForkLeading []selectOption`, `ListedForks []selectOption`, `OtherForks []selectOption`.
   - In `buildSelectionForkOptions(f catalog.Frame, selectedFork string, invalid bool)`: put `Invalid: ...` (when `invalid && selectedFork != ""`) and `Any fork` into `ForkLeading`; iterate `f.CompatibleForks()`, placing `f.IsListedFork(fk.ID)` entries into `ListedForks` and the rest into `OtherForks` (keeping single-variant vs `(all variants)` + variant `<option>` logic unchanged).
   - In `buildSelectionSagOptions` (`compare_page.go:845`): scan `f.CompatibleForks()` instead of only `f.Forks` so selecting an unlisted compatible suspension fork on `/` still offers/keeps its travel's sag options.
   - In `templates/compare.html`: render `ForkLeading` bare above `<optgroup label="Listed Forks">` and conditional `{{if .OtherForks}}<optgroup label="Other Compatible Forks">...</optgroup>{{end}}`.
3. **Web test updates & new tests (`web/web_test.go`, `web/compare_test.go`):**
   - `web/web_test.go`:
     - Update `/bike` fork-count / workbench tests (`2851`: `"All 12 fork setups"` → `"All 32 fork setups"`; `2917`, `2944`, `2976` per Probe 6b/6c/6d below).
     - Update `/bike` switch tests (`3423–3459`, `3482` `binary-havok`): Fenrir → `surly-krampus` keeps `lithic_carbon_mountain-495x44`; Fenrir → `surly-ecr` falls back to `surly_ecr_468`; `salsa-cutthroat` sees `IS52 1`; `binary-havok` default fork is `sid_100-506x44`.
     - Add `goquery` optgroup assertions on `/bike`:
       - Fenrir / `surly-krampus` renders 3 `<optgroup>` labels in order: `"Listed Forks"`, `"Other Compatible Forks"`, `"Chart Basis Fork"`.
       - `surly-ecr` and `tumbleweed-sunliner` render 2 `<optgroup>` labels in order: `"Listed Forks"`, `"Chart Basis Fork"` (omitting `"Other Compatible Forks"`).
   - `web/compare_test.go`:
     - Replace `pike_120` in the `surly-krampus` invalid-fork test (`483`) with `fox34_130` (and test `surly-ecr` + `firestarter_deluxe_483` negative error, while asserting `surly-krampus` + `pike_120` succeeds).
     - Replace `rigid_485-485x45` (`283`, `568`) with `sklar_steel_485` per Probe 4 & 4b.
     - Update default all-19 compare row assertions per Probe 2 (`Row #1` unchanged; `Row #2` is `sklar-pbj` L `Fox 36 SL 120mm, 15% sag`, `452.0*`; `Row #3` is `seven-custom`; `Row #8` is `esker-hayduke`; `Row #19` `salsa-cutthroat` `60cm` `score=50.3`, `oval-r=3.45`).
     - Add `goquery` optgroup assertions on `/`:
       - Selection row for `surly-krampus` / `otso-fenrir-stainless` has `"Any fork"` directly inside `<select>` before `<optgroup label="Listed Forks">` and `<optgroup label="Other Compatible Forks">` (and invalid selection puts `"Invalid: ..."` directly inside `<select>` before the optgroups).
       - Selection row for `surly-ecr` has `"Any fork"` + `<optgroup label="Listed Forks">` only (no `"Other Compatible Forks"`).

---

## Reference Tables

### Remaining Catalog Forks (`16` models: `7` rigid, `9` suspension)

| ID | Travel | Steerer | RigidClass | Variants (A2C / Rake mm) |
| --- | --- | --- | --- | --- |
| `sunliner_440` | rigid | `straight_118` | `440` | `440/45` |
| `surly_ecr_468` | rigid | `straight_118` | `470` | `468/45` |
| `firestarter_deluxe_483` | rigid | `tapered` | `485` | `483/51` |
| `surly_krampus_483` | rigid | `straight_118` | `485` | `483/47` |
| `sklar_steel_485` | rigid | `straight_118` | `485` | `485/45` |
| `enve_mountain_boost` | rigid | `tapered` | `495` | `490/44`, `492/52` |
| `lithic_carbon_mountain` | rigid | `tapered` | `495` | `495/44`, `498/51.5` |
| `sid_100` | `100mm` | `tapered` | `""` | `506/44`, `506/51` |
| `fox34_sl_120` | `120mm` | `tapered` | `""` | `530.5/44` |
| `sid_120` | `120mm` | `tapered` | `""` | `531/44`, `531/51` |
| `pike_120` | `120mm` | `tapered` | `""` | `531/44` |
| `fox34_120` | `120mm` | `tapered` | `""` | `534/44`, `534/51` |
| `fox36_sl_120` | `120mm` | `tapered` | `""` | `536/44` |
| `fox34_sl_130` | `130mm` | `tapered` | `""` | `540.5/44` |
| `fox34_130` | `130mm` | `tapered` | `""` | `544/44`, `544/51` |
| `fox36_sl_130` | `130mm` | `tapered` | `""` | `546/44` |

*(Deleted placeholders: `rigid_483`, `rigid_485`, `rigid_490`, `rigid_500`, `rigid_510`.)*

### Per-Frame Compatible Totals (`2,989` size × fork × sag configs; `2,238,761` builds)

| Frame | Sizes | Models | Variants | Configs / size | Total configs | Listed first in `CompatibleForks()` |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| `binary-havok` | 5 | 11 | 16 | 32 | 160 | `sid_100`, `fox34_120` |
| `binary-kinetik` | 6 | 11 | 16 | 32 | 192 | `fox34_120`, `sid_100` |
| `esker-hayduke` | 4 | 10 | 15 | 33 | 132 | `pike_120`, `fox34_130`, `enve_mountain_boost`, `lithic_carbon_mountain` |
| `fairlight-holt-2` | 4 | 11 | 16 | 32 | 128 | `pike_120`, `sid_100` |
| `moots-mountaineer` | 4 | 7 | 11 | 25 | 100 | `pike_120` |
| `otso-fenrir-stainless` | 5 | 11 | 16 | 32 | 160 | `lithic_carbon_mountain`, `sid_100`, `fox34_120` |
| `revel-el-jefe` | 4 | 14 | 20 | 40 | 160 | `sid_120`, `sid_100`, `fox34_130` |
| `ritchey-ultra` | 4 | 7 | 11 | 25 | 100 | `fox34_120` |
| `salsa-cutthroat` | 5 | 4 | 5 | 7 | 35 | `firestarter_deluxe_483`, `sid_100` |
| `salsa-fargo` | 5 | 4 | 5 | 7 | 35 | `firestarter_deluxe_483`, `sid_100` |
| `seven-custom` | 1 | 11 | 16 | 32 | 32 | `enve_mountain_boost`, `sid_100`, `fox34_120` |
| `seven-expedition-926d` | 24 | 7 | 11 | 25 | 600 | `enve_mountain_boost`, `lithic_carbon_mountain`, `fox34_120` |
| `seven-expedition-926f` | 24 | 7 | 11 | 25 | 600 | `enve_mountain_boost`, `lithic_carbon_mountain`, `fox34_120` |
| `sklar-pbj` | 5 | 11 | 16 | 32 | 160 | `sklar_steel_485`, `sid_100`, `fox34_120` |
| `sour-pasta-party` | 4 | 11 | 16 | 32 | 128 | `fox34_120`, `sid_100` |
| `surly-ecr` | 3 | 1 | 1 | 1 | 3 | `surly_ecr_468` |
| `surly-krampus` | 4 | 11 | 16 | 32 | 128 | `surly_krampus_483`, `sid_100`, `fox34_120` |
| `tumbleweed-sunliner` | 4 | 1 | 1 | 1 | 4 | `sunliner_440` |
| `turner-venn-ti` | 4 | 10 | 15 | 33 | 132 | `fox34_130`, `fox34_120` |

---

## Commit Split (3 Atomic Commits)

1. **Commit 1: `Add typical lower-cup catalog and per-fork cups in compare`** (`D1`, `D6`, partial `D5`)
   - Add `internal/reboot/catalog/lower_cups.go` (`TypicalLowerCup`, `LowerCupFor`) and `internal/reboot/catalog/lower_cups.md`.
   - Add `LowerCup` to `compare.Fork`, validate non-zero `LowerCup` in `compare.Request.validate()`, and apply it in `compare.Rank`.
   - Add positive and negative unit tests in `catalog/parts_test.go` and `compare/compare_test.go`.
   - Gate: `make pre-commit`.

2. **Commit 2: `Enable open fork choice and drop illustrative rigid forks`** (`D2`, `D3`, `D4`, `D7`, rest of `D5`)
   - Add `RigidClass` (`ValidFor`, `standInRigidClasses`) and `ForkOption.LowerCup`, drop all 5 `rigid_*` placeholders from `catalog/forks.go`, and add `internal/reboot/catalog/forks_rigid.md`.
   - Add `IsListedFork` and `CompatibleForks` to `catalog/frame.go`, resolve `LowerCup` in `ForkOptions()`, strengthen `frameProblems`, update the 4 frame lower cups and 5 frame fork lists, relax `frames_parity_test.go` to headset bore + standard only, and update `catalog` tests.
   - Wire `LowerCup` through `query/compare.go` (`CatalogFrames`) and `query/bike.go` (`parseBikeFork`, `ForkOptions`), updating all shifted assertions and adding open-fork / negative-steerer tests in `query/compare_test.go`, `query/bike_test.go`, and shifted assertions in `web/*_test.go`.
   - Gate: `make pre-commit`.

3. **Commit 3: `Group listed and compatible forks in web selects and record handoff`** (`D8`)
   - Update `web.go`, `bike_page.go`, `compare_page.go`, `templates/bike.html`, `templates/compare.html` for `<optgroup label="Listed Forks">` and `<optgroup label="Other Compatible Forks">` (omitted when empty), keeping `<optgroup label="Chart Basis Fork">` last on `/bike` and leading options above groups on `/`, plus `buildSelectionSagOptions` over `CompatibleForks()`.
   - Add `goquery` optgroup and negative tests in `web/web_test.go` and `web/compare_test.go`.
   - Run full verification + live UI/CLI spot checks, and write the `Implementation handoff` note in your final report (`DESIGN.md`).
   - Gate: `make pre-commit`.

---

## Measured Test Targets (Probed via Catalog + Query Placement)

Use these verified numbers (do not recalculate by hand):

- **Default All-19-Frames Compare (`TestCompareDefaultAll19Frames`):** `RowCount=2989`, `Evaluated=2238761`, `len(Rows)=19`:
  - `#1` (`[0]`): `surly-krampus` XL `fox34_120-534x44` (`25%`), `"Fox Float 34 120mm (534/44), 25% sag"`, `fits_all`, `total=85.120463740` (`85.1`), `gripR=0.945603917` (`0.95`), `brpMiss=(-5.298465,+0.000000)` (`(-5.3, +0.0) mm`), `stem=100/-7°/0`, `rail=+15.0`, `ins=164.291568` (`164.3`), `cs=435.0`, `wb=1169.3`, `rcwb=36.9%`, `bbdrop=56.9`, `trail=103.4` (all 12 attributes unchanged from Slice 16c).
  - `#2` (`[1]`): `sklar-pbj` L `fox36_sl_120` (`15%`), `"Fox 36 SL 120mm, 15% sag"`, `fits_all`, `total=76.200061329` (`76.2`), `gripR=0.887113437` (`0.89`), `brpMiss=(-9.555379,+0.000000)` (`(-9.6, +0.0) mm`), `stem=110/-17°/0`, `rail=+15.0`, `ins=149.7`, `cs=452.0` (sliding `*`), `reach=455.2`, `stack=655.2`, `ett=664.9`, `hta=67.3`, `sta=72.3`, `wb=1203.6`, `rcwb=37.3%`, `bbdrop=54.3`, `trail=108.3`.
  - `#3` (`[2]`): `seven-custom` `Custom` `fox34_120-534x44` (`25%`), `fits_brp`, `total=53.126792179` (`53.1`), `gripR=2.092042057` (`2.09`), `brpMiss=(-9.679803,+0.000000)` (`(-9.7, +0.0) mm`), `reach=415.8`, `stack=636.4`, `ett=619.4`.
  - `#4` (`[3]`): `seven-expedition-926f` `S 63cm` `enve_mountain_boost-492x52` (`0%`), `fits_brp`, `total=55.726426456` (`55.7`).
  - `#5` (`[4]`): `salsa-fargo` XL `sid_100-506x51` (`20%`), `fits_brp`, `total=54.488981918` (`54.5`).
  - `#6` (`[5]`): `seven-expedition-926d` `S 62cm` `enve_mountain_boost-492x52` (`0%`), `fits_brp`, `total=55.791926019` (`55.8`), `cs=453.0`.
  - `#7` (`[6]`): `binary-havok` L `fox36_sl_120` (`15%`), `fits_brp`, `total=45.756427671` (`45.8`), `gripR=5.949629731` (`5.95`), `brpMiss=(-10.0, -2.0) mm`.
  - `#8` (`[7]`): `esker-hayduke` S3 `enve_mountain_boost-492x52` (`0%`), `fits_grip`, `total=81.320323095` (`81.3`), `gripR=0.952602612` (`0.95`), `brpMiss=(-37.0, +0.0) mm`, `cs=454.0` (sliding `*`).
  - `#19` (`[18]`): `salsa-cutthroat` `60cm` `firestarter_deluxe_483` (`0%`), `fits_neither`, `total=50.260457620` (`50.3`), `gripR=3.453797202` (`3.45`).
- **Fenrir L Compare (`frame=otso-fenrir-stainless&size=L`):** `RowCount=32`, `Evaluated=23968`:
  - `#1`: `firestarter_deluxe_483` (`0%`, `fits_neither`, `total=63.094565950` / `63.1`, `gripR=1.273523966` / `1.27`, `reach=427.5`, `stack=642.3`, `ett=597.1`, `cs=450.0`).
  - `#2`: `sklar_steel_485` (`0%`, `fits_neither`, `total=61.655179580` / `61.7`, `gripR=1.364379137` / `1.36`, `reach=426.7`, `stack=642.9`, `ett=597.3`, `cs=450.0`).
  - `#3`: `surly_krampus_483` (`0%`, `fits_neither`, `total=62.306228949` / `62.3`, `gripR=1.364769922` / `1.36`, `reach=426.7`, `stack=642.9`, `ett=597.4`, `cs=450.0`).
  - `#4`: `sid_100-506x51` (`20%`, `fits_neither`, `total=60.642370057` / `60.6`, `gripR=1.446148070` / `1.45`, `reach=426.0`, `stack=643.4`, `ett=597.5`).
  - `#5`: `sid_100-506x44` (`20%`, `fits_neither`, `total=58.531309737` / `58.5`, `gripR=1.616431203` / `1.62`, `reach=424.5`, `stack=644.3`, `ett=597.9`).
- **Binary-Havok L + `sklar_steel_485` (replacing deleted `rigid_485-485x45`):**
  - Single selection (`RowCount=1`, `Evaluated=749`): `fits_neither`, `total=49.155334622` (`49.2`), `gripR=3.718610833` (`3.72`), `stem=120/-30°/0`, `ins=176.648907`, `rail=15.0`, `cs=460.0`.
  - Multi-frame `krampus` + `binary-havok L` (`sklar_steel_485`) with `post=setback_25`: `#1 surly-krampus XL fox34_120-534x44` (`fits_all`, `90.718077011`, `0.725092414`), `#2 binary-havok L sklar_steel_485` (`fits_neither`, `49.816319845`, `3.718610833`).
- **Binary-Havok L Default Bike (`fork=sid_100-506x44&sag=20` vs `basis`):**
  - Default: `wb=1125.947211` (`1125.9 mm`), `rc=454.839332` (`454.8 mm`), `reach=396.138719` (`396.1 mm`), `stack=657.125647` (`657.1 mm`), `ett=586.584158` (`586.6 mm`), `deltaPitch=0.160734` (`+0.16°`).
  - Basis: `wb=1110.618343` (`1110.6 mm`), `rc=439.461964` (`439.5 mm`), `deltaPitch=+0.00°` (unchanged).
- **`/bike` Fenrir L Workbench (`fork_options=all` spans `32` configs, `23968` builds evaluated; `+ bar_options=all` spans `359520` builds):**
  - Summary banner: `"All 32 fork setups"`.
  - Default post, both targets, 1 bar (`Evaluated=23968`):
    - `#1` `firestarter_deluxe_483`, `0%`, `fits_neither`, `total=63.094565950` (`63.1`), `gripR=1.273523966` (`1.27`), `stem=120/-20°/0`.
    - `#2` `sklar_steel_485`, `0%`, `fits_neither`, `total=61.655179580` (`61.7`), `gripR=1.364379137` (`1.36`), `stem=120/-20°/0`.
  - `brp=off`, 1 bar (`Evaluated=23968`): `#1` `firestarter_deluxe_483` (`total=68.321929313` / `68.3`), `#2` `sklar_steel_485` (`total=66.068976734` / `66.1`).
  - `post=setback_0`, 1 bar (`Evaluated=23968`): `#1` `firestarter_deluxe_483` (`total=63.094565950` / `63.1`, `brpMiss=(-59.5, +0.0) mm`, `ins=199.0`).
  - Default post, `bar_options=all` (`Evaluated=359520`): `#1` `sqlab311_flx_16_30` + `surly_krampus_483` (`fits_grip`, `total=71.384721774` / `71.4`, `stem=120/-17°/0`); `#2` `sqlab311_20_16_25` + `surly_krampus_483` (`71.2`); `#3` `sqlab311_20_16_15` + `surly_krampus_483` (`71.2`); `#4` `sqlab311_flx_16_15` + `surly_krampus_483` (`71.1`); `#5` `sqlab311_flx_16_30` + `firestarter_deluxe_483` (`71.0`).

---

## Rules That Bite

1. **`make boundary` scans ALL files under `internal/reboot` and `cmd/bikegeo-reboot` (except `*_parity_test.go`), including `.md` provenance files (`lower_cups.md`, `forks_rigid.md`)!** Forbidden case-insensitive substrings anywhere (even inside longer words): `chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, and `cad` (watch words like "CAD", "cascade", "cadence").
2. **`make exhaustive` (`exhaustive` tool):** every `switch` over `assembly.LowerCupStandard`, `assembly.LowerBore`, `assembly.Steerer`, `assembly.ForkTravel`, and `catalog.RigidClass` must name **all** defined constants of that type, even if it has a `default` branch.
3. **Zero external dependencies & strict isolation:** no third-party imports outside `_test.go`; never import legacy `bikegeo/...` packages outside `*_parity_test.go`; all trig/radian math stays inside `internal/reboot/pose`.
4. **Commit messages (`AGENTS.md`):** plain-language subjects only — Conventional Commits prefixes (`feat:`, `fix:`, `chore:`, etc.) are banned.
5. **`/bike` `<optgroup>` ordering invariant:** `web_test.go` checks that the last `<optgroup>` label in `/bike`'s fork `<select>` is `"Chart Basis Fork"`. Always keep `"Chart Basis Fork"` after `"Listed Forks"` and `"Other Compatible Forks"`.
6. **Clean diff on existing frames:** do not alter `BasisFork`, `BasisLowerCup`, or chart geometry on any frame; do not edit legacy `internal/frames`, `internal/catalog`, `internal/compare`, or `internal/web`.
7. **Implementation handoff (`DESIGN.md`):** before handing back after Commit 3, include an **Implementation handoff** section in your final response listing: (1) any judgment calls or tie-breakers you made that go beyond explicit plan lines, (2) any deviations from `SLICE16d.md` (with reasons), (3) non-obvious couplings noticed during implementation, and (4) the exact verification commands run and their results.

---

## Verification Steps

1. After every edit: `go vet ./cmd/bikegeo-reboot/... ./internal/reboot/... && go test ./cmd/bikegeo-reboot/... ./internal/reboot/...`
2. Before each of the 3 commits: `make pre-commit` (re-run if `go fix`/`go fmt` modified any file).
3. Verify zero third-party runtime dependencies:
   `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo-reboot` returns only `bikegeo/...`.
4. Quick live CLI + HTTP verification after Commit 3:
   - `go run ./cmd/bikegeo-reboot compare -spec otso-fenrir-stainless:L` shows expanded compatible forks (`firestarter_deluxe_483`, `sklar_steel_485`, `surly_krampus_483`, `sid_100`, etc.) and `go run ./cmd/bikegeo-reboot compare -spec surly-ecr:L+firestarter_deluxe_483` returns an error.
   - `go run ./cmd/bikegeo-reboot bike -frame surly-krampus -size L -fork pike_120` succeeds with `25%` sag and `EC44 12 mm`.
   - `make serve-reboot` (or targeted `httptest` curl against `/` and `/bike`) confirms `"Listed Forks"`, `"Other Compatible Forks"` (and omitted on `surly-ecr`), and `"Chart Basis Fork"` render cleanly.

---

## Out of Scope

- Otso Fenrir Stainless chart re-ingestion and promoting the basis-finding test helper (**Slice 16e**).
- Full parity audit across `/` vs `/bike` and legacy comparison (**Slice 17**).
- Adding missing suspension forks to Salsa Cutthroat / Salsa Fargo fork lists or re-ingesting legacy frames (`frame_<id>.md` records wait until each frame is re-ingested).
- Performance optimization of full-catalog `Compare` runtime (`Q86`).
