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

## Front Cockpit Optimization (`HBPoint`)

The steerer tube extends upward and rearward at polar angle `180° - HTA`. With stem clamp thickness of 40 mm and steerer spacers `s` (`0..max_spacer_mm` in 5 mm steps):
- `SteererX = Reach - (s + 20.0) * cos(HTA)`
- `SteererY = Stack + (s + 20.0) * sin(HTA)`
- `HbX = SteererX + LengthMm * cos((90° - HTA) + AngleDeg)`
- `HbY = SteererY + LengthMm * sin((90° - HTA) + AngleDeg)`

When targets are present, `SelectBestHBWithDrop` selects the optimal stem and spacer combination using strict hierarchy:
1. **Primary (`saddle_hb`)**: Minimum rounded mm distance to the distance target range `(d)`.
2. **Secondary (`saddle_hb_drop`)**: Minimum rounded mm distance to the drop target range `(c)`.
3. **Tie-breaker 1**: Closest stem length to `canonical_stem`.
4. **Tie-breaker 2**: Closest stem angle to `canonical_stem`.
5. **Tie-breaker 3**: Smallest spacer stack (`spacerMm`).
6. **Tie-breaker 4**: Closest unrounded distance to target midpoint.

---

## Ideal Profiles & Deprecation Strategy

### `data/ideals/idmatch.json`
Built directly around the idmatch Bikelab report:
- `saddle_height_mm`: `746`
- `brp_setback`: Target `219 mm` (`ideal: 219, acceptable: 214–224, edge_score: 75, falloff: 20`)
- `saddle_hb`: Target `700 mm` (`ideal: 695–705, acceptable: 685–715, edge_score: 75, falloff: 15`)
- `saddle_hb_drop`: Target `13 mm` (`ideal: 8–18, acceptable: 0–26, edge_score: 75, falloff: 10`)

### `data/ideals/default.json` (Legacy Profile)
Retained for 100% backwards compatibility with previous test suites and baselines. Legacy seat-tube metrics (`setback`, `aett`, `sparevert`, `spareaxial`, `seat_to_rc`) measure to the bare seat tube centerline and are deprecated in favor of BRP-referenced metrics.
