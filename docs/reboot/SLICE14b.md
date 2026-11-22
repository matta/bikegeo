# Slice 14b: Web `/` compare page and `/` → `/bike` verification

Status: Implemented 2026-11-21; awaiting Matt's acceptance

## Implementation handoff

- **Commits:**
  1. `8a2e017` — `Add compare URL and row bike-link helpers in query` (`internal/reboot/query/bike.go`, `internal/reboot/query/compare.go`, `internal/reboot/query/compare_test.go`)
  2. `dbcced0` — `Add web comparison page and root navigation` (`internal/reboot/web/web.go`, `internal/reboot/web/bike_page.go`, `internal/reboot/web/compare_page.go`, `internal/reboot/web/templates/compare.html`, `internal/reboot/web/templates/bike.html`, `internal/reboot/web/web_test.go`)
  3. `a66b379` — `Add compare page tests and update agent docs` (`internal/reboot/web/compare_test.go`, `AGENTS.md`)
- **Verification:**
  - `make pre-commit` passed on each of the 3 commits individually (`go fix`, `go fmt`, `go vet`, `go tool exhaustive`, `go test -v ./...`, `make boundary`).
  - `go list -deps ./internal/reboot/query | grep -E '(^net/http$|^html/template$|reboot/web)'` printed nothing.
  - `go test ./internal/reboot/query ./internal/reboot/web -count=1` passed.
- **Live web check (`./bin/bikegeo-reboot serve --port 18081`, Chrome):**
  - Desktop (`1440x900`), tablet (`1024x900`), and mobile (`390x844`): settings grid transitioned 3-col → 2-col (Ranking Limits spanning full width, zero orphan) → 1-col; all 3 tables rendered with right-aligned tabular numbers and scroll shadows; `<details>` 12-attribute score breakdown opened/closed cleanly; sliding-dropout `*` markers and footnote present; zero JS/runtime errors (only browser-initiated `/favicon.ico` 404).
  - Form flows: added `Surly Krampus` via `Add a frame…`, narrowed to `XL` + `surly_krampus_483` + `20%` sag → HTTP `400` with `#form-error` and `aria-invalid="true"` on `Sag`; recovered via `Any sag` → `200`; verified `Remove` and `All 19 Frames`.
  - Lossless `/` → `/bike` inspect & nav: Row #1 (`Surly Krampus XL`) reproduced `Submitted build score` (`Fits all`, `84.6`, Oval R `0.76`, BRP miss `(-4.1, +0.0) mm`) and `Ranked builds #1` (`84.6`, `100 mm, -10°`, `0 mm` spacers); Row #5 (`Salsa Fargo XL`) reproduced `Submitted build score` (`Fits BRP`, `54.5`, Oval R `4.33`, BRP miss `(-10.0, -0.9) mm`) and `Ranked builds #1` (`54.5`, `120 mm, -30°`, `0 mm` spacers); `Compare` nav link returned to `/`.
- **Minor plan note:** in `TestCompareCustomSelectionsAndAutoTopPerFrame`, `sid_100`'s catalog name is `"RockShox SID 100mm"` (so `"RockShox SID 100mm, 25% sag"`) and `FramesetPlacement.EttMm()` for `otso-fenrir-stainless` L + `sid_100` @ `25%` formats to `"597.2"` (not `"597.3"`).

**Rulings:** Q52, Q53, Q54, Q56, Q61, Q62, Q67, Q70, Q76, Q77, Q78, Q84, Q85, Q86, Q87, Q88, Q90, Q101, Q104, Q105, Q107, Q108, Q110, Q112, Q113; `AGENTS.md` rules.

**Builds on:** Slice 13e (commits `f156358`, `068a628`, `ec2827f`) for `LeveledReachMm`/`LeveledStackMm`/`EttMm` and `internal/reboot/compare`, Slice 13f (commits `c30af3b`, `cae0a01`, `801781b`) for `internal/reboot/query/compare.go` and `bikegeo-reboot compare`, and Slice 14a (commits `5af1fe2`, `a6a2c0b`, `1aac48d`, `b52224c`, `0def441`) for `internal/reboot/query/bike.go`, multi-frame `/bike`, and submitted-build scoring (`Q110`).

**Size:** larger (8 files across `query`, `web`, and `AGENTS.md`, 3 commits). Written for a lower-tier implementer. **Do not make design or UX decisions.** Every layout rule, color token, font rule, breakpoint, HTML landmark/attribute, `data-*` hook, Go signature, and probed test expectation is specified below. If anything below doesn't match committed code, stop and record `Blocked <date> on <question>` (see "Rules that bite").

## Goal

1. Add lossless row-to-`/bike` URL and canonical `/` compare URL helpers on `query.Compare` (`TopPerFrameExplicit`, `TopExplicit`, `BikeForRow`, `RowBikeURL`, `URL`, `WithoutSelection`, `AllFrames`) with unit tests covering positive and negative cases.
2. Replace the `/` → `/bike` redirect in `internal/reboot/web` with the pure Web 1.0 multi-frame comparison page (`compare_page.go`, `templates/compare.html`) and add shared `/` ↔ `/bike` primary navigation (`nav.site-nav`) to `templates/bike.html` and `templates/compare.html`, applying every layout, accessibility, and error-recovery lesson from `DESIGN.md` Deferred follow-ups (lines 412–421) from day one.
3. Add comprehensive browserless HTTP tests (`internal/reboot/web/compare_test.go`, updating `web_test.go`'s root test) including end-to-end HTTP verification of Q67/Q77/Q110 lossless `/` → `/bike` links against both `/bike`'s Submitted build score readout and Ranked builds row `#1`, and update stale reboot status notes in `AGENTS.md`.
4. Zero changes to `internal/reboot/pose`, `internal/reboot/assembly`, `internal/reboot/catalog`, `internal/reboot/compare`, or `cmd/bikegeo-reboot`.

---

## Commit 1: `Add compare URL and row bike-link helpers in query`

### Q1. `internal/reboot/query/bike.go`

Extract an unexported `queryValues` method from `Bike.URL()` (lines 497–535) so `Compare.RowBikeURL` can reuse the exact encoding logic without duplicating field lists:

```go
// queryValues encodes b into /bike query values (Q70, Q77), omitting prev_* keys.
func (b Bike) queryValues() url.Values {
	// exact body currently in Bike.URL(), returning v instead of "/bike?" + v.Encode()
}

// URL encodes the bike as a lossless /bike URL (Q70, Q77). It never emits
// prev_* keys.
func (b Bike) URL() string {
	return "/bike?" + b.queryValues().Encode()
}
```

Zero behavior change to `Bike.URL()`.

### Q2. `internal/reboot/query/compare.go`

1. Add `"strconv"` to imports if not already present (note: `"strconv"` is already imported).
2. Add two exported fields to `type Compare struct` right after `Top int`:

```go
	TopPerFrameExplicit bool
	TopExplicit         bool
```

3. In `ParseCompare(q url.Values)`:
   - Record:
     ```go
     topPerFrameExplicit := strings.TrimSpace(q.Get("top_per_frame")) != ""
     topExplicit := strings.TrimSpace(q.Get("top")) != ""
     ```
   - Populate `TopPerFrameExplicit: topPerFrameExplicit` and `TopExplicit: topExplicit` in the returned `Compare` struct.
4. Add five exported methods on `Compare`:

```go
// BikeForRow constructs the exact single-bike workbench state represented by a
// ranked compare row (Q67, Q77).
func (c Compare) BikeForRow(row compare.Row) (Bike, error) {
	frame, ok := catalog.FrameByID(row.FrameID)
	if !ok {
		return Bike{}, fmt.Errorf("unknown frame %q", row.FrameID)
	}
	size, ok := frame.Size(row.Size)
	if !ok {
		return Bike{}, fmt.Errorf("frame %q has no size %q", row.FrameID, row.Size)
	}
	ff, ok := findFrameFork(frame, row.ForkID)
	if !ok {
		return Bike{}, fmt.Errorf("frame %q has no fork %q", row.FrameID, row.ForkID)
	}
	bar, err := c.WinningHandlebar(row)
	if err != nil {
		return Bike{}, err
	}
	brp := c.BrpTarget
	grip := c.GripTarget
	return Bike{
		Frame:          frame,
		Size:           size,
		ForkID:         row.ForkID,
		FrameFork:      ff,
		Bar:            bar,
		Post:           c.Post,
		View:           ViewBike,
		BrpTarget:      &brp,
		GripTarget:     &grip,
		AllBarOptions:  c.AllBarOptions,
		AllForkOptions: false,
		Setup:          row.Build.Placement.Setup,
	}, nil
}

// RowBikeURL returns the lossless /bike inspect URL for a ranked compare row
// (Q67, Q77), carrying explicit bar_options and fork_options=selected.
func (c Compare) RowBikeURL(row compare.Row) (string, error) {
	b, err := c.BikeForRow(row)
	if err != nil {
		return "", err
	}
	v := b.queryValues()
	if b.AllBarOptions {
		v.Set("bar_options", BarOptionsAll)
	} else {
		v.Set("bar_options", BarOptionsSelected)
	}
	v.Set("fork_options", ForkOptionsSelected)
	return "/bike?" + v.Encode(), nil
}

// URL encodes c as a canonical / compare URL, omitting default global values.
func (c Compare) URL() string {
	v := url.Values{}
	for _, sel := range c.Selections {
		v.Add("frame", sel.FrameID)
		v.Add("size", sel.Size)
		v.Add("fork", sel.ForkID)
		if sel.SagPct != nil {
			v.Add("sag", FormatFloat(*sel.SagPct))
		} else {
			v.Add("sag", "")
		}
	}
	if c.TopPerFrameExplicit {
		v.Set("top_per_frame", strconv.Itoa(c.TopPerFrame))
	}
	if c.TopExplicit {
		v.Set("top", strconv.Itoa(c.Top))
	}
	if c.Post.ID != catalog.DefaultSeatpostID {
		v.Set("post", c.Post.ID)
	}
	if c.Bar.ID != catalog.DefaultHandlebarID {
		v.Set("bar", c.Bar.ID)
	}
	if c.AllBarOptions {
		v.Set("bar_options", BarOptionsAll)
	}
	if c.BarRollDeg != 0 {
		v.Set("bar_roll", FormatFloat(float64(c.BarRollDeg)))
	}
	if c.RailPitchDeg != 0 {
		v.Set("rail_pitch", FormatFloat(float64(c.RailPitchDeg)))
	}
	if c.BrpTarget != catalog.DefaultBrpTarget() {
		v.Set("brp_x", FormatFloat(c.BrpTarget.X))
		v.Set("brp_y", FormatFloat(c.BrpTarget.Y))
	}
	if c.GripTarget != catalog.DefaultGripTarget() {
		v.Set("grip_x", FormatFloat(c.GripTarget.X))
		v.Set("grip_y", FormatFloat(c.GripTarget.Y))
	}
	if len(v) == 0 {
		return "/"
	}
	return "/?" + v.Encode()
}

// WithoutSelection returns a copy of c with custom selection index removed.
// When removing the last selection and top_per_frame was not explicitly set,
// TopPerFrame reverts to the all-frames default of 1 (Q87).
func (c Compare) WithoutSelection(index int) Compare {
	if index < 0 || index >= len(c.Selections) {
		return c
	}
	next := make([]compare.Selection, 0, len(c.Selections)-1)
	next = append(next, c.Selections[:index]...)
	next = append(next, c.Selections[index+1:]...)
	if len(next) == 0 {
		next = nil
	}
	c.Selections = next
	c.Request.Selections = next
	if len(next) == 0 && !c.TopPerFrameExplicit {
		c.TopPerFrame = 1
		c.Request.TopPerFrame = 1
	}
	return c
}

// AllFrames returns a copy of c cleared back to "All 19 frames" mode with auto
// per-frame ranking (TopPerFrame = 1, TopPerFrameExplicit = false), keeping
// global seatpost, handlebar, posture, target, and overall top settings (Q112).
func (c Compare) AllFrames() Compare {
	c.Selections = nil
	c.Request.Selections = nil
	c.TopPerFrameExplicit = false
	c.TopPerFrame = 1
	c.Request.TopPerFrame = 1
	return c
}
```

### Q3. `internal/reboot/query/compare_test.go`

1. In `TestParseCompareDefaults`: verify `!c.TopPerFrameExplicit && !c.TopExplicit` and `c.URL() == "/"`.
2. In `TestParseCompareSelectionsAndFlags` (`explicit top limits`): verify `c.TopPerFrameExplicit && c.TopExplicit` when both are provided, and `!c.TopPerFrameExplicit && !c.TopExplicit` when omitted in `single selection`.
3. Add `TestCompareURLAndSelectionHelpers`:
   - Round-trip `c.URL()` through `url.Parse` → `query.ParseCompare` for:
     - `""` (`c.URL() == "/"`)
     - `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20"`
     - `"frame=surly-krampus&frame=binary-havok&top_per_frame=1&top=5&post=setback_25&bar=sqlab311_20_16_50&bar_options=all&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730"`
     - `"top_per_frame=0"` (must emit `top_per_frame=0` and keep `TopPerFrameExplicit == true`)
     Verify `Post.ID`, `Bar.ID`, `AllBarOptions`, `BarRollDeg`, `RailPitchDeg`, `BrpTarget`, `GripTarget`, `Selections`, `TopPerFrame`, `TopPerFrameExplicit`, `Top`, `TopExplicit` all round-trip identically.
   - `WithoutSelection`:
     - Starting from `"frame=surly-krampus&frame=binary-havok&post=setback_25"`, `WithoutSelection(0)` leaves only `binary-havok` with `TopPerFrame == 0` and keeps `post=setback_25`; calling `WithoutSelection(0)` again leaves `Selections == nil`, `TopPerFrame == 1`, `!TopPerFrameExplicit`, and `URL() == "/?post=setback_25"`.
     - Out-of-range indices (`-1`, `2`) return `Selections` unchanged.
   - `AllFrames`:
     - Starting from `"frame=surly-krampus&size=XL&top_per_frame=2&top=5&post=setback_25&bar_options=all"`, `c.AllFrames()` has `Selections == nil`, `TopPerFrame == 1`, `!TopPerFrameExplicit`, `Top == 5`, `TopExplicit == true`, `Post.ID == "setback_25"`, `AllBarOptions == true`.
4. Add `TestCompareRowBikeURLRoundTrip` (Q67, Q77, Q110):
   - Positive cases — for each query below, run `ParseCompare` → `compare.Rank` → `c.RowBikeURL(ranking.Rows[0])` → `url.Parse` → `query.ParseBike` → `bike.Evaluate(query.RankedBuildLimit)`:
     1. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20"` → `Score.Tier == FitsAll`, `Score.Total == 84.575011405`, `Builds.Builds[0].Score.Total == 84.575011405` (tol `1e-9`), `bar_options == "selected"`, `fork_options == "selected"`.
     2. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all"` → `Bar.ID == "sqlab311_flx_16_30"`, `bar_options == "all"`, `fork_options == "selected"`, `Score.Total == 86.650135345`, `Builds.Builds[0].Score.Total == 86.650135345` (tol `1e-9`), `Builds.Builds[0].HandlebarID == "sqlab311_flx_16_30"`.
     3. `"frame=binary-havok&size=L&fork=rigid_485"` (sliding dropout) → `Setup.Frame.ChainstayMm == 460`, `Score.Tier == FitsNeither`, `Score.Total == 49.866243596`, `Builds.Builds[0].Score.Total == 49.866243596` (tol `1e-9`).
     4. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730"` → `Score.Tier == FitsAll`, `Score.Total == 94.341904588`, `Builds.Builds[0].Score.Total == 94.341904588` (tol `1e-9`).
   - Also verify all required Q77 query keys are present in `parsedURL.Query()`: `frame`, `size`, `fork`, `sag`, `post`, `post_insert`, `rail_offset`, `rail_pitch`, `bar`, `bar_roll`, `stem_length`, `stem_angle`, `spacers`, `brp_x`, `brp_y`, `grip_x`, `grip_y`, `bar_options`, `fork_options`.
   - Negative cases (validation attached directly to `BikeForRow` and `RowBikeURL`):
     - `compare.Row{FrameID: "nope"}` → error containing `"unknown frame"`.
     - `compare.Row{FrameID: "surly-krampus", Size: "XXL"}` → error containing `"has no size"`.
     - `compare.Row{FrameID: "surly-krampus", Size: "XL", ForkID: "nope"}` → error containing `"has no fork"`.
     - On `mustParseCompare(t, "bar_options=all")`, `compare.Row{FrameID: "surly-krampus", Size: "XL", ForkID: "fox34_120", Build: assembly.ScoredBuild{HasHandlebarOption: true, HandlebarIndex: idx}}` for `idx` in `{-1, 15}` → error containing `"out of range"`.

---

## Commit 2: `Add web comparison page and root navigation`

### W1. `internal/reboot/web/web.go`

1. Embed both templates:
   ```go
   //go:embed templates/bike.html templates/compare.html
   var templateFS embed.FS

   var (
       bikeTemplate    = template.Must(template.ParseFS(templateFS, "templates/bike.html"))
       compareTemplate = template.Must(template.ParseFS(templateFS, "templates/compare.html"))
   )
   ```
2. Replace the `/{$}` redirect in `NewHandler()` with `mux.HandleFunc("/{$}", handleCompare)`. Keep `mux.HandleFunc("/bike", handleBikeWorkbench)`. (Any other path such as `/nope` continues to return `404`.)
3. Add `renderCompareTemplate`:
   ```go
   func renderCompareTemplate(w http.ResponseWriter, status int, view compareView) {
   	var buf bytes.Buffer
   	if err := compareTemplate.Execute(&buf, view); err != nil {
   		http.Error(w, err.Error(), http.StatusInternalServerError)
   		return
   	}

   	w.Header().Set("Content-Type", "text/html; charset=utf-8")
   	w.WriteHeader(status)
   	_, _ = buf.WriteTo(w)
   }
   ```

### W2. New file `internal/reboot/web/compare_page.go`

Imports: `errors`, `fmt`, `math`, `net/http`, `net/url`, `slices`, `strconv`, `strings`, `bikegeo/internal/reboot/assembly`, `bikegeo/internal/reboot/catalog`, `bikegeo/internal/reboot/compare`, `bikegeo/internal/reboot/pose`, `bikegeo/internal/reboot/query`.

Reuse existing helpers in `web` (`makeForkOption`, `forkCellName`, `fitTierPresentation`, `fitTierLegend`, `buildAttributeHeaders`, `frameSelectOption`, `fitTierLegendItem`, `buildAttributeHeader`) instead of duplicating them.

1. **Constants & view types:**

```go
const redshiftMixedStateNote = "Note: default BRP (−219, 713.1) mm comes from the unsagged Redshift transfer; this post models 12 mm sagged setback with no vertical compression adjustment."
```
*(Hint: extract `redshiftMixedStateNote` in `bike_page.go` or `compare_page.go` and share it across both pages.)*

```go
type selectOption struct {
	Value    string
	Label    string
	Selected bool
	Invalid  bool
}

type compareSelectionRowView struct {
	Index           int
	FrameID         string
	FrameLabel      string
	UnknownFrame    bool
	RawSize         string
	RawFork         string
	RawSag          string
	Sizes           []selectOption
	Forks           []selectOption
	Sags            []selectOption
	FrameInvalid    bool
	SizeInvalid     bool
	ForkInvalid     bool
	SagInvalid      bool
	RemoveURL       string
	RemoveAriaLabel string
}

type compareFormValues struct {
	PostOptions          []selectOption
	BarOptions           []selectOption
	BarOptionModes       []selectOption
	BarRoll              string
	RailPitch            string
	BrpX                 string
	BrpY                 string
	GripX                string
	GripY                string
	TopPerFrame          string
	Top                  string
	PostInvalid          bool
	BarInvalid           bool
	BarOptionModeInvalid bool
	BarRollInvalid       bool
	RailPitchInvalid     bool
	BrpXInvalid          bool
	BrpYInvalid          bool
	GripXInvalid         bool
	GripYInvalid         bool
	TopPerFrameInvalid   bool
	TopInvalid           bool
	SelectionRows        []compareSelectionRowView
	AddFrames            []frameSelectOption
	ShowAllFramesLink    bool
	AllFramesURL         string
	RedshiftMixedStateNote string
	ExampleBrpX          string
	ExampleBrpY          string
	ExampleGripX         string
	ExampleGripY         string
	HandlebarCount       int
}

type compareRowView struct {
	Rank              int
	InspectURL        string
	InspectAriaLabel  string
	FrameID           string
	FrameLabel        string
	Size              string
	ForkID            string
	ForkLabel         string
	SlidingDropouts   bool
	Tier              assembly.FitTier
	TierClass         string
	TierLabel         string
	Score             float64
	GripR             float64
	BrpMissText       string
	StemText          string
	SpacersText       string
	BarName           string
	RailOffsetText    string
	PostInsertionText string
	LeveledReachMm    float64
	LeveledStackMm    float64
	EttMm             float64
	LeveledHtaDeg     pose.Deg
	LeveledStaDeg     pose.Deg
	ChainstayMm       float64
	WheelbaseMm       float64
	RcWbRatioPct      float64
	LeveledBbDropMm   float64
	TrailMm           float64
	Attributes        []assembly.AttributeScore
}

type compareSummaryView struct {
	ModeLabel        string
	ShownCount       int
	RowCount         int
	Evaluated        int
	SeatpostName     string
	HandlebarSummary string
	BrpTargetText    string
	GripTargetText   string
	PostureNote      string
}

type compareView struct {
	PageTitle              string
	HeaderBadge            string
	HasResults             bool
	ErrorMessage           string
	Form                   compareFormValues
	Summary                compareSummaryView
	AllBarOptions          bool
	ActiveWeight           float64
	TierLegend             []fitTierLegendItem
	AttributeHeaders       []buildAttributeHeader
	Rows                   []compareRowView
	SlidingDropoutsSummary string
}
```

2. **Formatting helpers (zero `-0.0` in rendered cells):**
   ```go
   func cleanZero(v, halfStep float64) float64 {
       if math.Abs(v) < halfStep {
           return 0
       }
       return v
   }
   ```
   - `BrpMissText`: `fmt.Sprintf("(%+.1f, %+.1f) mm", cleanZero(sc.BrpMissXMm, 0.05), cleanZero(sc.BrpMissYMm, 0.05))`
   - `StemText`: `fmt.Sprintf("%.0f mm, %+.0f°", st.LengthMm, cleanZero(float64(st.AngleDeg), 0.5))`
   - `SpacersText`: `fmt.Sprintf("%.0f mm", st.SpacersMm)`
   - `RailOffsetText`: `fmt.Sprintf("%+.1f mm", cleanZero(setup.Saddle.RailOffsetMm, 0.05))`
   - `PostInsertionText`: `fmt.Sprintf("%.1f mm", setup.Seatpost.InsertionMm)`
   - `InspectAriaLabel`: `fmt.Sprintf("Inspect #%d %s %s, %s", i+1, frameLabel, row.Size, forkLabel)`
   - `RemoveAriaLabel`: `fmt.Sprintf("Remove selection %d: %s", i+1, frameLabel)`

3. **`handleCompare(w http.ResponseWriter, r *http.Request)`:**
   - If `r.URL.Path != "/"`, call `http.NotFound(w, r)` and return.
   - `q := r.URL.Query()`.
   - Call `parsed, err := query.ParseCompare(q)`. If `err != nil`, extract `field := ""` from `*query.InputError`, call `renderCompareError(w, q, field, err.Error())`, return.
   - Call `ranking, err := compare.Rank(parsed.Request)`. If `err != nil`, call `renderCompareError(w, q, "", err.Error())`, return.
   - Call `view, err := buildCompareView(parsed, ranking)`. If `err != nil`, `http.Error(w, err.Error(), http.StatusInternalServerError)` and return.
   - `renderCompareTemplate(w, http.StatusOK, view)`.

4. **`buildCompareView(parsed query.Compare, ranking compare.Ranking) (compareView, error)`:**
   - Pre-index `catalog.Frames()` by ID for `FrameLabel` (`f.Manufacturer + " " + f.Model`), `SlidingDropouts` (`f.MaxChainstayMm > 0`), `MaxChainstayMm`, and `Fork` lookup (`ForkLabel = forkCellName(ff.Fork.Name, ff.Fork.TravelMm, row.SagPct)`).
   - Iterate `ranking.Rows`:
     - Resolve `bar, err := parsed.WinningHandlebar(row)` and `inspectURL, err := parsed.RowBikeURL(row)` (returning error if either fails).
     - Read leveled geometry directly from `fs := row.Build.Placement.Frameset` and `setup := row.Build.Placement.Setup`:
       - `LeveledReachMm: fs.LeveledReachMm()`
       - `LeveledStackMm: fs.LeveledStackMm()`
       - `EttMm: fs.EttMm()`
       - `LeveledHtaDeg: fs.LeveledHeadTubeAngleDeg()`
       - `LeveledStaDeg: fs.LeveledSeatTubeAngleDeg()`
       - `ChainstayMm: setup.Frame.ChainstayMm`
       - `WheelbaseMm: fs.WheelbaseMm()`
       - `RcWbRatioPct: fs.RcWbRatioPct()`
       - `LeveledBbDropMm: fs.LeveledBbDropMm()`
       - `TrailMm: fs.TrailMm()`
     - Track distinct sliding-dropout frames appearing in `ranking.Rows` (by `FrameID`); sort them by `FrameID` ascending and format `SlidingDropoutsSummary` as `strings.Join(..., ", ")` of `fmt.Sprintf("%s %s (%.0f mm)", f.Manufacturer, f.Model, f.MaxChainstayMm)` (or `""` if none).
   - Set `PageTitle` & `HeaderBadge` & `Summary.ModeLabel`:
     - If `len(parsed.Selections) == 0`:
       - `PageTitle = "Compare 19 Frames — bikegeo-reboot"`
       - `HeaderBadge = "All 19 frames"`
       - `ModeLabel = "All 19 frames"`
     - Else (`n := len(parsed.Selections)`):
       - Singular `"1 Setup"` / `"1 selection"` vs plural `"%d Setups"` / `"%d selections"`
       - `PageTitle = fmt.Sprintf("Compare %d %s — bikegeo-reboot", n, ...)`
       - `HeaderBadge = fmt.Sprintf("Custom (%d %s)", n, ...)`
       - `ModeLabel = fmt.Sprintf("Custom (%d %s)", n, ...)`
   - `Summary.HandlebarSummary`: `parsed.Bar.Name` when `!parsed.AllBarOptions`, else `fmt.Sprintf("Best of all %d handlebars", len(catalog.Handlebars()))`.
   - `Summary.BrpTargetText`: `fmt.Sprintf("(%s, %s) mm", formatFormFloat(parsed.BrpTarget.X), formatFormFloat(parsed.BrpTarget.Y))` (or `%.1f`, e.g. `(-219.0, 713.1) mm`).
   - `Summary.GripTargetText`: `fmt.Sprintf("(%.1f, %.1f) mm", parsed.GripTarget.X, parsed.GripTarget.Y)`.
   - `Summary.PostureNote`: non-empty only when `parsed.BarRollDeg != 0 || parsed.RailPitchDeg != 0`, formatted as parts joined by `" · "`: `fmt.Sprintf("Bar roll %+.1f°", parsed.BarRollDeg)`, `fmt.Sprintf("Rail pitch %+.1f°", parsed.RailPitchDeg)`.
   - `TopPerFrame` and `Top` form values: `""` when `!parsed.TopPerFrameExplicit` / `!parsed.TopExplicit`; `strconv.Itoa(parsed.TopPerFrame)` / `strconv.Itoa(parsed.Top)` when explicit! That guarantees adding a frame or submitting the form in default mode keeps `top_per_frame` blank so `ParseCompare` auto-switches from `1` (all frames) to `0` (custom selections) per Q87.
   - `ActiveWeight`: `catalog.DefaultScoreWeights().ActiveTotal(true)` (`36`).

5. **Selection dropdown builders (shared by valid & error views):**
   - `Size` options: first `{Value: "", Label: "Any size", Selected: selectedSize == ""}`, then one option per `f.Sizes` (`Value: sz.Name, Label: sz.Name, Selected: sz.Name == selectedSize`). If `selectedSize != ""` and not found (error path), prepend `{Value: selectedSize, Label: "Invalid: " + selectedSize, Selected: true, Invalid: true}`.
   - `Fork` options: first `{Value: "", Label: "Any fork", Selected: selectedFork == ""}`, then one option per `f.Forks` (`Value: ff.Fork.ID, Label: makeForkOption(ff).Label, Selected: ff.Fork.ID == selectedFork`). If `selectedFork != ""` and not found (error path), prepend `{Value: selectedFork, Label: "Invalid: " + selectedFork, Selected: true, Invalid: true}`.
   - `Sag` options:
     - First `{Value: "", Label: "Any sag", Selected: !hasSag}`
     - Standard steps: `"0"` (`"0% (rigid)"`), `"15"` (`"15%"`), `"20"` (`"20%"`), `"25"` (`"25%"`). Match numeric equality when `hasSag` is true and valid (`*sagPct == step`) or raw trimmed string matches on valid sag.
     - If `hasSag` and valid (passes `ForkSetup{Spec: selectedFork.Spec, SagPct: v}.Validate()`) and not one of `{0, 15, 20, 25}`, append `{Value: query.FormatFloat(v), Label: query.FormatFloat(v) + "%", Selected: true}`.
     - If `hasSag` and invalid (e.g. `sag=20` with a rigid fork, `sag=20` with `fork=""`, `sag=abc`, `sag=105`), keep standard steps unselected and **prepend** `{Value: rawSag, Label: fmt.Sprintf("Invalid: %s", rawSag), Selected: true, Invalid: true}` (or for valid numeric sag `"20"` on a rigid fork, either select `"20"` as invalid or prepend `"Invalid: 20"` — **rule:** always keep `"20"` selected in the `"20%"` option if `"20"` already exists in the dropdown, else prepend `{Value: rawSag, Label: "Invalid: " + rawSag, Selected: true, Invalid: true}`, AND set `SagInvalid = true` so `aria-invalid="true"` highlights the `<select>`! Let's make the test expectation crisp: for `"20"` on rigid fork or without fork, the existing `"20"` option has `selected` and `select[name="sag"]` has `aria-invalid="true"`; for uncataloged `"XXL"` / `"fox36_150"` / `"abc"` / `"nope"`, prepend `<option value="XXL" selected>Invalid: XXL</option>` and set `aria-invalid="true"`).

6. **`renderCompareError(w http.ResponseWriter, q url.Values, invalidField, msg string)` (Q76, Q112):**
   - HTTP status: `http.StatusBadRequest` (`400`), `HasResults: false`, `PageTitle: "Invalid Comparison — bikegeo-reboot"`, `HeaderBadge: "Invalid query"`, `ErrorMessage: msg`.
   - Never drop valid submitted global controls or valid selection rows:
     - `post`: `raw := strings.TrimSpace(q.Get("post"))`; if `raw == ""` default to `catalog.DefaultSeatpostID`; if not in `catalog.Seatposts()`, prepend `{Value: raw, Label: "Invalid: " + raw, Selected: true, Invalid: true}` and set `PostInvalid = true`. Show `RedshiftMixedStateNote` when resolved post ID is `catalog.DefaultSeatpostID`.
     - `bar`: `raw := strings.TrimSpace(q.Get("bar"))`; if `raw == ""` default to `catalog.DefaultHandlebarID`; if not in `catalog.Handlebars()`, prepend invalid option and set `BarInvalid = true`.
     - `bar_options`: `raw := strings.TrimSpace(q.Get("bar_options"))`; `""` selects `"selected"`; `"selected"` or `"all"` selects itself; anything else prepends `{Value: raw, Label: "Invalid: " + raw, Selected: true, Invalid: true}` and sets `BarOptionModeInvalid = true`.
     - `bar_roll`, `rail_pitch`, `top_per_frame`, `top`: echo `strings.TrimSpace(q.Get(key))` (or `"0.0"` when `bar_roll`/`rail_pitch` omitted; `""` when `top_per_frame`/`top` omitted); set `*Invalid = (invalidField == key)`.
     - `brp` / `brp_x` / `brp_y`: if `q.Has("brp_x") || q.Has("brp_y")`, echo `strings.TrimSpace(q.Get("brp_x"))` and `strings.TrimSpace(q.Get("brp_y"))`; else if `strings.TrimSpace(q.Get("brp")) == "off"`, echo `""`, `""`; else echo defaults (`-219.0`, `713.1`). Set `BrpXInvalid = (invalidField == "brp" || invalidField == "brp_x")`, `BrpYInvalid = (invalidField == "brp" || invalidField == "brp_y")`.
     - `grip` / `grip_x` / `grip_y`: exact mirror of BRP (`GripXInvalid`, `GripYInvalid`).
   - Selection rows echoing (`buildErrorSelectionRows(q)`):
     - Let `framesVals := q["frame"]`, `sizeVals := q["size"]`, `forkVals := q["fork"]`, `sagVals := q["sag"]`.
     - If misaligned (`len(framesVals) == 0` when any of `size`/`fork`/`sag` is non-empty, or any non-empty `size`/`fork`/`sag` slice has length `!= len(framesVals)`): return `nil` (empty `SelectionRows`), and set `ShowAllFramesLink` if `len(framesVals) > 0 || len(sizeVals) > 0 || len(forkVals) > 0 || len(sagVals) > 0`.
     - If aligned: iterate `i := range framesVals`, skip all-blank tuples (`f == "" && sz == "" && fk == "" && sg == ""`), and build `RemoveURL` by cloning `q`, removing index `i` from `frame` and from each non-empty column (`size`, `fork`, `sag`), and deleting all 4 keys if every remaining tuple is blank (plus deleting `top_per_frame` if it wasn't explicitly set—wait: on raw `q`, `top_per_frame` is only present if the user/URL explicitly set it!). Encode remaining `q` (`"/"` if empty, else `"/?" + nextQ.Encode()`).
       - If `f == ""` or `frame, ok := catalog.FrameByID(f)` is false: emit row with `UnknownFrame: true`, `FrameID: f`, `FrameLabel` = `"(missing frame)"` (when empty) or `fmt.Sprintf("Invalid frame: %s", f)`, `RawSize: sz`, `RawFork: fk`, `RawSag: sg`, `FrameInvalid: true`, `RemoveURL`.
       - If `frame` exists: check `sz` against `frame.Sizes`, `fk` against `frame.Forks`, and `sg` against `fk` + `ForkSetup.Validate()`, setting `SizeInvalid`, `ForkInvalid`, `SagInvalid` and populating options as specified in §5 above.
   - `AllFramesURL` on error page: clone `q`, delete `frame`, `size`, `fork`, `sag`, `top_per_frame`, return `"/"` if empty else `"/?" + clean.Encode()`.

---

### W3. Exact UX, CSS, and HTML Specification for `internal/reboot/web/templates/compare.html`

**Implement `compare.html` strictly per this spec without changing colors, fonts, grid rules, breakpoints, spacing, semantic tags, or `data-*` attributes.**
- Zero `<script>` tags, zero `on*` event attributes (`Q48`, `Q54`).
- Zero static `style="..."` attributes anywhere in `compare.html`.
- Zero banned words (`chassis`, `price`, `pricing`, `cost`, `cheap`, `ledger`) anywhere in HTML, CSS class names, comments, or copy.
- Never put a heading (`<h2>`, etc.) inside `<summary>` (`accordion` rule).
- Never use CSS `:invalid` alone (use `:user-invalid` and `[aria-invalid="true"]` only, `forms` rule).
- Every `<table>` in results has a `<caption>` (styled `.visually-hidden` because its section `<h2>` immediately precedes it), `<thead>` with `<th scope="col">` on every column, and sits inside `<div class="table-scroll" tabindex="0" role="region" aria-labelledby="...">`.

#### Exact CSS for `compare.html`

```css
:root {
  color-scheme: light;
  --bg: #f6f5f0;
  --surface: #ffffff;
  --surface-subtle: #f9f8f4;
  --border: #d8d5cc;
  --border-strong: #bdb9ae;
  --input-border: #6e7781;
  --text: #1f2328;
  --muted: #57606a;
  --accent: #0969da;
  --accent-hover: #0550ae;
  --accent-subtle: #ddf4ff;
  --accent-border: #80ccff;
  --danger: #a40e26;
  --error: #cf222e;
  --error-text: #82071e;
  --error-bg: #ffebe9;
  --tier-all-bg: #dafbe1;
  --tier-all-text: #116329;
  --tier-all-border: #82e596;
  --tier-brp-bg: #ddf4ff;
  --tier-brp-text: #0550ae;
  --tier-brp-border: #80ccff;
  --tier-grip-bg: #fff8c5;
  --tier-grip-text: #7d4e00;
  --tier-grip-border: #eac54f;
  --tier-neither-bg: #ffebe9;
  --tier-neither-text: #82071e;
  --tier-neither-border: #ffaba8;
  --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-pill: 999px;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font-family: var(--font-sans);
  font-size: 0.9375rem;
  line-height: 1.5;
}
.visually-hidden {
  position: absolute;
  clip-path: inset(50%);
  overflow: hidden;
  inline-size: 1px;
  block-size: 1px;
  margin: -1px;
  padding: 0;
  border: 0;
  white-space: nowrap;
}
.page {
  max-inline-size: 96rem;
  margin-inline: auto;
  padding: 1.25rem 1.5rem 2.5rem;
}
.page-stack {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}
a {
  color: var(--accent);
}
a:focus-visible,
button:focus-visible,
input:focus-visible,
select:focus-visible,
summary:focus-visible,
.table-scroll:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.site-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem 1.25rem;
  padding-block-end: 0.85rem;
  margin-block-end: 1.25rem;
  border-block-end: 1px solid var(--border);
}
.brand {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.65rem;
}
.brand h1 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.badge {
  display: inline-block;
  padding: 0.15rem 0.55rem;
  border-radius: var(--radius-pill);
  background: var(--accent-subtle);
  color: var(--accent-hover);
  border: 1px solid var(--accent-border);
  font-size: 0.78rem;
  font-weight: 600;
}
.site-nav ul {
  list-style: none;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin: 0;
  padding: 0;
}
.site-nav a {
  display: inline-block;
  padding: 0.38rem 0.75rem;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  color: var(--accent);
  font-size: 0.9rem;
  font-weight: 600;
  text-decoration: none;
}
.site-nav a:hover {
  background: var(--surface);
  border-color: var(--border);
  text-decoration: underline;
}
.site-nav a[aria-current="page"] {
  color: var(--text);
  font-weight: 700;
  background: var(--accent-subtle);
  border-color: var(--accent-border);
  box-shadow: inset 0 -2px 0 var(--accent);
}
@media (forced-colors: active) {
  .site-nav a[aria-current="page"] {
    outline: 2px solid Highlight;
    outline-offset: -2px;
  }
}
.controls {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 1rem 1.15rem;
}
.form-error {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.65rem 1rem;
  margin-block-end: 1rem;
  padding: 0.7rem 0.9rem;
  border: 1px solid var(--error);
  border-radius: var(--radius-sm);
  background: var(--error-bg);
  color: var(--error-text);
  font-size: 0.9rem;
}
.control-group {
  margin: 0;
  padding: 0.75rem 0.9rem 0.9rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-subtle);
  min-inline-size: 0;
}
.control-group legend {
  padding: 0 0.35rem;
  font-size: 0.76rem;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.mode-note {
  margin: 0 0 0.75rem;
  font-size: 0.875rem;
  color: var(--muted);
}
.mode-note strong {
  color: var(--text);
}
.selection-table {
  inline-size: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}
.selection-table th,
.selection-table td {
  padding: 0.45rem 0.55rem;
  border-block-end: 1px solid var(--border);
  text-align: left;
  vertical-align: middle;
}
.selection-table thead th {
  background: var(--surface-subtle);
  color: var(--muted);
  font-size: 0.74rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  white-space: nowrap;
}
.selection-table tbody tr:last-child td {
  border-block-end: 0;
}
.selection-table .frame-name {
  font-weight: 600;
  white-space: nowrap;
}
.selection-table .invalid-inline {
  color: var(--error-text);
  font-weight: 600;
}
.selection-table .muted-cell {
  color: var(--muted);
  font-size: 0.84rem;
}
.add-frame-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.55rem 0.85rem;
  margin-block-start: 0.75rem;
}
.add-frame-bar label {
  font-size: 0.84rem;
  font-weight: 600;
  color: var(--text);
}
.add-frame-bar select {
  inline-size: auto;
  min-inline-size: min(100%, 19rem);
  max-inline-size: 100%;
}
.settings-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.9rem;
  margin-block-start: 0.9rem;
}
.field-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 0.85rem;
}
.field-stack {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  min-inline-size: 0;
}
.field label {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text);
}
.controls select,
.controls input[type="number"] {
  inline-size: 100%;
  font: inherit;
  font-size: max(1rem, 0.88em);
  min-block-size: 2.5rem;
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--input-border);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
}
.selection-table select {
  min-inline-size: 8.5rem;
}
.controls :is(input, select):user-invalid,
.controls :is(input, select)[aria-invalid="true"] {
  border-color: var(--error);
  background: var(--error-bg);
}
.field-note {
  margin: 0;
  font-size: 0.8rem;
  color: var(--muted);
  line-height: 1.4;
}
.targets-help {
  margin: 0 0 0.7rem;
}
.control-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.65rem 0.9rem;
  margin-block-start: 1rem;
}
.btn-primary,
.btn-secondary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-block-size: 2.5rem;
  padding: 0.45rem 0.95rem;
  border-radius: var(--radius-sm);
  font: inherit;
  font-size: 0.88rem;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
}
.btn-primary {
  border: 1px solid var(--accent);
  background: var(--accent);
  color: #ffffff;
}
.btn-primary:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}
.btn-secondary {
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--text);
}
.btn-secondary:hover {
  background: var(--surface-subtle);
  text-decoration: none;
}
.remove-link {
  color: var(--danger);
  font-size: 0.84rem;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}
.remove-link:hover {
  text-decoration: underline;
}
.inspect-link {
  color: var(--accent);
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}
.inspect-link:hover {
  text-decoration: underline;
}
.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 1rem 1.15rem;
}
.card h2 {
  margin: 0 0 0.35rem;
  font-size: 1.05rem;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.card-subtitle {
  margin: 0 0 0.75rem;
  font-size: 0.84rem;
  color: var(--muted);
}
.summary-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 0.75rem;
  margin-block: 0.65rem 0.85rem;
}
.chip {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.35rem;
  padding: 0.32rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-subtle);
  font-size: 0.84rem;
}
.chip-label {
  color: var(--muted);
}
.chip-value {
  color: var(--text);
  font-weight: 600;
}
.legend-box {
  background: var(--surface-subtle);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 0.65rem 0.85rem;
}
.tier-legend {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.35rem 0.75rem;
  margin: 0;
  font-size: 0.82rem;
  color: var(--muted);
  align-items: baseline;
}
.tier-legend dt,
.tier-legend dd {
  margin: 0;
}
.tier-badge {
  display: inline-block;
  padding: 0.14rem 0.5rem;
  border-radius: var(--radius-pill);
  border: 1px solid transparent;
  font-size: 0.76rem;
  font-weight: 600;
  white-space: nowrap;
}
.tier-fits-all {
  background: var(--tier-all-bg);
  color: var(--tier-all-text);
  border-color: var(--tier-all-border);
}
.tier-fits-brp {
  background: var(--tier-brp-bg);
  color: var(--tier-brp-text);
  border-color: var(--tier-brp-border);
}
.tier-fits-grip {
  background: var(--tier-grip-bg);
  color: var(--tier-grip-text);
  border-color: var(--tier-grip-border);
}
.tier-fits-neither {
  background: var(--tier-neither-bg);
  color: var(--tier-neither-text);
  border-color: var(--tier-neither-border);
}
.table-scroll {
  overflow-x: auto;
  max-inline-size: 100%;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background:
    linear-gradient(to right, var(--surface) 30%, transparent) left center / 2.25rem 100% no-repeat local,
    linear-gradient(to left, var(--surface) 30%, transparent) right center / 2.25rem 100% no-repeat local,
    radial-gradient(farthest-side at 0 50%, rgb(0 0 0 / 0.14), transparent) left center / 0.75rem 100% no-repeat scroll,
    radial-gradient(farthest-side at 100% 50%, rgb(0 0 0 / 0.14), transparent) right center / 0.75rem 100% no-repeat scroll,
    var(--surface);
}
table.data-table {
  inline-size: 100%;
  border-collapse: separate;
  border-spacing: 0;
  font-size: 0.875rem;
  background: transparent;
}
table.data-table thead th {
  position: sticky;
  inset-block-start: 0;
  z-index: 1;
  background: var(--surface-subtle);
  color: var(--muted);
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  text-align: left;
  white-space: nowrap;
  padding: 0.55rem 0.65rem;
  border-block-end: 2px solid var(--border-strong);
}
table.data-table tbody td {
  padding: 0.5rem 0.65rem;
  border-block-end: 1px solid var(--border);
  vertical-align: middle;
  white-space: nowrap;
  background: transparent;
}
table.data-table tbody tr:last-child td {
  border-block-end: 0;
}
table.data-table tbody tr:hover td {
  background: rgb(9 105 218 / 0.05);
}
table.data-table th.num,
table.data-table td.num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
table.data-table td.num {
  font-family: var(--font-mono);
  font-size: 0.82rem;
}
table.data-table td.score-cell {
  font-weight: 700;
}
.sliding-mark {
  color: var(--accent-hover);
  font-weight: 700;
  text-decoration: none;
  margin-inline-start: 0.1rem;
}
.table-note {
  margin: 0.65rem 0 0;
  font-size: 0.82rem;
  color: var(--muted);
}
.disclosure summary {
  list-style: none;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  font-weight: 600;
  color: var(--accent);
  padding: 0.35rem 0;
  margin-block-end: 0.25rem;
}
.disclosure summary::-webkit-details-marker {
  display: none;
}
.disclosure summary::before {
  content: "";
  inline-size: 0.48em;
  block-size: 0.48em;
  border-inline-end: 2px solid currentColor;
  border-block-end: 2px solid currentColor;
  rotate: -45deg;
  transform-origin: center;
  flex-shrink: 0;
}
.disclosure[open] > summary::before {
  rotate: 45deg;
}
.disclosure .table-scroll {
  margin-block-start: 0.5rem;
}
@media (prefers-reduced-motion: no-preference) {
  .disclosure summary::before {
    transition: rotate 0.2s ease;
  }
}
@supports (interpolate-size: allow-keywords) {
  .disclosure::details-content {
    interpolate-size: allow-keywords;
    overflow: clip;
    contain: layout;
  }
  .disclosure:not([open])::details-content {
    block-size: 0;
  }
  @media (prefers-reduced-motion: no-preference) {
    .disclosure::details-content {
      transition: block-size 0.25s ease, content-visibility 0.25s ease, display 0.25s ease;
      transition-behavior: allow-discrete;
    }
  }
}
@media (max-width: 1024px) {
  .settings-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .settings-grid > .control-group:last-child {
    grid-column: 1 / -1;
  }
}
@media (max-width: 680px) {
  .page {
    padding: 0.9rem 1rem 2rem;
  }
  .settings-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .settings-grid > .control-group:last-child {
    grid-column: auto;
  }
  .field-grid-2 {
    grid-template-columns: minmax(0, 1fr);
  }
  .tier-legend {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.2rem;
  }
  .tier-legend dt {
    margin-block-start: 0.35rem;
  }
}
@media (pointer: coarse) {
  .controls select,
  .controls input[type="number"],
  .btn-primary,
  .btn-secondary {
    min-block-size: 3rem;
  }
}
```

#### Exact HTML Structure & `data-*` Hooks for `compare.html`

1. **`<head>`:** `<meta charset="utf-8">`, `<meta name="viewport" content="width=device-width, initial-scale=1">`, `<title>{{.PageTitle}}</title>`, embedded `<style>` above.
2. **`<header class="site-header">`:**
   ```html
   <header class="site-header">
     <div class="brand">
       <h1>bikegeo-reboot</h1>
       <span class="badge">{{.HeaderBadge}}</span>
     </div>
     <nav class="site-nav" aria-label="Primary">
       <ul>
         <li><a href="/" aria-current="page">Compare</a></li>
         <li><a href="/bike">Bike Inspector</a></li>
       </ul>
     </nav>
   </header>
   ```
3. **`<main class="page-stack">`:**
   - **Form:** `<form class="controls" method="get" action="/" aria-label="Comparison controls">`
   - **Error alert (`{{if .ErrorMessage}}`):**
     ```html
     <div id="form-error" class="form-error" role="alert">
       <div><strong>Error:</strong> {{.ErrorMessage}}</div>
       <a class="btn-secondary" href="/" data-preset="error-reset">Reset Defaults</a>
     </div>
     ```
   - **Fieldset `Bikes to Compare`:**
     - When `not .Form.SelectionRows`:
       `<p class="mode-note" data-compare-mode="all">Comparing <strong>all 19 catalog frames</strong> (best configuration each). Add a frame below to narrow to custom selections.</p>`
     - When `.Form.SelectionRows`:
       `<div class="table-scroll" tabindex="0" role="region" aria-label="Custom frame selections">` wrapping `<table class="selection-table">` with `<thead><tr><th scope="col">#</th><th scope="col">Frame</th><th scope="col">Size</th><th scope="col">Fork</th><th scope="col">Sag</th><th scope="col">Action</th></tr></thead>` and `<tbody>` rows `<tr data-selection-row="{{.Index}}" data-frame-id="{{.FrameID}}">` (with `aria-invalid="true" aria-describedby="form-error"` on invalid selects and `a.remove-link` with `data-remove-selection="{{.Index}}"` and `aria-label="{{.RemoveAriaLabel}}"`).
     - `.add-frame-bar`:
       `<label for="ctrl-add-frame">Add a frame</label><select id="ctrl-add-frame" name="frame"><option value="">Add a frame…</option>{{range .Form.AddFrames}}<option value="{{.ID}}">{{.Label}}</option>{{end}}</select><input type="hidden" name="size" value=""><input type="hidden" name="fork" value=""><input type="hidden" name="sag" value=""><span class="field-note">Pick a frame and click Compare Bikes to narrow by size, fork, and sag.</span>`
   - **3-column `.settings-grid`:**
     - Fieldset `Rider Fit Targets`: `<p id="targets-help" class="field-note targets-help">World coordinates in mm (BB at origin, +X forward, +Y up). Both targets are required. Defaults: BRP ({{.Form.ExampleBrpX}}, {{.Form.ExampleBrpY}}), HandGrip ({{.Form.ExampleGripX}}, {{.Form.ExampleGripY}}).</p>` + `.field-grid-2` containing `BRP X (mm)` (`id="ctrl-brp-x"` `name="brp_x"`), `BRP Y (mm)` (`id="ctrl-brp-y"` `name="brp_y"`), `HandGrip X (mm)` (`id="ctrl-grip-x"` `name="grip_x"`), `HandGrip Y (mm)` (`id="ctrl-grip-y"` `name="grip_y"`), each `type="number" step="any" required aria-describedby="targets-help"`.
     - Fieldset `Cockpit &amp; Seatpost`: `.field-stack` containing `Seatpost` (`id="ctrl-post"` `name="post"`, with `{{if .Form.RedshiftMixedStateNote}}<p class="field-note" data-note="redshift-mixed-state">{{.Form.RedshiftMixedStateNote}}</p>{{end}}`), `Handlebar` (`id="ctrl-bar"` `name="bar"`), `Handlebar options` (`id="ctrl-bar-options"` `name="bar_options"`, with `<p class="field-note" data-note="bar-options-speed">All {{.Form.HandlebarCount}} handlebars evaluates ~15× more builds (~35 s across all 19 frames).</p>`), and `.field-grid-2` containing `Handlebar Roll (°)` (`id="ctrl-bar-roll"` `name="bar_roll"` `type="number" step="0.5"`) and `Saddle Rail Pitch (°)` (`id="ctrl-rail-pitch"` `name="rail_pitch"` `type="number" step="0.5"`).
     - Fieldset `Ranking Limits`: `.field-grid-2` containing `Per frame` (`id="ctrl-top-per-frame"` `name="top_per_frame"` `type="number" min="0" step="1" placeholder="auto" aria-describedby="top-per-frame-help"`) + `<span id="top-per-frame-help" class="field-note">Blank: 1 in all-frames mode, unlimited in custom mode. 0 = unlimited.</span>`, and `Overall limit` (`id="ctrl-top"` `name="top"` `type="number" min="0" step="1" placeholder="all" aria-describedby="top-help"`) + `<span id="top-help" class="field-note">Maximum rows to show overall. Blank or 0 = all.</span>`.
     - On any field where its `*Invalid` bool is true, emit `aria-invalid="true" aria-describedby="form-error"` (using the combined `"targets-help form-error"` etc. where a help text already exists).
   - **`.control-actions`:**
     - `<button id="ctrl-submit" class="btn-primary" type="submit">Compare Bikes</button>`
     - `<a id="preset-defaults" class="btn-secondary" href="/" data-preset="defaults">Reset Defaults</a>`
     - `{{if .Form.ShowAllFramesLink}}<a id="preset-all-frames" class="btn-secondary" href="{{.Form.AllFramesURL}}" data-preset="all-frames">All 19 Frames</a>{{end}}`
   - **Results (`{{if .HasResults}}`):**
     - **Summary Card:** `<section class="card" aria-labelledby="summary-heading">` with `<h2 id="summary-heading">Comparison Summary</h2>`, `.summary-chips` (`data-summary="mode"`, `data-summary="configs"`, `data-summary="post"`, `data-summary="bar"`, `data-summary="targets"`, optional `data-summary="posture"`), and `<div class="legend-box"><dl class="tier-legend" data-fit-tier-legend>...</dl></div>`.
     - **Table 1 (`Fit & Winning Build`):** `<section class="card" data-compare-table="fit" aria-labelledby="fit-heading">` with `<h2 id="fit-heading">Fit &amp; Winning Build</h2>`, `<p class="card-subtitle">...</p>`, `<div class="table-scroll" tabindex="0" role="region" aria-labelledby="fit-heading"><table class="data-table" data-table="fit">`:
       - Columns (`<th scope="col">` on every column, `.num` on numeric columns):
         `<th scope="col" class="num">#</th><th scope="col">Inspect</th><th scope="col">Frame</th><th scope="col">Size</th><th scope="col">Fork</th><th scope="col">Tier</th><th scope="col" class="num">Score</th><th scope="col" class="num">Oval R</th><th scope="col" class="num">BRP miss</th><th scope="col" class="num">Stem</th><th scope="col" class="num">Spacers</th>{{if .AllBarOptions}}<th scope="col">Handlebar</th>{{end}}<th scope="col" class="num">Rail</th><th scope="col" class="num">Post insert</th>`
       - Row `<tr data-compare-rank="{{.Rank}}" data-frame-id="{{.FrameID}}" data-size="{{.Size}}" data-fork-id="{{.ForkID}}">`:
         - `<td class="num">{{.Rank}}</td>`
         - `<td data-compare-cell="inspect"><a class="inspect-link" href="{{.InspectURL}}" data-compare-inspect aria-label="{{.InspectAriaLabel}}">Inspect</a></td>`
         - `<td data-compare-cell="frame">{{.FrameLabel}}{{if .SlidingDropouts}}<abbr class="sliding-mark" title="Sliding dropouts evaluated at max chainstay">*</abbr>{{end}}</td>`
         - `<td data-compare-cell="size">{{.Size}}</td>`
         - `<td data-compare-cell="fork">{{.ForkLabel}}</td>`
         - `<td data-compare-cell="tier"><span class="tier-badge tier-{{.TierClass}}" data-fit-tier="{{.Tier}}">{{.TierLabel}}</span></td>`
         - `<td class="num score-cell" data-compare-cell="score">{{printf "%.1f" .Score}}</td>`
         - `<td class="num" data-compare-cell="oval-r">{{printf "%.2f" .GripR}}</td>`
         - `<td class="num" data-compare-cell="brp-miss">{{.BrpMissText}}</td>`
         - `<td class="num" data-compare-cell="stem">{{.StemText}}</td>`
         - `<td class="num" data-compare-cell="spacers">{{.SpacersText}}</td>`
         - `{{if $.AllBarOptions}}<td data-compare-cell="bar">{{.BarName}}</td>{{end}}`
         - `<td class="num" data-compare-cell="rail">{{.RailOffsetText}}</td>`
         - `<td class="num" data-compare-cell="post-insert">{{.PostInsertionText}}</td>`
     - **Table 2 (`Leveled Geometry & Handling`):** `<section class="card" data-compare-table="geometry" aria-labelledby="geometry-heading">` with `<h2 id="geometry-heading">Leveled Geometry &amp; Handling</h2>`, `<p class="card-subtitle">...</p>`, `<div class="table-scroll" tabindex="0" role="region" aria-labelledby="geometry-heading"><table class="data-table" data-table="geometry">`:
       - Columns (`<th scope="col">` on every column, `.num` on numeric columns):
         `<th scope="col" class="num">#</th><th scope="col">Frame</th><th scope="col">Size</th><th scope="col">Fork</th><th scope="col" class="num">Reach (mm)</th><th scope="col" class="num">Stack (mm)</th><th scope="col" class="num">ETT (mm)</th><th scope="col" class="num">HTA</th><th scope="col" class="num">STA</th><th scope="col" class="num">CS (mm)</th><th scope="col" class="num">WB (mm)</th><th scope="col" class="num">RC/WB</th><th scope="col" class="num">BB drop (mm)</th><th scope="col" class="num">Trail (mm)</th>`
       - Row `<tr data-geometry-rank="{{.Rank}}" data-frame-id="{{.FrameID}}">`:
         - `<td class="num">{{.Rank}}</td>`
         - `<td>{{.FrameLabel}}{{if .SlidingDropouts}}<abbr class="sliding-mark" title="Sliding dropouts evaluated at max chainstay">*</abbr>{{end}}</td>`
         - `<td>{{.Size}}</td>`
         - `<td>{{.ForkLabel}}</td>`
         - `<td class="num" data-geometry-cell="reach">{{printf "%.1f" .LeveledReachMm}}</td>`
         - `<td class="num" data-geometry-cell="stack">{{printf "%.1f" .LeveledStackMm}}</td>`
         - `<td class="num" data-geometry-cell="ett">{{printf "%.1f" .EttMm}}</td>`
         - `<td class="num" data-geometry-cell="hta">{{printf "%.1f°" .LeveledHtaDeg}}</td>`
         - `<td class="num" data-geometry-cell="sta">{{printf "%.1f°" .LeveledStaDeg}}</td>`
         - `<td class="num" data-geometry-cell="chainstay">{{printf "%.1f" .ChainstayMm}}{{if .SlidingDropouts}}<abbr class="sliding-mark" title="Max sliding-dropout position">*</abbr>{{end}}</td>`
         - `<td class="num" data-geometry-cell="wheelbase">{{printf "%.1f" .WheelbaseMm}}</td>`
         - `<td class="num" data-geometry-cell="rc-wb">{{printf "%.1f%%" .RcWbRatioPct}}</td>`
         - `<td class="num" data-geometry-cell="bb-drop">{{printf "%.1f" .LeveledBbDropMm}}</td>`
         - `<td class="num" data-geometry-cell="trail">{{printf "%.1f" .TrailMm}}</td>`
       - Sliding-dropout footnote (`{{if .SlidingDropoutsSummary}}`): `<p class="table-note" data-note="sliding-dropouts">* Sliding dropouts evaluated at max chainstay: {{.SlidingDropoutsSummary}}.</p>`.
     - **Table 3 (`Attribute Score Breakdown`):** `<section class="card" data-compare-table="breakdown" aria-labelledby="breakdown-heading">` with `<h2 id="breakdown-heading">Attribute Score Breakdown</h2>` (**outside** `<details>`!), `<details class="disclosure" data-score-breakdown><summary>Show 12-attribute score breakdown (0–100 weighted, total weight {{printf "%.0f" .ActiveWeight}})</summary><div class="table-scroll" tabindex="0" role="region" aria-labelledby="breakdown-heading"><table class="data-table" data-table="breakdown">`:
       - Columns (`<th scope="col">`): `#` (`.num`), `Frame`, `Size`, `Fork`, `Tier`, `Score` (`.num`), plus `{{range .AttributeHeaders}}<th scope="col" class="num">{{.Label}}</th>{{end}}`.
       - Row `<tr data-breakdown-rank="{{.Rank}}" data-frame-id="{{.FrameID}}">`:
         - `<td class="num">{{.Rank}}</td><td>{{.FrameLabel}}</td><td>{{.Size}}</td><td>{{.ForkLabel}}</td><td><span class="tier-badge tier-{{.TierClass}}">{{.TierLabel}}</span></td><td class="num score-cell">{{printf "%.1f" .Score}}</td>`
         - `{{range .Attributes}}<td class="num" data-breakdown-attr="{{.Attribute}}">{{printf "%.1f" .Score}}</td>{{end}}`

### W4. `internal/reboot/web/templates/bike.html`

1. Update `<header>` markup to match `compare.html`'s primary navigation pattern (pointing `Bike Inspector` to `/bike` with `aria-current="page"` and `Compare` to plain `/`):
   ```html
   <header class="site-header">
     <div class="brand">
       <h1>bikegeo-reboot</h1>
       <span class="badge">{{.BikeName}}</span>
     </div>
     <nav class="site-nav" aria-label="Primary">
       <ul>
         <li><a href="/">Compare</a></li>
         <li><a href="/bike" aria-current="page">Bike Inspector</a></li>
       </ul>
     </nav>
   </header>
   ```
2. Update `bike.html`'s `<style>` block for `.site-header`, `.brand`, and `.site-nav` (with `forced-colors: active` and WCAG 1.4.1 bold + border + bottom bar `[aria-current="page"]`), keeping all existing `/bike` styles (`nav.diagram-views`, `.controls`, `.card`, `.diagram`, `.build-table`, etc.) intact so every existing `/bike` test passes without change.

### W5. `internal/reboot/web/web_test.go`

Replace `TestRootRedirectsToBikeInspector` with `TestRootServesComparePageAndNavLinks`:
- Request `GET /` on `NewHandler()`: assert `200`, `h1` contains `"bikegeo-reboot"`, `nav.site-nav a[aria-current="page"]` has count `1`, text `"Compare"`, `href="/"`, and `nav.site-nav a[href="/bike"]` has count `1` and text `"Bike Inspector"`.
- Request `GET /bike` on `NewHandler()`: assert `200`, `h1` contains `"bikegeo-reboot"`, `nav.site-nav a[aria-current="page"]` has count `1`, text `"Bike Inspector"`, `href="/bike"`, and `nav.site-nav a[href="/"]` has count `1` and text `"Compare"`.

---

## Commit 3: `Add compare page tests and update agent docs`

### T1. New file `internal/reboot/web/compare_test.go`

Browserless `httptest` (`req.Host = "127.0.0.1:8081"`) + `goquery` tests. Add a small helper `loadCompareDoc(t, rawPath) (int, *goquery.Document)` in `compare_test.go`.

1. **`TestCompareDefaultAll19Frames`:**
   - `GET /` → `200`, `<title>` == `"Compare 19 Frames — bikegeo-reboot"`, `[data-compare-mode="all"]` present, `0` `[data-selection-row]` rows, `#preset-all-frames` absent, `[data-note="redshift-mixed-state"]` present, `[data-fit-tier-legend] dt` length `4`, `a#preset-defaults` `href == "/"`.
   - Summary assertions (`[data-summary="configs"]` contains `"19"`, `"701"`, `"525049"`, `[data-summary="post"]` contains `"Redshift ShockStop PRO"`, `[data-summary="bar"]` contains `"Persuader"`).
   - All 3 tables (`table[data-table="fit"]`, `table[data-table="geometry"]`, `table[data-table="breakdown"]`) each have `19` `tbody tr` rows.
   - Exact probed row checks:
     - Row `#1` (`surly-krampus` XL, `fox34_120`, `20%` sag):
       - Fit table: `frame` `"Surly Krampus"` (no `abbr.sliding-mark`), `size` `"XL"`, `fork` `"Fox Float 34 120mm, 20% sag"`, `tier` `data-fit-tier="fits_all"` (`"Fits all"`), `score` `"84.6"`, `oval-r` `"0.76"`, `brp-miss` `"(-4.1, +0.0) mm"`, `stem` `"100 mm, -10°"`, `spacers` `"0 mm"`, `rail` `"+15.0 mm"`, `post-insert` `"163.9 mm"`. `data-compare-cell="bar"` absent (`0` elements).
       - Geometry table: `reach` `"457.2"`, `stack` `"630.5"`, `ett` `"664.1"`, `hta` `"67.8°"`, `sta` `"71.8°"`, `chainstay` `"435.0"` (no `*`), `wheelbase` `"1170.1"`, `rc-wb` `"36.9%"`, `bb-drop` `"56.2"`, `trail` `"104.1"`.
       - Breakdown table (12 attributes): `HandGrip` `"92.4"`, `Saddle BRP Y` `"100.0"`, `Saddle BRP X` `"82.7"`, `Stem` `"62.5"`, `Handlebar sweep` `"100.0"`, `Fork sag` `"100.0"`, `Spacers` `"100.0"`, `Saddle rail offset` `"0.0"`, `RC / WB ratio` `"95.5"`, `Seatpost insertion` `"100.0"`, `Leveled BB drop` `"56.1"`, `Trail` `"100.0"`.
     - Row `#2` (`seven-expedition-926f` `S 64cm`, `sid_100`, `25%`): `tier` `fits_brp`, `score` `"46.9"`, `oval-r` `"1.99"`, `brp-miss` `"(-10.0, -1.3) mm"`, `reach` `"425.0"`, `stack` `"678.1"`, `ett` `"641.1"`.
     - Row `#5` (`salsa-fargo` `XL`, `sid_100`, `20%`): `frame` contains `"Salsa Fargo"` and `abbr.sliding-mark` `"*"`; geometry `chainstay` == `"462.0*"`.
     - Row `#6` (`sour-pasta-party` `L`, `sid_100`, `20%`): `tier` `fits_grip`, `score` `"80.9"`, `brp-miss` `"(-45.9, +0.0) mm"` (verifying `-0.0` normalization!).
     - Row `#19` (`salsa-cutthroat` `60cm`, `sid_100`, `25%`): `tier` `fits_neither`, `score` `"45.9"`, `oval-r` `"3.27"`.
   - `[data-note="sliding-dropouts"]` mentions all 7 sliding-dropout frames (`Binary Havok`, `Binary Kinetik`, `Esker Hayduke`, `Otso Fenrir Stainless`, `Revel El Jefe`, `Salsa Fargo`, `Sklar PBJ`).

2. **`TestCompareCustomSelectionsAndAutoTopPerFrame`:**
   - Untouched Add-a-frame query `/?frame=&size=&fork=&sag=` stays in all-frames mode (`19` rows, `0` `[data-selection-row]`).
   - `/?frame=otso-fenrir-stainless&size=L`: `200`, `1` `[data-selection-row]`, `7` rows in Fit/Geometry/Breakdown tables (`5243` builds evaluated), `#ctrl-top-per-frame` `value == ""`, `#preset-all-frames` `href == "/"`, Row 1 `a[data-remove-selection="1"]` `href == "/"`, ranked row #1 is `sid_100, 25% sag` with `Score` `"58.8"`, `Oval R` `"1.32"`, `Reach` `"427.1"`, `Stack` `"642.6"`, `ETT` `"597.3"`, `CS` `"450.0*"`.
   - Multi-selection `/?frame=surly-krampus&size=XL&fork=fox34_120&sag=20&frame=binary-havok&size=L&fork=rigid_485&sag=&post=setback_25`: `2` selection rows (`Surly Krampus` and `Binary Havok`), `2` ranked rows (`Surly Krampus` #1, `Binary Havok` #2), removing row 1 has `href` keeping `binary-havok` and `post=setback_25` (`GET` that link → `200`, `1` ranked row); `#preset-all-frames` `href` has `post=setback_25` and no `frame`/`size`/`fork`/`sag`/`top_per_frame` (`GET` that link → `200`, `19` rows, `setback_25` selected).
   - Custom URL sag `/?frame=surly-krampus&size=XL&fork=fox34_120&sag=18`: `200`, selection row `select[name="sag"] option[selected]` has `value="18"` (`"18%"`).
   - Explicit limits `/?top_per_frame=1&top=3`: `3` rows, `#ctrl-top-per-frame` `value == "1"`, `#ctrl-top` `value == "3"`.

3. **`TestCompareAllBarOptionsShowsHandlebarColumn`:**
   - `/?frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all`: `200`, Fit table header includes `"Handlebar"`, cell `[data-compare-cell="bar"]` is `"SQlab 311 FL-X Carbon 16°, 30 mm rise (740 mm)"`, `Score` `"86.7"`, `Oval R` `"0.39"`, `Stem` `"100 mm, -7°"`, `[data-summary="configs"]` mentions `"11235"`.

4. **`TestCompareErrorPagesAndRecoveryQ76Q112`:**
   - Every invalid query below returns HTTP `400`, zero `[data-compare-table]`, `#form-error[role="alert"]`, `a[data-preset="error-reset"][href="/"]`, and `a#preset-defaults[href="/"]`:
     - `/?brp=off`, `/?brp_x=&brp_y=` → `#ctrl-brp-x` and `#ctrl-brp-y` both `value=""` and `aria-invalid="true"`.
     - `/?grip=off`, `/?grip_x=&grip_y=` → `#ctrl-grip-x` and `#ctrl-grip-y` both `value=""` and `aria-invalid="true"`.
     - `/?brp_x=-219` → `#ctrl-brp-x` `value="-219"`, `#ctrl-brp-y` `value=""` and `aria-invalid="true"`.
     - `/?frame=surly-krampus&size=XL&fork=surly_krampus_483&sag=20` → row 1 renders `Surly Krampus` with `XL` selected, `surly_krampus_483` selected, `select[name="sag"][aria-invalid="true"]` with `"20"` selected, plus `a[data-remove-selection="1"]` (`href == "/"`). Submitting the recovered query (`sag=`) returns `200`.
     - `/?frame=surly-krampus&size=XL&fork=&sag=20` → row 1 `select[name="sag"][aria-invalid="true"]` with `"20"` selected.
     - `/?frame=surly-krampus&size=XXL` → row 1 `select[name="size"][aria-invalid="true"] option[selected]` has `value="XXL"` (`"Invalid: XXL"`).
     - `/?frame=surly-krampus&fork=fox36_150` → row 1 `select[name="fork"][aria-invalid="true"] option[selected]` has `value="fox36_150"` (`"Invalid: fox36_150"`).
     - `/?frame=nope` → row 1 renders unknown-frame error row (`[aria-invalid="true"]`) with `a[data-remove-selection="1"]` (`href == "/"`).
     - `/?post=nope`, `/?bar=nope`, `/?bar_options=some` → respective `<select aria-invalid="true">` has selected `"Invalid: ..."` option.
     - `/?top=-1`, `/?top_per_frame=abc`, `/?bar_roll=NaN`, `/?rail_pitch=Inf` → respective `<input aria-invalid="true">` echoes raw text.
     - `/?bar_option=all` → `400`, `#form-error` contains `"bar_option"`.
     - `/?frame=surly-krampus&size=XL&size=L` → `400`, `#form-error` contains `"must match"`, `0` `[data-selection-row]` rows.

5. **`TestCompareToBikeLosslessEndToEndQ67Q77Q110`:**
   - For all 4 scenarios:
     1. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20"`
     2. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all"`
     3. `"frame=binary-havok&size=L&fork=rigid_485"` (sliding dropout)
     4. `"frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2&brp_x=-210&brp_y=720&grip_x=470&grip_y=730"`
   - Fetch `GET /?<query>` → extract row #1 `a[data-compare-inspect]` `href` → fetch `GET <href>` against `NewHandler()` → assert `200`, and verify:
     - `/bike` `[data-submitted-tier]` `data-submitted-tier` == `/` row `data-fit-tier`
     - `/bike` `[data-metric="submitted-score"]` == `/` row `[data-compare-cell="score"]`
     - `/bike` `[data-metric="submitted-oval-r"]` == `/` row `[data-compare-cell="oval-r"]`
     - `/bike` `[data-metric="submitted-brp-miss"]` == `/` row `[data-compare-cell="brp-miss"]`
     - All 12 `/bike` `[data-submitted-attr]` scores == all 12 `/` `[data-breakdown-attr]` scores!
     - `/bike` Ranked builds row `#1` (`tr[data-build-rank="1"]`) ALSO matches `/` row's tier, score, stem, spacers, and handlebar!

6. **`TestCompareHTMLAccessibilityAndNoJS`:**
   - On `GET /` (and `GET /?frame=surly-krampus`): assert zero `<script>` tags, zero `[style]` attributes, zero `[onclick]`/`[onchange]`/`[onsubmit]` attributes.
   - Assert every `table.data-table` has `1` `<caption>` with non-empty text, every `thead th` has `scope="col"` and non-empty text, every `.table-scroll` has `tabindex="0"`, `role="region"`, and non-empty `aria-labelledby` or `aria-label`, and `details summary` contains `0` `h1, h2, h3, h4, h5, h6` elements.

### T2. `AGENTS.md`

Under `## Reboot Tree (`cmd/bikegeo-reboot`, `internal/reboot/...`)`:
- **Web** bullet: remove `"/" redirects to "/bike" until the comparison page lands.` and replace with `"/" is the multi-frame comparison matrix and "/bike" is the single-bike inspector.`
- **Catalog** bullet: remove `"/bike" shows only the Otso Fenrir Stainless L until Slice 14.`

---

## Rules that bite

- **Isolation (`make boundary`):** `internal/reboot/query` must NEVER import `net/http`, `html/template`, or `internal/reboot/web`. `internal/reboot/assembly` and `internal/reboot/compare` must NEVER import `catalog` or `query`. Never import legacy `bikegeo/...` packages outside `*_parity_test.go`.
- **Banned terms (`make boundary`):** never write `chassis`, `price`, `pricing`, `cost`, `cheap`, or `ledger` (in any case or inside longer words) anywhere in `cmd/bikegeo-reboot` or `internal/reboot`.
- **Trig lives in `pose`:** `query` and `web` do zero trig (`math.Abs`, `math.IsNaN`, `math.IsInf` are fine).
- **`exhaustive`:** every `switch` on an enum-like type lists every constant.
- **Zero JS (`Q48`, `Q54`):** zero `<script>` tags, inline event handlers, or JS files.
- **Zero inline `style="..."` attributes** in `compare.html`, zero headings inside `<summary>`, no raw CSS `:invalid` selector (use `:user-invalid` + `[aria-invalid="true"]`).
- **Q8:** never use "search" for fitting in code, comments, or docs.
- **No optimization (`Q16`, `Q86`, `Q104`):** keep strict forward `PlaceBike` per candidate in `FitBuilds`.
- **No changes to `pose`, `assembly`, `catalog`, `compare`, or `cmd/bikegeo-reboot`.**
- Run `go vet ./... && go test ./...` after every code change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Plain-language commit subjects matching the three planned commit titles.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 14b Milestones line.

## Verification

1. `make pre-commit` passes on each of the 3 commits individually.
2. `go list -deps ./internal/reboot/query | grep -E '(^net/http$|^html/template$|reboot/web)'` prints nothing.
3. `go test ./internal/reboot/query ./internal/reboot/web -count=1` passes.
4. **Live web check:** build with `make build-reboot`, start `./bin/bikegeo-reboot serve --port 8081` (or ephemeral port), and inspect in Chrome / headless browser + Playwright/DevTools or rendered check:
   - Desktop (`1440px`), tablet (`1024px`), and mobile (`390px`) widths on `/`: verify 3-col → 2-col (last fieldset spanning full width, zero orphan) → 1-col settings grid; all 3 tables readable with right-aligned tabular numbers and scroll shadows; `<details>` 12-attribute score breakdown opens/closes cleanly; sliding-dropout `*` and footnote present; zero JS/console errors.
   - Add a frame via `Add a frame…`, narrow size/fork/sag, trigger rigid+20% sag `400` and confirm highlighted `Sag` recovery, test `Remove` and `All 19 Frames`.
   - Click `Inspect` on row #1 (`Surly Krampus XL`) and a sliding-dropout row (`Salsa Fargo XL`) and confirm `/bike` reproduces exact `Submitted build score` (`84.6`, `Fits all` on Krampus) and `Ranked builds #1`.
   - Click `Compare` in `/bike`'s nav to return to `/`.
   Record observed values and screenshot path (if taken) in the handoff.

## Out of scope

- `/bike` full UX overhaul (`DESIGN.md` Deferred follow-ups lines 412–421).
- Performance optimization (`Q86`, `Q104`).
- Cutover / deleting legacy packages.
- Any changes to `internal/reboot/pose`, `internal/reboot/assembly`, `internal/reboot/catalog`, `internal/reboot/compare`, or `cmd/bikegeo-reboot`.
