# bikegeo Reboot: Rolling-Wave Workflow & Prompt Playbook

This document codifies the **Rolling-Wave (Just-in-Time) workflow** for `cmd/bikegeo` and `internal/...`. Both Matt and agents consult it to know **what phase we are in**, **what level of detail belongs in each document**, **which model tier to use**, and **which prompt to run**.

## 1. Two Horizons & Three Decision Tiers

Keep work at the right level of resolution so design approval never stalls on implementation edge cases five slices away.

### Horizons

| Horizon | Artifact | Purpose | What belongs here | What does NOT belong here |
| --- | --- | --- | --- | --- |
| **1. Direction** | [DESIGN.md](DESIGN.md) | Compass: high-level intent, hard invariants, and confirmed domain rulings | Requirements, non-goals, coordinate/pose laws, package boundaries, physical/scoring models, confirmed `Q#` rulings, 2–3 sentence intent per upcoming slice | Future-slice struct field migrations, URL edge-case tables 4 slices ahead, HTML form layout details, internal file factoring |
| **2. Slice / Sub-slice Blueprint** | `SLICE<n>.md` (one slice ahead only) | Blueprint: exact plan written against **committed code** of Slice `N−1` | Open slice-local rulings, exact signatures, callers to update, commit split, measured test targets, rules that bite, verification steps | Pre-specifying Slice `N+1` or `N+2` |
| **3. Implementation & Acceptance** | Commits + handoff in `SLICE<n>.md` | Execution: build, verify (`make pre-commit` + live check), review, accept, delete `SLICE<n>.md` | Code, tests, handoff record, updating `DESIGN.md` Milestones on acceptance | Inventing unapproved design changes |

> **Existing detailed notes in `DESIGN.md`:** Confirmed `Q#` architectural and domain rulings stay binding. Implementation details already written in Milestones (`12a`–`14`) stay preserved as **head-start guidelines** that the slice planner refines against live code when each slice reaches planning, rather than pre-implementation contracts to audit upfront.

### Decision Triage

When a question surfaces during review or planning, classify it before escalating:

1. **Architectural / Domain / UX Fork (cross-slice)** → **Ask Matt now** (at Horizon 1) and record a `Q#` ruling in `DESIGN.md`. Examples: physical modeling laws, scoring philosophy, package dependency rules, removing a major surface.
2. **Slice-Local Product / API Choice** → **Defer until that slice is next** (Horizon 2), then ask Matt briefly before writing `SLICE<n>.md`. Examples: whether `/bike` adds a submitted-build score readout, how `/` handles invalid custom form combinations without JS.
3. **Obvious Implementation Mechanic** → **No upfront ruling.** The slice planner chooses the clean default consistent with existing invariants and notes it in one line in `SLICE<n>.md` so Matt can veto on plan review. Examples: compiler-enforced caller updates when a field changes, non-zero enum sentinels, returning `400` on half-empty query params, internal catalog file layout.

### When to Split a Slice into Sub-Slices

During Horizon 2 planning, split a planned slice (`12` → `12a`, `12b`, `12c`) when **any** of these hold:

- It mixes two distinct risk areas (for example, changing a physical model *and* porting 19 frames at once).
- It would require more than ~3–4 atomic commits or re-measuring unrelated test suites in the same pass.
- An intermediate state is independently testable and gives fast feedback before building the next layer.

Don't pre-split future slices aggressively in `DESIGN.md`; split just-in-time when the slice reaches planning and real code makes the boundary clear.

### Writing Actionable Slice Blueprints

When writing a `SLICE<n>.md` plan for a lower-tier implementer:

- **Attach every validation rule to its function and a negative test case:** never leave an invariant as a stray prose sentence inside an unrelated bullet; name the exact validator that enforces it and the test input that must fail.
- **Probe every readout that changes:** when a geometry or scoring change shifts test assertions, include independent probe values for all affected readouts (including web readouts and notes), not just core solver tables, so the implementer never has to bootstrap expected values from its own code.

---

## 2. "What's Next?" State Router & Model-Tier Guide

When Matt asks **"What's next?"**, any agent should inspect:
1. `git status --short` (uncommitted work?)
2. `docs/reboot/SLICE*.md` (any active slice plan, and its `Status:` line?)
3. `## Milestones` in [DESIGN.md](DESIGN.md) (next slice not marked `done and accepted`)

Then match the first applicable row below, state the **current phase**, recommend the **model tier**, and wait (or proceed if already on the right tier and asked to act):

| Observed Repo State | Current Phase | Recommended Model Tier | Next Action |
| --- | --- | --- | --- |
| `SLICE<n>.md` exists with `Status: Blocked` *(checked first, even if code is uncommitted)* | **Blocked on Design / Decision** | **Frontier** (e.g. Opus / Gemini Pro) | Resolve the blocking question with Matt, record ruling if lasting, update `SLICE<n>.md` |
| Uncommitted code/test changes in working tree | **In-progress work / awaiting review or commit** | Match the active `SLICE<n>.md` tier (or **Frontier** for review) | Review diff, verify `make pre-commit`, commit or revise before starting anything new |
| `SLICE<n>.md` exists with `Status: Planned` (or no `Status:` line yet, or `Partly implemented`) | **Slice Execution** | **Lower-tier** (e.g. Sonnet / Flash) for Larger slices; **Frontier** if Small (≤3 files, 1 commit) or if the plan explicitly requires Frontier | Implement `SLICE<n>.md` strictly per plan; stop and escalate to Frontier if an unspec'd design fork appears |
| `SLICE<n>.md` exists with `Status: Implemented …; awaiting Matt's acceptance` | **Review & Acceptance** | **Frontier** (for independent code review) or Matt directly | Review against `SLICE<n>.md` and invariants; once Matt accepts, delete `SLICE<n>.md` and update `DESIGN.md` Milestones |
| No `SLICE*.md` exists; next slice's direction in `DESIGN.md` has unresolved cross-slice ambiguity | **Horizon 1 — Direction Alignment** | **Frontier** | Resolve only the cross-slice fork(s) with Matt and update `DESIGN.md` |
| No `SLICE*.md` exists; next slice's intent in `DESIGN.md` is clear | **Horizon 2 — Plan Next Slice** | **Frontier** | Inspect committed code, surface only the 2–5 slice-local choices (if any), decide whether to sub-slice, and write `SLICE<n>.md` |

### Model-Tier Rule of Thumb

- **Frontier model** (Opus / Gemini Pro / top reasoning tier):
  - Horizon 1 design & directional reviews
  - Horizon 2 slice / sub-slice planning (`SLICE<n>.md`)
  - Independent review of implemented slices & acceptance cleanup
  - Unblocking any slice that hit an unexpected architectural or physical-model snag
  - Implementing **Small** slices (1 commit, ≤3 files) directly when planning them
- **Lower-tier model** (Sonnet / Flash / fast execution tier):
  - Implementing **Larger** planned slices strictly from an approved `SLICE<n>.md`
  - Mechanical refactors or repetitive catalog porting whose spec and tests are fully written in `SLICE<n>.md`
  - **Escalation rule:** if a lower-tier implementer discovers a real design choice not covered by `SLICE<n>.md`, it must **stop**, record `Status: Blocked <date> on <question>` in the handoff, and tell Matt to switch to a Frontier model rather than guessing.

---

## 3. Review Calibration (Anti-Nitpick Rules)

All review passes — whether reviewing `DESIGN.md`, `WORKFLOW.md`, a slice plan, or implemented code — must calibrate rigor to the artifact being reviewed:

- **Match rigor to the file type:**
  - **Code & tests (`internal/**`, `cmd/**`):** full rigor (geometry math, edge cases, physical invariants, nil/NaN safety, boundary rules).
  - **Docs (`DESIGN.md`, `WORKFLOW.md`, `AGENTS.md`):** directional rigor only — report real contradictions between active rules, broken provenance (`illustrative` mislabeled as measured/physical), or broken workflow routing/links.
- **No `nit`-severity noise:** do not report `nit`-severity items or stylistic phrasing preferences. (Practical git-hygiene catches — such as a newly linked untracked file — are still welcome.)
- **Assume a competent reader with live code:** never flag a doc omission that the Go compiler or the committed code of prior slices makes obvious (for example, skipping tie-breaks on a `nil` pointer, updating callers of a renamed field, or internal file layout).
- **Senior-engineer bar:** before emitting any finding, ask whether a senior engineer would actually ask for a change or roll their eyes. Drop the second bucket, and never invent contrived literalist failure scenarios to justify a finding.

---

## 4. Copy-Paste Prompt Bank

Copy any prompt below verbatim (filling in `<N>` where shown).

### A. Router — "What's next?" *(any model)*

```text
Consult docs/reboot/WORKFLOW.md, check git status, docs/reboot/SLICE*.md, and docs/reboot/DESIGN.md Milestones.
Tell me:
1. What phase we are currently in.
2. Exactly what the next step is.
3. Whether I should use a Frontier model or a lower-tier model for it (and whether you are the right tier to proceed).
Do not edit files or start investigating future slices yet.
```

### B. Horizon 1 — Direction Review of `DESIGN.md` / Workflow Docs *(Frontier model)*

```text
Review docs/reboot/DESIGN.md (and any changed reboot docs) using the Rolling-Wave Horizon rule and Review Calibration (Anti-Nitpick Rules) in docs/reboot/WORKFLOW.md.
Audit ONLY for directional soundness, physical-model bugs, broken provenance, and real contradictions between active rules.
Do NOT emit nit-severity findings, stylistic tweaks, or detailed specs for future slices; park genuine slice-local choices as one-line reminders under the owning slice.
End with a clear verdict: ready to green-light, or the minimal list of real issues that need fixing now.
```

### C. Horizon 2 — Plan Next Slice (or Sub-Slice) *(Frontier model)*

```text
Follow docs/reboot/WORKFLOW.md and docs/reboot/DESIGN.md (Slice planning) to plan the next unstarted slice: Slice <N>.
1. First enforce the planning gate (stop if any SLICE*.md exists).
2. Read the current committed code relevant to Slice <N> ONLY (do not pre-spec Slice <N+1>).
3. Check whether Slice <N> should be split into sub-slices; if so, propose the split first.
4. Apply Decision Triage: ask me ONLY the genuine slice-local product/domain choices that need my ruling before writing SLICE<N>.md; handle obvious mechanics by proposing clean defaults.
5. When writing SLICE<N>.md, open with a `Status: Planned <today>` line right below the title.
Do not write code yet.
```

### D. Horizon 3 — Implement Planned Slice *(Lower-tier model for Larger slices; Frontier for Small)*

```text
Implement docs/reboot/SLICE<N>.md following every requirement in docs/reboot/WORKFLOW.md and the plan's Rules that bite.
- Follow the planned commit split and run `make pre-commit` for each commit.
- Do not make unapproved design choices or deviate without a real blocker. If you hit a genuine design gap the plan didn't cover, stop, mark the plan Blocked with an Implementation handoff, and tell me to switch to a Frontier model.
- Finish by completing the Implementation handoff section in SLICE<N>.md and updating the slice's Status line in SLICE<N>.md and DESIGN.md Milestones.
```

### E. Horizon 3 — Review Implemented Slice *(Frontier model)*

```text
Independently review the implemented Slice <N> against docs/reboot/SLICE<N>.md, docs/reboot/DESIGN.md invariants, AGENTS.md rules, and the Review Calibration (Anti-Nitpick Rules) in docs/reboot/WORKFLOW.md.
Use full rigor on Go code/tests (math, invariants, edge cases, `make pre-commit`) and directional rigor on docs; drop nit-severity noise.
Append concise findings under the existing Implementation handoff in SLICE<N>.md (do not rewrite the implementer's record), and tell me whether Slice <N> is ready for acceptance.
```

### F. Horizon 3 — Accept Slice *(Frontier or Lower-tier model)*

```text
I accept Slice <N>. Execute the "After acceptance" checklist in docs/reboot/DESIGN.md:
1. Promote any lasting rulings or open issues I want kept into DESIGN.md.
2. Update the Milestones line for Slice <N> to `done and accepted <today>` with its commit hashes.
3. Delete docs/reboot/SLICE<N>.md and remove/rewrite any links to it.
Commit the acceptance cleanup and tell me what the next phase and recommended model tier are per docs/reboot/WORKFLOW.md.
```

### G. Drop-in Calibration Snippet *(for external/standalone code reviewers like Claude)*

```text
Follow the Review Calibration rules in docs/reboot/WORKFLOW.md (§3): full rigor on Go code/tests, directional rigor only on Markdown docs (contradictions, broken provenance, broken links/routing). Do not emit `nit`-severity findings or contrived literalist failure scenarios for things the compiler or live code makes obvious.
```
