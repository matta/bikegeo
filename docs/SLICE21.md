# Slice 21: `/` Compare UI Redesign — Engineering Workbench

Status: Planned 2026-11-22
Implementer: Medium (mid-tier) — single-route HTML/CSS workbench redesign in `internal/web` following established Web 1.0 template, Go view-model, and browserless `goquery` test patterns, plus a live visual check.
Review: optional — `/` UI and CSS refactor only; zero geometry, scoring, query-parsing, or package-boundary changes, covered by `compare_test.go` and live visual verification.

## Goal

Refactor `/` (`internal/web/templates/compare.html`, `internal/web/compare_page.go`, and `internal/web/compare_test.go`) from the card-heavy Slice 14b layout into a compact, zero-JavaScript **Engineering Workbench** so the controls and the top rows of the primary comparison table share the initial desktop viewport, without dropping any functionality, error recovery, accessibility, or Web 1.0 invariants.

## Confirmed Rulings

- **Pure Web 1.0 & zero JS:** Q48, Q54 (plain `GET` forms, URL query state, no `<script>`, no inline `style`, no `on*` handlers).
- **`/` comparison semantics, error recovery, and tables:** Q52, Q61, Q67, Q76, Q77, Q87, Q90, Q112, Q113, Q124, Q139 (derived frame count, strict `400` error recovery with editable echoed form and `Invalid: ...` options, grouped listed/other compatible forks, three one-row-per-bike tables with lossless `/bike` inspect links and sliding-dropout `*` flag).
- **Slice 21 rulings (2026-11-22):**
  - **Q148:** Unified flat-bordered `<form>` toolbar with a compact full-width `Scope` bar across the top, above a hairline-separated 3-column parameter band (`Targets` | wider 2-up `Components` | `Limits`).
  - **Q149:** Fold static helper prose (coordinates, `bar_options=all` speed note, limit rules) and the Q78 Redshift mixed-state caveat into a collapsed **Parameter & model notes** `<details>` drawer beneath the action/telemetry footer, flagging `(Redshift caveat)` in `<summary>` whenever `redshift_shockstop_pro` is selected.
  - **Q150:** Cap `.table-scroll` vertical height (`max-block-size: 70vh`, `overflow: auto`) so sticky `<thead>` headers genuinely stick when scrolling long result lists.

## Key Design & Contract Decisions

### 1. Unified Workbench Toolbar (`<form class="controls">`)
Replace the outer card + nested `<fieldset>` cards with one flat-bordered `<form>` container divided by subtle hairlines:
- **Top `Scope` bar (Q148):**
  - *All-frames mode:* single compact inline bar showing the derived mode summary (`Comparing all N catalog frames (best each)`, keeping `data-compare-mode="all"`) alongside the inline `Add a frame…` `<select>` (with its blank hidden `size`/`fork`/`sag` inputs).
  - *Custom-selection mode:* expands smoothly into the compact selection table (`#`, `Frame`, `Size`, `Fork` with `Listed Forks` / `Other Compatible Forks` `<optgroup>`s per Q139, `Sag`, `Remove`) plus the inline `Add a frame…` row at the bottom of the Scope strip.
- **3-column parameter band (Q148):** separated by vertical hairlines (`border-inline-end`), using visually reset `<fieldset>` + `<legend>` elements so screen-reader grouping stays intact:
  1. **Targets (`BB origin · mm`):** 2×2 grid for `BRP X`, `BRP Y`, `HandGrip X`, `HandGrip Y` (units in group legend/labels rather than prose).
  2. **Components (wider 2-up column):** arranges `Seatpost`, `Handlebar`, `Handlebar options`, and the `Bar Roll (°)` / `Rail Pitch (°)` pair across 2 compact rows so the entire parameter band stays ~2 control rows tall on desktop.
  3. **Limits:** `Per frame` and `Overall limit` inputs with concise `auto` / `all` placeholders.
- **Merged action & telemetry footer:** replaces the standalone `Comparison Summary` card:
  - *Left:* `Compare Bikes` primary submit button, `Reset Defaults` link (`#preset-defaults`), and conditional `All N Frames` link (`#preset-all-frames`) in custom mode.
  - *Right (when `.HasResults`):* concise execution telemetry (`Showing X of Y configurations · Z builds evaluated`), active target coordinates (and optional non-zero posture note), and — when `bar_options=selected` — the fixed handlebar name so Q113's fixed-bar indicator stays visible without a redundant summary card.
- **Progressive disclosure notes drawer (Q149):** directly below the footer strip, a collapsed native `<details>` drawer (`Parameter & model notes`, or `Parameter & model notes (Redshift caveat)` when `redshift_shockstop_pro` is selected) containing:
  - Coordinate convention and default target values (`targets-help`).
  - Conditional Q78 Redshift mixed-state caveat (`data-note="redshift-mixed-state"`).
  - Handlebar enumeration speed note (`data-note="bar-options-speed"`).
  - Blank/zero `top_per_frame` and `top` limit explanations (`top-per-frame-help`, `top-help`).
  - Note: keep `aria-describedby` valid (`#form-error` on invalid fields; only link notes IDs if present in the DOM).
- **`400` error state (Q76, Q112):** `#form-error[role="alert"]` banner with `Reset Defaults` (`data-preset="error-reset"`) at the top of the toolbar; echoes all submitted values, highlights invalid inputs/selects with `aria-invalid="true"` and `Invalid: ...` `<option>`s, preserves `Remove` links, and omits telemetry and results tables.

### 2. Data-First Results Area
Immediately below the toolbar (no standalone summary card in between):
- **Compact tier strip & disclosure:** a single horizontal bar showing the four colored tier badges (`Fits all`, `Fits BRP`, `Fits grip`, `Fits neither`) in rank order alongside an inline collapsed `<details>` toggle (`Show tier definitions`) revealing `<dl data-fit-tier-legend>` with all four Q61/Q90 definitions.
- **Three grouped one-row-per-bike tables (Q113):**
  1. **Fit & Winning Build (`table[data-table="fit"]`):** expanded, immediately visible above the fold on desktop. Crisp row borders, subtle hover highlight, right-aligned monospace `tabular-nums` numeric cells, lossless Q77 `Inspect` links, `*` sliding-dropout indicators, and conditional `Handlebar` column when `bar_options=all`.
  2. **Leveled Geometry & Handling (`table[data-table="geometry"]`):** expanded below the primary table in identical rank order, with the `*` sliding-dropout footnote (`data-note="sliding-dropouts"`).
  3. **12-Attribute Score Breakdown (`table[data-table="breakdown"]`):** collapsed inside native `<details data-score-breakdown>` by default, with `<h2>` outside `<summary>`.
- **Sticky headers & scroll container (Q150):** `.table-scroll` sets `overflow: auto; max-block-size: 70vh;` (retaining `tabindex="0"`, `role="region"`, and `aria-labelledby`/`aria-label`) so `thead th { position: sticky; inset-block-start: 0; }` stays pinned during vertical scroll while keeping horizontal scroll shadows intact.

### 3. Planner Defaults (One-Line Mechanics)
- **Compact desktop density with touch safety:** desktop form controls and buttons use `min-block-size: 2rem` (`~32px`) and tighter padding; `@media (pointer: coarse)` preserves `min-block-size: 2.75rem–3rem` touch targets.
- **Visual palette:** keep the existing light Engineering Workbench tokens (`color-scheme: light`, paper/ink surfaces, border hairlines, existing semantic tier badge colors); no dark mode or shared CSS extraction with `bike.html` in this slice.
- **Responsive breakpoints:** the 3-column parameter band collapses to 2 columns at `≤ 1024px` and 1 column at `≤ 680px`, switching vertical hairlines (`border-inline-end`) to horizontal dividers (`border-block-end`).
- **Modern web guidance:** consult the `modern-web-guidance` skill before writing HTML/CSS updates for `<details>`, responsive tables, sticky headers, and accessible forms.
- **View-model cleanup:** remove unused standalone-summary fields from `compareSummaryView` only if the refactored template no longer needs them, and add a small boolean/label helper if needed for the Redshift-caveat `<summary>` indicator. Keep existing `data-*` hooks intact wherever the element remains; update `compare_test.go` summary-chip assertions to target the merged telemetry footer.

## Rough Commit Split

- **Commit 1:** Refactor `internal/web/compare_page.go` and `internal/web/templates/compare.html` to the Engineering Workbench layout, update `internal/web/compare_test.go` for the merged telemetry strip and progressive-disclosure drawers, run live visual verification across desktop and mobile viewports, and include the `## Implementation handoff` in this file. Must pass `make pre-commit`.

## High-Level Test Cases

1. **Default `GET /` (all 18 frames):** `200 OK`; all 18 rows and pinned values unchanged across `fit`, `geometry`, and `breakdown` tables; `[data-compare-mode="all"]` present with derived frame count; zero selection rows; telemetry footer reports `18`, `2889`, `2163861`, targets, and fixed handlebar (`Persuader`); `Parameter & model notes` `<details>` is closed by default, flags `(Redshift caveat)` in `<summary>`, and contains `data-note="redshift-mixed-state"` and `data-note="bar-options-speed"`; tier strip renders all 4 badges and a closed `<details>` holding `[data-fit-tier-legend] dt` (4 items); breakdown `<details>` is closed by default.
2. **Non-Redshift seatpost (`/?post=setback_0`):** `Parameter & model notes` `<summary>` omits the Redshift caveat flag and `data-note="redshift-mixed-state"` is absent.
3. **Custom selections & `bar_options=all`:** untouched blank `Add a frame` stays in all-frames mode; single and multi-frame selections render inside the Scope bar with working `Remove` and `All 18 Frames` links and Q139 `<optgroup>` fork options; `bar_options=all` shows the `Handlebar` column and updates telemetry.
4. **Lossless `/` → `/bike` links (Q67, Q77, Q110):** existing end-to-end scenarios remain green without relaxing assertions.
5. **Negative / `400` error recovery (Q76, Q112):** cleared targets (`brp=off`, `grip=off`, empty boxes), partial targets, rigid+sag, any-fork+sag, unknown/incompatible frame/size/fork, invalid global selects/numbers, unknown query keys, and misaligned columns all return `400`, render `#form-error[role="alert"]` and `Reset Defaults`, omit results tables and telemetry, highlight invalid controls (`aria-invalid="true"` + `Invalid: ...` option), and keep `Remove` links working.
6. **Accessibility & pure Web 1.0 invariants:** zero `<script>`, zero `[style]`, zero `on*` attributes; every `table.data-table` has a non-empty `<caption>` and non-empty `<th scope="col">`; every `.table-scroll` has `tabindex="0"`, `role="region"`, and an accessible name; zero headings inside `<details><summary>`; `<fieldset>` + `<legend>` preserved for parameter groups.
7. **Live visual check:** start the server and verify `/` in a browser at desktop (`1440×900` and `1280×800`) and mobile (`390px`) widths in all-frames mode (top rows of `Fit & Winning Build` visible above the fold without scrolling), custom-selection mode (2+ frames), `400` error state, expanded `<details>` drawers, and sticky `<thead>` scrolling on a multi-dozen-row list (for example `/?frame=otso-fenrir-stainless&size=L`).

## Out of Scope

- `/bike` UX overhaul (separate companion slice in Milestone 5).
- Changes to `internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, or `cmd/bikegeo`.
- New URL query keys, client-side JavaScript or HTMX, dark mode, or visual changes to `internal/web/templates/bike.html`.
