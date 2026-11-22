---
name: ingest-frame
description: Ingest a new bike frame or geometry chart into the reboot catalog (internal/catalog) — record every gathered value and its source in the frame's source record, transcribe the published chart, find its basis with a temporary Go test, update the catalog-size pins, and pass the catalog go test.
---

# Frame ingestion

New frames go into `internal/catalog` only. The legacy app is gone; its history lives at tag `legacy-final` (DESIGN.md Q114, Q119). Every question a chart raises is answered with a temporary Go test, following AGENTS.md "One-off investigations".

## Rules

1. **Published numbers are ground truth.** Transcribe them exactly. Never nudge a value to make chart consistency pass. Leave an unpublished wheelbase at 0 (Q57) rather than deriving it. Correcting a typo needs a gross, documented error (a transposed digit, a value off by more than 5 mm) and Matt's ruling.
2. **Keep a source record, and label every value's provenance.** Everything you gather goes into the frame's source record, `internal/catalog/frame_<id>.md`, whether or not the catalog uses it (step 1). In the Go file, label each value's provenance in a comment: published, catalog, measured, owner-reported, illustrative, derived, or hypothesis. Link the record from the frame's doc comment. A value labelled published must appear digit for digit in the record's transcription. If you computed it, it is derived, and the record says how.
3. **The basis is a hypothesis.** `BasisFork` (static A2C with sag included, rake, `assembly.SteererBasis`, no travel) and `BasisLowerCup` are the front end the chart's numbers are computed on (Q44, Q121). Chart consistency shows agreement, not uniqueness: any cup + A2C pair with the same sum fits. Prefer the basis the manufacturer states. A sagged basis A2C is the fork's A2C minus sag × travel (a 530 mm, 120 mm fork at 20% is 506 mm).
4. **An allowance is a last resort.** A `ChartAllowance` must be needed (the size fails without it), at most 0.5 mm beyond the excess it covers, and have a `Reason` (Q93). Never use one to cover a wrong basis: the wrong fork, sag, cup, or rake.
5. **No scripts, no restated geometry.** Call `measureChartConsistency` and the placement functions. Change only copies of catalog values inside a temporary test.
6. **Adjustable geometry stays as printed.** When a flip chip, Tuning Chip, or sliding dropout changes the geometry, transcribe the position the chart prints (usually the nominal or middle one). Put the published per-position changes in the record. Never compute another position's columns and label them published.

## Recipe

### 1. Gather the sources and write the source record

Gather:

- **The geometry chart:** for each size, reach, stack, head tube angle, seat tube angle, head tube length, BB drop, chainstay, wheelbase, seat tube length, and (if published) maximum seatpost insertion. Also gather every other row the chart prints (top tube, trail, standover, fork rows, rider heights), even ones the catalog doesn't model. Seat tube length is center to top; add the collar offset to a center-to-center figure and say so (Q75d). Note which chainstay or chip position the chart is drawn at.
- **The frame specs:** headset standard and lower cup, the fork the frame is designed around (A2C, offset, travel, sag), the dropout range, and insertion depths.
- **Every copy of the chart:** the product page's table, the gallery image, spec pages, and PDFs. They often disagree.
- **The launch-era page.** Open the product page in the Wayback Machine (`https://web.archive.org/web/<year>/<url>`) from when the frame launched. The chart is usually drawn for the fork the frame first shipped with, not today's.
- **The parts' own specs:** the fork maker's A2C and offset for each dropout position, and the specified headset's lower cup stack. Use archived pages for discontinued parts.

Write `internal/catalog/frame_<id>.md` before any Go. Copy the shape of `frame_otso_fenrir_stainless.md`:

- **Sources:** each URL, what it holds, the date you read it, and an archived copy when one exists.
- **Published geometry:** the chart transcribed as printed. Ranges stay ranges, typos stay typos, and units and digits stay as shown. Add one table per copy that differs, or list the differences.
- **Published specs:** the frame spec lines, verbatim.
- **Component specs:** fork positions, headset cup stack, and which fork shipped when.
- **Discrepancies:** every disagreement between sources.
- **Catalog values:** each catalog field, its value, its provenance label, and the record row it comes from (or how it was derived).
- **Chart consistency:** the step 3 readouts, dated.
- **Open questions:** anything Matt must rule on.

Don't put the words the `make boundary` check bans in the record (for example "cad" anywhere in a word, as in "decade", or inside a URL's hash). If a URL trips the check, shorten it with `…` and say where it's linked from.

### 2. Write `internal/catalog/frame_<id>.go`

Copy the shape of `frame_otso_fenrir_stainless.go` and `frame_otso_fenrir_stainless.md`.

- `frameFamily` holds the physical `LowerBore` and `DefaultLowerCup`. `family.sizes(publishedSize{...}, ...)` holds each size's published columns, plus an `Allowance` only if step 3 proves one is needed.
- `ChartPrecision` is per column: half of the coarsest step that every size's value sits on. The length steps are 1, 0.5, 0.1, and 0.01 mm. The angle steps are 1, 0.5, 0.25, 0.1, 0.05, and 0.01°. A head angle of 68.25 in every size is on a 0.25° step, so its half-width is 0.125. Head angles of 68 and 68.5 give 0.25, and whole millimeters give 0.5 (Q80). Use the printed digits ("70.0°" is a 0.1° step) when a single size gives too little to judge.
- `Forks` lists the fork models the manufacturer specifies (using catalog `Fork` vars, which own their A2C and rake variants; `CompatibleForks()` automatically opens other compatible catalog forks whose travel/rigid class and steerer fit the frame, per Q128–Q130, Q135). Add a `Fork` to `forks.go` only for a real fork model.
- `MaxChainstayMm` is the rearmost sliding-dropout position (Q67), or 0 for fixed dropouts.
- Start `BasisFork` and `BasisLowerCup` from the manufacturer's stated basis, or from a guess with the bare head tube (`assembly.LowerCupBareHeadTube`, stack 0).
- Register the frame in `Frames()` in `frame.go`, in ID order.

`TestEveryChartIsConsistent` and the pins in step 5 fail until steps 3–5 are done. That's expected.

### 3. Find the basis with a temporary test

Write a short temporary test in `internal/catalog/scratch_basis_test.go` (`package catalog_test`) that calls `searchBasis`, `evalBasis`, and `logBasisSearch` from [internal/catalog/basis_search_test.go](internal/catalog/basis_search_test.go), then run `go test -run TestScratchBasis -v ./internal/catalog`:

```go
package catalog_test

import (
	"testing"

	"bikegeo/internal/assembly"
)

func TestScratchBasis(t *testing.T) {
	f := mustFrame(t, "<frame-id>")
	for _, tc := range []struct {
		label        string
		cup          assembly.LowerCupSetup
		rake         float64
		compareRakes []float64
	}{
		{"bare head tube", assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube}, 51, []float64{44, 51, 52}},
		{"typical cup", f.Sizes[0].ChartFrame.DefaultLowerCup, 51, []float64{44, 51, 52}},
	} {
		res, err := searchBasis(f, tc.cup, tc.rake, tc.compareRakes)
		if err != nil {
			t.Fatalf("%s: %v", tc.label, err)
		}
		logBasisSearch(t, tc.label, res)
	}
}
```

Use `evalBasis(f, cup, a2c, rake)` directly to check any specific `(cup, staticA2c, rake)` hypothesis (for example a stated fork position). Set `rake` and `compareRakes` to the offsets worth comparing for this chart, and try both the bare head tube and the specified headset's lower cup stack.

Read the output this way:

- **A stated basis that passes wins**, even if it isn't the minimax value. Record it as stated.
- **With no stated basis**, use the minimax A2C rounded to 0.5 mm, and record it as inferred from the published stacks, with the implied range, in the frame's Go comment (for example, "the published stacks imply a 508.1–508.9 mm A2C in every size, so the basis fork is 508.5 mm").
- **A spread of implied A2Cs wider than the axle-height budgets, or one size far from the rest,** points to a transcription error, a typo in the chart, or a cup the chart doesn't state. Investigate it; don't cover it with an allowance.
- **A wheelbase check that fails at the published offset** but passes at another means the chart may be drawn for a different fork. Record what you found and ask Matt.
- **Only if one size still fails** with the right basis does it get an allowance. Log that size's `m.Checks` (each check's `MismatchMm`, `Terms`, `BudgetMm`, and `ExcessMm`) from `measureChartConsistency` and size the allowance from `ExcessMm`, within rule 4.

Copy the readouts into the record's Chart consistency section with the date, then delete `internal/catalog/scratch_basis_test.go`.

### 4. Apply the basis

Set `BasisFork` and `BasisLowerCup` in the frame file, each with a `hypothesis (Q44, Q81):` comment that says how it was found. Add any allowance from step 3 with its `Reason`.

### 5. Update the catalog-size pins

Tests across `internal` pin the catalog's contents on purpose (frame and size counts, ID lists, fork and allowance lists, rendered row and option counts), so a dropped frame can't go unnoticed. Adding a frame breaks them. Run `go test ./internal/...` and let the failures show you where the pins are. Update a pin only after checking the change is exactly what the new frame adds: its sizes, its forks, its allowance. Any other failure is a real problem; fix it or report it, and don't adjust the test. Keep test names that carry an old count (for example `…All19Frames`).

The `/` page's labels count the catalog on their own (Q124). Don't edit DESIGN.md's history to match. Q-rulings record the catalog as it was when they were ruled.

### 6. Verify

- `go test ./internal/catalog` passes, including `TestEveryChartIsConsistent` and `TestFrameStructure`.- `make pre-commit` passes.
- The record's Catalog values table matches the Go file, value for value and label for label.
- In the `bikegeo` preview, `/bike?frame=<id>` and `/bike?frame=<id>&fork=basis` render, and the Chart Basis Fork readout's wheelbase is close to the published one. The new frame appears on `/`.
