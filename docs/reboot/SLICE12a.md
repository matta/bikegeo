# Slice 12a — Physical headset, seatpost and saddle on the Fenrir

Status: Implemented 2026-11-20; awaiting Matt's acceptance

## Implementation handoff

- **Who and when:** 2026-11-20, Claude Sonnet 5.5.
- **Commits:** all three planned commits are made on `reboot`, each passing `make pre-commit` on its own.
  - H, `Validate fork and headset pairings on the Fenrir` — `e5b3af5`.
  - P, `Fit and display seatpost insertion within frame and post limits` — `083e0a3`.
  - S, `Place the SQlab saddle landmarks above the rail cradle` — `4360b44`.
- **Done and not done:** H1–H5, P1–P5 and S1–S4 are done. The live check is done except for one item listed under Open issues.
- **Deviations:**
  - The frame's default cup is checked in `validateFrameGeometry` (through `validateDefaultLowerCup`), so every path that validates a frame also checks it. The plan named only the headset pairing check.
  - `forkOption.DefaultLowerCup` stays as a resolved field. The Fenrir options fill it from `Chart.Frame.DefaultLowerCup`, and the synthetic ZS44/EC44 test still supplies per-pair cups through `makeForkOption`. The plan said the frame default is the source, and it is, but the field was not removed.
  - The seat outline component keeps its `data-component="seatpost-clamp"` name. Only the landmark, struct field and diagram label were renamed to `RailCradle`.
  - Most reference-value assertions that predate P still read exposed length (`Seatpost.ExposedMm()`), because the old numbers were exposures and still hold. New assertions use insertion directly.
  - Added small public helpers the plan did not name: `SeatpostSpec.Validate`, `SeatpostSetup.ExposedMm`, `postInsertionBounds` in the web package (zero on error), and the `post-exposed` readout.
  - The `PlaceStem` rejection of `MaxSpacersMm > 30` is an untyped error, like the other configuration errors there.
- **Verification:** `make pre-commit` exits 0 on the final state. Every independent reference value in the plan reproduced without adjustment: the fitting table, the 25 mm post, the Redshift forward-limit and generic upper-limit regressions, the stock fork table, all stock scores (−20.4, −22.2, −28.1 and the −14.4 and −3.3 families), and the Q92 heights (746.000 unpitched, 746.008 CAD, 745.619 Lithic, BRP (−155.065, 729.317)). Live check with `make serve-reboot`: the default page shows 745.6 mm and the CAD preset 746.0 mm radial BRP height. Redshift with `post_insert=275` returns 400 and `post_insert=250` returns 200. The first stock Apply link is SID 20%, `post_insert=183.5237`, `rail_offset=15.0`, score −20.4. The seat view at `rail_pitch=10` shows the rail clearly below the profile and no clipped labels. The screenshot is at `/Users/matt/.claude/projects/-Users-matt-src-bikegeo/cb3a04f6-449b-47a8-b73f-f6367b0e2c13/tool-results/mcp-Claude_Browser-blob-1795212131961-3083jc.jpg` (not in the repo).
- **Open issues:**
  - The plan gives no independent values for the readouts that changed with the raised BRP: `brp-grip-dist` (544.9 mm default, 562.0 mm with the 3OX 12°/15 bar), `sitbones-grip-rise` (+3.3 mm on that bar), the 25 mm-post setback notes at (−189, 713.1) (rail +12.5, centered setback 37.0 mm at 187.3 mm insertion), the Redshift centered insertion (104.1 mm), and the seat-view `viewBox`. Those test values were taken from the app after the BRP landmark itself was confirmed against Q92. Matt may want an independent probe for them.
  - With centered rails, `RailCradle` and `RailCenter` sit at the same point, so their labels overlap in the seat view. This predates the slice.
  - The seat-view screenshot lives in the session's tool-results directory, not the repo. Matt can regenerate it with `make serve-reboot` and `/bike?view=seat&rail_pitch=10`.

A **Bike** is the full assembly in a particular configuration. `BikeSetup` specifies the components and their settings. `PlaceBike` returns a `BikePlacement` only when the components can be installed together. The lower headset cup must fit the frame’s head-tube bore and the installed fork’s steerer. Seatpost insertion must fall within the limits of both the frame and the post. The stem may have up to 30 mm of spacers between the upper headset cover and the stem clamp (Q82). Stock fitting tries 0 through 30 mm in 5 mm steps.

The Otso Fenrir fixture exercises these rules. Fitting uses the placed saddle surface above the rails, and the `/bike` page displays that geometry or reports an assembly error. With the default generic 0 mm setback post, centered level rails and the frame in its unpitched reference attitude, the distance from the bottom bracket to the saddle’s BRP is 746 mm.

Base: Slice 11 accepted at `b300893`. [Design rulings](DESIGN.md) Q66, Q72, Q75a–c, Q79, Q82, Q84, Q89 and Q92. Q79 gives the Fenrir model one acceptance point. Use the three independently verifiable commits below. H changes fork compatibility and spacer enumeration, P changes how insertion is represented, and S changes the saddle coordinates and their fit references. This is a larger slice suitable for a lower-tier implementer after plan review.

## H — Validate headset compatibility and set the spacer limit

Commit subject: `Validate fork and headset pairings on the Fenrir`

1. **H1.** In `assembly/frameset.go` add `type LowerBore string` with constants `Bore34Pressed`, `Bore44Pressed`, `Bore56Pressed`, `Bore42Integrated`, `Bore52Integrated`. Add `type Steerer string` with `SteererStraight118`, `SteererTapered`, `SteererCAD` (all nonempty). Add `FrameSetup.LowerBore LowerBore`, `FrameSetup.DefaultLowerCup LowerCupSetup` and `ForkSpec.Steerer Steerer`. Keep the installed cup argument to `PlaceFrameset`, the chart's `CadLowerCup`, and the purely dimensional `ForkGeometry`.
2. **H2.** Add `func validateHeadset(bore LowerBore, cup LowerCupSetup, steerer Steerer) error`. Reuse `cup.Validate()` for stack validity and `InputLowerCup` for incompatible pairings. Check the frame's declared default as a physical cup of the correct bore. Validate the actual pairing in `validateFramesetInputs`. Use the same pairing check with `SteererCAD` in `PlaceCadBaselineFork` and `CheckChartClosure`. Reject unknown bores and steerers even for bare-head-tube inputs. CAD has zero travel and sag. The existing chart residual calculation, rounding budgets and startup check still apply.

   | Installed standard | Required lower bore | Accepted physical steerers |
   | --- | --- | --- |
   | EC34 | 34 pressed | Straight |
   | EC44 | 44 pressed | Straight or tapered |
   | ZS44 | 44 pressed | Straight |
   | ZS56 | 56 pressed | Straight or tapered |
   | IS42 | 42 integrated | Straight |
   | IS52 | 52 integrated | Straight or tapered |

   CAD accepts each cup on its matching bore and also accepts a zero-stack bare head tube on any recognized bore. A physical fork always requires a physical cup. Add table tests in `frameset_test.go` for this matrix, mismatched bores, unknown values and the CAD exception. Verify a mismatched physical cup on a chart also fails through both CAD entry points.
3. **H3.** In `web/fixture.go`, put the fixture's 12 mm EC44 on the frame, infer its 44 mm pressed bore from EC44, and annotate the Lithic, SID and Fox steerers as catalog tapered. Preserve the existing CAD hypothesis (bare, 0 mm, 495/54 mm). Set the sentinel in `cadForkOption` only. Make the frame's default the source of each physical Fenrir option's resolved cup. Preserve the existing synthetic straight-ZS44/tapered-EC44 pairing test through an explicit per-pair cup where supplied. The frame's stock and interactive paths must resolve the same pair, including normal `cup` stack edits for the submitted build. Validate fixture pairings through assembly for all real options and the CAD option. Keep `fork=cad` locked through form submissions and preset links. Update the assembly test constructors/literals for frames and forks in frameset, closure, saddle, handlebar, fitting and stock fitting tests.
4. **H4.** In `assembly/stem.go` rename the setup field to `UpperStackMm float64`. Supply the fixture from named assembly constants `DefaultUpperStackMm = 11.0` (illustrative) and `DefaultMaxSpacersMm = 30.0`. Use the renamed field in `PlaceStem`, web outlines and test setups. Keep the lower hardware limits exercised by synthetic stem tests. Enforce the 30 mm ceiling on physical builds through `StemSetup.MaxSpacersMm` and `PlaceStem`, including rejecting a configured maximum above 30. Keep the 5 mm stock step. Relocate successful test requests that used 35 or 40 mm to legal values, keeping distinct values for edit/recovery tests. Check 30 accepted and 31 rejected on both form and assembly paths. Both successful and error forms show max 30.
5. **H5.** Update the grid/boundary probes in `fitting_test.go` and `stock_fitting_test.go`. Ten lengths × five angles × seven spacer heights gives 350 candidates, 700 for two bars, 2,450 for seven fork settings, 5,250 for 15 bars, and 36,750 for all bars and forks. The zero-spacer setup still gives 50. Check near-bound rounding at `30 − 5e-10` mm. With only HandGrip (481, 726) on the default fork and bar, rank 1 is still −17°, 130 mm, zero spacers, radius 1.358546, score 78.711983 (±0.00001). Update the count in every stock summary and exercise the existing Apply links.

## P — Use installed seatpost insertion throughout

Commit subject: `Fit and display seatpost insertion within frame and post limits`

1. **P1.** Add `MaxPostInsertionMm, MinFrameInsertionMm float64` to `FrameSetup`. In `assembly/seatpost.go` use these types and signatures:

   ```go
   type SeatpostSpec struct {
       TotalLengthMm, MaxInsertionMm, MinInsertionMm float64
       SetbackMm, RailClampLengthMm float64
   }
   type SeatpostSetup struct {
       Spec SeatpostSpec
       InsertionMm float64
   }
   type InsertionBounds struct {
       EffectiveMinInsertionMm, MaxAllowedInsertionMm float64
   }
   func SeatpostInsertionBounds(frame FrameSetup, post SeatpostSpec) (InsertionBounds, error)
   func PlaceSeatpost(seatTubeTop pose.Pose, setup SeatpostSetup, bounds InsertionBounds) (SeatpostPlacement, error)
   ```

   Require finite dimensions, a positive C-T seat tube length, nonnegative frame limits, and `0 < post.MinInsertionMm ≤ post.MaxInsertionMm < post.TotalLengthMm`. The lower bound is the greater minimum. The upper bound is the smaller of the post maximum and the frame cap. Use the declared positive frame cap, or C-T length minus 20 mm when the cap is zero. Reject an empty interval. Equal endpoints permit one installation. Validate finite nonnegative setback and finite positive rail clamp length. Exposed length is `Spec.TotalLengthMm − InsertionMm`. Advance that distance to `ShaftTop`, then translate by setback on local +Y to `RailCradle`. Reject a nonfinite or out-of-range insertion with the renamed `InputSeatpostInsertion`. Keep the typed setback error used in fitting notes. Rename the landmark in both placement structs, diagrams and labels.
2. **P2.** In `assembly/bike.go`, derive bounds before placing the post, pass its cradle and spec clamp length to `PlaceSaddle`, and update `BikeSetup.RailOffsetLimitMm`. In `fitting.go` derive the bounds for the selected post before constructing the saddle probe in `ApplyFitting` and `FitPostSetback`. Probe at maximum insertion (shortest exposure). Reuse the existing increasing shaft-axis solve over `[0, maxInsertion − minInsertion]`, then return `maxInsertion − solvedTravel`. Clamp for floating-point roundoff as today. In the setback note, copy the spec by value, set the probe setback to zero, and solve the perpendicular setback as today. Preserve the rail range, ordered limit reporting, Euclidean nearest-point fallback and final forward placement. Stock fitting must carry each fork's fitted insertion into its candidates. A HandGrip-only fit retains the submitted seat assembly.
3. **P3.** In `web/fixture.go` store `Spec assembly.SeatpostSpec` on `seatpostOption` and `DefaultPostInsertionMm float64` on the fixture. Put the published 290 mm insertion cap on the Fenrir L, with frame minimum 0. Q75c gives the generic posts 400/365/100 mm (total/max/min, illustrative) with their existing 0/15/25 mm setbacks and 30 mm clamp. The Redshift uses 350/250/100 mm, 12 mm sagged setback and 30 mm clamp. Keep Q75c's individual provenance labels, including the illustrative linkage stack and Redshift clamp. Validate each option against the frame. At this intermediate commit 136.5 mm insertion on the generic default preserves the original 263.5 mm exposure and zero-height saddle geometry. S applies Q92 when the 45 mm height lands.
4. **P4.** Migrate the state codec, form echo, presets, error field mapping, fitting/setback readouts, stock Apply links and bike/seat diagrams in `web.go`, `templates/bike.html` and `web_test.go`. Use `post_insert`, `#ctrl-post-insert`, and insertion in mm. Show min/max for the selected frame and post and show exposed length as a separate readout. A post change keeps submitted insertion and validates it against the selected post. A 400 echoes the submitted value and offers correction with the new bounds. Use the existing retired-key convention exercised by `post_setback` for `post_ext`. Preserve all other query keys and the float round trip. Use the fitted insertion in Apply links, keeping the current selected post.
5. **P5.** Update direct placement and validation callers in `saddle_test.go`, `handlebar_test.go`, `fitting_test.go`, `stock_fitting_test.go`, and the web fixture tests. Replace the unbounded-extension test with explicit bounded cases. For Fenrir, expect generic [100, 290] and Redshift [100, 250]. For a generic post and 260 mm C-T with unspecified frame cap and frame minimum 125, expect [125, 240]. Test equality at both endpoints, rejection just beyond them, a frame minimum above the cap, nonfinite dimensions and a post with max insertion equal to total length. With level rails on the generic post, 60 mm rail length and 30 mm clamp still allow ±15 mm offset. Exercise a generic-to-Redshift switch carrying 275 mm insertion (400), then correct it to 250 (200, 100 mm exposed). Repeat an applied fitted build through view navigation and reload.

## S — Place the saddle surface above the rails

Commit subject: `Place the SQlab saddle landmarks above the rail cradle`

1. **S1.** In `assembly/saddle.go` add `SaddleSetup.RailToBrpHeightMm float64` (finite, nonnegative). From `RailCradle.Face(RailPitchDeg)`, advance by `−RailOffsetMm` to rail center. In this rail frame translate BRP by `(RailCenterToBrpMm, RailToBrpHeightMm)` and SitBones by `(RailCenterToSitBonesMm, RailToBrpHeightMm + SitBonesElevationMm)`. The height rotates with the rails. Set the fixture height to 45 mm, explicitly illustrative, from 55 mm effective height and 10 mm measured SIT elevation. Retain the measured/derived provenance for the other landmarks. In the seat close-up render the rail at rail center and the existing profile through the raised BRP/SIT poses. Frame the outlines and all landmarks for both level and pitched rails.
2. **S2.** Apply Q92. Set the fixture's default insertion to **180.576 mm**. Keep the generic 0 mm setback post, centered rails, level pitch and blank optional targets at this stage. The default insertion is a fixture setting, separate from an insertion found for a submitted BRP target. For 490 mm C-T and 400 mm post length, shaft-top distance from BB is 709.424 mm and exposed length is 219.424 mm. In the unpitched attitude, radial BRP height is 746.000 mm. Expect 746.008 mm on the CAD fork and 745.619 mm on the installed Lithic (±0.001 mm). Its installed BRP is (−155.065, 729.317) mm. Verify both fork states and distinguish the reference attitude in test comments.
3. **S3.** Strengthen the 60° STA worked example in `saddle_test.go`: 60 mm tube, 40 mm exposure, zero setback, +12 mm rail offset and level rails put BRP at `(−30, 50√3 + 45)` and SIT at `(−110, 50√3 + 55)` mm. At +10° rail pitch the added height alone shifts both landmarks by `(−7.814168, +44.316349)` mm compared with the zero-height model (±0.000001 mm). In web tests verify the seat-view SVG shows the rail below the profile, includes the cradle, BRP and SIT in its framing, and uses the same coordinates as the full-bike view.
4. **S4.** Rebaseline the fitting, setback and stock reference tests against the independent values below. Keep tests for the specific bound or outcome they originally exercised. For synthetic forward/upper-limit targets intended to exercise the old branches, translate the synthetic target up 45 mm on level rails. Keep the actual fit-report target fixed. Browserless tests should assert the displayed rounding and follow the link back to the exact fitted configuration.

## Independent reference values

The expected test values below come from a one-off JavaScript calculation using the Fenrir fixture dimensions. It computed coordinates independently of the Go assembly code, leveled the frame at the axles, found the closest BRP allowed by the insertion and rail limits, and scored the available stem and spacer combinations. With BRP at rail height it reproduced the existing test values for all seven fork settings to six decimal places. Repeating the calculation with BRP 45 mm above the rails gave the values below. Use ±0.000001 for six-decimal values, ±0.001 mm for three-decimal values, and existing exact assertions for bounds, discrete choices and HTTP status.

**Saddle fitting.** Fenrir L, installed Lithic, level rails, target (−219, 713.1) mm:

| Post or setback note | Insertion mm | Rail offset mm | Setback mm | BRP miss mm |
| --- | ---: | ---: | ---: | ---: |
| Generic 0 mm setback post | 183.315151 | +15 | 0 | 51.479453 |
| Same post with synthetic 27 mm setback | 183.315151 | +15 | 27 | 24.479453 |
| Redshift | 133.315151 | +15 | 12 | 39.479453 |
| Generic, solve setback with rails centered | 179.359870 | 0 | 65.948584 | 0 |
| Generic, solve setback with offset held at +15 | 183.315151 | +15 | 51.479453 | 0 |

For the generic 0 mm setback post, target minus fitted BRP is (−49.657530, −13.574380) mm. The fitting block displays 183.3 mm insertion, and the Apply link carries the full value. The current-miss readout refers to the submitted build. For the 25 mm post at (−189, 713.1), expect reached, 190.553573 mm insertion and +12.450978 mm rail offset. For the Redshift forward-limit regression use (−150, 758.1), giving −15 mm rail offset, a centered-setback note of 0 mm (12.475261 mm short) and a held-rail note of 1.993870 mm setback at 100.191488 mm insertion. For the generic upper-limit regression use (−219, 945), giving 100 mm insertion, centered setback 4.799942 mm (144.332896 mm short), and held-rail setback 0 mm at +15 mm (140.710227 mm short).

**Stock fitting.** For both report targets, the generic 0 mm setback post, default bar and all forks, use this table for the existing per-fork saddle checks:

| Fork | Sag % | Fitted insertion mm | BRP miss mm |
| --- | ---: | ---: | ---: |
| Lithic | 0 | 183.315151 | 51.479453 |
| SID | 15 | 183.311056 | 51.423215 |
| SID | 20 | 183.523736 | 54.266925 |
| SID | 25 | 183.748612 | 57.119058 |
| Fox | 15 | 182.548574 | 39.573856 |
| Fox | 20 | 182.745127 | 42.943626 |
| Fox | 25 | 182.958627 | 46.326087 |

All seven fit at +15 mm rail offset. The BRP line is `2 × (miss − 1)` points for these inputs. With only the Lithic and default bar the first row scores −22.246923 with 100.958906 BRP points. With all forks the first five are SID20/0, Lithic/0, Fox20/0, SID20/5, Lithic/5 (fork/sag and spacer mm), all −17° × 130 mm. Scores are −20.368593, −22.246923, −28.129898, −29.007048 and −30.976712. With the Lithic and all bars the existing top five bar/stem configurations each display −14.4. Across all forks and bars the five existing Fox20 configurations display −3.3, −4.3, −5.3, −5.3 and −5.3. Stock rows with only a grip target retain their existing scoring formula.

## Verification and scope

- After every code change run `go vet ./... && go test ./...`. Each of H, P and S must independently pass `make pre-commit`. Repeat until its fix pass is clean.
- On the final state run `make serve-reboot`. Inspect `/bike?view=seat` at level rails, then at `rail_pitch=10`. Record a seat-view screenshot and check the rail/profile separation and unclipped labels. Follow the CAD preset. Check the default and CAD radial heights above, the 400 and recovery from P5, and the first stock Apply link for `/bike?stock_forks=all&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726` (SID 20%, 183.524 mm insertion, +15 mm rail, score −20.4). Use ordinary GET requests and the existing Go web tests.
- Out of scope: closure allowances, catalog port, changes to the saddle-fitting objective or scoring formula, default rider targets, compare, and the legacy app.

## Rules that bite

- Keep the physical fixture in `internal/reboot/web/fixture.go`. Preserve chart numbers, CAD hypotheses, the measured fit targets and Q75c's per-component provenance. Use pose operations for all production geometry and name every constant in enum switches.
- Keep one insertion-bound calculation for placement, fitting, form bounds and diagnostics. Use a legal probe even when the submitted fitted inputs would be out of range. Preserve which inputs a fit is allowed to change. Signed scores and residuals in the table are deliberate.
- Reboot web code uses server-rendered HTML/SVG and GET. Use the existing `httptest` and `goquery` tests for form recovery, URL round trips and SVG assertions.
- Finish or stop with the implementation handoff required by [DESIGN.md](DESIGN.md#implementation-handoff). Put it immediately after the status paragraph, record H/P/S step IDs, commits, deviations, verification and open issues, and update the 12a milestone status. A new physical or product fork goes to Matt. Acceptance is his decision.
