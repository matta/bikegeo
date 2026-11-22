# bikegeo Reboot: Rolling-Wave Workflow & Prompt Playbook

This document is the single source of truth for the **Rolling-Wave (Just-in-Time) workflow** for `cmd/bikegeo` and `internal/...`. Both Matt and agents consult it to know **what phase we are in**, **what level of detail belongs in each document**, **which model tier to use**, and **which prompt to run**.

## 1. Two Horizons & Three Decision Tiers

Keep work at the right level of resolution so design approval never stalls on implementation edge cases five slices away, and planning never turns into implementing the slice twice.

### Horizons

| Horizon | Artifact | Purpose | What belongs here | What does NOT belong here |
| --- | --- | --- | --- | --- |
| **1. Direction** | [DESIGN.md](DESIGN.md) | Compass: high-level intent, hard invariants, and confirmed domain rulings | Requirements, non-goals, coordinate/pose laws, package boundaries, physical/scoring models, confirmed `Q#` rulings, 2–3 sentence intent per upcoming slice | Future-slice struct field migrations, URL edge-case tables 4 slices ahead, HTML form layout details, internal file factoring |
| **2. Slice / Sub-slice Plan** | `SLICE<n>.md` (one slice ahead only) | Intent & contract written against **committed code** of Slice `N−1` | Goal, confirmed rulings, key design/API decisions, rough commit split, high-level test cases, out-of-scope, `Implementer:` and `Review:` headers | Pre-specifying Slice `N+1`, throwaway dry-runs, shell recipes, exhaustive caller lists, exact diff-stat counts, pre-measured readout tables |
| **3. Implementation & Acceptance** | Commits + brief handoff in `SLICE<n>.md` | Execution: build, verify (`make pre-commit` + live check if UI), diff-only review (when required), accept, delete `SLICE<n>.md` | Code, tests, 3–5 line handoff folded into the last commit, updating `DESIGN.md` Milestones on acceptance | Inventing unapproved design changes, repeating the implementer's validation during review |

> **Existing detailed notes in `DESIGN.md`:** Confirmed `Q#` architectural and domain rulings stay binding. Implementation details already written in Milestones stay preserved as **head-start guidelines** that the slice planner refines against live code when each slice reaches planning, rather than pre-implementation contracts to audit upfront.

### Decision Triage

When a question surfaces during review or planning, classify it before escalating:

1. **Architectural / Domain / UX Fork (cross-slice)** → **Ask Matt now** (at Horizon 1) and record a `Q#` ruling in `DESIGN.md`. Examples: physical modeling laws, scoring philosophy, package dependency rules, removing a major surface.
2. **Slice-Local Product / API Choice** → **Defer until that slice is next** (Horizon 2), then ask Matt briefly before writing `SLICE<n>.md`. Examples: whether `/bike` adds a submitted-build score readout, how `/` handles invalid custom form combinations without JS.
3. **Obvious Implementation Mechanic** → **No upfront ruling.** The slice planner (or implementer) chooses the clean default consistent with existing invariants. Note non-obvious defaults in one line in `SLICE<n>.md` so Matt can veto on plan review. Examples: compiler-enforced caller updates when a field changes, non-zero enum sentinels, returning `400` on half-empty query params, internal file layout.

### When to Split a Slice into Sub-Slices

During Horizon 2 planning, split a planned slice (`12` → `12a`, `12b`, `12c`) when **any** of these hold:

- It mixes two distinct risk areas (for example, changing a physical model *and* porting 19 frames at once).
- It would require more than ~3–4 atomic commits or re-measuring unrelated test suites in the same pass.
- An intermediate state is independently testable and gives fast feedback before building the next layer.

Don't pre-split future slices aggressively in `DESIGN.md`; split just-in-time when the slice reaches planning and real code makes the boundary clear.

### Slice Planning & Blueprints

Each slice plan lives in `docs/reboot/SLICE<n>.md` (for example `SLICE20.md`) only until Matt accepts the slice. Plan **one slice ahead** against committed code.

- **Planning gate:** don't start planning a new slice while any `SLICE*.md` file exists, whatever its status. List the remaining plan with its `Status:` line and follow the state router in §2 instead. Hold to this even if asked for a new plan.
- **Rulings first:** before writing `SLICE<n>.md`, get Matt's rulings on any genuine slice-local choices (`Q#`) and record lasting ones under **Confirmed decisions** in `DESIGN.md`. The plan cites them and doesn't decide them.
- **Required header lines** (immediately below the title):
  ```markdown
  Status: Planned <YYYY-MM-DD>
  Implementer: Low (fast/lower-tier) | Medium (mid-tier) | High (frontier) — <one-line why>
  Review: required | optional — <one-line why>
  ```
- **Small slices (1 commit, ≤3 code/test files):** keep the plan sparse (goal, rulings, files, tests) and offer to implement it directly in the same session.
- **What belongs in `SLICE<n>.md`:** goal, confirmed rulings, key design/API decisions and one-line planner defaults, rough commit split (each commit must pass `make pre-commit`), high-level test cases (including key negative cases for invariants), and out-of-scope.
- **What does NOT belong in `SLICE<n>.md` (no implementing twice):**
  - **Never dry-run or pre-implement the slice** in a throwaway worktree or scratch branch.
  - **No verbatim shell commands, `sed` scripts, exhaustive caller lists, or exact `git diff --stat` counts.** Trust the implementer and the Go compiler on mechanics.
  - **No pre-measured test readout tables** (HTML titles, CLI banners, etc.). Exception: a quick independent math/geometry probe is allowed *only* when changing physical formulas or scoring math, so the implementer doesn't bootstrap expected math from its own code.

### Implementation Handoff & Acceptance

- **Handoff (3–5 lines, folded into the last commit):** when finishing (or stopping early / getting blocked), update `SLICE<n>.md`'s `Status:` line and add a brief `## Implementation handoff` right below the header — **in the same commit as the final implementation change** (or uncommitted alongside partial/blocked work). Do **not** edit `DESIGN.md` Milestones during implementation.
  - `Status:` becomes one of `Implemented <date>; awaiting review` (when `Review: required`), `Implemented <date>; awaiting Matt's acceptance` (when `Review: optional`), `Partly implemented <date>; stopped after <step>`, or `Blocked <date> on <question>`.
  - Include only: **Who & when** (model + date), **Commits** (hashes), **Deviations** (or "None"; real deviations need a reason), **Verification** (`make pre-commit` pass/fail + 1-line live check if UI changed), and **Open issues** (if any).
- **After acceptance:** when Matt accepts the slice, make one cleanup commit that:
  1. Promotes any lasting rulings or open issues Matt wants kept into `DESIGN.md`.
  2. Updates the slice's line in `DESIGN.md` Milestones to `done and accepted <date>` with its commit hashes.
  3. Deletes `docs/reboot/SLICE<n>.md` and removes/rewrites any links to it.

---

## 2. "What's Next?" State Router & Model-Tier Guide

When Matt asks **"What's next?"**, inspect:
1. `git status --short` (uncommitted work?)
2. `docs/reboot/SLICE*.md` (any active slice plan, and its `Status:`, `Implementer:`, and `Review:` lines)
3. `## Milestones` in [DESIGN.md](DESIGN.md) (next slice not marked `done and accepted`)

Match the first applicable row below, state the **current phase**, recommend the **model tier**, and wait (or proceed if already on the right tier and asked to act):

| Observed Repo State | Current Phase | Recommended Model Tier | Next Action |
| --- | --- | --- | --- |
| `SLICE<n>.md` exists with `Status: Blocked` *(checked first, even if code is uncommitted)* | **Blocked on Design / Decision** | **Frontier** (e.g. Opus / Gemini Pro) | Resolve the blocking question with Matt, record ruling if lasting, update `SLICE<n>.md` |
| Uncommitted code/test changes in working tree | **In-progress work / awaiting commit** | Match the active `SLICE<n>.md` `Implementer:` tier | Verify `make pre-commit`, finish/commit or revise before starting anything new |
| `SLICE<n>.md` exists with `Status: Planned` (or `Partly implemented`) | **Slice Execution** | Read `Implementer:` in `SLICE<n>.md` (`Low` → fast/lower-tier, `Medium` → mid-tier, `High` → Frontier) | Implement `SLICE<n>.md`; stop and escalate to Frontier only if an unspec'd design fork appears |
| `SLICE<n>.md` exists with `Status: Implemented …` and `Review: required` (not yet reviewed) | **Independent Review** | **Frontier** | Diff-only judgment review (§3); append concise findings under `## Implementation handoff` and report readiness |
| `SLICE<n>.md` exists with `Status: Implemented …` and `Review: optional` (or review already done) | **Acceptance** | **Matt directly** (any model for cleanup commit) | Once Matt accepts, run the After acceptance cleanup (§1) |
| No `SLICE*.md` exists; next slice's direction in `DESIGN.md` has unresolved cross-slice ambiguity | **Horizon 1 — Direction Alignment** | **Frontier** | Resolve only the cross-slice fork(s) with Matt and update `DESIGN.md` |
| No `SLICE*.md` exists; next slice's intent in `DESIGN.md` is clear | **Horizon 2 — Plan Next Slice** | **Frontier** | Inspect relevant committed code, surface only genuine slice-local choices, decide whether to sub-slice, and write `SLICE<n>.md` |

### Complexity & Model-Tier Guide

- **Planning, Direction, Review, Unblocking:** always **Frontier** (Opus / Gemini Pro / top reasoning tier).
- **Implementation (`Implementer:` complexity rating in `SLICE<n>.md`):**
  - **`Low` (fast / lower-tier, e.g. Flash / Sonnet):** mechanical refactors, renames, doc updates, straightforward wiring, repetitive catalog entries.
  - **`Medium` (mid-tier, e.g. Sonnet):** multi-file feature work following established patterns, HTML/SVG template and query wiring, routine test additions.
  - **`High` (Frontier, e.g. Opus / Gemini Pro):** novel geometry/physics math, scoring changes, tricky invariants or state interactions, or Small slices implemented directly during planning.
- **Review trigger (`Review:` in `SLICE<n>.md`):**
  - **`required`:** geometry/physics math, scoring logic, or architectural/package-boundary changes.
  - **`optional`:** mechanical refactors, docs, catalog data with existing test validation, or small/low-risk slices.
- **Escalation rule:** if an implementer hits a genuine design choice not covered by `SLICE<n>.md`, stop, mark `Status: Blocked <date> on <question>` with a brief handoff, and tell Matt to switch to a Frontier model rather than guessing.

---

## 3. Review Scope & Calibration (Anti-Nitpick Rules)

### Diff-Only Judgment Review (Code Reviews)

When reviewing an implemented slice:
- **Read `git diff` against `SLICE<n>.md`, `DESIGN.md` invariants, and `AGENTS.md` rules.** Focus on what automated checks cannot catch: geometry/physics math, edge cases, nil/NaN safety, design drift, missing negative tests, and invariant violations.
- **Do not repeat the implementer's validation.** Trust the implementer's reported `make pre-commit` and live-check results. Never re-run builds, tests, `make boundary`, or manual checks unless you spot a concrete defect that needs proof or the handoff reports a skipped/failed check.

### Calibration (All Review Passes)

- **Match rigor to the file type:**
  - **Code & tests (`internal/**`, `cmd/**`):** full rigor (geometry math, edge cases, physical invariants, nil/NaN safety, boundary rules).
  - **Docs (`DESIGN.md`, `WORKFLOW.md`, `AGENTS.md`):** directional rigor only — report real contradictions between active rules, broken provenance (`illustrative` mislabeled as measured/physical), or broken workflow routing/links.
- **No `nit`-severity noise:** do not report `nit`-severity items or stylistic phrasing preferences. (Practical git-hygiene catches — such as a newly linked untracked file — are still welcome.)
- **Assume a competent reader with live code:** never flag a doc omission that the Go compiler or the committed code of prior slices makes obvious.
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
3. Which model tier to use (reading Implementer:/Review: from SLICE*.md when present, and whether you are the right tier to proceed).
Do not edit files or start investigating future slices yet.
```

### B. Horizon 1 — Direction Review of `DESIGN.md` / Workflow Docs *(Frontier model)*

```text
Review docs/reboot/DESIGN.md (and any changed reboot docs) using the Rolling-Wave Horizon rule and Review Calibration in docs/reboot/WORKFLOW.md.
Audit ONLY for directional soundness, physical-model bugs, broken provenance, and real contradictions between active rules.
Do NOT emit nit-severity findings, stylistic tweaks, or detailed specs for future slices; park genuine slice-local choices as one-line reminders under the owning slice.
End with a clear verdict: ready to green-light, or the minimal list of real issues that need fixing now.
```

### C. Horizon 2 — Plan Next Slice (or Sub-Slice) *(Frontier model)*

```text
Follow docs/reboot/WORKFLOW.md to plan the next unstarted slice: Slice <N>.
1. Enforce the planning gate (stop if any SLICE*.md exists).
2. Read the committed code relevant to Slice <N> ONLY (do not pre-spec Slice <N+1>).
3. Check whether Slice <N> should be split into sub-slices; if so, propose the split first.
4. Apply Decision Triage: ask me ONLY the genuine slice-local choices that need my ruling before writing SLICE<N>.md; handle obvious mechanics with one-line defaults.
5. Keep SLICE<N>.md intent-and-contract level: open with Status, Implementer (Low/Medium/High + why), and Review (required/optional + why). Do NOT dry-run the slice, write shell recipes, list every caller, or pre-measure readout tables.
Do not write code yet (unless it's a Small slice you are offering to implement directly).
```

### D. Horizon 3 — Implement Planned Slice *(tier per `Implementer:` in `SLICE<N>.md`)*

```text
Implement docs/reboot/SLICE<N>.md per docs/reboot/WORKFLOW.md.
- Follow the rough commit split and run `make pre-commit` for each commit.
- Handle routine mechanics cleanly; if you hit a genuine unspec'd design fork, stop, mark Status: Blocked with a brief handoff, and tell me to switch to a Frontier model.
- In the final implementation commit, update SLICE<N>.md's Status line and add the 3–5 line Implementation handoff. Do not edit DESIGN.md Milestones.
```

### E. Horizon 3 — Review Implemented Slice *(Frontier model, when `Review: required`)*

```text
Independently review Slice <N> per docs/reboot/WORKFLOW.md (§3): diff-only judgment review against docs/reboot/SLICE<N>.md, docs/reboot/DESIGN.md invariants, and AGENTS.md rules.
Read the git diff for math, invariants, edge cases, design drift, and test quality. Trust the implementer's reported `make pre-commit` and live-check results — do NOT re-run builds, tests, or manual checks unless proving a specific finding. Drop nit-severity noise.
Append concise findings under `## Implementation handoff` in SLICE<N>.md (do not rewrite the implementer's record) and state whether Slice <N> is ready for acceptance.
```

### F. Horizon 3 — Accept Slice *(any model)*

```text
I accept Slice <N>. Execute the "After acceptance" checklist in docs/reboot/WORKFLOW.md (§1):
1. Promote any lasting rulings or open issues I want kept into DESIGN.md.
2. Update the Milestones line for Slice <N> to `done and accepted <today>` with its commit hashes.
3. Delete docs/reboot/SLICE<N>.md and remove/rewrite any links to it.
Commit the acceptance cleanup and tell me what the next phase and recommended model tier are per docs/reboot/WORKFLOW.md.
```

### G. Drop-in Calibration Snippet *(for external/standalone code reviewers)*

```text
Follow docs/reboot/WORKFLOW.md (§3): diff-only judgment review on code/tests (trust the implementer's reported `make pre-commit` unless proving a specific finding), directional rigor only on Markdown docs, no `nit`-severity noise or contrived literalist failure scenarios.
```
