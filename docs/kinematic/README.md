# Clear bicycle geometry and assembly

## Goal and direction

Make correct geometry read clearly enough to review by local inspection. A
reviewer should see a stem being placed on a steerer, rather than reconstruct
that intention from scattered trigonometry.

Stay in Go. Use a small vocabulary of points, directions, local movement,
rotation, and named component operations. Prefer ordinary value types and
explicit functions. Types should communicate useful physical distinctions;
they do not need to make every incorrect program impossible to express.

This supersedes the earlier emphasis on hostile callers, unusable zero values,
conversion-proof stages, and comprehensive runtime invariant enforcement. The
cost of those mechanisms became disproportionate to this application's needs.
Keep useful geometry and regression evidence; simplify the scaffolding.

The independent seed documents, [Kinematic Assembly](KINEMATIC_ASSEMBLY.md) and
[Kinematic Stronger](KINEMATIC_STRONGER.md), remain sources of ideas. Their
examples were written without knowledge of bikegeo. They are not implementation
contracts. Earlier plans and review history are available in Git; this README
states the current direction.

## What is already useful

- `internal/assembly` owns stem, handlebar, and saddle placement. Its value-based
  `pose2` operations centralize local movement and rotation.
- Front-cockpit placement is simplified to `PlaceStem`, `PlaceHandlebar`, and
  `HandlebarOffset3D` with named setup structs and plain landmark results;
  `geometry.placeFrontCockpit` routes scene construction, cockpit replacement,
  and handlebar calculations through them.
- The saddle radius, rail-offset, and effective-STA solvers use forward assembly
  and verify their results against the requested target.
- Saddle placements retain their construction facts; legacy snapshot export
  takes only the placement. This removes repeated arguments from callers.
- Independent examples, transformation properties, and application tests cover
  the migrated behavior. Keep that evidence during simplification.

The front-cockpit slice is complete. Saddle placement and its solvers still
contain checked handles, getters, and repeated validation from earlier stages.
Legacy `BikeScene` and `BRPPoint` remain ordinary mutable snapshots. This is not a
whole-bike correctness proof or certification of physical hardware safety.

## Working principles

**Express the construction.** Keep trigonometry in geometric primitives and
component definitions, where the relationship needs to be understood once.
Recipes should name the movements and attachments. A small pose or turtle-like
API is useful when it makes those steps concrete; do not build a general CAD
framework or introduce a mutable global drawing cursor.

**Separate placement from solving.** Assembly answers where parts and landmarks
are for given settings. A solver finds settings for a requested measurement and
checks the assembled result. Search and scoring choose among candidates. Keep
these responsibilities explicit and preserve their existing objectives.

**Use proportionate validation.** Validate user/catalog inputs at appropriate
entry points and return errors for impossible constraints. Private arithmetic
helpers may assume validated inputs; they do not all need error returns or
repeated finiteness/orthonormality checks. Retain inexpensive final-result checks
where useful, especially solver residuals. Tests should cover realistic boundary
failures without promising detection of every intermediate invalid value.

**Preserve errors that matter.** If an operation returns an error, its callers
must not substitute successful zero geometry or silently discard a candidate.
Defaults belong in input resolution. A valid target outside the rail window is
useful diagnostic geometry; it is different from an unsolvable target. Missing
physical measurements remain unknown.

**Use tests to establish behavior.** Prioritize independent coordinates,
physical dimensions, orientation properties, and production regressions. Do not
add compile-negative fixtures, source-shape policing, or tests of every accessor's
zero-value panic merely to defend against invented misuse. Remove such tests
when retiring the machinery they enforce. Keep a type-specific test only when
it protects a concrete, worthwhile property.

## Conventions to preserve

| Area | Existing meaning |
| --- | --- |
| Coordinates | Planar, BB-relative, +X forward and +Y up; positive geometric rotations are counterclockwise |
| Units | Lengths are `float64` millimeters; angles are `model.Deg`, using the existing angle abstraction |
| Stem | Upper cover/spacers advance along the steerer; clamp center adds half clamp height; stem extends perpendicular to the steerer plus its stated angle |
| Handlebar | Preserve catalog width, rise-mode, sweep, and grip-inset conventions; roll is world-relative in the leveled bike, positive clockwise |
| Saddle | Rail pitch is world-relative, positive nose-up; positive rail station moves the saddle body rearward; post setback is perpendicular to the effective seat axis |
| Saddle targets | BRP height is radial from BB; projected clamp/rail datums are not measured hardware surface heights or insertion depth |
| Frame attitude | Apply parent attitude once; retain height-dependent effective-seat-axis selection for bent frames |
| Chart geometry | Preserve published values, chart versus declared-fork distinction, CAD baseline, and rounding budgets |
| Rider facts | Preserve [RIDER_FIT.md](../RIDER_FIT.md): modeled saddle support landmarks are not independently measured pelvis coordinates; do not change reference pitch or unsagged-transfer assumptions |

Readability refactors must preserve these meanings. A change to the physical
model needs a separate decision and independent evidence.

## Next slice: simplify saddle placement and its result plumbing

The front-cockpit simplification has been reviewed through `401dd1f`. Apply that
style to the remaining saddle stages before expanding into chassis geometry.
The opportunity is concrete: `assembly/saddle.go` still wraps a short placement
recipe in constructors, validity flags, and repeatedly checked getters.

### Scope and intended shape

Introduce a direct forward operation, conceptually:

```text
placement = PlaceSaddle(bb, effectiveSTA, SaddleSetup{
    Saddle: saddleSpec,
    AxialDistanceMm: axialDistance,
    PostSetbackMm: postSetback,
    RailOffsetMm: railOffset,
})
```

This returns `(SaddlePlacement, error)`. Settings have named fields and use the
existing `model.SaddleSpec`. The axial distance is an input already known to the
caller; finding it from a requested BRP radius remains the solver's job.

Use the existing private pose vocabulary to make this construction visible:

```text
start at BB, oriented along the effective seat axis
advance by axial distance
move perpendicular rearward/downward by post setback; record projected clamp
start a world-horizontal reference at that clamp and turn by saddle rail pitch
move backward along the rails by rail offset; record rail center
locate BRP and modeled sit-bone support from their saddle-local offsets
```

Positive rail offset moves the saddle body rearward; negative offsets are legal.
Pitch is world-relative, so it does not inherit seat-axis rotation. Post setback
is perpendicular to the seat axis, not a horizontal translation. Zero axial
distance and zero setback remain legal, including solver reference placements.

`SaddlePlacement` becomes an ordinary value containing the four landmarks
(`SeatpostClamp`, `RailCenter`, `BRP`, `SitBones`) and the construction facts
(saddle specification, axial distance, post setback, rail offset). Retain these
facts as copies so `BRPPointFromSaddlePlacement` still takes only the placement;
do not restore duplicate specification/setting arguments to snapshot export.
Callers check errors and treat a successful result as a snapshot. Mutation-proof
results and zero-value panic guarantees are no longer requirements.

### Solver integration without a solver rewrite

Keep `SolveSaddleForBRPRadius`, `SolveSaddleForBRPTarget`, and
`SolveSaddleSTAForBRPTarget` in `geometry`. Adapt their forward evaluations to
`PlaceSaddle`. Preserve equations, root/branch choices, supported domains,
parallel-axis rejection, and existing residual tolerances. Do not combine this
with a general constraint solver or a numerical-algorithm cleanup.

The solvers also need seat-axis direction, rearward rail direction, and the
clamp-to-BRP displacement. Preserve these as small named geometry queries in
`assembly`, sharing the pose/component calculations used by forward placement.
They may be standalone functions with ordinary values and boundary errors;
they must not require building staged mounts. Keep the query surface limited to
these actual consumers. Do not move component trigonometry into solvers or
estimate directions by subtracting two nearby placements.

Simplify `SaddleRailSolution` and `SaddleStaSolution` to ordinary results too:
the former carries its placement; the latter carries effective STA and its
placement. Read solved rail offset from the placement rather than storing a
second copy. Remove validity flags and getters that only guard access. Keep the
existing solver entry points and adapt consumers mechanically. A successful
solve still requires forward placement and the existing residual check before
returning; removing checked result wrappers must not remove those checks.

### Implementation boundaries

1. Replace `SeatAxis` / `SeatpostClampMount` attachment stages and the checked
   `Saddle` wrapper with direct placement and the needed geometry queries.
   Remove superseded constructors, getters, and unused helpers after migration.
   Prefer a few shared private helpers over another public builder hierarchy.
2. Validate placement inputs at its public boundary and retain useful final
   landmark checks. Do not require catalog identity, rail length, or clamp-window
   membership to locate a saddle: synthetic geometry and out-of-window solutions
   remain valid diagnostic results. Hardware feasibility stays with its existing
   consumers. Keep solver validation/residual logic intact in this slice.
3. Update `geometry/saddle_solve.go`, scene construction, snapshot export, and
   result consumers in saddle fitting/swapping and their callers. Preserve error
   propagation and candidate context all the way to reports. Never turn an error
   into successful zero geometry or silently skip a failed candidate.
4. Preserve height-dependent effective-STA resolution, reference-saddle transfer,
   search/scoring objectives, rail-window reporting, and rendering conventions.
   Do not alter catalog facts, default resolution, frame leveling, or chassis
   assembly. Leave `BikeScene`, `BRPPoint`, and unrelated result APIs as they are.

### Evidence and stopping point

Adapt the existing tests; most needed evidence already exists:

- Independent forward example: STA 60°, axial distance 100 mm, zero post setback,
  rail offset +12 mm, level rails, local BRP (+32, 0) and support (−48, +10) give
  clamp (−50, 50√3), rail center (−62, 50√3), BRP (−30, 50√3), and support
  (−110, 50√3 + 10), all in millimeters.
- Perpendicular post-setback translation, signed rail movement with a fixed
  clamp, rigid translation, and nonzero world-relative pitch across different
  seat angles. Geometry queries agree with the corresponding forward movements.
- Independent inverse examples and forward/inverse round trips for all three
  solvers, including tangent, unreachable, and nearly parallel cases. Keep
  existing residual tolerances and distinguish geometric success outside the
  hardware window from a failed solve.
- Snapshot facts and landmarks agree for synthetic and catalog saddles across
  all three solvers. Changing an input or exported snapshot does not alias another
  result; do not require public result fields themselves to be immutable.
- Representative invalid/nonfinite inputs fail through placement and real
  consuming boundaries. Keep successful baselines and end-to-end failure tests,
  plus scene, saddle-swap, ideal/compromise-STA, scoring, and report regressions.

Delete saddle compile-negative fixtures and zero-handle/getter-panic tests that
only enforce retired guarantees, including those embedded in otherwise useful
solver tests. Remove the compile-negative harness if no fixtures remain. Keep
independent numerical assertions; a forward/inverse round trip alone can hide
an error shared by both paths.

Run `go vet ./... && go test ./...` after code changes, then `make pre-commit`
and the repository runtime-dependency check. Review any modernization separately.

Stop after the production paths use the direct recipe, obsolete scaffolding is
deleted, and behavior tests pass. Review the net change across placement, queries,
solvers, and callers: fewer concepts and production lines should accompany the
clearer recipe. Do not count code merely moved elsewhere as simplification.
Chassis work and any deeper solver simplification need a separate next decision.
