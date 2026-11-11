# Ideal Profiles & Cockpit Fit Metrics

This directory contains ideal bike fit profiles used by `bikegeo compare` and the scoring engine to evaluate and rank frame configurations against personalized anthropometric and biomechanical targets.

---

## idmatch Biomechanical Reference Point (BRP) & Saddle Position

### Definition & Reference Points

The **BRP (Biomechanical Reference Point)** is the standardized anatomical point on a saddle where its width reaches exactly **70 mm**. Defining setup measurements relative to the BRP eliminates discrepancies caused by varying saddle lengths, nose shapes, and contour tapers:

> *The BRP (Biomechanical Reference Point) is the point, common to all saddles currently on the market, where the saddle reaches a width of 70mm. Defining the BRP allows you to have a common denominator from which you can calculate all the setup measurements for your saddle.* (idmatch Bikelab)

The **HB reference point** is the intersection between the handlebar and the stem clamp center.

### Four Standard idmatch Cockpit Measurements

From an idmatch Bikelab report (e.g. `206563 - Matt Armstrong - Report idmatch Bikelab.pdf`):
1. **Saddle height `(a)`**: `746 mm` — Radial distance from the Bottom Bracket spindle center `(0, 0)` to the BRP.
2. **Saddle setback `(b)`**: `219 mm` — Horizontal distance behind the Bottom Bracket to the BRP.
3. **Saddle-Handlebar distance `(d)`**: `700 mm` — Direct 2D Euclidean distance from BRP to HB: `hypot(HbX - BrpX, HbY - BrpY)`.
4. **Saddle-Handlebar Drop `(c)`**: `+13 mm` — Vertical height difference from BRP to HB: `HbY - BrpY` (positive = HB is higher than saddle).
5. **Saddle angle from BB `(g)`**: `72.9°` — Polar angle from the Bottom Bracket to the BRP: `atan2(713, 219) = 72.93°`. (Note: this is the angle to the BRP itself, not a frame's Seat Tube Angle).

---

## Hardware Modeling & Saddle Rails

On a real bicycle, the BRP sits offset from the seat tube centerline due to:
- **Seatpost Setback**: `model.DefaultSeatpostSetbackMm = 11.0 mm` (modeled on a Redshift ShockStop suspension seatpost: 7 mm static + 4 mm dynamic sag under rider weight).
- **Saddle Geometry**: Defined in the Canonical Saddle Coordinate System (`X_RC = 0.0 mm` at the rail center; inspect with `bikegeo saddles`).
  - For the reference **SQlab 610 Ergolux Active 2.1**: `RailCenterToBrpMm = +23.0 mm`, `RailCenterToSitBonesMm = -50.0 mm`.
  - For the **Berthoud Aspin Open**: `RailCenterToBrpMm = +43.5 mm`, `RailCenterToSitBonesMm = -44.5 mm`.
  - Pelvic fit equivalence between them differs by only `5.5 mm` on the rails (`0.44°` of Seat Tube Angle).
- **Primary Pelvic Invariant**: Sit Bone Setback is `292.0 mm` behind the BB (derived from idmatch BRP 219 mm + SQlab span 73 mm).
- **Usable Rail Range**: Determined by `(L_rail - L_clamp) / 2` (bounded by `[-half, +half]` along the straight clamping rails).
- **Ideal Seat Tube Angle**: Use `bikegeo saddles` to compute the required custom frame Seat Tube Angle to clamp any catalog saddle dead-center (`0.0 mm`) on your seatpost.

### Constant-Radius Rail Adjustment

Every candidate position on the saddle rails maintains exact radial distance `L = SaddleHeightMm` (`746 mm`) from the Bottom Bracket:
- `clamp_perp = SeatpostSetback + RailOffsetMm`
- `brp_perp = clamp_perp - RailCenterToBrpMm` (forward offset toward nose reduces rearward perpendicular offset from seat tube axis)
- `sit_bone_perp = clamp_perp - RailCenterToSitBonesMm`
- `d_axial = sqrt(L² - brp_perp²)`
- `BrpX = -d_axial * cos(STA) - brp_perp * sin(STA)` (`BrpSetback = -BrpX`)
- `BrpY =  d_axial * sin(STA) - brp_perp * cos(STA)`
- `SitBoneX = -d_axial * cos(STA) - sit_bone_perp * sin(STA)` (`SitBoneSetback = -SitBoneX`)

When a target setback is specified (e.g. `219 mm` in `idmatch`), `SelectBestBRP` slides the saddle on its rails toward that target (bounded by the saddle's physical clamp window), scoring the frame on how close the required rail offset is to dead center (`0 mm`), while `Saddle-HB` distance `(d)` and drop `(c)` are measured directly from the achieved BRP to HB.

---

## Front Cockpit Optimization (`HBPoint`) & Area of Reasonable Fit

The steerer tube extends upward and rearward at polar angle `180° - HTA`. With stem clamp thickness of 40 mm and steerer spacers `s` (`0..max_spacer_mm` in 5 mm steps), the center of the stem handlebar clamp sits at:
- `SteererX = Reach - (s + 20.0) * cos(HTA)`
- `SteererY = Stack + (s + 20.0) * sin(HTA)`
- `ClampX = SteererX + LengthMm * cos((90° - HTA) + AngleDeg)`
- `ClampY = SteererY + LengthMm * sin((90° - HTA) + AngleDeg)`

### Handlebar Component Geometry & Contact Point Offset
A handlebar component translates the stem clamp center (`P_HB`) to the hand grip center (`P_Grip`) and vice versa. Rather than relying solely on naive trigonometry (`Width/2 * tan(sweep)`), `bikegeo` implements a complete 3D piecewise linear kinematic transformation model (`pkg/geometry/handlebar.go`, `pkg/catalog/handlebars.go`):

1. **Lateral Hand Coordinate**: `z_hand = W / 2 - d_grip` (with standard `d_grip = 65 mm` inboard of bar tip, or `70 mm` for wing grips).
2. **Swept Lever Arm**: `L_z = max(0, z_hand - z_b)`, where `z_b` is the effective lateral bend start.
   - For **SQlab bars** (3OX, 311 2.0, 311 FL-X): `z_b = 55 + 0.9 * R_spec` because higher rise consumes more lateral run in the 45°–50° riser transition.
   - For **Tumbleweed Persuader**: `z_b = 135 mm` (wide 200 mm center clamping zone + riser bend).
3. **Rise Measurement Traps**:
   - **Standard Knuckle (SQlab / DIN)**: Rise `R_spec` is measured to the knuckle; outboard upsweep adds height on top (`y_k = R_spec`).
   - **Bar-End Center (Tumbleweed Persuader)**: Rise is measured from clamp center to bar-end tip; upsweep is baked in, so knuckle base rise is `y_k = R_spec - (W_0/2 - z_b) * tan(alpha)`.
4. **Neutral & Rolled Vectors**:
   - `Δx_0 = -L_z * tan(beta)`
   - `Δy_0 = y_k + L_z * tan(alpha)`
   - `ΔX_bar = Δx_0 * cos(theta_roll) + Δy_0 * sin(theta_roll)`
   - `ΔY_bar = -Δx_0 * sin(theta_roll) + Δy_0 * cos(theta_roll)`
   - `ΔZ_bar = z_hand`

Legacy 2D baseline calculation:
- `swept_span = max(0, (800 / 2) - 190.0 - 70.0) = 140.0 mm`
- `Offset_X = -swept_span * sin(20°) = -47.88 mm` (rearward pullback)
- `Offset_Y = +50.0 mm` (upward riser)
- `HbX = ClampX + Offset_X`
- `HbY = ClampY + Offset_Y`

To place the rider's hands at the target `(HbX, HbY)` from an idmatch report, the physical stem clamp center must sit downward (`-50.0 mm`) and forward (`+47.9 mm`), correctly driving frame selection toward models with appropriate reach and stack.

### Handlebar Clamp Fit (`handlebar_clamp_fit`)
When scoring cockpits against biomechanical profiles, handlebar positions are assessed against an **Area of Reasonable Fit** oval:
- **Ideal Center**: `(BrpX + ΔX_0, BrpY + ΔY_0)` where `ΔX_0 = √(d_0² - c_0²)` and `ΔY_0 = c_0` from target `saddle_hb` distance `d_0` and drop `c_0`.
- **Dimensions**: Semi-minor axis `a = 10.0 mm` (±1 cm horizontal) and semi-major axis `b = 20.0 mm` (±2 cm vertical).
- **Rotation**: 30° clockwise tilt, matching frame sizing geometry where longer reach requires taller stack.
- **Normalized Radius `R`**: `R = √((p_minor / 10)² + (p_major / 20)²)`
  - `R ≤ 1.0`: Inside oval (`BaseScore = 100 - 25 * R²`).
  - `R > 1.0`: Steep roll-off (`BaseScore = 75 - 1200 * (R - 1.0)²`, reaching 0 at `R = 1.25` and floor of -200).
- **Versatility Bonus**: Diminishing returns scheme (`+0` for 1 valid setup, `+5` for 2–3, `+10` for 4+ setups inside oval).
- **Missing-Fit Penalty**: Flat `-25 point` penalty if 0 setups land inside the oval.
- **Stem Extremity Penalty**: When evaluating setups against the frame's expected stem range `[L_min, L_max]` (midpoint `L_mid`, half-span `H`):
  - Normalized deviation: `u = |L - L_mid| / H`.
  - For `u ≤ 1.0`: `StemPenalty = 15.0 * u²` (steeper beyond bounds).
  - Candidates inside the oval are selected to maximize `CompositeScore = BaseScore - StemPenalty`, preferring sweet-spot stems (e.g., 80mm on a 60–100mm frame) over boundary stems (60mm or 100mm) that leave little room for fit adjustments.
  - Boundary stems incur up to a 15-point deduction (`FinalScore = min(100, BaseScore + Bonus) - StemPenalty`).

When targets are present in legacy profiles without `handlebar_clamp_fit`, `SelectBestHBWithDrop` selects the optimal stem and spacer combination using strict hierarchy:
1. **Primary (`saddle_hb`)**: Minimum rounded mm distance to the distance target range `(d)`.
2. **Secondary (`saddle_hb_drop`)**: Minimum rounded mm distance to the drop target range `(c)`.
3. **Tie-breaker 1**: Closest stem length to `canonical_stem`.
4. **Tie-breaker 2**: Closest stem angle to `canonical_stem`.
5. **Tie-breaker 3**: Smallest spacer stack (`spacerMm`).
6. **Tie-breaker 4**: Closest unrounded distance to target midpoint.

---

## Ideal Profile

### `idmatch` Profile
Built directly around the idmatch Bikelab report:
- `saddle_height_mm`: `746`
- `brp_setback`: Target `219 mm` (`ideal: 219, acceptable: 214–224, edge_score: 75, falloff: 20`, weight: 10)
- `handlebar_clamp_fit`: Target `≥100 pts` (`acceptable: ≥75, edge_score: 75, falloff: 75, floor: -200`, weight: 15)
- `saddle_hb`: Display-only with target `700 mm` (`ideal: 695–705, acceptable: 685–715`)
- `saddle_hb_drop`: Display-only with target `+3 mm` relative to sit-bone position (`ideal: -2–8, acceptable: -10–16`; corresponds to `+13 mm` above the lowered SQlab 70 mm BRP plane)
- `seatpost_extension`: Target `250–310 mm` (`acceptable: 210–340, edge_score: 75, falloff: 25`, weight: 4) — Proxy for standover clearance and seatpost/dropper fitment: `SaddleHeightMm - SeatTubeLength`.
- `frontwt`: Target `35.8–38.2%` (`acceptable: 34.8–39.2%, edge_score: 75, falloff: 2.0`, weight: 4) — Fore/aft rider centering between axles based on modeled target pedaling CoG relative to BB.
- `bbdrop`: Target `70 mm` (`acceptable: 60–80, edge_score: 75, falloff: 10`, weight: 3)
- `trail`: Target `98–106 mm` (`acceptable: 92–109 mm, edge_score: 75, falloff: 10`, weight: 2) — Steering feel ground trail based on active HTA, fork rake, and 29x2.4" baseline tires (372mm wheel radius).
- `hta`: Display-only (`ActiveHta`)
- `seat_to_rc`: Display-only Saddle-Rear Axle (`Saddle-Rear Axle`: horizontal distance from saddle to rear axle, deprecated in favor of `frontwt`)
- `sparevert`: Display-only Saddle-HT Drop (`Saddle-HT Drop`: vertical drop from saddle to head tube top `SaddleHeight * sin(STA) - Stack`)
