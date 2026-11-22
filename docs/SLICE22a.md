# Slice 22a: `/bike` UI Redesign — Engineering Workbench

Status: Planned 2026-11-22
Implementer: Medium (mid-tier) — single-route HTML/CSS workbench redesign and view-model cleanup in `internal/web` mirroring Slice 21 (`compare.html`), plus browserless `goquery` test updates and live visual verification.
Review: optional — `/bike` UI, template, and view-model refactor only; zero geometry, SVG-placement, scoring, query-parsing, or package-boundary changes, covered by `web_test.go` and live visual verification.

## Goal

Apply the Slice 21 **Engineering Workbench** design language from `/` to `/bike` (`internal/web/templates/bike.html`, `internal/web/bike_page.go`, `internal/web/web.go` view structs only, and `internal/web/web_test.go`) so the controls, schematic SVG (with its view switcher directly beside it), and structured geometry/fit diagnostics are clean, scannable, and above the fold on desktop, while fixing the Slice 14a accessibility, responsiveness, miss-readout, and `400` error-state defects — without changing any SVG geometry or Web 1.0 invariants.

## Confirmed Rulings

- **Pure Web 1.0 & zero JS:** Q48, Q54 (plain `GET` forms, URL query state, no `<script>`, no `on*` handlers; keep only the existing SVG `--units-per-px` CSS custom property on `<svg class="diagram">` and remove all static inline `style="..."` attributes).
- **`/bike` semantics, presets, scoring, and fork grouping:** Q61, Q62, Q67, Q70, Q78, Q90, Q109, Q110, Q139 (default Redshift post with Q78 caveat, durable `brp=off`/`grip=off` and `Clear Targets`, `prev_*` switch recovery, submitted-build score or off-catalog stem note, `Listed Forks` / `Other Compatible Forks` / `Chart Basis Fork` `<optgroup>`s).
- **Slice 22a rulings (2026-11-22):**
  - **Q151:** Split `/bike` UX overhaul into **Slice 22a** (Engineering Workbench UI, controls, readouts, tables, error recovery, accessibility) and **Slice 22b** (SVG diagram targets, fit-error visualization, and landmark label collisions).
  - **Q152:** Unified flat-bordered `<form class="controls">` toolbar: top full-width **Frameset** strip (`Frame`, `Size`, `Fork` + `Static A2C`, `Lower Cup`, `Sag`) above a hairline-separated 3-column parameter band (**Rear Cockpit** | **Front Cockpit** | **Fit Targets & Search**), followed by a merged action & status footer and a collapsed **Build & model notes** `<details>` drawer (flagging `(Redshift caveat)` in `<summary>` when `redshift_shockstop_pro` is selected).
  - **Q153:** Immediately below the toolbar, render a 2-column hero pairing the **SVG Diagram** panel (with `<nav class="diagram-views">` directly above `<svg class="diagram">`) alongside a structured **Geometry & Fit Status** panel, stacking on narrower viewports; **Fitting & Submitted Score** and **Ranked Builds** follow below.
  - **Q154:** Keep `table.build-table` to core columns (with a real `<th>` for `Apply`, distinct accessible link names, directional HandGrip miss, and explicit `target − placed` sign convention) and move the 12-attribute breakdowns (both ranked builds and submitted build) into collapsed native `<details>` disclosures alongside a compact tier strip with collapsed `Show tier definitions`.

## Key Design & Contract Decisions

### 1. Page Header, Title & Visual Tokens
- Match `compare.html`'s exact Engineering Workbench CSS tokens (`--bg`, `--surface`, `--surface-subtle`, `--border`, `--border-strong`, `--text`, `--muted`, `--accent`, tier badge colors, `--font-sans`, `--font-mono`, radii), `.page` container (`max-inline-size: 96rem`), and `.site-header` / `.site-nav` styling so `/` and `/bike` feel like one cohesive app.
- **Front-loaded `<title>`:** include the selected bike name and size first (for example `{{.BikeName}} — bikegeo`, or `Error — {{.BikeName}} — bikegeo` on `400`) so multi-tab comparisons (Q52) are distinguishable in browser tabs.

### 2. Unified Workbench Toolbar (`<form class="controls">`) (Q152)
Replace the 5 squeezed `.control-group` cards with one flat-bordered `<form>` divided by hairlines, preserving all hidden inputs (`prev_frame`, `prev_size`, `prev_fork`, `prev_post`, `view`) and control `id`/`name` attributes:
- **`400` error banner & invalid `<select>` options:**
  - `#form-error[role="alert"]` strip at the top of `<form class="controls">` with a `Reset Defaults` (`data-preset="error-reset"`) button, matching `/`.
  - When a `<select>` (`frame`, `size`, `fork`, `bar`, `post`, `bar_options`, `fork_options`) has a rejected submitted value (`InvalidField == <key>` and non-empty raw value), render a leading selected `<option value="<raw>" selected>Invalid: <raw></option>` above the valid options/`<optgroup>`s (matching `/` Q112) instead of leaving no option selected (which causes browsers to show the unrelated first option like `Binary Havok`).
- **Top `Frameset` strip (`<fieldset class="scope-bar">`):**
  - Visually reset `<legend>` (or `.visually-hidden` legend + `.scope-tag` label) grouping `Frame`, `Size`, `Fork` (preserving Q139 `<optgroup>`s), and the compact numeric trio `Fork Static A2C (mm)`, `{{.CupLabel}}`, and `Fork Sag (%)` (preserving `readonly` styling and state).
- **3-column hairline-separated parameter band (`.settings-grid`):**
  1. **Rear Cockpit (`<fieldset>`):** `Seatpost`, `Insertion (mm)`, `Rail Offset (mm)`, `Rail Pitch (°)` (with concise inline range/exposed-length hint if helpful, while full setback/clamp/length prose moves to the notes drawer).
  2. **Front Cockpit (`<fieldset>`):** `Handlebar`, `Spacers (mm)`, `Stem Length (mm)`, `Stem Angle (°)`, `Bar Roll (°)`.
  3. **Fit Targets & Search (`<fieldset>`):** `BRP X`, `BRP Y`, `HandGrip X`, `HandGrip Y` (2×2 grid with `BB origin · mm` in legend/sub-label; blank pair disables each target) plus `Handlebar options` and `Fork options` selects.
- **Merged action & status footer (`.workbench-footer`):**
  - *Left:* `Update Build` primary submit (`#ctrl-submit`), `Reset Defaults` (`#preset-defaults`), conditional `Clear Targets` (`#preset-clear-targets`), and `Chart Basis Fork` (`#preset-basis-fork`).
  - *Right (when `.HasBuild`):* concise build status telemetry — when `.Fitting.SubmittedScore` is present and scored, show the tier badge + score (`data-submitted-tier`, `data-metric="submitted-score"`) or off-catalog stem note; plus exposed post length (`data-metric="post-exposed"`) and `ΔPitch vs Basis` (`data-metric="delta-pitch"`) or quick status chips so key build health is visible immediately without redundancy.
- **Collapsed `Build & model notes` `<details>` drawer (`data-notes-drawer`, Q149/Q152):**
  - Directly below `.workbench-footer`, closed by default.
  - `<summary>` shows `Build & model notes` and appends `<span class="caveat-flag">(Redshift caveat)</span>` whenever `.RedshiftMixedStateNote` is non-empty.
  - Contains:
    - Conditional Q78 Redshift mixed-state caveat (`data-note="redshift-mixed-state"`).
    - Conditional Chart Basis Fork note (`data-note="basis-fork"`).
    - Conditional sliding-dropouts note (`data-note="sliding-dropouts"`).
    - Seatpost setback & frame/post insertion bounds (`data-note="post-setback"`, preserving `[data-metric="post-setback"]`).
    - Target coordinate conventions and default values (`BB at origin, +X forward, +Y up; clear both boxes to disable`).

### 3. 2-Column Hero Workspace (Diagram + Structured Geometry) (Q153)
Immediately below the toolbar (when `.HasBuild`):
- **Left / Primary column — Schematic Diagram panel (`.panel`):**
  - Panel header has `<h2>Leveled Bike Assembly</h2>` on the left and `<nav class="diagram-views" aria-label="Diagram view">` (`Whole bike · Seatpost & saddle · Stem & bar`) **directly above `<svg class="diagram">`** (kept outside `form.controls`).
  - `<svg class="diagram">` markup and all SVG attributes/elements stay completely unchanged in this slice (Slice 22b handles SVG targets and label collisions).
  - The illustrative-dashed-lines schematic footnote sits cleanly below the SVG (styled via CSS class, removing the static inline `style="..."`).
- **Right / Secondary column — Structured Geometry & Fit Status panel (`.panel`):**
  - Replaces the 17-item wall-of-text `.summary` and bottom inline-styled basis-fork `.summary` with grouped `<dl>` / key-value grids (monospace `tabular-nums`, preserving every existing `[data-metric="..."]` hook):
    1. *Frame & Handling:* `Pitch`, `Leveled HTA`, `Leveled STA`, `Leveled Reach`, `Leveled Stack`, `ETT`, `Effective A2C`, `Wheelbase`, `Leveled BB Drop`, `Trail`, `Rear Center`, `RC / WB`.
    2. *Contact Points & Cockpit:* `BB to post top (seat axis)`, `|BRP − BB|`, `BRP to HandGrip`, `Grip rise vs SitBones`, `Lateral grip (±Z)`, `Exposed post` (`data-note="post-exposed"` / `data-metric="post-exposed"`).
    3. *Chart Basis Reference:* basis fork specs, basis HTA/WB (+ published WB when present), and `ΔPitch vs Basis` (`data-metric="delta-pitch"`).
  - Collapses to a single column below `~1100px`.

### 4. Fit & Build Analysis + Ranked Builds (Q154)
Below the hero (when `.Fitting` is present), inside `<div data-fitting>`:
- **Continuous Fitting & Submitted Build panel (`.panel`):**
  - Real `<h2>` / `<h3>` headings (replacing bare `<strong>` headings).
  - **Saddle Fitting** and **Cockpit Fitting** rendered side-by-side (2-column grid, stacking on mobile) as crisp key-value summaries with clear `target − placed` sign convention, preserving all `[data-metric="fit-..."]` hooks and the `Apply fitted build` (`#apply-fitting`) action button (explaining clearly that it applies the continuous fitted insertion, rail offset, spacers, and stem length).
  - **Submitted Build Score** (`[data-submitted-score]`): concise summary bar (tier badge, score, active weight, Oval R, BRP miss `target − placed`, or `[data-note="off-catalog-stem"]`) with the 12-attribute breakdown table folded into a collapsed `<details data-submitted-breakdown>` (`Show 12-attribute score breakdown`) with proper `<caption>`, `<th scope="col">`, and `.table-scroll` region attributes.
- **Ranked Builds panel (`[data-build-ranking]`, Q154):**
  - Clear `<h2>` + subtitle explaining what Ranked Builds is ("Catalog part combinations evaluated for this frame/size; click Apply to load a build into the inspector above") and stating the `target − placed` sign convention.
  - Preserves `[data-build-summary]` and `[data-build-evaluated]` text hooks.
  - Compact **Fit tiers** strip showing all 4 tier badges inline + collapsed `<details data-tier-definitions>` (`Show tier definitions`) holding `[data-fit-tier-legend]` (matching `/`).
  - **Primary table (`table.data-table.build-table`):** wrapped in `.table-scroll` (`tabindex="0"`, `role="region"`, `aria-labelledby`), with `<caption>`, `<th scope="col">` on every column including `Action` (fixing Lighthouse `td-has-header`), sticky `<thead>`, right-aligned monospace `tabular-nums`, styled `Apply` links with distinct `aria-label` (for example `Apply rank 1 build: 80 mm stem, 20 mm spacers`), and **directional HandGrip miss** (`(ΔX, ΔY) mm · dist mm`, target − placed) in `[data-build-cell="grip-miss"]` alongside the other 10 core columns (`#`, `Action`, `Tier`, `Score`, `Stem`, `Spacers`, `HandGrip miss`, `Oval R`, `BRP miss`, `Handlebar`, `Fork`).
  - **12-Attribute Breakdown table:** collapsed inside `<details class="disclosure" data-build-breakdown>` below the primary table, with its own `.table-scroll` region, `<caption>`, `<th scope="col">`, `tr[data-build-breakdown-rank="{{.Rank}}"]`, and `[data-build-attr="{{.Attribute}}"]` cells (matching `/` Q113).

### 5. Planner Defaults (One-Line Mechanics)
- **Self-contained CSS:** keep `bike.html`'s `<style>` self-contained using the same tokens as `compare.html` (no shared CSS extraction in this slice).
- **Compact desktop density with touch safety:** desktop controls/buttons use `min-block-size: 2rem`; `@media (pointer: coarse)` preserves `min-block-size: 2.85rem` touch targets and `font-size: max(1rem, 0.88em)`.
- **Responsive breakpoints:** 3-column parameter band collapses to 2 columns at `≤ 1024px` and 1 column at `≤ 680px`; 2-column hero and 2-column fitting grid collapse to 1 column at `≤ 1100px` / `≤ 820px`.
- **Directional `HandGrip miss` view-model fields:** add `GripMissXMm` and `GripMissYMm` (using `cleanZero`, `target − placed`) to `scoredBuildView` in `bike_page.go` so `table.build-table` displays both signed `(ΔX, ΔY) mm` and distance.
- **Modern web guidance:** consult the `modern-web-guidance` skill before writing HTML/CSS updates for `<details>`, responsive tables, sticky headers, and accessible forms.

## Rough Commit Split

- **Commit 1:** Refactor `internal/web/bike_page.go`, `internal/web/web.go` (view structs only, no SVG geometry changes), and `internal/web/templates/bike.html` to the Engineering Workbench layout, update `internal/web/web_test.go` for the split breakdown table, directional HandGrip miss, `Invalid: ...` `<option>` on `400` pages, and accessibility invariants, run live visual verification across desktop and mobile viewports, and include the `## Implementation handoff` in this file. Must pass `make pre-commit`.

## High-Level Test Cases

1. **Default `GET /bike` (`200 OK`):** `<title>` front-loads the selected bike name & size; all `[data-metric="..."]` values match existing physics pins; `<nav class="diagram-views">` sits inside the diagram panel (outside `form.controls`); `[data-notes-drawer]` is closed by default, flags `(Redshift caveat)` in `<summary>`, and contains `[data-note="redshift-mixed-state"]` and `[data-note="post-setback"]`; `[data-submitted-breakdown]`, `[data-tier-definitions]`, and `[data-build-breakdown]` are closed by default; `table.build-table` has 11 core columns (no inline attribute columns) and directional HandGrip miss; `[data-build-breakdown]` holds all 12 `[data-build-attr]` cells per rank.
2. **Non-Redshift seatpost (`/bike?post=setback_0`):** `[data-notes-drawer] summary` omits `(Redshift caveat)` and `[data-note="redshift-mixed-state"]` is absent.
3. **Cleared targets & off-catalog stem:** `brp=off&grip=off` hides `[data-fitting]`, `[data-submitted-score]`, `[data-build-ranking]`, and `#preset-clear-targets`; off-catalog stem shows `[data-note="off-catalog-stem"]` and omits `[data-submitted-breakdown]`.
4. **Error recovery (`400 Bad Request`):** rejected `<select>` values (`frame=nope`, `size=nope`, `fork=nope`, `bar=nope`, `post=nope`, `bar_options=nope`, `fork_options=nope`) render `#form-error[role="alert"]`, `aria-invalid="true"`, and a selected leading `<option value="nope" selected>Invalid: nope</option>` while preserving `prev_*` recovery and valid options.
5. **Accessibility & pure Web 1.0 invariants:** zero `<script>`, zero `on*` attributes, zero `[style]` outside `svg.diagram`; every `<fieldset>` in `form.controls` has a direct non-empty `<legend>`; every `[aria-describedby]` resolves to an existing `#id`; every `table.data-table` has a non-empty `<caption>` and non-empty `<th scope="col">` on every header cell (including `Apply` / `Action`); every `.table-scroll` has `tabindex="0"`, `role="region"`, and an accessible name; zero headings inside `<details><summary>`; every `a[data-build-apply]` has a distinct accessible name.
6. **Live visual check:** start the server and verify `/bike` in a browser at desktop (`1440×900` and `1280×800`) and mobile (`390×844`) widths — checking toolbar compactness, diagram + view switcher + structured geometry above the fold, zero horizontal page overflow on mobile, expanded `<details>` drawers, `400` error state (`?frame=nope`), and `view=seat` / `view=stem` switching.

## Out of Scope

- SVG diagram target markers (BRP/HandGrip), fit-error visualization, and landmark label collision fixes (deferred to **Slice 22b** per Q151).
- Changes to `internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, `cmd/bikegeo`, or `internal/web/templates/compare.html`.
- New URL query keys, client-side JavaScript or HTMX, dark mode, or shared CSS extraction between `/` and `/bike`.
