# Slice 12b: closure allowances and unpublished wheelbases

Status: Planned 2026-11-20

**Size: Small** (one commit, 3 files). Builds on Slice 12a (commits `e5b3af5`, `083e0a3`, `4360b44`, `741670c`, `a88060c`). The planning agent offers to implement it directly.

## Goal

Make `assembly.CheckChartClosure` ready for the 19-frame port (12c) without touching the catalog yet:

1. `PublishedWheelbaseMm == 0` means the chart doesn't publish a wheelbase. The wheelbase check is skipped (Q57).
2. `FrameChart` carries an optional, documented `ClosureAllowance{AxleHeightMm, WheelbaseMm, Reason}` (Q65, Q71) that widens a check beyond its rounding budget. `CheckChartClosure` checks that every allowance is necessary and tight (Q93).

The Fenrir fixture path stays the same: it publishes a wheelbase, has no allowance, and `web.NewHandler` still runs the closure check at startup until 12c (Q91).

## Rulings this rests on

- **Q45:** budget method (unchanged).
- **Q57:** skip the wheelbase check when `PublishedWheelbaseMm == 0`.
- **Q65 / Q71:** `ClosureAllowance{AxleHeightMm, WheelbaseMm, Reason}`, with `Reason` required. `sklar-pbj` L is `AxleHeightMm: 0.4` (`−3.06` vs `±2.68 mm`). `esker-hayduke` S4 is `WheelbaseMm: 1.2` (`+5.42` vs `±4.31 mm`) and is the explicit acceptance test.
- **Q93:** allowances are per size (per `FrameChart`). `CheckChartClosure` rejects one that isn't needed, or one more than `0.5 mm` beyond the measured excess.

## Clean defaults (veto on review)

- `PublishedWheelbaseMm` that is negative or NaN is still rejected. Only exactly `0` means unpublished.
- When the wheelbase is unpublished, the `Wheelbase` precision column is skipped (no nudge). A non-zero `Precision.WheelbaseMm` is allowed and ignored there, because Q80's legacy-inferred literals give `0.5` even for frames with no published wheelbase.
- Each check reports at most one error, axle height first, joined as today. Messages for charts without an allowance are byte-for-byte what they are today.
- The allowance tolerance is one named constant, `closureAllowanceSlackMm = 0.5`.
- `Reason` is free text and appears in no error message. It's documentation in the data, checked only for being non-blank.

## Changes

### `internal/reboot/assembly/frameset.go`

- `FrameChart` gains `Allowance ClosureAllowance` after `Precision`, with the comment `// documented widening beyond rounding for this size only (Q65, Q71, Q93)`.
- `PublishedWheelbaseMm` comment: `0` means unpublished, and the wheelbase closure check is skipped (Q57).

### `internal/reboot/assembly/chart_closure.go`

```go
// ClosureAllowance widens one size's closure checks beyond what rounding of
// its published columns explains, for a chart that is known not to close
// (Q65, Q71). Each non-zero field must be needed: CheckChartClosure rejects an
// allowance on a check that passes on rounding alone, or one more than
// closureAllowanceSlackMm beyond that check's excess (Q93). Reason says why the
// chart doesn't close and is required whenever a field is non-zero.
type ClosureAllowance struct {
	AxleHeightMm float64
	WheelbaseMm  float64
	Reason       string
}

// closureAllowanceSlackMm is how far an allowance may exceed the excess it
// covers (legacy's tolerance), so allowances can be round numbers.
const closureAllowanceSlackMm = 0.5
```

`CheckChartClosure(chart FrameChart) error` keeps its signature. In order:

1. **Input validation** (after the existing frame, fork, and headset checks), each returning immediately:
   - Wheelbase: `!isFinite(wb) || wb < 0` → `published wheelbase must be finite and non-negative (0 = unpublished), got %v mm`.
   - Allowance fields, axle height first: `!isFinite(a) || a < 0` → `closure allowance: axle height must be finite and non-negative, got %v mm` (and the same with `wheelbase`).
   - Any field non-zero and `strings.TrimSpace(Reason) == ""` → `closure allowance: a non-zero allowance needs a reason`.
   - Both fields zero and `Reason` not blank → `closure allowance: a reason without an allowance`.
   - `WheelbaseMm > 0` with `PublishedWheelbaseMm == 0` → `closure allowance: wheelbase allowance on a chart with no published wheelbase`.
   - The existing precision half-width validation stays where it is.
2. **Budgets:** unchanged, except that the `Wheelbase` column is skipped when the wheelbase is unpublished.
3. **Per check** `i` (axle height, then wheelbase; wheelbase skipped when unpublished), with `r = |residual|`, `b = budget`, `a = allowance` and `ε = closureEpsilonMm`:
   - `a > 0 && r ≤ b + ε` → `closure allowance: %s allowance %.2f mm is unnecessary: residual %+.2f mm is within its ±%.2f mm rounding budget`.
   - `a > 0 && a > (r − b) + closureAllowanceSlackMm + ε` → `closure allowance: %s allowance %.2f mm exceeds the %.2f mm needed beyond rounding by more than 0.50 mm`.
   - `r > b + a + ε` → today's message. When `a > 0`, `rounding budget` becomes `rounding budget plus its %.2f mm allowance`, followed by the same term list.

Update the `CheckChartClosure` doc comment for the unpublished wheelbase and the allowance.

### `internal/reboot/assembly/chart_closure_test.go`

Add two in-package test charts. Their values are copied from the legacy charts in `internal/frames` (published), and their precision is the legacy-inferred grid (Q80). Note this in a comment. No legacy import.

```go
// haydukeS4Chart is esker-hayduke S4 (published chart, 12 mm EC44 CAD cup,
// CAD fork pike_120 @ 30% sag → 495 mm effective, 44 mm rake).
FrameSetup{ReachMm: 500, StackMm: 645, HeadTubeAngleDeg: 67.5, SeatTubeAngleDeg: 74,
	HeadTubeLengthMm: 140, BbDropMm: 65, ChainstayMm: 440, SeatTubeLengthMm: 530,
	LowerBore: Bore44Pressed, DefaultLowerCup: {LowerCupEC44, 12}}
CadLowerCup {LowerCupEC44, 12}; CadFork {A2cMm: 495, RakeMm: 44}; PublishedWheelbaseMm: 1218

// sklarLChart is sklar-pbj L (published chart, 12 mm EC44 CAD cup, CAD fork
// sklar_steel_485, 52 mm rake).
FrameSetup{ReachMm: 475, StackMm: 641, HeadTubeAngleDeg: 69, SeatTubeAngleDeg: 74,
	HeadTubeLengthMm: 140, BbDropMm: 68, ChainstayMm: 432, SeatTubeLengthMm: 505,
	LowerBore: Bore44Pressed, DefaultLowerCup: {LowerCupEC44, 12}}
CadLowerCup {LowerCupEC44, 12}; CadFork {A2cMm: 485, RakeMm: 52}; PublishedWheelbaseMm: 1182
```

Both use precision `{Reach 0.5, Stack 0.5, HTA 0.25, HTL 0.5, BB drop 0.5, Chainstay 0.5, Wheelbase 0.5}` and no allowance.

Expected values come from an independent Python forward walk with its own budget loop. It reproduces today's Fenrir test values exactly (`+0.22 / ±2.09`, `−0.34 / ±3.02`):

| Chart | Check | Residual | Budget | Excess | Terms |
| --- | --- | ---: | ---: | ---: | --- |
| Hayduke S4 | wheelbase | `+5.41926` | `±4.30931` | `+1.10995` | Reach 0.500, HTA 2.537, HTL 0.191, BB drop 0.075, Chainstay 0.506, Wheelbase 0.500 |
| Hayduke S4 | axle height | `−0.91200` | `±2.72520` | passes | — |
| Sklar L | axle height | `−3.05560` | `±2.68015` | `+0.37544` | Stack 0.500, HTA 1.213, HTL 0.467, BB drop 0.500 |
| Sklar L | wheelbase | `−3.55890` | `±4.28160` | passes | — |

Tests (exact error strings; `nil` means passes):

- **T1 `TestCheckChartClosureHaydukeS4Allowance`** (the Q65 acceptance test), table-driven over `Allowance`:
  - none → `chart closure: wheelbase residual +5.42 mm exceeds its ±4.31 mm rounding budget (Reach 0.50 mm, HTA 2.54 mm, HTL 0.19 mm, BB drop 0.07 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)`
  - `{WheelbaseMm: 1.2, Reason: "..."}` → `nil`
  - `{WheelbaseMm: 1.0, Reason: "..."}` → `chart closure: wheelbase residual +5.42 mm exceeds its ±4.31 mm rounding budget plus its 1.00 mm allowance (Reach 0.50 mm, HTA 2.54 mm, HTL 0.19 mm, BB drop 0.07 mm, Chainstay 0.51 mm, Wheelbase 0.50 mm)`
  - `{WheelbaseMm: 1.6, …}` → `nil` (the tightness edge is `1.60995`)
  - `{WheelbaseMm: 1.7, …}` → `closure allowance: wheelbase allowance 1.70 mm exceeds the 1.11 mm needed beyond rounding by more than 0.50 mm`
- **T2 `TestCheckChartClosureSklarLAllowance`:**
  - none → `chart closure: axle height residual -3.06 mm exceeds its ±2.68 mm rounding budget (Stack 0.50 mm, HTA 1.21 mm, HTL 0.47 mm, BB drop 0.50 mm)`
  - `{AxleHeightMm: 0.4, …}` → `nil`
  - `{AxleHeightMm: 0.3, …}` → `chart closure: axle height residual -3.06 mm exceeds its ±2.68 mm rounding budget plus its 0.30 mm allowance (Stack 0.50 mm, HTA 1.21 mm, HTL 0.47 mm, BB drop 0.50 mm)`
  - `{AxleHeightMm: 0.4, WheelbaseMm: 0.5, …}` → `closure allowance: wheelbase allowance 0.50 mm is unnecessary: residual -3.56 mm is within its ±4.28 mm rounding budget` (each field is checked on its own)
- **T3 `TestCheckChartClosureUnnecessaryAllowance`:** `fenrirChart()` with `{AxleHeightMm: 0.5, Reason: "..."}` → `closure allowance: axle height allowance 0.50 mm is unnecessary: residual +0.22 mm is within its ±2.09 mm rounding budget`.
- **T4 `TestCheckChartClosureInvalidAllowance`**, all on `fenrirChart()`:
  - `{AxleHeightMm: -0.1, Reason: "x"}` → `closure allowance: axle height must be finite and non-negative, got -0.1 mm`
  - `{WheelbaseMm: NaN, Reason: "x"}` → `closure allowance: wheelbase must be finite and non-negative, got NaN mm`
  - `{AxleHeightMm: 0.5, Reason: "  "}` → `closure allowance: a non-zero allowance needs a reason`
  - `{Reason: "x"}` → `closure allowance: a reason without an allowance`
  - `PublishedWheelbaseMm: 0` with `{WheelbaseMm: 1.0, Reason: "x"}` → `closure allowance: wheelbase allowance on a chart with no published wheelbase`
- **T5 `TestCheckChartClosureUnpublishedWheelbase`**, all on `fenrirChart()` with `PublishedWheelbaseMm = 0`:
  - as is → `nil`
  - `StackMm = 643.95` → only `chart closure: axle height residual +2.17 mm exceeds its ±2.09 mm rounding budget (Stack 0.50 mm, HTA 0.62 mm, HTL 0.46 mm, BB drop 0.50 mm)`
  - `Precision = {}` → only `chart closure: axle height residual +0.22 mm exceeds its ±0.00 mm rounding budget (no rounded columns)`
- **T6:** update `TestCheckChartClosureInvalidWheelbase` to `{-1, NaN}`, matching substring `published wheelbase must be finite and non-negative (0 = unpublished)`. Zero is now valid and covered by T5.
- The existing Fenrir pass, budget-edge, precision, and nudge tests stay unchanged and must still pass byte for byte.

## Commit

One commit: `Accept documented closure allowances and unpublished wheelbases`, containing the three files above plus this plan's Status line and handoff and the DESIGN.md Milestones line.

## Rules that bite

- Never nudge a published chart value to make a test pass. The test charts are the legacy published numbers.
- Trig stays in `pose`. This slice adds no geometry; the closure check still reruns the forward recipe (Q45).
- No `price`/`cost`/`ledger`-family words (`make boundary`). Say "allowance", "budget", and "excess".
- Don't touch `internal/reboot/web` or the fixture. An unpublished wheelbase on `/bike` (the "Published … mm" readout) is a 12c/14 concern.
- Run `go vet ./... && go test ./...` after each change, and `make pre-commit` before committing.
- Finish with the Implementation handoff (DESIGN.md "Implementation handoff"): this file's Status line, an `## Implementation handoff` section, and the DESIGN.md Milestones line for 12b.

## Verification

- `make pre-commit` is clean.
- Live check: `make serve-reboot`, then confirm `/bike` still loads (the Fenrir startup closure check still passes). There are no visible changes.

## Out of scope

- `internal/reboot/catalog/`, porting any frame, the catalog closure test across all 19 frames, and dropping the `web.NewHandler` panic (all 12c).
- Rendering charts without a published wheelbase on `/bike`.
- Front-center checks (legacy had them; the reboot closure has never checked front center).
