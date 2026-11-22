# Slice 16b: frame ingestion workflow

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-21, Orchestrator (Neon).
- **Commits:**
  - C1 `Let catalog-only frames pass the parity test`: `210ec1d`
  - C2 `Derive frame counts in / labels from the catalog`: `56f5205`
  - C3 `Rewrite the ingest-frame skill for the reboot catalog`: `8c413a1`
- **Done and not done:** C1 (`A1`, `T1`), C2 (`A2`–`A4`, `T2`), C3 (`S1`–`S2`), `P` (`P1`–`P5`), and Verification steps 1–6 are all done. Nothing is left undone.
- **Deviations:**
  - In `A2` (`internal/reboot/web/compare_page.go`), the form view struct is named `compareFormValues`, not `compareFormView` as written in the plan; added `FrameCount int` to `compareFormValues`.
  - In `P3`, `TestScratchLiveChart` evaluated six Tuning Chip/precision transcriptions on temporary copies (literal nominal HTML, chart-image `0.1°` HTA precision, `450 mm` max-CS and `430 mm` min-CS under both `±0.25°` directions) and checked Otso's `51 mm` Geometry-table offset alongside `44`, `52`, and `54 mm`.
- **Verification:**
  - `make pre-commit` passed after C1 (`210ec1d`), C2 (`56f5205`), and C3 (`8c413a1`). `git diff` over `internal/reboot/catalog/frame*.go` and `forks.go` changed only the single `Frames()` doc comment line in `frame.go` (in C2), and `git status` had zero scratch files before committing C3.
  - After C1: `go test -run 'Legacy|MissingLegacy' -v ./internal/reboot/catalog` ran `TestMissingLegacyFrames` (all 3 cases passed: `identical`, `catalog-only frame`, `dropped frame`) and `TestFramesMatchLegacy` (all 19 subtests ran and passed with 0 skips).
  - C2 snapshots (`/` and `/?frame=otso-fenrir-stainless&size=L&fork=lithic_carbon_mountain&sag=0`, `/bike`, `bikegeo-reboot compare`, and `bikegeo-reboot bike`): `diff -u` on `/bike` HTML, `compare` JSON, and `bike` JSON was empty (byte-identical); `/` and the custom `/` query differed only on `[data-note="bar-options-speed"]` (`(~35 s across all 19 frames)` → `(about 2 s per frame)`).
  - `go run ./cmd/bikegeo-reboot compare -h` prints `every catalog frame`.
  - **P1 URLs visited and read:**
    - `https://otsocycles.com/pages/bike-specs-tables` (`Fenrir Stainless Frame` specs: `ZS44/28.6` upper, `EC44/40.0` lower; fork A2C `490mm (suspension-corrected for 100mm suspension fork)`, offset `44mm or 52mm`; seatpost insertion depths `X-Small 170mm; Small 210mm; Medium 260mm; Large 290mm; X-Large 330mm`; tire clearance `2.3" at 420mm` / `2.8" (27.5") or 2.6" (29") at 430/440mm`).
    - `https://otsocycles.com/collections/fenrir-stainless`, `https://otsocycles.com/products/fenrir-stainless-frameset?_pos=18&_sid=10244ef9f&_ss=r`, and `https://otsocycles.com/products/fenrir-stainless-drop-bar` (identical HTML `Geometry` table across XS–XL; `Frame Specs` on `/collections/fenrir-stainless` and `/products/fenrir-stainless-frameset` says `490mm (suspension-corrected for 120mm suspension fork)` instead of `100mm`).
    - `https://otsocycles.com/cdn/shop/products/Otso-Fenrir_size_chart_69d560f9-1ee7-4bff-9bbc-c0a9cad269ce.jpg?v=1664562021&width=2400` (product-gallery geometry image: matches HTML table except `68.0°` `(Rigid and 100 mm suspension fork)` and Tuning Chip `±0.2°`).
    - Also checked `/search?q=fenrir`, `/search?q=fenrir+stainless`, `/collections/bikepacking-bikes`, `/collections/fenrir-ti`, `/collections/fenrir-steel`, and `/pages/tech-documents-and-manuals`.
  - **P2 catalog stand-in (`TestScratchBasis`, rake `54 mm`):** all readouts matched `SLICE16b.md` within tolerance:
    - Implied basis A2C XS / S / M / L / XL: `494.63 / 495.40 / 494.78 / 495.23 / 494.61 mm` (want `494.63 / 495.40 / 494.78 / 495.23 / 494.61 mm`).
    - Axle-height budgets XS / S / M / L / XL: `±2.04 / ±2.04 / ±2.06 / ±2.08 / ±2.09 mm` (want `±2.04 / 2.04 / 2.06 / 2.08 / 2.09 mm`).
    - Implied range, minimax: `494.61–495.40 mm, 495.00 mm` (rounded `a2c = 495.0`).
    - A2C passing range at rake `54`: `493.20–496.80 mm`; rake passing range at A2C `495`: `51.15–56.90 mm`.
    - `A2C 495.0, rake 44`: `passes false`, worst axle-height excess `+2.02 mm`, worst wheelbase excess `+6.60 mm`.
    - `A2C 495.0, rake 52`: `passes true`, `-0.96 mm`, `-0.82 mm`.
    - `A2C 495.0, rake 54`: `passes true`, `-1.69 mm`, `-2.68 mm`.
  - **P3 live transcription (`TestScratchLiveChart`):** ran across offsets `44, 51, 52, 54 mm`; full diffs, implied ranges, and reading-rule conclusions are recorded under **Open issues** below.
  - **P4 skill pin/test check:** verified every file and test name referenced in S1 step 5 and step 6 (`TestFramesRegistry`, `TestForkListsPlace`, `TestChartAllowances`, `TestEveryChartIsConsistent`, `TestFrameStructure`, `TestForks`, `TestCatalogFrames`, `TestParseCompareDefaultAll19Frames`, `TestCompareDefaultAll19Frames`, `TestBikeMultiFrameSelectAndSwitching`, and `.claude/launch.json` `"reboot"`).
  - **Live check (`http://127.0.0.1:8081/`, 0 console errors):**
    - On `/`: header badge `"All 19 frames"`, mode note `"Comparing all 19 catalog frames (best configuration each). Add a frame below to narrow to custom selections."`, speed note `"All 15 handlebars evaluates ~15× more builds (about 2 s per frame)."`. Screenshot: `/Users/matt/.gemini/antigravity/brain/8fcd79bf-1f2a-4a55-bc4f-09dc3c0be068/scratch/compare_home.png`.
    - After selecting `Otso Fenrir Stainless` in `#ctrl-add-frame` and submitting `#ctrl-submit`: `a#preset-all-frames` rendered `"All 19 Frames"`. Screenshot: `/Users/matt/.gemini/antigravity/brain/8fcd79bf-1f2a-4a55-bc4f-09dc3c0be068/scratch/compare_submitted.png`.
- **Open issues:**
  - **P3 live Otso Fenrir Stainless chart findings for Matt (Q123 — no catalog data changed):**
    - **Catalog vs. live chart differences (`size / column / catalog / live`):**
      - `XS–XL / Reach`: `387 / 403 / 416 / 428 / 441 mm` vs. `387 / 403 / 416 / 428 / 441 mm` (identical).
      - `XS–XL / Stack`: `595 / 605 / 623 / 642 / 660 mm` vs. `595 / 605 / 623 / 642 / 660 mm` (identical).
      - `XS–XL / HTL`: `90 / 100 / 120 / 140 / 160 mm` vs. `90 / 100 / 120 / 140 / 160 mm` (identical).
      - `XS–XL / Seat tube length`: `360 / 400 / 450 / 490 / 530 mm` vs. `360 / 400 / 450 / 490 / 530 mm` (identical).
      - `XS–XL / HTA`: `68.25°` vs. `68°` (HTML) / `68.0°` (image) nominal + separate Tuning Chip row `±0.25°` (HTML) / `±0.2°` (image) from middle position.
      - `XS–XL / STA`: `75.25°` vs. `75°` nominal + Tuning Chip `±0.25°` (image `±0.2°`).
      - `XS–XL / BB drop`: `72 mm` vs. `68-72mm` range.
      - `XS–XL / Chainstay`: `450 mm` (of `430–450 mm`) vs. `430-450mm` range in `Geometry` table (and `420/430/440mm` in `Frame Specs` tire-clearance row).
      - `XS / Wheelbase`: `1098 mm` vs. `1090-1110mm`.
      - `S / Wheelbase`: `1118 mm` vs. `1110-1120mm` (10 mm width, vs. 20 mm on XS/M/L/XL).
      - `M / Wheelbase`: `1138 mm` vs. `1130-1150mm`.
      - `L / Wheelbase`: `1158 mm` vs. `1150-1170mm`.
      - `XL / Wheelbase`: `1178 mm` vs. `1170-1190mm`.
      - `Basis fork offset`: `54 mm` vs. `51mm` in `Geometry` table (`44mm or 52mm` in `Frame Specs` and `/pages/bike-specs-tables`).
      - `Basis fork A2C`: `495 mm` (matches `Geometry` table `Rigid fork axle to crown length: 495mm`) vs. `Frame Specs` `490mm (suspension-corrected for 120mm suspension fork)` (`/pages/bike-specs-tables` says `100mm suspension fork`).
      - `XS / S / M / L / XL / MaxPostInsertionMm`: `0 / 0 / 0 / 290 / 0 mm` vs. `170 / 210 / 260 / 290 / 330 mm` published in `Frame Specs` and `/pages/bike-specs-tables` (Q95 follow-up candidate).
      - `Headset`: live `Frame Specs` explicitly states `ZS44/28.6 upper, EC44/40.0 lower`, confirming the catalog's physical `Bore44Pressed` + `EC44`.
    - **P3 `TestScratchLiveChart` sweep results (`44 / 51 / 52 / 54 mm` offsets):**
      1. *Nominal HTML (`68°`, `75°`, `BB drop 70`, `CS 440`, mid WB `1100/1115/1140/1160/1180`, `1°` → `0.5°` HTA precision):* implied A2C `498.05–498.83 mm`, minimax `498.44 mm` (rounded `498.5 mm`, axle-height budgets `±3.78–4.01 mm`). No A2C in `488.5–508.5 mm` passes at `51 mm` rake; at `498.5 mm`, `55.90–63.70 mm` rake passes (`498.5` fails at `44/51/52/54`: WB excess `+10.95/+4.48/+3.56/+1.71 mm`). At `495.0 mm` (`51 mm` rake), axle height passes (`-1.41` to `-2.24 mm` excess), but wheelbase fails every size (`-7.43` to `-12.44 mm` mismatch, `+0.99` to `+5.82 mm` excess).
      2. *Nominal image precision (`68.0°` → `0.05°` HTA precision, budgets `±1.69–1.72 mm`):* implied A2C `498.05–498.83 mm`, minimax `498.44 mm`. No basis passes in `488.5–508.5 mm` A2C (`51 mm` rake) or `36–66 mm` rake (`498.5 mm` A2C); `495/51` fails both AH (`+0.73 mm` excess) and WB (`+10.17 mm` excess).
      3. *450 mm max-CS at `68.25° / 75.25° / BB drop 72`, max WB `1110/1120/1150/1170/1190`, `0.125°` HTA precision:* recovers the catalog's exact implied A2C `494.61–495.40 mm`, minimax `495.00 mm` (budgets `±2.04–2.09 mm`), so `495.0 mm` passes axle height at `51/52/54 mm` (`-0.59/-0.96/-1.69 mm`) and fails at `44 mm` (`+2.02 mm`), **but** wheelbase fails everywhere (`+18.60/+12.10/+11.18/+9.32 mm` for `44/51/52/54`; per-size at `495/51`: WB mismatch `-14.65/-4.95/-14.54/-15.12/-14.71 mm`, excess `+11.73/+2.01/+11.56/+12.10/+11.65 mm`).
      4. *450 mm max-CS at `67.75° / 74.75° / BB drop 68`, max WB:* implied A2C `501.49–502.29 mm`, minimax `501.89 mm` (`502.0 mm`); fails `495.0 mm` axle height (`+0.92` to `+4.69 mm`) and fails WB everywhere (`+2.81` to `+13.02 mm`).
      5. *430 mm min-CS (`min WB 1090/1110/1130/1150/1170`):* under `68.25°/72`, implied A2C `494.61–495.40 mm` (`495.00 mm` minimax), WB fails everywhere (`+18.87/+12.37/+11.45/+9.59 mm` for `44/51/52/54`); under `67.75°/68`, implied A2C `501.49–502.29 mm` (`501.89 mm` minimax), at `502.0 mm` rake `55.70–58.75 mm` passes (`44/51/52/54` fail WB `+10.78/+4.31/+3.39/+1.54 mm`).
    - **S1 reading-rule conclusion:** Otso's live Fenrir Stainless chart is a **different, coarser chart revision** — single-position columns were replaced with Tuning Chip ranges (`68° ± 0.25°`, `75° ± 0.25°`, `68-72mm` BB drop, `430-450mm` CS, and 10-mm-rounded wheelbase ranges with an apparent `1110-1120mm` S typo) plus internal discrepancies between `Geometry` (`495 mm` A2C, `51 mm` offset, `430-450 mm` CS) and `Frame Specs` (`490 mm` A2C for `100 mm`/`120 mm` suspension fork, `44 mm or 52 mm` offset, `420/430/440 mm` CS). The existing catalog's single-position chart (`68.25° / 75.25° / 72 mm / 450 mm / 1098–1178 mm`) stays consistent at `495 mm` A2C, `54 mm` offset, bare head tube; none of the coarse live-range transcriptions pass wheelbase on a `495 mm` basis without Matt's ruling. Separately, Otso now publishes all five sizes' `MaxPostInsertionMm` (`170/210/260/290/330 mm`).

Implement this plan exactly. If the code disagrees with it, or a design choice comes up that the plan doesn't cover, stop. Record `Status: Blocked <date> on <question>` and the Implementation handoff, then tell Matt to switch to a Frontier model. "Seemed cleaner" is not a reason to deviate.

## Goal

Make `internal/reboot/catalog` the only place a new frame goes, with a written workflow that needs no legacy command. After this slice:

- a frame that exists only in the catalog passes the legacy parity test;
- `/` and the CLI usage text no longer hard-code "19";
- the ingest-frame skill describes the reboot workflow: transcribe the chart, find the basis with a temporary test, update the pins, and pass the catalog `go test`;
- one proof run on the Otso Fenrir shows the skill works as written.

No catalog data changes. The only user-visible change is the wording of one `/` field note (D3).

## Rulings this rests on

- **Q114:** no ingestion subcommand. Agents answer ingestion questions with temporary tests in `internal/reboot/catalog` that call `assembly.PlaceUnpitchedFrameset` and `measureChartConsistency`. A helper that temporary tests keep rewriting gets promoted.
- **Q119:** `frames_parity_test.go` checks only that every legacy frame is in the catalog and matches. A catalog-only frame passes.
- **Q122:** the proof frame is the Otso Fenrir Stainless.
- **Q123:** the proof transcribes Otso's live chart. Differences are reported, not fixed.
- **Q124:** UI frame counts come from `catalog.Frames()`. Test pins stay, and the skill lists them.
- **Unchanged:** Q44/Q81/Q121 (the basis is a hypothesis; only the cup + A2C sum is determined), Q45/Q57/Q93 (rounding budget, unpublished wheelbase, allowance rules), Q80 (precision from published digits), Q91 (the check runs only in `go test`), and AGENTS.md "One-off investigations".

## Planner defaults (Matt may veto on review)

- **D1. Parity relaxation.** One new helper, `missingLegacyFrames`, does the "every legacy frame is in the catalog" check and gets its own test. The two existing tests loop over the catalog and skip frames with no legacy counterpart.
- **D2. Count source.** Add one `FrameCount int` field to `compareFormView`, set at both of its builders. The Go-built title, badge, and mode label use `len(catalog.Frames())` directly. The function and usage docs say "every catalog frame" rather than a number.
- **D3. The speed note.** "~35 s across all 19 frames" was measured at 19 frames (Q104) and would go stale with a derived count. It becomes "(about 2 s per frame)": 36 s / 19 frames ≈ 1.9 s, so it stays true as frames are added. This is the slice's only change to rendered output.
- **D4. The skill is written here verbatim** (S1 below), including the basis-finding test template. The planner ran that template on the Fenrir. It compiles, and the T-values in P2 are its output. The implementer copies it exactly.
- **D5. AGENTS.md.** Delete the interim bullet. Replace the legacy "Frame Datasets" section's rules with a pointer to the skill, since after this slice they describe a workflow nobody may use (Q119). Slice 20 still folds the section away.
- **D6. Commit order.** C1 and C2 are code. C3 is Markdown, but the proof (P) runs on the C3 draft *before* C3 is committed, so a skill that fails its proof never lands.
- **D7. No pin changes.** The proof adds no frame, so every count pin keeps its value. The skill tells future agents where the pins are.

## C1: Let catalog-only frames pass the parity test

Subject: `Let catalog-only frames pass the parity test`

**A1. `internal/reboot/catalog/frames_parity_test.go`**

- `legacyFramesByID`: delete the `len(out) != len(catalog.Frames())` check and its `t.Fatalf`. It still returns the map. Drop the `t *testing.T` parameter if nothing else in it needs `t` (it won't), and update both callers.
- Add:
  ```go
  // missingLegacyFrames returns, in legacy order, the IDs of legacy frames that
  // frames lacks (Q119). A catalog-only frame is not an error.
  func missingLegacyFrames(legacy []model.FrameModel, frames []catalog.Frame) []string {
  	have := map[string]bool{}
  	for _, f := range frames {
  		have[f.ID] = true
  	}
  	var missing []string
  	for _, l := range legacy {
  		if !have[l.ID] {
  			missing = append(missing, l.ID)
  		}
  	}
  	return missing
  }
  ```
- `TestFramesMatchLegacy`: at the top, `if missing := missingLegacyFrames(legacyframes.AllFrames(), catalog.Frames()); len(missing) > 0 { t.Fatalf("legacy frames missing from the catalog: %v", missing) }`. In the loop, replace `t.Fatalf("no legacy frame %q", f.ID)` with `t.Skipf("catalog-only frame (Q119)")`. The subtest name stays `f.ID`.
- `TestPrecisionMatchesLegacyInference`: look up `l, ok := legacy[f.ID]` and `continue` when `!ok`. Pass `l` to `InferChartPrecision`.
- Update the file's test doc comments to mention Q119 in one clause each.

**T1. Same file:** add `TestMissingLegacyFrames`. Build its inputs from the real lists:

| Case | `legacy` | `frames` | Want |
| --- | --- | --- | --- |
| identical | `legacyframes.AllFrames()` | `catalog.Frames()` | `nil` |
| catalog-only frame | same | `append(catalog.Frames(), catalog.Frame{ID: "scratch-new-frame"})` | `nil` |
| dropped frame | same | `catalog.Frames()` without `otso-fenrir-stainless` (use `slices.DeleteFunc` on a clone) | `["otso-fenrir-stainless"]` |

Compare with `slices.Equal`.

## C2: Derive frame counts in / labels from the catalog

Subject: `Derive frame counts in / labels from the catalog`

**A2. `internal/reboot/web/compare_page.go`**

- `compareFormView` gains `FrameCount int`, placed after `HandlebarCount`.
- Set `FrameCount: len(catalog.Frames())` in both builders, next to each `HandlebarCount:` line (around lines 383 and 528).
- In the all-frames branch (around line 280), compute `n := len(catalog.Frames())` once:
  - `pageTitle = fmt.Sprintf("Compare %d Frames — bikegeo-reboot", n)`
  - `headerBadge = fmt.Sprintf("All %d frames", n)`
  - `modeLabel = headerBadge`

  The output is identical today.

**A3. `internal/reboot/web/templates/compare.html`**

- Line 641: `all 19 catalog frames` → `all {{.Form.FrameCount}} catalog frames`.
- Line 748: `(~35 s across all 19 frames)` → `(about 2 s per frame)` (D3).
- Line 783: `All 19 Frames` → `All {{.Form.FrameCount}} Frames`.

**A4. Docs that state the count**

- `internal/reboot/catalog/frame.go:87`: `// Frames returns every catalog frame in ascending ID order (TestFramesRegistry pins the list).`
- `internal/reboot/query/compare.go:379`: `"All 19 frames" mode` → `all-frames mode`.
- `cmd/bikegeo-reboot/main.go:33`: `size, fork, and sag of all 19 frames is ranked` → `size, fork, and sag of every catalog frame is ranked`. Keep the line breaks readable: re-wrap that one paragraph only.

**T2. `internal/reboot/web/compare_test.go`:** in the default-page test that already asserts the title (around line 31), add:

- `[data-compare-mode="all"]` text contains `all 19 catalog frames`;
- `[data-note="bar-options-speed"]` text, `strings.TrimSpace`d, equals `All 15 handlebars evaluates ~15× more builds (about 2 s per frame).` (15 is `len(catalog.Handlebars())` today; build the string with `fmt.Sprintf` from that call, not a literal 15).

In the multi-selection test near line 271, assert `a#preset-all-frames` text is `All 19 Frames`. The existing title pin `Compare 19 Frames — bikegeo-reboot` stays.

## C3: Rewrite the ingest-frame skill for the reboot catalog

Subject: `Rewrite the ingest-frame skill for the reboot catalog`

Draft S1 and S2 first, then run P, then commit C3 (D6). `.claude/skills` is a tracked symlink to `.agents/skills`, so edit only `.agents/skills/ingest-frame/SKILL.md`.

**S1. Replace `.agents/skills/ingest-frame/SKILL.md` entirely with:**

````markdown
---
name: ingest-frame
description: Ingest a new bike frame or geometry chart into the reboot catalog (internal/reboot/catalog) — transcribe the published chart, find its basis with a temporary Go test, update the catalog-size pins, and pass the catalog go test.
---

# Frame ingestion

New frames go into `internal/reboot/catalog` only. Legacy `internal/frames` is frozen and `bikegeo fit`/`validate` are not used (DESIGN.md Q114, Q119). Every question a chart raises is answered with a temporary Go test, following AGENTS.md "One-off investigations".

## Rules

1. **Published numbers are ground truth.** Transcribe them exactly. Never nudge a value to make chart consistency pass. Leave an unpublished wheelbase at 0 (Q57) rather than deriving it. Correcting a typo needs a gross, documented error (a transposed digit, a value off by more than 5 mm) and Matt's ruling.
2. **Label every value's provenance** in a comment: published, catalog, measured, owner-reported, illustrative, or hypothesis. Record the chart's source URL in the frame's doc comment.
3. **The basis is a hypothesis.** `BasisFork` (static A2C with sag included, rake, `assembly.SteererBasis`, no travel) and `BasisLowerCup` are the front end the chart's numbers are computed on (Q44, Q121). Chart consistency shows agreement, not uniqueness: any cup + A2C pair with the same sum fits. Prefer the basis the manufacturer states. A sagged basis A2C is the fork's A2C minus sag × travel (a 530 mm, 120 mm fork at 20% is 506 mm).
4. **An allowance is a last resort.** A `ChartAllowance` must be needed (the size fails without it), at most 0.5 mm beyond the excess it covers, and have a `Reason` (Q93). Never use one to cover a wrong basis: the wrong fork, sag, cup, or rake.
5. **No scripts, no restated geometry.** Call `measureChartConsistency` and the placement functions. Change only copies of catalog values inside a temporary test.

## Recipe

### 1. Gather the chart

For each size, the manufacturer's reach, stack, head tube angle, seat tube angle, head tube length, BB drop, chainstay, wheelbase, seat tube length, and (if published) maximum seatpost insertion. For the frame, the fork it is designed around (A2C, offset, travel, sag), the headset standard and lower cup, and the chainstay range for sliding dropouts. Seat tube length is center to top; add the collar offset to a center-to-center figure and say so (Q75d). Note which chainstay position the chart is drawn at.

### 2. Write `internal/reboot/catalog/frame_<id>.go`

Copy the shape of `frame_otso_fenrir_stainless.go`:

- `frameFamily` holds the physical `LowerBore` and `DefaultLowerCup`. `family.sizes(publishedSize{...}, ...)` holds each size's published columns, plus an `Allowance` only if step 3 proves one is needed.
- `ChartPrecision` is per column: half of the coarsest step that every size's value sits on. The length steps are 1, 0.5, 0.1, and 0.01 mm. The angle steps are 1, 0.5, 0.25, 0.1, 0.05, and 0.01°. A head angle of 68.25 in every size is on a 0.25° step, so its half-width is 0.125. Head angles of 68 and 68.5 give 0.25, and whole millimeters give 0.5 (Q80). Use the printed digits ("70.0°" is a 0.1° step) when a single size gives too little to judge.
- `Forks` lists the catalog forks the frame supports, each at this frame's A2C and rake. Add a `Fork` to `forks.go` only for a real fork model, and update `TestForks`'s count.
- `MaxChainstayMm` is the rearmost sliding-dropout position (Q67), or 0 for fixed dropouts.
- Start `BasisFork` and `BasisLowerCup` from the manufacturer's stated basis, or from a guess with the bare head tube (`assembly.LowerCupBareHeadTube`, stack 0).
- Register the frame in `Frames()` in `frame.go`, in ID order.

`TestEveryChartIsConsistent` and the pins in step 5 fail until steps 3–5 are done. That's expected.

### 3. Find the basis with a temporary test

Save this as `internal/reboot/catalog/scratch_basis_test.go`, set the frame ID and the published rake, and run `go test -run TestScratchBasis -v ./internal/reboot/catalog`:

```go
package catalog_test

import (
	"math"
	"testing"
)

// TestScratchBasis finds each size's implied basis A2C and the basis A2C and
// rake ranges where every size passes chart consistency.
func TestScratchBasis(t *testing.T) {
	f := mustFrame(t, "<frame-id>")
	f.BasisFork.StaticA2cMm = 480 // a starting guess; the solve doesn't depend on it

	// Each size's implied basis A2C, where its axle-height mismatch is zero.
	// The mismatch is linear in A2C, so two placements 1 mm apart solve it.
	lowest, highest := math.Inf(1), math.Inf(-1)
	for _, s := range f.Sizes {
		up := f
		up.BasisFork.StaticA2cMm++
		at, err := measureChartConsistency(f, s)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		above, err := measureChartConsistency(up, s)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		m0, m1 := at.Checks[0].MismatchMm, above.Checks[0].MismatchMm
		implied := f.BasisFork.StaticA2cMm - m0/(m1-m0)
		lowest, highest = min(lowest, implied), max(highest, implied)
		t.Logf("%s: implied basis A2C %.2f mm (axle-height rounding budget ±%.2f mm)", s.Name, implied, at.Checks[0].BudgetMm)
	}
	t.Logf("implied basis A2C %.2f–%.2f mm, minimax %.2f mm", lowest, highest, (lowest+highest)/2)

	// passes reports whether every size passes with this basis A2C and rake, and
	// each check's worst excess over its rounding budget (≤ 0: rounding explains it).
	passes := func(a2c, rake float64) (ok bool, axleExcess, wheelbaseExcess float64) {
		g := f
		g.BasisFork.StaticA2cMm, g.BasisFork.RakeMm = a2c, rake
		ok, axleExcess, wheelbaseExcess = true, math.Inf(-1), math.Inf(-1)
		for _, s := range g.Sizes {
			m, err := measureChartConsistency(g, s)
			if err != nil {
				t.Fatalf("%s at A2C %v, rake %v: %v", s.Name, a2c, rake, err)
			}
			if m.Err() != nil {
				ok = false
			}
			axleExcess = max(axleExcess, m.Checks[0].ExcessMm)
			if len(m.Checks) > 1 { // no wheelbase check when it is unpublished
				wheelbaseExcess = max(wheelbaseExcess, m.Checks[1].ExcessMm)
			}
		}
		return ok, axleExcess, wheelbaseExcess
	}
	// scan logs the passing range of one value with the other held fixed.
	scan := func(name string, from, to, step float64, at func(v float64) (float64, float64)) {
		first, last := math.NaN(), math.NaN()
		for v := from; v <= to+step/2; v += step {
			if ok, _, _ := passes(at(v)); ok {
				if math.IsNaN(first) {
					first = v
				}
				last = v
			}
		}
		t.Logf("%s: every size passes from %.2f to %.2f (NaN: nowhere in %v–%v)", name, first, last, from, to)
	}

	a2c := math.Round((lowest+highest)/2*2) / 2 // minimax, to 0.5 mm
	const rake = 54                              // the published or stated offset
	scan("basis A2C at the rake", a2c-10, a2c+10, 0.05, func(v float64) (float64, float64) { return v, rake })
	scan("rake at the basis A2C", rake-15, rake+15, 0.05, func(v float64) (float64, float64) { return a2c, v })
	for _, r := range []float64{44, 52, 54} {
		ok, ae, we := passes(a2c, r)
		t.Logf("A2C %.1f, rake %.0f: passes %v, worst axle-height excess %+.2f mm, worst wheelbase excess %+.2f mm", a2c, r, ok, ae, we)
	}
}
```

Read the output this way:

- **A stated basis that passes wins**, even if it isn't the minimax value. Record it as stated.
- **With no stated basis**, use the minimax A2C rounded to 0.5 mm, and record it as inferred from the published stacks, with the implied range (as `frame_ritchey_ultra.go` does).
- **A spread of implied A2Cs wider than the axle-height budgets, or one size far from the rest,** points to a transcription error, a typo in the chart, or a cup the chart doesn't state. Investigate it; don't cover it with an allowance.
- **A wheelbase check that fails at the published offset** but passes at another means the chart may be drawn for a different fork. Record what you found and ask Matt.
- **Only if one size still fails** with the right basis does it get an allowance. Log that size's `m.Checks` (each check's `MismatchMm`, `Terms`, `BudgetMm`, and `ExcessMm`) and size the allowance from `ExcessMm`, within rule 4.

Change `{44, 52, 54}` to the offsets worth comparing for this chart. Delete the file when you're done.

### 4. Apply the basis

Set `BasisFork` and `BasisLowerCup` in the frame file, each with a `hypothesis (Q44, Q81):` comment that says how it was found. Add any allowance from step 3 with its `Reason`.

### 5. Update the catalog-size pins

These tests pin the catalog's size on purpose, so a dropped frame can't go unnoticed. Adding a frame changes them. Run `go test ./internal/reboot/...`, and set each one from the failure message after you've checked the new count is what the frame should add:

- `internal/reboot/catalog/frames_test.go`: `TestFramesRegistry` (the ID-to-size-count map, the frame count, and the size total), `TestForkListsPlace` (fork-list and basis placement counts), and `TestChartAllowances` (add the size if it declares an allowance).
- `internal/reboot/catalog/parts_test.go`: `TestForks`, only if you added a fork.
- `internal/reboot/query/compare_test.go`: the size total, and `TestParseCompareDefaultAll19Frames` (row count, builds evaluated, and rows). Keep the test's name.
- `internal/reboot/web/compare_test.go`: the default page's title and its configs summary (frames, rows, and builds evaluated).
- `internal/reboot/web/web_test.go`: the `/bike` frame `<select>` option counts (`TestBikeMultiFrameSelectAndSwitching` and the error page's).

The `/` page's labels count the catalog on their own (Q124). Don't edit DESIGN.md's history to match. Q-rulings record the catalog as it was when they were ruled.

### 6. Verify

- `go test ./internal/reboot/catalog` passes, including `TestEveryChartIsConsistent` and `TestFrameStructure`. The legacy parity test skips the new frame (Q119).
- `make pre-commit` passes.
- In the `reboot` preview, `/bike?frame=<id>` and `/bike?frame=<id>&fork=basis` render, and the Chart Basis Fork readout's wheelbase is close to the published one. The new frame appears on `/`.
````

**S2. `AGENTS.md`**

- "One-off investigations": delete the bullet that starts `Interim, until Slice 16b rewrites the ingest-frame skill:`. In its place add: `- For a new frame chart, follow the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill; its basis-finding test is the template for chart questions.`
- "Frame Datasets (`internal/frames/*.go`)": replace the whole section (heading, intro line, and its four bullets) with:
  ```markdown
  ## Frame Datasets

  Legacy `internal/frames` is frozen: new frames go into `internal/reboot/catalog` only, following the [ingest-frame](.agents/skills/ingest-frame/SKILL.md) skill (DESIGN.md Q114, Q119). Published chart numbers are never nudged, and a chart allowance must be necessary and give its reason.
  ```
- Leave every other section alone, including the Reboot Tree "Catalog" bullet and the legacy Architecture bullets (Slice 20 owns them).

## P: Prove the workflow on the Otso Fenrir (between drafting and committing C3; no commit)

Follow the S1 skill as written. Don't add steps it doesn't name. Put all proof code in one gitignored `internal/reboot/catalog/scratch_basis_test.go`.

**P1. Find the live chart (Q123).** Locate Otso's current Fenrir Stainless geometry table and record each URL you read. Leads from planning:

- `https://otsocycles.com/pages/bike-specs-tables` has a "Fenrir Stainless Frame" spec list with no geometry table. It gives "Fork axle-to-crown: 490mm (suspension-corrected for 100mm suspension fork)", "Fork offset: 44mm or 52mm", insertion depths (L 290 mm, matching the catalog), and tire clearances at 420/430/440 mm chainstay positions. The catalog has a 430–450 mm range, drawn at 450.
- Try Otso's Fenrir Stainless product page next.
- If Otso publishes no table you can read as text, use a retailer page that reproduces Otso's table and say so. A search during planning showed Campfire Cycling's "otso fenrir stainless flat bar" page with a nominal 68° head angle, 75° seat angle, and an M wheelbase range of 1130–1150 mm. The catalog has 68.25°, 75.25°, and 1138 mm.
- If no source gives a full per-size table, skip P3, and record that under Open issues with the URLs tried.

**P2. Catalog stand-in (recipe check).** Run the S1 template unchanged on `mustFrame(t, "otso-fenrir-stainless")` with rake 54. The template's starting guess of 480 hides the catalog's 495. The planner's probe measured these values. A difference beyond the tolerance is a blocker.

| Readout | Expected | Tolerance |
| --- | --- | --- |
| Implied basis A2C XS / S / M / L / XL | 494.63 / 495.40 / 494.78 / 495.23 / 494.61 mm | ±0.01 |
| Axle-height budgets XS / S / M / L / XL | ±2.04 / 2.04 / 2.06 / 2.08 / 2.09 mm | ±0.01 |
| Implied range, minimax | 494.61–495.40 mm, 495.00 mm | ±0.01 |
| Rounded minimax `a2c` | 495.0 (the catalog's `BasisFork.StaticA2cMm`) | exact |
| A2C passing range at rake 54 | 493.20–496.80 mm | ±0.05 |
| Rake passing range at A2C 495 | 51.15–56.90 mm | ±0.05 |
| A2C 495, rake 44 | fails; worst axle-height excess +2.02 mm, worst wheelbase excess +6.60 mm | ±0.01 |
| A2C 495, rake 52 | passes; −0.96, −0.82 mm | ±0.01 |
| A2C 495, rake 54 | passes; −1.69, −2.68 mm | ±0.01 |

So the recipe recovers the catalog's basis, 495 mm with the bare head tube at 54 mm rake. It also shows that Otso's listed 44 mm offset can't be the chart's basis rake.

**P3. Live transcription.** In the same file, add `TestScratchLiveChart`. It copies the catalog Fenrir, replaces each size's published columns with your transcription (only on the copy, per rule 5), and sets `ChartPrecision` from the transcribed digits by S1 step 2. Then run the S1 template's logic on that copy, by copying the template body into a helper that takes the frame, for each offset the chart or Otso states (44, 52) and for 54. Record:

- every column where the transcription differs from the catalog, as size/column/catalog/live;
- the implied A2C range and minimax;
- whether any basis passes every size at each offset, and the passing ranges;
- what the skill's reading rules conclude (for example "the live chart is a different revision: …").

Change nothing in the catalog. Every finding goes under Open issues for Matt (Q123).

**P4. Skill defects.** If the S1 text is wrong in a mechanical way, fix it in the S1 draft and list each fix under Deviations. Mechanical means the template doesn't compile, it names a test or file that doesn't exist, or it gets a pin location wrong. If the method itself fails (the recipe can't recover 495 mm, or a reading rule gives a wrong conclusion), stop and record Blocked.

**P5.** Delete `scratch_basis_test.go`. Confirm `git status` shows no scratch file, then commit C3.

## Rules that bite

- No catalog data changes: no frame, fork, size, precision, basis, or allowance value moves. `git diff` over `internal/reboot/catalog/frame*.go` and `forks.go` must be empty, except the one `Frames()` doc line in A4.
- Count pins keep their values (D7). If one changes, something else changed, and that's a blocker.
- Rendered output from `/` and `/bike` is byte-identical before and after C2 except for the D3 speed note. CLI JSON output doesn't change.
- Temporary tests follow AGENTS.md "One-off investigations". No Python, standalone Go, or restated geometry. No `math` trig in test code: the template uses only `min`, `max`, `Round`, `Inf`, and `NaN`.
- Don't touch legacy code or data (`internal/frames`, `cmd/bikegeo`), `docs/RIDER_FIT.md`, the Ritchey Ultra (Slice 18b, Q125), or anything in Slice 17's scope (`FitOval.Validate`, `catalog.Forks`, dropped legacy Notes).
- `make boundary` must pass. Watch the banned words in new reboot text: no "CAD", "closure", "cost", or "chassis".
- Each commit passes `make pre-commit` on its own.
- Finish with the Implementation handoff (DESIGN.md "Implementation handoff"): set this file's Status line, add the handoff section below it, and set the Slice 16b Milestones line in DESIGN.md to the same status words. Record P1's URLs, P2's table with the measured numbers, and P3's findings under Verification and Open issues.

## Verification

1. Run `make pre-commit` after C1, C2, and C3.
2. After C1: `go test -run 'Legacy|MissingLegacy' -v ./internal/reboot/catalog` lists `TestMissingLegacyFrames` with three passing cases, and `TestFramesMatchLegacy` runs 19 subtests with no skips.
3. Snapshot C2. Before C2, render `/`, `/?frame=otso-fenrir-stainless&size=L&fork=lithic_carbon_mountain&sag=0`, and `/bike` through a gitignored `internal/reboot/web/scratch_snapshot_test.go` into the scratchpad (as Slice 16a's C0 did), and save `go run ./cmd/bikegeo-reboot compare` and `go run ./cmd/bikegeo-reboot bike`. After C2, `diff` shows only the speed-note line in the two `/` pages, and nothing else. Delete the scratch test.
4. `go run ./cmd/bikegeo-reboot compare -h` shows "every catalog frame".
5. P2's table matches within tolerance, and P3's findings are recorded.
6. Live check: start the `reboot` preview. On `/`, confirm the badge "All 19 frames", the mode note "all 19 catalog frames", and the speed note "(about 2 s per frame)". Add one frame selection and confirm the "All 19 Frames" link. Record a screenshot path in the handoff.

## Out of scope

- Fixing any Fenrir data that P3 shows differs from the live chart (Q123: report only).
- Deleting or correcting the Ritchey Ultra (Slice 18b, Q125).
- A shared basis-sweep helper in catalog test code. Q114 promotes one only once temporary tests keep rewriting it.
- Renaming tests whose names contain "19" (for example `TestParseCompareDefaultAll19Frames`).
- Slice 17's `deadcode` and Notes audit, and Slice 20's AGENTS.md consolidation.
