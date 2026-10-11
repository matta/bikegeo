# Agent Instructions & Verification Loop

This repository is built with **Go**, **templ**, and **HTMX**.

## Fast Feedback Command Loop

After **every** code modification, always execute the following command to verify template generation, type safety, and test suite pass rate:

```bash
templ generate && go vet ./... && go test ./...
```

If Go or templ are installed in Homebrew or user paths:
```bash
export PATH=$HOME/go/bin:$PATH && templ generate && go vet ./... && go test ./...
```

## Architectural Guidelines

- **Templates**: All UI components are authored in `.templ` files inside `pkg/web/templates/`.
- **Compiler Type Safety**: `templ` compiles templates into type-checked Go code (`*_templ.go`). Struct fields and component props are strictly typed.
- **Hypermedia / HTMX**:
  - Favor returning HTML fragments (`hx-target`, `hx-swap="outerHTML"`) instead of JSON client-side state layers.
  - Three target benchmark patterns implemented:
    1. **Inline-editable data row/cell**: `InlineCellDisplay` and `InlineCellEdit` via `hx-post` and outerHTML swap.
    2. **Debounced search box**: `hx-trigger="keyup changed delay:300ms, search"` updating `#comparison-matrix`.
    3. **Asynchronous form submission**: `hx-indicator` spinner with inline validation error highlighting.
- **Persistence**: Bikes are stored as individual JSON files in `data/bikes/*.json`.

## Testing Strategy & Conventions

This project uses a two-tier testing model optimized for Go + HTMX: fast HTTP fragment assertions in Go for coverage, and Playwright for browser-level DOM swap verification. **Do not use `go-rod`.**

---

### 1. HTTP Handler & Partial Tests (Go)

* **Coverage:** ~85–90% of test surface. Tests run sub-second without launching browsers.
* **Stack:** `net/http/httptest` + `github.com/PuerkitoBio/goquery`.
* **Conventions:**
  * Simulate HTMX requests by setting headers (`req.Header.Set("HX-Request", "true")`).
  * Verify dual behavior: standard GET returns the full HTML layout; HTMX requests return only the target fragment.
  * Use `goquery` CSS selectors to assert DOM elements and attributes (`hx-target`, `hx-swap`, `hx-swap-oob`) instead of raw string/regex matching.
  * Assert HTMX response headers (`HX-Trigger`, `HX-Redirect`, `HX-Push-Url`).
  * Ensure validation failures return `422 Unprocessable Entity` so HTMX swaps error partials.
* **Command:**
  ```bash
  go test ./...
  ```

---

### 2. Browser End-to-End Tests (Playwright)

* **Coverage:** Critical paths only (forms, inline editing, out-of-band updates, client-side events).
* **Stack:** Playwright (`@playwright/test`) with TypeScript.
* **Conventions:**
  * **Auto-waiting:** Always use web-first assertions (`await expect(locator).toBeVisible()`, `toHaveCount()`, `toContainText()`). Never use hardcoded sleeps; Playwright automatically polls while HTMX completes DOM mutations.
  * **Selectors:** Prioritize accessibility selectors (`page.getByRole()`, `page.getByLabel()`) or `data-testid` over brittle DOM hierarchy selectors.
  * Run tests against a local instance or an ephemeral test server.
* **Command:**
  ```bash
  npx playwright test
  ```

