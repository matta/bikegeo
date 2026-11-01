# Canonical Saddle Coordinate System & BRP Fit Translation

This guide explains how `bikegeo` models saddle geometry, mounting rail constraints, the **Biomechanical Reference Point (BRP)**, and the rider's **sit bones** for custom frame geometry and bike fit calculations.

---

## 1. Physical Roles: Frame Datum vs. Anatomical Contact vs. Caliper Landmark

In bike fit modeling, it is critical not to conflate three separate physical realities:
1. **Mechanical Mounting Datum (Saddle Rails):** The bicycle frame connects only to the saddle rails via the seatpost clamp. The longitudinal origin is the exact center of the straight clamping section (`X_RC = 0.0 mm`).
2. **Anatomical Contact Point (Sit Bones):** The rider's pelvis and ischial tuberosities rest on the rear platform of the saddle (`RailCenterToSitBonesMm`, typically `-45 mm` to `-50 mm` behind the rail center).
3. **Caliper Fit Landmark (70mm BRP):** The Biomechanical Reference Point popularized by idmatch is defined as the cross-section along the nose where the saddle measures **70 mm wide**. It is an external measurement landmark, not a contact point.

Because different saddles have different nose tapers, the distance from BRP to the sit bones varies considerably (e.g. `73 mm` on an SQlab 610 vs. `88 mm` on a Berthoud Aspin Open). **Any time a BRP coordinate is used to compute a physical location (such as a seat tube angle, clamp offset, or sit bone position), it must be resolved through a specific saddle.**

---

## 2. Canonical Coordinate Sign Convention

All longitudinal measurements are projected along the plane of the saddle rails:
* **Origin (`0.0 mm`):** Exact center of the straight rail clamping zone (`X_RC = 0.0 mm`).
* **Positive (`+`):** Forward toward the saddle nose.
* **Negative (`-`):** Rearward toward the saddle tail.

```text
  <--- REAR (Negative -)                       FRONT (Positive +) --->

    [Sit Bones]                 [Rail Center]               [BRP 70mm]
 RailCenterToSitBonesMm < 0        X_RC = 0            RailCenterToBrpMm > 0
--------|-----------------------------|-------------------------|-------->
        |<-- |RailCenterToSitBones| ->|<-- RailCenterToBrpMm -->|
        |<----------------- DeltaBrpToSitBones ----------------->|
```

* `DeltaBrpToSitBones = RailCenterToBrpMm - RailCenterToSitBonesMm`

---

## 3. Direct Measurements: SQlab 610 Ergolux vs. Berthoud Aspin Open

| Saddle Model | Straight Rail Length | Extended Rear Rail | Rail -> BRP (70mm) | Rail -> Sit Bones | BRP -> Sit Bones |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **SQlab 610 Ergolux Active 2.1** | `60.0 mm` (`-30` to `+30`) | `+7.5 mm` (to `-37.5 mm`) | `+23.0 mm` | `-50.0 mm` | `73.0 mm` |
| **Berthoud Aspin Open** | `64.0 mm` (`-32` to `+32`) | `0.0 mm` (hard stops) | `+43.5 mm` | `-44.5 mm` | `88.0 mm` |
| **Delta (`Aspin - SQlab`)** | `+4.0 mm` | — | **`+20.5 mm`** | **`+5.5 mm`** | **`+15.0 mm`** |

### Key Takeaway on Pelvic Fit Equivalence
* The sit bone position relative to the rail center differs by **only 5.5 mm** between the SQlab 610 and Berthoud Aspin.
* Translating from SQlab to Berthoud Aspin requires only a `5.5 mm` clamp shift on the rails, or represents only **`0.44°` of Seat Tube Angle** at a `746 mm` saddle height.
* The large `20.5 mm` difference in BRP position is an artifact of nose taper. If a fitter blindly placed the Berthoud Aspin at the SQlab's BRP setback without translating sit bones, the rider's pelvis would be pushed **`15.0 mm` too far back**.

---

## 4. Seatpost Clamp Travel Invariants

For a straight rail length `L_rail` and a seatpost clamp of length `L_clamp`, the usable travel of the clamp center on the rails is:
* `ClampCenterNominalMin = -((L_rail - L_clamp) / 2)`
* `ClampCenterNominalMax = +((L_rail - L_clamp) / 2)`
* `ClampCenterPhysicalMin = ClampCenterNominalMin - ExtendedRearRailMm`

With a **45 mm Fair Bicycle Drop Best clamp**:
* **SQlab 610:** Nominal `[-7.5, +7.5] mm`. With `7.5 mm` extended rear straight rail, physical travel reaches `[-15.0, +7.5] mm`.
* **Berthoud Aspin:** Hard dogleg stops at `±32.0 mm` limit clamp travel to `[-9.5, +9.5] mm`. Clamping the SQlab at `-15.0 mm` translates to `-9.5 mm` on the Aspin—landing right on the rear mechanical stop.

---

## 5. Ideal Custom Frame Seat Tube Angle (STA)

To hit a target sit bone setback of `292.0 mm` behind the BB at `746 mm` saddle height with the saddle clamped **dead-center (`0.0 mm`)**:

```text
IdealSTA = acos((TargetSitBoneSetback + RailCenterToSitBones - SeatpostSetback) / SaddleHeight)
```

* **Standard 15mm Setback Post:** `Ideal STA = 72.4°`
* **Standard 25mm Setback Post:** `Ideal STA = 73.2°`
* **Redshift Post (7mm static / 11mm sagged):** `Ideal STA = 71.7°` / `72.0°`
* **Redshift + Drop Best (35mm static / 39mm sagged):** `Ideal STA = 74.1°` / `74.5°`

Use `bikegeo saddles` to inspect the full table and cross-saddle translation interactively.
