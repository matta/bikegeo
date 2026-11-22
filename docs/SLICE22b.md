# Slice 22b: `/bike` SVG Diagram Targets & Label Collision Fixes

Status: Implemented 2026-11-22; awaiting Matt's acceptance
Implementer: Medium (mid-tier) — `internal/web`-only SVG target rendering, target-aware framing, deterministic 2D label collision avoidance with leader lines, `goquery`/unit test coverage, and live visual verification across all three diagram views.
Review: optional — `internal/web` SVG rendering and label layout only; zero physics, scoring, catalog, query-parsing, or package-boundary changes, verified by automated overlap/bounds tests and live visual inspection.

## Implementation handoff

- **Who & when:** Gemini 4 Hydrogen (High) · 2026-11-22
- **Commits:** `505b8b4` (Commit 1: target markers, tolerance zones, miss vectors, target-aware framing, diagram legend); Commit 2 (HEAD: per-view label curation, 2D collision placer with leader lines, stress tests, handoff).
- **Deviations:** None.
- **Verification:** `make pre-commit` passed; live browser visual check on `/bike` (`bike`, `seat`, `stem` views across default, `#apply-fitting`, `spacers=0`, and cleared targets) confirmed crisp target/oval/gate rendering, legend, leader lines, zero label overlaps, and zero clipping.
- **Open issues:** None.

## Goal

Upgrade the `/bike` schematic SVG (`internal/web`) so fit targets and fit errors are immediately visible in the diagram (`Target BRP` + `±10 × ±2 mm` gate box, `Target HandGrip` + tilted `±10 × ±20 mm` fit oval, and miss vectors from placed landmarks to targets), active targets stay in frame on `seat` and `stem` close-ups, and visible landmark/target labels never collide or clip outside the `viewBox` across the `bike`, `seat`, and `stem` views — while preserving pure Web 1.0 (zero JS) and keeping all trig inside `internal/pose`.

## Confirmed Rulings

- **Pure Web 1.0 & zero JS:** Q48, Q54 (no `<script>`, no `on*` handlers, no static inline `style="..."` outside `svg.diagram`'s `--units-per-px` custom property).
- **Trig lives in `internal/pose`:** `AGENTS.md` Architecture rule — outside `pose`, non-test code must not call raw trig (`math.Sin`, `math.Cos`, etc.); use `pose.Pose.Translate` and SVG transforms for rotated geometry, and call `assembly.FitOval.NormalizedRadius` / `assembly.FitGate*` constants rather than restating fit-gate math.
- **Fit-gate geometry:** Q17, Q59a (`assembly.DefaultGripOval()` semi-minor `10 mm`, semi-major `20 mm`, major axis heading `60°` world; `assembly.FitGateGripNormalizedRadius = 1.0`; `assembly.FitGateBrpXMm = 10.0`, `assembly.FitGateBrpYMm = 2.0`).
- **Slice 22b rulings (2026-11-22):**
  - **Q151:** Slice 22b addresses the `/bike` SVG diagram itself (target BRP and HandGrip markers, fit-error visualization, and landmark label collisions across `bike`/`seat`/`stem` views).
  - **Q155:** Draw target crosshairs, physical fit-gate tolerance zones (dashed BRP gate rectangle and dashed tilted HandGrip fit oval), and dashed miss connectors from placed `BRP`/`HandGrip` to their active targets, with pass/fail semantic color paired with distinct crosshair/dashed geometry so pass/fail never relies on color alone.
  - **Q156:** Keep every landmark's `<circle>` dot and `<title>` hover tooltip in the SVG DOM while curating visible text labels per view (`view=bike`: macro landmarks + active targets; `view=seat`: seat-cluster landmarks + `Target BRP`; `view=stem`: stem/bar landmarks + `Target HandGrip`) and running deterministic 2D collision avoidance with subtle leader lines when displaced so visible labels never overlap or clip outside the `viewBox`.
  - **Q157:** Include each view's active target and tolerance zone in its `viewBox` framing (`view=seat` includes `Target BRP` + gate box; `view=stem` includes `Target HandGrip` + fit oval; `view=bike` expands `wholeBox` only if an active target or tolerance zone falls outside the bike bounds), capped at the whole-bike `viewBox` via `closeUpViewBox`. Cleared targets preserve component-only framing.

## Key Design & Contract Decisions

### 1. Target Markers, Tolerance Zones & Fit-Error Vectors (Q155)
When `parsed.BrpTarget != nil` and/or `parsed.GripTarget != nil`, render a dedicated `<g class="fit-targets">` layer above component outlines and below landmark labels:
- **BRP target (`data-target="brp"`):**
  - **Pass/fail status (`data-target-status="pass|fail"`):** passes when `|target.X − placed.BRP.X| ≤ assembly.FitGateBrpXMm` and `|target.Y − placed.BRP.Y| ≤ assembly.FitGateBrpYMm`.
  - **Tolerance box (`data-target-zone="brp"`):** SVG `<rect>` centered on the target in physical mm (`width = 2 * FitGateBrpXMm`, `height = 2 * FitGateBrpYMm`), dashed stroke with subtle pass/fail tint fill.
  - **Miss vector (`data-target-miss="brp"`):** dashed `<line>` from placed `BRP` to `Target BRP` (omitted or zero-length guarded when coincident, e.g., `< 0.1 mm`).
  - **Target crosshair (`data-target-marker="brp"`):** crisp crosshair reticle (`+` + small ring) centered at `(target.X, -target.Y)`, scaled with `DrawScale` so it keeps consistent on-screen size in every view, with a `<title>` tooltip reporting target coordinates and pass/fail status.
- **HandGrip target (`data-target="grip"`):**
  - **Pass/fail status (`data-target-status="pass|fail"`):** passes when `assembly.DefaultGripOval().NormalizedRadius(*parsed.GripTarget, bike.Handlebar.HandGrip.Origin()) ≤ assembly.FitGateGripNormalizedRadius`.
  - **Fit oval (`data-target-zone="grip"`):** SVG `<ellipse cx="0" cy="0" rx="20.00" ry="10.00" transform="translate(X Y) rotate(-60.00)" />` derived from `assembly.DefaultGripOval()` (zero raw trig in `internal/web`), dashed stroke with subtle pass/fail tint fill. Verify sign: local `(+rx, 0)` rotated by `-60°` in SVG Y-down maps to world heading `+60°` (`30°` CW from vertical `+Y`).
  - **Miss vector (`data-target-miss="grip"`):** dashed `<line>` from placed `HandGrip` to `Target HandGrip` (omitted when coincident `< 0.1 mm`).
  - **Target crosshair (`data-target-marker="grip"`):** scaled crosshair reticle centered at `(target.X, -target.Y)` with `<title>` tooltip.
- **Zero-target and partial-target modes:** when `brp=off` (or `grip=off`), omit that target's zone, miss line, crosshair, and label completely; when both are cleared, omit `<g class="fit-targets">` cleanly.

### 2. Target-Aware Diagram Framing (Q157)
- **Whole-bike (`wholeBox`):** include any active BRP target + its 4 tolerance-box corners and any active HandGrip target + its 4 oriented fit-oval bounding corners (computed via `pose.NewPose(target, oval.MajorAxisHeadingDeg).Translate(±SemiMajorMm, ±SemiMinorMm).Origin()` — zero raw trig) in `pts` when building `wholeBox`. On typical builds (including default `/bike`), default targets lie well inside the wheel-to-wheel box, leaving `wholeBox` unchanged; extreme off-bike targets expand `wholeBox` cleanly instead of clipping.
- **Seat close-up (`view=seat`):** append `Target BRP` and its 4 gate-box corners to `framingPts` when `parsed.BrpTarget != nil`. On default `/bike`, this expands `view=seat` slightly upward so `Target BRP` (`Y = 713.1 mm`) and its tolerance box are cleanly visible alongside the saddle instead of clipped above the top edge.
- **Stem close-up (`view=stem`):** append `Target HandGrip` and its 4 fit-oval bounding corners to `framingPts` when `parsed.GripTarget != nil`.
- **Fallback safety:** `closeUpViewBox` already caps close-ups at `wholeBox`, so even a very distant custom target stays inside the diagram without inverting or overflowing.

### 3. Per-View Landmark Label Curation & Deterministic Collision Avoidance (Q156)
- **Landmark dots stay in DOM:** keep all 18 `[data-landmark="<Name>"]` `<g>` groups with their `<circle>` dot and add a `<title>{{.Name}}</title>` on every landmark so hovering still identifies detail points in `view=bike` and existing DOM presence contracts remain intact.
- **Curated visible text labels per view:**
  - `view=bike` (10 landmarks + up to 2 targets): `RearAxle`, `RearContact`, `BB`, `SeatTubeTop`, `BRP`, `SitBones`, `HeadTubeTop`, `HandGrip`, `FrontAxle`, `FrontContact`, plus `Target BRP` / `Target Grip` when active. Detail sub-landmarks (`RailCradle`, `RailCenter`, `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HeadTubeBottom`, `LowerCupBottom`) render smaller/muted dots (e.g., smaller radius or subtle opacity) without text labels in `view=bike`.
  - `view=seat` (seat cluster + active BRP target): `RailCradle`, `RailCenter`, `BRP`, `SitBones`, plus `Target BRP` when active (and `SeatTubeTop` only if its point lies inside the computed `viewBox`).
  - `view=stem` (cockpit cluster + active HandGrip target): `HeadTubeTop`, `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `HandGrip`, plus `Target Grip` when active.
  - Labels whose anchor point lies outside the current `viewBox` are never placed or drawn.
- **Deterministic 2D collision-avoidance placer:**
  - Run layout in **nominal screen-px space** (converting SVG coordinates via `UnitsPerPx = viewBox.W / nominalDiagramWidthPx` and measuring estimated monospace label bounding boxes from character count × glyph width and line height at the CSS label font size, with padding).
  - For each visible label (landmark or active target) in deterministic priority order (targets and primary contact points first, then frame/component landmarks):
    1. Generate candidate label placements starting from the landmark's preferred direction, then standard radial/cardinal slots around the point, then stepped-out callout offsets if crowded.
    2. Reject or heavily penalize overlaps with already-placed label boxes, other landmark/target points, and out-of-`viewBox` clipping (clamping/requiring every placed label box to stay inside `viewBox` with a small margin).
    3. If all ring candidates remain crowded (for example coincident points such as `spacers=0` where `CoverTop == SpacerTop`, `rail_offset=0` where `RailCradle == RailCenter`, or a fitted build where `BRP == Target BRP`), fall back to a deterministic non-overlapping vertical/horizontal nudge inside the `viewBox` so pairwise overlap is guaranteed zero.
  - **Leader lines:** when a placed label box's nearest point is displaced beyond a small threshold (for example `> 14 px` nominal screen distance from its landmark/target point), draw a subtle thin `<line class="label-leader">` connecting the point to the nearest edge of the label box.
  - Keep label font sizing in CSS `rem` (zero `font-size` SVG attributes on `<text>` elements) and store exact resolved SVG `LabelX`/`LabelY` + `Anchor` (and baseline `dy`, e.g., `0.35em` for vertically centered alignment) so SVG coordinates match the collision solver cleanly.

### 4. Diagram Legend, Styling & Pure Web 1.0 Constraints
- Add a concise visual legend right under `<svg class="diagram">` (above or integrated with `.diagram-note`) when targets are active — showing small inline SVG/CSS swatches for placed landmark dot, target crosshair, pass/fail tolerance zone, and miss vector — so a reader can interpret the diagram at a glance without hovering or guessing.
- Keep `bike.html`'s `<style>` self-contained using existing Workbench CSS variables (`--tier-all-text`, `--tier-all-bg`, `--danger`, `--error-bg`, `--accent`, `--border`, `--muted`).

### 5. Planner Defaults (One-Line Mechanics)
- **File organization:** extract SVG diagram building, target overlay math, and the 2D label collision placer into `internal/web/diagram.go` (with focused unit tests in `internal/web/diagram_test.go`) to keep `internal/web/web.go` concise.
- **Label typography:** tighten `.landmarks text` and `.target-label` font size from `1rem` to `0.8125rem` (`13 px` at default root size, still via `calc(0.8125rem * var(--units-per-px, 1))`) with the existing `#fcfcfa` halo stroke so close-up and whole-bike labels stay crisp and compact while honoring CSS `rem` scaling.
- **Target label text:** use concise `Target BRP` and `Target Grip` labels to minimize horizontal width in crowded cockpit/seat views.
- **Dot hierarchy in `view=bike`:** keep all 18 `[data-landmark]` dots in the DOM, rendering unlabelled detail dots slightly smaller (`r = Px(3)`) and subdued so macro dots (`r = Px(5)`) read cleanly.
- **Modern web guidance:** consult the `modern-web-guidance` skill if adding any new HTML/CSS patterns for the diagram legend or SVG accessibility.

## Rough Commit Split

- **Commit 1 — Target markers, tolerance zones, miss vectors, and target-aware framing:**
  - Extract SVG diagram assembly into `internal/web/diagram.go`, add BRP & HandGrip target markers, tolerance zones (gate box + tilted fit oval), pass/fail status, miss vectors, target-aware `wholeBox`/`closeUpViewBox` framing (using `pose.Pose.Translate`, zero raw trig), update `internal/web/templates/bike.html` SVG markup, CSS, and diagram legend, and update/add target & framing tests in `internal/web/diagram_test.go` and `internal/web/web_test.go`. Must pass `make pre-commit`.
- **Commit 2 — Per-view label curation, 2D collision avoidance with leader lines, and live visual verification:**
  - Implement per-view label curation (all 18 dots + `<title>` tooltips retained) and deterministic nominal-px 2D bounding-box label placement with leader lines and `viewBox` clamping in `internal/web/diagram.go`, wire `<line class="label-leader">` and conditional `<text>` rendering in `bike.html`, add exhaustive zero-overlap and inside-`viewBox` tests across representative builds and coincident edge cases in `diagram_test.go` and `web_test.go`, verify visually in a live browser across `bike`, `seat`, and `stem` views (default, fitted, cleared targets, `spacers=0`), and fold `## Implementation handoff` into this file. Must pass `make pre-commit`.

## High-Level Test Cases

1. **Active targets & pass/fail cues (`view=bike`, `view=seat`, `view=stem`):**
   - Default `/bike` renders `[data-target="brp"]` and `[data-target="grip"]` with crosshair, tolerance zone (`rect[data-target-zone="brp"]` `20.00 × 4.00`, `ellipse[data-target-zone="grip"]` `rx="20.00" ry="10.00"` with `rotate(-60.00)`), miss line, and `data-target-status` matching `assembly` fit-gate checks (`pass` after following `#apply-fitting` when targets are reached, `fail` when outside the gate).
   - Partial/cleared targets: `brp=off` omits `[data-target="brp"]` while keeping `[data-target="grip"]`; `grip=off` omits `[data-target="grip"]` while keeping `[data-target="brp"]`; `brp=off&grip=off` omits `.fit-targets` and matches component-only close-up framing.
2. **Target-aware close-up framing (Q157):**
   - On default `/bike?view=seat`, `viewBox` contains all seat landmarks, seat component outlines, AND `Target BRP` plus all 4 corners of its `±10 × ±2 mm` box.
   - On default `/bike?view=stem`, `viewBox` contains all stem/bar landmarks, stem outlines, AND `Target HandGrip` plus all 4 oriented corners of its fit oval.
   - Extreme custom target (e.g., 400 mm away) expands framing safely without exceeding/inverting `wholeBox`.
3. **Per-view label curation & DOM invariants (Q156):**
   - All 18 `[data-landmark="..."]` groups and `<circle>` dots remain present with non-empty `<title>` tooltips in every view.
   - `view=bike` renders `<text>` only for the 10 macro landmarks + active targets (omitting micro detail label text); `view=seat` renders `<text>` only for seat-cluster landmarks + active `Target BRP` (plus `SeatTubeTop` only when in view); `view=stem` renders `<text>` only for stem/bar landmarks + active `Target Grip`.
   - Zero SVG `font-size` attributes on any `<text>` inside `svg.diagram` (CSS `rem` scaling preserved).
4. **Zero label collisions & `viewBox` containment (negative & stress cases):**
   - Across a matrix of builds (`default`, `brp=off&grip=off`, fitted build via `#apply-fitting` where `BRP == Target BRP` and `HandGrip ≈ Target Grip`, `spacers=0` where `CoverTop == SpacerTop`, `rail_offset=0` where `RailCradle == RailCenter`, `fork=basis` on bare head tube where `HeadTubeBottom == LowerCupBottom`, extreme swept bar + short stem, and XS + XL frames) in all 3 views (`bike`, `seat`, `stem`):
     - No two visible label bounding boxes intersect.
     - Every visible label bounding box stays within `viewBox`.
     - Displaced labels emit `.label-leader` lines connecting the point to the label; undisplaced labels do not emit unnecessary clutter.
5. **Architecture & pure Web 1.0 invariants:**
   - No raw trig (`math.Sin`, `math.Cos`, `math.Tan`, `math.Atan2`) in `internal/web` non-test files.
   - Zero `<script>`, zero `on*` attributes, zero `[style]` outside `svg.diagram`.
6. **Live visual check:**
   - Run the server and visually inspect `/bike` in a browser across `Whole bike`, `Seatpost & saddle`, and `Stem & bar` views — testing default load, after clicking `Apply fitted build`, `spacers=0`, and `Clear Targets` — confirming crisp legibility, clean target/oval/gate rendering, zero text overlap, and clean leader lines.

## Out of Scope

- Changes to `internal/pose`, `internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, `cmd/bikegeo`, or `/` (`compare.html`).
- Interactive pan/zoom, client-side JavaScript or HTMX, or new URL query parameters.
- Changes to the `/bike` controls toolbar, geometry/fitting readouts, or ranked-builds tables beyond the diagram panel and its legend/note.
