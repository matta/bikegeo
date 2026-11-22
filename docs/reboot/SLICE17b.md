# Slice 17b: final parity audit and legacy frame notes

Status: Implemented 2026-11-22; awaiting Matt's acceptance

Larger slice: 2 commits touching 21 files (3 new Markdown files in `internal/reboot/catalog`, plus doc comments in 18 `frame_*.go` files). It is written for a lower-tier implementer, so everything below is exact. **If something doesn't match this plan (a frame comment reads differently, a banned word appears that the table doesn't cover, a parity test fails), stop and record `Status: Blocked` instead of improvising.** No Go behavior, data value, URL, or test changes.

## Implementation handoff

- **Who and when:** 2026-11-22, Claude Sonnet 5.5.
- **Commits:** `920bf72` Archive legacy frame notes in the catalog (N1, N2, N3); "Point catalog frames at their legacy notes" (G1, G2, and these status edits), the commit that contains this file's change.
- **Done and not done:** A1, N1, N2, N3, G1, G2 all done.
- **A1 output:** `git log --oneline 9942803..HEAD -- internal/frames internal/catalog internal/model internal/geometry` printed nothing. `go test -count=1 -run 'Legacy' -v ./internal/reboot/catalog` passed `TestMissingLegacyFrames`, `TestFramesMatchLegacy` (subtest `otso-fenrir-stainless` skipped), `TestPrecisionMatchesLegacyInference`, `TestHandlebarsMatchLegacy`, and `TestWheelRadiusMatchesLegacy`, as measured.
- **Deviations:** one. The N2 header note, written verbatim, contains the literal text `file:///`, which makes Verification 2 (`grep -rn 'file:///' internal/reboot` prints nothing) fail. I reworded "the removed local `file:///` links" to "the removed local file links" in both copies. Nothing else differs. In 926d the 4 original links became 3 plain-text replacements plus the one relative link to `legacy_seven_expedition_926f.md`; 926f had 2.
- **Verification:** (1) 18 sections, 1 `ritchey` hit (the intro). (2) no `file:///` under `internal/reboot`. (3) no `cad` in `legacy_*.md`. (4) only `frame_ritchey_ultra.go` lacks the pointer. (5) `internal/frames` is untouched. (6) the Go diff changes comment lines only. (7) `make pre-commit` passes after each commit. (8) No live check: nothing user-visible changes.
- **Open issues:** none for Matt to decide. Notes left alone that read as stale or doubtful: the Fenrir section (superseded, flagged in the archive), the Seven Custom section's long fit-provenance narrative (partly overtaken by later rulings, and the archive says so), and the Kinetik and Revel sections, whose fork and sag figures are the legacy basis hypotheses (Q44, Q81).

## Goal

Before Slice 18a deletes legacy:

1. Run the `*_parity_test.go` files a last time and confirm that no legacy data has changed since the port commit `9942803`.
2. Archive what the port dropped: every legacy frame's `Notes` string and the two Seven Expedition basis-fork analyses (Q145).
3. Point each frame's doc comment at its archived notes, and flag the Sour Pasta Party wheelbases as unverified (Q146).

## Rulings

- **Q145:** one `internal/reboot/catalog/legacy_frame_notes.md` holds each frame's legacy `Notes`, and the Seven Expedition analyses are copied beside it. Text is verbatim except the marked `cad` replacements. The Ritchey Ultra is left out (Slice 18b deletes it).
- **Q146:** the Sour Pasta Party wheelbases stay in the data, labelled published, but comments and the archive say they're unverified. Re-ingestion is a Deferred follow-up, not this slice.
- **Q144:** `make pre-commit` doesn't run `deadcode`. Don't run `make deadcode` or act on its output.
- **Q121 / `make boundary`:** no file under `internal/reboot` may contain `cad` in any case (this also catches "Cascade"), or `price`, `pricing`, `cost`, `cheap`, `ledger`, or `chassis`.

Obvious mechanics, decided here (veto on review):

- **Copy, don't move.** `internal/frames` is legacy and must stay unchanged until Slice 18a deletes it, so the Seven analyses are copied with plain `cp`, not `git mv`.
- Sections go in alphabetical order by frame ID.
- Each frame's doc comment points to the archive by file name and section ID, not by a Markdown anchor link.

## Already measured (planning, 2026-11-22)

- `git log --oneline 9942803..HEAD -- internal/frames internal/catalog internal/model internal/geometry` prints nothing.
- `go test -count=1 -run 'Legacy' -v ./internal/reboot/catalog` passes five tests: `TestMissingLegacyFrames`, `TestFramesMatchLegacy` (with subtest `otso-fenrir-stainless` skipped), `TestPrecisionMatchesLegacyInference`, `TestHandlebarsMatchLegacy`, and `TestWheelRadiusMatchesLegacy`.
- Legacy `GeoSlopMm.Reason` strings are already ported as `ChartAllowance` reasons (`esker-hayduke`, `sklar-pbj`), and Ritchey's goes with the frame. So the archive covers only `Notes` and the two analyses.

## Steps

### A1. Parity rerun and legacy data check (no files change)

Run both commands above at the start of implementation. Record their output in the handoff. If either differs from the measured result, stop: `Status: Blocked`.

### N1. `internal/reboot/catalog/legacy_frame_notes.md` (new)

Exact skeleton. Replace `<…>` placeholders and keep everything else as written:

```markdown
# Legacy frame notes

These are the `Notes` strings of the legacy frames in `internal/frames`, unchanged since the port commit `9942803` and carried over in Slice 17b (Q145) before Slice 18a deletes legacy. They are history, not current provenance: where a section disagrees with the frame's Go file, its source record (`frame_<id>.md`), or a ruling in `docs/reboot/DESIGN.md`, those win. When a frame is re-ingested (Q127), move its section into its source record and delete it here. The Ritchey Ultra is left out because Slice 18b deletes it (Q125).

The text is verbatim except where `make boundary` bans a word (Q121). Each replacement is followed by `[edited]`, and the original wording is in git history at `9942803`. Fork IDs and field names (for example `enve_mountain_472`, `GeoHeadsetSpacerMm`, `HeadsetCupType`) are legacy names and may no longer exist.

## <frame-id>

<Manufacturer> <Model>, from `internal/frames/<legacy file>.go`.

<Notes text>
```

Sections, in this order, with the legacy file for each. All are in `internal/frames/`:

| Section ID | Legacy file |
| --- | --- |
| `binary-havok` | `binary_havok.go` |
| `binary-kinetik` | `binary_kinetik.go` |
| `esker-hayduke` | `esker_hayduke.go` |
| `fairlight-holt-2` | `fairlight_holt_2.go` |
| `moots-mountaineer` | `moots_mountaineer.go` |
| `otso-fenrir-stainless` | `otso_fenrir_stainless.go` |
| `revel-el-jefe` | `revel_el_jefe.go` |
| `salsa-cutthroat` | `salsa_cutthroat.go` |
| `salsa-fargo` | `salsa_fargo.go` |
| `seven-custom` | `seven_custom.go` |
| `seven-expedition-926d` | `seven_expedition_926d.go` |
| `seven-expedition-926f` | `seven_expedition_926f.go` |
| `sklar-pbj` | `sklar_pbj.go` |
| `sour-pasta-party` | `sour_pasta_party.go` |
| `surly-ecr` | `surly_ecr.go` |
| `surly-krampus` | `surly_krampus.go` |
| `tumbleweed-sunliner` | `tumbleweed_sunliner.go` |
| `turner-venn-ti` | `turner_venn_ti.go` |

- **Manufacturer and Model:** copy the reboot catalog's `Manufacturer` and `Model` strings for that ID (for example "Fairlight Holt 2.0", "Seven Expedition 926d").
- **Notes text:** the legacy `Notes:` Go string literal as one unwrapped paragraph, with Go escapes undone (`\"` → `"`). Then apply the N3 replacements.
- **Extra lines,** each its own paragraph right after the Notes paragraph:
  - `otso-fenrir-stainless`: `Superseded: this describes the \`ea9d423\` chart that Q126 found wasn't Otso's. The current chart and its evidence are in \`frame_otso_fenrir_stainless.md\` (Q140–Q143).`
  - `seven-expedition-926d`: `Basis-fork analysis: \`legacy_seven_expedition_926d.md\`.`
  - `seven-expedition-926f`: `Basis-fork analysis: \`legacy_seven_expedition_926f.md\`.`
  - `sour-pasta-party`: `Audit (Slice 17b, Q146): "adapted" means these wheelbases may not be Sour's printed values. The catalog still labels them published and checks chart consistency against them until the frame is re-ingested.`

### N2. Seven analyses (new copies)

```sh
cp internal/frames/seven-expedition-926d.md internal/reboot/catalog/legacy_seven_expedition_926d.md
cp internal/frames/seven-expedition-926f.md internal/reboot/catalog/legacy_seven_expedition_926f.md
```

Then edit only the copies:

1. **Header note.** Insert this paragraph right after the `# …` title line of each copy, with a blank line on each side (use `926d` or `926f` to match the file):

   `> Copied from \`internal/frames/seven-expedition-926d.md\` in Slice 17b (Q145), unchanged since \`9942803\`, before Slice 18a deletes legacy. It's history and the evidence for the catalog's 472 mm / 55 mm basis fork. The fork ID \`enve_mountain_472\` no longer exists (Q133), and the \`.json\` data files it mentions were deleted long ago. The text is verbatim except the replacements marked \`[edited]\` (Q121) and the removed local \`file:///\` links.`

2. **Local links.** Replace every Markdown link whose target starts with `file:///` with its link text alone, without brackets. There are 4 in 926d (lines 5, 14, 25, and 207 of the original) and 2 in 926f (lines 9 and 200 of the original). One exception: in 926d line 14, `[Expedition 926f](file:///…/seven-expedition-926f.md)` becomes `[Expedition 926f](legacy_seven_expedition_926f.md)`. Keep the `https://sevencycles.com/…` link in 926f.

3. **Banned words.** Apply N3.

### N3. `cad` replacement table (N1 Notes and N2 copies)

Replace each phrase, then write ` [edited]` right after the replacement. Matching is case-sensitive as shown.

| Legacy phrase | Replacement |
| --- | --- |
| `CAD spacer` | `basis spacer` |
| `CAD A2C` | `basis A2C` |
| `CAD geometry chart` | `basis geometry chart` |
| `CAD geometry` | `basis geometry` |
| `CAD chart` | `basis chart` |
| `CadLowerCupMm` | `BasisLowerCup stack` |
| `in CAD around` (after modeled, drawn, or drafted) | `around` |
| `CAD model` | `basis model` |
| `CAD @ 472mm` | `basis @ 472mm` |
| `taller than CAD` | `taller than the basis` |
| `CAD baseline fork (\`cad_fork\`)` | `basis fork (\`BasisFork\`)` |
| `CAD Template Disconnect` | `Basis Template Disconnect` |
| `stack in CAD` | `stack in the basis` |
| `CAD preview` | `design-drawing preview` |
| `CAD frame preview drawing` | `frame design drawing` |
| `Cascade Bicycle Studio` | `the fit studio` |

Example: `with 12mm CAD spacer)` becomes `with 12mm basis spacer [edited])`.

If `grep -n -i cad` on the three new files still finds a hit the table doesn't cover, stop: `Status: Blocked` and quote the line.

### G1. Doc comment pointers (18 `frame_*.go` files)

In each frame file except `frame_ritchey_ultra.go`, add a pointer at the end of the doc comment above the frame function (the `// <name> returns …` block):

```go
//
// Legacy notes: legacy_frame_notes.md, section <frame-id> (Q145).
```

Exceptions:

- `frame_otso_fenrir_stainless.go`: put the pointer line right after `// Source record: frame_otso_fenrir_stainless.md.` with no blank `//` line between.
- `frame_seven_expedition_926d.go` and `frame_seven_expedition_926f.go`: the pointer reads `// Legacy notes: legacy_frame_notes.md, section seven-expedition-926d, and the basis-fork analysis legacy_seven_expedition_926d.md (Q145).`, with `926f` in both places in the 926f file. Wrap it at about 80 columns like the rest of the comment.

### G2. Sour Pasta Party (`frame_sour_pasta_party.go`, in addition to G1)

Replace the doc comment's last two sentences:

```go
// fork at 15% sag (512 mm effective) and a 12 mm EC44 cup. Wheelbases are for a
// 44 mm offset fork.
```

with:

```go
// fork at 15% sag (512 mm effective) and a 12 mm EC44 cup. Legacy says its
// wheelbases were adapted to a 44 mm offset fork, so they may not be Sour's
// printed values; they stay labelled published until the frame is re-ingested
// (Q146).
```

The G1 pointer goes after this. Change the `// published` comment above `Sizes:` to `// published; wheelbases unverified (Q146)`.

## Commits

Each commit passes `make pre-commit` on its own.

1. **`Archive legacy frame notes in the catalog`:** N1, N2, N3 (the three new Markdown files).
2. **`Point catalog frames at their legacy notes`:** G1, G2, plus the handoff and status edits to this file and DESIGN.md's Slice 17b bullet.

## Verification

1. `grep -c '^## ' internal/reboot/catalog/legacy_frame_notes.md` prints `18`, and `grep -ci ritchey internal/reboot/catalog/legacy_frame_notes.md` prints `1`. The one hit is the intro sentence.
2. `grep -rn 'file:///' internal/reboot` prints nothing.
3. `grep -rn -i cad internal/reboot/catalog/legacy_*.md` prints nothing.
4. `grep -L 'legacy_frame_notes.md' internal/reboot/catalog/frame_*.go` prints only `internal/reboot/catalog/frame_ritchey_ultra.go`.
5. `git status --short internal/frames` prints nothing, and `git diff --stat HEAD~2 -- internal/frames` is empty after both commits.
6. `git diff HEAD~2 -- '*.go' | grep '^[-+]' | grep -v '^[-+][-+]' | grep -v '^[-+]\s*//'` prints nothing: only comment lines changed.
7. `make pre-commit` passes for each commit.
8. No live check: nothing user-visible changes. Say so in the handoff.

## Out of scope

- Any data change, including the Sour Pasta Party wheelbases (Q146's re-ingestion is a Deferred follow-up).
- New `frame_<id>.md` source records, or moving sections into the Fenrir's record.
- The Ritchey Ultra (Slice 18b), `deadcode` findings (Q144), and anything in `internal/frames` or other legacy packages.
- `docs/RIDER_FIT.md`'s links to `internal/ideal/readme.md`, and `README.md`'s legacy paths (Slice 20).
- Rewording the legacy text beyond N2 and N3, including fixing stale claims. Stale claims are why the archive says it's history.

## Rules that bite

- Copy, never move, files out of `internal/frames`.
- Go changes are comments only. Don't reflow untouched comment lines.
- `[edited]` follows every N3 replacement and nothing else.
- Don't run `make deadcode` (Q144).
- Finish with the **Implementation handoff** (DESIGN.md, "Slice planning"): set this file's Status line and the Slice 17b bullet in DESIGN.md to `Implemented <date>; awaiting Matt's acceptance` (or Partly implemented / Blocked), and add `## Implementation handoff` right after the Status paragraph. Include A1's output, every verification result, and any Notes text you found that seemed wrong but left alone.
