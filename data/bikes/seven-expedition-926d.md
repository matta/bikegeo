# Seven Cycles Expedition 926d — Geometry Chart & Fork Analysis

## Context & Issue Description

On the [Seven Cycles Expedition 926d](file:///Users/matt/src/bikegeo/Bikepacking%20Mountain%20Drop%20Bar%20_%20Seven%20Cycles.html) webpage, there are direct contradictions regarding the fork specification:

1. **Product Overview List**:
   > *"Fork: Enve Mountain, carbon, or 100 mm suspension fork. 100 mm hub spacing."*
2. **Feature Specifications Table**:
   > *"Fork: Enve Mountain fork, carbon, Non-Boost 100 mm axle width. Optimized for 29 x 2.6\" tires (at 5.5 cm rake setting). Maximum 29 x 3.0\". Pack mounts included."*
3. **Rider-Ready Complete Bike Order Form**:
   > *"<p><strong>Note:</strong> This frame is spec'ed with a ENVE Mountain boost</p>"* (paired with 110/115 mm front and 148 mm rear Boost wheelsets).

This directly mirrors the issue identified on the flat-bar model, [Expedition 926f](file:///Users/matt/src/bikegeo/data/bikes/seven-expedition-926f.md). While Seven configures and ships complete complete builds with the **Boost ENVE Mountain fork** (490/492 mm Axle-to-Crown), the published Rider-Ready geometry chart was mathematically drafted in CAD around the legacy non-Boost **472 mm Axle-to-Crown (A2C)** fork (`enve_mountain_472`) paired with an external lower headset cup (10 mm EC44 spacer).

This document provides a mathematical analysis proving that:
1. **The published geometry chart is mathematically incompatible with a 492 mm A2C fork.**
2. **The published geometry chart was drafted around a 472 mm A2C fork.**
3. Complete bikes physically built with the 492 mm Boost fork kinematically pitch backward by ~1.0°, resulting in an actual geometry significantly slacker, taller, and shorter than the published chart.

---

## Published Specifications: Standard Fit S 58cm

Published specifications from [Seven Cycles Expedition 926d](file:///Users/matt/src/bikegeo/Bikepacking%20Mountain%20Drop%20Bar%20_%20Seven%20Cycles.html) for size **S 58cm**:

| Parameter | Published Value | Notes / Units |
| :--- | :--- | :--- |
| **Size** | 58.0 cm | Standard Fit ("S") |
| **Stack (BB-HT)** | 63.6 cm (636.0 mm) | Vertical distance from BB center to top-center of head tube |
| **Reach (BB-HT)** | 39.0 cm (390.0 mm) | Horizontal distance from BB center to top-center of head tube |
| **Head Tube Angle (HTA)** | 70.75° | `cos(70.75°) = 0.329691`, `sin(70.75°) = 0.944093` |
| **Seat Tube Angle (STA)** | 73.5° | Published nominal angle |
| **Head Tube Length (HTL)** | 12.1 cm (121.0 mm) | Physical length of head tube |
| **Bottom Bracket Drop** | 8.5 cm (85.0 mm) | Vertical distance of BB below axle line |
| **Front Center** | 63.9 cm (639.0 mm) | Horizontal distance from BB center to front axle center |
| **Chainstay Length** | 45.3 cm (453.0 mm) | Axle to BB center |
| **Fork Rake** | 5.5 cm (55.0 mm) | Seven spec note: "at 5.5 cm rake setting" |
| **Headset Cup Standard** | EC44 Lower | External cup on 44 mm titanium head tube adds 10.0 mm stack in CAD |

---

## Mathematical Proof 1: Master Stack Equation

Stack is the vertical distance from the bottom bracket center to the center of the top of the head tube. In bicycle frame coordinate geometry:

```
Stack = BB_Drop - Rake * cos(HTA) + (Fork_A2C + Headset_Cup + HTL) * sin(HTA)
```

For the S 58cm frame:
- `BB_Drop` = 85.0 mm
- `Rake * cos(70.75°)` = 55.0 mm * 0.329691 = **18.133 mm**
- `Headset_Cup` = 10.0 mm (EC44 lower external cup)
- `HTL` = 121.0 mm

### Evaluation with Seven's Boost Fork (492 mm Boost ENVE)
With `Fork_A2C = 492.0 mm`:
- Structural A2C = 492.0 mm + 10.0 mm = **502.0 mm**
- Total length along steering axis = 502.0 mm + 121.0 mm = **623.0 mm**
- Vertical rise along steering axis = 623.0 mm * sin(70.75°) = 623.0 mm * 0.944093 = **588.170 mm**
- Calculated Stack = 85.0 mm - 18.133 mm + 588.170 mm = **655.04 mm (~655.0 mm)**
- Published Stack = **636.0 mm**

```
Discrepancy: 655.04 mm - 636.0 mm = +19.04 mm (~19.0 mm)
```

Even if evaluated with an impossible 0.0 mm headset cup (zero-stack / flush bearing):
- Total length along steering axis = 492.0 mm + 121.0 mm = 613.0 mm
- Calculated Stack = 85.0 mm - 18.133 mm + 613.0 mm * 0.944093 = **645.60 mm**

```
Zero-Cup Discrepancy: 645.60 mm - 636.0 mm = +9.60 mm (~9.6 mm)
```

### Evaluation with Reverse-Engineered Fork (`enve_mountain_472`)
With `Fork_A2C = 472.0 mm`:
- Structural A2C = 472.0 mm + 10.0 mm = **482.0 mm**
- Total length along steering axis = 482.0 mm + 121.0 mm = **603.0 mm**
- Vertical rise along steering axis = 603.0 mm * sin(70.75°) = 603.0 mm * 0.944093 = **569.288 mm**
- Calculated Stack = 85.0 mm - 18.133 mm + 569.288 mm = **636.16 mm (~636.2 mm)**
- Published Stack = **636.0 mm**

```
Discrepancy: 636.16 mm - 636.0 mm = +0.16 mm (within 0.2 mm rounding precision)
```

### Inversion: Solving for Exact Implied Fork A2C
Rearranging the Master Stack Equation to solve for the structural A2C required to produce 636.0 mm Stack:

```
Structural_A2C = (Stack - BB_Drop + Rake * cos(HTA)) / sin(HTA) - HTL
Structural_A2C = (636.0 - 85.0 + 18.133) / 0.944093 - 121.0 = 481.84 mm
Fork_A2C       = Structural_A2C - 10.0 mm (EC44 cup) = 471.84 mm (~472 mm)
```

---

## Mathematical Proof 2: Front Center Equation

Front Center is the horizontal distance from the bottom bracket center to the front axle center:

```
Front_Center = Reach + (Fork_A2C + Headset_Cup + HTL) * cos(HTA) + Rake * sin(HTA)
```

For the S 58cm frame:
- `Reach` = 390.0 mm
- `Rake * sin(70.75°)` = 55.0 mm * 0.944093 = **51.925 mm**
- `Headset_Cup` = 10.0 mm
- `HTL` = 121.0 mm

### Evaluation with Seven's Boost Fork (492 mm Boost ENVE)
- Total length along steering axis = 492.0 mm + 10.0 mm + 121.0 mm = 623.0 mm
- Horizontal projection along steering axis = 623.0 mm * cos(70.75°) = 623.0 mm * 0.329691 = **205.397 mm**
- Calculated Front Center = 390.0 mm + 205.397 mm + 51.925 mm = **647.32 mm (~647.3 mm)**
- Published Front Center = **639.0 mm** (63.9 cm)

```
Discrepancy: 647.32 mm - 639.0 mm = +8.32 mm (~8.3 mm)
```

### Evaluation with Reverse-Engineered Fork (`enve_mountain_472`)
- Total length along steering axis = 472.0 mm + 10.0 mm + 121.0 mm = 603.0 mm
- Horizontal projection along steering axis = 603.0 mm * cos(70.75°) = 603.0 mm * 0.329691 = **198.804 mm**
- Calculated Front Center = 390.0 mm + 198.804 mm + 51.925 mm = **640.73 mm (~640.7 mm)**
- Published Front Center = **639.0 mm** (63.9 cm)

```
Discrepancy: 640.73 mm - 639.0 mm = +1.73 mm
```

*(Note: If evaluated at the 52.0 mm rake position of the ENVE flip-chip, Calculated Front Center is **637.49 mm**, a discrepancy of only -1.51 mm).*

---

## Chart-Wide Verification (All 24 Sizes)

The tight convergence to a 472 mm A2C fork is consistent across the entire Rider-Ready chart (all 12 Standard Fit sizes and all 12 Endurance Fit sizes), confirming this is not an artifact of an isolated size:

| Size | HTA | HTL (mm) | Stack (mm) | Reach (mm) | BB Drop (mm) | Published FC (mm) | Implied Fork A2C (mm) | Calc FC @ 472mm (mm) | ΔFC (mm) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **S 52cm** | 69.25° | 116 | 630 | 365 | 90 | 624.0 | **472.29** | 628.30 | +4.30 |
| **S 54cm** | 69.75° | 116 | 631 | 374 | 89 | 629.0 | **472.00** | 632.58 | +3.58 |
| **S 55cm** | 70.25° | 117 | 633 | 378 | 88 | 629.0 | **471.81** | 632.18 | +3.18 |
| **S 56cm** | 70.50° | 119 | 635 | 382 | 87 | 631.0 | **471.82** | 634.46 | +3.46 |
| **S 57cm** | 70.50° | 120 | 635 | 386 | 86 | 636.0 | **471.88** | 638.80 | +2.80 |
| **S 58cm** | 70.75° | 121 | 636 | 390 | 85 | 639.0 | **471.84** | 640.73 | +1.73 |
| **S 59cm** | 71.00° | 130 | 646 | 398 | 85 | 646.0 | **472.26** | 649.25 | +3.25 |
| **S 60cm** | 71.00° | 143 | 656 | 408 | 83 | 659.0 | **471.95** | 663.48 | +4.48 |
| **S 61cm** | 71.00° | 153 | 666 | 412 | 83 | 666.0 | **472.53** | 670.74 | +4.74 |
| **S 62cm** | 71.00° | 168 | 678 | 415 | 81 | 674.0 | **472.34** | 678.62 | +4.62 |
| **S 63cm** | 71.00° | 182 | 691 | 421 | 81 | 684.0 | **472.09** | 689.18 | +5.18 |
| **S 65cm** | 71.00° | 212 | 717 | 430 | 79 | 701.0 | **471.70** | 707.95 | +6.95 |
| **E 52cm** | 69.50° | 115 | 630 | 371 | 90 | 626.0 | **472.07** | 631.59 | +5.59 |
| **E 54cm** | 69.50° | 120 | 634 | 379 | 89 | 636.0 | **472.41** | 641.34 | +5.34 |
| **E 55cm** | 70.00° | 124 | 639 | 382 | 88 | 636.0 | **472.38** | 640.95 | +4.95 |
| **E 56cm** | 70.25° | 128 | 643 | 386 | 87 | 639.0 | **472.50** | 643.89 | +4.89 |
| **E 57cm** | 70.25° | 136 | 649 | 388 | 86 | 644.0 | **471.93** | 648.60 | +4.60 |
| **E 58cm** | 70.50° | 143 | 656 | 390 | 85 | 646.0 | **472.22** | 650.47 | +4.47 |
| **E 59cm** | 70.75° | 152 | 666 | 398 | 85 | 654.0 | **472.61** | 658.95 | +4.95 |
| **E 60cm** | 70.75° | 165 | 676 | 408 | 83 | 666.0 | **472.33** | 673.23 | +7.23 |
| **E 61cm** | 70.75° | 176 | 686 | 412 | 83 | 674.0 | **471.92** | 680.86 | +6.86 |
| **E 62cm** | 70.75° | 191 | 698 | 415 | 81 | 681.0 | **471.75** | 688.81 | +7.81 |
| **E 63cm** | 70.75° | 204 | 711 | 421 | 81 | 691.0 | **472.52** | 699.09 | +8.09 |
| **E 65cm** | 70.75° | 234 | 737 | 430 | 79 | 709.0 | **472.18** | 717.98 | +8.98 |

Across all 24 sizes:
- **Mean implied fork A2C**: **472.14 mm**
- **Min / Max range**: **471.70 mm to 472.61 mm** (total spread: **0.91 mm**)

---

## Root Cause Analysis

1. **Two Generations of ENVE Mountain Forks**:
   - **Generation 1 (Non-Boost 15x100 mm)**: Axle-to-Crown of **470 mm** (44 mm rake position) or **472 mm** (52/55 mm rake position).
   - **Generation 2 (Boost 15x110 mm)**: Axle-to-Crown of **490 mm** (44 mm rake position) or **492 mm** (52 mm rake position).
2. **CAD Template Disconnect**:
   Seven specifies in their order configurator that complete bikes are spec'ed with the Boost ENVE Mountain fork (492 mm A2C), but the CAD model used to draft the published geometry chart was built around the Generation 1 non-Boost fork (472 mm A2C) with an EC44 lower headset cup. Seven never updated the published Rider-Ready geometry chart to reflect the taller 492 mm fork.

---

## Physical Dynamics of Complete Builds

Because complete builds ship with the 492 mm Boost fork (+20.0 mm taller than the CAD model), installing this fork lifts the front axle and rotates the frame rearward around the rear axle:

- Front axle vertical lift: `ΔY = 20.0 mm * sin(70.75°) = 18.88 mm`
- Rearward rotation angle: `θ = arcsin(18.88 mm / 1085.7 mm) = 1.00°`

When a rider receives an Expedition 926d with the 492 mm Boost fork, the actual geometry deviates from the published chart as follows:

| Metric | Published Chart (CAD @ 472mm) | Actual Build (Boost @ 492mm) | Difference |
| :--- | :--- | :--- | :--- |
| **Head Tube Angle (HTA)** | 70.75° | **69.75°** | -1.0° slacker |
| **Seat Tube Angle (STA)** | 73.5° | **72.5°** | -1.0° slacker |
| **Stack** | 636.0 mm | **642.7 mm** | +6.7 mm higher |
| **Reach** | 390.0 mm | **378.9 mm** | -11.1 mm shorter |
| **BB Drop** | 85.0 mm | **77.2 mm** | +7.8 mm higher BB |
| **Front Center** | 639.0 mm | **646.3 mm** | +7.3 mm longer |

---

## Conclusion

The file [seven-expedition-926d.json](file:///Users/matt/src/bikegeo/data/bikes/seven-expedition-926d.json) correctly identifies `enve_mountain_472` as the CAD baseline fork (`cad_fork`), as it is the only fork that mathematically produces the published geometry numbers. However, because `enve_mountain_472` is a legacy 100 mm non-Boost fork that cannot physically be mounted to this frame, the supported forks array (`forks`) specifies `enve_mountain_boost` (492 mm A2C, 52 mm rake) and `sid_100` (506 mm A2C, 44 mm rake). Complete builds with the 492 mm Boost fork result in an actual geometry that is 1.0° slacker, ~7 mm taller, and ~11 mm shorter than published.
