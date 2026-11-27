# bikegeo: Design Specification

This document is the present-tense normative specification for `bikegeo` (`cmd/bikegeo` and `internal/...`). Each component section mirrors its Go file so a part can be design-reviewed and code-reviewed in isolation. Historical decisions live in `git log`; workflow rules live in [WORKFLOW.md](WORKFLOW.md).

---

## 1. Requirements & Package Boundaries

### Core Requirements

- **Isolated, correct-by-construction components:** Each bicycle part (`Frame`, `Fork`, `Seatpost`, `Saddle`, `Stem`, `Handlebar`) defines its own local coordinate system, validates its intrinsic invariants at construction, and composes with adjacent parts through explicit mating ports and bounds.
- **Focused Web 1.0 interface:** Go-rendered HTML and schematic SVG driven by plain `GET` forms and URL query state with zero JavaScript, no cookies, and no build step. `/` is the multi-frame comparison matrix; `/bike` is the single-bike inspector with three fixed diagram views (`view=bike|seat|stem`). Open separate browser tabs to compare builds visually.
- **Validated Go catalogs & thin JSON CLI:** Compiled Go structs in `internal/catalog` validated by `go test` (chart consistency, fork compatibility, seatpost bounds), plus thin JSON CLI output (`bikegeo compare [query]`, `bikegeo bike [query]`) via `encoding/json` over internal result structs. No file-based catalog persistence, no MCP server, and no chart-solver UI.
- **Fitting, `0–100` weighted scoring, and multi-frame comparison:** Bounded 2-DoF continuous fitting and discrete build fitting over the stem catalog (`docs/stems.yaml`) and part options, ranked by a shared 4-tier fit gate and weighted `0–100` attribute scores.

### Layout & Dependency Rules

```text
cmd/bikegeo/          entrypoint: `serve` on :8080, plus URL-query JSON CLI (`compare`, `bike`)
internal/pose/        concrete 2D affine primitives, fluent turtle poses, rigid transforms, runtime CoordinateSystems
internal/bounds/      LoadState, InputField/InputError, BoundedRange, BoundStatus, 1-DoF SlideJoint
internal/parts/       sealed Frame, Fork, Seatpost, Saddle, Stem, Handlebar, typed ports, part-local Sketches
internal/assembly/    Frameset → SeatedFrameset → Build, joint-projected Planar2DoF solver, PlacedNode & Placement
internal/scoring/     0–100 weighted BuildScore, FitOval, 4-tier comparator, discrete FitFrontBuilds
internal/catalog/     18 frames, 16 forks, 15 bars, 4 seatposts, 107 stems, SQlab 610, default targets & weights
internal/compare/     multi-frame expansion, fitting, ranking, and pruning
internal/query/       shared url.Values parsing and catalog resolution for CLI and web (no net/http or templates)
internal/web/         pure Web 1.0 handlers, HTML/SVG views, PlacedNode SVG projection, embedded templates (no JS)
```

- Dependencies form a strict DAG:
  - `cmd → {web, query, compare}`
  - `web → {query, compare, catalog, scoring, assembly, parts, bounds, pose}`
  - `query → {compare, catalog, scoring, assembly, parts, bounds, pose}`
  - `compare → {scoring, assembly, parts, bounds, pose}`
  - `catalog → {scoring, assembly, parts, bounds, pose}`
  - `scoring → {assembly, parts, bounds, pose}`
  - `assembly → {parts, bounds, pose}`
  - `parts → {bounds, pose}`
  - `bounds → {pose}`
- Neither `parts`, `assembly`, `scoring`, nor `compare` imports `catalog`; callers pass catalog data explicitly so domain packages unit-test with small fixtures.
- `cmd/bikegeo` depends only on `bikegeo` packages and the Go standard library (`make boundary`).
- `make boundary` bans `chassis`, `cad` (use `basis`), and non-score words (`price`, `pricing`, `cost`, `cheap`, `ledger`) in any case under `cmd/` and `internal/`.

### Go Idioms & Encapsulation Policy

- **Package-level sealing:** Go's privacy boundary is the package, not the struct. Separating `bounds`, `parts`, `assembly`, and `scoring` ensures outer layers cannot scrape private component or build fields or construct domain structs outside their validating constructors. Even within a package, files collaborate through constructors and methods rather than cross-file private field access.
- **Consumer-side thin interfaces + polymorphic part families:**
  - Single-implementation roles (`Frame`, `Saddle`, `Stem`, `Handlebar`, `Frameset`, `SeatedFrameset`, `Build`) export concrete structs with zero producer interfaces; orthogonal consumers (such as `web/diagram.go`) define 1–3 method interfaces at the point of use.
  - Polymorphic part families (`RigidPost` / `LinkagePost`; `RigidFork` / `SuspensionFork` / `BasisFork`) export one minimal 2–3 method physical interface from `internal/parts` (`parts.Seatpost`, `parts.Fork`, `parts.PhysicalFork`) so `catalog`, `compare`, and `assembly` hold and pass polymorphic part values without duplicating interface definitions across packages.
- **Delegation over type-switching & shadow bags:** `assembly`, `scoring`, and `web` never type-switch on concrete part types or reconstruct raw `BikeSetup` property bags. Adjustable joints carry their own local unit movement vectors (`bounds.SlideJoint`), parts expose their own local millimeter `Sketch` geometry in `PartLocal`, and readout structs (`Placement`, `FittedSettings`, `BoundStatus`, `BuildScore`, `Fitting`) use **exported fields** restricted to placed poses, bound statuses, and compact query settings.

---

## 2. Coordinate Systems, Affine Primitives, and `pose` Invariants

All trigonometry lives unexported inside `internal/pose`. Lengths are `float64` millimeters; scalar angles are `pose.Deg`.

### World & Part Coordinate Systems (`pose.CoordinateSystem`)

`pose.CoordinateSystem` is an opaque `uint16` identifier created via `pose.NewCoordinateSystem(name)` that names **both** a coordinate system's datum origin `(0, 0)` and its `(+X, +Y)` axes (`+Y` is `90°` counter-clockwise from `+X`). The zero `CoordinateSystem` is invalid, so any uninitialized zero-value geometric struct fails immediately on first use.

- **Ambient world coordinate systems** (shared datum `BB = (0, 0)`, `+X` horizontally forward, `+Y` vertically up, positive turns CCW):
  - `UnpitchedChart`: the chart reference attitude (`0°` frame pitch, `+X` along reach, `+Y` along stack).
  - `StaticWorld`: level-ground workshop attitude with no rider and all suspension at `0%` sag (`RearAxle.Y == FrontAxle.Y`).
  - `DynamicWorld`: level-ground riding attitude with the rider seated and suspension at seated sag (`RearAxle.Y == FrontAxle.Y`).
- **Component local coordinate systems** (invariant to the part; mapped into parent/world coordinate systems by rigid `pose.Transform`):
  - `FrameLocal`, `LowerHeadsetLocal`, `ForkLocal`, `SeatTubeCollar`, `SeatpostLocal`, `SeatpostCradle`, `SaddleLocal`, `SteererLocal`, `StemLocal`, `HandlebarLocal`.

### Primitives (`internal/pose`)

- **Affine separation (`Point` vs. `Vector`):** `Point` (position in a `CoordinateSystem`) and `Vector` (displacement in a `CoordinateSystem`) are distinct concrete structs. Subtracting two `Point`s in the same `CoordinateSystem` yields a `Vector`; adding a `Vector` to a `Point` in the same `CoordinateSystem` yields a `Point`; adding two `Point`s is unrepresentable; `Norm()` and `Dot(other)` exist only on `Vector`. `Point.MarshalJSON` emits `{"X": ..., "Y": ...}`.
- **`Heading`:** absolute angle in degrees within one `CoordinateSystem` (`0°` along `+X`, `90°` along `+Y`). `Turn(delta Deg)` rotates CCW; `Sub(other Heading)` asserts matching `CoordinateSystem` and returns `Deg`.
- **Fluent turtle `Pose`:** immutable `(Origin Point, Heading)` in one `CoordinateSystem` with dot-chained `Advance(distMm)`, `TranslateLocal(dx, dy)`, `Turn(delta Deg)`, and `Face(heading)` (asserting matching `CoordinateSystem`). `Anchor(childSystem CoordinateSystem)` returns the `Transform` mapping `childSystem` into `p.CoordinateSystem()`. `Pose.MarshalJSON` emits `{"Origin": ..., "HeadingDeg": ...}`.
- **Rigid `Transform`:** maps `Src CoordinateSystem → Dst CoordinateSystem` via rotation + translation. `ApplyPoint`, `ApplyVector`, and `ApplyPose` assert `input.CoordinateSystem() == t.Src()`; `Compose(ab, bc)` asserts `ab.Dst() == bc.Src()` and returns `ab.Src() → bc.Dst()`; `Inverse()` returns `t.Dst() → t.Src()`.
- **Deterministic runtime checks:** any `CoordinateSystem` mismatch or uninitialized zero `CoordinateSystem` panics immediately with both coordinate system names, catching both world-state mix-ups and origin mix-ups (`RailClamp` vs. `SaddleLocal`) deterministically in unit tests without generic type-parameter boilerplate.

---

## 3. Component & Mate Specifications

Each physical component lives in its own file under `internal/parts`, owns its local coordinate system and millimeter `Sketch` (`LocalSpan` and `LocalSegment` primitives with zero SVG/CSS knowledge), validates its own dimensions, and connects to neighbors exclusively through typed ports and `bounds.SlideJoint`s.

### Shared Bounds, Load States, and Sliding Joints (`internal/bounds`)

- `LoadState` is a closed enum (`Static`, `Dynamic`), checked by `exhaustive`, paired with the ambient world `CoordinateSystem`s (`UnpitchedChart`, `StaticWorld`, `DynamicWorld`).
- `InputField` and `*InputError` attribute validation failures and hardware bound clamps to a canonical physical input across `bounds`, `parts`, `assembly`, `scoring`, `query`, and `web`.
- `BoundedRange` holds a validated `[Min, Max]` interval with provenance labels (`MinSource`, `MaxSource`), supporting `IntersectRanges(a, b)` (taking the tighter bound and preserving its source), `Midpoint()`, `Clamp(v)`, and `Bind(field, value) (BoundStatus, error)`.
- `BoundStatus` is an exported readout struct (`Field`, `Value`, `Min`, `Max`, `MinSource`, `MaxSource`) with `MarginMm()`, `WithinMargin(mm)`, `AtMin(tol)`, and `AtMax(tol)`, serving fitting limits, margin scoring, UI notes, and workshop proximity warnings from one source of truth.
- `SlideJoint` encapsulates a bounded 1-DoF translational joint placing a child `CoordinateSystem` in a parent `CoordinateSystem` with fixed relative rotation `turnDeg` and unit-length movement vector `stepPerMm` (displacement of child origin per `+1 mm` of parameter increase), binding scalar values into `(pose.Transform, BoundStatus, error)` and projecting directly into world `LinearDoF` axes without external sign conventions.

### Frame & Headset (`frame.go`, `fork.go`)

- **Local coordinate system (`FrameLocal`):** origin `(0, 0)` at `BB`, `+X` along chart reach, `+Y` along chart stack.
- **Parameters (`FrameDimensions` → `NewFrame`):** `ReachMm > 0`, `StackMm > 0`, `HeadTubeAngleDeg ∈ (0°, 180°)`, `HeadTubeLengthMm > 0`, `SeatTubeAngleDeg ∈ (0°, 180°)` (straight effective STA for all catalog frames), `SeatTubeLengthMm > 0` (C-T; Seven frames include a `+35 mm` illustrative C-C → C-T collar offset), `BbDropMm`, `ChainstayMm > |BbDropMm|`, `LowerBore`, physical `DefaultLowerCup`, optional `MaxPostInsertionMm` (`0` = undeclared), optional `MinFrameInsertionMm` (`0` = none).
- **Sliding dropouts & Tuning Chips:** sliding-dropout frames evaluate at `MaxChainstayMm` on `/` and `/bike` (flagged `*`) and at published `ChainstayMm` for chart consistency and `fork=basis`. The Otso Fenrir Stainless uses its nominal middle Tuning Chip geometry (`68°` HTA, `75°` STA, `70 mm` BB drop, `440 mm` chainstay) with `MaxChainstayMm = 0`.
- **Exposed ports in `FrameLocal`:**
  - `RearAxle()`: `Point` at `(-sqrt(ChainstayMm² − BbDropMm²), BbDropMm)`.
  - `HeadTube() HeadTubePort`: sealed port constructed by `Frame` carrying `SteererInFrame` (`SteererLocal → FrameLocal` at `HeadTubeTop` `(ReachMm, StackMm)` pointing up the steerer, `180° − HTA`), `LowerHeadsetInFrame` (`LowerHeadsetLocal → FrameLocal` at `HeadTubeBottom`, `HeadTubeLengthMm` down the steerer pointing along `-HTA`), `LowerBore`, and `DefaultLowerCup`. Distinct `SteererLocal` and `LowerHeadsetLocal` `CoordinateSystem`s ensure passing the bottom mount where an upper steerer transform is expected panics deterministically.
  - `SeatTube() SeatTubeSocket`: `CollarInFrame` (`SeatTubeCollar → FrameLocal` at `SeatTubeTop`, `SeatTubeLengthMm` along `180° − STA`), and `AllowedInsertion` (`[MinFrameInsertionMm, frameCap]`, where `frameCap` is `MaxPostInsertionMm` when `> 0`, else `SeatTubeLengthMm − 20 mm` for BB-shell clearance).
- **SHIS Lower-Headset Compatibility (single canonical rule in `fork.go`):**

| Lower Cup Standard | Typical Stack (Cane Creek 40) | Required `LowerBore` | Accepted Steerers |
| --- | ---: | --- | --- |
| `EC34` | `12 mm` (`> 0`) | `Bore34Pressed` | `Straight 1-1/8"` |
| `EC44` | `12 mm` (`> 0`) | `Bore44Pressed` | `Straight 1-1/8"` (`EC44/30`), `Tapered` (`EC44/40`) |
| `ZS44` | `4 mm` (`≥ 0`) | `Bore44Pressed` | `Straight 1-1/8"` (`ZS44/30`) only |
| `ZS56` | `4 mm` (`≥ 0`) | `Bore56Pressed` | `Straight 1-1/8"`, `Tapered` |
| `IS42` | `1 mm` (`≥ 0`) | `Bore42Integrated` | `Straight 1-1/8"` only |
| `IS52` | `1 mm` (`≥ 0`) | `Bore52Integrated` | `Straight 1-1/8"`, `Tapered` |
| `LowerCupBareHeadTube` | `0 mm` (exact) | Any recognized bore | `BasisFork` only |

- **Upper headset & steerer limit:** shared `DefaultUpperStackMm = 11 mm` (illustrative) above `HeadTubeTop` and conservative `DefaultMaxSpacersMm = 30 mm` spacer cap above the upper cover.
- **Cup resolution in `catalog`:** when pairing a compatible physical fork with a frame, keep the frame's `DefaultLowerCup.Standard` whenever it accepts the steerer (so a straight steerer in a `Bore44Pressed` frame keeps `ZS44` if the frame defaults to `ZS44` and `EC44` otherwise; a tapered steerer always takes `EC44`).

### Fork (`internal/parts/fork.go`)

- **Local coordinate system (`ForkLocal`):** origin `(0, 0)` at the crown race seat (mating `LowerCupBottom`), `+X` down the steerer axis, `+Y` forward perpendicular to the steerer (rake direction, `90°` CCW from steerer-down).
- **Polymorphic fork family** (`parts.Fork` exports `CheckHeadset`, `AxleInFork(LoadState)`, and physical `SagStatus() (ForkTravel, float64, bool)`; `parts.PhysicalFork` embeds `Fork` and adds `Steerer() PhysicalSteerer`):
  - `RigidFork`: physical steerer (`Straight 1-1/8"` or `Tapered`), `StaticA2cMm > 0`, finite `RakeMm`; `AxleInFork(_)` is `(StaticA2cMm, RakeMm)` in both states; `SagStatus()` returns `(TravelRigid, 0, false)`.
  - `SuspensionFork`: physical steerer, `StaticA2cMm > 0`, closed `ForkTravel`, `SagPct ∈ [0, 100]` with dynamic `A2C > 0`; `AxleInFork(Static)` is `(StaticA2cMm, RakeMm)`, `AxleInFork(Dynamic)` is `(StaticA2cMm − Travel·SagPct/100, RakeMm)`, and `SagStatus()` returns `(Travel, SagPct, true)`.
  - `BasisFork`: chart-basis hypothesis (no physical steerer, no travel); `CheckHeadset` accepts `LowerCupBareHeadTube` or any physical cup fitting `LowerBore`; `AxleInFork(_)` is `(BasisA2cMm, RakeMm)`; `SagStatus()` returns `(TravelRigid, 0, false)`.
- **Closed travel & compatibility table:**

| Travel | Recommended Sag | Current Compare Sweep | `/bike` Default Sag | Seated Class Sag (Milestone 6) | Stand-In Rigid Class Unlocked |
| --- | --- | --- | --- | --- | --- |
| `Travel100` (`100 mm`) | `15–20%` | `15%, 20%` | `20%` | `15%` | `RigidClass485` |
| `Travel120` (`120 mm`) | `15–25%` | `15%, 20%, 25%` | `25%` | `20%` | `RigidClass495` |
| `Travel130` (`130 mm`) | `20–25%` | `20%, 25%` | `25%` | `25%` | None |

- **Open fork compatibility (`catalog`):** Each fork model owns its published `(StaticA2cMm, RakeMm)` variants; a model is compatible in all its variants or none. A frame's listed forks are a minimum set: a listed rigid fork unlocks only its own `RigidClass` (`440`, `470`, `485`, `495`); a listed suspension fork unlocks its `ForkTravel` row and any stand-in rigid class above (one-way: rigid never unlocks suspension). Every compatible fork must also fit the frame's `LowerBore` in a valid lower cup. Multi-variant query IDs format as `<Fork.ID>-<StaticA2cMm>x<RakeMm>`.

### Seatpost (`seatpost.go`)

- **Local coordinate systems:**
  - `SeatpostLocal`: origin `(0, 0)` at the bottom tip of the shaft, `+X` upward along the shaft centerline, `+Y` perpendicular rearward (`90°` CCW).
  - `SeatpostCradle`: origin `(0, 0)` at the rail-clamp pivot (the transverse axis where saddle rails sit and tilt), `+X` parallel to the shaft centerline.
- **Intrinsic invariants:** `0 < MinInsertionMm ≤ MaxInsertionMm < TotalLengthMm` (guaranteeing positive head/linkage stack height `TotalLengthMm − MaxInsertionMm`), `SetbackMm ≥ 0`, `RailClampLengthMm > 0`.
- **Concrete seatpost types** (satisfying `parts.Seatpost{TotalLengthMm, InsertionLimits, RailClamp}`):
  - `RigidPost`: constant cradle at `NewPoint(SeatpostLocal, TotalLengthMm, SetbackMm)` at `0°` relative rotation in both `Static` and `Dynamic`.
  - `LinkagePost`: static cradle at `NewPoint(SeatpostLocal, TotalLengthMm, StaticSetbackMm)`, dynamic cradle shifted by `sagStep Vector` in `SeatpostLocal` at `0°` relative rotation (keeping rail pitch constant relative to the shaft through travel). Catalog dimensions, shim/illustrative assumptions, and the Endurance-manual-inferred 20% sag vector for the 350 mm Redshift ShockStop PRO Endurance (whose rider-seated sag remains unmeasured) live in [seatpost_redshift_shockstop_pro.md](../internal/catalog/seatpost_redshift_shockstop_pro.md).
- **Mate (`MateSeatpost(socket SeatTubeSocket, post postMounter)`):**
  - Intersects `post.InsertionLimits()` with `socket.AllowedInsertion`; rejects incompatible pairs immediately.
  - Binding `InsertionMm` places shaft coordinate `X = InsertionMm` at `(0, 0)` of `SeatTubeCollar`, so `SeatpostLocal` sits at `(-InsertionMm, 0)` in `SeatTubeCollar` (`ExposedMm = TotalLengthMm − InsertionMm` follows automatically) and records `Insertion BoundStatus`.

### Saddle (`saddle.go`)

- **Local coordinate system (`SaddleLocal`):** origin `(0, 0)` at `RailCenter` (midpoint of the straight clamping span), `+X` forward along the rails toward the nose, `+Y` perpendicular upward (`90°` CCW).
- **Intrinsic geometry (`Saddle`):**
  - `BRP` (top surface on centerline where saddle is 70 mm wide, W70): `NewPoint(SaddleLocal, RailCenterToBrpMm, RailToBrpHeightMm)`.
  - `SIT` (saddle support landmark, currently `SitBones` in code/UI until Milestone 6 rename): `NewPoint(SaddleLocal, RailCenterToSitBonesMm, RailToBrpHeightMm + SitBonesElevationMm)` (`SitBonesElevationMm` is height above BRP).
  - SQlab 610 Ergolux Active 2.1: `RailCenterToBrpMm = +32 mm`, `RailToBrpHeightMm = +45 mm` (illustrative), `RailCenterToSitBonesMm = −48 mm`, `SitBonesElevationMm = +10 mm`, `RailLengthMm = 60 mm` (measured). Milestone 6 rail-position scoring uses `+5°` dynamic rail angle (nose flat horizontal).
- **Mate (`MateSaddle(clamp RailClampPort, saddle Saddle)`):**
  - Rejects `clamp.ClampLengthMm > saddle.RailLengthMm()`.
  - Legal `RailOffsetMm ∈ [−limit, +limit]` where `limit = (RailLengthMm − ClampLengthMm)/2`. Positive `RailOffsetMm` slides the saddle rearward (`RailCenter` sits at `−RailOffsetMm` along the rail heading from `RailClamp`).
  - Stores `railTurnFromCradle` (physical clamp bolt angle relative to `SeatpostCradle`) so static and dynamic placements share the exact same clamped saddle orientation. Helper `RailTurnForWorldPitch` solves `railTurnFromCradle` from a desired world rail pitch during dynamic fitting.

### Stem & Handlebar (`stem.go`, `handlebar.go`)

- **Steerer mate (`SteererPort` + `MountStem`):**
  - `SteererPort` carries a sealed `SteererInFrame` transform (`SteererLocal → FrameLocal`, constructed by `Frame`/`Frameset` with `(0, 0)` at `HeadTubeTop` and `+X` pointing up the steerer), `UpperStackMm` (`11 mm`), and `AllowedSpacers` (`[0, 30] mm`).
  - `Stem` holds pure physical dimensions (`ClampHeightMm > 0`, `LengthMm > 0`, finite `AngleDeg` CCW from perpendicular to the steerer) with no solver search bounds.
  - `MountStem` binds `SpacersMm` into `BoundStatus`, walks in `SteererLocal` along `+X` by `UpperStackMm` → `CoverTop`, `SpacersMm` → `SpacerTop`, `ClampHeightMm/2` → `SteererClamp`, turns `−90° + AngleDeg`, advances `LengthMm` → `BarClampPort`, and maps into `FrameLocal` via `SteererInFrame`.
- **Handlebar (`HandlebarLocal`):**
  - Origin `(0, 0)` at `BarClamp` center, `+X` forward at `0°` roll, `+Y` upward along rise/upsweep.
  - `Handlebar` separates 3D bar geometry (`StockWidthMm`, `CurrentWidthMm`, `GripInsetMm`, `BendStartMm`, `StatedRiseMm`, `RiseMode` [`standard_knuckle` or `bar_end_center`], `BacksweepDeg`, `UpsweepDeg`) from its analytical 2D sagittal projection `GripInBar() pose.Point` (at `HandLateralMm = effectiveWidth/2 − GripInsetMm`, with `sweptSpan = max(0, HandLateralMm − BendStartMm)`).
  - `MountHandlebar` locks physical `rollTurnFromStem` relative to `Stem` (computed via `BarTurnForWorldRoll` when orienting the bar in `DynamicWorld`, where positive `BarRollDeg` rolls the top clockwise = `−BarRollDeg` CCW).

---

## 4. Incremental Assembly & Load States

Assembly progresses monotonically through immutable, unforgeable domain objects:

1. **`Frameset` (`MateFrameset`):**
   - Validates SHIS `(LowerBore, LowerCup, fork)` and positive `WheelSetup.RadiusMm` (`372 mm` catalog default, equal front and rear radii; tire compression ignored).
   - Computes `rearAxle` and `frontAxle(state)` in `FrameLocal`.
   - **Single-pass placement:**
     - `Unpitched()` maps `FrameLocal → UnpitchedChart` at `0°` pitch (used by catalog chart-consistency tests).
     - `Place(state)` (`Static` or `Dynamic`) computes `pitch = −rearAxle.AngleTo(frontAxle(state)).Deg()`, builds `frameToWorld` (`FrameLocal → StaticWorld` or `DynamicWorld`) in one pass, transforms all frame/headset/fork landmarks, and drops plumb-down (`−90°`) tire contacts one wheel radius below each leveled axle.
2. **`RearCockpitDoF` → `SeatedFrameset` (`OpenRearCockpit`):**
   - Mates `SeatTubeSocket`, `postMounter`, and `Saddle` immediately, checking insertion and rail bounds upfront.
   - Clamping via `.Clamp(insertionMm, railOffsetMm)` or solving via `.FitToBRP(brpTarget)` locks the rear cockpit and returns `*SeatedFrameset`.
3. **`Build` (`(*SeatedFrameset).AttachFront`):**
   - Mates `(spacersMm, Stem, Handlebar)` at `SteererPort` and locks physical `rollTurnFromStem`, returning `*Build`.
4. **Unified Multi-State Evaluation (`(*Build).Place(state LoadState) Placement`):**
   - Placing a `*Build` in `Static` and `Dynamic` runs the **exact same `pose.Compose` chain** using `Frameset.Place(state)` and `ClampedSeatpost.Place(frameToWorld, state)`.
   - Clamped saddle and handlebar orientations move rigidly with the post head and frame pitch between states without re-leveling to gravity.
   - Returns an exported-field `Placement` readout snapshot (`State`, `Settings`, `Frameset`, `Seatpost`, `Saddle`, `Stem`, `Handlebar`, `Bounds`, `Summary`, plus `Nodes []PlacedNode` carrying `json:"-"` for uniform SVG `Sketch` projection) with derived readout methods (`SeatAxisToPostTopMm`, `SaddleHeightMm`, `BrpToGripDistMm`, `SitBonesToGripRiseMm`). `Settings` (`FittedSettings`) holds only the compact scalar build settings needed for lossless `/bike` URL round-tripping and CLI JSON output, with zero duplicate `BikeSetup` spec bags.

---

## 5. Degrees of Freedom, Fitting, and `0–100` Scoring

### Explicit 2-DoF Affine Solver (`internal/assembly`)

Continuous fitting (`FitToBRP`, `OpenFrontCockpit`, and `FitPostSetback`) projects pairs of `bounds.SlideJoint`s through their parent-to-world `pose.Transform`s into a standalone `Planar2DoF` value (origin point plus two bounded `LinearDoF` movement headings in `DynamicWorld`) solved via `SolvePlanar2DoF`:

- **Reachable target:** exact 2×2 decomposition (`pose.DecomposeAlong`) within bounds (`fitBoundTolMm = 1e-9 mm`).
- **`PriorityNearest` (cockpit & setback):** minimizes Euclidean miss over the feasible boundary.
- **`PriorityBRPGate` (saddle):** prefers any legal point passing the BRP fit gate (`|ΔY| ≤ 2.0 mm` and `|ΔX| ≤ 10.0 mm`) first; within the same pass/fail class, minimizes `(|ΔY|, |ΔX|)` lexicographically (height first, fore-aft second).
- Every fitted result is verified by forward `Build.Place(Dynamic)`.
- **Saddle-first rule:** saddle fit is primary; build fitting fits the saddle once per fork configuration and holds it fixed while evaluating front-cockpit options.

### Discrete Build Fitting & `0–100` Weighted Scoring (`internal/scoring`)

- **Stem catalog (`docs/stems.yaml` → `internal/catalog/stems.go`):** 107 concrete `(length, angle)` stems across `40–120 mm`, each carrying a composite `0–100` score from four sub-attributes: length availability (35), angle availability (30), length room in `40–100 mm` (20), and angle room in the length's mainstream span (15).
- **HandGrip Fit Oval:** centered on the HandGrip target, semi-minor `10 mm`, semi-major `20 mm`, major axis tilted `30°` clockwise from vertical (`60°` world heading). Normalized radius `R ≤ 1.0` passes the grip gate.
- **12 Scored Attributes (total weight `36`; `23` when `brp=off` on `/bike` drops the 4 saddle attributes):**
  - Overall score (`0–100`) = `Σ(weight × score) / Σ(active weight)`.
  - Fit-gate attributes use 3-breakpoint piecewise linear ramps (`100` ideal → gate-edge score → `0` worst); non-fit range attributes use single linear ramps; rigid and basis forks score `100` on `Fork sag`.

| Attribute | Fit Gate | Weight | `100` (Ideal) | Gate Edge | `0` (Worst) |
| --- | --- | ---: | --- | --- | --- |
| HandGrip (`R`) | `R ≤ 1.0` | 6 | `R = 0` | `90` at `R = 1.0` | `R ≥ 2.0` |
| Saddle BRP Y | `|ΔY| ≤ 2.0 mm` | 6 | `|ΔY| ≤ 0.5 mm` | `50` at `2.0 mm` | `|ΔY| ≥ 4.0 mm` |
| Saddle BRP X | `|ΔX| ≤ 10.0 mm` | 4 | `|ΔX| ≤ 1.0 mm` | `50` at `10.0 mm` | `|ΔX| ≥ 20.0 mm` |
| Stem (composite) | No | 5 | Mainstream `70–80 mm` (`100`) | — | Extreme (`0`) |
| Handlebar sweep | No | 3 | `16°–20°` | — | `≤ 10°` or `≥ 26°` |
| Fork sag | No | 3 | Travel `maxSag` (rigid/basis = `100`) | Travel `minSag` = `90` | `10%` past `[min, max]` |
| Spacers | No | 2 | `0–20 mm` | — | `30 mm` |
| Saddle rail offset | No | 2 | `−5` to `+5 mm` | — | `±15 mm` |
| RC / WB ratio | No | 2 | `37–39%` | — | `≤ 34%` or `≥ 42%` |
| Seatpost insertion | No | 1 | Margin `≥ 30 mm` from both limits | — | `0 mm` margin |
| Leveled BB drop | No | 1 | `65–75 mm` | — | `≤ 45 mm` or `≥ 95 mm` |
| Trail | No | 1 | `98–106 mm` | — | `≤ 78 mm` or `≥ 126 mm` |

- **Unified 4-Tier Ranking Comparator (`CompareFitRank`):**
  1. `FitsAll` (passes BRP + Grip): descending `Total` → ascending `GripR` → ascending `|ΔY|` → ascending `|ΔX|`.
  2. `FitsBRP` (passes BRP, misses Grip): ascending `GripR` → descending `Total` → ascending `|ΔY|` → ascending `|ΔX|`.
  3. `FitsGrip` (passes Grip, misses BRP; also the only passing tier when `brp=off` on `/bike`): descending `Total` → ascending `|ΔY|` → ascending `|ΔX|` → ascending `GripR`.
  4. `FitsNeither`: ascending `GripR` → descending `Total` → ascending `|ΔY|` → ascending `|ΔX|`.
- **Deterministic tie-breaks:**
  - Within a bike (cockpit choice): fewer spacers → shorter stem → smaller `|stem angle|` (`+` over `−`) → handlebar order → fork order.
  - Across rows (`compare`): `FrameID` ascending → declared size order → declared fork order → smaller `|sag − 20%|` → lower sag.

---

## 6. Web, CLI JSON, and Catalog Validation

### Web Surfaces (`internal/web`, `internal/query`)

- **Shared strict query parsing (`internal/query`):** parses `url.Values` for both CLI and HTTP with zero `net/http` dependency, rejecting any unrecognized query key with a field-attributed `*query.InputError`.
- **Defaults:** Otso Fenrir Stainless L, `redshift_shockstop_pro` (carrying the mixed-state caveat note until Milestone 6 separates static and dynamic states), `persuader20_800`, and idmatch targets `BRP (−219, 713.1)` / `HandGrip (481, 726)`.
- **Cleared targets:** `brp=off` / `grip=off` (or empty submitted pairs) persist across links on `/bike`; `/` compare requires both targets and returns `400` with a Reset Defaults link if either is cleared.
- **`/` Compare Workbench:** ranks every size, compatible fork, and sag of all 18 catalog frames (`top_per_frame` default `1`, `top` default `0`/unlimited), presenting three rank-aligned one-row-per-bike tables (Fit & Winning Build, Leveled Geometry & Handling, and collapsed 12-Attribute Breakdown) with lossless `/bike?...` inspect links (`strconv.FormatFloat(v, 'f', -1, 64)`).
- **`/bike` Single-Bike Inspector:** unified workbench toolbar (`prev_*` manual-switch clamping on frame/size/fork/post changes while direct invalid URLs return `400` with echoed values and explicit `Invalid: ...` `<option>` markers), 2-column hero pairing the schematic SVG (`view=bike|seat|stem`, target crosshairs, tolerance zones, miss vectors, curated per-view labels with collision avoidance) with structured geometry readouts, submitted-build score for catalog stems, continuous fitting, setback notes, and ranked builds. `fork=basis` locks chart A2C, basis cup, and `0%` sag read-only.

### Catalog Validation (`internal/catalog`)

- **Provenance:** every catalog value is labelled `published`, `catalog`, `measured`, `owner-reported`, or `illustrative`. Each newly ingested or updated frame keeps a source record at `internal/catalog/frame_<id>.md`.
- **Chart consistency (`chart_consistency_test.go`):** runs `Frameset.Unpitched()` on every frame size's published chart with its `BasisFork` and `BasisLowerCup`, nudges each rounded chart column by `±ChartPrecision`, sums per-column rounding terms, and verifies axle-height and wheelbase (when `PublishedWheelbaseMm > 0`) mismatches stay within rounding budget plus any documented `ChartAllowance` (strictly necessary, mandatory `Reason`, within `0.5 mm` of excess).
- **Basis search (`basis_search_test.go`):** committed helper and regression tests used as the template for temporary ingestion tests per the `ingest-frame` skill.
- **Fork-list & seatpost checks (`frames_test.go`, `parts_test.go`):** verify every frame size mates with every compatible fork/sag and every catalog seatpost without error.

---

## 7. Active Roadmap

1. **Package-Sealed Delegation & `BikeSetup` Retirement ([implementation_plan.md](implementation_plan.md), Stages 1–5):**
   - Runs ahead of the paused Fenrir check, preserving all physical math, catalog invariants, and SVG visual outputs while sealing invariant layers across four domain packages, delegating joint kinematics and part sketches, retiring `Placement.Setup` (`BikeSetup`) in favor of compact `FittedSettings`, and deleting `implementation_plan.md` upon completion.
   - Stage 1: Extract `internal/bounds` (`LoadState`, `InputField`/`InputError`, `BoundedRange`/`BoundStatus`, and 1-DoF `SlideJoint`).
   - Stage 2: Extract sealed `internal/parts` (`Frame`, `Fork` with `SagStatus`, `Seatpost`, `Saddle`, `Stem`, `Handlebar`, `SlideJoint` mates, and part-local millimeter `Sketch`es).
   - Stage 3: Delegate 2-DoF joint projection (`ProjectSlideJoint`) in `internal/assembly` and `PlacedNode` `Sketch` projection in `internal/web/diagram.go`.
   - Stage 4: Extract `internal/scoring` (`ScoreBuild` delegating directly to `*assembly.Build` and `FitFrontBuilds` constructing kept builds via `AttachFront`).
   - Stage 5: Retire `Placement.Setup` (`setup_snapshot.go` and `forkMetadata`) in favor of `FittedSettings`, verify `make deadcode`, update roadmap links, and delete `docs/implementation_plan.md`.
2. **Fit Audit Stage 2 & Physical Fenrir Check ([docs/fit-audit/README.md](fit-audit/README.md), paused until after item 1):**
   - Verify forward placement kinematics and reconcile Matt's physical static Otso Fenrir measurements ([fenrir-measurements.md](fit-audit/fenrir-measurements.md), currently frozen) against the matching installed component and static placements — accounting for the installed FAIR Drop Best rear assembly, matching grip inset station (`80 mm` workshop mark vs. `70 mm` catalog default), and converting straight crown-to-axle tape readings to steerer-parallel fork A2C rather than comparing directly against stock catalog defaults.
3. **Milestone 6: Static/Dynamic Model & Rider Fit Updates (gated on Stage 2 Fenrir check):**
   - Switch catalog `redshift_shockstop_pro` to its `7 mm` static setback and Endurance-manual-inferred `(-6, +5) mm` 20% sag step (see provenance and unmeasured-sag status in [seatpost_redshift_shockstop_pro.md](../internal/catalog/seatpost_redshift_shockstop_pro.md)).
   - Seated fork sag per travel class (`15%` / `20%` / `25%`), single rider-chosen handlebar at `0°` dynamic roll, seatpost option fitting, and `+5°` SQlab 610 rail angle for rail-position scoring.
   - Saddle-to-cockpit coupling (`coupledHbTarget = BRP_dynamic + (HB_target − BRP_target)`).
   - Static-to-dynamic target entry on `/bike` → `/`, static workshop readouts alongside dynamic fit results, workshop proximity warnings (`≤ 30 mm` insertion margin, `≤ 5 mm` usable rail slide), and `SIT` / `BRP.x` terminology updates.
   - Fit Audit Stages 4–6 (solver proof, ranking/sensitivity audit, end-to-end surface audit).

### Deferred Follow-Ups

- Re-ingest the Sour Pasta Party from Sour's own chart (existing wheelbases may be adapted rather than published).
- Radial BB-to-saddle distance (`|BRP − BB|`) fitting when rails clamp.
- Bent seat tube offsets (`revel-el-jefe`, `fairlight-holt-2`).
- Published seatpost insertion depths for frames beyond the Otso Fenrir Stainless.
- Physical upper headset standards and fork-specific steerer spacer limits.
- Additional saddle models (Berthoud Aspin Open, Selle Italia SLR Boost) with sit-bone transfer.
- Vendored single-file HTMX if plain `GET` forms feel clunky in daily use.
- Further compare performance optimizations if benchmarks show a need.
