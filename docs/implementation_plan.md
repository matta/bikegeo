# Package-Sealed Delegation & `BikeSetup` Retirement Plan

Status: Planned 2026-11-27
Implementer: High (Frontier) for Stages 1–3, Medium for Stages 4–5 — seals package boundaries, delegates joint vectors and part sketches, and retires `BikeSetup`
Review: required — architectural package boundaries, kinematic joint projection, and CLI JSON readout contract

## Goal Description

Finish decoupling `bikegeo` along genuine object and invariant boundaries while keeping typed outer bicycle stages (`Frameset` → `SeatedFrameset` → `Build`) and preserving all physical geometry math, catalog invariants, ranking results, and SVG visual outputs:

1. **Compiler-enforced sealing via a 4-package domain split (`bounds` → `parts` → `assembly` → `scoring`):** replace the single 16-file `internal/assembly` scope (where outer orchestrators can read private component fields or literal-construct `&Build{...}`) with strict package walls so private invariants are sealed by the Go compiler.
2. **Self-describing 1-DoF translational joints (`bounds.SlideJoint`):** let each sliding joint (`Insertion`, `RailOffset`, `Spacers`, continuous `StemLength`, hypothetical `Setback`) own its `BoundedRange` and local unit movement vector (`stepPerMm`) in its parent `CoordinateSystem`, eliminating manual `Sign: -1` / `Turn(180)` probe choreography in `RearCockpitDoF`, `FrontCockpitDoF`, and `FitPostSetback`.
3. **Delegation over type-switching and duplicate interfaces:** export minimal 2–3 method polymorphic part-family interfaces from `internal/parts` (`parts.Seatpost`, `parts.Fork` with physical `SagStatus`, `parts.PhysicalFork`), deleting the duplicate interface copies in `catalog` and `compare` as well as `forkMetadata`'s type-switch in `frameset.go`.
4. **Part-owned local millimeter `Sketch`es + uniform `PlacedNode` projection:** let each component describe its schematic spans and segments in its own `PartLocal` coordinate system (zero SVG/CSS concepts) so `web/diagram.go` projects `Placement.Nodes` through `NodeToWorld` without reaching into component scalar bags.
5. **Retire `Placement.Setup` (`setup_snapshot.go` / `BikeSetup`) and delete this plan on completion:** replace the 70-field `BikeSetup` shadow struct with a compact `FittedSettings` readout carrying only the scalar build settings needed for lossless `/bike` URLs and CLI JSON, and delete `docs/implementation_plan.md` in Stage 5.

---

## Target Package Architecture

```mermaid
flowchart TD
    Pose["internal/pose\nPoint, Vector, Heading, Pose, Transform, CoordinateSystem"]
    Bounds["internal/bounds\nLoadState, InputField/InputError, BoundedRange, BoundStatus, SlideJoint"]
    Parts["internal/parts\nSealed Frame, Fork, Seatpost, Saddle, Stem, Handlebar, Ports, Local Sketch"]
    Assembly["internal/assembly\nFrameset -> SeatedFrameset -> Build, Planar2DoF, PlacedNode, Placement + FittedSettings"]
    Scoring["internal/scoring\nScoreWeights, FitOval, ScoreBuild, CompareFitRank, FitFrontBuilds"]
    Catalog["internal/catalog"]
    Compare["internal/compare"]
    Query["internal/query"]
    Web["internal/web"]
    CLI["cmd/bikegeo"]

    Bounds --> Pose
    Parts --> Bounds
    Parts --> Pose
    Assembly --> Parts
    Assembly --> Bounds
    Assembly --> Pose
    Scoring --> Assembly
    Scoring --> Parts
    Scoring --> Bounds
    Scoring --> Pose
    Catalog --> Scoring
    Compare --> Scoring
    Query --> Compare
    Query --> Catalog
    Web --> Query
    CLI --> Web
    CLI --> Query
```

---

## Execution Strategy (5 Atomic Stages)

Every stage below must leave the repository compiling cleanly and passing `make pre-commit` (`go fix`, `go fmt`, `go vet`, `make boundary`, `exhaustive`, `go test ./...`). Do not introduce any banned terms (`chassis`, `cad`, `price`, `pricing`, `cost`, `cheap`, `ledger`) in code, filenames, or comments.

---

### Stage 1: Extract `internal/bounds` & Self-Describing `SlideJoint`

Create `internal/bounds` (importing only `internal/pose` and standard library) as the shared foundation for load states, attributed input errors, bounded intervals, and 1-DoF linear sliding joints.

#### [NEW] `internal/bounds/` (`state.go`, `input.go`, `range.go`, `joint.go`, `bounds_test.go`)
- **`LoadState` & ambient world coordinate systems (`state.go`):**
  - Move `LoadState` (`Static`, `Dynamic`), `UnpitchedChart`, `StaticWorld`, `DynamicWorld`, `LoadState.Valid()`, `LoadState.WorldSystem()`, and `LoadStateForWorld(pose.CoordinateSystem)` from `internal/assembly/state.go`.
- **Attributed validation errors (`input.go`):**
  - Move `InputField` (all 13 constants) and `*InputError` / `NewInputError` from `internal/assembly/validate.go` so `bounds`, `parts`, `assembly`, `scoring`, and `web` share one exhaustive error-attribution enum without import cycles.
- **Validated intervals & bound status (`range.go`):**
  - Move `BoundedRange`, `NewBoundedRange`, `IntersectRanges`, and `BoundStatus` (including `MarshalJSON`/`UnmarshalJSON`, `MarginMm`, `WithinMargin`, `AtMin`, `AtMax`) from `internal/assembly/bounds.go`.
- **1-DoF translational joint (`joint.go`):**
  - Define `SlideJoint` (unexported fields, zero value invalid):
    - Constructed via `NewSlideJoint(field InputField, allowed BoundedRange, originAtZero pose.Point, stepPerMm pose.Vector, turnDeg pose.Deg, childSystem pose.CoordinateSystem) SlideJoint`.
    - Asserts `allowed.Valid()`, `originAtZero.CoordinateSystem() == stepPerMm.CoordinateSystem()`, `childSystem.Valid()`, `childSystem != originAtZero.CoordinateSystem()`, and `|stepPerMm.Norm() - 1| <= 1e-9` (unit-length displacement vector in the parent `CoordinateSystem` per `+1 mm` of parameter increase).
    - `Field() InputField`, `Allowed() BoundedRange`, `StepPerMm() pose.Vector`, `ParentSystem() pose.CoordinateSystem`, `ChildSystem() pose.CoordinateSystem`.
    - `TransformAt(val float64) pose.Transform` (places `childSystem` at `originAtZero.Add(stepPerMm.Scale(val))` rotated by `turnDeg`, for internal probe placement).
    - `Bind(val float64) (pose.Transform, BoundStatus, error)` (validates `val` via `allowed.Bind(field, val)` and returns `TransformAt(val)` + `BoundStatus`).
- **Tests (`bounds_test.go`):**
  - Migrate existing `bounds_test.go` cases and add unit tests for `SlideJoint` (`Bind` in-range/out-of-range, `TransformAt` along `(-1, 0)` vs. `(+1, 0)` vs. rotated rail directions, and deterministic panics on zero-value `SlideJoint`, non-unit `stepPerMm`, or mismatched parent/child `CoordinateSystem`s).

#### [MODIFY] Caller Updates (`internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, `internal/web`)
- Update references to `bounds.LoadState`, `bounds.Static` / `bounds.Dynamic`, `bounds.StaticWorld` / `bounds.DynamicWorld` / `bounds.UnpitchedChart`, `bounds.BoundedRange`, `bounds.BoundStatus`, `bounds.InputField`, and `*bounds.InputError` (keeping `exhaustive` switches green in `web/web.go` and `web/bike_page.go`).

---

### Stage 2: Extract Sealed `internal/parts` with `SlideJoint` Mates & Local `Sketch`es

Move all physical bicycle components and mating ports into `internal/parts` (importing only `internal/pose` and `internal/bounds`), sealing component internals away from `assembly`, `scoring`, `compare`, `query`, and `web`.

#### [NEW] `internal/parts/` (`spaces.go`, `sketch.go`, `frame.go`, `fork.go`, `seatpost.go`, `saddle.go`, `stem.go`, `handlebar.go`, `*_test.go`)
- **Local `CoordinateSystem`s (`spaces.go`):**
  - `FrameLocal`, `SteererLocal`, `LowerHeadsetLocal`, `ForkLocal`, `SeatTubeCollar`, `SeatpostLocal`, `SeatpostCradle`, `SaddleLocal`, `StemLocal`, `HandlebarLocal`.
- **Part-local millimeter visual geometry (`sketch.go`):**
  - Pure geometric primitives with zero SVG/color/pixel fields:
    - `SpanKind` enum (`SpanUpperCover`, `SpanSpacers`, `SpanSteererClamp`, `SpanStemBody`, `SpanRailClamp`, `SpanSaddleRail`) and `LocalSpan{Kind SpanKind; Axis pose.Pose; StartMm, LengthMm float64}`.
    - `SegmentKind` enum (`SegChainstay`, `SegSeatTube`, `SegSeatstay`, `SegHeadTube`, `SegLowerCup`, `SegForkSteerer`, `SegForkRake`, `SegSeatpostShaft`, `SegSeatpostSetback`, `SegSaddleProfile`, `SegHandlebarSweep`) and `LocalSegment{Kind SegmentKind; From, To pose.Point}`.
    - `Sketch{Spans []LocalSpan; Segments []LocalSegment}`.
- **Polymorphic part-family interfaces (replacing 3× duplicate caller interfaces):**
  - `parts.Fork` (3 physical methods):
    ```go
    type Fork interface {
        CheckHeadset(bore LowerBore, cup LowerCupSetup) error
        AxleInFork(state bounds.LoadState) pose.Point
        SagStatus() (travel ForkTravel, sagPct float64, ok bool)
    }
    type PhysicalFork interface {
        Fork
        Steerer() PhysicalSteerer
    }
    ```
    (`SuspensionFork.SagStatus()` returns `(f.travel, f.sagPct, true)`; `RigidFork` and `BasisFork` return `(TravelRigid, 0, false)`.)
  - `parts.Seatpost` (3 physical methods):
    ```go
    type Seatpost interface {
        TotalLengthMm() float64
        InsertionLimits() bounds.BoundedRange
        RailClamp(state bounds.LoadState) RailClampPort
    }
    ```
- **`SlideJoint` ownership inside part mates & local `Sketch` methods:**
  - `Frame` (`frame.go`): owns `LocalSketch()` (`SegChainstay`, `SegSeatTube`, `SegSeatstay`, `SegHeadTube` in `FrameLocal`), `RearAxle()`, `SeatAxis()`, `HeadTube()`, and `SeatTube()`.
  - `SeatpostDoF` & `ClampedSeatpost` (`seatpost.go`): `MateSeatpost(socket SeatTubeSocket, post Seatpost)` builds an `InsertionJoint() bounds.SlideJoint` in `SeatTubeCollar` with `stepPerMm = NewVector(SeatTubeCollar, -1, 0)`; `Clamp(insertionMm)` delegates to `InsertionJoint().Bind(insertionMm)`. `ClampedSeatpost` exposes `LocalSketch(state)` (shaft segment `[0, ExposedMm]` in `SeatTubeCollar` + setback segment to cradle, and `SpanRailClamp` in `Saddle` / clamp orientation).
  - `SaddleDoF` & `ClampedSaddle` (`saddle.go`): exposes `RailSlideJoint(railTurnFromCradle pose.Deg) bounds.SlideJoint` in `SeatpostCradle` with `stepPerMm = Along(NewHeading(SeatpostCradle, railTurnFromCradle), -1)`; `Clamp` binds that joint. `Saddle` / `ClampedSaddle` exposes `LocalSketch()` (`SpanSaddleRail` centered at `(0, 0)` along `+X` in `SaddleLocal` and `SegSaddleProfile` from `SIT` to `BRP`).
  - `SteererPort`, `Stem`, & `ClampedStem` (`stem.go`): `SteererPort` exposes `SpacersJoint() bounds.SlideJoint` in `SteererLocal` (`originAtZero = (UpperStackMm, 0)`, `stepPerMm = (+1, 0)`); `MountStem` binds `SpacersJoint()` and exposes `LocalSketch()` (`SpanUpperCover`, `SpanSpacers`, `SpanSteererClamp`, `SpanStemBody`). Also provide `StemExtensionJoint(spacersMm, clampHeightMm float64, angleDeg pose.Deg, lengthRange bounds.BoundedRange) bounds.SlideJoint` for continuous stem-length fitting.
  - `Handlebar` & `ClampedHandlebar` (`handlebar.go`): exposes `LocalSketch()` (`SegHandlebarSweep` from `(0, 0)` to `GripInBar()` in `HandlebarLocal`).
- **Tests (`internal/parts/*_test.go`):**
  - Move `seatpost_test.go`, `saddle_test.go`, `stem_test.go`, and `handlebar_test.go` into `internal/parts`, plus isolated unit tests for `Frame`, `RigidFork`/`SuspensionFork`/`BasisFork`, `CheckSHIS`, `SlideJoint` bindings, and part-local `Sketch` geometry.

#### [MODIFY] `internal/assembly`, `internal/catalog`, `internal/compare`, `internal/query`, `internal/web`
- Delete duplicate `SeatpostPart` / `Seatpost` / `PhysicalFork` interfaces from `catalog/seatposts.go`, `catalog/forks.go`, `compare/compare.go`, `assembly/seatpost.go`, and `assembly/frameset.go` in favor of `parts.Seatpost`, `parts.Fork`, and `parts.PhysicalFork`.

---

### Stage 3: Joint-Projected `Solve2DoF` & `PlacedNode` Sketch Projection

Eliminate manual `Planar2DoF` probe/sign choreography and unexported-field scraping in `internal/assembly`, and switch `internal/web/diagram.go` to uniform `PlacedNode` sketch projection.

#### [MODIFY] `internal/assembly/dof.go`, `bike.go`, `frameset.go`, `fitting.go`
- **Direct `SlideJoint` → `LinearDoF` projection (`dof.go`):**
  - Add `ProjectSlideJoint(joint bounds.SlideJoint, probeVal float64, parentToWorld pose.Transform) LinearDoF`:
    rotates `joint.StepPerMm()` through `parentToWorld.ApplyVector(...)` into the target world so `Heading` points directly along `+1 mm` of parameter increase. Remove `LinearDoF.Sign` once all three callers use `ProjectSlideJoint` (with `paramFromTravel(travel) = Range.Clamp(Probe + travel)`).
- **Simplify `RearCockpitDoF`, `FrontCockpitDoF`, and `FitPostSetback`:**
  - `RearCockpitDoF.Planar2DoF()` projects `postDoF.InsertionJoint()` (through `collarInWorld`) and `saddleDoF.RailSlideJoint(railTurn)` (through `cradleInWorld`) — zero `Sign: -1` or `Turn(180)` hand-wiring.
  - `FrontCockpitDoF.Planar2DoF()` projects `steerer.SpacersJoint()` and `parts.StemExtensionJoint(...)` through `steererInWorld`.
  - Refactor `(*Build).FitPostSetback(brp pose.Point, railOffsetMm float64) (PostSetbackFitting, error)` to use public `Frameset` / `SeatedFrameset` / `Build` methods and `ProjectSlideJoint` over `(InsertionJoint, SetbackJoint)`, returning `PostSetbackFitting{Fitting; SetbackMm float64}` with zero access to private post fields.
- **Uniform `PlacedNode` slice on `FramesetPlacement` and `Placement`:**
  - Define `PlacedNode{Transform pose.Transform; Sketch parts.Sketch}` in `internal/assembly`.
  - Populate `FramesetPlacement.Nodes` (`json:"-"`) in `Frameset.placeWithTransform` (covering frame tubes, lower cup segment, and fork steerer/rake segments) and `Placement.Nodes` (`json:"-"`) in `Build.Place` (appending seatpost, rail-clamp, saddle, steerer/stem, and handlebar nodes).
  - Expose clean accessors on `SeatedFrameset` and `Build` (`Frameset()`, `Fork()`, `Seatpost()`, `Saddle()`, `DynamicRailPitchDeg()`, `Stem()`, `Handlebar()`, `DynamicBarRollDeg()`) so external packages (`scoring`, `query`, `web`) inspect domain objects cleanly without private field access.

#### [MODIFY] `internal/web/diagram.go` & `diagram_test.go`
- Replace `stemOutlines(stem, bike.Setup.Stem)`, `seatParts` `bike.Setup.*` reads, and hardcoded segment lists in `buildBikeDiagram` with a thin consumer helper projecting `bike.Nodes` (`[]assembly.PlacedNode`), mapping `parts.SpanKind` and `parts.SegmentKind` to SVG names, thicknesses, stroke widths, colors, and close-up framing groups (`view=seat` vs. `view=stem`).
- Read `WheelRadiusMm` from `bike.Frameset.WheelRadiusMm` (added to `FramesetPlacement` or `Frameset` readout) instead of `bike.Setup.Wheels.RadiusMm`.
- Verify all existing `diagram_test.go` and `web_test.go` SVG assertions pass unchanged, and add a unit test in `diagram_test.go` projecting an isolated `parts.Saddle` / `ClampedSeatpost` `PlacedNode` without constructing a `Build`.

---

### Stage 4: Extract `internal/scoring`

Move `0–100` rider-preference scoring and discrete front-cockpit search out of `internal/assembly` into `internal/scoring`, sealing `Frameset`, `SeatedFrameset`, and `Build` against scoring/search internals.

#### [NEW] `internal/scoring/` (`scoring.go`, `build_fitting.go`, `scoring_test.go`, `build_fitting_test.go`, `build_fitting_reference_test.go`)
- Move `FitTier`, `AttributeName`, `AttributeScore`, `ScoreWeights`, `StemOption`, `BuildScore`, `FitOval`, `DefaultGripOval`, `CompareFitRank`, `ScoreBuild`, `ScoredBuild`, `CompareScoredBuilds`, `BuildRanking`, `FrontBuildOptions`, and `FitFrontBuilds` from `internal/assembly` to `internal/scoring` (importing `assembly`, `parts`, `bounds`, `pose`).
- **Delegate `ScoreBuild` to `*assembly.Build` instead of `Placement.Setup`:**
  - Change `ScoreBuild(build *assembly.Build, target assembly.FitTarget, stemScore float64, weights ScoreWeights) (BuildScore, error)` (or accept `build *assembly.Build` alongside an already-computed dynamic `assembly.Placement` if desired) so fork sag comes from `build.Fork().SagStatus()`, handlebar sweep from `build.Handlebar().Dimensions().BacksweepDeg`, spacers/rail/insertion from `build.Bounds()`, and handling geometry from `placement.Frameset` — **zero reads of `Placement.Setup`**.
- **Keep `Build` construction sealed in `FitFrontBuilds`:**
  - In `evalFrontOnSeated`, keep the fast `parts.MountStem` + `parts.MountHandlebar` inner-loop pruning, and for the `<= limit` kept builds call `seated.AttachFront(spacers, st.part, bar.part, dynamicBarRollDeg)` instead of `&assembly.Build{...}` struct literals.

#### [MODIFY] `internal/catalog`, `internal/compare`, `internal/query`, `internal/web`
- Update imports and type references (`scoring.ScoreWeights`, `scoring.StemOption`, `scoring.BuildScore`, `scoring.ScoredBuild`, `scoring.FitTier`, `scoring.AttributeScore`, `scoring.FitFrontBuilds`, `scoring.ScoreBuild`, `scoring.DefaultGripOval`).

---

### Stage 5: Retire `Placement.Setup` (`BikeSetup`), Verify `deadcode`, and Delete Plan

Delete `setup_snapshot.go` and `forkMetadata`, replace `Placement.Setup` with compact `FittedSettings`, clean up remaining callers, and delete this plan file.

#### [DELETE] `internal/assembly/setup_snapshot.go`
- Delete `BikeSetup`, `FrameSetup`, `ForkSpec`, `ForkSetup`, `SeatpostSpec`, `SeatpostSetup`, `SaddleSetup`, `StemSetup`, and `HandlebarSetup`.
- Delete `forkMetadata`, `forkSetupSnapshot`, `frameSetupSnapshot` from `internal/assembly/frameset.go` and `setupSnapshot` from `parts`/`assembly`.

#### [MODIFY] `internal/assembly/bike.go`, `internal/scoring/build_fitting.go`, `internal/query/`, `internal/web/`, `cmd/bikegeo/main_test.go`
- Add compact `FittedSettings` to `Placement.Settings`:
  ```go
  // FittedSettings records the scalar installation settings of a placed Build
  // for lossless /bike URL reconstruction and CLI JSON output.
  type FittedSettings struct {
      ForkStaticA2cMm float64
      ForkEffectiveA2cMm float64
      ForkRakeMm      float64
      ForkSagPct      float64
      ForkHasSag      bool
      LowerCupMm      float64
      WheelRadiusMm   float64
      PostInsertMm    float64
      PostExposedMm   float64
      RailOffsetMm    float64
      RailPitchDeg    pose.Deg
      SpacersMm       float64
      StemLengthMm    float64
      StemAngleDeg    pose.Deg
      BarRollDeg      pose.Deg
  }
  ```
- Update `scoring.CompareScoredBuilds` to read `Placement.Settings.{SpacersMm, StemLengthMm, StemAngleDeg}`.
- Update `query.paramsFromSettings(p.Settings)` in `query/bike.go` and `query/compare.go`.
- Update `web/web.go`, `web/bike_page.go` (`forkCellName` taking `assembly.FittedSettings`, `makePostSetbackView` reading `assembly.PostSetbackFitting`), and `web/compare_page.go` (`ChainstayMm: f.InstalledDims(sz).ChainstayMm`).
- Update `cmd/bikegeo/main_test.go` CLI JSON assertions from `Placement.Setup.Stem.*` / `Placement.Setup.Seatpost.InsertionMm` to `Placement.Settings.*`, keeping all numerical score/pose assertions identical.

#### [MODIFY] Docs & [DELETE] `docs/implementation_plan.md` (Final Step)
- Run `make pre-commit` and `make deadcode` (fixing any unreachable helper surfaced by retiring `BikeSetup`).
- Update `docs/DESIGN.md` (§7 Active Roadmap) and `docs/fit-audit/README.md` to remove the completed refactor item and mark **Fit Audit Stage 2 & Physical Fenrir Check** as the active next step.
- **Delete `docs/implementation_plan.md`** in the final commit of Stage 5.

---

## Verification Plan

After **every** stage (Stages 1 through 5):
```sh
make pre-commit
```
Which runs:
1. `go fix ./...` and `go fmt ./...`
2. `go vet ./...`
3. `make boundary` (standard-library-only binary check + banned-word checks)
4. `go tool exhaustive ./...` (all enum switches across `bounds`, `parts`, `assembly`, `scoring`, `web`)
5. `go test -v ./...`

In Stage 5 before the final commit:
```sh
make deadcode
```
Confirming zero unreachable functions in production or test packages after `setup_snapshot.go` and `forkMetadata` are removed.
