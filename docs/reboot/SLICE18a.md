# Slice 18a: tag and delete legacy

Status: Implemented 2026-11-22; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-22, Claude Sonnet 5.5.
- **Commits:** A1 tag `legacy-final` (lightweight, local, not pushed) on `c74f331`; A2–A4 `Delete the legacy parity tests` (`3f9e6c1`); B1–B6 `Delete the legacy app` (`001bfea`).
- **Done and not done:** all of A1–A4 and B1–B6 done.
- **Deviations:** the plan's B3 list didn't name the "legacy must not import reboot" boundary block as a separate deletion from the `go list -deps ./cmd/bikegeo` block; it did (three blocks), and both were removed. `AGENTS.md` line "Angles are `model.Deg`" in Architecture was left for Slice 20, as the plan only listed the Isolation bullet.
- **Verification:** `make pre-commit` passes after each commit. A3 negative test: a temporary reboot file importing `bikegeo/internal/model` failed `make boundary` with "must not import legacy"; removed. `go list ./...` lists only reboot packages; `go mod tidy -diff` is empty; `git ls-tree` on `legacy-final` shows 22 `internal/frames/` files; no legacy import hits; `make build`/`make run` fail with "No rule". Live check: `/` and `/bike` return 200; `bikegeo-reboot compare` JSON lists 19 frames. No screenshot taken.
- **Open issues:** `AGENTS.md` still describes legacy details (Architecture bullets on `model.Deg`, `internal/frames`, scores) until Slice 20.

Rulings it rests on: Q115 (retire every legacy-only workflow), Q116 (UI overhauls follow cutover), Q119 and Q145 (parity tests are the last link to legacy; Slice 17b already archived the legacy notes), Q144 (`deadcode` is not a gate). Tag-push choice (slice-local, Matt, 2026-11-22): **create the tag locally and never push it.**

Size: larger by file count, but mechanical. Two commits, each passing `make pre-commit`. A lower-tier model can implement it. There is no new design.

## Goal

Make the reboot the only app. Tag the last commit that carries legacy, then delete the legacy tree, its tests, its docs, its skill, and its Makefile and boundary plumbing. Reboot code and tests don't change, except that two parity test files are deleted. The reboot is still named `bikegeo-reboot` here; Slice 19 renames it.

## Current state (measured 2026-11-22)

- `go list ./...` shows 2 commands (`cmd/bikegeo`, `cmd/bikegeo-reboot`), 15 legacy packages (`internal/{assembly,canonicalizer,catalog,cli,compare,fit,frames,geom2d,geometry,ideal,model,report,scoring,validator,web}`), and the `internal/reboot/...` packages. The legacy side is 119 Go files, tests included.
- Only two reboot files import legacy: `internal/reboot/catalog/frames_parity_test.go` and `parts_parity_test.go`. No other reboot file or `cmd/bikegeo-reboot` does (`make boundary` enforces that today).
- `goquery` is used only by reboot tests plus legacy tests, so `go.mod` keeps it. `exhaustive` and `deadcode` stay as `tool` dependencies.
- `.claude/skills` is a symlink to `.agents/skills`. Deleting `.agents/skills/simplify-step` removes it from both.

## Commit A: tag, then delete the parity tests

Subject: `Delete the legacy parity tests`

1. **A1, tag.** Before editing, commit this plan and the DESIGN.md ruling edits (`Plan Slice 18a and record Q115-Q116`). Then run `git tag legacy-final` on that commit, the last commit that still has legacy and the parity tests. Annotated or lightweight is the implementer's call. Do **not** push the tag. Record its hash in the handoff.
2. **A2.** Delete `internal/reboot/catalog/frames_parity_test.go` and `internal/reboot/catalog/parts_parity_test.go`.
3. **A3.** Edit `Makefile` `boundary`:
   - Remove the first block's `! -name '*_parity_test.go'` exemption and the `grep -vE` pass-through. It becomes a check that no file under `cmd/bikegeo-reboot` or `internal/reboot` imports a path matching `"bikegeo/` outside `bikegeo/(internal/reboot|cmd/bikegeo-reboot)`. Keep it; it still guards against a stray import of a deleted package name coming back.
   - Remove the `'cad'` ban's `! -name '*_parity_test.go'` exemptions (two places).
   - Leave the other blocks for commit B.
4. Fix any test or comment that names the deleted tests. Run `grep -rn 'parity' internal/reboot cmd/bikegeo-reboot Makefile` and expect zero hits afterward.

**Negative test (A3):** temporarily add `import _ "bikegeo/internal/model"` to any non-test reboot file, run `make boundary`, and see it fail with the "must not import legacy" message. Remove the import. Don't commit it.

Verify: `make pre-commit` passes. The reboot test counts don't change except for the two removed files' tests.

## Commit B: delete legacy

Subject: `Delete the legacy app`

1. **B1, delete code.** `cmd/bikegeo` and the 15 legacy `internal/` packages listed above. After this, `go list ./...` shows only `bikegeo/cmd/bikegeo-reboot` and `bikegeo/internal/reboot/...`.
2. **B2, delete stale files.** `INTENT.md`, `docs/WEB.md`, `docs/kinematic/` (whole directory), `.agents/skills/simplify-step/` (whole directory).
3. **B3, `Makefile`.**
   - Delete the `build`, `run`, `serve`, and `watch-serve` targets and their `.PHONY` entries (`build`, `run`, `serve`, `watch-serve`).
   - `all: check build-reboot`. `serve-reboot` depends on `build-reboot` only. Keep `build-reboot`, `serve-reboot`, `watch`, `clean`, and `deadcode` as they are.
   - Edit `boundary`: delete the three blocks that exist only for legacy (legacy-must-not-import-reboot, `go list -deps ./cmd/bikegeo` must not contain reboot, and the one that scans `cmd/bikegeo internal -path internal/reboot -prune`). Keep the first block (A3), the third block (`go list -deps ./cmd/bikegeo-reboot` has only `bikegeo/cmd/bikegeo-reboot` and `bikegeo/internal/reboot` packages), and the `chassis`, score-word, and `cad` bans. Update the comment above `boundary` to say it enforces the reboot's zero-dependency rule and banned words, with no mention of a legacy tree.
4. **B4, `AGENTS.md`.** Fix only what became false or broken. Slice 20 does the real rewrite.
   - Reboot Tree, "Isolation" bullet: drop the parity-test exception and the "legacy code never imports reboot packages" sentence. Keep "Angles use `pose.Deg`, not `model.Deg`" reworded to "Angles use `pose.Deg`".
   - Architecture, "Zero runtime dependencies": change `./cmd/bikegeo` in the `go list -deps` command to `./cmd/bikegeo-reboot`.
   - Architecture, "Interfaces": remove the `(see [docs/WEB.md](docs/WEB.md))` link. The sentence keeps its rule.
5. **B5, `.agents/skills/ingest-frame/SKILL.md`.** Step 6's first bullet: delete the sentence "The legacy parity test skips the new frame (Q119)." Line 8's reference to frozen legacy `internal/frames` and `bikegeo fit`/`validate` becomes a one-line statement that legacy is gone (history lives at tag `legacy-final`). Leave every other step alone.
6. **B6, leave alone:** `README.md`, `docs/RIDER_FIT.md`, `docs/SQLAB_SADDLE.md`, `DESIGN.md` other than the Milestones line, `.github/workflows/test.yml`, `.claude/launch.json`, `go.mod`, `go.sum`. README, RIDER_FIT, and SQLAB_SADDLE are Slice 20. CI is Slice 19.

**Rules that bite**

- `go mod tidy` must leave `go.mod` and `go.sum` unchanged. If it removes anything, a reboot test imports it; stop and check.
- `make deadcode` is not a gate (Q144). Don't clean up its findings here.
- Don't touch any file under `internal/reboot` or `cmd/bikegeo-reboot` in commit B.
- The `cad` ban stays: `make boundary` now bans the word in every reboot file. If it fails because a file you didn't edit contains it, report it and don't rewrite catalog data.
- Never push the tag or any branch.
- **Implementation handoff:** the last step is the `## Implementation handoff` section and the Status and Milestones updates (see DESIGN.md "Implementation handoff").

## Verification

- After each commit: `make pre-commit` passes.
- `go list ./...` lists only `bikegeo/cmd/bikegeo-reboot` and `bikegeo/internal/reboot/...`.
- `go mod tidy -diff` prints nothing.
- `git tag -l legacy-final` shows the tag, and `git ls-tree -r --name-only legacy-final | grep -c '^internal/frames/'` is non-zero (legacy is still readable there).
- `grep -rIn 'bikegeo/internal/\(model\|frames\|compare\|web\)' --include='*.go' .` has no hits.
- Live check: `make serve-reboot`, then `/` and `/bike` load with the same frame count as before (19 frames), and `bikegeo-reboot compare` still prints JSON. No screenshot needed beyond one of `/`.
- `make build-reboot` produces `bin/bikegeo-reboot`; `make build` and `make run` now fail with "No rule".

## Out of scope

Renaming anything `reboot` (Slice 19), CI changes (Slice 19), README and the other docs rewrite (Slice 20), deleting the Ritchey Ultra (Slice 18b), and `deadcode` cleanup (Q144).
