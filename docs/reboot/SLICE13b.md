# Slice 13b: Stem catalog and frame-handling readouts

Status: Implemented 2026-11-20; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-20, Gemini 4 Hydrogen (Medium)
- **Commits:**
  - `11ecb6d` `Add trail and rear-center frameset readouts`
  - `eebbb1e` `Port the stem catalog with composite stem scores`
- **Done and not done:** A1, A2, W1, W2, W3, C1, C2 all done.
- **Deviations:** None.
- **Verification:** `make pre-commit` passed on both commits (`go test -v ./internal/reboot/catalog/...` and `make boundary` also verified). Live check via `make serve-reboot` on `http://127.0.0.1:8081/bike`: default Lithic showed `Trail: 94.1 mm`, `Rear Center: 444.9 mm`, `RC / WB: 38.3%`; switching fork to Fox Float 34 120mm at 20% sag showed `Trail: 110.0 mm`, `Rear Center: 445.6 mm`, `RC / WB: 38.5%`. Screenshot: `/Users/matt/.gemini/antigravity/brain/22137c5a-8bfe-4f08-b250-7f2791d5b0cd/.system_generated/steps/20/media_0.png`.
- **Open issues:** None.

**Rulings:** Q58, Q64 (stem catalog and composite score), Q96 (split: 13b is data and readouts only; scoring is 13c), Q72 (physical modeling in scope), AGENTS.md trig rule. Q97–Q99 apply to 13c, not here.

**Size:** larger (7 code/test/template files, 2 commits). Written for a lower-tier implementer. **Don't make design choices.** If something below doesn't match the code or a test value disagrees beyond its tolerance, stop and record `Blocked <date> on <question>` (see "Rules that bite").

## Goal

1. `FramesetPlacement` gains three methods: `TrailMm`, `RearCenterMm`, and `RcWbRatioPct`. `/bike` shows them as readouts. Slice 13c scores these attributes.
2. `internal/reboot/catalog/stems.go` ports the 107 stems in `docs/reboot/stems.yaml` as Go values, with each stem's Q58 composite `0–100` score. Nothing outside tests calls `catalog.Stems()` until 13c (`deadcode` is disabled during the reboot).

There are no scoring, ranking, stock fitting, or URL changes. Every existing test value stays the same.

## Commit 1: `Add trail and rear-center frameset readouts`

### A1. `internal/reboot/assembly/frameset.go`

Add these right after `LeveledBbDropMm`:

```go
// TrailMm returns the ground trail: the horizontal distance on the ground from
// the front tire contact back to where the leveled steering axis meets the
// ground. Positive when the contact sits behind that point.
func (f FramesetPlacement) TrailMm() float64 {
	// Split HeadTubeTop → FrontTireContact into a run down the steering axis
	// and a horizontal remainder. The steering axis meets the ground (the
	// contact's height) at HeadTubeTop plus the run, so the remainder runs
	// from that point to the contact, and trail is its negation.
	steererDown := f.HeadTubeTop.HeadingDeg() + 180.0
	_, ahead, ok := pose.DecomposeAlong(f.HeadTubeTop.OffsetTo(f.FrontTireContact), steererDown, 0)
	if !ok {
		return math.NaN() // a horizontal steering axis never meets the ground
	}
	return -ahead
}

// RearCenterMm returns the horizontal distance from the rear axle to BB on the
// leveled bike (the RC in RC / WB).
func (f FramesetPlacement) RearCenterMm() float64 {
	return f.RearAxle.OffsetTo(f.BB).X
}

// RcWbRatioPct returns RearCenterMm as a percentage of WheelbaseMm.
func (f FramesetPlacement) RcWbRatioPct() float64 {
	return 100.0 * f.RearCenterMm() / f.WheelbaseMm()
}
```

- `frameset.go` already imports `pose` and `math`. `math.NaN` is allowed outside `pose`: no trig, no radians.
- Don't restate any trail formula (`R·cos(HTA)…`) in non-test code. The method reads the placed poses (DESIGN "Any scalar derived from placed poses is a method on a placement type").
- The `!ok` branch needs a leveled steering axis within `1e-12` of horizontal. `validateFrameGeometry` keeps HTA in `(0°, 180°)`, so only an absurd leveling pitch could get there, and no real frame does. It needs no test.

### A2. `internal/reboot/assembly/frameset_test.go`

Add `TestFramesetHandlingReadouts`, a table test using the existing `fenrirSizeLFrame()`, `fenrirWheels()`, `fenrirEc44Cup()`, `fenrirCadCup()`, and `assertNear`.

| Case | Lower cup | Fork | `TrailMm` | `RearCenterMm` | `RcWbRatioPct` | `WheelbaseMm` (cross-check) |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| Lithic (rigid) | `fenrirEc44Cup()` | `{StaticA2cMm: 495, RakeMm: 54, Steerer: SteererTapered}`, sag 0 | `94.123289` | `444.860145` | `38.278721` | `1162.160430` |
| Fox 34 at 20% | `fenrirEc44Cup()` | `{StaticA2cMm: 530, TravelMm: 120, RakeMm: 44, Steerer: SteererTapered}`, sag 20 | `109.963183` | `445.646528` | `38.511960` | `1157.163974` |
| CAD baseline | `fenrirCadCup()` | `{StaticA2cMm: 495, RakeMm: 54, Steerer: SteererCAD}`, sag 0 | `90.197878` | `444.189213` | `38.369496` | `1157.662365` |

- Tolerance: `1e-5` (the probe prints 6 decimals).
- Also assert `RcWbRatioPct == 100·RearCenterMm/WheelbaseMm` to `1e-12`, and `RearCenterMm == −RearAxle.Origin().X` to `1e-12` (BB is at the origin).
- **Provenance:** the expected values come from an independent closed-form probe ([readouts.py](file:///Users/matt/.gemini/antigravity/brain/1a9688ee-9e72-4387-aec7-7bfab620583f/scratch/readouts.py)). It places the unpitched chart points, levels the axles, rotates about BB, and computes `trail = (R·cos(HTA_lev) − rake) / sin(HTA_lev)`, with `R = 372`. The Lithic wheelbase `1162.160430` matches `/bike`'s existing `1162.2 mm` readout. Put a one-line comment in the test naming that formula as the probe; independent `math` is allowed in tests.

### W1. `internal/reboot/web/web.go`

- Add `TrailMm`, `RearCenterMm`, and `RcWbRatioPct float64` to `bikeView`, next to `WheelbaseMm` and `BbDropLeveledMm`.
- Fill them in the same place as `WheelbaseMm: placed.WheelbaseMm()` (around line 1579): `placed.TrailMm()`, `placed.RearCenterMm()`, `placed.RcWbRatioPct()`. `renderBikeError` doesn't render readouts and needs no change.

### W2. `internal/reboot/web/templates/bike.html`

In the `.summary` block, right after the `Leveled BB Drop` span, add:

```html
<span>Trail: <strong class="mono" data-metric="trail">{{printf "%.1f mm" .TrailMm}}</strong></span>
<span>Rear Center: <strong class="mono" data-metric="rear-center">{{printf "%.1f mm" .RearCenterMm}}</strong></span>
<span>RC / WB: <strong class="mono" data-metric="rc-wb">{{printf "%.1f%%" .RcWbRatioPct}}</strong></span>
```

### W3. `internal/reboot/web/web_test.go`

Add `TestBikeWorkbenchFrameHandlingReadouts`, a table test using `loadBikeDoc` and `metricText`:

| Path | `trail` | `rear-center` | `rc-wb` |
| --- | --- | --- | --- |
| `/bike` (default Lithic, 12 mm EC44 cup) | `94.1 mm` | `444.9 mm` | `38.3%` |
| `/bike?fork=fox34_120&sag=20` | `110.0 mm` | `445.6 mm` | `38.5%` |
| `/bike?fork=cad` | `90.2 mm` | `444.2 mm` | `38.4%` |

These are the A2 probe values, rounded. If the `fox34_120` query renders an error page, check whether the form needs other keys (for example the fork's cup) before changing anything, and stop if it does.

## Commit 2: `Port the stem catalog with composite stem scores`

### C1. `internal/reboot/catalog/stems.go` (new)

Exported API:

```go
// LengthAvailability is how widely trail/XC-rated stems of one length are sold (stems.yaml).
type LengthAvailability string

const (
	LengthAbundant LengthAvailability = "abundant"
	LengthCommon   LengthAvailability = "common"
	LengthLimited  LengthAvailability = "limited"
	LengthSparse   LengthAvailability = "sparse"
)

// AngleAvailability is how widely one stem angle is sold at a given length (stems.yaml).
type AngleAvailability string

const (
	AngleDominant AngleAvailability = "dominant"
	AngleCommon   AngleAvailability = "common"
	AngleNiche    AngleAvailability = "niche"
)

// Stem is one concrete stem: a length and a signed angle (negative drops,
// matching assembly.StemSetup.AngleDeg), its availability, and its Q58
// composite 0–100 score.
type Stem struct {
	LengthMm           float64
	AngleDeg           pose.Deg
	LengthAvailability LengthAvailability
	AngleAvailability  AngleAvailability
	Score              float64
}

// Stems returns the 107 stems from docs/reboot/stems.yaml, ordered by length
// and then by signed angle, both ascending.
func Stems() []Stem
```

Unexported data mirrors `stems.yaml` so a reviewer can check it line by line:

```go
// stemAngle is one entry of a stems.yaml angle list. Reversible entries
// ("+/-X") are flip-flop stems sold as both −X° and +X°.
type stemAngle struct {
	Deg        pose.Deg
	Reversible bool
}

func flip(deg pose.Deg) stemAngle  { return stemAngle{Deg: deg, Reversible: true} }
func fixed(deg pose.Deg) stemAngle { return stemAngle{Deg: deg} }

// stemLength is one stems_by_length entry of docs/reboot/stems.yaml (catalog).
type stemLength struct {
	LengthMm                 float64
	Availability             LengthAvailability
	Dominant, Common, Niche []stemAngle
}

var stemLengths = []stemLength{
	{LengthMm: 40, Availability: LengthAbundant, Dominant: []stemAngle{fixed(0)}, Common: []stemAngle{flip(6)}},
	// … 50 through 120, transcribed from stems.yaml's represented_angles_deg
	// in its own order. `0` and fixed negatives like `-17` use fixed(...), and
	// "+/-X" uses flip(X).
}
```

`Stems()` builds the list:

1. For each `stemLength`, expand every angle in tier order dominant → common → niche. `flip(X)` gives `−X` and `+X`, and `fixed(X)` gives `X`. **Dedup by signed angle, keeping the first (higher) tier.** For example, at 70 mm `common: "+/-17"` and `niche: -17` give one `−17°` stem at `common`.
2. The mainstream span `[minMs, maxMs]` is the min and max signed angle tiered dominant or common at that length, after dedup. Every length has at least one, so `panic` if one doesn't (a catalog programming error).
3. Score every stem with `stemScore` and sort by `(LengthMm, AngleDeg)` ascending.

```go
// Q58 composite stem score sub-attribute weights (sum 100).
const (
	stemLengthAvailabilityWeight = 35.0
	stemAngleAvailabilityWeight  = 30.0
	stemLengthRoomWeight         = 20.0
	stemAngleRoomWeight          = 15.0
)

// stemScore returns the Q58 composite 0–100 score.
func stemScore(lengthMm float64, length LengthAvailability, angle pose.Deg, tier AngleAvailability, minMs, maxMs pose.Deg) float64
```

- **Length availability:** `abundant 100`, `common 80`, `limited 35`, `sparse 0`.
- **Angle availability:** `dominant 100`, `common 80`, `niche 0`.
- **Length room:** `100 × clamp(min(L − 40, 100 − L) / 20, 0, 1)`.
- **Angle room:** `100 × clamp(min(α − minMs, maxMs − α) / 10, 0, 1)`.
- `score = (35·lenAvail + 30·angAvail + 20·lenRoom + 15·angRoom) / 100`.
- Write the availability lookups as `switch` statements that name every constant, with a `default: panic(...)` (`make exhaustive`). This is plain arithmetic on `pose.Deg`, with no trig.

Update the package doc comment in `catalog.go` to list stems.

### C2. `internal/reboot/catalog/stems_test.go` (new, package `catalog_test`)

`TestStems`:

- `len(catalog.Stems()) == 107`. Counts per length: 40: 3, 50: 7, 60: 12, 70: 16, 80: 14, 90: 15, 100: 13, 110: 13, 120: 14.
- The list is sorted by `(LengthMm, AngleDeg)` and has no duplicate pair.
- Every `Score` is in `[0, 100]`, and the scores sum to `6229.25` (tolerance `1e-9`).
- **Full table:** the appendix below, held as a literal `[]struct{L float64; A pose.Deg; LA catalog.LengthAvailability; AA catalog.AngleAvailability; Score float64}` with 107 rows. Each row must match `Stems()[i]` exactly in length, angle, and both tiers, and match the score to `1e-9`.
- Name the Q58/Q64 anchor rows in a comment: `40 mm ±6° = 59.0`, `40 mm 0° = 74.0`, `50 mm ±6° = 69.0`, `50 mm 0° = 84.0`, `70 mm 0°/±6°/±7° = 100`, `80 mm ±6°/±7° = 100`, `120 mm −20°/±30° = 0`.

`TestStemsDedupKeepsHigherTier`: assert from `Stems()` that 70 mm −17° is `AngleCommon` (yaml lists `common: "+/-17"` and `niche: -17`), that 60 mm −17° appears once, as `AngleNiche`, and that 120 mm 0° is `AngleNiche`.

## Appendix: Independent stem probe (expected `TestStems` table)

Generated by [stems.py](file:///Users/matt/.gemini/antigravity/brain/1a9688ee-9e72-4387-aec7-7bfab620583f/scratch/stems.py), which parses `stems.yaml` directly and implements Q58 by hand. Format: `angle tier score`. `D`/`C`/`N` = dominant/common/niche.

| Length (availability) | Stems |
| --- | --- |
| 40 (abundant) | −6 C 59.0 · 0 D 74.0 · +6 C 59.0 |
| 50 (abundant) | −20 N 45.0 · −17 N 45.0 · −6 C 69.0 · −5 N 46.5 · 0 D 84.0 · +5 N 46.5 · +6 C 69.0 |
| 60 (abundant) | −30 N 55.0 · −20 N 55.0 · −17 N 55.0 · −12 N 55.0 · −7 C 79.0 · −6 D 86.5 · 0 D 95.5 · +6 D 86.5 · +7 C 79.0 · +12 N 55.0 · +17 N 55.0 · +30 N 55.0 |
| 70 (abundant) | −30 N 55.0 · −20 N 55.0 · −17 C 79.0 · −15 N 58.0 · −12 N 62.5 · −10 C 89.5 · −7 D 100 · −6 D 100 · 0 D 100 · +6 D 100 · +7 D 100 · +10 C 89.5 · +12 N 62.5 · +15 N 58.0 · +17 C 79.0 · +30 N 55.0 |
| 80 (abundant) | −30 N 55.0 · −20 N 55.0 · −17 C 79.0 · −12 N 62.5 · −10 C 89.5 · −7 D 100 · −6 D 100 · 0 C 94.0 · +6 D 100 · +7 D 100 · +10 C 89.5 · +12 N 62.5 · +17 C 79.0 · +30 N 55.0 |
| 90 (common) | −30 N 38.0 · −20 N 38.0 · −17 C 62.0 · −15 N 41.0 · −12 N 45.5 · −10 C 72.5 · −7 D 83.0 · −6 D 83.0 · 0 C 77.0 · +6 D 83.0 · +7 D 83.0 · +10 C 72.5 · +15 N 41.0 · +17 C 62.0 · +30 N 38.0 |
| 100 (common) | −30 N 28.0 · −20 N 28.0 · −17 C 52.0 · −12 N 35.5 · −10 C 62.5 · −7 D 73.0 · −6 D 73.0 · 0 C 67.0 · +6 D 73.0 · +7 D 73.0 · +10 C 62.5 · +17 C 52.0 · +30 N 28.0 |
| 110 (limited) | −30 N 12.25 · −20 N 12.25 · −17 C 36.25 · −12 N 19.75 · −10 C 46.75 · −7 D 57.25 · −6 D 57.25 · 0 C 51.25 · +6 D 57.25 · +7 D 57.25 · +10 C 46.75 · +17 C 36.25 · +30 N 12.25 |
| 120 (sparse) | −30 N 0 · −20 N 0 · −17 C 24.0 · −12 N 7.5 · −10 C 34.5 · −7 D 45.0 · −6 D 45.0 · 0 N 15.0 · +6 D 45.0 · +7 D 45.0 · +10 C 34.5 · +12 N 7.5 · +17 C 24.0 · +30 N 0 |

Mainstream spans: 40–50 mm `[−6, +6]`, 60 mm `[−7, +7]`, 70–120 mm `[−17, +17]`.

## Mechanics chosen by the planner (veto on review)

- The stem data mirrors `stems.yaml` row by row (`flip`/`fixed`), and `Stems()` derives the signed list and scores. This keeps the yaml audit trivial and puts the Q58 math in one place. Stems carry no IDs; 13c identifies a stem by `(LengthMm, AngleDeg)`.
- `RearCenterMm` is horizontal on the leveled bike (DESIGN Readouts table), not the chainstay length.
- The trail readout uses the front tire contact that `PlaceFrameset` already places, so it follows the installed fork, sag, and pitch.

## Rules that bite

- `make boundary`: none of the banned words in new identifiers, comments, or test names (`price`, `pricing`, `cost`, `cheap`, `ledger`, `chassis`). Call the stem number its **score**.
- Trig stays in `pose`: `stems.go` uses no `math` trig, and `TrailMm` uses only `pose.DecomposeAlong` and pose methods.
- `catalog` imports `assembly` and `pose` only. `assembly` never imports `catalog`.
- Published or catalog values aren't nudged. The stem list is a transcription of `stems.yaml`; if a probe row disagrees, fix the transcription, never the expected table.
- Run `go vet ./... && go test ./...` after each change and `make pre-commit` before each commit (re-run if `go fix` changes anything).
- Commit subjects are plain language, exactly as above.
- **Implementation handoff:** finish by recording the handoff in this file per DESIGN.md "Implementation handoff": update this Status line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on DESIGN.md's Slice 13b Milestones line.

## Verification

1. `make pre-commit` passes on each commit.
2. Live check: `make serve-reboot`, then open `http://127.0.0.1:8081/bike`. The summary shows `Trail: 94.1 mm`, `Rear Center: 444.9 mm`, and `RC / WB: 38.3%`. Switch the fork to Fox 34 at 20% sag and confirm `110.0 mm` / `445.6 mm` / `38.5%`. Record a screenshot path in the handoff, or say why you skipped it.

## Out of scope

- Any scoring change: `PenaltyBand`, `CockpitPreferences`, weights, tiers, and the stock table all stay as they are (13c).
- Wiring `catalog.Stems()` into `CockpitStock` or the workbench (13c, Q97).
- A submitted-build score readout (Q98, Slice 14).
- `LeveledReachMm`, `LeveledStackMm`, `EttMm` (13d).
- Changing `StemSetup.MinLengthMm`/`MaxLengthMm`, default stems, or the default seatpost.
