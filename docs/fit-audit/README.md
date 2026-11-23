# Method for auditing bikegeo fit

Status: methodology revised 2026-11-23 from Matt’s review; the audit has not started. The code revision at the time of this proposal is `b961e8818121`. Each stage note pins its own revision.

The audit has one purpose: to earn trust in `bikegeo`’s geometry, fitting, and ranking before Matt uses it to specify a custom frame. It has three outcomes:

1. **Clear nomenclature.** Every concept that fitting depends on has one plain-English meaning, one name, and one owner in [`DESIGN.md`](../DESIGN.md) and the code. A name is settled only after its meaning has been discussed and ruled.
2. **Understood correctness.** Each claim about placement, fitting, or ranking comes with its premises and the evidence behind it. Where the mathematics allows, that evidence is a proof.
3. **A physical check on Matt’s Otso Fenrir.** Matt can measure his unsagged Fenrir with a tape measure and level and compare the results with the model’s placement of the same installed bike. Agreement within the stated measurement uncertainty, or a documented reason for disagreement, validates the frame model he owns.

Bugs are fixed as the audit finds them (see [Defects and fixes](#defects-and-fixes)).

Every stage separates three questions:

1. **Physical reality and evidence:** What physical state does the model represent? How is it measured on a real bike? What assumptions fill in missing measurements?
2. **Mathematical consequence:** If those physical premises hold, what must follow geometrically? When a target can’t be reached exactly, what objective does the fitting or ranking promise?
3. **Implementation and tests:** Does the argument cover the code’s whole supported domain, including clamped and degenerate cases? Which committed tests would catch a regression?

## Kinds of evidence

Label each claim in a stage note with the strongest evidence it has:

| Label | Meaning |
| --- | --- |
| **Proved** | Derived in plain text from stated premises over the whole stated domain, and checked by an independent reviewer. A committed test guards the implementation of it. |
| **Tested** | A committed test checks the claim on chosen cases, but no derivation covers the whole domain. |
| **Observed** | A numerical result from a scratch test or tool run, with its exact inputs recorded. It is not guarded against regression. |
| **Measured** | A physical measurement on a real bike, with its procedure, load state, and uncertainty. |
| **Assumed** | A premise taken without evidence, such as an unmeasured value, a manufacturer figure applied to a different part, or a domain preference. |

A test is evidence for a proof, not a substitute for one. A grid of samples that contains no better point is **Observed** unless an argument rules out a better point between the samples.

## Questions the audit must answer

The audit gives Matt a verdict on each question below. Possible verdicts are *holds*, *defect found* (with a link to its fix), or *open*, meaning a physical premise or ruling is still missing. Matt decides whether to investigate further, plan a fix, or move on.

| # | Question | Delivered by |
| --- | --- | --- |
| T1 | **Nomenclature.** Does every quantity used in fitting have a ruled meaning and name, its type (point, displacement, pose, or scalar), its owning part and axes, and the things that can change it? Do the code, form, URL, CLI, and diagram use those names? | Stage 1, then each stage for its own terms |
| T2 | **Load states.** When the same installed bike moves between unloaded and rider-loaded states (fork sag, suspension post sag), do clamped parts rotate and translate rigidly with the frame and post? World-relative angles such as `RailPitchDeg` and `BarRollDeg` must not counter-rotate silently, and unloaded targets must not mix with sagged part dimensions. | Stages 1–2 |
| T3 | **Measurement.** For each landmark that can be observed, is there a procedure to reproduce its unloaded BB-relative coordinates with a tape measure and level, naming the BB datum, the axes, and how the bike is supported and leveled? (A workstand alone leaves frame pitch undetermined.) For a derived or inaccessible landmark, what are the observable inputs, the inference, and its uncertainty? | Stage 1, applied in the [Fenrir check](#physical-check-on-the-fenrir) |
| T4 | **Fenrir agreement.** Do Matt’s measurements of his unsagged Fenrir agree with the model’s placement of the same build within the stated uncertainty? | Stage 2 |
| T5 | **Saddle-to-cockpit coupling.** Suppose a seat tube or rail limit leaves placed BRP short of the target in X, even by a miss within the ±10 mm BRP X gate. Should `HandGrip` stay targeted relative to `BB`, or move to preserve the rider’s `BRP → HandGrip` reach? The audit sets out the options and their consequences, and Matt rules on them as a `Q#`. | Stage 4 |
| T6 | **Solver and ranking correctness.** What is the objective of each `fitBox` caller, and why does its result satisfy that objective over the continuous feasible region, including boundaries and any unbounded adjustment? For `FitBuilds`, does the ordered top list match an exhaustive `ScoreBuild` ranking of the same candidates, including ties at the cutoff? | Stages 4–5 |
| T7 | **Target sensitivity.** Which perturbations change a tier or ranking? Vary BRP and HandGrip coordinates one at a time across an exploratory ±2–5 mm. Also vary a physical input such as fork sag, and let placement move both landmarks together. Any estimate of measurement uncertainty needs its own evidence. | Stage 5 |

For T6, sample a grid of legal points and look for one that beats the solver under the caller’s objective. A sample that beats it by more than the tolerance stated in the note is a counterexample. A finite grid can miss an optimum between samples, so a clean grid alone leaves the claim **Observed**.

Fit gates referenced throughout are the `FitGate*` constants and `fitBoundTolMm` in [`fitting.go`](../../internal/assembly/fitting.go), applied in [`scoring.go`](../../internal/assembly/scoring.go): `|BRP miss Y| ≤ 2 mm`, `|BRP miss X| ≤ 10 mm`, and `GripR ≤ 1`. `GripR` is the HandGrip miss normalized by the fit oval, where 1 is the oval’s edge. A BRP miss is `target − placed` (`BuildScore.BrpMissXMm`, `BrpMissYMm`).

## Nomenclature

Follow the naming rules in [`AGENTS.md`](../../AGENTS.md). Define what something is and does in plain English, and discuss it with Matt, before proposing a name.

- **Working glossary.** The Stage 1 note keeps a glossary table: term, plain-English meaning, type, owner and axes, what can change it, how its value is known, the decision that consumes it, and status (*proposed*, *ruled Q#*, or *in code at commit*). Later stages add their own terms to it.
- **Ruling.** A term becomes binding only after Matt confirms it in chat. The agent then records it as a `Q#` in `DESIGN.md`, and the glossary cites that `Q#`. `DESIGN.md` stays the source of truth, and the glossary is the audit’s index into it.
- **Renaming.** A rename of code, form, URL, or CLI follows a ruling and lands as its own commit, separate from behavior changes, so it is easy to review.
- Known tension to settle early: the code’s `SitBones` landmark and [`RIDER_FIT.md`](../RIDER_FIT.md)’s **SIT**, which that record says is a saddle support reference and not a pelvic location.

## Physical check on the Fenrir

Record the protocol and results in [`fenrir-measurements.md`](fenrir-measurements.md), created in Stage 1. It has three parts:

1. **Installed build.** Before measuring, record the inputs the model needs, with the source of each: frame size, Tuning Chip position (the catalog transcribes the middle chip), fork and its static A2C and rake, headset cups, tire size and measured wheel radius, seatpost and insertion, saddle rail offset and pitch, stem length and angle, spacers, handlebar, and bar roll. Values Matt has already confirmed in `RIDER_FIT.md` stay settled, and he isn’t asked for them again.
2. **Procedure.** Specify the unloaded state: on the floor or in a stand, no rider, and fork and ShockStop at full extension. Specify how to establish level and frame pitch, the BB datum, and the axes. Then list each landmark to measure, such as wheelbase, BB height, saddle and grip landmarks, and their BB-relative X/Y, along with the expected precision of each.
3. **Comparison.** Run the same build through `bikegeo bike` and put the model’s prediction next to each measurement. A difference larger than the combined measurement and chart uncertainty is a finding. It may be a model defect, a wrong catalog value, a wrong installed input, or a measurement error, and the note says which and why.

When the audit needs a measurement, the agent writes it as a **measurement request** in the stage note’s handoff. The request says what to measure, the load state, the datum and axes, the tools, and the precision needed. Matt adds the result to `fenrir-measurements.md`, labeled **Measured**.

## First session: definitions and load states

Take one rigid bike, one saddle, one grip, and one setup. Describe what someone can measure, in which axes, and what stays fixed when one adjustment changes. Then describe the same installed bike, with a suspension fork or post, in two load states: unloaded on a stand and rider-loaded at sag. Use the contrast to find quantities whose definition or axis ownership is ambiguous. Discuss their meanings with Matt in plain English before settling names for code.

Use these prompts for that first discussion. Their answers become glossary columns.

| Question | Example distinction |
| --- | --- |
| What kind of quantity is it? | A landmark is a point. Motion between two positions is a displacement. A pose supplies an origin and oriented axes. |
| What owns the quantity, and in which axes is it expressed? | A rail adjustment follows the rails. A linkage displacement belongs to the post. Saddle pitch and bar roll are clamped to the post and stem once installed, even if they were first leveled in world axes. |
| What can change it? | Choosing a stem selects a part. Setting insertion adjusts an installed part. Loading the bike changes the suspension state of the same installation. A change of coordinates alone can’t move a physical part. |
| How do we know its value? | Manufacturer dimension, rider measurement under stated conditions, inferred parameter, or calculated result. A target also needs the load state and the position the rider wants to reproduce. |
| What decision consumes it? | A hardware limit constrains feasibility. A fit tolerance classifies an error. A score weight expresses a preference. |

## Stages, in dependency order

Keep one note per stage, plus focused notes such as [`brp-transfer.md`](brp-transfer.md) and `fenrir-measurements.md` where a physical problem needs its own record. Update this table whenever a stage changes status.

| Stage | Note | Status | Revision | Waiting on Matt | Questions to settle |
| --- | --- | --- | --- | --- | --- |
| 1. Definitions, coordinates, and load states | `01-definitions-and-states.md` | Not started | — | — | Can two readers reconstruct the same bike attitude, load state, and measurement axes? Which landmarks can they measure directly, and which must they infer? What is the glossary, and how will the Fenrir be measured? |
| 2. Forward placement | `02-forward-placement.md` | Not started | — | — | Given parts, adjustments, and a load state, where do the wheels, `BB`, `BRP`, `SitBones`, and `HandGrip` go? Do frame pitch (fork sag) and post travel carry clamped parts rigidly? Does the model agree with the measured Fenrir? |
| 3. Measurement and transfer | `03-measurement-and-transfer.md` | Not started | — | — | Under what conditions does a source measurement, such as Matt’s unsagged Redshift idmatch transfer, determine the desired seated position on another build? Which missing inputs allow more than one answer? |
| 4. Inverse fitting | `04-inverse-fitting.md` | Not started | — | — | For fixed parts and load state, which targets are reachable? When a target is unreachable, why does the chosen solution satisfy the caller’s objective across the legal range? Should HandGrip fitting stay BB-relative when BRP clamps short in X? |
| 5. Build fitting, scoring, and sensitivity | `05-build-fitting-and-ranking.md` | Not started | — | — | Does pruning in `FitBuilds` preserve the ordered top builds? Do the tiers and weights match Matt’s priorities? Where do tiers and rankings change in the coordinate and sag sweeps? |
| 6. End-to-end surface | `06-end-to-end-surface.md` | Not started | — | — | Do the form, URL, CLI, diagram, and scores describe the same landmarks and load state with the ruled names? If placed BRP is 3 mm forward of target, does compare report “3 mm forward” while the raw X miss (`target − placed`) is −3 mm? |

Statuses are *Not started*, *In progress*, *Waiting on Matt* (a ruling or measurement), *Verdict reached*, and *Reopened*. **Revision** is the commit the current verdict was checked against.

**Reopening.** Each verdict names the Go symbols it covers. If a later commit changes one of those symbols, including a fix from another stage, the verdict is marked *Reopened* until someone re-checks it against the new revision. When a fix lands, its stage note lists the downstream verdicts it reopens.

## Starting a session

Read, in order: this README, the stage note being worked on, [`RIDER_FIT.md`](../RIDER_FIT.md), [`brp-transfer.md`](brp-transfer.md), `fenrir-measurements.md` once it exists, and every `Q#` the stage note cites. Check `git status` and `docs/SLICE*.md`. Confirm that the stage’s pinned revision still matches the symbols it covers, and mark it *Reopened* if it doesn’t. Then work only on the question Matt chose for the session.

## Structure of each stage note

Each stage note should be concise and self-contained:

1. **Scope and premises.** State the physical inputs, coordinate axes, load state, fixed and varied quantities, upstream stages relied on, and the glossary terms used.
2. **Physical and mathematical check.** Derive what must hold in plain text or inline code math. Label each claim with its [kind of evidence](#kinds-of-evidence). Call out every premise labeled **Assumed**.
3. **Code audit and tests.** Pin the Go symbols and trace the call path.
   - Use temporary `scratch_*_test.go` tests only for one-off numerical exploration or sensitivity sweeps, following [`AGENTS.md`](../../AGENTS.md). Record exact inputs and outputs in the note before deleting the scratch file.
   - When a claim establishes an invariant, boundary behavior, ranking equivalence, or solver property of `pose`, `assembly`, or `compare`, cover it with a committed `_test.go` test so `make check` guards it permanently.
4. **Adversarial review and verdict.** Before a stage reaches *Verdict reached*, an independent pass (a second agent or a fresh session) challenges its claims by looking for counterexamples. The reviewer receives the claims, premises, and pinned symbols, but not the derivation, so that it doesn’t just follow the author’s reasoning. It probes clamp limits, gate boundaries (including `fitBoundTolMm`), extreme angles, and sag transitions. Its findings go in the note as a dated **Adversarial review** entry. The note then records confirmed defects with links to their fixes, rulings made (as `Q#` links), and open physical questions for Matt.

When Matt asks for a review of a stage note, append a dated **Review response** that names the step the reviewer verified or challenged and why. This is separate from the adversarial review.

## Defects and fixes

Bugs are fixed as the audit finds them, in the same session when the fix is small.

1. **Confirm.** State the defect in the stage note with a concrete failing case: inputs, expected result with its derivation, and actual result. If the expected result depends on a physical or domain choice that hasn’t been ruled, ask Matt for a `Q#` ruling first.
2. **Commit a disabled failing test.** Write a committed test that fails on the defect. Before disabling it, run it to confirm that it fails for the stated reason. Then disable it with `t.Skip("fit-audit defect: <note>.md — <one-line defect>")` and commit it on its own. A failing but disabled test is better than no test. `grep -rn "fit-audit defect" internal` lists every open defect.
3. **Fix.** After Matt agrees to the approach in chat, fix the defect, remove the `t.Skip`, and pass `make pre-commit`. A fix that spans more than about three commits, changes a physical model, or needs slice-local rulings gets a `SLICE<n>.md` under [`WORKFLOW.md`](../WORKFLOW.md) instead. Its disabled test stays committed until then.
4. **Record.** Add the test and fix commit hashes to the stage note, update the glossary or `DESIGN.md` if the fix changed a ruled meaning, and mark the verdicts the fix reopens.

Audit work on stages that don’t touch the symbols under repair can continue while a fix is pending.

## Session handoff

Matt chooses the next question and passes it to the next agent. End each session with these short entries in the stage note, even if an argument is unfinished:

- **Conclusion so far.** State what follows, under which assumptions, with which evidence label, and where the argument stops.
- **Evidence.** Link the derivation, measurement, test, or counterexample. For an implementation claim, include the code revision and the symbol examined.
- **Waiting on Matt.** List each ruling needed (with its options) and each measurement request, or write “None.”
- **Next question.** Write one question Matt can paste into another chat, for example: “An exact fit survives identical fork rotation on both bikes. Can an imperfect fit cross the vertical BRP gate under that same rotation?”

Update the stage table above to match.

## Parked discussion

The [BRP and HandGrip transfer note](brp-transfer.md) preserves the motivating case, earlier numerical observations, and questions for Stage 3. The first session starts with Stage 1 definitions.
