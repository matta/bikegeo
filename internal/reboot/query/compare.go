// Package query parses URL query values into catalog-resolved inputs shared by
// the CLI and web handlers (Q88). It has zero net/http or template dependencies.
package query

import (
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/compare"
	"bikegeo/internal/reboot/pose"
)

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
	Post                catalog.Seatpost
	Bar                 catalog.Handlebar
	AllBarOptions       bool
	OfferedBars         []catalog.Handlebar // all catalog bars when AllBarOptions is true; nil otherwise
	BarRollDeg          pose.Deg
	RailPitchDeg        pose.Deg
	BrpTarget           pose.Point
	GripTarget          pose.Point
	Selections          []compare.Selection
	TopPerFrame         int
	Top                 int
	TopPerFrameExplicit bool
	TopExplicit         bool
	Request             compare.Request
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

// selectionCompareKeys are the repeated, aligned selection columns (Q87).
var selectionCompareKeys = []string{"frame", "size", "fork", "sag"}

// singleCompareKeys may each appear at most once.
var singleCompareKeys = []string{
	"top_per_frame", "top",
	"post", "bar", "bar_options",
	"bar_roll", "rail_pitch",
	"brp_x", "brp_y", "brp",
	"grip_x", "grip_y", "grip",
}

// knownCompareKeys are every query key / accepts (Q107).
var knownCompareKeys = slices.Concat(selectionCompareKeys, singleCompareKeys)

// CatalogFrames adapts catalog.Frames() into compare frames in ascending
// frame ID order with declared size and fork order (Q69). Sliding-dropout
// frames are installed at MaxChainstayMm (Q67).
func CatalogFrames() []compare.Frame {
	frames := catalog.Frames()
	out := make([]compare.Frame, len(frames))
	for i, f := range frames {
		sizes := make([]compare.Size, len(f.Sizes))
		for j, sz := range f.Sizes {
			sizes[j] = compare.Size{Name: sz.Name, Frame: f.InstalledFrame(sz)}
		}
		forks := make([]compare.Fork, len(f.Forks))
		for j, fk := range f.Forks {
			forks[j] = compare.Fork{ID: fk.Fork.ID, Spec: fk.Spec()}
		}
		out[i] = compare.Frame{ID: f.ID, Sizes: sizes, Forks: forks}
	}
	return out
}

// ParseCompare parses a / compare query into catalog-resolved inputs and a
// ready-to-run compare.Request, applying the catalog compare defaults (Q53,
// Q56, Q62, Q87). Every rejection is an *InputError naming its query key.
func ParseCompare(q url.Values) (Compare, error) {
	if err := checkKnownKeys(q, knownCompareKeys); err != nil {
		return Compare{}, err
	}
	if err := checkSingleValueKeys(q, singleCompareKeys); err != nil {
		return Compare{}, err
	}

	frames := CatalogFrames()
	selections, err := parseSelections(q, frames)
	if err != nil {
		return Compare{}, err
	}

	topPerFrameExplicit := strings.TrimSpace(q.Get("top_per_frame")) != ""
	topExplicit := strings.TrimSpace(q.Get("top")) != ""
	defaultTopPerFrame := 1
	if len(selections) > 0 {
		defaultTopPerFrame = 0
	}
	topPerFrame, err := parseNonNegInt(q, "top_per_frame", defaultTopPerFrame)
	if err != nil {
		return Compare{}, err
	}
	top, err := parseNonNegInt(q, "top", 0)
	if err != nil {
		return Compare{}, err
	}

	postID := strings.TrimSpace(q.Get("post"))
	if postID == "" {
		postID = catalog.DefaultSeatpostID
	}
	post, ok := catalog.SeatpostByID(postID)
	if !ok {
		return Compare{}, &InputError{Field: "post", Err: fmt.Errorf("unknown seatpost %q", postID)}
	}

	barID := strings.TrimSpace(q.Get("bar"))
	if barID == "" {
		barID = catalog.DefaultHandlebarID
	}
	bar, ok := catalog.HandlebarByID(barID)
	if !ok {
		return Compare{}, &InputError{Field: "bar", Err: fmt.Errorf("unknown handlebar %q", barID)}
	}

	var allBars bool
	switch raw := strings.TrimSpace(q.Get("bar_options")); raw {
	case "", BarOptionsSelected:
		allBars = false
	case BarOptionsAll:
		allBars = true
	default:
		return Compare{}, &InputError{Field: "bar_options", Err: fmt.Errorf("unknown handlebar options value %q", raw)}
	}

	barRollDeg, err := parseAngle(q, "bar_roll", "handlebar roll")
	if err != nil {
		return Compare{}, err
	}
	railPitchDeg, err := parseAngle(q, "rail_pitch", "saddle rail pitch")
	if err != nil {
		return Compare{}, err
	}

	brp, err := parseTarget(q, "brp", "brp_x", "brp_y", "BRP target", catalog.DefaultBrpTarget())
	if err != nil {
		return Compare{}, err
	}
	if brp == nil {
		return Compare{}, &InputError{Field: "brp", Err: fmt.Errorf("compare requires a BRP target")}
	}
	grip, err := parseTarget(q, "grip", "grip_x", "grip_y", "HandGrip target", catalog.DefaultGripTarget())
	if err != nil {
		return Compare{}, err
	}
	if grip == nil {
		return Compare{}, &InputError{Field: "grip", Err: fmt.Errorf("compare requires a HandGrip target")}
	}

	var offeredBars []catalog.Handlebar
	var offeredSetups []assembly.HandlebarSetup
	if allBars {
		offeredBars = catalog.Handlebars()
		offeredSetups = make([]assembly.HandlebarSetup, len(offeredBars))
		for i, b := range offeredBars {
			offeredSetups[i] = b.Setup // BarRollDeg 0: FitBuilds installs Request.Handlebar.BarRollDeg (Q24)
		}
	}

	saddle := catalog.DefaultSaddle().Setup
	saddle.RailPitchDeg = railPitchDeg
	barSetup := bar.Setup
	barSetup.BarRollDeg = barRollDeg

	return Compare{
		Post:                post,
		Bar:                 bar,
		AllBarOptions:       allBars,
		OfferedBars:         offeredBars,
		BarRollDeg:          barRollDeg,
		RailPitchDeg:        railPitchDeg,
		BrpTarget:           *brp,
		GripTarget:          *grip,
		Selections:          selections,
		TopPerFrame:         topPerFrame,
		Top:                 top,
		TopPerFrameExplicit: topPerFrameExplicit,
		TopExplicit:         topExplicit,
		Request: compare.Request{
			Frames:            frames,
			Selections:        selections,
			SuspensionSagPcts: catalog.SuspensionSagPcts(),
			Wheels:            assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm},
			Seatpost:          post.Spec,
			Saddle:            saddle,
			Stem:              catalog.DefaultStemSetup(),
			Handlebar:         barSetup,
			Target:            assembly.FitTarget{BRP: brp, HandGrip: grip},
			Parts: assembly.PartOptions{
				Stems:        catalog.StemOptions(),
				SpacerStepMm: catalog.SpacerStepMm,
				Handlebars:   offeredSetups,
			},
			Weights:     catalog.DefaultScoreWeights(),
			TopPerFrame: topPerFrame,
			Top:         top,
		},
	}, nil
}

// WinningHandlebar returns the catalog handlebar a ranked row's winning build
// installed: the offered bar at HandlebarIndex, or the selected bar (Q106).
func (c Compare) WinningHandlebar(row compare.Row) (catalog.Handlebar, error) {
	if !row.Build.HasHandlebarOption {
		return c.Bar, nil
	}
	i := row.Build.HandlebarIndex
	if i < 0 || i >= len(c.OfferedBars) {
		return catalog.Handlebar{}, fmt.Errorf("frame %q size %q: handlebar option index %d out of range [0, %d)", row.FrameID, row.Size, i, len(c.OfferedBars))
	}
	return c.OfferedBars[i], nil
}

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

// ResolveResult pairs each ranked row with its winning catalog handlebar ID
// (Q106).
func (c Compare) ResolveResult(ranking compare.Ranking) (CompareResult, error) {
	rows := make([]CompareRow, len(ranking.Rows))
	for i, r := range ranking.Rows {
		bar, err := c.WinningHandlebar(r)
		if err != nil {
			return CompareResult{}, err
		}
		rows[i] = CompareRow{Row: r, HandlebarID: bar.ID}
	}
	return CompareResult{
		PostID:        c.Post.ID,
		BarID:         c.Bar.ID,
		AllBarOptions: c.AllBarOptions,
		Rows:          rows,
		RowCount:      ranking.RowCount,
		Evaluated:     ranking.Evaluated,
	}, nil
}

// checkKnownKeys rejects any query key not in known (Q107). The first unknown
// key in ascending order is reported so the error is deterministic.
func checkKnownKeys(q url.Values, known []string) error {
	var unknown []string
	for key := range q {
		if !slices.Contains(known, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	key := unknown[0]
	return &InputError{Field: key, Err: fmt.Errorf("unknown query parameter %q", key)}
}

// checkSingleValueKeys rejects a repeated single-value key.
func checkSingleValueKeys(q url.Values, single []string) error {
	for _, key := range single {
		if len(q[key]) > 1 {
			return &InputError{Field: key, Err: fmt.Errorf("parameter %q may only be specified once", key)}
		}
	}
	return nil
}

// parseSelections reads the aligned frame/size/fork/sag columns into
// selections, validated against frames (Q87). All-blank tuples from unused
// HTML form rows are skipped.
func parseSelections(q url.Values, frames []compare.Frame) ([]compare.Selection, error) {
	framesVals := q["frame"]
	sizeVals := q["size"]
	forkVals := q["fork"]
	sagVals := q["sag"]

	type column struct {
		name string
		vals []string
	}
	columns := []column{{"size", sizeVals}, {"fork", forkVals}, {"sag", sagVals}}

	if len(framesVals) == 0 {
		for _, c := range columns {
			if len(c.vals) > 0 {
				return nil, &InputError{Field: c.name, Err: fmt.Errorf("%q requires \"frame\"", c.name)}
			}
		}
		return nil, nil
	}
	for _, c := range columns {
		if len(c.vals) > 0 && len(c.vals) != len(framesVals) {
			return nil, &InputError{Field: c.name, Err: fmt.Errorf("%q count (%d) must match \"frame\" count (%d)", c.name, len(c.vals), len(framesVals))}
		}
	}

	framesByID := make(map[string]compare.Frame, len(frames))
	for _, f := range frames {
		framesByID[f.ID] = f
	}

	at := func(vals []string, i int) string {
		if len(vals) == 0 {
			return ""
		}
		return strings.TrimSpace(vals[i])
	}

	var selections []compare.Selection
	for i := range framesVals {
		f := strings.TrimSpace(framesVals[i])
		sz := at(sizeVals, i)
		fk := at(forkVals, i)
		sg := at(sagVals, i)

		if f == "" && sz == "" && fk == "" && sg == "" {
			continue
		}
		if f == "" {
			return nil, &InputError{Field: "frame", Err: fmt.Errorf("selection %d is missing \"frame\"", i+1)}
		}
		fr, ok := framesByID[f]
		if !ok {
			return nil, &InputError{Field: "frame", Err: fmt.Errorf("selection %d: unknown frame %q", i+1, f)}
		}
		if sz != "" && !slices.ContainsFunc(fr.Sizes, func(s compare.Size) bool { return s.Name == sz }) {
			return nil, &InputError{Field: "size", Err: fmt.Errorf("selection %d: frame %q has no size %q", i+1, f, sz)}
		}
		var selectedFork compare.Fork
		if fk != "" {
			j := slices.IndexFunc(fr.Forks, func(k compare.Fork) bool { return k.ID == fk })
			if j < 0 {
				return nil, &InputError{Field: "fork", Err: fmt.Errorf("selection %d: frame %q has no fork %q", i+1, f, fk)}
			}
			selectedFork = fr.Forks[j]
		}
		var sagPtr *float64
		if sg != "" {
			if fk == "" {
				return nil, &InputError{Field: "sag", Err: fmt.Errorf("selection %d: sag requires a fork", i+1)}
			}
			v, ok := parseFiniteFloat(sg)
			if !ok {
				return nil, &InputError{Field: "sag", Err: fmt.Errorf("selection %d: fork sag must be a finite number, got %q", i+1, sg)}
			}
			if err := (assembly.ForkSetup{Spec: selectedFork.Spec, SagPct: v}).Validate(); err != nil {
				return nil, &InputError{Field: "sag", Err: fmt.Errorf("selection %d: %w", i+1, err)}
			}
			sagPtr = &v
		}
		selections = append(selections, compare.Selection{FrameID: f, Size: sz, ForkID: fk, SagPct: sagPtr})
	}
	return selections, nil
}

// parseNonNegInt reads a non-negative integer; blank selects defaultVal.
func parseNonNegInt(q url.Values, key string, defaultVal int) (int, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, &InputError{Field: key, Err: fmt.Errorf("%s must be a non-negative integer, got %q", key, raw)}
	}
	return n, nil
}

// parseAngle reads a finite posture angle in degrees; blank selects 0.
func parseAngle(q url.Values, key, label string) (pose.Deg, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return 0, nil
	}
	v, ok := parseFiniteFloat(raw)
	if !ok {
		return 0, &InputError{Field: key, Err: fmt.Errorf("%s must be a finite number, got %q", label, raw)}
	}
	return pose.Deg(v), nil
}

// parseTarget reads a fit target from its x/y keys and its "off" flag (Q53,
// Q70, Q76). A complete coordinate pair wins over "off"; a cleared target
// ("off", or both keys present and blank) returns nil; neither key nor flag
// selects defaultPt.
func parseTarget(q url.Values, flagKey, xKey, yKey, label string, defaultPt pose.Point) (*pose.Point, error) {
	flagRaw := strings.TrimSpace(q.Get(flagKey))
	if flagRaw != "" && flagRaw != "off" {
		return nil, &InputError{Field: flagKey, Err: fmt.Errorf("unknown %s flag %q; only \"off\" is supported", label, flagRaw)}
	}
	hasXKey, hasYKey := q.Has(xKey), q.Has(yKey)
	xRaw, yRaw := strings.TrimSpace(q.Get(xKey)), strings.TrimSpace(q.Get(yKey))
	incomplete := func(field string) error {
		return &InputError{Field: field, Err: fmt.Errorf("%s requires both %s and %s", label, xKey, yKey)}
	}

	switch {
	case xRaw != "" && yRaw != "":
		x, ok := parseFiniteFloat(xRaw)
		if !ok {
			return nil, &InputError{Field: xKey, Err: fmt.Errorf("%s %s must be a finite number, got %q", label, xKey, xRaw)}
		}
		y, ok := parseFiniteFloat(yRaw)
		if !ok {
			return nil, &InputError{Field: yKey, Err: fmt.Errorf("%s %s must be a finite number, got %q", label, yKey, yRaw)}
		}
		return &pose.Point{X: x, Y: y}, nil
	case xRaw == "" && yRaw != "":
		return nil, incomplete(xKey)
	case xRaw != "" && yRaw == "":
		return nil, incomplete(yKey)
	}

	switch {
	case flagRaw == "off" || (hasXKey && hasYKey):
		return nil, nil
	case hasXKey:
		return nil, incomplete(yKey)
	case hasYKey:
		return nil, incomplete(xKey)
	}
	pt := defaultPt
	return &pt, nil
}

// parseFiniteFloat parses raw as a finite float64.
func parseFiniteFloat(raw string) (float64, bool) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
