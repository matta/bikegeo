# Slice 20: Docs Consolidation

Status: Implemented 2026-11-22; awaiting Matt's acceptance
Implementer: Low (fast/lower-tier) — mechanical doc moves, path updates, and retiring legacy-only rules; only two comment lines change in Go.
Review: optional — documentation consolidation and two Go comments; no behavior, geometry, or scoring changes.

## Implementation handoff

- **Who & when:** Claude Sonnet 5.5, 2026-11-22.
- **Commits:** `1044117` (move `docs/reboot/*` to `docs/`); the consolidation commit follows it.
- **Deviations:** `RIDER_FIT.md` points to `internal/catalog/catalog.go` (`DefaultGripTarget`, `DefaultBrpTarget`) because `internal/catalog/defaults.go` doesn't exist. The AGENTS.md `Interfaces` and `Comparison columns` legacy bullets are replaced by a short `CLI` bullet.
- **Verification:** `make pre-commit` passes; no stale `docs/reboot/`, `internal/ideal`, `internal/frames`, or `bikegeo-reboot` references remain outside historical `Q#`/Milestones text and `legacy_*` notes; relative links resolve.
- **Open issues:** the Slice 19 CI workflow is still unverified until the first push.

## Goal

Consolidate repository documentation after the Slice 18–19 cutover so `docs/` holds the single set of active docs (removing `docs/reboot/`), `AGENTS.md` describes only the current app, stale legacy links and catalog counts are refreshed, and all `docs/reboot/` path references are updated.

## Confirmed rulings

- **Q115 / Q116 / Q147:** Legacy is deleted (`legacy-final` tag), `bikegeo` is promoted to `cmd/bikegeo` and `internal/...` on port `8080`, and UI overhauls (Milestone 5) follow cutover. Historical `Q#` rulings and completed Milestones entries keep their old names/paths as history.
- **Q144:** `make deadcode` is run by hand and never part of `make check` or `make pre-commit`.

## Key decisions & planner defaults

- **Move all of `docs/reboot/` to `docs/`:** Move `DESIGN.md`, `WORKFLOW.md`, `stems.yaml`, and this active plan (`SLICE20.md`) from `docs/reboot/` to `docs/` via `git mv`, removing the `docs/reboot/` directory. *(Planner default: `stems.yaml` moves to `docs/stems.yaml` alongside the other reference docs rather than `internal/catalog/`; veto on review if you prefer `internal/catalog/stems.yaml`.)*
- **Update active `docs/reboot/` references:** Point `README.md`, `AGENTS.md`, `docs/WORKFLOW.md` (including its §4 copy-paste prompts and `docs/SLICE*.md` paths), `docs/DESIGN.md` (active sections only), `internal/catalog/legacy_frame_notes.md`, and the two comments in `internal/catalog/stems.go` to `docs/...`. Keep historical `Q#` rulings and completed Milestones entries in `docs/DESIGN.md` untouched per Q147.
- **Consolidate `AGENTS.md`:**
  - Fold the active rules from `## Reboot Tree` (workflow pointer, Web 1.0, `pose.Deg` and trig-in-`pose`, `internal/catalog`, terminology, `make boundary` banned words) into `## Architecture` (and a brief `## Workflow` or bullet), and delete the `## Reboot Tree` section and its dual-app precedence/isolation prose.
  - Drop retired legacy `## Architecture` bullets (`internal/frames`, `internal/ideal/loader.go`, `compare.DefaultComparisonState()`, `report.Document` Markdown CLI, points-lost ledger scoring, `+spec`/`-spec`, `internal/canonicalizer`, `model.Deg`, `bikegeo fit` tied candidates) and replace them with the current app's Go-struct catalog, JSON CLI (`bikegeo compare` / `bikegeo bike` via `internal/query`), and Web 1.0 architecture.
  - Keep a concise `## Frame Datasets` section (referenced by `CLAUDE.md`) describing `internal/catalog`, the `ingest-frame` skill, `frame_<id>.md`, and chart-consistency/allowance rules, without "Legacy `internal/frames` is frozen:".
  - Update `## Testing` to drop the legacy `runCLIRows` bullet and keep browserless web tests (`httptest` + `goquery`) and JSON CLI tests.
- **Clean up `docs/DESIGN.md` and `docs/WORKFLOW.md`:**
  - Drop `Reboot` from the top-level titles/intros of both files.
  - In `docs/DESIGN.md`, drop `### Precedence over legacy specs` and the legacy-porting isolation bullet under `## Layout and boundaries`.
  - Refresh stale active sections in `docs/DESIGN.md`: `## Requirements` (`docs/stems.yaml`, "all catalog frames"), `## Run and inspect` (`fork=basis`, current counts: 18 frames, 16 forks, 15 handlebars, 4 seatposts, 107 stems from `docs/stems.yaml`, SQlab 610 saddle, default fit targets), and `## Layout and boundaries` (current counts and drop completed-slice future-tense parentheticals).
- **Fix legacy references in `docs/RIDER_FIT.md` and `.agents/skills/ingest-frame/SKILL.md`:**
  - In `docs/RIDER_FIT.md`, replace the two broken `../internal/ideal/readme.md` links (deleted in Slice 18a) with pointers to `DESIGN.md` (coordinate conventions and saddle/cockpit placement) and `internal/catalog/defaults.go`.
  - `docs/SQLAB_SADDLE.md` has no broken links or legacy paths; leave unchanged.
  - In `.agents/skills/ingest-frame/SKILL.md`, change `reboot catalog` in the description to `catalog`.

## Rough commit split

1. **Move `docs/reboot/*` to `docs/`:** pure `git mv` of `DESIGN.md`, `WORKFLOW.md`, `stems.yaml`, and `SLICE20.md` from `docs/reboot/` to `docs/` so git preserves rename history cleanly.
2. **Consolidate docs and refresh references:** edit `AGENTS.md`, `docs/DESIGN.md`, `docs/WORKFLOW.md`, `docs/RIDER_FIT.md`, `README.md`, `internal/catalog/legacy_frame_notes.md`, `.agents/skills/ingest-frame/SKILL.md`, and the two `docs/stems.yaml` comments in `internal/catalog/stems.go`. Update `docs/SLICE20.md` `Status:` and add the implementation handoff in this commit.

## Verification

- `make pre-commit` passes (required because `internal/catalog/stems.go` comments are edited).
- Repository-wide search confirms zero remaining active references to `docs/reboot/`, `internal/ideal/readme.md`, `internal/frames`, or `bikegeo-reboot` outside historical `Q#` rulings / completed Milestones entries in `docs/DESIGN.md` and historical context in `internal/catalog/legacy_frame_notes.md`.
- Relative Markdown links in `AGENTS.md`, `README.md`, `docs/DESIGN.md`, `docs/WORKFLOW.md`, and `docs/RIDER_FIT.md` resolve to existing files.

## Out of scope

- Any Go logic, catalog data, template, or test changes beyond the two comment lines in `internal/catalog/stems.go`.
- Editing historical `Q#` rulings or completed Milestones entries in `docs/DESIGN.md` (preserved as history per Q147).
- Verifying the Slice 19 CI workflow change (remains unverified until the next push).
- Milestone 5 UI overhauls (Slice 21+).
