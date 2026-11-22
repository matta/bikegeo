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
