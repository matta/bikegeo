package query

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/pose"
)

// BasisForkID is the reserved fork query value selecting the frame's chart
// basis fork and basis lower cup.
const BasisForkID = "basis"

// fork_options values.
const (
	ForkOptionsSelected = "selected"
	ForkOptionsAll      = "all"
)

// Diagram view values for /bike.
const (
	ViewBike = "bike"
	ViewSeat = "seat"
	ViewStem = "stem"
)

// RankedBuildLimit is the default number of ranked builds on /bike and CLI.
const RankedBuildLimit = 5

// BikeForkOption is one physical fork configuration a frame offers to build
// fitting when fork_options=all.
type BikeForkOption struct {
	ForkOption catalog.ForkOption
	SagPct     float64
	LowerCup   assembly.LowerCupSetup
}

// BikeParams holds the numeric geometry inputs parsed from a /bike query.
// Keeping raw numbers allows error pages to echo rejected values while
// Bike.Assemble validates and clamps the physical parts.
type BikeParams struct {
	ForkA2cMm     float64
	LowerCupMm    float64
	SagPct        float64
	SpacersMm     float64
	StemLengthMm  float64
	StemAngleDeg  pose.Deg
	BarRollDeg    pose.Deg
	PostInsertMm  float64
	RailOffsetMm  float64
	RailPitchDeg  pose.Deg
	WheelRadiusMm float64
}

func paramsFromSetup(s assembly.BikeSetup) BikeParams {
	return BikeParams{
		ForkA2cMm:     s.Fork.Spec.StaticA2cMm,
		LowerCupMm:    s.LowerCup.StackMm,
		SagPct:        s.Fork.SagPct,
		SpacersMm:     s.Stem.SpacersMm,
		StemLengthMm:  s.Stem.LengthMm,
		StemAngleDeg:  s.Stem.AngleDeg,
		BarRollDeg:    s.Handlebar.BarRollDeg,
		PostInsertMm:  s.Seatpost.InsertionMm,
		RailOffsetMm:  s.Saddle.RailOffsetMm,
		RailPitchDeg:  s.Saddle.RailPitchDeg,
		WheelRadiusMm: s.Wheels.RadiusMm,
	}
}

// Bike holds the parsed, catalog-resolved /bike query and numeric build inputs.
type Bike struct {
	Frame          catalog.Frame
	Size           catalog.FrameSize
	ForkID         string             // concrete fork variant ID or BasisForkID
	ForkOption     catalog.ForkOption // zero when ForkID == BasisForkID
	Bar            catalog.Handlebar
	Post           catalog.Seatpost
	View           string // ViewBike, ViewSeat, or ViewStem
	BrpTarget      *pose.Point
	GripTarget     *pose.Point
	AllBarOptions  bool
	AllForkOptions bool
	Params         BikeParams
}

// BikeScoredBuild pairs a ranked assembly.ScoredBuild with the build's catalog
// handlebar and fork IDs. Embedding ScoredBuild flattens its JSON fields.
type BikeScoredBuild struct {
	assembly.ScoredBuild
	HandlebarID string
	ForkID      string
}

// BikeBuildRanking is the catalog-resolved ranked builds list for /bike and
// bikegeo bike JSON output.
type BikeBuildRanking struct {
	Evaluated int
	Builds    []BikeScoredBuild
}

// BikeResult is a catalog-resolved single-bike evaluation for /bike and
// bikegeo bike JSON output.
type BikeResult struct {
	FrameID         string
	Size            string
	ForkID          string
	BarID           string
	PostID          string
	AllBarOptions   bool
	AllForkOptions  bool
	Placement       assembly.Placement
	Score           *assembly.BuildScore // non-nil when HandGrip target is set and stem is in catalog
	Fitting         *assembly.Fitting    // non-nil when BRP or HandGrip target is set
	SetbackCentered *assembly.Fitting    // non-nil when BRP target is set
	SetbackReach    *assembly.Fitting    // non-nil when BRP target is set and saddle fit is limited
	Builds          *BikeBuildRanking    // non-nil when HandGrip target is set
}

// knownBikeKeys are every query key /bike accepts; all are single-valued.
var knownBikeKeys = []string{
	"frame", "size",
	"fork", "fork_a2c", "cup", "sag",
	"bar", "spacers", "stem_length", "stem_angle", "bar_roll",
	"post", "post_insert", "rail_offset", "rail_pitch",
	"brp_x", "brp_y", "brp",
	"grip_x", "grip_y", "grip",
	"bar_options", "fork_options",
	"view",
	"prev_frame", "prev_size", "prev_fork", "prev_post",
}

// FormatFloat formats v losslessly for URLs, normalizing -0 to "0".
func FormatFloat(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// DefaultBike returns the default /bike build.
func DefaultBike() Bike {
	b, err := ParseBike(nil)
	if err != nil {
		panic(fmt.Sprintf("default bike is invalid: %v", err))
	}
	return b
}

// findForkOption returns the frame's single concrete ForkOption with the given ID.
func findForkOption(frame catalog.Frame, id string) (catalog.ForkOption, bool) {
	matches, ok := frame.LookupFork(id)
	if !ok || len(matches) != 1 {
		return catalog.ForkOption{}, false
	}
	return matches[0], true
}

// parseFloatOrDefault reads a finite float; blank selects def.
func parseFloatOrDefault(q url.Values, key string, def float64, label string) (float64, error) {
	raw := strings.TrimSpace(q.Get(key))
	if raw == "" {
		return def, nil
	}
	v, ok := parseFiniteFloat(raw)
	if !ok {
		return 0, &InputError{Field: key, Err: fmt.Errorf("%s must be a finite number, got %q", label, raw)}
	}
	return v, nil
}

// ParseBike parses a /bike query into a catalog-resolved Bike, applying the
// catalog defaults and manual-switch rules. Every rejection is an *InputError
// naming its query key.
func ParseBike(q url.Values) (Bike, error) {
	if err := checkKnownKeys(q, knownBikeKeys); err != nil {
		return Bike{}, err
	}
	if err := checkSingleValueKeys(q, knownBikeKeys); err != nil {
		return Bike{}, err
	}

	frame, size, frameSwitched, sizeSwitched, err := parseBikeFrameAndSize(q)
	if err != nil {
		return Bike{}, err
	}

	b := Bike{Frame: frame, Size: size}
	a2cMm, cupMm, sagPct, err := parseBikeFork(q, &b, frameSwitched)
	if err != nil {
		return Bike{}, err
	}

	barID := strings.TrimSpace(q.Get("bar"))
	if barID == "" {
		barID = catalog.DefaultHandlebarID
	}
	bar, ok := catalog.HandlebarByID(barID)
	if !ok {
		return Bike{}, &InputError{Field: "bar", Err: fmt.Errorf("unknown handlebar %q", barID)}
	}
	postID := strings.TrimSpace(q.Get("post"))
	if postID == "" {
		postID = catalog.DefaultSeatpostID
	}
	post, ok := catalog.SeatpostByID(postID)
	if !ok {
		return Bike{}, &InputError{Field: "post", Err: fmt.Errorf("unknown seatpost %q", postID)}
	}
	prevPost := strings.TrimSpace(q.Get("prev_post"))
	postSwitched := prevPost != "" && prevPost != post.ID

	framePart, err := b.FramePart()
	if err != nil {
		return Bike{}, &InputError{Field: "frame", Err: err}
	}
	insertionMm, err := parseBikePostInsertion(q, framePart, post, frameSwitched || sizeSwitched || postSwitched)
	if err != nil {
		return Bike{}, err
	}

	spacers, err := parseFloatOrDefault(q, "spacers", catalog.DefaultSpacersMm, "spacers")
	if err != nil {
		return Bike{}, err
	}
	stemLength, err := parseFloatOrDefault(q, "stem_length", catalog.DefaultStemLengthMm, "stem length")
	if err != nil {
		return Bike{}, err
	}
	stemAngle, err := parseFloatOrDefault(q, "stem_angle", float64(catalog.DefaultStemAngleDeg), "stem angle")
	if err != nil {
		return Bike{}, err
	}
	barRoll, err := parseFloatOrDefault(q, "bar_roll", 0, "handlebar roll")
	if err != nil {
		return Bike{}, err
	}
	railOffset, err := parseFloatOrDefault(q, "rail_offset", 0, "saddle rail offset")
	if err != nil {
		return Bike{}, err
	}
	railPitch, err := parseFloatOrDefault(q, "rail_pitch", 0, "saddle rail pitch")
	if err != nil {
		return Bike{}, err
	}

	brp, err := parseTarget(q, "brp", "brp_x", "brp_y", "BRP target", catalog.DefaultBrpTarget())
	if err != nil {
		return Bike{}, err
	}
	grip, err := parseTarget(q, "grip", "grip_x", "grip_y", "HandGrip target", catalog.DefaultGripTarget())
	if err != nil {
		return Bike{}, err
	}

	allBars, allForks, view, err := parseBikeOptionsAndView(q)
	if err != nil {
		return Bike{}, err
	}

	b.Bar = bar
	b.Post = post
	b.View = view
	b.BrpTarget = brp
	b.GripTarget = grip
	b.AllBarOptions = allBars
	b.AllForkOptions = allForks
	b.Params = BikeParams{
		ForkA2cMm:     a2cMm,
		LowerCupMm:    cupMm,
		SagPct:        sagPct,
		SpacersMm:     spacers,
		StemLengthMm:  stemLength,
		StemAngleDeg:  pose.Deg(stemAngle),
		BarRollDeg:    pose.Deg(barRoll),
		PostInsertMm:  insertionMm,
		RailOffsetMm:  railOffset,
		RailPitchDeg:  pose.Deg(railPitch),
		WheelRadiusMm: catalog.WheelRadiusMm,
	}
	return b, nil
}

// parseBikeFrameAndSize resolves frame and size, honoring manual-switch
// fallbacks driven by prev_frame and prev_size.
func parseBikeFrameAndSize(q url.Values) (frame catalog.Frame, size catalog.FrameSize, frameSwitched, sizeSwitched bool, err error) {
	frameID := strings.TrimSpace(q.Get("frame"))
	if frameID == "" {
		frameID = catalog.DefaultFrameID
	}
	frame, ok := catalog.FrameByID(frameID)
	if !ok {
		return frame, size, false, false, &InputError{Field: "frame", Err: fmt.Errorf("unknown frame %q", frameID)}
	}
	prevFrame := strings.TrimSpace(q.Get("prev_frame"))
	frameSwitched = prevFrame != "" && prevFrame != frame.ID

	rawSize := strings.TrimSpace(q.Get("size"))
	switch sz, found := frame.Size(rawSize); {
	case rawSize == "":
		size, _ = frame.Size(frame.DefaultSize)
	case found:
		size = sz
	case frameSwitched:
		size, _ = frame.Size(frame.DefaultSize)
	default:
		return frame, size, false, false, &InputError{Field: "size", Err: fmt.Errorf("frame %q has no size %q", frame.ID, rawSize)}
	}
	prevSize := strings.TrimSpace(q.Get("prev_size"))
	sizeSwitched = prevSize != "" && prevSize != size.Name
	return frame, size, frameSwitched, sizeSwitched, nil
}

// parseBikeFork resolves the fork, static A2C, lower cup stack, and sag,
// recording ForkID and ForkOption on b.
func parseBikeFork(q url.Values, b *Bike, frameSwitched bool) (a2cMm, cupMm, sagPct float64, err error) {
	frame := b.Frame
	rawFork := strings.TrimSpace(q.Get("fork"))
	prevFork := strings.TrimSpace(q.Get("prev_fork"))

	var opt catalog.ForkOption
	var isBasis bool
	var forkID string
	matches, ok := frame.LookupFork(rawFork)
	switch {
	case rawFork == "":
		opt = frame.ForkOptions()[0]
		forkID = opt.ID()
	case rawFork == BasisForkID:
		isBasis = true
		forkID = BasisForkID
	case ok && len(matches) == 1:
		opt = matches[0]
		forkID = opt.ID()
	case ok && len(matches) > 1:
		if frameSwitched {
			opt = frame.ForkOptions()[0]
			forkID = opt.ID()
		} else {
			return 0, 0, 0, &InputError{Field: "fork", Err: fmt.Errorf("fork %q has multiple variants; select a specific variant such as %q", rawFork, matches[0].ID())}
		}
	case frameSwitched:
		opt = frame.ForkOptions()[0]
		forkID = opt.ID()
	default:
		return 0, 0, 0, &InputError{Field: "fork", Err: fmt.Errorf("frame %q has no fork %q", frame.ID, rawFork)}
	}
	forkSwitched := !frameSwitched && prevFork != "" && prevFork != forkID

	b.ForkID = forkID
	if isBasis {
		b.ForkOption = catalog.ForkOption{}
		return frame.BasisFork.BasisA2cMm(), frame.BasisLowerCup.StackMm, 0, nil
	}
	b.ForkOption = opt

	defaultSag := opt.Fork.Travel.DefaultSagPct()
	defaultCup := opt.LowerCup.StackMm

	switch {
	case frameSwitched:
		a2cMm, cupMm, sagPct = opt.Variant.StaticA2cMm, defaultCup, defaultSag
	case forkSwitched:
		a2cMm, sagPct = opt.Variant.StaticA2cMm, defaultSag
		var prevCupStd assembly.LowerCupStandard
		if prevFork == BasisForkID {
			prevCupStd = frame.BasisLowerCup.Standard
		} else if prevMatches, known := frame.LookupFork(prevFork); known {
			prevCupStd = prevMatches[0].LowerCup.Standard
		}
		// The basis's cup stack is locked to the chart, so leaving it always
		// restores the physical default, even when both share a standard.
		if prevFork == BasisForkID || prevCupStd != opt.LowerCup.Standard {
			cupMm = defaultCup
		} else if cupMm, err = parseFloatOrDefault(q, "cup", defaultCup, "lower cup"); err != nil {
			return 0, 0, 0, err
		}
	default:
		if a2cMm, err = parseFloatOrDefault(q, "fork_a2c", opt.Variant.StaticA2cMm, "fork static A2C"); err != nil {
			return 0, 0, 0, err
		}
		if cupMm, err = parseFloatOrDefault(q, "cup", defaultCup, "lower cup"); err != nil {
			return 0, 0, 0, err
		}
		if sagPct, err = parseFloatOrDefault(q, "sag", defaultSag, "fork sag"); err != nil {
			return 0, 0, 0, err
		}
	}
	return a2cMm, cupMm, sagPct, nil
}

// parseBikePostInsertion reads post_insert, defaulting to the midpoint of the
// frame/post bounds and clamping it into them after a manual switch.
func parseBikePostInsertion(q url.Values, frame assembly.Frame, post catalog.Seatpost, clamp bool) (float64, error) {
	postDoF, err := assembly.MateSeatpost(frame.SeatTube(), post.Part)
	if err != nil {
		return 0, &InputError{Field: "post", Err: err}
	}
	bounds := postDoF.AllowedInsertion()
	defaultInsert := (bounds.Min() + bounds.Max()) / 2.0
	insertionMm, err := parseFloatOrDefault(q, "post_insert", defaultInsert, "seatpost insertion")
	if err != nil {
		return 0, err
	}
	if clamp {
		insertionMm = min(max(insertionMm, bounds.Min()), bounds.Max())
	}
	return insertionMm, nil
}

// parseBikeOptionsAndView reads bar_options, fork_options, and view.
func parseBikeOptionsAndView(q url.Values) (allBars, allForks bool, view string, err error) {
	switch raw := strings.TrimSpace(q.Get("bar_options")); raw {
	case "", BarOptionsSelected:
	case BarOptionsAll:
		allBars = true
	default:
		return false, false, "", &InputError{Field: "bar_options", Err: fmt.Errorf("unknown handlebar options value %q", raw)}
	}
	switch raw := strings.TrimSpace(q.Get("fork_options")); raw {
	case "", ForkOptionsSelected:
	case ForkOptionsAll:
		allForks = true
	default:
		return false, false, "", &InputError{Field: "fork_options", Err: fmt.Errorf("unknown fork options value %q", raw)}
	}
	switch raw := strings.TrimSpace(q.Get("view")); raw {
	case "", ViewBike:
		view = ViewBike
	case ViewSeat:
		view = ViewSeat
	case ViewStem:
		view = ViewStem
	default:
		return false, false, "", &InputError{Field: "view", Err: fmt.Errorf("unknown diagram view %q", raw)}
	}
	return allBars, allForks, view, nil
}

// IsBasisFork reports whether b selects the chart basis fork.
func (b Bike) IsBasisFork() bool {
	return b.ForkID == BasisForkID
}

// FramePart constructs b's validated bare assembly.Frame (published chart
// chainstay for the basis fork, max chainstay for physical forks).
func (b Bike) FramePart() (assembly.Frame, error) {
	if b.IsBasisFork() {
		return b.Frame.ChartFrame(b.Size)
	}
	return b.Frame.InstalledFrame(b.Size)
}

// LowerCup returns b's lower headset cup specification.
func (b Bike) LowerCup() assembly.LowerCupSetup {
	if b.IsBasisFork() {
		return b.Frame.BasisLowerCup
	}
	return assembly.LowerCupSetup{
		Standard: b.ForkOption.LowerCup.Standard,
		StackMm:  b.Params.LowerCupMm,
	}
}

// Wheels returns b's wheel setup.
func (b Bike) Wheels() assembly.WheelSetup {
	return assembly.WheelSetup{RadiusMm: b.Params.WheelRadiusMm}
}

// RearBounds returns the legal seatpost insertion and saddle rail offset ranges
// for b's selected frame, seatpost, and saddle without requiring a valid fork.
func (b Bike) RearBounds() (insertion, railOffset bounds.BoundedRange, err error) {
	frame, err := b.FramePart()
	if err != nil {
		return bounds.BoundedRange{}, bounds.BoundedRange{}, err
	}
	postDoF, err := assembly.MateSeatpost(frame.SeatTube(), b.Post.Part)
	if err != nil {
		return bounds.BoundedRange{}, bounds.BoundedRange{}, err
	}
	saddleDoF, err := assembly.MateSaddle(b.Post.Part.RailClamp(bounds.Dynamic), catalog.DefaultSaddle().Part)
	if err != nil {
		return bounds.BoundedRange{}, bounds.BoundedRange{}, err
	}
	return postDoF.AllowedInsertion(), saddleDoF.AllowedRailOffset(), nil
}

// Assemble constructs and validates b's complete clamped *assembly.Build.
func (b Bike) Assemble() (*assembly.Build, error) {
	frame, err := b.FramePart()
	if err != nil {
		return nil, err
	}
	var fs *assembly.Frameset
	if b.IsBasisFork() {
		fs, err = assembly.MateFrameset(frame, b.Frame.BasisLowerCup, assembly.DefaultUpperStackMm, b.Frame.BasisFork, b.Wheels())
	} else {
		fk, fkErr := b.ForkOption.PhysicalForkWithA2c(b.Params.ForkA2cMm, b.Params.SagPct)
		if fkErr != nil {
			return nil, fkErr
		}
		fs, err = assembly.MateFrameset(frame, b.LowerCup(), assembly.DefaultUpperStackMm, fk, b.Wheels())
	}
	if err != nil {
		return nil, err
	}
	rear, err := assembly.OpenRearCockpit(fs, b.Post.Part, catalog.DefaultSaddle().Part, b.Params.RailPitchDeg)
	if err != nil {
		return nil, err
	}
	seated, err := rear.Clamp(b.Params.PostInsertMm, b.Params.RailOffsetMm)
	if err != nil {
		return nil, err
	}
	stem, err := assembly.NewStem(catalog.DefaultStemClampHeightMm, b.Params.StemLengthMm, b.Params.StemAngleDeg)
	if err != nil {
		return nil, err
	}
	return seated.AttachFront(b.Params.SpacersMm, stem, b.Bar.Part, b.Params.BarRollDeg)
}

// FitTarget returns copies of the bike's BRP and HandGrip targets.
func (b Bike) FitTarget() assembly.FitTarget {
	var t assembly.FitTarget
	if b.BrpTarget != nil {
		p := *b.BrpTarget
		t.BRP = &p
	}
	if b.GripTarget != nil {
		p := *b.GripTarget
		t.HandGrip = &p
	}
	return t
}

// ForkOptions enumerates the frame's physical fork configurations offered to
// build fitting, listed forks first: a rigid fork once at 0% sag, a suspension
// fork at each Travel.SagPcts(), each with its own cup.
func (b Bike) ForkOptions() []BikeForkOption {
	var out []BikeForkOption
	for _, opt := range b.Frame.ForkOptions() {
		cup := opt.LowerCup
		if !opt.Fork.Travel.IsSuspension() {
			out = append(out, BikeForkOption{ForkOption: opt, LowerCup: cup})
			continue
		}
		for _, sag := range opt.Fork.Travel.SagPcts() {
			out = append(out, BikeForkOption{ForkOption: opt, SagPct: sag, LowerCup: cup})
		}
	}
	return out
}

// WithPlacement returns a copy of b whose numeric parameters match p.Setup.
func (b Bike) WithPlacement(p assembly.Placement) Bike {
	b.Params = paramsFromSetup(p.Setup)
	return b
}

// WithView returns a copy of b with View replaced.
func (b Bike) WithView(view string) Bike {
	b.View = view
	return b
}

// WithBasisFork returns a copy of b using the basis fork, lower cup, and
// published chainstay.
func (b Bike) WithBasisFork() Bike {
	b.ForkID = BasisForkID
	b.ForkOption = catalog.ForkOption{}
	b.Params.ForkA2cMm = b.Frame.BasisFork.BasisA2cMm()
	b.Params.LowerCupMm = b.Frame.BasisLowerCup.StackMm
	b.Params.SagPct = 0
	return b
}

// WithoutTargets returns a copy of b with both fit targets cleared.
func (b Bike) WithoutTargets() Bike {
	b.BrpTarget = nil
	b.GripTarget = nil
	return b
}

// queryValues encodes b into /bike query values, omitting prev_* keys.
func (b Bike) queryValues() url.Values {
	p := b.Params
	v := url.Values{}
	v.Set("frame", b.Frame.ID)
	v.Set("size", b.Size.Name)
	v.Set("fork", b.ForkID)
	v.Set("bar", b.Bar.ID)
	v.Set("post", b.Post.ID)
	v.Set("fork_a2c", FormatFloat(p.ForkA2cMm))
	v.Set("cup", FormatFloat(p.LowerCupMm))
	v.Set("sag", FormatFloat(p.SagPct))
	v.Set("spacers", FormatFloat(p.SpacersMm))
	v.Set("stem_length", FormatFloat(p.StemLengthMm))
	v.Set("stem_angle", FormatFloat(float64(p.StemAngleDeg)))
	v.Set("bar_roll", FormatFloat(float64(p.BarRollDeg)))
	v.Set("post_insert", FormatFloat(p.PostInsertMm))
	v.Set("rail_offset", FormatFloat(p.RailOffsetMm))
	v.Set("rail_pitch", FormatFloat(float64(p.RailPitchDeg)))
	v.Set("view", b.View)
	if b.BrpTarget != nil {
		v.Set("brp_x", FormatFloat(b.BrpTarget.X()))
		v.Set("brp_y", FormatFloat(b.BrpTarget.Y()))
	} else {
		v.Set("brp", "off")
	}
	if b.GripTarget != nil {
		v.Set("grip_x", FormatFloat(b.GripTarget.X()))
		v.Set("grip_y", FormatFloat(b.GripTarget.Y()))
	} else {
		v.Set("grip", "off")
	}
	if b.AllBarOptions {
		v.Set("bar_options", BarOptionsAll)
	}
	if b.AllForkOptions {
		v.Set("fork_options", ForkOptionsAll)
	}
	return v
}

// URL encodes the bike as a lossless /bike URL. It never emits prev_* keys.
func (b Bike) URL() string {
	return "/bike?" + b.queryValues().Encode()
}

// BasisFrameset places b's size on its published chart frame with the chart's
// basis fork and lower cup, for the basis readouts and ΔPitch.
func (b Bike) BasisFrameset() (assembly.FramesetPlacement, error) {
	chartFrame, err := b.Frame.ChartFrame(b.Size)
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	fs, err := assembly.MateFrameset(chartFrame, b.Frame.BasisLowerCup, assembly.DefaultUpperStackMm, b.Frame.BasisFork, b.Wheels())
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	return fs.Place(bounds.Dynamic), nil
}

// Evaluate assembles and places the bike and, when targets are set, scores the
// submitted build, fits it, and ranks builds.
func (b Bike) Evaluate(limit int) (BikeResult, error) {
	build, err := b.Assemble()
	if err != nil {
		return BikeResult{}, err
	}
	placement := build.Place(bounds.Dynamic)
	res := BikeResult{
		FrameID:        b.Frame.ID,
		Size:           b.Size.Name,
		ForkID:         b.ForkID,
		BarID:          b.Bar.ID,
		PostID:         b.Post.ID,
		AllBarOptions:  b.AllBarOptions,
		AllForkOptions: b.AllForkOptions,
		Placement:      placement,
	}
	if b.BrpTarget == nil && b.GripTarget == nil {
		return res, nil
	}
	target := b.FitTarget()
	weights := catalog.DefaultScoreWeights()

	if target.HandGrip != nil {
		if stem, ok := catalog.StemByDimensions(b.Params.StemLengthMm, b.Params.StemAngleDeg); ok {
			sc, err := assembly.ScoreBuild(placement, target, stem.Score, weights)
			if err != nil {
				return BikeResult{}, err
			}
			res.Score = &sc
		}
	}

	// Continuous rear + front cockpit fitting on the submitted frameset.
	var (
		fittedSeated *assembly.SeatedFrameset
		limitedBy    []bounds.InputField
	)
	if target.BRP != nil {
		rear, err := assembly.OpenRearCockpit(build.SeatedFrameset().Frameset(), b.Post.Part, catalog.DefaultSaddle().Part, b.Params.RailPitchDeg)
		if err != nil {
			return BikeResult{}, err
		}
		var rearLimited []bounds.InputField
		fittedSeated, rearLimited, err = rear.FitToBRP(*target.BRP)
		if err != nil {
			return BikeResult{}, err
		}
		limitedBy = append(limitedBy, rearLimited...)
	} else {
		fittedSeated = build.SeatedFrameset()
	}

	var fittedBuild *assembly.Build
	if target.HandGrip != nil {
		front, err := assembly.OpenFrontCockpit(
			fittedSeated,
			b.Params.StemAngleDeg,
			catalog.DefaultStemClampHeightMm,
			catalog.DefaultStemLengthRange(),
			b.Bar.Part,
			b.Params.BarRollDeg,
		)
		if err != nil {
			return BikeResult{}, err
		}
		var frontLimited []bounds.InputField
		fittedBuild, frontLimited, err = front.FitToGrip(*target.HandGrip)
		if err != nil {
			return BikeResult{}, err
		}
		limitedBy = append(limitedBy, frontLimited...)
	} else {
		stem, err := assembly.NewStem(catalog.DefaultStemClampHeightMm, b.Params.StemLengthMm, b.Params.StemAngleDeg)
		if err != nil {
			return BikeResult{}, err
		}
		fittedBuild, err = fittedSeated.AttachFront(b.Params.SpacersMm, stem, b.Bar.Part, b.Params.BarRollDeg)
		if err != nil {
			return BikeResult{}, err
		}
	}

	fit := assembly.Fitting{
		Target:    target,
		Placement: fittedBuild.Place(bounds.Dynamic),
		Outcome:   assembly.FitReached,
	}
	if len(limitedBy) > 0 {
		fit.Outcome = assembly.FitLimited
		fit.LimitedBy = limitedBy
	}
	res.Fitting = &fit

	if target.BRP != nil {
		centered, err := build.FitPostSetback(*target.BRP, 0)
		if err != nil {
			return BikeResult{}, err
		}
		res.SetbackCentered = &centered
		if slices.Contains(fit.LimitedBy, bounds.InputSeatpostInsertion) || slices.Contains(fit.LimitedBy, bounds.InputSaddleRailOffset) {
			reach, err := build.FitPostSetback(*target.BRP, fit.Placement.Setup.Saddle.RailOffsetMm)
			if err != nil {
				return BikeResult{}, err
			}
			res.SetbackReach = &reach
		}
	}

	if target.HandGrip != nil {
		var offeredBars []catalog.Handlebar
		bars := []assembly.Handlebar{b.Bar.Part}
		if b.AllBarOptions {
			offeredBars = catalog.Handlebars()
			bars = make([]assembly.Handlebar, len(offeredBars))
			for i, bar := range offeredBars {
				bars[i] = bar.Part
			}
		}
		frontOpts := assembly.FrontBuildOptions{
			StemClampHeightMm: catalog.DefaultStemClampHeightMm,
			Stems:             catalog.StemOptions(),
			SpacerStepMm:      catalog.SpacerStepMm,
			Handlebars:        bars,
			HasHandlebarOpts:  b.AllBarOptions,
			DynamicBarRollDeg: b.Params.BarRollDeg,
		}

		var (
			offeredForks []BikeForkOption
			ranking      assembly.BuildRanking
		)
		if !b.AllForkOptions {
			ranking, err = assembly.FitFrontBuilds(fittedSeated, target, frontOpts, weights, limit)
			if err != nil {
				return BikeResult{}, err
			}
		} else {
			offeredForks = b.ForkOptions()
			// Installed physical forks use the sliding-dropout max chainstay
			// even when the submitted bike is on the basis fork.
			installedFrame, err := b.Frame.InstalledFrame(b.Size)
			if err != nil {
				return BikeResult{}, err
			}
			ranking, err = b.fitAcrossForks(installedFrame, offeredForks, target, frontOpts, weights, limit)
			if err != nil {
				return BikeResult{}, err
			}
		}

		rows := make([]BikeScoredBuild, len(ranking.Builds))
		for i, c := range ranking.Builds {
			row := BikeScoredBuild{ScoredBuild: c, HandlebarID: b.Bar.ID, ForkID: b.ForkID}
			if c.HasHandlebarOption {
				if c.HandlebarIndex < 0 || c.HandlebarIndex >= len(offeredBars) {
					return BikeResult{}, fmt.Errorf("handlebar option index %d out of range [0, %d)", c.HandlebarIndex, len(offeredBars))
				}
				row.HandlebarID = offeredBars[c.HandlebarIndex].ID
			}
			if c.HasForkOption {
				if c.ForkIndex < 0 || c.ForkIndex >= len(offeredForks) {
					return BikeResult{}, fmt.Errorf("fork option index %d out of range [0, %d)", c.ForkIndex, len(offeredForks))
				}
				row.ForkID = offeredForks[c.ForkIndex].ForkOption.ID()
			}
			rows[i] = row
		}
		res.Builds = &BikeBuildRanking{Evaluated: ranking.Evaluated, Builds: rows}
	}
	return res, nil
}

func (b Bike) fitAcrossForks(
	installedFrame assembly.Frame,
	offeredForks []BikeForkOption,
	target assembly.FitTarget,
	frontOpts assembly.FrontBuildOptions,
	weights assembly.ScoreWeights,
	limit int,
) (assembly.BuildRanking, error) {
	if limit < 1 {
		return assembly.BuildRanking{}, fmt.Errorf("build fitting needs a positive build limit, got %d", limit)
	}
	best := make([]assembly.ScoredBuild, 0, min(limit, 16))
	evaluated := 0
	for fi, o := range offeredForks {
		fk, err := o.ForkOption.PhysicalFork(o.SagPct)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		fs, err := assembly.MateFrameset(installedFrame, o.LowerCup, assembly.DefaultUpperStackMm, fk, b.Wheels())
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		rear, err := assembly.OpenRearCockpit(fs, b.Post.Part, catalog.DefaultSaddle().Part, b.Params.RailPitchDeg)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		var seated *assembly.SeatedFrameset
		if target.BRP != nil {
			seated, _, err = rear.FitToBRP(*target.BRP)
		} else {
			seated, err = rear.Clamp(b.Params.PostInsertMm, b.Params.RailOffsetMm)
		}
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		sub, err := assembly.FitFrontBuilds(seated, target, frontOpts, weights, limit)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		evaluated += sub.Evaluated
		for _, cand := range sub.Builds {
			cand.HasForkOption = true
			cand.ForkIndex = fi
			best = mergeBestBuild(best, cand, limit)
		}
	}
	return assembly.BuildRanking{Builds: best, Evaluated: evaluated}, nil
}

func mergeBestBuild(best []assembly.ScoredBuild, build assembly.ScoredBuild, limit int) []assembly.ScoredBuild {
	if len(best) == limit && assembly.CompareScoredBuilds(build, best[len(best)-1]) >= 0 {
		return best
	}
	i := sort.Search(len(best), func(i int) bool { return assembly.CompareScoredBuilds(build, best[i]) < 0 })
	if len(best) < limit {
		best = append(best, assembly.ScoredBuild{})
	}
	copy(best[i+1:], best[i:len(best)-1])
	best[i] = build
	return best
}
