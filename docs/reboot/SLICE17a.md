# Slice 17a: `deadcode` leaves the presubmit gate

Status: Implemented 2026-11-22; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-22, Claude Opus 5.5 (planning agent, implementing the Small slice itself).
- **Commits:** `Take deadcode out of the presubmit gate` (the commit that carries this handoff). The plan was committed before it as `b4000d6`.
- **Done and not done:** C1 and C2 done. Nothing left undone.
- **Deviations:** None.
- **Verification:**
  1. The grep in Verification step 1 prints nothing.
  2. `make pre-commit` exits 0 with 21 packages `ok`, and its log has no `deadcode` line.
  3. `make deadcode` exits non-zero, reporting exactly `FitOval.Validate` (`assembly/scoring.go:107`), `RigidClass.ValidFor` (`catalog/forks.go:41`), and `ForkByID` (`catalog/forks.go:205`) under `unreachable functions from main`, as expected.
  
  There's no live check: nothing user-visible changed.
- **Open issues:** None.

Small slice: one commit and 2 files (`Makefile`, `AGENTS.md`). DESIGN.md's active rules were updated when Q144 was recorded, so this slice only touches DESIGN.md's Slice 17a status line.

## Goal

Make permanent what is a temporary gap today: `make check` (and so `make pre-commit`, and CI once Slice 19 points it at `make check`) doesn't run `deadcode`. `make deadcode` stays, unchanged, as the only way to run it, by hand and periodically. Remove every "temporarily disabled / re-enabled at cutover" phrase.

## Rulings

- **Q144:** `deadcode` isn't a presubmit gate, before or after cutover. Its findings don't block a commit, and today's three stay: `assembly.FitOval.Validate`, `catalog.RigidClass.ValidFor`, and `catalog.ForkByID` (measured 2026-11-22, all unreachable from `main`, none with `-test`; legacy reports none).

## Changes

### C1. `Makefile`

Replace the two-line comment above `check:` (lines 96–97):

```make
# Temporarily omit `deadcode` during the reboot so slices can land foundational
# functions before every caller exists; re-enable in `check` at cutover.
check: vet boundary exhaustive test
```

with:

```make
# `deadcode` is deliberately not part of `check` (Q144): run `make deadcode` by
# hand, periodically. Its findings don't block a commit.
check: vet boundary exhaustive test
```

The `check:` recipe line, the `deadcode:` target and its comment, and `.PHONY` don't change.

### C2. `AGENTS.md`

1. **Verification** bullet (line 24). Replace the parenthetical

   `` (vet, `boundary`, `exhaustive`, and Go tests; `deadcode` is temporarily disabled during the reboot and will be re-enabled at cutover) ``

   with

   `` (vet, `boundary`, `exhaustive`, and Go tests; not `deadcode`, which runs only by hand) ``

2. **Architecture → `deadcode`** bullet (line 59). Replace the whole bullet with:

   ``- **`deadcode`:** not part of `make check` or `make pre-commit` (Q144). Run `make deadcode` by hand, periodically; its findings don't block a commit. When you choose to clean one up, delete the unreachable function or move a test-only helper into a `*_test.go` file.``

Leave the **Zero runtime dependencies** bullet (line 57) alone: its `deadcode` mention is about `go.mod` tool dependencies and is still true.

## Out of scope

- Fixing or deleting the three reported functions (Q144).
- Any Go code or test change.
- `.github/workflows/test.yml` (Slice 19 owns CI).
- The parity audit (Slice 17b).

## Verification

1. `grep -n -i "temporarily\|re-enable\|cutover" AGENTS.md Makefile | grep -i deadcode` prints nothing. (DESIGN.md's Q144 and Slice 17a text mention re-enabling on purpose, so don't grep it.)
2. `make pre-commit` passes and its output shows no `deadcode` step.
3. `make deadcode` still runs and still fails, reporting exactly the three functions above under `unreachable functions from main` (and exits 1 before the `-test` pass). That's the expected state, not a regression.

## Rules that bite

- Change only the lines named above. Don't reflow nearby text.
- Don't touch DESIGN.md beyond the Slice 17a bullet's status words.
- Commit subject: `Take deadcode out of the presubmit gate`.
- Finish with the **Implementation handoff** (DESIGN.md, "Slice planning"): set this file's Status line and the Slice 17a bullet in DESIGN.md to `Implemented <date>; awaiting Matt's acceptance`, and add the `## Implementation handoff` section right after the Status paragraph.
