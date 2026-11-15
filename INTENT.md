Overall, `bikegeo` already gets several fundamentals right: there are **no circular imports**, **no `init()` functions**, [cmd/bikegeo/main.go](cmd/bikegeo/main.go) keeps the entry point thin, and [internal/report/report.go](internal/report/report.go) cleanly decouples command/page output from both CLI and HTML rendering.

The original review flagged a **technical-layer `model` bucket**, **over-broad packages** (`geometry`, `web`), **implicit reliance on package-level catalogs/profiles**, and **application packages living under `pkg/` instead of `internal/`**.

**Review status (2026-11-15):** The model/geometry findings in sections 1–2 and refactoring 3 below have been reassessed against the current code. Packages now live under `internal/`, `main` injects `web.RunServe` into `cli.Run`, and `geometry` no longer imports `catalog`. The remaining sections retain the original review and may describe issues already resolved; their `pkg/` paths are historical. The focused assessment below supersedes the original recommendation to dissolve `model` wholesale.

---

### Principle-by-Principle Review

#### 1. Organize by domain, not technical layer

* **Current state:** Most packages are domain-named (`frames`, `catalog`, `scoring`, `validator`, `fit`, `compare`, `ideal`). [internal/model](internal/model/bike.go) holds ~1,575 production lines across 7 files, mixing shared bicycle vocabulary, component calculations, validation results, and comparison state. Its name and size alone do not establish a problem; a shared domain vocabulary can be useful.
* **Concrete ownership problems:** Saddle and handlebar types and some of their calculations live in [model/bike_components.go](internal/model/bike_components.go), while related analysis lives in [geometry/saddle.go](internal/geometry/saddle.go), [geometry/saddle_swap.go](internal/geometry/saddle_swap.go), and [geometry/handlebar.go](internal/geometry/handlebar.go). `TargetRef` is defined in `model` and re-exported by `geometry`, explicitly to avoid import cycles. `StemSpacerFit` is separately defined in both packages, requiring field-by-field conversion in analysis. These are stronger reasons to reconsider the boundary than the existence of methods on model types.
* **Aggregate ownership needs design first:** [CanonicalComparisonRow](internal/model/fit_types.go) combines setup, bike geometry, validation, baseline, and fit results. Moving `ValidationReport` into `validator` while keeping this aggregate in `model` would create `model → validator → model`. Moving the row into `compare` would create a different cycle because [ideal's attribute registry](internal/ideal/registry.go) consumes the row and `compare` depends on `ideal`. These moves require dependency redesign, not just declaration relocation.
* **Recommendation:** Co-locate component types and their calculations when a focused extraction simplifies real work. Keep methods with their types: `HeadsetCupType.CupStackMm` and `ErrorBudget.Saturation` are appropriate methods, wherever those types eventually live. Defer wholesale redistribution of `model` until aggregate ownership and dependency direction are settled. Do not force component types into the existing `fit` package, whose responsibility is CAD diagnosis and solving, merely because both concern “fit.”

#### 2. Keep packages cohesive and reasonably sized
* **Oversized packages:**
  * [pkg/web](file:///Users/matt/src/bikegeo/pkg/web/server.go) is ~3,489 lines across 16 non-test files and imports 11 of the 13 domain/core packages. More importantly, it contains substantial domain assembly and workbench/page-builder logic that has no shared builder outside `web` (e.g. `buildSceneForComparisonRow` in [ledger.go](file:///Users/matt/src/bikegeo/pkg/web/ledger.go#L17-L80), workbench setup resolution in [bike_workbench.go](file:///Users/matt/src/bikegeo/pkg/web/bike_workbench.go#L81-L170), and domain builders in [handlebars.go](file:///Users/matt/src/bikegeo/pkg/web/handlebars.go), [saddles.go](file:///Users/matt/src/bikegeo/pkg/web/saddles.go), and [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/web/saddle_swap.go)).
  * [internal/geometry](internal/geometry/geometry.go) is ~2,682 production lines across 9 files, covering chart closure and rounding budgets, frame materialization, saddle and handlebar calculations, cockpit fitting, and 2D scene construction. However, it now imports only `model`, `geom2d`, and the standard library. [Materialization](internal/geometry/materialize.go) accepts expanded stems explicitly, and [analysis](internal/geometry/analysis.go) requires an explicit saddle and valid rider/handlebar inputs. The earlier catalog-coupling rationale has already been addressed.
* **Useful distinction:** Chart precision, rounding budgets, and declared allowances explain whether published dimensions agree. Installed-bike scene calculations describe a physical setup. Separating those responsibilities could clarify where manufacturer-data interpretation ends and physical geometry begins. Generic geometric primitives already have a separate home in `geom2d`.
* **The boundary is not a file move:** [BuildScene](internal/geometry/scene.go) uses `EvaluateChartClosure`, which is also used by budgets and validation. `FrameSizeFromBikeGeometry` uses both `A2cBudget` and `ChartPrecisionFor`. Extracting chart policy therefore requires deciding where this adapter belongs and retaining one shared implementation of closure equations. Duplicating formulas or adding conversion structs just to cross a new package boundary would weaken the result.
* **Utilities are not a reason to fragment packages:** `model.Opt[T]` and `model.Deg` are small shared building blocks. They do not each need a new package as part of this work. Prioritize the duplicate `StemSpacerFit` definition and split component behavior over generic utility relocation or package-size targets.
* **Recommendation:** Consider one bounded extraction of chart policy or component behavior, chosen to simplify an upcoming change. Preserve shared physical mathematics and explicit inputs. The original web-extraction proposal remains a separate concern; it is not a prerequisite for the model/geometry work.

#### Focused assessment: is the model/geometry breakup worth doing?

**Selectively, at moderate priority. A complete breakup is low priority until its dependency design and practical payoff are clearer.**

| Potential benefit | Cost or limitation |
| --- | --- |
| Component types and their calculations become easier to find and change together. | Moving them touches catalog declarations, geometry, profiles, and callers; a new package must own a coherent capability. |
| Consolidating duplicate result types removes conversions and the possibility of definitions drifting. | Shared ownership must be resolved; aliases alone can preserve the old coupling under new names. |
| Separating chart uncertainty policy from installed-bike calculations clarifies responsibilities. | Both use the same closure mathematics, and current adapters depend on both. A split must preserve a one-way dependency graph and one formula implementation. |
| Giving comparison and validation aggregates explicit owners reveals hidden coupling. | Their consumers span `ideal`, scoring, canonicalization, and comparison. Redistributing all types is a larger redesign. |

The existing structure already has no package cycles, explicit inputs in the numerical core, and a separate `geom2d` package. Package relocation alone improves neither numerical results nor runtime performance. Broad churn consumes review effort around sensitive calculations without an immediate user-visible benefit.

| Candidate change | Assessment |
| --- | --- |
| Co-locate saddle/handlebar types and their calculations | Worthwhile when next extending that area. |
| Separate chart precision and budget policy from physical scene calculations | Worth a bounded refactor once shared equations and adapter ownership are designed. |
| Consolidate duplicate result types | Useful concrete cleanup, with ownership resolved first. |
| Redistribute every `model` type into existing packages | Defer until aggregate ownership and dependency direction are settled. |
| Create separate packages for every small concept | Unlikely to repay the added navigation. |

Choose one boundary that removes actual duplication or makes an upcoming change simpler. Keep type methods with their types, preserve published chart values and numerical behavior, and use the existing geometry, validation, and integration tests to verify the extraction. Judge success by fewer conversions, clearer ownership, and easier subsequent changes—not by eliminating the package named `model`.

#### 3. Name packages for what they provide, not what they contain
* **Plural "container" names:** `frames`, `model`, and `catalog` are named for what they hold, while `canonicalizer` and `validator` are `-er` agent nouns.
* **Stuttering across the API:**
  * `catalog.CommonForkCatalog`, `catalog.CommonSaddleCatalog`, `catalog.CommonStemCatalog`, `catalog.CommonHandlebarCatalog`, `catalog.GetCatalogFork`, `catalog.GetCatalogStem` ([catalog.go](file:///Users/matt/src/bikegeo/pkg/catalog/catalog.go), [saddles.go](file:///Users/matt/src/bikegeo/pkg/catalog/saddles.go), [handlebars.go](file:///Users/matt/src/bikegeo/pkg/catalog/handlebars.go)) -> `catalog.Forks`, `catalog.Fork(id)`, `catalog.Stem(id)` read much more cleanly.
  * `validator.ValidateBikeGeometry`, `validator.ValidateForkRef`, `validator.ValidateFrame` ([validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go)) -> `validator.Frame(...)`, `validator.Bike(...)` (or `validate.Frame(...)`).
  * `scoring.ScoreRow`, `scoring.ScoreAllRows`, `scoring.ScoredFrame` ([scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go)) -> `score.Row(...)`, `score.All(...)`.
  * `ideal.ValidateProfile`, `ideal.IdmatchProfile` -> `ideal.Profile` / `ideal.Validate`.
  * `frames.AllFrames`, `frames.GetFrameByID` -> `frame.All()`, `frame.ByID(id)`.
  * `geometry.CalculateGeometricA2c`, `geometry.CalculateTrail`, `geometry.AnalyzeBikeFitWithSaddle`, `geometry.AnalyzeSaddleSwap` — verbose Java-style `Calculate*` / `Get*` prefixes throughout `geometry` and `model` (`GetCadA2c`, `GetGeoHeadsetSpacerMm`, `GetSizes`).

#### 4. Design packages from the caller's perspective (minimal exported surface)
* **Pass-through re-exports:** [pkg/canonicalizer/canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L16-L27) re-exports `ForkSuggestion`, `CommonForkCatalog`, `GetCatalogFork`, and `GetStandardForkForTravel` from `pkg/catalog` ("Re-export catalog types and helpers for canonicalizer callers"). Callers should depend on `catalog` directly or pass the lookup in; re-exports widen the API surface without adding an abstraction.
* **Internal helpers exported prematurely:**
  * [pkg/compare](file:///Users/matt/src/bikegeo/pkg/compare/specs.go) exports `FormatAttrRange`, `FormatCost`, `DisambiguateFrameNames`, `PruneRows`, `ResolveAllColumns` alongside `Run` and `BuildDocument`.
  * [pkg/fit/fit.go](file:///Users/matt/src/bikegeo/pkg/fit/fit.go) exports `MaxTryCombinations`, `CloneFrame`, `ApplyAssignment`, `ParseTryFlag`, `TrySpec` even though [pkg/cli/fit.go](file:///Users/matt/src/bikegeo/pkg/cli/fit.go) only uses `RunSweeps`, `BuildSweepDocument`, and `BuildAnalysisDocument`.
  * [pkg/web](file:///Users/matt/src/bikegeo/pkg/web/server.go) exports `CompareQuery`, `ParseCompareQuery`, `CellView`, `Control`, `FieldError`, `NavItem`, `NudgeControl`, `NudgeLink`, `Option`, `RowView`, `TableView`, `LayoutAuto` — most of which are HTML template view structs that only need exported *fields* for `html/template`, not exported *type names*.
  * [pkg/frames](file:///Users/matt/src/bikegeo/pkg/frames/frames.go) exports every individual `FrameModel` variable (`BinaryHavok`, `MootsMountaineer`, etc.) even though production callers use `AllFrames()` / `GetFrameByID()`.
* **Recommendation:** Unexport view structs in `pkg/web` (keep fields exported for `html/template`), drop the `catalog` re-exports in `pkg/canonicalizer`, and audit `compare`, `fit`, `geometry`, and `frames` exports against actual external callers.

#### 5. Define interfaces where they are consumed, not implemented
* **Current state:** There are **zero interfaces** defined anywhere in `cmd/` or `pkg/`. Because all frames, catalogs, and profiles are compiled-in Go structs and there is no database or network I/O, concrete types work cleanly in most places—and you should **not** invent interfaces speculatively.
* **Where small consumer-side interfaces *would* pay off:**
  1. **Decoupling domain engines from global catalogs/frames:** `compare`, `canonicalizer`, `validator`, `fit`, and `geometry` currently reach directly into `frames.AllFrames()` and `catalog.GetCatalogFork` / `catalog.DefaultSaddle()` / `catalog.DefaultHandlebar()`. Small 1-method consumer interfaces (e.g., `type FrameStore interface { Frame(id string) (model.FrameModel, bool) }` or passing slices/structs explicitly) would break those hard imports and make tests independent of the compiled catalog.
  2. **Command dispatch / rendering boundary:** `pkg/cli` currently imports `pkg/web` (`web.RunServe`) just to launch the HTTP server, and `pkg/web` duplicates command-line wiring (`serve` flags vs `cli` flags). Defining a runner or injecting the `serve` handler from `main` avoids `cli -> web` coupling.

#### 6. Keep `main` thin and use `internal/` to enforce boundaries
* **`main` is thin, but wiring lives in `cli`:** [cmd/bikegeo/main.go](file:///Users/matt/src/bikegeo/cmd/bikegeo/main.go) is 11 lines calling `cli.Run(...)`. However, `pkg/cli` owns wiring `serve` to `pkg/web` (`pkg/cli/cli.go:10`, `:62`, `:80`), and `pkg/cli` also imports `pkg/frames`, `pkg/catalog`, and `pkg/model` directly.
* **Everything is in `pkg/` instead of `internal/`:** There is no `internal/` directory at all. `bikegeo` is a self-contained CLI + local web server (`go.mod` has no library consumers). Keeping 14 packages in `pkg/` advertises a public supported library API to external modules and prevents the Go compiler from enforcing internal boundaries.
* **Recommendation:** Move application packages (`cli`, `web`, `compare`, `fit`, `canonicalizer`, `validator`, `scoring`, `ideal`, `geometry`, `catalog`, `frames`, `model`, `report`, `geom2d`) under `internal/` (unless you intentionally publish a stable Go library subset like `geom2d` or `frames`). Wire `serve` in `cmd/bikegeo/main.go` (e.g., passing a `serveFunc` or `ServeCommand` to `cli.Run`) so `internal/cli` does not need to import `internal/web`.

#### 7. Pass dependencies explicitly rather than using package-level state
* **What's good:** Zero `init()` functions, and compiled catalogs/frames are functionally immutable data (though `frames.AllFrames()` defensively clones on every call to guard against mutation of package-level vars).
* **Hidden default/global dependencies:**
  * [pkg/compare/run.go](file:///Users/matt/src/bikegeo/pkg/compare/run.go#L42-L47) calls `frames.AllFrames()` and `DefaultComparisonState()` internally rather than accepting frames/baseline state as parameters.
  * [pkg/scoring/scoring.go](file:///Users/matt/src/bikegeo/pkg/scoring/scoring.go#L46) (`Fits`, `ScoreRow`, `ScoreAllRows`) silently falls back to `ideal.IdmatchProfile()` when `prof == nil`.
  * [pkg/canonicalizer/canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L535), [pkg/geometry/analysis.go](file:///Users/matt/src/bikegeo/pkg/geometry/analysis.go#L20), and [pkg/geometry/cockpit_fit.go](file:///Users/matt/src/bikegeo/pkg/geometry/cockpit_fit.go#L38) silently fall back to `catalog.DefaultSaddle()`, `catalog.DefaultHandlebar()`, and `catalog.DefaultRiderProfile()` when fields are empty, and `geometry.materialize.go` calls `catalog.ExpandStemSets` directly.
  * [pkg/validator/validator.go](file:///Users/matt/src/bikegeo/pkg/validator/validator.go#L602) calls `catalog.GetCatalogStem` and `catalog.GetCatalogFork` as globals; [pkg/fit/fit.go](file:///Users/matt/src/bikegeo/pkg/fit/fit.go#L752) iterates `catalog.CommonForkCatalog` directly.
  * [pkg/ideal/registry.go](file:///Users/matt/src/bikegeo/pkg/ideal/registry.go#L449-L455) builds `registryMap` via an anonymous IIFE `var registryMap = func() ... ()` (functionally an `init()`).
  * [pkg/web/server.go](file:///Users/matt/src/bikegeo/pkg/web/server.go#L30) uses package-level `nav` and `pages()` rather than attaching routes/deps to a `Server` struct constructed with explicit dependencies.
* **Recommendation:** Resolve defaults (`IdmatchProfile`, `DefaultSaddle`, `DefaultHandlebar`, `DefaultComparisonState`) once at the entry boundary (`cli` / `web` / `main`) and pass them down explicitly into `compare.Run`, `canonicalizer`, `geometry`, `scoring`, `validator`, and `fit`. Require `prof != nil` in `scoring` instead of silently substituting `IdmatchProfile()`. That eliminates `pkg/geometry -> pkg/catalog` and `pkg/compare -> pkg/frames` hard dependencies in one stroke.

#### 8. Ensure the dependency graph is a strict DAG
* **Current state:** Because Go forbids package cycles, the import graph is already a strict DAG:
  * Layer 0: `geom2d`, `report`
  * Layer 1: `model` (`geom2d`)
  * Layer 2: `catalog`, `frames`, `ideal`
  * Layer 3: `geometry` (`catalog`, `geom2d`, `model`)
  * Layer 4: `validator` (`catalog`, `geometry`, `model`, `report`), `scoring` (`geometry`, `ideal`, `model`)
  * Layer 5: `canonicalizer` (`catalog`, `geometry`, `ideal`, `model`, `validator`), `fit` (`catalog`, `geometry`, `model`, `report`, `validator`)
  * Layer 6: `compare` (imports 9 packages), `web` (imports 11 packages), `cli` (imports 8 packages including `web`)
  * Entry: `cmd/bikegeo` -> `cli`
* **Where the layering leaks:**
  1. `geometry` (core math) imports `catalog` (data) only for default components and stem expansion ([materialize.go](file:///Users/matt/src/bikegeo/pkg/geometry/materialize.go#L65), [analysis.go](file:///Users/matt/src/bikegeo/pkg/geometry/analysis.go#L20), [cockpit_fit.go](file:///Users/matt/src/bikegeo/pkg/geometry/cockpit_fit.go#L38)). Inverting that (passing expanded stems and resolved saddle/handlebar into `geometry`) turns `geometry` into a pure math layer depending only on `geom2d` + core domain types.
  2. `cli` imports `web` rather than `cmd/bikegeo` wiring both transports.
  3. `canonicalizer` imports `validator` (just to validate frames during canonicalization?), coupling setup resolution to report/chart validation.
  4. `web` reaches down into `geom2d`, `frames`, `catalog`, `geometry`, `canonicalizer`, `scoring`, `ideal`, `validator`, and `compare`—acting as both HTTP adapter and orchestration layer.

#### 9. Start flat and introduce hierarchy only when complexity demands it
* **Current state:** The package hierarchy is reasonably flat (depth 1 under `pkg/`), with no unnecessary nested trees like `pkg/a/b/c`.
* **Imbalance:** Some packages are split prematurely into small single-purpose siblings (`canonicalizer` + `compare` + `scoring` + `ideal`, or `catalog` + `frames`), while `geometry` (2.7k LOC) and `web` (3.5k LOC) grew very large at the same flat level.
* **Recommendation:** When moving to `internal/`, you do **not** need deep nesting—a clean ~8–10 package flat layout under `internal/` is ideal for a ~30k LOC codebase (including tests). Consider merging `canonicalizer` + `compare` (or keeping `compare` as the orchestration package and folding `fork_format`/resolution into it) and splitting `web`'s workbench/page-building logic from HTTP transport, while keeping `internal/` only one level deep.

#### 10. Co-locate tests with the code they test
* **Current state:** Every `*_test.go` file is properly co-located with its package!
* **White-box vs black-box mix:**
  * External (`package foo_test`) is used consistently in `cli`, `canonicalizer`, `fit`, `validator`, and most of `geometry` (`geometry_test.go`, `scene_test.go`, `saddle_test.go`, `handlebar_test.go`, `budget_test.go`, `seat_axis_test.go`). That is great practice.
  * Internal (`package foo`) is used in `geom2d`, `model`, `catalog`, `ideal`, `scoring`, `compare`, `report`, `web`, plus `geometry/canonical_hb_test.go` and `geometry/saddle_swap_test.go`.
* **Recommendation:** Shift `compare`, `scoring`, `ideal`, `catalog`, `geom2d`, and `report` tests to `package foo_test` where they only test exported behavior (keep `package foo` only for genuinely unexported unit tests, e.g. `web`'s `query_test.go` or `pages_content_test.go` if testing unexported helpers). Moving `package compare_test` and `package scoring_test` to black-box tests will immediately expose any unnecessarily exported internal helpers.

---

### Top 4 High-Leverage Refactorings (If You Want to Act on This)

1. **Move `pkg/` -> `internal/` and wire `serve` in `cmd/bikegeo/main.go`:** Enforces module boundaries via the Go compiler and removes the `cli -> web` dependency.
2. **Invert global `catalog`/`frames`/`ideal` lookups into explicit inputs:** Pass `FrameModel` slices, resolved `SaddleSpec`/`HandlebarSpec`/`StemSpec`s, and non-nil `*ideal.Profile` from the application boundary (`cli`/`web`/`compare`) down into `geometry`, `canonicalizer`, `scoring`, and `validator`. This immediately eliminates `geometry -> catalog` and makes unit testing much cleaner.
3. **Selectively clarify `internal/model` and `internal/geometry` ownership (reassessed 2026-11-15):** Moderate priority for one focused extraction that co-locates component types and calculations, separates chart policy from physical scene calculations, or consolidates duplicate result types. Keep methods with their types and shared closure equations in one place. Defer a complete breakup until comparison/validation aggregate ownership and dependency direction are designed; package names and line counts alone do not justify it. See the focused assessment above.
4. **Slim down `pkg/web`:** Extract non-HTTP workbench/scene/table assembly (like `buildSceneForComparisonRow` in [ledger.go](file:///Users/matt/src/bikegeo/pkg/web/ledger.go#L17)) into domain builders, unexport view-model types only used by templates, and drop the re-exports in [pkg/canonicalizer/canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L16-L27).
