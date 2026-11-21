# Slice 13f: Shared compare query package and `compare` JSON CLI

Status: Planned 2026-11-21

**Rulings:** Q24, Q49a, Q52, Q53, Q56, Q62, Q67, Q69, Q70, Q76, Q82, Q86, Q87, Q88, Q101, Q103, Q104, Q106, Q107; `AGENTS.md` rules.

**Builds on:** Slice 12c (commits `9942803`, `41f6123`, `425bdb8`, `0933899`) for `internal/reboot/catalog`, Slice 13b–13d (commits `11ecb6d`, `bf99568`, `9676745`, etc.) for `StemOptions()` and `PartOptions` / `BuildRanking`, and Slice 13e (commits `f156358`, `068a628`, `ec2827f`) for `internal/reboot/compare`.

**Size:** larger (8 code/test files, 3 commits). Written for a lower-tier implementer. **Don't make design choices.** If anything below doesn't match committed code, stop and record `Blocked <date> on <question>` (see "Rules that bite").

## Goal

1. Add `MarshalJSON` on `pose.Pose` so `encoding/json` serializes placement poses instead of empty `{}` objects.
2. Add small default/lookup helpers in `internal/reboot/catalog` (`DefaultHandlebarID`, `HandlebarByID`, `DefaultSeatpostID`, `SeatpostByID`, `SpacerStepMm`, `DefaultStemSetup`) and create `internal/reboot/query` (zero `net/http` or `html/template` dependencies) to parse `/` compare `url.Values`, reject unknown keys (Q107), adapt the 19 catalog frames into `compare` inputs at max chainstay (Q67), apply catalog compare defaults (Q53, Q56, Q62, Q87), and resolve winning catalog handlebar IDs onto ranked rows (Q106).
3. Add `bikegeo-reboot compare [query]` to `cmd/bikegeo-reboot` writing indented JSON to stdout via `encoding/json` (Q49a, Q88, Q106).
4. Zero change to `assembly`, `compare`, `web`, or `/bike`.

---

## Commit 1: `Add JSON serialization for pose.Pose`

### P1. `internal/reboot/pose/pose.go`

Add `"encoding/json"` to imports and add `MarshalJSON` right after `HeadingDeg()`:

```go
// MarshalJSON serializes a Pose's unexported Origin and HeadingDeg for
// encoding/json CLI output (Q49a, Q88).
func (p Pose) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Origin     Point
		HeadingDeg Deg
	}{
		Origin:     p.origin,
		HeadingDeg: p.headingDeg,
	})
}
```

### P2. `internal/reboot/pose/pose_test.go`

Add `TestPoseMarshalJSON` (imports `"encoding/json"`):

1. `pose.NewPose(pose.Point{X: -219, Y: 713.1}, 104.75)` marshals to `{"Origin":{"X":-219,"Y":713.1},"HeadingDeg":104.75}`.
2. Zero value `pose.Pose{}` marshals to `{"Origin":{"X":0,"Y":0},"HeadingDeg":0}`.
3. Negative tests (non-finite inputs rejected by `Pose.MarshalJSON` via `json.Marshal`):
   - `pose.NewPose(pose.Point{X: math.NaN(), Y: 0}, 0)` returns a non-nil error.
   - `pose.NewPose(pose.Point{X: 0, Y: math.Inf(1)}, 0)` returns a non-nil error.
   - `pose.NewPose(pose.Point{}, pose.Deg(math.NaN()))` returns a non-nil error.

---

## Commit 2: `Add compare query parsing and catalog adapter`

### C1. `internal/reboot/catalog/handlebars.go`

Add after `Handlebars()`:

```go
// DefaultHandlebarID is the default handlebar for / and /bike (Q56).
const DefaultHandlebarID = "persuader20_800"

// HandlebarByID returns the handlebar with the exact ID.
func HandlebarByID(id string) (Handlebar, bool) {
	for _, b := range Handlebars() {
		if b.ID == id {
			return b, true
		}
	}
	return Handlebar{}, false
}
```

### C2. `internal/reboot/catalog/seatposts.go`

Add after `Seatposts()`:

```go
// DefaultSeatpostID is the default seatpost for / and /bike (Q62, Q78).
const DefaultSeatpostID = "redshift_shockstop_pro"

// SeatpostByID returns the seatpost with the exact ID.
func SeatpostByID(id string) (Seatpost, bool) {
	for _, p := range Seatposts() {
		if p.ID == id {
			return p, true
		}
	}
	return Seatpost{}, false
}
```

### C3. `internal/reboot/catalog/stems.go`

Add after `StemOptions()`:

```go
// SpacerStepMm is the steerer spacer increment build fitting enumerates (Q82).
const SpacerStepMm = 5.0

// DefaultStemSetup returns the baseline stem dimensions used to place builds.
// UpperStackMm (11 mm) and MaxSpacersMm (30 mm) are Q82 constants; ClampHeightMm
// (40 mm) and the continuous-fit length range (40–130 mm) are illustrative.
// Build fitting overwrites LengthMm, AngleDeg, and SpacersMm from StemOptions().
func DefaultStemSetup() assembly.StemSetup {
	return assembly.StemSetup{
		UpperStackMm:  assembly.DefaultUpperStackMm,
		SpacersMm:     20.0,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
		MaxSpacersMm:  assembly.DefaultMaxSpacersMm,
		MinLengthMm:   40.0,
		MaxLengthMm:   130.0,
	}
}
```

### C4. `internal/reboot/catalog/parts_test.go`

Extend tests (or add `TestCatalogDefaultsAndLookups`):

- `catalog.HandlebarByID(catalog.DefaultHandlebarID)` returns `ok == true` and `ID == "persuader20_800"`; `catalog.HandlebarByID("missing")` returns `ok == false`.
- `catalog.SeatpostByID(catalog.DefaultSeatpostID)` returns `ok == true` and `ID == "redshift_shockstop_pro"`; `catalog.SeatpostByID("missing")` returns `ok == false`.
- `catalog.SpacerStepMm == 5.0`.
- `assembly.PlaceStem(pose.NewPose(pose.Point{}, 111.75), catalog.DefaultStemSetup())` succeeds without error, and `DefaultStemSetup()` has `UpperStackMm == assembly.DefaultUpperStackMm`, `MaxSpacersMm == assembly.DefaultMaxSpacersMm`, `ClampHeightMm == 40`.

### Q1. New file `internal/reboot/query/compare.go`

Imports: `fmt`, `math`, `net/url`, `slices`, `strconv`, `strings`, `bikegeo/internal/reboot/assembly`, `bikegeo/internal/reboot/catalog`, `bikegeo/internal/reboot/compare`, `bikegeo/internal/reboot/pose`.
**Never import `net/http`, `html/template`, or `internal/reboot/web`.**

Exported API:

```go
// Package query parses URL query values into catalog-resolved inputs shared by
// the CLI and web handlers (Q88). It has zero net/http or template dependencies.
package query
```

Constants and types:

```go
// bar_options values (Q22, Q101).
const (
	BarOptionsSelected = "selected"
	BarOptionsAll      = "all"
)

// InputError identifies a rejected query parameter. Field is the URL query key
// (for example "post", "frame", "brp_x", or an unrecognized key name).
type InputError struct {
	Field string
	Err   error
}

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

// Compare holds the parsed, catalog-resolved compare query and the
// ready-to-run compare.Request.
type Compare struct {
	Post          catalog.Seatpost
	Bar           catalog.Handlebar
	AllBarOptions bool
	OfferedBars   []catalog.Handlebar // all catalog bars when AllBarOptions is true; nil otherwise
	BarRollDeg    pose.Deg
	RailPitchDeg  pose.Deg
	BrpTarget     pose.Point
	GripTarget    pose.Point
	Selections    []compare.Selection
	TopPerFrame   int
	Top           int
	Request       compare.Request
}

// CompareRow pairs a ranked compare.Row with the winning build's catalog
// handlebar ID (Q106). Embedding compare.Row flattens its JSON fields.
type CompareRow struct {
	compare.Row
	HandlebarID string
}

// CompareResult is a catalog-resolved comparison ranking for CLI JSON output
// (Q49a, Q88, Q106).
type CompareResult struct {
	PostID        string
	BarID         string
	AllBarOptions bool
	Rows          []CompareRow
	RowCount      int
	Evaluated     int
}
```

Functions and methods:

1. **`CatalogFrames() []compare.Frame`**:
   Adapts `catalog.Frames()` into `[]compare.Frame`, preserving ascending frame ID order and declared size/fork order (Q69). For each size `sz`, sets `Frame: f.InstalledFrame(sz)` so sliding-dropout frames use `MaxChainstayMm` (Q67). For each fork `fk`, sets `ID: fk.Fork.ID` and `Spec: fk.Spec()`.

2. **`ParseCompare(q url.Values) (Compare, error)`**:
   Known keys (17 total):
   - Repeated selection keys (4): `frame`, `size`, `fork`, `sag`
   - Single-value keys (13): `top_per_frame`, `top`, `post`, `bar`, `bar_options`, `bar_roll`, `rail_pitch`, `brp_x`, `brp_y`, `brp`, `grip_x`, `grip_y`, `grip`

   Validation order and exact rules (each enforced by its named helper):
   - **`checkKnownKeys(q, knownCompareKeys)` (Q107):** collect any key in `q` not in the 17 known keys; if any exist, sort them ascending (`slices.Sort`) so the error is deterministic and return `&InputError{Field: key, Err: fmt.Errorf("unknown query parameter %q", key)}`.
   - **`checkSingleValueKeys(q, singleCompareKeys)`:** iterate the 13 single-value keys in fixed order; if `len(q[key]) > 1`, return `&InputError{Field: key, Err: fmt.Errorf("parameter %q may only be specified once", key)}`.
   - **`parseSelections(q, frames)` (Q87):**
     - `framesVals := q["frame"]`, `sizeVals := q["size"]`, `forkVals := q["fork"]`, `sagVals := q["sag"]`.
     - If `len(framesVals) == 0`: check `("size", sizeVals)`, `("fork", forkVals)`, `("sag", sagVals)` in that order; if any has `len > 0`, return `&InputError{Field: name, Err: fmt.Errorf("%q requires \"frame\"", name)}`.
     - Otherwise, for each optional column `("size", sizeVals)`, `("fork", forkVals)`, `("sag", sagVals)`: if `len(vals) > 0 && len(vals) != len(framesVals)`, return `&InputError{Field: name, Err: fmt.Errorf("%q count (%d) must match \"frame\" count (%d)", name, len(vals), len(framesVals))}`.
     - Build lookup map `framesByID` from `frames` (the `CatalogFrames()` slice).
     - For `i := 0; i < len(framesVals); i++`:
       - Trim whitespace from `f := framesVals[i]` and from `sz`, `fk`, `sg` (empty if that column slice is empty).
       - If `f == "" && sz == "" && fk == "" && sg == ""`: `continue` (skip unused blank HTML form row).
       - If `f == ""`: return `&InputError{Field: "frame", Err: fmt.Errorf("selection %d is missing \"frame\"", i+1)}`.
       - Look up `fr, ok := framesByID[f]`: if `!ok`, return `&InputError{Field: "frame", Err: fmt.Errorf("selection %d: unknown frame %q", i+1, f)}`.
       - If `sz != ""`: verify `sz` exists in `fr.Sizes`; if not, return `&InputError{Field: "size", Err: fmt.Errorf("selection %d: frame %q has no size %q", i+1, f, sz)}`.
       - If `fk != ""`: find `selectedFork` in `fr.Forks`; if not found, return `&InputError{Field: "fork", Err: fmt.Errorf("selection %d: frame %q has no fork %q", i+1, f, fk)}`.
       - If `sg != ""`:
         - If `fk == ""`: return `&InputError{Field: "sag", Err: fmt.Errorf("selection %d: sag requires a fork", i+1)}`.
         - Parse finite float `v` with `parseFiniteFloat(sg)`; on failure return `&InputError{Field: "sag", Err: fmt.Errorf("selection %d: fork sag must be a finite number, got %q", i+1, sg)}`.
         - Validate physical sag via `(assembly.ForkSetup{Spec: selectedFork.Spec, SagPct: v}).Validate()`; on failure return `&InputError{Field: "sag", Err: fmt.Errorf("selection %d: %w", i+1, err)}`.
         - Set `sagPtr = &v`.
       - Append `compare.Selection{FrameID: f, Size: sz, ForkID: fk, SagPct: sagPtr}`.
   - **`parseNonNegInt(q, key, defaultVal)` (Q87):**
     - `defaultTopPerFrame := 1`; if `len(selections) > 0`, `defaultTopPerFrame = 0`.
     - Parse `top_per_frame` with `defaultTopPerFrame`, then `top` with `0`.
     - Trimmed blank → `defaultVal`. Otherwise `strconv.Atoi(raw)`; if error or `< 0`, return `&InputError{Field: key, Err: fmt.Errorf("%s must be a non-negative integer, got %q", key, raw)}`.
   - **Seatpost (`post`, Q62):** trimmed; if `""`, `catalog.DefaultSeatpostID`. Look up with `catalog.SeatpostByID`; if not found, return `&InputError{Field: "post", Err: fmt.Errorf("unknown seatpost %q", postID)}`.
   - **Handlebar (`bar`, Q56):** trimmed; if `""`, `catalog.DefaultHandlebarID`. Look up with `catalog.HandlebarByID`; if not found, return `&InputError{Field: "bar", Err: fmt.Errorf("unknown handlebar %q", barID)}`. (Validated even when `bar_options=all`.)
   - **Handlebar options (`bar_options`, Q101):** trimmed; `""` or `BarOptionsSelected` → `allBars = false`; `BarOptionsAll` → `allBars = true`; otherwise return `&InputError{Field: "bar_options", Err: fmt.Errorf("unknown handlebar options value %q", raw)}`.
   - **Posture angles (`bar_roll`, `rail_pitch`):** trimmed; `""` → `0`; otherwise finite float via `parseFiniteFloat`, else `&InputError{Field: key, Err: fmt.Errorf("%s must be a finite number, got %q", label, raw)}` (`label`: `"handlebar roll"` / `"saddle rail pitch"`).
   - **Fit targets (`parseTarget`, Q53, Q70, Q76):**
     Unexported helper `parseTarget(q url.Values, flagKey, xKey, yKey, label string, defaultPt pose.Point) (*pose.Point, error)` designed so Slice 14 can reuse it for `/bike` (where `nil` is allowed):
     - `flagRaw := strings.TrimSpace(q.Get(flagKey))`
     - If `flagRaw != "" && flagRaw != "off"`: return `nil, &InputError{Field: flagKey, Err: fmt.Errorf("unknown %s flag %q; only \"off\" is supported", label, flagRaw)}`.
     - `hasXKey, hasYKey := q.Has(xKey), q.Has(yKey)`
     - `xRaw, yRaw := strings.TrimSpace(q.Get(xKey)), strings.TrimSpace(q.Get(yKey))`
     - Case 1 — both non-empty (`xRaw != "" && yRaw != ""`): parse both as finite floats (`Field: xKey` / `Field: yKey` on error) and return `&pose.Point{X: x, Y: y}, nil`. (Takes precedence even if `flagRaw == "off"`, per Q70.)
     - Case 2 — exactly one non-empty: if `xRaw == ""`, return `nil, &InputError{Field: xKey, Err: fmt.Errorf("%s requires both %s and %s", label, xKey, yKey)}`; else `Field: yKey`.
     - Case 3 — both empty (`xRaw == "" && yRaw == ""`):
       - If `flagRaw == "off"` || `(hasXKey && hasYKey)`: return `nil, nil` (deliberately cleared per Q70).
       - If `hasXKey` (and not `hasYKey`): return `nil, &InputError{Field: yKey, Err: fmt.Errorf("%s requires both %s and %s", label, xKey, yKey)}`.
       - If `hasYKey` (and not `hasXKey`): return `nil, &InputError{Field: xKey, Err: fmt.Errorf("%s requires both %s and %s", label, xKey, yKey)}`.
       - Neither key nor `=off` present: copy `defaultPt` and return `&pt, nil`.
     - In `ParseCompare`:
       - Call `parseTarget(q, "brp", "brp_x", "brp_y", "BRP target", catalog.DefaultBrpTarget())`. If `brp == nil`: return `&InputError{Field: "brp", Err: fmt.Errorf("compare requires a BRP target")}`.
       - Call `parseTarget(q, "grip", "grip_x", "grip_y", "HandGrip target", catalog.DefaultGripTarget())`. If `grip == nil`: return `&InputError{Field: "grip", Err: fmt.Errorf("compare requires a HandGrip target")}`.
   - **Assemble `compare.Request`:**
     - `Frames: frames`
     - `Selections: selections`
     - `SuspensionSagPcts: catalog.SuspensionSagPcts()`
     - `Wheels: assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm}`
     - `Seatpost: post.Spec`
     - `Saddle: saddle` (`catalog.DefaultSaddle().Setup` with `RailPitchDeg = railPitchDeg`)
     - `Stem: catalog.DefaultStemSetup()`
     - `Handlebar: barSetup` (`bar.Setup` with `BarRollDeg = barRollDeg`)
     - `Target: assembly.FitTarget{BRP: brp, HandGrip: grip}`
     - `Parts: assembly.PartOptions{Stems: catalog.StemOptions(), SpacerStepMm: catalog.SpacerStepMm, Handlebars: offeredSetups}` (when `allBars`, `offeredBars = catalog.Handlebars()` and `offeredSetups` holds each bar's `Setup` with `BarRollDeg == 0` so `FitBuilds` installs every bar at `Request.Handlebar.BarRollDeg` per Q24; when `!allBars`, both are `nil`)
     - `Weights: catalog.DefaultScoreWeights()`
     - `TopPerFrame: topPerFrame`
     - `Top: top`

3. **`(c Compare) WinningHandlebar(row compare.Row) (catalog.Handlebar, error)`**:
   If `!row.Build.HasHandlebarOption`, return `c.Bar, nil`. Otherwise check `0 <= row.Build.HandlebarIndex && row.Build.HandlebarIndex < len(c.OfferedBars)` and return `c.OfferedBars[row.Build.HandlebarIndex], nil` (else return an error naming the out-of-range index).

4. **`(c Compare) ResolveResult(ranking compare.Ranking) (CompareResult, error)`**:
   Allocates `rows := make([]CompareRow, len(ranking.Rows))`, calls `c.WinningHandlebar(r)` for each row, sets `rows[i] = CompareRow{Row: r, HandlebarID: bar.ID}`, and returns `CompareResult{PostID: c.Post.ID, BarID: c.Bar.ID, AllBarOptions: c.AllBarOptions, Rows: rows, RowCount: ranking.RowCount, Evaluated: ranking.Evaluated}`.

### Q2. New file `internal/reboot/query/compare_test.go` (`package query_test`)

Tests to include (all numerical expectations below come from an independent probe against the committed `catalog` + `compare` packages):

1. **`TestCatalogFrames`**:
   - Calls `query.CatalogFrames()` and checks `len == 19`, matching `catalog.Frames()` order, size count (119 total), fork IDs, and specs.
   - For every frame and size, checks `cf.Sizes[j].Frame == f.InstalledFrame(sz)`.
   - Explicitly checks `binary-havok` size `"L"` has `ChainstayMm == 460` (max chainstay, Q67) whereas `sz.Chart.Frame.ChainstayMm == 445` (published).

2. **`TestParseCompareDefaults`**:
   - Empty `url.Values{}`: `Post.ID == "redshift_shockstop_pro"`, `Bar.ID == "persuader20_800"`, `AllBarOptions == false`, `OfferedBars == nil`, `BarRollDeg == 0`, `RailPitchDeg == 0`, `BrpTarget == {-219, 713.1}`, `GripTarget == {481, 726}`, `len(Selections) == 0`, `TopPerFrame == 1`, `Top == 0`, `Request.Parts.Handlebars == nil`, `len(Request.Parts.Stems) == 107`, `Request.Parts.SpacerStepMm == 5`.
   - All-blank HTML form row `frame=&size=&fork=&sag=`: identical to defaults (`len(Selections) == 0`, `TopPerFrame == 1`).

3. **`TestParseCompareSelectionsAndFlags`**:
   - Single selection `frame=surly-krampus&size=XL&fork=fox34_120&sag=20`: `len(Selections) == 1`, `TopPerFrame == 0`, `*Selections[0].SagPct == 20`.
   - Omitted optional columns `frame=surly-krampus&frame=otso-fenrir-stainless`: `len(Selections) == 2`, `Size == ""`, `ForkID == ""`, `SagPct == nil`, `TopPerFrame == 0`.
   - Trailing blank HTML form slot `frame=surly-krampus&size=XL&fork=&sag=&frame=&size=&fork=&sag=`: `len(Selections) == 1`, `TopPerFrame == 0`.
   - Explicit `top_per_frame=2&top=5` overrides default in both all-frames and custom modes; explicit `top_per_frame=0` in all-frames mode yields `TopPerFrame == 0`.
   - Explicit `sag=0` on a rigid fork `frame=surly-krampus&fork=surly_krampus_483&sag=0`: succeeds with `*SagPct == 0`.
   - URL-decoded space in size `frame=seven-expedition-926d&size=S+58cm`: `Selections[0].Size == "S 58cm"`.
   - `bar_options=all`: `AllBarOptions == true`, `len(OfferedBars) == 15`, `len(Request.Parts.Handlebars) == 15`, every `Request.Parts.Handlebars[i].BarRollDeg == 0`, and with `bar_roll=10`, `Request.Handlebar.BarRollDeg == 10` (Q24).
   - Target `=off` precedence (Q70): `brp_x=-210&brp_y=720&brp=off&grip_x=470&grip_y=730&grip=off` succeeds with `BrpTarget == {-210, 720}` and `GripTarget == {470, 730}`.

4. **`TestParseCompareRejectsInvalidQueries`** (table-driven, asserting both error substring and `errors.As(err, &inputErr)` `inputErr.Field`):
   Every negative test case mapped directly to its validator above:
   - `checkKnownKeys`: `bar_option=all` (`Field: "bar_option"`), `top_per_fram=0` (`Field: "top_per_fram"`), `stock_bars=all` (`Field: "stock_bars"`), `post_ext=200` (`Field: "post_ext"`), `z_bad=1&a_bad=2` (`Field: "a_bad"`).
   - `checkSingleValueKeys`: `top=1&top=2` (`Field: "top"`), `post=setback_0&post=setback_15` (`Field: "post"`), `brp_x=-219&brp_x=-200` (`Field: "brp_x"`).
   - `parseSelections`:
     - `size=L` (`Field: "size"`), `fork=sid_100` (`Field: "fork"`), `sag=20` (`Field: "sag"`), `size=` (`Field: "size"`).
     - `frame=surly-krampus&frame=otso-fenrir-stainless&size=XL` (`Field: "size"`).
     - `frame=surly-krampus&fork=fox34_120&fork=surly_krampus_483` (`Field: "fork"`).
     - `frame=surly-krampus&frame=otso-fenrir-stainless&sag=20` (`Field: "sag"`).
     - `frame=&size=XL&fork=&sag=` (`Field: "frame"`).
     - `frame=surly-krampus&size=XL&fork=&sag=&frame=&size=&fork=sid_100&sag=` (`Field: "frame"`).
     - `frame=unknown-bike` (`Field: "frame"`).
     - `frame=surly-krampus&size=XXL` (`Field: "size"`).
     - `frame=surly-krampus&fork=sid_100` (`Field: "fork"`).
     - `frame=surly-krampus&fork=cad` (`Field: "fork"`).
     - `frame=surly-krampus&size=XL&sag=20` (`Field: "sag"`).
     - `frame=surly-krampus&fork=fox34_120&sag=abc` (`Field: "sag"`), `sag=NaN` (`Field: "sag"`), `sag=Inf` (`Field: "sag"`).
     - `frame=surly-krampus&fork=fox34_120&sag=-5` (`Field: "sag"`), `sag=105` (`Field: "sag"`).
     - `frame=surly-krampus&fork=surly_krampus_483&sag=5` (`Field: "sag"`, error contains `"rigid fork"`).
   - `parseNonNegInt`: `top_per_frame=-1`, `top_per_frame=1.5`, `top_per_frame=abc`, `top=-1`, `top=abc`.
   - `post` / `bar` / `bar_options` / angles: `post=nope`, `bar=nope`, `bar=nope&bar_options=all`, `bar_options=some`, `bar_roll=NaN`, `bar_roll=abc`, `rail_pitch=Inf`, `rail_pitch=abc`.
   - `parseTarget` + required targets:
     - `brp=on` (`Field: "brp"`), `grip=yes` (`Field: "grip"`).
     - `brp_x=-219` (`Field: "brp_y"`), `brp_y=713.1` (`Field: "brp_x"`), `brp_x=-219&brp_y=` (`Field: "brp_y"`), `brp_x=-219&brp=off` (`Field: "brp_y"`), `brp_x=` (`Field: "brp_y"`), `brp_y=` (`Field: "brp_x"`).
     - `grip_x=481` (`Field: "grip_y"`), `grip_y=726` (`Field: "grip_x"`), `grip_x=481&grip_y=` (`Field: "grip_y"`), `grip_x=` (`Field: "grip_y"`).
     - `brp_x=abc&brp_y=713.1` (`Field: "brp_x"`), `brp_x=-219&brp_y=NaN` (`Field: "brp_y"`), `grip_x=Inf&grip_y=726` (`Field: "grip_x"`).
     - `brp=off` (`Field: "brp"`), `brp_x=&brp_y=` (`Field: "brp"`), `brp=off&brp_x=&brp_y=` (`Field: "brp"`).
     - `grip=off` (`Field: "grip"`), `grip_x=&grip_y=` (`Field: "grip"`), `grip=off&grip_x=&grip_y=` (`Field: "grip"`).

5. **`TestParseCompareRankAndResolve`**:
   Runs `ParseCompare` → `compare.Rank` → `ResolveResult` on five narrow catalog queries (all tolerances `1e-6` on floats unless exact):

   | Case | Query | `RowCount` | `Evaluated` | `PostID` | Row 0 `HandlebarID` | `Tier` | `Total` | `GripR` | Stem `(L, deg, sp)` | `InsertionMm` | `RailOffsetMm` | Extra |
   | --- | --- | ---: | ---: | --- | --- | --- | ---: | ---: | --- | ---: | ---: | --- |
   | Selected bar | `frame=surly-krampus&size=XL&fork=fox34_120&sag=20` | 1 | 749 | `redshift_shockstop_pro` | `persuader20_800` | `FitsAll` | `84.575011405` | `0.759424168` | `(100, -10, 0)` | `163.906090` | `15.0` | `AllBarOptions == false` |
   | All bars | `frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all` | 1 | 11235 | `redshift_shockstop_pro` | `sqlab311_flx_16_30` | `FitsAll` | `86.650135345` | `0.389349805` | `(100, -7, 0)` | `163.906090` | `15.0` | `AllBarOptions == true`, `HandlebarIndex == 12` |
   | Sliding dropout | `frame=binary-havok&size=L&fork=rigid_485` | 1 | 749 | `redshift_shockstop_pro` | `persuader20_800` | `FitsNeither` | `49.866243596` | `3.875593862` | `(120, -30, 0)` | `176.175807` | `15.0` | `Setup.Frame.ChainstayMm == 460.0` (Q67) |
   | Non-default parts | `frame=surly-krampus&size=XL&fork=fox34_120&sag=20&post=setback_25&bar=sqlab311_20_16_50&bar_roll=10&rail_pitch=-2` | 1 | 749 | `setback_25` | `sqlab311_20_16_50` | `FitsAll` | `91.201799677` | `0.082545290` | `(80, -30, 0)` | `208.681544` | `6.694435` | `BarID == "sqlab311_20_16_50"` |
   | Custom targets | `frame=surly-krampus&size=XL&fork=fox34_120&sag=20&brp_x=-210&brp_y=720&grip_x=470&grip_y=730` | 1 | 749 | `redshift_shockstop_pro` | `persuader20_800` | `FitsAll` | `93.409769610` | `0.703926294` | `(90, -7, 0)` | `156.644040` | `7.855946` | — |

6. **`TestWinningHandlebarOutOfRange`**:
   Negative test on `Compare.WinningHandlebar`: with `HasHandlebarOption: true` and `HandlebarIndex: -1` or `HandlebarIndex: 15`, returns a non-nil error (and `ResolveResult` propagates it).

7. **`TestParseCompareDefaultAll19Frames`**:
   Runs `ParseCompare(url.Values{})` → `compare.Rank` → `ResolveResult` (~1.8 s on selected bar, Q104):
   - `RowCount == 701`, `Evaluated == 525049`, `len(Rows) == 19`, `PostID == "redshift_shockstop_pro"`, `BarID == "persuader20_800"`, `AllBarOptions == false`.
   - Row 0 (`#1`): `surly-krampus`, `XL`, `fox34_120`, `20`, `FitsAll`, `Total == 84.575011405 ± 1e-6`, `GripR == 0.759424168 ± 1e-6`, `HandlebarID == "persuader20_800"`.
   - Row 1 (`#2`): `seven-expedition-926f`, `S 64cm`, `sid_100`, `25`, `FitsBRP`, `Total == 46.899733053 ± 1e-6`, `GripR == 1.992641645 ± 1e-6`.
   - Row 5 (`#6`): `sour-pasta-party`, `L`, `sid_100`, `20`, `FitsGrip`, `Total == 80.881655493 ± 1e-6`, `GripR == 0.899510156 ± 1e-6`.
   - Row 18 (`#19`): `salsa-cutthroat`, `60cm`, `sid_100`, `25`, `FitsNeither`, `Total == 45.904045523 ± 1e-6`, `GripR == 3.265137449 ± 1e-6`.

---

## Commit 3: `Add bikegeo-reboot compare JSON CLI subcommand`

### M1. `cmd/bikegeo-reboot/main.go`

Imports: add `"encoding/json"`, `"io"`, `"net/url"`, `"strings"`, `"bikegeo/internal/reboot/compare"`, `"bikegeo/internal/reboot/query"`.

Refactor `run` to take `stdout, stderr io.Writer` so tests can invoke it in-process:

```go
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
```

- Top-level usage (printed to `stdout` on empty args, `-h`, `--help`, or `help`, exit `0`; printed to `stderr` on unknown command, exit `2`):
  ```text
  Usage:
    bikegeo-reboot serve [--port N] [--open]
    bikegeo-reboot compare [query]
  ```
- Dispatch `"serve"` (existing logic, writing to `stdout`/`stderr`, including `fs.SetOutput(stderr)`) and `"compare"` (`runCompare(args[1:], stdout, stderr)`).
- `runCompare(args []string, stdout, stderr io.Writer) int`:
  1. If `len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help")`: print concise `compare` usage (syntax, query keys, and two examples) to `stdout`, return `0`.
  2. If `len(args) > 1`: `fmt.Fprintln(stderr, "Usage: bikegeo-reboot compare [query]")`, return `2`.
  3. `raw := ""`; if `len(args) == 1`, `raw = strings.TrimPrefix(args[0], "?")`.
  4. `values, err := url.ParseQuery(raw)`: on error, `fmt.Fprintf(stderr, "Error: invalid query: %v\n", err)`, return `2`.
  5. `parsed, err := query.ParseCompare(values)`: on error, `fmt.Fprintf(stderr, "Error: %v\n", err)`, return `2`.
  6. `ranking, err := compare.Rank(parsed.Request)`: on error, `fmt.Fprintf(stderr, "Error: %v\n", err)`, return `2`.
  7. `result, err := parsed.ResolveResult(ranking)`: on error, `fmt.Fprintf(stderr, "Error: %v\n", err)`, return `1`.
  8. Encode 2-space indented JSON to `stdout`:
     ```go
     enc := json.NewEncoder(stdout)
     enc.SetIndent("", "  ")
     if err := enc.Encode(result); err != nil {
     	fmt.Fprintf(stderr, "Error: encode JSON: %v\n", err)
     	return 1
     }
     return 0
     ```

### M2. New file `cmd/bikegeo-reboot/main_test.go` (`package main`)

Note on JSON unmarshaling of `pose.Pose`: `pose.Pose` intentionally implements `MarshalJSON` only (no `UnmarshalJSON`), so decoding straight into `query.CompareResult` would leave `Pose` fields zeroed. Define a small test-local JSON struct mirroring the fields checked — including `Pose` as `struct{ Origin pose.Point; HeadingDeg pose.Deg }` — so the test actually verifies serialized `Pose` values in the JSON output!

Tests to include:

1. **`TestRunCompareSingleRowJSON`**:
   - Calls `run([]string{"compare", "frame=surly-krampus&size=XL&fork=fox34_120&sag=20"}, &stdout, &stderr)`.
   - Expects exit `0`, empty `stderr`.
   - Unmarshals `stdout` and checks: `PostID == "redshift_shockstop_pro"`, `BarID == "persuader20_800"`, `AllBarOptions == false`, `RowCount == 1`, `Evaluated == 749`, `len(Rows) == 1`, `FrameID == "surly-krampus"`, `Size == "XL"`, `ForkID == "fox34_120"`, `SagPct == 20`, `HandlebarID == "persuader20_800"`, `Build.Score.Tier == "fits_all"`, `Total == 84.575011405 ± 1e-6`, `GripR == 0.759424168 ± 1e-6`, `len(Attributes) == 12`, `Stem` `(100, -10, 0)`.
   - Verifies serialized `Pose` landmarks are non-empty and physical: `Saddle.BRP.Origin.Y == 713.1 ± 1e-6`, `Saddle.BRP.HeadingDeg == 0 ± 1e-9`, `Handlebar.HandGrip.HeadingDeg == 0 ± 1e-9`, `Frameset.HeadTubeTop.Origin.Y > 600`, `Frameset.FrontAxle.Origin.X > 600`.
   - Also verifies leading `?` (`?frame=surly-krampus&size=XL&fork=fox34_120&sag=20`) produces identical stdout and exit `0`.

2. **`TestRunCompareAllBarsJSON`**:
   - Calls `run([]string{"compare", "frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all"}, &stdout, &stderr)`.
   - Expects exit `0`, empty `stderr`, `AllBarOptions == true`, `RowCount == 1`, `Evaluated == 11235`, `Rows[0].HandlebarID == "sqlab311_flx_16_30"`, `Total == 86.650135345 ± 1e-6`.

3. **`TestRunCompareHelpAndUsage`**:
   - `run(nil, ...)` and `run([]string{"--help"}, ...)`: exit `0`, `stdout` mentions both `serve` and `compare`, empty `stderr`.
   - `run([]string{"compare", "--help"}, ...)`: exit `0`, `stdout` mentions `bikegeo-reboot compare [query]`, empty `stderr`.

4. **`TestRunCompareErrors`**:
   Table-driven negative tests asserting exit code, empty `stdout`, and `stderr` substring:
   - Unknown subcommand `[]string{"nope"}` → exit `2`, `"Unknown command"`.
   - Too many args `[]string{"compare", "a=1", "b=2"}` → exit `2`, `"Usage: bikegeo-reboot compare [query]"`.
   - Bad URL escape `[]string{"compare", "frame=%zz"}` → exit `2`, `"invalid query"`.
   - Semicolon query `[]string{"compare", "frame=surly-krampus;size=XL"}` → exit `2`, `"invalid query"`.
   - Unknown query key `[]string{"compare", "bar_option=all"}` → exit `2`, `"unknown query parameter"`.
   - Cleared BRP `[]string{"compare", "brp=off"}` → exit `2`, `"compare requires a BRP target"`.
   - Cleared HandGrip `[]string{"compare", "grip=off"}` → exit `2`, `"compare requires a HandGrip target"`.
   - Unknown frame `[]string{"compare", "frame=nope"}` → exit `2`, `"unknown frame"`.
   - Failing stdout writer on a valid query → exit `1`, `"encode JSON"`.

---

## Mechanics chosen by the planner (veto on review)

1. `pose.Pose.MarshalJSON` serializes `{"Origin":{"X":...,"Y":...},"HeadingDeg":...}`, mirroring `Pose.Origin()`, `Pose.HeadingDeg()`, and `pose.Point` elsewhere in the JSON; no `UnmarshalJSON` is added since nothing unmarshals poses in production.
2. Repeated single-value keys (for example `top=1&top=2`) are rejected by `checkSingleValueKeys` rather than silently taking the first value via `q.Get`.
3. Repeated aligned `frame`/`size`/`fork`/`sag` columns accept omitted optional columns (`len == 0`) or equal lengths (`len == len(frame)`), skip all-blank tuples (`frame=&size=&fork=&sag=` from unused plain-HTML form slots), and reject blank `frame` paired with non-blank `size`/`fork`/`sag` or non-blank `sag` without `fork`.
4. `brp` / `grip` flags accept only absent/blank or `"off"`; a single empty coordinate key without the other (for example `brp_x=` without `brp_y` or `brp=off`) is rejected as an incomplete pair.
5. `query.ParseCompare` validates selection frame/size/fork/sag against the catalog and `ForkSetup.Validate()` upfront so field-attributed `InputError`s return immediately without invoking `compare.Rank`; `compare.Rank` retains its own validation for direct callers.
6. `bikegeo-reboot compare` strips a leading `?` if present, prints 2-space indented JSON to stdout, exits `0` on success/help, `2` on usage/query/validation errors, and `1` on JSON encode/write failure.

## Rules that bite

- `make boundary`: never use the words `price`, `pricing`, `cost`, `cheap`, `ledger`, or `chassis` in reboot file names, identifiers, comments, or tests (in any case or inside longer words).
- Import boundaries: `internal/reboot/query` must NEVER import `net/http`, `html/template`, or `internal/reboot/web`. `internal/reboot/compare` must still NEVER import `catalog` or `query`. `internal/reboot/pose` imports standard library only.
- Trig stays in `internal/reboot/pose`: `query`, `catalog`, and `cmd` must not do trig; `math.IsNaN`/`math.IsInf` (and `math.Abs` in tests) are fine.
- `exhaustive`: any `switch` on an enum-like type must list every constant.
- Q16, Q86, Q104: no optimization (no caching, hoisting, or parallel workers). Keep `compare` tests fast by using narrow selections except for the single default 19-frame test (~1.8 s on selected bar).
- Q8: never use "search" for fitting in code, comments, or docs.
- Don't touch `internal/reboot/assembly`, `internal/reboot/compare`, or `internal/reboot/web` (including `/bike`).
- Run `go vet ./... && go test ./...` after every change and `make pre-commit` before each commit (re-run if `go fix` edits anything).
- Plain-language commit subjects matching the three planned commit titles.
- **Implementation handoff:** finish by recording the handoff in this file per `DESIGN.md` "Implementation handoff": update this `Status:` line (`Implemented <date>; awaiting Matt's acceptance`, `Partly implemented …`, or `Blocked …`), add the `## Implementation handoff` section right after the Status paragraph, and set the same status words on `DESIGN.md`'s Slice 13f Milestones line.

## Verification

1. `make pre-commit` passes on each of the 3 commits individually.
2. `go list -deps ./internal/reboot/query | grep -E '(^net/http$|^html/template$|reboot/web)'` prints nothing.
3. `go test ./internal/reboot/pose ./internal/reboot/catalog ./internal/reboot/query ./cmd/bikegeo-reboot -count=1 -v` passes.
4. **Live CLI check:** build with `make build-reboot` and run:
   - `./bin/bikegeo-reboot compare 'frame=surly-krampus&size=XL&fork=fox34_120&sag=20'` → verify exit `0`, valid JSON with `PostID: "redshift_shockstop_pro"`, `HandlebarID: "persuader20_800"`, `Total ≈ 84.575`, non-empty `Pose` objects (`Origin` + `HeadingDeg`).
   - `./bin/bikegeo-reboot compare 'frame=surly-krampus&size=XL&fork=fox34_120&sag=20&bar_options=all'` → verify `HandlebarID: "sqlab311_flx_16_30"`, `Total ≈ 86.650`.
   - `./bin/bikegeo-reboot compare` (default all 19 frames) → verify `RowCount: 701`, `Evaluated: 525049`, `19` rows, row 1 `surly-krampus XL fox34_120 20%`.
   - `./bin/bikegeo-reboot compare 'bar_option=all'` → verify exit `2` with unknown-parameter error on stderr.
   Record observed values and run time in the handoff (no browser screenshot needed: no web page changed).

## Out of scope

- Moving `/bike` URL parsing into `internal/reboot/query/` or adding `bikegeo-reboot bike [query]` (Slice 14).
- `/` web compare page, `/bike` multi-frame support, switching `/bike`'s default post from `setback_0` to `redshift_shockstop_pro`, or `/bike` link generation (Slice 14).
- Rejecting unknown query keys on `/bike` or other future endpoints (tracked under `DESIGN.md` Deferred follow-ups).
- Any change to `assembly`, `compare`, fitting, scoring, or catalog geometry values.
- Performance optimization (Q86, Q104).
