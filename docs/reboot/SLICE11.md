# Slice 11 — Remove construction tracing and serve the bike inspector with plain GET

Status: Planned 2026-11-20

Q48 and Q54 settle the scope. Starting at `e7ff487`, retire the standalone stem preview, construction trace, and client script. The complete bike inspector includes its stem close-up. Its physical results, controls, fitting results, and three diagram views are the acceptance baseline.

**Size and sequence.** Larger slice, suitable for a lower-tier implementer after plan review. The change spans 19 Go source and test files, three templates, two JS assets, and their verification wiring. One plan covers two commits. The first removes the only production consumer that requests a trace. The second deletes the recording machinery and its arguments throughout the call graph. The existing physical tests and the same HTTP baseline verify both commits. No new domain ruling is needed.

## W — Make the bike inspector a plain GET page

Commit subject: `Serve the bike inspector without JavaScript or construction overlays`

1. **W1 — Routes and rendering.** In `internal/reboot/web/web.go`, keep the startup fixture closure check and the existing `/bike` handler. Register an exact root pattern `/{$}` with a temporary redirect to the literal `/bike` (`http.StatusFound`, 302). This sends a bookmarked preview URL to the default inspector. Remove the JS route and embed, `handleStemWorkbench`, `handleWorkbenchJS`, `stemTemplate`, and parsing of the trace partial. Embed and parse `templates/bike.html`. Delete `templates/stem.html` and `templates/trace_steps.html`.
2. **W2 — View data.** Use `assembly.PlaceBike(resolved.Setup, nil)` until commit G. Change the builder and its handler and direct test callers to `buildBikeView(fix bikeFixture, submitted workbenchState, bike assembly.BikePlacement, cadBaselineFork assembly.ChartBaseline) bikeView`. Delete `stemView`, `buildStemView`, `stemOutlineRects`, `svgTraceSteps`, `svgStep`, `buildSVGSteps`, `turnArcPath`, the preview ring and trace drawing constants, and the `bikeView.Steps` and `TraceSteps` fields. Keep `stemOutlines`, used to draw physical parts and frame the stem close-up. Drop landmark step IDs and the JS overlap priorities. Use the existing projected positions, label offsets, server-supplied nominal label scale, and per-view framing.
3. **W3 — Template and navigation.** In `bike.html`, remove the preview link, script, trace card, inspector, step and local-axis SVG groups and definitions, hover/selection styling, fetch-only metadata and busy state. Give the diagram card the available width. Retain the fixed world axes, ground, two wheels, component outlines, landmark dots, illustrative-line note, and stock table styling. Keep the submit button and the ordinary preset, view, and Apply links. Drop `bikeView.CanonicalURL` and `data-canonical-url` used by the fetch controller. The active diagram-view anchor already carries the resolved build URL.
4. **W4 — Request contract.** Preserve the existing `workbenchState` codec, validation, form echo, and link builders. The GET form submits to `/bike` with `view`, `fork`, transient hidden `prev_fork`, `fork_a2c`, `cup`, `sag`, `bar`, `spacers`, `stem_length`, `stem_angle`, `bar_roll`, `post`, `post_ext`, `rail_offset`, `rail_pitch`, `brp_x`, `brp_y`, `grip_x`, `grip_y`, `stock_bars`, and `stock_forks`. Keep existing default omission in generated links, fork-switch resets, CAD locks, optional target handling, and error recovery. Browser navigation and reload use normal GET requests.
5. **W5 — Retire the JS toolchain in the same commit as its tests.** Delete `internal/reboot/web/static/workbench.js`, `workbench_test.js` beside it, and `.node-version`. Remove the `test-js` recipe, phony entry, and `check` prerequisite in `Makefile`, and the Node setup and JS test steps in `.github/workflows/test.yml`. Update the Verification and Reboot Tree → Web bullets in `AGENTS.md` to describe the Go checks and server-rendered GET workflow.
6. **W6 — Web tests.** In `web_test.go`, replace the standalone preview and static asset tests with the route checks below. Retire trace-inspection, fetched-update-hook, and step-ID tests. In mixed tests keep the assertions on fitted parts, field values, metrics, input errors, drawing dimensions, landmark positions, and close-up containment. For successful pages, read resolved state from `nav.diagram-views a[aria-current]` via the existing href helpers. Exercise error recovery through the rendered form and preset links. Replace selectors for removed JS hooks with the existing anchor IDs or stock Apply selector. Keep the codec and form round-trip coverage.

## G — Remove trace state from geometry

Commit subject: `Remove trace recording from pose and assembly`

1. **G1 — Pose API.** In `internal/reboot/pose/pose.go`, keep `Pose` as a value with fields `origin Point` and `headingDeg Deg`. Its constructor and movement operations become:

   ```go
   func NewPose(origin Point, headingDeg Deg) Pose
   func (p Pose) Advance(distMm float64) Pose
   func (p Pose) Translate(localX, localY float64) Pose
   func (p Pose) Turn(deltaDeg Deg) Pose
   func (p Pose) Face(worldHeadingDeg Deg) Pose
   ```

   Remove `Trace`, `NewTrace`, `Step`, `PoseSnapshot`, their methods, `Pose.StepID`, `snapshot`, and trace formatting. Remove `Deg.Normalized`, whose only production caller was `Step.TurnDeg`. Preserve the arithmetic and heading conventions in the four movement operations.
2. **G2 — Assembly API and callers.** Remove the final trace argument from these functions and delete the deferred freezes:

   ```go
   func PlaceFrameset(frame FrameSetup, lowerCup LowerCupSetup, fork ForkSetup, wheels WheelSetup) (FramesetPlacement, error)
   func placeLeveledFrameset(frame FrameSetup, lowerCupMm float64, fork ForkGeometry, wheels WheelSetup, pitchDeg pose.Deg) FramesetPlacement
   func PlaceBike(setup BikeSetup) (BikePlacement, error)
   func ApplyFitting(setup BikeSetup, target FitTarget) (Fitting, error)
   ```

   Migrate calls in `assembly/{frameset,bike,fitting,stock_fitting}.go`, including the CAD baseline, setback fit, stock enumeration, validation, and final forward placement. Update `web/{fixture,web}.go` and both direct and helper-based test calls. In `poseOutline.corners`, translate from the outline's pose directly. Use the same two leveling passes and operations in the same order.
3. **G3 — Physical construction.** Drop movement labels in `assembly/{frameset,stem,handlebar,seatpost,saddle}.go`. Put short comments beside non-obvious operations, especially the turn from steerer-down into fork rake, the perpendicular reference for stem angle, the clockwise world-relative bar roll, local +Y seatpost setback, and the rearward sign of rail offset. Keep useful validation labels, field names, landmark names, and user-facing labels.
4. **G4 — Geometry tests.** Migrate `pose/pose_test.go`, `assembly/{frameset,stem,handlebar,saddle,fitting,stock_fitting}_test.go`, and `web/web_test.go`. Remove freeze, trace-parent, and trace-on/off assertions. In mixed tests keep the independently calculated expectations for branching, absolute heading, physical dimensions, Fenrir landmarks, and fit round trips. For example, the stem test with a 120° steerer, 50 mm to the clamp center, 100 mm stem, and 0° stem angle must still put BarClamp at `(-25 + 50√3, 25√3 + 50) mm` with heading 30°, within the existing 1e-9 tolerance. Keep validation and stock ranking coverage.

## Acceptance checks

Use the existing `httptest` and goquery helpers for automated coverage. Reuse the current assertions wherever they already cover an outcome. The following response values were sampled with independent HTTP GETs against the running binary at `e7ff487`. Compare displayed strings at their existing precision and use the existing numerical tolerances in assembly tests.

| Request | Expected result after W and G |
| --- | --- |
| `GET /` | 302, `Location: /bike`. Following it returns the default inspector with status 200. |
| `GET /static/workbench.js` and an unknown path | 404 for each. |
| `GET /bike` | Wheelbase `1162.2 mm`, BRP to HandGrip `556.6 mm`, SVG viewBox `-911.86 -850.12 2096.16 1254.30`. |
| `GET /bike?view=seat` | Same two readouts, SVG viewBox `-275.26 -770.36 137.14 82.06`. |
| `GET /bike?view=stem` | Same two readouts, SVG viewBox `299.99 -768.56 241.85 144.72`. |
| `GET /bike?brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726` | Fitted extension `260.1 mm`, rail offset `15.0 mm`, spacers `0.0 mm`. Stock summary shows 450 combinations and five rows with displayed scores `1.5, -7.2, -15.1, -16.2, -23.3` in order. Fit and stock Apply links open the selected settings. |
| `GET /bike?spacers=-1` | 400, submitted value echoed, invalid spacer field, working recovery links. |

For each valid view assert one schematic, the native GET form and submit button, and zero matches for `script, table.trace-table, [data-step-inspector], [data-step-id], .trace-steps, .step-frames`. Keep the world-axis and component-outline assertions. The 400 page also has zero scripts. Exercise the existing GET form and link round trips, including switching forks and carrying targets between views.

**Live check.** Run `make serve-reboot`, follow the root redirect, submit a changed build, and reload its URL. Use the three view links and compare readouts and framing to the table. Check the default whole-bike and close-up labels at normal browser width, and that removing the trace card leaves no empty diagram column. Open the two-target URL, follow a stock Apply link and confirm the fitted hardware in the controls. Record observed values and a screenshot path in the handoff.

## Rules that bite

- Q48 and Q54 govern this slice. Keep trig in `pose`, the chart closure startup check, immutable placement values, and the current fixture and scoring behavior. Out of scope are the compare page, catalog port, new hardware models, scoring changes, and the legacy application.
- After every code change run `go vet ./... && go test ./...`. Run `make pre-commit` for each planned commit and again if the fix step rewrites files. Keep the verification commands runnable at both commit boundaries.
- If an unresolved design choice blocks implementation, record it and ask Matt. On completion or interruption, follow [Implementation handoff in DESIGN.md](DESIGN.md#implementation-handoff): insert the handoff below the status, list W1–W6 and G1–G4 with commits, deviations, verification and open issues, and synchronize the Slice 11 milestone status. Matt accepts the slice.
