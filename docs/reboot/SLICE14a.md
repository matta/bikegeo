# Slice 14a: Multi-frame `/bike`, shared bike query, and `bike` JSON CLI

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-21, Claude Sonnet 5.5 (`claude-sonnet-5-5`).
- **Commits:**
  1. `Add catalog frame defaults and stem lookup` — `5af1fe2`
  2. `Add bike query parsing and evaluation in query` — `a6a2c0b`
  3. `Add bikegeo-reboot bike JSON CLI subcommand` — `1aac48d`
  4. `Support all catalog frames and defaults on /bike` — `b52224c`
- **Done and not done:** C1–C4, Q1–Q2, M1–M2, W1–W4 are all done. Nothing is left undone.
- **Deviations:**
  1. **`GripR` probe corrected (Q2 `TestBikeEvaluate`, W4 `TestBikeSubmittedBuildScoreQ110`, M2 unaffected).** The plan lists default-bike `Score.GripR ≈ 9.067441` and `submitted-oval-r = "9.07"`. The code returns `9.461226` / `"9.46"`. I recomputed it independently from the placed HandGrip `(389.217618, 755.120167)` against the `(481, 726)` target on the Q17 oval (±10 × ±20 mm, major axis 60°): `9.461226`. Every other probe in the plan (totals, BRP misses, insertions, reach/stack/ETT, 749 and 5243 evaluated builds, all score rows) matched as written, so I treat `9.067441` as a plan typo and used the verified value. This is not a design choice; Matt should confirm.
  2. **`parseFloatOrDefault` did not exist** in `compare.go` (the plan says to reuse `parseTarget`, `checkKnownKeys`, `checkSingleValueKeys`, which do exist). I added `parseFloatOrDefault(q, key, def, label)` to `bike.go` on top of the existing `parseFiniteFloat`.
  3. **`fork=cad` where max chainstay equals the published chainstay:** the plan's `SlidingDropoutsNote` cases cover physical-fork (both) and `fork=cad` (differs) only. For `fork=cad` with equal values I used the "matching published chart" wording. No catalog frame currently renders this case in a test.
  4. **Error-page plumbing (W2):** the plan fixes the echo rules but not the mechanism. `web/bike_page.go` resolves the page structure on a `ParseBike` failure by dropping the rejected key(s) and re-parsing (`sanitizedBike`), echoes the submitted text through `echoForm`, and builds the CAD Baseline Fork preset from the resolved frame, size, bar, post, targets, and options with every other input at its default (`presetBase`), as the old workbench did. `DefaultsURL` keeps the (valid) view.
  5. **New `submitted-table` class** for the Q110 attribute table (plus one CSS selector), instead of reusing `build-table`, so `TestBikeWorkbenchRanksBuilds`'s `table.build-table thead th` count of 23 still holds.
  6. **`web_test.go` migration choices not spelled out in W4:** added `legacyPost`/`legacyBuild` constants for tests that pin numbers for the old `setback_0` at `180.576`; `TestWorkbenchStateCodecRoundTrip` became `TestFormatFormFloat` and `TestWorkbenchValidationAndCadBaselineForkInvariant` became `TestBikeCadBaselineForkURLDecodesToChartFork` (URL round-trip coverage now lives in `bike_test.go`); `TestBikeWorkbenchHalfEnteredTargetIsQuietlyIgnored` became `…IsRejected`; the three retired-key tests and `…IgnoresRetiredPostExtKey` collapsed into `TestBikeWorkbenchRejectsUnknownAndRetiredKeys`; `TestBikeWorkbenchDefaultSaddleHeightQ92` became `…DefaultInsertionMidpointQ109`. The form label count is now 21 (frame and size added). Expected values the plan did not probe were taken from the new code: `brp-grip-dist = "548.9 mm"` (independently recomputed: 548.86) and the seat close-up viewBox `"-263.50 -704.33 137.14 82.06"` (framing only, not independently probed). The whole-bike and stem viewBoxes did not change.
- **Verification:**
  - `make pre-commit` passed on each of the four commits, and a re-run on the final state leaves the tree unchanged.
  - `go list -deps ./internal/reboot/query | grep -E '(^net/http$|^html/template$|reboot/web)'` prints nothing.
  - `go test ./internal/reboot/catalog ./internal/reboot/query ./cmd/bikegeo-reboot ./internal/reboot/web -count=1` passes.
  - Live CLI (`make build-reboot`): `bike` → exit 0, `FrameID otso-fenrir-stainless`, `Size L`, `PostID redshift_shockstop_pro`, `InsertionMm 175`, `Score.Total 53.142123`, `Builds.Evaluated 749`; the Krampus XL winning query → `fits_all`, `Score.Total 84.575011`, equal to `Builds.Builds[0]`; `bike 'fram=surly-krampus'` → exit 2, `unknown query parameter "fram"`.
  - Live web (`serve --port 8081`, in-app browser): `/bike` opens on Otso Fenrir Stainless (2024, Size L) with Redshift post, the Q78 note, default idmatch targets, Submitted build score `53.1` / `Fits neither` / Oval R `9.46` / BRP miss `(-64.1, +29.8) mm`, Leveled Reach/Stack/ETT `421.9 / 646.0 / 598.5 mm`, and 5 ranked builds. Switching Frame to Esker Hayduke through the form gave sizes `S1`–`S4` (`S3` selected), fork `pike_120`, A2C `531.0`, sag `20.0`, and the sliding-dropout note (454 mm vs published 440 mm) with no error and a score of `52.8`. Clear Targets removed the fitting, score, and ranking blocks and the Clear Targets link, blanked all four target boxes, and the cleared state survived a `stem_length=80` form edit, the Seat view link (`brp=off&grip=off`), and a reload. No screenshot was saved to disk.
- **Open issues:** confirm deviation 1 (the `GripR` plan typo). Nothing else noticed or left alone.

### Independent review

- **Who and when:** 2026-11-21, independent review (code/tests/CLI/live browser + Lighthouse across 1440px, 1280px, 1024px, 390px).
- **Verification:** `make check` (fmt, vet, boundary, exhaustive, `go test ./...`), `go test -count=1 ./internal/reboot/catalog ./internal/reboot/query ./cmd/bikegeo-reboot ./internal/reboot/web`, `go list -deps ./internal/reboot/query` isolation check, and all three live CLI checks pass. No changes to `pose`, `assembly`, or `compare`; `/` still redirects to `/bike`; zero JS; zero trig outside `pose`.
- **Deviations audit:**
  - Deviation 1 (`GripR` `9.461226` vs. plan `9.067441`) is **confirmed as a plan typo**: independently recomputing Q17 oval radius from placed HandGrip `(389.217618, 755.120167)` against `(481, 726)` (semi-minor 10 mm, semi-major 20 mm, major heading 60°) gives `9.461225660` (`9.46`) and Total `53.142123480`.
  - Deviation 2 (`parseFloatOrDefault`), Deviation 5 (`submitted-table` class), and Deviation 6 (`web_test.go` migration; `brp-grip-dist` independently verified as `548.86 mm` → `"548.9 mm"`) are clean. Splitting `/bike` handler/form helpers into `internal/reboot/web/bike_page.go` is also clean.
  - Deviation 3 note inaccuracy: `otso-fenrir-stainless` (450 mm) and `salsa-fargo` (462 mm) both hit `fork=cad` with `MaxChainstayMm == published` (including `/bike?fork=cad` in `web_test.go`), though the chosen wording is accurate.
  - Deviation 4 (`sanitizedBike`/`echoForm` error-page plumbing) has a recovery bug (finding 2 below).
- **Findings to fix before acceptance:**
  1. **Switching off `fork=cad` keeps the locked CAD cup stack on 6 catalog frames (`internal/reboot/query/bike.go:354–366`):** `parseBikeFork` only resets `cup` to `defaultCup` when `prevCupStd != frameSetup.DefaultLowerCup.Standard`. Six frames (`binary-havok` 10→12, `binary-kinetik` 10→12, `seven-custom` 11→12, `seven-expedition-926d` 10→12, `seven-expedition-926f` 10→12, `tumbleweed-sunliner` 13→12) share `EC44` between `CadLowerCup` and `DefaultLowerCup` with different stack heights, so switching from `CAD Baseline Fork` to an installed fork silently carries the locked CAD cup instead of restoring the physical default promised by `cadBaselineForkNote`. Inherited from `SLICE14a.md` line 245; fix by always adopting `defaultCup` when `prevFork == CadForkID` and add a `bike_test.go` regression case.
  2. **400 error pages fail to recover cleanly on frame/fork switch (`internal/reboot/web/bike_page.go:167–175`):** On `/bike?frame=nope`, `echoForm` blanks `prev_frame` to `""` while the form renders Otso Fenrir's `Size` (`L`) and `Fork` (`lithic_carbon_mountain`) options and browsers display the first `<option>` (`Binary Havok`). Picking any valid frame other than Fenrir (or submitting as-is) sends `prev_frame=""`, so `frameSwitched` is `false` and `ParseBike` immediately returns a second `400` (`has no size`/`has no fork`). Same stale-carry applies to `prev_fork` on `?fork=nope`. Keep `base`'s resolved `Prev*` values (matching the dependent options actually rendered) when `raw(key) == ""`.
  3. **Unused function left behind (`internal/reboot/web/web.go:238–245`):** `parseDiagramView` is now completely unreachable (`go tool deadcode -test ./internal/reboot/web`) after `query.ParseBike` replaced it; delete it.
  4. **Missing `SetbackMm` assertions in `TestBikeEvaluate` (`internal/reboot/query/bike_test.go:264–265`):** Line 264 labels `"centered setback"` while asserting `InsertionMm` (`129.359870`), and neither `SetbackCentered` nor `SetbackReach` asserts `Spec.SetbackMm` (`65.948584` and `51.479453` from Q2).
  5. **Minor Commit-4 template spacing bug (`internal/reboot/web/templates/bike.html:370`):** `RedshiftMixedStateNote` is placed at the top of `Rear Cockpit` with `.field-note` (`margin: 0.6rem 0 0 0`, intended for notes below inputs), pushing it down below adjacent fieldsets and jamming it with `0` bottom margin against the `"Seatpost"` label. Move it below `.group-fields` (matching `cad-baseline-fork` and `sliding-dropouts`) or style with `.fit-note`.
- **Deferred to `/bike` UX overhaul (`DESIGN.md` `b8e0373`):** Lighthouse `td-has-header` failure on `.build-table`'s empty `Apply` `<th>`; table `<caption>`, `<th scope="col">`, and keyboard-scrollable region attributes on `.build-scroll`; semantic `<h3>` section headings instead of bare `<strong>`; unstyled/undisambiguated `Apply` links; static `<title>` across frames/tabs; `.control-groups`/`.group-fields` grid squeezing (select truncation at 1440px, orphan fieldset at 1280px, empty half-row beside `Size` and gap under `Saddle Rail Offset` at 1024px/390px); flat `.card` hierarchy, left-aligned numeric cells, mobile `<thead>` stretching, and missing scroll affordances; explicit invalid/placeholder `<option>` for rejected `<select>` controls on 400 pages; static inline styles in `bike.html`.
- **Verdict:** **Not quite ready for acceptance** — solid overall implementation of a large slice, with 5 small, low-risk fixes (findings 1–5 above) recommended before accepting.

**Rulings:** Q15, Q18, Q22, Q24, Q27–Q34, Q48, Q49a, Q53, Q54, Q56, Q57, Q58, Q59a, Q61–Q63, Q67, Q69, Q70, Q76, Q77, Q78, Q81, Q82, Q84, Q85, Q86, Q88, Q89, Q90, Q97, Q100, Q101, Q105, Q106, Q107, Q108–Q111; `AGENTS.md` rules.

**Builds on:** Slice 12c (commits `9942803`, `41f6123`, `425bdb8`, `0933899`) for `internal/reboot/catalog`, Slice 13b–13d (commits `11ecb6d`, `bf99568`, `9676745`, etc.) for stems, `0–100` scoring, and `PartOptions`/`BuildRanking`, Slice 13e (commits `f156358`, `068a628`, `ec2827f`) for `LeveledReachMm`/`LeveledStackMm`/`EttMm`, and Slice 13f (commits `c30af3b`, `cae0a01`, `801781b`) for `pose.Pose.MarshalJSON`, `internal/reboot/query`, and `cmd/bikegeo-reboot`.

**Size:** larger (12 code/test/template files, 4 commits). Written for a lower-tier implementer. **Don't make design choices.** If anything below doesn't match committed code, stop and record `Blocked <date> on <question>` (see "Rules that bite").

## Goal

1. Add small default/lookup helpers in `internal/reboot/catalog` (`DefaultFrameID`, `DefaultSuspensionSagPct`, `StemByDimensions`).
2. Add `internal/reboot/query/bike.go` (zero `net/http` or `html/template` imports) parsing `/bike` `url.Values` across all 19 catalog frames & 119 sizes, enforcing strict unknown-key and single-value validation (Q111), midpoint default `post_insert` and manual `frame`/`size`/`post` switch handling (Q109), durable `brp=off`/`grip=off` target clearing (Q70), lossless URL generation (Q77), and shared `Evaluate` with submitted-build scoring for catalog stems (Q110).
3. Add `bikegeo-reboot bike [query]` to `cmd/bikegeo-reboot` writing indented JSON to stdout via `encoding/json` (Q49a, Q88, Q106, Q110).
4. Wire `/bike` to `query.ParseBike` and `Evaluate`, delete the Fenrir-only `internal/reboot/web/workbench.go`, switch `/bike` to catalog defaults (`redshift_shockstop_pro` with Q78 mixed-state UI note, `persuader20_800`, default idmatch BRP & HandGrip targets with a Clear Targets link), render `Leveled Reach`/`Leveled Stack`/`ETT` (Q105) and the submitted-build score readout (Q110), handle sliding-dropout vs. CAD chainstay and unpublished wheelbases (Q57, Q67), and update `internal/reboot/web/web_test.go`.
5. Keep `/` redirecting to `/bike` until Slice 14b. Zero changes to `pose`, `assembly`, or `compare`.

---

## Commit 1: `Add catalog frame defaults and stem lookup`

### C1. `internal/reboot/catalog/frame.go`

Add right after `Frames()`:

```go
// DefaultFrameID is the default frame for /bike (Q109).
const DefaultFrameID = "otso-fenrir-stainless"
```

### C2. `internal/reboot/catalog/catalog.go`

Add right after `SuspensionSagPcts()`:

```go
// DefaultSuspensionSagPct is the default initial sag for a suspension fork on
// /bike (the Q32/Q59a ideal sag).
const DefaultSuspensionSagPct = 20.0
```

### C3. `internal/reboot/catalog/stems.go`

Add `"math"` to imports (only for `math.Abs`, `math.IsNaN`, `math.IsInf` — no trig) and add after `Stems()`:

```go
// StemByDimensions returns the catalog stem matching lengthMm and angleDeg
// within 1e-9, or false if the stem is off-catalog or non-finite (Q110).
func StemByDimensions(lengthMm float64, angleDeg pose.Deg) (Stem, bool) {
	if math.IsNaN(lengthMm) || math.IsInf(lengthMm, 0) ||
		math.IsNaN(float64(angleDeg)) || math.IsInf(float64(angleDeg), 0) {
		return Stem{}, false
	}
	const tol = 1e-9
	for _, s := range Stems() {
		if math.Abs(s.LengthMm-lengthMm) <= tol && math.Abs(float64(s.AngleDeg-angleDeg)) <= tol {
			return s, true
		}
	}
	return Stem{}, false
}
```

### C4. Catalog tests

- `internal/reboot/catalog/frames_test.go`: assert `catalog.FrameByID(catalog.DefaultFrameID)` returns `ok == true` with `ID == "otso-fenrir-stainless"` and `DefaultSize == "L"`.
- `internal/reboot/catalog/parts_test.go`: assert `catalog.DefaultSuspensionSagPct == 20.0`.
- `internal/reboot/catalog/stems_test.go`: add `TestStemByDimensions`:
  - Positive matches: `(60, -6)` → `ok == true`, `Score == 86.5`; `(70, -6)` → `ok == true`, `Score == 100.0`; `(100, -10)` → `ok == true`, `Score == 62.5`; `(120, -30)` → `ok == true`, `Score == 0.0`; `(70 + 1e-12, -6 - 1e-12)` → `ok == true`, `LengthMm == 70`, `AngleDeg == -6`.
  - Negative matches (enforced by `StemByDimensions`): `(65, -6)` → `ok == false`; `(70, -8)` → `ok == false`; `(134.2, -6)` → `ok == false`; `(math.NaN(), -6)` → `ok == false`; `(70, pose.Deg(math.Inf(1)))` → `ok == false`.

---

## Commit 2: `Add bike query parsing and evaluation in query`

### Q1. New file `internal/reboot/query/bike.go`

Imports: `fmt`, `net/url`, `slices`, `strconv`, `strings`, `bikegeo/internal/reboot/assembly`, `bikegeo/internal/reboot/catalog`, `bikegeo/internal/reboot/pose`.
**Never import `net/http`, `html/template`, or `internal/reboot/web`.**

Exported constants and types:

```go
// CadForkID is the reserved fork query value selecting the frame's CAD baseline
// fork and CAD lower cup (Q81, Q89).
const CadForkID = "cad"

// fork_options values (Q33, Q101).
const (
	ForkOptionsSelected = "selected"
	ForkOptionsAll      = "all"
)

// Diagram view values for /bike.
const (
	ViewBike = "bike"
	ViewSeat = "seat"
	ViewStem = "stem"
)

// RankedBuildLimit is the default number of ranked builds on /bike and CLI (Q18).
const RankedBuildLimit = 5

// BikeForkOption is one physical fork configuration a frame offers to build
// fitting when fork_options=all (Q31).
type BikeForkOption struct {
	FrameFork catalog.FrameFork
	SagPct    float64
	LowerCup  assembly.LowerCupSetup
}

// Bike holds the parsed, catalog-resolved /bike query and ready-to-place
// BikeSetup (Q88).
type Bike struct {
	Frame          catalog.Frame
	Size           catalog.FrameSize
	ForkID         string            // catalog Fork.ID or CadForkID
	FrameFork      catalog.FrameFork // zero when ForkID == CadForkID
	Bar            catalog.Handlebar
	Post           catalog.Seatpost
	View           string // ViewBike, ViewSeat, or ViewStem
	BrpTarget      *pose.Point
	GripTarget     *pose.Point
	AllBarOptions  bool
	AllForkOptions bool
	Setup          assembly.BikeSetup
}

// BikeScoredBuild pairs a ranked assembly.ScoredBuild with the build's catalog
// handlebar and fork IDs (Q88, Q106). Embedding ScoredBuild flattens its JSON
// fields.
type BikeScoredBuild struct {
	assembly.ScoredBuild
	HandlebarID string
	ForkID      string
}

// BikeBuildRanking is the catalog-resolved ranked builds list for /bike and
// bikegeo-reboot bike JSON output.
type BikeBuildRanking struct {
	Evaluated int
	Builds    []BikeScoredBuild
}

// BikeResult is a catalog-resolved single-bike evaluation for /bike and
// bikegeo-reboot bike JSON output (Q49a, Q88, Q106, Q110).
type BikeResult struct {
	FrameID         string
	Size            string
	ForkID          string
	BarID           string
	PostID          string
	AllBarOptions   bool
	AllForkOptions  bool
	Placement       assembly.BikePlacement
	Score           *assembly.BuildScore // non-nil when HandGrip target is set and stem is in catalog (Q110)
	Fitting         *assembly.Fitting    // non-nil when BRP or HandGrip target is set
	SetbackCentered *assembly.Fitting    // non-nil when BRP target is set (Q29)
	SetbackReach    *assembly.Fitting    // non-nil when BRP target is set and saddle fit is limited (Q29)
	Builds          *BikeBuildRanking    // non-nil when HandGrip target is set
}
```

Known keys (28 total, all single-valued):

```go
var knownBikeKeys = []string{
	"frame", "size",
	"fork", "fork_a2c", "cup", "sag",
	"bar", "spacers", "stem_length", "stem_angle", "bar_roll",
	"post", "post_insert", "rail_offset", "rail_pitch",
	"brp_x", "brp_y", "brp",
	"grip_x", "grip_y", "grip",
	"bar_options", "fork_options",
	"view",
	"prev_frame", "prev_size", "prev_fork", "prev_post",
}
```

Exported functions and methods:

1. **`FormatFloat(v float64) string`**:
   Returns `"0"` when `v == 0` (normalizing `-0.0`), else `strconv.FormatFloat(v, 'f', -1, 64)` (lossless float64 formatting per Q77).

2. **`DefaultBike() Bike`**:
   Returns the default `/bike` build (calls `ParseBike(nil)` — guaranteed valid by catalog invariants: `otso-fenrir-stainless`, size `L`, `lithic_carbon_mountain`, `EC44` `12 mm`, `495 mm` A2C, `0%` sag, `persuader20_800`, `bar_roll=0`, `redshift_shockstop_pro`, midpoint `post_insert=175`, `rail_offset=0`, `rail_pitch=0`, `DefaultStemSetup` (`60 mm, -6°, 20 mm spacers`), default idmatch BRP `(-219, 713.1)` & HandGrip `(481, 726)`, `AllBarOptions=false`, `AllForkOptions=false`, `View=ViewBike`).

3. **`ParseBike(q url.Values) (Bike, error)`**:
   Every rejection returns `*InputError` naming its URL query key. Exact validation order and named helpers:
   - **`checkKnownKeys(q, knownBikeKeys)` (Q111):** reuse from `compare.go`. Rejects any key not in the 28 known keys (sorted ascending so errors are deterministic).
   - **`checkSingleValueKeys(q, knownBikeKeys)`:** reuse from `compare.go`. Rejects any key appearing more than once.
   - **`parseBikeFrameAndSize(q)`:**
     - `frameID := strings.TrimSpace(q.Get("frame"))`; if `""`, `frameID = catalog.DefaultFrameID`.
     - `frame, ok := catalog.FrameByID(frameID)`; if `!ok` → `&InputError{Field: "frame", Err: fmt.Errorf("unknown frame %q", frameID)}`.
     - `prevFrame := strings.TrimSpace(q.Get("prev_frame"))`; `frameSwitched := prevFrame != "" && prevFrame != frame.ID`.
     - `rawSize := strings.TrimSpace(q.Get("size"))`:
       - If `rawSize == ""`: resolve `frame.Size(frame.DefaultSize)`.
       - Else if `sz, ok := frame.Size(rawSize); ok`: keep `sz` (even when `frameSwitched`, per Q109).
       - Else if `frameSwitched`: fall back to `frame.Size(frame.DefaultSize)` (Q109).
       - Else: `&InputError{Field: "size", Err: fmt.Errorf("frame %q has no size %q", frame.ID, rawSize)}`.
     - `prevSize := strings.TrimSpace(q.Get("prev_size"))`; `sizeSwitched := prevSize != "" && prevSize != size.Name`.
   - **`parseBikeFork(q, frame, size, frameSwitched)`:**
     - `rawFork := strings.TrimSpace(q.Get("fork"))`.
     - `prevFork := strings.TrimSpace(q.Get("prev_fork"))`.
     - Resolve fork ID:
       - If `rawFork == ""`: `ff = frame.Forks[0]`, `forkID = ff.Fork.ID`.
       - Else if `rawFork == CadForkID`: `isCad = true`, `forkID = CadForkID`.
       - Else if `found, ok := findFrameFork(frame, rawFork); ok`: `ff = found`, `forkID = ff.Fork.ID`.
       - Else if `frameSwitched`: `ff = frame.Forks[0]`, `forkID = ff.Fork.ID` (Q109).
       - Else: `&InputError{Field: "fork", Err: fmt.Errorf("frame %q has no fork %q", frame.ID, rawFork)}`.
     - `forkSwitched := !frameSwitched && prevFork != "" && prevFork != forkID`.
     - Determine `frameSetup`:
       - If `isCad`: `frameSetup = size.Chart.Frame` (published chainstay so CAD baseline fork reproduces the published chart with `ΔPitch = +0.00°`).
       - Else: `frameSetup = frame.InstalledFrame(size)` (max chainstay on sliding-dropout frames per Q67).
     - Resolve `lowerCup` and `forkSetup`:
       - If `isCad`: ignore any submitted `fork_a2c`, `cup`, `sag`; `lowerCup = size.Chart.CadLowerCup`; `forkSetup = assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: size.Chart.CadFork.A2cMm, TravelMm: 0, RakeMm: size.Chart.CadFork.RakeMm, Steerer: assembly.SteererCAD}, SagPct: 0}`.
       - Else (physical fork `ff`):
         - `defaultSag := 0.0`; if `ff.Fork.TravelMm != 0 { defaultSag = catalog.DefaultSuspensionSagPct }`.
         - `defaultCup := frameSetup.DefaultLowerCup.StackMm`.
         - If `frameSwitched`: adopt all three defaults (`a2cMm = ff.StaticA2cMm`, `cupMm = defaultCup`, `sagPct = defaultSag`), ignoring stale values from the previous frame (Q109).
         - Else if `forkSwitched`: adopt `a2cMm = ff.StaticA2cMm` and `sagPct = defaultSag`; determine `prevCupStd` from `prevFork` on `frame` (`CadForkID` → `size.Chart.CadLowerCup.Standard`, known frame fork → `frameSetup.DefaultLowerCup.Standard`, unknown → `""`); if `prevCupStd != frameSetup.DefaultLowerCup.Standard`, adopt `cupMm = defaultCup`, else parse `cup` with `parseFloatOrDefault(q, "cup", defaultCup, "lower cup")`.
         - Else: parse `fork_a2c` (default `ff.StaticA2cMm`), `cup` (default `defaultCup`), `sag` (default `defaultSag`) via `parseFloatOrDefault`.
         - Set `lowerCup = assembly.LowerCupSetup{Standard: frameSetup.DefaultLowerCup.Standard, StackMm: cupMm}`; `spec := ff.Spec(); spec.StaticA2cMm = a2cMm`; `forkSetup = assembly.ForkSetup{Spec: spec, SagPct: sagPct}`.
   - **Handlebar & Seatpost:**
     - `barID := strings.TrimSpace(q.Get("bar"))`; if `""`, `catalog.DefaultHandlebarID`. Unknown → `&InputError{Field: "bar", Err: fmt.Errorf("unknown handlebar %q", barID)}`.
     - `postID := strings.TrimSpace(q.Get("post"))`; if `""`, `catalog.DefaultSeatpostID`. Unknown → `&InputError{Field: "post", Err: fmt.Errorf("unknown seatpost %q", postID)}`.
     - `prevPost := strings.TrimSpace(q.Get("prev_post"))`; `postSwitched := prevPost != "" && prevPost != post.ID`.
   - **Seatpost insertion (`parseBikePostInsertion` — Q109):**
     - `bounds, err := assembly.SeatpostInsertionBounds(frameSetup, post.Spec)` (if error, return `&InputError{Field: "post", Err: err}`).
     - `defaultInsert := (bounds.EffectiveMinInsertionMm + bounds.MaxAllowedInsertionMm) / 2.0`.
     - Parse `post_insert` via `parseFloatOrDefault(q, "post_insert", defaultInsert, "seatpost insertion")`.
     - If `frameSwitched || sizeSwitched || postSwitched`: clamp `insertionMm = min(max(insertionMm, bounds.EffectiveMinInsertionMm), bounds.MaxAllowedInsertionMm)`.
   - **Cockpit & saddle floats (`parseFloatOrDefault`):**
     - `defaultStem := catalog.DefaultStemSetup()`
     - `spacers` (default `defaultStem.SpacersMm`, label `"spacers"`), `stem_length` (default `defaultStem.LengthMm`, label `"stem length"`), `stem_angle` (default `float64(defaultStem.AngleDeg)`, label `"stem angle"`), `bar_roll` (default `0`, label `"handlebar roll"`), `rail_offset` (default `0`, label `"saddle rail offset"`), `rail_pitch` (default `0`, label `"saddle rail pitch"`).
   - **Targets (`parseTarget`, reused from `compare.go` — Q53, Q70, Q76):**
     - `brp, err := parseTarget(q, "brp", "brp_x", "brp_y", "BRP target", catalog.DefaultBrpTarget())`
     - `grip, err := parseTarget(q, "grip", "grip_x", "grip_y", "HandGrip target", catalog.DefaultGripTarget())`
     - Both `nil`, either `nil`, or both non-nil are legal on `/bike`.
   - **Options & View (`parseBikeOptions`, `parseBikeView`):**
     - `bar_options`: `""` or `BarOptionsSelected` → `false`; `BarOptionsAll` → `true`; else `&InputError{Field: "bar_options", Err: fmt.Errorf("unknown handlebar options value %q", raw)}`.
     - `fork_options`: `""` or `ForkOptionsSelected` → `false`; `ForkOptionsAll` → `true`; else `&InputError{Field: "fork_options", Err: fmt.Errorf("unknown fork options value %q", raw)}`.
     - `view`: `""` or `ViewBike` → `ViewBike`; `ViewSeat` → `ViewSeat`; `ViewStem` → `ViewStem`; else `&InputError{Field: "view", Err: fmt.Errorf("unknown diagram view %q", raw)}`.

4. **Helper methods on `Bike`:**
   - `func (b Bike) FitTarget() assembly.FitTarget`: returns copies of `b.BrpTarget` and `b.GripTarget`.
   - `func (b Bike) ForkOptions() []BikeForkOption`: enumerates `b.Frame.Forks` in declared order (rigid once at `0%`, suspension at each `catalog.SuspensionSagPcts()`), each with `LowerCup: b.Frame.InstalledFrame(b.Size).DefaultLowerCup`.
   - `func (b Bike) WithSetup(setup assembly.BikeSetup) Bike`: returns a copy of `b` with `Setup = setup` (preserving frame, size, fork/bar/post IDs, view, targets, and options).
   - `func (b Bike) WithView(view string) Bike`: returns a copy of `b` with `View = view`.
   - `func (b Bike) WithCadBaselineFork() Bike`: returns a copy of `b` with `ForkID = CadForkID`, `FrameFork = catalog.FrameFork{}`, `Setup.Frame = b.Size.Chart.Frame`, `Setup.LowerCup = b.Size.Chart.CadLowerCup`, `Setup.Fork = assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: b.Size.Chart.CadFork.A2cMm, TravelMm: 0, RakeMm: b.Size.Chart.CadFork.RakeMm, Steerer: assembly.SteererCAD}, SagPct: 0}`, keeping `Setup.Seatpost`, `Setup.Saddle`, `Setup.Stem`, `Setup.Handlebar`, targets, options, and view.
   - `func (b Bike) WithoutTargets() Bike`: returns a copy of `b` with `BrpTarget = nil`, `GripTarget = nil`.
   - `func (b Bike) URL() string`: encodes `/bike?<query>` using `FormatFloat` for all floats:
     - Always sets `frame`, `size`, `fork`, `bar`, `post`, `fork_a2c`, `cup`, `sag`, `spacers`, `stem_length`, `stem_angle`, `bar_roll`, `post_insert`, `rail_offset`, `rail_pitch`, `view`.
     - If `b.BrpTarget != nil`: sets `brp_x`, `brp_y`; else sets `brp=off` (Q70).
     - If `b.GripTarget != nil`: sets `grip_x`, `grip_y`; else sets `grip=off` (Q70).
     - If `b.AllBarOptions`: sets `bar_options=all`.
     - If `b.AllForkOptions`: sets `fork_options=all`.
     - Never sets `prev_frame`, `prev_size`, `prev_fork`, or `prev_post`.

5. **`func (b Bike) Evaluate(limit int) (BikeResult, error)`**:
   - Places the bike: `placement, err := assembly.PlaceBike(b.Setup)`.
   - Initializes `res := BikeResult{FrameID: b.Frame.ID, Size: b.Size.Name, ForkID: b.ForkID, BarID: b.Bar.ID, PostID: b.Post.ID, AllBarOptions: b.AllBarOptions, AllForkOptions: b.AllForkOptions, Placement: placement}`.
   - If `b.BrpTarget == nil && b.GripTarget == nil`: return `res, nil`.
   - `target := b.FitTarget()`.
   - **Submitted-build score (Q110):** if `target.HandGrip != nil`:
     - If `stem, ok := catalog.StemByDimensions(b.Setup.Stem.LengthMm, b.Setup.Stem.AngleDeg); ok`:
       - `sc, err := assembly.ScoreBuild(placement, target, stem.Score, catalog.DefaultScoreWeights())` → `res.Score = &sc`.
     - Else: `res.Score = nil` (off-catalog stem).
   - **Continuous fitting & setback notes:**
     - `fit, err := assembly.ApplyFitting(b.Setup, target)` → `res.Fitting = &fit`.
     - If `target.BRP != nil`:
       - `centered, err := assembly.FitPostSetback(b.Setup, *target.BRP, 0)` → `res.SetbackCentered = &centered`.
       - If `slices.Contains(fit.LimitedBy, assembly.InputSeatpostInsertion) || slices.Contains(fit.LimitedBy, assembly.InputSaddleRailOffset)`:
         - `reach, err := assembly.FitPostSetback(b.Setup, *target.BRP, fit.Placement.Setup.Saddle.RailOffsetMm)` → `res.SetbackReach = &reach`.
   - **Ranked builds:** if `target.HandGrip != nil`:
     - `var offeredBars []catalog.Handlebar`, `var barSetups []assembly.HandlebarSetup`; if `b.AllBarOptions`, populate from `catalog.Handlebars()` (`BarRollDeg` stays `0`, `FitBuilds` installs `b.Setup.Handlebar.BarRollDeg` per Q24).
     - `var offeredForks []BikeForkOption`, `var forkSetups []assembly.ForkOption`; if `b.AllForkOptions`, populate from `b.ForkOptions()`.
     - Note: if `b.ForkID == CadForkID && b.AllForkOptions`, `b.Setup.Frame` is the published chart frame (`size.Chart.Frame`), whereas installed physical fork options on a sliding-dropout frame must use `b.Frame.InstalledFrame(b.Size)` (Q67)! Set `fitSetup := b.Setup; if b.AllForkOptions { fitSetup.Frame = b.Frame.InstalledFrame(b.Size) }`.
     - Call `ranking, err := assembly.FitBuilds(fitSetup, target, parts, catalog.DefaultScoreWeights(), limit)`.
     - Map each `assembly.ScoredBuild` into `BikeScoredBuild`, resolving `HandlebarID` (`b.Bar.ID` or `offeredBars[c.HandlebarIndex].ID`) and `ForkID` (`b.ForkID` or `offeredForks[c.ForkIndex].FrameFork.Fork.ID`) with bounds checks.
     - Set `res.Builds = &BikeBuildRanking{Evaluated: ranking.Evaluated, Builds: rows}`.
   - Return `res, nil`.

### Q2. New file `internal/reboot/query/bike_test.go`

Use `package query_test`. Cover every rule and every negative test case:

1. **`TestParseBikeDefaults`:** `ParseBike(nil)` and `ParseBike(url.Values{})` return `otso-fenrir-stainless`, size `L`, `lithic_carbon_mountain`, `EC44 12 mm`, `495 mm` A2C, `0%` sag, `persuader20_800`, `redshift_shockstop_pro`, `post_insert == 175.0`, `spacers == 20`, `stem_length == 60`, `stem_angle == -6`, `BRP == (-219, 713.1)`, `HandGrip == (481, 726)`, `AllBarOptions == false`, `AllForkOptions == false`, `View == "bike"`.
2. **`TestParseBikeAllFramesAndSlidingDropouts`:**
   - `frame=surly-krampus&size=XL&fork=fox34_120&sag=20` → `Frame.ID == "surly-krampus"`, `Size.Name == "XL"`, `ForkID == "fox34_120"`, `SagPct == 20`, `LowerCup == EC44 12 mm`, `InsertionMm == 175.0`.
   - `frame=binary-havok&size=L` (physical fork `rigid_485`) → `Setup.Frame.ChainstayMm == 460` (max chainstay per Q67), `LowerCup == EC44 12 mm`, `InsertionMm == 175.0`.
   - `frame=binary-havok&size=L&fork=cad&cup=99&fork_a2c=999&sag=50` → `Setup.Frame.ChainstayMm == 445` (published chainstay!), `LowerCup == EC44 10 mm`, `Fork.Spec.StaticA2cMm == 485`, `SagPct == 0`, `Steerer == assembly.SteererCAD` (overrides ignored).
   - `frame=salsa-fargo&size=XS&post=setback_0` → bounds `[100, 335.6]`, midpoint `InsertionMm == 217.8` (tol `1e-9`).
3. **`TestParseBikeSwitchesQ109`:**
   - Frame switch with non-matching size & fork: `prev_frame=otso-fenrir-stainless&frame=esker-hayduke&size=L&fork=lithic_carbon_mountain&fork_a2c=495&cup=12&sag=0&stem_length=80&bar=sqlab3ox_16_30` → adopts `S3`, `pike_120`, `531 mm` A2C, `12 mm` EC44 cup, `20%` sag, keeps `stem_length=80` and `sqlab3ox_16_30`.
   - Frame switch keeping matching size: `prev_frame=otso-fenrir-stainless&frame=surly-krampus&size=L&fork=lithic_carbon_mountain&fork_a2c=495&cup=12&sag=0` → keeps `L`, adopts `surly_krampus_483`, `483 mm`, `12 mm` EC44, `0%` sag.
   - Frame switch keeping shared fork ID across different cup/rake: `prev_frame=otso-fenrir-stainless&frame=salsa-cutthroat&size=L&fork=sid_100&fork_a2c=506&cup=12&sag=25` → adopts `56cm`, keeps `sid_100` with Cutthroat's `IS52 0 mm` cup, `506 mm` A2C, `51 mm` rake, `20%` sag.
   - Frame switch keeping `fork=cad`: `prev_frame=otso-fenrir-stainless&frame=binary-havok&fork=cad&fork_a2c=495&cup=0&sag=0` → keeps `fork=cad` with Binary Havok L's `485 mm` A2C, `10 mm` EC44 cup, `445 mm` published chainstay.
   - Post switch clamping out-of-bounds `post_insert`: `prev_post=setback_0&post=redshift_shockstop_pro&post_insert=275` → clamps `InsertionMm == 250`; direct URL without `prev_post` (`post=redshift_shockstop_pro&post_insert=275`) keeps `InsertionMm == 275` (rejected later by `Evaluate`/`PlaceBike`).
   - Size switch clamping out-of-bounds `post_insert`: `frame=otso-fenrir-stainless&prev_size=XL&size=L&post=setback_0&post_insert=350` → clamps `InsertionMm == 290` (Fenrir L's `MaxPostInsertionMm`).
   - Fork switch on same frame: `prev_fork=cad&fork=fox34_120&fork_a2c=495&cup=0&sag=0` → adopts `530 mm`, `12 mm` EC44, `20%` sag; `prev_fork=lithic_carbon_mountain&fork=fox34_120&fork_a2c=495&cup=14&sag=0` → keeps `cup=14` (same `EC44` standard), adopts `530 mm`, `20%` sag.
4. **`TestParseBikeTargetsAndURLRoundTrip`:**
   - Default, `brp=off`, `grip=off`, `brp=off&grip=off`, `brp_x=&brp_y=` (cleared), `brp=off&brp_x=-200&brp_y=710` (complete pair wins over `off`), `bar_options=all`, `fork_options=all`, `fork=cad`, custom floats (`post_insert=144.10725197204226`, `cup=12.5000000001`).
   - For each, verify `b.URL()` never contains `prev_*`, emits `brp=off`/`grip=off` when cleared, formats `-0.0` as `"0"`, and round-trips through `url.Parse` + `ParseBike` with exact equality (`Setup`, `BrpTarget`, `GripTarget`, IDs, options, `View`).
5. **`TestParseBikeValidationErrors` (negative tests — every validator named above):**
   Assert `*query.InputError` with exact `Field` for:
   - Unknown keys (`checkKnownKeys`, Q111): `fram=surly-krampus` (`Field: "fram"`), `bar_option=all` (`Field: "bar_option"`), `stock_bars=all` (`Field: "stock_bars"`), `stock_forks=all` (`Field: "stock_forks"`), `post_ext=200` (`Field: "post_ext"`), `post_setback=15` (`Field: "post_setback"`), `z=1&a=2` (deterministic sorted `Field: "a"`).
   - Repeated keys (`checkSingleValueKeys`): `frame=otso-fenrir-stainless&frame=surly-krampus` (`Field: "frame"`), `stem_length=60&stem_length=70` (`Field: "stem_length"`).
   - Unknown frame/size/fork/bar/post: `frame=nope` (`"frame"`), `frame=esker-hayduke&size=L` (`"size"`), `frame=surly-ecr&fork=sid_100` (`"fork"`), `bar=nope` (`"bar"`), `post=nope` (`"post"`).
   - Non-finite / non-numeric floats: `fork_a2c=abc`, `fork_a2c=NaN`, `cup=inf`, `sag=nope`, `spacers=x`, `stem_length=x`, `stem_angle=x`, `bar_roll=x`, `post_insert=x`, `rail_offset=x`, `rail_pitch=x`.
   - Incomplete / invalid targets (`parseTarget`, Q70): `brp_x=-219` (`"brp_y"`), `brp_y=713.1` (`"brp_x"`), `brp_x=-219&brp_y=` (`"brp_y"`), `brp=on` (`"brp"`), `grip_x=481` (`"grip_y"`), `grip=yes` (`"grip"`), `brp_x=NaN&brp_y=713.1` (`"brp_x"`).
   - Invalid options / view: `bar_options=some` (`"bar_options"`), `fork_options=some` (`"fork_options"`), `view=top` (`"view"`).
6. **`TestBikeEvaluate` (probed values):**
   - **Default Fenrir L:**
     - `Placement.Frameset.LeveledReachMm()` ≈ `421.943305`, `LeveledStackMm()` ≈ `645.996786`, `EttMm()` ≈ `598.532950`, `WheelbaseMm()` ≈ `1162.160430` (tol `1e-5`).
     - `Placement.Saddle.BRP.Origin()` ≈ `(-154.926089, 683.300585)` (tol `1e-5`).
     - `Score != nil`: `Tier == assembly.FitsNeither`, `Total` ≈ `53.142123`, `GripR` ≈ `9.067441`, `BrpMissXMm` ≈ `-64.073911`, `BrpMissYMm` ≈ `+29.799415` (tol `1e-5`).
     - `Fitting != nil`: `Placement.Setup.Seatpost.InsertionMm` ≈ `144.107252`, `RailOffsetMm == 15.0`, `SpacersMm == 0`, `Stem.LengthMm == 130.0`.
     - `SetbackCentered != nil` (`SetbackMm` ≈ `65.948584`, `InsertionMm` ≈ `129.359870`), `SetbackReach != nil` (`SetbackMm` ≈ `51.479453`, `InsertionMm` ≈ `133.315151`) (tol `1e-5`).
     - `Builds != nil`: `Evaluated == 749`, `len(Builds) == 5`, `Builds[0].Score.Tier == assembly.FitsNeither`, `Total` ≈ `55.768206`, `GripR` ≈ `1.764743`, `HandlebarID == "persuader20_800"`, `ForkID == "lithic_carbon_mountain"`, stem `(120, -30, 0)`.
   - **Off-catalog stem (`stem_length=65&stem_angle=-6`):** `Score == nil`, `Fitting != nil`, `Builds != nil`.
   - **BRP cleared (`brp=off`):** `Score != nil`, `Score.Tier == assembly.FitsNeither`, `Score.HasBrp == false`, `Score.Total` ≈ `70.135498` (tol `1e-5`), `SetbackCentered == nil`, `SetbackReach == nil`, `Builds.Builds[0].Score.Total` ≈ `56.854583` (tol `1e-5`).
   - **HandGrip cleared (`grip=off`):** `Score == nil`, `Builds == nil`, `Fitting != nil`, `SetbackCentered != nil`.
   - **Both targets cleared (`brp=off&grip=off`):** `Score == nil`, `Fitting == nil`, `SetbackCentered == nil`, `SetbackReach == nil`, `Builds == nil`.
   - **Winning `compare` build on Surly Krampus XL (`frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=redshift_shockstop_pro&post_insert=163.90608956654532&rail_offset=15&rail_pitch=0&bar=persuader20_800&bar_roll=0&stem_length=100&stem_angle=-10&spacers=0`):**
     - `Score != nil`: `Tier == assembly.FitsAll`, `Total` ≈ `84.575011405`, `GripR` ≈ `0.759424168`, `BrpMissXMm` ≈ `-4.120315310`, `BrpMissYMm` ≈ `0` (tol `1e-6`).
     - `Builds.Builds[0].Score.Total` matches `Score.Total` within `1e-9` and `Builds.Builds[0].Score.Tier == assembly.FitsAll`!
   - **All forks (`fork_options=all` on default Fenrir L):** `Builds.Evaluated == 5243`, `Builds.Builds[0].ForkID == "sid_100"`, `SagPct == 25`, `Score.Total` ≈ `58.827962` (tol `1e-5`), `GripR` ≈ `1.318877`.
   - **Negative `Evaluate` test (out-of-bounds direct URL):** `post=redshift_shockstop_pro&post_insert=275` returns `*assembly.InputError` with `Field == assembly.InputSeatpostInsertion`.

---

## Commit 3: `Add bikegeo-reboot bike JSON CLI subcommand`

### M1. `cmd/bikegeo-reboot/main.go`

1. Update `usage`:
   ```go
   const usage = `Usage:
     bikegeo-reboot serve [--port N] [--open]
     bikegeo-reboot compare [query]
     bikegeo-reboot bike [query]
   `
   ```
2. Add `bikeUsage` describing `bikegeo-reboot bike [query]`, its query keys (`frame`, `size`, `fork`, `fork_a2c`, `cup`, `sag`, `bar`, `spacers`, `stem_length`, `stem_angle`, `bar_roll`, `post`, `post_insert`, `rail_offset`, `rail_pitch`, `brp_x`/`brp_y`/`brp`, `grip_x`/`grip_y`/`grip`, `bar_options`, `fork_options`, `view`), and examples:
   - `bikegeo-reboot bike`
   - `bikegeo-reboot bike 'frame=surly-krampus&size=XL&fork=fox34_120&sag=20'`
   - `bikegeo-reboot bike 'frame=binary-havok&size=L&brp=off&grip=off'`
3. Add `case "bike": return runBike(args[1:], stdout, stderr)` in `run`.
4. Add `runBike(args []string, stdout, stderr io.Writer) int` mirroring `runCompare`:
   - Handle `-h` / `--help` / `help` → print `bikeUsage`, return `0`.
   - If `len(args) > 1` → print `Usage: bikegeo-reboot bike [query]` to `stderr`, return `2`.
   - Trim optional leading `?`, parse with `url.ParseQuery`, call `query.ParseBike(values)`, then `parsed.Evaluate(query.RankedBuildLimit)` (all validation/input errors print to `stderr` and return `2`).
   - Encode `result` to `stdout` with `json.NewEncoder(stdout)` (`SetIndent("", "  ")`; encode failure returns `1`).

### M2. `cmd/bikegeo-reboot/main_test.go`

Add tests mirroring `compare` CLI tests:
1. **`TestRunBikeDefaultAndWinningBuildJSON`:**
   - `bike` (no query): exit `0`, empty stderr, decodes `FrameID == "otso-fenrir-stainless"`, `Size == "L"`, `ForkID == "lithic_carbon_mountain"`, `PostID == "redshift_shockstop_pro"`, `BarID == "persuader20_800"`, `Score.Tier == "fits_neither"`, `Score.Total` ≈ `53.142123` (tol `1e-5`), `Builds.Evaluated == 749`, `len(Builds.Builds) == 5`, non-zero `Placement.Frameset.HeadTubeTop` and `Saddle.BRP` poses.
   - `bike` with leading `?` and Surly Krampus XL winning build query: exit `0`, `Score.Tier == "fits_all"`, `Score.Total` ≈ `84.575011405` (tol `1e-6`), `Builds.Builds[0].Score.Total` ≈ `84.575011405`, identical output with and without leading `?`.
   - `bike 'brp=off&grip=off'`: exit `0`, `Score == nil`, `Fitting == nil`, `Builds == nil`.
   - `bike 'stem_length=65&stem_angle=-6'`: exit `0`, `Score == nil`, `Builds != nil`.
2. **`TestRunBikeHelpAndErrors`:**
   - Root `--help` mentions `serve`, `compare`, and `bike`; `bike --help` mentions `bikegeo-reboot bike [query]`.
   - Negative tests (exit `2`, empty stdout, expected stderr substring): too many args, bad URL escape (`frame=%zz`), semicolon query, unknown query key (`fram=surly-krampus`), retired key (`post_ext=200`), unknown frame (`frame=nope`), out-of-bounds insertion (`post=redshift_shockstop_pro&post_insert=275`), half target (`brp_x=-219`).
   - Failing stdout writer returns exit `1` containing `"encode JSON"`.

---

## Commit 4: `Support all catalog frames and defaults on /bike`

### W1. Delete `internal/reboot/web/workbench.go`

Remove the Fenrir-only `bikeWorkbench` / `fenrirWorkbench` file entirely. Move the small display helper `makeForkOption` / `forkCellName` (for formatting `<option>` labels and ranked-build fork cell text from `catalog.FrameFork` and `assembly.FrameChart`) into `internal/reboot/web/web.go`.

### W2. Update `internal/reboot/web/web.go`

Import `"bikegeo/internal/reboot/query"`. Keep `NewHandler()` redirecting `/{$}` → `/bike` (Slice 14b replaces `/`).

1. **Replace `workbenchState` / `decodeWorkbenchState` with `query.Bike` + `Evaluate`:**
   - In `handleBikeWorkbench`:
     - Call `parsed, err := query.ParseBike(q)`.
     - If `err != nil`: extract `invalidField` from `*query.InputError` (empty string if other) and call `renderBikeParseError(w, q, invalidField, err.Error())` (400 Bad Request, echoing submitted values, resolving best-effort frame/size/fork/bar/post selects and bounds so the form remains editable and preset links stay valid).
     - Call `result, err := parsed.Evaluate(query.RankedBuildLimit)`.
     - If `err != nil`: call `renderBikeEvalError(w, q, parsed, bikeInputField(err), err.Error())` (400 Bad Request, echoing raw submitted text from `q` where non-empty and falling back to `parsed`'s formatted values, keeping locked CAD values when `parsed.ForkID == query.CadForkID`).
     - Call `cadBaselineFork, err := assembly.PlaceCadBaselineFork(parsed.Size.Chart, parsed.Setup.Wheels)` (500 on internal error).
     - Build `bikeView` from `parsed`, `result`, and `cadBaselineFork`, and render `200 OK`.
   - **Important form-echo rules on 400 error pages:**
     - If `frame` was invalid, set `SelectedFrameID = ""` and fall back to `catalog.DefaultFrameID` only for populating size/fork `<select>` options and bounds.
     - If `size` was invalid, set `SelectedSize = ""` and fall back to `frame.DefaultSize` for bounds and CAD chart.
     - If `fork` was invalid, set `SelectedForkID = ""` and fall back to `frame.Forks[0]` for cup label / readonly flags (and leave `sag` editable if invalid/non-zero sag was submitted on a rigid fork so `TestBikeWorkbenchRecoversFromInvalidRigidForkSag` passes!).
     - If `view` was invalid, echo raw `view` in hidden form input, while `DefaultsURL` and `CadBaselineForkURL` use `query.ViewBike` (`TestBikeWorkbenchUnknownViewRecoversThroughPresets`).
     - Echo `bar_options` / `fork_options` raw values and keep valid ones in `CadBaselineForkURL` (`TestBikeWorkbenchBarOptionsErrorPage`, `TestBikeWorkbenchForkOptionsErrorPage`).
     - Preserve exact raw text typed in `q` for numeric fields (e.g., `post_insert=275` echoes `"275"`, `brp_x=NaN` echoes `"NaN"`), except under `fork=cad` where `fork_a2c`, `cup`, and `sag` echo the locked chart values.
2. **Form values (`bikeFormValues`):**
   - Add `Frame`, `Size`, `PrevFrame`, `PrevSize`, `PrevPost` fields alongside existing fields.
   - Keep `formatFormFloat` (`"0.0"`, `"12.0"`, `"144.10725197204226"`) for HTML `<input value="...">` fields; `query.Bike.URL()` uses `query.FormatFloat` (`"0"`, `"12"`, `"144.10725197204226"`) for URLs (Q77).
   - When `parsed.BrpTarget == nil`, `Form.BrpX = ""`, `Form.BrpY = ""`; when `parsed.GripTarget == nil`, `Form.GripX = ""`, `Form.GripY = ""`.
3. **New `bikeView` fields:**
   - `Frames []frameSelectOption` (`ID`, `Label`: `fmt.Sprintf("%s %s (%d)", f.Manufacturer, f.Model, f.Year)`)
   - `SelectedFrameID string`
   - `Sizes []string` (size names of the selected frame in declared order)
   - `SelectedSize string`
   - `ClearTargetsURL string` (set to `parsed.WithoutTargets().URL()` when `parsed.BrpTarget != nil || parsed.GripTarget != nil`, empty otherwise)
   - `SlidingDropoutsNote string` (non-empty when `frame.MaxChainstayMm > 0`:
     - physical fork with `MaxChainstayMm != published`: `fmt.Sprintf("Sliding dropouts evaluated at max chainstay (%.0f mm; published chart %.0f mm).", frame.MaxChainstayMm, size.Chart.Frame.ChainstayMm)`
     - physical fork with `MaxChainstayMm == published`: `fmt.Sprintf("Sliding dropouts evaluated at max chainstay (%.0f mm, matching published chart).", frame.MaxChainstayMm)`
     - `fork=cad` with `MaxChainstayMm != published`: `fmt.Sprintf("CAD Baseline Fork uses published chainstay (%.0f mm); installed physical forks evaluate sliding dropouts at max chainstay (%.0f mm).", size.Chart.Frame.ChainstayMm, frame.MaxChainstayMm)`)
   - `RedshiftMixedStateNote string` (non-empty when `SelectedPostID == catalog.DefaultSeatpostID`: `"Note: default BRP (−219, 713.1) mm comes from the unsagged Redshift transfer; this post models 12 mm sagged setback with no vertical compression adjustment."`, Q78)
   - `LeveledReachMm`, `LeveledStackMm`, `EttMm float64` (from `placed.LeveledReachMm()`, `placed.LeveledStackMm()`, `placed.EttMm()`, Q105)
   - `SubmittedScore *submittedScoreView` on `bikeFittingView` (Q110):
     ```go
     type submittedScoreView struct {
     	Scored           bool
     	OffCatalogNote   string
     	Tier             assembly.FitTier
     	TierClass        string
     	TierLabel        string
     	Score            float64
     	ActiveWeight     float64
     	HasBrp           bool
     	GripR            float64
     	BrpMissXMm       float64
     	BrpMissYMm       float64
     	AttributeHeaders []buildAttributeHeader
     	Attributes       []assembly.AttributeScore
     }
     ```
     - Nil when `parsed.GripTarget == nil`.
     - When `result.Score != nil`: `Scored: true`, populated from `*result.Score` + `buildAttributeHeaders(catalog.DefaultScoreWeights())`.
     - When `parsed.GripTarget != nil && result.Score == nil`: `Scored: false`, `OffCatalogNote: fmt.Sprintf("Off-catalog stem (%s mm, %+.1f°): build score applies to catalog stems.", query.FormatFloat(stem.LengthMm), stem.AngleDeg)`.
4. **`cadBaselineForkNote` update:**
   - Return `""` when `selectedForkID != query.CadForkID`.
   - When `size.Chart.CadLowerCup.Standard == assembly.LowerCupBareHeadTube`, keep the existing text (`"CAD Baseline Fork: the published frame chart omits the lower headset cup (bare head tube, 0 mm), and these chart fields are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes."`).
   - Otherwise (physical CAD lower cup, e.g. `binary-havok` `EC44 10 mm` or `surly-krampus` `EC44 12 mm`), return: `fmt.Sprintf("CAD Baseline Fork: chart A2C, lower cup (%s, %s mm), and 0%% sag are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.", lowerCupStandardName(cadCup.Standard), query.FormatFloat(cadCup.StackMm), lowerCupStandardName(physCup.Standard), query.FormatFloat(physCup.StackMm))`.
5. **Preset & Apply URLs:**
   - `DefaultsURL`: `query.DefaultBike().WithView(view).URL()` (resets to Fenrir L + catalog defaults, keeps `view`).
   - `CadBaselineForkURL`: `cadBase.WithView(view).WithCadBaselineFork().URL()`.
   - `ApplyURL` on `bikeFittingView`: `parsed.WithSetup(result.Fitting.Placement.Setup).URL()`.
   - `ApplyURL` on each ranked `scoredBuildView`: start from `parsed.WithSetup(c.Placement.Setup)`; if `c.HasHandlebarOption`, update `Bar`; if `c.HasForkOption`, update `ForkID`/`FrameFork`; call `.URL()`.
   - `ViewLinks`: `parsed.WithView(v).URL()` for `ViewBike`, `ViewSeat`, `ViewStem`.

### W3. Update `internal/reboot/web/templates/bike.html`

1. **Header badge:** `<span class="badge">{{.BikeName}}</span>` (remove stale `"Milestone 1 Slice 4 (controls & inspection) · "`).
2. **Hidden inputs:** emit `prev_frame`, `prev_size`, `prev_fork`, `prev_post`, `view`.
3. **First fieldset (`Frame, Fork & Headset`):**
   - Add `<select id="ctrl-frame" name="frame">` (`aria-invalid` when `InvalidField == "frame"`) over `.Frames`.
   - Add `<select id="ctrl-size" name="size">` (`aria-invalid` when `InvalidField == "size"`) over `.Sizes`.
   - Keep `Fork`, `Fork Static A2C`, lower cup, `Fork Sag`, and `data-note="cad-baseline-fork"`.
   - Add `{{if .SlidingDropoutsNote}}<p class="field-note" data-note="sliding-dropouts">{{.SlidingDropoutsNote}}</p>{{end}}`.
4. **Rear Cockpit fieldset:**
   - Add `{{if .RedshiftMixedStateNote}}<p class="field-note" data-note="redshift-mixed-state">{{.RedshiftMixedStateNote}}</p>{{end}}`.
5. **Target fieldsets:**
   - Update `.fit-note` text to `"Enter both X and Y (mm, world coordinates, BB at origin). Default: {{.ExampleBrpX}}, {{.ExampleBrpY}}. Clear both boxes to disable."` (and matching HandGrip note).
6. **Control actions:**
   - Add `{{if .ClearTargetsURL}}<a id="preset-clear-targets" href="{{.ClearTargetsURL}}" data-preset="clear-targets">Clear Targets</a>{{end}}`.
7. **Summary readouts:**
   - Add right after `Leveled STA`:
     - `<span>Leveled Reach: <strong class="mono" data-metric="leveled-reach">{{printf "%.1f mm" .LeveledReachMm}}</strong></span>`
     - `<span>Leveled Stack: <strong class="mono" data-metric="leveled-stack">{{printf "%.1f mm" .LeveledStackMm}}</strong></span>`
     - `<span>ETT: <strong class="mono" data-metric="ett">{{printf "%.1f mm" .EttMm}}</strong></span>`
8. **Submitted build score block (Q110):**
   - Inside `{{with .Fitting}}`, before `{{with .Saddle}}`, render `{{with .SubmittedScore}}`:
     - Container `<div class="build-ranking" data-submitted-score><strong>Submitted build score</strong>...</div>`
     - When `.Scored`: tier badge (`data-submitted-tier="{{.Tier}}"`), score (`data-metric="submitted-score"`), Oval R (`data-metric="submitted-oval-r"`), BRP miss (`data-metric="submitted-brp-miss"`), and 12-attribute row (`data-submitted-attr="{{.Attribute}}"` with `—` for inactive).
     - When `not .Scored`: `<p class="fit-note" data-note="off-catalog-stem">{{.OffCatalogNote}}</p>`.
9. **CAD baseline footer line:**
   - Wrap published wheelbase in `{{if gt .PublishedWheelbaseMm 0.0}} (Published <strong class="mono">{{printf "%.1f mm" .PublishedWheelbaseMm}}</strong>){{end}}` (Q57).

### W4. Update `internal/reboot/web/web_test.go`

Because `/bike` now defaults to `redshift_shockstop_pro`, midpoint `post_insert=175`, and active idmatch targets (Q53, Q62, Q109), and rejects unknown keys (Q111) and half targets (Q70):

1. **Migrate existing test helpers & tests cleanly:**
   - Replace `fenrirWorkbench()`, `decodeWorkbenchState(...)`, `defaultWorkbenchState(...)`, and `workbenchState` references with `query.DefaultBike()`, `query.ParseBike(...)`, and `query.Bike`.
   - Remove the obsolete `bikeWorkbench.validate()` struct-mutation tests (`TestWorkbenchValidationAndCadBaselineForkInvariant` validate table, `TestBikeWorkbenchValidatesPartOptionsAndWeights`, `TestBikeWorkbenchValidatesHandlebars`, `TestWorkbenchHeadsetPairings`), keeping the CAD URL round-trip and cup-standard switch coverage via `query.ParseBike` / HTTP requests.
   - Tests that check Fenrir's old `setback_0` / `post_insert=180.576` / manual no-target baseline: either pass explicit query params (`post=setback_0&post_insert=180.576&brp=off&grip=off`) where they specifically test that build, or update assertions to the new default (`redshift_shockstop_pro`, `post_insert=175`, `post-exposed=175.0 mm`, `seat-axis-post-top=665.0 mm`, `brp-height=700.6 mm`, active targets).
   - Update `TestBikeWorkbenchWithoutBrpTargetHasNoFitting`, `TestBikeWorkbenchWithoutGripTargetHasNoCockpitFitting`, `TestBikeWorkbenchBuildRankingRequiresGrip`, `TestBikeWorkbenchSetbackNotesAbsentWithoutBrpTarget`, `TestBikeWorkbenchKeepsBarOptionsWithoutTarget`, `TestBikeWorkbenchKeepsForkOptionsWithoutTarget` to pass `brp=off` / `grip=off` explicitly (since omitted targets now default to idmatch per Q53).
   - Update `TestBikeWorkbenchHalfEnteredTargetIsQuietlyIgnored` → assert `400` Bad Request with `aria-invalid="true"` on the missing coordinate (`brp_x=-219` → `brp_y`, `grip_y=726` → `grip_x`) per Q70!
   - Update `TestBikeWorkbenchIgnoresRetiredSetbackKey`, `TestBikeWorkbenchIgnoresRetiredStockKeys`, `TestBikeWorkbenchIgnoresRetiredPostExtKey` → assert `400` Bad Request (`#form-error` containing `"unknown query parameter"`) per Q111!
   - Update `TestBikeWorkbenchDefaultSaddleHeightQ92` → `TestBikeWorkbenchDefaultInsertionMidpointQ109`: bare `/bike` has `post=redshift_shockstop_pro`, `post_insert=175.0`, `post-exposed=175.0 mm`, `brp-height=700.6 mm`, `BRP=(-154.926, 683.301)` (tol `0.001`); `/bike?post=setback_0` has `post_insert=195.0`, `post-exposed=205.0 mm`.
   - Update URL float assertions on `hrefQuery` where needed (`"12"` instead of `"12.0"`, `"-219"` instead of `"-219.0"`).
   - Keep `TestBikeWorkbenchPostSwitchKeepsInsertionAndRecovers` checking direct URL `/bike?post=redshift_shockstop_pro&post_insert=275` returns `400` (echoing `"275"`, `max="250"`), and extend it with form submission switching `setback_0` → `redshift_shockstop_pro` clamping `275` → `250` (`200 OK`, Q109).
   - Add `frame` and `size` mutations to `TestBikeWorkbenchEveryEnabledControlIsLive`!
2. **Add new Slice 14a web tests (with probed expected values):**
   - **`TestBikeMultiFrameSelectAndSwitching`:**
     - All 19 frames render in `select[name="frame"]`, selected frame's sizes render in `select[name="size"]`.
     - Form submit switching Fenrir L → `esker-hayduke` (`200 OK`): adopts `S3`, `pike_120`, `531.0` A2C, `12.0` cup, `20.0` sag, keeps `bar` and `stem_length`.
     - Form submit switching Fenrir L → `surly-krampus` (`200 OK`): keeps `L`, adopts `surly_krampus_483`, `483.0` A2C, `0.0` sag.
     - Form submit switching Fenrir L (`sid_100`, `cup=12`, `sag=25`) → `salsa-cutthroat` (`200 OK`): adopts `56cm`, keeps `sid_100`, adopts `0.0` IS52 cup and `20.0` sag.
     - Unknown `frame=nope` (`400`, `select[name="frame"][aria-invalid="true"]`) and unknown `size=XXL` (`400`, `select[name="size"][aria-invalid="true"]`).
   - **`TestBikeSlidingDropoutsAndUnpublishedWheelbase`:**
     - `/bike?frame=binary-havok&size=L`: physical `rigid_485` shows `wheelbase = "1126.5 mm"`, `rear-center = "454.8 mm"`, `leveled-reach = "396.9 mm"`, `leveled-stack = "656.7 mm"`, `ett = "586.4 mm"`, `delta-pitch = "+0.09°"`, `[data-note="sliding-dropouts"]` mentions `460 mm` and `445 mm`.
     - `/bike?frame=binary-havok&size=L&fork=cad`: shows `wheelbase = "1110.6 mm"`, `rear-center = "439.5 mm"`, `delta-pitch = "+0.00°"`, `cup` readonly `"10.0"`, `[data-note="cad-baseline-fork"]` non-empty.
     - `/bike?frame=moots-mountaineer&size=M`: renders `200 OK` and does NOT print `"(Published"` anywhere in the CAD baseline line (Q57).
   - **`TestBikeLeveledReadoutsAndRedshiftNote`:**
     - Default `/bike`: `leveled-reach = "421.9 mm"`, `leveled-stack = "646.0 mm"`, `ett = "598.5 mm"`; `[data-note="redshift-mixed-state"]` present (Q78).
     - `/bike?fork=cad`: `leveled-reach = "428.1 mm"`, `leveled-stack = "641.9 mm"`, `ett = "597.0 mm"` (Q105).
     - `/bike?post=setback_0`: `[data-note="redshift-mixed-state"]` absent.
   - **`TestBikeSubmittedBuildScoreQ110`:**
     - Default `/bike` (catalog stem `60 mm, -6°`): `[data-submitted-score]` present, `[data-submitted-tier="fits_neither"]`, `submitted-score = "53.1"`, `submitted-oval-r = "9.07"`, `submitted-brp-miss = "(-64.1, +29.8) mm"`, 12 `[data-submitted-attr]` cells (none `"—"`), `Stem` attr `"86.5"`.
     - Surly Krampus XL winning compare build URL: `[data-submitted-tier="fits_all"]`, `submitted-score = "84.6"`, `submitted-oval-r = "0.76"`, `submitted-brp-miss = "(-4.1, +0.0) mm"`, matching `tr[data-build-rank="1"]` (`Fits all`, `84.6`, `0.76`, `(-4.1, +0.0) mm`)!
     - Cleared BRP (`/bike?brp=off`): `[data-submitted-tier="fits_neither"]`, `submitted-score = "70.1"`, `submitted-brp-miss = "—"`, 4 saddle `[data-submitted-attr]` cells show `"—"`, other 8 show numeric scores.
     - Off-catalog stem (`/bike?stem_length=65&stem_angle=-6`): `[data-submitted-score]` renders `[data-note="off-catalog-stem"]` and zero `[data-submitted-tier]` / `[data-metric="submitted-score"]`.
     - Cleared HandGrip (`/bike?grip=off`): zero `[data-submitted-score]`.
   - **`TestBikeDurableClearedTargetsQ70`:**
     - Follow `a#preset-clear-targets` from default `/bike` → `brp=off&grip=off`, zero `[data-fitting]`, zero `[data-submitted-score]`, zero `[data-build-ranking]`, `a#preset-clear-targets` hidden, `#ctrl-brp-x`/`#ctrl-brp-y`/`#ctrl-grip-x`/`#ctrl-grip-y` all `value=""`.
     - Submit form unchanged (changing `stem_length=80`) from cleared-targets page → stays cleared (zero `[data-fitting]`), view links & `CAD Baseline Fork` link keep `brp=off&grip=off`, reload stays cleared!
     - Clear BRP only (`brp_x=&brp_y=&grip_x=481&grip_y=726`) → Apply link (`#apply-fitting`), Ranked build Apply link, view links, and CAD link all carry `brp=off` and survive reload!
     - Clear HandGrip only (`grip=off`) → `#apply-fitting` and view links carry `grip=off` and survive reload!

---

## Rules that bite

- **Isolation (`make boundary`):** `internal/reboot/query` must NEVER import `net/http`, `html/template`, or `internal/reboot/web`. `internal/reboot/assembly` and `internal/reboot/compare` must NEVER import `catalog` or `query`. Never import legacy `bikegeo/...` packages outside `*_parity_test.go`.
- **Banned terms (`make boundary`):** never write `chassis`, `price`, `pricing`, `cost`, `cheap`, or `ledger` (in any case or inside longer words) anywhere in `cmd/bikegeo-reboot` or `internal/reboot`.
- **Trig lives in `pose`:** `catalog`, `query`, `cmd`, and `web` do zero trig (`math.Abs`, `math.IsNaN`, `math.IsInf` are fine).
- **`exhaustive`:** every `switch` on an enum-like type lists every constant.
- **No JS (`Q48`, `Q54`):** zero `<script>` tags, inline event handlers, or JS files.
- **No optimization (`Q16`, `Q86`, `Q104`):** keep strict forward `PlaceBike` per candidate in `FitBuilds`.
- **Q8:** never use "search" for fitting in code, comments, or docs.
- **No changes to `pose`, `assembly`, or `compare`**, and keep `/` redirecting to `/bike` until Slice 14b.
- Run `go vet ./... && go test ./...` after every code change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Plain-language commit subjects matching the four planned commit titles.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 14a Milestones line.

## Verification

1. `make pre-commit` passes on each of the 4 commits individually.
2. `go list -deps ./internal/reboot/query | grep -E '(^net/http$|^html/template$|reboot/web)'` prints nothing.
3. `go test ./internal/reboot/catalog ./internal/reboot/query ./cmd/bikegeo-reboot ./internal/reboot/web -count=1` passes.
4. **Live CLI check:** build with `make build-reboot` and run:
   - `./bin/bikegeo-reboot bike` → exit `0`, valid JSON with `FrameID: "otso-fenrir-stainless"`, `Size: "L"`, `PostID: "redshift_shockstop_pro"`, `InsertionMm: 175`, `Score.Total ≈ 53.142`, `Builds.Evaluated: 749`.
   - `./bin/bikegeo-reboot bike 'frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=redshift_shockstop_pro&post_insert=163.90608956654532&rail_offset=15&rail_pitch=0&bar=persuader20_800&bar_roll=0&stem_length=100&stem_angle=-10&spacers=0'` → exit `0`, `Score.Tier: "fits_all"`, `Score.Total ≈ 84.575`, matching `Builds.Builds[0]`.
   - `./bin/bikegeo-reboot bike 'fram=surly-krampus'` → exit `2` with unknown-parameter error on stderr.
5. **Live web check:** start `./bin/bikegeo-reboot serve --port 8081` (or ephemeral port) and verify in a browser or rendered inspection:
   - `/bike` opens on Otso Fenrir Stainless (2024, Size L) with Redshift seatpost, Q78 note, default idmatch targets, Submitted build score (`53.1`, `Fits neither`), Leveled Reach/Stack/ETT (`421.9`, `646.0`, `598.5 mm`), and Ranked builds.
   - Switching Frame to `Esker Hayduke` updates sizes (`S1`–`S4`, `S3` selected), forks (`pike_120`, `20%` sag), and sliding-dropout note without error.
   - Clicking `Clear Targets` removes fitting/score blocks and survives a form edit + view switch + reload.
   Record observed values and screenshot path (if taken) in the handoff.

## Out of scope

- Web `/` multi-frame comparison page, custom aligned selection form UX, `/` table layout, `/` ↔ `/bike` nav links, and end-to-end `/` → `/bike` round-trip tests (Slice 14b).
- Any change to `internal/reboot/pose`, `internal/reboot/assembly`, or `internal/reboot/compare`.
- Performance optimization (Q86, Q104).
