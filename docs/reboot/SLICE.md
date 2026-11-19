# Next slice: three fixed views of the `/bike` diagram

**Status: Approved by Matt (2026-11-19), not yet implemented.** This plans the next single unit of work in the order confirmed in [IMPLEMENTATION.md §5](IMPLEMENTATION.md). It replaces Slice 4's free "pan/zoom" item with three fixed views chosen by the server. Matt chose this trade-off, giving up some usability to finish Milestone 1 sooner, and accepted every recommendation in §4. The decision is recorded in [DIALOGUE.md](DIALOGUE.md) ("Fixed diagram views replace free pan/zoom in Slice 4").

## 1. Where this fits

Milestone 1, Slice 4 ("Controls and inspection") lists: input controls and Go validation, fetched updates, linked trace/diagram selection, keyboard access, URL/history handling, and pan/zoom.

Already committed:

- Input controls, structured validation errors, and locked CAD Baseline Fork fields.
- Linked trace/diagram selection with the step inspector and local axes (`7518da8`), including keyboard navigation of trace steps.
- Fetched updates and URL/history handling (`4f0af5f`).

This slice covers the remaining pan/zoom item. Slice 5 (milestone acceptance with Matt) follows it.

## 2. Goal

The `/bike` diagram has three display modes:

| `view` | Shows | Framed around |
| --- | --- | --- |
| `bike` (default) | The whole bike, exactly as today | Every point the diagram already fits |
| `seat` | Seatpost head and saddle | Every pose above `SeatTubeTop`: `ShaftTop`, `SeatpostClamp`, `RailFrame`, `RailCenter`, `BRP`, `SitBones` |
| `stem` | Upper cover, spacers, stem, and bar | Every pose above `HeadTubeTop`: `CoverTop`, `SpacerTop`, `SteererClamp`, `BarClamp`, `BarRollFrame`, `HandGrip` |

Each close-up also frames every corner of the component outlines it draws (seatpost clamp and saddle rail; upper cover, spacers, steerer clamp, and stem body), so no rendered part is cut off.

Matt picks a mode with three links in the diagram card: **Whole bike · Seat · Stem**. The mode stays selected while he changes controls, and Back/Forward moves between modes like any other build change.

What this gives up compared with free pan/zoom: arbitrary zoom levels and framing, scroll/pinch zoom, dragging, and zoom shortcut keys.

## 3. Why this is simple

- **Small JavaScript extension.** View links use a separate `a[data-diagram-view]` hook that submits the current form values with the requested view. It reuses the existing fetched navigation, history, focus restoration, and trace selection handling. Unlike presets, switching views must preserve pending control edits.
- **The view is ordinary build state.** One more query parameter goes through the same decode → `workbenchState` → `URL()` path as every other control. A hidden form field keeps it across control changes.
- **Works without JavaScript**, because the links and the form are plain `GET`s. Without JavaScript, apply control edits before following a view link; its URL represents the last rendered build.
- **Go tests cover rendering and validation**, using the existing `httptest` + `goquery` tests. Controller tests and browser checks cover pending edits, navigation, and focus.

## 4. Choices Matt confirmed (all recommendations accepted)

1. **Presets keep the current view.**
   - *Recommended:* "Reset Defaults" and "CAD Baseline Fork" reset the build but keep the current view, so Matt can compare stem setups without leaving the stem view.
   - *Alternative:* Presets also return to the whole bike.
2. **Unknown `view` values.**
   - *Recommended:* Reject them with a `400` and a form error ("unknown diagram view"), the same as any other bad query value.
   - *Alternative:* Silently fall back to `bike`.
   - *Recovery:* Keep the invalid value echoed in the error form, but make both preset links use `view=bike` when the submitted view is invalid. Following either preset then recovers a valid build. Valid views still survive presets, including on error pages caused by another control.
3. **Label and line sizes in close-up views.** The close-up views are roughly 3–4× closer than the whole bike.
   - *Recommended:* Keep labels, landmark dots, label offsets, stroke widths, and trace arrows the same size on screen in every view. Go scales those drawing sizes by the view's zoom, which it knows exactly. Physical sizes (wheels, component outlines, tubes) are not scaled. With `view=bike` the rendered SVG is identical to today's.
   - *Alternative:* Let everything grow with the view. No work, but the labels and wide lines would cover the millimetre-scale detail the close-up views are for.
   - *Amended during implementation (2026-11-19):* Labels render at 1rem on screen, set by CSS rather than by Go, and overlapping labels are hidden until hovered, focused, or selected. Dots, strokes, and trace arrows are still scaled by Go. See [DIALOGUE.md](DIALOGUE.md) ("Close-up framing and readable diagram labels").

## 5. Implementation outline

Implementing the confirmed choices in §4 and the review fixes:

- **`internal/reboot/web/web.go`**
  - Add a `diagramView` string type with constants `bike`, `seat`, and `stem`. Every `switch` on it names all three cases (`exhaustive`).
  - Add `View diagramView` to `workbenchState`. It defaults to `bike`. Decode it in `decodeWorkbenchState`, reporting an unknown value as an input error on field `view` and still echoing the submitted value. Write it in `URL()`, carry it through `withCadBaselineFork`, and echo it in `bikeFormValues`.
  - Build the Reset Defaults URL from the default state with the current valid view, rather than a bare `/bike`. The error page's CAD Baseline Fork URL also keeps a valid submitted view. If the submitted view is unknown, both preset URLs use `bike`; keep the raw submitted value only in the error form echo. Rejecting the original request with `400` remains unchanged.
  - Turn `computeViewBoxFromPoints` into a function that returns a numeric `svgViewBox{X, Y, W, H}` with a `String()` method, so view boxes can be compared and padded.
  - In `buildBikeView`, compute the whole-bike box from today's point set and padding, then the box for the selected view from its points (§2). Pad a close-up box around its centre to the whole-bike aspect ratio, so the card does not change height between views.
  - Set `scale = view.W / whole.W` (exactly `1` for `bike`). Multiply every drawing-only size by it: label offsets in `makeLandmark` calls, the `svgTraceSteps` widths, radii, and font sizes, `bikeTurnArcRadiusMm`, `bikeAxisLengthMm`, `bikeAxisLabelOffsetMm`, and segment widths. Add a `bikeView.Px(v float64) string` method so the template scales its hard-coded sizes (`font-size="18"`, `r="5"`, `stroke-width="1.5"`, the world-axis lengths and labels, and the `--active-step-width` / `--ancestor-step-width` CSS variables set inline on the SVG).
  - Expose `View` and the three link URLs (`state` with each view) on `bikeView`.
- **`internal/reboot/web/templates/bike.html`**
  - Add `<input type="hidden" name="view" value="{{.Form.View}}">` next to `prev_fork`.
  - In the diagram card, inside `{{if .HasBuild}}`, add a small nav: three links with ids `view-bike`, `view-seat`, `view-stem` and `data-diagram-view="bike|seat|stem"` (the corresponding value on each link). Do not mark them `data-preset`. Keep real server-generated `href`s for navigation without JavaScript and modified clicks. The current view gets `aria-current="true"` and is not styled as a link.
  - Route hard-coded drawing sizes through `{{$.Px …}}`.
  - Change the SVG `aria-label` to name the view (for example "Leveled bike geometry, stem close-up").
- **`internal/reboot/web/static/workbench.js`**
  - Add a view-link handler that reads the live controls form, replaces only its `view` query value, and passes that URL to the existing navigation controller. Preserve `prev_fork` and pending edits so Go still resolves fork transitions and validates the submitted build. Do not navigate using the link's stale rendered `href` for an ordinary click.
  - Coordinate pointer-down, blur/change, and click handling for view links so clicking a view immediately after editing a control issues the combined build-and-view request without an intervening swap losing the click. Reuse the existing request cancellation and stale-response protection for changes already in flight. Keyboard activation must also use the live form. Keep modified clicks and non-primary clicks as ordinary link navigation.
  - Apply the existing `badInput` check before view navigation: report an unparseable control without replacing the form or discarding its value. Other invalid values still go to Go and render the normal `400` response with the requested view echoed.
- **Not touched:** `assembly`, `pose`, the stem preview (`/`), and the badge text.

Keep this to one reviewable change covering Go rendering, template controls, the small controller extension, and their regression tests; the event-ordering fixes make the earlier line-count estimate unreliable.

## 6. Verification

- **Go handler tests (`internal/reboot/web/web_test.go`)**
  - Default `/bike`: the canonical URL and hidden input carry `view=bike`. The `viewBox` and drawing sizes match today's output.
  - `view=seat` and `view=stem`: the `viewBox` contains every framing landmark's centre for that view and leaves out the landmark just below it (`SeatTubeTop` for `seat`, `HeadTubeBottom` for `stem`). The aspect ratio matches the whole-bike box within 0.01. The landmark label `font-size` is smaller than in the `bike` view.
  - The view links: there are three, with the expected ids and `data-diagram-view` values, and none has `data-preset`. Each `href` carries the current build values plus its own view, and only the current view has `aria-current`.
  - A control change keeps the view: `view=stem&spacers=30` renders `view=stem` in the canonical URL and the hidden input.
  - The presets keep the view (§4 item 1).
  - `view=zoom` returns `400` with the form error, no diagram, and the submitted value echoed. Both preset links carry `view=bike`; following each returns `200` with a diagram and a valid hidden view value.
  - A bad control value with `view=seat` returns `400` and still echoes `view=seat`; both preset links retain `seat` and recover successfully.
  - Update any existing assertions on exact canonical URLs to include `view=bike`.
- **JavaScript controller tests (`internal/reboot/web/static/*_test.js`)**
  - Edit spacers, then activate a view link before the control change is submitted: the request contains the edited spacers and requested view, not the link's old build values. Cover the pointer-down → change/focusout → click sequence and keyboard activation.
  - Switch view while a control request is in flight: the combined request wins, an older response cannot overwrite it, and `prev_fork` is preserved for a pending fork change.
  - An unparseable input blocks view navigation without losing the edit; a server validation error keeps the requested view in the returned form.
- **Fast loop:** `go vet ./... && go test ./...` after each code change, plus `make test-js` for controller changes. Then run `make pre-commit`, including `boundary`, `exhaustive`, both `deadcode` runs, and the zero-npm `node:test` suite.
- **Browser check** (manual, or with the built-in browser), against `make serve-reboot`:
  1. Each link switches the view without a full page load, and focus stays on the clicked link.
  2. While in the seat or stem view, changing spacers, the fork, the seatpost, or a preset keeps the view. Edit spacers and immediately click another view without first submitting: both the edit and view change survive. Repeat with keyboard activation and while an earlier update is still loading.
  3. Back/Forward steps through view changes.
  4. Labels, dots, and lines look the same size in all three views, and the card height does not change.
  5. Hovering and selecting trace steps still highlights the right steps and local axes in close-up views.
  6. With JavaScript disabled, apply a control edit, then follow a view link: both work as full-page loads and preserve the applied build.
  7. Open `/bike?view=zoom`, then use each preset in turn from that error page: each recovers to the whole-bike diagram. Repeat with a bad control value and `view=seat`: recovery retains the seat view. Check with and without JavaScript.

## 7. Out of scope

- Free zoom, panning, and scroll/pinch/drag gestures.
- More views, such as rear axle, front axle, or BB.
- Views on the stem preview (`/`).
- Anything belonging to Slice 5 or Milestone 2.

## 8. Done when

- ~~Matt has confirmed §4 and the decision is recorded in `DIALOGUE.md`, `IMPLEMENTATION.md`, and `DESIGN.md`.~~ Done 2026-11-19.
- The behaviour above is implemented in one reviewable commit (for example "Add fixed seat and stem views to the reboot bike diagram"), with the Go tests passing, `make pre-commit` clean, and the browser checklist in §6 checked.
- Slice 4's acceptance line in IMPLEMENTATION.md §5 holds: Matt can change the approved settings, inspect the responsible steps, and navigate without stale responses or mismatched trace/diagram state. After that, Slice 5 begins.
