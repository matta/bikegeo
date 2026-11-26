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

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/pose"
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
	BrpTarget           pose.Point[assembly.DynamicWorld]
	GripTarget          pose.Point[assembly.DynamicWorld]
	TopPerFrame         int
	Top                 int
	TopPerFrameExplicit bool // whether top_per_frame was set in the query (distinguishes an explicit "1" from the omitted default)
	TopExplicit         bool // whether top was explicitly set in the query (distinguishes explicit "0" from omitted)
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

// knownCompareKeys are every query key / accepts (Q107); each may appear at
// most once.
var knownCompareKeys = []string{
	"top_per_frame", "top",
	"post", "bar", "bar_options",
	"brp_x", "brp_y", "brp",
	"grip_x", "grip_y", "grip",
}

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
		opts := f.ForkOptions()
		forks := make([]compare.Fork, len(opts))
		for j, opt := range opts {
			forks[j] = compare.Fork{ID: opt.ID(), Spec: opt.Spec(), LowerCup: opt.LowerCup}
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
	if err := checkSingleValueKeys(q, knownCompareKeys); err != nil {
		return Compare{}, err
	}

	topPerFrameExplicit := strings.TrimSpace(q.Get("top_per_frame")) != ""
	topExplicit := strings.TrimSpace(q.Get("top")) != ""
	topPerFrame, err := parseNonNegInt(q, "top_per_frame", 1)
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
			offeredSetups[i] = b.Setup
		}
	}

	return Compare{
		Post:                post,
		Bar:                 bar,
		AllBarOptions:       allBars,
		OfferedBars:         offeredBars,
		BrpTarget:           *brp,
		GripTarget:          *grip,
		TopPerFrame:         topPerFrame,
		Top:                 top,
		TopPerFrameExplicit: topPerFrameExplicit,
		TopExplicit:         topExplicit,
		Request: compare.Request{
			Frames:    CatalogFrames(),
			Wheels:    assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm},
			Seatpost:  post.Spec,
			Saddle:    catalog.DefaultSaddle().Setup,
			Stem:      catalog.DefaultStemSetup(),
			Handlebar: bar.Setup,
			Target:    assembly.FitTarget{BRP: brp, HandGrip: grip},
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
	opt, ok := findForkOption(frame, row.ForkID)
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
		ForkOption:     opt,
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

// parseTarget reads a fit target from its x/y keys and its "off" flag (Q53,
// Q70, Q76). A complete coordinate pair wins over "off"; a cleared target
// ("off", or both keys present and blank) returns nil; neither key nor flag
// selects defaultPt.
func parseTarget(q url.Values, flagKey, xKey, yKey, label string, defaultPt pose.Point[assembly.DynamicWorld]) (*pose.Point[assembly.DynamicWorld], error) {
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
		return &pose.Point[assembly.DynamicWorld]{X: x, Y: y}, nil
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
