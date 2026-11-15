Overall, `bikegeo` already gets several fundamentals right: there are **no circular imports**, **no `init()` functions**, [cmd/bikegeo/main.go](file:///Users/matt/src/bikegeo/cmd/bikegeo/main.go) is 11 lines, and [pkg/report/report.go](file:///Users/matt/src/bikegeo/pkg/report/report.go) cleanly decouples command/page output from both CLI and HTML rendering.

Where the codebase drifts from your principles is mostly around a **technical-layer `model` bucket**, **over-broad packages** (`geometry`, `web`), **implicit reliance on package-level catalogs/profiles**, and **everything living under `pkg/` instead of `internal/`**.

---

### Principle-by-Principle Review

#### 1. Organize by domain, not technical layer
* **Current state:** Most packages are domain-named (`frames`, `catalog`, `scoring`, `validator`, `fit`, `compare`, `ideal`), which is good—**except** [pkg/model](file:///Users/matt/src/bikegeo/pkg/model/bike.go) and [pkg/geometry](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go).
* **`pkg/model` is a horizontal `models/` layer with logic attached:** It holds ~1,575 lines of production code across 7 files, mixing pure DTOs (`ComparisonSetup`, `SavedComparisonState`, `ValidationReport`), frame definitions (`FrameModel`), hardware specs (`SaddleSpec`, `HandlebarSpec`, `StemCatalogEntry`), generic utilities (`Opt[T]`), and ~60 methods of kinematics and geometry math (`SaddleSpec.BrpToSitBonesVec`, `SaddleSpec.IdealSeatTubeAngle`, `SaddleSpec.MapPossibleBRPs`, `HeadsetCupType.CupStackMm`, `ErrorBudget.Saturation` in [bike.go](file:///Users/matt/src/bikegeo/pkg/model/bike.go) and [bike_components.go](file:///Users/matt/src/bikegeo/pkg/model/bike_components.go)). Because `pkg/model` sits at the bottom of the graph, any shared type (such as `TargetRef` or `StemSpacerFit`) ends up pushed into `model` to avoid import cycles—as noted in the comment at [scene.go](file:///Users/matt/src/bikegeo/pkg/geometry/scene.go#L11-L13).
* **Recommendation:** Dissolve `pkg/model` over time into domain-owned types:
  * Frame/fork/headset types (`FrameModel`, `BikeGeometry`, `ForkRef`, `HeadsetCupType`, `GeoSlopMm`, `ChartPrecision`, `ErrorBudget`) belong with frame/geometry domain packages.
  * Component specs (`SaddleSpec`, `HandlebarSpec`, `StemSpec`) and their kinematics belong together in a `fit`/`cockpit`/`saddle` domain (or `bike`) rather than split between `pkg/model/bike_components.go` and `pkg/geometry/saddle.go`.
  * Report/validation data (`ValidationIssue`, `ValidationReport`) belongs in `validator`.

#### 2. Keep packages cohesive and reasonably sized
* **Oversized packages:**
  * [pkg/web](file:///Users/matt/src/bikegeo/pkg/web/server.go) is ~3,489 lines across 16 non-test files and imports 11 of the 13 domain/core packages. More importantly, it contains substantial domain assembly and workbench/page-builder logic that has no shared builder outside `web` (e.g. `buildSceneForComparisonRow` in [ledger.go](file:///Users/matt/src/bikegeo/pkg/web/ledger.go#L17-L80), workbench setup resolution in [bike_workbench.go](file:///Users/matt/src/bikegeo/pkg/web/bike_workbench.go#L81-L170), and domain builders in [handlebars.go](file:///Users/matt/src/bikegeo/pkg/web/handlebars.go), [saddles.go](file:///Users/matt/src/bikegeo/pkg/web/saddles.go), and [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/web/saddle_swap.go)).
  * [pkg/geometry](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go) is ~2,687 lines across 10 files and acts as a catch-all for 5 distinct concerns: CAD chart closure & rounding budgets ([budget.go](file:///Users/matt/src/bikegeo/pkg/geometry/budget.go)), frame materialization ([materialize.go](file:///Users/matt/src/bikegeo/pkg/geometry/materialize.go)), saddle/seat-axis & saddle-swap analysis ([saddle.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle.go), [saddle_swap.go](file:///Users/matt/src/bikegeo/pkg/geometry/saddle_swap.go)), handlebar & cockpit fitting ([handlebar.go](file:///Users/matt/src/bikegeo/pkg/geometry/handlebar.go), [cockpit_fit.go](file:///Users/matt/src/bikegeo/pkg/geometry/cockpit_fit.go), [analysis.go](file:///Users/matt/src/bikegeo/pkg/geometry/analysis.go)), and 2D scene construction ([scene.go](file:///Users/matt/src/bikegeo/pkg/geometry/scene.go), 930 lines).
* **Micro/redundant utilities:** `model.Opt[T]` in [opt.go](file:///Users/matt/src/bikegeo/pkg/model/opt.go) (43 lines) and `geometry.Round` in [geometry.go](file:///Users/matt/src/bikegeo/pkg/geometry/geometry.go#L12) vs `ideal.RoundPrinted` in `pkg/ideal`. Notice `pkg/geometry` also re-declares `TargetRef` (`type TargetRef = model.TargetRef`) and `StemSpacerFit` (separate struct from `model.StemSpacerFit`).
* **Recommendation:** Split `pkg/geometry` along real boundaries (e.g., `cad` / `chart` for closure + budgets + materialization; `cockpit` or `fitscene` for `BikeScene`, handlebars, saddles, and cockpit fit), and move `buildSceneForComparisonRow` and the web-only `docb` builders' calculation logic out of `pkg/web` so `pkg/web` is strictly HTTP routing, query parsing, SVG/HTML view-models, and templates.

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
3. **Break up the `pkg/model` horizontal layer & `pkg/geometry` catch-all:** Separate pure geometric primitives/scenes/saddle-bar kinematics from chart-closure/validation budgets, and move domain methods off `pkg/model` into the domain packages that own them.
4. **Slim down `pkg/web`:** Extract non-HTTP workbench/scene/table assembly (like `buildSceneForComparisonRow` in [ledger.go](file:///Users/matt/src/bikegeo/pkg/web/ledger.go#L17)) into domain builders, unexport view-model types only used by templates, and drop the re-exports in [pkg/canonicalizer/canonicalizer.go](file:///Users/matt/src/bikegeo/pkg/canonicalizer/canonicalizer.go#L16-L27).
