package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/query"
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

const basisForkName = "Chart Basis Fork"

func makeForkOption(opt catalog.ForkOption) forkOption {
	var kind string
	if !opt.Fork.Travel.IsSuspension() {
		kind = "Rigid"
	} else {
		kind = fmt.Sprintf("%.0f mm travel", opt.Fork.Travel.Mm())
	}
	return forkOption{
		ID:       opt.ID(),
		Name:     opt.Name(),
		Label:    fmt.Sprintf("%s (%s, %s mm A2C / %s mm rake)", opt.Fork.Name, kind, query.FormatFloat(opt.Variant.StaticA2cMm), query.FormatFloat(opt.Variant.RakeMm)),
		TravelMm: opt.Fork.Travel.Mm(),
	}
}

func makeBasisForkOption(frame catalog.Frame, selected bool) selectOption {
	return selectOption{
		Value: query.BasisForkID,
		Label: fmt.Sprintf("%s (%.0f mm A2C / %.0f mm rake, %.0f mm cup)",
			basisForkName, frame.BasisFork.BasisA2cMm(), frame.BasisFork.RakeMm(), frame.BasisLowerCup.StackMm),
		Selected: selected,
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
	if id == query.BasisForkID {
		return basisForkName, 0
	}
	for _, opt := range b.Frame.ForkOptions() {
		if opt.ID() == id {
			return opt.Name(), opt.Fork.Travel.Mm()
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
	p := b.Params
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
		ForkA2c:     formatFormFloat(p.ForkA2cMm),
		Cup:         formatFormFloat(p.LowerCupMm),
		Sag:         formatFormFloat(p.SagPct),
		Spacers:     formatFormFloat(p.SpacersMm),
		StemLength:  formatFormFloat(p.StemLengthMm),
		StemAngle:   formatFormFloat(float64(p.StemAngleDeg)),
		BarRoll:     formatFormFloat(float64(p.BarRollDeg)),
		PostInsert:  formatFormFloat(p.PostInsertMm),
		RailOffset:  formatFormFloat(p.RailOffsetMm),
		RailPitch:   formatFormFloat(float64(p.RailPitchDeg)),
		BarOptions:  query.BarOptionsSelected,
		ForkOptions: query.ForkOptionsSelected,
	}
	if b.BrpTarget != nil {
		form.BrpX, form.BrpY = formatFormFloat(b.BrpTarget.X()), formatFormFloat(b.BrpTarget.Y())
	}
	if b.GripTarget != nil {
		form.GripX, form.GripY = formatFormFloat(b.GripTarget.X()), formatFormFloat(b.GripTarget.Y())
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
// in q where there is any, otherwise the values of base. A locked basis
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
	if base.ForkID != query.BasisForkID || sel.Fork == "" {
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

func buildBikeFrameOptions(selectedID, invalidRaw string, invalid bool) []selectOption {
	frames := catalog.Frames()
	opts := make([]selectOption, 0, len(frames)+1)
	if invalid && invalidRaw != "" {
		opts = append(opts, selectOption{Value: invalidRaw, Label: "Invalid: " + invalidRaw, Selected: true, Invalid: true})
	}
	for _, f := range frames {
		opts = append(opts, selectOption{
			Value:    f.ID,
			Label:    fmt.Sprintf("%s %s (%d)", f.Manufacturer, f.Model, f.Year),
			Selected: !invalid && f.ID == selectedID,
		})
	}
	return opts
}

func buildBikeSizeOptions(f catalog.Frame, selectedSize, invalidRaw string, invalid bool) []selectOption {
	opts := make([]selectOption, 0, len(f.Sizes)+1)
	if invalid && invalidRaw != "" {
		opts = append(opts, selectOption{Value: invalidRaw, Label: "Invalid: " + invalidRaw, Selected: true, Invalid: true})
	}
	for _, sz := range f.Sizes {
		opts = append(opts, selectOption{
			Value:    sz.Name,
			Label:    sz.Name,
			Selected: !invalid && sz.Name == selectedSize,
		})
	}
	return opts
}

func buildBikeModeOptions(allSelected bool, selectedLabel, allLabel, invalidRaw string, invalid bool) []selectOption {
	opts := make([]selectOption, 0, 3)
	if invalid && invalidRaw != "" {
		opts = append(opts, selectOption{Value: invalidRaw, Label: "Invalid: " + invalidRaw, Selected: true, Invalid: true})
	}
	opts = append(opts,
		selectOption{Value: query.BarOptionsSelected, Label: selectedLabel, Selected: !invalid && !allSelected},
		selectOption{Value: query.BarOptionsAll, Label: allLabel, Selected: !invalid && allSelected},
	)
	return opts
}

// controlsView fills the parts of bikeView the controls form needs: the
// selects, bounds, notes, and form values. It is shared by the build page and
// the error pages.
func controlsView(b query.Bike, form bikeFormValues, sel selectedIDs, invalidField, invalidRaw string) bikeView {
	frameForkOptions := b.Frame.ForkOptions()
	var forkLeading, listedForks, otherForks []selectOption
	forkInvalid := invalidField == "fork" && invalidRaw != ""
	if forkInvalid {
		forkLeading = append(forkLeading, selectOption{Value: invalidRaw, Label: "Invalid: " + invalidRaw, Selected: true, Invalid: true})
	}
	for _, opt := range frameForkOptions {
		fo := makeForkOption(opt)
		so := selectOption{Value: fo.ID, Label: fo.Label, Selected: !forkInvalid && fo.ID == sel.Fork}
		if b.Frame.IsListedFork(opt.Fork.ID) {
			listedForks = append(listedForks, so)
		} else {
			otherForks = append(otherForks, so)
		}
	}
	barInvalid := invalidField == "bar" && invalidRaw != ""
	barSelected := sel.Bar
	if barInvalid {
		barSelected = invalidRaw
	}
	postInvalid := invalidField == "post" && invalidRaw != ""
	postSelected := sel.Post
	if postInvalid {
		postSelected = invalidRaw
	}
	bars := catalog.Handlebars()
	forkA2cRO, cupRO, sagRO, cupLabel := forkFieldViewState(b, sel.Fork, form.Sag)
	var minPostIns, maxPostIns, minRail, maxRail float64
	if insBounds, railBounds, err := b.RearBounds(); err == nil {
		minPostIns, maxPostIns = insBounds.Min(), insBounds.Max()
		minRail, maxRail = railBounds.Min(), railBounds.Max()
	}
	brp, grip := catalog.DefaultBrpTarget(), catalog.DefaultGripTarget()
	v := bikeView{
		Form:                 form,
		MinRailOffsetMm:      minRail,
		MaxRailOffsetMm:      maxRail,
		ExampleBrpX:          formatFormFloat(brp.X()),
		ExampleBrpY:          formatFormFloat(brp.Y()),
		ExampleGripX:         formatFormFloat(grip.X()),
		ExampleGripY:         formatFormFloat(grip.Y()),
		MaxSpacersMm:         assembly.DefaultMaxSpacersMm,
		MinPostInsertionMm:   minPostIns,
		MaxPostInsertionMm:   maxPostIns,
		PostTotalLengthMm:    b.Post.TotalLengthMm(),
		PostSetbackMm:        b.Post.SetbackMm(),
		PostClampLengthMm:    b.Post.RailClampLengthMm(),
		BikeName:             fmt.Sprintf("%s %s (%d, Size %s)", b.Frame.Manufacturer, b.Frame.Model, b.Frame.Year, b.Size.Name),
		FrameOptions:         buildBikeFrameOptions(sel.Frame, invalidRaw, invalidField == "frame" && invalidRaw != ""),
		SizeOptions:          buildBikeSizeOptions(b.Frame, sel.Size, invalidRaw, invalidField == "size" && invalidRaw != ""),
		ForkLeading:          forkLeading,
		ListedForkOptions:    listedForks,
		OtherForkOptions:     otherForks,
		BasisForkOptions:     []selectOption{makeBasisForkOption(b.Frame, !forkInvalid && sel.Fork == query.BasisForkID)},
		BarSelectOptions:     buildBarSelectOptions(barSelected, barInvalid),
		PostSelectOptions:    buildPostSelectOptions(postSelected, postInvalid),
		BarOptionModes:       buildBikeModeOptions(form.BarOptions == query.BarOptionsAll, "Selected handlebar only", fmt.Sprintf("All %d handlebars", len(bars)), invalidRaw, invalidField == "bar_options" && invalidRaw != ""),
		ForkOptionModes:      buildBikeModeOptions(form.ForkOptions == query.ForkOptionsAll, "Selected fork only", fmt.Sprintf("All %d fork setups", len(b.ForkOptions())), invalidRaw, invalidField == "fork_options" && invalidRaw != ""),
		CupLabel:             cupLabel,
		ForkA2cReadOnly:      forkA2cRO,
		CupReadOnly:          cupRO,
		SagReadOnly:          sagRO,
		BasisForkNote:        basisForkNote(b, sel.Fork),
		SlidingDropoutsNote:  slidingDropoutsNote(b),
		BasisFork:            b.Frame.BasisFork,
		BasisLowerCupMm:      b.Frame.BasisLowerCup.StackMm,
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
	isBasis := selectedForkID == query.BasisForkID
	forkA2cRO = isBasis
	cupRO = isBasis
	// On a known rigid fork, keep sag read-only only when the displayed value is
	// already valid zero; if an invalid or non-zero sag was submitted, leave the
	// input editable so the user can reset it to 0 without losing other settings.
	rigid := !isBasis && !b.ForkOption.Fork.Travel.IsSuspension()
	sagRO = isBasis || (selectedForkID != "" && rigid && isZeroFormFloat(sagValue))
	cupLabel = lowerCupLabel(b.LowerCup().Standard)
	return forkA2cRO, cupRO, sagRO, cupLabel
}

func basisForkNote(b query.Bike, selectedForkID string) string {
	if selectedForkID != query.BasisForkID {
		return ""
	}
	basisCup := b.Frame.BasisLowerCup
	physCup := b.Frame.InstalledDims(b.Size).DefaultLowerCup
	if basisCup.Standard == assembly.LowerCupBareHeadTube {
		return fmt.Sprintf(
			"Chart Basis Fork: the published frame chart omits the lower headset cup (%s, %s mm), and these chart fields are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.",
			lowerCupStandardName(basisCup.Standard), query.FormatFloat(basisCup.StackMm),
			lowerCupStandardName(physCup.Standard), query.FormatFloat(physCup.StackMm),
		)
	}
	return fmt.Sprintf(
		"Chart Basis Fork: chart A2C, lower cup (%s, %s mm), and 0%% sag are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.",
		lowerCupStandardName(basisCup.Standard), query.FormatFloat(basisCup.StackMm),
		lowerCupStandardName(physCup.Standard), query.FormatFloat(physCup.StackMm),
	)
}

// slidingDropoutsNote explains which chainstay a sliding-dropout frame is
// evaluated at; empty for fixed dropouts.
func slidingDropoutsNote(b query.Bike) string {
	maxCs := b.Frame.MaxChainstayMm
	if maxCs <= 0 {
		return ""
	}
	published := b.Size.ChartDims.ChainstayMm
	switch {
	case b.ForkID == query.BasisForkID && maxCs != published:
		return fmt.Sprintf("Chart Basis Fork uses published chainstay (%.0f mm); installed physical forks evaluate sliding dropouts at max chainstay (%.0f mm).", published, maxCs)
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

// presetBase returns the build the Chart Basis Fork preset applies the chart
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
		vals.Set("brp_x", query.FormatFloat(base.BrpTarget.X()))
		vals.Set("brp_y", query.FormatFloat(base.BrpTarget.Y()))
	} else {
		vals.Set("brp", "off")
	}
	if base.GripTarget != nil {
		vals.Set("grip_x", query.FormatFloat(base.GripTarget.X()))
		vals.Set("grip_y", query.FormatFloat(base.GripTarget.Y()))
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
	view := controlsView(base, echoForm(q, base, sel), sel, invalid, strings.TrimSpace(q.Get(invalid)))
	view.PageTitle = "Error — " + view.BikeName + " — bikegeo"
	view.ErrorMessage = msg
	view.InvalidField = invalid
	view.DefaultsURL = query.DefaultBike().WithView(base.View).URL()
	view.BasisForkURL = presetBase(base).WithView(base.View).WithBasisFork().URL()
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
		railLimit := bike.Bounds.SaddleRailOffset.Max
		miss := bike.Saddle.BRP.Origin().OffsetTo(*brp)
		fv.Saddle = &saddleFittingView{
			TargetX: brp.X(), TargetY: brp.Y(),
			MissX: cleanZero(miss.X(), 0.05), MissY: cleanZero(miss.Y(), 0.05), MissMm: miss.Norm(),
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
		miss := bike.Handlebar.HandGrip.Origin().OffsetTo(*grip)
		stem := bike.Setup.Stem
		fv.Cockpit = &cockpitFittingView{
			TargetX: grip.X(), TargetY: grip.Y(),
			MissX: cleanZero(miss.X(), 0.05), MissY: cleanZero(miss.Y(), 0.05), MissMm: miss.Norm(),
			SpacersMm:       fitted.Stem.SpacersMm,
			MaxSpacersMm:    bike.Bounds.StemSpacers.Max,
			StemLengthMm:    fitted.Stem.LengthMm,
			MinStemLengthMm: catalog.DefaultMinStemLengthMm,
			MaxStemLengthMm: catalog.DefaultMaxStemLengthMm,
			StemAngleDeg:    stem.AngleDeg,
			BarRollDeg:      bike.Setup.Handlebar.BarRollDeg,
			LimitedBy:       cockpitLimits,
		}
		fv.SubmittedScore = makeSubmittedScoreView(parsed, res.Score)
	}
	fv.ApplyURL = parsed.WithPlacement(fitting.Placement).URL()
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
// that an off-catalog stem has none.
func makeSubmittedScoreView(parsed query.Bike, score *assembly.BuildScore) *submittedScoreView {
	if score == nil {
		return &submittedScoreView{
			OffCatalogNote: fmt.Sprintf("Off-catalog stem (%s mm, %+.1f°): build score applies to catalog stems.", query.FormatFloat(parsed.Params.StemLengthMm), parsed.Params.StemAngleDeg),
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
		BrpMissXMm:       cleanZero(score.BrpMissXMm, 0.05),
		BrpMissYMm:       cleanZero(score.BrpMissYMm, 0.05),
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
		applied := parsed.WithPlacement(c.Placement)
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
			opt, ok := forkOptionByID(parsed, c.ForkID)
			if !ok {
				return nil, fmt.Errorf("scored build %d names unknown fork %q", i+1, c.ForkID)
			}
			applied.ForkID = c.ForkID
			applied.ForkOption = opt
		}
		forkName, travelMm := forkNameAndTravel(parsed, c.ForkID)
		forkLabel := forkCellName(forkName, travelMm, c.Placement.Setup.Fork.SagPct)
		tierClass, tierLabel := fitTierPresentation(c.Score.Tier)
		gripMiss := c.GripMiss()
		view.Rows = append(view.Rows, scoredBuildView{
			Rank:           i + 1,
			Tier:           c.Score.Tier,
			TierClass:      tierClass,
			TierLabel:      tierLabel,
			BarName:        bar.Name,
			ForkName:       forkLabel,
			StemLengthMm:   stem.LengthMm,
			SpacersMm:      stem.SpacersMm,
			StemAngleDeg:   stem.AngleDeg,
			GripMissMm:     gripMiss.Norm(),
			GripMissXMm:    cleanZero(gripMiss.X(), 0.05),
			GripMissYMm:    cleanZero(gripMiss.Y(), 0.05),
			GripR:          c.Score.GripR,
			HasBrp:         c.Score.HasBrp,
			BrpMissXMm:     cleanZero(c.Score.BrpMissXMm, 0.05),
			BrpMissYMm:     cleanZero(c.Score.BrpMissYMm, 0.05),
			Score:          c.Score.Total,
			Attributes:     c.Score.Attributes,
			ApplyURL:       applied.URL(),
			ApplyAriaLabel: fmt.Sprintf("Apply rank %d build: %.0f mm %+.0f° stem, %.0f mm spacers, %s, %s", i+1, stem.LengthMm, cleanZero(float64(stem.AngleDeg), 0.5), stem.SpacersMm, bar.Name, forkLabel),
		})
	}
	return view, nil
}

func forkOptionByID(b query.Bike, id string) (catalog.ForkOption, bool) {
	for _, opt := range b.Frame.ForkOptions() {
		if opt.ID() == id {
			return opt, true
		}
	}
	return catalog.ForkOption{}, false
}
