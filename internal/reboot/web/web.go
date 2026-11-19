// Package web serves the reboot HTML/SVG workbench.
package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/workbench.js
var workbenchJS []byte

var (
	stemTemplate = template.Must(template.ParseFS(templateFS, "templates/stem.html", "templates/trace_steps.html"))
	bikeTemplate = template.Must(template.ParseFS(templateFS, "templates/bike.html", "templates/trace_steps.html"))
)

func toSVG(p pose.Point) svgPoint {
	return svgPoint{X: p.X, Y: -p.Y}
}

type svgTraceSteps struct {
	Steps         []svgStep
	LineWidth     float64
	ArcWidth      float64
	DashArray     string
	Opacity       float64
	UseMarkers    bool
	ZeroArcRadius float64
	AxisWidth     float64
	AxisFontSize  float64
	OriginRadius  float64
}

type svgRect struct {
	Name      string
	X         float64
	Y         float64
	Width     float64
	Height    float64
	Transform string
}

type svgPoint struct {
	X float64
	Y float64
}

type svgStep struct {
	ID         int
	Op         string
	X1         float64
	Y1         float64
	X2         float64
	Y2         float64
	Path       string
	AxisXEnd   svgPoint
	AxisYEnd   svgPoint
	AxisXLabel svgPoint
	AxisYLabel svgPoint
}

type svgLandmark struct {
	Name   string
	StepID int
	X      float64
	Y      float64
	LabelX float64
	LabelY float64
	Anchor string
	// LabelDx and LabelDy offset the label in ems, so the offset follows the
	// label's on-screen font size (used on /bike).
	LabelDx string
	LabelDy string
	// LabelPriority orders labels when overlapping ones are hidden; lower
	// numbers win.
	LabelPriority int
}

// Illustrative outline dimensions (not measured hardware geometry) used only
// to draw component silhouettes in the SVG workbench.
const (
	upperCoverOutlineWidthMm   = 46.0
	spacerOutlineWidthMm       = 34.0
	steererClampOutlineWidthMm = 38.0
	stemBodyOutlineWidthMm     = 30.0
	barClampRingRadiusMm       = 15.9 // nominal 31.8 mm clamp diameter; illustrative, not measured
	seatpostClampOutlineHeight = 10.0 // illustrative rail-clamp head thickness
	saddleRailOutlineHeight    = 5.0  // illustrative saddle rail thickness
	turnArcRadiusMm            = 12.0 // illustrative radius for turn-step SVG arcs
	stemAxisLengthMm           = 20.0 // illustrative local +X/+Y frame arrow length on /
	stemAxisLabelOffsetMm      = 6.0  // illustrative label offset past local axis tip on /
	bikeTurnArcRadiusMm        = 40.0 // illustrative radius for turn-step SVG arcs on /bike
	bikeAxisLengthMm           = 60.0 // illustrative local +X/+Y frame arrow length on /bike
	bikeAxisLabelOffsetMm      = 18.0 // illustrative label offset past local axis tip on /bike
)

type stemView struct {
	HtaDeg           pose.Deg
	Setup            assembly.StemSetup
	HeadTubeTopSVG   svgPoint
	BarClamp         pose.PoseSnapshot
	BarClampSVG      svgPoint
	BarClampRadiusMm float64
	ViewBox          string
	Rects            []svgRect
	TraceSteps       svgTraceSteps
	Landmarks        []svgLandmark
	Steps            []pose.Step
}

// NewHandler returns the HTTP handler for the reboot workbench.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleStemWorkbench)
	mux.HandleFunc("/bike", handleBikeWorkbench)
	mux.HandleFunc("/static/workbench.js", handleWorkbenchJS)
	return mux
}

func handleWorkbenchJS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(workbenchJS)
}

func handleStemWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fix := otsoFenrirFixture()
	if err := fix.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defaultFork, ok := fix.resolveForkOption("")
	if !ok {
		http.Error(w, "missing default fork option", http.StatusInternalServerError)
		return
	}
	forkSetup := assembly.ForkSetup{
		Spec:   defaultFork.Spec,
		SagPct: defaultFork.DefaultSagPct,
	}
	frameset, err := assembly.PlaceFrameset(fix.Chart.Frame, defaultFork.DefaultLowerCup, forkSetup, fix.Wheels, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Slice 1 stem-only preview roots a fresh trace at the leveled HeadTubeTop;
	// Slice 3 will trace one unified tree from BB across frameset and cockpits.
	tr := pose.NewTrace()
	steererUp := pose.NewPose(frameset.HeadTubeTop.Origin(), frameset.HeadTubeTop.HeadingDeg(), tr)
	placed, err := assembly.PlaceStem(steererUp, fix.DefaultStem)
	tr.Freeze()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildStemView(frameset.LeveledHeadTubeAngleDeg(), fix.DefaultStem, placed, tr.Steps())

	var buf bytes.Buffer
	if err := stemTemplate.Execute(&buf, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// poseOutline is a component silhouette drawn as a rectangle in a pose's
// local frame: lengthMm along the heading starting at startMm, thicknessMm
// across it, centred on the pose.
type poseOutline struct {
	name        string
	pose        pose.Pose
	startMm     float64
	lengthMm    float64
	thicknessMm float64
}

func (o poseOutline) rect() svgRect {
	return poseRect(o.name, o.pose, o.startMm, o.lengthMm, o.thicknessMm)
}

// corners returns the outline's four corners in SVG coordinates.
func (o poseOutline) corners() []svgPoint {
	// Detach from the trace so measuring the outline records no steps.
	p := pose.NewPose(o.pose.Origin(), o.pose.HeadingDeg(), nil)
	half := o.thicknessMm / 2.0
	pts := make([]svgPoint, 0, 4)
	for _, along := range []float64{o.startMm, o.startMm + o.lengthMm} {
		for _, across := range []float64{-half, half} {
			pts = append(pts, toSVG(p.Translate("", along, across).Origin()))
		}
	}
	return pts
}

func outlineRects(outlines []poseOutline) []svgRect {
	rects := make([]svgRect, 0, len(outlines))
	for _, o := range outlines {
		rects = append(rects, o.rect())
	}
	return rects
}

func stemOutlines(placed assembly.StemPlacement, setup assembly.StemSetup) []poseOutline {
	return []poseOutline{
		{"upper-cover", placed.HeadTubeTop, 0, setup.UpperCoverMm, upperCoverOutlineWidthMm},
		{"spacers", placed.CoverTop, 0, setup.SpacersMm, spacerOutlineWidthMm},
		{"steerer-clamp", placed.SteererClamp, -setup.ClampHeightMm / 2.0, setup.ClampHeightMm, steererClampOutlineWidthMm},
		{"stem-body", placed.BarClamp, -setup.LengthMm, setup.LengthMm, stemBodyOutlineWidthMm},
	}
}

func stemOutlineRects(placed assembly.StemPlacement, setup assembly.StemSetup) []svgRect {
	return outlineRects(stemOutlines(placed, setup))
}

func buildStemView(htaDeg pose.Deg, setup assembly.StemSetup, placed assembly.StemPlacement, steps []pose.Step) stemView {
	rects := stemOutlineRects(placed, setup)

	svgSteps := buildSVGSteps(steps, turnArcRadiusMm, stemAxisLengthMm, stemAxisLabelOffsetMm)

	landmarks := []svgLandmark{
		makeLandmark("HeadTubeTop", placed.HeadTubeTop, -8, 10, "end"),
		makeLandmark("CoverTop", placed.CoverTop, -8, 2, "end"),
		makeLandmark("SpacerTop", placed.SpacerTop, -8, 2, "end"),
		makeLandmark("SteererClamp", placed.SteererClamp, -8, -4, "end"),
		makeLandmark("BarClamp", placed.BarClamp, 8, -18, "start"),
	}

	pts := make([]svgPoint, 0, len(landmarks))
	for _, lm := range landmarks {
		pts = append(pts, svgPoint{X: lm.X, Y: lm.Y})
	}

	return stemView{
		HtaDeg:         htaDeg,
		Setup:          setup,
		HeadTubeTopSVG: toSVG(placed.HeadTubeTop.Origin()),
		BarClamp: pose.PoseSnapshot{
			Origin:     placed.BarClamp.Origin(),
			HeadingDeg: placed.BarClamp.HeadingDeg(),
		},
		BarClampSVG:      toSVG(placed.BarClamp.Origin()),
		BarClampRadiusMm: barClampRingRadiusMm,
		ViewBox:          computeViewBoxFromPoints(pts, 85.0, 85.0, 45.0, 40.0).String(),
		Rects:            rects,
		TraceSteps: svgTraceSteps{
			Steps:         svgSteps,
			LineWidth:     1.8,
			ArcWidth:      1.5,
			UseMarkers:    true,
			ZeroArcRadius: 4.5,
			AxisWidth:     1.5,
			AxisFontSize:  6.5,
			OriginRadius:  3.5,
		},
		Landmarks: landmarks,
		Steps:     steps,
	}
}

func buildSVGSteps(steps []pose.Step, turnRadiusMm, axisLengthMm, axisLabelOffsetMm float64) []svgStep {
	svgSteps := make([]svgStep, 0, len(steps))
	for _, s := range steps {
		from := toSVG(s.From.Origin)
		to := toSVG(s.To.Origin)
		toPose := s.To.Pose()
		step := svgStep{
			ID:         s.ID,
			Op:         s.Op,
			X1:         from.X,
			Y1:         from.Y,
			X2:         to.X,
			Y2:         to.Y,
			AxisXEnd:   toSVG(toPose.Advance("", axisLengthMm).Origin()),
			AxisYEnd:   toSVG(toPose.Translate("", 0, axisLengthMm).Origin()),
			AxisXLabel: toSVG(toPose.Advance("", axisLengthMm+axisLabelOffsetMm).Origin()),
			AxisYLabel: toSVG(toPose.Translate("", 0, axisLengthMm+axisLabelOffsetMm).Origin()),
		}
		if s.Op == "turn" || s.Op == "face" {
			step.Path = turnArcPath(s, turnRadiusMm)
		}
		svgSteps = append(svgSteps, step)
	}
	return svgSteps
}

func turnArcPath(s pose.Step, radiusMm float64) string {
	delta := float64(s.TurnDeg())
	if math.Abs(delta) < 1e-6 {
		return ""
	}
	p1 := toSVG(s.From.Pose().Advance("", radiusMm).Origin())
	p2 := toSVG(s.To.Pose().Advance("", radiusMm).Origin())

	largeArc := 0
	if math.Abs(delta) > 180.0 {
		largeArc = 1
	}
	// Bike +angles rotate CCW; in SVG (Y-down), CCW is sweep-flag=0 and CW is sweep-flag=1.
	sweep := 0
	if delta < 0 {
		sweep = 1
	}
	return fmt.Sprintf("M %.2f %.2f A %.2f %.2f 0 %d %d %.2f %.2f",
		p1.X, p1.Y, radiusMm, radiusMm, largeArc, sweep, p2.X, p2.Y)
}

func poseRect(name string, p pose.Pose, startMm, lengthMm, thicknessMm float64) svgRect {
	o := toSVG(p.Origin())
	return svgRect{
		Name:      name,
		X:         startMm,
		Y:         -thicknessMm / 2.0,
		Width:     lengthMm,
		Height:    thicknessMm,
		Transform: fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)", o.X, o.Y, -float64(p.HeadingDeg())),
	}
}

func makeLandmark(name string, p pose.Pose, dx, dy float64, anchor string) svgLandmark {
	o := toSVG(p.Origin())
	return svgLandmark{
		Name:   name,
		StepID: p.StepID(),
		X:      o.X,
		Y:      o.Y,
		LabelX: o.X + dx,
		LabelY: o.Y + dy,
		Anchor: anchor,
	}
}

// svgViewBox is a numeric SVG viewBox in SVG user units (mm, Y down).
type svgViewBox struct {
	X float64
	Y float64
	W float64
	H float64
}

func (b svgViewBox) String() string {
	return fmt.Sprintf("%.2f %.2f %.2f %.2f", b.X, b.Y, b.W, b.H)
}

func pointBounds(pts []svgPoint) (minX, minY, maxX, maxY float64) {
	minX, maxX = math.Inf(1), math.Inf(-1)
	minY, maxY = math.Inf(1), math.Inf(-1)
	for _, pt := range pts {
		minX = min(minX, pt.X)
		maxX = max(maxX, pt.X)
		minY = min(minY, pt.Y)
		maxY = max(maxY, pt.Y)
	}
	return minX, minY, maxX, maxY
}

func computeViewBoxFromPoints(pts []svgPoint, padLeft, padRight, padTop, padBottom float64) svgViewBox {
	minX, minY, maxX, maxY := pointBounds(pts)
	return svgViewBox{
		X: minX - padLeft,
		Y: minY - padTop,
		W: (maxX - minX) + padLeft + padRight,
		H: (maxY - minY) + padTop + padBottom,
	}
}

// closeUpViewBox frames pts with the aspect ratio of whole, centred on pts.
// padX and padY are the side and top/bottom margins around pts measured in
// whole-view units, so they stay the same size on screen as the box zooms in.
// The result is never larger than whole.
func closeUpViewBox(pts []svgPoint, whole svgViewBox, padX, padY float64) svgViewBox {
	minX, minY, maxX, maxY := pointBounds(pts)
	aspect := whole.W / whole.H
	// With pad = pad{X,Y} * W / whole.W on each side, solve for the smallest W
	// whose padded box contains pts horizontally and vertically.
	padXPerW := 2 * padX / whole.W
	padYPerW := 2 * padY / whole.W
	widthFit := (maxX - minX) / (1 - padXPerW)
	heightFit := aspect * (maxY - minY) / (1 - aspect*padYPerW)
	w := max(widthFit, heightFit)
	if padXPerW >= 1 || aspect*padYPerW >= 1 || !(w > 0) || w >= whole.W {
		return whole
	}
	h := w / aspect
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	return svgViewBox{X: cx - w/2, Y: cy - h/2, W: w, H: h}
}

// formatSVGSize prints a drawing size with at most two decimals and no
// trailing zeros, so unscaled sizes print exactly like their literals ("18",
// "1.5").
func formatSVGSize(v float64) string {
	s := strings.TrimSuffix(strings.TrimRight(strconv.FormatFloat(v, 'f', 2, 64), "0"), ".")
	if s == "" || s == "-0" || s == "-" {
		return "0"
	}
	return s
}

type svgSegment struct {
	Name         string
	X1           float64
	Y1           float64
	X2           float64
	Y2           float64
	Color        string
	Width        float64
	Illustrative bool
}

// diagramView is one of the fixed framings of the /bike diagram.
type diagramView string

const (
	diagramViewBike diagramView = "bike"
	diagramViewSeat diagramView = "seat"
	diagramViewStem diagramView = "stem"
)

var diagramViews = []diagramView{diagramViewBike, diagramViewSeat, diagramViewStem}

func parseDiagramView(raw string) (diagramView, bool) {
	for _, v := range diagramViews {
		if raw == string(v) {
			return v, true
		}
	}
	return diagramViewBike, false
}

// LinkText is the view's link text in the diagram card.
func (v diagramView) LinkText() string {
	switch v {
	case diagramViewBike:
		return "Whole bike"
	case diagramViewSeat:
		return "Seat"
	case diagramViewStem:
		return "Stem"
	default:
		return string(v)
	}
}

// Description names the view in the diagram's accessible label.
func (v diagramView) Description() string {
	switch v {
	case diagramViewBike:
		return "whole bike"
	case diagramViewSeat:
		return "seat close-up"
	case diagramViewStem:
		return "stem close-up"
	default:
		return string(v)
	}
}

type workbenchState struct {
	View          diagramView
	ForkID        string
	ForkA2cMm     float64
	LowerCupMm    float64
	SagPct        float64
	SpacersMm     float64
	StemLengthMm  float64
	StemAngleDeg  pose.Deg
	BarRollDeg    pose.Deg
	PostExtMm     float64
	PostSetbackMm float64
	RailOffsetMm  float64
	RailPitchDeg  pose.Deg
	// BrpTarget is the optional saddle fitting target (query keys brp_x, brp_y).
	HasBrpTarget bool
	BrpTargetX   float64
	BrpTargetY   float64
}

func defaultWorkbenchState(fix bikeFixture) workbenchState {
	defaultFork, _ := fix.resolveForkOption("")
	return workbenchState{
		View:          diagramViewBike,
		ForkID:        defaultFork.ID,
		ForkA2cMm:     defaultFork.Spec.StaticA2cMm,
		LowerCupMm:    defaultFork.DefaultLowerCup.StackMm,
		SagPct:        defaultFork.DefaultSagPct,
		SpacersMm:     fix.DefaultStem.SpacersMm,
		StemLengthMm:  fix.DefaultStem.LengthMm,
		StemAngleDeg:  fix.DefaultStem.AngleDeg,
		BarRollDeg:    fix.DefaultHandlebar.BarRollDeg,
		PostExtMm:     fix.DefaultSeatpost.ExtensionMm,
		PostSetbackMm: fix.DefaultSeatpost.PostSetbackMm,
		RailOffsetMm:  fix.DefaultSaddle.RailOffsetMm,
		RailPitchDeg:  fix.DefaultSaddle.RailPitchDeg,
	}
}

func workbenchStateFromSetup(view diagramView, forkID string, setup assembly.BikeSetup) workbenchState {
	return workbenchState{
		View:          view,
		ForkID:        forkID,
		ForkA2cMm:     setup.Fork.Spec.StaticA2cMm,
		LowerCupMm:    setup.LowerCup.StackMm,
		SagPct:        setup.Fork.SagPct,
		SpacersMm:     setup.Stem.SpacersMm,
		StemLengthMm:  setup.Stem.LengthMm,
		StemAngleDeg:  setup.Stem.AngleDeg,
		BarRollDeg:    setup.Handlebar.BarRollDeg,
		PostExtMm:     setup.Seatpost.ExtensionMm,
		PostSetbackMm: setup.Seatpost.PostSetbackMm,
		RailOffsetMm:  setup.Saddle.RailOffsetMm,
		RailPitchDeg:  setup.Saddle.RailPitchDeg,
	}
}

func (s workbenchState) withCadBaselineFork(fix bikeFixture) workbenchState {
	s.ForkID = cadForkOptionID
	s.ForkA2cMm = fix.Chart.CadFork.A2cMm
	s.LowerCupMm = fix.Chart.CadLowerCup.StackMm
	s.SagPct = 0.0
	return s
}

func (s workbenchState) withView(view diagramView) workbenchState {
	s.View = view
	return s
}

// presetURLs returns the Reset Defaults and CAD Baseline Fork links. Both keep
// view; decodeWorkbenchState resolves an unknown submitted view to
// diagramViewBike so the presets recover a valid build. cadBase is the build
// the CAD Baseline Fork preset applies the chart fork to.
func presetURLs(fix bikeFixture, cadBase workbenchState, view diagramView) (defaultsURL, cadBaselineForkURL string) {
	return defaultWorkbenchState(fix).withView(view).URL(),
		cadBase.withView(view).withCadBaselineFork(fix).URL()
}

func (s workbenchState) URL() string {
	q := url.Values{}
	q.Set("view", string(s.View))
	q.Set("fork", s.ForkID)
	q.Set("fork_a2c", formatFormFloat(s.ForkA2cMm))
	q.Set("cup", formatFormFloat(s.LowerCupMm))
	q.Set("sag", formatFormFloat(s.SagPct))
	q.Set("spacers", formatFormFloat(s.SpacersMm))
	q.Set("stem_length", formatFormFloat(s.StemLengthMm))
	q.Set("stem_angle", formatFormFloat(float64(s.StemAngleDeg)))
	q.Set("bar_roll", formatFormFloat(float64(s.BarRollDeg)))
	q.Set("post_ext", formatFormFloat(s.PostExtMm))
	q.Set("post_setback", formatFormFloat(s.PostSetbackMm))
	q.Set("rail_offset", formatFormFloat(s.RailOffsetMm))
	q.Set("rail_pitch", formatFormFloat(float64(s.RailPitchDeg)))
	if s.HasBrpTarget {
		q.Set("brp_x", formatFormFloat(s.BrpTargetX))
		q.Set("brp_y", formatFormFloat(s.BrpTargetY))
	}
	return "/bike?" + q.Encode()
}

func (s workbenchState) formValues() bikeFormValues {
	var brpX, brpY string
	if s.HasBrpTarget {
		brpX = formatFormFloat(s.BrpTargetX)
		brpY = formatFormFloat(s.BrpTargetY)
	}
	return bikeFormValues{
		BrpX:        brpX,
		BrpY:        brpY,
		View:        string(s.View),
		Fork:        s.ForkID,
		PrevFork:    s.ForkID,
		ForkA2c:     formatFormFloat(s.ForkA2cMm),
		Cup:         formatFormFloat(s.LowerCupMm),
		Sag:         formatFormFloat(s.SagPct),
		Spacers:     formatFormFloat(s.SpacersMm),
		StemLength:  formatFormFloat(s.StemLengthMm),
		StemAngle:   formatFormFloat(float64(s.StemAngleDeg)),
		BarRoll:     formatFormFloat(float64(s.BarRollDeg)),
		PostExt:     formatFormFloat(s.PostExtMm),
		PostSetback: formatFormFloat(s.PostSetbackMm),
		RailOffset:  formatFormFloat(s.RailOffsetMm),
		RailPitch:   formatFormFloat(float64(s.RailPitchDeg)),
	}
}

type bikeFormValues struct {
	View        string
	Fork        string
	PrevFork    string
	ForkA2c     string
	Cup         string
	Sag         string
	Spacers     string
	StemLength  string
	StemAngle   string
	BarRoll     string
	PostExt     string
	PostSetback string
	RailOffset  string
	RailPitch   string
	BrpX        string
	BrpY        string
}

type bikeInputError struct {
	Field string
	Err   error
}

func (e *bikeInputError) Error() string {
	return e.Err.Error()
}

type bikeResolvedInput struct {
	State workbenchState
	Form  bikeFormValues // submitted values, echoed on error pages
	Setup assembly.BikeSetup
}

type bikeView struct {
	HasBuild             bool
	ErrorMessage         string
	InvalidField         string
	Form                 bikeFormValues
	CanonicalURL         string
	DefaultsURL          string
	CadBaselineForkURL   string
	View                 diagramView
	ViewLinks            []diagramViewLink
	DrawScale            float64 // view width / whole-bike width; 1 for the whole bike
	UnitsPerPx           float64 // SVG units per on-screen px at the nominal width; JavaScript measures the real value
	MinPostExtensionMm   float64
	MaxPostExtensionMm   float64
	BikeName             string
	SaddleName           string
	HandlebarName        string
	Forks                []forkOption
	CadForkOption        forkOption
	SelectedForkID       string
	CupLabel             string
	ForkA2cReadOnly      bool
	CupReadOnly          bool
	SagReadOnly          bool
	CadBaselineNote      string
	CadFork              assembly.ForkGeometry
	CadLowerCupMm        float64
	LowerCupMm           float64
	SagPct               float64
	EffectiveA2cMm       float64
	PitchDeg             pose.Deg
	LeveledHtaDeg        pose.Deg
	LeveledStaDeg        pose.Deg
	WheelbaseMm          float64
	BbDropLeveledMm      float64
	SeatAxisToPostTopMm  float64
	SaddleHeightMm       float64
	BrpToGripDistMm      float64
	SitBonesToGripRiseMm float64
	HandLateralMm        float64
	PublishedWheelbaseMm float64
	CadHtaDeg            pose.Deg
	CadWheelbaseMm       float64
	DeltaPitchDeg        pose.Deg
	ResidualAxleYMm      float64
	ResidualWheelbaseMm  float64
	WheelRadiusMm        float64
	WheelLabel           string
	RearAxleSVG          svgPoint
	FrontAxleSVG         svgPoint
	GroundY1             float64
	GroundY2             float64
	GroundX1             float64
	GroundX2             float64
	ViewBox              string
	Rects                []svgRect
	Segments             []svgSegment
	TraceSteps           svgTraceSteps
	Landmarks            []svgLandmark
	Steps                []pose.Step
	Fitting              *bikeFittingView // nil unless a BRP fit target is set
	MinRailOffsetMm      float64
	MaxRailOffsetMm      float64
	ExampleBrpX          string
	ExampleBrpY          string
}

// bikeFittingView is the saddle fitting summary for a submitted build and BRP target.
type bikeFittingView struct {
	TargetX, TargetY   float64
	MissX, MissY       float64 // target − the submitted build's BRP
	MissMm             float64
	ExtensionMm        float64 // fitted
	RailOffsetMm       float64 // fitted
	RailLimitMm        float64
	Reached            bool
	LimitedByExtension bool
	LimitedByRailOff   bool
	ApplyURL           string
}

// OutcomeText describes the fitting outcome as a fact about this build.
func (f bikeFittingView) OutcomeText() string {
	switch {
	case f.Reached:
		return "reached"
	case f.LimitedByExtension && f.LimitedByRailOff:
		return "limited by seatpost extension, saddle rail offset"
	case f.LimitedByExtension:
		return "limited by seatpost extension"
	default:
		return "limited by saddle rail offset"
	}
}

type diagramViewLink struct {
	View    diagramView
	URL     string
	Current bool
}

// Px scales a drawing-only size (label, dot, stroke, axis) by the view's zoom
// so it keeps the same on-screen size in every view.
func (v bikeView) Px(size float64) string {
	return formatSVGSize(size * v.DrawScale)
}

func handleBikeWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bike" {
		http.NotFound(w, r)
		return
	}

	fix := otsoFenrirFixture()
	if err := fix.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()

	resolved, inputErr := decodeWorkbenchState(fix, q)
	if inputErr != nil {
		renderBikeError(w, fix, resolved, inputErr.Field, inputErr.Error())
		return
	}

	tr := pose.NewTrace()
	bike, err := assembly.PlaceBike(resolved.Setup, tr)
	if err != nil {
		renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
		return
	}

	cadBaselineFork, err := assembly.PlaceCadBaselineFork(fix.Chart, fix.Wheels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildBikeView(
		fix,
		resolved.State.View,
		resolved.State.ForkID,
		bike,
		cadBaselineFork,
		tr.Steps(),
	)
	if resolved.State.HasBrpTarget {
		target := assembly.SaddleFitTarget{BRP: pose.Point{X: resolved.State.BrpTargetX, Y: resolved.State.BrpTargetY}}
		fitting, err := assembly.ApplyFitting(resolved.Setup, target, nil)
		if err != nil {
			renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
			return
		}
		view.withBrpTarget(fix, bike.Setup, resolved.State)
		view.Fitting = buildFittingView(resolved.State, bike, fitting)
	}
	renderBikeTemplate(w, http.StatusOK, view)
}

func buildFittingView(state workbenchState, bike assembly.BikePlacement, fitting assembly.Fitting) *bikeFittingView {
	railLimit, _ := bike.Setup.RailOffsetLimitMm() // PlaceBike and ApplyFitting already accepted this setup
	miss := bike.Saddle.BRP.OffsetTo(pose.NewPose(fitting.Target.BRP, 0, nil))
	fv := &bikeFittingView{
		TargetX:      fitting.Target.BRP.X,
		TargetY:      fitting.Target.BRP.Y,
		MissX:        miss.X,
		MissY:        miss.Y,
		MissMm:       miss.Norm(),
		ExtensionMm:  fitting.Placement.Setup.Seatpost.ExtensionMm,
		RailOffsetMm: fitting.Placement.Setup.Saddle.RailOffsetMm,
		RailLimitMm:  railLimit,
		Reached:      fitting.Outcome == assembly.FitReached,
	}
	for _, f := range fitting.LimitedBy {
		switch f {
		case assembly.InputSeatpostExtension:
			fv.LimitedByExtension = true
		case assembly.InputSaddleRailOffset:
			fv.LimitedByRailOff = true
		case assembly.InputForkA2c, assembly.InputForkSag, assembly.InputLowerCup, assembly.InputStemSpacers,
			assembly.InputStemLength, assembly.InputStemAngle, assembly.InputHandlebarRoll,
			assembly.InputSeatpostSetback, assembly.InputSaddleRailPitch, assembly.InputFitTargetBRP:
			// Fitting never limits on these inputs.
		}
	}
	applied := workbenchStateFromSetup(state.View, state.ForkID, fitting.Placement.Setup)
	applied.HasBrpTarget, applied.BrpTargetX, applied.BrpTargetY = true, state.BrpTargetX, state.BrpTargetY
	fv.ApplyURL = applied.URL()
	return fv
}

// railOffsetRange returns the saddle rail offset input bounds ∓L for the
// fixture's saddle and seatpost.
func (f bikeFixture) railOffsetRange() (minMm, maxMm float64) {
	limit, err := assembly.BikeSetup{Saddle: f.DefaultSaddle, Seatpost: f.DefaultSeatpost}.RailOffsetLimitMm()
	if err != nil {
		return 0, 0 // validate() rejects such a fixture before any page renders
	}
	return -limit, limit
}

func renderBikeError(w http.ResponseWriter, fix bikeFixture, resolved bikeResolvedInput, invalidField, msg string) {
	forkA2cRO, cupRO, sagRO, cupLabel, cadNote := forkFieldViewState(fix, resolved.State.ForkID, resolved.Form.Sag)
	defaultsURL, cadURL := presetURLs(fix, defaultWorkbenchState(fix), resolved.State.View)
	minRail, maxRail := fix.railOffsetRange()
	view := bikeView{
		MinRailOffsetMm:    minRail,
		MaxRailOffsetMm:    maxRail,
		ExampleBrpX:        formatFormFloat(fix.ExampleBrpTarget.X),
		ExampleBrpY:        formatFormFloat(fix.ExampleBrpTarget.Y),
		HasBuild:           false,
		ErrorMessage:       msg,
		InvalidField:       invalidField,
		Form:               resolved.Form,
		DefaultsURL:        defaultsURL,
		CadBaselineForkURL: cadURL,
		MinPostExtensionMm: fix.DefaultSeatpost.MinExtensionMm,
		MaxPostExtensionMm: fix.DefaultSeatpost.MaxExtensionMm,
		BikeName:           fix.Name,
		Forks:              fix.Forks,
		CadForkOption:      fix.cadForkOption(),
		SelectedForkID:     resolved.State.ForkID,
		CupLabel:           cupLabel,
		ForkA2cReadOnly:    forkA2cRO,
		CupReadOnly:        cupRO,
		SagReadOnly:        sagRO,
		CadBaselineNote:    cadNote,
		CadFork:            fix.Chart.CadFork,
		CadLowerCupMm:      fix.Chart.CadLowerCup.StackMm,
	}
	renderBikeTemplate(w, http.StatusBadRequest, view)
}

func renderBikeTemplate(w http.ResponseWriter, status int, view bikeView) {
	var buf bytes.Buffer
	if err := bikeTemplate.Execute(&buf, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// bikeQueryParser parses query floats while echoing each submitted (or
// default) value into the form, so an error page shows exactly what was sent.
// It records the first parse error and keeps going so every field is echoed.
type bikeQueryParser struct {
	q   url.Values
	err *bikeInputError
}

func (p *bikeQueryParser) float(key string, fallback float64, label string, echo *string) float64 {
	raw := strings.TrimSpace(p.q.Get(key))
	if raw == "" {
		*echo = formatFormFloat(fallback)
		return fallback
	}
	*echo = raw
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFiniteFloat(v) {
		p.fail(key, fmt.Errorf("%s must be a finite number, got %q", label, raw))
		return fallback
	}
	return v
}

// optionalFloat parses a query float that may be absent. It echoes the raw
// submitted text; present is false when the value is blank or invalid.
func (p *bikeQueryParser) optionalFloat(key, label string, echo *string) (v float64, present bool) {
	raw := strings.TrimSpace(p.q.Get(key))
	*echo = raw
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFiniteFloat(v) {
		p.fail(key, fmt.Errorf("%s must be a finite number, got %q", label, raw))
		return 0, false
	}
	return v, true
}

func (p *bikeQueryParser) fail(field string, err error) {
	if p.err == nil {
		p.err = &bikeInputError{Field: field, Err: err}
	}
}

// decodeWorkbenchState resolves the URL query into a BikeSetup. The returned
// bikeResolvedInput always carries Form and State.ForkID even when parsing
// fails, so the error page can echo the submitted controls.
func decodeWorkbenchState(fix bikeFixture, q url.Values) (bikeResolvedInput, *bikeInputError) {
	p := bikeQueryParser{q: q}
	defaults := defaultWorkbenchState(fix)
	state := defaults
	var form bikeFormValues

	selectedFork, ok := fix.resolveForkOption(q.Get("fork"))
	prevFork := q.Get("prev_fork")
	form.PrevFork = prevFork
	if ok {
		state.ForkID = selectedFork.ID
		form.Fork = selectedFork.ID
		form.PrevFork = selectedFork.ID
	} else {
		state.ForkID = ""
		p.fail("fork", fmt.Errorf("unknown fork option %q", q.Get("fork")))
		// Echo the default fork's values beside the rejected selection.
		selectedFork, _ = fix.resolveForkOption("")
	}

	var lowerCup assembly.LowerCupSetup
	var forkSpec assembly.ForkSpec
	if ok && selectedFork.ID == cadForkOptionID {
		// CAD Baseline Fork locks A2C, lower cup, and sag to FrameChart values
		// in one place, ignoring any submitted overrides.
		lowerCup = fix.Chart.CadLowerCup
		forkSpec = selectedFork.Spec
		state.LowerCupMm = lowerCup.StackMm
		state.ForkA2cMm = forkSpec.StaticA2cMm
		state.SagPct = 0.0
		form.Cup = formatFormFloat(state.LowerCupMm)
		form.ForkA2c = formatFormFloat(state.ForkA2cMm)
		form.Sag = formatFormFloat(state.SagPct)
	} else {
		// When the user switches forks via the form, adopt the newly selected fork's
		// default static A2C and sag instead of carrying over the previous fork's values.
		// When switching between forks with different lower-cup standards (such as the
		// CAD Baseline Fork vs. an EC44 fork), also adopt the new pairing's cup.
		forkSwitched := ok && prevFork != "" && prevFork != selectedFork.ID
		defaultCup := selectedFork.DefaultLowerCup.StackMm
		prevOpt, prevOK := fix.resolveForkOption(prevFork)
		cupSwitched := forkSwitched && (!prevOK || prevOpt.DefaultLowerCup.Standard != selectedFork.DefaultLowerCup.Standard)
		if cupSwitched {
			state.LowerCupMm = defaultCup
			form.Cup = formatFormFloat(state.LowerCupMm)
		} else {
			state.LowerCupMm = p.float("cup", defaultCup, "lower cup", &form.Cup)
		}

		state.ForkA2cMm = selectedFork.Spec.StaticA2cMm
		state.SagPct = selectedFork.DefaultSagPct
		if forkSwitched {
			form.ForkA2c = formatFormFloat(state.ForkA2cMm)
			form.Sag = formatFormFloat(state.SagPct)
		} else {
			state.ForkA2cMm = p.float("fork_a2c", state.ForkA2cMm, "fork static A2C", &form.ForkA2c)
			state.SagPct = p.float("sag", state.SagPct, "fork sag", &form.Sag)
		}

		lowerCup = assembly.LowerCupSetup{
			Standard: selectedFork.DefaultLowerCup.Standard,
			StackMm:  state.LowerCupMm,
		}
		forkSpec = selectedFork.Spec
		forkSpec.StaticA2cMm = state.ForkA2cMm
	}
	forkSetup := assembly.ForkSetup{
		Spec:   forkSpec,
		SagPct: state.SagPct,
	}

	state.SpacersMm = p.float("spacers", defaults.SpacersMm, "spacers", &form.Spacers)
	state.StemLengthMm = p.float("stem_length", defaults.StemLengthMm, "stem length", &form.StemLength)
	state.StemAngleDeg = pose.Deg(p.float("stem_angle", float64(defaults.StemAngleDeg), "stem angle", &form.StemAngle))
	state.BarRollDeg = pose.Deg(p.float("bar_roll", float64(defaults.BarRollDeg), "handlebar roll", &form.BarRoll))
	state.PostExtMm = p.float("post_ext", defaults.PostExtMm, "seatpost extension", &form.PostExt)
	state.PostSetbackMm = p.float("post_setback", defaults.PostSetbackMm, "seatpost setback", &form.PostSetback)
	state.RailOffsetMm = p.float("rail_offset", defaults.RailOffsetMm, "saddle rail offset", &form.RailOffset)
	state.RailPitchDeg = pose.Deg(p.float("rail_pitch", float64(defaults.RailPitchDeg), "saddle rail pitch", &form.RailPitch))

	brpX, hasX := p.optionalFloat("brp_x", "BRP target X", &form.BrpX)
	brpY, hasY := p.optionalFloat("brp_y", "BRP target Y", &form.BrpY)
	switch {
	case hasX && hasY:
		state.HasBrpTarget, state.BrpTargetX, state.BrpTargetY = true, brpX, brpY
	case hasX && form.BrpY == "":
		p.fail("brp_y", fmt.Errorf("BRP target needs both X and Y"))
	case hasY && form.BrpX == "":
		p.fail("brp_x", fmt.Errorf("BRP target needs both X and Y"))
	}

	// An unknown view keeps its raw value in the form echo, but the state (and
	// so both preset links) falls back to the whole bike.
	form.View = string(defaults.View)
	if raw := strings.TrimSpace(q.Get("view")); raw != "" {
		form.View = raw
		view, ok := parseDiagramView(raw)
		if !ok {
			p.fail("view", fmt.Errorf("unknown diagram view %q", raw))
		}
		state.View = view
	}

	stem := fix.DefaultStem
	stem.SpacersMm = state.SpacersMm
	stem.LengthMm = state.StemLengthMm
	stem.AngleDeg = state.StemAngleDeg

	bar := fix.DefaultHandlebar
	bar.BarRollDeg = state.BarRollDeg

	post := fix.DefaultSeatpost
	post.ExtensionMm = state.PostExtMm
	post.PostSetbackMm = state.PostSetbackMm

	saddle := fix.DefaultSaddle
	saddle.RailOffsetMm = state.RailOffsetMm
	saddle.RailPitchDeg = state.RailPitchDeg

	return bikeResolvedInput{
		State: state,
		Form:  form,
		Setup: assembly.BikeSetup{
			Frame:     fix.Chart.Frame,
			LowerCup:  lowerCup,
			Fork:      forkSetup,
			Wheels:    fix.Wheels,
			Seatpost:  post,
			Saddle:    saddle,
			Stem:      stem,
			Handlebar: bar,
		},
	}, p.err
}

func bikeInputField(err error) string {
	var inputErr *assembly.InputError
	if !errors.As(err, &inputErr) {
		return ""
	}
	switch inputErr.Field {
	case assembly.InputForkA2c:
		return "fork_a2c"
	case assembly.InputForkSag:
		return "sag"
	case assembly.InputLowerCup:
		return "cup"
	case assembly.InputStemSpacers:
		return "spacers"
	case assembly.InputStemLength:
		return "stem_length"
	case assembly.InputStemAngle:
		return "stem_angle"
	case assembly.InputHandlebarRoll:
		return "bar_roll"
	case assembly.InputSeatpostExtension:
		return "post_ext"
	case assembly.InputSeatpostSetback:
		return "post_setback"
	case assembly.InputSaddleRailOffset:
		return "rail_offset"
	case assembly.InputSaddleRailPitch:
		return "rail_pitch"
	case assembly.InputFitTargetBRP:
		return "brp_x"
	default:
		return ""
	}
}

func isZeroFormFloat(raw string) bool {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return err == nil && isFiniteFloat(v) && v == 0
}

func forkFieldViewState(fix bikeFixture, selectedForkID, sagValue string) (forkA2cRO, cupRO, sagRO bool, cupLabel, cadNote string) {
	isCad := selectedForkID == cadForkOptionID
	opt, ok := fix.resolveForkOption(selectedForkID)
	if !ok {
		opt, _ = fix.resolveForkOption("")
	}
	forkA2cRO = isCad
	cupRO = isCad
	// On a known rigid fork, keep sag read-only only when the displayed value is
	// already valid zero; if an invalid or non-zero sag was submitted, leave the
	// input editable so the user can reset it to 0 without losing other settings.
	sagRO = isCad || (selectedForkID != "" && opt.Spec.TravelMm == 0 && isZeroFormFloat(sagValue))
	cupLabel = lowerCupLabel(opt.DefaultLowerCup.Standard)
	cadNote = cadBaselineForkNote(fix, selectedForkID)
	return forkA2cRO, cupRO, sagRO, cupLabel, cadNote
}

func lowerCupLabel(std assembly.LowerCupStandard) string {
	switch std {
	case assembly.LowerCupEC34, assembly.LowerCupEC44:
		return fmt.Sprintf("%s lower cup (mm, > 0)", std)
	case assembly.LowerCupZS44, assembly.LowerCupZS56, assembly.LowerCupIS42, assembly.LowerCupIS52:
		return fmt.Sprintf("%s lower cup (mm, ≥ 0)", std)
	case assembly.LowerCupBareHeadTube:
		return "No lower cup (bare head tube)"
	default:
		return "Lower Cup (mm)"
	}
}

func lowerCupStandardName(std assembly.LowerCupStandard) string {
	switch std {
	case assembly.LowerCupEC34, assembly.LowerCupEC44, assembly.LowerCupZS44, assembly.LowerCupZS56, assembly.LowerCupIS42, assembly.LowerCupIS52:
		return string(std)
	case assembly.LowerCupBareHeadTube:
		return "bare head tube"
	default:
		return string(std)
	}
}

func cadBaselineForkNote(fix bikeFixture, selectedForkID string) string {
	if selectedForkID != cadForkOptionID || fix.Chart.CadLowerCup.Standard != assembly.LowerCupBareHeadTube {
		return ""
	}
	defaultOpt, ok := fix.resolveForkOption("")
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"CAD Baseline Fork: the published frame chart omits the lower headset cup (%s, %s mm), and these chart fields are locked. Physical fork pairings default to %s (%s mm) — switch to an installed fork to explore cup or sag changes.",
		lowerCupStandardName(fix.Chart.CadLowerCup.Standard),
		strconv.FormatFloat(fix.Chart.CadLowerCup.StackMm, 'f', -1, 64),
		lowerCupStandardName(defaultOpt.DefaultLowerCup.Standard),
		strconv.FormatFloat(defaultOpt.DefaultLowerCup.StackMm, 'f', -1, 64),
	)
}

func formatFormFloat(v float64) string {
	if v == 0 {
		return "0.0"
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		return s + ".0"
	}
	return s
}

func makeSegment(name string, from, to pose.Pose, color string, width float64, illustrative bool) svgSegment {
	p1 := toSVG(from.Origin())
	p2 := toSVG(to.Origin())
	return svgSegment{
		Name:         name,
		X1:           p1.X,
		Y1:           p1.Y,
		X2:           p2.X,
		Y2:           p2.Y,
		Color:        color,
		Width:        width,
		Illustrative: illustrative,
	}
}

const (
	// nominalDiagramWidthPx is the diagram's typical on-screen width. It
	// converts screen sizes to SVG units before JavaScript measures the real
	// width, and when JavaScript is off.
	nominalDiagramWidthPx = 720.0
	// Margins around the landmarks a close-up view frames, in on-screen px at
	// the nominal width. The side margin fits the longest 1rem label
	// ("SeatpostClamp") beside a point on the edge of the framing.
	closeUpPadXPx = 150.0
	closeUpPadYPx = 40.0
	// bikeLabelEmUnits converts the label offsets below, written in SVG units
	// for the original 18-unit labels, to ems.
	bikeLabelEmUnits = 18.0
)

// bikeLandmarkSpec places a landmark label at an offset from its point
// (written in units of an 18-unit font, rendered in ems). Labels with a lower
// priority number win when overlapping labels are hidden.
type bikeLandmarkSpec struct {
	name     string
	pose     pose.Pose
	dx, dy   float64
	anchor   string
	priority int
}

// Label priorities: frame reference points, then fit points, then the rest.
const (
	labelPriorityFrame = 1
	labelPriorityFit   = 2
	labelPriorityOther = 3
)

// workbenchLinks returns the Reset Defaults and CAD Baseline Fork links and
// the diagram view links for state; the latter two keep its BRP fit target.
func workbenchLinks(fix bikeFixture, state workbenchState) (defaultsURL, cadURL string, viewLinks []diagramViewLink) {
	defaultsURL, cadURL = presetURLs(fix, state, state.View)
	for _, v := range diagramViews {
		viewLinks = append(viewLinks, diagramViewLink{View: v, URL: state.withView(v).URL(), Current: v == state.View})
	}
	return defaultsURL, cadURL, viewLinks
}

// withBrpTarget carries the submitted BRP fit target into the view's form echo,
// canonical URL, and links. The diagram itself still shows the submitted build.
func (v *bikeView) withBrpTarget(fix bikeFixture, setup assembly.BikeSetup, submitted workbenchState) {
	state := workbenchStateFromSetup(submitted.View, submitted.ForkID, setup)
	state.HasBrpTarget, state.BrpTargetX, state.BrpTargetY = true, submitted.BrpTargetX, submitted.BrpTargetY
	v.Form = state.formValues()
	v.CanonicalURL = state.URL()
	v.DefaultsURL, v.CadBaselineForkURL, v.ViewLinks = workbenchLinks(fix, state)
}

func buildBikeView(
	fix bikeFixture,
	view diagramView,
	forkID string,
	bike assembly.BikePlacement,
	cadBaselineFork assembly.ChartBaseline,
	steps []pose.Step,
) bikeView {
	placed := bike.Frameset
	seatpost := bike.Seatpost
	saddle := bike.Saddle
	stem := bike.Stem
	handlebar := bike.Handlebar

	r := bike.Setup.Wheels.RadiusMm
	rearSVG := toSVG(placed.RearAxle.Origin())
	frontSVG := toSVG(placed.FrontAxle.Origin())
	rearContactSVG := toSVG(placed.RearTireContact.Origin())
	frontContactSVG := toSVG(placed.FrontTireContact.Origin())
	pts := []svgPoint{
		{X: rearSVG.X - r, Y: rearSVG.Y - r},
		{X: frontSVG.X + r, Y: frontContactSVG.Y},
		rearContactSVG,
		toSVG(placed.HeadTubeTop.Origin()),
		toSVG(placed.SeatTubeTop.Origin()),
		toSVG(seatpost.SeatpostClamp.Origin()),
		toSVG(saddle.RailCenter.Origin()),
		toSVG(saddle.BRP.Origin()),
		toSVG(saddle.SitBones.Origin()),
		toSVG(stem.SteererClamp.Origin()),
		toSVG(stem.BarClamp.Origin()),
		toSVG(handlebar.HandGrip.Origin()),
	}
	wholeBox := computeViewBoxFromPoints(pts, 95.0, 95.0, 95.0, 100.0)

	stemParts := stemOutlines(stem, bike.Setup.Stem)
	seatParts := []poseOutline{
		{"seatpost-clamp", saddle.RailFrame, -bike.Setup.Seatpost.ClampLengthMm / 2.0, bike.Setup.Seatpost.ClampLengthMm, seatpostClampOutlineHeight},
		{"saddle-rail", saddle.RailCenter, -bike.Setup.Saddle.RailLengthMm / 2.0, bike.Setup.Saddle.RailLengthMm, saddleRailOutlineHeight},
	}

	// A close-up frames its poses and every corner of the outlines drawn
	// around them, so no rendered component is cut off at the edge.
	var framingPoses []pose.Pose
	var framingOutlines []poseOutline
	switch view {
	case diagramViewBike:
	case diagramViewSeat:
		// Every seatpost and saddle pose above SeatTubeTop, not the seat tube.
		framingPoses = []pose.Pose{
			seatpost.ShaftTop, seatpost.SeatpostClamp, saddle.RailFrame,
			saddle.RailCenter, saddle.BRP, saddle.SitBones,
		}
		framingOutlines = seatParts
	case diagramViewStem:
		// Every headset, stem, and handlebar pose above HeadTubeTop, not the
		// head tube. The upper cover outline starts at HeadTubeTop, so its
		// corners keep that end of the stack in view.
		framingPoses = []pose.Pose{
			stem.CoverTop, stem.SpacerTop, stem.SteererClamp, stem.BarClamp,
			handlebar.BarRollFrame, handlebar.HandGrip,
		}
		framingOutlines = stemParts
	}
	viewBox := wholeBox
	if len(framingPoses) > 0 {
		framingPts := make([]svgPoint, 0, len(framingPoses)+4*len(framingOutlines))
		for _, p := range framingPoses {
			framingPts = append(framingPts, toSVG(p.Origin()))
		}
		for _, o := range framingOutlines {
			framingPts = append(framingPts, o.corners()...)
		}
		unitsPerNominalPx := wholeBox.W / nominalDiagramWidthPx
		viewBox = closeUpViewBox(framingPts, wholeBox, closeUpPadXPx*unitsPerNominalPx, closeUpPadYPx*unitsPerNominalPx)
	}
	// Drawing-only sizes (labels, dots, strokes, trace arrows) scale with the
	// view so they keep their on-screen size; physical outlines do not.
	scale := viewBox.W / wholeBox.W

	// seat-tube (straight along effective STA), seatstay (to SeatTubeTop),
	// saddle-profile (SitBones to BRP), and handlebar-rise-sweep (side projection
	// from BarClamp to HandGrip) are illustrative schematic lines rather than
	// measured physical outlines.
	segments := []svgSegment{
		makeSegment("chainstay", placed.RearAxle, placed.BB, "#2f363d", 6*scale, false),
		makeSegment("seat-tube", placed.SeatAxis, placed.SeatTubeTop, "#2f363d", 7*scale, true),
		makeSegment("seatstay", placed.RearAxle, placed.SeatTubeTop, "#2f363d", 4*scale, true),
		makeSegment("head-tube", placed.HeadTubeTop, placed.HeadTubeBottom, "#2f363d", 9*scale, false),
		makeSegment("lower-cup", placed.HeadTubeBottom, placed.LowerCupBottom, "#0969da", 9*scale, false),
		makeSegment("fork-a2c", placed.LowerCupBottom, placed.ForkSteererBase, "#59636e", 7*scale, false),
		makeSegment("fork-rake", placed.ForkSteererBase, placed.FrontAxle, "#59636e", 5*scale, false),
		makeSegment("seatpost", placed.SeatTubeTop, seatpost.ShaftTop, "#59636e", 5*scale, false),
		makeSegment("seatpost-setback", seatpost.ShaftTop, seatpost.SeatpostClamp, "#59636e", 5*scale, false),
		makeSegment("saddle-profile", saddle.SitBones, saddle.BRP, "#2f363d", 4*scale, true),
		makeSegment("handlebar-rise-sweep", handlebar.BarClamp, handlebar.HandGrip, "#2f363d", 5*scale, true),
	}

	rects := append(outlineRects(stemParts), outlineRects(seatParts)...)

	landmarkSpecs := []bikeLandmarkSpec{
		{"RearAxle", placed.RearAxle, 0, -14, "middle", labelPriorityFrame},
		{"RearContact", placed.RearTireContact, 0, 24, "middle", labelPriorityOther},
		{"BB", placed.BB, 0, 26, "middle", labelPriorityFrame},
		{"SeatTubeTop", placed.SeatTubeTop, -14, -10, "end", labelPriorityFrame},
		{"SeatpostClamp", seatpost.SeatpostClamp, 16, 6, "start", labelPriorityOther},
		{"RailCenter", saddle.RailCenter, 16, 24, "start", labelPriorityOther},
		{"BRP", saddle.BRP, 14, -10, "start", labelPriorityFit},
		{"SitBones", saddle.SitBones, -14, -10, "end", labelPriorityFit},
		{"HeadTubeTop", placed.HeadTubeTop, 14, 6, "start", labelPriorityFrame},
		{"CoverTop", stem.CoverTop, -14, 8, "end", labelPriorityOther},
		{"SpacerTop", stem.SpacerTop, -14, -2, "end", labelPriorityOther},
		{"SteererClamp", stem.SteererClamp, -14, -14, "end", labelPriorityOther},
		{"BarClamp", stem.BarClamp, 14, 4, "start", labelPriorityFit},
		{"HandGrip", handlebar.HandGrip, -14, -12, "end", labelPriorityFit},
		{"HeadTubeBottom", placed.HeadTubeBottom, 16, 4, "start", labelPriorityOther},
		{"LowerCupBottom", placed.LowerCupBottom, 16, 18, "start", labelPriorityOther},
		{"FrontAxle", placed.FrontAxle, 0, -14, "middle", labelPriorityFrame},
		{"FrontContact", placed.FrontTireContact, 0, 24, "middle", labelPriorityOther},
	}
	landmarks := make([]svgLandmark, 0, len(landmarkSpecs))
	for _, spec := range landmarkSpecs {
		lm := makeLandmark(spec.name, spec.pose, 0, 0, spec.anchor)
		lm.LabelDx = formatSVGSize(spec.dx/bikeLabelEmUnits) + "em"
		lm.LabelDy = formatSVGSize(spec.dy/bikeLabelEmUnits) + "em"
		lm.LabelPriority = spec.priority
		landmarks = append(landmarks, lm)
	}

	state := workbenchStateFromSetup(view, forkID, bike.Setup)
	form := state.formValues()
	forkA2cRO, cupRO, sagRO, cupLabel, cadNote := forkFieldViewState(fix, forkID, form.Sag)
	defaultsURL, cadURL, viewLinks := workbenchLinks(fix, state)

	minRail, maxRail := fix.railOffsetRange()
	return bikeView{
		MinRailOffsetMm:      minRail,
		MaxRailOffsetMm:      maxRail,
		ExampleBrpX:          formatFormFloat(fix.ExampleBrpTarget.X),
		ExampleBrpY:          formatFormFloat(fix.ExampleBrpTarget.Y),
		HasBuild:             true,
		Form:                 form,
		CanonicalURL:         state.URL(),
		DefaultsURL:          defaultsURL,
		CadBaselineForkURL:   cadURL,
		View:                 view,
		ViewLinks:            viewLinks,
		DrawScale:            scale,
		UnitsPerPx:           viewBox.W / nominalDiagramWidthPx,
		MinPostExtensionMm:   bike.Setup.Seatpost.MinExtensionMm,
		MaxPostExtensionMm:   bike.Setup.Seatpost.MaxExtensionMm,
		BikeName:             fix.Name,
		SaddleName:           fix.SaddleName,
		HandlebarName:        fix.HandlebarName,
		Forks:                fix.Forks,
		CadForkOption:        fix.cadForkOption(),
		SelectedForkID:       forkID,
		CupLabel:             cupLabel,
		ForkA2cReadOnly:      forkA2cRO,
		CupReadOnly:          cupRO,
		SagReadOnly:          sagRO,
		CadBaselineNote:      cadNote,
		CadFork:              fix.Chart.CadFork,
		CadLowerCupMm:        fix.Chart.CadLowerCup.StackMm,
		LowerCupMm:           bike.Setup.LowerCup.StackMm,
		SagPct:               bike.Setup.Fork.SagPct,
		EffectiveA2cMm:       bike.Setup.Fork.EffectiveA2cMm(),
		PitchDeg:             placed.PitchDeg(),
		LeveledHtaDeg:        placed.LeveledHeadTubeAngleDeg(),
		LeveledStaDeg:        placed.LeveledSeatTubeAngleDeg(),
		WheelbaseMm:          placed.WheelbaseMm(),
		BbDropLeveledMm:      placed.LeveledBbDropMm(),
		SeatAxisToPostTopMm:  bike.SeatAxisToPostTopMm(),
		SaddleHeightMm:       bike.SaddleHeightMm(),
		BrpToGripDistMm:      bike.BrpToGripDistMm(),
		SitBonesToGripRiseMm: bike.SitBonesToGripRiseMm(),
		HandLateralMm:        handlebar.HandLateralMm,
		PublishedWheelbaseMm: fix.Chart.PublishedWheelbaseMm,
		CadHtaDeg:            cadBaselineFork.Placement.LeveledHeadTubeAngleDeg(),
		CadWheelbaseMm:       cadBaselineFork.Placement.WheelbaseMm(),
		DeltaPitchDeg:        cadBaselineFork.DeltaPitchDeg(placed),
		ResidualAxleYMm:      cadBaselineFork.Residuals.AxleHeightDiffMm,
		ResidualWheelbaseMm:  cadBaselineFork.Residuals.WheelbaseDiffMm,
		WheelRadiusMm:        r,
		WheelLabel:           fix.WheelLabel,
		RearAxleSVG:          rearSVG,
		FrontAxleSVG:         frontSVG,
		GroundY1:             rearContactSVG.Y,
		GroundY2:             frontContactSVG.Y,
		GroundX1:             rearContactSVG.X - r - 40,
		GroundX2:             frontContactSVG.X + r + 40,
		ViewBox:              viewBox.String(),
		Rects:                rects,
		Segments:             segments,
		TraceSteps: svgTraceSteps{
			Steps:         buildSVGSteps(steps, bikeTurnArcRadiusMm*scale, bikeAxisLengthMm*scale, bikeAxisLabelOffsetMm*scale),
			LineWidth:     2.0 * scale,
			ArcWidth:      2.5 * scale,
			DashArray:     formatSVGSize(6*scale) + " " + formatSVGSize(4*scale),
			Opacity:       0.7,
			ZeroArcRadius: 12.0 * scale,
			AxisWidth:     3.5 * scale,
			AxisFontSize:  18.0 * scale,
			OriginRadius:  8.0 * scale,
		},
		Landmarks: landmarks,
		Steps:     steps,
	}
}
