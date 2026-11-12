# bikegeo web interface: design and conventions

`bikegeo serve` renders every report the CLI can produce as local, stateless HTML. The CLI remains fully supported; both front ends sit on the same engine and are tested against each other.

## Why

Comparison matrices, score breakdowns and validation proofs are dense, semantically rich tables. Terminal box-drawing cannot sort, colour by meaning, keep headers in view, link a metric to its definition, or let the reader change one input and see the effect. HTML and CSS can, without bringing a JavaScript toolchain into a zero-dependency Go project.

## Principles

- **Web 1.0 state model.** Every page is a pure function of its URL. Forms submit with `GET`; the response is a full HTML document. There is no session, cookie, database, or client-side state.
- **URL as the single source of truth.** Any view can be bookmarked, shared or reproduced from its URL. Each page also shows the equivalent `bikegeo …` command line, and the compare URL grammar reuses the CLI spec grammar verbatim.
- **No client toolchain.** No Node, no bundler, no framework, no `<script>`. Templates and CSS are embedded with `embed` into the single binary.
- **Local only.** The listener binds `127.0.0.1`, and requests whose `Host` is not `127.0.0.1`, `localhost` or `::1` get `403` (DNS-rebinding guard).
- **One engine.** Web and CLI call the same code, so numbers cannot drift. A test compares web cells with the CLI's CSV output.

Non-goals: HTMX/AJAX/partial updates, SPA routing, authentication, remote deployment, an external database, browser-driven end-to-end tests.

## Running

```bash
make serve                              # builds, serves on 127.0.0.1:8080, opens the browser
./bin/bikegeo serve --port 0 --open     # pick a free port
```

`serve` is dispatched from `cmd/bikegeo/main.go` rather than `pkg/cli` because `pkg/web` renders reports by calling `pkg/cli`; the dependency runs one way.

## Architecture

```
pkg/report     presentation-neutral Document / Table / Row / Cell (+ Zone, MetricID, Attrs). No I/O.
pkg/compare    comparison engine: spec parsing, Run(Options), PruneRows, BuildDocument -> report.Document
pkg/cli        flag parsing and terminal/md/csv rendering; RunDocument() runs any subcommand and returns a Document
pkg/web        net/http handlers, query parsing, html/template views, embedded templates + CSS
```

- `compare.Run(Options)` is the transport-neutral form of the old `runCompare`. `Options` is filled from CLI flags or from a URL query.
- `compare.BuildDocument` lays a result out as tables and tags each cell with the zone (`ideal` / `acceptable` / `outside` / `missing`) and metric ID that the terminal output used to discard when it flattened to strings.
- Every other command's table output goes through `App.render`. In CLI mode that prints as before; under `cli.RunDocument` it records ordered table and text sections instead. The web pages for `frames`, `forks`, `stems`, `ideals`, `saddles`, `saddle-swap`, `handlebars`, `validate` and `fit` are therefore the CLI commands, with query parameters mapped to the same arguments.

## Routes

All routes are `GET`. Unknown paths are `404`, other methods `405`.

| Route | Content | Parameters |
| --- | --- | --- |
| `/` | Compare matrix with editor form | see below |
| `/frames` | Frame catalog | `q`, `category`, `size`, `all-sizes` |
| `/forks`, `/stems` | Presets | none |
| `/ideals` | Profiles, or one profile | `name` |
| `/saddles` | Catalog, or one saddle | `id` |
| `/saddle-swap` | Swap analysis | `from`, `to`, `frame` |
| `/handlebars` | Catalog, or one bar / comparison | `id`, `compare` |
| `/validate` | CAD validation (all frames if `frame` is empty) | `frame`, `size` |
| `/fit` | Residual solver (needs a frame) | `frame`, `size`, `try`, `detail` |
| `/help` | Spec grammar and metric glossary, with `#metric-<id>` anchors | none |
| `/static/…` | Embedded stylesheet | none |

### Compare parameters

| Parameter | Meaning |
| --- | --- |
| `col` (repeatable) | CLI spec token, e.g. `fenrir:L@110mm@20%`. `+spec` / `-spec` incremental tokens work as on the CLI. |
| `frame`, `size`, `fork`, `sag` (repeatable, aligned) | What the form submits, one set per column; folded into specs. A blank `frame` is ignored (the "add a frame" row). |
| `saddle`, `handlebar` | Catalog IDs |
| `layout` | `auto`, `rows` (one row per bike), `columns` (one column per bike) |
| `score` | `0` for geometry only |
| `all`, `top`, `top_per_frame` | As the CLI flags |

No columns means the compiled default baseline (`compare.DefaultComparisonState()`).

### Errors

Invalid input returns `400` and re-renders the page with an alert banner (`#errors`, `role="alert"`) and an inline `.field-error` beside the offending control. Valid parts of the request still render. If the column specs fail to resolve, the editor falls back to plain text inputs echoing each spec so it can be corrected.

## HTML conventions

- Semantic tables: `table[data-table=<id>]`, `thead`, `tbody`, `th scope`, `td`.
- Rows: `tr[data-frame-id][data-size][data-rank][data-col]`. Metric cells: `td[data-metric][data-zone]` with class `zone-ideal|zone-acceptable|zone-outside|zone-missing`. In the one-column-per-bike layout the frame attributes are on the cells and column headers instead.
- Ideal and Acceptable reference rows are `thead tr.ref`, so they stay visible with the header.
- Colour is never the only signal: values keep their `✓ ~ ✗` markers.
- `th.sticky` pins the frame/metric column; `font-variant-numeric: tabular-nums` aligns digits.
- Form controls are populated from the parsed query, so the form cannot drift from the URL. The form posts to `/#results` so the browser returns to the results after a reload.

## Testing

Everything runs under `go test ./...`, in process, with no browser:

1. Build a request with `httptest.NewRequest` (set `req.Host = "127.0.0.1:8080"`).
2. Serve it with `web.New().ServeHTTP(recorder, req)`.
3. Assert the status code.
4. Parse the body with `goquery` and assert on selectors such as `tr[data-frame-id="…"] td[data-metric="…"]`.

`pkg/web/web_test.go` covers default rendering, query overrides, form echo, a form round trip that must reproduce the default view, zone classes, CLI/web cell parity, every route, validation errors, the host guard and method handling. `pkg/compare` and `pkg/report` test the document layer without HTTP. Existing CLI tests are unchanged and still assert the terminal output byte for byte.

**Dependencies:** `github.com/PuerkitoBio/goquery` is imported only from `_test.go` files. The shipped binary links the standard library only; check with `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` (it should list only `bikegeo/…` packages).

## Adding a page for a new CLI command

1. Make the command's table output go through `a.render(…)` / `a.renderFooters(…)`.
2. Add a `page{…}` entry in `pkg/web/pages.go` mapping query parameters to the command's arguments, plus a nav entry in `pkg/web/server.go`.
3. Add the route to `TestPages_AllRoutesRender`.
