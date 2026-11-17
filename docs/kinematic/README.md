# Locally checkable bicycle assembly

Status: the first production slice is implemented and reviewed on
`codex/checked-bicycle-assembly`. Commit `e25d28e` introduced checked stem
assembly; `3622cc9` closed stage-conversion and mutable-identity loopholes.
The next steps are to simplify stem completion, then evaluate handlebar attachment.

Read this before the seed documents, [Kinematic Assembly](KINEMATIC_ASSEMBLY.md)
and [Kinematic Stronger](KINEMATIC_STRONGER.md). They were written independently
of bikegeo, with no knowledge of its implementation or data conventions. They
supply the motivation and candidate techniques, rather than a proposed patch to
the existing application. Their example implementations are not verified
libraries or specifications to copy literally. This document selects a smaller
next step and identifies the guarantees that implementation must establish.

## Decision

Pursue a small trusted geometry kernel, explicit bicycle assembly recipes, and
immutable solved results. Make physical intent visible at the call site. The
upper headset, spacer stack, and stem placement now use this approach in
production. Continue the experiment after simplifying that first slice, then
test component composition through handlebar attachment. Preserve the existing
mathematical model and outputs while changing how correctness is enforced.

Do not begin with a whole-program rewrite or a general 3D CAD framework. First
demonstrate that one real production path is easier to audit, rejects its stated
misuses, and needs fewer independent geometric decisions from its callers.

## What the guarantee means

The attainable contract is:

> Assuming the trusted primitives and component definitions implement their
> stated contracts, safe Go callers outside that boundary cannot successfully
> construct a result violating the enumerated assembly invariants. Violations
> are rejected by types, checked errors, or immediate invariant panics.

This is a conditional engineering guarantee, not a proof that every possible bug
in the program will fail loudly. A valid 60 mm stem supplied when someone meant
80 mm is indistinguishable from an intentional choice. A wrong but internally
consistent physical formula also needs an independent oracle to expose it.
Tests provide evidence for the trusted implementation; they do not prove every
possible execution correct. Unexecuted invalid paths do not produce runtime
failures until exercised.

For each migrated operation, record its inputs, coordinate convention,
preconditions, postconditions, failure behavior, and independent test evidence.
“Correct by construction” applies only to those recorded properties. It does
not imply that incomplete catalog data certifies a physically safe bicycle.

### Failure policy

| Situation | Required behavior |
| --- | --- |
| Incompatible attachment role or assembly stage | Unavailable operation or compile error |
| User/catalog value outside the modeled domain, including NaN or infinity | Descriptive error at the validation boundary |
| Zero or forged internal handle, wrong owner, invalid operation after completion | Panic when the invalid value is used |
| Finite inputs overflow during a geometric operation | Reject before returning a successful result; error at a fallible boundary, invariant panic inside the trusted kernel |
| Missing required part | Completion unavailable or completion error |
| Missing information needed for a physical compatibility check | Explicitly unavailable check, or error when that check is required |
| Valid model with wrong source facts or wrong user intent | Not detectable by this architecture alone |

Go lets a caller ignore an error. Failed constructors must therefore return an
unusable result, whose operations reject use, rather than a plausible origin or
empty “solved” bike. Panics are programmer failures, not routine user-input
validation. Never recover a panic and substitute a successful zero result.

## What already exists

The current code is partly aligned with this direction:

| Existing seam | Preserve or improve |
| --- | --- |
| `geometry.BuildScene` in [scene.go](../../internal/geometry/scene.go) | Pure construction from explicit frame, build, cockpit, seat, and fit inputs; already validates many inputs |
| `placeFrontCockpit` in the same file | Shared placement used by scene construction, `WithCockpit`, and `CalculateHBPointWithHandlebar`; best first replacement seam |
| `geom2d` in [geom2d.go](../../internal/geom2d/geom2d.go) | Useful point/vector separation, but exported fields permit invalid axes and arbitrary coordinate mixing; zero-vector normalization silently returns zero |
| `BikeScene` | Useful common source of landmarks, but its exported inputs and outputs can be changed independently after construction |
| `EvaluateChartClosure` and the CAD baseline tests | Explicit separation of chart-implied and declared-fork geometry; preserve this distinction and the rounding budgets |
| [bike_diagram.go](../../internal/web/bike_diagram.go) and [ledger.go](../../internal/web/ledger.go) | Already consume scene landmarks, but presentation still reconstructs some geometry explanations and intermediate quantities |

Avoid creating a second permanent geometry engine alongside these paths. Migrate
at an existing seam, route its callers through it, and remove the superseded
calculation. During migration, exported legacy snapshots remain outside the new
guarantee; adapting a checked result into `BikeScene` does not make all of
`BikeScene` immutable or validated.

## Seed design review and integration considerations

The first six findings concern the seed examples' own guarantees, not bugs in
the current production implementation. The final item identifies integration
differences expected of an independently developed model; those differences are
not defects in the seeds.

1. **At most once is not completeness.** `socketState.consumed` catches a second
   attachment, but `Solve` only sets `asm.frozen = true`; it never checks the
   socket registry for missing required attachments. Also, grips and saddle
   contact points are measurements, not necessarily mounts awaiting another part.
   Represent required mounts and observation landmarks separately.
2. **Local versus world does not identify the owning part.** `LocalPlug` and
   `LocalSocket` carry no builder identity. `FinishPart1` can combine a plug from
   one builder, a socket from another, and primitives from a third. Public
   `PromoteSocket` can mint a world mount from an arbitrary pose without attaching
   the owning part. Keep promotion internal and tie local geometry to its owner.
3. **A frozen pointer receiver is still copyable.** A caller can copy `*Builder`
   into another Go value; its `frozen` flag then has independent storage. Either
   use immutable construction or keep lifecycle state in shared private storage
   behind every handle. A pointer-returning constructor alone does not prevent
   copying. Any mutable assembly design must also define concurrency behavior.
4. **Attachment is not atomic as written.** `Attach` consumes the parent and
   appends primitives before executing an arbitrary promotion callback. A panic
   can leave partial state. Prefer immutable results; otherwise prepare and
   validate before committing, or poison the assembly so it cannot continue.
5. **Pitch is applied twice.** `AssembleChassis` gives `bb` the frame rotation
   `pitch`, then constructs head/seat tube local rotations using angles already
   adjusted by `pitch`. In the head-tube example the composed Y rotation is
   `pitch + (HTA + pitch − 90°)`, instead of `pitch + (HTA − 90°)`.
   For HTA 70° and nose-down pitch 5°, that produces an 80° tube instead of 75°.
   Orthonormality checks accept both. This needs an independent geometry test.
6. **Finiteness must include outputs.** Finite lengths can overflow when added.
   `TranslateLocal` constructs a result without checking the sum; `TransformPoint`
   returns that origin directly. A later check is insufficient for an operation
   promising an immediately valid result. NaN also defeats inequalities such as
   `abs(error) > tolerance` unless finiteness is checked separately.
7. **Independent models need explicit reconciliation.** The seed handlebar's riser segment
   length and bend recipe are not the current catalog's stated-rise conventions.
   Its world origin/axes also differ from existing outputs. Neither change is a
   behavior-preserving refactor. Similarly, effective A2C versus wheel radius
   alone cannot establish crown/tire clearance without a model of those shapes.

Retain the seeds' useful ideas: one placement operation owns downstream geometry;
callers cannot expose a child's placed mounts prematurely; attachment roles are
distinct; derived state is recomputed rather than patched. Generic roles are one
possible implementation technique, not themselves the correctness argument.

## Intended boundaries

### 1. Input resolution and evidence

Parse requests and resolve catalog choices into explicit inputs. Apply defaults
here, once. Preserve source values, missing values, and modeled assumptions as
different things. Never change a manufacturer chart number to close an equation.

Existing lengths remain `float64` millimeters and angles remain `model.Deg`, with
trigonometry through the existing angle abstraction. Do not introduce the seed's
second `MM`/`Deg`/`Rad` system as part of this migration. A scalar type alias alone
would not prevent swapping two lengths with different physical meanings.

### 2. Trusted mathematical primitives

Own finite arithmetic, directions, point/vector operations, rigid placement, and
the specific intersections/constraints actually needed. Keep invalid internal
representations inaccessible and reject unusable zero values. Check operation
results, including arithmetic overflow. State numerical tolerances in the
appropriate units; chart uncertainty is separate from numerical tolerance.

Do not expose raw matrices or arbitrary local-to-world promotion to assembly
callers. Introduce additional primitives only when a real component needs them.
The initial planar implementation keeps +X forward, +Y up, BB-relative coordinates
and counterclockwise positive rotations. Any later 3D convention requires named,
tested conversions at the boundary, including rotation signs.

### 3. Trusted component definitions and assembly recipes

A component definition says where its mounts and landmarks lie relative to its
own datum. Its small local geometric construction is reviewed against the
physical meaning of its parameters. A recipe states which components connect.

Use distinct stages or role types so a stem requires the mount produced after
the upper stack. Account for a legitimate zero-height stack explicitly. Every
offset has one owner: upper stack owns cover/spacer advance, stem owns its half
clamp height and extension. Parent attitude is inherited exactly once.

For a fixed bicycle recipe, prefer immutable staged values over an open mutable
graph with a socket registry. Reusing an immutable head-tube value to evaluate
two stems means two candidate builds, which the optimizer needs. It must not
mean two stems installed in one completed build. If later work truly needs a
mutable graph, require shared owner/consumption state, required-port completion,
and synchronized mutation (or enforceable rejection of concurrent use).

Completeness is relative to the recipe: a completed stem placement does not
certify a completed bicycle. Whole-bike completion will require each mandatory
subassembly. Optional equipment must be explicit; observation landmarks are not
unconsumed assembly obligations.

### 4. Solved results and consumers

Solved values have private state and read-only accessors, with no aliased mutable
slices/maps. Changing inputs constructs a new result. Measurements and drawing
geometry derive from the same placement. Renderers may project, scale, and format
those results; they should not independently locate physical parts.

Fit search generates candidate inputs and evaluates solved results. Scoring
keeps its separate points-lost contract. Report builders keep their document
contract. The eventual whole-program goal is a few trusted kernels appropriate
to each domain, not expressing parsing, scoring, or HTML as tube geometry.

## First implementation: upper stack and stem placement

The following contract describes the migrated production path. Its recipe has
been revised to reflect the agreed simplification, which is still pending.

Scope: replace the stem-placement portion of `placeFrontCockpit`, ending at the
handlebar clamp center. Keep the existing handlebar offset calculation after it.
Do not change frame leveling, chart closure, saddle modeling, fit selection,
catalog facts, or rendering conventions in this step.

The implementation should read like this sequence (conceptual API, not a
committed Go signature):

```text
headTube = checked head-tube top in the active bicycle attitude
stemBase = headTube.WithUpperStack(upperCover, spacers)
placement = stemBase.AttachStem(stem)
```

The public boundary validates raw inputs and returns an error. The recipe uses
validated values and typed stages. The current root adapter, `NewHeadTubeTop`,
accepts a finite point and a supported angle. Its caller remains responsible for
identifying the actual head-tube top. Local reasoning begins after that choice.
A later frame assembly should produce this mount to extend the guarantee
upstream. Merely renaming a `Point2` to `HeadTubeTop` is not a provenance check.

Keep the mathematical implementation private to a narrow package boundary,
`internal/assembly`. A wrapper retaining the old unchecked formula in the caller
does not satisfy the objective. Whether stages use concrete types or generic
roles should be settled by the smallest implementation that meets the tests below.

### Package name: `assembly`

Choose the name for the package's responsibility: expressing how bicycle parts
connect and producing checked placements. `assembly` makes that responsibility
visible at its call sites and remains appropriate as the first stem recipe grows
to include the chassis, handlebar, and saddle.

`kinematic` is defensible as a description of the mathematics: composing rigid
poses is forward kinematics, even when evaluating one stationary configuration.
But it emphasizes a technique rather than the package's bicycle construction
contract. If a separate general mechanism/pose-solving package becomes useful,
`kinematics` would be the more natural noun for that responsibility.

Other candidates are `placement`, which describes the initial calculation but
understates attachment and completeness rules, and `geometry`, which is already
used for a broader collection of calculations. Neither communicates this boundary
as clearly as `assembly`. Keep low-level mathematical helpers private or in the
existing geometry primitives as appropriate; the name does not justify a new
general-purpose mathematics package. The `docs/kinematic` directory retains its
seed-document name and does not prescribe a Go package name.

### Observable contract

Inputs: finite head-tube position, active HTA in the existing supported domain,
nonnegative upper cover/spacers, positive clamp height and stem length, finite
stem angle. Output: checked steerer-clamp center and handlebar-clamp center.

The recipe must establish these properties:

- The steerer-clamp center is on the steerer axis, at upper cover + spacers +
  half the stem clamp height from the head-tube top.
- The handlebar-clamp center is one stem length from the steerer-clamp center,
  in the direction perpendicular to the steerer plus the stated stem angle.
- A parent rigid transformation carries both points and directions together.
- No successful result contains NaN or infinity or can be mutated into a
  different placement through its public API.

This scope does not verify steerer length, clamp diameters, or hardware clearance:
the current inputs do not supply all the evidence those checks need. Future
physical compatibility checks must distinguish “unknown” from “passed.”

### Acceptance evidence

| Test | What it establishes |
| --- | --- |
| External-package compile-negative examples | Wrong stage/role cannot mount a stem; explicit conversions cannot bypass or repeat the upper stack; a local offset cannot stand in for a placed mount |
| Zero-value and ignored-error misuse | Every checked result/handle rejects use before successful construction |
| Boundary table | Negative stacks, nonpositive dimensions, NaN, infinity, and overflow fail at the stated boundary; zero cover/spacers remain legal |
| Independent 60° HTA example | With head-tube top (0, 0), cover 10, spacers 20, clamp height 40, stem length 100 and angle 0°, steerer clamp is (−25, 25√3) mm and bar clamp is (50√3 − 25, 50 + 25√3) mm |
| Rigid transformation property | Rotating/translating the parent transforms the result once; catches duplicated or omitted attitude |
| Dimension properties | Clamp separation equals stem length; adding spacers translates both landmarks along the steerer by that amount |
| Existing scene/cockpit tests and catalog comparisons | All callers retain existing valid behavior within a stated tolerance; report rounding is unchanged |
| Deliberately broken implementations | Omitting the cover, omitting half-clamp height, reversing the stem angle, or applying attitude twice each causes an independent test failure |

Use a nonzero stem angle in the angle-sign test; the zero-angle worked example
cannot detect that fault. Keep compile-negative fixtures outside normal package
discovery and have the test harness check the relevant type-check failure, not
just any compiler failure. Tests should exercise the exported boundary rather
than reproduce the implementation's formula as their only oracle.

Route `BuildScene`, `WithCockpit`, and `CalculateHBPointWithHandlebar` through the
same implementation and propagate errors to their callers as necessary. Remove
the old stem-placement arithmetic after equivalence is established. Do not ship
an unused prototype; repository dead-code checks require a production consumer.

Run the repository's fast verification after every code change, then
`make pre-commit` and the runtime-dependency check before completing the slice.
Review any modernization separately. No published frame values may change.

### Progress and assessment

The first slice demonstrates stronger safety more convincingly than simpler
code. It is worthwhile to continue, with the amount of assembly scaffolding
treated as an explicit cost.

| Objective | Evidence so far |
| --- | --- |
| Physical intent is visible | `WithUpperStack` and `AttachStem` name the actual operations |
| Geometric decisions have local owners | Upper stack owns its steerer advance, stem owns its clamp midpoint and extension, attachment owns placement in bicycle coordinates |
| Invalid inputs fail explicitly | Boundary tests cover dimensions, nonfinite inputs, and overflow |
| Stages cannot be bypassed by direct conversion | External compile-negative fixtures cover both directions between head-tube and stem-base stages |
| Solved placements remain valid | Private fields and value semantics protect stored coordinates; zero-value use panics |
| Parent attitude is applied once | Worked geometry examples and rigid-transformation tests cover the stem placement |
| Production callers share the implementation | `BuildScene`, `WithCockpit`, and `CalculateHBPointWithHandlebar` use the new path; repository checks passed |

The tests include independent coordinates, dimension properties, and sensitivity
to geometric faults. A full deliberate mutation run and a new exhaustive catalog
equivalence comparison have not been recorded. Existing scene and catalog tests
provide regression coverage; the acceptance table above remains the target for
the evidence we claim.

The new package contains five public types and roughly 330 lines of production
code for a small placement calculation. Some of that is reusable mathematics
and validation. Every additional stage should enforce a distinct obligation.

The conversion bypass and mutable identity were weaknesses introduced by the
new implementation. Their regression tests improve the abstraction's safety.
The longer-term justification must come from preventing ordinary bicycle-modeling
mistakes and reducing the geometry knowledge required at call sites.

Only one component definition currently exercises the approach. Reuse across
components, meaningful downstream mounts, and shared construction evidence for
measurements and rendering still need to be demonstrated.

## Next step 1: simplify stem completion

Have `StemBase.AttachStem` return the checked immutable `StemPlacement` directly.
Remove `MountedStem` and `Finish`, and adapt callers and tests. Successful
attachment already computes both landmarks and completes the subassembly. There
is no remaining part or unresolved constraint that justifies another stage.

Retain validation, unusable zero values, immutable results, and the distinction
between head-tube and stem-base stages. Preserve the compile-negative tests for
skipping or repeating the upper stack and the regression test for independence
from mutable geometry globals. Remove tests whose only purpose is enforcing the
redundant completion stage. Run the repository verification loop and review this
simplification before extending the assembly.

## Next step 2: evaluate handlebar attachment

Use the handlebar as the second component to test whether the primitives and
assembly boundaries compose well. Begin by making the existing rise, sweep,
width, grip-inset, and roll conventions explicit in the component contract.
In particular, define the reference attitude for roll and preserve current
world-relative grip behavior when frame attitude or stem angle changes.

The placed stem should expose a meaningful handlebar mount. Attaching the bar
should produce checked hand-contact landmarks through the component definition
and placement operations. The caller should select parts and state their
connection, with coordinate conversion and orientation rules owned by those
operations. Choose any additional pose representation needed from that concrete
requirement.

Route existing cockpit placement through this composition and remove the
superseded placement calculation. Preserve catalog measurement conventions and
exercise supported rise modes, widths, nonzero roll, stem angles, and frame
attitudes against independent examples and existing behavior. Measurements and
the cockpit drawing should consume the same resulting landmarks.

Review the resulting call site against the current one. The next slice earns
its complexity if adding the bar requires less geometry knowledge from its
caller and reuses the trusted placement primitives. If callers still manage
coordinate conversions or compensate for angle conventions, revise the boundary
before expanding further. Larger whole-bike claims require this evidence.

## Decisions deferred beyond these steps

- Whether a full 3D pose kernel and tube-bending cursor improve the next real
  component. A planar first slice does not settle the eventual representation.
- How to preserve saddle reference semantics in later component definitions.
  Saddle tilt needs an explicitly declared reference attitude. Handlebar
  conventions will be addressed in the next component contract.
- How to ground unequal wheel radii and incorporate CAD rounding residuals. The
  current scene levels axle heights and preserves the published CAD baseline;
  the seed grounds loaded wheel radii directly. Reconcile that modeling choice
  explicitly before replacing the chassis, never by adjusting chart data.
- Which additional catalog facts support physical checks such as insertion,
  clamp compatibility, and clearance. Missing measurements remain missing.
- How to make the whole scene opaque and have the computation ledger and drawing
  consume construction evidence directly. That follows a successful placement
  slice rather than being a prerequisite for testing the idea.

The agreed direction is to simplify the completed stem slice and then evaluate
handlebar composition. Each step should yield a small, reviewable change and
concrete evidence for the broader architecture.
