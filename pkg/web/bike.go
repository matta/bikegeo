package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/compare"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/scoring"
)

// NudgeLink is one zero-JS GET hyperlink that shifts a single parameter on /bike.
type NudgeLink struct {
	Label string
	Href  string
	Delta string
}

// NudgeControl groups the current value and incremental GET links for one parameter.
type NudgeControl struct {
	Param string
	Label string
	Value string
	Unit  string
	Links []NudgeLink
}

type bikeData struct {
	base
	Banner         []string
	Ready          bool
	FrameID        string
	Size           string
	Fork           string
	Sag            string
	SaddleID       string
	HandlebarID    string
	TargetRef      string
	TargetRefLabel string
	SaddleH        string
	StemLen        string
	StemAng        string
	Spacer         string
	BarRoll        string
	PostSB         string
	RailOff        string
	TargetX        string
	TargetY        string
	Frames         []Option
	Sizes          []Option
	Forks          []Option
	Sags           []Option
	Saddles        []Option
	Handlebars     []Option
	TargetRefs     []Option
	Nudges         []NudgeControl
	SVG            bikeSVGView
	CockpitSVG     bikeSVGView
	SaddleSVG      bikeSVGView
	SummaryTable   TableView
	LedgerTable    TableView
	Permalink      string
}

type bikeParams struct {
	frameModel model.FrameModel
	frameID    string
	size       string
	fork       string
	sag        string
	saddle     model.SaddleSpec
	handlebar  model.HandlebarSpec
	targetRef  model.TargetRef
	saddleH    float64
	hasStemLen bool
	stemLen    float64
	hasStemAng bool
	stemAng    float64
	hasSpacer  bool
	spacer     float64
	barRoll    float64
	postSB     float64
	hasRailOff bool
	railOff    float64
	hasTargetX bool
	targetX    float64
	hasTargetY bool
	targetY    float64
}

func (s *server) bike(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	allFrames := frames.AllFrames()
	p, errs := parseBikeQuery(v, allFrames)

	d := bikeData{
		base:           newBase("Bike", "/bike"),
		Banner:         errs,
		FrameID:        p.frameID,
		Size:           p.size,
		Fork:           p.fork,
		Sag:            p.sag,
		SaddleID:       p.saddle.ID,
		HandlebarID:    p.handlebar.ID,
		TargetRef:      string(p.targetRef),
		TargetRefLabel: p.targetRef.Label(),
		SaddleH:        formatFloatInput(p.saddleH),
		BarRoll:        formatFloatInput(p.barRoll),
		PostSB:         formatFloatInput(p.postSB),
		TargetRefs: []Option{
			{Value: string(model.TargetHandGrip), Label: "HandGrip", Selected: p.targetRef == model.TargetHandGrip},
			{Value: string(model.TargetStemClamp), Label: "StemClamp", Selected: p.targetRef == model.TargetStemClamp},
			{Value: string(model.TargetReferenceBarClamp), Label: "RefBarClamp", Selected: p.targetRef == model.TargetReferenceBarClamp},
		},
	}

	if p.frameModel.ID != "" {
		d.Frames = make([]Option, 0, len(allFrames))
		for _, f := range allFrames {
			d.Frames = append(d.Frames, Option{Value: f.ID, Label: f.Manufacturer + " " + f.Model, Selected: f.ID == p.frameModel.ID})
		}
		d.Sizes = options(p.frameModel.GetSizes(), p.size, "")
		d.Forks = options(forkChoices(p.frameModel), p.fork, "")
		d.Sags = options([]string{"20", "25", "30"}, p.sag, "—")
	} else {
		for _, f := range allFrames {
			d.Frames = append(d.Frames, Option{Value: f.ID, Label: f.Manufacturer + " " + f.Model, Selected: f.ID == "otso-fenrir-stainless"})
		}
	}
	d.Saddles = saddleOptions(d.SaddleID, "")
	d.Handlebars = handlebarOptions(d.HandlebarID, "")

	if p.hasStemLen {
		d.StemLen = formatFloatInput(p.stemLen)
	}
	if p.hasStemAng {
		d.StemAng = formatFloatInput(p.stemAng)
	}
	if p.hasSpacer {
		d.Spacer = formatFloatInput(p.spacer)
	}
	if p.hasRailOff {
		d.RailOff = formatFloatInput(p.railOff)
	}
	if p.hasTargetX {
		d.TargetX = formatFloatInput(p.targetX)
	}
	if p.hasTargetY {
		d.TargetY = formatFloatInput(p.targetY)
	}

	if len(errs) > 0 {
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}

	spec := p.frameModel.ID + ":" + p.size
	if p.fork != "" {
		spec += "@" + p.fork
	}
	if p.sag != "" {
		spec += "@" + p.sag + "%"
	}
	setup, err := compare.ParseComparisonSpec(allFrames, spec)
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}

	riderProf := catalog.DefaultRiderProfile()
	riderProf.SaddleHeightMm = p.saddleH
	riderProf.Handlebar = p.handlebar
	riderProf.TargetRef = p.targetRef
	riderProf.ReferenceBar = model.Some(catalog.DefaultHandlebar())

	row, err := canonicalizer.ResolveFrameForkSetupWithSaddle(p.frameModel, setup, riderProf, p.saddle)
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}
	prof := ideal.IdmatchProfile()
	prof.TargetRef = p.targetRef
	prof.ReferenceBar = model.Some(catalog.DefaultHandlebar())
	scanned := []model.CanonicalComparisonRow{row}
	canonicalizer.ApplyIdealCockpitTargets(scanned, prof)
	row = scanned[0]

	d.Fork = forkToken(p.frameModel, row)
	if row.Setup.SagPct > 0 {
		d.Sag = strconv.FormatFloat(row.Setup.SagPct, 'f', -1, 64)
	} else {
		d.Sag = ""
	}
	d.Forks = options(forkChoices(p.frameModel), d.Fork, "")
	d.Sags = options([]string{"20", "25", "30"}, d.Sag, "—")

	// Resolve effective stem/spacer defaults from ApplyIdealCockpitTargets if not overridden in query.
	stemLen := p.stemLen
	stemAng := p.stemAng
	spacer := p.spacer
	if !p.hasStemLen {
		if row.Canonical.SelectedHB != nil && row.Canonical.SelectedHB.Stem.LengthMm > 0 {
			stemLen = row.Canonical.SelectedHB.Stem.LengthMm
		} else {
			stemLen = 80.0
		}
	}
	if !p.hasStemAng {
		if row.Canonical.SelectedHB != nil {
			stemAng = float64(row.Canonical.SelectedHB.Stem.AngleDeg)
		} else {
			stemAng = -6.0
		}
	}
	if !p.hasSpacer {
		if row.Canonical.SelectedHB != nil {
			spacer = row.Canonical.SelectedHB.SpacerMm
		} else {
			spacer = 0.0
		}
	}

	frameSize, err := geometry.FrameSizeFromBikeGeometry(row.Bike)
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}
	if frameSize.ChainstayMm <= 0 {
		frameSize.ChainstayMm = math.Max(430.0, math.Abs(frameSize.BbDropMm)+1.0)
	}

	buildFork := frameSize.CadFork
	buildFork.RakeMm = row.EffectiveRake
	buildSag := frameSize.CadSagPct
	physicalLowerCup := frameSize.CadLowerCupMm
	if row.Bike.CadFork.A2cMm <= 0 || math.Abs(row.EffectiveA2c-row.Bike.CadFork.A2cMm) > 1e-6 {
		buildFork = geometry.Fork{StaticA2cMm: row.EffectiveA2c, RakeMm: row.EffectiveRake}
		buildSag = 0
		if row.Bike.HeadsetCupType.IsValid() {
			physicalLowerCup = row.Bike.HeadsetCupType.CupStackMm()
		}
	}
	build := geometry.BuildSpec{
		Fork:               buildFork,
		SagPct:             buildSag,
		PhysicalLowerCupMm: physicalLowerCup,
		WheelRadiusFrontMm: row.Bike.WheelRadiusMm(),
		WheelRadiusRearMm:  row.Bike.WheelRadiusMm(),
	}

	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          spacer,
		Stem:              model.StemSpec{LengthMm: stemLen, AngleDeg: model.Deg(stemAng)},
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         p.handlebar,
		BarRollDeg:        model.Deg(p.barRoll),
		GripInsetMm:       model.DefaultGripInsetMm,
	}

	seat := geometry.SeatSpec{
		PostSetbackMm: p.postSB,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        p.saddle,
		RailOffsetMm:  0.0,
	}
	halfWin := seat.HalfRailWindowMm()
	if p.hasRailOff && math.Abs(p.railOff) > halfWin+1e-6 {
		d.Banner = append(d.Banner, fmt.Sprintf("rail_off %.1f mm exceeds saddle rail travel [%.1f, %.1f]", p.railOff, -halfWin, halfWin))
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}

	refSitBones := model.ReferenceSitBonePoint(p.saddleH, model.DefaultTargetSitBoneSetbackMm)
	brpH := p.saddle.BrpPointFromSitBones(refSitBones).DistanceToOrigin()

	targetPt := geom2d.Point2{X: 480.9, Y: 726.1}
	if p.targetRef == model.TargetStemClamp {
		targetPt = geom2d.Point2{X: 480.9, Y: 676.1}
	} else if row.Canonical.CockpitFitResult != nil {
		ov := row.Canonical.CockpitFitResult.TargetOval
		if ov.CenterX != 0 || ov.CenterY != 0 {
			targetPt = geom2d.Point2{X: ov.CenterX, Y: ov.CenterY}
		}
	}
	if p.hasTargetX {
		targetPt.X = p.targetX
	}
	if p.hasTargetY {
		targetPt.Y = p.targetY
	}

	fit := geometry.RiderFit{
		FrameSeatAxisHeightMm: p.saddleH,
		BrpHeightMm:           brpH,
		TargetSitBonesXMm:     -model.DefaultTargetSitBoneSetbackMm,
		TargetPoint:           targetPt,
		TargetRef:             p.targetRef,
		ReferenceSaddle:       catalog.DefaultSaddle(),
		ReferenceBar:          model.Some(catalog.DefaultHandlebar()),
	}

	scene, err := geometry.BuildScene(frameSize, build, cockpit, seat, fit)
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}

	railOff := p.railOff
	if !p.hasRailOff {
		railOff = math.Max(-halfWin, math.Min(halfWin, scene.RequiredRailOffsetMm))
	}
	seat.RailOffsetMm = railOff
	scene, err = geometry.BuildScene(frameSize, build, cockpit, seat, fit)
	if err != nil {
		d.Banner = append(d.Banner, err.Error())
		s.render(w, http.StatusBadRequest, "bike.html", d)
		return
	}

	d.Ready = true
	d.StemLen = formatFloatInput(stemLen)
	d.StemAng = formatFloatInput(stemAng)
	d.Spacer = formatFloatInput(spacer)
	d.RailOff = formatFloatInput(railOff)
	d.TargetX = formatFloatInput(targetPt.X)
	d.TargetY = formatFloatInput(targetPt.Y)

	baseVals := url.Values{}
	baseVals.Set("frame", p.frameModel.ID)
	baseVals.Set("size", p.size)
	if d.Fork != "" {
		baseVals.Set("fork", d.Fork)
	}
	if d.Sag != "" {
		baseVals.Set("sag", d.Sag)
	}
	baseVals.Set("saddle", p.saddle.ID)
	baseVals.Set("handlebar", p.handlebar.ID)
	baseVals.Set("target_ref", string(p.targetRef))
	baseVals.Set("saddle_h", d.SaddleH)
	baseVals.Set("stem_len", d.StemLen)
	baseVals.Set("stem_ang", d.StemAng)
	baseVals.Set("spacer", d.Spacer)
	baseVals.Set("bar_roll", d.BarRoll)
	baseVals.Set("post_sb", d.PostSB)
	baseVals.Set("rail_off", d.RailOff)
	if p.hasTargetX {
		baseVals.Set("target_x", d.TargetX)
	}
	if p.hasTargetY {
		baseVals.Set("target_y", d.TargetY)
	}
	d.Permalink = "/bike?" + baseVals.Encode()

	d.Nudges = buildBikeNudges(baseVals, stemLen, stemAng, spacer, p.barRoll, p.saddleH, railOff, halfWin)
	d.SVG = buildBikeSVG(scene)
	d.CockpitSVG = buildCockpitZoomSVG(scene, row)
	d.SaddleSVG = buildSaddleZoomSVG(scene)
	d.SummaryTable = toView(buildBikeSummaryTable(scene))
	score := scoring.ScoreRow(row, prof)
	d.LedgerTable = toView(buildComputationLedger("bike-ledger", "5-stage arithmetic computation ledger", scene, row.Canonical.CockpitFitResult, &score))

	s.render(w, http.StatusOK, "bike.html", d)
}

func buildBikeNudges(baseVals url.Values, stemLen, stemAng, spacer, barRoll, saddleH, railOff, halfRailWin float64) []NudgeControl {
	specs := []struct {
		param    string
		label    string
		val      float64
		unit     string
		min, max float64
		deltas   []float64
	}{
		{"stem_len", "Stem len", stemLen, "mm", 10, 250, []float64{-10, -5, 5, 10}},
		{"stem_ang", "Stem ang", stemAng, "°", -85, 85, []float64{-10, -5, 5, 10}},
		{"spacer", "Spacers", spacer, "mm", 0, 200, []float64{-10, -5, 5, 10}},
		{"bar_roll", "Bar roll", barRoll, "°", -90, 90, []float64{-10, -5, 5, 10}},
		{"saddle_h", "Saddle H", saddleH, "mm", 100, 1200, []float64{-10, -5, 5, 10}},
		{"rail_off", "Rail offset", railOff, "mm", -halfRailWin, halfRailWin, []float64{-10, -5, 5, 10}},
	}
	out := make([]NudgeControl, 0, len(specs))
	for _, sp := range specs {
		ctrl := NudgeControl{
			Param: sp.param,
			Label: sp.label,
			Value: formatFloatInput(sp.val),
			Unit:  sp.unit,
		}
		for _, delta := range sp.deltas {
			next := math.Max(sp.min, math.Min(sp.max, sp.val+delta))
			q := cloneValues(baseVals)
			q.Set(sp.param, formatFloatInput(next))
			sign := ""
			if delta > 0 {
				sign = "+"
			}
			dLabel := fmt.Sprintf("%s%g", sign, delta)
			ctrl.Links = append(ctrl.Links, NudgeLink{
				Label: dLabel,
				Delta: dLabel,
				Href:  "/bike?" + q.Encode(),
			})
		}
		out = append(out, ctrl)
	}
	return out
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for k, vs := range v {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}
