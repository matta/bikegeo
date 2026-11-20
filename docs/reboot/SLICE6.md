# M2 Slice 6, seatpost choice and setback notes

**Status: Planned 2026-11-19 from Matt's rulings Q26–Q29 ([DESIGN.md](DESIGN.md), "Seatposts, bar sweep, and fork choice"). Not started.** Read [AGENTS.md](../../AGENTS.md) and [DESIGN.md](DESIGN.md) first. They override anything here that disagrees. This plan builds on Slice 5 as committed ([SLICE5.md](SLICE5.md)). Slices 4 and 5 are awaiting Matt's acceptance.

This slice has three commits and touches 6 code, test, and template files, so per DESIGN.md "Slice planning" it is written so a less capable model (for example Sonnet) can implement it without making design choices. **Delegate it.** If something here seems wrong or missing, stop and ask Matt rather than choosing.

## 1. Goal

Today `/bike` has a free numeric `post_setback` field, and saddle fitting never says what setback a BRP target would need. On the default Fenrir build with a 0 mm setback post, fitting to the fit report BRP (−219, 713.1) stops at the +15 mm rail limit and still misses by 39.6 mm. After this slice:

- `/bike` gets a **Seatpost** select, `post`, offering four fixture posts (Q28). It replaces `post_setback` (Q27). The selected post sets the setback, the extension limits, and the clamp length, and the page shows the setback read-only.
- `assembly.FitPostSetback` fits the post setback and extension to a BRP target with the rails held at a given offset. Forward assembly checks the result.
- With a BRP target, the saddle fitting block always shows the **setback for centered rails**. When saddle fitting stops at a limit, it also shows the **setback to reach the target** with the rails held at the fitted offset (Q29). These are notes only, never applied or linked.

Rulings:

- **Q27:** the post is fixed and user-selected. Fitting and stock fitting never change it. `post` replaces `post_setback`.
- **Q28:** the posts are generic 0, 15, and 25 mm setback posts (illustrative), and the Redshift ShockStop PRO at 12 mm setback when sagged (owner-reported). There is no Drop Best entry.
- **Q29:** the centered-rails note always appears. The reach note appears only when saddle fitting is limited. Both keep the current post's extension limits.
- **Q6/Q12:** a direct solve reads each fitted input's direction from a probe placement's poses, and PlaceBike checks every fitted result.

## 2. Terminology

- **Seatpost** (**post** in short labels and URL keys). The URL key is `post`; the form control is `#ctrl-post`, labelled "Seatpost".
- **Setback** is `SeatpostSetup.PostSetbackMm`: the perpendicular offset from the seat-tube axis (seat-axis local +Y) to `SeatpostClamp`.
- **Fitting** / **fit** (Q8). Never write "search".
- Don't write the term AGENTS.md bans for the frameset (`make boundary` checks it).

## 3. Commit 1: assembly (`internal/reboot/assembly`)

Files: `fitting.go`, `fitting_test.go`. Nothing else changes in `assembly` or `pose`.

Commit subject: `Explain the post setback a reboot BRP target needs`.

### A1. `FitPostSetback` (`fitting.go`, after `ApplyFitting`)

```go
// FitPostSetback fits Seatpost.PostSetbackMm and Seatpost.ExtensionMm so the
// saddle BRP lands on brp, with Saddle.RailOffsetMm held at railOffsetMm. It
// explains the post setback a BRP target needs (Q29), and callers never
// install its build. Extension keeps the setup's limits. Setback is at least 0
// and has no upper bound. LimitedBy lists InputSeatpostExtension, then
// InputSeatpostSetback, for the inputs that sit on a bound. The submitted
// extension, setback, and rail offset are ignored; every other setup field is kept.
func FitPostSetback(setup BikeSetup, brp pose.Point, railOffsetMm float64) (Fitting, error)
```

Body, in this order:

1. If `!isFinitePoint(brp)`, return `invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", brp.X, brp.Y)`.
2. Build `probeSetup := setup` with `Seatpost.ExtensionMm = setup.Seatpost.MinExtensionMm`, `Seatpost.PostSetbackMm = 0`, and `Saddle.RailOffsetMm = railOffsetMm`. `probe, err := PlaceBike(probeSetup, nil)`. Return `err` unchanged. A rail offset beyond the clamp range fails here with `PlaceSaddle`'s `InputSaddleRailOffset` error (Q9). Don't add a second check.
3. Compute `eMin`, `eMax` exactly as `fitSaddle` does (`eMax = +Inf` when `MaxExtensionMm == 0`).
4. Solve:

   ```go
   delta := brp.Sub(probe.Saddle.BRP.Origin())
   shaft := probe.Seatpost.ShaftTop.HeadingDeg()
   a := fitAxis{shaft, 0, eMax - eMin, InputSeatpostExtension}
   b := fitAxis{shaft + 90, 0, math.Inf(1), InputSeatpostSetback} // PlaceSeatpost translates setback along local +Y
   de, ds, limited, ok := fitBox(delta, a, b)
   if !ok {
       return Fitting{}, fmt.Errorf("seatpost setback is parallel to the seat axis; cannot fit the setback")
   }
   ```

   `fitBox` already handles an infinite upper bound on either axis. Don't change `fitBox`.
5. `fitted := probeSetup` with `Seatpost.ExtensionMm = clampFit(eMin+de, eMin, eMax)` and `Seatpost.PostSetbackMm = clampFit(ds, 0, math.Inf(1))`. `placement, err := PlaceBike(fitted, nil)`.
6. Return `Fitting{Target: FitTarget{BRP: &p}, Placement: placement, Outcome: FitReached}` with `p := brp`. When `limited` is non-empty, set `Outcome = FitLimited` and `LimitedBy = limited`. When it is empty and `BrpMiss()` is more than 1e-6 mm, return `fmt.Errorf("post setback fitting forward check missed target by %v mm", miss.Norm())`. This is the same forward check `ApplyFitting` does.

`FitPostSetback` takes no trace. It has no caller that renders its construction.

### A2. Tests (`fitting_test.go`, package `assembly_test`)

Use `fenrirFittingSetup()` and `fitReportBRP`. "Lands" means `BrpMiss()` within 1e-9 mm. The reference values were measured by an independent probe (§6). Tolerances: 1e-6 for Newton-probed values, 1e-5 for golden-section-probed values.

1. **`TestFitPostSetbackCentersRails`.** Rail 0: setback 54.082742 and extension 264.047523 (1e-6). Outcome `FitReached`, `LimitedBy` nil, and BRP lands. `Placement.Setup.Saddle.RailOffsetMm == 0`.
2. **`TestFitPostSetbackAtRailLimit`.** Rail 15: setback 39.613611 and extension 260.092242 (1e-6), reached. Cross-check against `ApplyFitting(fenrirFittingSetup(), FitTarget{BRP: &fitReportBRP}, nil)`, which is limited at rail +15. Its `BrpMiss().Norm()` equals this setback, and its fitted extension equals this extension, both to 1e-9. When the rails are pinned, the remaining miss lies along the setback direction.
3. **`TestFitPostSetbackIgnoresSubmittedSeatInputs`.** Set `PostSetbackMm = 25`, `ExtensionMm = 200`, and `RailOffsetMm = -10`. Rail 0 gives test 1's setback and extension to 1e-9.
4. **`TestFitPostSetbackStopsAtZeroSetback`.** Target (−150, 713.1), rail 0: setback exactly 0, extension 245.853232 (1e-5), `LimitedBy == [InputSeatpostSetback]`, `Outcome == FitLimited`, and BRP miss 12.475261 mm (1e-5).
5. **`TestFitPostSetbackForwardTarget`.** Target (−150, 713.1), rail −15: setback 1.993870 and extension 249.808512 (1e-6), reached. Cross-check: with `PostSetbackMm = 12`, `ApplyFitting` is limited at rail −15 with miss *m*, and 12 − *m* equals this setback to 1e-9.
6. **`TestFitPostSetbackStopsAtExtension`.** Target (−219, 900). Rail 0: extension exactly 300, setback 4.799945 (1e-5), `LimitedBy == [InputSeatpostExtension]`, miss 144.332896 (1e-5). Rail 15: extension 300, setback 0, `LimitedBy == [InputSeatpostExtension, InputSeatpostSetback]`, and miss 140.710227 (1e-6), which equals `ApplyFitting`'s miss for that target on the 0 mm post (1e-9).
7. **`TestFitPostSetbackRejectsRailBeyondClamp`.** Rail 15.5: the error is an `*assembly.InputError` with `Field == InputSaddleRailOffset`. A target with NaN X is rejected with `Field == InputFitTargetBRP`.

Every existing test must pass unchanged.

## 4. Commit 2: the seatpost select (`internal/reboot/web`)

Files: `fixture.go`, `web.go`, `templates/bike.html`, `web_test.go`. No CSS or JS change: `workbench.js` submits the whole form on `change`.

Commit subject: `Choose the seatpost from a fixture list in the reboot workbench`.

### W1. Fixture (`fixture.go`)

Add beside `handlebarOption`:

```go
// seatpostOption is one seatpost the workbench can install. Setup.ExtensionMm
// is always 0: the extension comes from the workbench state (Q27).
type seatpostOption struct {
	ID    string
	Name  string
	Setup assembly.SeatpostSetup
}
```

In `bikeFixture`, replace `DefaultSeatpost assembly.SeatpostSetup` with:

```go
DefaultSeatpostID string
DefaultPostExtMm  float64
Seatposts         []seatpostOption
```

Add `resolveSeatpost(requestedID string) (seatpostOption, bool)`. It mirrors `resolveHandlebar`: empty means the default.

In `validate()`, after the handlebar checks, add the following, with messages in the same style as the bar checks:

- `DefaultSeatpostID` is non-empty and resolves.
- IDs are non-empty and unique.
- Each `Setup.ExtensionMm` is 0.
- Each post places: `assembly.PlaceSeatpost(pose.NewPose(pose.Point{}, 0, nil), setup)` with `setup.ExtensionMm = setup.MinExtensionMm` returns no error.
- The default post with `ExtensionMm = DefaultPostExtMm` places.

In `otsoFenrirFixture()`, replace the `DefaultSeatpost` literal. Keep the `const` block and its comments:

```go
DefaultSeatpostID: "setback_0",
DefaultPostExtMm:  263.5,
Seatposts: []seatpostOption{
	genericSeatpost("setback_0", 0),
	genericSeatpost("setback_15", 15),
	genericSeatpost("setback_25", 25),
	// owner-reported 2026-11-19: 12 mm setback when sagged (Q28). The fit
	// report was transferred with this post unsagged (docs/RIDER_FIT.md).
	// illustrative: extension limits and clamp length not yet measured; generic values.
	{ID: "redshift_shockstop_pro", Name: "Redshift ShockStop PRO (12 mm setback, sagged)", Setup: assembly.SeatpostSetup{
		MinExtensionMm: postLengthMm - frameInsertionDepthMm,
		MaxExtensionMm: postLengthMm - postMinInsertionMm,
		PostSetbackMm:  12.0,
		ClampLengthMm:  30.0,
	}},
},
```

`genericSeatpost` is a closure inside `otsoFenrirFixture()`, defined just after the `const` block:

```go
// illustrative: generic posts with the Ritchey-based extension limits above
genericSeatpost := func(id string, setbackMm float64) seatpostOption {
	return seatpostOption{
		ID:   id,
		Name: fmt.Sprintf("Generic %.0f mm setback post", setbackMm),
		Setup: assembly.SeatpostSetup{
			MinExtensionMm: postLengthMm - frameInsertionDepthMm,
			MaxExtensionMm: postLengthMm - postMinInsertionMm,
			PostSetbackMm:  setbackMm,
			ClampLengthMm:  30.0, // measured
		},
	}
}
```

The default post's values equal today's `DefaultSeatpost`, so every existing default number holds.

### W2. State, URL, form (`web.go`)

- `workbenchState`: replace `PostSetbackMm float64` with `PostID string`, placed after `BarID`.
- `defaultWorkbenchState`: `PostID: fix.DefaultSeatpostID` and `PostExtMm: fix.DefaultPostExtMm`. Delete the `PostSetbackMm` line.
- `workbenchStateFromSetup(view diagramView, forkID, barID, postID string, setup assembly.BikeSetup)`: add the `postID` parameter, set `PostID: postID`, and delete the `PostSetbackMm` line. Its only caller is `withSetup`, which passes `s.PostID`. Update `withSetup`'s comment to "keeping the submitted view, fork, bar, post, fit targets, and stock bar option".
- `URL()`: replace `q.Set("post_setback", ...)` with `q.Set("post", s.PostID)`. It is always set, like `bar`.
- `bikeFormValues`: replace `PostSetback string` with `Post string`, placed after `Bar`. `formValues()` sets `Post: s.PostID`.
- `renderBikeError`: after the `cadBase.BarID` lines, add `if resolved.State.PostID != "" { cadBase.PostID = resolved.State.PostID }`.

### W3. Decoding (`decodeWorkbenchState`)

Right after the handlebar block:

```go
selectedPost, postOK := fix.resolveSeatpost(q.Get("post"))
if postOK {
	state.PostID = selectedPost.ID
	form.Post = selectedPost.ID
} else {
	state.PostID = ""
	p.fail("post", fmt.Errorf("unknown seatpost %q", q.Get("post")))
	// Build with the default post so the error page can echo the other controls.
	selectedPost, _ = fix.resolveSeatpost("")
}
```

Delete the `post_setback` parse line. A submitted `post_setback` is now ignored like any unknown key. Replace `post := fix.DefaultSeatpost` with `post := selectedPost.Setup`. Keep `post.ExtensionMm = state.PostExtMm` and delete `post.PostSetbackMm = ...`.

`bikeInputField`: `assembly.InputSeatpostSetback` now returns `"post"`.

### W4. View (`web.go`)

- `bikeView` gains `Seatposts []seatpostOption`, `SelectedPostID string`, `PostSetbackMm float64`, and `PostClampLengthMm float64`.
- Change `railOffsetRange` to a function of the placed setup: `func railOffsetRange(saddle assembly.SaddleSetup, post assembly.SeatpostSetup) (minMm, maxMm float64)`. The body is the same, built from its arguments.
- `buildBikeView`: set `Seatposts: fix.Seatposts`, `SelectedPostID: submitted.PostID`, `PostSetbackMm: bike.Setup.Seatpost.PostSetbackMm`, and `PostClampLengthMm: bike.Setup.Seatpost.ClampLengthMm`. Call `railOffsetRange(bike.Setup.Saddle, bike.Setup.Seatpost)`.
- `renderBikeError`: read the post from `resolved.Setup.Seatpost`. `decodeWorkbenchState` always builds it, with the default post on a bad `post`. Set `MinPostExtensionMm`, `MaxPostExtensionMm`, `PostSetbackMm`, and `PostClampLengthMm` from it, along with `Seatposts`, `SelectedPostID: resolved.State.PostID`, and `railOffsetRange(resolved.Setup.Saddle, resolved.Setup.Seatpost)`.

### W5. Template (`bike.html`, Rear Cockpit fieldset)

Replace the Seatpost Setback field with a full-width select, placed **first** in the fieldset's `group-fields`:

```html
<div class="field field-full">
  <label for="ctrl-post">Seatpost</label>
  <select id="ctrl-post" name="post"{{if and $.ErrorMessage (eq $.InvalidField "post")}} aria-invalid="true" aria-describedby="form-error"{{end}}>
    {{range .Seatposts}}
    <option value="{{.ID}}"{{if eq .ID $.SelectedPostID}} selected{{end}}>{{.Name}}</option>
    {{end}}
  </select>
  <p class="field-note" data-note="post-setback">Setback <span class="mono" data-metric="post-setback">{{printf "%.1f mm" .PostSetbackMm}}</span> · extension {{printf "%.0f" .MinPostExtensionMm}}–{{printf "%.0f" .MaxPostExtensionMm}} mm · clamp {{printf "%.0f" .PostClampLengthMm}} mm. The post sets its setback.</p>
</div>
```

The label count stays 18: one label is removed and one added.

### W6. Tests (`web_test.go`)

Add a helper beside `defaultHandlebarSetup`:

```go
// fixtureSeatpost returns the fixture post with id, at extension extMm.
func fixtureSeatpost(t *testing.T, fix bikeFixture, id string, extMm float64) assembly.SeatpostSetup
```

It calls `t.Fatalf` when the ID is unknown.

**Update existing tests** (find each one with `grep -n 'post_setback\|PostSetback\|DefaultSeatpost' internal/reboot/web/web_test.go`):

- Lines that build `fix.DefaultSeatpost` (about 4 sites) use `fixtureSeatpost(t, fix, fix.DefaultSeatpostID, fix.DefaultPostExtMm)`.
- `TestBikeWorkbenchCockpitControlsUpdateBuildAndTrace`: change the query's `post_setback=20` to `post=setback_25`. `postSetup` is `fixtureSeatpost(t, fix, "setback_25", 250)`. Delete `"post_setback"` from the input-value map. Add a check that `select[name="post"] option[selected]` has value `setback_25` and that `[data-metric="post-setback"]` reads `25.0 mm`.
- `TestBikeWorkbenchRejectsInvalidQuery`: replace `{"/bike?post_setback=-2", "post_setback", "-2"}` with `{"/bike?post=unknown", "post", ""}`.
- `TestBuildBikeViewUsesPlacedDimensions`: `customSeatpost := fixtureSeatpost(t, fix, "setback_15", 240)`. Pass `PostID: "setback_15"` in the `buildBikeView` state. In `wantForm`, replace `PostSetback: "15.0"` with `Post: "setback_15"`. In `wantCadState`, replace `PostSetbackMm: 15.0` with `PostID: "setback_15"`.
- `TestWorkbenchStateCodecRoundTrip`: replace `PostSetbackMm: 20.0` with `PostID: "setback_25"` in `want` and in any other literal state in that test.
- `TestBikeWorkbenchFormRoundTripPreservesEveryBuild`: in `customURL`, replace `post_setback=20` with `post=setback_25`.
- `TestBikeWorkbenchEveryEnabledControlIsLive`: replace `{field: "post_setback", val: "20.0"}` with `{field: "post", val: "setback_25", isSelect: true}`.
- Any other `post_setback` in a URL or expected href: replace `post_setback=<v>` with the matching post ID (0 → `setback_0`, 15 → `setback_15`, 25 → `setback_25`). There is no 20 mm post. Where a test used 20, use `setback_25` and recompute any expected number from `PlaceBike` in the test, as that test already does. Don't hard-code a new number without an independent check.

**New tests:**

1. **`TestBikeWorkbenchSeatpostSelect`.** On `/bike`: `select[name="post"]` has 4 options in fixture order, `setback_0` is selected, `[data-metric="post-setback"]` reads `0.0 mm`, the note contains `extension 110–300 mm · clamp 30 mm`, and the canonical URL has `post=setback_0` and no `post_setback`. `/bike?post=redshift_shockstop_pro`: `12.0 mm`, and `[data-metric="brp-height"]` equals `PlaceBike` of the default setup with `fixtureSeatpost(t, fix, "redshift_shockstop_pro", 263.5)`, formatted `%.1f mm`.
2. **`TestBikeWorkbenchSeatpostSurvivesLinks`.** `/bike?post=setback_15&brp_x=-219&brp_y=713.1&grip_x=481&grip_y=726&view=seat`: the canonical URL, every `a[data-diagram-view]` href, the CAD Baseline Fork preset href, `#apply-fitting`'s href, and every `[data-stock-apply]` href have `post=setback_15`. The Reset Defaults href has `post=setback_0`.
3. **`TestBikeWorkbenchIgnoresRetiredSetbackKey`.** `/bike?post_setback=20` returns 200 with no `#form-error`. `[data-metric="post-setback"]` reads `0.0 mm`, and the canonical URL has no `post_setback`.
4. **`TestBikeWorkbenchUnknownSeatpost`.** `loadBikeDocStatus(t, "/bike?post=unknown", http.StatusBadRequest)`: `select[name="post"]` has `aria-invalid="true"`, and `#form-error` contains `unknown seatpost "unknown"`.

## 5. Commit 3: setback notes (`internal/reboot/web`)

Files: `web.go`, `templates/bike.html`, `web_test.go`.

Commit subject: `Show the post setback a BRP target needs in the reboot workbench`.

### N1. View types (`web.go`)

```go
// postSetbackView is one setback note (Q29): the setback and extension
// FitPostSetback found with the rails held at RailOffsetMm. It is never applied.
type postSetbackView struct {
	RailOffsetMm, SetbackMm, ExtensionMm, MissMm float64
	LimitedBy                                    []assembly.InputField
}

// OutcomeText describes the setback fitting outcome.
func (v postSetbackView) OutcomeText() string { return fitOutcomeText(v.LimitedBy) }

func makePostSetbackView(f assembly.Fitting) postSetbackView
```

`makePostSetbackView` reads every value from `f.Placement.Setup` (`Saddle.RailOffsetMm`, `Seatpost.PostSetbackMm`, `Seatpost.ExtensionMm`), `MissMm` from `f.BrpMiss()` (norm), and `LimitedBy` from `f.LimitedBy`.

`saddleFittingView` gains:

```go
Centered postSetbackView  // rails centered (Q29)
Reach    *postSetbackView // rails held at the fitted offset; nil unless saddle fitting is limited
```

### N2. Handler (`web.go`)

- Change `buildFittingView` to `buildFittingView(state workbenchState, bike assembly.BikePlacement, fitting assembly.Fitting, centered, reach *assembly.Fitting)`. When `fv.Saddle` is built, set `Centered: makePostSetbackView(*centered)`. When `reach != nil`, set `Reach` to `makePostSetbackView(*reach)`. `centered` is non-nil exactly when `fitting.Target.BRP` is set. If that invariant ever fails, it is a programming error, so dereference without a check.
- In `handleBikeWorkbench`, between `ApplyFitting` and `buildFittingView`:

  ```go
  var centered, reach *assembly.Fitting
  if brp := target.BRP; brp != nil {
      c, err := assembly.FitPostSetback(resolved.Setup, *brp, 0)
      if err != nil {
          renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
          return
      }
      centered = &c
      saddleLimited := slices.Contains(fitting.LimitedBy, assembly.InputSeatpostExtension) ||
          slices.Contains(fitting.LimitedBy, assembly.InputSaddleRailOffset)
      if saddleLimited {
          r, err := assembly.FitPostSetback(resolved.Setup, *brp, fitting.Placement.Setup.Saddle.RailOffsetMm)
          if err != nil {
              renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
              return
          }
          reach = &r
      }
  }
  view.Fitting = buildFittingView(resolved.State, bike, fitting, centered, reach)
  ```

  This replaces the existing `view.Fitting = buildFittingView(...)` line. Add `"slices"` to the imports if it isn't there.

### N3. Template (`bike.html`, inside the Saddle fitting summary, after the Outcome span)

```html
<span>Setback for centered rails: <strong class="mono" data-metric="fit-setback-centered">{{printf "%.1f mm" .Centered.SetbackMm}}</strong> at extension <span class="mono" data-metric="fit-setback-centered-ext">{{printf "%.1f mm" .Centered.ExtensionMm}}</span>; <span data-metric="fit-setback-centered-outcome">{{.Centered.OutcomeText}}</span>{{if .Centered.LimitedBy}}, {{printf "%.1f" .Centered.MissMm}} mm short{{end}}</span>
{{with .Reach}}
<span>Setback to reach the target with rails at <span class="mono" data-metric="fit-setback-reach-rail">{{printf "%+.1f mm" .RailOffsetMm}}</span>: <strong class="mono" data-metric="fit-setback-reach">{{printf "%.1f mm" .SetbackMm}}</strong> at extension <span class="mono" data-metric="fit-setback-reach-ext">{{printf "%.1f mm" .ExtensionMm}}</span>; <span data-metric="fit-setback-reach-outcome">{{.OutcomeText}}</span>{{if .LimitedBy}}, {{printf "%.1f" .MissMm}} mm short{{end}}</span>
{{end}}
<span class="fit-note">Setback notes keep this post's extension limits and are not applied.</span>
```

### N4. Tests (`web_test.go`)

Use `metricText` (or `collectMetrics`). The values come from §6, formatted as the template prints them.

1. **`TestBikeWorkbenchSetbackNotesFitReport`.** `/bike?brp_x=-219&brp_y=713.1`: saddle outcome `limited by saddle rail offset`. Centered `54.1 mm` at `264.0 mm`, `reached`. Reach rail `+15.0 mm`, setback `39.6 mm`, extension `260.1 mm`, `reached`. Then `post=setback_25` with the same target gives the same note values (the notes don't depend on the submitted post's setback), while `fit-brp-miss`, the submitted build's miss, changes.
2. **`TestBikeWorkbenchSetbackNotesReachedFit`.** `/bike?post=setback_25&brp_x=-189&brp_y=713.1`: saddle outcome `reached`, fitted rail offset `0.1 mm`, and there is no `[data-metric="fit-setback-reach"]`. Centered `25.1 mm` at `256.1 mm`, `reached`.
3. **`TestBikeWorkbenchSetbackNotesForwardTarget`.** `/bike?post=redshift_shockstop_pro&brp_x=-150&brp_y=713.1`: saddle outcome `limited by saddle rail offset` with fitted rail `-15.0 mm`. Reach rail `-15.0 mm`, setback `2.0 mm`, extension `249.8 mm`, `reached`. Centered `0.0 mm` at `245.9 mm`, `limited by seatpost setback`, and the span contains `12.5 mm short`.
4. **`TestBikeWorkbenchSetbackNotesExtensionLimit`.** `/bike?brp_x=-219&brp_y=900`: saddle outcome `limited by seatpost extension, saddle rail offset`. Centered `4.8 mm` at `300.0 mm`, `limited by seatpost extension`, `144.3 mm short`. Reach rail `+15.0 mm`, `0.0 mm` at `300.0 mm`, `limited by seatpost extension, seatpost setback`, `140.7 mm short`.
5. **No notes without a BRP target.** `/bike?grip_x=481&grip_y=726` has no `[data-metric^="fit-setback"]`.
6. **Notes are never applied.** In test 1's page, `#apply-fitting`'s href has `post=setback_0`, and following it shows `[data-metric="post-setback"]` `0.0 mm`.

## 6. Reference numbers (measured 2026-11-19)

The probe was independent of `fitBox`. For the reached cases, it ran Newton's method with finite differences on `PlaceBike` over (extension, setback) at a fixed rail offset. For the bound cases, it ran a golden-section search over the free input with the other pinned. The default Fenrir build is the Lithic fork, EC44 12 mm cup, post extension limits 110–300 mm, 30 mm clamp, and rail limit ±15 mm.

| Target | Rail | Setback | Extension | Outcome | Miss |
| --- | --- | --- | --- | --- | --- |
| (−219, 713.1) | 0 | 54.082742 | 264.047523 | reached | 0 |
| (−219, 713.1) | +15 | 39.613611 | 260.092242 | reached | 0 |
| (−189, 713.1) | 0 | 25.144479 | 256.136961 | reached | 0 |
| (−150, 713.1) | 0 | 0 | 245.853232 | setback | 12.475261 |
| (−150, 713.1) | −15 | 1.993870 | 249.808512 | reached | 0 |
| (−219, 900) | 0 | 4.799945 | 300 | extension | 144.332896 |
| (−219, 900) | +15 | 0 | 300 | extension, setback | 140.710227 |

Saddle fitting (`ApplyFitting`, BRP only) for the same targets by post:

| Target | 0 mm | 12 mm (Redshift) | 15 mm | 25 mm |
| --- | --- | --- | --- | --- |
| (−219, 713.1) | rail +15, miss 39.613611 | rail +15, miss 27.613611 | rail +15, miss 24.613611 | rail +15, miss 14.613611 |
| (−189, 713.1) | rail +15, miss 10.675348 | reached, rail 13.626747 | reached, rail 10.516678 | reached, rail 0.149780 |
| (−150, 713.1) | reached, rail −12.932976 | rail −15, miss 10.006130 | rail −15, miss 13.006130 | rail −15, miss 23.006130 |

Findings worth noting (not defects). On the Fenrir with this saddle, the fit report BRP needs 54.1 mm of setback with the rails centered, or 39.6 mm with the rails at their +15 mm limit. None of the four posts reaches it: the 25 mm post still misses by 14.6 mm, and the Redshift by 27.6 mm. If your numbers differ beyond the stated tolerances, find out why before changing a test.

## 7. Not in this slice

Fitting or stock fitting choosing the post (Q27); a Drop Best or any adapter (Q28); measured Redshift extension limits and clamp length (they come when Matt measures them); applying or linking a setback note; setback notes for posts with other extension limits; Redshift sag modeling; the `/` stem preview page; CLI/MCP access; other frames; any change to `pose`, or to `assembly` outside `fitting.go`.

## 8. Rules that bite here

- **Forward assembly is the evaluator (Q6, Q12).** `FitPostSetback` reads its directions from the probe's `ShaftTop` heading and checks the result with `PlaceBike`. Don't restate the seatpost formulas.
- **The notes are never applied (Q29).** No link, preset, or form value takes a setback from `FitPostSetback`. `withSetup` doesn't read `PostSetbackMm`.
- **The URL is the only state.** `post` must round-trip through the form, the canonical URL, and every link. `post_setback` is gone from URLs and is ignored on input.
- **The web never recomputes geometry.** Note values come from the returned `Fitting`'s placement and setup.
- `deadcode`: `resolveSeatpost`, `fixtureSeatpost` (test only), `makePostSetbackView`, and `FitPostSetback` must be reached. `exhaustive`: `buildFittingView`'s existing switch on `InputField` stays complete. No new `InputField` is added.
- `boundary`: no legacy imports.
- Commit messages: plain subject, no Conventional Commits prefix; end with the attribution line from the session reminder.
- Don't edit DESIGN.md beyond marking Slice 6 done at the end, and only after Matt accepts it.

## 9. Verification

After the code compiles, run `go vet ./... && go test ./...` after every change. Before each commit, run `make pre-commit` and re-run it until `go fix` is clean. Each commit must pass on its own.

Then check it live: `make serve-reboot`, then open `http://127.0.0.1:8081/bike?brp_x=-219&brp_y=713.1&view=seat`. Confirm that:

- The Seatpost select shows four posts with "Generic 0 mm setback post" selected, and the setback note reads 0.0 mm.
- The saddle fitting block shows "Setback for centered rails: 54.1 mm" and "Setback to reach the target with rails at +15.0 mm: 39.6 mm".
- Choosing "Generic 25 mm setback post" moves the saddle back in the seat view and keeps both notes the same. Following "Apply fitted build" then shows a current BRP miss of 14.6 mm.

Take a screenshot for Matt.
