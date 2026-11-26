# Stage 1: Definitions, coordinates, and load states

Status: In progress, started 2026-11-23. Code revision `b4a46e974e02`. No Go files have changed since `b961e8818121`. Names in the glossary are the current code names. Q169 defines the static and dynamic placements of one build.

## Scope and premises

Stage 1 describes what the model's quantities mean physically, and in which axes. The bike is placed as one rigid frame on two wheels, saddle, and grip. It is then compared with the same build unloaded and at fork sag. No upstream stage is relied on.

Symbols examined: `PlaceBike`, `PlaceFrameset`, `placeFramesetAtPitch`, `ForkSetup.EffectiveA2cMm`, `PlaceSeatpost`, `PlaceSaddle`, `PlaceStem`, `PlaceHandlebar`, `pose.Pose.Face`/`Turn`/`Translate`, `catalog.Seatposts`, `catalog.DefaultSaddle`, `catalog.DefaultBrpTarget`, `catalog.DefaultGripTarget`.

## How the model places a bike today

The construction below summarizes the code. Each item is **Tested** by the existing placement tests, or follows directly from the code.

1. **World axes come from the axle line.** `PlaceFrameset` builds the frame at 0° pitch, measures the angle of the rear-axle → front-axle line, and rebuilds it rotated by that angle about `BB`. The world origin is `BB`, `+X` points forward along the axle line, and `+Y` points up. There is one `WheelSetup.RadiusMm`, so the ground line sits one radius below both axles and is parallel to the axle line. "Leveled" in code means *axle line horizontal*.
2. **The frame chart is a different attitude.** `FrameSetup` holds chart values (reach, stack, HTA, STA, BB drop) in the chart's *unpitched* attitude, which levels the axles only with the chart's basis fork and cup. An installed fork pitches the frame. Example: Fenrir L with SID 100 (506 × 44) unloaded pitches +1.08° (nose up) from the chart. The `Leveled…` readouts (`LeveledReachMm`, `LeveledSeatTubeAngleDeg`, and so on) are installed values in world axes, and they're what a tape measure would find.
3. **Load state is a single number, the fork's sag.** `SagPct` shortens the fork's effective A2C by `travel × sag`, and the frame re-levels. Nothing else changes with load: tires don't compress, there's no rear suspension, and the seatpost has no unloaded and loaded pair.
4. **Most parts ride rigidly with the frame.** The seatpost extends along the seat axis, and its `SetbackMm` is perpendicular to that axis. The stem starts from the steerer axis, and `AngleDeg` is measured from the perpendicular to the steerer. Rise and sweep are fixed relative to the bar clamp. All of these rotate with frame pitch.
5. **Saddle tilt and bar roll are set in the leveled world frame.** After pitching the frame so the axles are level for the given fork A2C and sag, `PlaceSaddle` uses `Face(RailPitchDeg)` and `PlaceHandlebar` uses `Face(−BarRollDeg)`. That matches dynamic fitting: for any candidate build at its seated sag, the rider sets the saddle tilt and handlebar roll to their desired world angles at that pitched riding attitude before fitting insertion, rail offset, stem, and spacers.
6. **The ShockStop is modeled in one state.** `redshift_shockstop_pro` has a fixed 12 mm `SetbackMm`, which is owner-reported *when sagged* (Q28). It has no vertical compression. Q78 already notes that the default BRP target was measured with this post unsagged.
7. **Targets are world points relative to BB, with no load state attached.** `DefaultBrpTarget` (−219, 713.1) and `DefaultGripTarget` (481, 726) are compared directly with placed landmarks in whatever state the placement used.

## Ambiguities found

Each item is a question about meaning, not yet about names.

1. **What defines "level"?** The model uses one wheel radius for both ends. On a level floor the two axles therefore have the same height. From BB, +X is horizontally forward and +Y is vertically up. Keep the shared radius.

   *Note (Matt, 2026-11-23):* a level frame is the normal, expected case, so saying a frame is "leveled" is usually unnecessary, especially in the UI. The unleveled state is the one that needs calling out. It is degenerate and needs correcting. In code, the unleveled state is the chart's own attitude, used by `PlaceUnpitchedFrameset` and the chart values in `FrameSetup`.
2. **In which state are the saddle-angle and bar-roll orientations defined?** During a dynamic fit attempt, `PlaceBike` pitches the frame so the axles are level at the fork's seated sag, and then sets the saddle tilt (`SaddleSetup.RailPitchDeg`) and handlebar roll (`HandlebarSetup.BarRollDeg`) relative to world horizontal in that dynamic state. Once a build is fitted and clamped, unloading that same build to the static workshop state (`sag=0`) rotates the frame, stem, and seatpost by `Δpitch = pitch_static − pitch_dynamic` about `BB`, carrying the clamped saddle and handlebar with them (Q163, Q169).
3. **What is a load state?** In code it is fork `SagPct` alone. Physically it also includes post compression. The ShockStop's 12 mm setback is a loaded-state value for a part that also has an unloaded state. So the model can't represent Matt's *unsagged* Fenrir with the ShockStop installed, which the Fenrir check (T4) needs.

   *Ruling (Matt, 2026-11-23):* tire compression stays out of the model. It is close enough to identical across bikes that it doesn't matter.
4. **In which state was each target measured?** The default BRP comes from an unsagged post, on a source bike whose fork state isn't recorded. The default HandGrip comes from the idmatch report. A target with no load state can't be compared with a placement unambiguously. Stage 3 owns transfer, but Stage 1 needs a term for a target's reference state.
5. **Seatpost setback and BRP.x (Q171).** Measure seatpost setback perpendicular to the post shaft, from its axis to the rail clamp center. Use BRP.x for the horizontal saddle coordinate relative to BB. The fit report’s 219 mm behind BB corresponds to BRP.x = −219 mm.
6. **`SitBones` vs SIT.** The code's `SitBones` landmark and the `SitBonesToGripRiseMm` comment ("sit-bone contact point") suggest anatomy. `RIDER_FIT.md` says the landmark is a saddle support reference (SIT) and not a pelvic location.
7. **What is measured at HandGrip and BRP?** `HandGrip` is the side-view projection of a 3D point on the bar, `GripInsetMm` in from the cut end, with `HandLateralMm` kept separately. BRP is a saddle-local point at the W70 station, whose height above the rails (`RailToBrpHeightMm` = 45 mm) is illustrative. Q162 defines the workshop points: BRP is the top of the saddle on its centerline at W70. HB is the center of the handlebar tube 8 cm in from its cut end.
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
| `RailPivot` | Post head's clamp point | Pose | Post-local coordinates | Post, insertion | Calculated | Saddle |
| `RailPitchDeg` | Rail heading in world axes | Angle | **World** (not the post) | Input | Input | Saddle landmarks |
| `RailOffsetMm` | Saddle slide along rails; positive is rearward | Scalar | Rails | Adjustment | Input or fitting | Saddle landmarks |
| `BRP` | Saddle landmark at W70, `RailCenterToBrpMm` forward of and `RailToBrpHeightMm` above rail center | Landmark (stored as Pose) | Saddle-local, placed in world | Everything upstream | Saddle data (height illustrative) | BRP target, gates, scores |
| `SitBones` | Saddle support landmark (SIT under Q170), 80 mm behind and 10 mm above BRP | Landmark (stored as Pose) | Saddle-local | Everything upstream | Owner-measured saddle | Rise readout |
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

Things a real bike has that `BikeSetup` lacks (examples, not a complete list): the Fenrir's Tuning Chip position (the catalog uses the middle chart only), a seatpost's static state, a setback adapter such as the FAIR Drop Best, and the static position of a suspension post's head.

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

Matt decided the role of every build input in matching a dynamic fit. The table assigns roles to the choices available to the rider and fitting. Placement also uses the fixed angle assumptions described below.

| Role | Inputs | Notes |
| --- | --- | --- |
| **Compared** (never fitted) | frame and size, with the wheel size that comes with the frame | What compare ranks. |
| **Fixed by the rider** (never fitted) | saddle; handlebar, including its cut width (Q162 fixes the HB inset at 8 cm); sag of each suspension part | The handlebar is chosen once and applied to every frame. Because it never changes, hand width needs no separate model. |
| **Part choices fitting may make** | fork, from the frame's supported forks (only frames that support more than one); seatpost, from an option list unless the rider fixes one post; stem, from the catalog; spacer stack, in steps; lower cup, which follows from the fork | |
| **Settings fitting solves for** | insertion; rail offset; stem length, only where stems are hypothetical (`/bike`) | |

Rulings behind the table:

1. **Fork sag is a property of the fork.** Each fork class gets one sag value. Examples of classes: 100 mm XC, 120 mm XC, 130 mm trail. Fitting never chooses sag. Use seated sag of 15% for 100 mm, 20% for 120 mm, and 25% for 130 mm (Q161).
2. **The seatpost is a fitting choice, but can be fixed.** Fitting may choose the post from an option list. The rider must also be able to fix the fit to a single post: for example, a rider who needs the ShockStop, or a zero-offset post such as a dropper.
3. **The handlebar is not a fitting choice.** The rider picks one bar, and it is applied to all frames. Using the same bar at the same fixed dynamic roll gives the hands the same angle across candidates. It also makes hand width a property of the bar, not a fit item.
4. **The fork varies only in limited cases.** Some frames support a range of suspension forks or a set of rigid forks. For those, fitting varies the fork among the frame's supported forks, each at its class sag.

**Dynamic angle assumptions (Matt, 2026-11-23).** Place the saddle level and the handlebar at its catalog orientation (0° additional roll, Q163) in the dynamic state. Both angles affect landmark positions slightly. The model uses them internally, with no configuration, fitting search, or UI readout for either angle. In the static placement of that same build, the saddle follows the post head and the bar follows the stem.

### Behavior changes these rulings imply

These rulings deliberately change behavior. They aren't defects, so they become slice candidates under [`WORKFLOW.md`](../WORKFLOW.md), not audit fixes:

- **Compare and fork sag.** Compare would place each fork at its class sag and stop trying each travel's standard sag values. Q132's sag score would no longer rank sag. It could become a check that the class sag lies in the manufacturer's range. This revises Q129, Q132, and Q134.
- **Seatposts.** Seatpost options would become a fitting choice, with a control to fix one post.
- **Handlebars.** Fitting would stop choosing among handlebars (`bar_options=all` in compare and `/bike`). This revises Q22–Q24 and Q101.
- **Angles and static/dynamic conversion.** Dynamic placement continues to set the fork A2C at sag, pitch the frame so the axles are level, and set the dynamic saddle tilt and 0° bar roll in that leveled world frame (`Face`). Static placement of that finished build unloads the suspension and rotates the clamped saddle and bar with the frame pitch change (`Δpitch = pitch_static − pitch_dynamic`).

## Static and dynamic states

Matt set the direction on 2026-11-23: bicycle geometry is measured static and evaluated dynamic. The same day he chose the names **static** and **dynamic**, replacing "seated" (see [Names for the state pair](#names-for-the-state-pair)). Matt confirmed the one-build placement rule in Q169.

Definitions:

- **Static:** a build placed with no rider, every suspension part fully extended, both tires on level ground.
- **Dynamic:** a build placed with the rider seated in their riding position, at steady state, and each suspension part at the rider's seated sag for that part. If a standing posture is ever added, it is a second dynamic posture and gets an explicit qualifier. None is proposed now.
- Tire compression is ignored in both states (Matt's ruling above).

Rules:

1. **One build, two placements.** A build is the same in both states; only its placement differs. Between the static and dynamic placements of one build, every clamped part moves rigidly with the frame or with the post head. The only motions are suspension motions. For the parts bikegeo models, this is the complete list:
   - A suspension fork shortens, which pitches the frame.
   - A suspension seatpost moves its head relative to its shaft. On the ShockStop this is a translation: the source record says saddle pitch stays constant relative to the post's mounting orientation through travel.
2. **Rule 1 describes results, not method.** It constrains the static and dynamic placements of a finished build. It doesn't constrain how fitting finds that build. Fitting may adjust settings with the bike placed dynamically, or in an intermediate state that is neither, as long as the result is one build whose two placements obey rule 1.
3. **BRP and HB each have a static and a dynamic position (Matt's ruling, 2026-11-23).** BRP is the saddle reference point. HB is the physical handgrip point (`HandGrip` in code). For each landmark, the static position is what the user measures on the riderless bike. The model derives its dynamic position with the rider seated. Fixed dynamic saddle tilt and bar roll affect these positions internally.
4. **Fitting matches dynamic positions.** The source bike’s dynamic BRP and HB are the targets. Fitting seeks settings that bring the candidate’s dynamic BRP and HB as close as possible to those targets, and the fit score reflects that comparison.
5. **Enter either a dynamic target directly or static measurements with a source build (Q160).** If the dynamic BRP and HB are known, enter those two points on `/` and proceed to step 3. For static measurements, compare links to `/bike`.
   1. Start from the source bike's static fit, its build, and the rider's sag on each of its suspension parts.
   2. On `/bike`, derive the dynamic target and carry both points into compare.
   3. For each candidate (its frame, size, and part choices, with each suspension part at the rider's sag), choose the settings allowed by [Matt’s fitting rulings](#what-fitting-may-vary-matts-rulings-2026-11-23) to reproduce that dynamic fit as closely as possible.
   4. Open the exact fitted candidate in `/bike` with the target. Show its achieved dynamic fit and derive its static workshop measurements from that build, including when the candidate misses the target.

   For example, a rigid-post candidate can match the seated BRP of a suspension-post source bike even when their static BRP measurements differ.
6. **Make the state and purpose clear in the UX (Matt's ruling, 2026-11-23).** Identify source measurements and candidate workshop values as static. Identify fit targets, candidate positions used in scoring, and differences shown with the score as dynamic. Wherever both appear, label them so the user can tell which values to measure and which values explain the score.

Consequences to carry into later stages:

- The default targets (Q53) are a static fit measured on Matt's ShockStop bike. The fork state of that bike is part of the Fenrir build facts.
- Q78's mixed-state caveat would be resolved, not just documented.
- During dynamic fitting, `PlaceBike` sets fork A2C at sag, pitches the frame so the axles are level, and sets saddle tilt and bar roll in world axes (`Face`). When converting a single fitted build between dynamic and static states (rule 1, Q169), the clamped saddle and handlebar rotate with the frame's pitch change (`Δpitch = pitch_static − pitch_dynamic`) rather than re-leveling to the floor at `sag=0`.
- **Seated fork sag (Q161).** Across all catalog fork variants, dynamic A2C is 491 mm at 100 mm travel, 506–512 mm at 120 mm, and 508–513.5 mm at 130 mm. Every Fox family offered at both 120 and 130 mm gains 1.5 mm. The two classes overlap across families.

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

Placement can derive the static angles internally from the dynamic assumptions and the motion of the clamped parts.

### Renames this implies (after the ruling)

- `ForkSetup.EffectiveA2cMm` is the sagged A2C, so it is the dynamic A2C. "Effective" also means something unrelated in `InsertionBounds.EffectiveMinInsertionMm` (the combined frame and post bound) and in ETT and effective seat tube angle.
- Drop the `Leveled…` prefix from readouts, since level is the default (item 1 note). Name the state instead wherever static and dynamic values differ. Name the unleveled chart attitude explicitly where it appears.
- For internal angle calculations, identify the state whenever it affects the meaning of a value.

## Code audit and tests

During dynamic placement and fitting, `PlaceBike` already behaves as intended: it sets the fork's dynamic A2C, pitches the frame about the rear axle so the axles are level, sets the saddle tilt (`Face(RailPitchDeg)`) and handlebar roll (`Face(−BarRollDeg)`) in that leveled world frame, and fits insertion, rail offset, stem, and spacers.

When Phase 2 adds the static ↔ dynamic conversion for a single clamped build (Q160, Q169), its tests will verify that unloading a fitted build from dynamic (`SagPct > 0`) to static (`SagPct = 0`) reverses any seatpost suspension displacement and rotates the frame-clamped saddle and cockpit landmarks about `BB` by `Δpitch = pitch_static − pitch_dynamic`.

## Session handoff

- **Conclusion so far.** In dynamic placement, `PlaceBike` sets fork A2C at sag, levels the axle line, and sets saddle tilt and bar roll in world axes (`Face`) before varying the fit settings. Matt ruled **Q159–Q172** in [`DESIGN.md`](../DESIGN.md), settling the static/dynamic state pair and unloading rule for a single clamped build (Q169), Redshift ShockStop static (`7 mm` setback) vs. dynamic (`.Translate(-6, +5)` at 20% sag) geometry in seatpost coordinates (Q159), target entry and workshop readout flow (Q160), seated fork sag per travel class (Q161), physical BRP and HB landmarks (Q162), fixed 0° dynamic bar roll (Q163), rail-position scoring at +5° on the SQlab 610 (Q164–Q165), workshop proximity warnings (Q166–Q167), height-first bounded saddle fitting (Q168), `SIT` / `BRP.x` terminology (Q170–Q171), and `brpToHbOffset` saddle-to-cockpit coupling (Q172). The turtle-graphics specification for static ↔ dynamic placement and transfer is recorded in [`brp-transfer.md`](brp-transfer.md).
- **Next blocking task (Phase 1, Stage 2):**
  - **Stage 2 forward placement & Fenrir check** (verifying forward placement kinematics in `02-forward-placement.md` and measuring Matt's physical static Fenrir in [`fenrir-measurements.md`](fenrir-measurements.md) to compare against the model's static placement, T4). Complete Stage 2 before planning the Phase 2 (`Q159–Q172`) behavior-changing implementation slices.
