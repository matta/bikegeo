# Seven Cycles Expedition 926f — Geometry Chart & Fork Analysis

## Context & Issue Description

Seven Cycles stated via email regarding the **Expedition 926f**:

> *"For the bike you’re looking at, it’s a Boost Enve Mountain fork, 110 spacing at the axle and 492mm axle-to-crown in the 52mm rake position which is what we designed this bike for."*

This claim contradicts the fork parameters reverse-engineered when ingesting [seven-expedition-926f.json](file:///Users/matt/src/bikegeo/data/bikes/seven-expedition-926f.json), which established that the published geometry chart was drawn in CAD around a **472 mm Axle-to-Crown (A2C)** fork (`enve_mountain_472`) paired with an external lower headset cup (10 mm EC44 spacer).

This document provides a complete, self-contained mathematical analysis proving that:
1. **The published geometry chart is mathematically incompatible with a 492 mm A2C fork.**
2. **The published geometry chart was drafted around a 472 mm A2C fork.**
3. Complete bikes physically built with the 492 mm Boost fork kinematically pitch backward by ~1.0°, resulting in an actual geometry significantly slacker, taller, and shorter than the published chart.

---

## Published Specifications: Standard Fit S 60cm

Published specifications from [Seven Cycles Expedition 926f](https://sevencycles.com/category/bikepacking/bp-926f.php) for size **S 60cm**:

| Parameter | Published Value | Notes / Units |
| :--- | :--- | :--- |
| **Size** | 60.0 cm | Standard Fit ("S") |
| **Stack (BB-HT)** | 64.0 cm (640.0 mm) | Vertical distance from BB center to top-center of head tube |
| **Reach (BB-HT)** | 40.9 cm (409.0 mm) | Horizontal distance from BB center to top-center of head tube |
| **Head Tube Angle (HTA)** | 70.0° | `cos(70.0°) = 0.342020`, `sin(70.0°) = 0.939693` |
| **Seat Tube Angle (STA)** | 73.5° | Published nominal angle |
| **Head Tube Length (HTL)** | 12.7 cm (127.0 mm) | Physical length of head tube |
| **Bottom Bracket Drop** | 8.6 cm (86.0 mm) | Vertical distance of BB below axle line |
| **Front Center** | 66.6 cm (666.0 mm) | Horizontal distance from BB center to front axle center |
| **Chainstay Length** | 46.3 cm (463.0 mm) | Axle to BB center |
| **Fork Rake** | 5.2 cm (52.0 mm) | Seven spec note: "Optimized... at 5.2 cm rake setting" |
| **Headset Cup Standard** | EC44 Lower | External cup on 44 mm titanium head tube adds 10.0 mm stack |

---

## Mathematical Proof 1: Master Stack Equation

Stack is the vertical distance from the bottom bracket center to the center of the top of the head tube. In bicycle frame coordinate geometry:

```
Stack = BB_Drop - Rake * cos(HTA) + (Fork_A2C + Headset_Cup + HTL) * sin(HTA)
```

For the S 60cm frame:
- `BB_Drop` = 86.0 mm
- `Rake * cos(70.0°)` = 52.0 mm * 0.342020 = **17.785 mm**
- `Headset_Cup` = 10.0 mm (EC44 lower external cup)
- `HTL` = 127.0 mm

### Evaluation with Seven's Claimed Fork (492 mm Boost ENVE)
With `Fork_A2C = 492.0 mm`:
- Structural A2C = 492.0 mm + 10.0 mm = **502.0 mm**
- Total length along steering axis = 502.0 mm + 127.0 mm = **629.0 mm**
- Vertical rise along steering axis = 629.0 mm * sin(70.0°) = 629.0 mm * 0.939693 = **591.067 mm**
- Calculated Stack = 86.0 mm - 17.785 mm + 591.067 mm = **659.28 mm (~659.3 mm)**
- Published Stack = **640.0 mm**

```
Discrepancy: 659.28 mm - 640.0 mm = +19.28 mm (~19.3 mm)
```

Even if evaluated with an impossible 0.0 mm headset cup (zero-stack / flush bearing):
- Total length along steering axis = 492.0 mm + 127.0 mm = 619.0 mm
- Calculated Stack = 86.0 mm - 17.785 mm + 619.0 mm * 0.939693 = **649.88 mm**

```
Zero-Cup Discrepancy: 649.88 mm - 640.0 mm = +9.88 mm (~9.9 mm)
```

### Evaluation with Reverse-Engineered Fork (`enve_mountain_472`)
With `Fork_A2C = 472.0 mm`:
- Structural A2C = 472.0 mm + 10.0 mm = **482.0 mm**
- Total length along steering axis = 482.0 mm + 127.0 mm = **609.0 mm**
- Vertical rise along steering axis = 609.0 mm * sin(70.0°) = 609.0 mm * 0.939693 = **572.273 mm**
- Calculated Stack = 86.0 mm - 17.785 mm + 572.273 mm = **640.49 mm (~640.5 mm)**
- Published Stack = **640.0 mm**

```
Discrepancy: 640.49 mm - 640.0 mm = +0.49 mm (within 0.5 mm rounding precision)
```

### Inversion: Solving for Exact Implied Fork A2C
Rearranging the Master Stack Equation to solve for the structural A2C required to produce 640.0 mm Stack:

```
Structural_A2C = (Stack - BB_Drop + Rake * cos(HTA)) / sin(HTA) - HTL
Structural_A2C = (640.0 - 86.0 + 17.785) / 0.939693 - 127.0 = 481.48 mm
Fork_A2C       = Structural_A2C - 10.0 mm (EC44 cup) = 471.48 mm (~472 mm)
```

---

## Mathematical Proof 2: Front Center Equation

Front Center is the horizontal distance from the bottom bracket center to the front axle center:

```
Front_Center = Reach + (Fork_A2C + Headset_Cup + HTL) * cos(HTA) + Rake * sin(HTA)
```

For the S 60cm frame:
- `Reach` = 409.0 mm
- `Rake * sin(70.0°)` = 52.0 mm * 0.939693 = **48.864 mm**
- `Headset_Cup` = 10.0 mm
- `HTL` = 127.0 mm

### Evaluation with Seven's Claimed Fork (492 mm Boost ENVE)
- Total length along steering axis = 492.0 mm + 10.0 mm + 127.0 mm = 629.0 mm
- Horizontal projection along steering axis = 629.0 mm * cos(70.0°) = 629.0 mm * 0.342020 = **215.131 mm**
- Calculated Front Center = 409.0 mm + 215.131 mm + 48.864 mm = **672.99 mm (~673.0 mm)**
- Published Front Center = **666.0 mm** (66.6 cm)

```
Discrepancy: 672.99 mm - 666.0 mm = +6.99 mm (~7.0 mm)
```

### Evaluation with Reverse-Engineered Fork (`enve_mountain_472`)
- Total length along steering axis = 472.0 mm + 10.0 mm + 127.0 mm = 609.0 mm
- Horizontal projection along steering axis = 609.0 mm * cos(70.0°) = 609.0 mm * 0.342020 = **208.290 mm**
- Calculated Front Center = 409.0 mm + 208.290 mm + 48.864 mm = **666.15 mm (~66.6 cm)**
- Published Front Center = **666.0 mm** (66.6 cm)

```
Discrepancy: 666.15 mm - 666.0 mm = +0.15 mm (within 0.15 mm rounding precision)
```

---

## Chart-Wide Verification (All 24 Sizes)

The tight convergence to a 472 mm A2C fork is consistent across the entire Rider-Ready chart (all 12 Standard Fit sizes and all 12 Endurance Fit sizes), confirming this is not an artifact of a single size:

| Size | HTA | HTL (mm) | Stack (mm) | Reach (mm) | BB Drop (mm) | Published FC (mm) | Implied Fork A2C (mm) | Calc FC @ 472mm (mm) | ΔFC (mm) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **S 55cm** | 69.25° | 114 | 628 | 390 | 90 | 649.0 | **471.02** | 649.78 | +0.78 |
| **S 57cm** | 69.25° | 117 | 630 | 399 | 89 | 659.0 | **471.23** | 659.85 | +0.85 |
| **S 58cm** | 69.75° | 121 | 635 | 402 | 88 | 659.0 | **471.22** | 659.49 | +0.49 |
| **S 59cm** | 70.00° | 125 | 639 | 405 | 87 | 661.0 | **471.35** | 661.47 | +0.47 |
| **S 60cm** | 70.00° | 127 | 640 | 409 | 86 | 666.0 | **471.48** | 666.15 | +0.15 |
| **S 61cm** | 70.25° | 130 | 643 | 412 | 85 | 669.0 | **471.55** | 667.75 | -1.25 |
| **S 62cm** | 70.50° | 140 | 653 | 419 | 85 | 676.0 | **470.98** | 675.65 | -0.35 |
| **S 63cm** | 70.50° | 153 | 663 | 429 | 83 | 689.0 | **470.71** | 689.98 | +0.98 |
| **S 64cm** | 70.50° | 163 | 673 | 433 | 83 | 696.0 | **471.31** | 697.32 | +1.32 |
| **S 65cm** | 70.50° | 178 | 685 | 436 | 81 | 704.0 | **471.17** | 705.33 | +1.33 |
| **S 66cm** | 70.50° | 192 | 698 | 442 | 81 | 714.0 | **470.96** | 716.00 | +2.00 |
| **S 68cm** | 70.50° | 222 | 724 | 451 | 79 | 731.0 | **470.66** | 735.02 | +4.02 |
| **E 55cm** | 69.25° | 114 | 628 | 390 | 90 | 649.0 | **471.02** | 649.78 | +0.78 |
| **E 57cm** | 69.25° | 126 | 638 | 396 | 89 | 659.0 | **470.78** | 660.04 | +1.04 |
| **E 58cm** | 69.75° | 130 | 643 | 400 | 88 | 659.0 | **470.75** | 660.61 | +1.61 |
| **E 59cm** | 70.00° | 135 | 648 | 402 | 87 | 661.0 | **470.93** | 661.89 | +0.89 |
| **E 60cm** | 70.00° | 141 | 653 | 405 | 86 | 666.0 | **471.32** | 666.94 | +0.94 |
| **E 61cm** | 70.25° | 152 | 663 | 406 | 85 | 669.0 | **470.80** | 669.18 | +0.18 |
| **E 62cm** | 70.50° | 161 | 673 | 413 | 85 | 676.0 | **471.19** | 676.66 | +0.66 |
| **E 63cm** | 70.50° | 174 | 683 | 423 | 83 | 689.0 | **470.92** | 690.99 | +1.99 |
| **E 64cm** | 70.50° | 184 | 693 | 427 | 83 | 696.0 | **471.53** | 698.33 | +2.33 |
| **E 65cm** | 70.50° | 199 | 705 | 430 | 81 | 704.0 | **471.38** | 706.34 | +2.34 |
| **E 66cm** | 70.50° | 213 | 718 | 436 | 81 | 714.0 | **471.17** | 717.01 | +3.01 |
| **E 68cm** | 70.50° | 243 | 744 | 445 | 79 | 731.0 | **470.88** | 736.03 | +5.03 |

Across all 24 sizes:
- **Mean implied fork A2C**: **471.1 mm**
- **Min / Max range**: **470.66 mm to 471.55 mm** (total spread: **0.89 mm**)

---

## Root Cause Analysis

1. **Two Generations of ENVE Mountain Forks**:
   - **Generation 1 (Non-Boost 15x100 mm)**: Axle-to-Crown of **470 mm** (44 mm rake position) or **472 mm** (52 mm rake position).
   - **Generation 2 (Boost 15x110 mm)**: Axle-to-Crown of **490 mm** (44 mm rake position) or **492 mm** (52 mm rake position).
2. **CAD Template Disconnect**:
   Seven sells the Expedition 926f with the Boost ENVE Mountain fork (492 mm A2C, 52 mm rake), but the CAD model used to generate the published geometry chart used the geometry of the Generation 1 non-Boost fork (472 mm A2C, 52 mm rake) with an EC44 headset cup. Seven never updated the published geometry table to reflect the taller 492 mm fork.

---

## Physical Dynamics of Complete Builds

Because complete builds ship with the 492 mm Boost fork (+20.0 mm taller than CAD), installing this fork lifts the front axle and rotates the frame rearward around the rear axle:

- Front axle vertical lift: `ΔY = 20.0 mm * sin(70.0°) = 18.79 mm`
- Rearward rotation angle: `θ = arcsin(18.79 mm / 1121 mm) = 0.96° (~1.0°)`

When a rider receives an Expedition 926f with the 492 mm Boost fork, the actual geometry deviates from the published chart as follows:

| Metric | Published Chart (CAD @ 472mm) | Actual Build (Boost @ 492mm) | Difference |
| :--- | :--- | :--- | :--- |
| **Head Tube Angle (HTA)** | 70.0° | **69.0°** | -1.0° slacker |
| **Seat Tube Angle (STA)** | 73.5° | **72.5°** | -1.0° slacker |
| **Stack** | 640.0 mm | **646.8 mm** | +6.8 mm higher |
| **Reach** | 409.0 mm | **398.2 mm** | -10.8 mm shorter |
| **BB Drop** | 86.0 mm | **78.4 mm** | +7.6 mm higher BB |
| **Front Center** | 666.0 mm | **672.2 mm** | +6.2 mm longer |

---

## Conclusion

The file [seven-expedition-926f.json](file:///Users/matt/src/bikegeo/data/bikes/seven-expedition-926f.json) correctly identifies `enve_mountain_472` as the CAD baseline fork (`cad_fork`), as it is the only fork that mathematically produces the published geometry numbers. Seven's email assertion that the published geometry was designed for the 492 mm fork is contradicted by elementary trigonometry and bicycle frame geometry laws.
