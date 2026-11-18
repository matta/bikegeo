# Reboot implementation and transition plan

**Status: Confirmed by Matt (Turn 6, resolves Q5).** The product design and four milestones in [DESIGN.md](DESIGN.md) are confirmed. This document defines how to organize, build, verify, and transition the implementation.

## 1. Confirmed arrangement

Build the reboot as a separate application inside the existing repository and Go module. Leave the existing implementation at its current paths. Isolate new production code under `internal/reboot/`, reached through `cmd/bikegeo-reboot` and a temporary executable named `bikegeo-reboot`.

```text
go.mod                         one existing module and pinned toolchain
cmd/
  bikegeo/main.go              existing application
  bikegeo-reboot/main.go       reboot entrypoint
internal/
  assembly/, geometry/, ...   existing application, left in place
  reboot/
    pose/                     poses, units, geometric primitives, optional trace
    assembly/                 component specs, placements, whole-bike construction
    web/                      HTTP handlers, Go HTML/SVG rendering, embedded CSS/JS,
                              and the Otso Fenrir Stainless Milestone 1 fixture
docs/reboot/
  README.md                   collaboration protocol
  DESIGN.md                   approved product design and remaining decisions
  IMPLEMENTATION.md           this confirmed execution/transition plan
  DIALOGUE.md                 decision history and discussion notes
```

This is a starting layout, not a requirement to create empty directories. Add packages and APIs only as a working slice needs them. Keep a component's setup and placement types with its assembly code rather than recreating the old all-purpose `model` package. The web fixture is application input; assembly must not import it or know about HTTP.

Use a reboot-owned named degree type in `pose` with the existing degree/radian conversion conventions. This isolation deliberately replaces the old `model.Deg` import requirement for the new tree; existing packages retain their current type. Do not pull the old model package into the reboot solely for a unit type.

The initial dependency direction is `cmd/bikegeo-reboot → reboot/web → reboot/assembly → reboot/pose`. Web rendering can also use `reboot/pose`. Later search/catalog/agent packages belong under the same reboot root and call the same assembly implementation; choose their exact boundaries when their milestones arrive.

### Why isolate the new code rather than move the old code?

- Moving current packages to `internal/legacy/` would change their imports, tests, and references before providing any new capability. The existing program is useful as a working reference throughout the reboot.
- A single executable linking both implementations would invite temporary dispatch flags, shared types, and adapters. Separate executables make the runtime boundary observable and make side-by-side use straightforward.
- Keeping one module preserves the current dependency/toolchain policy and repository-wide Go checks. No nested `go.mod`, `go.work`, or build tags are needed to choose an implementation.
- Go's `internal` visibility alone does not enforce separation between these two trees in the same repository. Enforce the intended boundary with the pre-commit shell/regex and `go list` checks described in §4.

## 2. What “greenfield” means here

The new runtime architecture and ordinary unit test suite are completely independent; the accumulated domain knowledge remains input to the work.

| Material | Treatment |
| --- | --- |
| Existing production packages and types | **Zero imports** from reboot production code or ordinary `*_test.go` files (`model`, `geometry`, `geom2d`, `assembly`, `frames`, `catalog`, `ideal`, `scoring`, etc.). No reverse imports from old packages into reboot packages. |
| Transitional parity tests (`*_parity_test.go`) | **Only files named `*_parity_test.go`** under `internal/reboot/...` may import legacy `bikegeo/internal/...` packages for transitional side-by-side comparisons. All regular `*_test.go` files in `internal/reboot/...` must remain 100% self-contained so deleting `*_parity_test.go` and the old tree at cutover leaves the reboot test suite intact. |
| Small useful algorithms or geometric primitives | Read, audit, and port only what is needed into reboot-owned packages. Preserve appropriate independent examples and conventions. Do not wrap an old implementation behind a new name. |
| Milestone 1 bike fixture (**Otso Fenrir Stainless**) | Copy the sourced **Otso Fenrir Stainless** (`otso-fenrir-stainless`) build fixture into the reboot with its published chart numbers, CAD rigid fork baseline (`495 mm` Lithic carbon rigid fork, `54 mm` rake, `0 mm` CAD lower cup), physical `12 mm` EC44 lower cup, and rigid/suspension fork options (`100 mm` SID @ `506 mm` A2C / `44 mm` rake; `120 mm` Fox 34 @ `530 mm` A2C / `44 mm` rake), paired with Matt's cockpit fixture (SQlab 610 Ergolux 2.1 saddle and Tumbleweed Persuader handlebar). Derive all placements through the new model; do not copy precomputed frame endpoints or import the old `frames`/`catalog` packages. |
| Rider-fit facts and physical conventions | Preserve the established meaning and provenance in `docs/RIDER_FIT.md` and the relevant domain documentation. Record any deliberate model changes separately. |
| Later catalog migration | After Q4 selects the format, explicitly convert/import source observations and provenance into the reboot catalog. Review migrated values; never silently adjust published geometry to make the new engine agree. |

Deliberate duplication of a small audited primitive or fixture is acceptable during this transition. A shared runtime utility package extracted merely to avoid that duplication would weaken the boundary. Cross-implementation comparisons belong only in `*_parity_test.go` files (or a deliberately scoped migration tool), never in production files or standard `*_test.go` files.

The existing application remains usable; ordinary fixes to it can continue when requested. A reboot task should not opportunistically refactor it. Shared build/configuration changes must continue to work for both applications.

## 3. Executables and development commands

Keep `cmd/bikegeo` and `bin/bikegeo` as the existing application during development. Add `cmd/bikegeo-reboot` and `bin/bikegeo-reboot` for the reboot. The latter initially serves the workbench through `serve`; it is not a compatibility wrapper for the old CLI.

Proposed commands, to be implemented in the first slice:

```sh
go run ./cmd/bikegeo serve --port 8080
go run ./cmd/bikegeo-reboot serve --port 8081
go build -o bin/bikegeo-reboot ./cmd/bikegeo-reboot
```

Add `make build-reboot` and `make serve-reboot` without changing existing `build`, `run`, or `serve` behavior. Give the reboot server a different default port (`8081`), permit an explicit port, and visibly label its workbench so the two versions cannot be confused. A tiny command dispatcher using the standard library is sufficient; do not reuse `internal/cli` or add command scaffolding for future features.

`bikegeo-reboot` is a development name, not a permanent second product. No change to the main executable's behavior occurs merely because a reboot milestone finishes.

## 4. Verification and pre-commit boundary enforcement

- Continue the repository's code-change loop: `go vet ./... && go test ./...` after code modifications, and the full `make pre-commit` before completing or committing code work. Markdown-only design discussions keep their existing exemption.
- **Explicit Pre-Commit Import Boundary Check (`make boundary`, included in `make check` / `make pre-commit`):**
  Enforce the import rules across all source and test files using shell + regex checks alongside `go list -deps`:
  1. **No legacy imports in reboot production or standard `*_test.go` files (only `*_parity_test.go` allowed):**
     ```sh
     # Reject any "bikegeo/..." import outside "bikegeo/internal/reboot/..." or "bikegeo/cmd/bikegeo-reboot/..."
     # in any reboot .go file except *_parity_test.go:
     bad="$$(find cmd/bikegeo-reboot internal/reboot -type f -name '*.go' ! -name '*_parity_test.go' \
       -exec grep -HnE '"bikegeo/' {} + | grep -vE '"bikegeo/(internal/reboot|cmd/bikegeo-reboot)(/|")' || true)"; \
     if [ -n "$$bad" ]; then \
       echo "$$bad"; \
       echo "boundary: reboot non-parity files must not import legacy bikegeo packages"; \
       exit 1; \
     fi
     ```
  2. **No reboot imports anywhere in legacy packages:**
     ```sh
     bad="$$(find cmd/bikegeo internal -path internal/reboot -prune -o -type f -name '*.go' \
       -exec grep -HnE '"bikegeo/(internal/reboot|cmd/bikegeo-reboot)(/|")' {} + || true)"; \
     if [ -n "$$bad" ]; then \
       echo "$$bad"; \
       echo "boundary: legacy packages must not import reboot packages"; \
       exit 1; \
     fi
     ```
  3. **Transitive production dependency check (`go list -deps`):**
     Verify that `./cmd/bikegeo-reboot` transitively depends only on the Go standard library and `bikegeo/{cmd/bikegeo-reboot,internal/reboot/...}`, and `./cmd/bikegeo` depends on zero `bikegeo/internal/reboot/...` packages.
- **Dead-code analysis (`make deadcode`):**
  Run `go tool deadcode ./...` and `go tool deadcode -test ./...` across both application entrypoints. Build vertical slices so every new non-test function in `internal/reboot/...` is reachable from `cmd/bikegeo-reboot/main.go` on every commit.
- Adapt independent component examples, check dimensions and leveling invariants, and verify identical placements with tracing on/off. A comparison with old output in `*_parity_test.go` must explain differences rather than automatically declaring the old result correct.
- Go handler/template tests cover generated output and trace/diagram correspondence. Check the real browser interactions as well, initially manually if appropriate. No new browser-test framework is required by this plan.

The reboot retains the existing zero-runtime-dependency rule. Existing test/tool dependencies remain allowed in their appropriate roles; sharing `go.mod` does not make them production dependencies of either executable.

## 5. Milestone 1 in reviewable implementation slices

Each slice produces a runnable visual result plus the tests needed for its behavior. These are proposed work units, not promises of exact line counts or immutable commit sizes. The sequence expands to the full chassis and cockpit scope already approved in Turn 5.

| Slice | Work | Visible result and acceptance evidence |
| --- | --- | --- |
| 1. Separate runnable workbench | Add the new entrypoint, build targets, and `make boundary` pre-commit checks. Implement only the pose/trace operations needed for one stem recipe, Go-rendered SVG, and a simple construction table. | `bikegeo-reboot serve` shows a stem with its construction; old `bikegeo` still runs independently. Independent stem dimensions, trace-on/off equivalence, and `make pre-commit` (including `boundary` and `deadcode`) pass. |
| 2. Frame, fork, and leveling (**Otso Fenrir**) | Add the sourced **Otso Fenrir Stainless** fixture (`fixture.go`); construct the chassis from published chart dimensions and its declared CAD rigid fork baseline (`495 mm` / `54 mm` / `0 mm` lower cup), install the selected fork/cup (e.g., `12 mm` EC44 lower cup and rigid or `100 mm`/`120 mm` suspension fork with sag), and trace the attitude change and BB recentering. | A whole chassis drawing responds to fork/cup/sag/rake changes. Wheel/ground relationships, rigid dimensions, baseline-versus-installed setup, and coordinate conventions are checked. No hardcoded pre-leveled endpoints. |
| 3. Complete component assembly | Add seatpost/saddle (SQlab 610) and handlebar (Tumbleweed Persuader) construction on the Otso Fenrir, using the agreed pose landmarks and explicit orientation meanings. Preserve the lateral handlebar computation behind the side projection. | The one-bike drawing includes both cockpits and contact landmarks; independent rear/front examples and full-build tracing equivalence pass. |
| 4. Controls and inspection | Complete input controls and Go validation; add fetched updates, linked trace/diagram selection, keyboard access, URL/history handling, and pan/zoom with dependency-free JavaScript. | Matt can change the approved settings, inspect the responsible steps, and navigate without stale responses or mismatched trace/diagram state. Handler tests and browser checks cover the flow. |
| 5. Milestone acceptance | Review the full Otso Fenrir fixture and construction with Matt, address issues, and document how to run and inspect it. Recheck both applications and their import boundaries. | Matt can read the recipes, predict changes, and verify them in the diagram and trace. All Turn 5 Milestone 1 criteria are satisfied. The new workbench still has the temporary binary name. |

The chart fixture may need component dimensions beyond the published frame chart. Label measured values, catalog values, and deliberate illustrative assumptions; do not invent a complete physical outline and present it as manufacturer ground truth. Settle fixture-specific measurement conventions before relying on them in acceptance tests.

Milestones 2–4 remain in their confirmed order. Add fit search, chart reconstruction/persistence/agent operations, and frame comparison inside the reboot tree. Refine their implementation slices when those milestones are next; the new plan does not settle their deferred algorithms or storage format.

## 6. Switching over and retiring the old program

Do not move old packages into `internal/legacy/` now, or keep both implementations linked behind a mode switch. When the reboot meets the uses Matt needs and he chooses to switch:

1. Inventory the old commands, datasets, URLs, and workflows Matt still relies on. Explicitly replace or retire each relevant capability; reaching Milestone 1 alone is not parity, and preserving every old CLI command is not assumed.
2. Ensure published datasets, provenance, and useful independent test cases are retained or intentionally migrated. Delete any transitional `*_parity_test.go` files before deleting their referenced legacy packages.
3. Mark a known-good old revision with a Git tag so the prior application remains reproducible from history.
4. In a reviewed cutover change, retire the old production packages and entrypoint, promote the reboot entrypoint to `cmd/bikegeo`, and rename its output to `bin/bikegeo`. Move reboot packages from `internal/reboot/<package>` to their permanent `internal/<package>` locations once old-name collisions are removed. Remove the temporary `bikegeo-reboot` command and adjust imports, tests, build targets, dependency checks, and run documentation together.
5. Run the full required verification and check the preserved workflows. Git history/tag provides the old implementation; there is no permanent second engine shipped in the new binary.

This cutover is a separate decision and implementation task. Until then, both runnable applications coexist, with independent production package trees and the old default command unchanged.
