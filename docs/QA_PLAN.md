# bikegeo QA plan

A stepwise plan for auditing the whole feature surface of `bikegeo` for three qualities:

- **Coherence** — similar features surface the same way everywhere (naming, layout, flags, errors, help).
- **Uniformity** — one concept has one definition, one vocabulary and one format wherever it appears.
- **Correctness** — every displayed number is right, and numbers that describe the same thing agree with each other.

It is written for agents. Each step has an ID, a procedure, and an **oracle** (how to decide pass or fail without trusting the code under test). Steps inside a phase are independent unless stated. Baseline surveyed: commit `759311c`.

---

## 0. Ground rules

1. **This is an audit, not a fix pass.** Do not change product code while executing a step. Record findings (section 9). Fixes are separate work, triaged by the owner.
2. **Read `AGENTS.md` first.** Its rules bind QA too. In particular: published chart numbers in `pkg/frames/*.go` are never edited, `GeoSlopMm` is never used to make a finding go away, and scoring code is never clamped or stepped.
3. **Never use the code under test as its own oracle.** An oracle is one of:
   - a closed-form recomputation you do independently (a scratch script outside the repo, or by hand);
   - an invariant or metamorphic relation (change input A, output B must move in a known direction or not at all);
   - agreement between two surfaces that are supposed to show the same thing;
   - the written spec (`README.md`, `AGENTS.md`, `docs/WEB.md`, CLI help, `/help`). When spec and behaviour disagree, that is a finding either way; do not decide which is wrong.
4. **A lead is not a finding.** Section 8 lists things noticed during the survey. Reproduce each before filing it.
5. **Regression tests follow the repo conventions**: `go test` only, `httptest` + `goquery` for web, `runCLIRows` for CLI cell values, no browser automation, no new non-test dependencies. Exploration may use `bikegeo serve` plus `curl`.
6. **Determinism.** Every command except `fit` (documented tie-order variance) must be byte-identical across two runs. Check this once per command in Phase 1 and treat any other variance as a finding.

### Setup (step S0)

```bash
go vet ./... && go test ./...
```

```bash
go build -o bin/bikegeo ./cmd/bikegeo
```

```bash
./bin/bikegeo serve --port 18731
```

Record the commit hash, `go version`, and the test result at the top of your findings file. If the suite is not green, stop and report.

---

## 1. Surface inventory (what must be covered)

### CLI (agents) — `pkg/cli`

| Command | Aliases | Flags | Builder |
| --- | --- | --- | --- |
| `compare [spec...]` | `cmp` | `--all`, `--all-configs`, `--top`, `--top-per-frame`, `--saddle`, `--handlebar`, `--transpose`, `--score-detail`, `--no-score` | `compare.Run` → `compare.BuildDocument` (`Flat`) |
| `validate [frame...]` | `val`, `check` | `--all`, `--size`, `--config` | `compare.ResolveValidationTargets` → `validator.BuildValidationDocument` |
| `list` | `ls`, `bikes`, `frames` | `-q`/`--search`, `--category`, `--size`, `--all-sizes` | `compare.BuildFrameListDocument` |
| `fit <frame[:size]>` | — | `--try` (repeatable), `--detail`, `--config` | `fit.BuildAnalysisDocument` / `fit.BuildSweepDocument` |
| `serve` | — | `--port`, `--open` | `web.RunServe` |
| `help` | `-h`, `--help`, no args | — | `printUsage` |

### Web (people) — `pkg/web`

| Route | Parameters | CLI counterpart |
| --- | --- | --- |
| `/` | `col`, `frame`/`size`/`fork`/`sag` (aligned), `saddle`, `handlebar`, `layout`, `score`, `all`, `top`, `top_per_frame` | `compare` |
| `/frames` | `q`, `category`, `size`, `all-sizes` | `list` |
| `/validate` | `frame`, `size` | `validate` |
| `/forks`, `/stems` | none | none |
| `/ideals` | `name` | none |
| `/saddles` | `id`, `saddle-height`, `sit-bone-setback` | none |
| `/saddle-swap` | `from`, `to`, `all`, `frame`, `saddle-height`, `sit-bone-setback`, `brp-setback`, `seatpost`, `clamp-length`, `cockpit-dist`, `cockpit-drop` | none |
| `/handlebars` | `id`, `compare`, `grip-inset`, `roll`, `width` | none |
| `/help` | none | none |
| `/static/…` | — | — |

### Spec grammar (shared by CLI positional args and web `col`)

`[+|-]frame[:size][@mod]...` where a mod is one of: travel (`120mm`, `120`), `rigid`/`0mm`, fork preset ID, `a2c=<mm>/<rake>`, sag (`20%`, `sag=25`, bare number < 60), `all`/`all-forks`/`forks`; `frame` may be an ID, a unique substring, or `all`; size may be `all`. Removal: `-N`, `-frame`, `-frame:size`, `-frame:size@fork`, `-all`.

### Data

19 frames (`pkg/frames`), 22 forks and 2 stem sets (`pkg/catalog/catalog.go`), 3 saddles, 15 handlebars, 1 ideal profile `idmatch` (18 attributes, 6 scored), 26 registered metrics (`pkg/ideal/registry.go`), the default baseline (20 columns), the default rider profile.

---

## 2. Concept registry (the uniformity yardstick)

Phase 2 audits each row: collect every place the concept is rendered, parsed or explained, and check all of them against the single rule on the right. Build the registry as a table in your findings file with one line per occurrence (`surface`, `location`, `exact text`).

| # | Concept | Uniformity rule to test |
| --- | --- | --- |
| C1 | Frame identity | One ID form, one display name form (`Manufacturer Model`). Every frame mention on the web links to the same target. Substring/ambiguity resolution identical in `compare`, `validate`, `fit`, `/`, `/validate`, `/saddle-swap`. |
| C2 | Size | Same matching rules (case, spaces such as `S 60cm`) and the same "unknown size" message in every command and page. Default-size marker (`*`) means only default size. |
| C3 | Fork | One label per fork state across compare matrix (both layouts), breakdown, column disambiguators, form selects, permalinks, validate, list, `/forks`. A fork that is the same physical thing reads the same no matter how it was specified (default, `@rigid`, preset ID, `a2c=`). |
| C4 | Sag | One allowed set, stated identically in errors, CLI help, `/help`, the form's select and `fit`. One rule for bare numbers. |
| C5 | A2C | Always clear which A2C is shown: static, sagged/effective, structural (with spacer), or CAD-implied. Same word for the same quantity in compare, validate, fit, list, forks. |
| C6 | Metric | One ID, one label, one unit, one decimal precision, one direction vocabulary (`fwd/aft`, `sht/lng`, `lo/hi`, `qck/slw`, `slk/stp`) per metric, in every table, footer sentence, glossary row and profile page. |
| C7 | Reference ranges | Ideal and Acceptable for a metric are the same text in the compare reference rows, `/help`, `/ideals?name=`, and any footer sentence. |
| C8 | Zone | `✓ ~ ✗` marker, `data-zone`, CSS class and sub-score always agree; the same thresholds decide a zone everywhere. Web-only pages that colour cells follow the same meaning. |
| C9 | Score | One stated range, one aggregation rule, one rounding rule; the same score for the same setup wherever it appears (matrix, breakdown, losses, head-to-head, both layouts, CLI and web). |
| C10 | Rank and `#` | `#`, `[n]`, `(#n)`, `Rank` each have one meaning in all layouts. |
| C11 | Rider and hardware constants | Saddle height 746, sit-bone setback 292, BRP 219, seatpost setback 11, clamp 30, grip inset 65: one source of truth, same value in subtitles, footers, form hints, `/saddles`, `/saddle-swap`, `/ideals`, `/help`. |
| C12 | Saddle / handlebar selection | Same IDs, same name matching, same defaults and the same "unrecognized" message on CLI flags and every web select. |
| C13 | Number formatting | Units always present or always absent per table kind; unit spacing (`468 mm` vs `468mm`); sign display (`+0.0`, `-0`, Unicode minus vs hyphen); en dash for ranges; negative ranges (`-2–8`). |
| C14 | Sign conventions | Each Δ column states or consistently implies `what minus what`; the same convention for the same pair on validate, fit, saddle swap, handlebars. |
| C15 | Errors | One shape: CLI `Error: …` on stderr with exit 1 (usage errors exit 2); web 400 plus banner plus field error. Same wording for the same cause on both. No message points to a command or page that does not exist. |
| C16 | Defaults and empties | "No input" means the documented default on every surface; an unknown filter value is either rejected everywhere or ignored everywhere. Empty results are stated, not shown as a bare header. |
| C17 | Titles and headings | Table titles follow one style (no leftover terminal decoration), counts in titles are computed, text sections vs tables used for the same kind of content everywhere. |
| C18 | Help | Every command has help reached the same way with the same exit code; every flag in help exists; every flag that exists is in help and does something. |

---

## 3. Phase 1 — Smoke and surface integrity

Goal: every entry point runs, exits correctly, and is reachable. Cheap, do first.

- **P1.1 Command matrix.** Run every command and alias from section 1 with no args, with `--help`, `-h`, `help`, and with one unknown flag. Record stdout/stderr split and exit code. *Oracle:* help prints to stdout and exits 0 for every command; unknown flags exit 2 with a message; unknown commands exit 2.
- **P1.2 Flag liveness.** For each flag, find one input where toggling it changes output. *Oracle:* a flag with no observable effect is a finding (C18).
- **P1.3 Route matrix.** GET every route with no query; then with every documented parameter set to a valid value, one at a time. *Oracle:* 200, a `<title>`, the nav with exactly one `aria-current`, no `<script>`, no `Set-Cookie`, forms are `method="get"`.
- **P1.4 Link crawl.** From `/`, follow every `href` on every page reached (bounded to same host). *Oracle:* no 404/500; every `/help#metric-<id>` anchor exists; every `/validate?frame=` link resolves 200.
- **P1.5 Server guards.** Non-loopback `Host` → 403; non-GET → 405; unknown path → 404; trailing-slash variants; `--port` out of range, in use, and `0`.
- **P1.6 Determinism.** Run each non-`fit` command and fetch each page twice; diff. *Oracle:* identical bytes.
- **P1.7 Dependency rule.** `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` lists only `bikegeo/...`; `pkg/web` does not import `pkg/cli`.

---

## 4. Phase 2 — Coherence and uniformity audit

Goal: fill the concept registry (section 2) and judge each concept. One agent can take C1–C5, another C6–C10, another C11–C18.

- **P2.1 Harvest.** Capture the full output of a fixed corpus into a scratch directory: `compare` (default; `--transpose=false` with 2 specs; `--score-detail`; `--no-score`; `--all`; one frame `:all@all`), `validate` (summary and one detail), `list` (both modes), `fit` (analysis, sweep, sweep `--detail`), and every web route in every mode (catalog vs single item vs comparison). Keep the files; later phases reuse them.
- **P2.2 Label census.** Extract every column header, row label, table title, subtitle, footer title, form label and hint. Group by concept. *Oracle:* one concept → one label. Known variation that is by design must be explained on the page (for example a suffix that states the saddle height); anything else is a finding.
- **P2.3 Format census.** For each metric and each physical quantity, list every rendered form (decimals, unit, spacing, sign). *Oracle:* rule C13. Pay special attention to quantities shown at different precision on different pages, since those produce apparent disagreements.
- **P2.4 Same thing, two specs.** Specify one physical setup several ways (default fork, `@rigid`, preset ID, `a2c=<same>/<same>`, `@0%`, travel with and without `mm`). *Oracle:* identical metric cells, and an identical fork label unless the page explains the difference (C3).
- **P2.5 Layout parity.** For one fixed set of specs render: CLI columns layout, CLI rows layout, web `layout=columns`, web `layout=rows` (split tables). *Oracle:* the set of metrics, their order, labels, reference ranges and every cell value are the same in all four; footers are the same set of lines.
- **P2.6 CLI ↔ web parity.** For `compare`, `validate`, `list`: take the "Equivalent terminal command" printed on the page, run it, and compare cell-for-cell with the page's tables. Also the reverse: for each CLI invocation in the corpus build the URL. *Oracle:* same cells. Anything the page shows that the printed command would not (or the reverse) is a finding.
- **P2.7 Catalog-page pattern.** `/forks`, `/stems`, `/ideals`, `/saddles`, `/handlebars`, `/frames` are the same kind of feature (catalog, optional single-item view, optional comparison). Tabulate for each: has a filter? has a single-item view? does the single-item view replace or append to the catalog? ID column name? ID links to what? count in title? *Oracle:* same answers, or a reason visible to the user.
- **P2.8 Analysis-page inputs.** `/saddles`, `/saddle-swap`, `/handlebars` take rider and hardware numbers. *Oracle:* the same parameter has the same name, default, hint and validation on every page that takes it; defaults shown in hints equal the values the engine actually uses (change nothing, read the subtitle); conflicting inputs (for example two parameters that derive the same quantity) are rejected or the precedence is stated.
- **P2.9 Error uniformity.** Trigger each error class on every surface that can produce it: unknown frame, ambiguous frame, unknown size, unsupported fork, sag on rigid, bad sag, unknown saddle, unknown handlebar, unknown profile, non-numeric number, negative or zero where physically meaningless, out-of-range column index, unknown filter value. *Oracle:* rule C15 and C16.
- **P2.10 Docs versus behaviour.** Run every example command in `README.md`, `AGENTS.md`, `docs/WEB.md`, `printUsage`, `printCompareUsage`, `/help`, and every hint text that names a command, page or count. *Oracle:* each runs with exit 0 and does what the sentence says; every number quoted in prose (frame counts, defaults, ranges, thresholds) matches the data.
- **P2.11 Interface placement.** Check the capability table in `AGENTS.md` against reality in both directions (no web-only feature has a CLI command or is referred to as one; `fit` has no page; nothing else leaked).

---

## 5. Phase 3 — Correctness of data

- **P3.1 Structural integrity of frames.** For every frame: unique kebab-case ID; registered in `allFrames`; default size exists; every size has all required fields; `CadFork` and every `Forks` entry resolve in the catalog; default fork is inside the supported A2C range; category is one of the values the list filter offers (and every offered category has at least one frame, or the filter says so); headset type has a cup-stack entry; every `GeoSlopMm` override has a `Reason`; every size override has `Notes`.
- **P3.2 Monotone sanity across sizes.** Within a frame, reach, stack, wheelbase, head-tube length and seat-tube length are non-decreasing with size order, or the exception is real (check the source chart). Flags transcription typos without touching data.
- **P3.3 Chart transcription audit.** For each frame, compare every published value against the manufacturer chart cited in the frame file (`Notes`, source URL, or the `.md` beside the Seven frames). This needs network access: ask the owner before fetching, and report mismatches only — never edit.
- **P3.4 Catalog integrity.** Fork IDs unique; `Standard` set for at most one fork per travel; A2C and rake plausible for the travel; stem sets expand to the stated counts; saddle derived columns satisfy `BRP→SitBones = Rail→BRP − Rail→SitBones`; handlebar derived offsets reproduce from the documented formula at default grip inset.
- **P3.5 Profile integrity.** `idmatch`: acceptable contains ideal; edge score, falloff and floor present for scored attributes; weights as documented; every attribute ID is in the metric registry; every registry metric not in the profile is deliberately absent (the glossary lists all 26, the matrix shows 18).
- **P3.6 Default baseline.** Every column of `DefaultComparisonState()` resolves through the public spec grammar to the same row (the web form's fallback to `a2c=…` tokens marks baseline rows that do not). List them.

---

## 6. Phase 4 — Correctness of computation

Write the oracles as a standalone script in your scratch directory (any language), fed with the published numbers copied from `list --all-sizes` / `validate`, not imported from the repo's Go code.

### 4A Frame geometry

- **P4.1 Identity at CAD.** For every frame and size, a setup equal to the CAD fork at CAD sag must reproduce the published reach, stack, HTA, STA and BB drop (to display precision). *Oracle:* published chart.
- **P4.2 Master stack equation.** Recompute `A2C_geo = (Stack − BB_drop + Rake·cos(HTA)) / sin(HTA) − HTL − spacer` for every size; compare with `validate` ("CAD Geo A2C", Δ) and `fit` ("Implied Struct A2C"). *Oracle:* your value; and `validate` and `fit` agree with each other including sign.
- **P4.3 Rigid-body rotation.** Changing fork length rotates the frame about the rear axle. For a ladder of A2C values on one frame check invariants: BB→head-tube-top distance `sqrt(reach² + stack²)` constant; chainstay length constant; `HTA − STA` constant; and directions: longer fork → slacker HTA and STA, more stack, less reach, less BB drop, more trail, longer wheelbase. Monotone and smooth (no jumps between adjacent 1 mm steps).
- **P4.4 Sag equivalence.** `frame@Tmm@S%` must equal `frame@a2c=<effective>/<rake>` in every frame-only metric. Effective A2C itself must equal static − travel × sag (confirm the convention from the fork label, then test it on every travel and allowed sag).
- **P4.5 Closed forms.** Recompute from the displayed dynamic values and compare with the displayed cell: `ETT = reach + stack / tan(STA)`; `Seat Axis Setback = H·cos(STA)`; `Seat Axis–HT Drop = H·sin(STA) − stack`; `Seat Axis–Rear Axle = rear centre − setback`; AETT per its footer definition; `Trail = (R·cos(HTA) − rake) / sin(HTA)` (find the wheel radius used and check it is one value everywhere); front centre and wheelbase from reach, HTL, HTA, rake, A2C, BB drop and chainstay. Tolerance: rounding of inputs only.
- **P4.6 Special frame features.** Bent seat tube (El Jefe): effective STA at H versus nominal. Sliding dropouts: `*` shown, max-stay position used for chainstay, wheelbase, front weight and Seat Axis–Rear Axle, and the footer range matches the frame data. Custom single-size frame (`seven-custom`). Fixed-fork frames (ECR, Sunliner).
- **P4.7 Independence claims.** The footer says which metrics are independent of saddle, seatpost, stem and bar. Vary `--saddle` and `--handlebar` over the whole catalog. *Oracle:* claimed-independent cells are byte-identical; claimed-dependent cells move in the direction the saddle/bar geometry implies.

### 4B Fit and cockpit

- **P4.8 BRP and rail position.** For several frames recompute the BRP point at radial distance H with the stated seatpost setback and the saddle's rail→BRP offset; check BRP Setback, the `rail +Nmm` note in the losses line, and that the rail position stays inside the saddle's clamp window. Cross-check the same frame on `/saddle-swap?frame=` (required clamp offset, feasibility) — the two pages must tell one story.
- **P4.9 Best stem and Oval Fits.** Enumerate the stem set × spacers yourself for one frame; compute hand position via the handlebar offsets on `/handlebars`; count positions inside the fit oval; pick the best by the documented rule. *Oracle:* same count, same best stem, same reach/stack offsets as the losses line.
- **P4.10 BRP–HB and HB Rise.** Distance and height from the BRP point to the selected HB point, recomputed from P4.8 and P4.9.
- **P4.11 Front weight.** Recompute from rider centre of mass (default rider profile), front centre and rear centre; check directions (longer chainstay → more front weight; longer front centre → less).
- **P4.12 Seatpost extension.** From H, seat-tube length and the stated definition; check frames missing seat-tube length show the missing-data marker and are renormalised, not scored as zero.

### 4C Scoring

- **P4.13 Curve properties.** For each scored attribute sweep the value finely across ideal, acceptable and far outside: sub-score is 100 inside ideal, continuous, strictly decreasing with distance, equals `EdgeScore` at the acceptable bound, crosses 0 at the documented falloff distance, never reaches the floor. The `/ideals?name=` "Sample Evaluation Points" table must agree with your sweep.
- **P4.14 Zone ↔ value ↔ score.** For every scored cell in the corpus: recompute the zone from the value and the reference rows shown in the same table; recompute the delta from the nearest ideal bound. *Oracle:* marker, delta, direction word and sub-score band all agree. Probe the boundaries on purpose (values that round onto a bound), since display rounding and zone decision use different precision.
- **P4.15 Aggregation.** Total = Σ(weight × sub-score) / Σ(weight) over non-missing criteria. In the breakdown, points-lost per criterion sum to `100 − total`; the weights in the subtitle match the profile; the displayed total matches the matrix.
- **P4.16 Ranking and pruning.** Rows sorted by unrounded score descending; `(#n)` contiguous; ties reported as ties. `--top N` output is exactly the first N of the unpruned ranking. `--top-per-frame K` keeps the best K per frame. Stage 1 keeps one sag per (frame, size, fork) with ties toward 20%. `--all` returns one row per frame that can be configured, and the count matches the help text. Negative and zero values behave the same on CLI and web.
- **P4.17 Explanations.** Each "Biggest Losses" line: value, delta and points equal the cell and the breakdown. Each "Head-to-Head" line: per-factor contributions sum (within rounding) to the stated margin, and every quoted value is the quantity the factor is named for, in its unit.
- **P4.18 Saddle adaptation.** With a non-default saddle the BRP reference rows shift (`AdaptProfileAttributeForSaddle`). *Oracle:* sit-bone position is invariant; the shifted ideal equals 292 − (BRP→sit bones) for that saddle; `/saddles?id=` and `/saddle-swap` quote the same number.

### 4D Validation and fit solver

- **P4.19 Verdict logic.** For each frame recompute every residual (A2C, front centre, wheelbase, cross-config) and the tolerance actually applied (including the dynamic wheelbase tolerance with its 2.0 mm floor). *Oracle:* the PASS/FAIL, the "Inferred Published Mode", and the tolerance printed in each notice. The "Allowed Slop" row, the "Overrides" line and the tolerance quoted in notices must agree with each other and with `fit`'s "Minimum passing slop".
- **P4.20 Negative controls.** Use `fit --try` to perturb a valid frame (stack ±5, HTA ±0.5, wrong sag, wrong spacer). *Oracle:* validation fails for a stated, correct reason; the sweep table explains why a combination fails when its residual columns look identical to a passing one.
- **P4.21 Solver.** Take a valid frame, record its CAD fork/spacer/sag, perturb one via `--try` to make it fail, and check the automatic solver proposes the original values among its top candidates. Check `--config` and `frame:size` actually select what they say.
- **P4.22 Summary ↔ detail.** Every row of the `validate` summary equals the headline of that frame's detail view; exit code is 1 iff any frame is invalid; the web status mirrors it.

### 4E Web-only analyses

- **P4.23 Saddle maths.** Recompute Ideal STA and safe ranges for each seatpost row from H, sit-bone setback, hardware setback and rail geometry; check the prose sentences on `/saddles` (they contain literal numbers) against the tables on the same page and against `/saddle-swap` for the same pair. Vary `saddle-height` and `sit-bone-setback`: every number on the page, including prose, must move.
- **P4.24 Saddle swap.** Invariants: sit-bone point and BB→sit-bone distance preserved; swap A→A is all zeros; A→B then B→A returns to the start; `all=1` overview rows equal the per-pair detail tables; `brp-setback` derives the same sit-bone setback the footer formula states.
- **P4.25 Handlebars.** Forward and inverse transforms round-trip; bar compared with itself is zero at any roll and inset; delta(A,B) = −delta(B,A); stem compensation = −grip shift; `width` trimming only moves what the formula says and rejects widths that are impossible; catalog table values equal single-bar view values at default inputs; the "Key Kinematic Insights" prose numbers equal the table.

---

## 7. Phase 5 — State, robustness and conventions

- **P5.1 URL round trips.** For the default view and for 5 varied views: submit the form unchanged → same tables; follow "Permalink" → same tables; follow each "Remove" link → exactly that column gone and nothing else changed; add a frame → appended with its defaults. Controls always show the parsed query.
- **P5.2 Incremental grammar.** `+spec`, `-N`, `-frame`, `-frame:size`, `-frame:size@fork`, `-all`, mixed with flags in any position (flag reordering in `reorderFlagsForStdlib`), on CLI and as `col=` on the web. *Oracle:* the documented column set. Watch for removal tokens that collide with flag names.
- **P5.3 Parser fuzz.** Feed the spec parser, `--try` parser and every numeric query parameter with empty, whitespace, very long, Unicode, `%`-encoded, negative, `NaN`, `Inf`, `1e9`, and repeated parameters. *Oracle:* never a panic or 500; always a clear 400 / exit 1. (`ideal.Evaluate` panics on an invalid spec: confirm it is unreachable from user input.)
- **P5.4 Output safety.** Every user-controlled string echoed on a page (spec echo in raw mode, banners, select fallback options, CLI line, permalink) is HTML-escaped; the printed CLI command is safely shell-quoted and, pasted into a shell, reproduces the view.
- **P5.5 Markdown integrity.** Every CLI table has equal cell counts per row, escaped `|`, no raw newlines inside cells; text sections do not masquerade as headings; output parses with a strict Markdown table parser.
- **P5.6 HTML conventions.** Per `docs/WEB.md`: `data-table`, `data-frame-id`, `data-size`, `data-rank`, `data-col`, `data-metric`, `data-zone` present where specified in both layouts; reference rows in `thead`; every zone-coloured cell also has its marker; `th scope`; every control has a label; one `h1`.
- **P5.7 Scale.** Time `compare --all`, `compare all:all@all --top-per-frame 0`, `validate --all`, and `/?all=1&top_per_frame=0`. Record wall time and row counts; report anything over a few seconds or any truncation.

---

## 8. Leads from the survey (reproduce before filing)

Observed at `759311c`. Each names the step that should confirm and scope it.

| # | Lead | Step |
| --- | --- | --- |
| L1 | Top-level `help` advertises `bikegeo saddle-swap` (unknown command) and uses frames that are not registered (`sendero`, `chameleon`, `chumba-sendero`). | P2.10 |
| L2 | Compare help says `--all` yields "18 rows"; 19 frames are registered. It also lists only two saddles for `--saddle` (catalog has three) and calls the profile `default` while output says `idmatch`. | P2.10 |
| L3 | Messages point to commands that do not exist: `see 'bikegeo saddles'`, `see 'bikegeo handlebars'` (shown on the web too), and `/ideals` subtitle `Use 'bikegeo ideals show <name>'`. | P2.9, P2.11 |
| L4 | `compare -all` (documented as "remove all columns") behaves like the `--all` flag. | P5.2 |
| L5 | `validate --config` and `fit --config` accept any value with no visible effect. | P1.2 |
| L6 | `list --bogus` is treated as a search string: empty table, exit 0. `/frames?category=bogus` likewise returns 200 with nothing. | P1.1, P2.9 |
| L7 | `validate --help` and `fit --help` print stdlib usage and exit 2; `compare --help` prints custom help and exits 0. `list` and `validate` are absent from detailed help. | P1.1 |
| L8 | `compare --top -1` is accepted silently on the CLI; `/?top=-1` is a 400. | P2.9, P4.16 |
| L9 | Sag: error text and `/help` say allowed sags are 20/25/30, yet `@0%` is accepted; `fenrir@25` is read as sag on the (rigid) default fork while `fenrir@60` is read as travel. | P2.4, C4 |
| L10 | Same fork, different labels: `fenrir` → `Stock -> 495mm A2C`, `fenrir@rigid` → `Rigid (495mm)`; suspension → `110mm @ 20% sag (499mm)`; custom → `a2c=500/51 -> 500mm A2C`. | P2.4, C3 |
| L11 | Header label differs by layout: `Fork & Eff A2C` (rows) vs `Fork & Eff. A2C` (columns); metric labels gain `@ 746mm` only in the columns layout. | P2.5 |
| L12 | Oval Fits: reference rows say Ideal `≥4`, Acceptable `1–3`, but cells show `4 ~`, `6 ~`, `27 ~`. `/ideals?name=idmatch` shows the same metric as `≥100` / `≥75` with sample cells like `0 ✗ 100 lo`, a third presentation. | P4.14, C7 |
| L13 | Head-to-head lines quote Oval Fits as sub-scores (`71 vs 65`, `-30 vs 2`) while other factors quote raw values; the losses lines call the same metric "Handlebar Clamp Fit". | P4.17, C6 |
| L14 | Cell delta and footer delta disagree for the same setup: Turner Venn Ti (L) cell `188 ✗ 31 fwd`, losses line "188mm is 30mm forward". Similar: El Jefe (XL) `184 ✗ 36 fwd` against a 219 ideal. | P4.14, P4.17 |
| L15 | Same displayed value, different zone: Fargo (L) @ rigid `214 ~ 5 fwd` vs Fargo (XL) @ rigid `214 ✗ 5 fwd`. | P4.14 |
| L16 | Footer mentions Wheelbase "calculated at this exact position" but the matrix has no Wheelbase column; the glossary lists 26 metrics, 8 of which never appear in any table. | P3.5, C6 |
| L17 | Score range: `/help` says "down to -200"; curves approach the floor without reaching it and some attributes have a floor of -100. | P4.13, C9 |
| L18 | Validation detail titles carry terminal decoration (`### === … [CAD COHERENT (VALID)] ===`) and proofs are preformatted text, unlike other pages' tables. | C17 |
| L19 | ECR validation: "Allowed Slop" row lists `fc` and `cross` at ±1.0 while "Overrides" lists only `a2c` and `wb`; wheelbase notice quotes ±1.0 although the documented floor is 2.0 mm. | P4.19 |
| L20 | `fit fenrir --try sag=0,20`: both rows show identical residuals and needed slop, one PASS and one FAIL, no reason given. | P4.20 |
| L21 | `/validate?size=XL` with no frame applies the size to all frames and fails on the first frame lacking it. | P2.9 |
| L22 | `/saddles?id=<one>` still shows the whole catalog and hard-coded cross-saddle prose; `/handlebars?id=` replaces the catalog. Prose on `/saddles` quotes fixed numbers (for example "~0.44°") while `/saddle-swap` shows `-0.31°` for the same pair. | P2.7, P4.23 |
| L23 | Web analysis pages accept physically meaningless input with 200: `saddle-height=0` or `-5`, `width=100`, `from` equal to `to`, `brp-setback` together with `sit-bone-setback`. | P2.8, P2.9 |
| L24 | Banner wording differs: some start with `Error:`, some do not; the compare page uses a list banner, generic pages a `<pre>`. | C15 |
| L25 | The web compare page always shows the scoring breakdown, but the printed equivalent command omits `--score-detail`. | P2.6 |
| L26 | `/saddle-swap?frame=binary-havok` reports the reference saddle itself as infeasible (`false (24.1mm)`), while compare scores the same frame with `rail +15mm`. | P4.8 |
| L27 | Precision drift: "Equivalent Saddle Height: 751mm" vs `751.4 mm` on the swap page; A2C shown as `485 mm`, `485.0 mm`, `485mm` across list, validate and compare. | P2.3 |

---

## 9. Reporting

Write findings to `docs/qa/findings-<phase>.md` (one file per phase, so parallel agents do not collide). One entry per finding:

```
### F-<phase>-<n>: <one-line statement of what is wrong>
- Quality: coherence | uniformity | correctness | robustness | docs
- Severity: S1 wrong number or wrong ranking · S2 contradictory or misleading output · S3 inconsistent presentation · S4 cosmetic
- Step: P4.14        Lead: L14 (if any)
- Reproduce: exact command or URL
- Observed: exact text
- Expected: the oracle and its value (show the arithmetic for S1)
- Scope: where else the same cause shows (list every occurrence found)
- Suggested regression test: package, test name, assertion
```

Rules: group by root cause, not by symptom (one finding for "delta is computed from unrounded values in cells and from rounded values in footers", listing all places). An S1 needs independent arithmetic shown. End each file with a coverage line per step: `done`, `partial (why)`, or `blocked (why)`.

## 10. Order and parallelism

1. **S0, then Phase 1** — one agent, short; blocks everything else only if the build or suite is broken.
2. **P2.1 harvest** — one agent; its corpus is input to Phases 2 and 4.
3. In parallel, each with its own findings file:
   - Phase 2 (split C1–C5 / C6–C10 / C11–C18);
   - Phase 3 (P3.3 only with the owner's approval for network access);
   - Phase 4A+4B, Phase 4C, Phase 4D, Phase 4E as four agents;
   - Phase 5.
4. **Consolidation** — one agent merges findings by root cause, checks every lead in section 8 has a verdict (`confirmed`, `not reproduced`, `by design: where documented`), and produces a ranked list (S1 first).
5. **Regression tests** — only after the owner triages: one test per accepted finding, following `AGENTS.md` conventions. Prefer property tests that encode the oracle (parity across layouts and surfaces, zone ↔ score agreement, aggregation sums, docs examples all run) over snapshot tests, so the same class of defect cannot return. Current statement coverage for orientation: catalog 36%, fit 47%, compare 55%, canonicalizer 56%, ideal 57%, scoring 64%, validator 69%; `frames` and `model` have no tests.
