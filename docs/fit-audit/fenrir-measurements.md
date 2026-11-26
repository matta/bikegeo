# Fenrir physical check & component measurements

Status: Ready for measurement, 2026-11-24.

## Purpose

Before using `bikegeo` to specify a custom frame, we measure Matt's physical Otso Fenrir to establish primary ground truth and verify our geometry engine against a real bicycle:

1. **Establish a new, primary fit baseline directly from the Fenrir:**
   - We do not trust the earlier idmatch fit report as a primary source and expect this effort to produce new, distinct saddle (`BRP`) and hand (`HB`) coordinates directly from the physical bike. Comparing against the idmatch report afterward is strictly a secondary sanity check ([`RIDER_FIT.md`](../RIDER_FIT.md)).
   - This session directly establishes the unloaded static baseline (plus seated fork sag). The dynamic riding targets used by `bikegeo` are derived afterward from that static baseline using measured fork sag and Redshift's documented seatpost sag; dynamic seatpost sag is not measured here.
2. **Validate `bikegeo`'s physical geometry model against reality:**
   - Check that `bikegeo` models a real bicycle accurately—especially its most esoteric 3D-to-2D handlebar projection (`PlaceHandlebar`), as well as the stem/cockpit (`PlaceStem`) and bare Otso Fenrir frame geometry (`PlaceFrameset`).
   - Resolve genuine unknowns on Matt's current build, specifically the installed stem's uncertain length (probably `100 mm` or `110 mm`) and unknown angle.
3. **Replace guessed or suspect catalog constants with verified physical values:**
   - Several catalog constants are currently illustrative guesses or indirect derivations that warrant verification: SQlab 610 `RailToBrpHeightMm` (guessed at `45.0 mm`), SQlab 610 `W70` station (derived at `120 mm`), headset `UpperStackMm` (guessed at `11 mm`), and stock Redshift `RailClampLengthMm` (guessed at `30.0 mm`, if the removed clamp piece is handy).
   - *(Scope note: because the FAIR Drop Best replaces the Redshift ShockStop PRO's original saddle-rail jaw, verifying the stock Redshift seatpost's static setback on the bike is out of scope while the Drop Best remains installed; static and dynamic Redshift linkage geometry remains on Redshift's published specifications.)*

---

## Approach

### 1. Measure every component boundary in the kinematic chain so errors cannot hide or cancel

Instead of measuring only the endpoints (`BRP` and `HB`) and guessing which component caused a discrepancy, we measure the intermediate attachment landmarks along both branches of the kinematic tree:

- **Rear branch:** `BB` → **`SeatTubeTop`** (checks bare frame seat tube) → **`BRP`** (measures Matt's actual static saddle coordinate directly on the SQlab 610, bypassing the unmodeled Drop Best).
- **Front branch:** `BB` → **`HeadTubeTop`** (checks bare frame reach and stack) → **`BarClamp`** (checks headset stack, spacers, and stem length/angle) → **`HB`** (checks handlebar rise, backsweep, upsweep, and roll in isolation).

### 2. Why no parts need to be unbolted or swapped

- **Leave the FAIR Drop Best bolted to the Redshift post (and the SQlab 610 clamped in the Drop Best):**
  - Because the Drop Best replaces the Redshift post's original saddle-rail jaw, we do **not** try to measure a stock Redshift `RailPivot` on the bike (static Redshift setback and head stack stay on Redshift's published specs).
  - Instead, we tape the exact `SeatTubeTop` collar line on the shaft, slide the assembled `Redshift + Drop Best + SQlab 610` unit out as one piece for the mandatory flat-surface bench measurement of `SeatTubeTop → BRP` in the seatpost's local coordinates from `SeatTubeTop` (Part A1), and then reinstall it flush to the tape mark before Part B.
  - Measuring `BB → BRP` directly on the bike in Part B (`B3`) and cross-checking it against `SeatTubeTop + bench(SeatTubeTop → BRP)` (**Closure 4**) locks down Matt's actual static riding saddle position `BRP_static`. When converting `BRP_static` to `BRP_dynamic` ([`brp-transfer.md` Section 3.C](brp-transfer.md#c-static--dynamic-target-conversion-source-bike-static-measurements--dynamic-targets)), the Drop Best is a rigid clamp attached to the top of the Redshift linkage: it rotates about `BB` with fork sag and translates by `Translate(-6, +5)` in the seatpost's local coordinates with Redshift linkage sag, so its internal offset cancels out completely.
- **Leave the RockShox SID 35 mm (110 mm) fork installed:**
  - Do not swap in the Lithic rigid fork. To [`PlaceFrameset`](../../internal/assembly/frameset.go), a fork is completely determined by its axle-to-crown length (`A2cMm`) and offset (`RakeMm`, `44 mm`). Measuring the installed SID 110's static A2C and its actual seated O-ring sag in millimeters tests the frame as built and captures the true pitch and fork sag of Matt's riding setup.

### 3. Why we use an axle-leveled datum and a center-plane `HB` bridge instead of floor/wall or 3D grip triangulation

1. **Old-house floor and wall slopes:** Measuring coordinates blindly from an unlevel floor or an out-of-plumb wall injects floor pitch and wall tilt into every horizontal and vertical span. Shimming one tire until a **long level held across the front and rear axle centers** reads level (`0°`, verified by flipping the level `180°` end-for-end) establishes the bike's true `StaticWorld` horizontal axis (`+X`) independently of room slope.
2. **Flat-floor vertical heights for `LeveledBbDropMm` instead of 3-chord triangle solving (`6.7×` lower uncertainty):** Because the base triangle `(RearAxle, BB, FrontAxle)` is very flat (`70 mm` drop over a `1156 mm` wheelbase), inferring `LeveledBbDropMm` from the three chord lengths (`c = d(RearAxle, BB)`, `f = d(BB, FrontAxle)`, `w = d(RearAxle, FrontAxle)`) amplifies chord errors by **`3.9×`** (`∂(BbDrop)/∂c = +3.92`, `∂(BbDrop)/∂f = +3.89`, `∂(BbDrop)/∂w = -3.87`), producing an unusable **`±16.2 mm` 95% error band**. Instead, checking that a `1.2 m` strip of floor beneath the tires is **flat** (no gap under the long level laid on the floor) and measuring vertical heights from the flat floor to the rear axle (`y_rear`), front axle (`y_front`), and `BB` center (`y_bb`) gives:
   - `LeveledBbDropMm = (1 − α) · y_rear + α · y_front − y_bb`, where `α = c / (c + f) ≈ 0.394` (which reduces to `(y_rear + y_front)/2 − y_bb` when the floor itself is level so `y_rear = y_front`, and automatically cancels any linear floor slope when interpolated at `α`).
   - That cuts the 95% error band on `LeveledBbDropMm` by **`6.7×`, from `±16.2 mm` down to `±2.42 mm` (`1σ = ±1.23 mm`)**, and turns the measured wheelbase `w` into an overdetermined closure check: `r_wheelbase = w − (sqrt(c² − h²) + sqrt(f² − h²))`.
3. **3D lateral parallax at the handlebar (`Z ≈ ±320 mm`):** [`PlaceHandlebar`](../../internal/assembly/handlebar.go) computes the lateral grip coordinate `zHand = width/2 − GripInsetMm` (`HandLateralMm`, ~`320 mm` outboard of the frame's `Z = 0` center plane) and orthographically projects `HB` onto the 2D center plane `(X_HB, Y_HB, Z = 0)`. Stretching a tape measure diagonally in 3D from `BB` or `FrontAxle` (`Z ≈ 0`) out to a grip at `Z = ±320 mm` measures the 3D hypotenuse `sqrt(X² + Y² + 320²)`, adding **~`57 mm` of systematic 3D error**. Bridging a straight dowel/ruler across the left and right `80 mm` grip marks physically performs that exact 3D → 2D projection: its midpoint sits at `(X_HB, Y_HB + D_grip/2, Z = 0)` right in the bicycle's center plane behind the stem.
4. **Eliminating tube-center eyeballing and 1.2-meter stretches:** Measuring to the top surface of the `HB` centerline bridge (and subtracting half the caliper-measured grip diameter `D_grip/2`) locates the exact center of the handlebar tube without eyeballing, and measuring `BRP → HB`, `BarClamp → HB`, and `HeadTubeTop → BarClamp` across the top of the bike with the horizontal long level avoids 1.2-meter diagonals from the axles.

### 4. High-value physical measurement discipline

To keep every derived coordinate inside a known, tight uncertainty band without wasting effort on low-sensitivity inputs, apply four workshop rules:

1. **`100 mm` "burned-decimeter" zero offset on tape-measure chords:** A tape measure's sliding metal hook has `0.5–1.5 mm` of play. For every critical point-to-point chord (`d(BB, BRP)`, `d(BB, HeadTubeTop)`, `d(BB, SeatTubeTop)`, `d(RearAxle, BB)`, `d(BB, FrontAxle)`, `d(RearAxle, FrontAxle)`, `d(BRP, HeadTubeTop)`), align the **`100 mm` graduation** directly on the origin mark, read the target mark, and subtract `100 mm` (or record both the raw reading and `−100 mm`).
2. **Flip the long spirit level `180°` end-for-end on every horizontal sighting:** Whenever you use the long level to level the axle line or to project a horizontal datum between `HB_bridge` and `BRP` / `BarClamp` / `HeadTubeTop`, take one reading, flip the level end-for-end (`180°`), re-center the bubble, and average the two sightings. End-for-end reversal completely cancels any systematic vial calibration tilt (`Δ_sys`).
3. **Measure bore/tube edges instead of eyeballing centers on floor verticals:** When measuring `y_rear`, `y_front`, and `y_bb` vertically from the flat floor with a rigid ruler/square (or tape butted against the floor), measure to the **bottom edge and top edge** of the thru-axle hex recess or `BB` spindle bore and average `(y_bottom + y_top) / 2` (or measure to the bottom edge of a known diameter and add its radius).
4. **Take 3 independent re-seated readings (`R1, R2, R3`) on the high-variance-share inputs:** Jacobian variance-share analysis (`Share_i = ((∂f/∂x_i) · σ_i)² / Σ c_j`) on the nominal Fenrir geometry shows which inputs dominate output uncertainty:
   - **`d(BB, BRP)` drives `89.0%` of `BRP_static.Y` variance** (`∂Y/∂d = +1.05`) and **`72.9%` of `HB_static.Y` variance**—whereas plumb-line setback `-BRP.X` contributes only `11.0%` of `BRP_static.Y` variance (`∂Y/∂x = -0.31`).
   - **`ΔX(BRP → HB)` (`61.0%`) and `-BRP.X` (`39.0%`)** together drive `100%` of `HB_static.X` variance.
   - **`y_bb` (`65.6%`), `y_rear` (`24.4%`), and `y_front` (`10.0%`)** drive `100%` of `LeveledBbDropMm` variance.
   - **`d(BB, HeadTubeTop)` drives `76.8%` of `LeveledStackMm` variance** (`∂Y/∂d = +1.20`).
   Re-seat the tape/level/plumb and record **3 independent readings** on those critical inputs so any single-read misread is caught immediately.

### 5. Expected error bands & overdetermined loop-closure checks

Running central-difference Jacobians and a 20,000-trial Monte Carlo simulation in `internal/assembly` with standard workshop uncertainties (`σ = 1.0 mm` for `100 mm`-offset radial chords and floor heights, `σ = 1.2 mm` for plumb-line horizontal offsets, `σ = 1.5 mm` for `700+ mm` level spans, `σ = 0.5 mm` for short vertical level gaps, `σ = 0.2 mm` for caliper diameters) yields the expected precision of each derived quantity and closure loop:

- **`LeveledBbDropMm` (`h`)** (via flat-floor `y_rear, y_front, y_bb`):
  - Nominal: `70.0 mm` | `1σ`: `±1.23 mm` | Linear `95%`: **`±2.42 mm`** | Monte Carlo `95%`: `[-2.42, +2.41] mm`
  - Dominant variance contributors: `65.6%` `y_bb` (`∂h/∂y_bb = -1.00`), `24.4%` `y_rear` (`+0.61`), `10.0%` `y_front` (`+0.39`)
- **`RearCenterMm`** (`sqrt(c² − h²)`):
  - Nominal: `434.4 mm` | `1σ`: `±1.03 mm` | Linear `95%`: **`±2.03 mm`** | Monte Carlo `95%`: `[-2.03, +2.02] mm`
  - Dominant variance contributors: `96.3%` chainstay chord `c = d(RearAxle, BB)` (`∂X/∂c = +1.01`), `3.7%` `LeveledBbDropMm`
- **`BRP_static.X`**:
  - Nominal: `-215.5 mm` | `1σ`: `±1.20 mm` | Linear `95%`: **`±2.35 mm`** | Monte Carlo `95%`: `[-2.37, +2.36] mm`
  - Dominant variance contributors: `100.0%` plumb setback `-BRP.X` (`∂X/∂x = -1.00`)
- **`BRP_static.Y`**:
  - Nominal: `+680.8 mm` | `1σ`: `±1.11 mm` | Linear `95%`: **`±2.17 mm`** | Monte Carlo `95%`: `[-2.14, +2.13] mm`
  - Dominant variance contributors: **`89.0%` radial `d(BB, BRP)`** (`∂Y/∂d = +1.05`), **`11.0%` plumb setback `-BRP.X`** (`∂Y/∂x = -0.31`)
- **`HB_static.X`**:
  - Nominal: `+438.9 mm` | `1σ`: `±1.92 mm` | Linear `95%`: **`±3.77 mm`** | Monte Carlo `95%`: `[-3.81, +3.77] mm`
  - Dominant variance contributors: **`61.0%` horizontal level span `ΔX(BRP → HB)`**, **`39.0%` plumb setback `-BRP.X`**
- **`HB_static.Y`**:
  - Nominal: `+679.2 mm` | `1σ`: `±1.22 mm` | Linear `95%`: **`±2.40 mm`** | Monte Carlo `95%`: `[-2.37, +2.37] mm`
  - Dominant variance contributors: **`72.9%` radial `d(BB, BRP)`**, **`16.7%` vertical gap `ΔY(BRP → HB_bridge)`**, `9.1%` `-BRP.X`
- **`PlaceHandlebar` (`BarClamp → HB`)** `(ΔX_bar, ΔY_bar)`:
  - Nominal: `(-46.8, +29.5) mm` | `1σ`: `(±0.80, ±0.52) mm` | Linear `95%`: **`(±1.57, ±1.02) mm`** | Monte Carlo `95%`: `(±1.57, ±1.01) mm`
  - Dominant variance contributors: `ΔX` `100%` short bar horizontal offset; `ΔY` `92.6%` vertical gap to bridge, `3.7%` `D_grip`, `3.7%` `t_bridge`
- **`PlaceFrameset` (`HeadTubeTop`)** `(Reach, Stack)`:
  - Nominal: `(+418.7, +648.6) mm` | `1σ`: `(±1.20, ±1.64) mm` | Linear `95%`: **`(±2.35, ±3.21) mm`** | Monte Carlo `95%`: `(±2.35, ±3.18) mm`
  - Dominant variance contributors: `Reach` `100%` plumb `HeadTubeTop.X`; `Stack` **`76.8%` radial `d(BB, HeadTubeTop)`** (`∂Y/∂d = +1.20`), `23.2%` `HeadTubeTop.X`
- **Off-bike bench `SeatTubeTop → BRP` in the seatpost's local coordinates from `SeatTubeTop` `(ΔX_axial, ΔY_perp)`**:
  - `1σ`: `(±0.80, ±0.80) mm` | Linear `95%`: **`(±1.57, ±1.57) mm`**
  - Isolates the rigid combined `Redshift + Drop Best + SQlab 610` offset directly on a flat bench without room-datum error
- **Closure 1: Wheelbase residual `r_wheelbase`**:
  - Nominal: `0.0 mm` | `1σ`: `±2.10 mm` | `2σ` gate: **`±4.12 mm`** | Monte Carlo `95%`: `[-4.10, +4.11] mm`
  - Check: `w − (sqrt(c² − h²) + sqrt(f² − h²))` — catches any base-triangle or `BB` drop reading blunder `> 4.1 mm`
- **Closure 2: Top-triangle chord residual `r_top_chord`**:
  - Nominal: `0.0 mm` | `1σ`: `±1.92 mm` | `2σ` gate: **`±3.76 mm`** | Monte Carlo `95%`: `[-3.74, +3.75] mm`
  - Check: `d(BRP, HeadTubeTop) − ||HeadTubeTop − BRP||` — checks `BRP_static` against `HeadTubeTop_static` before the cockpit
- **Closure 3: Full-bike 2D loop `r_loop = HB_front − HB_rear`**:
  - Nominal: `(0.0, 0.0) mm` | `1σ`: `(±2.60, ±2.25) mm` | `2σ` gate: **`(±5.10, ±4.41) mm`** | Monte Carlo `95%`: `(±5.10, ±4.42) mm`
  - Check: `(HeadTubeTop + ΔStem + ΔBar) − (BRP + ΔCockpit)` — closes the entire bike across both branches
- **Closure 4: Off-bike bench vs. on-bike `BRP` residual `r_brp_bench = BRP_via_bench − BRP_static`**:
  - Nominal: `(0.0, 0.0) mm` | `1σ`: `(±2.10, ±1.80) mm` | `2σ` gate: **`(±4.10, ±3.50) mm`**
  - Check: `(SeatTubeTop_static + bench(ΔX_axial, ΔY_perp) rotated by LeveledSTA) − BRP_static` — independently cross-checks Matt's on-bike `BRP_static` (`B3`) against the bench seatpost+saddle geometry and frame seat tube (`B2`)

---

## Part A: Direct Component, Frame, Post, and Stem Measurements (~10 minutes, calipers/ruler & digital level)

### A1. Off-bike seatpost + saddle bench protocol (mandatory — `SeatTubeTop → BRP` & SQlab 610 geometry)

*(Because the FAIR Drop Best completely replaces the Redshift ShockStop PRO's original saddle-rail clamp jaw, we skip measuring a stock Redshift rail cradle on the bike. Instead, we pull the assembled `Redshift + Drop Best + SQlab 610` unit out as one piece onto a flat bench to measure both the SQlab 610 saddle geometry and the exact rigid `SeatTubeTop → BRP` vector in the seatpost's local coordinates from `SeatTubeTop` for **Closure 4**.)*

#### Bench setup steps
1. **Mark `SeatTubeTop` and yaw before loosening the seat collar:** Wrap a strip of painter's tape tightly around the `Ø27.2 mm` Redshift shaft **flush with the top edge of the frame's seat tube collar** (`SeatTubeTop`), and draw a vertical alignment tick across the tape and seat collar so you can reinstall the post at the exact same height and straightness. Then slide the assembled post + Drop Best + saddle out of the frame.
2. **Locate and mark `BRP` (`W70`) on the saddle:** With calipers on the bench, find where the SQlab 610 external width is `70 mm`, put painter's tape across the saddle centerline there, and draw a sharp pencil crosshair `+` on the top surface (`BRP`). Leave this crosshair on the saddle for Part B.
3. **Set the `Ø27.2 mm` shaft parallel to a flat table/floor** *(the surface only needs to be **flat**, not level)*:
   - Place the assembly on its back (rear/tail of the saddle facing **down** toward the surface, nose pointing up) so that `SeatTubeDir` `local +X` runs horizontally along the shaft and `local +Y` (perpendicular rearward) points **straight down** toward the surface.
   - Prop/shim the bottom of the `Ø27.2 mm` shaft until both ends of the cylindrical shaft sit at the **exact same height `h_shaft_bottom`** above the surface. Shaft centerline height above the surface is then `h_shaft_center = h_shaft_bottom + 13.6 mm`.
4. **Reinstall after recording the bench readings below:** Slide the post back into the Fenrir seat tube flush with the `SeatTubeTop` tape ring and vertical yaw alignment tick before starting Part B.

#### Part A1 Recording Sheet
- **SQlab 610 `W70` station behind `TIP`**
  - Current model / catalog value: `120 mm` (derived in [`SQLAB_SADDLE.md`](../SQLAB_SADDLE.md))
  - **Measured:** `=`
  - How to measure: Measure longitudinal distance parallel to the straight saddle rails from the front tip (`TIP`) to the `W70` crosshair.
- **SQlab 610 `RailToBrpHeightMm`**
  - Current model / catalog value: `45.0 mm` (illustrative)
  - **Measured:** `=`
  - How to measure: At the `W70` station, measure the perpendicular distance (normal to the straight rail clamping section) from the **center of the `7 mm` saddle rail tube** up to the **top centerline surface of the saddle** (`BRP`, Q162). *(If measuring from the bottom or top of the `7 mm` rail tube is easier, record the raw number and note `from rail bottom` or `from rail top` — we will add/subtract `3.5 mm`.)*
- **Bench shaft-to-`BRP` perpendicular offset (`SeatTubeDir` `local +Y`, rearward)**
  - **Measured:** `h_shaft_bottom =` | `h_BRP =`
  - Formula: `h_shaft_center = h_shaft_bottom + 13.6 mm`; **`ΔY_perp = h_shaft_center − h_BRP`** *(positive when `BRP` sits closer to the table / rearward of the shaft centerline; negative if forward of it)*.
  - How to measure: With the shaft shimmed parallel to the flat surface (verify `h_shaft_bottom` matches at both ends of the cylindrical shaft), measure vertically from the flat surface up to the `BRP` (`W70`) crosshair.
- **Bench `SeatTubeTop`-to-`BRP` axial offset (`SeatTubeDir` `local +X`, along shaft)**
  - **Measured:** `ΔX_axial =` *(take `R1 / R2 / R3` if using a square)*
  - How to measure: Stand a square upright on the flat surface against the `SeatTubeTop` collar tape mark and against the `BRP` crosshair, mark both stations on the surface (or along a straightedge lying parallel to the shaft), and measure the distance parallel to the shaft between them.
- **Seatpost shaft graduation & exposed cylindrical shaft check**
  - **Measured:** `shaft mark at SeatTubeTop =` | `SeatTubeTop to max-insertion line =`
  - How to measure: Record the shaft graduation mark at the collar tape ring and measure straight along the `Ø27.2 mm` shaft from the `SeatTubeTop` tape mark up to the maximum-insertion line.
- *(Optional, off-bike only if the removed stock Redshift clamp jaw is handy)* **Stock Redshift clamp footprint (`RailClampLengthMm`)**
  - Current model / catalog value: `30.0 mm` (illustrative)
  - **Measured:** `=`
  - How to measure: Measure the fore-aft clamping length of the removed stock Redshift rail jaw with calipers.

### A2. Bare Fenrir frame, fork, stem (`100` vs. `110 mm` & angle), and handlebar checks

With the bike's axle line shimmed level (`0°` under the long level, see Part B Step 1), digital-level angle readings directly check leveled world angles (`LeveledSeatTubeAngleDeg`, `LeveledHeadTubeAngleDeg`, and stem world heading).

- **Frame size & Tuning Chip**
  - Catalog / nominal: Size `L`; middle chip (`440 mm` CS, `70 mm` BB drop, `68°` HTA, `75°` STA)
  - **Measured:** `=`
  - How to check: Confirm size (`L`) and check whether the rear dropout Tuning Chip is in the front (`430 mm`), middle (`440 mm`), or rear (`450 mm`) position.
- **Frame seat tube length (`SeatTubeLengthMm`, C-T)**
  - Catalog / nominal: `490 mm`
  - **Measured:** `=`
  - How to measure: Measure straight from `BB` spindle center to the top face of the seat tube (`SeatTubeTop`).
- **Frame head tube length (`HeadTubeLengthMm`)**
  - Catalog / nominal: `140 mm`
  - **Measured:** `=`
  - How to measure: Measure the bare head tube barrel length (excluding external lower cup and upper headset cover).
- **Leveled seat tube & head tube angles (digital level)**
  - Catalog / nominal: `~75°` STA, `~68°` HTA (at basis fork; pitched nose-up by the taller SID 110 fork)
  - **Measured:** `STA =` | `HTA =`
  - How to measure: With the axle line leveled (`0°`), hold the digital level against the seatpost shaft (for leveled STA) and against the head tube / steerer spacers (for leveled HTA).
- **Fork static A2C (`StaticA2cMm`) & offset**
  - Catalog / nominal: RockShox SID 35 mm `110 mm` (~`521 mm` A2C, `44 mm` offset)
  - **Measured:** `A2C =` | `Offset =`
  - How to measure: Bike unloaded (`0%` sag): measure straight from the **front thru-axle center** to the **crown race seat** (bottom of the EC44 lower cup). Check crown decal/serial for `44 mm` offset.
- **Fork seated sag (mm)**
  - Catalog / nominal: `—`
  - **Measured:** `=`
  - How to measure: Slide the stanchion O-ring down against the dust wiper, sit gently on the saddle in your normal riding posture on level ground, dismount without pressing on the bars, and measure the O-ring travel in **mm**.
- **Lower headset cup (`EC44`) stack**
  - Catalog / nominal: `12 mm` (`12.25 mm` Wolf Tooth EC44)
  - **Measured:** `=`
  - How to measure: Measure axial height of the external lower headset cup below the head tube.
- **Upper headset stack (`UpperStackMm`)**
  - Catalog / nominal: `11 mm` (illustrative)
  - **Measured:** `=`
  - How to measure: Measure axial height from the top of the bare head tube to the bottom of the lowest steerer spacer (top cup + dust cover).
- **Spacers below stem (`SpacersMm`)**
  - Catalog / nominal: `20 mm` (catalog default)
  - **Measured:** `=`
  - How to measure: Measure total axial height of spacers **below** the stem's steerer clamp.
- **Stem steerer-clamp height (`ClampHeightMm`)**
  - Catalog / nominal: `40 mm` (catalog default)
  - **Measured:** `=`
  - How to measure: Measure axial height of the stem's steerer clamp barrel along the steerer.
- **Stem length (`LengthMm`: `100 mm` vs. `110 mm`)**
  - Catalog / nominal: Uncertain (`100 mm` or `110 mm`)
  - **Measured:** `=`
  - How to measure: Measure center-to-center along the stem extension axis from the **centerline of the steerer tube** (center of the top-cap bolt) to the **center of the `31.8 mm` handlebar clamp** (the faceplate split line / bar center). Note any printed markings on the stem body/faceplate.
- **Stem angle (`AngleDeg`, unknown)**
  - Catalog / nominal: Unknown (`0°`, `±6°`, `±7°`, etc.)
  - **Measured:** `Stem world angle =` | `Steerer angle =`
  - How to measure:
    1. Look for any printed `±X°` marking inside or on the stem.
    2. With the axle line leveled (`0°`), place the digital level along the top and bottom of the stem body barrel (if symmetric/conical, average top and bottom surface angles to get the stem centerline angle above horizontal) and compare against the steerer angle (`90° − LeveledHTA + StemAngleDeg`). *(Part B8 also measures `HeadTubeTop → BarClamp` `(ΔX, ΔY)`, which algebraically solves both `LengthMm` and `AngleDeg` as an independent check!)*
- **Handlebar model, cut width, & clamp diameter**
  - Catalog / nominal: `persuader20_800` (`800 mm` stock, `31.8 mm` clamp)
  - **Measured:** `Model =` | `Cut width =`
  - How to check: Confirm handlebar model markings and measure total cut width end-to-end in **mm**.
- **Saddle static angle**
  - Catalog / nominal: `—`
  - **Measured:** `=`
  - How to measure: With the axle line leveled (`0%` sag), place the digital level along the flat top of the saddle nose (or along the straight saddle rail section; note which) and record the angle in degrees (note nose-up vs. nose-down).

---

## Part B: Center-Plane Static Landmark Protocol (~15–20 minutes, modular chain + closure checks)

### The 4-step physical workshop protocol

#### Step 1: Pick a flat floor strip, level the axle line (with `180°` level flip), and measure floor heights (`y_rear`, `y_front`, `y_bb`)
- Lay your **long level flat on the floor** along the `~1.2 m` line where the two tires will stand to verify the floor strip is **flat** (no localized dip or crown under the beam).
- Stand the bike upright on that flat strip (in a wheel chock/trainer or leaned lightly at the tire/pedal) with `0%` sag.
- Hold your **long level across the rear thru-axle center and front thru-axle center**, flip the level `180°` end-for-end to confirm symmetric bubble centering, and slip a thin shim under the lower tire until the axle-to-axle line reads level (`0°`). *(If the flat floor strip itself is already level, `y_rear` and `y_front` above the floor will be equal to within `1–2 mm`.)*
- Measure vertical height from the flat floor up to:
  1. **`y_rear`** (floor to `RearAxle` center),
  2. **`y_front`** (floor to `FrontAxle` center), and
  3. **`y_bb`** (floor to `BB` spindle center, **3 independent readings `R1, R2, R3`**).
  *(Tip: measure from the floor to the bottom edge and top edge of each thru-axle socket / `BB` bore and average `(y_bottom + y_top)/2` so you never eyeball the center of a hole.)*

#### Step 2: Build a physical `HB` centerline bridge (`Z = 0`) and mark landmarks with painter's tape
- **Handlebar `HB` centerline bridge (physical 3D → 2D orthographic projection):**
  1. Measure `80 mm` (`8 cm`, Q162) inward from **both** the left and right bar ends (`±zHand`), wrap painter's tape around each grip, and draw a sharp pencil ring at `80 mm` on both sides.
  2. Measure the outer diameter of the grip at the `80 mm` mark with calipers (`D_grip`, typically ~`30–34 mm`).
  3. **Tape a straight dowel, ruler, or yardstick across the *top* of both grips at the `80 mm` marks**, bridging left-to-right across the handlebar, and measure its thickness `t_bridge` with calipers.
  4. Mark a pencil crosshair `+` at the **midpoint** of that bridge (`half_width − 80 mm` from either mark). That midpoint sits in open air **right in the bicycle's 2D sagittal center plane (`Z = 0`)** behind the stem, at exact coordinates `(X_HB, Y_HB + D_grip/2 + t_bridge, Z = 0)`.
- **Saddle `BRP` crosshair (`Z = 0`):**
  - Confirm the `W70` painter's tape crosshair `+` from Part A1 is still in place on the saddle centerline and that the seatpost is reinstalled flush with the `SeatTubeTop` collar tape ring and vertical yaw alignment tick.
- **`BB` / axle `100 mm` zero-offset reference:**
  - Mark the exact center of the `BB` spindle and thru-axle sockets (or insert a snug `6 mm` hex key / centered dowel). For all tape-measure chords below, hold the **`100 mm` graduation** on the origin center mark, read the target mark, and subtract `100 mm` to eliminate hook-tab play.

#### Step 3: Measure the frame base chords, rear branch (`BB → SeatTubeTop` and `BB → BRP`), and `BB → HeadTubeTop`
Using the **`100 mm` tape zero-offset rule** for radial chords and a weighted plumb string (nut or socket on thread/dental floss hung from the landmark down past `BB` height) for horizontal offsets from `BB`:
1. **Base chords (`B1`, checks `PlaceFrameset` lower triangle + Closure 1):**
   - Measure `c = d(RearAxle, BB)` (~`440 mm`), `f = d(BB, FrontAxle)` (~`725 mm`), and `w = d(RearAxle, FrontAxle)` (~`1160 mm`).
   - Combined with `LeveledBbDropMm` (`h`) from Step 1, `RearCenterMm = sqrt(c² − h²)`, `FrontCenterX = sqrt(f² − h²)`, and **Closure 1** checks `r_wheelbase = w − (RearCenterMm + FrontCenterX)` (`|r_wheelbase| ≤ 4.1 mm`).
2. **Bare frame `SeatTubeTop` from `BB` (`B2`, checks `PlaceFrameset` seat tube):**
   - Measure radial `d(BB, SeatTubeTop)` (center of top edge of seat tube collar) and plumb-string horizontal setback `-SeatTubeTop.X`.
3. **Saddle `BRP` from `BB` (`B3`, ascertains Matt's true `BRP_static` directly on the SQlab 610 — take 3 readings `R1, R2, R3`):**
   - Measure radial **`d(BB, BRP)`** (`3` readings; `89%` of `BRP.Y` variance!) and plumb-string horizontal setback **`-BRP.X`** (`3` readings; `100%` of `BRP.X` variance).
4. **Bare frame `HeadTubeTop` from `BB` + top chord `d(BRP, HeadTubeTop)` (`B7` & `B9`, checks `PlaceFrameset` reach/stack + Closure 2):**
   - Measure radial **`d(BB, HeadTubeTop)`** (`3` readings, to the center of the top face of the bare head tube at `Z = 0`) and plumb-string horizontal reach **`+HeadTubeTop.X`** forward of `BB`.
   - Measure the direct top-of-bike chord **`d(BRP, HeadTubeTop)`** (`B9`) between the `BRP` crosshair and `HeadTubeTop` center (`|r_top_chord| ≤ 3.8 mm`).

#### Step 4: Measure the front branch (`BRP → HB`, `BarClamp → HB`, and `HeadTubeTop → BarClamp`) across the top of the bike using your long level (`180°` flipped)
With the bike's axles leveled, rest your **long level horizontally** (bubble centered, verified by flipping the level `180°` end-for-end) on the `HB` centerline bridge (`Z = 0`) extending backward over the stem, head tube, and saddle:
1. **`BRP → HB` (`B5`, ascertains Matt's true `HB_static` and `brpToHbOffset = HB − BRP` — take 3 readings on `ΔX`):**
   - Read the horizontal distance **`ΔX (BRP → HB)`** (`3` readings, ~`650–700 mm`) along the level from the `BRP` crosshair (or a plumb line/square held vertically at `BRP`) to the `HB` centerline bridge mark.
   - Measure the vertical gap **`ΔY_bridge (BRP → HB_bridge)`** from the `BRP` crosshair to the horizontal level plane (average normal and `180°`-flipped level sightings).
   - Then `HB_static.X = BRP_static.X + ΔX` and `HB_static.Y = BRP_static.Y + ΔY_bridge − center_offset` (where `center_offset = D_grip/2 + t_bridge`).
2. **`BarClamp → HB` (`B6`, tests `PlaceHandlebar` in complete isolation from the stem and frame):**
   - With the horizontal level resting on the `HB` centerline bridge extending over the stem's `31.8 mm` handlebar clamp (`BarClamp`), measure:
     - **Horizontal bar offset `ΔX_bar = HB.X − BarClamp.X`** (along the horizontal level from the center of the `31.8 mm` bar clamp back to the `HB` bridge mark; negative because backsweep places `HB` behind `BarClamp`).
     - **Vertical bar rise `ΔY_bar_bridge = HB_bridge.Y − BarClamp.Y`** (vertical distance from the underside of the horizontal level down to the **top of the `Ø31.8 mm` handlebar clamp tube** right beside the stem clamp, then add the `15.9 mm` bar-clamp radius to get the drop to `BarClamp` center; or measure directly to the stem faceplate split line if centered).
   - This `(ΔX_bar, ΔY_bar)` vector depends **only** on `PlaceHandlebar` (`Persuader` rise, backsweep, upsweep, `BendStartMm`, `RiseMode`, cut width, and roll) and has a **`±1.57 mm × ±1.02 mm` 95% error band**!
3. **`HeadTubeTop → BarClamp` (`B8`, tests `PlaceStem`, solves unknown stem length/angle, and closes the full-bike loop Closure 3):**
   - Measure the horizontal advance **`ΔX_stem = BarClamp.X − HeadTubeTop.X`** and vertical rise **`ΔY_stem = BarClamp.Y − HeadTubeTop.Y`** from the top center of the bare head tube (`HeadTubeTop`) to the center of the stem's `31.8 mm` handlebar clamp (`BarClamp`).
   - Adding `HeadTubeTop_static + (ΔX_stem, ΔY_stem) + (ΔX_bar, ΔY_bar)` gives `HB_via_HeadTubeTop`, which closes **Closure 3** against `HB_via_BRP = BRP_static + (ΔX, ΔY)` (`|r_loop_X| ≤ 5.1 mm`, `|r_loop_Y| ≤ 4.4 mm`).

---

### Part B Recording Sheet

*(For all tape-measure chords, record the net value after subtracting the `100 mm` burned-decimeter offset, or note `raw − 100`. For high-variance-share spans marked `R1 / R2 / R3`, record 3 independent re-seated readings.)*

- **B1 — Flat-floor heights & base chords (`RearAxle`, `BB`, `FrontAxle`)**
  - Isolates & verifies: Bare Fenrir lower triangle (`PlaceFrameset`). Expected `95%`: `BbDrop = ±2.4 mm`; `RearCenter = ±2.0 mm`; `|r_wb| ≤ 4.1 mm`.
  - Raw readings (mm):
    - Floor to `RearAxle` `y_rear =`
    - Floor to `FrontAxle` `y_front =`
    - Floor to `BB` `y_bb` (`R1 / R2 / R3`) `=`
    - Chord `c = d(RearAxle, BB) =`
    - Chord `f = d(BB, FrontAxle) =`
    - Chord `w = d(RearAxle, FrontAxle) =`
  - Notes / tube-radius offsets: Note if measured to bore bottom+top edges `(y_bot + y_top) / 2`
  - Derived (mm): `LeveledBbDropMm =` | `RearCenterMm =` | `FrontCenterX =` | **Closure 1 `r_wheelbase =`**
- **B2 — Bare frame `SeatTubeTop` from `BB` (`Z = 0`)**
  - Isolates & verifies: Bare Fenrir seat tube length & leveled STA (`PlaceFrameset`).
  - Raw readings (mm):
    - Radial `d(BB, SeatTubeTop) =`
    - Plumb setback `-SeatTubeTop.X =`
  - Formula: `Y = sqrt(d² − X²)`
  - Derived (mm): `SeatTubeTop_static =`
- **B3 — Saddle `BRP` from `BB` (`Z = 0`)**
  - Isolates & verifies: Ascertains Matt's true static saddle coordinate directly on the SQlab 610. Expected **`95% = (±2.35, ±2.17) mm`**.
  - Raw readings (mm):
    - Radial `d(BB, BRP)` (`R1 / R2 / R3`) `=`
    - Plumb setback `-BRP.X` (`R1 / R2 / R3`) `=`
  - Formula: `Y = sqrt(d² − X²)`
  - Derived (mm): **`BRP_static =`**
- **B4 — `HB` centerline bridge setup (`80 mm` from bar ends, `Z = 0`)**
  - Isolates & verifies: Projects 3D grips (`Z = ±zHand`) to center plane (`Z = 0`).
  - Raw readings (mm):
    - Grip OD `D_grip =`
    - Bridge thickness `t_bridge =`
  - Formula: `center_offset = D_grip/2 + t_bridge =`
- **B5 — Full cockpit offset `BRP → HB` (`Z = 0`, `180°`-flipped level)**
  - Isolates & verifies: Ascertains Matt's true `HB_static` and `brpToHbOffset`. Expected **`95% = (±3.77, ±2.40) mm`**.
  - Raw readings (mm):
    - Horizontal `ΔX (BRP → HB)` (`R1 / R2 / R3`) `=`
    - Vertical gap `ΔY_bridge` (`normal / flipped`) `=`
  - Formula: `ΔY = ΔY_bridge − center_offset`
  - Derived (mm): `brpToHbOffset_static =` | **`HB_static = BRP_static + (ΔX, ΔY) =`**
- **B6 — Handlebar in isolation: `BarClamp → HB` (`Z = 0`, horizontal level)**
  - Isolates & verifies: Isolates **`PlaceHandlebar`** (`BarClamp → HB`) from stem and frame. Expected **`95% = (±1.57, ±1.02) mm`**.
  - Raw readings (mm):
    - Horizontal `ΔX_bar (BarClamp → HB) =`
    - Vertical gap `ΔY_bar_bridge (BarClamp → HB_bridge) =`
  - Notes / tube-radius offsets: Note if vertical was measured to top of `Ø31.8 mm` bar (`+15.9 mm` to center) or faceplate split line
  - Derived (mm): `(HB − BarClamp)_static =`
- **B7 — Bare frame `HeadTubeTop` from `BB` (`Z = 0`)**
  - Isolates & verifies: Isolates bare Fenrir front end (**`PlaceFrameset`**). Expected **`95% = (±2.35, ±3.21) mm`**.
  - Raw readings (mm):
    - Radial `d(BB, HeadTubeTop)` (`R1 / R2 / R3`) `=`
    - Plumb reach `+HeadTubeTop.X` (`R1 / R2 / R3`) `=`
  - Formula & landmark: `Y = sqrt(d² − X²)`; center of top face of bare head tube
  - Derived (mm): `HeadTubeTop_static = (LeveledReachMm, LeveledStackMm) =`
- **B8 — Stem & headset: `HeadTubeTop → BarClamp` (`Z = 0`)**
  - Isolates & verifies: Isolates **`PlaceStem`** (cross-checking `100` vs. `110 mm` length & stem angle).
  - Raw readings (mm):
    - Horizontal `ΔX_stem (HeadTubeTop → BarClamp) =`
    - Vertical rise `ΔY_stem (HeadTubeTop → BarClamp) =`
  - Notes: Note if measured from bare head tube top or upper headset cover top
  - Derived (mm): `(BarClamp − HeadTubeTop)_static =`
- **B9 — Overdetermined loop-closure chord `d(BRP, HeadTubeTop)` (`Z = 0`)**
  - Isolates & verifies: Catches any blunder between rear (`BB → BRP`) and front (`BB → HeadTubeTop → BarClamp → HB`) chains (`|r| ≤ 2σ_r`).
  - Raw readings (mm):
    - Direct chord `d(BRP, HeadTubeTop) =`
  - Check: Compare against `||HeadTubeTop_static − BRP_static||`
  - Derived (mm): **Closure 2 `r_top_chord =`** | **Closure 3 `r_loop (X, Y) =`**

---

## Part C: Go uncertainty analysis, loop-closure verification, & derived fit targets (to be run after measurements)

Once Part A and Part B are recorded, we run our `internal/assembly` Go test (calling the real `PlaceFrameset`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, and `PlaceHandlebar` functions directly) and complete the audit in five steps:

1. **Repeatability & loop-closure consistency check (`|r| / σ_r ≤ 2.0`):**
   - Compute sample mean and repeatability standard uncertainty from each `R1 / R2 / R3` triplet.
   - Check the four overdetermined closure residuals (`r_wheelbase`, `r_top_chord`, 2D `r_loop = HB_via_HeadTubeTop − HB_via_BRP`, and 2D `r_brp_bench = BRP_via_bench − BRP_static`). If any `|r| > 2σ_r`, run **leave-one-out standardized residual inspection** to identify the single raw reading whose adjustment restores loop closure, so only that one span needs to be re-checked.
2. **Error propagation (central-difference Jacobian + 20,000-trial Monte Carlo) & variance-share ranking:**
   - Compute exact `1σ`, linear `95%` (`±1.96σ`), and empirical Monte Carlo `[2.5%, 97.5%]` confidence bands and variance shares (`Share_i`) for every derived coordinate from the actual measurement uncertainties.
3. **Component-by-component model audit (comparing measured `±95%` intervals against `bikegeo` predictions):**
   - **Bare Fenrir frame (`PlaceFrameset`):** compare measured vs. modeled `LeveledBbDropMm`, `RearCenterMm`, `WheelbaseMm`, `SeatTubeTop`, and `HeadTubeTop` (`LeveledReachMm`, `LeveledStackMm`).
   - **Stem (`PlaceStem`):** confirm the installed stem's length (`100` vs. `110 mm`) and angle, and compare measured vs. modeled `HeadTubeTop → BarClamp`.
   - **Handlebar (`PlaceHandlebar`):** compare measured vs. modeled `BarClamp → HB`.
   - *(Note: Stock Redshift `PlaceSeatpost` static setback `7 mm` and head stack `90 mm` remain on Redshift's published specifications because the Drop Best replaces the stock Redshift rail clamp jaw.)*
4. **New static baseline, derived dynamic riding targets, & idmatch double-check:**
   - Record Matt's directly measured unloaded `(BRP_static, HB_static)` and `brpToHbOffset_static` (with `95%` confidence bands) as the new primary physical baseline.
   - Derive the dynamic riding targets `(BRP_dynamic, HB_dynamic)` (with propagated `95%` error bands) by pitching the measured static baseline by the measured seated fork sag and applying Redshift's documented linkage sag (`Translate(-6, +5)` in the seatpost's local coordinates).
   - Compare the reported idmatch jig coordinates `(-219, 713.1)` and `(481, 726)` with the measured Fenrir coordinates `(BRP_static, HB_static)` and record the difference.
5. **Catalog & fit-target updates to close the audit:**
   - Replace the illustrative constants in `internal/catalog` with verified values: `SaddleSetup.RailToBrpHeightMm` (from Part A1), `UpperStackMm` (from Part A2), and (if measured off-bike from the removed stock Redshift clamp piece) `SeatpostSpec.RailClampLengthMm`.
   - Update [`RIDER_FIT.md`](../RIDER_FIT.md) and the catalog default fit target provenance with the newly established Fenrir static baseline and derived dynamic targets.
