package web

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/compare"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
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

type svgLine struct {
	X1, Y1, X2, Y2 string
	Class          string
}

type svgCircle struct {
	CX, CY, R string
	Class     string
}

type svgPoint struct {
	ID, Label  string
	XStr, YStr string
	SvgX, SvgY string
	LabelX     string
	LabelY     string
	Radius     string
	Kind       string
}

type bikeSVGView struct {
	ViewBox string
	Lines   []svgLine
	Circles []svgCircle
	Points  []svgPoint
}

type bikeData struct {
	base
	Banner       []string
	Ready        bool
	FrameID      string
	Size         string
	Fork         string
	Sag          string
	SaddleID     string
	HandlebarID  string
	TargetRef    string
	SaddleH      string
	StemLen      string
	StemAng      string
	Spacer       string
	BarRoll      string
	PostSB       string
	RailOff      string
	TargetX      string
	TargetY      string
	Frames       []Option
	Sizes        []Option
	Forks        []Option
	Sags         []Option
	Saddles      []Option
	Handlebars   []Option
	TargetRefs   []Option
	Nudges       []NudgeControl
	SVG          bikeSVGView
	SummaryTable TableView
	Permalink    string
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
		base:        newBase("Bike", "/bike"),
		Banner:      errs,
		FrameID:     p.frameID,
		Size:        p.size,
		Fork:        p.fork,
		Sag:         p.sag,
		SaddleID:    p.saddle.ID,
		HandlebarID: p.handlebar.ID,
		TargetRef:   string(p.targetRef),
		SaddleH:     formatFloatInput(p.saddleH),
		BarRoll:     formatFloatInput(p.barRoll),
		PostSB:      formatFloatInput(p.postSB),
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
	canonicalizer.ApplyIdealCockpitTargets([]model.CanonicalComparisonRow{row}, prof)

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
	d.SummaryTable = toView(buildBikeSummaryTable(scene))

	s.render(w, http.StatusOK, "bike.html", d)
}

func parseBikeQuery(v url.Values, allFrames []model.FrameModel) (bikeParams, []string) {
	var errs []string
	var p bikeParams

	frameID := strings.TrimSpace(v.Get("frame"))
	prevFrame := strings.TrimSpace(v.Get("prev_frame"))
	switchedFrame := prevFrame != "" && frameID != "" && !strings.EqualFold(prevFrame, frameID)

	if frameID == "" {
		frameID = "otso-fenrir-stainless"
	}
	fm, err := frames.GetFrameByID(frameID)
	if err != nil {
		errs = append(errs, err.Error())
		p.frameID = frameID
	} else {
		p.frameModel = *fm
		p.frameID = fm.ID
		sizes := fm.GetSizes()
		reqSize := strings.TrimSpace(v.Get("size"))
		switch {
		case reqSize == "":
			p.size = defaultSizeForFrame(sizes)
		case hasExactFold(sizes, reqSize) != "":
			p.size = hasExactFold(sizes, reqSize)
		case switchedFrame:
			p.size = defaultSizeForFrame(sizes)
		default:
			errs = append(errs, fmt.Sprintf("size %q not found on frame %q", reqSize, fm.ID))
			p.size = reqSize
		}

		choices := forkChoices(*fm)
		reqFork := strings.TrimSpace(v.Get("fork"))
		switch {
		case reqFork == "":
			if hasExactFold(choices, "rigid") != "" {
				p.fork = "rigid"
			} else if len(choices) > 0 {
				p.fork = choices[0]
			}
		case switchedFrame && hasExactFold(choices, reqFork) == "":
			if hasExactFold(choices, "rigid") != "" {
				p.fork = "rigid"
			} else if len(choices) > 0 {
				p.fork = choices[0]
			}
		default:
			p.fork = reqFork
		}
	}

	if rawSag := strings.TrimSpace(v.Get("sag")); rawSag != "" {
		clean := strings.TrimSuffix(rawSag, "%")
		sagVal, err := parseFiniteFloat("sag", clean)
		if err != nil {
			errs = append(errs, err.Error())
		} else if sagVal < 0 || sagVal >= 100 {
			errs = append(errs, fmt.Sprintf("sag must be in [0, 100), got %.1f", sagVal))
		} else if !switchedFrame || p.fork != "rigid" || sagVal == 0 {
			p.sag = strconv.FormatFloat(sagVal, 'f', -1, 64)
		}
	}

	saddleID := strings.TrimSpace(v.Get("saddle"))
	if saddleID == "" {
		p.saddle = catalog.DefaultSaddle()
	} else if sd, ok := catalog.GetSaddle(saddleID); ok {
		p.saddle = sd
	} else {
		errs = append(errs, fmt.Sprintf("unknown saddle %q", saddleID))
	}

	barID := strings.TrimSpace(v.Get("handlebar"))
	if barID == "" {
		p.handlebar = catalog.DefaultHandlebar()
	} else if hb, ok := catalog.GetHandlebar(barID); ok {
		p.handlebar = hb
	} else {
		errs = append(errs, fmt.Sprintf("unknown handlebar %q", barID))
	}

	tr := strings.TrimSpace(v.Get("target_ref"))
	if tr == "" {
		p.targetRef = model.TargetHandGrip
	} else {
		ref := model.TargetRef(tr)
		if !ref.IsValid() {
			errs = append(errs, fmt.Sprintf("unknown target_ref %q (use hand_grip, stem_clamp, or reference_bar_clamp)", tr))
		} else {
			p.targetRef = ref
		}
	}

	p.saddleH = model.DefaultIdmatchSaddleHeightMm
	if raw := strings.TrimSpace(v.Get("saddle_h")); raw != "" {
		val, err := parseFiniteFloat("saddle_h", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val <= 0 {
			errs = append(errs, fmt.Sprintf("saddle_h must be positive, got %.1f", val))
		} else {
			p.saddleH = val
		}
	}

	if raw := strings.TrimSpace(v.Get("stem_len")); raw != "" {
		val, err := parseFiniteFloat("stem_len", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val <= 0 {
			errs = append(errs, fmt.Sprintf("stem_len must be positive, got %.1f", val))
		} else {
			p.hasStemLen = true
			p.stemLen = val
		}
	}

	if raw := strings.TrimSpace(v.Get("stem_ang")); raw != "" {
		val, err := parseFiniteFloat("stem_ang", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if math.Abs(val) >= 90 {
			errs = append(errs, fmt.Sprintf("stem_ang must satisfy |ang| < 90°, got %.1f", val))
		} else {
			p.hasStemAng = true
			p.stemAng = val
		}
	}

	if raw := strings.TrimSpace(v.Get("spacer")); raw != "" {
		val, err := parseFiniteFloat("spacer", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val < 0 {
			errs = append(errs, fmt.Sprintf("spacer must be non-negative, got %.1f", val))
		} else {
			p.hasSpacer = true
			p.spacer = val
		}
	}

	if raw := strings.TrimSpace(v.Get("bar_roll")); raw != "" {
		val, err := parseFiniteFloat("bar_roll", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.barRoll = val
		}
	}

	p.postSB = model.DefaultSeatpostSetbackMm
	if raw := strings.TrimSpace(v.Get("post_sb")); raw != "" {
		val, err := parseFiniteFloat("post_sb", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else if val < 0 {
			errs = append(errs, fmt.Sprintf("post_sb must be non-negative, got %.1f", val))
		} else {
			p.postSB = val
		}
	}

	if raw := strings.TrimSpace(v.Get("rail_off")); raw != "" {
		val, err := parseFiniteFloat("rail_off", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasRailOff = true
			p.railOff = val
		}
	}

	if raw := strings.TrimSpace(v.Get("target_x")); raw != "" {
		val, err := parseFiniteFloat("target_x", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasTargetX = true
			p.targetX = val
		}
	}

	if raw := strings.TrimSpace(v.Get("target_y")); raw != "" {
		val, err := parseFiniteFloat("target_y", raw)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			p.hasTargetY = true
			p.targetY = val
		}
	}

	return p, errs
}

func parseFiniteFloat(name, raw string) (float64, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s: %q is not a finite number", name, raw)
	}
	return f, nil
}

func defaultSizeForFrame(sizes []string) string {
	for _, s := range sizes {
		if strings.EqualFold(s, "L") {
			return s
		}
	}
	if len(sizes) > 0 {
		return sizes[0]
	}
	return ""
}

func hasExactFold(vals []string, want string) string {
	for _, v := range vals {
		if strings.EqualFold(v, want) {
			return v
		}
	}
	return ""
}

func formatFloatInput(v float64) string {
	if math.Abs(v) < 1e-9 {
		return "0"
	}
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
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

func buildBikeSVG(sc geometry.BikeScene) bikeSVGView {
	groundY := sc.RearAxle.Y - sc.Build.WheelRadiusRearMm
	minX := math.Floor(sc.RearAxle.X - sc.Build.WheelRadiusRearMm - 90)
	maxX := math.Ceil(sc.FrontAxle.X + sc.Build.WheelRadiusFrontMm + 90)
	maxY := math.Ceil(math.Max(math.Max(sc.SitBones.Y, sc.HandGrip.Y), math.Max(sc.TargetPoint.Y, sc.RequiredStemClamp.Y)) + 110)
	minY := math.Floor(groundY - 70)

	f2 := func(x float64) string { return fmt.Sprintf("%.2f", x) }
	addLine := func(a, b geom2d.Point2, cls string) svgLine {
		return svgLine{X1: f2(a.X), Y1: f2(a.Y), X2: f2(b.X), Y2: f2(b.Y), Class: cls}
	}

	var lines []svgLine
	// Ground & BB axes
	lines = append(lines,
		addLine(geom2d.Point2{X: minX + 20, Y: groundY}, geom2d.Point2{X: maxX - 20, Y: groundY}, "ground"),
		addLine(geom2d.Point2{X: minX + 20, Y: 0}, geom2d.Point2{X: maxX - 20, Y: 0}, "axis"),
		addLine(geom2d.Point2{X: 0, Y: groundY}, geom2d.Point2{X: 0, Y: maxY - 20}, "axis"),
	)

	// Frame tubes & stays
	lines = append(lines,
		addLine(sc.BB, sc.RearAxle, "frame"),
		addLine(sc.BB, sc.HeadTubeBottom, "frame"),
		addLine(sc.HeadTubeBottom, sc.HeadTubeTop, "frame"),
		addLine(sc.RearAxle, sc.SeatpostClamp, "frame"),
		addLine(sc.SeatpostClamp, sc.HeadTubeTop, "frame"),
	)
	if sc.Frame.HasBentSeatTube() {
		bendPt := sc.FrameSeatAxis.PointAt(180)
		lines = append(lines,
			addLine(sc.BB, bendPt, "frame"),
			addLine(bendPt, sc.SeatpostClamp, "frame"),
		)
	} else {
		lines = append(lines, addLine(sc.BB, sc.SeatpostClamp, "frame"))
	}

	// Fork & steering axis
	lines = append(lines,
		addLine(sc.HeadTubeBottom, sc.LowerCupBottom, "cup"),
		addLine(sc.LowerCupBottom, sc.FrontAxle, "fork"),
	)
	if steerGround, ok := sc.SteererAxis.IntersectHorizontalLine(groundY); ok {
		lines = append(lines, addLine(sc.HeadTubeTop, steerGround, "steer-ray"))
	}

	// Seatpost & saddle
	lines = append(lines,
		addLine(sc.SeatpostClamp, sc.RailCenter, "cockpit"),
		addLine(sc.RailCenter, sc.SitBones, "saddle"),
		addLine(sc.SitBones, sc.BRP, "saddle"),
	)

	// Steerer stack, stem, handlebar offset & rider reach
	lines = append(lines,
		addLine(sc.HeadTubeTop, sc.SteererClamp, "cockpit"),
		addLine(sc.SteererClamp, sc.StemClamp, "cockpit"),
		addLine(sc.StemClamp, sc.HandGrip, "bar"),
		addLine(sc.SitBones, sc.HandGrip, "rider-reach"),
		addLine(sc.StemClamp, sc.RequiredStemClamp, "target-err"),
	)

	circles := []svgCircle{
		{CX: f2(sc.RearAxle.X), CY: f2(sc.RearAxle.Y), R: f2(sc.Build.WheelRadiusRearMm), Class: "wheel"},
		{CX: f2(sc.FrontAxle.X), CY: f2(sc.FrontAxle.Y), R: f2(sc.Build.WheelRadiusFrontMm), Class: "wheel"},
	}

	type ptSpec struct {
		id, label string
		p         geom2d.Point2
		dx, dy    float64
		r         float64
		kind      string
	}
	ptSpecs := []ptSpec{
		{"BB", "BB (0,0)", sc.BB, 12, 22, 7, "frame"},
		{"RA", "RearAxle", sc.RearAxle, -95, 22, 7, "frame"},
		{"FA", "FrontAxle", sc.FrontAxle, 12, 22, 7, "frame"},
		{"HTB", "HeadTubeBottom", sc.HeadTubeBottom, 12, 20, 6, "frame"},
		{"HT", "HeadTubeTop", sc.HeadTubeTop, 12, 24, 7, "frame"},
		{"SPC", "SeatpostClamp", sc.SeatpostClamp, -145, -14, 6, "seat"},
		{"RC", "RailCenter", sc.RailCenter, -120, 20, 5, "seat"},
		{"SB", "SitBones", sc.SitBones, -100, -18, 8, "contact"},
		{"BRP", "BRP", sc.BRP, 12, -16, 7, "contact"},
		{"SC", "SteererClamp", sc.SteererClamp, -135, -8, 6, "cockpit"},
		{"STC", "StemClamp", sc.StemClamp, 12, 24, 7, "contact"},
		{"GR", "HandGrip", sc.HandGrip, -110, -24, 8, "contact"},
		{"TP", fmt.Sprintf("TargetPoint (%s)", sc.TargetRef.Label()), sc.TargetPoint, 14, -18, 9, "target"},
		{"RSC", "RequiredStemClamp C*", sc.RequiredStemClamp, 14, 4, 8, "target"},
	}

	points := make([]svgPoint, 0, len(ptSpecs))
	for _, spec := range ptSpecs {
		svgX := spec.p.X
		svgY := -spec.p.Y
		points = append(points, svgPoint{
			ID:     spec.id,
			Label:  spec.label,
			XStr:   f2(spec.p.X),
			YStr:   f2(spec.p.Y),
			SvgX:   f2(svgX),
			SvgY:   f2(svgY),
			LabelX: f2(svgX + spec.dx),
			LabelY: f2(svgY + spec.dy),
			Radius: f2(spec.r),
			Kind:   spec.kind,
		})
	}

	return bikeSVGView{
		ViewBox: fmt.Sprintf("%.0f %.0f %.0f %.0f", minX, -maxY, maxX-minX, maxY-minY),
		Lines:   lines,
		Circles: circles,
		Points:  points,
	}
}

func buildBikeSummaryTable(sc geometry.BikeScene) report.Table {
	solvedStr := "n/a (parallel)"
	if sol, ok := sc.SolveStemSpacerFit(sc.Cockpit.Stem.AngleDeg); ok {
		sign := "+"
		if sol.SpacerMm < 0 {
			sign = ""
		}
		solvedStr = fmt.Sprintf("%.1fmm %.0f° %s%.1fmm", sol.StemLengthMm, sol.StemAngleDeg, sign, sol.SpacerMm)
	}
	errVec := sc.TargetPointError()
	railStatus := "in window ✓"
	if sc.RailMarginMm < 0 {
		railStatus = fmt.Sprintf("%.1f mm short ✗", -sc.RailMarginMm)
	}
	ptStr := func(p geom2d.Point2) string { return fmt.Sprintf("(%.1f, %.1f) mm", p.X, p.Y) }

	rows := [][]string{
		{"Frame (active)", fmt.Sprintf("Reach %.1f mm, Stack %.1f mm, HTA %.1f°, STA %.1f°", sc.Reach(), sc.Stack(), sc.HtaDeg, sc.EffectiveStaDeg), fmt.Sprintf("Pitch Δθ %+.2f°, Trail %.1f mm, RC/WB %.1f%%", sc.PitchDeg, sc.TrailMm(), sc.RearCenterWheelbaseRatio()*100)},
		{"Stem / Spacer Fit (exact)", solvedStr, fmt.Sprintf("At current stem angle %.0f° to hit C*", sc.Cockpit.Stem.AngleDeg)},
		{"Target Error (C* − StemClamp)", fmt.Sprintf("(%.1f, %.1f) mm (|Δ|=%.1f mm)", errVec.X, errVec.Y, errVec.Norm()), fmt.Sprintf("TargetRef: %s", sc.TargetRef.Label())},
		{"RequiredStemClamp C*", ptStr(sc.RequiredStemClamp), "Stem clamp center required by TargetPoint + bar"},
		{"StemClamp", ptStr(sc.StemClamp), fmt.Sprintf("%.0fmm %.0f° stem + %.0fmm spacers", sc.Cockpit.Stem.LengthMm, sc.Cockpit.Stem.AngleDeg, sc.Cockpit.SpacerMm)},
		{"HandGrip", ptStr(sc.HandGrip), fmt.Sprintf("BRP–Grip %.1f mm, Grip–SitBones Y %+.1f mm", sc.GripDistFromBrpMm(), sc.GripRiseAboveSitBonesMm())},
		{"TargetPoint", ptStr(sc.TargetPoint), sc.TargetRef.Label()},
		{"SitBones", ptStr(sc.SitBones), fmt.Sprintf("Target X = %.1f mm", sc.Fit.TargetSitBonesXMm)},
		{"BRP", ptStr(sc.BRP), fmt.Sprintf("Required rail offset %+.1f mm, margin %.1f mm (%s)", sc.RequiredRailOffsetMm, sc.RailMarginMm, railStatus)},
	}
	t := report.FromStrings(
		"bike-scene",
		"Scene coordinates and solved hardware",
		fmt.Sprintf("TargetRef: %s | Saddle: %s | Handlebar: %s", sc.TargetRef.Label(), sc.Seat.Saddle.Name, sc.Cockpit.Handlebar.Name),
		[]string{"Quantity", "Value", "Notes"},
		rows,
	)
	annotateZones(&t)
	return t
}
