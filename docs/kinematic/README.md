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

## Next slice: simplify front-cockpit placement

### Scope and intended shape

Replace the front-cockpit stage scaffolding with a small value-based placement
API. Work at `geometry.placeFrontCockpit` and the stem/handlebar implementation
it calls. Preserve the reusable handlebar-offset calculation used by comparisons
and required-clamp calculations.

Use named input fields instead of a long list of interchangeable scalar
arguments. Return ordinary landmark values. Illustrative orchestration:

```text
stemPlacement = PlaceStem(headTubeTop, activeHTA, stemAndStackSettings)
handGrip = PlaceHandlebar(stemPlacement.BarClamp, handlebarSettings)
```

Both operations may return errors at their validation boundaries. Exact names
are an implementation choice. Within `PlaceStem`, the recipe should expose:

```text
start at the head-tube top, oriented up the steerer
advance by upper cover and spacers
advance by half the stem clamp height; record steerer clamp center
turn toward the stem extension, including the stated stem angle
advance by stem length; record handlebar clamp center
```

The bar's placement starts a world-relative reference attitude at that clamp;
it must not inherit the stem's extension angle. Keep handlebar geometry in its
component calculation, including the lateral grip coordinate required by the
existing offset API.

### Implementation boundaries

1. Consolidate stem input validation at the placement entry point. Preserve
   supported dimensions/angles and descriptive errors for invalid requests.
   Name intermediate physical quantities rather than inline trigonometry.
2. Replace `HeadTubeTop`, `StemBase`, and `BarClampMount` transitions with the
   ordinary placement operations above. Retain a separate stem definition only
   if a real production caller benefits from it; do not preserve a type solely
   to narrate a stage. Remove superseded APIs and test fixtures.
3. Expose result landmarks directly where getters only repeat validity checks.
   Drop front-cockpit validity flags and mandatory zero-value panics. Document
   that callers must check returned errors, following normal Go conventions.
4. Reuse a compact pose vocabulary for movement and turning. Simplify the private
   pose implementation if needed to remove per-step checking from the recipe;
   use value semantics, not a shared mutable cursor. If shared pose signatures
   change, adapt saddle internals mechanically while retaining saddle inputs,
   outputs, solver checks, and public APIs. Do not simultaneously redesign rear
   cockpit assembly. Avoid adding a second permanent pose engine.
5. Route scene construction, `WithCockpit`, and
   `CalculateHBPointWithHandlebar` through the same implementation. Preserve
   error propagation from handlebar-offset helpers to their callers. A thin
   wrapper around all the old stages does not complete this slice: remove the
   machinery that no longer serves a production purpose.

This slice changes expression and structure, not geometry, catalog data,
scoring, fit selection, presentation, or the language/toolchain. Leave broader
scene immutability, additional components, and general constraint solving out.

### Evidence and stopping point

Keep or adapt these tests to the simpler API:

- Independent stem example: HTA 60°, head top (0, 0), cover 10, spacers 20,
  clamp height 40, stem length 100, angle 0° gives steerer clamp (−25, 25√3)
  and bar clamp (50√3 − 25, 50 + 25√3), all in millimeters.
- Nonzero stem-angle examples, stem-length separation, and spacer translation
  along the steerer; rigid transformation of stem placement applies once.
- Handlebar rise modes and nonzero roll; changed frame/stem attitude preserves
  the declared world-relative bar orientation.
- Invalid requests fail through a real consuming boundary; successful baselines
  precede injected errors. Include representative nonfinite inputs and verify
  successful scene landmarks are finite, without testing every arithmetic step.
- Existing scene, comparison, fit, saddle, and rendering regressions retain
  valid outputs within their current tolerances and preserve report rounding.

Delete front-cockpit compile-negative and zero-handle tests whose requirements
are intentionally retired. Do not add AST checks to enforce API spelling.

Run `go vet ./... && go test ./...` after code changes, then `make pre-commit`
and the repository runtime-dependency check. Review any modernization separately.

Stop when the recipe exposes the physical construction, obsolete stages are
removed, and the tests pass. Compare the whole affected implementation before
and after: it should have fewer concepts, less repeated validation, and fewer
production lines, not merely move complexity behind a new facade. Explain any
remaining complexity in terms of a real component convention or caller need.
Review that result before applying the style to saddle placement or the chassis.
