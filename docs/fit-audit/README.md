# Method for auditing bikegeo fit

Status — methodology awaiting Matt’s review. The audit has not started. The code revision at the time of this proposal is `b961e8818121`.

The goal of this audit is to earn trust in `bikegeo`’s geometry, fitting, and ranking before using it to specify a custom frame. Every stage separates three questions:

1. **Physical reality and evidence:** What physical state does the model represent, how is it measured on a real bike, and what assumptions bridge missing measurements?
2. **Mathematical consequence:** If those physical premises hold, what must follow geometrically, and what objective does the solver or ranking promise when a target cannot be reached exactly?
3. **Implementation and tests:** Does the argument cover the code’s supported domain, including clamped and degenerate cases? Which permanent tests would catch a regression?

## What trust requires

The audit should give Matt a verdict on each of the following before he relies on a build to specify a frame. A stage may conclude with a supported defect or a missing physical premise. Matt decides whether to investigate further, plan a fix, or move on.

- **Unambiguous load states and orientations:** Moving the *same installed bike* between unloaded (workstand) and rider-loaded (fork sag, suspension post sag) states must rotate and translate clamped components rigidly with the frame and post rather than silently counter-rotating world-relative angles (`RailPitchDeg`, `BarRollDeg`) or mixing unloaded targets with sagged part dimensions.
- **Physical measurement:** For each observable landmark, describe how to reproduce its unloaded BB-relative coordinates with a tape measure and level. Specify the BB datum, axes, and how the bike is supported and leveled. A workstand alone leaves frame pitch undetermined. For a derived or inaccessible landmark, identify the observable inputs, the inference, and its uncertainty.
- **Saddle-to-cockpit coupling clarity:** When a frame’s seat tube or rail limit prevents reaching the target BRP in `X` (even within the `±10 mm` BRP gate), the audit must explicitly settle whether `HandGrip` is targeted relative to `BB` or adjusted to preserve the rider’s `BRP → HandGrip` reach.
- **Solver and ranking correctness.** State the objective of each `fitBox` caller and justify its result over the continuous feasible region, including boundaries and any unbounded adjustment. Use a grid to look for a legal point that beats the solver under that objective. A better sample is a counterexample, subject to numerical tolerance. A finite grid can miss an optimum between samples. For `FitBuilds`, check the fields used by the ranking comparator and compare the ordered top builds with an exhaustive `ScoreBuild` ranking of the same candidates, including ties at the cutoff.
- **Target sensitivity.** Use ±2–5 mm as an exploratory range for varying BRP and HandGrip coordinates individually. Also vary a physical input such as fork sag and let placement determine how both landmarks move together. Report which perturbations change a tier or ranking. Any estimate of measurement uncertainty needs its own evidence.

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
| 1. Definitions and coordinates | `01-definitions-and-states.md` | Can two readers reconstruct the same bike attitude, load state, and measurement axes? Which landmarks can they measure directly, and which must they infer? |
| 2. Forward placement | `02-forward-placement.md` | Given parts, adjustments, and a load state, where do the wheels, `BB`, `BRP`, `SitBones`, and `HandGrip` go? Do frame pitch (fork sag) and post travel carry clamped parts rigidly? |
| 3. Measurement and transfer | `03-measurement-and-transfer.md` | Under what conditions does a source measurement (such as Matt’s unsagged Redshift idmatch transfer) determine the desired seated position on another build? Which missing inputs allow multiple answers? |
| 4. Inverse fitting | `04-inverse-fitting.md` | For fixed parts and load state, which targets are reachable? When a target is unreachable, why does the chosen solution satisfy the caller’s objective across the legal range? Should HandGrip fitting remain BB-relative when BRP clamps short in X? |
| 5. Search, scoring, and sensitivity | `05-search-and-ranking.md` | Does pruning preserve the ordered top builds? Do the tiers and weights match Matt’s priorities? Where do tiers and rankings change in the coordinate and sag sweeps? |
| 6. End-to-end surface | `06-end-to-end-surface.md` | Do the form, URL, CLI, diagram and scores describe the same landmarks and load state? If placed BRP is 3 mm forward of target, does compare report “3 mm forward” while the raw X miss (`target − placed`) is −3 mm? |

## Structure of each stage note and test policy

Each stage note should be concise and self-contained:

1. **Scope and premises:** State the physical inputs, coordinate axes, load state, fixed vs. varied quantities, and upstream stages relied upon.
2. **Physical and mathematical check:** Derive what must hold in plain text or inline math. Call out any physical assumption that relies on an unmeasured value or domain preference.
3. **Code audit and permanent tests:** Pin the relevant Go symbols and trace the call path.
   - Use temporary `scratch_*_test.go` tests only for one-off numerical exploration or sensitivity sweeps; record the exact inputs and outputs in the note before deleting the scratch file.
   - Whenever a claim establishes a mathematical invariant, boundary behavior, ranking equivalence, or solver property of `pose`, `assembly`, or `compare`, **cover it with a committed `_test.go` test** so `make check` guards it permanently.
4. **Adversarial review and verdict:** Before marking a stage complete, challenge its claims (using an independent pass or second agent) by actively looking for a counterexample—especially at clamp limits, gate boundaries (`|ΔY| = 2.0 mm`, `|ΔX| = 10.0 mm`, `GripR = 1.0`), extreme angles, and sag transitions. Record confirmed defects, accepted design rulings (to carry into [`DESIGN.md`](../DESIGN.md) and the slice workflow in [`WORKFLOW.md`](../WORKFLOW.md)), and any open physical questions for Matt.

## Session handoff

Matt chooses the next question and passes it to the next agent. End each session with three short entries in the stage note, even if an argument is unfinished:

- **Conclusion so far.** State what follows, under which assumptions, and where the argument stops.
- **Evidence.** Link the derivation, measurement, test, or counterexample. For an implementation claim, include the code revision and symbol examined.
- **Next question.** Write one question Matt can paste into another chat. For example, “An exact fit survives identical fork rotation on both bikes. Can an imperfect fit cross the vertical BRP gate under that same rotation?”

When Matt requests a review, append a dated response to the same note identifying the step the reviewer verified or challenged and why.

## Parked discussion

The [BRP and HandGrip transfer note](brp-transfer.md) preserves the motivating case, earlier numerical observations, and questions for Stage 3. The first session starts with Stage 1 definitions.
