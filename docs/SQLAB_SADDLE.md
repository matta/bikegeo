
# SQlab 610 ERGOLUX active 2.1 — Saddle Geometry Specification

## 1. Identification and manufacturer specifications

| Property | Value |
|---|---|
| Manufacturer | SQlab |
| Model | 610 ERGOLUX active 2.1 |
| Color | Black |
| Effective width | 140 mm |
| Overall length | Approximately 280 mm |
| Effective height | Approximately 55 mm |
| Weight (14 cm model) | Approximately 334 g |
| Rails | Hollow CrMo steel |
| Base | Glass-fiber-reinforced polyamide |
| Padding | Light Foam, 50 SQ-Shore |
| Active mechanism | Active Sport 2.1 |
| Intended use | Sport trekking / touring |
| Maximum rider weight | 100 kg |

**Source:** SQlab manufacturer specifications.

https://www.sq-lab.com/en/products/610-ergolux-active-2-1-57230-2017

The longitudinal geometry and SIT elevation below are based primarily on the owner's physical measurements of one 14 cm saddle.

## 2. Reference points and conventions

### 2.1 Reference definitions

| Reference | Definition |
|---|---|
| TIP | Extreme front tip of the saddle |
| TAIL | Extreme rear end of the saddle |
| W70 | Saddle centerline station where the external saddle width reaches 70 mm |
| SIT | Sit-bone reference landmark: front foot of the raised rear lip, on the middle seating shelf |
| R+30 | Center of the +30 rail marking |
| R0 | Midpoint between the +30 and -30 rail markings |
| R-30 | Center of the -30 rail marking |

The W70 reference corresponds to the saddle landmark used by the idmatch system to identify its Biomechanical Reference Point (BRP).

**W70 and BRP are not interchangeable coordinates.**

- W70 is defined in the saddle's intrinsic coordinate system.
- BRP is a point expressed in the bicycle-fitting coordinate system, typically relative to the bottom bracket.
- This specification uses W70 exclusively for saddle geometry.

The idmatch 70 mm criterion is documented at:

https://www.idmatch.cc/saddle-adjustment-brp/

### 2.2 Coordinate conventions

All longitudinal positions use a common axis parallel to the saddle rails.

- Origin: R0 for rail-centered geometry and fitting parameters; TIP for the explicitly labeled TIP-relative tables.
- Positive X direction: forward, toward TIP, matching the program.
- Negative X direction: rearward, toward TAIL.
- Positive elevation: upward above the W70 / BRP reference plane.
- Units: millimeters.
- Distances are longitudinal projections, not distances along the curved saddle surface.

R0 is the longitudinal midpoint of the marked 60 mm rail range. It is not necessarily the midpoint of the full exposed rail length.

The coordinate system describes the intrinsic geometry of the saddle, independently of its installation on a bicycle.

Physical measurement lengths such as TIP-to-SIT remain unsigned positive distances. Coordinates and displacements are signed: a point behind TIP has a negative TIP-relative X coordinate. Changing the origin translates coordinates without changing the axis direction.

## 3. Owner's physical measurements

| Measurement | Value | Provenance |
|---|---:|---|
| TIP to TAIL | 280 mm | Measured |
| TIP to SIT | 200 mm | Measured, corrected |
| W70 to SIT | 80 mm | Owner-reported measurement |
| SIT elevation above the W70 / BRP reference plane | 10 mm | Physically measured by Matt; confirmed 2026-11-16 |
| TIP to R+30 | 122 mm | Measured |
| R+30 to R-30 | 60 mm | Measured |
| R-30 to TAIL | ~100 mm | Approximate measurement |
| R-30 to SIT | ~18–20 mm | Approximate direct check |

The original TIP-to-SIT measurement was 215 mm. The owner subsequently corrected it to 200 mm after identifying an inconsistency with the overall saddle length.

The W70-to-SIT measurement was originally reported using the term BRP. It is treated here as the distance from the idmatch 70 mm-width station to SIT. The 70 mm width at that station has not been independently verified.

The TIP-to-R+30 distance was measured parallel to the saddle rails using a perpendicular projection through the saddle tip.

The 10 mm SIT elevation is an owner-measured property of this actual saddle, not an assumed step height or a value inferred from the manufacturer's approximately 55 mm effective saddle height. It supplies the vertical separation of SIT from the W70 / BRP reference plane; it does not establish the saddle's installed pitch or its height relative to the rails or seatpost clamp.

### 3.1 Measurement closure

The principal saddle-length measurements are consistent:

- TIP to SIT: 200 mm
- SIT to TAIL: 80 mm
- Total: 280 mm

The SIT-to-TAIL distance is derived from the total length and TIP-to-SIT measurements. It happens to equal the separately reported W70-to-SIT distance.

The rail-related measurements give:

- TIP to R+30: 122 mm
- R+30 to R-30: 60 mm
- R-30 to TAIL: approximately 100 mm
- Sum: approximately 282 mm

This differs from the measured 280 mm overall length by 2 mm.

For the canonical coordinate model, the 280 mm overall length and the 122 mm and 60 mm rail measurements take precedence.

The resulting R-30-to-TAIL distance is 98 mm, consistent with the approximate 100 mm direct measurement.

## 4. Canonical longitudinal coordinates

### 4.1 Coordinates relative to TIP

| Reference | X coordinate | Provenance |
|---|---:|---|
| TIP | 0 mm | Definition |
| W70 | -120 mm | Derived |
| R+30 | -122 mm | Measured |
| R0 | -152 mm | Derived |
| R-30 | -182 mm | Derived |
| SIT | -200 mm | Measured, corrected |
| TAIL | -280 mm | Measured |

### 4.2 Derivations

W70 coordinate:

    X_SIT = -TIP-to-SIT = -200 mm
    X_W70 = X_SIT + W70-to-SIT
          = -200 + 80
          = -120 mm

Rail midpoint:

    X_R0 = X_R+30 - 60/2
         = -122 - 30
         = -152 mm

Rear rail marking:

    X_R-30 = X_R+30 - 60
           = -122 - 60
           = -182 mm

The physical ordering from front to rear has decreasing X coordinates:

    TIP = 0
    W70 = -120
    R+30 = -122
    R0 = -152
    R-30 = -182
    SIT = -200
    TAIL = -280

All subsequent translations use these canonical coordinates.

## 5. Complete longitudinal translation matrix

Each cell gives the signed displacement from the row's reference point to the column's reference point.

- Positive (+): forward.
- Negative (-): rearward.
- Zero: same longitudinal position.

All values are in millimeters.

| FROM / TO | TIP | W70 | R+30 | R0 | R-30 | SIT | TAIL |
|---|---:|---:|---:|---:|---:|---:|---:|
| TIP | 0 | -120 | -122 | -152 | -182 | -200 | -280 |
| W70 | +120 | 0 | -2 | -32 | -62 | -80 | -160 |
| R+30 | +122 | +2 | 0 | -30 | -60 | -78 | -158 |
| R0 | +152 | +32 | +30 | 0 | -30 | -48 | -128 |
| R-30 | +182 | +62 | +60 | +30 | 0 | -18 | -98 |
| SIT | +200 | +80 | +78 | +48 | +18 | 0 | -80 |
| TAIL | +280 | +160 | +158 | +128 | +98 | +80 | 0 |

Translation rule:

    displacement(A, B) = coordinate(B) - coordinate(A)

The matrix is mathematically consistent and antisymmetric: reversing a translation reverses its sign.

## 6. Rail-centered geometry

### 6.1 Coordinates relative to R0

With R0 as the origin and positive values forward, matching the program:

| Reference | Offset from R0 |
|---|---:|
| TIP | +152 mm |
| W70 | +32 mm |
| R+30 | +30 mm |
| R0 | 0 mm |
| R-30 | -30 mm |
| SIT | -48 mm |
| TAIL | -128 mm |

Two principal saddle-fitting parameters are:

    Rail0ToW70Mm      = +32
    Rail0ToSitBonesMm = -48

The distance between W70 and SIT is 80 mm.

These correspond to the program fields `RailCenterToBrpMm = +32` and `RailCenterToSitBonesMm = -48`; no sign conversion is required.

### 6.2 Translations for different rail positions

The following table assumes the longitudinal center of the seatpost clamp is aligned with the indicated rail reference.

Positive offsets are forward; negative offsets are rearward.

| Clamp at | TIP | W70 | SIT | TAIL |
|---|---:|---:|---:|---:|
| R+30 | +122 | +2 | -78 | -158 |
| R0 | +152 | +32 | -48 | -128 |
| R-30 | +182 | +62 | -18 | -98 |

The rail marking convention is:

- R+30 is 30 mm forward of R0.
- R-30 is 30 mm rearward of R0.

These are geometric reference positions, not assertions about the compatibility of a particular seatpost clamp with the endpoints.

### 6.3 General clamp-position translation

Define:

    clampOffsetMm = signed clamp-center position relative to R0 (+ forward)

Then:

    ClampToTipMm      = +152 - clampOffsetMm
    ClampToW70Mm      = +32  - clampOffsetMm
    ClampToSitBonesMm = -48  - clampOffsetMm
    ClampToTailMm     = -128 - clampOffsetMm

Examples:

- Clamp at R+30: clampOffsetMm = +30
- Clamp at R0: clampOffsetMm = 0
- Clamp at R-30: clampOffsetMm = -30

These formulas describe saddle-relative geometry only.

`clampOffsetMm` has the same sign as the program's `RailOffsetMm`: positive means the clamp lies forward of R0 on the saddle. With a fixed clamp on the bicycle, that places the saddle body farther rearward relative to the clamp.

### 6.4 Measured SIT elevation

In addition to the longitudinal coordinates above:

    SitBonesElevationMm = +10

SIT is 10 mm above the W70 / BRP reference plane, physically measured by Matt and confirmed on 2026-11-16.

For the level-rail reference approximation, the W70-to-SIT displacement is 80 mm rearward and 10 mm upward: `(-80, +10) mm` in the shared positive-forward / positive-upward convention.

Longitudinal distances run parallel to the saddle rails, which are closer to parallel to the ground than perpendicular to the seat tube. They must not be applied along the seat-tube perpendicular. Any conversion to bicycle-relative coordinates must state the assumed rail pitch and use a consistent saddle-coordinate transform. No numerical installed rail pitch has been supplied.

## 7. W70 measurement and external cross-check

An earlier external estimate placed the 70 mm-width reference 23 mm forward of the nominal rail center.

The present measurement-derived geometry gives:

    Rail0ToW70Mm = +32 mm

Comparison:

| Source | W70 relative to R0 |
|---|---:|
| Owner-measurement derivation | +32 mm |
| Earlier external estimate | +23 mm |
| Absolute difference | 9 mm |

The owner's measurements take precedence in the canonical model.

However, W70 was inferred from the originally reported 80 mm distance between the idmatch reference and SIT. Its position has not been independently confirmed by directly measuring the saddle's external width at 120 mm behind TIP.

Accordingly:

- +32 mm is the canonical measurement-derived value.
- +23 mm is a secondary external estimate.
- The 9 mm difference is an observed disagreement, not an established measurement uncertainty.
- No manufacturer-published W70 coordinate for this specific saddle has been established.

A direct width measurement would provide independent validation.

## 8. Accuracy, provenance, and scope

### Manufacturer specifications

Manufacturer specifications are nominal values for the product model and may differ slightly from an individual specimen.

### Physical measurements

The owner's measurements describe one actual 14 cm saddle.

No calibrated measurement uncertainty was provided. Values should not be interpreted as exact to the millimeter merely because integer millimeter coordinates are used.

The approximate R-30-to-TAIL and R-30-to-SIT measurements are consistent with the canonical geometry.

### Derived values

All derived coordinates and translations are exact arithmetic consequences of the chosen canonical measurements, not independent high-precision observations.

### W70 interpretation

The W70 location depends on the owner's reported 80 mm distance from the idmatch reference to SIT.

Its identification as the actual 70 mm-width station remains an assumption pending direct width verification.

### Scope

This specification covers intrinsic longitudinal saddle geometry and the owner-measured 10 mm SIT elevation above the W70 / BRP reference plane.

It does not specify:

- Bicycle-relative BRP coordinates.
- Saddle or seatpost installation height.
- Saddle pitch or roll.
- Seatpost geometry.
- Bicycle suspension.
- Rider-dependent contact-pressure position.
- Absolute vertical reference-point coordinates, including rail-to-surface and clamp-to-surface heights.

No loading or sag translations are included.

## 9. Summary

| Parameter | Canonical value | Provenance |
|---|---:|---|
| Overall length | 280 mm | Measured |
| Marked rail span | 60 mm | Measured |
| TIP to R0 distance | 152 mm | Derived |
| R0 to TAIL distance | 128 mm | Derived |
| TIP to W70 distance | 120 mm | Derived |
| TIP to SIT distance | 200 mm | Measured, corrected |
| W70 to SIT distance | 80 mm | Owner-reported |
| SIT elevation above W70 / BRP reference plane | +10 mm | Physically measured by Matt; confirmed 2026-11-16 |
| SIT to TAIL distance | 80 mm | Derived |
| Rail0ToW70Mm | +32 mm | Derived |
| Rail0ToSitBonesMm | -48 mm | Derived |
| W70 relative to SIT | 80 mm forward | Owner-reported |
| External W70 estimate | +23 mm from R0 | Secondary estimate |

### Canonical TIP-relative reference values

    TIP   =   0
    W70   = -120
    R+30  = -122
    R0    = -152
    R-30  = -182
    SIT   = -200
    TAIL  = -280

### Canonical rail-centered values

    Rail0ToTipMm      = +152
    Rail0ToW70Mm      =  +32
    Rail0ToSitBonesMm =  -48
    Rail0ToTailMm     = -128

### Measured elevation

    SitBonesElevationMm = +10

All values are in millimeters.

Positive longitudinal offsets indicate forward displacement, matching the program. Positive elevation indicates upward displacement above the W70 / BRP reference plane. Unsigned measurement lengths remain positive regardless of direction.

The canonical geometry is internally consistent. Its principal unresolved empirical detail is independent verification of the W70 location.
