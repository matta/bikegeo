# Multi-Agent Design Protocol (`Gemini` ↔ `ChatGPT` + Human Operator)

## 1. Purpose & Roles

This directory hosts an asynchronous, filesystem-and-Git-mediated design conversation between **Gemini** and **ChatGPT**, steered by the **Human Operator (Matt)**.

The goal is to converge on a single, clear, actionable design intent for the clean-slate `bikegeo` reboot without falling into premature over-engineering or endless agent back-and-forth.

### Role Boundaries
- **Human Operator (Matt) — Decision Maker:**
  - Holds sole authority over significant architectural choices (programming language, UI/visualization stack, physical modeling scope, data storage, and trade-offs between simplicity and generality).
  - Prompts whichever agent is up next, answers open questions in the Decision Queue, resolves disagreements, and declares when a topic is settled.
- **Gemini & ChatGPT — Advisors, Synthesizers, and Reviewers:**
  - Act as sources of concrete ideas, illustrative code sketches, critical review, and trade-off analysis.
  - Actively look for flaws, hidden complexity, or readability traps in both their own and the other agent's proposals.
  - **Defer to the human:** Neither agent may unilaterally lock in a major architectural choice or build a deep specification on top of an unconfirmed assumption. When a major fork arises, frame the options clearly in the **Decision Queue** and wait for the human's ruling.

---

## 2. Shared Filesystem State (`docs/reboot/`)

All collaboration happens across three files in `docs/reboot/`:

1. **`docs/reboot/README.md`** *(this file)*
   - The directory entrypoint: rules of engagement, file structure, turn lifecycle, and commit conventions.
2. **`docs/reboot/DESIGN.md`** *(the living canonical design intent)*
   - The single source of truth that the agents are working to converge on. It is organized into four strict sections:
     1. **Human-Confirmed Decisions**: Architectural choices explicitly approved by the human. Agents must treat these as fixed constraints unless the human reopens them.
     2. **Converged Proposals (Pending Human Sign-off)**: Areas where both agents agree on a specific approach and recommend human approval.
     3. **Active Design Options (Under Discussion)**: Competing ideas or API shapes currently being compared, with brief pros/cons and small code sketches.
     4. **Decision Queue (For the Human Operator)**: A numbered list of crisp, high-leverage questions where the agents need the human to pick a direction before designing further.
3. **`docs/reboot/DIALOGUE.md`** *(the turn-by-turn conversation log)*
   - An append-only chronological log of agent turns (`## Turn 1 — Gemini`, `## Turn 2 — ChatGPT`, etc.).
   - Holds the critique, counter-proposals, and reasoning behind changes made to `DESIGN.md` so neither agent overwrites the other's rationale in silence.

---

## 3. Turn Lifecycle (Step-by-Step)

Whenever the human invokes either **Gemini** or **ChatGPT** for a turn:

### Step 1: Sync Context from Git & Disk
1. Run `git status -s` and `git log -n 10 --oneline -- docs/reboot/` to see recent turns and check for any uncommitted edits made directly by the human.
2. Read `docs/reboot/DESIGN.md` and the latest turns in `docs/reboot/DIALOGUE.md`.
3. If the human leaves uncommitted edits in `docs/reboot/` (for example, inline answers in `DESIGN.md`), incorporate and preserve them as part of the turn.

### Step 2: Process Human Guidance First
1. Check whether the human operator provided rulings or feedback—either in the chat prompt invoking the turn, or via edits to `DESIGN.md` / `DIALOGUE.md`.
2. Move any answered items from the **Decision Queue** into **Human-Confirmed Decisions** in `DESIGN.md`, recording the turn number.

### Step 3: Draft the Turn in `DIALOGUE.md`
Append a new section to `docs/reboot/DIALOGUE.md` using this template:

```markdown
## Turn <N> — <Gemini | ChatGPT> (<YYYY-MM-DD>)

### 1. Human Guidance Applied
- Bullet summary of any human decisions or steering applied at the start of this turn (or "None this turn").

### 2. Points of Convergence
- Explicitly name what you agree with from the previous turn so settled points stop being debated.

### 3. Critique, Ideas & Code Sketches
- Constructive review of the other agent's proposals.
- Concrete counter-proposals or refinements (use 15–30 line code sketches for APIs so readability can be judged by eye).
- Call out any complexity creep or unnecessary abstraction.

### 4. Proposed Rulings for the Human
- Reference the numbered items added or updated in the `DESIGN.md` Decision Queue.
```

### Step 4: Update `docs/reboot/DESIGN.md`
Update `DESIGN.md` so it reflects the latest state of convergence:
- Promote newly agreed points to **Converged Proposals**.
- Refine **Active Design Options** so both agents' best variants are represented side-by-side when a trade-off remains open.
- Keep the **Decision Queue** short (at most 3–5 active questions at a time) and phrased as concrete choices rather than open-ended essays.

### Step 5: Commit the Turn to Git
Stage `docs/reboot/` and create a single atomic commit for the turn using this commit message format:

```text
reboot(turn-<N>/<agent>): <concise summary of turn>
```

Examples:
- `reboot(turn-1/gemini): initialize reboot protocol and seed Turn 1 proposal`
- `reboot(turn-2/chatgpt): critique turtle scene-graph API and solver boundaries`

### Step 6: Report Back to the Human
In the chat response to the human operator, keep the summary brief:
1. State what converged in this turn.
2. Highlight the top questions in the **Decision Queue** that would benefit from the human's ruling before the next agent takes a turn.

---

## 4. Behavioral Guardrails (Lessons from `bikegeo`)

Both agents must enforce these guardrails on themselves and on each other:

1. **No Complexity Escalation ("One-Upmanship"):**
   In multi-agent reviews, agents often try to add value by inventing edge cases and proposing heavier type machinery (unusable zero values, phantom stage types, compile-negative harnesses, or general CAD frameworks). **Push in the opposite direction:** earn your keep by showing how a concept can be expressed with *fewer* types and *clearer* physical steps.
2. **Optimize for Local Visual & Code Inspection:**
   Remember Requirement (a): the human operator wants to verify geometry by reading high-level physical steps (like a turtle/pose recipe) and seeing the resulting structure graphically—not by auditing trigonometry. Whenever proposing an API, show how a real bicycle assembly step reads in code.
3. **Never Speculate Past an Unresolved Fork:**
   If a design choice depends on a question in the **Decision Queue** (e.g., whether the core runs in Go with an embedded web canvas vs. all-TypeScript in the browser), do not write 200 lines assuming one answer. Sketch both options briefly, state your recommendation, and wait for the human.
4. **Preserve Proven Domain Truths:**
   Unless the human explicitly decides to change the physical model, preserve the hard-won domain insights from `bikegeo`:
   - Published manufacturer charts are sacred ground truth with rounding budgets, not numbers to nudge.
   - `StemClamp` vs. `HandGrip` and `BRP` vs. `SitBones` are distinct physical landmarks.
   - Rear cockpit (seat tube → post → rail → saddle) and front cockpit (steerer → headset/spacers → stem → handlebar) sit on the leveled frame.
5. **Explicit Convergence Criteria:**
   The conversation is complete when:
   - Every requirement (`a` through `e`) has a **Human-Confirmed Decision** in `DESIGN.md` covering its architecture, core data types, and verification strategy.
   - The **Decision Queue** is empty.
   - `DESIGN.md` contains a concrete milestone sequence ready for implementation.
