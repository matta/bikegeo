# Stage 1: Definitions, coordinates, and load states

Status: In progress, started 2026-11-23. Code revision `b4a46e974e02`. No Go files have changed since `b961e8818121`. Names in the glossary are the current code names. Matt has chosen the state names static and dynamic, which are not yet recorded as a `Q#` (see [Nomenclature](README.md#nomenclature)).

## Scope and premises

Stage 1 describes what the model's quantities mean physically, and in which axes. The bike is placed as one rigid frame on two wheels, saddle, and grip. It is then compared with the same build unloaded and at fork sag. No upstream stage is relied on.

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

## Contrast: the same build unloaded and at sag

**Observed** with the `bikegeo bike` CLI at `b4a46e974e02`. The build is Fenrir L, `sid_100-506x44`, with defaults otherwise (ShockStop at 175 mm insertion, SQlab 610 centered, 60 mm −6° stem, Persuader 800, `rail_pitch=0`, `bar_roll=0`).

| Case | Frame pitch | BRP | SitBones | HandGrip |
| --- | --- | --- | --- | --- |
| A. `sag=0` | +1.0765° | (−163.68, 680.67) | (−243.68, 690.67) | (382.25, 759.57) |
| B. `sag=20`, angles held in world (current code) | +0.1535° | (−153.42, 683.74) | (−233.42, 693.74) | (393.70, 752.18) |
| C. `sag=20`, angles carried with frame (`rail_pitch=−0.923`, `bar_roll=+0.923`) | +0.1535° | (−152.70, 683.22) | (−232.53, 694.51) | (394.44, 753.31) |

Case C reproduces a rigidly clamped saddle and bar exactly. The rail frame and bar-roll frame are the only world-faced poses, so rotating each by the frame's pitch change (−0.923°) carries every downstream landmark rigidly. From A to C, the same build sagging 20% moves BRP by (+10.98, +2.55) mm and HandGrip by (+12.19, −6.26) mm.

The current code (B) differs from the rigid case (C) by:

- BRP (−0.72, +0.52) mm and HandGrip (−0.74, −1.13) mm;
- the saddle staying level in the world when it should tip 0.92° nose-down with the frame. Bar roll has the same 0.92° error.

The landmark error is about 1 mm, half the BRP Y gate. It grows with fork travel and sag.

## Ambiguities found

Each item is a question about meaning, not yet about names.

1. **What defines "level"?** The model levels the axle line. A real bike is measured standing on a floor. The two agree only if both tires have the same radius and the floor is level. Matt's front and rear tires may differ.

   *Note (Matt, 2026-11-23):* a level frame is the normal, expected case, so saying a frame is "leveled" is usually unnecessary, especially in the UI. The unleveled state is the one that needs calling out. It is degenerate and needs correcting. In code, the unleveled state is the chart's own attitude, used by `PlaceUnpitchedFrameset` and the chart values in `FrameSetup`.
2. **What are the saddle-angle and bar-roll settings measured against?** Two of a build's settings are angles: `SaddleSetup.RailPitchDeg` (saddle rail angle) and `HandlebarSetup.BarRollDeg` (handlebar roll). `PlaceSaddle` and `PlaceHandlebar` measure both against world horizontal in whatever state the bike is placed in. Today that is the fork at its `SagPct`. On a real bike, a clamped saddle's rails hold a fixed angle relative to the seatpost, and a clamped bar holds a fixed roll relative to the stem. What a level reads on either one depends on the state the bike is in. The code doesn't separate the clamped angle from the level reading, which causes the difference between cases B and C above. This bears on T2 in the README's [question table](README.md#questions-the-audit-must-answer): "Do clamped parts rotate and translate rigidly with the frame and post when the same bike moves between unloaded and rider-loaded states?"
3. **What is a load state?** In code it is fork `SagPct` alone. Physically it also includes post compression. The ShockStop's 12 mm setback is a loaded-state value for a part that also has an unloaded state. So the model can't represent Matt's *unsagged* Fenrir with the ShockStop installed, which the Fenrir check (T4) needs.

   *Ruling (Matt, 2026-11-23):* tire compression stays out of the model. It is close enough to identical across bikes that it doesn't matter.
4. **In which state was each target measured?** The default BRP comes from an unsagged post, on a source bike whose fork state isn't recorded. The default HandGrip comes from the idmatch report. A target with no load state can't be compared with a placement unambiguously. Stage 3 owns transfer, but Stage 1 needs a term for a target's reference state.
5. **One word, two meanings: "setback".** `SeatpostSpec.SetbackMm` is a post-local perpendicular offset of the rail cradle from the shaft axis. "BRP setback behind BB" (219 mm in `RIDER_FIT.md`) is a world horizontal distance. They're different quantities with different owners.
6. **`SitBones` vs SIT.** The code's `SitBones` landmark and the `SitBonesToGripRiseMm` comment ("sit-bone contact point") suggest anatomy. `RIDER_FIT.md` says the landmark is a saddle support reference (SIT) and not a pelvic location.
7. **What is measured at HandGrip and BRP?** `HandGrip` is the side-view projection of a 3D point on the bar, `GripInsetMm` in from the cut end, with `HandLateralMm` kept separately. BRP is a saddle-local point at the W70 station, whose height above the rails (`RailToBrpHeightMm` = 45 mm) is illustrative. Reproducing either on a real bike needs a physical definition of the point and its datum.
8. **`Leveled` names the wrong thing.** `LeveledReachMm` and similar values change with sag, but their names don't say which state they describe. Per Matt's note under item 1, "leveled" is the default and doesn't need saying. The state does.

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

## What a build is

This section uses the project's existing terms from [`AGENTS.md`](../../AGENTS.md):

- A **build** is one choice of every part plus one value for every setting. Its inputs are the code's `BikeSetup`.
- A **bike placement** (`BikePlacement`, `PlaceBike`, `/bike`) is where every part and landmark lands when a build is placed in a given state.

Earlier drafts of this note said "installation." In every case that word meant a build, so it is no longer used.

### Inputs of a build

This is the complete list of `BikeSetup` inputs at `b4a46e974e02`. A **part** is chosen from the catalog. A **setting** is adjusted on an installed part. A **fitting range** isn't physical: it bounds what fitting may choose.

| Group | Parts | Settings | Fitting ranges |
| --- | --- | --- | --- |
| Frame | frame and size (`FrameSetup`: chart values, head tube bore, insertion limits) | none | none |
| Headset | lower cup (`LowerCupSetup`); upper stack (`StemSetup.UpperStackMm`, fixed 11 mm) | none | none |
| Fork | fork (`ForkSpec`: static A2C, travel, rake, steerer) | sag (`ForkSetup.SagPct`) | none |
| Wheels | wheel radius, one for both (`WheelSetup.RadiusMm`) | none | none |
| Seatpost | seatpost (`SeatpostSpec`: length, insertion limits, setback, clamp length) | insertion (`InsertionMm`) | none |
| Saddle | saddle (rail-to-BRP and rail-to-SIT geometry, rail length) | rail offset (`RailOffsetMm`); rail angle (`RailPitchDeg`) | none |
| Stem | stem length and angle; steerer clamp height | spacers (`SpacersMm`) | spacer allowance (`MaxSpacersMm`); stem length range (`MinLengthMm`, `MaxLengthMm`) |
| Handlebar | bar (stock width, backsweep, upsweep, rise and its convention, bend start) | cut width (`CurrentWidthMm`); grip position (`GripInsetMm`); roll (`BarRollDeg`) | none |

Things a real bike has that `BikeSetup` lacks (examples, not a complete list): the Fenrir's Tuning Chip position (the catalog uses the middle chart only), separate front and rear radii, a seatpost's static state, a setback adapter such as the FAIR Drop Best, and the static position of a suspension post's head.

Two inputs don't fit the part/setting split cleanly:

- **Fork sag** is a suspension setup (air pressure) as well as a load state.
- **Stem length** is a part choice, but `ApplyFitting` treats it as continuous within the stem length range.

### What fitting may vary today

This is the complete list for each fitting operation at `b4a46e974e02`. Every input not listed is held at the submitted value.

| Operation | Varies continuously | Varies by discrete choice | Fits to |
| --- | --- | --- | --- |
| `ApplyFitting` (`/bike`) | insertion and rail offset; spacers and stem length | none | BRP target; HandGrip target |
| `FitPostSetback` | insertion and the post's setback (a hypothetical post) | none | BRP target, at a given rail offset |
| `FitBuilds` (per compare row) | insertion and rail offset (through `ApplyFitting`) | catalog stem (length and angle); spacer stack in steps; handlebar from the listed options; fork configuration (fork, sag, cup) from the listed options | BRP target, then a scored HandGrip match |
| `compare.Rank` (rows) | none | frame, size, fork, and fork sag (each travel's standard sag values) | ranks each row's best build |

No fitting operation varies the saddle model, seatpost model, wheel radius, lower cup (except through a fork option), upper stack, rail angle, bar roll, cut width, or grip position.

### What fitting may vary: Matt's rulings, 2026-11-23

Matt decided the role of every build input in matching a dynamic fit. Each input of a build has exactly one of the four roles below; this table is the complete assignment.

| Role | Inputs | Notes |
| --- | --- | --- |
| **Compared** (never fitted) | frame and size, with the wheel size that comes with the frame | What compare ranks. |
| **Fixed by the rider** (never fitted) | saddle; handlebar, including its cut width and grip position; dynamic saddle tilt; dynamic bar roll; sag of each suspension part | The handlebar is chosen once and applied to every frame. Because it never changes, hand width needs no separate model. |
| **Part choices fitting may make** | fork, from the frame's supported forks (only frames that support more than one); seatpost, from an option list unless the rider fixes one post; stem, from the catalog; spacer stack, in steps; lower cup, which follows from the fork | |
| **Settings fitting solves for** | insertion; rail offset; rail angle, solved so the dynamic saddle tilt matches; bar roll, solved so the dynamic bar roll matches; stem length, only where stems are hypothetical (`/bike`) | |

Rulings behind the table:

1. **Fork sag is a property of the fork.** Each fork class gets one sag value. Examples of classes: 100 mm XC, 120 mm XC, 130 mm trail. Fitting never chooses sag. The values per class are still to be chosen, and they must be seated sag (see the consequences of the static and dynamic ruling).
2. **The seatpost is a fitting choice, but can be fixed.** Fitting may choose the post from an option list. The rider must also be able to fix the fit to a single post: for example, a rider who needs the ShockStop, or a zero-offset post such as a dropper.
3. **The handlebar is not a fitting choice.** The rider picks one bar, and it is applied to all frames. This keeps bar roll meaningful across candidates: matching the dynamic roll of the same bar gives the hands the same angle. It also makes hand width a property of the bar, not a fit item.
4. **The fork varies only in limited cases.** Some frames support a range of suspension forks or a set of rigid forks. For those, fitting varies the fork among the frame's supported forks, each at its class sag.

Rail angle and bar roll are only nominally "solved": with the saddle and bar fixed, matching each dynamic angle fixes the setting. Static rail angle and static bar roll then differ per build by the frame's pitch change between states.

### Behavior changes these rulings imply

These rulings deliberately change behavior. They aren't defects, so they become slice candidates under [`WORKFLOW.md`](../WORKFLOW.md), not audit fixes:

- **Compare and fork sag.** Compare would place each fork at its class sag and stop trying each travel's standard sag values. Q132's sag score would no longer rank sag. It could become a check that the class sag lies in the manufacturer's range. This revises Q129, Q132, and Q134.
- **Seatposts.** Seatpost options would become a fitting choice, with a control to fix one post.
- **Handlebars.** Fitting would stop choosing among handlebars (`bar_options=all` in compare and `/bike`). This revises Q22–Q24 and Q101.
- **Angles.** Static and dynamic saddle tilt and bar roll would be converted on input and output (the case B defect).

## Proposed ruling: static and dynamic states (awaiting Matt's confirmation)

Matt set the direction on 2026-11-23: bicycle geometry is measured static and evaluated dynamic. The same day he chose the names **static** and **dynamic**, replacing "seated" (see [Names for the state pair](#names-for-the-state-pair)). The ruling is ready to record as a `Q#` once he confirms this text.

Definitions:

- **Static:** a build placed with no rider, every suspension part fully extended, both tires on level ground.
- **Dynamic:** a build placed with the rider seated in their riding position, at steady state, and each suspension part at the rider's seated sag for that part. If a standing posture is ever added, it is a second dynamic posture and gets an explicit qualifier. None is proposed now.
- Tire compression is ignored in both states (Matt's ruling above).

Rules:

1. **One build, two placements.** A build is the same in both states; only its placement differs. Between the static and dynamic placements of one build, every clamped part moves rigidly with the frame or with the post head. The only motions are suspension motions. For the parts bikegeo models, this is the complete list:
   - A suspension fork shortens, which pitches the frame.
   - A suspension seatpost moves its head relative to its shaft. On the ShockStop this is a translation: the source record says saddle pitch stays constant relative to the post's mounting orientation through travel.
2. **Rule 1 describes results, not method.** It constrains the static and dynamic placements of a finished build. It doesn't constrain how fitting finds that build. Fitting may adjust settings with the bike placed dynamically, or in an intermediate state that is neither, as long as the result is one build whose two placements obey rule 1.
3. **"Fit" covers both states.** A bike fitter's report is a set of static measurements, and it is commonly called the fit. When the state matters, say **static fit** or **dynamic fit**. A fit is a set of landmark positions and angles. Examples, not a complete list: BRP, HandGrip, saddle tilt, bar roll. Which items a fit includes is open.
4. **Matching aims at the dynamic fit.** Across candidate builds, the goal is the dynamic fit closest to the source bike's dynamic fit. Scoring measures closeness to that goal.
5. **The core workflow runs static → dynamic → static.**
   1. Start from the source bike's static fit, its build, and the rider's sag on each of its suspension parts.
   2. Place the source build dynamically to get its dynamic fit.
   3. For each candidate (its frame, size, and part choices, with each suspension part at the rider's sag), choose the settings fitting is allowed to vary (open; see [What fitting may vary today](#what-fitting-may-vary-today)) to reproduce that dynamic fit as closely as possible.
   4. Report the candidate's static fit, for setting it up in the workshop.

   One example of the idea, not the whole rule: the saddle might read static angle X on bike A and static angle Y on bike B, while the two dynamic angles are as close to equal as possible.
6. **Every target names its build and state.** A measured target is the static fit of a named build. A goal is a dynamic fit.

Consequences to carry into later stages:

- The default targets (Q53) are a static fit measured on Matt's ShockStop bike. The fork state of that bike is part of the Fenrir build facts.
- Q78's mixed-state caveat would be resolved, not just documented.
- Under rule 1, the world-held `RailPitchDeg` and `BarRollDeg` (case B above) are a defect whenever one build is placed in both states.
- **Seated sag is a physical premise to check.** The dynamic state is seated, so every sag value must be seated sag. Q132's fork ranges (`[15%, 20%]` for 100 mm, `[15%, 25%]` for 120 mm, `[20%, 25%]` for 130 mm) don't record whether they are seated or standing figures, and trail forks are often set up with the rider standing. If they are standing figures, the dynamic state needs its own fork sag. Q159's 20% post sag is seated (Redshift's manual identifies sag with the rider seated).

**Correction, 2026-11-23.** In chat, Claude claimed that the ShockStop linkage tips the saddle as the post compresses. That claim was wrong. The [source record](../../internal/catalog/seatpost_redshift_shockstop_pro.md) gives saddle pitch through travel as constant relative to the post's mounting orientation, as Matt said. The rail angle can therefore be clamped relative to the post shaft.

## Names for the state pair

**Chosen by Matt, 2026-11-23: static / dynamic.** This replaces static / seated, chosen earlier the same day. Matt wants a congruent pair: the same quantity, named once per state, as in `StaticBrp` / `DynamicBrp` or `StaticSaddleTiltDeg` / `DynamicSaddleTiltDeg`.

Reasons:

- A reader who sees `Static…` expects `Dynamic…` without having to recall a project-specific choice.
- The cycling industry already says "dynamic geometry" for geometry at sag.
- "Dynamic" covers fork sag and post compression under one word. "Seated" names a posture, and "sagged" reads oddly on rigid parts.
- "Static" already means riderless and fully extended in `ForkSpec.StaticA2cMm` and in Redshift's "7 mm static".

Collisions to define away in the glossary:

- In physics, a seated rider at sag is still in static equilibrium. In bikegeo, "dynamic" is a term of art for the steady seated state.
- In bike fitting, "dynamic fit" can mean assessing the rider while pedaling. In bikegeo it means the build's landmarks in the dynamic state.

Rejected pairs and why:

| Pair | Reason |
| --- | --- |
| static / seated | Asymmetric: a reader seeing `Static…` doesn't expect `Seated…`. |
| static / ridden | "Ridden" covers seated and standing. |
| unridden / ridden | "Unridden" can suggest a new, never-ridden bike. |
| unsagged / sagged | Reads oddly on rigid parts. |
| installed / ridden | The words name different things: a ridden bike is still a build. |
| unloaded / loaded | "Loaded" means carrying bags on bikepacking frames, which make up most of the catalog. |
| unweighted / weighted | Collides with scoring's `ScoreWeights` and "weighted score". |
| nominal / effective | "Effective" is already overloaded (next section). |

With this pair, users may not need a third term for the clamp setting. A build's settings are identified by its static fit, and its dynamic fit is computed from them. Code may still store a part-relative angle internally.

### Renames this implies (after the ruling)

- `ForkSetup.EffectiveA2cMm` is the sagged A2C, so it is the dynamic A2C. "Effective" also means something unrelated in `InsertionBounds.EffectiveMinInsertionMm` (the combined frame and post bound) and in ETT and effective seat tube angle.
- Drop the `Leveled…` prefix from readouts, since level is the default (item 1 note). Name the state instead wherever static and dynamic values differ. Name the unleveled chart attitude explicitly where it appears.
- `RailPitchDeg` and `BarRollDeg` become a static or dynamic saddle tilt and bar roll, depending on which the input is.

## Code audit and tests

Nothing has been committed yet. Once the static and dynamic ruling is confirmed, a committed test should check that changing only `SagPct` moves every saddle and cockpit landmark by one rigid rotation about `BB` plus a translation, with the saddle and bar angles changing by the frame's pitch change. Under current code that test would fail, and it would be committed disabled under the [defect procedure](README.md#defects-and-fixes).

## Session handoff

- **Conclusion so far.** The model defines world axes by the axle line and load state by fork sag alone, and it holds rail pitch and bar roll fixed to the world. As a result, it doesn't carry the saddle and bar rigidly when sag changes. On the Fenrir L with SID 100 at 20% sag, that misplaces BRP and HandGrip by about 1 mm and the saddle and bar angles by 0.92°. Matt agreed to separate the static and dynamic placements of one build, chose those names, and ruled tire compression out of the model. The note now lists every input of a build and what each fitting operation varies, and records Matt's rulings on the role of each input. The ruling text awaits his confirmation. Once it is confirmed, case B is a defect.
- **Evidence.** The code at `b4a46e974e02` (symbols listed under Scope, plus `ApplyFitting`, `FitPostSetback`, `FitBuilds`, and `compare.Rank`), and the CLI table above (**Observed**). Matt's direction, rulings, and choice of names are from chat on 2026-11-23.
- **Waiting on Matt.**
  1. Confirm the **static and dynamic ruling** text above, so it can be recorded as Q160 in `DESIGN.md`. The fitting-role rulings would be recorded with it.
  2. Choose the **sag value for each fork class**, as seated sag. Today's defaults per travel are 20% for 100 mm and 25% for 120 mm and 130 mm (`ForkTravel.DefaultSagPct`).
  3. Which items does a fit include, beyond BRP, HandGrip, saddle tilt, and bar roll? With the handlebar fixed, hand width no longer needs to be one.
  4. Should **world axes** come from the floor, with front and rear radii given separately? This is part of the static state's definition.
  5. For the Fenrir check, how should the **static ShockStop** be represented? Options: (a) measure with a rigid post installed; (b) add a static state for the post, from Redshift's published offsets at rest (see the [source record](../../internal/catalog/seatpost_redshift_shockstop_pro.md)); (c) measure the post's static head offset directly.
  6. Build facts for the Fenrir: see the [Fenrir measurement note](fenrir-measurements.md). They include the fork's state when the idmatch BRP was measured.
- **Next question.** "With static and dynamic confirmed, what does the model need to place one build in both states? List each suspension part's motion (fork length, post linkage translation), its inputs, and where each comes from."
