---
name: simplify-step
description: Commit approved changes (if any) and execute the next single atomic commit from SIMPLIFY_G.md (Part IV §3, Commits 1–23) with human review after every step.
---

# Simplify Step Workflow

We are executing `SIMPLIFY_G.md` (Part IV §3, Commits 1–23) one atomic commit at a time with human review after every step.

## Critical Ordering Rule: Commit Prior Step Before Any Investigation

**NEVER** read, grep, or investigate codebase files for the next step (`N`) while approved changes from the prior step (`N-1`) are still uncommitted in the working tree.

### Step 1: Check State & Commit the Approved Prior Step First

1. In a single initial tool batch, run `git status -s && git log -n 5 --oneline` and read `SIMPLIFY_G.md` lines 505–585 (Part IV §3 and §4). Do **not** read or search any other files yet.
2. Check the user's message for any requested revisions to the previous step.
3. If there are uncommitted working-tree changes from the previous step (ignoring `.gemini/`, `.agent/`, or `.agents/`) and the user did not ask for revisions:
   - **If Plan Mode is active**: call `exit_plan_mode` immediately after Step 1.1 without investigating the codebase for Commit `N` first. Hand off a concise plan where Step 1 commits the prior step (`N-1`) and Step 2 investigates and implements Commit `N`.
   - **Stage and commit immediately**: stage only the files belonging to the prior step (`N-1`) and commit them using the exact commit title from `SIMPLIFY_G.md` (`BypassSandbox: true` is required for `git add` / `git commit` to write `.git/index.lock`).
   - Do **not** proceed to Step 2 until `git commit` for the prior step has succeeded.

### Step 2: Investigate, Implement, and Verify the Next Commit (`N`)

1. Once the prior step is committed (or if the working tree had no prior-step changes), identify the next unexecuted commit `N` in `SIMPLIFY_G.md` (Part IV §3) and check Part IV §4 to see if `AGENTS.md`, `docs/WEB.md`, or `.agents/skills/ingest-frame/SKILL.md` must be updated in the same commit.
2. Now investigate the relevant files and implement **ONLY** Commit `N`. Do **NOT** start Commit `N+1`.
3. Run `make pre-commit` and ensure zero warnings/failures (check that `go fix` did not alter published frame chart numbers per `AGENTS.md`).
4. Stop with the changes for Commit `N` in the working tree (do not commit Commit `N` yet). Summarize the files changed, key verification results, and the exact commit message ready for approval.
