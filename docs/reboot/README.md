# Multi-Agent Design Protocol (`Gemini` ↔ `ChatGPT` + Human Operator)

## 1. Purpose & Roles

This directory preserves a design conversation between **Gemini**, **ChatGPT**, and **Matt**. Discussion may happen in either agent's chat, with Matt copying responses between them. Files and Git provide the shared memory; they need not carry every conversational exchange.

The goal is to converge on a single, clear, actionable design intent for the clean-slate `bikegeo` reboot without falling into premature over-engineering or endless agent back-and-forth.

### Role Boundaries
- **Human Operator (Matt) — Decision Maker:**
  - Holds sole authority over significant architectural choices (programming language, UI/visualization stack, physical modeling scope, data storage, and trade-offs between simplicity and generality).
  - Prompts whichever agent is up next, answers open questions in the Decision Queue, resolves disagreements, and declares when a topic is settled.
- **Gemini & ChatGPT — Advisors, Synthesizers, and Reviewers:**
  - Act as sources of concrete ideas, illustrative code sketches, critical review, and trade-off analysis.
  - Actively look for flaws, hidden complexity, or readability traps in both their own and the other agent's proposals.
  - **Defer to the human:** Neither agent may unilaterally lock in a major architectural choice or build a deep specification on top of an unconfirmed assumption. When a major fork arises, frame the options clearly in the **Decision Queue** and wait for the human's ruling on dependent work. Continue reasoning that does not depend on that choice.
  - **Record, rather than infer, approval:** Only the human approves significant decisions; agents may record that explicit approval in **Human-Confirmed Decisions**. Silence, agreement between agents, and unexplained file edits are not approval.

### Scope of a Request
- An explicit request to take an agent turn starts a **design turn** and follows the lifecycle below.
- Ordinary design conversation with Matt uses the lightweight discussion notes below. The agent receiving the conversation is responsible for recording useful developments without waiting for Matt to request a formal turn or say "record this."
- Requests to review, explain, or revise the protocol follow the requested scope; they do not automatically create a design turn, change design decisions, or require a turn commit.
- Design turns may propose implementation or experiments. Executing them requires authorization covering that work; a design turn alone does not provide it. Authorization already given remains valid.
- Use one active writer at a time. The human hands the conversation to the next agent; this protocol does not require agents to launch one another.

---

## 2. Shared Filesystem State (`docs/reboot/`)

Durable collaboration state lives in three files in `docs/reboot/`; relevant chat developments are recorded through formal turns or short discussion notes:

1. **`docs/reboot/README.md`** *(this file)*
   - The directory entrypoint: rules of engagement, file structure, turn lifecycle, and commit conventions.
2. **`docs/reboot/DESIGN.md`** *(the living canonical design intent)*
   - The single source of truth that the agents are working to converge on. It is organized into four strict sections:
     1. **Human-Confirmed Decisions**: Architectural choices explicitly approved by the human (tagged with the recording turn or dated discussion note). Agents must treat these as fixed constraints unless the human reopens them.
     2. **Converged Proposals (Pending Human Sign-off)**: Specific proposals explicitly endorsed by both agents, awaiting human approval. Agreement between agents is review evidence, not human approval or proof of correctness.
     3. **Active Design Options (Under Discussion)**: Competing ideas or API shapes currently being compared, with brief pros/cons and small code sketches.
     4. **Decision Queue (For the Human Operator)**: A short list of numbered questions (`Q1`, `Q2`, ...) needed now, stating the options, agent recommendations, and what work each blocks. Do not renumber or reuse `Q#` IDs once assigned so references in `DIALOGUE.md` stay unambiguous. Deferred decisions may be listed separately at the bottom of this section with a trigger for revisiting them.
3. **`docs/reboot/DIALOGUE.md`** *(the conversation log)*
   - An append-only chronological log of numbered agent turns and unnumbered, dated discussion notes.
   - Holds the critique, counter-proposals, and reasoning behind changes made to `DESIGN.md` so neither agent overwrites the other's rationale in silence.
   - Correct an earlier claim in a later entry, referencing the original; do not silently rewrite history.

### Recording Ordinary Discussion

- Record a short note when a chat changes a requirement, recommendation, rationale, disagreement, or open question, including substantive responses pasted from the other agent. Routine explanations and acknowledgments that add nothing new need no entry.
- Before editing, check the Git state and read the current design and latest log entries as in Step 1 below, preserving unrelated changes.
- Capture the main points, not a transcript. Attribute them to Matt, Gemini, or ChatGPT; distinguish an explicit decision from a preference or tentative suggestion. A pasted agent proposal is not Matt's approval of it.
- Use a heading such as `## Discussion — Q2 tracing — ChatGPT recording (2026-11-18)` and a few bullets covering what changed, why, and what remains open. No new numbering scheme or mandatory full turn template is needed.
- Before finishing a substantive design response, append the note and update DESIGN.md if its current summary or queue has changed. Incorporate the reply being given as well as the incoming guidance. If the same exchange is already recorded, add only new information; do not duplicate it when it is pasted back.
- Review and commit the documentation changes together using `reboot(notes/<agent>): <summary>`, following Step 5's diff-review and staging rules. One note and commit may cover an exchange with several points; do not create entries for every individual message or invent a formal agent turn.
- The next agent reads DESIGN.md and the latest dialogue entries, including discussion notes. Matt can keep copying responses when convenient; he is not responsible for maintaining the durable summary.

This is permission to maintain the design record during discussion, not to treat proposals as approved or start implementation. Protocol-only conversations remain scoped to the protocol and need no design-log entry.

---

## 3. Turn Lifecycle (Step-by-Step)

When the human explicitly asks either **Gemini** or **ChatGPT** to take a formal design turn:

### Step 1: Sync Context from Git & Disk
1. Run `git status -s` and `git log -n 10 --oneline -- docs/reboot/` to see recent turns and identify existing uncommitted changes. Git status alone does not establish who made them or whether they express approval.
2. Read `docs/reboot/DESIGN.md` and the latest turns in `docs/reboot/DIALOGUE.md`.
3. Inspect existing edits, preserve unrelated work, and incorporate relevant explicit human guidance. Do not treat ambiguous edits as rulings. If another writer is active, coordinate the handoff before editing.

### Step 2: Process Human Guidance First
1. Check whether the human operator provided rulings or feedback—either in the chat prompt invoking the turn, or via explicit edits to `DESIGN.md` / `DIALOGUE.md`.
2. Move any explicitly answered `Q#` items from the **Decision Queue** into **Human-Confirmed Decisions** in `DESIGN.md` (recording `(Turn N, resolves Q#)`).
3. If guidance is ambiguous, preserve the uncertainty and ask only for the clarification needed for dependent work. Do not turn a preference or partial approval into a broader ruling.

### Step 3: Draft the Turn in `DIALOGUE.md`
Append a new section to `docs/reboot/DIALOGUE.md` using this template:

```markdown
## Turn <N> — <Gemini | ChatGPT> (<YYYY-MM-DD>)

### 1. Human Guidance Applied
- Bullet summary of any human decisions or steering applied at the start of this turn (or "None this turn").

### 2. Points of Convergence
- State which proposals or `Q#` items you agree with, disagree with, or agree with under a condition.
- Record only new or changed positions; "No new objection" is a valid outcome.

### 3. Critique, Ideas & Code Sketches
- Focus on 1–2 unresolved topics per turn (an initial review turn may triage the full initial queue).
- Add critique or refinements only when useful; do not invent objections to fill the template.
- For competing APIs, show the same real bicycle operation in short, comparable sketches.
- State the basis for nontrivial technical or performance claims (back-of-the-envelope math, existing `bikegeo` code, or a quick measurement) rather than asserting unverified numbers as fact.

### 4. Proposed Rulings for the Human
- Reference `Q#` IDs, recommendations, and the specific work blocked (or "None").
- When evidence is missing, propose a small experiment with a clear question and success criterion instead of asking the human to settle a technical claim by guesswork.
```

### Step 4: Update `docs/reboot/DESIGN.md`
Update `DESIGN.md` so it reflects the latest state of convergence:
- Promote a proposal to **Converged Proposals** only when both agents explicitly endorse the same version. Material revisions require renewed agreement.
- Refine **Active Design Options** so both agents' best variants are represented side-by-side when a trade-off remains open.
- Keep the **Decision Queue** short (at most 3–5 active questions at a time). Each `Q#` entry states the decision, recommendation (or specific disagreement), practical consequences, and what work it blocks.
- Ask the human to judge preferences and trade-offs. Before asking which API is clearer, show comparable code examples.
- Defer decisions that do not block the next milestone, with a concrete trigger for revisiting them.

### Step 5: Commit the Turn to Git
Inspect `git diff` to review your changes (run code verification `go vet ./... && go test ./...` only if a turn modifies code; markdown-only turns in `docs/reboot/` do not need to run `make pre-commit`). Stage only the files or hunks belonging to the turn, preserving any unrelated edits. Create a single atomic commit for the turn using this commit message format:

```text
reboot(turn-<N>/<agent>): <concise summary of turn>
```

Examples:
- `reboot(turn-1/gemini): initialize reboot protocol and seed Turn 1 proposal`
- `reboot(turn-2/chatgpt): critique turtle scene-graph API and solver boundaries`

### Step 6: Report Back to the Human
In the chat response to the human operator, keep the summary brief:
1. State what changed or converged, including when no new objection remains.
2. Highlight only the decisions or proposed experiments needed next, with the recommendation and what they unblock. Do not manufacture questions to keep the conversation going.

---

## 4. Behavioral Guardrails (Lessons from `bikegeo`)

Both agents must enforce these guardrails on themselves and on each other:

1. **No Complexity Escalation ("One-Upmanship"):**
   In multi-agent reviews, agents often try to add value by inventing edge cases and proposing heavier type machinery (unusable zero values, phantom stage types, compile-negative harnesses, or general CAD frameworks). **Push in the opposite direction:** earn your keep by showing how a concept can be expressed with *fewer* types and *clearer* physical steps.
2. **Optimize for Local Visual & Code Inspection:**
   Remember Requirement (a): the human operator wants to inspect geometry by reading high-level physical steps (like a turtle/pose recipe) and seeing the resulting structure graphically. Whenever proposing an API, show how a real bicycle assembly step reads in code. Shared geometry keeps drawings and measurements consistent, but both can share a mistake; consequential correctness claims also need independent examples, measurements, or physical invariants.
3. **Bound Work at an Unresolved Fork:**
   If a design choice depends on a question in the **Decision Queue** (e.g., Go core vs. all-TypeScript), sketch alternatives briefly and state your recommendation. Do not write a deep specification assuming an answer. Pause only dependent work; continue independent reasoning and evidence gathering within the authorized scope.
4. **Preserve Proven Domain Truths:**
   Unless the human explicitly decides to change the physical model, preserve the hard-won domain insights from `bikegeo`:
   - Published manufacturer charts are sacred ground truth with rounding budgets, not numbers to nudge.
   - `StemClamp` vs. `HandGrip` and `BRP` vs. `SitBones` are distinct physical landmarks.
   - Rear cockpit (seat tube → post → rail → saddle) and front cockpit (steerer → headset/spacers → stem → handlebar) sit on the leveled frame.
5. **Ground Technical Claims:**
   Distinguish measured facts, existing `bikegeo` behavior, back-of-the-envelope estimates, and hypotheses. When estimating performance or search-space size, show the arithmetic and assumptions; both agents agreeing does not validate an unmeasured claim.
6. **Stop Unproductive Debate:**
   Focus each turn on at most 1–2 unresolved topics (after initial queue triage). Do not repeat settled arguments or require new objections. After two consecutive review turns on a topic without new evidence or substantive progress, summarize the disagreement and recommend a human decision or a small experiment. Preserve useful dissent rather than forcing consensus.
7. **Converge on the Next Milestone:**
   The design phase is ready to hand off when the human confirms that:
   - All requirements (`a` through `e`) have an agreed architectural direction and an outline of how they will be verified in `DESIGN.md`.
   - The first implementation milestone has sufficient detail to build and evaluate, including an acceptance criterion based on readable construction and visual inspection.
   - No unanswered decision blocks that milestone. Later decisions are explicitly deferred with triggers for revisiting them; future core data types need not all be specified now.
   - A concrete milestone sequence is recorded, with later milestones described only as deeply as current evidence supports.

   An empty active Decision Queue means the next milestone is unblocked, not that every future choice has been made. The human decides when discussion is sufficient; a handoff does not itself authorize implementation.
