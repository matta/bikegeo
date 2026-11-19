# Next slice: M2 Slice 2, front-cockpit fitting (continuous)

**Status: Planned 2026-11-19 from Matt's rulings Q11–Q14 ([DESIGN.md](DESIGN.md), "Fitting"). Not started.** This file is the execution plan for one agent (Sonnet is fine). Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first; they override anything here that disagrees. If a step here conflicts with the code you find, stop and ask Matt rather than improvising. Slice 1 (saddle fitting) is done; this slice extends its code in `internal/reboot/assembly/fitting.go` and the `/bike` workbench.

## 1. Goal

Given a BikeSetup and a target **HandGrip** point (world coordinates: BB = (0,0), +X forward, +Y up, mm), **fit** the front cockpit: choose `StemSetup.SpacersMm` and `StemSetup.LengthMm` that put HandGrip on the target, within hardware limits. When the target is out of reach, return the closest reachable build and say which limit stopped it. The same `ApplyFitting` call fits the saddle (BRP target, Slice 1), the cockpit (HandGrip target), or both, and returns one fitted build. PlaceBike on the fitted setup is the final check (Q6, Q12). The `/bike` workbench shows both fittings and one link to the fitted build.

Fixed inputs for the cockpit fit: frame, fork, cup, wheels, `UpperCoverMm`, `ClampHeightMm`, **stem angle**, and the whole handlebar (including roll). Spacers and stem length are fitted as **continuous** values; discrete stock parts and preference costs are Slice 3 (Q12).

## 2. Terminology (Q8, mandatory)

- Noun / result type: **Fitting**. Verb: **fit**. Entry point: **`ApplyFitting`**.
- Never write "search", "solver search", or "resolve" for this operation in code, comments, test names, or docs.
- The cockpit target landmark is **HandGrip** (the existing `HandlebarPlacement.HandGrip`). Don't call it "HB", "bar position", or "stem clamp".

## 3. The math (why the Slice 1 direct solve applies again)

From `PlaceStem` and `PlaceHandlebar`, with everything else fixed:

```text
HandGrip = HeadTubeTop
         + (UpperCoverMm + SpacersMm + ClampHeightMm/2) · dir(steerHeading)   // Advance up the steerer
         + LengthMm · dir(stemHeading)                                         // Advance along the stem axis
         + G                                                                   // BarClamp → HandGrip
```

`steerHeading` is the steerer heading (≈112.29° on the default build). `stemHeading = steerHeading − 90° + StemAngleDeg` (≈16.29° at −6°). `G` depends only on `HandlebarSetup`, because `PlaceHandlebar` uses `Face(−BarRollDeg)`, a world heading. None of these depend on the two fitted inputs, so HandGrip is **linear** in `(SpacersMm, LengthMm)` along two fixed directions. This is the same 2×2 box-constrained problem as Slice 1, and `fitBox` solves it once it's generalized (Commit 1).

### Rule: fitted directions come from placed poses (Q12)

Matt's ruling: **forward pose-based assembly stays the source of truth for geometry.** The formulas above explain why the solve works. **Code must not restate them.** Each fitted input's world direction is read from the pose that the input's own `Advance` step produced in the probe placement:

| Fitted input | Direction read from the probe | Why |
| --- | --- | --- |
| `Seatpost.ExtensionMm` | `probe.Seatpost.ShaftTop.HeadingDeg()` | `ShaftTop = SeatTubeTop.Advance(ExtensionMm)` |
| `Saddle.RailOffsetMm` | `probe.Saddle.RailCenter.HeadingDeg() + 180` | `RailCenter = RailFrame.Advance(−RailOffsetMm)` |
| `Stem.SpacersMm` | `probe.Stem.SpacerTop.HeadingDeg()` | `SpacerTop = CoverTop.Advance(SpacersMm)` |
| `Stem.LengthMm` | `probe.Stem.BarClamp.HeadingDeg()` | `BarClamp = stemAxis.Advance(LengthMm)` |

Slice 1 currently uses `probe.Frameset.SeatTubeTop.HeadingDeg()` and `setup.Saddle.RailPitchDeg + 180`. Commit 1 switches both to the table above. The values are bit-identical: `Advance` keeps the heading and `Face` sets it exactly. That keeps every Slice 1 test number unchanged.

### Limits

- Spacers: `[0, MaxSpacersMm]`. `MaxSpacersMm` is a new hardware field (the steerer allowance). `PlaceStem` rejects `SpacersMm > MaxSpacersMm` (Commit 2), the way Q9 made `PlaceSaddle` reject out-of-range rail offsets. `MaxSpacersMm == 0` means "no spacers fit". It is **not** an "unbounded" sentinel.
- Stem length: `[MinLengthMm, MaxLengthMm]`, new fields on `StemSetup` that only fitting uses (Q13). `PlaceStem` doesn't check them. `ApplyFitting` requires finite `0 < MinLengthMm ≤ MaxLengthMm` when it fits the cockpit.
- Fixture values (Commit 2), all **illustrative**: `MaxSpacersMm = 40`, `MinLengthMm = 40`, `MaxLengthMm = 130`.

### Box logic

Unchanged from Slice 1 §3 (unconstrained solve, then four edges in a fixed order, smallest miss wins, ties keep the earlier edge, snap within `fitBoundTolMm` of a bound counts as reached). Commit 1 only generalizes the inputs, from "extension in `[0, extRange]`, rail in `[−L, L]`" to two `fitAxis` values with arbitrary `[lo, hi]`.

### Front and rear are independent

The frameset doesn't depend on any of the four fitted inputs, so fitting the saddle and the cockpit together gives exactly the same values as fitting each one alone. Tests assert this with `==`.

## 4. Code changes

Do these as **four commits**, in order, each passing `make pre-commit`. Run `go vet ./... && go test ./...` after every edit. Each commit was checked against `deadcode` and `exhaustive` when it was planned. If one still fails on its own, stop and tell Matt rather than merging commits silently.

### Commit 1: generalize fitting (no behavior change)

Files: `internal/reboot/assembly/fitting.go`, `fitting_test.go`, `internal/reboot/web/web.go` (call site only).

1. Rename `SaddleFitTarget` → `FitTarget` and make the landmark optional:

   ```go
   // FitTarget holds the landmark targets for ApplyFitting, in world coordinates
   // (BB at origin, +X forward, +Y up, mm). A nil landmark is not fitted, and the
   // inputs that would fit it are kept from the setup.
   type FitTarget struct {
   	BRP *pose.Point // saddle BRP (Q7)
   }
   ```

   `ApplyFitting` returns a plain `fmt.Errorf("fit target has no landmarks")` when every landmark is nil. It stores **copies** of the target points in `Fitting.Target` (`p := *target.BRP; fit.Target.BRP = &p`), so later caller mutation can't change a result.

2. Replace `BrpMiss() pose.Point` and `BrpMissMm()` with one method:

   ```go
   // BrpMiss returns target − placed BRP (world mm), and false when the target has no BRP.
   func (f Fitting) BrpMiss() (pose.Point, bool)
   ```

   Callers take `.Norm()` for the magnitude. Delete `BrpMissMm`.

3. Generalize `fitBox`:

   ```go
   // fitAxis is one fitted input: the world heading its Advance moves the target
   // landmark along, its bounds relative to the probe value (hi may be +Inf), and
   // the input reported when the fitted value sits on a bound.
   type fitAxis struct {
   	heading pose.Deg
   	lo, hi  float64
   	field   InputField
   }

   // fitBox solves delta ≈ da·dir(a.heading) + db·dir(b.heading) with da in
   // [a.lo, a.hi] and db in [b.lo, b.hi]. ok is false when the headings are parallel.
   func fitBox(delta pose.Point, a, b fitAxis) (da, db float64, limited []InputField, ok bool)
   ```

   Keep the existing algorithm and comments, with these substitutions. The reachability test uses `[lo − fitBoundTolMm, hi + fitBoundTolMm]` on both axes. The edge candidates, in order, are `a` at `lo`, `a` at `hi` (skip if `+Inf`), `b` at `lo`, `b` at `hi` (skip if `+Inf`). `limited` lists `a.field` if `da == a.lo || da == a.hi`, then `b.field` likewise. The caller writes the parallel-headings error message.

4. Restructure `ApplyFitting` so that a second landmark can slot in (Commit 3 adds it):

   1. Validate: at least one landmark is set, and each set landmark is finite (`InputFitTargetBRP` as today).
   2. Build one probe setup from `setup`. For a BRP target, set `ExtensionMm = MinExtensionMm` and `RailOffsetMm = 0`. Run `probe, err := PlaceBike(probeSetup, nil)` and return errors unchanged.
   3. `fitted := setup`. For a BRP target, call the unexported `fitSaddle(setup, probe, brp) (ext, rail float64, limited []InputField, err error)`. It holds today's rail limit, `eMax` (+Inf when `MaxExtensionMm == 0`), the delta, the two axes (directions from the §3 table), the `fitBox` call, the parallel error ("saddle rail pitch is parallel to the seat axis; cannot fit the saddle"), and the clamping into range. Write its results into `fitted` and append `limited` to the fitting's `LimitedBy`.
   4. `placement, err := PlaceBike(fitted, trace)`. The trace records only this final placement.
   5. `Outcome = FitLimited` if `LimitedBy` is non-empty, else `FitReached`.
   6. **Final check, per landmark:** for each set landmark whose own fit added nothing to `LimitedBy`, return an error if its miss is `> 1e-6` mm ("saddle fitting forward check missed target by …").

5. `web.go`: build `assembly.FitTarget{BRP: &brp}` and read `*fitting.Target.BRP` in `buildFittingView`. Nothing else changes in the web layer in this commit.

6. Tests: switch to the new API. `mustFit` wraps the point. Add a test helper `brpMissMm(t, fit)` that fails when `BrpMiss` returns false. **Every existing assertion and tolerance stays as it is.** If a number moves, the refactor is wrong. Add:
   - Empty `FitTarget{}` → error.
   - Mutating the caller's target point after `ApplyFitting` returns doesn't change `fit.Target.BRP`.

Commit subject: `Generalize reboot fitting for more than one landmark`.

### Commit 2: spacer and stem length limits on `StemSetup`

Files: `internal/reboot/assembly/stem.go`, `stem_test.go`, `handlebar_test.go`, `internal/reboot/web/fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`.

1. Add three fields to `StemSetup` and document them in its doc comment list:

   ```go
   MaxSpacersMm float64 // most spacer stack the steerer allows above the upper cover; 0 means none fit
   MinLengthMm  float64 // shortest stem length fitting may choose; PlaceStem ignores it
   MaxLengthMm  float64 // longest stem length fitting may choose; PlaceStem ignores it
   ```

2. `PlaceStem`, right after the existing spacers check:
   - If `MaxSpacersMm` is not finite or is `< 0`, return `fmt.Errorf("max spacers must be finite and non-negative, got %v mm", …)`.
   - If `SpacersMm > MaxSpacersMm`, return `invalidInput(InputStemSpacers, "spacers must be at most %v mm (steerer allowance), got %v mm", MaxSpacersMm, SpacersMm)`. Equality places, with no tolerance, so fitted values exactly at the bound place.

3. Fixture `DefaultStem`: `MaxSpacersMm: 40.0, // illustrative: steerer length not measured`, `MinLengthMm: 40.0, MaxLengthMm: 130.0, // illustrative stem length range`. In `bikeFixture.validate()`, reject a `DefaultStem` whose length range isn't finite with `0 < MinLengthMm ≤ MaxLengthMm`.

4. Add `MaxSpacersMm: 40` to **every** `StemSetup` literal in the assembly tests, including each `TestPlaceStemValidation` case so each still fails for the reason it names. Also add `MinLengthMm: 40, MaxLengthMm: 130` to `fenrirDefaultStem()` in `handlebar_test.go` (Commit 3 uses it). New `TestPlaceStemValidation` cases: spacers 41 with max 40 → `*InputError` with `InputStemSpacers`; max −1 → error; max NaN → error. Add a separate test that spacers exactly 40 with max 40 places.

5. Workbench: add `MaxSpacersMm float64` to `bikeView`. Fill it from `fix.DefaultStem.MaxSpacersMm` in both `buildBikeView` and `renderBikeError`, and render `max="{{.MaxSpacersMm}}"` on `#ctrl-spacers`. Tests: the `max` attribute is `"40"`; `/bike?spacers=40` → 200; `/bike?spacers=45` → 400 with `aria-invalid="true"` on `#ctrl-spacers`. Existing tests use spacers ≤ 40 and must pass unchanged.

Commit subject: `Add spacer and stem length limits to the reboot stem setup`.

### Commit 3: cockpit fitting in `assembly`

Files: `internal/reboot/assembly/validate.go`, `fitting.go`, `fitting_test.go`, `internal/reboot/web/web.go` (exhaustive switches only).

1. `validate.go`: add `InputFitTargetGrip InputField = "HandGrip fit target"`. In `web.go`, map it to `"grip_x"` in `bikeInputField`. Add it to the "never limits" case in `buildFittingView`'s switch, which Commit 4 rewrites. `make exhaustive` must pass.

2. `FitTarget` gains `HandGrip *pose.Point // handlebar HandGrip (Q11)`. Add:

   ```go
   // GripMiss returns target − placed HandGrip (world mm), and false when the target has no HandGrip.
   func (f Fitting) GripMiss() (pose.Point, bool)
   ```

3. `ApplyFitting` additions, slotted into the Commit 1 steps:
   - Step 1: a non-finite `HandGrip` → `invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", …)`.
   - Step 2, for a grip target: validate the stem length range (finite, `0 < MinLengthMm ≤ MaxLengthMm`, else a plain `fmt.Errorf`), then set probe `SpacersMm = 0` and `LengthMm = MinLengthMm`. One probe `PlaceBike` serves both landmarks.
   - Step 3: `fitCockpit(setup, probe, grip) (spacers, length float64, limited []InputField, err error)`:
     - `delta := grip.Sub(probe.Handlebar.HandGrip.Origin())`
     - `a := fitAxis{probe.Stem.SpacerTop.HeadingDeg(), 0, setup.Stem.MaxSpacersMm, InputStemSpacers}`
     - `b := fitAxis{probe.Stem.BarClamp.HeadingDeg(), 0, setup.Stem.MaxLengthMm - setup.Stem.MinLengthMm, InputStemLength}`
     - If `fitBox` is not ok, return the error "stem axis is parallel to the steerer; cannot fit the cockpit".
     - `spacers = clamp(0 + ds, 0, MaxSpacersMm)` and `length = clamp(MinLengthMm + dl, MinLengthMm, MaxLengthMm)`.
   - `LimitedBy` order is always: `InputSeatpostExtension`, `InputSaddleRailOffset`, `InputStemSpacers`, `InputStemLength` (saddle part first, then cockpit).
   - Step 6: the per-landmark final check also covers HandGrip ("cockpit fitting forward check missed target by …").
   - Update the `ApplyFitting` and `Fitting` doc comments: it fits the saddle inputs for a BRP target and the cockpit inputs for a HandGrip target, keeps every other field, and `LimitedBy` uses the order above.

4. Tests in `fitting_test.go`. Add `fenrirDefaultStem()` limits (Commit 2) to the setup and a `gripOf(t, setup)` helper like `brpOf`. Use a package-level `fitReportGrip = pose.Point{X: 481, Y: 726}` with a comment: idmatch report handlebar coordinates, `docs/RIDER_FIT.md`; whether the report point is HandGrip or the stem clamp is unresolved. Independent math in tests may use `math.Cos`/`math.Sin` directly.
   - **Round trip, reached:** take the target from spacers 10 / length 90 (stem −6°) and start from the default 20 / 60 → `FitReached`, 10 and 90 to 1e-9, `GripMiss` norm < 1e-9, `LimitedBy == nil`. Seatpost and saddle setups equal the input, because there's no BRP target.
   - **Reached on bounds:** targets taken from (0, 40), (40, 130), and (0, 130) → `FitReached`, values exactly on the bounds (this exercises the snap).
   - **Fit report grip, default build (−6°):** `LimitedBy == [InputStemSpacers, InputStemLength]`, spacers exactly 0, length exactly 130, miss ≈ (17.007, −30.248), |miss| ≈ 34.701. Assert to 0.01 mm.
   - **Same target, stem angle −17°:** both limits, 0 and 130, miss ≈ (12.342, −5.769), |miss| ≈ 13.624.
   - **Spacers limit only:** target `= gripOf(spacers 0, length 100) − 20·u`, where `u` is the unit vector at `SpacerTop.HeadingDeg()` from a placement. Expect `LimitedBy == [InputStemSpacers]`, spacers 0, length `100 − 20·cos(96°)` ≈ 102.0906, and |miss| `20·sin(96°)` ≈ 19.8904 (1e-6).
   - **Length limit only:** target `= gripOf(spacers 20, length 130) + 15·v`, where `v` is the unit vector at `BarClamp.HeadingDeg()`. Expect `LimitedBy == [InputStemLength]`, length 130, spacers `20 + 15·cos(96°)` ≈ 18.4321, and |miss| `15·sin(96°)` ≈ 14.9178 (1e-6).
   - **Fixed inputs kept:** after any cockpit fit, `Stem.AngleDeg`, `UpperCoverMm`, `ClampHeightMm`, and the whole `Handlebar` equal the input.
   - **Both landmarks:** `FitTarget{BRP: &fitReportBRP, HandGrip: &fitReportGrip}` → `LimitedBy == [InputSaddleRailOffset, InputStemSpacers, InputStemLength]`. All four fitted values `==` the values from the two single-landmark fits.
   - **Trace on/off equivalence** with both landmarks set, like the existing test.
   - **Errors:** non-finite grip → `*InputError` with `InputFitTargetGrip`. With a grip target, `MinLengthMm = 0`, `MinLengthMm > MaxLengthMm`, and NaN `MaxLengthMm` → error. With only a BRP target, that same invalid stem range → no error. `MaxSpacersMm = −1` → PlaceBike's error unchanged.
   - **Minimality:** for the spacers-only case, sweep spacers 0..40 and length 40..130 on a 1 mm grid with forward `PlaceBike`. Assert that no grid point beats the fitting's miss by more than 1e-6 mm. Test-only.

Commit subject: `Add front-cockpit fitting to a HandGrip target in reboot assembly`.

### Commit 4: `/bike` workbench

Files: `internal/reboot/web/web.go`, `fixture.go`, `templates/bike.html`, `web_test.go`.

1. **State refactor.** Replace `HasBrpTarget`, `BrpTargetX`, and `BrpTargetY` on `workbenchState` with one field `Targets fitTargetState`:

   ```go
   // fitTargetState is the optional fitting targets in the URL query
   // (brp_x/brp_y and grip_x/grip_y). Both blank in a pair means no target.
   type fitTargetState struct {
   	HasBrp       bool
   	BrpX, BrpY   float64
   	HasGrip      bool
   	GripX, GripY float64
   }

   func (t fitTargetState) any() bool
   func (t fitTargetState) fitTarget() assembly.FitTarget
   ```

   `URL()` writes `brp_x`/`brp_y` when `HasBrp`, then `grip_x`/`grip_y` when `HasGrip`. `formValues()` echoes `BrpX`, `BrpY`, `GripX`, `GripY` (blank when unset). `bikeFormValues` gains `GripX` and `GripY`. `withCadBaselineFork` and `withView` carry `Targets` unchanged. Reset Defaults clears both targets, because the default state has none. Rename `withBrpTarget` → `withFitTargets`; it copies `submitted.Targets` in one assignment.

2. **Parsing.** Replace the BRP-specific block in `decodeWorkbenchState` with a helper used twice:

   ```go
   // optionalPoint parses an optional X/Y pair. Both blank → (zero, false).
   // Exactly one blank → input error on the blank key ("<label> needs both X and Y").
   func (p *bikeQueryParser) optionalPoint(xKey, yKey, label string, echoX, echoY *string) (pose.Point, bool)
   ```

   Call it with `("brp_x", "brp_y", "BRP target", …)` and `("grip_x", "grip_y", "HandGrip target", …)`. It must keep the existing BRP error text, "BRP target needs both X and Y".

3. **Fixture.** Add `ExampleGripTarget pose.Point` = `(481, 726)` with the comment `// measured: idmatch fit report handlebar coordinates (docs/RIDER_FIT.md); whether the report point is HandGrip or the stem clamp is unresolved`. Use it only as the grip inputs' `placeholder`. `bikeView` gains `ExampleGripX` and `ExampleGripY`, filled in both builders.

4. **Handler.** If `resolved.State.Targets.any()`, call `assembly.ApplyFitting(resolved.Setup, resolved.State.Targets.fitTarget(), nil)` once. A fitting error renders like any other input error through `bikeInputField`. The diagram, trace, and form still show the **submitted** build.

5. **View data.** Replace `bikeFittingView` with:

   ```go
   type bikeFittingView struct {
   	Saddle   *saddleFittingView  // nil unless a BRP target is set
   	Cockpit  *cockpitFittingView // nil unless a HandGrip target is set
   	ApplyURL string              // the fitted build, with the submitted targets kept
   }
   type saddleFittingView struct {
   	TargetX, TargetY, MissX, MissY, MissMm float64 // miss: target − the submitted build's BRP
   	ExtensionMm, RailOffsetMm, RailLimitMm  float64 // fitted values; ±RailLimitMm range
   	LimitedBy                               []assembly.InputField
   }
   type cockpitFittingView struct {
   	TargetX, TargetY, MissX, MissY, MissMm float64 // miss: target − the submitted build's HandGrip
   	SpacersMm, MaxSpacersMm                 float64 // fitted; range 0..MaxSpacersMm
   	StemLengthMm, MinStemLengthMm, MaxStemLengthMm float64
   	StemAngleDeg, BarRollDeg                pose.Deg // fixed inputs the fit used
   	LimitedBy                               []assembly.InputField
   }
   ```

   - The current-build miss uses `bike.Handlebar.HandGrip.OffsetTo(pose.NewPose(target, 0, nil))`, mirroring the BRP code. Limits and ranges come from `bike.Setup` (`RailOffsetLimitMm()`, `Stem.MaxSpacersMm`, `Stem.MinLengthMm`/`MaxLengthMm`), never from the fixture.
   - `buildFittingView` sorts `fitting.LimitedBy` into the two views with one **exhaustive** `switch`: extension and rail offset go to the saddle; spacers and length go to the cockpit; every other constant goes in a no-op case.
   - `OutcomeText()` on both views calls one helper, `fitOutcomeText(limits []assembly.InputField) string`. It returns `"reached"` when `limits` is empty, else `"limited by "` + the `string(field)` values joined with `", "`. This reproduces Slice 1's texts exactly ("limited by seatpost extension, saddle rail offset") and gives "limited by stem spacers, stem length" for the cockpit. Word it as a fact about this build (Q10).
   - `ApplyURL = workbenchStateFromSetup(view, forkID, fitting.Placement.Setup)` with `Targets` set to the submitted targets.

6. **Template.**
   - Controls: after the "BRP fit target" fieldset, add a "HandGrip fit target" fieldset with `ctrl-grip-x` / `ctrl-grip-y` (`name="grip_x"`/`"grip_y"`, `type="number"`, `step="any"`, placeholders from the fixture, and the same `aria-invalid` pattern).
   - Diagram card: keep a `<div data-fitting>` wrapper around `{{with .Saddle}}` and `{{with .Cockpit}}` blocks, each its own `<div class="summary">` with a heading ("Saddle fitting" / "Cockpit fitting"), then one link: `<a id="apply-fitting" href="{{.ApplyURL}}" data-preset="apply-fitting">Apply fitted build</a>`.
   - Saddle hooks: keep `fit-brp-miss`, `fit-extension`, `fit-rail-offset`. **Rename** `fit-outcome` → `fit-saddle-outcome`, and update the three existing test assertions that read it.
   - Cockpit hooks: `fit-grip-miss` (magnitude, like `fit-brp-miss`), `fit-spacers` (`"%.1f mm"`, with range text "0–40 mm on this steerer" built from the view), `fit-stem-length` (`"%.1f mm"`, range text "40–130 mm available"), `fit-cockpit-outcome`. Add one plain line: "Fitted at stem angle −6.0°, bar roll 0.0°; spacers and stem length are continuous values." Format the numbers from the view.
   - No JS change: `buildFormURL` serializes the whole form, and `a[data-preset]` navigation is generic. If you find otherwise, add a `workbench_test.js` case.
   - No SVG target markers in this slice.

7. **Tests** (`httptest` + `goquery`, per AGENTS.md). Add a `gripTestTarget` helper like `fittingTestTarget`.
   - No targets → no `[data-fitting]`, no `#apply-fitting`, grip inputs blank with placeholders.
   - `grip_x=481&grip_y=726` → 200. `fit-cockpit-outcome` = "limited by stem spacers, stem length", `fit-spacers` = "0.0 mm", `fit-stem-length` = "130.0 mm", `fit-grip-miss` = "96.3 mm" (the default build's grip is ≈ (389.218, 755.120)). There's no saddle block. The apply href has `spacers=0.0`, `stem_length=130.0`, `grip_x=481.0`, `grip_y=726.0`, and no `brp_x`.
   - Reachable round trip: target from `spacers=10&stem_length=90` → "reached". Following `apply-fitting` gives `fit-grip-miss` "0.0 mm" and "reached".
   - Both targets (`brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`) → both blocks and one `#apply-fitting`. The href has `rail_offset=15.0`, `post_ext` ≈ 260.092, `spacers=0.0`, `stem_length=130.0`, and all four target keys.
   - Only `grip_x` → 400 with `aria-invalid` on `#ctrl-grip-y`; only `grip_y` → on `#ctrl-grip-x`; `grip_x=NaN&grip_y=1` → on `#ctrl-grip-x`.
   - Reset Defaults drops both targets. CAD Baseline Fork, the view links, and the canonical URL keep both.
   - All Slice 1 fitting tests still pass, with only the `fit-saddle-outcome` rename.

Commit subject: `Show front-cockpit fitting to a HandGrip target in the reboot bike workbench`.

## 5. Reference numbers (default Fenrir build, measured 2026-11-19 with an independent probe)

Default build: Lithic fork, EC44 12 mm cup, spacers 20, stem 60 mm at −6°, Persuader 800 at 0° roll. Limits: spacers 0–40, stem 40–130.

| Quantity | Value |
| --- | --- |
| HeadTubeTop | (421.94, 646.00), steerer heading 112.289° |
| BarClamp heading (stem axis at −6°) | 16.289° |
| BarClamp → HandGrip (`G`) | (−70.974, +45.105) |
| Default HandGrip | (389.218, 755.120) |
| Grip at spacers 10 / stem 90 | (421.806, 754.282) |
| Fit report grip (481, 726), unconstrained, −6° | spacers −33.99, stem 134.29 |
| Same, unconstrained, −17° / 0° / +6° | (−7.20, 139.65) / (−48.03, 133.55) / (−62.06, 134.29) |
| Limited fit, −6° | spacers 0, stem 130, miss (17.007, −30.248), \|miss\| 34.701 |
| Limited fit, −17° | spacers 0, stem 130, miss (12.342, −5.769), \|miss\| 13.624 |
| Limited fit, 0° | spacers 0, stem 130, \|miss\| 48.158 |

Finding to record in the commit message for Commit 3 (not a defect): on this frame, the fit report's handlebar point needs negative spacers at every common stem angle, whether it is read as HandGrip or as the stem clamp (stem clamp (481, 676) needs ≈ −18.7 mm spacers at −6°). If your numbers differ by more than 0.01 mm, find out why before changing a test.

## 6. Not in this slice

Discrete stock stems and spacer steps, choosing a stem angle, handlebar choice or roll, preference costs (all Slice 3); a target relative to the saddle; SVG target markers; post setback choice; other frames; CLI/MCP access.

## 7. Rules that bite here

- **Q12:** fitted directions come from placed probe poses (§3 table). No trig, radians, or restated geometry in `assembly` or `web`. If you need a vector operation, add it to `pose` with a test (none should be needed: `Along`, `ProjectOnto`, `DecomposeAlong`, `Sub`, `Norm`, and `OffsetTo` exist).
- Scalars derived from placed poses are methods or come straight from placements. The web never recomputes a fitted value or a limit.
- `exhaustive`: every `switch` on `InputField` or `FitOutcome` names every constant, including `InputFitTargetGrip`.
- `deadcode`: no unused exports or helpers. `boundary`: no legacy imports, and never write the banned term listed in AGENTS.md.
- Placements must be identical with tracing on or off.
- Commit messages: plain subjects, no Conventional Commits prefixes; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 2 done at the end, and only after Matt accepts it.

## 8. Verification

After each commit, run `make pre-commit` (re-run until `go fix` is clean). After Commit 4, check it live: `make serve-reboot`, then open `http://127.0.0.1:8081/bike?grip_x=481&grip_y=726&view=stem` and `…&brp_x=-219&brp_y=713.1&view=bike`. Confirm both fitting blocks, the single apply link, and that applying it moves HandGrip to the fitted position (miss 34.7 mm, limited). Take a screenshot for Matt.
