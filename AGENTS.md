# Agent Instructions & Verification Loop

This repository is built with **Go**, **templ**, and **HTMX**.

## Fast Feedback Command Loop

After **every** code modification, always execute the following command to verify template generation, type safety, and test suite pass rate:

```bash
templ generate && go vet ./... && go test ./...
```

If Go or templ are installed in Homebrew or user paths:
```bash
export PATH=/home/linuxbrew/.linuxbrew/bin:$HOME/go/bin:$PATH && templ generate && go vet ./... && go test ./...
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
