# Slice 15: Chart-consistency rename

Status: Planned 2026-11-21

**Rulings:** Q117 (the words), Q91 (the check runs only in `catalog`'s `go test`), Q47 (no `/bike` readout); `AGENTS.md` rules.

**Builds on:** Slice 10 (commits `8b91fb1`, `2626d1f`), Slice 12b (commit `3b00a64`), and Slice 12c (commits `9942803`, `41f6123`, `425bdb8`, `0933899`), which added and wired the check being renamed.

**Size:** larger by file count (13 code and test files plus `AGENTS.md` and `DESIGN.md`) but one mechanical commit. Written so a lower-tier implementer can do it without choices: every identifier, error string, comment, and test name is listed below. **No behavior change.** Every number, tolerance, and check stays as it is; only words change. If committed code doesn't match a line below, stop and record `Blocked <date> on <question>` (see "Rules that bite").

## Goal

Rename the chart closure check to **chart consistency** across the reboot tree, per Q117. "Closure" becomes "consistency", and "residual" becomes "mismatch". "Rounding budget" stays. Slice 16 then moves the check and rewrites the ingest-frame skill using the new words.

## Planner defaults (veto on plan review)

- **D1, identifiers:** the map in R1 below. Q117 named `CheckChartConsistency` and `ChartAllowance`. The rest follow the same pattern: `chart` where the name says what is checked, and `consistency` where it names the check itself.
- **D2, error text:** the `chart closure:` prefix becomes `chart consistency:`, and `closure allowance:` becomes `chart allowance:`. The word `residual` becomes `mismatch`. These messages are the only way to read the numbers until Slice 16, so they use the new words now.
- **D3, Q47 guards in `web_test.go`:** delete the two assertions that `/bike` has no `residual-*` metric and no "Closure Residuals" text. The readout was removed in Slice 10 and its template no longer exists. Renaming the guards would check for strings that never existed, and keeping them would leave the only "closure" in reboot code.
- **D4, no `make boundary` ban:** "closure" can't be added to the banned terms, because the `/` page's `.disclosure` CSS class contains it. The grep checks in V2 enforce the rename for this slice. After that, Q117 is a review rule.
- **D5, catalog `Reason` strings** are prose, not published values, so they get reworded too (R3). Every chart number stays as it is.

## Commit 1: `Rename chart closure to chart consistency`

### R1. `internal/reboot/assembly/chart_closure.go` → `chart_consistency.go` (`git mv`)

| Old | New |
| --- | --- |
| `CheckChartClosure` | `CheckChartConsistency` |
| `ClosureAllowance` | `ChartAllowance` |
| `closureAllowanceSlackMm` | `chartAllowanceSlackMm` |
| `closureEpsilonMm` | `consistencyEpsilonMm` |
| `closureCheckNames` | `consistencyCheckNames` |
| `chartResiduals` | `chartMismatches` |
| locals `residuals`, `residual`, `ru`, `rd` | `mismatches`, `mismatch`, `mu`, `md` |

`ChartPrecision`, `chartColumns`, `FrameChart`, and the `Allowance` field keep their names.

Error format strings (the format verbs and everything else stay unchanged):

| Old prefix / wording | New |
| --- | --- |
| `"closure allowance: %s must be finite …"` | `"chart allowance: %s must be finite …"` |
| `"closure allowance: a non-zero allowance needs a reason"` | `"chart allowance: a non-zero allowance needs a reason"` |
| `"closure allowance: a reason without an allowance"` | `"chart allowance: a reason without an allowance"` |
| `"closure allowance: wheelbase allowance on a chart with no published wheelbase"` | `"chart allowance: wheelbase allowance on a chart with no published wheelbase"` |
| `"closure allowance: %s allowance %.2f mm is unnecessary: residual %+.2f mm is within …"` | `"chart allowance: %s allowance %.2f mm is unnecessary: mismatch %+.2f mm is within …"` |
| `"closure allowance: %s allowance %.2f mm exceeds the %.2f mm needed …"` | `"chart allowance: %s allowance %.2f mm exceeds the %.2f mm needed …"` |
| `"chart closure: %s residual %+.2f mm exceeds its ±%.2f mm %s (%s)"` | `"chart consistency: %s mismatch %+.2f mm exceeds its ±%.2f mm %s (%s)"` |

`"chart precision: …"` messages are unchanged.

Doc comments. Rewrite only the words named here, and keep each comment's meaning:

- `ChartPrecision`: "the closure checks read" → "the consistency checks read".
- `ChartAllowance`: "widens one size's consistency checks beyond what rounding of its published columns explains (Q65, Q71). CheckChartConsistency rejects … chartAllowanceSlackMm … Reason documents why the chart is inconsistent and is required whenever a field is non-zero."
- `chartAllowanceSlackMm`: unchanged apart from its name.
- `consistencyEpsilonMm`: "a residual may exceed its budget" → "a mismatch may exceed its budget".
- `consistencyCheckNames`: "names the two mismatches chartMismatches returns, in order."
- `chartColumns`: "the closure checks read" → "the consistency checks read".
- `CheckChartConsistency`: "checks that the chart's 0° mismatches with its CAD baseline fork are no larger than rounding of the published columns can explain (Q45). Each column's term is the larger change in a mismatch …". Keep "close to the measured excess (Q93)". It means *near*.
- `chartMismatches`: "returns the chart's 0°-pitch mismatches with its CAD baseline fork: …" (the formula sentence stays).

### R2. `internal/reboot/assembly/chart_closure_test.go` → `chart_consistency_test.go` (`git mv`)

- Rename the 11 tests `TestCheckChartClosure<Suffix>` → `TestCheckChartConsistency<Suffix>` (suffixes: `FenrirPasses`, `BudgetEdges`, `WithoutPrecision`, `InvalidPrecision`, `NudgeOutOfRange`, `InvalidWheelbase`, `HaydukeS4Allowance`, `SklarLAllowance`, `UnnecessaryAllowance`, `InvalidAllowance`, `UnpublishedWheelbase`).
- `requireChartClosure` → `requireChartConsistency`. Every `assembly.CheckChartClosure` → `assembly.CheckChartConsistency`, and every `assembly.ClosureAllowance` → `assembly.ChartAllowance`. That includes the `t.Fatalf` labels (`"CheckChartConsistency: got …"`).
- Every expected string: `chart closure:` → `chart consistency:`, `closure allowance:` → `chart allowance:`, ` residual ` → ` mismatch `. The numbers don't change. The 16 strings are at current lines 52, 70, 96 (two occurrences in one string), 216, 226, 240, 249, 263, 273–277, 300, and 305.

### R3. Other files

| File | Change |
| --- | --- |
| `internal/reboot/assembly/frameset.go` | `FrameChart.Allowance` type → `ChartAllowance` (realign the struct's comment column with `gofmt`). `PublishedWheelbaseMm` comment: "skips the wheelbase closure check" → "skips the wheelbase consistency check". |
| `internal/reboot/assembly/frameset_test.go` | Line 72 comment: "closes within chart rounding" → "is consistent within chart rounding". Lines 422–423 and 430: `CheckChartClosure` → `CheckChartConsistency` (call and message). |
| `internal/reboot/catalog/frame.go` | `publishedSize.Allowance` type → `assembly.ChartAllowance`. |
| `internal/reboot/catalog/frame_esker_hayduke.go` | `assembly.ChartAllowance{`. Reason: "S1–S3 close within rounding (Q71)." → "S1–S3 are consistent within rounding (Q71)." |
| `internal/reboot/catalog/frame_sklar_pbj.go` | `assembly.ChartAllowance{`. Reason: "every other size closes within rounding (Q71)." → "every other size is consistent within rounding (Q71)." |
| `internal/reboot/catalog/frame_otso_fenrir_stainless.go` | Doc comment: "The chart closes to the bare head tube;" → "The chart is consistent with the bare head tube;". |
| `internal/reboot/catalog/frame_salsa_fargo.go` | Doc comment: "The chart closes to the bare head tube;" → "The chart is consistent with the bare head tube;" (rewrap with the next line). |
| `internal/reboot/catalog/frames_test.go` | `TestEverySizeCloses` → `TestEveryChartIsConsistent`, with comment "runs the chart-consistency check on every size (Q91)." `TestClosureAllowances` → `TestChartAllowances`, with the comment unchanged apart from its name. All `ClosureAllowance` → `ChartAllowance` and `CheckChartClosure` → `CheckChartConsistency`. The two `errPrefix` values become `"chart consistency: axle height mismatch -3.06 mm exceeds its ±2.68 mm rounding budget"` and `"chart consistency: wheelbase mismatch +5.42 mm exceeds its ±4.31 mm rounding budget"`. |
| `internal/reboot/catalog/frames_parity_test.go` | Line 149: `assembly.ClosureAllowance{}` → `assembly.ChartAllowance{}`. |
| `internal/reboot/web/web.go` | `NewHandler` comment: "Chart closure runs in the catalog's go test" → "The chart-consistency check runs in the catalog's go test". |
| `internal/reboot/web/web_test.go` | D3: delete current lines 221–226 (both Q47 guard `if` blocks). Nothing else. |

### R4. Docs (same commit)

- `AGENTS.md` line 36: "(chart closure, fork-list placement, seatpost bounds)" → "(chart consistency, fork-list placement, seatpost bounds)". Line 49: `assembly.CheckChartClosure`'s → `assembly.CheckChartConsistency`'s (Slice 16 drops that note).
- `docs/reboot/DESIGN.md`, active rules only:
  - Requirement (c): `assembly.CheckChartClosure` → `assembly.CheckChartConsistency`.
  - Frameset leveling step 1: "gives the raw chart-closure residuals, which `catalog`'s `go test` closure check holds" → "gives the raw chart-consistency mismatches, which `catalog`'s `go test` consistency check holds".
  - Layout block: `assembly/` line "chart closure" → "chart consistency".
  - Cutover Inventory: `CheckChartClosure` → `CheckChartConsistency` (frame ingestion bullet and deadcode bullet), `chartResiduals` → `chartMismatches`.
  - Leave the Q41–Q93 rulings and the done Milestones lines as they are. They are history (Q117).
- Leave `.agents/skills/ingest-frame/SKILL.md` as it is. It documents legacy `bikegeo fit`, and Slice 16 rewrites it.

## Verification

- **V1:** `make pre-commit` passes (re-run if `go fix` changes anything).
- **V2, grep gates** (run from the repo root, after the change):
  - `grep -rniE 'closure|residual' cmd/bikegeo-reboot internal/reboot` prints only the `.disclosure` CSS lines in `internal/reboot/web/templates/compare.html` (currently lines 524–569 and 929).
  - `grep -rnwiE 'closes?' cmd/bikegeo-reboot internal/reboot` prints only `chart_consistency.go`'s "close to the measured excess" and the `web` "close-up" lines.
  - `grep -nE 'CheckChartClosure|ClosureAllowance|chartResiduals' AGENTS.md docs/reboot/DESIGN.md` prints only history: rulings in Q41–Q93 (for example Q47), the done Milestones lines for Slices 10 and 12b, and the Q117 ruling itself.
- **V3, test counts unchanged:** `go test -list '.*' ./internal/reboot/<pkg> | grep -c '^Test'` gives `assembly 100`, `catalog 26`, and `web 91` (measured 2026-11-21 before the change). `go test -list 'Closure|Closes' ./internal/reboot/...` lists nothing, and `go test -list 'Consistency|Consistent|ChartAllowances' ./internal/reboot/assembly ./internal/reboot/catalog` lists 13 tests.
- **V4, no live check:** nothing user-visible changes, since Q47 keeps the check off every page. `make pre-commit` is the verification.

## Out of scope

- Moving the check out of `assembly`, exporting the unpitched frameset placement, or the result type (Slice 16).
- The ingest-frame skill and AGENTS.md's interim note (Slice 16).
- Legacy code, legacy parity strings, and `docs/SQLAB_SADDLE.md`'s unrelated "Measurement closure".
- Any change to chart values, precisions, allowance amounts, or check logic.

## Rules that bite

- Pure rename: if any test needs a new number, the rename changed behavior. Stop.
- `git mv` both files so history follows them.
- Don't touch Q41–Q93 or done Milestones prose in DESIGN.md.
- Commit subject is exactly `Rename chart closure to chart consistency`, with no Conventional Commits prefix.
- Finish with the Implementation handoff (DESIGN.md "Implementation handoff"): this file's Status line, an `## Implementation handoff` section right after it, and the DESIGN.md Slice 15 line updated with the same status words.
