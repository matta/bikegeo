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
- Saddle placement now uses `PlaceSaddle`, named settings, plain landmarks, and
  small geometry queries. The radius, rail-offset, and effective-STA solvers
  retain forward verification and return ordinary results.
- Saddle placements retain their construction facts; legacy snapshot export
  takes only the placement. This removes repeated arguments from callers.
- Independent examples, transformation properties, and application tests cover
  the migrated behavior. Keep that evidence during simplification.

Front-cockpit and saddle simplification are complete through `f7f949e`, including
`BBToClampAlongSeatAxisMm` naming and removal of the compile-negative harness.
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

## Next slice: make frame-and-fork placement a readable recipe

The next seam is the chassis construction embedded in `geometry.BuildScene`:
chart geometry, a fork change, frame leveling, and lower headset placement.
Separate that recipe from cockpit placement and fit measurements. The goal is
that a reviewer can follow the change of fork without decoding `chartL`, `dL`,
`fa0`, and several similar axle positions across the larger scene function.

### Scope and intended shape

Introduce a private `placeChassis(frame, build)` operation in `geometry`, returning
an ordinary result and an error. A small `chassis.go` is a suitable home for it
and the existing frame-leveling types/helper. Keep this in `geometry`: it combines
chart interpretation and leveling with placement, rather than defining a reusable
physical component. Reuse `geom2d` operations; no exported pose API, new checked
handles, or general frame builder is needed.

The recipe should read as these steps:

```text
evaluate the published chart and declared CAD fork separately
calculate the changes in fork-plus-lower-cup length and fork rake
move the chart front axle down the steerer by the length change
move it forward perpendicular to the steerer by the rake change
level the frame using the existing rear-axle rotation
return BB to the origin
locate the physical lower cup below the leveled head-tube bottom
```

Use descriptive names for the two signed changes, calculate them once, and use
the same values for axle placement and leveling. A short private operation such
as `frontAxleAfterForkChange` can own the two directional movements. Keep its
inputs/results ordinary and its directions explicit: increasing fork length
moves the axle down the steerer; increasing rake moves it forward perpendicular
to the steerer. Negative changes undo those movements.

Return the existing chart closure, leveled frame landmarks, pitch, lower-cup
bottom, steerer axis, and effective/structural fork lengths needed by `BuildScene`.
Group existing values rather than inventing a second scene or duplicating its
inputs and derived metrics. `BuildScene` should call this recipe, place its two
cockpits, and assemble measurements/results. Keep all existing `BikeScene`
fields and consumers compatible.

### Preserve the chart baseline explicitly

The active scene starts from **ChartFrontAxle**, not **DeclFrontAxle**. Those can
differ because published chart values and the declared CAD fork do not close
exactly. This distinction is part of the existing model, not an error for this
slice to correct.

Express the active, unpitched axle as the chart axle plus the two fork-change
movements. This is algebraically equivalent to the existing calculation from
head-tube top using `ChartL + deltaStructuralA2c`. It exposes the reason for the
baseline and avoids presenting a chart-adjusted distance as a catalog fork fact.
When fork, sag, cup, and rake match the CAD configuration, both changes are zero:
retain the exact zero-pitch path and published frame dimensions.

Do not rebuild the active axle from the declared physical fork alone, alter
manufacturer values, hide the closure residual, or move the lower cup to absorb
it. The cup bottom remains the head-tube bottom moved down the active steerer by
the physical cup stack. Both chart and declared-fork diagnostic points remain
in their original, unpitched chart coordinates.

### Boundaries

- Leave `EvaluateChartClosure`, `frontEndLength`, chart rounding budgets, and
  residual definitions unchanged. Diagnostic callers use partial hypotheses,
  including zero fork length; do not route them through stricter physical-build
  validation as part of this extraction.
- Retain the current `levelFrame2D` algorithm, rotation sign, rear-axle pivot,
  BB recentering, and zero-change shortcut. Do not replace its pitch calculation
  with one based on tiny rounded chart axle-height differences. Update its
  names/location only as needed to read with the recipe.
- Keep existing `BuildScene` input validation and propagate closure/placement
  errors. Preserve the positive-wheelbase check before leveling. No duplicate
  validation hierarchy or silent fallback geometry is needed.
- Continue leveling axle centers. Grounding unequal wheel radii would change the
  physical model and is deferred. Keep sag conventions, seat-angle adjustment,
  bent-seat-tube resolution, cockpit placement, scoring, and reports unchanged.
- Remove the superseded chassis calculation from `BuildScene`. This must improve
  the recipe itself, especially its chart-versus-declared baseline, rather than
  only moving the same abbreviated arithmetic into a helper.

### Evidence and stopping point

Use the existing catalog-wide tests as the main regression evidence:
`TestBuildScene_CADBaselineAndResiduals` and
`TestBuildScene_LevelsBothAxlesOnRakeAndCupChanges`. Preserve their tolerances and
exact CAD-baseline assertions, including zero pitch. Keep scene, analysis,
validator, fit, and rendering tests passing without refreshing expected output
just to accommodate the refactor.

Add only focused evidence where the distinction needs an independent example:

- At HTA 60°, increasing fork-plus-cup length by 20 mm and rake by 10 mm moves
  the unpitched axle by `(10 + 5√3, 5 − 10√3)` mm. Test the two contributions and
  their signs, including a decrease, independently of the implementation formula.
- A synthetic chart with a deliberately nonzero declared-fork residual still
  preserves its chart baseline when the build matches the CAD configuration.
  Changing the fork starts from that chart axle and leaves the diagnostic
  residuals unchanged. Use the existing closure behavior as the fixed reference.
- Retain cup-versus-fork-length equivalence, rigid frame distances, and equal
  leveled axle heights across rake, sag, and cup changes. These already have
  production coverage; adapt it rather than duplicating the test matrix.

Run `go vet ./... && go test ./...` after code changes, then `make pre-commit`
and the runtime-dependency check. Review any modernization separately.

Stop when `BuildScene` delegates chassis placement, the recipe makes the chart
baseline and fork movements explicit, and existing geometry/report behavior is
preserved. A small helper and result type are justified by that separation;
additional stage types or a new geometry engine are not. Review this slice before
changing chart closure, leveling mathematics, wheel grounding, or renderer geometry.
