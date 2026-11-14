# SIMPLIFY_C — assessment of `bikegeo`

Scope: read-only review of the repository at `db9e574` (18,060 non-test Go lines, 9,193 test lines, 19 frames, 10 web routes, 4 CLI commands). No source was changed. `go vet ./... && go test ./...` passes as-is.

Method: I read the model, geometry, canonicalizer, scoring, ideal, validator, compare, and fit packages in full and skimmed the web layer; then I ran the built binary and wrote two throwaway probe programs (outside the repo) that rebuild the same quantities by an independent construction. Numbers quoted below come from those runs unless marked as an estimate.

---

## 1. Summary

The program answers two questions:

1. **Is this published chart self-consistent, and what fork was it drawn around?** (`validate`, `fit`)
2. **If I put my contact points on this frame, what does the bike look like, and can ordinary hardware get me there?** (`compare` and the web pages)

Question 1 is implemented rigorously. Question 2 is implemented as a long pipeline of scalar formulas, selection heuristics, and a composite score. That pipeline is where most of the code, most of the special cases, and all of the correctness problems I found live.

The recommendation, in one paragraph: **replace the scalar pipeline with one explicit 2D point model of the bike ("the drawing is the model"), make every reported number a measurement between named points, replace the fit *score* with a fit *solve* (which stem, how many spacers, what rail position), and draw the points as a server-rendered SVG side view.** This removes roughly two thirds of the non-data code (estimate, section 5), makes several of today's bugs impossible to express, and gives the transparency asked for almost for free, because the drawing and the table are produced from the same dozen points.

Before any of that, three findings deserve attention because they change today's output:

| # | Finding | Size of effect | Status |
| --- | --- | --- | --- |
| C1 | The handlebar reference point the tool measures to is about 70 mm behind the point that the Seven Custom CAD note (and so, apparently, the idmatch target) uses | ≈ 70 mm of reach on every frame | Verified arithmetic; which definition is right needs your idmatch report |
| C3 | A fork swap ignores the change in rake, so the "pitched" bike does not have both wheels on the ground | 2.1–3.6 mm; HTA off by 0.1–0.2° | Verified |
| C4 | Comparisons do not apply the physical lower headset cup, although the validator says they do | Fenrir and Fargo: ≈ 0.55° HTA, 6 mm reach, 4 mm stack, 4 mm BB drop | Verified against the code; intent needs your call |

---

## 2. What is solid (keep it)

- **Rounding budgets** (`pkg/geometry/budget.go`). Perturbing each published input by its rounding half-width through the real closure equation is the right idea, is small (263 lines), and replaces arbitrary tolerances. This is the best part of the program.
- **The pitch rotation itself.** For a fork swap with unchanged rake, the tool's reach, stack, HTA and BB drop match an independent four-point construction to 0.01 mm (Fenrir L, 495 → 506 mm A2C).
- **`pkg/curve`** has property tests that make flats and jumps unrepresentable. If scoring survives, this is the right foundation.
- **Frames as Go data with one automatic test** (`TestAllFrameData`). Adding a frame needs no test boilerplate.
- **Web 1.0 discipline.** URL is the state; no JavaScript; in-process tests. The side view proposed in section 4 fits inside these rules.
- **Zero runtime dependencies**, enforced.

---

## 3. Correctness findings

Ordered by how much they move the output.

### C1. The handlebar reference point disagrees with the frame's own CAD note by about 70 mm

The idmatch numbers compiled into the code are: saddle height 746 mm (BB to BRP, radial), BRP setback 219 mm, BRP–HB 700 mm (ideal 695–705), HB rise +3 mm above the sit bones (ideal −2 to +8), which is +13 mm above the BRP on the SQlab saddle. That puts the BRP at `(−219, 713.1)`.

`pkg/frames/seven_custom.go` records the builder's cockpit point: "HX 481mm, HY 727mm with 70mm -6° stem, 0mm spacer, and 50mm rise handlebar".

- Distance from the BRP to `(481, 727)` is **700.1 mm**; rise above the sit bones is **+3.9 mm**. Both sit in the centre of the ideal ranges. The frame was evidently drawn to this fit.
- For the same hardware the tool computes clamp `(485.1, 666.7)` and hand point `(412.3, 712.2)`. Measured from a BRP at 219 mm setback that is **631 mm, 69 mm short**.

The gap decomposes cleanly:

| Component | ΔX | ΔY | Source |
| --- | --- | --- | --- |
| Backsweep of the Persuader 20° bar, applied by the tool, absent from the CAD point | −72.8 | — | `GetHandlebarOffset` |
| Rise at the hand: tool uses 45.5 mm, CAD note uses 50 mm | — | −4.5 | bar-end rise convention |
| About 11 mm of stack between head-tube top and stem that the tool omits (see C2) | +3.8 | −10.3 | `CalculateHBPointWithHandlebar` |

Consequences visible in the output today:

- Seven Custom, the frame built to this fit, scores **23–26 out of 100** with "Oval Fits 0" and "reach: 39mm short" even after the picker selects the longest stem in the catalog.
- In `bikegeo compare all`, **11 of 19 frames peg the 100 mm stem** (the catalog maximum) and **11 of 19 have zero setups in the oval**. The largest sizes rank first. That pattern is what a constant ≈ 70 mm reach bias looks like.

I cannot tell from the repository which definition idmatch used. The two readings:

- **(a) The 700 mm target is to the bar centre at the stem** (the usual HX/HY convention), with rise added. Then the tool should not apply backsweep when comparing against it, or the target must first be translated by the offset of the bar that was on the fit bike. Most cockpit scores move by about 70 mm.
- **(b) The 700 mm target is to where the hands are.** Then the tool is right, and the CAD point reproduces the target only if backsweep is ignored: with the specified 70 mm stem and that bar, the grips would sit about 70 mm behind the fit position. The CAD preview is dated 2026-11-11, so if (b) is the case it is worth raising with the fitter now.

Either way one of the two is off by about 70 mm, and nothing in the current output makes that visible. A side view with the target point drawn on it would have shown it at a glance; this finding is the strongest argument for section 4.

### C2. No stack is modelled between the head-tube top and the stem

`CalculateHBPointWithHandlebar` places the stem centre at `spacers + StemThicknessMm/2` above the head-tube top. Real bikes have an upper headset cup and top cover there. The Seven note calibrates it: the tool's clamp matches the CAD point when **≈ 11 mm** is added (`(481.3, 677.1)` versus HX 481, HY 727 − 50). The effect is common to all frames (clamp ≈ 4 mm further back and ≈ 10 mm higher), so rankings barely move, but the HB-rise ideal window is only ±5 mm wide.

Suggested fix: a `HeadsetTopStackMm` on the frame (or one constant per cup type, next to `CupStackMm`), stated in the page header like the seatpost assumption is.

### C3. Fork swaps ignore the change in rake, so the wheels are not level

`CalculateDynamicGeometry` rotates the frame for the change in A2C only. The stack equation it is inverting, `Stack = BB_drop − Rake·cos(HTA) + (A2C + spacer + HTL)·sin(HTA)`, also contains rake. Most comparison forks have 44 mm rake while several CAD forks have 51–55 mm.

Checking the tool's own output for closure (front-axle height computed from its pitched reach, stack and HTA, versus its pitched BB drop):

| Setup | CAD fork | Compared fork | Front axle off level | Tool HTA | Rebuilt HTA | Tool reach / stack | Rebuilt reach / stack |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Fenrir L | 495 / 54 | 495 / 54 | +0.2 mm (chart rounding) | 68.25° | 68.26° | 428.0 / 642.0 | 428.1 / 641.9 |
| Fenrir L | 495 / 54 | 506 / 44 | −3.6 mm | 67.75° | 67.57° | 422.3 / 645.7 | 420.3 / 647.0 |
| Seven Custom | 492 / 52 | 506 / 44 | −2.5 mm | 69.34° | 69.21° | 416.7 / 635.8 | 415.3 / 636.8 |
| Seven 926f S 60cm | 472 / 55 | 506 / 44 | −3.5 mm | 68.39° | 68.21° | 390.8 / 651.2 | 388.8 / 652.5 |
| Salsa Fargo L | 483 / 51 | 486 / 44 | −2.9 mm | 68.86° | 68.71° | 386.4 / 662.0 | 384.7 / 663.0 |
| Surly Krampus L | 483 / 47 | 506 / 44 | −2.1 mm | 67.93° | 67.82° | 436.5 / 620.3 | 435.3 / 621.1 |

The error is the same size as one sag step (20% versus 25% of 100 mm is 5 mm of A2C), which the tool treats as significant. It is also a clean example of a bug the proposed point model cannot have: there the front axle is a point, and "both axles at the same height" is the definition of the solve rather than a formula to keep in sync.

### C4. The physical headset cup is not used in comparisons

`validator.go:209` tells the user: "Validation uses geo_headset_spacer_mm to verify the published chart, while comparisons pitch the frame using the physical headset_cup_type stack." The comparison path does not: `AnalyzeBikeFitWithSaddle` uses `GeoHeadsetSpacerMm` for both the baseline and the target, and `CupStackMm()` is read only for display.

For most frames the CAD spacer and the cup stack agree within 1–2 mm. Two do not: **Otso Fenrir** and **Salsa Fargo** are `EC44` (12 mm) with a CAD spacer of 0. If those declarations are right, the physical bikes sit 12 mm taller at the crown than the tool evaluates. For Fenrir L with its own rigid fork:

| | Tool (chart) | With 12 mm cup |
| --- | --- | --- |
| Reach | 428.0 | 421.8 |
| Stack | 642.0 | 646.1 |
| HTA | 68.25° | 67.70° |
| BB drop | 72.0 | 67.7 |
| Trail | 90.3 | 94.2 |

The Fenrir is the rider's baseline bike and appears four times in the default comparison. Decide which behaviour is intended, then make the code and the message agree. "Apples-to-apples" argues for the physical cup.

### C5. "Front Wt" does not depend on the bike's cockpit

`CalculateBaselinePosture` (≈ 120 lines of anthropometry: torso 0.288 × height, arm 0.380 × height, segment masses 58/10/32%, a 35 mm hip offset, a 25° elbow) runs once on a hard-coded Otso Fenrir L with a 110 mm stem. Its only output that reaches the comparison is the system centre of gravity. I recovered it from four different frames: **−13.60 mm from the BB in every case**, back angle 53.6° in every case.

So `Front Wt % = (rear centre − 13.6) / wheelbase`. It carries weight 4 in the score, and "Modeled Back Angle: 53.6°" is printed in every header although it describes no bike in the table. Either replace it with an honest, labelled ratio (rear centre / wheelbase, or BB position as a fraction of wheelbase), or compute posture per bike from the actual hand and hip points once the point model exists. I would do the first and delete the posture model.

### C6. The rider's fit is scattered as literals

The same few numbers appear as bare literals across seven packages: `746.0` and `219.0` about ten times each, `292.0` four times, `700.0`/`13.0`/`3.0` several, plus the Redshift 11 mm seatpost setback as a package constant. `catalog.DefaultRiderProfile()` says saddle height **780**, which `compare.Run` then overwrites with the profile's 746. The saddle translation helpers are called with a literal `746.0` rather than the comparison's saddle height, so the two can only agree by coincidence of both being constant.

Today there is no way to change saddle height in `compare`, so this is latent rather than active. Fix by construction: one `Fit` value (`SaddleHeight`, `BRPSetback`, `HandReach`, `HandRise`, reference saddle, reference bar), passed explicitly, no defaults inside geometry functions.

### C7. Missing inputs silently become invented numbers

About two dozen sites replace a missing value with a plausible constant and carry on:

| Value | Fallback | Where |
| --- | --- | --- |
| Wheelbase | 1150 | `geometry.go` 719, 976, 1073 |
| Chainstay | **430** in two places, **440** in two others | `geometry.go` 722, 1077 / 967, 1141 |
| Head tube length | 120 | `geometry.go` 1151 |
| Fork rake | 44 | `geometry.go` 1267, `canonicalizer.go` 91, 315, 333, 502 |
| STA / HTA / reach / stack | 74.71 / 67.71 / 421.9 / 646.0 | `geometry.go` 854–876 |
| Saddle height / sit-bone setback / HB distance / HB drop | 746 / 292 / 700 / 13 | `saddle_swap.go` 106–120 |
| Ellipse semi-axes | 10 / 20 | `geom2d.go` 369–375 |

`hbDropMm == 0` being replaced by 13 means a level bar cannot be entered. The validator already rejects frames with missing essentials, so most of these can never fire for registry data; they exist to keep hand-built test structs alive. They should be errors. A wrong answer that looks right is the worst failure this program can have.

### C8. Derived values are stored as published ground truth

AGENTS.md calls published chart numbers "sacred ground truth". Three frames' notes say their wheelbases were computed "from horizontal rear center plus front center" (Moots Mountaineer, Seven 926d, Seven 926f). The validator then confirms that wheelbase equals rear centre plus front centre. That check is circular for those frames, and it reports success.

The cause is structural: the validator lists Wheelbase as a required field, which forces fabrication when a maker does not publish it. Related: chart precision is inferred from float literals, so a one-size frame with `HtaDeg: 70` is assumed rounded to the nearest degree (±0.5°, contributing ±2.4 mm to the A2C budget) even though the Seven note says 70.0°.

Suggested fix: make Wheelbase optional (the model computes it), and let each frame declare what the chart actually printed, for example `Published: Precision{Hta: 0.1, Length: 1}`, with inference only as a fallback. Declared data beats inference.

### C9. Cockpit selection is implemented twice and executed up to four times

`canonicalizer.ApplyIdealCockpitTargets` (lines 680–767) and the first half of `scoring.ScoreRow` (lines 115–189) are the same ≈ 70 lines: read targets out of the profile by string ID, translate them for the saddle, pick a BRP, re-map handlebar points, pick a stem. `PruneRows` calls both; `BuildDocument` calls both again. `ScoreRow` performs the selection on a copy and discards it. Any change must be made in two places to keep the displayed stem and the scored stem the same.

### C10. Typed facts are re-derived from strings

- `ForkRef.ForkTravel()` parses travel out of the fork **ID** (`"fox34_120"` → 120) by looking for a number between 60 and 200, although the catalog entry has `TravelMm`.
- Rigid versus suspension is decided by whether the **display label** starts with "rigid" (`IsRigidFork`), and fork type by splitting the label at "@".
- `ForkRef.A2cMm` means sagged A2C when `SagPct > 0` and static A2C otherwise. `{Fork: "pike_120", SagPct: 30, A2cMm: 495}` states one fact three ways that must be kept consistent by hand.
- `DefaultComparisonState` hand-types derived values (`A2cMm: 499, DisplayLabel: "110mm @ 20% sag"`) that the catalog could compute.
- Attribute IDs are matched as lower-cased strings with alias lists repeated at 18 sites in five files (`"brp_setback" || "brp" || "brpsetback"`). `ideal.LookupAttribute` accepts about 55 aliases. Profiles are compiled in, so no input exists that needs this leniency.

AGENTS.md already has the right rule for zones ("never infer them from strings"). Apply it everywhere: an enum for attribute IDs, a `Fork` value with `TravelMm`, `StaticA2c`, `Rake`, and a `Sag` on the setup.

### C11. Smaller items

- **Stem penalty slope.** `curve.StemPenalty` says it grows "more steeply" beyond the range. Measured: 0.98 points/mm just inside the boundary, 0.55 points/mm just outside. Continuous and increasing, but the slope halves at the boundary.
- **"Oval Fits" cell.** The cell shows a count, the zone marker comes from the sub-score, and the Ideal row reads "≥100". The default output has `2 ✗` (Krampus XL) and `2 ~` (Hayduke S3) in the same column.
- **Seatpost Ext** is `saddle height − seat tube length`: a radial BB-to-saddle-top distance minus an axial tube length whose convention (centre-to-top or centre-to-centre) varies by maker. The footer describes it as "exposed seatpost length from collar to saddle rails". It is scored with weight 4.
- **`HasBentSeatTube`** means `ActualStaDeg > 0 and differs from nominal` on `BikeGeometry` but only `ActualStaDeg > 0` on `FrameModel`. The effective-STA formula exists three times, the BRP point formula twice.
- **`FrameValidationReport.ErrorSummary`** ranges over a map and returns the first message, so the error text can differ between runs. (AGENTS.md records a similar ordering issue in `fit` as "pre-existing".)
- **Pruning chooses sag by score.** With `@all`, the sag (20/25/30%) that scores best is kept per frame. Sag is a suspension setting, not a fit dial; this flatters each frame differently.
- **Wheel radius** is 372 mm for every 29er regardless of tire (the comment calls it "loaded" but 311 + 60.96 is the unloaded figure). Trail moves ≈ 0.4 mm per mm of radius, and the trail ideal window is 8 mm wide. Fine as a stated normalisation; it should be stated.
- **Unused code.** By grep, roughly 25 exported symbols are referenced only by their definition or by tests: for example `geom2d.Transform2`/`RotationForLift`/`AngleFromLift` (a rigid-transform type the pitch code does not use), `GripToHB`, `HBToGrip`, `CalculateHandlebarOffset`, `ClampOffsetFromBrp`, `ForkSpec`, `geometry.CalculateEffectiveSta`, `geometry.MapPossibleBRPs`, and several constants. Confirm with `deadcode` before deleting.

---

## 4. The simplification: one point model, drawn

### 4.1 The model

Everything this program computes is rigid-body geometry in one plane. Today it is spread over `BikeGeometry`, `BaselineMetrics`, `DynamicGeometryResult`, `BikeFitAnalysis`, `BRPPoint`, `HBPoint`, `CockpitFitResult` and `CanonicalComparisonRow`, each holding some derived scalars. Replace them with one function that returns named points:

```
Build(frame size, fork, cockpit, seat, fit) -> Layout   (or an error; never a default)
```

```
                         GR o<----- bar offset ---o CL
                                                 /
                  SB o                          / stem
               BRP o  \ saddle                 o SC
                    \                          |  spacers + headset top
                     \                         o HT  (Reach, Stack)
                      \  seat tube            /
                       \                     /  steerer, HTL + cup + A2C
                        \                   /
          RA o-----------o BB              /
           ( )                            o--o FA   (rake, perpendicular)
            |                                ( )
   ---------o--------------------------------o--------- ground
           RC                                FC
```

Inputs, each a small typed value:

| Input | Fields |
| --- | --- |
| Chart row | Reach, Stack, HTA, STA, HTL, BB drop, chainstay, (optional published wheelbase, front centre) |
| Fork | static A2C, rake, travel |
| Setup | fork, sag %, lower cup stack, wheel radius front and rear |
| Cockpit | headset top stack, spacers, stem length, stem angle, stem clamp height, bar, roll |
| Seat | post setback, saddle, rail offset |
| Fit | saddle height, BRP setback, hand reach, hand rise, reference saddle, reference bar |

Construction, in chart coordinates with the BB at the origin, X forward, Y up:

```
u  = (−cos HTA,  sin HTA)            up the steerer
n  = ( sin HTA,  cos HTA)            forward, perpendicular to the steerer
s  = (−cos STA,  sin STA)            up the seat tube
r  = (−sin STA, −cos STA)            rearward, perpendicular to the seat tube

BB = (0, 0)
HT = (Reach, Stack)
RA = (−sqrt(Chainstay² − BBdrop²), BBdrop)
FA = HT − (HTL + cup + A2C_static − travel·sag)·u + rake·n

level:  θ = −atan2(FA.y − RA.y, FA.x − RA.x); rotate every point by θ about RA

SC = HT + (headsetTop + spacers + stemHeight/2)·u
CL = SC + stemLength · rotate(n, stemAngle)
GR = CL + barOffset(bar, roll)

p   = postSetback + railOffset − saddle.RailCentreToBRP
BRP = sqrt(H² − p²)·s + p·r
SB  = BRP + saddle.BRPToSitBones
```

(The cockpit and seat points are built on the rotated frame; written here before rotation for brevity. A bent seat tube changes only `s` and its origin.)

Every reported number is then one measurement:

| Metric | Measurement |
| --- | --- |
| Reach, Stack | `HT − BB` components |
| HTA, STA | chart angle − θ |
| BB drop | `RA.y − BB.y` |
| Wheelbase | `FA.x − RA.x` |
| Front centre, rear centre | `FA.x − BB.x`, `BB.x − RA.x` |
| Trail | `(R·cos HTA − rake) / sin HTA` (or: ground intercept of the steering axis minus the contact patch) |
| BRP setback | `BB.x − BRP.x` |
| Hand reach, hand rise | `distance(GR, BRP)`, `GR.y − SB.y` |
| ETT | X distance from `HT` to the seat-tube line at `HT.y` |

The validator becomes the same build with the CAD fork and no rotation: `FA.y − RA.y` is the stack-equation residual, `FA.x − RA.x − Wheelbase` the wheelbase residual, `distance(FA, BB) − FrontCenter` the front-centre residual. The rounding-budget code perturbs inputs through this one function instead of three hand-written residuals.

Why this is "obviously correct":

- A fork swap is "change the fork value and rebuild". There is no pitch formula to forget a term in (C3), and the cup is an explicit input (C4).
- Invariants are testable for every frame, fork and sag in the registry in one loop: both axles level after the solve; all distances between frame points unchanged by the solve; building with the CAD fork reproduces the chart within its rounding budget; flipping two setups and flipping back is the identity.
- The table and the picture cannot disagree, because both read the same `Layout`.

### 4.2 Replace the fit score with a fit solve

The question behind the oval, the membership sum, the versatility bonus, the stem penalty, the six-level tie-break and the weighted total is: *can I put my hands and saddle where the fit says, with normal parts?* That has a direct answer.

```
target hand point   G* = BRP* + (sqrt(D² − h²), h)        D = hand reach, h = rise above BRP
required clamp      C* = G* − barOffset(bar, roll)
for each stem angle a:   solve   HT + t·u + L·rotate(n, a) = C*   for (t, L)    — two linear equations
        stem length = L,   spacers = t − headsetTop − stemHeight/2
required rail offset: the existing closed form (RailOffsetForBrpAtSta)
```

Output per frame, in the units the rider shops in:

```
Seven Custom       −6°: 71 mm stem, 0 mm spacers     rail +13 mm      all within range
Otso Fenrir L      −6°: 118 mm stem, 22 mm spacers   rail +28 mm      stem > 100, rail > +15
```

(Illustrative values, not computed.) Flags replace zones: within range, at the edge, out of range. Sorting is by any column the reader chooses. If a single ordering is still wanted, "millimetres outside the hardware limits" is a one-line, explainable key.

What this removes: the fit oval, `EvaluateCockpitFit`, `SelectBestHB*`, `SelectBestBRP`, `MapPossible*`, the versatility and stem-penalty curves, weights, floors, edge scores, "Biggest Losses", "Head-to-Head", the breakdown table, and the two duplicated selection blocks (C9). It also removes the `±1–2 point` rounding caveats the breakdown has to print.

If a score must stay, keep `pkg/curve`, score only the few solved quantities (stem length versus preferred, spacers, rail offset), and drop everything that is not a property of the frame-plus-fit (front weight, seatpost extension) to display-only.

### 4.3 The side view (transparency)

A new page, for example `/bike`, whose response is an inline `<svg>` written by `html/template` from the `Layout`. No JavaScript, no state outside the URL, so it stays inside the Web 1.0 rules in AGENTS.md and is testable in process like the other pages.

**What is drawn**

- Ground line, both wheels, contact patches.
- Frame: BB, rear axle, head tube, seat tube line, the Reach/Stack right angle.
- Fork: steering axis extended to the ground, crown, rake offset, trail marked on the ground.
- Front end: lower cup, head tube, headset top, spacer stack, stem, bar clamp, the bar offset as an arrow to the hand point.
- Seat: seat tube axis, post setback, saddle rails with the usable clamp window shaded, BRP, sit bones.
- **Targets**: the fit hand point with its tolerance drawn to scale, the target BRP, and the line and length between them. This is what would have exposed C1.
- **Reachable region**: the outline of every clamp position the stem catalog and spacer range can produce, drawn as a polygon. It replaces "N in oval" with something a person can read.
- Dimension lines with values for whatever the table shows.

**Tweaking, without JavaScript**

- One `GET` form: frame, size, fork, sag; stem length and angle; spacers; bar, roll, cut width; saddle, rail offset, post setback; saddle height.
- Beside each numeric field, plain links that re-request the page with that field nudged (`−10 −5 +5 +10`). Each click is a full page whose URL is the complete setup, so every state is bookmarkable and reproducible, consistent with docs/WEB.md.
- **Ghost overlay**: a second parameter set (`b.stem=…`) drawn in a muted stroke on the same axes, with a small table of deltas at the contact points ("hands +12 mm forward, −4 mm lower"). Swapping a bar, a stem or a saddle becomes "see where the point moved", which replaces the handlebar-compare and saddle-swap pages.
- Every cell in the comparison matrix links to the side view of that setup.

**Show the working**

Under the drawing, one table: quantity, formula in plain text, the substituted numbers, the result. For example:

```
Front axle Y   Stack − (HTL + cup + A2C)·sin(HTA) + rake·cos(HTA)
               642 − (140 + 12 + 506)·sin(68.25°) + 44·cos(68.25°)  =  47.1    (rear axle is at 72.0 → nose up, rotate ≈ 1.2°)
```

(Illustrative.) The validation page can use the same drawing with the residuals marked: where the chart says the front axle is, and where the declared fork puts it.

**Testing**

Assert point coordinates and attribute values (`circle[data-point="GR"]`), not pixels, with the existing `goquery` setup. A golden file of named points for three or four setups is enough.

**Size**: I estimate about 250–350 lines of Go and template for the SVG, given the `Layout`.

---

## 5. Feature reduction

Line counts are non-test Go lines from `wc -l`; "after" figures are estimates.

| Area | Today | Proposal | Why |
| --- | --- | --- | --- |
| Composite score: `scoring` 682, `ideal` 1,149, `curve` 164, `compare/prune` 183, breakdown and explanation code in `compare/document` | ≈ 2,400 | Fit solve + range flags, ≈ 200 | Section 4.2. The score currently rates the custom-fit frame 23/100 (C1); a solve states the same facts in millimetres |
| Posture and weight distribution (`geometry.go` 795–991) | ≈ 200 | One labelled ratio, ≈ 10 | C5: it is a constant |
| Saddle swap (`geometry/saddle_swap` 340, `web/saddle_swap` 447), ideal and "universal compromise" STA (`geometry/saddle` 306), saddles page 362, 22 helper methods on `SaddleSpec` | ≈ 1,900 | Saddle selector plus ghost overlay; `SaddleSpec` keeps two vectors | The analysis is one vector subtraction. Ideal-STA tables are a custom-frame design aid, and that frame is now ordered |
| Handlebar pages (`web/handlebars` 338, compare/compensation helpers) | ≈ 450 | Bar selector plus ghost overlay | Same reason |
| `fit` solver and assignment parser (`fit` 1,193; `ApplyAssignment` alone is 330 lines of field aliases) | ≈ 1,200 | Per-size implied structural A2C and an explicit candidate sweep, ≈ 250 | Frames are Go literals; editing the struct and re-running `validate` is already the fast loop. The combinatorial search over travel × sag × A2C × rake × spacer hides which assumption did the work |
| Spec grammar (`compare/specs` 412, `compare/run` 283): positional modifiers, numeric guessing (≥ 60 is travel, < 60 is sag), substring and legacy-suffix frame matching, `+spec`, `−spec`, `−N`, `all:all@all`, top-per-frame defaults | ≈ 700 | Explicit keys, exact IDs, ≈ 150 | The web form already submits `frame`, `size`, `fork`, `sag` separately. Removing column 3 of a compiled default by index is state emulation in a stateless tool |
| `canonicalizer` (871): five-way fork token resolver, three copies of the catalog lookup, label formatters and parsers | 871 | `Fork` value + `Setup{Fork, Sag}`, ≈ 150 | C10 |
| `model/bike.go` (1,611): parallel method sets on `BikeGeometry` and `FrameModel`, trivial `Get*` getters, `ForkSpec`, `ForkPreset`, `CanonicalConfig`, `SavedComparisonState`, `BaselineMetrics` | 1,611 | Input types plus `Layout`, ≈ 400 | One representation per concept |
| `geometry/geometry.go` (1,305) | 1,305 | `Build`, bar offset, budget, ≈ 450 | Section 4.1 |
| Layout variants: transposed, flat, split fit/chassis tables, detail, footers-on-detail | — | One table shape | Each variant multiplies test surface |
| Web (2,817 for 10 routes) | 2,817 | Compare, side view, frames, validate, help, ≈ 1,000 | Catalog pages fold into selectors |

Rough total: about 16,000 non-data lines today to somewhere around 4,500–5,500, with frames (2,024) and catalogs untouched. The estimate is coarse; the direction is not.

Features I would keep as they are: frame data and ingestion rules, `validate` with rounding budgets and `GeoSlopMm` discipline, bent-seat-tube support (two frames use it), adjustable chainstays, the fork catalog, per-column `frame + size + fork + sag`, the URL-as-state web model, the host guard.

A question worth asking about the CLI: if its audience is agents and scripts, a JSON dump of the `Layout` and the solve (`bikegeo layout frame=… fork=…`) is easier to consume and to test than Markdown tables, and removes the need for `report.Document` to serve two renderers.

---

## 6. Implementation quality

Principles, each tied to something observed:

1. **Errors, not defaults.** No geometry function substitutes a number for a missing input (C7). Constructors validate; everything downstream may assume valid data.
2. **One home for each fact.** Rider fit in one value (C6). Fork travel in the catalog (C10). Effective STA in one function (C11). Sizes as one ordered slice of rows rather than a `Sizes` slice plus a `Geometries` map that must agree.
3. **Types over strings.** Enum for metric IDs, checked by the `exhaustive` linter already in `make check`. `WheelSize` as a type (the model has a TODO for it). `Sag` as a field, never recovered from a label.
4. **Declare, do not infer.** Chart precision and provenance (published or derived) belong in the frame file (C8).
5. **State every assumption on the page.** Seatpost setback, headset top stack, wheel radius, grip inset and reference saddle should be inputs shown in the header, not constants inside functions. The current header does this for the seatpost; extend it.
6. **Invariant tests before example tests.** The suite has 9,193 lines, many asserting formatted strings. Four registry-wide invariants (section 4.1) would have caught C3 and C4 and survive any refactor of presentation.
7. **Deterministic output.** Sort map keys before producing text (C11); it also resolves the "tied candidates in varying order" note in AGENTS.md.
8. **Names that say what is measured.** "HB" currently means the hand point; "Seatpost Ext" is not an extension; "Oval Fits" shows a count next to a score threshold. With named points the labels can be literal: "BRP → grip centre", "BB → saddle top minus seat tube".
9. **Delete unused code** and run `deadcode` in `make check`.

---

## 7. Suggested order of work

Each step is small, independently verifiable, and leaves the suite green.

1. **Resolve C1** against the idmatch report: which point is 700 mm measured to, and with which bar. No code needed to decide; it changes what "correct" means for everything after.
2. **Add the invariant tests** (axles level after every fork swap; CAD fork reproduces the chart; frame distances preserved). Expect the first to fail (C3). This is test code only and pins down the target.
3. **Introduce `Layout` and `Build` alongside the existing code**, and assert in tests that it reproduces today's reach, stack, HTA and BB drop where rake is unchanged. Then fix C3 and decide C4 inside `Build` only.
4. **Ship the side view** on top of `Layout`, with targets drawn. From here the remaining decisions can be made by looking.
5. **Add the fit solve** and show it beside the existing score for a while. If the solve answers the question, remove the score, the oval, and the selection code.
6. **Move the remaining metrics onto `Layout`**, then delete `BikeFitAnalysis`, `BaselineMetrics`, `CanonicalComparisonRow` and the posture model.
7. **Collapse the inputs**: `Fit` value, `Fork`/`Setup` types, explicit-key spec grammar, declared precision. Turn the fallbacks in C7 into errors.
8. **Fold the catalog and analysis pages** into selectors and the ghost overlay; shrink `fit`.

Steps 1–4 deliver the transparency and the correctness fixes without removing any feature. Steps 5–8 are the reduction, and each can be judged with the side view in hand.

---

## 8. Reproduction and limits

Commands used (binary built to a scratch directory):

```bash
bikegeo compare fenrir:L@rigid fenrir:L@120mm@20% seven-custom --score-detail
```

```bash
bikegeo compare
```

```bash
bikegeo compare all
```

```bash
bikegeo validate seven-custom
```

```bash
bikegeo fit seven-custom
```

The independent checks (C1–C5 and the stem-penalty slope) were two small Go programs that import `bikegeo/pkg/...` through a `replace` directive from outside the repository. They rebuild the bike from chart points (BB, rear axle, head-tube top, front axle), swap the fork, re-level about the rear axle, and compare with `geometry.AnalyzeBikeFit`. They are not part of this change.

Not verified:

- Which handlebar point idmatch reports (C1), and whether the Fenrir and Fargo charts really are drawn to a bare head tube (C4). Both depend on documents outside the repository.
- Catalog values (fork A2C figures, saddle and bar dimensions) against manufacturer data.
- The web pages in a browser; I rendered routes with `serve --render` and read the handlers.
- The dead-code list is from `grep`, not from a call-graph tool.
- Line-count targets in section 5 are estimates.
