# M2 Slice 7, bar sweep band

**Status: Implemented 2026-11-19; awaiting Matt's acceptance.**

## Implementation handoff

- **Who and when** — OpenAI Codex (GPT-6) and a Codex subagent, 2026-11-19.
- **Commits** — Both planned implementation commits are in `6ea0295`, `Penalize and show handlebar sweep in reboot stock fitting`, with Matt's approval.
- **Steps** — A1, A2, and W1–W4 are complete.
- **Deviations** — The assembly change caused the web template to render a fifth ledger cell before the header and test expectations were updated. Matt approved one implementation commit so the suite passes at the commit boundary. The adjacent `LedgerLine` comment also names the installed handlebar as a source of band inputs. The session supplied no attribution trailer, so the commit uses `Assisted-by: OpenAI Codex`.
- **Verification** — `go vet ./... && go test ./...` and `make pre-commit` passed for the combined change. Ran `make serve-reboot`, which built the binary. Port 8081 was occupied, so the live Chrome checks used the updated build on port 8082. In all-bars mode, 6,750 combinations produced five displayed 16° bars with 0.0 sweep points each. The SQlab 311 2.0 Alloy 16° with 15 mm rise led at 86.6 with a 130 mm, −17° stem and 0 mm spacers. Switching to Selected handlebar only and the SQlab 311 FL-X Carbon 12° with 15 mm rise showed 8.0 sweep points on all five rows. [All-bars screenshot](/Users/matt/.codex/visualizations/2026/11/20/01a1f77b-c784-7203-871b-51dd80acaa67/slice7-all-bars.png) · [Selected-bar screenshot](/Users/matt/.codex/visualizations/2026/11/20/01a1f77b-c784-7203-871b-51dd80acaa67/slice7-selected-bar.png)
- **Open issues** — None.

The plan follows Matt's ruling Q30 ([DESIGN.md](DESIGN.md), "Seatposts, bar sweep, and fork choice"). Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. This plan builds on Slice 5, which is accepted (commits `599fcf4`, `bf3b2a4`, `fe0a707`). It doesn't depend on Slice 6 and touches none of Slice 6's code, so it may land before or after it. Slice 8 builds on this slice.

This slice has two commits and touches 6 code, test, and template files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Today bars carry no penalty in stock fitting (Q23), so the 12° SQlab bars win the all-bars ranking. After this slice:

- `assembly.CockpitPreferences` gains a `BarSweep` band (type `PricedBand`, the existing name) on the installed bar's `BacksweepDeg`. The fixture sets it free from 16° to 20° and penalizes 2 points per degree outside that range (Q30).
- Every stock candidate's ledger gains a fifth line, **bar sweep**, after spacers. It appears in both `stock_bars` modes: in `selected` mode it is the same on every row.
- `/bike`'s stock table gains a **Sweep pts** column, and the preference note names the band.

Rulings:

- **Q30:** score bars by backsweep only, free for 16–20° with every bar in that range scoring the same, 2 points per degree outside. The line is always on the ledger. This supersedes Q23's "bars are free".
- **Q16, Q18, Q25:** enumeration, PlaceBike, the points-lost ledger, and the 5 best-scoring builds are unchanged. Ties still favor smaller oval R, then enumeration order.
- AGENTS.md: a band is free inside its range and linear outside it, with no weights, floors, or clamping.

## 2. Terminology

- **Bar sweep** is `HandlebarSetup.BacksweepDeg`. The ledger band name is `"bar sweep"`, and the column header is "Sweep pts".
- **Stock fitting** / **fit** (Q8). Never write "search".
- Don't write the term AGENTS.md bans for the frameset.

## 3. Commit 1: assembly (`internal/reboot/assembly`)

Files: `stock_fitting.go`, `stock_fitting_test.go`.

Commit subject: `Penalize handlebar sweep outside 16–20° in reboot stock fitting`.

### A1. Preferences and ledger (`stock_fitting.go`)

- `CockpitPreferences` gains `BarSweep PricedBand // degrees of backsweep (Q30)` after `Spacers`.
- `Validate()` adds `{string(BandBarSweep), p.BarSweep}` to its band list, last.
- Add the constant `BandBarSweep LedgerBand = "bar sweep"` after `BandSpacers`.
- Update `StockCandidate.Ledger`'s comment to `// oval, stem length, stem angle, spacers, bar sweep`.
- In `ApplyStockFitting`, append one line to the `Ledger` literal after spacers:

  ```go
  {Band: BandBarSweep, PointsLost: prefs.BarSweep.PointsLost(float64(bar.setup.BacksweepDeg))},
  ```

- Update `ApplyStockFitting`'s doc comment: replace "and costs nothing (Q23)" with "and is scored by its backsweep (Q30)".

Don't change the enumeration, the sort, or the saddle-first block.

### A2. Tests (`stock_fitting_test.go`)

- `illustrativeCockpitPreferences()` gains `BarSweep: assembly.PricedBand{FreeMin: 16, FreeMax: 20, CostBelow: 2, CostAbove: 2}`.
- **Existing tests that change.** Every Persuader 20° result is unchanged, because 20° is free. Update only these:
  - The ledger-band list (around line 163, `wantBands`) gains `assembly.BandBarSweep` last. The fit-report ledger check (around line 153) gains a fifth expected line of 0.
  - `TestStockFittingChoosesAmongHandlebars`: every row is the 12° FL-X bar (index 1), so each total drops by 8. The table becomes 82.6, 82.6, 80.8, 80.6, 80.6. Ranks, stems, and R are unchanged. The single-bar cross-check still matches, since both runs penalize the same bar.
  - `TestStockFittingInstallsEveryBarAtSubmittedRoll`: totals become 82.6 (rank 1) and 80.6 (rank 3).
- **New `TestBarSweepBand`.** `PointsLost` is 8 at 12°, 0 at 16°, 18°, and 20°, and 4 at 22° (exact).
- **New `TestStockFittingPenalizesSweep`.** `twoBarStock`, grip only, limit 900. Every candidate with `HandlebarIndex == 0` (Persuader, 20°) has a sweep line of 0, and every candidate with index 1 (FL-X, 12°) has 8. In every candidate, `Ledger[4].Band == BandBarSweep`.
- `TestCockpitPreferencesValidation` (or the existing validation test for preferences): a `BarSweep` with `FreeMin > FreeMax` is rejected with an error containing `bar sweep`.

## 4. Commit 2: workbench (`internal/reboot/web`)

Files: `fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`.

Commit subject: `Show the handlebar sweep penalty in reboot stock fitting`.

### W1. Fixture

In `CockpitPreferences`, add after `Spacers`:

```go
BarSweep: assembly.PricedBand{FreeMin: 16, FreeMax: 20, CostBelow: 2, CostAbove: 2}, // Matt's preference (Q30)
```

### W2. Preference note (`buildStockFittingView`)

Change the format to:

```go
"Free: HandGrip inside the ±%.0f × ±%.0f mm oval, stem length %s, stem angle %s, spacers %s, bar sweep %s. Penalties are illustrative except bar sweep (Q30)."
```

Pass `freeRangeText(prefs.BarSweep, "°")` as the last argument.

### W3. Template

In the stock table header, insert `<th>Sweep pts</th>` after `<th>Spacer pts</th>`, before `<th>Handlebar</th>`. The row cells already come from `{{range .Lines}}`, so no row change is needed.

### W4. Tests (`web_test.go`)

The stem cell format is `%.0f mm, %+.0f°`, R is `%.2f`, and the total is `%.1f`. Bar names are as in the fixture.

**Update existing tests:**

- `TestBikeWorkbenchRanksStockFitting`: the preference note becomes `Free: HandGrip inside the ±10 × ±20 mm oval, stem length 70–80 mm, stem angle 0°, spacers 0–20 mm, bar sweep 16–20°. Penalties are illustrative except bar sweep (Q30).`. Points cells go from 4 to 5 and headers from 12 to 13. Add `{"bar sweep", "0.0", "Sweep pts"}` to the ledger table (header index 11). The last header is still `Handlebar`. The Persuader totals 78.7, 70.0, 62.2 are unchanged.
- `TestBikeWorkbenchFitsAroundChosenHandlebar` (`bar=sqlab3ox_12_15`): row 1 total `88.6` → `80.6`. The stem, spacers, and R are unchanged.
- `TestBikeWorkbenchStockFittingAcrossAllBars`: replace the expected rows with §5 table A. Row 2 is now `sqlab311_flx_16_15`, so its Apply href checks become `bar=sqlab311_flx_16_15`, `stem_length=130.0`, `stem_angle=-17.0`, and `spacers=0.0`. Keep `stock_bars=all`, `bar_roll=0.0`, `rail_offset=15.0`, `brp_x=-219.0`, and `grip_x=481.0`.
- The "following an all-bars apply link" test: load row 2's href. The selected bar is `sqlab311_flx_16_15`, and row 1 is still `SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)` at `130 mm, -17°`, `0 mm`, total `86.6`.
- "The submitted bar doesn't change the all-bars ranking" (`bar=sqlab3ox_12_15`): compare against table A.
- `TestBikeWorkbenchStockFittingKeepsSubmittedBarRoll`: replace the expected rows with §5 table B. Every Apply href still has `bar_roll=5.0`.
- `TestBikeWorkbenchSubmitsStockBarsSelect`: row 1's bar cell becomes `SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm)`.
- Any other test that asserts a total for a bar outside 16–20° in stock fitting: subtract 2 × (degrees outside). Find them with `grep -n '"total"\|stockCell(.*"total")' internal/reboot/web/web_test.go` and check the bar each one uses.

**New test `TestBikeWorkbenchStockSweepColumn`.** On `/bike?stock_bars=all&grip_x=481&grip_y=726`, every row's `[data-stock-points="bar sweep"]` is `0.0` (all five rows are 16° bars). On `/bike?bar=sqlab311_flx_12_15&grip_x=481&grip_y=726`, every row's sweep cell is `8.0`.

## 5. Reference numbers (measured 2026-11-19)

The probe called the **existing** single-bar `ApplyStockFitting` once per fixture bar (limit 450, so every combination) and added 2 × (degrees outside 16–20°) to each candidate. It merged the results and sorted them by points lost, then R. It is independent of the new ledger code. Default Fenrir build, HandGrip (481, 726), no BRP target (a BRP target doesn't change the ranking; it only carries the saddle fit).

**Table A:** `stock_bars=all`, roll 0, 6750 evaluated.

| Rank | Bar | Stem | Spacers | R | Total |
| --- | --- | --- | --- | --- | --- |
| 1 | SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm) | 130 mm, −17° | 0 | 0.596107 | 86.6 |
| 2 | SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm) | 130 mm, −17° | 0 | 0.606849 | 86.6 |
| 3 | SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm) | 130 mm, −17° | 0 | 0.650924 | 86.6 |
| 4 | SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm) | 130 mm, −17° | 0 | 0.710204 | 86.6 |
| 5 | SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm) | 130 mm, −17° | 5 | 0.767825 | 86.6 |

Every row's ledger is oval 0, length 10, angle 3.4, spacers 0, sweep 0.

**Table B:** `stock_bars=all`, `bar_roll=5`, 6750 evaluated.

| Rank | Bar | Stem | Spacers | R | Total |
| --- | --- | --- | --- | --- | --- |
| 1 | SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm) | 130 mm, −17° | 0 | 0.368028 | 86.6 |
| 2 | SQlab 311 2.0 Alloy 16°, 15 mm rise (740 mm) | 130 mm, −17° | 0 | 0.463537 | 86.6 |
| 3 | SQlab 311 FL-X Carbon 16°, 15 mm rise (740 mm) | 130 mm, −17° | 5 | 0.689285 | 86.6 |
| 4 | SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm) | 130 mm, −17° | 0 | 0.725213 | 86.6 |
| 5 | SQlab 311 2.0 Alloy 16°, 25 mm rise (740 mm) | 130 mm, −17° | 0 | 0.751685 | 86.6 |

**Selected mode.** Persuader 800 (20°): unchanged, rank 1 is 130 mm, −17°, 0, total 78.711983. `bar=sqlab3ox_12_15`: rank 1 is 120 mm, −17°, 0, R 0.452807, total 80.6, with ranks 1–3 tied at 80.6 and separated by R.

Findings worth noting (not defects). With the sweep band, the 16° SQlab bars take the top five, but only by reaching the oval at the longest stem (130 mm, the stem range's maximum), and all five tie at 86.6, separated only by R. The 12° bars' best is now 82.6. If your numbers differ beyond the stated tolerances, find out why before changing a test.

## 6. Not in this slice

Other bar preferences (rise, width, upsweep); roll enumeration (Q24); fork choice (Slice 8); seatposts (Slice 6); any change to `pose` or to `assembly` outside `stock_fitting.go`.

## 7. Rules that bite here

- **Ledger order is fixed:** oval, stem length, stem angle, spacers, bar sweep. Web tests find header and cell positions by index, so the sweep column goes before Handlebar.
- **Q30:** the line is present in both modes. Don't skip it when `stock_bars=selected`.
- **The web never recomputes penalties.** The sweep cell is the candidate's ledger line.
- `exhaustive`: no new switch. `deadcode`: no new helpers outside tests.
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Finish with the implementation handoff (DESIGN.md, "Slice planning"): the Status line, an `## Implementation handoff` section in this file, and the Slice 7 Milestones line. Don't touch any other part of DESIGN.md. Only Matt's acceptance marks the slice done.

## 8. Verification

Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit, re-running it until `go fix` is clean. Then `make serve-reboot` and open `http://127.0.0.1:8081/bike?stock_bars=all&grip_x=481&grip_y=726&view=stem`. Confirm that row 1 is the SQlab 311 2.0 Alloy 16° 15 mm bar at 130 mm, −17°, total 86.6, and that the Sweep pts column reads 0.0. Then switch to `bar=sqlab311_flx_12_15` with "Selected handlebar only" and confirm that every row's Sweep pts reads 8.0. Take a screenshot for Matt.
