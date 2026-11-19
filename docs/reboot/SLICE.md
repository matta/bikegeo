# Next slice: M2 Slice 1, saddle fitting

**Status: Done 2026-11-19 (Matt accepted). Implemented from Matt's rulings Q6–Q9 ([DESIGN.md](DESIGN.md), "Fitting"). Commits 1–3 landed as one commit because `deadcode` rejects them separately; Commit 4 landed on its own.** This file is the execution plan for one agent (Sonnet is fine). Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first; they override anything here that disagrees. If a step here conflicts with the code you find, stop and ask Matt rather than improvising.

## 1. Goal

Given a BikeSetup and a target **BRP** point (world coordinates: BB = (0,0), +X forward, +Y up, mm), **fit** the saddle: choose `SeatpostSetup.ExtensionMm` and `SaddleSetup.RailOffsetMm` that put BRP on the target, respecting hardware limits. When the target is out of reach, return the closest reachable build and say which limit stopped it. PlaceBike on the fitted setup is the final check (Q6). The `/bike` workbench shows the result and links to the fitted build.

Fixed inputs (not fitted in this slice): frame, fork, cup, wheels, stem, handlebar, `PostSetbackMm`, `RailPitchDeg`, and all saddle/post dimensions.

## 2. Terminology (Q8, mandatory)

- Noun / result type: **Fitting**. Verb: **fit**. Entry point: **`ApplyFitting`**.
- Never write "search", "solver search", or "resolve" for this operation in code, comments, test names, or docs.

## 3. The math (why a direct solve works)

In `PlaceSeatpost`/`PlaceSaddle`, with everything else fixed:

```text
BRP = SeatTubeTop
    + ExtensionMm   · dir(seatHeading)          // Advance along the seat axis
    + PostSetbackMm · dir(seatHeading + 90°)    // Translate local +Y (fixed here)
    − RailOffsetMm  · dir(RailPitchDeg)         // Advance −RailOffsetMm along the rails
    + RailCenterToBrpMm · dir(RailPitchDeg)     // fixed
```

`seatHeading` is `SeatTubeTop.HeadingDeg()` (≈105.29° on the default Fenrir build); `RailPitchDeg` is a world heading set by `Face`. Neither depends on the two fitted inputs, so BRP is **linear** in `(ExtensionMm, RailOffsetMm)` along two fixed, non-parallel directions:

```text
BRP(e, r) = BRP(e0, r0) + (e − e0)·dir(seatHeading) + (r − r0)·dir(RailPitchDeg + 180°)
```

So: place the bike once at a probe setup `(e0, r0)`, take `Δ = target − BRP(e0, r0)`, and decompose `Δ` along the two headings. No geometry is restated in `assembly`; the probe placement supplies `BRP(e0, r0)` and `seatHeading`, and the final PlaceBike proves the linear step was right.

### Limits

- Extension: `[MinExtensionMm, MaxExtensionMm]`; `MaxExtensionMm == 0` means no upper bound (existing sentinel, see the TODO in `seatpost.go`; keep it).
- Rail offset: `[−L, +L]` where `L = (Saddle.RailLengthMm − Seatpost.ClampLengthMm) / 2`. If `L < 0`, return an error (the clamp is longer than the rails). Fenrir fixture: `(60 − 30)/2 = ±15 mm`.
- Commits 1–3 do not change `PlaceSaddle`; Commit 4 (§9, Q9) makes it reject offsets outside this range.

### Clamped case (box-constrained least squares, exact in 2D)

Minimize `|Δ − Δe·u − Δr·w|` over the box, where `u = dir(seatHeading)`, `w = dir(RailPitchDeg + 180°)`, both unit vectors.

1. Unconstrained solve. If both values are inside their ranges → outcome **Reached**.
2. Otherwise evaluate four edge candidates in this fixed order: extension at min, extension at max (skip if unbounded), rail offset at min, rail offset at max. On each edge, fix that variable at its bound, set the other to its 1D optimum (projection of the remaining vector onto its unit direction), then clamp the other to its range. Compute each candidate's miss with the linear model. Pick the smallest miss; on ties (difference < 1e-9 mm) keep the earlier candidate. → outcome **Limited**.
3. `LimitedBy` lists, in the order extension then rail offset, each field whose chosen value equals one of its bounds (only in the Limited outcome).

This is exact because the objective is a convex quadratic: if the unconstrained minimum is outside the box, the box minimum lies on an edge, and on each edge the clamped 1D optimum is that edge's minimum.

## 4. Code changes

Do these as **four commits**, in order (Commit 4 is in §9), each passing `make pre-commit`. Run `go vet ./... && go test ./...` after every edit.

### Commit 1: `pose` decomposition primitives

File: `internal/reboot/pose/primitives.go` (tests in `pose_test.go`).

```go
// DecomposeAlong returns the coefficients (a, b) with v = a·dir(headingA) + b·dir(headingB),
// where dir(h) is the unit vector at sagittal world heading h. ok is false when the
// headings are parallel or antiparallel (|sin(headingB − headingA)| < 1e-12).
func DecomposeAlong(v Point, headingA, headingB Deg) (a, b float64, ok bool)

// ProjectOnto returns the signed length of v's component along the unit vector at heading.
func ProjectOnto(v Point, heading Deg) float64

// Along returns the vector of length lengthMm at sagittal world heading heading.
func Along(heading Deg, lengthMm float64) Point
```

Formulas (cross(p, q) = p.X·q.Y − p.Y·q.X, dA = dir(headingA), dB = dir(headingB)): `den = cross(dA, dB)` (= sin(B − A)); `a = cross(v, dB)/den`; `b = cross(dA, v)/den`. `ProjectOnto = v.X·cos(h) + v.Y·sin(h)`. Use the existing unexported `Deg.cos`/`sin`.

Also add, in `pose.go`:

```go
// OffsetTo returns other.Origin() − p.Origin() as a world-frame vector.
func (p Pose) OffsetTo(other Pose) Point
// Sub returns p − q.
func (p Point) Sub(q Point) Point
```

Only add what Commits 2–3 use (deadcode runs in `make check`). If `Sub` turns out to be unused, drop it.

Tests (independent math, plain `math` is fine in tests):
- Axis-aligned: `DecomposeAlong({3, 4}, 0, 90)` → `(3, 4, true)`.
- `Along(90, 2)` → `(0, 2)` to 1e-12.
- Oblique round trip: pick headings 105.29° and 180°, a vector, decompose, rebuild with `cos/sin`, compare to 1e-9.
- Parallel (`30°, 30°`) and antiparallel (`30°, 210°`) → `ok == false`.
- `ProjectOnto({1, 1}, 45)` → `√2`.

Commit subject: `Add vector decomposition primitives to reboot pose`.

### Commit 2: `assembly` saddle fitting

New file `internal/reboot/assembly/fitting.go`, tests in `fitting_test.go`. Reuse the existing `InputField` constants; add no new limit enum.

```go
// SaddleFitTarget is the BRP target point for saddle fitting (Q7), in world
// coordinates (BB at origin, +X forward, +Y up, mm).
type SaddleFitTarget struct {
	BRP pose.Point
}

// FitOutcome says whether a Fitting reached its target or stopped at a hardware limit.
type FitOutcome string

const (
	FitReached FitOutcome = "reached"
	FitLimited FitOutcome = "limited"
)

// Fitting is the result of ApplyFitting: the fitted setup, its forward
// placement (the final check), and which limits stopped it.
type Fitting struct {
	Target    SaddleFitTarget
	Placement BikePlacement // PlaceBike(fitted setup); Placement.Setup is the fitted setup
	Outcome   FitOutcome
	LimitedBy []InputField // InputSeatpostExtension and/or InputSaddleRailOffset; nil when reached
}

// BrpMiss returns target − placed BRP (world mm). Zero vector when reached.
func (f Fitting) BrpMiss() pose.Point
// BrpMissMm returns |BrpMiss()|.
func (f Fitting) BrpMissMm() float64

// RailOffsetLimitMm returns the symmetric rail offset bound L = (RailLengthMm − ClampLengthMm)/2.
// Method on BikeSetup in bike.go; error if L < 0 or not finite.
func (s BikeSetup) RailOffsetLimitMm() (float64, error)

// ApplyFitting fits Seatpost.ExtensionMm and Saddle.RailOffsetMm so the saddle BRP
// lands on target, within hardware limits. All other setup fields are kept.
// trace, if non-nil, records only the final PlaceBike of the fitted setup and is
// frozen on return.
func ApplyFitting(setup BikeSetup, target SaddleFitTarget, trace *pose.Trace) (Fitting, error)
```

`ApplyFitting` steps:

1. Validate `target.BRP` is finite (else `invalidInput`; add `InputFitTargetBRP InputField = "BRP fit target"` to `validate.go`). Get `L` from `RailOffsetLimitMm`. Compute `eMin`, `eMax` (`+Inf` when `MaxExtensionMm == 0`).
2. Probe: copy `setup`, set `ExtensionMm = eMin`, `RailOffsetMm = 0`; `probe, err := PlaceBike(probeSetup, nil)`. Return errors unchanged (bad frame, fork, etc.).
3. `Δ = target.BRP − probe.Saddle.BRP.Origin()`; `seat := probe.Frameset.SeatTubeTop.HeadingDeg()`; `rail := setup.Saddle.RailPitchDeg + 180`. `de, dr, ok := pose.DecomposeAlong(Δ, seat, rail)`; if `!ok` return an error (rail pitch parallel to the seat axis). Unconstrained: `e = eMin + de`, `r = dr`.
4. Box logic from §3. Write it as a small unexported helper that takes `Δ`, the two headings, and the bounds expressed relative to the probe (`[0, eMax − eMin]` and `[−L, L]`), and returns `(de, dr, limited []InputField)`. Use `pose.ProjectOnto` for the edge optimum and the linear model for each candidate's miss: `|Δ − Along(seat, de) − Along(rail, dr)|`. Build every vector with `pose.Along`, `Point.Sub`, and `Point.Add` if you need it; no `cos/sin` in `assembly`.
5. Build the fitted setup (copy of `setup` with the two fitted values) and run `PlaceBike(fitted, trace)`. The trace records only this final placement.
6. **Final check:** if `Outcome == FitReached` and `BrpMissMm() > 1e-6`, return an error ("saddle fitting forward check missed target by …"); this guards the linear model. (Test tolerance is tighter, see below.)

Tests in `fitting_test.go` (build setups inline like `saddle_test.go`; `assembly` must not import the web fixture, so copy the Fenrir numbers you need into a test helper with a comment pointing at `internal/reboot/web/fixture.go`):

- **Round trip, reached:** place a known setup (e.g. ext 200, rail offset +5), take its BRP as the target, start `ApplyFitting` from a different setup (ext 263.5, offset 0) → `FitReached`, fitted ext 200 and offset +5 to 1e-9, `BrpMissMm() < 1e-9`, `LimitedBy == nil`.
- **Rail limit, Fenrir with Matt's fit report BRP (−219, 713.1)** ([RIDER_FIT.md](../RIDER_FIT.md)), default fork (`DefaultForkID`), zero setback, level rails. Unconstrained needs ≈ +56.07 mm rail offset, so expect `FitLimited`, `LimitedBy == [InputSaddleRailOffset]`, rail offset exactly +15, extension ≈ 260.092 mm, miss ≈ (−38.212, −10.446), |miss| ≈ 39.614 mm. Assert these to 0.01 mm; they were measured on 2026-11-19 with an independent probe, so if your numbers differ by more, find out why before changing the test.
- **Same target, `PostSetbackMm = 27`** (the FAIR Drop Best offset): still limited by rail offset (unconstrained needs ≈ +28.08 mm), extension ≈ 260.092 mm, |miss| ≈ 12.614 mm. Setback is perpendicular to the seat axis, so the extension matches the zero-setback case; assert that explicitly.
- **Extension limit:** target far above (e.g. BRP + 200 mm along the seat axis) → `LimitedBy` contains `InputSeatpostExtension`, extension == max. Target far below → extension == min.
- **Both limits:** target needing too much extension *and* too much rail offset → both fields listed, in order.
- **Unbounded max:** `MaxExtensionMm = 0`, high target → `FitReached`.
- **Trace on/off equivalence:** `ApplyFitting` with a fresh trace and with nil gives identical `Placement`, `Outcome`, `LimitedBy`; the traced trace is frozen and non-empty.
- **Errors:** non-finite target → `*InputError` with `InputFitTargetBRP`; clamp longer than rails → error; invalid frame input → PlaceBike's error unchanged.
- **Minimality (independent check of the clamped case):** for one limited case, sweep a 1 mm grid over the box using forward `PlaceBike` and assert no grid point beats the fitting's miss by more than 1e-6 mm. This sweep is test-only; it must not appear in non-test code.

Commit subject: `Add saddle fitting to a BRP target in reboot assembly`.

### Commit 3: `/bike` workbench

Files: `internal/reboot/web/web.go`, `templates/bike.html`, `web_test.go`, possibly `fixture.go`, `static/workbench.js` and its test only if needed.

- **State:** add an optional BRP target to `workbenchState`: `BrpTargetX`, `BrpTargetY float64` and `HasBrpTarget bool`. Query keys `brp_x`, `brp_y`. Both blank → no target. Exactly one blank → input error on the blank field ("BRP target needs both X and Y"). Non-finite → input error. Add an `optionalFloat` method to `bikeQueryParser` for this rather than overloading `float`. Write them in `URL()` only when `HasBrpTarget`; echo in `formValues`/`bikeFormValues` (`BrpX`, `BrpY`); carry through `withCadBaselineFork` and `withView` unchanged. Reset Defaults clears the target (it is the default state). The CAD Baseline Fork preset keeps it.
- **Fixture:** add `ExampleBrpTarget pose.Point` = `(-219, 713.1)` with a comment: "measured: Matt's idmatch fit report, docs/RIDER_FIT.md". Use it only as the inputs' `placeholder`, never as a default value.
- **Handler:** after the normal `PlaceBike`, if `HasBrpTarget`, call `assembly.ApplyFitting(resolved.Setup, target, nil)`. A fitting error renders like any other input error (map `InputFitTargetBRP` → field `brp_x` in `bikeInputField`; keep the `switch` exhaustive). The diagram and trace still show the **submitted** build, not the fitted one.
- **View data** (on `bikeView`, only when a target is set): target X/Y; current build's BRP miss (`target − bike.Saddle.BRP`, via `pose.Pose.OffsetTo` on a pose built at the target) as X, Y and magnitude; fitted extension and rail offset; outcome text; limits text; rail range `±L`; and `ApplyFittingURL` = `workbenchStateFromSetup(view, forkID, fitting.Placement.Setup)` with the target kept. `formatFormFloat` round-trips exactly, so following the link gives miss < 1e-9 when reached.
- **Template:** in the controls form, a "BRP fit target" fieldset with `ctrl-brp-x` / `ctrl-brp-y` number inputs (`step="any"`, placeholders from the fixture, `aria-invalid` like the other controls). In the diagram card, when a target is set, a "Saddle fitting" summary row with `data-metric` hooks: `fit-brp-miss`, `fit-extension`, `fit-rail-offset`, `fit-outcome` (text "reached" or "limited by seatpost extension, saddle rail offset"), and a link `<a id="apply-fitting" href="{{.ApplyFittingURL}}" data-preset="apply-fitting">Apply fitted saddle</a>`. `data-preset` reuses the existing JS preset navigation (discards pending edits, pushes history); check `workbench.js` handles an unknown preset name generically and add a JS test only if it doesn't. Optionally draw the target as a small cross landmark in the SVG with its own label; if you do, it must not change any existing step IDs (`TestBikeTraceStepIDs…` must still pass) and it must be included in the `seat` view framing.
- **Tests** (`httptest` + `goquery`, per AGENTS.md):
  - No `brp_x`/`brp_y` → no fitting row; existing pages unchanged (existing tests stay green without edits beyond new fields).
  - `brp_x=-219&brp_y=713.1` → 200, outcome text says limited by saddle rail offset, `fit-rail-offset` shows 15.0 mm, `apply-fitting` href decodes to `rail_offset=15`, the expected extension, and still contains `brp_x`/`brp_y`.
  - Following the apply link for a reachable target (use the round-trip target from Commit 2) → `fit-brp-miss` shows 0.0 mm and outcome "reached".
  - Only `brp_x` set → 400 with `aria-invalid` on `ctrl-brp-y`.
  - Reset Defaults URL drops the target; CAD Baseline Fork URL keeps it.

Commit subject: `Show saddle fitting to a BRP target in the reboot bike workbench`.

## 5. Not in this slice

Choosing a post setback (0 / 25 / 27 mm), rail pitch, the front cockpit (Slice 2), preference costs, multiple frames, CLI/MCP access.

## 6. Rules that bite here

- No trig, radians, or hand-written geometry outside `pose` in non-test code. If you need a vector operation in `assembly` or `web`, add it to `pose` (Commit 1) with a test.
- Scalars derived from placed poses are methods (`Fitting.BrpMissMm`, `BikeSetup.RailOffsetLimitMm`); the web reads them, never recomputes them.
- `exhaustive`: every `switch` on `FitOutcome` or `InputField` names every constant.
- `deadcode`: no unused exports. `boundary`: no legacy imports, and never write the banned term listed in AGENTS.md.
- Placements must be identical with tracing on or off.
- Commit messages: plain subjects, no Conventional Commits prefixes; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 1 done at the end, and only after Matt accepts it.

## 7. Verification

After each commit: `make pre-commit` (re-run until `go fix` is clean). After Commit 3, check it live: `make serve-reboot`, open `http://127.0.0.1:8081/bike?brp_x=-219&brp_y=713.1&view=seat`, and confirm the fitting row, the apply link, and that applying it moves BRP to the clamped position. Take a screenshot for Matt.

## 8. Reference numbers (default Fenrir build, measured 2026-11-19)

| Quantity | Default fork | CAD Baseline Fork |
| --- | --- | --- |
| SeatTubeTop | (−129.21, 472.66), heading 105.289° | (−124.67, 473.88), heading 104.739° |
| Unconstrained fit, setback 0 | ext 249.263, rail +56.067 | ext 247.364, rail +63.399 |
| Unconstrained fit, setback 27 | ext 256.644, rail +28.076 | ext 254.467, rail +35.480 |
| Limited fit (rail +15), setback 0 | ext 260.092, \|miss\| 39.614 | ext 259.678, \|miss\| 46.806 |
| Limited fit (rail +15), setback 27 | ext 260.092, \|miss\| 12.614 | ext 259.678, \|miss\| 19.806 |

## 9. Matt's rulings on this slice's questions (2026-11-19)

- **Q9:** `PlaceSaddle` must reject rail offsets outside `±(RailLengthMm − ClampLengthMm)/2`. Do it as Commit 4 (below), separate from fitting.
- **Q10:** the fixture's 60 mm `RailLengthMm` and 30 mm `ClampLengthMm` are **measured and authoritative**. The "limited" result for the fit report BRP is a real finding. In Commit 3, label both values `// measured` in `fixture.go`, and word the workbench's limited outcome as a fact about this build, not as a possible data problem.

### Commit 4: enforce the rail range in `PlaceSaddle` (Q9)

- Move the bound into one place that both `PlaceSaddle` and `ApplyFitting` use. `PlaceSaddle` receives only the clamp pose and `SaddleSetup`, so pass the clamp length in: change the signature to `PlaceSaddle(seatpostClamp pose.Pose, clampLengthMm float64, setup SaddleSetup)` and update its single caller in `PlaceBike` plus the tests. Put the bound in one unexported function `railOffsetLimitMm(railLengthMm, clampLengthMm float64) (float64, error)`; `BikeSetup.RailOffsetLimitMm` calls it.
- Reject `|RailOffsetMm| > L` (allow equality, with no tolerance, so fitted values exactly at ±L place) with `invalidInput(InputSaddleRailOffset, "saddle rail offset must be within ±%v mm, got %v mm", L, offset)`.
- In the workbench, set the rail offset input's `min`/`max` to `∓L`, like `post_ext`. An out-of-range `rail_offset` in the URL becomes a 400 with `aria-invalid` on `ctrl-rail-offset`.
- Tests: within range places; exactly ±L places; just outside → `*InputError` with `InputSaddleRailOffset`; clamp longer than rails → error; existing tests that use offsets beyond ±15 mm with the Fenrir numbers must change their inputs rather than be loosened. Web: `rail_offset=20` → 400.
- Commit subject: `Reject saddle rail offsets beyond the rail clamping range`.
