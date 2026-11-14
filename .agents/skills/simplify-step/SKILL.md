---
name: simplify-step
description: Commit approved changes (if any) and execute the next single atomic commit from SIMPLIFY_G.md (Part IV §3, Commits 1–23) with human review after every step.
---

# Simplify Step Workflow

We are executing `SIMPLIFY_G.md` (Part IV §3, Commits 1–23) one atomic commit at a time with human review after every step.

## Preparation

Before acting, gather the current repository state and plan context:
1. Run `git status -s` and `git log -n 5 --oneline` to inspect uncommitted working-tree changes and recent commits.
2. Read `SIMPLIFY_G.md` (Part IV §3 and Part IV §4).
3. Check the user's message in the current turn for any specific instructions or requested revisions to the previous step.

## Rules for Each Turn

1. If there are uncommitted working-tree changes from the previous step (ignoring `.gemini/` or `.agent/`) and the user did not ask for revisions, stage only those step files and commit them first using the exact commit title from `SIMPLIFY_G.md`.
2. Identify the next unexecuted commit `N` in `SIMPLIFY_G.md` (Part IV §3) and check Part IV §4 to see if `AGENTS.md`, `docs/WEB.md`, or `.agents/skills/ingest-frame/SKILL.md` must be updated in the same commit.
3. Implement **ONLY** Commit `N`. Do **NOT** start Commit `N+1`.
4. Run `make pre-commit` and ensure zero warnings/failures (check that `go fix` did not alter published frame chart numbers per `AGENTS.md`).
5. Stop with the changes for Commit `N` in the working tree (do not commit Commit `N` yet). Summarize the files changed, key verification results, and the exact commit message ready for approval.
