# Static and dynamic transfer specification

Status: Ruled 2026-11-23 ([Q159–Q172](../DESIGN.md)). Expresses static and dynamic placement, target conversion, and saddle-to-cockpit coupling using `pose.Pose` (turtle graphics).

## 1. Coordinate systems and variables

### Coordinate systems

1. **World coordinate system (bike frame at a given state):**
   - A 2D side-view plane with origin at `BB = (0, 0)`, `+X` horizontally forward along the leveled axle line (`0°` heading), and `+Y` vertically upward (`90°` heading). Positive turns are counter-clockwise.
   - Each build has two world states (Q169):
     - **Static world:** both wheels on level ground with the fork and seatpost fully extended (`0%` sag).
     - **Dynamic world:** both wheels on level ground with the fork and seatpost compressed to their steady seated sag (Q159, Q161).
2. **Seatpost coordinate system (invariant, owned by the seatpost):**
   - Fixed to the seatpost shaft itself; it never changes relative to the seatpost.
   - Matches `PlaceSeatpost`'s turtle at `SeatTubeTop`:
     - **Along-shaft axis (`local +X`):** points upward along the seatpost shaft from bottom to head.
     - **Setback axis (`local +Y`):** points rearward, perpendicular to the seatpost shaft (`90°` counter-clockwise from `local +X`).
   - Because a turtle placed at `SeatTubeTop` already points along `local +X` (up the shaft), `Pose.Translate(alongShaftMm, rearwardSetbackMm)` rigidly transforms seatpost-local displacements into the active world coordinate system.

### Variables

- **`BRP_target`, `HB_target`:** The rider's desired saddle (`BRP`) and handgrip (`HB`) positions, expressed as points `(X, Y)` in the **dynamic world** coordinate system.
- **`brpToHbOffset = HB_target − BRP_target`:** The rider's desired 2D saddle-to-handgrip displacement vector `(ΔX, ΔY)` in the **dynamic world** coordinate system (Q172).
- **`BRP_dynamic`, `HB_dynamic`:** A build's placed saddle and handgrip points `(X, Y)` in the **dynamic world** coordinate system.
- **`coupledHbTarget = BRP_dynamic + brpToHbOffset`:** The front-cockpit handgrip target in the **dynamic world** coordinate system after fitting the saddle to `BRP_dynamic` (Q172).
- **`BRP_static`, `HB_static`:** A build's placed (or workshop-measured) saddle and handgrip points `(X, Y)` in the **static world** coordinate system.
- **Frame leveling pitch (`FramesetPlacement.BB.HeadingDeg()`):** The angle by which `PlaceFrameset` rotates the unpitched chart frame about `BB` so the rear-to-front axle line is horizontal for the current fork length.
- **`forkSagPitchDelta`:** How many degrees the frame rotates about `BB` when the fork compresses from static (`0%` sag) to dynamic (seated sag):
  `forkSagPitchDelta = dynamicFrameset.BB.HeadingDeg() − staticFrameset.BB.HeadingDeg()`
  *(negative when fork sag drops the front end nose-down; `0°` for a rigid fork).*

## 2. Seatpost geometry in seatpost coordinates (`redshift_shockstop_pro`, Q159)

In the seatpost's invariant `(alongShaft, rearwardSetback)` coordinates ([source record](../../internal/catalog/seatpost_redshift_shockstop_pro.md)):

- **Static (unloaded) rail cradle from `SeatTubeTop`:**
  - Advance up the shaft by `ExposedMm = 350 − InsertionMm`, then translate `7 mm` rearward perpendicular to the shaft (`StaticSetbackMm = 7 mm`):
    `staticCradle = seatTubeTop.Advance(ExposedMm).Translate(0, 7)`
- **20% seated sag step (static → dynamic):**
  - When the rider sits down, the linkage moves **6 mm down along the shaft** (`−6 mm`) and **5 mm rearward perpendicular to the shaft** (`+5 mm`), keeping saddle pitch constant relative to the post shaft:
    `dynamicCradle = staticCradle.Translate(-6, +5)`
    *(landing at `ExposedMm − 6 mm` along the shaft and `7 + 5 = 12 mm` perpendicular rearward setback).*
- **Unloading step (dynamic → static):**
  - The inverse turtle step in seatpost coordinates moves **6 mm up along the shaft** (`+6 mm`) and **5 mm forward perpendicular to the shaft** (`−5 mm`):
    `staticCradle = dynamicCradle.Translate(+6, -5)`
- *(For a rigid seatpost, the sag and unloading steps are `Translate(0, 0)`.)*

## 3. Turtle-graphics operations

### A. Fitting a candidate build in the Dynamic World (`PlaceBike` + `ApplyFitting`)

1. **Frameset:** Place the frame with the fork's dynamic A2C (at class sag, Q161) and level the axles (`dynamicFrameset`).
2. **Seatpost:** From `dynamicFrameset.SeatTubeTop`, walk to `staticCradle` and apply the seatpost sag step `.Translate(-6, +5)` to reach `dynamicCradle`.
3. **Saddle:** At `dynamicCradle`, face the dynamic world rail heading (`Face(+5°)` on SQlab 610, Q165), slide by `−RailOffsetMm`, and place `BRP_dynamic` and `SIT_dynamic`.
   - Record the clamped post-to-rail angle: `clampedRailTurn = +5° − dynamicCradle.HeadingDeg()`.
   - Fit `InsertionMm` and `RailOffsetMm` to `BRP_target` (Q168), yielding the fitted `BRP_dynamic`.
4. **Cockpit (Q172 coupling):**
   - Compute `coupledHbTarget = BRP_dynamic + brpToHbOffset` (or `HB_target` when `brp=off` on `/bike`).
   - At `BarClamp`, face the dynamic world roll (`Face(0°)`, Q163), recording the clamped stem-to-bar angle `clampedBarTurn = 0° − BarClamp.HeadingDeg()`, and fit/score the cockpit against `coupledHbTarget`.

### B. Dynamic → Static unloading (Candidate Dynamic Build → Static Workshop Coordinates)

To find `(BRP_static, HB_static)` for a fitted candidate build without loosening any clamp bolts:

1. **Unload the seatpost (in the seatpost's coordinate system):**
   - Place a turtle at `BRP_dynamic` oriented along the dynamic seatpost shaft (`dynamicFrameset.SeatTubeTop.HeadingDeg()`) and step `.Translate(+6, -5)` (`6 mm` up the shaft, `5 mm` forward perpendicular to the shaft).
2. **Unload the fork (frame rotation about `BB`):**
   - Rotate both the unloaded-post `BRP` point and `HB_dynamic` about `BB` by `−forkSagPitchDelta` (as the fork extends to `0%` sag and pitches the frame, stem, clamped bar, seatpost, and clamped saddle back to the static attitude), yielding `(BRP_static, HB_static)`.
   - *(This produces the exact same `(BRP_static, HB_static)` as walking the static frameset from `BB` at `0%` fork sag, stopping at `staticCradle`, and using the clamped turns `Turn(clampedRailTurn)` and `Turn(clampedBarTurn)` instead of `Face`.)*

### C. Static → Dynamic target conversion (Source Bike Static Measurements → Dynamic Targets)

When a rider enters static workshop measurements `(BRP_static, HB_static)` taken on a source bike:

1. **Apply the source fork's sag (frame rotation about `BB`):**
   - Rotate both `BRP_static` and `HB_static` about `BB` by the source bike's `forkSagPitchDelta`.
   - The rotated `HB` point is `HB_dynamic`.
2. **Apply the source seatpost's sag (in the seatpost's coordinate system):**
   - Place a turtle at the fork-rotated `BRP` point, give it the heading of the source bike's dynamic seatpost shaft (`dynamicFrameset.SeatTubeTop.HeadingDeg()`), and step `.Translate(-6, +5)` (`6 mm` down the shaft, `5 mm` rearward perpendicular to the shaft) to land on `BRP_dynamic`.
