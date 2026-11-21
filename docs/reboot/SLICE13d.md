# Slice 13d: Build-fitting vocabulary rename

Status: Planned 2026-11-21

**Rulings:** Q8, Q22, Q33, Q40, Q77, Q97, Q100, Q101, Q102; `AGENTS.md` rules.

**Size:** larger (10 code/test/template files, 2 commits). Written for a lower-tier implementer. **Don't make design choices.** If anything below doesn't match committed code, stop and record `Blocked <date> on <question>` (see "Rules that bite").

*(Sub-slice decision: no split needed. Commit 1 is a pure Go API/file rename with zero UI or URL changes; Commit 2 cuts over `/bike`'s URL parameters, HTML/CSS/hooks, UI copy, and web tests. Both commits compile and pass `make pre-commit` independently.)*

## Goal

1. Replace the misused "stock" discrete-fitting vocabulary in `assembly`, `catalog`, and `web` with the split `Part` / `Build` vocabulary (Q100), with zero geometry, fitting, or scoring behavior changes.
2. Rename `/bike`'s URL parameters `stock_bars`/`stock_forks` → `bar_options`/`fork_options` as a clean break with no compatibility aliases (Q101), update `/bike`'s labels, heading, summary, CSS classes, and test hooks, and verify retired `stock_bars`/`stock_forks` parameters are silently ignored.
3. Preserve `HandlebarSetup.StockWidthMm` and conventional factory/uncut uses of "stock" (Q102).

---

## Commit 1: `Rename stock fitting Go API to PartOptions, FitBuilds, and BuildRanking`

*(Zero URL, HTML, CSS, or rendered-output changes; pure Go API, internal workbench wiring, and file renames.)*

### A1. `internal/reboot/assembly/scoring.go` and `internal/reboot/assembly/scoring_test.go`

1. In `scoring.go`, rename `StockStem` to `StemOption`:
   ```go
   // StemOption is one discrete stem option (length, angle, and 0–100 composite score, Q58, Q97, Q100).
   type StemOption struct {
   	LengthMm float64
   	AngleDeg pose.Deg
   	Score    float64
   }
   ```
2. Update `Validate()` on `StemOption` and its error messages:
   - `"stem option length must be positive and finite, got %v mm"`
   - `"stem option angle must be finite, got %v°"`
   - `"stem option score must be within 0–100, got %v"`
3. In `scoring_test.go`, rename `TestStockStemValidation` → `TestStemOptionValidation` and replace `assembly.StockStem` with `assembly.StemOption`.

### A2. Rename `internal/reboot/assembly/stock_fitting.go` → `internal/reboot/assembly/build_fitting.go` (via `git mv`)

Update the types, methods, functions, comments, and error strings with zero behavior changes:

1. **Exported types & signatures:**
   ```go
   // ForkOption is one fork configuration build fitting may install: a fork at
   // a sag with its lower headset cup (Q31, Q100).
   type ForkOption struct {
   	Fork     ForkSetup
   	LowerCup LowerCupSetup
   }

   // PartOptions lists the discrete part options build fitting may choose from.
   // Stems are the discrete catalog stems (Q97). Spacer stacks start at zero and
   // advance in SpacerStepMm steps up to the setup's hardware limit. Handlebars, when
   // non-empty, lists the bars build fitting may install. Each is installed at
   // the submitted setup's BarRollDeg (Q24). Empty keeps the submitted bar (Q22).
   // Forks, when non-empty, lists the fork configurations build fitting may install.
   // Empty keeps the submitted fork and lower cup (Q33).
   type PartOptions struct {
   	Stems        []StemOption
   	SpacerStepMm float64
   	Handlebars   []HandlebarSetup
   	Forks        []ForkOption
   }

   func (p PartOptions) Validate() error
   ```
   `PartOptions.Validate()` error messages:
   - `"spacer step must be positive and finite, got %v mm"`
   - `"build fitting needs at least one stem option"`
   - `"stem option %d: %w"`
   - `"stem option %d duplicates %v mm at %v°"`
   - `"handlebar option %d must not set BarRollDeg; build fitting installs every bar at the submitted roll (Q24)"`
   - `"handlebar option %d: %w"`
   - `"fork option %d: %w"`
   - `"fork option %d lower cup: %w"`

   ```go
   // ScoredBuild is one cockpit/fork build, placed forward and scored.
   type ScoredBuild struct {
   	Placement          BikePlacement
   	Target             pose.Point  // HandGrip target
   	HasHandlebarOption bool        // true when installed from PartOptions.Handlebars
   	HandlebarIndex     int         // index into PartOptions.Handlebars; meaningful only when HasHandlebarOption
   	HasForkOption      bool        // true when installed from PartOptions.Forks
   	ForkIndex          int         // index into PartOptions.Forks; meaningful only when HasForkOption
   	BrpTarget          *pose.Point // saddle BRP target; nil without one
   	Score              BuildScore  // 4-tier fit gate, 0–100 weighted total, misses, and 12 attribute scores
   }

   func (b ScoredBuild) GripMiss() pose.Point
   func (b ScoredBuild) BrpMiss() (pose.Point, bool)
   func CompareScoredBuilds(a, b ScoredBuild) int

   // BuildRanking is the result of FitBuilds.
   type BuildRanking struct {
   	Builds    []ScoredBuild // best first; at most the requested limit
   	Evaluated int           // combinations placed and scored
   }

   func FitBuilds(setup BikeSetup, target FitTarget, parts PartOptions, weights ScoreWeights, limit int) (BuildRanking, error)
   ```
   `FitBuilds` error messages:
   - `"build fitting needs a HandGrip target"`
   - `invalidInput(InputFitTargetGrip, ...)` / `invalidInput(InputFitTargetBRP, ...)` (unchanged)
   - `"build fitting needs a positive build limit, got %d"`
   - `"score weights have zero active weight for this target"` (unchanged)

2. **Unexported locals and helpers in `build_fitting.go`:**
   - `stockForkChoice` → `forkChoice` (field `stock` → `fromOptions bool`)
   - `stockBar` → `barChoice` (field `stock` → `fromOptions bool`)
   - `stockSteps` → `spacerSteps`
   - Local slices/variables named `stock` → `parts` / `builds`.

### A3. Rename `internal/reboot/assembly/stock_fitting_test.go` → `internal/reboot/assembly/build_fitting_test.go` (via `git mv`)

1. Update all exported type/function/field references (`assembly.StemOption`, `assembly.ForkOption`, `assembly.PartOptions`, `assembly.ScoredBuild`, `assembly.BuildRanking`, `assembly.FitBuilds`, `assembly.CompareScoredBuilds`, `.Builds`, `.HasHandlebarOption`, `.HasForkOption`).
2. Update unexported test helper and test names to drop `Stock` (`fenrirStemOptions`, `fenrirPartOptions`, `twoBarOptions`, `fenrirForkOptions`, `fenrirMultiForkOptions`, `mustFitBuildsWith`, `mustFitBuilds`, `stemConfig`, `stemConfigOf`, `requireStemConfig`, `buildRank`, `TestFitBuilds*`, `TestCompareScoredBuildsTieBreaks`, `TestPartOptionsValidation`).
3. Update the error substring assertions in `TestFitBuildsErrors` and `TestPartOptionsValidation`:
   - `"expected build fitting error"`
   - `"fork option 1: fork sag"`
   - `"fork option 0 lower cup"`
4. **No numeric expectations or tolerances change.**

### C1. `internal/reboot/catalog/stems.go` and `internal/reboot/catalog/stems_test.go`

1. In `stems.go`, rename `StockStems` → `StemOptions`:
   ```go
   // StemOptions returns the 107 catalog stems in assembly.StemOption form for
   // build fitting (Q97, Q100).
   func StemOptions() []assembly.StemOption
   ```
2. In `stems_test.go`, rename `TestStockStems` → `TestStemOptions`, calling `catalog.StemOptions()` and updating log/error text to `"StemOptions = ..."` / `"stem option %d ..."`.

### W1. `internal/reboot/web/workbench.go`, `internal/reboot/web/web.go` (callers only), and `internal/reboot/web/web_test.go` (`validate()` test only)

*(Keep HTML, CSS, URL query keys, and rendered text unchanged until Commit 2.)*

1. In `workbench.go`:
   - `bikeWorkbench.CockpitStock` → `PartOptions assembly.PartOptions`
   - `bikeWorkbench.StockSagPcts` → `SuspensionSagPcts []float64`
   - `stockForkOption` → `forkSagOption` (distinct from `forkOption`)
   - `(w bikeWorkbench) stockForks() []stockForkOption` → `(w bikeWorkbench) forkOptions() []forkSagOption`
   - `stockForkSetups(offered []stockForkOption) []assembly.StockFork` → `forkOptionSetups(offered []forkSagOption) []assembly.ForkOption`
   - Update `bikeWorkbench.validate()` error messages:
     - `"workbench %q part options invalid: %w"`
     - `"workbench %q missing SuspensionSagPcts"`
     - `"workbench %q fork option %q at %v%% sag invalid: %w"`
   - In `fenrirWorkbench()`:
     ```go
     PartOptions: assembly.PartOptions{
     	Stems:        catalog.StemOptions(),
     	SpacerStepMm: 5,
     },
     ScoreWeights:      catalog.DefaultScoreWeights(),
     SuspensionSagPcts: catalog.SuspensionSagPcts(),
     ```
2. In `web.go` (callers only in Commit 1):
   - In `handleBikeWorkbench`: use `offeredForks []forkSagOption`, `wb.forkOptions()`, `parts := wb.PartOptions`, `parts.Handlebars = handlebarSetups(offered)`, `parts.Forks = forkOptionSetups(offeredForks)`, `assembly.FitBuilds(resolved.Setup, target, parts, wb.ScoreWeights, stockCandidateLimit)`.
   - In `buildStockFittingView`: change signature to `(state workbenchState, wb bikeWorkbench, offered []catalog.Handlebar, offeredForks []forkSagOption, ranking assembly.BuildRanking) (*stockFittingView, error)`, iterate `ranking.Builds`, read `ranking.Evaluated`, `c.HasHandlebarOption`, and `c.HasForkOption`.
   - In `renderBikeError` and `buildBikeView`: `len(wb.forkOptions())`.
3. In `web_test.go`:
   - Rename `TestBikeWorkbenchValidatesStockAndPreferences` → `TestBikeWorkbenchValidatesPartOptionsAndWeights`, mutating `wb.PartOptions` and asserting `"part options invalid"`.

---

## Commit 2: `Rename /bike ranked-builds UI, URL parameters, and test hooks`

### W2. `internal/reboot/web/web.go`

1. **URL query parameters (Q101 — clean break, no aliases):**
   - `stock_bars` → `bar_options` with constants `barOptionsSelected = "selected"`, `barOptionsAll = "all"`.
   - `stock_forks` → `fork_options` with constants `forkOptionsSelected = "selected"`, `forkOptionsAll = "all"`.
   - In `decodeWorkbenchState`:
     - Parse `q.Get("bar_options")`; on invalid value: `p.fail("bar_options", fmt.Errorf("unknown handlebar options value %q", raw))`.
     - Parse `q.Get("fork_options")`; on invalid value: `p.fail("fork_options", fmt.Errorf("unknown fork options value %q", raw))`.
   - In `(workbenchState).URL()` and `(workbenchState).formValues()`: emit/populate `bar_options` / `fork_options`.
2. **Internal structs, fields, functions, and constants:**
   - `workbenchState`: `AllStockBars` → `AllBarOptions`, `AllStockForks` → `AllForkOptions` (and update `withSetup`, `renderBikeError`).
   - `bikeFormValues`: `StockBars` → `BarOptions`, `StockForks` → `ForkOptions`.
   - `bikeView`: `StockForkCount` → `ForkOptionCount`.
   - `bikeFittingView`: `Stock *stockFittingView` → `Builds *buildRankingView`.
   - `stockAttributeHeader` → `buildAttributeHeader`, `stockAttributeHeaders` → `buildAttributeHeaders`.
   - `stockFittingView` → `buildRankingView`.
   - `stockCandidateView` → `scoredBuildView`.
   - `buildStockFittingView` → `makeBuildRankingView` (out-of-range errors: `"scored build %d names handlebar %d of %d offered"` and `"scored build %d names fork %d of %d offered"`).
   - `stockPartsText` → `buildPartsText`.
   - `stockCandidateLimit` → `rankedBuildLimit`.

### W3. `internal/reboot/web/templates/bike.html`

1. **CSS classes:** `.stock-fitting` → `.build-ranking`, `.stock-scroll` → `.build-scroll`, `table.stock-table` → `table.build-table`.
2. **Form controls:**
   - Handlebar options:
     ```html
     <label for="ctrl-bar-options">Handlebar options</label>
     <select id="ctrl-bar-options" name="bar_options"{{if and $.ErrorMessage (eq $.InvalidField "bar_options")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
       <option value="selected"{{if ne .Form.BarOptions "all"}} selected{{end}}>Selected handlebar only</option>
       <option value="all"{{if eq .Form.BarOptions "all"}} selected{{end}}>All {{len .Handlebars}} handlebars</option>
     </select>
     ```
   - Fork options:
     ```html
     <label for="ctrl-fork-options">Fork options</label>
     <select id="ctrl-fork-options" name="fork_options"{{if and $.ErrorMessage (eq $.InvalidField "fork_options")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
       <option value="selected"{{if ne .Form.ForkOptions "all"}} selected{{end}}>Selected fork only</option>
       <option value="all"{{if eq .Form.ForkOptions "all"}} selected{{end}}>All {{.ForkOptionCount}} fork setups</option>
     </select>
     ```
3. **Ranked builds block:**
   - `{{with .Builds}}`
   - `<div class="build-ranking" data-build-ranking>`
   - `<strong>Ranked builds</strong>`
   - `<p class="fit-note" data-build-summary><span data-build-evaluated>{{.Evaluated}}</span> {{.PartsText}} combinations placed; top {{len .Rows}} by fit tier and 0–100 weighted score (total weight {{printf "%.0f" .ActiveWeight}}{{if not .HasBrp}}; 4 saddle attributes omitted without BRP target{{end}}). {{if .AllForks}}The saddle is refitted for each fork.{{else}}The saddle fit is kept.{{end}}</p>`
   - `<div class="build-scroll">` and `<table class="build-table">`
   - `<tr data-build-rank="{{.Rank}}">`
   - `<a href="{{.ApplyURL}}" data-preset="apply-build" data-build-apply>Apply</a>`
   - `data-build-cell="tier|score|stem|spacers|grip-miss|oval-r|brp-miss|bar|fork"` (same 9 cell names)
   - `data-build-attr="{{.Attribute}}"`

### W4. `internal/reboot/web/web_test.go`

1. Update all URL keys, selectors, CSS classes, form values, summary assertions, error messages, test helpers, and test names:
   - Query keys: `stock_bars` → `bar_options`, `stock_forks` → `fork_options`.
   - Selectors:
     - `[data-stock-fitting]` → `[data-build-ranking]`
     - `[data-stock-summary]` → `[data-build-summary]`
     - `[data-stock-evaluated]` → `[data-build-evaluated]`
     - `tr[data-stock-rank]` → `tr[data-build-rank]`
     - `a[data-stock-apply]` → `a[data-build-apply]`
     - `data-preset="apply-stock"` → `data-preset="apply-build"`
     - `[data-stock-cell=...]` → `[data-build-cell=...]`
     - `[data-stock-attr=...]` → `[data-build-attr=...]`
     - `table.stock-table` → `table.build-table`
   - Helpers & constants: `stockCell` → `buildCell`, `stockAttr` → `buildAttr`, `assertSelectedStockBars` → `assertSelectedBarOptions`, `assertSelectedStockForks` → `assertSelectedForkOptions`, `stockBarRow` → `barOptionRow`, `assertStockBarRows` → `assertBarOptionRows`, `assertStockForkRows` → `assertForkOptionRows`, `firstStockApply` → `firstBuildApply`, `stockBar*` / `stockFork*` → `bar*` / `fork*`.
   - Summary assertions omit `stock `:
     - `"749 stem and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 23; 4 saddle attributes omitted without BRP target). The saddle fit is kept."`
     - `"handlebar, stem, and spacer combinations"`
     - `"fork, stem, and spacer combinations"`
     - `"78645 fork, handlebar, stem, and spacer combinations placed; top 5 by fit tier and 0–100 weighted score (total weight 36). The saddle is refitted for each fork."`
   - Error page assertions:
     - `unknown handlebar options value "some"`
     - `unknown fork options value "some"`
   - Test function names updated to drop `Stock` (for example `TestBikeWorkbenchBuildRankingRequiresGrip`, `TestBikeWorkbenchRanksBuilds`, `TestBikeWorkbenchBarOptionsDefaultKeepsSubmittedBar`, `TestBikeWorkbenchFitsBuildsAcrossAllBars`, `TestBikeWorkbenchFitsBuildsKeepsSubmittedBarRoll`, `TestBikeWorkbenchBuildSweepColumn`, `TestBikeWorkbenchKeepsBarOptionsWithoutTarget`, `TestBikeWorkbenchSubmitsBarOptionsSelect`, `TestBikeWorkbenchBarOptionsErrorPage`, `TestBikeWorkbenchAppliesFreeScoredBuild`, `TestBikeWorkbenchBuildRankingKeepsSaddleFitAndTargets`, `TestBikeWorkbenchForkOptionsDefaultKeepsSubmittedFork`, `TestBikeWorkbenchFitsBuildsAcrossAllForks`, `TestBikeWorkbenchForkOptionsWithSaddle`, `TestBikeWorkbenchForkOptionsAndBarOptions`, `TestBikeWorkbenchKeepsForkOptionsWithoutTarget`, `TestBikeWorkbenchForkOptionsErrorPage`, `TestBikeWorkbenchBuildTiersAndLegend`).
2. **Add new test `TestBikeWorkbenchIgnoresRetiredStockKeys` (Q101):**
   - Load `/bike?stock_bars=all&stock_forks=all&grip_x=481&grip_y=726`: assert HTTP `200`, zero `#form-error`, `select[name="bar_options"] option[selected]` is `"selected"`, `select[name="fork_options"] option[selected]` is `"selected"`, `[data-build-evaluated]` is `"749"`, and canonical URL (`activeViewHref`) omits `stock_bars`, `stock_forks`, `bar_options`, and `fork_options`.
3. **All numeric expected values (evaluated counts, scores, stems, spacers, misses, tiers, attribute scores) remain identical.**

---

## Mechanics chosen by the planner (veto on review)

1. `web/workbench.go` names the unexported fork+sag option struct `forkSagOption` so it does not collide with `workbench.go`'s existing `forkOption` dropdown struct.
2. Exported option-origin flags on `ScoredBuild` are `HasHandlebarOption` and `HasForkOption` (matching `PartOptions`), replacing `StockHandlebar` and `StockFork`.
3. `decodeWorkbenchState` error messages read `unknown handlebar options value %q` and `unknown fork options value %q`.
4. Retired `stock_bars`/`stock_forks` query parameters are silently ignored (matching `TestBikeWorkbenchIgnoresRetiredPostExtKey` and `TestBikeWorkbenchIgnoresRetiredSetbackKey`), verified by `TestBikeWorkbenchIgnoresRetiredStockKeys`.

## Rules that bite

- `make boundary`: never use the words `price`, `pricing`, `cost`, `cheap`, `ledger`, or `chassis` in reboot file names, identifiers, comments, templates, or tests (in any case or inside longer words).
- `exhaustive`: every `switch` on `FitTier`, `AttributeName`, `InputField`, etc. must name every constant even when it has a `default`.
- Trig stays in `internal/reboot/pose`: outside `pose`, non-test reboot code may only use `math.Abs`, `math.IsNaN`, `math.IsInf` from `math`.
- Import boundaries: `assembly` and `assembly_test` never import `catalog`.
- Q8: never use "search" for fitting in code or docs.
- Q102: keep `HandlebarSetup.StockWidthMm` and `frameset.go`'s `"stock physical lower cup"` comment untouched; do not add `stock` to `Makefile`'s `boundary` check.
- Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Plain-language commit subjects matching the two planned commit titles.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 13d Milestones line.

## Verification

1. `make pre-commit` passes on each of the 2 commits individually.
2. Grep check: `grep -rni 'stock' cmd/bikegeo-reboot/ internal/reboot/` returns **only** `StockWidthMm` (and its uncut-width comments in `handlebar.go`, `handlebar_test.go`, `handlebars.go`, `parts_parity_test.go`), `frameset.go:23` (`"stock physical lower cup"`), and `TestBikeWorkbenchIgnoresRetiredStockKeys` in `web_test.go`.
3. Live check (`make serve-reboot` on `:8081`):
   - Open `http://127.0.0.1:8081/bike?grip_x=481&grip_y=726`: confirm controls read `"Handlebar options"` and `"Fork options"`, heading reads `"Ranked builds"`, summary reads `"749 stem and spacer combinations placed; ..."`, and row 1 is unchanged (`Fits neither`, `56.9`, `120 mm, -30°`, `0 mm`).
   - Open `http://127.0.0.1:8081/bike?post=setback_25&brp_x=-189&brp_y=713.1&grip_x=481&grip_y=726&bar_options=all`: confirm `"11235 handlebar, stem, and spacer combinations placed; ..."` and row 1 is unchanged (`Fits all`, `80.0`, `SQlab 311 2.0 Alloy 16°, 50 mm rise (740 mm)`).
   - Record screenshot path(s) in the handoff, or explain why skipped.

## Out of scope

- Multi-frame `compare` engine, `LeveledReachMm`/`LeveledStackMm`/`EttMm`, `internal/reboot/query`, and `bikegeo-reboot compare` JSON CLI (now Slice 13e).
- Multi-frame `/bike`, `/` compare page, and `bikegeo-reboot bike` JSON CLI (Slice 14).
- Any geometry, fitting, or scoring behavior changes.
- Renaming `HandlebarSetup.StockWidthMm` or adding `stock` to `make boundary` (Q102).
