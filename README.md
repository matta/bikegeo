# bikegeo 🚲

A zero-dependency Go application that compares bicycle frames the way a fitter does. It assembles each candidate bike physically (frame, fork, headset, stem, spacers, handlebar, seatpost, saddle), finds the best-fitting build for a rider's saddle and hand-position targets, and scores it. Two interfaces share one engine: a **local web app** (`bikegeo serve`) for people, and a **JSON command line** (`bikegeo compare`, `bikegeo bike`) for agents and scripts.

Every frame, fork, handlebar, stem, seatpost, and saddle is compiled into Go structs under [`internal/catalog/`](internal/catalog), with no data or configuration files. Each frame has a source record (`frame_<id>.md`) that lists every value gathered and where it came from.

---

## What it does

1. **Physical assembly.** A bike is built by walking its parts in order: frame, fork (with sag), headset cup, spacers, stem, handlebar, and seatpost and saddle. The result is each landmark's position, so every readout comes from one placement rather than from separate formulas.
2. **Fitting to two targets.** The rider's targets are the saddle's **BRP** and the **HandGrip** point, both measured from the bottom bracket. For each frame, size, fork, and sag, the engine finds the best stem, spacers, and saddle position it can. Builds fall into four fit tiers: `fits_all`, `fits_brp`, `fits_grip`, and `fits_neither`.
3. **0–100 scoring.** Each build gets a score from 12 weighted attributes: three fit attributes, plus ranges such as trail, fork sag, spacer stack, and seatpost insertion. Builds are ranked by tier first, then by each tier's own tie-breaks and the score (DESIGN.md, Q61, Q69, Q90).
4. **Open fork choice.** Any catalog fork that is compatible with a frame can be tried, in every variant and at the sags its travel allows. A frame's listed forks come first.
5. **Charts are checked.** Each frame's published geometry chart is checked against its chart basis fork in `go test`, so a transcription error fails the build instead of reaching the app.

---

## Quick start

```bash
make build                           # builds bin/bikegeo
make serve                           # build, serve on http://127.0.0.1:8080, open the browser
./bin/bikegeo serve --port 0 --open  # or choose a free port
```

### Web (for people)

- `/` is the multi-frame comparison matrix: the best configuration of every frame (or a custom selection), ranked, with a leveled-geometry table and a collapsible score breakdown.
- `/bike` is the single-bike inspector: the diagram, the fitting readouts, the submitted build's score, and the ranked builds.

Everything is a plain `GET` URL that you can bookmark or share, for example `/?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20` and `/bike?frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20`. The URL query is the only state, there is no JavaScript, and the server listens on `127.0.0.1` only.

### CLI (for agents and scripts)

`compare` and `bike` take the same URL query as the web pages and print indented JSON. `--help` lists every query key.

```bash
./bin/bikegeo --help
./bin/bikegeo compare --help

# Rank every frame (best row each), keep the top 3:
./bin/bikegeo compare 'top_per_frame=1&top=3'

# One selection, then two frames across every handlebar:
./bin/bikegeo compare 'frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20'
./bin/bikegeo compare 'frame=surly-krampus&frame=otso-fenrir-stainless&bar_options=all&top=5'

# Evaluate one bike, with or without fit targets:
./bin/bikegeo bike
./bin/bikegeo bike 'frame=surly-krampus&size=XL&fork=fox34_120-534x44&sag=20'
./bin/bikegeo bike 'frame=binary-havok&size=L&brp=off&grip=off'

# Pick fields out with jq:
./bin/bikegeo compare 'top_per_frame=1&top=3' | jq '[.Rows[] | {frame: .FrameID, size: .Size, fork: .ForkID, tier: .Build.Score.Tier, total: .Build.Score.Total}]'
```

---

## Adding a frame

Follow the [ingest-frame skill](.agents/skills/ingest-frame/SKILL.md): record every value and its source first, transcribe the published chart, find its basis fork with a temporary Go test, and let the catalog's `go test` confirm the chart is consistent.

## Running verification and tests

```bash
make check       # format, vet, banned-term and dependency checks, exhaustive enum check, and tests
make pre-commit  # required before committing: runs go fix, then make check
make deadcode    # unreachable-function report; run by hand, never a gate
```

The design and its rulings are in [`docs/reboot/DESIGN.md`](docs/reboot/DESIGN.md), the workflow in [`docs/reboot/WORKFLOW.md`](docs/reboot/WORKFLOW.md), and the working rules for agents in [`AGENTS.md`](AGENTS.md).
