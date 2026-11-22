# Slice 18b: Delete the Ritchey Ultra

Status: Implemented 2026-11-22; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** Sonnet 5.5, 2026-11-22.
- **Commits:** `Delete the Ritchey Ultra` (see `git log` for the hash). Plan changes 1–7 are all in that one commit.
- **Done and not done:** changes 1–7 done; Verification 1 and 2 done. Nothing left.
- **Deviations:** None.
- **Verification:** `make pre-commit` exits 0 on the final state. Live check on port 8081: `/` titled "Compare 18 Frames — bikegeo-reboot", "Showing 18 of 2889 configurations (2163861 builds evaluated)", 18 rows in each table, no Ritchey in the page text; `/bike` has 18 frame options and no Ritchey text; `/bike?frame=ritchey-ultra` returns 400 with the frame select `aria-invalid="true"`; `compare` CLI JSON gives RowCount 2889, Evaluated 2163861, 18 rows, and no `ritchey-ultra` row. No screenshot was taken.
- **Open issues:** the `/bike?frame=ritchey-ultra` 400 page still contains the string "ritchey" somewhere in its body text, almost certainly the existing unknown-frame message echoing the rejected value. I didn't locate the exact element. Today's behavior, no change made.

**Goal:** remove the `ritchey-ultra` frame from the catalog (Q125). Its catalog wheelbases aren't published values, and Ritchey's own chart is gone, so it can't be re-ingested. Slice 18a deleted the parity tests, so Q119 no longer requires every legacy frame to be in the catalog. The only user-visible change is that the frame disappears from `/`, `/bike`, and the CLI.

**Rulings:** Q125 (delete it), Q119 (now moot), Q124 and Q127 (UI counts are derived; test pins stay and are updated, not listed in the skill). No new rulings are needed.

**Size:** one commit, seven files (one deleted, five Go edits, one skill line). Every edit is mechanical, and every expected value below was measured on 2026-11-22 by deleting the frame in a throwaway worktree and running the suite; `make pre-commit` passed there with exactly these edits.

**Planner defaults (veto on review):**

- Test names that carry the old count (`TestParseCompareDefaultAll19Frames`, `TestCompareDefaultAll19Frames`) keep their names, as the ingest-frame skill's step 5 says.
- The dated provenance records that mention the frame (`lower_cups.md`'s Q136 table, `forks_rigid.md`'s Q137 section, `legacy_frame_notes.md`'s header) stay unchanged. They record what an earlier slice did, and the archive already leaves the Ritchey out.
- The `seatposts.go` Ritchey Comp Trail Zero comments stay (that's a seatpost, not the frame).
- No archive of the deleted Go data: git history and the `legacy-final` tag keep it, and Q125 quotes the Notes.

## Measured effect

Ritchey's winning row is rank 13 of 19 today (XL, `enve_mountain_boost-490x44`, `fits_grip`, 79.265898878). After the deletion every row below it moves up one place with an unchanged score. No other row changes.

| Pin | Before | After |
| --- | ---: | ---: |
| Frames | 19 | 18 |
| Sizes (and basis placements) | 119 | 115 |
| Fork-list placements = default compare `RowCount` | 2989 | 2889 |
| Default compare builds evaluated | 2238761 | 2163861 |
| Fork variants across all frames (`ForkOptions`) | 234 | 223 |
| Otso Fenrir XL row (query index / web rank) | 16 / 17 | 15 / 16 |
| Salsa Cutthroat 60cm row (query index / web rank) | 18 / 19 | 17 / 18 |

## Changes (one commit: `Delete the Ritchey Ultra`)

1. **Delete** `internal/reboot/catalog/frame_ritchey_ultra.go` (`git rm`).
2. **`internal/reboot/catalog/frame.go`:** remove the `ritcheyUltra(),` line from `Frames()`.
3. **`internal/reboot/catalog/frames_test.go`:**
   - `TestFramesRegistry`: doc comment "checks the 19 frames" → "checks the 18 frames"; drop `"ritchey-ultra": 4,` from the `want` map; `19` → `18` in the frame-count check and its message; `119` → `115` in the size-total check and its message.
   - `TestForkListsPlace`: `placements != 2989 || bases != 119` → `2889` and `115`, and the message to match.
   - `TestInstalledFrameUsesMaxChainstay`: delete the `{"ritchey-ultra", "L", 0, 0}` case. The Fenrir case still covers a non-sliding frame.
4. **`internal/reboot/query/compare_test.go`:**
   - `TestCatalogFrames`: `19` → `18` (count check and message); `119` → `115` (size total and message).
   - `TestParseCompareDefaultAll19Frames`: `2989, 2238761, 19` → `2889, 2163861, 18` (check and message). In the `want` table, change the index `16` (Fenrir) → `15` and `18` (Cutthroat) → `17`. Keep every other field, including totals and GripR at 1e-6. Indices 0, 1, 2, and 7 don't change.
   - `TestParseCompareOpenForkChoice`: `234` → `223` (check and message). The `krampus pike_120` 12-row check doesn't change.
5. **`internal/reboot/web/compare_test.go`:**
   - `TestCompareDefaultAll19Frames`: the title becomes `Compare 18 Frames — bikegeo-reboot` (check and message); the mode note substring becomes `all 18 catalog frames`; the configs-summary substrings become `"18", "2889", "2163861"`; the three table row counts become `18`.
   - Same test, rank checks: rename the "Row #17" Fenrir block to rank 16 (`fit17` → `fit16`, `data-compare-rank="17"` and `data-geometry-rank="17"` → `"16"`, messages "row 17" → "row 16") and the "Row #19" Cutthroat block to rank 18 (`fit19` → `fit18`, `data-compare-rank="19"` → `"18"`, "row 19" → "row 18"). Expected cell values don't change (Fenrir `fits_grip`, `69.1`, `0.96`, chainstay `440.0`, no sliding mark; Cutthroat `fits_neither`, `50.3`, `3.45`). Rows 6 and 8 don't change.
   - `TestCompareCustomSelectionsAndAutoTopPerFrame`: the blank-frame row count `19` → `18` (check and message); the comment "Multi-selection + Remove + All 19 Frames links" → "All 18 Frames"; the preset link text `All 19 Frames` → `All 18 Frames`; the after-all-frames row count `19` → `18`.
6. **`internal/reboot/web/web_test.go`**, `TestBikeMultiFrameSelectAndSwitching`: the frame option and catalog count `19` → `18` (check and message), and the `/bike?frame=nope` error-page frame option count `19` → `18` (check and message).
7. **`.agents/skills/ingest-frame/SKILL.md`**, step 4's "With no stated basis" bullet: replace "(as `frame_ritchey_ultra.go` does)" with a self-contained example, since no remaining frame has an inferred basis: "…with the implied range, in the frame's Go comment (for example, "the published stacks imply a 508.1–508.9 mm A2C in every size, so the basis fork is 508.5 mm")."

## Tests

No new tests. The pins above are the regression guard. After the edits, `rg -i ritchey internal/reboot cmd .agents` finds only `seatposts.go` and the three provenance records named in the defaults.

## Verification

1. `go vet ./... && go test ./...`, then `make pre-commit` (re-run if `go fix` changes anything).
2. **Live check** with `preview_start` `reboot` (port 8081):
   - `/`: the title is "Compare 18 Frames — bikegeo-reboot", the summary says "Showing 18 of 2889 configurations (2163861 builds evaluated)", and no row names Ritchey.
   - `/bike`: the frame `<select>` has 18 options with no Ritchey.
   - `/bike?frame=ritchey-ultra`: HTTP 400 with today's unknown-frame error, the frame select marked `aria-invalid`.
   - CLI: `go run ./cmd/bikegeo-reboot compare | jq '.RowCount, (.Rows | length)'` prints `2889` and `18`.

## Out of scope

- Re-ingesting the Sour Pasta Party (Q146 stays a Deferred follow-up).
- Renaming tests that carry an old count, or rewording the dated provenance records.
- Slice 19's move out of `internal/reboot`, and any `deadcode` cleanup (Q144).

## Rules that bite

- Never change a published chart value or any other frame's data; this slice only deletes.
- If any test fails other than the pins listed here, or a value differs from the table, stop: the deletion changed something it shouldn't. Mark the plan Blocked and report it; don't adjust the test.
- No Conventional Commits prefix. End the commit message with the attribution line.
- Finish with the **Implementation handoff** (DESIGN.md "Slice planning"): update this Status line, add the `## Implementation handoff` section, and set DESIGN.md's Slice 18b Milestones line to the same status words.
