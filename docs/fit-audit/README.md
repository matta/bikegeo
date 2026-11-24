# Method for auditing bikegeo fit

Status: methodology revised 2026-11-23; Stage 1 in progress (Q160–Q171 ruled in [`DESIGN.md`](../DESIGN.md)).

The audit has one purpose: to earn trust in `bikegeo`’s geometry, fitting, and ranking before Matt uses it to specify a custom frame. It has three outcomes:

1. **Clear nomenclature.** Every concept that fitting depends on has one plain-English meaning, one name, and one owner in [`DESIGN.md`](../DESIGN.md) and the code. A name is settled only after its meaning has been discussed and ruled.
2. **Understood correctness.** Each claim about placement, fitting, or ranking comes with its premises and the evidence behind it. Where the mathematics allows, that evidence is a proof.
3. **A physical check on Matt’s Otso Fenrir.** Matt can measure his unsagged Fenrir with a tape measure and level and compare the results with the model’s placement of the same installed bike. Agreement within the stated measurement uncertainty, or a documented reason for disagreement, validates the frame model he owns.

Every stage separates three questions:

1. **Physical reality and evidence:** What physical state does the model represent? How is it measured on a real bike? What assumptions fill in missing measurements?
2. **Mathematical consequence:** If those physical premises hold, what must follow geometrically? When a target can’t be reached exactly, what objective does the fitting or ranking promise?
3. **Implementation and tests:** Does the argument cover the code’s whole supported domain, including clamped and degenerate cases? Which committed tests guard against a regression?

## Kinds of evidence

Label each claim in a stage note with the strongest evidence it has:

| Label | Meaning |
| --- | --- |
| **Proved** | Derived in plain text from stated premises over the whole stated domain. A committed test guards the implementation of it. |
| **Tested** | A committed test checks the claim on chosen cases, but no derivation covers the whole domain. |
| **Observed** | A numerical result from a scratch test or tool run, with its exact inputs recorded. It is not guarded against regression. |
| **Measured** | A physical measurement on a real bike, with its procedure, load state, and uncertainty. |
| **Assumed** | A premise taken without evidence, such as an unmeasured value, a manufacturer figure applied to a different part, or a domain preference. |

A test is evidence for a proof, not a substitute for one. A grid of samples that contains no better point is **Observed** unless an argument rules out a better point between the samples.

## Questions the audit must answer

The audit gives Matt a verdict on each question below: *holds*, *defect found*, or *open* (a physical premise or ruling is still missing).

| # | Question | Delivered by |
| --- | --- | --- |
| T1 | **Nomenclature.** Does every quantity used in fitting have a ruled meaning and name, its type (point, displacement, pose, or scalar), its owning part and axes, and the things that can change it? Do the code, form, URL, CLI, and diagram use those names? | Stage 1 (Q160–Q171 ruled), enforced in Phase 2 & Stage 6 |
| T2 | **Load states.** During a dynamic fit attempt, `PlaceBike` sets the fork's dynamic A2C at sag, pitches the frame so the axles are level, and sets saddle tilt and bar roll in that leveled world frame. When converting a single fitted build between static and dynamic states (Q160, Q169), do clamped parts rotate and translate rigidly with the frame and post, and are static measurements kept distinct from dynamic targets and part dimensions? | Stages 1–2 (Q160, Q163, Q169 ruled) |
| T3 | **Measurement.** For each landmark that can be observed, is there a procedure to reproduce its static BB-relative coordinates with a tape measure and level, naming the BB datum, the axes, and how the bike is supported and leveled? For a derived or inaccessible landmark, what are the observable inputs, the inference, and its uncertainty? | Stage 1 (Q162 ruled), applied in the [Fenrir check](#physical-check-on-the-fenrir) |
| T4 | **Fenrir agreement.** Do Matt’s measurements of his static Fenrir agree with the model’s static placement of the same build within the stated uncertainty? | Stage 2 |
| T5 | **Saddle-to-cockpit coupling.** Suppose a seat tube or rail limit leaves placed BRP short of the target in X, even by a miss within the ±10 mm BRP X gate (Q168). Should `HandGrip` stay targeted relative to `BB`, or move to preserve the rider’s `BRP → HandGrip` reach? Set out the options and their consequences so Matt can rule as a `Q#` before Phase 2 implementation. | Stage 3 (decided before Phase 2 slices) |
| T6 | **Solver and ranking correctness.** What is the objective of each `fitBox` caller, and why does its result satisfy that objective over the continuous feasible region, including boundaries and any unbounded adjustment? For `FitBuilds`, does the ordered top list match an exhaustive `ScoreBuild` ranking of the same candidates, including ties at the cutoff? | Stages 4–5 (on post-Phase 2 code) |
| T7 | **Target sensitivity.** Which perturbations change a tier or ranking? Vary BRP and HandGrip coordinates one at a time across an exploratory ±2–5 mm. Also vary a physical input such as fork sag, and let placement move both landmarks together. Any estimate of measurement uncertainty needs its own evidence. | Stage 5 (on post-Phase 2 code) |

For T6, sample a grid of legal points and look for one that beats the solver under the caller’s objective. A sample that beats it by more than the tolerance stated in the note is a counterexample. A finite grid can miss an optimum between samples, so a clean grid alone leaves the claim **Observed**.

Fit gates referenced throughout are the `FitGate*` constants and `fitBoundTolMm` in [`fitting.go`](../../internal/assembly/fitting.go), applied in [`scoring.go`](../../internal/assembly/scoring.go): `|BRP miss Y| ≤ 2 mm`, `|BRP miss X| ≤ 10 mm`, and `GripR ≤ 1`. `GripR` is the HandGrip miss normalized by the fit oval, where 1 is the oval’s edge. A BRP miss is `target − placed` (`BuildScore.BrpMissXMm`, `BrpMissYMm`).

## Nomenclature

Follow the naming rules in [`AGENTS.md`](../../AGENTS.md). Define what something is and does in plain English, and discuss it with Matt, before proposing a name.

- **Working glossary.** The Stage 1 note keeps a glossary table of current terms and meanings, and [`DESIGN.md`](../DESIGN.md) stays the single source of truth for confirmed `Q#` rulings.
- **Renaming.** A rename of code, form, URL, or CLI follows a ruling and lands as a dedicated slice or commit under [`WORKFLOW.md`](../WORKFLOW.md), separate from behavior changes, so it is easy to review.
- Q170 settles the landmark currently called `SitBones`: use **SIT** for the physical saddle support reference and **SIT-to-grip rise** for the vertical distance to HB. See [Rider fit provenance](../RIDER_FIT.md).

## Physical check on the Fenrir

Record the protocol and results in [`fenrir-measurements.md`](fenrir-measurements.md). It has three parts:

1. **Installed build.** Before measuring, record the inputs the model needs, with the source of each: frame size, Tuning Chip position (the catalog transcribes the middle chip), fork and its static A2C and rake, headset cups, tire size and measured wheel radius, seatpost and insertion, saddle rail offset and pitch, stem length and angle, spacers, handlebar, and bar roll. Values Matt has already confirmed in `RIDER_FIT.md` stay settled, and he isn’t asked for them again.
2. **Procedure.** Specify the static state: on the floor, no rider, and fork and ShockStop at full extension. Specify how to establish level and frame pitch, the BB datum, and the axes. Then list each landmark to measure, such as wheelbase, BB height, saddle and grip landmarks, and their BB-relative X/Y, along with the expected precision of each.
3. **Comparison.** Run the same build through `bikegeo bike` (or a catalog scratch test where static post/adapter geometry isn't yet in the CLI) and put the model’s prediction next to each measurement. A difference larger than the combined measurement and chart uncertainty is a finding: a model defect, a wrong catalog value, a wrong installed input, or a measurement error.

## Execution sequence: settle physical model → implement slices → audit solver & surface

Stage 1 rulings (**Q159–Q171**) significantly change the physical and fitting model: static vs. dynamic placements of one build, fork class sag instead of sag sweeps, one fixed rider handlebar at 0° dynamic roll, seatpost options, +5° SQlab 610 rail pitch, and workshop proximity warnings.

Auditing `fitBox`, `FitBuilds`, sensitivity sweeps, or UI/CLI labels against the pre-Q160 code would verify code that is about to be replaced. Instead, the work proceeds in three phases:

### Phase 1: Settle physical model, transfer math, and Fenrir check (Stages 1–3)

Finish settling the physical geometry and domain rules before writing the new model slices:

| Stage | Note | Status | Open questions to settle |
| --- | --- | --- | --- |
| 1. Definitions, coordinates, and load states | [`01-definitions-and-states.md`](01-definitions-and-states.md) | Mostly ruled (Q160–Q171) | Static representation of the Redshift ShockStop + FAIR Drop Best adapter for the Fenrir check and static-to-dynamic transfer. |
| 2. Forward placement & Fenrir check | `02-forward-placement.md` · [`fenrir-measurements.md`](fenrir-measurements.md) | Next | Verify forward placement math, rigid rotation of clamped parts between static and dynamic states (Q169), and agreement with Matt's measured static Fenrir (T4). |
| 3. Measurement, transfer, and coupling | `03-measurement-and-transfer.md` · [`brp-transfer.md`](brp-transfer.md) | Not started | Settle static ↔ dynamic transfer equations (Redshift linkage + fork sag, Q159–Q160) and rule on **T5 (saddle-to-cockpit coupling)** when BRP clamps short in X. |

### Phase 2: Implement the ruled model slices (`WORKFLOW.md`)

Once Phase 1 rulings are complete in `DESIGN.md`, plan and implement the Go code slices under [`WORKFLOW.md`](../WORKFLOW.md) (Horizon 2 → Horizon 3):

- Rigidly clamped saddle and handlebar orientation between static and dynamic placements (Q163, Q169).
- Seated fork sag per travel class (Q161), single fixed handlebar (Q163), seatpost option fitting, and +5° SQlab 610 rail angle for rail-position scoring (Q164–Q165).
- Static-to-dynamic target conversion and workshop readout derivation on `/bike` and `/` (Q159–Q160), T5 ruling, workshop proximity warnings (Q166–Q167), and `SIT` / `BRP.x` terminology updates (Q170–Q171).

### Phase 3: Audit solver, ranking, sensitivity, and surface on the new code (Stages 4–6)

Run Stages 4–6 once against the post-Phase 2 implementation:

| Stage | Note | Status | Questions to verify on the new code |
| --- | --- | --- | --- |
| 4. Inverse fitting | `04-inverse-fitting.md` | After Phase 2 | Does `fitBox` / bounded saddle and cockpit fitting satisfy the Q168 and T5 objectives across the entire legal region, including boundaries? (T6) |
| 5. Build fitting, scoring, and sensitivity | `05-build-fitting-and-ranking.md` | After Phase 2 | Does `FitBuilds` match exhaustive `ScoreBuild` ranking? Do tiers and weights behave as intended across coordinate and sag perturbations? (T6, T7) |
| 6. End-to-end surface | `06-end-to-end-surface.md` | After Phase 2 | Do `/`, `/bike`, CLI JSON, and SVG diagrams present static workshop measurements and dynamic fit results consistently with the ruled names and sign conventions? (T1, T2) |

## Structure of each stage note and defects

Keep each stage note concise and focused on substance rather than process bookkeeping:

1. **Scope and premises.** State the physical inputs, coordinate axes, load state, and `Q#` rulings relied on.
2. **Physical and mathematical check.** Derive what must hold in plain text or inline code math, labeled with its [kind of evidence](#kinds-of-evidence), and call out any **Assumed** premise.
3. **Code check and tests.** Use temporary `scratch_*_test.go` tests for one-off numerical probes (recording exact inputs and outputs before deleting the scratch file per [`AGENTS.md`](../../AGENTS.md)). When a claim establishes a lasting invariant or catches a bug, cover it with a committed `_test.go` test.
4. **Defects vs. model changes.**
   - **Small localized bug:** fix it directly with a committed regression test and pass `make pre-commit`.
   - **Multi-commit change or physical/fitting redesign:** record the ruling as a `Q#` in `DESIGN.md` and schedule it as a slice in Phase 2 under [`WORKFLOW.md`](../WORKFLOW.md).
5. **Handoff.** End the active stage note with a brief summary of what is settled, any ruling or measurement needed from Matt, and the next question to tackle.
