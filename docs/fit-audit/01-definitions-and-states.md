# Stage 1: Definitions, coordinates, and load states

Status: In progress, started 2026-11-23. Code revision `b4a46e974e02`. No Go files have changed since `b961e8818121`. Names below are the current code names. No new name has been proposed or ruled yet (see [Nomenclature](README.md#nomenclature)).

## Scope and premises

Stage 1 describes what the model's quantities mean physically, and in which axes. The bike is placed as one rigid frame on two wheels, saddle, and grip. It is then compared with the same installed bike unloaded and at fork sag. No upstream stage is relied on.

Symbols examined: `PlaceBike`, `PlaceFrameset`, `placeFramesetAtPitch`, `ForkSetup.EffectiveA2cMm`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, `PlaceHandlebar`, `pose.Pose.Face`/`Turn`/`Translate`, `catalog.Seatposts`, `catalog.DefaultSaddle`, `catalog.DefaultBrpTarget`, `catalog.DefaultGripTarget`.

## How the model places a bike today

The construction below summarizes the code. Each item is **Tested** by the existing placement tests, or follows directly from the code.

1. **World axes come from the axle line.** `PlaceFrameset` builds the frame at 0° pitch, measures the angle of the rear-axle → front-axle line, and rebuilds it rotated by that angle about `BB`. The world origin is `BB`, `+X` points forward along the axle line, and `+Y` points up. There is one `WheelSetup.RadiusMm`, so the ground line sits one radius below both axles and is parallel to the axle line. "Leveled" in code means *axle line horizontal*.
2. **The frame chart is a different attitude.** `FrameSetup` holds chart values (reach, stack, HTA, STA, BB drop) in the chart's *unpitched* attitude, which levels the axles only with the chart's basis fork and cup. An installed fork pitches the frame. Example: Fenrir L with SID 100 (506 × 44) unloaded pitches +1.08° (nose up) from the chart. The `Leveled…` readouts (`LeveledReachMm`, `LeveledSeatTubeAngleDeg`, and so on) are installed values in world axes, and they're what a tape measure would find.
3. **Load state is a single number, the fork's sag.** `SagPct` shortens the fork's effective A2C by `travel × sag`, and the frame re-levels. Nothing else changes with load: tires don't compress, there's no rear suspension, and the seatpost has no unloaded and loaded pair.
4. **Most parts ride rigidly with the frame.** The seatpost extends along the seat axis, and its `SetbackMm` is perpendicular to that axis. The stem starts from the steerer axis, and `AngleDeg` is measured from the perpendicular to the steerer. Rise and sweep are fixed relative to the bar clamp. All of these rotate with frame pitch.
5. **Two angles are fixed to the world instead.** `PlaceSaddle` uses `Face(RailPitchDeg)` and `PlaceHandlebar` uses `Face(−BarRollDeg)`. Both set an absolute world heading, so they ignore the frame's pitch.
6. **The ShockStop is modeled in one state.** `redshift_shockstop_pro` has a fixed 12 mm `SetbackMm`, which is owner-reported *when sagged* (Q28). It has no vertical compression. Q78 already notes that the default BRP target was measured with this post unsagged.
7. **Targets are world points relative to BB, with no load state attached.** `DefaultBrpTarget` (−219, 713.1) and `DefaultGripTarget` (481, 726) are compared directly with placed landmarks in whatever state the placement used.

## Contrast: the same installed bike unloaded and at sag

**Observed** with the `bikegeo bike` CLI at `b4a46e974e02`. The build is Fenrir L, `sid_100-506x44`, with defaults otherwise (ShockStop at 175 mm insertion, SQlab 610 centered, 60 mm −6° stem, Persuader 800, `rail_pitch=0`, `bar_roll=0`).

| Case | Frame pitch | BRP | SitBones | HandGrip |
| --- | --- | --- | --- | --- |
| A. `sag=0` | +1.0765° | (−163.68, 680.67) | (−243.68, 690.67) | (382.25, 759.57) |
| B. `sag=20`, angles held in world (current code) | +0.1535° | (−153.42, 683.74) | (−233.42, 693.74) | (393.70, 752.18) |
| C. `sag=20`, angles carried with frame (`rail_pitch=−0.923`, `bar_roll=+0.923`) | +0.1535° | (−152.70, 683.22) | (−232.53, 694.51) | (394.44, 753.31) |

Case C reproduces a rigid installation exactly. The rail frame and bar-roll frame are the only world-faced poses, so rotating each by the frame's pitch change (−0.923°) carries every downstream landmark rigidly. From A to C, the same installed bike sagging 20% moves BRP by (+10.98, +2.55) mm and HandGrip by (+12.19, −6.26) mm.

The current code (B) differs from the rigid case (C) by:

- BRP (−0.72, +0.52) mm and HandGrip (−0.74, −1.13) mm;
- the saddle staying level in the world when it should tip 0.92° nose-down with the frame. Bar roll has the same 0.92° error.

The landmark error is about 1 mm, half the BRP Y gate. It grows with fork travel and sag.

## Ambiguities found

Each item is a question about meaning, not yet about names.

1. **What defines "level"?** The model levels the axle line. A real bike is measured standing on a floor. The two agree only if both tires have the same loaded radius and the floor is level. Matt's front and rear tires may differ.
2. **What does an angle input mean?** `RailPitchDeg` and `BarRollDeg` are world angles in whatever load state is being placed. Physically, a clamped saddle or bar has a fixed *installed angle relative to the post or stem*. Its *world attitude* is a measurement taken in a stated load state. The code treats these as one quantity, which causes the counter-rotation above. This is the core of T2.
3. **What is a load state?** In code it is fork `SagPct` alone. Physically it also includes post compression and, to a lesser degree, tire compression. The ShockStop's 12 mm setback is a loaded-state value for a part that also has an unloaded state. So the model can't represent Matt's *unsagged* Fenrir with the ShockStop installed, which the Fenrir check (T4) needs.
4. **In which state was each target measured?** The default BRP comes from an unsagged post, on a source bike whose fork state isn't recorded. The default HandGrip comes from the idmatch report. A target with no load state can't be compared with a placement unambiguously. Stage 3 owns transfer, but Stage 1 needs a term for a target's reference state.
5. **One word, two meanings: "setback".** `SeatpostSpec.SetbackMm` is a post-local perpendicular offset of the rail cradle from the shaft axis. "BRP setback behind BB" (219 mm in `RIDER_FIT.md`) is a world horizontal distance. They're different quantities with different owners.
6. **`SitBones` vs SIT.** The code's `SitBones` landmark and the `SitBonesToGripRiseMm` comment ("sit-bone contact point") suggest anatomy. `RIDER_FIT.md` says the landmark is a saddle support reference (SIT) and not a pelvic location.
7. **What is measured at HandGrip and BRP?** `HandGrip` is the side-view projection of a 3D point on the bar, `GripInsetMm` in from the cut end, with `HandLateralMm` kept separately. BRP is a saddle-local point at the W70 station, whose height above the rails (`RailToBrpHeightMm` = 45 mm) is illustrative. Reproducing either on a real bike needs a physical definition of the point and its datum.
8. **`Leveled` hides the load state.** `LeveledReachMm` and similar values change with sag, but their names don't say which state they describe.

## Working glossary

Every row has the status *as in code*: it records the current meaning so it can be discussed, not a proposed name. "World" means the placement's leveled world axes (item 1 above).

| Term (code) | Meaning in code | Type | Owner and axes | Changed by | Value known from | Consumed by |
| --- | --- | --- | --- | --- | --- | --- |
| `BB` | Bottom bracket center; world origin | Pose (origin + frame pitch) | Frame | Nothing; it is the datum | Definition | Every coordinate |
| World axes | `+X` along the leveled axle line, `+Y` up | Axes | Placement | Fork, cup, sag, chart | Construction | Every coordinate |
| Frame pitch (`PitchDeg`) | Rotation from the chart's unpitched attitude to leveled | Angle | Frame, about `BB` | Fork A2C and rake, lower cup, sag | Calculated | All landmarks |
| Chart values (`ReachMm`, `StackMm`, `HeadTubeAngleDeg`, `SeatTubeAngleDeg`, `BbDropMm`, `ChainstayMm`) | Frame dimensions in the unpitched attitude | Scalars | Frame | Choosing a frame or size | Manufacturer chart | Placement |
| `Leveled…` readouts | Chart-like values measured in world axes after leveling | Scalars | Placement | Anything that pitches the frame | Calculated | Display, scoring |
| `SagPct` | Fork compression as a percentage of travel | Scalar | Fork | Load | Input (default per travel) | Effective A2C, so frame pitch |
| `RadiusMm` | One radius for both wheels | Scalar | Wheels | Choosing tires | Input (catalog default 372 mm) | Ground line, BB height |
| `InsertionMm` | Post shaft length below the seat tube top | Scalar | Post, along seat axis | Adjustment | Input or fitting | Rail cradle |
| `SetbackMm` (post) | Rail cradle offset perpendicular to the shaft, rearward | Scalar | Post-local | Choosing a post | Catalog (ShockStop: owner-reported, sagged) | Rail cradle |
| `RailCradle` | Post head's clamp point | Pose | Post-local axes | Post, insertion | Calculated | Saddle |
| `RailPitchDeg` | Rail heading in world axes | Angle | **World** (not the post) | Input | Input | Saddle landmarks |
| `RailOffsetMm` | Saddle slide along rails; positive is rearward | Scalar | Rails | Adjustment | Input or fitting | Saddle landmarks |
| `BRP` | Saddle landmark at W70, `RailCenterToBrpMm` forward of and `RailToBrpHeightMm` above rail center | Landmark (stored as Pose) | Saddle-local, placed in world | Everything upstream | Saddle data (height illustrative) | BRP target, gates, scores |
| `SitBones` | Saddle support landmark (SIT), 80 mm behind and 10 mm above BRP | Landmark (stored as Pose) | Saddle-local | Everything upstream | Owner-measured saddle | Rise readout |
| `AngleDeg` (stem) | Stem angle from perpendicular to the steerer | Angle | Steerer axis | Choosing a stem | Catalog or input | Bar clamp |
| `SpacersMm`, `UpperStackMm` | Axial stack below the stem; upper stack fixed at 11 mm | Scalars | Steerer axis | Adjustment (spacers) | Input; 11 mm illustrative | Bar clamp |
| `BarRollDeg` | Bar roll in world axes; positive rolls the top forward | Angle | **World** (not the stem) | Input | Input | HandGrip |
| `HandGrip` | Side projection of the grip point `GripInsetMm` in from the cut bar end | Landmark (stored as Pose) | Bar-local, placed in world | Everything upstream | Calculated | Grip target, `GripR` |
| BRP target, HandGrip target | Desired landmark positions relative to BB, in world axes | Points | Rider | Rider's choice | idmatch report (load state not attached) | Fitting, scoring |
| BRP miss | `target − placed` | Displacement | World | Placement, target | Calculated | Gates, scores |

## Proposed meanings to discuss

These are proposals for discussion, not rulings.

- **Reference state.** The unloaded bike stands on a level floor with no rider, both tires on the floor, and the fork and post fully extended. Every installed angle and every workshop measurement is defined in this state, so it is the state Matt can measure.
- **Installed angles.** A saddle's rail angle and a bar's roll are installation settings, relative to the post head and stem clamp. They could be *entered* as the world angle measured in the reference state, which is what a digital level reads. Other load states then carry them rigidly with the frame (case C).
- **Load state.** A named, complete set of compressions: fork sag, post sag, and possibly tire compression. Every placement, readout, and target says which load state it is in.
- **World axes.** Defined by the floor, with each wheel's radius (or measured axle height) given separately, rather than by the axle line.

## Code audit and tests

Nothing has been committed yet. If Matt confirms the meaning in "Installed angles," a committed test should check that changing only `SagPct` moves every saddle and cockpit landmark by one rigid rotation about `BB` plus a translation, with the saddle and bar angles changing by the frame's pitch change. Under current code that test would fail, and it would be committed disabled under the [defect procedure](README.md#defects-and-fixes).

## Session handoff

- **Conclusion so far.** The model defines world axes by the axle line and load state by fork sag alone, and it holds rail pitch and bar roll fixed to the world. As a result, it doesn't carry the saddle and bar rigidly when sag changes. On the Fenrir L with SID 100 at 20% sag, that misplaces BRP and HandGrip by about 1 mm and the saddle and bar angles by 0.92°. Whether this is a defect depends on the meaning of the angle inputs (item 2), which Matt hasn't ruled on.
- **Evidence.** The code at `b4a46e974e02` (symbols listed under Scope), and the CLI table above (**Observed**).
- **Waiting on Matt.**
  1. Is the **reference state** above the right one for measuring and entering installed values?
  2. Should rail pitch and bar roll be **installed angles** carried rigidly through load changes, entered as the reference-state world angle? Or is a world-relative angle intended for some use?
  3. For the Fenrir check, how should the **unsagged ShockStop** be represented? Options: (a) measure with a rigid post installed; (b) add an unloaded state for the post, from Redshift's published offsets at rest (see the [source record](../../internal/catalog/seatpost_redshift_shockstop_pro.md)); (c) measure the unloaded post's cradle offset directly.
  4. Should **world axes** come from the floor, with front and rear radii given separately?
  5. Build facts for the Fenrir: see the [Fenrir measurement note](fenrir-measurements.md).
- **Next question.** "Settle what a load state is and what an installed angle is. Should rail pitch and bar roll be installation settings relative to the post and stem, entered as the angle measured on the unloaded bike on a level floor?"
