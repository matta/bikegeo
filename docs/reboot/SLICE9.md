# M2 Slice 9, score-centered scoring words

**Status: Planned 2026-11-19 from Matt's rulings Q35–Q40 ([DESIGN.md](DESIGN.md), "Scoring words in code"). Not started.** Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. This plan builds on Slice 8 (commits `e29da70`, `6ea4d65`), whose bands it renames.

This slice has three commits and touches 7 code, test, template, and build files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Stock fitting still talks like a shop: `PricedBand` with `CostBelow`/`CostAbove`, an oval `CostPerR`, a `Ledger` of `LedgerLine`s, comments that "charge" and "price", and a workbench note that lists "the cheapest 5 on the points-lost ledger (total = …)". After this slice the code, the page, and DESIGN.md use score words only:

- A band is a `PenaltyBand` whose rates are `PointsPerUnitBelow`/`PointsPerUnitAbove`. The oval's rate is `PointsPerR` (Q35).
- A stock build carries `Penalties []Penalty`, and band names have type `BandName` (Q36).
- A build has a `Score()`, the stock table has a **Score** column, and the note says "the 5 best-scoring (score = 100 − points lost)" (Q37).
- `make boundary` rejects `pric`, `cost`, `cheap`, and `ledger` in reboot files (Q40).

**No scoring value, ranking, URL key, or band name string changes.** Every existing expected number in the tests stays as it is.

Rulings:

- **Q35–Q40** as recorded in DESIGN.md. In particular, *free* stays (Q38: `FreeMin`, `FreeMax`, `TestStockFittingFreeBuild`, `TestBikeWorkbenchAppliesFreeStockBuild`, "Free: …" in the preference note), and AGENTS.md's legacy Architecture bullet "Scores are a points-lost …" is not edited (Q39).
- AGENTS.md: commit subjects are plain language; `exhaustive` and `deadcode` must stay clean.

## 2. Terminology

- **Score** = 100 − points lost. **Penalty** = the points one band takes off (an amount). **Points per unit** = a band's rate. **Best-scoring** replaces "cheapest".
- **Penalty lines** replaces "ledger lines" in prose, comments, and test messages.
- **Stock fitting** / **fit** (Q8). Never write "search". Don't write the term AGENTS.md bans for the frameset.

## 3. Commit 1: rename the assembly identifiers

Subject: `Rename reboot stock fitting identifiers to penalty and score words`

Files: `internal/reboot/assembly/stock_fitting.go`, `internal/reboot/assembly/stock_fitting_test.go`, `internal/reboot/web/fixture.go`, `internal/reboot/web/web.go`.

This commit is a pure rename: identifiers, comments, error messages, and test names. It changes no visible page text (that is commit 2).

### A1. Identifier map (apply everywhere in the four files)

| Old | New |
| --- | --- |
| `PricedBand` (type) | `PenaltyBand` |
| `PricedBand.CostBelow` | `PenaltyBand.PointsPerUnitBelow` |
| `PricedBand.CostAbove` | `PenaltyBand.PointsPerUnitAbove` |
| `FitOval.CostPerR` | `FitOval.PointsPerR` |
| `LedgerBand` (type of `BandGripOval` … `BandBrpMiss`) | `BandName` (constants and their string values unchanged) |
| `LedgerLine` | `Penalty` (fields `Band BandName`, `PointsLost float64` unchanged) |
| `StockCandidate.Ledger` | `StockCandidate.Penalties` |
| `StockCandidate.Total()` | `StockCandidate.Score()` |

`FreeMin`, `FreeMax`, `PointsLost`, `GripR`, and every `Band…` constant keep their names.

### A2. `stock_fitting.go` comments and messages

Replace these exactly (line numbers are from `bc19fb2`):

- L68: `// PenaltyBand is free inside [FreeMin, FreeMax] and penalizes linearly outside it:` + new line `// PointsPerUnitBelow points per unit below FreeMin, PointsPerUnitAbove per unit above FreeMax.`
- L74: `// PointsLost returns the penalty of v under the band.`
- L90 error: `"%s band: points per unit must be finite and non-negative, got %v below and %v above"` (the L87 "free range" error is unchanged).
- L96–97: `// Its major axis follows MajorAxisHeadingDeg in world coordinates. It is free` / `// inside (R ≤ 1) and penalizes PointsPerR points per unit of R beyond the edge.`
- L112: `// PointsLost returns PointsPerR · max(0, r − 1).`
- L117: `// CockpitPreferences holds the bands that score a stock cockpit build (score = 100 − points lost).`
- L128: `// Validate checks the oval and each penalty band.`
- L138 error: `"HandGrip oval: points per R must be finite and non-negative, got %v"`
- L158: `// BandName names one penalty line of a stock cockpit build.`
- L171: `// Penalty is the points one band takes off a build's score.`
- L177: `// StockCandidate is one stock cockpit build, placed forward and scored.`
- L187: `Penalties []Penalty // oval, stem length, stem angle, spacers, bar sweep, fork sag, BRP miss` (let `gofmt` realign the struct).
- L190–196: comment `// PointsLost returns the sum of the penalties' points lost, in penalty order.`; rename the local `total` to `lost` and the loop variable `line` to `p`.
- L199–200: `// Score returns 100 − PointsLost().` and `func (c StockCandidate) Score() float64`.
- L219: `Candidates []StockCandidate // best-scoring first; at most the requested limit`
- L220: `Evaluated  int              // stock combinations placed and scored`
- L328: `result.Penalties = []Penalty{`
- L356 and L364: parameter type `band PenaltyBand`.

### A3. `fixture.go` (L415–L421)

Rename the type and field keys only. Values and trailing comments stay. For example L416 becomes:

```go
StemLength: assembly.PenaltyBand{FreeMin: 70, FreeMax: 80, PointsPerUnitBelow: 0.2, PointsPerUnitAbove: 0.2}, // illustrative
```

and L415's oval uses `PointsPerR: 22`. Let `gofmt` realign the comments.

### A4. `web.go`

- L823: `Lines []assembly.Penalty // assembly's penalty order` (the field keeps the name `Lines` in this commit).
- L1101: `Lines: c.Penalties,`
- L1102: `Total: c.Score(),` (the view field is renamed in commit 2).
- L1128: `func freeRangeText(b assembly.PenaltyBand, unit string) string` (the name stays: Q38).

### A5. `stock_fitting_test.go`

- `illustrativeCockpitPreferences()` (L64–L70): same renames as A3.
- `TestPricedBand` → `TestPenaltyBand`. `TestStockFittingIndependentLedger` → `TestStockFittingIndependentPenalties`. The local type `pricedStem` → `scoredStem`.
- Every `.Ledger` → `.Penalties`, `assembly.LedgerLine` → `assembly.Penalty`, `assembly.LedgerBand` → `assembly.BandName`, `.Total()` → `.Score()`.
- Rename every struct field, local variable, and message word `total` that means the score to `score` (L148–149, L175, L183, L227–228, L325, L339, L349, L399, L410, L454, L469, L575). For example L183 becomes `assertNear(t, "fit report score", c.Score(), want.score, 1e-5)` and L228 becomes `t.Errorf("last score = %v, want a negative score", score)`.
- Message words: L189 `"fit report penalties"`; L368 `"rank %d has %d penalty lines, want 7"`; L446 `"rank %d fork sag and BRP miss penalties = %v, want zero points"`.
- L710–711: `p.GripOval.PointsPerR = -1`, `p.StemLength.PointsPerUnitBelow = -1`.
- **New assertions in `TestCockpitPreferencesValidation`** (after the existing fork sag check), so the reworded messages are pinned:

  ```go
  prefs = illustrativeCockpitPreferences()
  prefs.StemLength.PointsPerUnitBelow = -1
  if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "stem length band: points per unit must be finite and non-negative") {
      t.Errorf("expected stem length rate error, got %v", err)
  }
  prefs = illustrativeCockpitPreferences()
  prefs.GripOval.PointsPerR = -1
  if err := prefs.Validate(); err == nil || !strings.Contains(err.Error(), "HandGrip oval: points per R must be finite and non-negative") {
      t.Errorf("expected oval rate error, got %v", err)
  }
  ```

No expected number changes. `web_test.go` and `handlebar_parity_test.go` need no change in this commit (they use none of the renamed identifiers; confirm with `go vet ./...`).

## 4. Commit 2: say Score and best-scoring on the page

Subject: `Show a Score column and best-scoring note in the reboot stock table`

Files: `internal/reboot/web/web.go`, `internal/reboot/web/templates/bike.html`, `internal/reboot/web/web_test.go`.

### W1. `web.go`

In `stockCandidateView`:

- `Lines []assembly.Penalty` → `Penalties []assembly.Penalty // assembly's penalty order`
- `Total float64` → `Score float64`

In `buildStockFittingView`: `Penalties: c.Penalties,` and `Score: c.Score(),`.

### W2. `bike.html`

- L631 becomes exactly (one line; it gains `data-stock-summary`):

  ```html
  <p class="fit-note" data-stock-summary><span data-stock-evaluated>{{.Evaluated}}</span> stock {{.PartsText}} combinations placed; the {{len .Rows}} best-scoring (score = 100 − points lost). {{if .AllForks}}The saddle is refitted for each fork.{{else}}The saddle fit is kept.{{end}}</p>
  ```

- L636: `<th>Total</th>` → `<th>Score</th>`. No other header changes.
- L643: `<td class="mono" data-stock-cell="score"><strong>{{printf "%.1f" .Score}}</strong></td>`
- `{{range .Lines}}` → `{{range .Penalties}}`. The `data-stock-points="{{.Band}}"` attribute values are unchanged.

`static/` has no reference to the `total` cell (checked at `bc19fb2`); grep `static/` again for `total` and stop if that changed.

### W3. `web_test.go`

- Every `data-stock-cell` key `"total"` → `"score"` (L2800, L2837, L2932, L3091, L3114, L3295, L3669, L3689, L3723) and the `stockBarRow` field `total` → `score` (L2921 and its uses). Messages say `score` (for example L2801 `"row %d score = %q, want %q"`, L3115 `"row 1 score = %q, want 1.5"`).
- L2829–2831: comment `// Apply and Score sit next to the rank …`, want `"Score"`, message `"header 3 = %q, want Score"`.
- L2838: `"row 1 cell 3 = %q, want score"`.
- L2823 and L2826: `"penalty cell %d …"` and `"penalty header %d …"`.
- **New assertion in `TestBikeWorkbenchRanksStockFitting`** (`/bike?grip_x=481&grip_y=726&view=stem`), after the evaluated check:

  ```go
  const summary = "450 stock stem and spacer combinations placed; the 5 best-scoring (score = 100 − points lost). The saddle fit is kept."
  if got := doc.Find("[data-stock-summary]").Text(); got != summary {
      t.Errorf("stock summary = %q, want %q", got, summary)
  }
  ```

- **New assertion in `TestBikeWorkbenchStockForksAndBars`** (`/bike?stock_forks=all&stock_bars=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726`):

  ```go
  const summary = "47250 stock fork, handlebar, stem, and spacer combinations placed; the 5 best-scoring (score = 100 − points lost). The saddle is refitted for each fork."
  ```

  checked the same way. Both strings are today's rendered text (probed at `bc19fb2`: "450 stock stem and spacer …" and "47250 stock fork, handlebar, stem, and spacer …") with only the "cheapest … ledger (total" wording replaced. Compare exactly; no trimming.

All expected scores (78.7, 70.0, 62.2, 100.0, 1.5, 80.6, …) are unchanged.

## 5. Commit 3: ban the pricing words in reboot files

Subject: `Ban pricing words in reboot files`

Files: `Makefile`, `AGENTS.md`, `docs/reboot/DESIGN.md`.

### B1. `Makefile`

In the `boundary` target, directly after the `chassis` block, add:

```make
	@bad="$$({ \
	  find cmd/bikegeo-reboot internal/reboot \( -iname '*pric*' -o -iname '*cost*' -o -iname '*cheap*' -o -iname '*ledger*' \) -print; \
	  find cmd/bikegeo-reboot internal/reboot -type f -exec grep -HinIE 'pric|cost|cheap|ledger' {} +; \
	} || true)"; \
	if [ -n "$$bad" ]; then \
	  echo "$$bad"; \
	  echo "boundary: reboot files must not use pricing words (pric, cost, cheap, ledger); say score, penalty, points per unit, or best-scoring"; \
	  exit 1; \
	fi
```

Use tabs for recipe indentation, like the block above it. `-I` skips binary files.

### B2. `AGENTS.md`

Only the reboot section's **Terminology** bullet changes (Q39 leaves the legacy "Scores are a points-lost …" bullet alone). Append this sentence to the end of that bullet:

> Scoring uses score words: `pric`, `cost`, `cheap`, and `ledger` are banned in reboot files too (`make boundary`; DESIGN.md "Scoring words").

### B3. `docs/reboot/DESIGN.md`

- **Scoring words** (Working rules): replace the sentence "Code still says `PricedBand` and `CostBelow`/`CostAbove` until Slice 9 renames them (Q35–Q40). Name those identifiers as they are until then." with:

  > Call a band's rate its points per unit, the list of a build's penalties its penalty lines, and the best builds best-scoring. `make boundary` bans `pric`, `cost`, `cheap`, and `ledger` in reboot files (Q40).

- Reword the older rulings and milestone lines (substance unchanged, Q39):
  - Q18: "each with its ledger lines" → "each with its penalty lines".
  - Q30: "The line appears on every stock fitting ledger, including" → "The line appears in every stock fitting result, including".
  - Q34: "it appears on every stock fitting ledger that has a BRP target" → "it appears in every stock fitting result that has a BRP target"; "every total drops by 77.2 points" → "every score drops by 77.2 points".
  - Slice 3: "scoring each on the points-lost ledger (Q17 oval" → "scoring each as 100 − points lost (Q17 oval".
  - Slice 7: "as a fifth ledger line" → "as a fifth penalty line".
  - Slice 8: "The ledger gains a fork sag line and a BRP miss line" → "Each build gains a fork sag penalty line and a BRP miss penalty line".
- After these edits, `grep -niE 'pric|cost|cheap|ledger' docs/reboot/DESIGN.md` must print only the Scoring words bullet and the Q40 ruling. Fix any other hit by the same kind of rewording, and list it under Deviations.

Then do the implementation handoff (section 8): the Status line, the handoff section, and the Milestones line for Slice 9.

## 6. Rules that bite

- **Each commit passes `make pre-commit` alone.** Commit 1 must leave the page text identical (the existing web tests prove it). Commit 3's ban must pass on the tree commit 2 leaves; if `make boundary` finds a hit you didn't expect, reword it rather than weakening the pattern, and record it under Deviations.
- **Don't touch legacy code or the legacy AGENTS.md bullet.** The ban covers only `cmd/bikegeo-reboot` and `internal/reboot`. A future `*_parity_test.go` that needs a legacy `Cost` field would hit the ban; none does today, and adding an exemption is out of scope.
- **Don't rename** `FreeMin`, `FreeMax`, `freeRangeText`, the `Band…` constants or their strings, `PointsLost`, URL keys, `data-stock-points` values, or any other header than Total → Score.
- **No number moves.** If any expected score, rank, or count changes, stop: a rename changed behavior.
- `exhaustive`: `BandName` has no switches today; if you find one, it must still list every constant.
- `frameset_test.go` L123 says "Total front-end length". It is a length, not a score. Leave it.
- **Finish with the implementation handoff** in DESIGN.md "Slice planning" → "Implementation handoff", whether you finished, stopped early, or got blocked.

## 7. Verification

1. After every edit: `go vet ./... && go test ./...`. Before each commit: `make pre-commit`, re-run until `go fix` is clean.
2. After commit 1: `grep -rnE 'PricedBand|CostBelow|CostAbove|CostPerR|LedgerLine|LedgerBand|\.Ledger\b|\.Total\(\)' internal/reboot` prints nothing.
3. After commit 2: `grep -rniw total internal/reboot | grep -v frameset_test.go` prints nothing.
4. After commit 3, probe the ban: `printf 'ledger\n' > internal/reboot/pose/ban_probe.txt && make boundary` must fail and name `ban_probe.txt`; then `rm internal/reboot/pose/ban_probe.txt` and `make boundary` passes. Don't commit the probe.
5. **Live check:** `make serve-reboot`, open `http://127.0.0.1:8081/bike?grip_x=481&grip_y=726&view=stem`. Confirm the stock table's third header is **Score**, rows 1–3 score 78.7, 70.0, 62.2, and the note reads "450 stock stem and spacer combinations placed; the 5 best-scoring (score = 100 − points lost). The saddle fit is kept." Save a screenshot of the stock table to the session scratchpad and name its full path in the handoff.

## 8. Implementation handoff

Required. Follow DESIGN.md "Slice planning" → "Implementation handoff": set this file's Status line, add the `## Implementation handoff` section right after the Status paragraph, and set the Slice 9 Milestones line in DESIGN.md to the same status words.

## 9. Out of scope

- Legacy code and AGENTS.md's legacy Architecture bullet (Q39).
- Renaming *free* (Q38) or changing any band value, ruling substance, or ranking.
- New bands, new columns, or reordering the stock table.
- A parity-test exemption from the ban.
