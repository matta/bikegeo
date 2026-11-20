# Slice 12c: Go catalog port and 19-frame closure

Status: Implemented 2026-11-20; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-20, Claude Opus 5.5 (`claude-opus-5-5`) in Claude Code.
- **Commits:**
  1. `9942803` Add the reboot parts catalog
  2. `41f6123` Port the 19 legacy frames into the reboot catalog
  3. `425bdb8` Build the bike workbench from the reboot catalog
  4. `0933899` Rename the web fixture to the bike workbench

  This handoff and the DESIGN.md Milestones line are a separate docs-only commit.
- **Done:** A1–A7, F1–F9 (with F2n and F5n), W1–W4, and commit 4's rename. All 19 frame files came from a throwaway generator that read legacy `internal/frames` and `geometry.InferChartPrecision` and wrote the Go literals and labels. The generator was deleted and never committed, and the F8/F9 parity tests pin its output. Changing one published value (Fenrir XS Reach 387 → 388) failed F8 as expected, and the change was reverted. **Not done:** nothing.
- **Deviations:**
  - **F5 count.** The plan's 701 counts only the fork-list placements. The CAD baseline adds one placement per size (119), so the total is 820. I recounted from the legacy fork lists with an independent `awk` pass over `internal/frames/*.go`. It also gives 820 = 701 + 119. `TestForkListsPlace` asserts 701 fork-list placements and 119 CAD baseline placements separately, so the plan's number holds without being changed.
  - **F5n** is its own test, `TestForkListsPlaceChecksSteerers` (the plan doesn't name it). It uses 20% sag.
  - **F2 max chainstay** reports one problem per frame, comparing against the longest published chainstay. This makes F2n's "exactly one problem" hold for the five-size Fenrir.
- **Verification:**
  - `make pre-commit` exits 0 on each of the four commits and on the final state.
  - `go test ./internal/reboot/catalog/ -run 'TestEverySizeCloses|TestForkListsPlace' -v` shows 119 passing closure subtests and logs `701 fork-list placements, 119 CAD baseline placements`.
  - Precision literals match the plan's P column exactly.
  - Web tests pass with only the fork-name strings changed. Every numeric expectation is byte for byte.
  - **Live check:** port 8081 was taken, so the server ran from `.claude/launch.json` (`go run ./cmd/bikegeo-reboot serve`) on port 8082, not through `make serve-reboot`. On `/bike`, the heading reads `Otso Fenrir Stainless (2024, Size L)`. The fork select lists `Lithic Carbon Mountain (Rigid, 495 mm A2C / 54 mm rake)`, `RockShox SID 100mm (100 mm travel, 506 mm A2C / 44 mm rake)`, `Fox Float 34 120mm (120 mm travel, 530 mm A2C / 44 mm rake)`, and `CAD Baseline Fork (495 mm A2C / 54 mm rake, 0 mm cup)`. `|BRP - BB| (SQlab 610 Ergolux Active 2.1)` reads `745.6 mm`. `/bike?fork=cad` and `/bike?fork=sid_100&sag=25` return HTTP 200.
  - **Screenshot:** `/Users/matt/.claude/projects/-Users-matt-src-bikegeo/3f6f0175-f47a-475a-b37e-fa56c2798604/tool-results/mcp-Claude_Browser-blob-1795217955154-4m2yjq.jpg` (outside the repo).
- **Open issues:**
  - `grep -rni fixture internal/reboot/web` finds nothing after commit 4.
  - DESIGN.md still mentions `internal/reboot/web/fixture.go`: the Catalog bullet (around line 95), the "Catalog data lives in" bullet (around line 317), and the Q47 text. These are left for acceptance, as the plan says.
  - No frame lists three of the 22 fork models: `sid_110`, `fox36_150`, and `rigid_483`. They are ported as catalog models only.
  - The test-local custom `sid_100` fork in `web_test.go` keeps its old `"RockShox SID"` name, as W3 says.
  - The `turner-venn-ti` XL stack (670 mm), which legacy notes is probably a transcription error for 690 mm, is kept as published, and it closes. Legacy's own corrections (`sklar-pbj` M wheelbase 1156 mm, `seven-expedition-926f` Endurance chainstay 463 mm) are ported as legacy has them and noted in each frame's doc comment.

**Size: Larger** (4 commits, ~30 files, most of them frame data). Builds on Slice 12a (commits `e5b3af5`, `083e0a3`, `4360b44`, `741670c`, `a88060c`) and Slice 12b (commit `3b00a64`). Delegate the implementation to a lower-tier model (for example Sonnet). If you hit a design choice this plan doesn't cover, stop, set `Status: Blocked <date> on <question>`, and write the handoff. Don't guess.

## Goal

1. Add `internal/reboot/catalog/`: all 19 legacy frames (119 sizes) as `assembly.FrameChart`s with per-frame fork lists, 22 fork models, 15 handlebars, 4 seatposts, the SQlab 610, the shared wheel, the suspension sag steps, and the idmatch fit targets. Everything is Go structs, and every value is labelled.
2. Validate the catalog in `go test` only (Q91): chart closure on every size, the Q83 fork-list placement and SHIS check, and seatpost bounds on every size. Parity tests pin every ported value to legacy.
3. Build `/bike`'s Fenrir L workbench from catalog lookups, drop the `web.NewHandler` closure panic, and delete `internal/reboot/web/fixture.go`. `/bike` still shows only the Fenrir L (multi-frame `/bike` is Slice 14). Its only visible change is fork names, which now come from the legacy catalog.

## Rulings this rests on

- **Q50:** Go-struct catalogs; port 19 frames (119 sizes), 22 forks, 15 bars, 4 seatposts, SQlab 610.
- **Q66, Q72:** frame `LowerBore` + `DefaultLowerCup`, fork `Steerer`, per-frame `ForkSpec` variants, SHIS validation. Legacy `HeadsetCupType` → bore: `EC34 → Bore34Pressed`, `EC44 → Bore44Pressed`, `ZS56 → Bore56Pressed`, `IS52 → Bore52Integrated` (no legacy frame uses ZS44 or IS42). Steerers: `sunliner_440`, `surly_ecr_468`, `surly_krampus_483`, `sklar_steel_485` are straight 1-1/8" (`catalog`); the other 12 named forks are tapered (`catalog`); the 6 `rigid_*` are tapered (`illustrative`).
- **Q67:** sliding dropouts evaluate at max chainstay on `/` and `/bike`; closure uses the published chainstay.
- **Q71, Q93:** `ritchey-ultra` `CadFork.A2cMm = 508.5` with no allowance; `sklar-pbj` L `ClosureAllowance{AxleHeightMm: 0.4}`; `esker-hayduke` S4 `ClosureAllowance{WheelbaseMm: 1.2}`; every other size has none.
- **Q75a, Q75d, Q75f:** `SeatTubeLengthMm` is C-T; `+35 mm` illustrative C-C → C-T on the 3 Seven frames; straight published effective `StaDeg` (legacy `ActualStaDeg` is not ported).
- **Q80:** each frame's `ChartPrecision` is legacy `geometry.InferChartPrecision`'s output, copied as literals and pinned by a parity test.
- **Q81:** `CadLowerCup.StackMm` = legacy `GeoHeadsetSpacerMm`; standard `LowerCupBareHeadTube` when that is `0`, else the frame's `DefaultLowerCup.Standard`. `CadFork.A2cMm` = legacy `CadFork.A2cMm` (already effective); the legacy CAD fork identity and sag are a provenance comment.
- **Q83:** a catalog test places every size with every fork in its list at every enumerated sag, plus the CAD baseline fork.
- **Q91:** closure runs in `catalog`'s `go test` only; the `web.NewHandler` startup panic goes.
- **Q94:** 12c stays one slice of four commits, and `/bike` is built from catalog lookups with no behavior change beyond fork names.
- **Q95:** `MaxPostInsertionMm` is ported only for the Fenrir L (Otso's published `290 mm`). Every other size is `0` (fallback `SeatTubeLengthMm − 20 mm`).

## Clean defaults (veto on review)

- **Physical lower cup stack** is legacy `HeadsetCupType.CupStackMm()`: EC = `12 mm`, ZS = `2 mm`, IS = `0 mm`. It is the cup legacy installs for every physical fork (`geometry.ComparisonBuild`). Every physical fork option uses the frame's `DefaultLowerCup`.
- **Frame names** keep legacy `Manufacturer`, `Model`, and `Year`, except the Fenrir's `Model` is `"Fenrir Stainless"` (matching its ID and today's `/bike` heading). The parity test lists that one override.
- **Fork names** are legacy `CommonForkCatalog` names (`"RockShox SID 100mm"`, `"Fox Float 34 120mm"`). This changes the `/bike` fork labels and stock-row fork cells.
- **Legacy `Notes`** are not a runtime field. Each frame file's doc comment summarizes their provenance (CAD fork and cup, dropouts, chart quirks) in a few lines.
- **Not ported:** `FrontCenter`, `StandoverHeight`, `Category`, `WheelSize` (all 19 are 29"), `CanonicalStem`, `Stems`, `ExpectedStemRangeMm`, `MaxSpacerMm` (Q82 constant), `GeoSlopMm` (replaced by Q71/Q93 allowances), `ActualStaDeg` (Q75f), the minimum of `ChainstayRangeMm`, and legacy fork `Options`/`Description`/`Standard`.
- **Workbench defaults stay in `web`** (default fork, stem, cockpit stock and preferences, default bar and post IDs, Q92's `180.576 mm` insertion). They are Fenrir-L workbench settings, and Slices 13b and 14 replace them. The catalog holds parts data, the sag steps, and the fit targets.
- **Immutability:** `Frames()`, `Forks()`, `Handlebars()`, `Seatposts()`, and `SuspensionSagPcts()` build fresh values on every call, so callers can't mutate shared slices. Fork models are package-level values with no slices.
- **Legacy package name clash:** parity tests import the legacy `bikegeo/internal/catalog` as `legacycatalog` and `bikegeo/internal/frames` as `legacyframes`.

## Probe values

An independent probe (a throwaway `*_parity_test.go` that built every legacy size by the rules above and ran the reboot `assembly` functions, deleted before this plan was written) measured:

- All **119 sizes pass** `assembly.CheckChartClosure` with exactly the two Q71 allowances. `PlaceCadBaselineFork` succeeds on all 119.
- `PlaceFrameset` succeeds for every size × listed fork × enumerated sag, on both the published frame and the max-chainstay installed frame. There are no SHIS failures.
- `SeatpostInsertionBounds` succeeds for every size with both the generic `400/365/100` and the Redshift `350/250/100` posts. The shortest C-T seat tube is `salsa-fargo` XS at `355.6 mm` (generic bounds `[100, 335.6]`).

Per-frame family values (P = precision columns that differ from `0.5`; every other column is `0.5`):

| Frame | Sizes (declared order) | Default | Bore / default cup | CAD cup | CAD fork A2C / rake (legacy identity) | Max CS | P |
| --- | --- | --- | --- | --- | --- | ---: | --- |
| `binary-havok` | S M ML L XL | L | 44 pressed / EC44 12 | EC44 10 | 485 / 45 (`rigid_485`) | 460 | HTA 0.25 |
| `binary-kinetik` | S SMD M ML L XL | L | 44 pressed / EC44 12 | EC44 10 | 512 / 44 (`fox34_120` @ 15%) | 472 | — |
| `esker-hayduke` | S1 S2 S3 S4 | S3 | 44 pressed / EC44 12 | EC44 12 | 495 / 44 (`pike_120` @ 30%) | 454 | HTA 0.25 |
| `fairlight-holt-2` | S M L XL | L | 44 pressed / EC44 12 | EC44 12 | 507 / 44 (`pike_120` @ 20%) | 0 | — |
| `moots-mountaineer` | S M L XL | M | 44 pressed / EC44 12 | EC44 12 | 507 / 44 (`pike_120` @ 20%) | 0 | Chainstay 0.25 |
| `otso-fenrir-stainless` | XS S M L XL | L | 44 pressed / EC44 12 | bare 0 | 495 / 54 (`lithic_carbon_mountain`) | 450 | HTA 0.125 |
| `revel-el-jefe` | S M L XL | L | 52 integrated / IS52 0 | bare 0 | 506 / 44 (`sid_120` @ 20%) | 435 | HTA 0.25 |
| `ritchey-ultra` | S M L XL | L | 52 integrated / IS52 0 | bare 0 | **508.5** / 44 (`fox34_120` @ 20%, legacy 506; Q71) | 0 | HTA 0.25 |
| `salsa-cutthroat` | 52cm 54cm 56cm 58cm 60cm | 56cm | 52 integrated / IS52 0 | bare 0 | 481 / 51 (`sid_100` @ 25%) | 0 | — |
| `salsa-fargo` | XS S M L XL | L | 44 pressed / EC44 12 | bare 0 | 483 / 51 (`firestarter_deluxe_483`) | 462 | HTA 0.25 |
| `seven-custom` | Custom | Custom | 44 pressed / EC44 12 | EC44 11 | 492 / 52 (`enve_mountain_boost`) | 0 | HTA 0.05, Wheelbase 0.05 |
| `seven-expedition-926d` | S 52cm … S 65cm, E 52cm … E 65cm (24, legacy order) | S 58cm | 44 pressed / EC44 12 | EC44 10 | 472 / 55 (`enve_mountain_472`) | 0 | HTA 0.125 |
| `seven-expedition-926f` | S 55cm … S 68cm, E 55cm … E 68cm (24, legacy order) | S 60cm | 44 pressed / EC44 12 | EC44 10 | 472 / 55 (`enve_mountain_472`) | 0 | HTA 0.125 |
| `sklar-pbj` | XS S M L XL | M | 44 pressed / EC44 12 | EC44 12 | 485 / 52 (`sklar_steel_485`) | 452 | HTA 0.25 |
| `sour-pasta-party` | S M L XL | L | 44 pressed / EC44 12 | EC44 12 | 512 / 44 (`fox34_120` @ 15%) | 0 | HTA 0.05 |
| `surly-ecr` | M L XL | L | 34 pressed / EC34 12 | EC34 12 | 468 / 43 (`surly_ecr_468`) | 0 | Reach 0.25, Stack 0.25, Wheelbase 0.25 |
| `surly-krampus` | S M L XL | L | 44 pressed / EC44 12 | EC44 12 | 483 / 47 (`surly_krampus_483`) | 0 | — |
| `tumbleweed-sunliner` | S M L XL | L | 44 pressed / EC44 12 | EC44 13 | 440 / 55 (`sunliner_440`) | 0 | — |
| `turner-venn-ti` | S M L XL | L | 56 pressed / ZS56 2 | bare 0 | 541 / 44 (`fox34_130`) | 0 | — |

Unpublished wheelbases (Q57): `moots-mountaineer`, `seven-expedition-926d`, `seven-expedition-926f`. Their `Precision.WheelbaseMm` literal is still legacy's `0.5`, which `CheckChartClosure` ignores when the wheelbase is unpublished. The 12b unit-test messages for the two allowance sizes hold unchanged on the catalog charts, which carry the same values.

## Package API (`internal/reboot/catalog`)

Imports only `assembly`, `pose`, and the standard library.

### `catalog.go`: package doc, shared constants, targets

```go
// Package catalog holds the reboot's frames, forks, handlebars, seatposts,
// saddle, and default fit targets as Go values. Every value is labelled
// published, catalog, measured, owner-reported, or illustrative.
package catalog

// WheelRadiusMm is the shared 29" × 2.4" wheel radius (legacy DefaultWheelRadius29Mm).
const WheelRadiusMm = 372.0

// WheelLabel names the shared wheel.
const WheelLabel = `29" × 2.4"`

// SuspensionSagPcts returns the sag steps a suspension fork is enumerated at (Q31, Q56).
func SuspensionSagPcts() []float64 { return []float64{15, 20, 25} }

// DefaultBrpTarget returns the idmatch fit report's BRP (measured; docs/RIDER_FIT.md, Q53).
func DefaultBrpTarget() pose.Point { return pose.Point{X: -219.0, Y: 713.1} }

// DefaultGripTarget returns the idmatch fit report's HandGrip (measured; Q53, Q64).
func DefaultGripTarget() pose.Point { return pose.Point{X: 481.0, Y: 726.0} }
```

### `forks.go`

```go
// Fork is a catalog fork model. Frames install it at a frame-specific A2C and rake (Q66).
type Fork struct {
	ID       string
	Name     string
	TravelMm float64 // 0 for rigid forks
	Steerer  assembly.Steerer
}

// FrameFork is one fork a frame supports, at that frame's A2C and rake variant.
type FrameFork struct {
	Fork        Fork
	StaticA2cMm float64
	RakeMm      float64
}

// Spec returns the installed fork's assembly specification.
func (f FrameFork) Spec() assembly.ForkSpec

// Forks returns the 22 fork models in legacy catalog order.
func Forks() []Fork
```

The 22 models are unexported package-level `Fork` values (`sid100`, `sid110`, `fox34_120`, `sid120`, `pike120`, `fox34_130`, `pike140`, `fox36_150`, `sunliner440`, `surlyEcr468`, `enveMountain472`, `enveMountainBoost`, `firestarterDeluxe483`, `surlyKrampus483`, `rigid483`, `rigid485`, `sklarSteel485`, `rigid490`, `lithicCarbonMountain`, `rigid495`, `rigid500`, `rigid510`; any Go-legal names work). They carry legacy `ID`/`Name`/`TravelMm` and the Q72 steerer, with `// catalog` or `// illustrative` labels. Frame files reference these values directly, so an unknown fork can't compile.

### `handlebars.go`, `seatposts.go`, `saddle.go`

```go
type Handlebar struct {
	ID    string
	Name  string
	Setup assembly.HandlebarSetup // BarRollDeg is always 0; the roll is a workbench input (Q21)
}
func Handlebars() []Handlebar // the 15 bars, today's fixture order and values (Q20)

type Seatpost struct {
	ID   string
	Name string
	Spec assembly.SeatpostSpec
}
func Seatposts() []Seatpost // setback_0, setback_15, setback_25, redshift_shockstop_pro (Q28, Q75c)

type Saddle struct {
	ID    string
	Name  string
	Setup assembly.SaddleSetup // RailOffsetMm and RailPitchDeg are 0
}
func DefaultSaddle() Saddle // ID "sqlab_610", Name "SQlab 610 Ergolux Active 2.1"
```

Move the values and their provenance comments from `fixture.go` unchanged: `legacyHandlebar`, `gripInsetMm = 70` (illustrative), the SQlab bend-start note, the generic Ritchey-based post constants and URL, the Redshift spec comments (Q28, Q75c, the unsagged-transfer note per Q78), and the SQlab 610 setup (including the Q75b `45 mm` illustrative note). The default targets lose the stale "HandGrip or stem clamp is unresolved" comment (Q64 settled it).

### `frame.go`

```go
// Frame is one frame model with its sizes and the forks it supports.
type Frame struct {
	ID           string
	Manufacturer string
	Model        string
	Year         int
	DefaultSize  string
	Sizes        []FrameSize // declared order (Q69)
	Forks        []FrameFork // declared order (Q69)
	// MaxChainstayMm is the rearmost sliding-dropout position, where / and
	// /bike evaluate the frame (Q67); 0 means fixed dropouts.
	MaxChainstayMm float64
}

// FrameSize is one size's published chart.
type FrameSize struct {
	Name  string
	Chart assembly.FrameChart
}

// Size returns the named size.
func (f Frame) Size(name string) (FrameSize, bool)

// InstalledFrame returns the size's frame as / and /bike build it: sliding
// dropouts at MaxChainstayMm (Q67), otherwise the published chart frame.
func (f Frame) InstalledFrame(size FrameSize) assembly.FrameSetup

// Frames returns the 19 frames in ascending ID order.
func Frames() []Frame

// FrameByID returns the frame with the exact ID.
func FrameByID(id string) (Frame, bool)
```

Unexported helpers keep each frame file to published numbers plus one family block:

```go
// frameFamily holds what every size of a frame shares: its headset, its CAD
// hypotheses (Q44, Q81), and its declared precision (Q80).
type frameFamily struct {
	LowerBore       assembly.LowerBore
	DefaultLowerCup assembly.LowerCupSetup
	CadLowerCup     assembly.LowerCupSetup
	CadFork         assembly.ForkGeometry
	Precision       assembly.ChartPrecision
}

// publishedSize is one size's published chart columns.
type publishedSize struct {
	Name                                                        string
	ReachMm, StackMm                                            float64
	HeadTubeAngleDeg, SeatTubeAngleDeg                          pose.Deg
	HeadTubeLengthMm, BbDropMm, ChainstayMm, SeatTubeLengthMm   float64
	WheelbaseMm                                                 float64 // 0 = unpublished (Q57)
	MaxPostInsertionMm                                          float64 // 0 = unpublished (Q95)
	Allowance                                                   assembly.ClosureAllowance
}

func (f frameFamily) sizes(published ...publishedSize) []FrameSize
```

`sizes` copies the columns into `assembly.FrameChart{Frame: FrameSetup{…, LowerBore, DefaultLowerCup, MaxPostInsertionMm, MinFrameInsertionMm: 0}, CadLowerCup, CadFork, PublishedWheelbaseMm, Precision, Allowance}`.

### `frame_<id>.go` × 19

One file per frame (for example `frame_otso_fenrir_stainless.go`), each an unexported constructor (`func otsoFenrirStainless() Frame`) that `Frames()` calls in ID order. Copy every published column from `internal/frames/<id>.go` exactly, in legacy `Sizes` order. Specific labels:

- Family comments: `// published` on the columns block; `// inferred from published digits (legacy), Q80` on `Precision`; `// hypothesis (Q44, Q81): legacy <fork_id> @ <sag>% sag → <A2C> mm effective` on `CadFork`; `// Q66: inferred from legacy HeadsetCupType <X>; stack is legacy CupStackMm (illustrative)` on the bore and default cup.
- **Seven** (all 3): `const sevenCollarOffsetMm = 35.0 // illustrative C-C → C-T until Seven confirms (Q75d)` in one of the Seven files, and every size writes `SeatTubeLengthMm: <published C-C> + sevenCollarOffsetMm`, so the published value stays visible.
- **Fenrir L:** `MaxPostInsertionMm: 290` with the Otso URL comment from `fixture.go` (Q95).
- **`ritchey-ultra`:** `CadFork.A2cMm: 508.5` with a comment citing Q71 and legacy's `506`.
- **`sklar-pbj` L:** `Allowance: assembly.ClosureAllowance{AxleHeightMm: 0.4, Reason: "Size L's published Stack (641 mm) implies a 481.7 mm fork A2C with the 12 mm EC44 CAD cup, 3.3 mm short of the 485 mm rigid fork; every other size closes within rounding (Q71)."}`.
- **`esker-hayduke` S4:** `Allowance: assembly.ClosureAllowance{WheelbaseMm: 1.2, Reason: "The published S4 wheelbase is 5.42 mm longer than the declared front axle position (HTL + cup + A2C) gives, 1.11 mm beyond rounding; S1–S3 close within rounding (Q71)."}`.
- **`sklar-pbj`, `tumbleweed-sunliner`:** note that the straight-steerer fork runs in an EC44/30 reducer cup (Q66).
- **`revel-el-jefe`, `fairlight-holt-2`:** note that the seat tube is bent and only the effective STA is modeled (Q75f).
- Each fork list mirrors legacy `Forks` in order: `{Fork: sid100, StaticA2cMm: 506, RakeMm: 44}`.

## Commits

Each commit passes `make pre-commit` on its own.

### Commit 1: `Add the reboot parts catalog`

Files: `catalog/catalog.go`, `catalog/forks.go`, `catalog/handlebars.go`, `catalog/seatposts.go`, `catalog/saddle.go`, `catalog/parts_test.go`, `catalog/parts_parity_test.go`. `web` is untouched.

Tests (`parts_test.go`, package `catalog_test`):

- **A1 `TestForks`:** 22 models with unique non-empty IDs. Each `Steerer` is `SteererStraight118` or `SteererTapered`, and the 4 straight IDs are exactly the Q72 list. `TravelMm ≥ 0`.
- **A2 `TestHandlebars`:** 15 bars with unique IDs, the first `persuader20_800`. Each has `BarRollDeg == 0`, and `assembly.PlaceHandlebar(pose.NewPose(pose.Point{}, 0), bar.Setup)` returns nil.
- **A3 `TestSeatposts`:** IDs in order `setback_0, setback_15, setback_25, redshift_shockstop_pro`. Each `Spec.Validate()` is nil, setbacks are `0, 15, 25, 12`, and the Redshift is `350/250/100` while the generic posts are `400/365/100`.
- **A4 `TestDefaultSaddleAndTargets`:** `DefaultSaddle()` equals today's fixture setup field by field (`32, −48, 45, 10, 60, 0, 0`). Targets are `(−219, 713.1)` and `(481, 726)`. `SuspensionSagPcts()` is `[15 20 25]`, and mutating the returned slice doesn't change the next call's result (the negative case for immutability).

Parity (`parts_parity_test.go`, package `catalog_test`):

- **A5 `TestForksMatchLegacy`:** `Forks()` matches `legacycatalog.CommonForkCatalog` in order on `ID`, `Name`, and `TravelMm`.
- **A6 `TestHandlebarsMatchLegacy`:** port `web/handlebar_parity_test.go` against `catalog.Handlebars()`, with the same fields and lowercased-ID order.
- **A7 `TestWheelRadiusMatchesLegacy`:** `WheelRadiusMm == model.DefaultWheelRadius29Mm`.

### Commit 2: `Port the 19 legacy frames into the reboot catalog`

Files: `catalog/frame.go`, 19 × `catalog/frame_<id>.go`, `catalog/frames_test.go`, `catalog/frames_parity_test.go`.

Tests (`frames_test.go`, package `catalog_test`):

- **F1 `TestFramesRegistry`:** 19 frames, IDs strictly ascending, 119 sizes total, with per-frame size counts `binary-havok 5, binary-kinetik 6, esker-hayduke 4, fairlight-holt-2 4, moots-mountaineer 4, otso-fenrir-stainless 5, revel-el-jefe 4, ritchey-ultra 4, salsa-cutthroat 5, salsa-fargo 5, seven-custom 1, seven-expedition-926d 24, seven-expedition-926f 24, sklar-pbj 5, sour-pasta-party 4, surly-ecr 3, surly-krampus 4, tumbleweed-sunliner 4, turner-venn-ti 4`. `FrameByID("otso-fenrir-stainless")` is found and `FrameByID("Otso-Fenrir-Stainless")` is not (exact match).
- **F2 `frameProblems(f catalog.Frame) []string`**, a test helper run on every frame by `TestFrameStructure`, which wants none. It reports:
  - an empty ID or no sizes;
  - a duplicate size name, or a `DefaultSize` not among the sizes;
  - no forks, or a duplicate `Fork.ID` in `Forks`;
  - `MaxChainstayMm` that is nonzero and below any size's published chainstay.

  **F2n `TestFrameProblemsCatchMistakes`:** table of mutations on the Fenrir, each of which must yield exactly one problem containing the given substring: duplicate size `"L"` → `duplicate size`; `DefaultSize = "XXL"` → `default size`; `Forks = nil` → `no forks`; `Forks[1] = Forks[0]` → `duplicate fork`; `MaxChainstayMm = 440` → `max chainstay`.
- **F3 `TestEverySizeCloses` (Q91):** `assembly.CheckChartClosure(size.Chart)` is nil for all 119 sizes. Subtest name `<id>/<size>`.
- **F4 `TestClosureAllowances` (Q71, Q93):** exactly two sizes have a non-zero `Allowance`: `sklar-pbj` L `{AxleHeightMm: 0.4}` and `esker-hayduke` S4 `{WheelbaseMm: 1.2}`, both with a non-blank `Reason`. Setting each size's `Allowance` to the zero value (reason included) makes `CheckChartClosure` fail with an error starting `chart closure: axle height residual -3.06 mm exceeds its ±2.68 mm rounding budget` (Sklar L) and `chart closure: wheelbase residual +5.42 mm exceeds its ±4.31 mm rounding budget` (Hayduke S4).
- **F5 `TestForkListsPlace` (Q83):** for every size, for every `FrameFork` in its frame's list, and for each sag (`0` when `Fork.TravelMm == 0`, else each of `SuspensionSagPcts()`), `assembly.PlaceFrameset(frame.InstalledFrame(size), size.Chart.Frame.DefaultLowerCup, assembly.ForkSetup{Spec: ff.Spec(), SagPct: sag}, assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm})` is nil. `assembly.PlaceCadBaselineFork(size.Chart, wheels)` is nil. Count the placements and expect **701** (hand count from the legacy fork lists; if the count differs, recount from the fork table before changing it).

  **F5n:** placing `surly-ecr` L (EC34 bore) with `{Fork: sid100, StaticA2cMm: 506, RakeMm: 44}` fails with an error containing `does not fit a tapered steerer`. This is the negative case showing that F5 exercises SHIS.
- **F6 `TestSeatpostBoundsEverySize`:** `assembly.SeatpostInsertionBounds(size.Chart.Frame, post.Spec)` is nil for every size × every `Seatposts()` entry. Exact values (±1e-9): Fenrir L × `setback_0` → `[100, 290]`; Fenrir L × `redshift_shockstop_pro` → `[100, 250]`; `salsa-fargo` XS × `setback_0` → `[100, 335.6]`; `seven-expedition-926d` S 52cm × `setback_0` → `[100, 365]` (post-limited: the frame cap is C-C `355` + `35` − `20` = `370`; without the Q75d offset it would be frame-limited at `335`).
- **F7 `TestInstalledFrameUsesMaxChainstay` (Q67):** `esker-hayduke` S4 → chainstay `454` (chart `440`); Fenrir L → `450` (chart `450`); `ritchey-ultra` L (fixed) → equals the chart frame exactly. The returned setup differs from `size.Chart.Frame` only in `ChainstayMm` (compare the struct with `ChainstayMm` copied over).

Parity (`frames_parity_test.go`, package `catalog_test`), over `legacyframes.AllFrames()` matched by ID (19 ↔ 19):

- **F8 `TestFramesMatchLegacy`:** for each frame, `Manufacturer`, `Model` (override map `{"otso-fenrir-stainless": "Fenrir Stainless"}`), `Year`, `DefaultSize`, and the size names in legacy `Sizes` order. `MaxChainstayMm` is `ChainstayRangeMm[1]` when legacy declares a range, else `0`. `Forks` matches legacy in order on fork ID, `TravelMm`, `A2cMm`, and `RakeMm`. For each size, every published column equals the legacy `SizeGeometry` (`Reach`, `Stack`, `HtaDeg`, `StaDeg`, `Htl`, `BbDrop`, `ChainstayLength`, `Wheelbase`, and `SeatTubeLength`, plus `35` on frames whose ID starts with `seven-`). `LowerBore` and `DefaultLowerCup` come from the Q66 mapping of `HeadsetCupType` and `CupStackMm()`, and `CadLowerCup` from Q81. `CadFork` equals legacy `CadFork.A2cMm`/`RakeMm`, except `ritchey-ultra` A2C is `508.5`. `MaxPostInsertionMm` is `290` on Fenrir L only and `MinFrameInsertionMm` is `0` everywhere. Allowances appear on exactly the two F4 sizes.
- **F9 `TestPrecisionMatchesLegacyInference` (Q80):** every size's `Precision` equals `geometry.InferChartPrecision(legacy)` field by field (`HtaDeg → HeadTubeAngleDeg`, `HtlMm → HeadTubeLengthMm`; ignore `FrontCenterMm`).

### Commit 3: `Build the bike workbench from the reboot catalog`

Files: `web/fixture.go`, `web/web.go`, `web/web_test.go`, delete `web/handlebar_parity_test.go` (A6 replaces it), and `AGENTS.md`.

- **W1 `fixture.go`:** `otsoFenrirFixture()` keeps its signature and return type. It now builds from `catalog.FrameByID("otso-fenrir-stainless")` and `frame.Size("L")` (panicking with a clear message if either is missing):
  - `Name: fmt.Sprintf("%s %s (%d, Size %s)", frame.Manufacturer, frame.Model, frame.Year, size.Name)` → still `Otso Fenrir Stainless (2024, Size L)`.
  - `Chart: size.Chart` with `Chart.Frame = frame.InstalledFrame(size)` and the comment `// Q67: sliding dropouts at max chainstay (450 mm, the L's published value)`.
  - `Forks`: one `makeForkOption(ff.Fork.ID, ff.Fork.Name, ff.Spec(), sag, chart.Frame.DefaultLowerCup)` per `frame.Forks` entry, with `sag` `0` for rigid and `defaultSuspensionSagPct = 20.0` (`// Q32 ideal sag`) otherwise.
  - `Wheels: assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm}`, `WheelLabel: catalog.WheelLabel`, `StockSagPcts: catalog.SuspensionSagPcts()`, `Handlebars: catalog.Handlebars()`, `Seatposts: catalog.Seatposts()`, `SaddleName`/`DefaultSaddle` from `catalog.DefaultSaddle()`, `ExampleBrpTarget: catalog.DefaultBrpTarget()`, `ExampleGripTarget: catalog.DefaultGripTarget()`.
  - These stay literal workbench defaults, unchanged: `DefaultForkID "lithic_carbon_mountain"`, `DefaultStem`, `CockpitStock`, `CockpitPreferences`, `DefaultHandlebarID "persuader20_800"`, `DefaultSeatpostID "setback_0"`, and `DefaultPostInsertionMm 180.576` (Q92 comment kept).
  - Delete `handlebarOption`, `seatpostOption`, `legacyHandlebar`, `gripInsetMm`, and the inline chart/fork/bar/post/saddle data. `bikeFixture.Handlebars` becomes `[]catalog.Handlebar` and `Seatposts` becomes `[]catalog.Seatpost` (same field names: `ID`, `Name`, `Setup`/`Spec`). `forkOption`, `makeForkOption`, `stockForkOption`, and `validate()` stay.
- **W2 `web.go`:** `NewHandler` no longer calls `checkFixtureChart`. Delete `checkFixtureChart` and update the doc comment (no panic; closure runs in `catalog` tests, Q91). Replace `handlebarOption`/`seatpostOption` with the catalog types (lines around `Handlebars`, `Seatposts`, `offered`, `buildStockFittingView`).
- **W3 `web_test.go`:** delete `TestCheckFixtureChart`. Update the fork-name strings only: `"RockShox SID"` → `"RockShox SID 100mm"` and `"Fox 34"` → `"Fox Float 34 120mm"` in expected cell values and labels (for example the `stockForkSID20`/`stockForkSID25`/`stockForkFox20` constants and the `"RockShox SID, 25% sag"` row). Test-case *names* and the test-local `"rigid_straight_483"`/`"sid_100"` custom forks stay as they are. **Every numeric expectation stays byte for byte**: the Fenrir L data is identical, so any numeric change is a porting bug, not something to re-measure.
- **W4 `AGENTS.md`:** replace the **Fixture** bullet with: `- **Catalog:** reboot frames, forks, handlebars, seatposts, the SQlab 610, and default fit targets are Go values in internal/reboot/catalog (not internal/frames), validated by its go test (chart closure, fork-list placement, seatpost bounds). Published chart values are never nudged. /bike shows only the Otso Fenrir Stainless L until Slice 14.` (Wrap the paths in backticks.)

### Commit 4: `Rename the web fixture to the bike workbench`

A pure rename with no behavior change. Use `gopls rename` where it applies, and `git mv` for the file:

- `web/fixture.go` → `web/workbench.go`; `bikeFixture` → `bikeWorkbench`; `otsoFenrirFixture` → `fenrirWorkbench`; method receivers and locals `fix`/`f`/`customFix` → `wb`/`w`/`customWb` where they hold one; test helper `fixtureSeatpost` → `workbenchSeatpost`.
- Test functions whose names contain `Fixture` → `Workbench` (for example `TestBikeFixtureValidatesHandlebars` → `TestBikeWorkbenchValidatesHandlebars`).
- `validate()` error prefixes `fixture %q …` → `workbench %q …`, updating the matching test substrings. Comments that say "fixture" for this struct say "workbench".
- After the commit, `grep -rni fixture internal/reboot/web` should only find unrelated test-data wording, if any. List any hits in the handoff.

## Rules that bite

- **Never nudge a published chart value.** If F3 or F8 fails, the port has a typo. Fix the port, never the expectation.
- `catalog` imports only `assembly`, `pose`, and the standard library. `assembly` and `compare` never import `catalog`. Legacy imports appear only in `*_parity_test.go` files (`make boundary`).
- No trig or geometry in `catalog`; it is data plus lookups. The only arithmetic is `+ sevenCollarOffsetMm`.
- Label every value (published, catalog, measured, owner-reported, illustrative). Don't present an illustrative value as manufacturer truth.
- Banned words (`make boundary`): `chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`, in comments and `Reason` strings too.
- `exhaustive`: any new `switch` on `LowerBore`, `LowerCupStandard`, `Steerer`, or `model.HeadsetCupType` names every constant.
- Don't change `assembly`, `pose`, templates, or `/bike` behavior beyond the fork names. Don't move workbench defaults into `catalog`. Don't add `/bike` frame/size selection (Slice 14).
- Run `go vet ./... && go test ./...` after each change and `make pre-commit` before each commit.
- Finish with the Implementation handoff (DESIGN.md "Implementation handoff"): this file's Status line, an `## Implementation handoff` section right after it, and the DESIGN.md Milestones line for 12c.

## Verification

- `make pre-commit` is clean on each commit.
- `go test ./internal/reboot/catalog/ -run 'TestEverySizeCloses|TestForkListsPlace' -v` shows 119 closure subtests and the 701-placement count.
- Live check: `make serve-reboot`, then confirm on `/bike` (default URL):
  - the heading reads `Otso Fenrir Stainless (2024, Size L)`;
  - the fork select lists `Lithic Carbon Mountain`, `RockShox SID 100mm`, `Fox Float 34 120mm`, and the CAD Baseline Fork;
  - the `|BRP - BB|` readout is unchanged from before the slice (the Lithic default reads `745.6 mm`, per the Q92 web test);
  - `/bike?fork=cad` and `/bike?fork=sid_100&sag=25` render without error.

  Save a screenshot and record its path in the handoff.

## Out of scope

- `/bike` frame and size selection, compare defaults (Redshift post, idmatch targets as defaults), and the `*` sliding-dropout flag (Slices 13c and 14).
- `stems.yaml`, attribute weights, and scoring (Slice 13b). `CockpitPreferences` stays in `web` until then.
- Published insertion depths beyond the Fenrir L (Q95; deferred follow-up), bent seat tubes (Q75f), and more saddles (Q52).
- DESIGN.md's **Run and inspect** and **Layout** text about the fixture; it's updated at acceptance.
