# Fenrir physical check

Status: Draft, 2026-11-23. The procedure depends on the Stage 1 static and dynamic ruling and the open world-axes question in [`01-definitions-and-states.md`](01-definitions-and-states.md). No measurements have been taken yet.

Purpose: compare Matt's own Otso Fenrir, measured static, with the model's static placement of the same build (T4 in the [README](README.md#questions-the-audit-must-answer)).

## 1. Installed build

Matt fills in the **Installed** column. Model defaults are what `bikegeo bike 'frame=otso-fenrir-stainless&size=L'` uses at `b4a46e974e02`, and they're shown only to make the list concrete. Facts already confirmed in [`RIDER_FIT.md`](../RIDER_FIT.md) are filled in.

| Input | Model default | Installed | Source / how known |
| --- | --- | --- | --- |
| Frame size | L | | |
| Tuning Chip position | middle (the catalog transcribes only this chart) | | |
| Fork model and variant | `lithic_carbon_mountain-495x44` (rigid) | | |
| Fork static A2C, rake | 495, 44 mm | | measure A2C if possible |
| Lower headset cup | EC44, 12 mm | | |
| Upper stack (top cup + cover) | 11 mm (illustrative) | | measure |
| Spacers below stem | 20 mm | | measure |
| Stem length, angle | 60 mm, −6° | | |
| Handlebar | `persuader20_800` | | |
| Bar width (cut), grip inset | 800 mm, 70 mm | | measure |
| Bar roll | 0° | | see procedure |
| Front tire, rear tire | one radius, 372 mm | | record both |
| Seatpost | Redshift ShockStop PRO Endurance | Redshift ShockStop PRO Endurance, spring and elastomer | `RIDER_FIT.md` |
| Setback adapter | none modeled | FAIR Drop Best, rearward orientation | `RIDER_FIT.md` |
| Post insertion | 175 mm | | measure exposed length |
| Saddle | SQlab 610 Ergolux Active 2.1 | SQlab 610 ERGOLUX active 2.1, 14 cm | `RIDER_FIT.md` |
| Saddle rail position | centered | | measure clamp position on the rails |
| Saddle rail angle | 0° | | digital level, static state |

The Drop Best adapter isn't represented in the model's default post. How to model it is part of the open Stage 1 question on the static ShockStop.

## 2. Procedure (draft)

Static state (per the proposed Stage 1 ruling): bike standing on a level floor, no rider, fork and post fully extended, tires at riding pressure. Record the floor's levelness and the tire pressures.

Datum and axes: `X` is horizontal along the bike's centerline, positive forward. `Y` is height above the floor. Measure BB-relative coordinates as differences from the BB center.

Method: snap a chalk line or lay a straightedge on the floor through both tire contact patches. Drop a plumb bob from each landmark to the floor and measure its position along the line. Measure heights from the floor with a tape or height gauge.

| # | Measurement | Model counterpart | Expected precision (to estimate) |
| --- | --- | --- | --- |
| M1 | Front and rear axle heights above floor | wheel radius per wheel; axle-line slope | |
| M2 | BB center height above floor | axle height − `LeveledBbDropMm` | |
| M3 | Wheelbase (axle to axle, horizontal) | `WheelbaseMm` (unpublished for the Fenrir) | |
| M4 | Rear axle to BB, horizontal | `RearCenterMm` | |
| M5 | Head tube top (top of the upper cover) relative to BB, X and Y | `HeadTubeTop` advanced by the upper stack | |
| M6 | BRP relative to BB, X and Y | `Saddle.BRP` | |
| M7 | Saddle rail angle | static saddle tilt (`RailPitchDeg` today) | |
| M8 | HandGrip relative to BB, X and Y | `Handlebar.HandGrip` | |

Open definitions before M6 and M8 can be measured (Stage 1 item 7):

- **BRP** needs a physical point on the saddle: the top surface on the centerline at the W70 station. Measuring the saddle's height above the rails at W70 also replaces the illustrative 45 mm `RailToBrpHeightMm`.
- **HandGrip** needs a physical point on the bar: the bar's axis, `GripInsetMm` in from the cut end. RIDER_FIT confirms that the idmatch HandGrip is the physical grip location, but not which point of the grip.

## 3. Comparison

To be filled after measurement: each measured value, the model's prediction for the recorded installed build, the difference, and the combined uncertainty. A difference larger than the uncertainty is a finding, classified as a model defect, a catalog value, an installed input, or a measurement error.
