package query

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/pose"
)

// BasisForkID is the reserved fork query value selecting the frame's basis
// fork and basis lower cup (Q81, Q89).
const BasisForkID = "basis"

// fork_options values (Q33, Q101).
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

// RankedBuildLimit is the default number of ranked builds on /bike and CLI (Q18).
const RankedBuildLimit = 5

// BikeForkOption is one physical fork configuration a frame offers to build
// fitting when fork_options=all (Q31).
type BikeForkOption struct {
	ForkOption catalog.ForkOption
	SagPct     float64
	LowerCup   assembly.LowerCupSetup
}

// Bike holds the parsed, catalog-resolved /bike query and ready-to-place
// BikeSetup (Q88).
type Bike struct {
	Frame          catalog.Frame
	Size           catalog.FrameSize
	ForkID         string             // concrete Q131 variant ID or BasisForkID
	ForkOption     catalog.ForkOption // zero when ForkID == BasisForkID
	Bar            catalog.Handlebar
	Post           catalog.Seatpost
	View           string // ViewBike, ViewSeat, or ViewStem
	BrpTarget      *pose.Point
	GripTarget     *pose.Point
	AllBarOptions  bool
	AllForkOptions bool
	Setup          assembly.BikeSetup
}

// BikeScoredBuild pairs a ranked assembly.ScoredBuild with the build's catalog
// handlebar and fork IDs (Q88, Q106). Embedding ScoredBuild flattens its JSON
// fields.
type BikeScoredBuild struct {
	assembly.ScoredBuild
	HandlebarID string
	ForkID      string
}

// BikeBuildRanking is the catalog-resolved ranked builds list for /bike and
// bikegeo-reboot bike JSON output.
type BikeBuildRanking struct {
	Evaluated int
	Builds    []BikeScoredBuild
}

// BikeResult is a catalog-resolved single-bike evaluation for /bike and
// bikegeo-reboot bike JSON output (Q49a, Q88, Q106, Q110).
type BikeResult struct {
	FrameID         string
	Size            string
	ForkID          string
	BarID           string
	PostID          string
	AllBarOptions   bool
	AllForkOptions  bool
	Placement       assembly.BikePlacement
	Score           *assembly.BuildScore // non-nil when HandGrip target is set and stem is in catalog (Q110)
	Fitting         *assembly.Fitting    // non-nil when BRP or HandGrip target is set
	SetbackCentered *assembly.Fitting    // non-nil when BRP target is set (Q29)
	SetbackReach    *assembly.Fitting    // non-nil when BRP target is set and saddle fit is limited (Q29)
	Builds          *BikeBuildRanking    // non-nil when HandGrip target is set
}

// knownBikeKeys are every query key /bike accepts (Q111); all are single-valued.
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

// FormatFloat formats v losslessly for URLs (Q77), normalizing -0 to "0".
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

// findForkOption returns the frame's single concrete ForkOption with the given Q131 ID.
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

// basisForkSetup returns the chart basis fork for frame (Q81, Q120).
func basisForkSetup(frame catalog.Frame) assembly.ForkSetup {
	return assembly.ForkSetup{Spec: frame.BasisFork}
}

// ParseBike parses a /bike query into a catalog-resolved Bike, applying the
// catalog defaults and manual-switch rules (Q53, Q70, Q109, Q111). Every
// rejection is an *InputError naming its query key.
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
	frameSetup, lowerCup, forkSetup, err := parseBikeFork(q, &b, frameSwitched)
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

	insertionMm, err := parseBikePostInsertion(q, frameSetup, post, frameSwitched || sizeSwitched || postSwitched)
	if err != nil {
		return Bike{}, err
	}

	defaultStem := catalog.DefaultStemSetup()
	spacers, err := parseFloatOrDefault(q, "spacers", defaultStem.SpacersMm, "spacers")
	if err != nil {
		return Bike{}, err
	}
	stemLength, err := parseFloatOrDefault(q, "stem_length", defaultStem.LengthMm, "stem length")
	if err != nil {
		return Bike{}, err
	}
	stemAngle, err := parseFloatOrDefault(q, "stem_angle", float64(defaultStem.AngleDeg), "stem angle")
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

	stem := defaultStem
	stem.SpacersMm = spacers
	stem.LengthMm = stemLength
	stem.AngleDeg = pose.Deg(stemAngle)
	saddle := catalog.DefaultSaddle().Setup
	saddle.RailOffsetMm = railOffset
	saddle.RailPitchDeg = pose.Deg(railPitch)
	barSetup := bar.Setup
	barSetup.BarRollDeg = pose.Deg(barRoll)

	b.Bar = bar
	b.Post = post
	b.View = view
	b.BrpTarget = brp
	b.GripTarget = grip
	b.AllBarOptions = allBars
	b.AllForkOptions = allForks
	b.Setup = assembly.BikeSetup{
		Frame:     frameSetup,
		LowerCup:  lowerCup,
		Fork:      forkSetup,
		Wheels:    assembly.WheelSetup{RadiusMm: catalog.WheelRadiusMm},
		Seatpost:  assembly.SeatpostSetup{Spec: post.Spec, InsertionMm: insertionMm},
		Saddle:    saddle,
		Stem:      stem,
		Handlebar: barSetup,
	}
	return b, nil
}

// parseBikeFrameAndSize resolves frame and size, honoring Q109 manual-switch
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

// parseBikeFork resolves the fork, installed frame, lower cup, and fork setup,
// recording ForkID and ForkOption on b.
func parseBikeFork(q url.Values, b *Bike, frameSwitched bool) (assembly.FrameSetup, assembly.LowerCupSetup, assembly.ForkSetup, error) {
	frame, size := b.Frame, b.Size
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
			return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{},
				&InputError{Field: "fork", Err: fmt.Errorf("fork %q has multiple variants; select a specific variant such as %q", rawFork, matches[0].ID())}
		}
	case frameSwitched:
		opt = frame.ForkOptions()[0]
		forkID = opt.ID()
	default:
		return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{},
			&InputError{Field: "fork", Err: fmt.Errorf("frame %q has no fork %q", frame.ID, rawFork)}
	}
	forkSwitched := !frameSwitched && prevFork != "" && prevFork != forkID

	b.ForkID = forkID
	if isBasis {
		b.ForkOption = catalog.ForkOption{}
		return size.ChartFrame, frame.BasisLowerCup, basisForkSetup(frame), nil
	}
	b.ForkOption = opt

	frameSetup := frame.InstalledFrame(size)
	defaultSag := opt.Fork.Travel.DefaultSagPct()
	defaultCup := opt.LowerCup.StackMm

	var a2cMm, cupMm, sagPct float64
	var err error
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
			return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{}, err
		}
	default:
		if a2cMm, err = parseFloatOrDefault(q, "fork_a2c", opt.Variant.StaticA2cMm, "fork static A2C"); err != nil {
			return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{}, err
		}
		if cupMm, err = parseFloatOrDefault(q, "cup", defaultCup, "lower cup"); err != nil {
			return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{}, err
		}
		if sagPct, err = parseFloatOrDefault(q, "sag", defaultSag, "fork sag"); err != nil {
			return assembly.FrameSetup{}, assembly.LowerCupSetup{}, assembly.ForkSetup{}, err
		}
	}
	lowerCup := assembly.LowerCupSetup{Standard: opt.LowerCup.Standard, StackMm: cupMm}
	spec := opt.Spec()
	spec.StaticA2cMm = a2cMm
	return frameSetup, lowerCup, assembly.ForkSetup{Spec: spec, SagPct: sagPct}, nil
}

// parseBikePostInsertion reads post_insert, defaulting to the midpoint of the
// frame/post bounds and clamping it into them after a manual switch (Q109).
func parseBikePostInsertion(q url.Values, frameSetup assembly.FrameSetup, post catalog.Seatpost, clamp bool) (float64, error) {
	bounds, err := assembly.SeatpostInsertionBounds(frameSetup, post.Spec)
	if err != nil {
		return 0, &InputError{Field: "post", Err: err}
	}
	defaultInsert := (bounds.EffectiveMinInsertionMm + bounds.MaxAllowedInsertionMm) / 2.0
	insertionMm, err := parseFloatOrDefault(q, "post_insert", defaultInsert, "seatpost insertion")
	if err != nil {
		return 0, err
	}
	if clamp {
		insertionMm = min(max(insertionMm, bounds.EffectiveMinInsertionMm), bounds.MaxAllowedInsertionMm)
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
// fork at each Travel.SagPcts() (Q31, Q129), each with its own cup (Q136).
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

// WithSetup returns a copy of b with Setup replaced.
func (b Bike) WithSetup(setup assembly.BikeSetup) Bike {
	b.Setup = setup
	return b
}

// WithView returns a copy of b with View replaced.
func (b Bike) WithView(view string) Bike {
	b.View = view
	return b
}

// WithBasisFork returns a copy of b using the basis fork, lower
// cup, and published chainstay (Q81).
func (b Bike) WithBasisFork() Bike {
	b.ForkID = BasisForkID
	b.ForkOption = catalog.ForkOption{}
	b.Setup.Frame = b.Size.ChartFrame
	b.Setup.LowerCup = b.Frame.BasisLowerCup
	b.Setup.Fork = basisForkSetup(b.Frame)
	return b
}

// WithoutTargets returns a copy of b with both fit targets cleared (Q70).
func (b Bike) WithoutTargets() Bike {
	b.BrpTarget = nil
	b.GripTarget = nil
	return b
}

// queryValues encodes b into /bike query values (Q70, Q77), omitting prev_* keys.
func (b Bike) queryValues() url.Values {
	s := b.Setup
	v := url.Values{}
	v.Set("frame", b.Frame.ID)
	v.Set("size", b.Size.Name)
	v.Set("fork", b.ForkID)
	v.Set("bar", b.Bar.ID)
	v.Set("post", b.Post.ID)
	v.Set("fork_a2c", FormatFloat(s.Fork.Spec.StaticA2cMm))
	v.Set("cup", FormatFloat(s.LowerCup.StackMm))
	v.Set("sag", FormatFloat(s.Fork.SagPct))
	v.Set("spacers", FormatFloat(s.Stem.SpacersMm))
	v.Set("stem_length", FormatFloat(s.Stem.LengthMm))
	v.Set("stem_angle", FormatFloat(float64(s.Stem.AngleDeg)))
	v.Set("bar_roll", FormatFloat(float64(s.Handlebar.BarRollDeg)))
	v.Set("post_insert", FormatFloat(s.Seatpost.InsertionMm))
	v.Set("rail_offset", FormatFloat(s.Saddle.RailOffsetMm))
	v.Set("rail_pitch", FormatFloat(float64(s.Saddle.RailPitchDeg)))
	v.Set("view", b.View)
	if b.BrpTarget != nil {
		v.Set("brp_x", FormatFloat(b.BrpTarget.X))
		v.Set("brp_y", FormatFloat(b.BrpTarget.Y))
	} else {
		v.Set("brp", "off")
	}
	if b.GripTarget != nil {
		v.Set("grip_x", FormatFloat(b.GripTarget.X))
		v.Set("grip_y", FormatFloat(b.GripTarget.Y))
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

// URL encodes the bike as a lossless /bike URL (Q70, Q77). It never emits
// prev_* keys.
func (b Bike) URL() string {
	return "/bike?" + b.queryValues().Encode()
}

// BasisFrameset places b's size on its published chart frame with the chart's
// basis fork and lower cup (Q81, Q121), for the basis readouts and ΔPitch.
func (b Bike) BasisFrameset() (assembly.FramesetPlacement, error) {
	return assembly.PlaceFrameset(b.Size.ChartFrame, b.Frame.BasisLowerCup, basisForkSetup(b.Frame), b.Setup.Wheels)
}

// Evaluate places the bike and, when targets are set, scores the submitted
// build (Q110), fits it, and ranks builds (Q88, Q106).
func (b Bike) Evaluate(limit int) (BikeResult, error) {
	placement, err := assembly.PlaceBike(b.Setup)
	if err != nil {
		return BikeResult{}, err
	}
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

	if target.HandGrip != nil {
		if stem, ok := catalog.StemByDimensions(b.Setup.Stem.LengthMm, b.Setup.Stem.AngleDeg); ok {
			sc, err := assembly.ScoreBuild(placement, target, stem.Score, catalog.DefaultScoreWeights())
			if err != nil {
				return BikeResult{}, err
			}
			res.Score = &sc
		}
	}

	fit, err := assembly.ApplyFitting(b.Setup, target)
	if err != nil {
		return BikeResult{}, err
	}
	res.Fitting = &fit

	if target.BRP != nil {
		centered, err := assembly.FitPostSetback(b.Setup, *target.BRP, 0)
		if err != nil {
			return BikeResult{}, err
		}
		res.SetbackCentered = &centered
		if slices.Contains(fit.LimitedBy, assembly.InputSeatpostInsertion) || slices.Contains(fit.LimitedBy, assembly.InputSaddleRailOffset) {
			reach, err := assembly.FitPostSetback(b.Setup, *target.BRP, fit.Placement.Setup.Saddle.RailOffsetMm)
			if err != nil {
				return BikeResult{}, err
			}
			res.SetbackReach = &reach
		}
	}

	if target.HandGrip != nil {
		parts := assembly.PartOptions{
			Stems:        catalog.StemOptions(),
			SpacerStepMm: catalog.SpacerStepMm,
		}
		var offeredBars []catalog.Handlebar
		if b.AllBarOptions {
			offeredBars = catalog.Handlebars()
			parts.Handlebars = make([]assembly.HandlebarSetup, len(offeredBars))
			for i, bar := range offeredBars {
				parts.Handlebars[i] = bar.Setup // BarRollDeg 0: FitBuilds installs Setup.Handlebar.BarRollDeg (Q24)
			}
		}
		var offeredForks []BikeForkOption
		fitSetup := b.Setup
		if b.AllForkOptions {
			offeredForks = b.ForkOptions()
			parts.Forks = make([]assembly.ForkOption, len(offeredForks))
			for i, o := range offeredForks {
				parts.Forks[i] = assembly.ForkOption{
					Fork:     assembly.ForkSetup{Spec: o.ForkOption.Spec(), SagPct: o.SagPct},
					LowerCup: o.LowerCup,
				}
			}
			// Installed physical forks use the sliding-dropout max chainstay even
			// when the submitted bike is on the basis fork (Q67).
			fitSetup.Frame = b.Frame.InstalledFrame(b.Size)
		}
		ranking, err := assembly.FitBuilds(fitSetup, target, parts, catalog.DefaultScoreWeights(), limit)
		if err != nil {
			return BikeResult{}, err
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
