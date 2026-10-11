# BikeGeo Analyst 🚲

A web application for **bicycle geometry validation and apples-to-apples frame comparisons**. Built with **Go**, **templ**, and **HTMX**.

Bike geometry is stored directly in **local JSON files** under [`data/bikes/`](data/bikes).

---

## Architectural Philosophy: Go + templ + HTMX

This project is built around the hypermedia architecture:

```
[Local JSON / Disk] ↔ [Go Handlers + templ Components] ↔ [HTML Over the Wire] ↔ [DOM]
```

### Why This Stack Excels
1. **Deterministic, Rapid Compiler Feedback**:
   Go compiles in seconds, and `templ generate && go vet ./... && go test ./...` provides strict, unambiguous compiler feedback.
2. **Compile-Time Type-Safe Templates**:
   [`templ`](https://templ.guide) compiles HTML templates into native Go code. If a model field or prop is renamed or mismatched, `go build` fails immediately at compile time.
3. **Single Mental Model Without State Desynchronization**:
   HTMX eliminates client-side state hydration and JSON glue layers. The server returns HTML fragments directly swapped into the DOM.
4. **Offline & Self-Contained**:
   HTMX is vendored directly under `static/js/htmx.min.js`.

---

## Key Features

1. **Local JSON Bike Storage**:
   - Supports `manufacturer`, `model`, `category`, `headset_cup_type`, `cup_stack`, `seat_height`, per-size frame dimensions (`reach`, `stack`, `hta_deg`, `sta_deg`, `htl`, `bb_drop`, `wheelbase`), and configurations with modular fork specifications (`fork`: `{id, name, travel_mm, a2c, rake}`).
   - Central fork registry in [`data/forks.json`](data/forks.json) allows configurations to reference standard forks by ID/name, auto-populating inferred values in the UI.
   - Built-in UI to add, edit, and delete bikes directly saving to disk.

2. **Benchmark UX Patterns Implemented**:
   - **Inline-Editable Data Cells**: Click any cell (e.g. Reach or Stack) to edit in place, saved via `hx-post`, swapping back into updated display markup.
   - **Debounced Search-as-You-Type**: Search box with `hx-trigger="keyup changed delay:300ms, search"` filters the comparison matrix without full page reloads.
   - **Asynchronous Form Submission**: Add/Edit view with `hx-indicator` loading spinner and live inline validation errors.

3. **Hardtail Geometry Dynamics**:
   - Hardtails compress **only at the front fork**, which dynamically rotates the entire frame around the rear axle.
   - Accurately computes how changes in fork axle-to-crown (A2C) or suspension sag dynamically steepen/slacken head & seat angles, alter reach & stack, and shift bottom bracket drop.

4. **Master Stack Equation & CAD Reverse-Engineering**:
   - Proves whether published manufacturer charts match physical front-end geometry:
     $$\text{Stack} = \text{BB\_drop} - \text{Rake} \cdot \cos(\text{HTA}) + (\text{A2C} + \text{cup\_stack} + \text{HTL}) \cdot \sin(\text{HTA})$$
   - Reverse-engineers geometric Axle-to-Crown:
     $$\text{A2C}_{\text{geo}} = \frac{\text{Stack} - \text{BB\_drop} + \text{Rake} \cdot \cos(\text{HTA})}{\sin(\text{HTA})} - \text{HTL} - \text{cup\_stack}$$
   - Automatically detects whether the manufacturer published numbers at **0% static sag** or **15–25% sagged CAD**, or whether there is a published typo/inconsistency.

5. **Headset Hardware Reality**:
   - Explicitly accounts for external lower headset cups.
   - Straight 44mm head tubes running tapered forks require an **EC44 lower cup**, adding **12 mm** of physical stack below the head tube.

6. **Apples-to-Apples Canonicalized Comparisons**:
   - Compares multiple frames on an identical benchmark (same canonical fork A2C, same suspension sag %, same headset cup standard, same rider saddle height).
   - Computes:
     - Canonical Reach, Stack, HTA, STA, and BB Drop.
     - Actual Effective Top Tube (AETT) at rider saddle height.
     - Spare Steerer Stack (vertical and axial length required to level bars with saddle).
     - Delta values ($\Delta$) relative to baseline and reference frames.

7. **Interactive 2D Cartesian SVG Frame Diagram**:
   - Superimposes baseline frame pose against the dynamically pitched canonical pose relative to the rear axle origin $(0, 0)$.

8. **Backward-Compatible REST API**:
   - Provides `/api/bikes`, `/api/bikes/{id}`, `/api/validate`, `/api/canonicalize`, `/api/analyze`.

---

## Quick Start (Go)

### Prerequisites
- Go (v1.22+)
- `templ` CLI (`go install github.com/a-h/templ/cmd/templ@latest`)

### Build & Run
```bash
make run
```
Or directly:
```bash
templ generate
go run ./cmd/server
```
Open [http://localhost:3001](http://localhost:3001) in your browser.

---

## Running the Verification Loop & Tests

```bash
make check
```
Or via the agent loop command:
```bash
templ generate && go vet ./... && go test -v ./...
```

---

## Project Structure

```
bikegeo/
├── cmd/
│   └── server/
│       └── main.go           # Server entrypoint
├── pkg/
│   ├── model/
│   │   └── bike.go           # BikeGeometry, ValidationReport, Fit types
│   ├── geometry/
│   │   ├── geometry.go       # 2D Cartesian physics & pitching math
│   │   └── geometry_test.go  # Numerical parity tests
│   ├── validator/
│   │   ├── validator.go      # Master Stack equation proof & CAD checks
│   │   └── validator_test.go # Validation tests
│   ├── canonicalizer/
│   │   ├── canonicalizer.go  # Apples-to-apples normalization engine
│   │   └── canonicalizer_test.go
│   ├── storage/
│   │   ├── storage.go        # JSON file persistence in data/bikes/
│   │   └── storage_test.go   # CRUD storage tests
│   └── web/
│       ├── handlers.go       # HTTP handlers (HTMX fragments & REST API)
│       ├── handlers_test.go  # Integration test suite (Target Patterns 1, 2, 3)
│       └── templates/        # Type-safe templ templates
│           ├── layout.templ
│           ├── navbar.templ
│           ├── compare.templ
│           ├── validator.templ
│           ├── editor.templ
│           ├── diagram.templ
│           └── inline_edit.templ
├── static/
│   └── js/
│       └── htmx.min.js       # Vendored offline HTMX
├── data/
│   └── bikes/                # Local JSON bike files
├── Makefile                  # Build & test tasks
├── AGENTS.md                 # Agent feedback loop instructions
├── go.mod
└── go.sum
```
