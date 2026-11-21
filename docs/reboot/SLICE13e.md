# Slice 13e: Leveled frame readouts and multi-frame compare engine

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-21, Gemini 4 Hydrogen (Medium)
- **Commits:**
  - `Add leveled reach, stack, and ETT frameset readouts` (`f156358`)
  - `Add multi-frame compare engine` (`068a628`)
- **Done and not done:** A1, A2, C1, C2, C3 all done.
- **Deviations:** None.
- **Verification:** `make pre-commit` passed cleanly on both commits. `go list -deps ./internal/reboot/compare | grep -E 'reboot/(catalog|query|web)'` produced no matches. `go test ./internal/reboot/compare/ -count=1 -v` passed in `0.333s`. No live check: this slice changes no page, URL, or CLI output.
- **Open issues:** None.

**Rulings:** Q16, Q51, Q56, Q61, Q66, Q67, Q69, Q76, Q83, Q86, Q87, Q89, Q90, Q103, Q104, Q105; `AGENTS.md` rules.

**Builds on:** Slice 13c (commits `bf99568`, `d203497`, `796c306`) for `FitBuilds`, `ScoreBuild`, and `CompareFitRank`, and Slice 13d (commits `9676745`, `2341512`, `5a43f75`) for the `PartOptions` / `ScoredBuild` / `BuildRanking` names.

**Size:** larger (5 code/test files, 2 commits). Written for a lower-tier implementer. **Don't make design choices.** If anything below doesn't match committed code, stop and record `Blocked <date> on <question>` (see "Rules that bite").

*(Sub-slice decision, Q103: old 13e is split. This slice is the Go engine only, tested with small fixtures. Slice 13f adds `internal/reboot/query`, the `catalog` → `compare` adapter, the compare defaults, and `bikegeo-reboot compare [query]`.)*

## Goal

1. Add the three leveled frame readouts of Q105 to `assembly.FramesetPlacement`.
2. Add `internal/reboot/compare`: expand `(frame, size, fork, sag)` rows from all frames or from custom selections, find each row's winning build with `assembly.FitBuilds` (limit 1), rank rows with `assembly.CompareFitRank` plus the Q69 cross-row tie-breaks, and prune with `TopPerFrame` and `Top`.
3. Zero change to `assembly` fitting or scoring, `catalog`, `web`, or `cmd`.

---

## Commit 1: `Add leveled reach, stack, and ETT frameset readouts`

### A1. `internal/reboot/assembly/frameset.go`

Add three methods right after `RcWbRatioPct`:

```go
// LeveledReachMm returns the horizontal distance from BB to HeadTubeTop on the
// leveled bike. It differs from the chart's reach by the leveling pitch (Q105).
func (f FramesetPlacement) LeveledReachMm() float64 {
	return f.BB.OffsetTo(f.HeadTubeTop).X
}

// LeveledStackMm returns the vertical distance from BB to HeadTubeTop on the
// leveled bike (Q105).
func (f FramesetPlacement) LeveledStackMm() float64 {
	return f.BB.VerticalOffsetTo(f.HeadTubeTop)
}

// EttMm returns the effective top tube: the horizontal distance, at
// HeadTubeTop's height, from the leveled seat axis through BB to HeadTubeTop
// (legacy BikeScene.EttMm, Q105).
func (f FramesetPlacement) EttMm() float64 {
	// Split BB → HeadTubeTop into a run up the seat axis and a horizontal
	// remainder; the remainder is the ETT.
	_, ett, ok := pose.DecomposeAlong(f.SeatAxis.OffsetTo(f.HeadTubeTop), f.SeatAxis.HeadingDeg(), 0)
	if !ok {
		return math.NaN() // a horizontal seat axis never reaches HeadTubeTop's height
	}
	return ett
}
```

(`SeatAxis` has its origin at `BB`, so `SeatAxis.OffsetTo(HeadTubeTop)` is BB → HeadTubeTop.)

### A2. `internal/reboot/assembly/frameset_test.go`

Extend `TestFramesetHandlingReadouts`. Add `wantReach`, `wantStack`, and `wantEtt` fields to its case struct, with these values for the three existing cases:

| Case | `wantReach` | `wantStack` | `wantEtt` |
| --- | ---: | ---: | ---: |
| `Lithic (rigid)` | `421.943304877` | `645.996785959` | `598.532949715` |
| `Fox 34 at 20%` | `414.113186529` | `651.043983723` | `600.558793289` |
| `CAD baseline` | `428.119798484` | `641.920118197` | `596.995173525` |

In the loop, add:

```go
assertNear(t, "LeveledReachMm", placed.LeveledReachMm(), tc.wantReach, 1e-6)
assertNear(t, "LeveledStackMm", placed.LeveledStackMm(), tc.wantStack, 1e-6)
assertNear(t, "EttMm", placed.EttMm(), tc.wantEtt, 1e-6)
assertNear(t, "LeveledReachMm == HeadTubeTop.X", placed.LeveledReachMm(), placed.HeadTubeTop.Origin().X, 1e-12)
assertNear(t, "LeveledStackMm == HeadTubeTop.Y", placed.LeveledStackMm(), placed.HeadTubeTop.Origin().Y, 1e-12)
```

Extend the test's leading comment to say the reach, stack, and ETT values come from an independent Python probe that rebuilds the frameset from the chart (head tube down by `HTL + cup + effective A2C`, rake at 90°, rear axle at `(−√(CS² − drop²), drop)`, pitch `= −atan2(front − rear)`), rotates `(Reach, Stack)` by the pitch, and takes `ETT = x − y·cot(180° − STA + pitch)`. That probe and the Go readouts agree to 1e-9 mm. (For reference, the unpitched chart ETT is `428 + 642·cot(75.25°) = 597.024 mm`.)

---

## Commit 2: `Add multi-frame compare engine`

### C1. New file `internal/reboot/compare/compare.go`

Imports: `cmp`, `fmt`, `math`, `slices`, `bikegeo/internal/reboot/assembly`. **Never import `catalog`, `query`, or `web`.**

```go
// Package compare ranks bikes across frames. It expands frame, size, fork, and
// sag rows, finds each row's winning build with assembly.FitBuilds, and ranks
// and prunes the rows (Q52, Q61, Q69). It never imports catalog: callers pass
// frames, parts, and weights as arguments.
package compare

// Frame is one frame model offered for comparison.
type Frame struct {
	ID    string
	Sizes []Size // declared order (Q69)
	Forks []Fork // declared order (Q69)
}

// Size is one frame size as compare installs it. Callers set sliding
// dropouts to max chainstay before passing it (Q67).
type Size struct {
	Name  string
	Frame assembly.FrameSetup
}

// Fork is one physical fork the frame supports, at this frame's A2C and rake
// (Q66). Every row installs it on Size.Frame.DefaultLowerCup.
type Fork struct {
	ID   string
	Spec assembly.ForkSpec
}

// Selection picks rows from one frame (Q87). An empty Size or ForkID selects
// every size or fork. A nil SagPct selects 0% for a rigid fork and each of
// Request.SuspensionSagPcts for a suspension fork. A non-nil SagPct needs a ForkID.
type Selection struct {
	FrameID string
	Size    string
	ForkID  string
	SagPct  *float64
}

// Request is one comparison.
type Request struct {
	Frames            []Frame
	Selections        []Selection // empty selects every size, fork, and sag of every frame
	SuspensionSagPcts []float64   // sags an open selection expands a suspension fork to (Q56)
	Wheels            assembly.WheelSetup
	Seatpost          assembly.SeatpostSpec
	Saddle            assembly.SaddleSetup    // the saddle fit sets RailOffsetMm; RailPitchDeg is kept
	Stem              assembly.StemSetup      // build fitting sets LengthMm, AngleDeg, and SpacersMm
	Handlebar         assembly.HandlebarSetup // the selected bar; its BarRollDeg applies to every bar (Q24)
	Target            assembly.FitTarget      // needs both BRP and HandGrip (Q76)
	Parts             assembly.PartOptions    // Forks must be empty: each row fixes its fork
	Weights           assembly.ScoreWeights
	TopPerFrame       int // most rows kept per frame; 0 keeps all
	Top               int // most rows kept overall; 0 keeps all
}

// Row is one distinct (frame, size, fork, sag) and its winning build (Q69).
type Row struct {
	FrameID   string
	Size      string
	ForkID    string
	SagPct    float64
	Build     assembly.ScoredBuild
	Evaluated int // combinations FitBuilds placed for this row

	sizeIndex int // position of Size in Frame.Sizes (Q69 tie-break)
	forkIndex int // position of ForkID in Frame.Forks (Q69 tie-break)
}

// Ranking is the result of Rank.
type Ranking struct {
	Rows      []Row // best first, after pruning
	RowCount  int   // distinct rows ranked before pruning
	Evaluated int   // combinations placed across every row
}

// idealSagPct is the sag the Q69 cross-row tie-break prefers (the Q59a ideal).
const idealSagPct = 20.0

// Rank expands the requested rows, fits each row's winning build with
// assembly.FitBuilds, orders the rows best first, and prunes them.
func Rank(req Request) (Ranking, error)
```

`Rank` does exactly this, in order:

1. **Validate** with an unexported `func (r Request) validate() error`. Each check returns the exact error below (`%q` for strings, `%d` for ints):

   | Check (in this order) | Error |
   | --- | --- |
   | `len(r.Frames) == 0` | `compare needs at least one frame` |
   | For each frame: `ID == ""` | `frame %d has an empty ID` (index) |
   | Frame ID seen before | `duplicate frame %q` |
   | `len(Sizes) == 0` | `frame %q has no sizes` |
   | `len(Forks) == 0` | `frame %q has no forks` |
   | A size `Name == ""` | `frame %q size %d has an empty name` (index) |
   | Size name seen before in this frame | `frame %q has duplicate size %q` |
   | A fork `ID == ""` | `frame %q fork %d has an empty ID` (index) |
   | Fork ID seen before in this frame | `frame %q has duplicate fork %q` |
   | `fork.Spec.Steerer == assembly.SteererCAD` | `frame %q fork %q is the CAD baseline fork; compare installs physical forks only` |
   | `len(r.SuspensionSagPcts) == 0` | `compare needs suspension sag steps` |
   | `r.Target.BRP == nil` | `compare needs a BRP target (Q76)` |
   | `r.Target.HandGrip == nil` | `compare needs a HandGrip target (Q76)` |
   | `len(r.Parts.Forks) != 0` | `compare fixes each row's fork; part options must not list forks` |
   | `r.TopPerFrame < 0` | `top per frame must be 0 or more, got %d` |
   | `r.Top < 0` | `top must be 0 or more, got %d` |
   | For each selection: no frame with `FrameID` | `unknown frame %q` |
   | `Size != ""` and the frame has no such size | `frame %q has no size %q` |
   | `ForkID != ""` and the frame has no such fork | `frame %q has no fork %q` |
   | `SagPct != nil && ForkID == ""` | `selection on frame %q sets sag without a fork` |

   Everything else (finite targets, stems, spacer step, weights, fork sag range, rigid fork with non-zero sag, seatpost bounds) is left to `assembly`, which already rejects it per row (step 3).

2. **Expand rows.** Use `r.Selections`, or when it is empty, one `Selection{FrameID: f.ID}` per frame in `r.Frames` order. For each selection, in order: each matching size in declared order, then each matching fork in declared order, then each sag: `[*SagPct]` if set; else `[0]` for a rigid fork (`Spec.TravelMm == 0`); else `r.SuspensionSagPcts` in order. Skip a row whose `(FrameID, Size, ForkID, SagPct)` (exact float equality) was already expanded, so duplicate selections are ranked once (Q69). Record `sizeIndex` and `forkIndex` as positions in the frame's `Sizes` and `Forks`.

3. **Fit each row** in expansion order:

   ```go
   bounds, err := assembly.SeatpostInsertionBounds(size.Frame, r.Seatpost)
   // on error: return fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", frameID, size, forkID, sag, err)
   setup := assembly.BikeSetup{
   	Frame:     size.Frame,
   	LowerCup:  size.Frame.DefaultLowerCup,
   	Fork:      assembly.ForkSetup{Spec: fork.Spec, SagPct: sag},
   	Wheels:    r.Wheels,
   	Seatpost:  assembly.SeatpostSetup{Spec: r.Seatpost, InsertionMm: bounds.EffectiveMinInsertionMm}, // the saddle fit replaces it
   	Saddle:    r.Saddle,
   	Stem:      r.Stem,
   	Handlebar: r.Handlebar,
   }
   ranking, err := assembly.FitBuilds(setup, r.Target, r.Parts, r.Weights, 1)
   // on error: the same wrapped message as above
   ```

   The row's `Build` is `ranking.Builds[0]` and its `Evaluated` is `ranking.Evaluated`. The first row error aborts `Rank` (Q83 keeps the catalog compatible; Q104 measured zero row errors).

4. **Sort** with `slices.SortFunc(rows, compareRows)`, where

   ```go
   // compareRows orders rows best first: by CompareFitRank on the winning
   // builds, then the Q69 cross-row tie-breaks (frame ID, declared size order,
   // declared fork order, sag closest to 20%, then lower sag).
   func compareRows(a, b Row) int {
   	return cmp.Or(
   		assembly.CompareFitRank(a.Build.Score, b.Build.Score),
   		cmp.Compare(a.FrameID, b.FrameID),
   		cmp.Compare(a.sizeIndex, b.sizeIndex),
   		cmp.Compare(a.forkIndex, b.forkIndex),
   		cmp.Compare(math.Abs(a.SagPct-idealSagPct), math.Abs(b.SagPct-idealSagPct)),
   		cmp.Compare(a.SagPct, b.SagPct),
   	)
   }
   ```

5. **Prune.** Walk the sorted rows. Skip a row when `TopPerFrame > 0` and its frame already has `TopPerFrame` kept rows. Stop once `Top > 0` rows are kept. Return `Ranking{Rows: kept, RowCount: len(sorted rows), Evaluated: Σ row.Evaluated}`. `Evaluated` and `RowCount` count every row, including pruned ones.

### C2. New file `internal/reboot/compare/compare_test.go` (`package compare_test`)

Imports `assembly` and `pose` only (no `catalog`; small fixtures per DESIGN "Layout and boundaries"). Define these fixtures (values are the catalog's; label them so in one comment):

- `func fixtureFrames() []compare.Frame` returning, **in this order** (so the test proves `Rank` sorts by ID, not by input order):
  1. `surly-krampus`: sizes `L` `{Reach 448, Stack 612, HTA 69, STA 73, HTL 110, BbDrop 65, Chainstay 435, SeatTubeLength 470}` and `XL` `{470, 621, 69, 73, 120, 65, 435, 521}`; forks `surly_krampus_483` `{StaticA2cMm 483, TravelMm 0, RakeMm 47, Steerer SteererStraight118}` and `fox34_120` `{530, 120, 44, SteererTapered}`.
  2. `otso-fenrir-stainless`: sizes `L` `{428, 642, 68.25, 75.25, 140, 72, 450, 490, MaxPostInsertionMm 290}` and `XL` `{441, 660, 68.25, 75.25, 160, 72, 450, 530}`; forks `lithic_carbon_mountain` `{495, 0, 54, SteererTapered}` and `sid_100` `{506, 100, 44, SteererTapered}`.
  
  Every size has `LowerBore: assembly.Bore44Pressed`, `DefaultLowerCup: {LowerCupEC44, 12}`, and `MinFrameInsertionMm: 0` (and `MaxPostInsertionMm: 0` unless given).
- `func fixtureRequest() compare.Request`:
  - `Frames: fixtureFrames()`, `SuspensionSagPcts: []float64{15, 20, 25}`, `Wheels: {RadiusMm: 372}`.
  - `Seatpost`: Redshift `{TotalLengthMm 350, MaxInsertionMm 250, MinInsertionMm 100, SetbackMm 12, RailClampLengthMm 30}`.
  - `Saddle`: SQlab 610 `{RailCenterToBrpMm 32, RailCenterToSitBonesMm −48, RailToBrpHeightMm 45, SitBonesElevationMm 10, RailLengthMm 60, RailPitchDeg 0, RailOffsetMm 0}`.
  - `Stem`: `{UpperStackMm assembly.DefaultUpperStackMm, SpacersMm 20, ClampHeightMm 40, LengthMm 60, AngleDeg −6, MaxSpacersMm assembly.DefaultMaxSpacersMm, MinLengthMm 40, MaxLengthMm 130}`.
  - `Handlebar`: `persuaderBar()` = Persuader 20° 800 `{StockWidthMm 800, CurrentWidthMm 800, StatedRiseMm 50, RiseMode RiseModeBarEndCenter, BacksweepDeg 20, UpsweepDeg 4, BendStartMm 135, GripInsetMm 70, BarRollDeg 0}`.
  - `Target`: BRP `(−219, 713.1)`, HandGrip `(481, 726)`.
  - `Parts`: `{Stems: fixtureStems(), SpacerStepMm: 5}` where `fixtureStems()` is the 10 stems of `assembly`'s `fenrirStemOptions()` (copy them: `60/0 95.5`, `70/−6 100`, `70/0 100`, `70/6 100`, `90/−17 62`, `110/−17 36.25`, `120/−30 0`, `120/−20 0`, `120/−17 24`, `120/−10 34.5`), giving 10 × 7 = 70 combinations per row.
  - `Weights`: Q59a `{HandGrip 6, SaddleBrpY 6, SaddleBrpX 4, Stem 5, HandlebarSweep 3, ForkSag 3, Spacers 2, SaddleRailOffset 2, RcWbRatio 2, SeatpostInsertion 1, LeveledBbDrop 1, Trail 1}`.
  - `TopPerFrame: 0`, `Top: 0`.
- `sqlab311Bar()` = SQlab 311 2.0 Alloy 16°, 50 mm rise `{740, 740, 50, RiseModeStandardKnuckle, 16, 5, 100.0, 70, 0}` (field order as in the Persuader line).

A helper `assertRows(t, got []compare.Row, want []wantRow)` checks the length, then per row: `FrameID`, `Size`, `ForkID`, `SagPct` (exact), `Build.Score.Tier`, `Build.Score.Total` (±1e-6), `Build.Score.GripR` (±1e-6), `Build.Placement.Setup.Stem.LengthMm` / `AngleDeg` / `SpacersMm` (exact), and, when the case sets it, `HandlebarIndex` (exact). Shorthand below: **K** = `surly-krampus`, **F** = `otso-fenrir-stainless`, `krampus` = `surly_krampus_483`, `fox` = `fox34_120`, `lithic` = `lithic_carbon_mountain`, `sid` = `sid_100`.

Expected values come from an independent probe that called `assembly.FitBuilds(…, 1)` directly for each row (on the catalog's values) and sorted the winners by hand with `CompareFitRank` then the Q69 keys.

#### T1. `TestRankFullFixture`

`compare.Rank(fixtureRequest())`: `RowCount == 16`, `Evaluated == 1120`, and every row's `Evaluated == 70`. Rows, in order:

| # | Frame | Size | Fork | Sag | Tier | Total | GripR | Stem | Spacers |
| -: | --- | --- | --- | -: | --- | -: | -: | --- | -: |
| 1 | K | XL | fox | 20 | `FitsAll` | 81.951358 | 0.146116 | 110 / −17 | 10 |
| 2 | K | L | fox | 20 | `FitsAll` | 79.262788 | 0.850141 | 120 / −10 | 0 |
| 3 | K | XL | fox | 15 | `FitsAll` | 79.151692 | 0.254290 | 110 / −17 | 5 |
| 4 | K | XL | fox | 25 | `FitsAll` | 76.070832 | 0.089092 | 110 / −17 | 15 |
| 5 | K | L | fox | 25 | `FitsAll` | 73.659999 | 0.613347 | 120 / −10 | 0 |
| 6 | K | L | fox | 15 | `FitsBRP` | 73.775410 | 1.069619 | 120 / −17 | 5 |
| 7 | K | XL | krampus | 0 | `FitsGrip` | 76.852325 | 0.741170 | 90 / −17 | 15 |
| 8 | K | L | krampus | 0 | `FitsGrip` | 72.694567 | 0.245436 | 120 / −10 | 15 |
| 9 | F | XL | sid | 20 | `FitsGrip` | 68.479876 | 0.578741 | 120 / −30 | 0 |
| 10 | F | XL | lithic | 0 | `FitsGrip` | 67.571942 | 0.811560 | 120 / −30 | 0 |
| 11 | F | XL | sid | 25 | `FitsGrip` | 64.472103 | 0.483405 | 120 / −30 | 0 |
| 12 | F | XL | sid | 15 | `FitsGrip` | 63.915801 | 0.817186 | 120 / −30 | 0 |
| 13 | F | L | sid | 25 | `FitsNeither` | 58.827962 | 1.318877 | 120 / −20 | 0 |
| 14 | F | L | sid | 20 | `FitsNeither` | 58.531310 | 1.616431 | 120 / −20 | 0 |
| 15 | F | L | lithic | 0 | `FitsNeither` | 55.768206 | 1.764743 | 120 / −30 | 0 |
| 16 | F | L | sid | 15 | `FitsNeither` | 52.097664 | 1.767563 | 120 / −30 | 0 |

Also assert two fitted seatpost insertions (±1e-6): row 1 `Build.Placement.Setup.Seatpost.InsertionMm == 163.906090`, row 15 `== 144.107252`; and every row's `Build.Placement.Setup.Saddle.RailOffsetMm == 15` (±1e-9, the rail limit). (Row 5 ranks above row 6 despite the lower total, because `FitsAll` ranks above `FitsBRP`; rows 13–16 are ordered by ascending `GripR`, Q90.)

#### T2. `TestRankRowsMatchFitBuilds`

For each row of `Rank(fixtureRequest())`, build the row's setup independently in the test (fixture size frame, its `DefaultLowerCup`, `ForkSetup{Spec, SagPct}`, the request's wheels, seatpost at `InsertionMm` = `assembly.SeatpostInsertionBounds(...).EffectiveMinInsertionMm`, saddle, stem, handlebar), call `assembly.FitBuilds(setup, req.Target, req.Parts, req.Weights, 1)`, and require `reflect.DeepEqual(row.Build, want.Builds[0])` and `row.Evaluated == want.Evaluated`. This is the Q61/Q67 agreement by construction.

#### T3. `TestRankAllHandlebars`

`req.Parts.Handlebars = []assembly.HandlebarSetup{persuaderBar(), sqlab311Bar()}`. Expect `RowCount == 16`, `Evaluated == 2240`, every row `Build.HasHandlebarOption == true`, and rows (last column is `HandlebarIndex`):

| # | Frame | Size | Fork | Sag | Tier | Total | GripR | Stem | Spacers | Bar |
| -: | --- | --- | --- | -: | --- | -: | -: | --- | -: | -: |
| 1 | K | XL | fox | 20 | `FitsAll` | 84.481849 | 0.773655 | 90 / −17 | 0 | 1 |
| 2 | K | L | fox | 20 | `FitsAll` | 80.065939 | 0.514084 | 110 / −17 | 0 | 1 |
| 3 | K | XL | fox | 15 | `FitsAll` | 79.151692 | 0.254290 | 110 / −17 | 5 | 0 |
| 4 | K | XL | fox | 25 | `FitsAll` | 79.110589 | 0.411071 | 90 / −17 | 0 | 1 |
| 5 | K | L | fox | 15 | `FitsAll` | 76.661843 | 0.915548 | 110 / −17 | 0 | 1 |
| 6 | K | L | fox | 25 | `FitsAll` | 74.677138 | 0.148898 | 110 / −17 | 0 | 1 |
| 7 | K | XL | krampus | 0 | `FitsGrip` | 77.253680 | 0.500357 | 90 / −17 | 10 | 1 |
| 8 | K | L | krampus | 0 | `FitsGrip` | 75.370042 | 0.931818 | 90 / −17 | 0 | 1 |
| 9 | F | L | sid | 20 | `FitsGrip` | 68.546221 | 0.538934 | 120 / −30 | 0 | 1 |
| 10 | F | XL | sid | 20 | `FitsGrip` | 68.479876 | 0.578741 | 120 / −30 | 0 | 0 |
| 11 | F | XL | lithic | 0 | `FitsGrip` | 67.571942 | 0.811560 | 120 / −30 | 0 | 0 |
| 12 | F | L | lithic | 0 | `FitsGrip` | 67.511970 | 0.836426 | 120 / −30 | 0 | 1 |
| 13 | F | L | sid | 25 | `FitsGrip` | 64.755971 | 0.313084 | 120 / −30 | 0 | 1 |
| 14 | F | XL | sid | 25 | `FitsGrip` | 64.472103 | 0.483405 | 120 / −30 | 0 | 0 |
| 15 | F | XL | sid | 15 | `FitsGrip` | 63.915801 | 0.817186 | 120 / −30 | 0 | 0 |
| 16 | F | L | sid | 15 | `FitsGrip` | 63.873308 | 0.842682 | 120 / −30 | 0 | 1 |

#### T4. `TestRankPrunes`

Table over `(TopPerFrame, Top)` on `fixtureRequest()`; each case also asserts `RowCount == 16` and `Evaluated == 1120`. Rows are named by T1's `#`:

| `TopPerFrame` | `Top` | Rows |
| -: | -: | --- |
| 1 | 0 | T1 #1, #9 |
| 0 | 3 | T1 #1, #2, #3 |
| 2 | 3 | T1 #1, #2, #9 |
| 1 | 1 | T1 #1 |
| 0 | 100 | all 16, as T1 |

#### T5. `TestRankSelections`

Table on `fixtureRequest()` with `Selections` set (`sag(v)` returns `&v`):

| Case | Selections | `TopPerFrame` | `RowCount` | Rows |
| --- | --- | -: | -: | --- |
| one fork, open sag | `{K, "XL", fox, nil}` | 0 | 3 | T1 #1, #3, #4 |
| one fork, all sizes | `{F, "", lithic, nil}` | 0 | 2 | T1 #10, #15 |
| frame only | `{K, "", "", nil}` | 0 | 8 | T1 #1–#8 |
| duplicates ranked once | `{K, "XL", fox, nil}`, `{F, "", lithic, nil}`, `{K, "XL", fox, nil}` | 0 | 5 | T1 #1, #3, #4, #10, #15 |
| per-frame pruning on a selection | `{K, "XL", fox, nil}`, `{F, "", lithic, nil}` | 1 | 5 | T1 #1, #10 |
| explicit sag | `{K, "L", fox, sag(22)}` | 0 | 1 | K L fox `SagPct 22`, `FitsAll`, total 77.053503, GripR 0.736472, stem 120 / −10, spacers 0 |

For the explicit-sag case also assert `Evaluated == 70` and the fitted `InsertionMm == 113.339366` (±1e-6).

#### T6. `TestRankRejectsInvalidRequests`

Table: each case mutates a fresh `fixtureRequest()` and requires `Rank` to return an error whose message **contains** the given text.

| Case | Mutation | Error contains |
| --- | --- | --- |
| no frames | `Frames = nil` | `compare needs at least one frame` |
| empty frame ID | `Frames[0].ID = ""` | `frame 0 has an empty ID` |
| duplicate frame | `Frames[1].ID = "surly-krampus"` | `duplicate frame "surly-krampus"` |
| no sizes | `Frames[0].Sizes = nil` | `frame "surly-krampus" has no sizes` |
| no forks | `Frames[0].Forks = nil` | `frame "surly-krampus" has no forks` |
| empty size name | `Frames[0].Sizes[1].Name = ""` | `frame "surly-krampus" size 1 has an empty name` |
| duplicate size | `Frames[0].Sizes[1].Name = "L"` | `frame "surly-krampus" has duplicate size "L"` |
| empty fork ID | `Frames[0].Forks[1].ID = ""` | `frame "surly-krampus" fork 1 has an empty ID` |
| duplicate fork | `Frames[0].Forks[1].ID = "surly_krampus_483"` | `frame "surly-krampus" has duplicate fork "surly_krampus_483"` |
| CAD baseline fork | `Frames[0].Forks[0].Spec.Steerer = assembly.SteererCAD` | `is the CAD baseline fork` |
| no sag steps | `SuspensionSagPcts = nil` | `compare needs suspension sag steps` |
| no BRP | `Target.BRP = nil` | `compare needs a BRP target` |
| no HandGrip | `Target.HandGrip = nil` | `compare needs a HandGrip target` |
| fork options | `Parts.Forks = []assembly.ForkOption{{}}` | `part options must not list forks` |
| negative top per frame | `TopPerFrame = -1` | `top per frame must be 0 or more, got -1` |
| negative top | `Top = -1` | `top must be 0 or more, got -1` |
| unknown frame | `Selections = {{FrameID: "nope"}}` | `unknown frame "nope"` |
| unknown size | `{K, "M", "", nil}` | `frame "surly-krampus" has no size "M"` |
| unknown fork | `{K, "", "sid_100", nil}` | `frame "surly-krampus" has no fork "sid_100"` |
| sag without fork | `{K, "L", "", sag(20)}` | `selection on frame "surly-krampus" sets sag without a fork` |
| rigid fork with sag | `{K, "L", krampus, sag(5)}` | `frame "surly-krampus" size "L" fork "surly_krampus_483" at 5% sag:` and `rigid fork` |
| seatpost too deep for a size | `Seatpost.MinInsertionMm = 300; Seatpost.MaxInsertionMm = 320` | `frame "otso-fenrir-stainless" size "L" fork "lithic_carbon_mountain" at 0% sag:` |

(The last case: Krampus sizes accept insertion `[300, 320]`, and Fenrir L's published `290 mm` insertion cap rejects it, so the first failing row is Fenrir L with Lithic.)

### C3. New file `internal/reboot/compare/compare_internal_test.go` (`package compare`)

`TestCompareRowsTieBreaks`: build rows sharing one `Build` (`assembly.ScoredBuild{Score: assembly.BuildScore{Tier: assembly.FitsAll, Total: 50}}`) except where noted, and sort them with `slices.SortFunc(rows, compareRows)`:

| Name | `FrameID` | `sizeIndex` | `forkIndex` | `SagPct` | Build |
| --- | --- | -: | -: | -: | --- |
| `frameB` | `b` | 0 | 0 | 20 | shared |
| `size1` | `a` | 1 | 0 | 20 | shared |
| `fork1` | `a` | 0 | 1 | 20 | shared |
| `sag25` | `a` | 0 | 0 | 25 | shared |
| `sag15` | `a` | 0 | 0 | 15 | shared |
| `sag20` | `a` | 0 | 0 | 20 | shared |
| `better` | `z` | 9 | 9 | 0 | `Total: 60` |

Expected order by name: `better`, `sag20`, `sag15`, `sag25`, `fork1`, `size1`, `frameB`. (Fit rank first; then frame ID, size order, fork order, `|sag − 20|`, then lower sag.) Shuffle the input order (for example, reverse it) so the test doesn't pass by accident.

---

## Mechanics chosen by the planner (veto on review)

1. `compare` defines its own `Frame` / `Size` / `Fork` inputs rather than taking `catalog` types (DESIGN "Layout and boundaries": `compare` never imports `catalog`). Slice 13f's `query` adapts `catalog.Frames()` into them, applying `InstalledFrame` (Q67).
2. `Rank` rejects a fork with the CAD steerer sentinel: Q56 compares each frame's *supported* forks, and the CAD baseline fork is a chart hypothesis (Q89), not an installable fork.
3. Each row starts its seatpost at `EffectiveMinInsertionMm`; `FitBuilds`' saddle fit replaces it (`ApplyFitting` probes from the bounds, so the start never changes the result).
4. Duplicate selections are ranked once by exact `(FrameID, Size, ForkID, SagPct)`.
5. An explicit `SagPct` requires a `ForkID`; its range and the rigid-fork-zero rule stay with `assembly.ForkSetup.Validate`, reported through the row-context wrapper.
6. Any row error aborts `Rank` instead of skipping the row (Q83).
7. Names: `compare.Rank` → `compare.Ranking{Rows, RowCount, Evaluated}`, `compare.Row{FrameID, Size, ForkID, SagPct, Build, Evaluated}`, unexported `compareRows`, and `idealSagPct`. `TopPerFrame`/`Top` default values (`1`/`0` in all-frames mode, `0`/`0` with custom selections, Q87) are 13f's `query` job; the engine takes explicit values, with `0` meaning unlimited.

## Rules that bite

- `make boundary`: never use the words `price`, `pricing`, `cost`, `cheap`, `ledger`, or `chassis` in reboot file names, identifiers, comments, or tests (in any case or inside longer words, so not "cheapest" either).
- Import boundaries: `compare` and both its test files import only the standard library, `assembly`, and `pose`; never `catalog`, `query`, or `web`. `assembly_test` never imports `catalog`.
- Trig stays in `internal/reboot/pose`: `frameset.go` uses `pose.DecomposeAlong`; `compare.go` uses only `math.Abs` from `math`. Tests may use `math` freely.
- `exhaustive`: any new `switch` on an enum-like type (for example, `FitTier`) names every constant.
- Q16, Q86, Q104: no optimization. Every row calls `FitBuilds` with strict `PlaceBike` enumeration; don't hoist placement, cache, or parallelize.
- Q8: never use "search" for fitting in code, comments, or docs.
- Don't touch `web`, `catalog`, `cmd`, or `/bike` (Q105: the readouts aren't shown on `/bike` yet).
- Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Plain-language commit subjects matching the two planned commit titles.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 13e Milestones line.

## Verification

1. `make pre-commit` passes on each of the 2 commits individually.
2. `go list -deps ./internal/reboot/compare | grep -E 'reboot/(catalog|query|web)'` prints nothing.
3. `go test ./internal/reboot/compare/ -count=1 -v` passes and reports the package's run time (it should take well under a second: 16 rows × 70 or 140 combinations per test).
4. No live check: this slice changes no page, URL, or CLI output. Say so in the handoff.

## Out of scope

- `internal/reboot/query`, the `catalog` → `compare` adapter, the compare defaults, and `bikegeo-reboot compare [query]` JSON output (Slice 13f).
- `/` compare page, multi-frame `/bike`, the `/bike` readouts for the new methods, and `bikegeo-reboot bike` (Slice 14).
- Any change to fitting, scoring, `CompareFitRank`, `CompareScoredBuilds`, or catalog values.
- Performance work of any kind (Q86, Q104).
