# Measuring Saddle Biomechanical Reference Point (BRP) Offset

This guide explains how to measure the physical relationship between a saddle's **Biomechanical Reference Point (BRP)** and its **mounting rails** for use in `bikegeo` fit calculations.

---

## 1. What is the BRP?

The **Biomechanical Reference Point (BRP)** is an industry-standard fit benchmark popularized by Selle Italia and the **idmatch** fitting system:
* It is defined as the exact cross-section along the saddle's longitudinal centerline where the saddle shell reaches a **width of 70 mm**.
* Because saddle lengths and shapes vary dramatically between short-nosed modern saddles (240 mm) and traditional saddles (275–285 mm), measuring saddle position from the nose is unreliable. The BRP provides a consistent, anatomically anchored landmark that aligns with the rider's pelvis and ischial tuberosities (sit bones).

---

## 2. Why Rail Offset Matters in `bikegeo`

When a saddle is clamped in a seatpost:
1. **Seatpost Setback**: The seatpost clamp typically sits offset from the extended seat tube centerline. For example, a **Redshift ShockStop suspension seatpost** introduces a `+7.0 mm` rearward static setback.
2. **Saddle Rail Position**: The saddle rails provide a fore-aft adjustment window (typically `±20 mm` from the center of the rail markings).
3. **Ideal Frame Fit**: An ideal frame geometry allows clamping the saddle **dead-center (`0 mm`)** on its rails. If a frame has an excessively steep seat tube angle, the rider must slide the saddle far back on the rails (e.g. `+15 mm` to `+20 mm`) or install an offset adapter (such as a Fair Bicycle Drop Best) to achieve their biomechanical setback.

In `bikegeo`, the total neutral perpendicular offset from the seat tube centerline to the BRP is:

$$\text{DefaultBrpOffsetMm} = \text{DefaultSeatpostSetbackMm} + \text{DefaultSaddleBrpOffsetMm}$$

---

## 3. How to Measure Your Saddle BRP Offset

To determine `DefaultSaddleBrpOffsetMm` for your specific saddle:

### Step 1: Find the 70 mm BRP Line
1. Place the saddle on a flat workbench with its rails parallel to the table.
2. Using calipers or a precise millimeter ruler, locate the point along the saddle nose where the top shell is **exactly 70.0 mm wide**.
3. Mark this position lightly with a pencil or small piece of tape across the top or side edge of the saddle cover. (Some Selle Italia saddles have this line pre-marked on the cover or underside).

### Step 2: Locate the Center of the Saddle Rails
1. Look at the printed scale on the straight clamping section of the saddle rails.
2. Identify the **`0` mark** (the midpoint of the manufacturer's designated clamping range).
   * *Note: If the rails do not have a `0` mark, measure the total straight length of the clamping section between the front and rear bends and find the exact halfway point.*

### Step 3: Measure the Fore-Aft Offset
1. Sight straight down (perpendicular to the straight section of the rails) from your marked 70 mm BRP line down to the rail scale:
   * **If the 70 mm line is directly above the rail center (`0` mark)**:
     `Saddle BRP Offset = 0.0 mm`
   * **If the 70 mm line sits rearward (toward the saddle tail) from the rail center**:
     `Saddle BRP Offset = +X mm` (positive offset)
   * **If the 70 mm line sits forward (toward the saddle nose) from the rail center**:
     `Saddle BRP Offset = -X mm` (negative offset)

---

## 4. Special Considerations for SQlab Saddles

SQlab saddles (e.g. 611, 612 Ergowave, 60X) feature an ergonomic stepped wave design:
* The rider's sit bones do not rest on the narrow nose taper; they sit firmly on the raised rear Ergowave platform.
* To accommodate this posture without forcing the rider to reach too far forward, SQlab designs their saddle rails roughly **10–15 mm longer** and positioned further forward under the shell than standard saddles.
* Consequently, when an SQlab saddle is mounted with its rails centered, the effective anatomical reference point sits much closer to the center of the rails (`0.0 mm`), rather than behind the clamp. `bikegeo` therefore defaults `DefaultSaddleBrpOffsetMm = 0.0 mm`.
