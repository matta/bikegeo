# Fenrir physical check & component measurements

Status: Ready for measurement, 2026-11-23.

## Purpose and why no parts need to be swapped or removed

Measuring Matt's physical Otso Fenrir serves three concrete goals before specifying a custom frame:

1. **Replace guessed (`illustrative`) catalog dimensions with physical measurements:**
   - `SaddleSetup.RailToBrpHeightMm` on the SQlab 610 (currently guessed at `45.0 mm` in [`internal/catalog/saddle.go`](../../internal/catalog/saddle.go); every `1 mm` shift changes every candidate frame's predicted `post_insert` and Q166 `30 mm` insertion warning by ~`1 mm`), plus direct caliper verification of the `70 mm`-width station (`W70`, derived as `120 mm` behind `TIP` in [`SQLAB_SADDLE.md`](../SQLAB_SADDLE.md)).
   - `SeatpostSpec.RailClampLengthMm` on the Redshift ShockStop PRO (currently guessed at `30.0 mm` in [`internal/catalog/seatposts.go`](../../internal/catalog/seatposts.go); sets the `±(60 − clampLength)/2` rail-offset limit and Q167 `5 mm` rail warning).
2. **Capture the source Fenrir's suspension state to convert the static idmatch targets `(-219, 713.1)` and `(481, 726)` into dynamic targets ([`brp-transfer.md` Section 3.C](brp-transfer.md#c-static--dynamic-target-conversion-source-bike-static-measurements--dynamic-targets)):**
   - **Keep the FAIR Drop Best installed:** `BRP_static = (-219, 713.1)` was measured by idmatch on this bike *with the Drop Best installed* ([`RIDER_FIT.md`](../RIDER_FIT.md)). Because the Drop Best is a rigid clamp between the Redshift post head and the saddle rails, it rotates about `BB` with fork sag and translates by `Translate(-6, +5)` along the seatpost shaft (`SeatTubeAxis`) with Redshift linkage sag. Its geometry cancels out completely in `Static → Dynamic` target conversion.
   - **Keep the RockShox SID 35 mm (110 mm) fork installed:** Do not swap in the Lithic rigid fork. To [`PlaceFrameset`](../../internal/assembly/frameset.go), a fork is completely defined by its axle-to-crown length (`A2cMm`) and offset (`RakeMm`, `44 mm`). Measuring the installed SID 110's static A2C and its actual seated stanchion O-ring sag in millimeters captures the exact pitch and sag of the source bike that produced the idmatch fit.
3. **End-to-end physical check of forward placement (`T4`) and the idmatch coordinates:**
   - With the bike assembled as-is, measuring static `(X, Y)` from `BB` to `HeadTubeTop`, `HB` (`HandGrip`), the Redshift's `RailCradle` (visible below the Drop Best), and `BRP` verifies `PlaceFrameset`, `PlaceStem`, `PlaceHandlebar`, and `PlaceSeatpost` against the real bike while cross-checking that the physical bike's static `BRP` and `HB` agree with the idmatch report's `(-219, 713.1)` and `(481, 726)`.

---

## Part A (Tier 1): Component dimensions & source-bike state (~5 minutes, ruler/calipers)

### A1. Saddle & seatpost clamp dimensions (replaces `illustrative` catalog constants)

| Item | Current catalog value | Measured value (mm) | How to measure |
| --- | --- | --- | --- |
| **SQlab 610 `W70` station behind `TIP`** | `120 mm` (derived from `200 − 80 mm` in [`SQLAB_SADDLE.md`](../SQLAB_SADDLE.md)) | | Use calipers to find where the external saddle width is `70 mm`; measure longitudinal distance parallel to the rails from the front tip (`TIP`) to that `W70` station. |
| **SQlab 610 `RailToBrpHeightMm`** | `45.0 mm` (illustrative) | | At the `W70` station, measure the perpendicular distance (normal to the straight rail clamping section) from the **center of the `7 mm` rail tube** up to the **top centerline surface of the saddle** (`BRP`, Q162). *(If measuring from the bottom or top of the `7 mm` rail tube is easier, record that raw number and note `from rail bottom` or `from rail top` — we will add/subtract `3.5 mm`.)* |
| **Redshift ShockStop PRO `RailClampLengthMm`** | `30.0 mm` (illustrative) | | Measure the fore-aft clamping footprint length of the Redshift's rail clamp cradle (where it grips the rails / Drop Best lower rails). |

### A2. Installed Fenrir build & seated fork sag

Facts already confirmed in [`RIDER_FIT.md`](../RIDER_FIT.md) are pre-filled.

| Input | Pre-known / catalog reference | Installed on Matt's Fenrir | How to measure / check |
| --- | --- | --- | --- |
| **Frame size** | `L` (catalog default) | | Confirm size on frame (`L`). |
| **Tuning Chip position** | Middle (`440 mm` CS, `70 mm` BB drop, `68°` HTA, `75°` STA) | | Check whether the rear dropout Tuning Chip is in the front (`430 mm`), middle (`440 mm`), or rear (`450 mm`) position. |
| **Fork model** | RockShox SID 35 mm, `110 mm` travel (`44 mm` offset) | RockShox SID 35 mm, `110 mm` | Check fork serial/trailhead decal on crown back if offset (`44 mm`) is printed. |
| **Fork static A2C (`StaticA2cMm`)** | ~`521 mm` (RockShox published 29" SID 110) | | Bike unloaded (`0%` sag): measure straight from the **front thru-axle center** to the **crown race seat** (bottom edge of the EC44 lower headset cup). |
| **Fork seated sag (mm)** | — | | Slide the stanchion O-ring against the dust wiper, sit gently on the saddle in your normal riding posture on level ground, dismount without bouncing the front end, and measure the O-ring travel in **mm**. |
| **Lower headset cup (`EC44`) stack** | `12 mm` (`12.25 mm` Wolf Tooth EC44) | | Measure axial height of the external lower headset cup below the head tube. |
| **Upper headset stack (`UpperStackMm`)** | `11 mm` (illustrative) | | Measure axial height from the top of the head tube to the bottom of the lowest steerer spacer (top cup + dust cover). |
| **Spacers below stem (`SpacersMm`)** | `20 mm` (catalog default) | | Measure total axial height of spacers **below** the stem clamp. |
| **Stem length, angle, clamp height** | `110 mm` length ([`RIDER_FIT.md`](../RIDER_FIT.md)); default `-6°`, `40 mm` clamp | | Record stem length (`110 mm`), printed angle (`±deg` and whether flipped up or down), and steerer clamp height. |
| **Handlebar model & cut width** | `persuader20_800` (`800 mm` stock) | | Record handlebar model and total cut width (end to end, in mm). |
| **Seatpost** | Redshift ShockStop PRO Endurance (`350 mm`) | Redshift ShockStop PRO Endurance | Confirmed in [`RIDER_FIT.md`](../RIDER_FIT.md). |
| **Seatpost exposed length (`ExposedMm`)** | `175 mm` (`InsertionMm = 350 − ExposedMm`) | | Bike unloaded: measure parallel to the seatpost shaft from the **top of the seat tube collar** (`SeatTubeTop`) to the **center of the Redshift's rail cradle** (`RailCradle`). |
| **Setback adapter** | FAIR Drop Best (rearward orientation) | FAIR Drop Best (rearward) | Confirmed in [`RIDER_FIT.md`](../RIDER_FIT.md); leave installed. |
| **Saddle** | SQlab 610 ERGOLUX active 2.1, `14 cm` | SQlab 610 ERGOLUX active 2.1, `14 cm` | Confirmed in [`RIDER_FIT.md`](../RIDER_FIT.md). |
| **Saddle static angle** | — | | With the bike's axle line leveled (`0%` sag), place a digital level along the flat top of the saddle nose (or along the straight saddle rail section; note which) and record the angle in degrees (note nose-up vs. nose-down). |

---

## Part B (Tier 2): Center-Plane Static Landmark Protocol (~15 minutes, no floor/wall reliance)

### Why floor/wall and direct 3D grip triangulation fail

1. **Old-house floor and wall slopes:** Measuring heights from an unlevel floor or horizontal distances from an out-of-plumb wall injects floor pitch and wall tilt directly into every reading.
2. **3D lateral parallax at the handlebar (`Z ≈ ±320 mm`):** [`PlaceHandlebar`](../../internal/assembly/handlebar.go) computes the lateral grip station `zHand = width/2 − GripInsetMm` (`HandLateralMm`, ~`320 mm` outboard of the frame's `Z = 0` center plane), projects the bar's backsweep and upsweep along `±Z`, and orthographically projects `HB` onto the 2D center plane `(X_HB, Y_HB, Z = 0)`. Stretching a tape measure diagonally in 3D from `BB` or `FrontAxle` (`Z ≈ 0`) out to a grip at `Z = ±320 mm` measures the 3D hypotenuse `sqrt(X² + Y² + 320²)`, adding **~`57 mm` of systematic 3D diagonal error**.
3. **Tube-center eyeballing and 1.2-meter two-handed stretches:** `HB` lies *inside* a round grip tube (`Ø30–34 mm`), and `RearAxle → HB` is over `1.1 m` away with the handlebar itself blocking the tape path from the front axle.

### The 4-step physical workshop protocol

#### Step 1: Level the axle line (eliminates floor slope)
- Stand the bike upright (in a wheel chock/trainer or leaned lightly at the tire/pedal) with `0%` sag.
- Hold your **long level directly across the rear thru-axle center and front thru-axle center**, and slip a thin shim (book, magazine, or scrap wood) under the lower tire until the axle-to-axle bubble reads level (`0°`). Now the floor slope underneath the tires is completely out of the loop.

#### Step 2: Build a physical `HB` centerline bridge (`Z = 0`) and mark `BRP`
- **Handlebar `HB` centerline bridge (physical 3D → 2D orthographic projection):**
  1. Measure `80 mm` (`8 cm`, Q162) inward from **both** the left and right bar ends (`±zHand`), wrap painter's tape around each grip, and draw a sharp pencil ring at `80 mm` on both sides.
  2. Measure the outer diameter of the grip at the `80 mm` mark with calipers (`D_grip`, typically ~`30–34 mm`).
  3. **Tape a straight dowel, ruler, or yardstick (or stretch a taut string) across the *top* of both grips at the `80 mm` marks**, bridging left-to-right across the handlebar.
  4. Mark a pencil crosshair `+` at the **midpoint** of that bridge (`half_width − 80 mm` from either mark). That midpoint sits in open air **right in the bicycle's 2D sagittal center plane (`Z = 0`)** behind the stem, at exact coordinates `(X_HB, Y_HB + D_grip/2, Z = 0)` (plus the bridge's thickness if measuring to the top face of a ruler/dowel).
- **Saddle `BRP` crosshair:**
  - Place a strip of painter's tape along the saddle centerline at `W70` (`120 mm` behind `TIP`) and draw a sharp pencil crosshair `+` on the top surface (`Z = 0`).
- **Hands-free `BB` / axle tape anchor:**
  - Insert a snug Allen key (e.g. `6 mm` hex wrench) into the thru-axle hex socket or center a bolt/dowel in the hollow `BB` spindle so you can **hook the tape measure's end slot over the shaft hands-free** (note the shaft radius, e.g. `3 mm`, if hooking the outside of the shaft).

#### Step 3: Measure the base triangle, `BRP`, and `RailCradle` from `BB`
This reproduces the exact method by which idmatch / bike fitters derive `BRP = (-219, 713.1)` from radial distance `746 mm` and horizontal setback `219 mm` (`Y = sqrt(746² − 219²) = 713.13 mm`, [`RIDER_FIT.md`](../RIDER_FIT.md)):
- **Base triangle (center to center):**
  - `d(RearAxle, BB)` (chainstay, ~`440 mm`), `d(RearAxle, FrontAxle)` (wheelbase, ~`1160 mm`), and `d(BB, FrontAxle)` (front-center, ~`725 mm`).
- **Saddle `BRP` relative to `BB`:**
  1. **Radial distance `d(BB, BRP)` (~`746 mm`):** Hook the tape at `BB` center and measure straight up to the `BRP` crosshair on the saddle.
  2. **Horizontal setback `-BRP.X` (~`219 mm`):** Hang a weighted string (a heavy nut, socket, or wrench on thread or dental floss) from the `BRP` crosshair down past the `BB`. At `BB` height, measure the short horizontal distance from the `BB` center back to the hanging string with a small ruler. *(Because `dY / dX = -219 / 713.1 = -0.31`, a `1 mm` horizontal error shifts `BRP.Y = sqrt(d² − X²)` by only `0.3 mm`.)*
- **Redshift `RailCradle` relative to `BB` (checks `PlaceFrameset` + `PlaceSeatpost` below the Drop Best):**
  1. **Radial distance `d(BB, RailCradle)`:** Measure from `BB` center to the center of the Redshift's own rail clamp cradle (below the Drop Best).
  2. **Horizontal setback `-RailCradle.X`:** Hang the weighted string from the center of the Redshift's rail cradle down past `BB` and measure the short horizontal distance from `BB` center back to the string.

#### Step 4: Measure `HB` and `HeadTubeTop` across the top of the bike using your long level
Because `BRP` (`Y ≈ 713 mm`) and the `HB` centerline bridge (`Y ≈ 726 mm + D_grip/2`) are both at the top of the bike, in the exact center plane (`Z = 0`), and within ~`13–30 mm` of the same height, you never need to measure `HB` from the axles or `BB`:
1. **`BRP → HB` (`brpToHbOffset = HB − BRP`, Q172):**
   - Rest one end of your **long level** on the `HB` centerline bridge mark (`Z = 0`) and extend the level horizontally backward over the `BRP` crosshair on the saddle.
   - Hold the long level **horizontal** (bubble centered):
     - **Horizontal cockpit reach (`ΔX = HB.X − BRP.X`, ~`700 mm`):** Read the horizontal distance along the level from the `BRP` crosshair to the `HB` centerline bridge mark.
     - **Vertical gap (`ΔY_bridge = HB_bridge.Y − BRP.Y`, ~`13–30 mm`):** Measure the small vertical gap between the saddle's `BRP` crosshair and the underside of the horizontal level right above it using **calipers or a short mm ruler**.
   - Then `HB.X = BRP.X + ΔX` and `HB.Y = BRP.Y + ΔY_bridge − D_grip/2` (minus bridge thickness if the level rested on top of a ruler of thickness `t_bridge`).
   - *(Optional cross-check: measure the direct straight-line chord distance `d(BRP, HB_bridge)` from the `BRP` crosshair to the `HB` bridge mark; it must equal `sqrt(ΔX² + ΔY_bridge²)` within `~1 mm`.)*
2. **`HeadTubeTop → HB` (checks `PlaceFrameset` reach/stack and `PlaceStem` + `PlaceHandlebar` separately):**
   - With the horizontal level resting on the `HB` centerline bridge and extending backward over the stem/headset, measure the short (`~50–120 mm`) horizontal distance `ΔX_ht = HB.X − HeadTubeTop.X` along the level and the vertical drop `ΔY_ht = HB_bridge.Y − HeadTubeTop.Y` down to the center of the top edge of the head tube (or top of the upper headset cover — note which) with calipers or a short ruler.

---

### Part B Recording Table

| # | Measurement | Raw reading(s) (mm) | Notes / offsets (e.g. `D_grip`, bridge thickness, hex-key radius) | Derived BB-relative `(X, Y)` (mm) |
| --- | --- | --- | --- | --- |
| **B1** | **Base triangle (center to center)** | `d(RearAxle, BB)` = <br>`d(RearAxle, FrontAxle)` = <br>`d(BB, FrontAxle)` = | | `RearCenterMm` = <br>`LeveledBbDropMm` = <br>`WheelbaseMm` = |
| **B2** | **Saddle `BRP` from `BB` (`Z = 0`)** | Radial `d(BB, BRP)` = <br>Plumb setback `-BRP.X` = | `BRP.Y = sqrt(d² − X²)` | `BRP_static` = |
| **B3** | **Redshift `RailCradle` from `BB` (`Z = 0`)** | Radial `d(BB, RailCradle)` = <br>Plumb setback `-RailCradle.X` = | `RailCradle.Y = sqrt(d² − X²)` | `RailCradle_static` = |
| **B4** | **`HB` centerline bridge (`80 mm` from bar ends, `Z = 0`)** | Grip OD `D_grip` = <br>Bridge thickness `t_bridge` = | `center_offset = D_grip/2 + t_bridge` | — |
| **B5** | **Cockpit offset `BRP → HB` (`Z = 0`, horizontal level)** | Horizontal `ΔX (BRP → HB)` = <br>Vertical gap `ΔY_bridge (BRP → HB_bridge)` = <br>*(Optional chord `d(BRP, HB_bridge)` = )* | `ΔY = ΔY_bridge − center_offset` | `brpToHbOffset_static` = <br>`HB_static = BRP_static + (ΔX, ΔY)` = |
| **B6** | **`HeadTubeTop → HB` (`Z = 0`, horizontal level)** *(note whether measured to bare head tube top or upper cover top)* | Horizontal `ΔX_ht (HeadTubeTop → HB)` = <br>Vertical drop `ΔY_ht (HeadTubeTop → HB_bridge)` = | `HeadTubeTop = HB_static − (ΔX_ht, ΔY_ht − center_offset)` | `HeadTubeTop_static` = |

---

## Part C: Model comparison (to be filled in once Part A / Part B values are recorded)

Once you fill in any or all of Part A and Part B above, we will run the exact installed inputs through a Go test calling `PlaceFrameset`, `PlaceSeatpost`, `PlaceStem`, and `PlaceHandlebar` and populate the model comparison and your derived dynamic targets `(BRP_dynamic, HB_dynamic)` here.
