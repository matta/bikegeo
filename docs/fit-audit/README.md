# Proposed method for auditing bikegeo fit

Status — methodology awaiting Matt’s review. The audit has not started. The code revision at the time of this proposal is `b961e8818121`.

The audit should answer three questions separately. What physical situation does the model represent, and with what evidence? What follows mathematically if its assumptions hold? Does the Go implementation deliver that result across its supported inputs? A fitting preference, such as how to rank two unreachable targets, needs an explicit objective before anyone can evaluate the algorithm against it.

## First session

Take one rigid bike, one saddle, one grip and one setup. Describe what someone can measure, in which axes, and what stays fixed when one adjustment changes. Then describe the *same installed bike* with a suspension fork or post in two load states. Use the contrast to find ambiguous quantities. The deliverable is a short vocabulary for discussion, an inventory of quantities that lack a definition, and the first precisely stated claim. Discuss the meanings with Matt before settling names for code.

Use these as prompts for that first discussion:

| Question | Example distinction |
| --- | --- |
| What kind of quantity is it? | A landmark is a point. Motion between two positions is a displacement. A pose supplies an origin and oriented axes. |
| What owns the quantity, and in which axes is it expressed? | A rail adjustment follows the rails. A linkage displacement belongs to the post. Both can be expressed in BB-relative axes after placement. |
| What can change it? | Choosing a stem selects a part. Setting insertion adjusts an installed part. Loading the bike changes suspension state with the same installation. A change of coordinates alone cannot move a physical part. |
| How do we know its value? | Manufacturer dimension, rider measurement under stated conditions, inferred parameter, or calculated result. A target also needs the position the rider wants to reproduce. |
| What decision consumes it? | A hardware limit constrains feasibility. A fit tolerance classifies an error. A score weight expresses a preference. |

## Work in dependency order

| Stage | Question to settle |
| --- | --- |
| Definitions and coordinates | Can two readers reconstruct the same physical state from the description? |
| Forward placement | Given parts, adjustments and a load state, where do the wheels, BB, BRP and HandGrip go? Establish rigid placement and leveling before adding relative component motion. |
| Measurement and transfer | Under what conditions does a measurement determine the desired position on another build? Which missing inputs allow multiple answers? |
| Inverse fitting | For fixed parts and load state, which targets are reachable within the adjustment bounds? Does the solver return a legal exact solution when one exists? For an unreachable target, what result does it promise? |
| Search and ranking | What discrete options are considered? Can pruning discard a better feasible build? Do the returned order and tie handling implement the declared preferences? |
| End-to-end interpretation | Do the form, URL, CLI, diagram and score refer to the same landmarks and state? Can a user reproduce the indicated setup on a real bike? |

Each stage can take several sessions. Pick one claim whose dependencies are ready. An example of a suitably sized claim is “With fork state, post, rail pitch and insertion fixed, sliding the saddle a legal distance along the rails translates BRP and SIT by the same vector.” An example about the inverse solver needs a different argument covering the full feasible set and its boundaries.

## One claim per Markdown note

Give each claim a stable ID, a descriptive filename, and one row in a small index in this README once work begins. The row holds the question, dependencies, author, reviewer, status and link. Keep one working note per claim so agents can investigate independent claims without rewriting each other’s reasoning.

Each note should let a fresh reader reconstruct the case:

1. **Claim and scope.** State the input domain, units, axes, load state, fixed quantities, permitted changes, and promised output. Link any prerequisite claims.
2. **Argument.** Derive the result from those premises in plain language or plain-text math. State where existence, uniqueness, optimality or numerical tolerance matters. Identify any physical assumption that requires measurement.
3. **Implementation and evidence.** Pin the code commit and relevant symbols. Trace the normal call path. Look for a counterexample, including a limit or degenerate case. For a scratch Go investigation, record the exact fixture, changed inputs, functions called and observed values before deleting the scratch file. A numerical example can disprove a general claim. A passing example alone cannot establish it.
4. **Verdict and handoff.** Record the mathematical conclusion, the implementation finding, and any outstanding physical or product question separately. End with the last verified step and one concrete next action, even if the session stops halfway through a proof.

## Review and resume

The author hands off a saved revision of the note. A different agent starts with the claim, premises and pinned code, sketches an independent argument or counterexample, then reads the author’s argument. The reviewer appends a dated finding identifying the revision reviewed, the strongest challenge tried, and the exact step accepted or disputed. Preserve disagreements and link the author’s resolution. A later reader should be able to see what changed the verdict.

Use **open**, **in progress**, **awaiting review**, **reviewed**, or **reopened** for the index status. A reviewed note may establish a result under assumptions, exhibit a defect, or identify an unresolved premise. At a handoff between checkouts, share the Git revision containing the notes as well as the revision of the code examined. On resuming, read the index and the note’s handoff first. If a relevant symbol, assumption or upstream claim changed, reopen the affected conclusion and flag its dependents for recheck.

A handoff can be as short as “Review claim 03 at notes commit … against code commit …. Claim 01 supplies the coordinate convention. The outstanding question is whether the argument covers a target exactly on the rail limit.” A partial handoff should name the first unresolved step just as precisely.

Finish the audit with a coverage table linking each forward-placement, fitting and ranking promise to its reviewed conclusion or explicit gap. Carry accepted design rulings into DESIGN.md and implementation work into the existing slice workflow. [Repository workflow](../WORKFLOW.md)

## Parked discussion

The [BRP and HandGrip transfer note](brp-transfer.md) preserves the motivating case, earlier numerical observations, and questions for the audit. The first session starts with definitions.
