# Ideal Profiles & Cockpit Fit Metrics

This directory contains ideal bike fit profiles used by `bikegeo compare` and the scoring engine to evaluate and rank frame configurations against personalized anthropometric and biomechanical targets.

---

## Saddle-Handlebar Distance (`saddle_hb`)

### Definition & Reference Points

The **Saddle-Handlebar distance** (`saddle_hb`, in mm) measures the 2D Euclidean distance in the sagittal plane from the **top of the seatpost** (at the rider's seat height along the active effective seat tube angle) to the **HB reference point**:

> *The HB reference corresponds to the intersection point between the handlebar and the stem of your bike. Defining the HB point facilitates operations to correctly restore the measurements suggested by your bike-fitting on the bike.* (idmatch Bikelab)

### Geometric Modeling

In a sagittal coordinate system where the Bottom Bracket (BB) spindle center is at `(0, 0)` with `+X` pointing forward and `+Y` pointing upward:

1. **Seatpost Top (`SeatX, SeatY`)**:
   At rider saddle height `L = SaddleHeightMm` along effective seat tube angle `STA`:
   - `SeatX = -L * cos(STA)`
   - `SeatY = L * sin(STA)`

2. **Steerer Tube & Stem Clamp Center (`SteererX, SteererY`)**:
   The steerer tube extends upward and rearward from the head tube top `(ActiveReach, ActiveStack)` at polar angle `180° - HTA`.
   With a fixed stem steerer clamp thickness of **40 mm** (`model.StemThicknessMm`) and steerer spacers `s` (`0 <= s <= max_spacer_mm` in discrete **5 mm** increments, `model.SpacerStepMm`), the center of the stem clamp sits at axial distance `d_steerer = s + 20.0 mm`:
   - `SteererX = ActiveReach - (s + 20.0) * cos(HTA)`
   - `SteererY = ActiveStack + (s + 20.0) * sin(HTA)`

3. **Stem Angle Rotation & HB Coordinates (`HbX, HbY`)**:
   A stem perpendicular to the steerer tube (`0°` angle) is rotated `90°` clockwise from the upward steerer axis, pointing at polar angle `(180° - HTA) - 90° = 90° - HTA` above horizontal.
   Applying stem angle `AngleDeg` (e.g. `-6°` rotates an additional `6°` clockwise / downward):
   - `theta_stem = (90.0° - HTA) + AngleDeg`
   - `HbX = SteererX + LengthMm * cos(theta_stem)`
   - `HbY = SteererY + LengthMm * sin(theta_stem)`

4. **Saddle-Handlebar Distance**:
   - `SaddleHbMm = hypot(HbX - SeatX, HbY - SeatY)`

---

## Frame Requirements

Every frame definition in `data/bikes/*.json` explicitly configures its cockpit hardware envelope:
- `max_spacer_mm`: Maximum allowable steerer spacer stack (typically `40` mm).
- `canonical_stem`: The reference stem length and angle (e.g. `60 mm / -6°` for mountain/rigid; `80 mm / -6°` for gravel).
- `stems`: The complete set of allowed stems (lengths and angles) suitable for the frame.

These fields are strictly validated by `pkg/validator`—no implicit defaults are assumed.

---

## State of Affairs in `default.json`

In [`data/ideals/default.json`](file:///Users/matt/src/bikegeo/data/ideals/default.json), `saddle_hb` is registered as a **display-only attribute** (no `weight` or `ideal` range). This ensures:
- Existing scoring baselines, total scores, and ranking regression tests remain 100% backward-compatible.
- The default comparison table displays the Saddle-Handlebar distance achieved using the frame's `canonical_stem` at `0 mm` spacer.

### Activating `saddle_hb` for Target Optimization & Scoring

To optimize stems and spacers for a specific target Saddle-Handlebar distance (such as **700 mm** from an idmatch Bikelab report), define an `ideal` range in an ideal profile:

```json
{
  "id": "saddle_hb",
  "weight": 8,
  "ideal": { "min": 695, "max": 705 },
  "acceptable": { "min": 685, "max": 715 },
  "edge_score": 75,
  "falloff": { "below": 15, "above": 15 }
}
```

When an ideal target is present, `bikegeo compare` automatically searches the full envelope of mapped HB points across all allowed stems and spacer stacks (`0..max_spacer_mm`), selecting the optimal `(stem, spacer)` combination that satisfies the target distance:
1. Minimizes distance outside the `ideal` range (points inside `[min, max]` are prioritized).
2. Breaks ties using the stem closest to the frame's `canonical_stem` (length first, then angle).
3. Breaks remaining ties with the lowest spacer stack height.
4. Breaks final ties by proximity to the target midpoint.
