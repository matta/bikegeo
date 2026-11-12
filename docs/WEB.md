# bikegeo web interface: design and conventions

`bikegeo serve` is the human interface: local, stateless HTML. The CLI is the agent interface and prints Markdown. Each capability is offered where it is used: `compare`, `validate` and `list` are on both (sharing builders), `fit` is CLI-only, and the catalog and analysis pages (forks, stems, ideals, saddles, saddle swap, handlebars) are web-only.

## Why

Comparison matrices, score breakdowns and validation proofs are dense, semantically rich tables. Terminal box-drawing cannot sort, colour by meaning, keep headers in view, link a metric to its definition, or let the reader change one input and see the effect. HTML and CSS can, without bringing a JavaScript toolchain into a zero-dependency Go project.

## Principles

- **Web 1.0 state model.** Every page is a pure function of its URL. Forms submit with `GET`; the response is a full HTML document. There is no session, cookie, database, or client-side state.
- **URL as the single source of truth.** Any view can be bookmarked, shared or reproduced from its URL. Pages that have a CLI counterpart (compare, validate, frames) also show the equivalent `bikegeo …` command line, and the compare URL grammar reuses the CLI spec grammar verbatim.
- **No client toolchain.** No Node, no bundler, no framework, no `<script>`. Templates and CSS are embedded with `embed` into the single binary.
- **Local only.** The listener binds `127.0.0.1`, and requests whose `Host` is not `127.0.0.1`, `localhost` or `::1` get `403` (DNS-rebinding guard).
- **Shared builders where both surfaces exist.** `compare`, `validate` and `list` build one `report.Document` that the CLI prints as Markdown and the web renders as HTML, so numbers cannot drift.

Non-goals: HTMX/AJAX/partial updates, SPA routing, authentication, remote deployment, an external database, browser-driven end-to-end tests, and mirroring every CLI command on the web (or the reverse).

## Running

```bash
make serve                              # builds, serves on 127.0.0.1:8080, opens the browser
./bin/bikegeo serve --port 0 --open     # pick a free port
```

`serve` is dispatched from `pkg/cli`; `pkg/web` does not import `pkg/cli`.

## Architecture

```
pkg/report     presentation-neutral Document (ordered Sections) / Table / Row / Cell (+ Zone, MetricID, Attrs). No I/O.
pkg/compare    spec parsing, Run(Options), BuildDocument, BuildFrameListDocument, ResolveValidationTargets
pkg/validator  BuildValidationDocument
pkg/fit        BuildSweepDocument / BuildAnalysisDocument (CLI only)
pkg/cli        flag parsing and Markdown printing of the above
pkg/web        net/http handlers, query parsing, web-only builders (docb), html/template views, embedded templates + CSS
```

- A web page is a `page{}` in `pkg/web/pages.go`: `Controls` (form inputs from the query), `Build` (query → `report.Document` or an error banner), optional `Link` (cell text → URL) and optional `Args` (the equivalent `bikegeo` command, only for pages with a CLI counterpart).
- Web-only builders use `docb` (`pkg/web/docb.go`): `render` / `renderFooters` add tables, `text` adds prose, `fail` records the error that becomes the 400 banner.

## Routes

All routes are `GET`. Unknown paths are `404`, other methods `405`.

| Route | Content | Parameters |
| --- | --- | --- |
| `/` | Compare matrix with editor form | see below |
| `/frames` | Frame catalog | `q`, `category`, `size`, `all-sizes` |
| `/forks`, `/stems` | Presets | none |
| `/ideals` | Profiles, or one profile | `name` |
| `/saddles` | Catalog, or one saddle | `id`, `saddle-height`, `sit-bone-setback` |
| `/saddle-swap` | Swap analysis | `from`, `to`, `all`, `frame`, `saddle-height`, `sit-bone-setback`, `brp-setback`, `seatpost`, `clamp-length`, `cockpit-dist`, `cockpit-drop` |
| `/handlebars` | Catalog, or one bar / comparison | `id`, `compare`, `grip-inset`, `roll`, `width` |
| `/validate` | CAD validation (all frames if `frame` is empty) | `frame`, `size` |
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

`pkg/web/web_test.go` and `pkg/web/pages_content_test.go` cover default rendering, query overrides, form echo, a form round trip that must reproduce the default view, zone classes, every route and its headline content, validation errors, the host guard and method handling. `pkg/compare` and `pkg/report` test the document layer without HTTP. CLI tests live in `pkg/cli/cli_test.go`.

**Dependencies:** `github.com/PuerkitoBio/goquery` is imported only from `_test.go` files. The shipped binary links the standard library only; check with `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo` (it should list only `bikegeo/…` packages).

## Adding a page

1. Write a builder that returns a `report.Document` (a `docb` method in `pkg/web`, or a shared builder in the domain package if the CLI needs it too).
2. Add a `page{…}` entry in `pkg/web/pages.go` with `Controls` and `Build` (use `run(...)` to adapt a `docb` method), and a nav entry in `pkg/web/server.go`.
3. Add the route to `TestPages_AllRoutesRender` and assert content in a goquery test.
4. Only if an agent needs the same report, add a CLI subcommand that calls the same builder and prints it with `printDocument`.
