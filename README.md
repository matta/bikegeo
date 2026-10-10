# BikeGeo Analyst 🚲

A simple, rigorous web application for **bicycle geometry validation and apples-to-apples frame comparisons**. Built in **TypeScript** and managed via **pnpm**.

Bike geometry is stored directly in **local JSON files** under [`data/bikes/`](file:///home/matt/src/bikegeo/data/bikes).

---

## Key Features

1. **Local JSON Bike Storage**:
   - Each frame model is stored as an individual JSON document in [`data/bikes/`](file:///home/matt/src/bikegeo/data/bikes).
   - Supports `manufacturer`, `model`, `size`, `category`, `reach`, `stack`, `hta_deg`, `sta_deg`, `htl`, `bb_drop`, `fork_rake`, `wheelbase`, `fork_travel_mm`, `fork_a2c`, `headset_cup_type`, `cup_stack`, `seat_height`, and more.
   - Built-in UI to add, edit, duplicate, and delete bikes directly saving to disk.

2. **Hardtail Geometry Dynamics**:
   - Hardtails compress **only at the front fork**, which dynamically rotates the entire frame around the rear axle.
   - Accurately computes how changes in fork axle-to-crown (A2C) or suspension sag dynamically steepen/slacken head & seat angles, alter reach & stack, and shift bottom bracket drop.

3. **Master Stack Equation & CAD Reverse-Engineering**:
   - Proves whether published manufacturer charts match physical front-end geometry:
     $$\text{Stack} = \text{BB\_drop} - \text{Rake} \cdot \cos(\text{HTA}) + (\text{A2C} + \text{cup\_stack} + \text{HTL}) \cdot \sin(\text{HTA})$$
   - Reverse-engineers geometric Axle-to-Crown:
     $$\text{A2C}_{\text{geo}} = \frac{\text{Stack} - \text{BB\_drop} + \text{Rake} \cdot \cos(\text{HTA})}{\sin(\text{HTA})} - \text{HTL} - \text{cup\_stack}$$
   - Automatically detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.

4. **Headset Hardware Reality**:
   - Explicitly accounts for external lower headset cups.
   - Straight 44mm head tubes running tapered forks require an **EC44 lower cup**, adding **12 mm** of physical stack below the head tube.

5. **Apples-to-Apples Canonicalized Comparisons**:
   - Compares multiple frames on an identical benchmark (same canonical fork A2C, same suspension sag %, same headset cup standard, same rider saddle height).
   - Computes:
     - Canonical Reach, Stack, HTA, STA, and BB Drop.
     - Actual Effective Top Tube (AETT) at rider saddle height.
     - Spare Steerer Stack (vertical and axial length required to level bars with saddle).
     - Delta values ($\Delta$) relative to baseline and reference frames.

6. **Interactive 2D Cartesian SVG Frame Diagram**:
   - Superimposes baseline frame pose against the dynamically pitched canonical pose relative to the rear axle origin $(0, 0)$.

---

## Quick Start (pnpm)

### Prerequisites
- Node.js (v18+)
- pnpm (v9+)

### Installation
```bash
pnpm install
```

### Development Server
Starts the web application and embedded JSON REST API:
```bash
pnpm dev
```
Open [http://localhost:5173](http://localhost:5173) in your browser.

### Run Geometry Test Suite
Runs the 2D Cartesian mathematical model test suite verifying parity against the analyst Python reference specification:
```bash
pnpm test
```

### Production Build & Server
```bash
pnpm build
pnpm server
```
Runs the production server on [http://localhost:3001](http://localhost:3001).

---

## Unit Conventions

All inputs and outputs strictly adhere to:
- **Lengths / Distances**: Millimeters (`mm`) (Reach, Stack, HTL, BB Drop, Rake, A2C, Wheelbase, Seat Height).
- **Angles**: Degrees (`°`) (HTA: Head Tube Angle, STA: Seat Tube Angle). Internal conversions to radians are handled automatically.

---

## Sample Bikes Included

| File | Bike Model | Size | Published Fork | Headset | CAD Convention |
| :--- | :--- | :--- | :--- | :--- | :--- |
| [`binary-havok-m.json`](file:///home/matt/src/bikegeo/data/bikes/binary-havok-m.json) | Binary Havok | M | RockShox SID 110mm (521mm) | EC44 (+12mm) | Sagged CAD (~20%) |
| [`santa-cruz-chameleon-l.json`](file:///home/matt/src/bikegeo/data/bikes/santa-cruz-chameleon-l.json) | Santa Cruz Chameleon 29 | L | Fox 34 130mm (541mm) | ZS44 (+2mm) | Static CAD (0% Sag) |
| [`kona-honzo-esd-l.json`](file:///home/matt/src/bikegeo/data/bikes/kona-honzo-esd-l.json) | Kona Honzo ESD | L | Marzocchi Z1 150mm (561mm) | ZS44 (+2mm) | Static CAD (0% Sag) |
| [`trek-procaliber-l.json`](file:///home/matt/src/bikegeo/data/bikes/trek-procaliber-l.json) | Trek Procaliber Gen 3 | L | RockShox SID 120mm (530mm) | IS52 (0mm) | Static CAD (0% Sag) |
| [`chromag-rootdown-m-l.json`](file:///home/matt/src/bikegeo/data/bikes/chromag-rootdown-m-l.json) | Chromag Rootdown | M/L| RockShox Pike 150mm (561mm) | EC44 (+12mm) | Static CAD (0% Sag) |
| [`demo-inconsistent-hardtail.json`](file:///home/matt/src/bikegeo/data/bikes/demo-inconsistent-hardtail.json) | Sample Inconsistent Spec | M | 130mm (541mm) | EC44 (0mm cup) | **Mismatched Demo** |

---

## Project Structure

```
bikegeo/
├── data/
│   └── bikes/                   # Local JSON bike files
│       ├── binary-havok-m.json
│       ├── santa-cruz-chameleon-l.json
│       └── ...
├── src/
│   ├── types/
│   │   └── bike.ts              # BikeGeometry, ValidationReport, Fit types
│   ├── lib/
│   │   ├── geometry.ts          # 2D Cartesian physics & pitching math
│   │   ├── validator.ts         # Master stack proof & consistency checks
│   │   └── canonicalizer.ts     # Apples-to-apples normalization engine
│   ├── server/
│   │   ├── index.ts             # Express REST API
│   │   ├── bikesHandler.ts      # File system JSON read/write service
│   │   └── start.ts             # Server launcher
│   ├── components/
│   │   ├── Navbar.tsx           # Navigation & actions
│   │   ├── BikeCompare.tsx      # Multi-frame comparison table & deltas
│   │   ├── BikeValidator.tsx    # Single-bike CAD proof & sag slider
│   │   ├── BikeDiagram.tsx      # 2D SVG frame geometry visualizer
│   │   └── BikeEditorModal.tsx  # Add / edit JSON modal
│   ├── App.tsx                  # Main application state & views
│   ├── main.tsx                 # React entry point
│   └── index.css                # Tailwind CSS v4 styles
├── test/
│   └── geometry.test.ts         # Parity test suite
├── package.json
├── tsconfig.json
└── vite.config.ts
```
