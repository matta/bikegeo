# Ideal Profiles & Cockpit Fit Metrics

This directory contains ideal bike fit profiles used by `bikegeo compare` and the scoring engine to evaluate and rank frame configurations against personalized anthropometric and biomechanical targets.

---

## idmatch Biomechanical Reference Point (BRP) & Saddle Position

For Matt's confirmed saddle, suspension seatpost, riding history, and the **unsagged source-bike measurement and same-saddle transfer to the test jig**, see [Rider Fit: Facts and Measurement Provenance](../../docs/RIDER_FIT.md). That record establishes how the reported position was obtained; it does not establish an optimal setback or supply a measured seatpost sag correction.

### Definition & Reference Points

The **BRP (Biomechanical Reference Point)** is the standardized anatomical point on a saddle where its width reaches exactly **70 mm**. Defining setup measurements relative to the BRP eliminates discrepancies caused by varying saddle lengths, nose shapes, and contour tapers:

> *The BRP (Biomechanical Reference Point) is the point, common to all saddles currently on the market, where the saddle reaches a width of 70mm. Defining the BRP allows you to have a common denominator from which you can calculate all the setup measurements for your saddle.* (idmatch Bikelab)

Canonical idmatch documentation defines the handlebar reference point (HB) as **`StemClamp`** (the intersection between the handlebar and the stem clamp center). By contrast, a fit session recorded with a tracking marker at the rider's hands on a straight test bar measures **`HandGrip`**.

To prevent the ~70 mm clamp-vs-grip ambiguity hidden by bare `HB` / `HX` / `HY` labels, cockpit values and targets distinguish the two physical reference points:
- **`StemClamp`** (`StemClampPoint`, `StemClampX`, `StemClampY`): Intersection of the handlebar and the stem clamp center (canonical idmatch `HB` / frame-CAD stem center).
- **`HandGrip`** (`GripPoint`, `GripX`, `GripY`): Rider's physical hand contact center after backsweep, upsweep, grip inset, and bar roll (`StemClamp + BarOffset(Bar, GripInset, Roll)`).

Every cockpit fit target `TargetPoint` (or distance `d` and drop `c` from `BRP`) must declare an explicit **`TargetRef`** governing how it translates to the required stem clamp center `C*`:
- **`TargetHandGrip`**: Target is the physical hand grip position (`HandGrip`); required stem clamp is `C* = TargetPoint - BarOffset(Bar, Roll)`.
- **`TargetStemClamp`**: Target is the stem clamp center directly (`StemClamp`, e.g., Seven Cycles' stem-center fit-sheet coordinate `(482, 676)` after subtracting `50 mm` bar rise from `726`); required stem clamp is `C* = TargetPoint`.
- **`TargetReferenceBarClamp`**: Target is the stem clamp of the fit bike's handlebar (`ReferenceBar`), reproducing hand position via `C* = TargetPoint + BarOffset(ReferenceBar) - BarOffset(Bar, Roll)`.

### Four Standard idmatch Cockpit Measurements & Provenance

From an idmatch Bikelab report (e.g. `206563 - Matt Armstrong - Report idmatch Bikelab.pdf`):
1. **Saddle height `(a)`**: `746 mm` — Radial distance from the Bottom Bracket spindle center `(0, 0)` to the BRP.
2. **Saddle setback `(b)`**: `219 mm` — Horizontal distance behind the Bottom Bracket to the BRP.
3. **Saddle-Handlebar distance `(d)`**: `700 mm` — Direct 2D Euclidean distance from BRP to the target cockpit point (`(481, 726)` relative to `BRP = (-219.0, 713.1)`): `hypot(TargetX - BrpX, TargetY - BrpY)`.
4. **Saddle-Handlebar Drop `(c)`**: `+13 mm` — Vertical height difference from BRP to the target cockpit point: `TargetY - BrpY` (positive = target is higher than BRP, `+3 mm` above `SitBones` at `Y = 723.1`).
5. **Saddle angle from BB `(g)`**: `72.9°` — Polar angle from the Bottom Bracket to the BRP: `atan2(713, 219) = 72.93°`. (Note: this is the angle to the BRP itself, not a frame's Seat Tube Angle).

**Seven fit sheet.** The sheet records 746 mm saddle height, 76 mm nose setback, 245 mm saddle length, 37 mm saddle-top-to-stem-center drop, and 803 mm saddle-tail-to-bar reach. The sheet's nose setback and saddle length imply `X_nose = −76 mm` and `X_tail = −321 mm` if the length is used horizontally. Adding the reported tail-to-bar reach gives `StemClampX = 482 mm`. The existing fit-sheet reconstruction puts `StemClampY` at about `676 mm`.

**Measured SQlab.** Matt measured 280 mm from tip to tail, 35 mm longer than the length on the Seven sheet. Under the level-rail approximation, using the SQlab's inferred W70 station 120 mm behind TIP with the idmatch BRP at `X = −219 mm` gives a modeled `X_TIP = −99 mm` and `X_TAIL = −379 mm`. These are estimates based on the idmatch BRP and the saddle specification. The reason for the discrepancy with Seven's sheet is unresolved. The owner-confirmed idmatch transfer used the same physical SQlab saddle and the reading on the source bike was taken with the Redshift post unsagged. See [the saddle measurements](../../docs/SQLAB_SADDLE.md) and [the rider fit record](../../docs/RIDER_FIT.md).

**Cockpit references.** The idmatch point at approximately `(481, 726) mm` is 700 mm from BRP and 13 mm above it. Subtracting 50 mm of stated bar rise without backsweep compensation gives about `(481, 676) mm`, close to the Seven reconstruction and the program's `TargetStemClamp`. Under a `TargetHandGrip` interpretation, placing the grip at `(481, 726) mm` with the Tumbleweed Persuader 20° bar (`BarOffset = (−72.8, +45.5) mm`) requires an estimated `StemClamp = (553.7, 680.6) mm`. The physical reference for the idmatch cockpit point remains unresolved.

---

## Hardware Modeling & Saddle Rails

Open `/saddles` in `bikegeo serve` for the catalog, modeled clamp windows and centered seat tube angles. Use `/saddle-swap` to compare matching BRP coordinates with preserving SIT, and `/bike` to inspect the installed landmarks and computation ledger.

- **Rail coordinates.** R0 is the center of the catalog rail span. Positive X points toward the nose along the rails. Positive Y is normal to the rails toward the saddle cover. `SaddleSpec.RailPitchDeg` is the assumed angle above horizontal. Catalog saddles assume level rails across fork configurations. Matt's installed rail pitch remains unmeasured.
- **Landmarks.** W70 is the longitudinal station of BRP. SIT is the modeled support landmark at the front foot of the raised rear lip on the SQlab. The W70 station is inferred from the owner-reported span and awaits direct width verification. The 10 mm elevation is owner-measured. See [the saddle specification](../../docs/SQLAB_SADDLE.md).

| Saddle | R0 to BRP | R0 to SIT | SIT elevation | Catalog rail span |
| --- | ---: | ---: | ---: | ---: |
| SQlab 610 Ergolux Active 2.1 | +32.0 mm | −48.0 mm | +10.0 mm | 60 mm |
| Berthoud Aspin Open | +43.5 mm | −44.5 mm | 0.0 mm | 64 mm |
| Selle Italia SLR Boost Superflow L3 | +17.7 mm | −65.3 mm | 0.0 mm | 40 mm |

- **Modeled support target.** The level-rail SQlab BRP-to-SIT vector is `(−80, +10) mm`. The report's 219 mm BRP setback and 746 mm radius put BRP at about `(−219, 713.1) mm` and SIT at about `(−299, 723.1) mm`. At that SIT point, the Aspin needs 211 mm setback and 753.3 mm radial height, and the SLR Boost needs 216 mm setback and 754.7 mm radial height. Their local rail-center-to-SIT offsets differ from the SQlab by +3.5 and −17.3 mm respectively. A swap solves for the clamp position and axial extension using both coordinates. BB-to-SIT distance is a geometric proxy for comparing those placements.
- **Clamp windows.** The modeled half window is `(rail span − clamp footprint) / 2`. An equal footprint permits only rail center. A longer footprint has no interval. SQlab's 60 mm is the measured span between marks. Usable rail length also depends on clamp shape, rail profile and shell clearance. The standard and Drop Best examples assume symmetric 30 mm and 45 mm footprints.
- **Seatpost examples.** Comparisons use a zero-setback dropper. Matt's Redshift was unsagged when his saddle position was transferred to the jig with the same physical saddle. The [rider fit record](../../docs/RIDER_FIT.md) documents that transfer. The hardware examples treat the [Redshift PRO nominal offsets](https://redshiftsports.com/products/shockstop-pro-suspension-seatpost) of 7 mm static and 12 mm at 25% travel as horizontal. The [FAIR Drop Best UC M6 redshift](https://www.fairbicycle.com/product-page/drop-best-uc-m6-redshift) adds a nominal 27 mm. In the rearward examples this gives 34 and 39 mm horizontal offset. The solver applies that displacement separately from any seat-perpendicular setback. Those examples retain the report's unsagged fit target.

### Constant-Radius Rail Adjustment

Let `u` be the seat axis pointing up and rearward, `n` its rearward perpendicular, and `r` the rail axis pointing forward at the saddle's assumed pitch. Let `p` be perpendicular seatpost setback, `o` the signed clamp station from R0, and `b` the local R0-to-BRP distance.

- `BRP = d × u + q`, where `q = p × n + (b − o) × r`.
- Choose the positive circle intersection `d = −dot(q, u) + sqrt(H² − dot(q, n)²)` to keep the BRP exactly `H` mm from the BB. A negative radicand or invalid extension has no placement.
- `Clamp = d × u + p × n` and `R0 = Clamp − o × r`. Apply the shared rotation about R0 to the local BRP and SIT coordinates. R0 and the clamp use a projected datum in the BRP plane because absolute clamp-to-cover height is unavailable.
- The inverse rail solve projects the target onto `n`. The inverse STA solve uses the required clamp point for a chosen rail station. The compromise STA minimizes the largest required rail offset across saddles. The catalog reports whether the resulting stations fall within the assumed windows.

---

## Front Cockpit Optimization (`StemClamp` / `HandGrip`) & Area of Reasonable Fit

The steerer tube extends upward and rearward at polar angle `180° - HTA`. With upper headset bearing cover stack `HeadsetTopStackMm` (`~11 mm` on `EC44`/`ZS44` headsets), steerer spacers `s` (`0..max_spacer_mm` in 5 mm steps), and stem clamp thickness of `40 mm` (`StemThicknessMm`), the center of the stem handlebar clamp (`StemClamp`) sits at:
- `SteererX = Reach - (HeadsetTopStackMm + s + StemThicknessMm/2) * cos(HTA)`
- `SteererY = Stack + (HeadsetTopStackMm + s + StemThicknessMm/2) * sin(HTA)`
- `StemClampX = SteererX + LengthMm * cos((90° - HTA) + AngleDeg)`
- `StemClampY = SteererY + LengthMm * sin((90° - HTA) + AngleDeg)`

### Handlebar Component Geometry & Contact Point Offset
A handlebar component translates the stem clamp center (`StemClamp`) to the hand grip center (`HandGrip`) and vice versa. Rather than relying solely on naive trigonometry (`Width/2 * tan(sweep)`), `bikegeo` implements a complete 3D piecewise linear kinematic transformation model (`internal/geometry/handlebar.go`, `internal/catalog/handlebars.go`):

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
- `GripX = StemClampX + Offset_X`
- `GripY = StemClampY + Offset_Y`

Under `TargetHandGrip`, to place the rider's hands at `(GripX, GripY)` from an idmatch report, the physical stem clamp center (`StemClamp`) must sit downward (`-50.0 mm`) and forward (`+47.9 mm`), correctly driving frame selection toward models with appropriate reach and stack. Under `TargetStemClamp`, the stem clamp target coordinate `(StemClampX, StemClampY)` (e.g., after subtracting `StatedRiseMm` without backsweep compensation on Seven's fit sheet) is targeted directly.

### Handlebar Clamp Fit (`handlebar_clamp_fit`)
When scoring cockpits against biomechanical profiles, handlebar positions are assessed against an **Area of Reasonable Fit** oval:
- **Ideal Center**: `(BrpX + ΔX_0, SitBoneY + c_0)` where `ΔX_0 = √(d_0² - (SitBoneY + c_0 - BrpY)²)`, from the profile's explicit cockpit target: `d_0 = CockpitTargetDistMm` (700 mm from the BRP) and `c_0 = CockpitTargetRiseMm` (3 mm above the sit bones), defined for the reference saddle. For another saddle `Profile.CockpitTarget` keeps the hands where they sit relative to the sit bones: it re-measures `d_0` from that saddle's BRP (Berthoud Aspin `691.9 mm`, SLR Boost `696.9 mm`).
- **Dimensions**: Semi-minor axis `a = 10.0 mm` (±1 cm horizontal) and semi-major axis `b = 20.0 mm` (±2 cm vertical).
- **Rotation**: 30° clockwise tilt, matching frame sizing geometry where longer reach requires taller stack.
- **Normalized Radius `R`**: `R = √((p_minor / 10)² + (p_major / 20)²)`
- **Points lost** (the ledger line): `22 × max(0, R − 1)` points of total for the selected setup, linear with no tail. Inside the oval is free.

### Stock Build Selection (`canonicalizer.ApplyIdealCockpitTargets`)
Each setup gets its cheapest stock build on the points-lost ledger, in two scans:
1. **Saddle**: every position in the assumed clamp window (1 mm steps) is priced by `brp_setback + rail_offset + seatpost_extension`, where published seat tube length is available. The cheapest wins (ties: nearer dead center). Beyond the clamp limit, the setback band absorbs the miss.
2. **Cockpit**: every stock combination (stem lengths in 10 mm steps × −6°/0°/+6° × spacers 0–`max_spacer_mm` in 5 mm steps; 189 on the `mtb` catalog) is placed relative to the chosen BRP and priced by `handlebar_clamp_fit + stem_length + stem_angle + spacers`; the cheapest is kept (ties: smaller R). Past the hardware limits the oval absorbs the miss.

Both totals are continuous in the frame geometry: each candidate's cost is continuous and the scan keeps their minimum. The closed-form 2×2 solve (`Stem/Spacer Fit`) still explains the result ("needs a 127 mm stem") and measures the excess past the hardware limits.

A profile without a scored `handlebar_clamp_fit` (or without a cockpit target) keeps the row's default selection: the canonical stem with no spacers. There is no separate range-based selector.

---

## Ideal Profile

### `idmatch` Profile
Built directly around the idmatch Bikelab report:
- `saddle_height_mm`: `746`
- `brp_setback`: Free at exactly `219 mm`; `1.2` points per mm either side
- `rail_offset`: Free within `±5 mm` of dead center; linear to `8` points at the saddle's clamp limit (`±15 mm` on the SQlab 610)
- `handlebar_clamp_fit`: Free inside the fit oval (`R ≤ 1`); `22` points per unit of R beyond the edge (displayed free range: `≥1` setup inside the oval)
- `stem_length`: Free `70–80 mm`; linear to `8` points at the stem catalog limits (`mtb` 40/100, `gravel` 60/110)
- `stem_angle`: Free at `0°`; `1` point at `−6°` or `+6°`
- `spacers`: Free `0–20 mm`; linear to `8` points at `max_spacer_mm` (`40 mm`)
- `saddle_hb`, `saddle_hb_drop`: Display-only with no ranges. The cockpit target is the profile's explicit `CockpitTargetDistMm = 700` (from the BRP) and `CockpitTargetRiseMm = +3` (above the sit bones; `+13 mm` above the lowered SQlab 70 mm BRP plane)
- `seatpost_extension`: Free `250–310 mm`; `0.18` points per mm below, `0.21` per mm above — Axial extension estimate: solved seat-axis distance to the projected clamp minus published seat tube length. The projected clamp uses the BRP plane as its height datum, so this is a proxy rather than a direct hardware fit measurement.
- `rc_wb_ratio`: Free `37–39 %` (`RearCenter / Wheelbase`); `4` points per percentage point either side
- `bbdrop`: Free `65–75 mm`; `0.44` points per mm either side
- `trail`: Free `98–106 mm`; `0.41` points per mm either side — Steering feel ground trail based on active HTA, fork rake, and 29x2.4" baseline tires (372mm wheel radius).
- `hta`: Display-only (`ActiveHta`)
- `seat_to_rc`: Display-only Seat Axis–Rear Axle (horizontal distance from the Seat Axis Point to rear axle)
- `sparevert`: Display-only Seat Axis–HT Drop (vertical drop from the Seat Axis Point to head tube top `SaddleHeight * sin(STA) - Stack`)

## Points-Lost Ledger (`internal/scoring`)

`Total = 100 − Σ points lost`, and it may go below 0.

- Each scored attribute is free inside its `Ideal` range and linear outside it: either `Cost.Below` / `Cost.Above` points of total per unit, or (hardware bands) `LimitPoints` reached at the setup's hardware limit (`ideal.HardwareLimits`: rail clamp window, stem catalog length and angle range, spacer stack). There are no weights, weighted averages, floors or curve tails, so adding or removing a band never rescales the others.
- A scored cell is `✓` when the printed value is inside the free range (or the band costs nothing) and `✗ N pts` otherwise; display-only bands keep a `✗` with their distance. There are no acceptable ranges.
- **Fit gate** (`scoring.Fits`): a setup fits when some stock build puts the hands inside the oval (`CountInside > 0`, or `R ≤ 1` for the selected build) and the BRP lands within 10 mm of the `brp_setback` free range. Every fitting setup ranks above every non-fitting one (`scoring.Better`); points order each group.
- Points use the unrounded value; the cell's marker and delta use the printed value.
- A profile needs only a free range and a cost per scored band: there are no `EdgeScore`, `Falloff`, `Floor` or `Acceptable` fields. `ForSaddle` carries only the `brp_setback` free value onto another saddle through the sit-bone invariant.
