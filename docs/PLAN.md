# SQlab Saddle Geometry and Rider-Fit Update Plan

Status: steps 1–3 completed on 2026-11-16. The provenance records, shared saddle data, derived defaults and shared forward/inverse rail geometry are updated; steps 4–7 remain planned.

## Established inputs and boundaries

- Treat [SQLAB_SADDLE.md](SQLAB_SADDLE.md) as the source for the owner's actual 14 cm SQlab 610 ERGOLUX active 2.1 saddle. TIP-to-SIT is the corrected 200 mm, not the superseded 215 mm.
- Matt physically measured SIT 10 mm above the W70 / BRP reference plane and confirmed that measurement on 2026-11-16. Keep `SitBonesElevationMm = 10`; do not call it unverified, replace it with zero, or infer it from nominal saddle height.
- Longitudinal measurements are parallel to the rails. The rails are closer to parallel to the ground than perpendicular to the seat tube. Their installed pitch has not been supplied numerically.
- Preserve the established history in [RIDER_FIT.md](RIDER_FIT.md): the source Redshift post was unsagged and the fitter transferred the same physical saddle to the jig. No measured compression or sag correction is available.
- Preserve the reported 219 mm BRP setback, 746 mm BB-to-BRP radial height, approximately `(-219, 713.1) mm` BRP coordinates and approximately `(481, 726) mm` cockpit coordinates. The cockpit's stem-clamp versus hand-grip interpretation remains a separate provenance question.
- SIT is a physical saddle landmark. A model preserving it does not independently measure Matt's pelvis, contact-pressure center, physiological leg extension, or preferred setback range.
- Keep the zero-dependency runtime, pure-Go compiled data, existing points-lost scoring costs and published frame geometry. Keep human saddle tools on the web and use GET forms with URL state if an installation control is necessary.

## 1. Clarify the saddle specification and rider-fit record (completed 2026-11-16)

- [x] Retain the newly documented measured elevation and cross-link the two records.
- [x] Clarify the W70 / BRP terminology: W70 names the saddle-local longitudinal station of the physical BRP landmark; bicycle-relative BRP coordinates describe that landmark after installation. BRP is not a term reserved for bicycle-coordinate values.
- [x] Keep W70's provenance explicit: its 120 mm station behind TIP is inferred from the reported 80 mm W70-to-SIT distance, pending direct width verification. The old 23 mm rail-center estimate is superseded by the canonical 32 mm value; their 9 mm disagreement is not a calibrated uncertainty interval.
- [x] Update `RIDER_FIT.md` to identify SIT, the corrected longitudinal geometry and the measured elevation. Replace the old derived 292 mm support setback with 299 mm under the level-rail approximation: `219 + 80 = 299 mm`. With the measured elevation, the corresponding modeled SIT point is approximately `(-299, 723.1) mm`.
- [x] Identify those SIT coordinates as model-derived under an orientation assumption, not a new fit-report reading, independent pelvic measurement, or demonstrated optimum. Preserve the riding observations and the untested preferred-setback range.

Acceptance met: both records distinguish owner measurements, fit-report readings, derived coordinates and installation assumptions, without questioning the confirmed 10 mm elevation. The unsagged same-saddle transfer, recorded report values and riding observations are preserved.

## 2. Consolidate the canonical saddle data and derived defaults (completed 2026-11-16)

Affected starting points: `internal/model/bike_components.go`, `internal/catalog/saddles.go`, `internal/ideal/loader.go`.

| Quantity | Before step 2 | Current value in the program's positive-forward convention |
| --- | ---: | ---: |
| Rail center to W70 / BRP | +23 mm | +32 mm |
| Rail center to SIT | -50 mm | -48 mm |
| W70-to-SIT rearward span | 73 mm | 80 mm |
| SIT elevation | +10 mm | +10 mm, owner-measured |
| Marked rail span | 60 mm | 60 mm |

- [x] Use one shared compiled saddle definition for the catalog and idmatch reference, without introducing an import cycle.
- [x] Keep the specification and program on the same positive-forward / positive-upward convention. Relative to R0, R+30 is at `+30 mm` and R-30 at `-30 mm`; the specification's signed clamp coordinate has the same sign as the program's rail offset. Distinguish unsigned measurement lengths from signed coordinates, and label TIP-relative versus rail-centered origins explicitly.
- [x] Derive the default support target from the reference saddle and recorded BRP instead of maintaining an independent `292` or `299` constant. Replace all production uses, help hints and explanatory literals that would otherwise diverge.
- [x] Keep geometry for the other catalog saddles unchanged; recalculate their translated targets from the corrected reference.

Acceptance met: all default consumers use the same reference data and derive a level-rail 299 mm SIT setback while retaining a 219 mm reference BRP setback and 746 mm radial BRP height. The catalog uses `model.ReferenceIdmatchSaddle()`; `model.DefaultTargetSitBoneSetbackMm()` derives the default support setback. Regression checks cover catalog/reference agreement, profile and translated targets, preserved report anchors, and both saddle pages' derived defaults and hints. Existing derived-output assertions were refreshed; the rail-placement equations remain unchanged for step 3.

## 3. Correct rail placement and inverse geometry (completed 2026-11-16)

Affected starting points: `internal/model/bike_components.go`, `internal/geometry/scene.go`, `internal/geometry/saddle.go`, `internal/geometry/saddle_swap.go`, `internal/geometry/analysis.go`.

- [x] Separate the seatpost's existing perpendicular setback convention from saddle displacement along the rails. Stop adding rail offset and rail-to-BRP distance directly to seatpost perpendicular setback.
- [x] Define one shared saddle-local transform for rail center, W70 / BRP, SIT and clamp translations. Use positive forward along the rails and a clearly documented elevation convention. At the level-rail reference, BRP-to-SIT is `(-80, +10) mm`.
- [x] State the rail-pitch convention explicitly. Use a named level-rail approximation for the reference attitude, not a claim that Matt measured zero pitch. Keep saddle orientation independent of STA. Assume the rider readjusts saddle tilt at the clamp for each fork configuration: rails stay approximately level relative to the ground regardless of changes in frame attitude or STA. Any explicit nonzero rail pitch is also relative to horizontal, not the frame; do not make the rails perpendicular to each active seat tube.
- [x] Do not invent absolute rail-to-cover or clamp-to-cover heights from the nominal 55 mm effective height. Where the model needs an unmeasured vertical datum, document the approximation and distinguish inferred hardware coordinates from measured landmark separation.
- [x] Retain the exact radial BRP-height constraint when solving seatpost extension for each rail candidate. Handle impossible geometry explicitly rather than producing a plausible coordinate by silently clamping an invalid solution.
- [x] Update forward placement and inverse rail-offset / ideal-STA solutions together. Route scene construction, analysis, saddle swaps and stock selection through the same geometry, removing duplicate equations that implement the old seat-perpendicular approximation.
- [x] Review fields such as `TotalPerpOffsetMm` whose meaning becomes misleading when rail and seatpost offsets use different axes; replace or redefine them and their callers consistently.

Acceptance met: `model.SaddleTransform` places rail center, BRP, SIT and clamp stations on a shared rail axis. `CalculatePoint` solves the positive seat-axis extension at the exact radial BRP height; impossible placements return errors and are omitted from candidate sets. Scalar inverse solvers return NaN for impossible targets instead of clamping them. Scene construction, analysis candidates, stock-selection inputs, ideal-STA/compromise solutions and saddle swaps use this geometry. The misleading `TotalPerpOffsetMm` and legacy perpendicular saddle-offset fields/helpers are removed; each placement carries its transform, projected clamp and solved `SeatAxisDistanceMm`.

Adopted installation convention: positive rail pitch raises the nose. The named `LevelRailPitchDeg` reference is an approximation, not a measurement of Matt's installation. `SaddleSpec.RailPitchDeg` is the saddle model’s assumed rail angle above horizontal, independent of frame attitude and STA. The model assumes the rider readjusts saddle tilt at the clamp for each fork configuration, keeping the rails level by default. Fork-induced frame rotation is never added to rail pitch; a saddle model with a nonzero assumed pitch also stays fixed relative to horizontal. There is no per-setup pitch setting or separate pitch argument to the geometry functions. Local X is forward along the rails and local Y is normal toward the cover. Thus `(-80, +10) mm` is the reference BRP-to-SIT displacement before rotation. Rail center and clamp coordinates are projected datums in the BRP plane, not measurements of the hardware's absolute vertical position; no rail-to-cover height is inferred from 55 mm nominal saddle height.

The placement equation is `BRP = axial * seatAxis + postSetback * seatNormal + (railToBrp - railOffset) * railAxis`. The positive circle intersection solves `|BRP| = saddleHeight` without adding offsets from different axes. Saddle translations recompute both rail position and extension to preserve the modeled support landmark, including different saddle elevations. Their feasibility remains conditional on the assumed rail segment and clamp footprint.

Independent analytic tests cover multiple STAs, level and nonzero pitches, TIP-origin translation, exact BRP radius, forward/inverse rail and STA round trips, level rails despite fork-induced frame rotation, scene/analysis candidate agreement, support-preserving swaps and impossible geometry. Existing derived-output assertions were refreshed: for example, the reference zero-post ideal STA is about 70.61°, and the level-rail SQlab-to-Aspin swap on its source-centered zero-post axis needs approximately 0.0 mm of target rail offset rather than treating the +3.5 mm local longitudinal delta as the installed translation. The Sunliner L / Havok XL / Havok L comparison fixture now rounds to 3 / -149 / -150 points instead of 1 / -151 / -152; scoring costs and bands are unchanged.

Verification: `go vet ./... && go test ./...`, `make pre-commit` (including exhaustive and both deadcode checks), and the runtime-dependency audit pass. No manufacturer frame geometry, report measurements or scoring costs changed. Step 4 still owns the target/selection/ledger propagation audit, including explicit nonzero saddle-pitch targets and selected-build metrics; step 5 still owns diagram reconstruction and presentation/hardware-assumption cleanup. W70 width verification, measured installation pitch and the cockpit-reference provenance question remain unresolved.

## 4. Propagate the correction through targets, selection and scoring

Affected starting points: `internal/ideal/loader.go`, `internal/canonicalizer/canonicalizer.go`, `internal/geometry/analysis.go`, `internal/web/ledger.go`.

- Preserve the report's reference BRP and cockpit anchors. The current cockpit target is 700 mm from BRP and 3 mm above SIT, corresponding to 13 mm above BRP with the measured 10 mm elevation. Keep that relationship explicit and do not move the recorded cockpit coordinate as an accidental consequence of changing saddle geometry.
- Carry targets to other saddles through the modeled support landmark, consistently with the shared transform. At the level-rail reference, equivalent BRP setbacks are 211 mm for Aspin and 216 mm for SLR Boost, replacing 204 and 209 mm.
- Recompute equivalent radial heights, cockpit distances, rail requirements, ideal STA, fit gates, selected stock builds and comparison totals from the corrected geometry. Do not preserve obsolete results by changing scoring costs, ideal bands, tie rules or published frame dimensions.
- Update rail-center-to-SIT differences: Aspin versus SQlab becomes `+3.5 mm`, and SLR Boost versus SQlab becomes `-17.3 mm`. Recompute full placement and feasibility rather than treating these longitudinal deltas alone as exact installed translations.

Acceptance: documented report anchors stay fixed, derived saddle targets update coherently, and any ranking changes can be traced to geometry and the existing points-lost ledger.

## 5. Update diagrams, descriptions and hardware assumptions

Affected starting points: `internal/web/saddles.go`, `internal/web/saddle_swap.go`, `internal/web/pages.go`, `internal/web/bike_diagram.go`, `internal/web/ledger.go`, `internal/ideal/readme.md`.

- Draw rail windows, clamp, rail center, BRP and SIT using the shared transform. Remove the diagram's independent reconstruction of rails as perpendicular to the seat tube. Explain the coordinate calculation in the ledger using the same convention.
- Replace claims such as “True Pelvic Alignment”, exact physiological leg-extension preservation and unconditional framebuilder guarantees with descriptions of preserving the modeled support landmark. Name any BB-to-SIT distance as a geometric proxy.
- Generate saddle comparison explanations from the current data instead of hard-coded `-50`, 73 mm, 5.5 mm or STA examples. Refresh all derived heights, cockpit distances and compromise-STA examples after the geometry correction.
- Distinguish the measured 60 mm marking span from verified usable clamping length. Label windows calculated from rail span and clamp footprint as conditional on those assumptions. A marking endpoint alone does not prove a particular clamp fits there.
- Review the incompatible-clamp case: a clamp longer than the assumed usable rail segment must not be reported as feasible merely because the current window helper returns `[0, 0]`.
- Label the fixed Redshift static/sagged and Redshift-plus-Drop-Best examples as hardware assumptions or illustrative cases, not measurements of Matt's setup. Audit the offset arithmetic against the documented nominal 27 mm adapter offset while keeping horizontal and seat-perpendicular conventions distinct. Do not apply an invented sag correction to the transferred fit.
- Refresh form hints and descriptions that currently say “default 292”. Clarify that the default zero-setback dropper is a comparison assumption, not Matt's confirmed Redshift equipment.
- Update `internal/ideal/readme.md` with the new data, formulas, terminology and provenance. Replace obsolete saddle CLI references with the existing web routes; do not add saddle CLI commands.

Acceptance: diagrams and ledgers agree numerically with geometry, all derived prose reflects current data, and modeled fit or hardware feasibility is not presented as independently measured rider fit or verified clamp compatibility.

## 6. Record the Seven fit-sheet discrepancy

- The existing Seven provenance in `internal/ideal/readme.md` and `internal/frames/seven_custom.go` cites a 245 mm saddle length; Matt's actual saddle measures 280 mm. Record this unresolved discrepancy alongside the associated nose-setback and tail-coordinate claims.
- Preserve each source's reported values and distinguish them from coordinates derived from the measured saddle. Do not silently reconcile the records, assume a saddle substitution, or alter the confirmed same-saddle idmatch transfer history.
- If updating the frame's provenance note, change only explanatory text. Leave every published or already-recorded frame geometry field unchanged.

Acceptance: both sources remain attributable and the discrepancy is visible without fabricated replacement measurements.

## 7. Regression checks and final verification

- Add meaningful geometry checks for the shared positive-forward convention, TIP-to-R0 origin translation and `(-80, +10) mm` displacement, shared reference data, and derived default target.
- Use independent analytic cases to check level rails at multiple STAs and a declared nonzero rail pitch, including the assumption that the rider readjusts saddle tilt to keep rails level across fork configurations. Verify rail offsets and landmark separations follow the rail transform rather than the seat-tube perpendicular.
- Check radial BRP height, forward/inverse rail and STA round trips, exact modeled support-landmark preservation on saddle swaps, and unchanged report BRP/cockpit anchors.
- Cover valid clamp windows and an overlong incompatible clamp without claiming that the marked span independently proves usable rail length.
- Update existing geometry, ideal-profile, canonicalizer, scoring, CLI and browserless web assertions affected by the correction. Keep arbitrary synthetic test inputs distinct from obsolete reference-fit expectations; do not blindly replace every `292` in the repository.
- Verify `/saddles`, `/saddle-swap` and `/bike` descriptions, defaults, diagram coordinates and ledger agreement using the existing `httptest` / goquery approach. No browser automation.
- After every code modification, run `go vet ./... && go test ./...`.
- Before completion, run `make pre-commit`; review any `go fix` changes and repeat until stable. Keep modernization output separate from feature work if committing.
- Verify runtime dependencies with `go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/bikegeo`; only `bikegeo/...` packages may appear.
- Review the final diff for unchanged frame geometry, report measurements and scoring costs. Record intentional derived-output changes and unresolved W70, installation-pitch and source-document questions.

Suggested implementation order: provenance and shared data; shared forward/inverse geometry; target and selection propagation; presentation and source-discrepancy cleanup. Keep each change internally consistent and passing the required feedback loop.
