# Method for auditing bikegeo fit

Status — methodology awaiting Matt’s review. The audit has not started. The code revision at the time of this proposal is `b961e8818121`.

The goal of this audit is to earn trust in `bikegeo`’s geometry, fitting, and ranking before using it to specify a custom frame. Every stage separates three questions:

1. **Physical reality and evidence:** What physical state does the model represent, how is it measured on a real bike, and what assumptions bridge missing measurements?
2. **Mathematical consequence:** If those physical premises hold, what must follow geometrically, and what objective does the solver or ranking promise when a target cannot be reached exactly?
3. **Implementation and executable proof:** Does the Go code deliver that result across its full domain, including boundary, clamped, and degenerate cases, backed by permanent invariant or grid-oracle tests?

## What trust requires

Before the audit is complete, `bikegeo` must satisfy five concrete criteria:

- **Unambiguous load states and orientations:** Moving the *same installed bike* between unloaded (workstand) and rider-loaded (fork sag, suspension post sag) states must rotate and translate clamped components rigidly with the frame and post rather than silently counter-rotating world-relative angles (`RailPitchDeg`, `BarRollDeg`) or mixing unloaded targets with sagged part dimensions.
- **Physical tape-measure round-trip:** Given a real reference bike (Matt’s current setup) and a candidate build on `/bike`, every landmark coordinate can be reconciled with a physical measurement on an unloaded bike with a tape measure and level.
- **Saddle-to-cockpit coupling clarity:** When a frame’s seat tube or rail limit prevents reaching the target BRP in `X` (even within the `±10 mm` BRP gate), the audit must explicitly settle whether `HandGrip` is targeted relative to `BB` or adjusted to preserve the rider’s `BRP → HandGrip` reach.
- **Verified solver and fast-path optimality:** For unreachable targets, `fitBox` must match a dense brute-force grid oracle over the legal 2D adjustment box `[lo_a, hi_a] × [lo_b, hi_b]`. In `FitBuilds`, the `forkScoreBase.score` pruning fast path must agree identically with full `ScoreBuild` across all candidate combinations and tier boundaries.
- **Target sensitivity visibility:** We must know how sensitive frame rankings and tier classifications (`FitsAll` vs. `FitsGrip`) are to `±2–5 mm` perturbations in `BRP` and `HandGrip` (the scale of unmeasured suspension sag and measurement uncertainty), exposing cliff edges rather than hiding them behind a single point score.

## First session: definitions and load states

Take one rigid bike, one saddle, one grip, and one setup. Describe what someone can measure, in which axes, and what stays fixed when one adjustment changes. Then describe the *same installed bike* with a suspension fork or post in two load states (unloaded on a stand vs. rider-loaded at sag). Use the contrast to find quantities whose current definition or axis ownership is ambiguous. Discuss meanings with Matt in plain English before settling names for code.

Use these prompts for that first discussion:

| Question | Example distinction |
| --- | --- |
| What kind of quantity is it? | A landmark is a point. Motion between two positions is a displacement. A pose supplies an origin and oriented axes. |
| What owns the quantity, and in which axes is it expressed? | A rail adjustment follows the rails. A linkage displacement belongs to the post. Saddle pitch and bar roll are clamped to the post/stem once installed, even if initially leveled in world axes. |
| What can change it? | Choosing a stem selects a part. Setting insertion adjusts an installed part. Loading the bike changes suspension state with the same installation. A change of coordinates alone cannot move a physical part. |
| How do we know its value? | Manufacturer dimension, rider measurement under stated conditions, inferred parameter, or calculated result. A target also needs the load state and position the rider wants to reproduce. |
| What decision consumes it? | A hardware limit constrains feasibility. A fit tolerance classifies an error. A score weight expresses a preference. |

## Work in dependency order

Keep one audit note per stage (plus focused deep-dives like [`brp-transfer.md`](brp-transfer.md) where a physical transfer problem warrants its own record), tracked in a simple table in this README once work starts:

| Stage | Note | Questions to settle |
| --- | --- | --- |
| 1. Definitions and coordinates | `01-definitions-and-states.md` | Can two readers reconstruct the same physical state from the description? How do unloaded and sagged states of the same installation relate? How can someone verify a placed build with a tape measure? |
| 2. Forward placement | `02-forward-placement.md` | Given parts, adjustments, and a load state, where do the wheels, `BB`, `BRP`, `SitBones`, and `HandGrip` go? Do frame pitch (fork sag) and post travel carry clamped parts rigidly? |
| 3. Measurement and transfer | `03-measurement-and-transfer.md` | Under what conditions does a source measurement (such as Matt’s unsagged Redshift idmatch transfer) determine the desired seated position on another build? Which missing inputs allow multiple answers? |
| 4. Inverse fitting | `04-inverse-fitting.md` | For fixed parts and load state, which targets are reachable within hardware bounds? When unreachable, does `fitBox` provably match a brute-force grid oracle over the legal 2D box and gate boundaries? Should `HandGrip` fitting remain strictly `BB`-relative when `BRP` clamps short in `X`? |
| 5. Search, scoring, and sensitivity | `05-search-and-ranking.md` | Does `FitBuilds` pruning (`forkScoreBase.score`) ever discard or misorder a build compared to `ScoreBuild`? Do the 4-tier comparator and 12-attribute weights match Matt’s actual priorities, and how stable are rankings under `±2–5 mm` target perturbations? |
| 6. End-to-end surface | `06-end-to-end-surface.md` | Do the web form, URL query, JSON CLI, SVG diagram, and score readouts refer to the exact same landmarks, sign conventions (`target − placed`), and load state? |

## Structure of each stage note and test policy

Each stage note should be concise and self-contained:

1. **Scope and premises:** State the physical inputs, coordinate axes, load state, fixed vs. varied quantities, and upstream stages relied upon.
2. **Physical and mathematical check:** Derive what must hold in plain text or inline math. Call out any physical assumption that relies on an unmeasured value or domain preference.
3. **Code audit and permanent tests:** Pin the relevant Go symbols and trace the call path.
   - Use temporary `scratch_*_test.go` tests only for one-off numerical exploration or sensitivity sweeps; record the exact inputs and outputs in the note before deleting the scratch file.
   - Whenever a claim establishes a mathematical invariant, boundary behavior, fast-path equivalence, or grid-oracle property of `pose`, `assembly`, or `compare`, **promote it into a committed `_test.go` test** so `make check` guards it permanently.
4. **Adversarial review and verdict:** Before marking a stage complete, challenge its claims (using an independent pass or second agent) by actively looking for a counterexample—especially at clamp limits, gate boundaries (`|ΔY| = 2.0 mm`, `|ΔX| = 10.0 mm`, `GripR = 1.0`), extreme angles, and sag transitions. Record confirmed defects, accepted design rulings (to carry into [`DESIGN.md`](../DESIGN.md) and the slice workflow in [`WORKFLOW.md`](../WORKFLOW.md)), and any open physical questions for Matt.

## Parked discussion

The [BRP and HandGrip transfer note](brp-transfer.md) preserves the motivating case, earlier numerical observations, and questions for Stage 3. The first session starts with Stage 1 definitions.
