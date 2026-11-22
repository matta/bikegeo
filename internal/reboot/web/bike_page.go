package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
	"bikegeo/internal/reboot/query"
)

// frameSelectOption is one entry of the frame <select>.
type frameSelectOption struct {
	ID    string
	Label string
}

// forkOption is one fork <option> for the selected frame.
type forkOption struct {
	ID       string
	Name     string
	Label    string
	TravelMm float64
}

const cadForkName = "CAD Baseline Fork"

func makeForkOption(ff catalog.FrameFork) forkOption {
	var kind string
	if ff.Fork.TravelMm == 0 {
		kind = "Rigid"
	} else {
		kind = fmt.Sprintf("%.0f mm travel", ff.Fork.TravelMm)
	}
	return forkOption{
		ID:       ff.Fork.ID,
		Name:     ff.Fork.Name,
		Label:    fmt.Sprintf("%s (%s, %.0f mm A2C / %.0f mm rake)", ff.Fork.Name, kind, ff.StaticA2cMm, ff.RakeMm),
		TravelMm: ff.Fork.TravelMm,
	}
}

func makeBasisForkOption(frame catalog.Frame) forkOption {
	return forkOption{
		ID:   query.CadForkID,
		Name: cadForkName,
		Label: fmt.Sprintf("%s (%.0f mm A2C / %.0f mm rake, %.0f mm cup)",
			cadForkName, frame.BasisFork.StaticA2cMm, frame.BasisFork.RakeMm, frame.BasisLowerCup.StackMm),
	}
}

// forkCellName names a fork configuration in a ranked build row: the fork
// name, plus ", N% sag" for a suspension fork.
func forkCellName(name string, travelMm, sagPct float64) string {
	if travelMm == 0 {
		return name
	}
	return fmt.Sprintf("%s, %.0f%% sag", name, sagPct)
}

// forkNameAndTravel resolves a build's fork ID on b's frame.
func forkNameAndTravel(b query.Bike, id string) (string, float64) {
	if id == query.CadForkID {
		return cadForkName, 0
	}
	for _, ff := range b.Frame.Forks {
		if ff.Fork.ID == id {
			return ff.Fork.Name, ff.Fork.TravelMm
		}
	}
	return id, 0
}

type bikeFormValues struct {
	View        string
	Frame       string
	Size        string
	Fork        string
	Bar         string
	Post        string
	PrevFrame   string
	PrevSize    string
	PrevFork    string
	PrevPost    string
	ForkA2c     string
	Cup         string
	Sag         string
	Spacers     string
	StemLength  string
	StemAngle   string
	BarRoll     string
	PostInsert  string
	RailOffset  string
	RailPitch   string
	BrpX        string
	BrpY        string
	GripX       string
	GripY       string
	BarOptions  string
	ForkOptions string
}

// selectedIDs names the option each <select> marks selected; an empty ID
// means the submitted value was rejected and no option is selected.
type selectedIDs struct {
	Frame, Size, Fork, Bar, Post string
}

// formFromBike returns the form values describing a parsed bike.
func formFromBike(b query.Bike) bikeFormValues {
	s := b.Setup
	form := bikeFormValues{
		View:        b.View,
		Frame:       b.Frame.ID,
		Size:        b.Size.Name,
		Fork:        b.ForkID,
		Bar:         b.Bar.ID,
		Post:        b.Post.ID,
		PrevFrame:   b.Frame.ID,
		PrevSize:    b.Size.Name,
		PrevFork:    b.ForkID,
		PrevPost:    b.Post.ID,
		ForkA2c:     formatFormFloat(s.Fork.Spec.StaticA2cMm),
		Cup:         formatFormFloat(s.LowerCup.StackMm),
		Sag:         formatFormFloat(s.Fork.SagPct),
		Spacers:     formatFormFloat(s.Stem.SpacersMm),
		StemLength:  formatFormFloat(s.Stem.LengthMm),
		StemAngle:   formatFormFloat(float64(s.Stem.AngleDeg)),
		BarRoll:     formatFormFloat(float64(s.Handlebar.BarRollDeg)),
		PostInsert:  formatFormFloat(s.Seatpost.InsertionMm),
		RailOffset:  formatFormFloat(s.Saddle.RailOffsetMm),
		RailPitch:   formatFormFloat(float64(s.Saddle.RailPitchDeg)),
		BarOptions:  query.BarOptionsSelected,
		ForkOptions: query.ForkOptionsSelected,
	}
	if b.BrpTarget != nil {
		form.BrpX, form.BrpY = formatFormFloat(b.BrpTarget.X), formatFormFloat(b.BrpTarget.Y)
	}
	if b.GripTarget != nil {
		form.GripX, form.GripY = formatFormFloat(b.GripTarget.X), formatFormFloat(b.GripTarget.Y)
	}
	if b.AllBarOptions {
		form.BarOptions = query.BarOptionsAll
	}
	if b.AllForkOptions {
		form.ForkOptions = query.ForkOptionsAll
	}
	return form
}

// echoForm returns the form values of an error page: the exact text submitted
// in q where there is any, otherwise the values of base. A locked CAD baseline
// fork echoes its chart values.
func echoForm(q url.Values, base query.Bike, sel selectedIDs) bikeFormValues {
	form := formFromBike(base)
	raw := func(key string) string { return strings.TrimSpace(q.Get(key)) }
	override := func(dst *string, key string) {
		if v := raw(key); v != "" {
			*dst = v
		}
	}
	// The prev_* values stay base's resolved selections: they match the
	// dependent size and fork options the page renders, so choosing another
	// frame after a rejected one is seen as a switch and recovers (Q109).
	override(&form.View, "view")
	if base.ForkID != query.CadForkID || sel.Fork == "" {
		override(&form.ForkA2c, "fork_a2c")
		override(&form.Cup, "cup")
		override(&form.Sag, "sag")
	}
	override(&form.Spacers, "spacers")
	override(&form.StemLength, "stem_length")
	override(&form.StemAngle, "stem_angle")
	override(&form.BarRoll, "bar_roll")
	override(&form.PostInsert, "post_insert")
	override(&form.RailOffset, "rail_offset")
	override(&form.RailPitch, "rail_pitch")
	override(&form.BarOptions, "bar_options")
	override(&form.ForkOptions, "fork_options")
	// A target pair echoes what was typed whenever either box was submitted, so
	// a half-entered or cleared target shows exactly that.
	if q.Has("brp_x") || q.Has("brp_y") {
		form.BrpX, form.BrpY = raw("brp_x"), raw("brp_y")
	}
	if q.Has("grip_x") || q.Has("grip_y") {
		form.GripX, form.GripY = raw("grip_x"), raw("grip_y")
	}
	return form
}

// controlsView fills the parts of bikeView the controls form needs: the
// selects, bounds, notes, and form values. It is shared by the build page and
// the error pages.
func controlsView(b query.Bike, form bikeFormValues, sel selectedIDs) bikeView {
	frames := catalog.Frames()
	frameOptions := make([]frameSelectOption, len(frames))
	for i, f := range frames {
		frameOptions[i] = frameSelectOption{ID: f.ID, Label: fmt.Sprintf("%s %s (%d)", f.Manufacturer, f.Model, f.Year)}
	}
	sizes := make([]string, len(b.Frame.Sizes))
	for i, sz := range b.Frame.Sizes {
		sizes[i] = sz.Name
	}
	forks := make([]forkOption, len(b.Frame.Forks))
	for i, ff := range b.Frame.Forks {
		forks[i] = makeForkOption(ff)
	}
	forkA2cRO, cupRO, sagRO, cupLabel := forkFieldViewState(b, sel.Fork, form.Sag)
	minRail, maxRail := railOffsetRange(b.Setup.Saddle, b.Setup.Seatpost)
	postBounds := postInsertionBounds(b.Setup.Frame, b.Setup.Seatpost.Spec)
	brp, grip := catalog.DefaultBrpTarget(), catalog.DefaultGripTarget()
	v := bikeView{
		Form:                 form,
		MinRailOffsetMm:      minRail,
		MaxRailOffsetMm:      maxRail,
		ExampleBrpX:          formatFormFloat(brp.X),
		ExampleBrpY:          formatFormFloat(brp.Y),
		ExampleGripX:         formatFormFloat(grip.X),
		ExampleGripY:         formatFormFloat(grip.Y),
		MaxSpacersMm:         b.Setup.Stem.MaxSpacersMm,
		MinPostInsertionMm:   postBounds.EffectiveMinInsertionMm,
		MaxPostInsertionMm:   postBounds.MaxAllowedInsertionMm,
		PostTotalLengthMm:    b.Setup.Seatpost.Spec.TotalLengthMm,
		PostSetbackMm:        b.Setup.Seatpost.Spec.SetbackMm,
		PostClampLengthMm:    b.Setup.Seatpost.Spec.RailClampLengthMm,
		BikeName:             fmt.Sprintf("%s %s (%d, Size %s)", b.Frame.Manufacturer, b.Frame.Model, b.Frame.Year, b.Size.Name),
		Frames:               frameOptions,
		SelectedFrameID:      sel.Frame,
		Sizes:                sizes,
		SelectedSize:         sel.Size,
		Forks:                forks,
		ForkOptionCount:      len(b.ForkOptions()),
		CadForkOption:        makeBasisForkOption(b.Frame),
		SelectedForkID:       sel.Fork,
		Handlebars:           catalog.Handlebars(),
		SelectedBarID:        sel.Bar,
		Seatposts:            catalog.Seatposts(),
		SelectedPostID:       sel.Post,
		CupLabel:             cupLabel,
		ForkA2cReadOnly:      forkA2cRO,
		CupReadOnly:          cupRO,
		SagReadOnly:          sagRO,
		CadBaselineNote:      cadBaselineForkNote(b, sel.Fork),
		SlidingDropoutsNote:  slidingDropoutsNote(b),
		BasisFork:            b.Frame.BasisFork,
		CadLowerCupMm:        b.Frame.BasisLowerCup.StackMm,
		PublishedWheelbaseMm: b.Size.PublishedWheelbaseMm,
	}
	if sel.Post == catalog.DefaultSeatpostID {
		v.RedshiftMixedStateNote = redshiftMixedStateNote
	}
	return v
}

// forkFieldViewState says which fork inputs are read-only and how the lower
// cup is labelled. selectedForkID is empty when the submitted fork was rejected.
func forkFieldViewState(b query.Bike, selectedForkID, sagValue string) (forkA2cRO, cupRO, sagRO bool, cupLabel string) {
	isCad := selectedForkID == query.CadForkID
	forkA2cRO = isCad
	cupRO = isCad
	// On a known rigid fork, keep sag read-only only when the displayed value is
	// already valid zero; if an invalid or non-zero sag was submitted, leave the
	// input editable so the user can reset it to 0 without losing other settings.
	rigid := !isCad && b.Setup.Fork.Spec.TravelMm == 0
	sagRO = isCad || (selectedForkID != "" && rigid && isZeroFormFloat(sagValue))
	cupLabel = lowerCupLabel(b.Setup.LowerCup.Standard)
	return forkA2cRO, cupRO, sagRO, cupLabel
}

func cadBaselineForkNote(b query.Bike, selectedForkID string) string {
	if selectedForkID != query.CadForkID {
		return ""
	}
	cadCup := b.Frame.BasisLowerCup
	physCup := b.Frame.InstalledFrame(b.Size).DefaultLowerCup
	if cadCup.Standard == assembly.LowerCupBareHeadTube {
		return fmt.Sprintf(
			"CAD Baseline Fork: the published frame chart omits the lower headset cup (%s, %s mm), and these chart fields are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.",
			lowerCupStandardName(cadCup.Standard), query.FormatFloat(cadCup.StackMm),
			lowerCupStandardName(physCup.Standard), query.FormatFloat(physCup.StackMm),
		)
	}
	return fmt.Sprintf(
		"CAD Baseline Fork: chart A2C, lower cup (%s, %s mm), and 0%% sag are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.",
		lowerCupStandardName(cadCup.Standard), query.FormatFloat(cadCup.StackMm),
		lowerCupStandardName(physCup.Standard), query.FormatFloat(physCup.StackMm),
	)
}

// slidingDropoutsNote explains which chainstay a sliding-dropout frame is
// evaluated at (Q57, Q67); empty for fixed dropouts.
func slidingDropoutsNote(b query.Bike) string {
	maxCs := b.Frame.MaxChainstayMm
	if maxCs <= 0 {
		return ""
	}
	published := b.Size.ChartFrame.ChainstayMm
	switch {
	case b.ForkID == query.CadForkID && maxCs != published:
		return fmt.Sprintf("CAD Baseline Fork uses published chainstay (%.0f mm); installed physical forks evaluate sliding dropouts at max chainstay (%.0f mm).", published, maxCs)
	case maxCs != published:
		return fmt.Sprintf("Sliding dropouts evaluated at max chainstay (%.0f mm; published chart %.0f mm).", maxCs, published)
	default:
		return fmt.Sprintf("Sliding dropouts evaluated at max chainstay (%.0f mm, matching published chart).", maxCs)
	}
}

func handleBikeWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bike" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()

	parsed, err := query.ParseBike(q)
	if err != nil {
		field := ""
		if inputErr, ok := errors.AsType[*query.InputError](err); ok {
			field = inputErr.Field
		}
		renderBikeError(w, q, sanitizedBike(q), field, err.Error())
		return
	}
	result, err := parsed.Evaluate(query.RankedBuildLimit)
	if err != nil {
		renderBikeError(w, q, parsed, bikeInputField(err), err.Error())
		return
	}
	basis, err := parsed.BasisFrameset()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildBikeView(parsed, result.Placement, basis)
	if result.Fitting != nil {
		view.Fitting, err = buildFittingView(parsed, result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	renderBikeTemplate(w, http.StatusOK, view)
}

// sanitizedBike returns a valid bike resolved from q with every rejected
// parameter dropped, so an error page can still show the frame, size, fork,
// and bounds the form needs.
func sanitizedBike(q url.Values) query.Bike {
	clean := make(url.Values, len(q))
	for k, v := range q {
		clean[k] = slices.Clone(v)
	}
	for range 64 {
		b, err := query.ParseBike(clean)
		if err == nil {
			return b
		}
		var inputErr *query.InputError
		if !errors.As(err, &inputErr) {
			break
		}
		switch inputErr.Field {
		case "brp", "brp_x", "brp_y":
			delete(clean, "brp")
			delete(clean, "brp_x")
			delete(clean, "brp_y")
		case "grip", "grip_x", "grip_y":
			delete(clean, "grip")
			delete(clean, "grip_x")
			delete(clean, "grip_y")
		default:
			if _, ok := clean[inputErr.Field]; !ok {
				return query.DefaultBike()
			}
			delete(clean, inputErr.Field)
		}
	}
	return query.DefaultBike()
}

// presetBase returns the build the CAD Baseline Fork preset applies the chart
// fork to on an error page: base's frame, size, bar, post, targets, and bar and
// fork options, with every other input at its default, so the link is valid.
func presetBase(base query.Bike) query.Bike {
	vals := url.Values{
		"frame": {base.Frame.ID},
		"size":  {base.Size.Name},
		"bar":   {base.Bar.ID},
		"post":  {base.Post.ID},
	}
	if base.BrpTarget != nil {
		vals.Set("brp_x", query.FormatFloat(base.BrpTarget.X))
		vals.Set("brp_y", query.FormatFloat(base.BrpTarget.Y))
	} else {
		vals.Set("brp", "off")
	}
	if base.GripTarget != nil {
		vals.Set("grip_x", query.FormatFloat(base.GripTarget.X))
		vals.Set("grip_y", query.FormatFloat(base.GripTarget.Y))
	} else {
		vals.Set("grip", "off")
	}
	if base.AllBarOptions {
		vals.Set("bar_options", query.BarOptionsAll)
	}
	if base.AllForkOptions {
		vals.Set("fork_options", query.ForkOptionsAll)
	}
	b, err := query.ParseBike(vals)
	if err != nil {
		return query.DefaultBike()
	}
	return b
}

// renderBikeError renders the editable controls with a 400. base supplies the
// resolved frame, size, fork, and bounds; q supplies the text to echo; invalid
// names the rejected query key.
func renderBikeError(w http.ResponseWriter, q url.Values, base query.Bike, invalid, msg string) {
	sel := selectedIDs{Frame: base.Frame.ID, Size: base.Size.Name, Fork: base.ForkID, Bar: base.Bar.ID, Post: base.Post.ID}
	switch invalid {
	case "frame":
		sel.Frame = ""
	case "size":
		sel.Size = ""
	case "fork":
		sel.Fork = ""
	case "bar":
		sel.Bar = ""
	case "post":
		sel.Post = ""
	}
	view := controlsView(base, echoForm(q, base, sel), sel)
	view.ErrorMessage = msg
	view.InvalidField = invalid
	view.DefaultsURL = query.DefaultBike().WithView(base.View).URL()
	view.CadBaselineForkURL = presetBase(base).WithView(base.View).WithCadBaselineFork().URL()
	renderBikeTemplate(w, http.StatusBadRequest, view)
}

// buildFittingView presents the fitting, setback notes, submitted-build score,
// and ranked builds of an evaluated bike.
func buildFittingView(parsed query.Bike, res query.BikeResult) (*bikeFittingView, error) {
	fitting, bike := *res.Fitting, res.Placement
	fv := &bikeFittingView{}
	var saddleLimits, cockpitLimits []assembly.InputField
	for _, f := range fitting.LimitedBy {
		switch f {
		case assembly.InputSeatpostInsertion, assembly.InputSaddleRailOffset:
			saddleLimits = append(saddleLimits, f)
		case assembly.InputStemSpacers, assembly.InputStemLength:
			cockpitLimits = append(cockpitLimits, f)
		case assembly.InputForkA2c, assembly.InputForkSag, assembly.InputLowerCup, assembly.InputStemAngle,
			assembly.InputHandlebarRoll, assembly.InputSeatpostSetback, assembly.InputSaddleRailPitch,
			assembly.InputFitTargetBRP, assembly.InputFitTargetGrip:
			// Fitting never limits on these inputs.
		}
	}
	fitted := fitting.Placement.Setup
	if brp := fitting.Target.BRP; brp != nil {
		railLimit, _ := bike.Setup.RailOffsetLimitMm() // PlaceBike and ApplyFitting already accepted this setup
		miss := bike.Saddle.BRP.OffsetTo(pose.NewPose(*brp, 0))
		fv.Saddle = &saddleFittingView{
			TargetX: brp.X, TargetY: brp.Y,
			MissX: miss.X, MissY: miss.Y, MissMm: miss.Norm(),
			InsertionMm:  fitted.Seatpost.InsertionMm,
			RailOffsetMm: fitted.Saddle.RailOffsetMm,
			RailLimitMm:  railLimit,
			LimitedBy:    saddleLimits,
			Centered:     makePostSetbackView(*res.SetbackCentered),
		}
		if res.SetbackReach != nil {
			r := makePostSetbackView(*res.SetbackReach)
			fv.Saddle.Reach = &r
		}
	}
	if grip := fitting.Target.HandGrip; grip != nil {
		miss := bike.Handlebar.HandGrip.OffsetTo(pose.NewPose(*grip, 0))
		stem := bike.Setup.Stem
		fv.Cockpit = &cockpitFittingView{
			TargetX: grip.X, TargetY: grip.Y,
			MissX: miss.X, MissY: miss.Y, MissMm: miss.Norm(),
			SpacersMm:       fitted.Stem.SpacersMm,
			MaxSpacersMm:    stem.MaxSpacersMm,
			StemLengthMm:    fitted.Stem.LengthMm,
			MinStemLengthMm: stem.MinLengthMm,
			MaxStemLengthMm: stem.MaxLengthMm,
			StemAngleDeg:    stem.AngleDeg,
			BarRollDeg:      bike.Setup.Handlebar.BarRollDeg,
			LimitedBy:       cockpitLimits,
		}
		fv.SubmittedScore = makeSubmittedScoreView(parsed, res.Score)
	}
	fv.ApplyURL = parsed.WithSetup(fitted).URL()
	if res.Builds != nil {
		var err error
		fv.Builds, err = makeBuildRankingView(parsed, *res.Builds)
		if err != nil {
			return nil, err
		}
	}
	return fv, nil
}

// makeSubmittedScoreView presents the submitted build's score, or the note
// that an off-catalog stem has none (Q110).
func makeSubmittedScoreView(parsed query.Bike, score *assembly.BuildScore) *submittedScoreView {
	if score == nil {
		stem := parsed.Setup.Stem
		return &submittedScoreView{
			OffCatalogNote: fmt.Sprintf("Off-catalog stem (%s mm, %+.1f°): build score applies to catalog stems.", query.FormatFloat(stem.LengthMm), stem.AngleDeg),
		}
	}
	weights := catalog.DefaultScoreWeights()
	class, label := fitTierPresentation(score.Tier)
	return &submittedScoreView{
		Scored:           true,
		Tier:             score.Tier,
		TierClass:        class,
		TierLabel:        label,
		Score:            score.Total,
		ActiveWeight:     weights.ActiveTotal(score.HasBrp),
		HasBrp:           score.HasBrp,
		GripR:            score.GripR,
		BrpMissXMm:       score.BrpMissXMm,
		BrpMissYMm:       score.BrpMissYMm,
		AttributeHeaders: buildAttributeHeaders(weights),
		Attributes:       score.Attributes,
	}
}

// makeBuildRankingView presents the ranked builds of an evaluated bike.
func makeBuildRankingView(parsed query.Bike, ranking query.BikeBuildRanking) (*buildRankingView, error) {
	weights := catalog.DefaultScoreWeights()
	hasBrp := parsed.BrpTarget != nil
	view := &buildRankingView{
		AllForks:         parsed.AllForkOptions,
		HasBrp:           hasBrp,
		ActiveWeight:     weights.ActiveTotal(hasBrp),
		PartsText:        buildPartsText(parsed.AllForkOptions, parsed.AllBarOptions),
		Evaluated:        ranking.Evaluated,
		AttributeHeaders: buildAttributeHeaders(weights),
		TierLegend:       fitTierLegend(),
		Rows:             make([]scoredBuildView, 0, len(ranking.Builds)),
	}
	for i, c := range ranking.Builds {
		stem := c.Placement.Setup.Stem
		applied := parsed.WithSetup(c.Placement.Setup)
		bar := parsed.Bar
		if c.HasHandlebarOption {
			found, ok := catalog.HandlebarByID(c.HandlebarID)
			if !ok {
				return nil, fmt.Errorf("scored build %d names unknown handlebar %q", i+1, c.HandlebarID)
			}
			bar = found
			applied.Bar = found
		}
		if c.HasForkOption {
			ff, ok := frameForkByID(parsed, c.ForkID)
			if !ok {
				return nil, fmt.Errorf("scored build %d names unknown fork %q", i+1, c.ForkID)
			}
			applied.ForkID = c.ForkID
			applied.FrameFork = ff
		}
		forkName, travelMm := forkNameAndTravel(parsed, c.ForkID)
		tierClass, tierLabel := fitTierPresentation(c.Score.Tier)
		view.Rows = append(view.Rows, scoredBuildView{
			Rank:         i + 1,
			Tier:         c.Score.Tier,
			TierClass:    tierClass,
			TierLabel:    tierLabel,
			BarName:      bar.Name,
			ForkName:     forkCellName(forkName, travelMm, c.Placement.Setup.Fork.SagPct),
			StemLengthMm: stem.LengthMm,
			SpacersMm:    stem.SpacersMm,
			StemAngleDeg: stem.AngleDeg,
			GripMissMm:   c.GripMiss().Norm(),
			GripR:        c.Score.GripR,
			HasBrp:       c.Score.HasBrp,
			BrpMissXMm:   c.Score.BrpMissXMm,
			BrpMissYMm:   c.Score.BrpMissYMm,
			Score:        c.Score.Total,
			Attributes:   c.Score.Attributes,
			ApplyURL:     applied.URL(),
		})
	}
	return view, nil
}

func frameForkByID(b query.Bike, id string) (catalog.FrameFork, bool) {
	for _, ff := range b.Frame.Forks {
		if ff.Fork.ID == id {
			return ff, true
		}
	}
	return catalog.FrameFork{}, false
}
