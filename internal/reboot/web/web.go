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
	"slices"
	"strconv"
	"strings"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/catalog"
	"bikegeo/internal/reboot/pose"
)

//go:embed templates/bike.html
var templateFS embed.FS

var bikeTemplate = template.Must(template.ParseFS(templateFS, "templates/bike.html"))

func toSVG(p pose.Point) svgPoint {
	return svgPoint{X: p.X, Y: -p.Y}
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

type svgLandmark struct {
	Name   string
	X      float64
	Y      float64
	LabelX float64
	LabelY float64
	Anchor string
	// LabelDx and LabelDy offset the label in ems, so the offset follows the
	// label's on-screen font size (used on /bike).
	LabelDx string
	LabelDy string
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
)

// NewHandler returns the HTTP handler for the reboot workbench. Chart closure
// runs in the catalog's go test, not at startup (Q91).
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	// The root redirects to the inspector until the comparison page lands.
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/bike", http.StatusFound)
	})
	mux.HandleFunc("/bike", handleBikeWorkbench)
	return mux
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
	half := o.thicknessMm / 2.0
	pts := make([]svgPoint, 0, 4)
	for _, along := range []float64{o.startMm, o.startMm + o.lengthMm} {
		for _, across := range []float64{-half, half} {
			pts = append(pts, toSVG(o.pose.Translate(along, across).Origin()))
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
		{"upper-cover", placed.HeadTubeTop, 0, setup.UpperStackMm, upperCoverOutlineWidthMm},
		{"spacers", placed.CoverTop, 0, setup.SpacersMm, spacerOutlineWidthMm},
		{"steerer-clamp", placed.SteererClamp, -setup.ClampHeightMm / 2.0, setup.ClampHeightMm, steererClampOutlineWidthMm},
		{"stem-body", placed.BarClamp, -setup.LengthMm, setup.LengthMm, stemBodyOutlineWidthMm},
	}
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
	View            diagramView
	ForkID          string
	BarID           string
	PostID          string
	ForkA2cMm       float64
	LowerCupMm      float64
	SagPct          float64
	SpacersMm       float64
	StemLengthMm    float64
	StemAngleDeg    pose.Deg
	BarRollDeg      pose.Deg
	PostInsertionMm float64
	RailOffsetMm    float64
	RailPitchDeg    pose.Deg
	Targets         fitTargetState
	AllStockBars    bool // stock fitting offers every fixture handlebar (Q22)
	AllStockForks   bool // stock fitting offers every fixture fork configuration (Q33)
}

// fitTargetState is the optional fitting targets in the URL query
// (brp_x/brp_y and grip_x/grip_y). Both blank in a pair means no target.
type fitTargetState struct {
	HasBrp       bool
	BrpX, BrpY   float64
	HasGrip      bool
	GripX, GripY float64
}

func (t fitTargetState) any() bool { return t.HasBrp || t.HasGrip }

func (t fitTargetState) fitTarget() assembly.FitTarget {
	var target assembly.FitTarget
	if t.HasBrp {
		target.BRP = &pose.Point{X: t.BrpX, Y: t.BrpY}
	}
	if t.HasGrip {
		target.HandGrip = &pose.Point{X: t.GripX, Y: t.GripY}
	}
	return target
}

func defaultWorkbenchState(fix bikeFixture) workbenchState {
	defaultFork, _ := fix.resolveForkOption("")
	defaultBar, _ := fix.resolveHandlebar("")
	defaultPost, _ := fix.resolveSeatpost("")
	return workbenchState{
		View:            diagramViewBike,
		ForkID:          defaultFork.ID,
		BarID:           defaultBar.ID,
		PostID:          defaultPost.ID,
		ForkA2cMm:       defaultFork.Spec.StaticA2cMm,
		LowerCupMm:      defaultFork.DefaultLowerCup.StackMm,
		SagPct:          defaultFork.DefaultSagPct,
		SpacersMm:       fix.DefaultStem.SpacersMm,
		StemLengthMm:    fix.DefaultStem.LengthMm,
		StemAngleDeg:    fix.DefaultStem.AngleDeg,
		BarRollDeg:      defaultBar.Setup.BarRollDeg,
		PostInsertionMm: fix.DefaultPostInsertionMm,
		RailOffsetMm:    fix.DefaultSaddle.RailOffsetMm,
		RailPitchDeg:    fix.DefaultSaddle.RailPitchDeg,
	}
}

func workbenchStateFromSetup(view diagramView, forkID, barID, postID string, setup assembly.BikeSetup) workbenchState {
	return workbenchState{
		View:            view,
		ForkID:          forkID,
		BarID:           barID,
		PostID:          postID,
		ForkA2cMm:       setup.Fork.Spec.StaticA2cMm,
		LowerCupMm:      setup.LowerCup.StackMm,
		SagPct:          setup.Fork.SagPct,
		SpacersMm:       setup.Stem.SpacersMm,
		StemLengthMm:    setup.Stem.LengthMm,
		StemAngleDeg:    setup.Stem.AngleDeg,
		BarRollDeg:      setup.Handlebar.BarRollDeg,
		PostInsertionMm: setup.Seatpost.InsertionMm,
		RailOffsetMm:    setup.Saddle.RailOffsetMm,
		RailPitchDeg:    setup.Saddle.RailPitchDeg,
	}
}

// withSetup returns s with every setup-derived field replaced by setup's,
// keeping the submitted view, fork, bar, post, fit targets, and stock bar and fork options.
func (s workbenchState) withSetup(setup assembly.BikeSetup) workbenchState {
	next := workbenchStateFromSetup(s.View, s.ForkID, s.BarID, s.PostID, setup)
	next.Targets = s.Targets
	next.AllStockBars = s.AllStockBars
	next.AllStockForks = s.AllStockForks
	return next
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
	q.Set("bar", s.BarID)
	q.Set("post", s.PostID)
	q.Set("fork_a2c", formatFormFloat(s.ForkA2cMm))
	q.Set("cup", formatFormFloat(s.LowerCupMm))
	q.Set("sag", formatFormFloat(s.SagPct))
	q.Set("spacers", formatFormFloat(s.SpacersMm))
	q.Set("stem_length", formatFormFloat(s.StemLengthMm))
	q.Set("stem_angle", formatFormFloat(float64(s.StemAngleDeg)))
	q.Set("bar_roll", formatFormFloat(float64(s.BarRollDeg)))
	q.Set("post_insert", formatFormFloat(s.PostInsertionMm))
	q.Set("rail_offset", formatFormFloat(s.RailOffsetMm))
	q.Set("rail_pitch", formatFormFloat(float64(s.RailPitchDeg)))
	if s.Targets.HasBrp {
		q.Set("brp_x", formatFormFloat(s.Targets.BrpX))
		q.Set("brp_y", formatFormFloat(s.Targets.BrpY))
	}
	if s.Targets.HasGrip {
		q.Set("grip_x", formatFormFloat(s.Targets.GripX))
		q.Set("grip_y", formatFormFloat(s.Targets.GripY))
	}
	if s.AllStockBars {
		q.Set("stock_bars", stockBarsAll)
	}
	if s.AllStockForks {
		q.Set("stock_forks", stockForksAll)
	}
	return "/bike?" + q.Encode()
}

func (s workbenchState) formValues() bikeFormValues {
	var brpX, brpY, gripX, gripY string
	if s.Targets.HasBrp {
		brpX = formatFormFloat(s.Targets.BrpX)
		brpY = formatFormFloat(s.Targets.BrpY)
	}
	if s.Targets.HasGrip {
		gripX = formatFormFloat(s.Targets.GripX)
		gripY = formatFormFloat(s.Targets.GripY)
	}
	stockBars := stockBarsSelected
	if s.AllStockBars {
		stockBars = stockBarsAll
	}
	stockForks := stockForksSelected
	if s.AllStockForks {
		stockForks = stockForksAll
	}
	return bikeFormValues{
		BrpX:       brpX,
		BrpY:       brpY,
		GripX:      gripX,
		GripY:      gripY,
		StockBars:  stockBars,
		StockForks: stockForks,
		View:       string(s.View),
		Fork:       s.ForkID,
		Bar:        s.BarID,
		Post:       s.PostID,
		PrevFork:   s.ForkID,
		ForkA2c:    formatFormFloat(s.ForkA2cMm),
		Cup:        formatFormFloat(s.LowerCupMm),
		Sag:        formatFormFloat(s.SagPct),
		Spacers:    formatFormFloat(s.SpacersMm),
		StemLength: formatFormFloat(s.StemLengthMm),
		StemAngle:  formatFormFloat(float64(s.StemAngleDeg)),
		BarRoll:    formatFormFloat(float64(s.BarRollDeg)),
		PostInsert: formatFormFloat(s.PostInsertionMm),
		RailOffset: formatFormFloat(s.RailOffsetMm),
		RailPitch:  formatFormFloat(float64(s.RailPitchDeg)),
	}
}

type bikeFormValues struct {
	View       string
	Fork       string
	Bar        string
	Post       string
	PrevFork   string
	ForkA2c    string
	Cup        string
	Sag        string
	Spacers    string
	StemLength string
	StemAngle  string
	BarRoll    string
	PostInsert string
	RailOffset string
	RailPitch  string
	BrpX       string
	BrpY       string
	GripX      string
	GripY      string
	StockBars  string
	StockForks string
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
	DefaultsURL          string
	CadBaselineForkURL   string
	View                 diagramView
	ViewLinks            []diagramViewLink
	DrawScale            float64 // view width / whole-bike width; 1 for the whole bike
	UnitsPerPx           float64 // SVG units per on-screen px at the nominal width
	MinPostInsertionMm   float64
	MaxPostInsertionMm   float64
	PostTotalLengthMm    float64
	PostExposedMm        float64
	BikeName             string
	SaddleName           string
	HandlebarName        string
	Forks                []forkOption
	StockForkCount       int
	CadForkOption        forkOption
	SelectedForkID       string
	Handlebars           []catalog.Handlebar
	SelectedBarID        string
	Seatposts            []catalog.Seatpost
	SelectedPostID       string
	PostSetbackMm        float64
	PostClampLengthMm    float64
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
	Landmarks            []svgLandmark
	Fitting              *bikeFittingView // nil unless a fit target is set
	MinRailOffsetMm      float64
	MaxRailOffsetMm      float64
	ExampleBrpX          string
	ExampleBrpY          string
	ExampleGripX         string
	ExampleGripY         string
	MaxSpacersMm         float64
}

// bikeFittingView is the fitting summary for a submitted build and its targets.
type bikeFittingView struct {
	Saddle   *saddleFittingView  // nil unless a BRP target is set
	Cockpit  *cockpitFittingView // nil unless a HandGrip target is set
	Stock    *stockFittingView   // nil unless a HandGrip target is set
	ApplyURL string              // the fitted build, with the submitted targets kept
}

type stockFittingView struct {
	AllForks       bool
	PartsText      string
	Evaluated      int
	PreferenceNote string
	Rows           []stockCandidateView
}

type stockCandidateView struct {
	BarName                 string
	ForkName                string
	HasBrp                  bool
	BrpMissMm               float64
	Rank                    int
	StemLengthMm, SpacersMm float64
	StemAngleDeg            pose.Deg
	GripMissMm, GripR       float64
	Penalties               []assembly.Penalty // assembly's penalty order
	Score                   float64
	ApplyURL                string // the candidate build, with the submitted targets kept
}

// postSetbackView is one setback note (Q29): the setback and extension
// FitPostSetback found with the rails held at RailOffsetMm. It is never applied.
type postSetbackView struct {
	RailOffsetMm, SetbackMm, InsertionMm, MissMm float64
	LimitedBy                                    []assembly.InputField
}

// OutcomeText describes the setback fitting outcome.
func (v postSetbackView) OutcomeText() string { return fitOutcomeText(v.LimitedBy) }

func makePostSetbackView(f assembly.Fitting) postSetbackView {
	miss, _ := f.BrpMiss()
	return postSetbackView{
		RailOffsetMm: f.Placement.Setup.Saddle.RailOffsetMm,
		SetbackMm:    f.Placement.Setup.Seatpost.Spec.SetbackMm,
		InsertionMm:  f.Placement.Setup.Seatpost.InsertionMm,
		MissMm:       miss.Norm(),
		LimitedBy:    f.LimitedBy,
	}
}

type saddleFittingView struct {
	TargetX, TargetY, MissX, MissY, MissMm float64 // miss: target − the submitted build's BRP
	InsertionMm, RailOffsetMm, RailLimitMm float64 // fitted values; ±RailLimitMm range
	LimitedBy                              []assembly.InputField
	Centered                               postSetbackView  // rails centered (Q29)
	Reach                                  *postSetbackView // rails held at the fitted offset; nil unless saddle fitting is limited
}

type cockpitFittingView struct {
	TargetX, TargetY, MissX, MissY, MissMm         float64 // miss: target − the submitted build's HandGrip
	SpacersMm, MaxSpacersMm                        float64 // fitted; range 0..MaxSpacersMm
	StemLengthMm, MinStemLengthMm, MaxStemLengthMm float64
	StemAngleDeg, BarRollDeg                       pose.Deg // fixed inputs the fit used
	LimitedBy                                      []assembly.InputField
}

// OutcomeText describes the saddle fitting outcome as a fact about this build.
func (f saddleFittingView) OutcomeText() string { return fitOutcomeText(f.LimitedBy) }

// OutcomeText describes the cockpit fitting outcome as a fact about this build.
func (f cockpitFittingView) OutcomeText() string { return fitOutcomeText(f.LimitedBy) }

func fitOutcomeText(limits []assembly.InputField) string {
	if len(limits) == 0 {
		return "reached"
	}
	names := make([]string, len(limits))
	for i, f := range limits {
		names[i] = string(f)
	}
	return "limited by " + strings.Join(names, ", ")
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

// stockCandidateLimit is the number of ranked builds shown in the workbench (Q18).
const stockCandidateLimit = 5

// stock_bars values (Q22).
const (
	stockBarsSelected = "selected"
	stockBarsAll      = "all"
)

// stock_forks values (Q33).
const (
	stockForksSelected = "selected"
	stockForksAll      = "all"
)

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

	bike, err := assembly.PlaceBike(resolved.Setup)
	if err != nil {
		renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
		return
	}

	cadBaselineFork, err := assembly.PlaceCadBaselineFork(fix.Chart, fix.Wheels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildBikeView(fix, resolved.State, bike, cadBaselineFork)
	if resolved.State.Targets.any() {
		target := resolved.State.Targets.fitTarget()
		fitting, err := assembly.ApplyFitting(resolved.Setup, target)
		if err != nil {
			renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
			return
		}
		var centered, reach *assembly.Fitting
		if brp := target.BRP; brp != nil {
			c, err := assembly.FitPostSetback(resolved.Setup, *brp, 0)
			if err != nil {
				renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
				return
			}
			centered = &c
			saddleLimited := slices.Contains(fitting.LimitedBy, assembly.InputSeatpostInsertion) ||
				slices.Contains(fitting.LimitedBy, assembly.InputSaddleRailOffset)
			if saddleLimited {
				r, err := assembly.FitPostSetback(resolved.Setup, *brp, fitting.Placement.Setup.Saddle.RailOffsetMm)
				if err != nil {
					renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
					return
				}
				reach = &r
			}
		}
		view.Fitting = buildFittingView(resolved.State, bike, fitting, centered, reach)
		if resolved.State.Targets.HasGrip {
			// offered is the bar list stock fitting enumerates; its indices
			// name each candidate's bar.
			var offered []catalog.Handlebar
			if resolved.State.AllStockBars {
				offered = fix.Handlebars
			}
			var offeredForks []stockForkOption
			if resolved.State.AllStockForks {
				offeredForks = fix.stockForks()
			}
			stock := fix.CockpitStock
			stock.Handlebars = handlebarSetups(offered)
			stock.Forks = stockForkSetups(offeredForks)
			result, err := assembly.ApplyStockFitting(resolved.Setup, target, stock, fix.CockpitPreferences, stockCandidateLimit)
			if err != nil {
				renderBikeError(w, fix, resolved, bikeInputField(err), err.Error())
				return
			}
			view.Fitting.Stock, err = buildStockFittingView(resolved.State, fix, offered, offeredForks, result)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	// A half-entered target is not applied, but the box keeps what was typed.
	if !resolved.State.Targets.HasBrp {
		view.Form.BrpX, view.Form.BrpY = resolved.Form.BrpX, resolved.Form.BrpY
	}
	if !resolved.State.Targets.HasGrip {
		view.Form.GripX, view.Form.GripY = resolved.Form.GripX, resolved.Form.GripY
	}
	renderBikeTemplate(w, http.StatusOK, view)
}

func buildFittingView(state workbenchState, bike assembly.BikePlacement, fitting assembly.Fitting, centered, reach *assembly.Fitting) *bikeFittingView {
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
			Centered:     makePostSetbackView(*centered),
		}
		if reach != nil {
			r := makePostSetbackView(*reach)
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
	}
	fv.ApplyURL = state.withSetup(fitted).URL()
	return fv
}

// buildStockFittingView renders stock fitting results. The offered bars and
// forks are in the same order as CockpitStock.Handlebars and CockpitStock.Forks.
func buildStockFittingView(state workbenchState, fix bikeFixture, offered []catalog.Handlebar, offeredForks []stockForkOption, stock assembly.StockFitting) (*stockFittingView, error) {
	prefs := fix.CockpitPreferences
	o := prefs.GripOval
	view := &stockFittingView{
		AllForks:  state.AllStockForks,
		PartsText: stockPartsText(state.AllStockForks, state.AllStockBars),
		Evaluated: stock.Evaluated,
		PreferenceNote: fmt.Sprintf("Free: HandGrip inside the ±%.0f × ±%.0f mm oval, stem length %s, stem angle %s, spacers %s, bar sweep %s, fork sag %s (suspension forks), BRP miss %s. Penalties are illustrative except bar sweep and fork sag (Q30, Q32).",
			o.SemiMinorMm, o.SemiMajorMm, freeRangeText(prefs.StemLength, " mm"), freeRangeText(prefs.StemAngle, "°"), freeRangeText(prefs.Spacers, " mm"), freeRangeText(prefs.BarSweep, "°"), freeRangeText(prefs.ForkSag, "%"), freeRangeText(prefs.BrpMiss, " mm")),
		Rows: make([]stockCandidateView, 0, len(stock.Candidates)),
	}
	submittedBar, _ := fix.resolveHandlebar(state.BarID)
	submittedFork, _ := fix.resolveForkOption(state.ForkID)
	for i, c := range stock.Candidates {
		stem := c.Placement.Setup.Stem
		applied := state.withSetup(c.Placement.Setup)
		bar := submittedBar
		if c.StockHandlebar {
			if c.HandlebarIndex < 0 || c.HandlebarIndex >= len(offered) {
				return nil, fmt.Errorf("stock candidate %d names handlebar %d of %d offered", i+1, c.HandlebarIndex, len(offered))
			}
			bar = offered[c.HandlebarIndex]
			applied.BarID = bar.ID
		}
		fork := submittedFork
		if c.StockFork {
			if c.ForkIndex < 0 || c.ForkIndex >= len(offeredForks) {
				return nil, fmt.Errorf("stock candidate %d names fork %d of %d offered", i+1, c.ForkIndex, len(offeredForks))
			}
			fork = offeredForks[c.ForkIndex].Option
			applied.ForkID = fork.ID
		}
		row := stockCandidateView{
			Rank:         i + 1,
			BarName:      bar.Name,
			ForkName:     forkCellName(fork, c.Placement.Setup.Fork.SagPct),
			StemLengthMm: stem.LengthMm,
			SpacersMm:    stem.SpacersMm,
			StemAngleDeg: stem.AngleDeg,
			GripMissMm:   c.GripMiss().Norm(),
			GripR:        c.GripR,
			Penalties:    c.Penalties,
			Score:        c.Score(),
			ApplyURL:     applied.URL(),
		}
		if miss, ok := c.BrpMiss(); ok {
			row.HasBrp = true
			row.BrpMissMm = miss.Norm()
		}
		view.Rows = append(view.Rows, row)
	}
	return view, nil
}

// stockPartsText names the parts enumerated by stock fitting.
func stockPartsText(allForks, allBars bool) string {
	switch {
	case allForks && allBars:
		return "fork, handlebar, stem, and spacer"
	case allForks:
		return "fork, stem, and spacer"
	case allBars:
		return "handlebar, stem, and spacer"
	default:
		return "stem and spacer"
	}
}

func freeRangeText(b assembly.PenaltyBand, unit string) string {
	if b.FreeMin == b.FreeMax {
		return fmt.Sprintf("%.0f%s", b.FreeMin, unit)
	}
	return fmt.Sprintf("%.0f–%.0f%s", b.FreeMin, b.FreeMax, unit)
}

// railOffsetRange returns the saddle rail offset input bounds ∓L for saddle
// and post.
func railOffsetRange(saddle assembly.SaddleSetup, post assembly.SeatpostSetup) (minMm, maxMm float64) {
	limit, err := assembly.BikeSetup{Saddle: saddle, Seatpost: post}.RailOffsetLimitMm()
	if err != nil {
		return 0, 0 // validate() rejects such a fixture before any page renders
	}
	return -limit, limit
}

// postInsertionBounds returns the insertion interval the frame and post allow,
// the one calculation behind placement, fitting, and the form bounds. It is
// zero when the pair allows no insertion; placement then reports the error.
func postInsertionBounds(frame assembly.FrameSetup, spec assembly.SeatpostSpec) assembly.InsertionBounds {
	bounds, err := assembly.SeatpostInsertionBounds(frame, spec)
	if err != nil {
		return assembly.InsertionBounds{}
	}
	return bounds
}

func renderBikeError(w http.ResponseWriter, fix bikeFixture, resolved bikeResolvedInput, invalidField, msg string) {
	forkA2cRO, cupRO, sagRO, cupLabel, cadNote := forkFieldViewState(fix, resolved.State.ForkID, resolved.Form.Sag)
	cadBase := defaultWorkbenchState(fix)
	if resolved.State.BarID != "" {
		cadBase.BarID = resolved.State.BarID
	}
	if resolved.State.PostID != "" {
		cadBase.PostID = resolved.State.PostID
	}
	cadBase.AllStockBars = resolved.State.AllStockBars
	cadBase.AllStockForks = resolved.State.AllStockForks
	defaultsURL, cadURL := presetURLs(fix, cadBase, resolved.State.View)
	minRail, maxRail := railOffsetRange(resolved.Setup.Saddle, resolved.Setup.Seatpost)
	postBounds := postInsertionBounds(resolved.Setup.Frame, resolved.Setup.Seatpost.Spec)
	view := bikeView{
		MinRailOffsetMm:    minRail,
		MaxRailOffsetMm:    maxRail,
		ExampleBrpX:        formatFormFloat(fix.ExampleBrpTarget.X),
		ExampleBrpY:        formatFormFloat(fix.ExampleBrpTarget.Y),
		ExampleGripX:       formatFormFloat(fix.ExampleGripTarget.X),
		ExampleGripY:       formatFormFloat(fix.ExampleGripTarget.Y),
		MaxSpacersMm:       fix.DefaultStem.MaxSpacersMm,
		HasBuild:           false,
		ErrorMessage:       msg,
		InvalidField:       invalidField,
		Form:               resolved.Form,
		DefaultsURL:        defaultsURL,
		CadBaselineForkURL: cadURL,
		MinPostInsertionMm: postBounds.EffectiveMinInsertionMm,
		MaxPostInsertionMm: postBounds.MaxAllowedInsertionMm,
		PostTotalLengthMm:  resolved.Setup.Seatpost.Spec.TotalLengthMm,
		PostSetbackMm:      resolved.Setup.Seatpost.Spec.SetbackMm,
		PostClampLengthMm:  resolved.Setup.Seatpost.Spec.RailClampLengthMm,
		BikeName:           fix.Name,
		Forks:              fix.Forks,
		StockForkCount:     len(fix.stockForks()),
		CadForkOption:      fix.cadForkOption(),
		SelectedForkID:     resolved.State.ForkID,
		Handlebars:         fix.Handlebars,
		SelectedBarID:      resolved.State.BarID,
		Seatposts:          fix.Seatposts,
		SelectedPostID:     resolved.State.PostID,
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

// optionalPoint parses an optional X/Y pair. Both blank → (zero, false).
// Exactly one filled → also (zero, false): the target is not complete yet, so
// it is quietly not applied. The raw echoes keep what was typed.
func (p *bikeQueryParser) optionalPoint(xKey, yKey, label string, echoX, echoY *string) (pose.Point, bool) {
	x, hasX := p.optionalFloat(xKey, label+" X", echoX)
	y, hasY := p.optionalFloat(yKey, label+" Y", echoY)
	if hasX && hasY {
		return pose.Point{X: x, Y: y}, true
	}
	return pose.Point{}, false
}

func (p *bikeQueryParser) fail(field string, err error) {
	if p.err == nil {
		p.err = &bikeInputError{Field: field, Err: err}
	}
}

// decodeWorkbenchState resolves the URL query into a BikeSetup. The returned
// bikeResolvedInput carries the form values and resolved selections even when
// parsing fails, so the error page can echo the submitted controls.
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

	selectedBar, barOK := fix.resolveHandlebar(q.Get("bar"))
	if barOK {
		state.BarID = selectedBar.ID
		form.Bar = selectedBar.ID
	} else {
		state.BarID = ""
		p.fail("bar", fmt.Errorf("unknown handlebar %q", q.Get("bar")))
		// Build with the default bar so the error page can echo the other controls.
		selectedBar, _ = fix.resolveHandlebar("")
	}

	selectedPost, postOK := fix.resolveSeatpost(q.Get("post"))
	if postOK {
		state.PostID = selectedPost.ID
		form.Post = selectedPost.ID
	} else {
		state.PostID = ""
		p.fail("post", fmt.Errorf("unknown seatpost %q", q.Get("post")))
		// Build with the default post so the error page can echo the other controls.
		selectedPost, _ = fix.resolveSeatpost("")
	}

	state.SpacersMm = p.float("spacers", defaults.SpacersMm, "spacers", &form.Spacers)
	state.StemLengthMm = p.float("stem_length", defaults.StemLengthMm, "stem length", &form.StemLength)
	state.StemAngleDeg = pose.Deg(p.float("stem_angle", float64(defaults.StemAngleDeg), "stem angle", &form.StemAngle))
	state.BarRollDeg = pose.Deg(p.float("bar_roll", float64(defaults.BarRollDeg), "handlebar roll", &form.BarRoll))
	state.PostInsertionMm = p.float("post_insert", defaults.PostInsertionMm, "seatpost insertion", &form.PostInsert)
	state.RailOffsetMm = p.float("rail_offset", defaults.RailOffsetMm, "saddle rail offset", &form.RailOffset)
	state.RailPitchDeg = pose.Deg(p.float("rail_pitch", float64(defaults.RailPitchDeg), "saddle rail pitch", &form.RailPitch))

	if brp, ok := p.optionalPoint("brp_x", "brp_y", "BRP target", &form.BrpX, &form.BrpY); ok {
		state.Targets.HasBrp, state.Targets.BrpX, state.Targets.BrpY = true, brp.X, brp.Y
	}
	if grip, ok := p.optionalPoint("grip_x", "grip_y", "HandGrip target", &form.GripX, &form.GripY); ok {
		state.Targets.HasGrip, state.Targets.GripX, state.Targets.GripY = true, grip.X, grip.Y
	}

	form.StockBars = stockBarsSelected
	switch raw := strings.TrimSpace(q.Get("stock_bars")); raw {
	case "", stockBarsSelected:
	case stockBarsAll:
		state.AllStockBars = true
		form.StockBars = stockBarsAll
	default:
		form.StockBars = raw
		p.fail("stock_bars", fmt.Errorf("unknown stock fitting handlebar option %q", raw))
	}
	form.StockForks = stockForksSelected
	switch raw := strings.TrimSpace(q.Get("stock_forks")); raw {
	case "", stockForksSelected:
	case stockForksAll:
		state.AllStockForks = true
		form.StockForks = stockForksAll
	default:
		form.StockForks = raw
		p.fail("stock_forks", fmt.Errorf("unknown stock fitting fork option %q", raw))
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

	bar := selectedBar.Setup
	bar.BarRollDeg = state.BarRollDeg

	post := assembly.SeatpostSetup{Spec: selectedPost.Spec, InsertionMm: state.PostInsertionMm}

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
	case assembly.InputFitTargetGrip:
		return "grip_x"
	case assembly.InputStemSpacers:
		return "spacers"
	case assembly.InputStemLength:
		return "stem_length"
	case assembly.InputStemAngle:
		return "stem_angle"
	case assembly.InputHandlebarRoll:
		return "bar_roll"
	case assembly.InputSeatpostInsertion:
		return "post_insert"
	case assembly.InputSeatpostSetback:
		return "post"
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
	// converts screen sizes to SVG units for labels and close-up margins.
	nominalDiagramWidthPx = 720.0
	// Margins around the landmarks a close-up view frames, in on-screen px at
	// the nominal width. The side margin fits the longest 1rem label
	// ("RailCradle") beside a point on the edge of the framing.
	closeUpPadXPx = 150.0
	closeUpPadYPx = 40.0
	// bikeLabelEmUnits converts the label offsets below, written in SVG units
	// for the original 18-unit labels, to ems.
	bikeLabelEmUnits = 18.0
)

// bikeLandmarkSpec places a landmark label at an offset from its point
// (written in units of an 18-unit font, rendered in ems).
type bikeLandmarkSpec struct {
	name   string
	pose   pose.Pose
	dx, dy float64
	anchor string
}

// workbenchLinks returns the Reset Defaults and CAD Baseline Fork links and
// the diagram view links for state; the latter two keep its BRP fit target.
func workbenchLinks(fix bikeFixture, state workbenchState) (defaultsURL, cadURL string, viewLinks []diagramViewLink) {
	defaultsURL, cadURL = presetURLs(fix, state, state.View)
	for _, v := range diagramViews {
		viewLinks = append(viewLinks, diagramViewLink{View: v, URL: state.withView(v).URL(), Current: v == state.View})
	}
	return defaultsURL, cadURL, viewLinks
}

// buildBikeView renders the placed bike. The form echo and links carry submitted's options that are not part of the placed setup (fit
// targets, stock bar and fork options).
func buildBikeView(
	fix bikeFixture,
	submitted workbenchState,
	bike assembly.BikePlacement,
	cadBaselineFork assembly.ChartBaseline,
) bikeView {
	view, forkID, barID := submitted.View, submitted.ForkID, submitted.BarID
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
		toSVG(seatpost.RailCradle.Origin()),
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
		{"seatpost-clamp", saddle.RailFrame, -bike.Setup.Seatpost.Spec.RailClampLengthMm / 2.0, bike.Setup.Seatpost.Spec.RailClampLengthMm, seatpostClampOutlineHeight},
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
			seatpost.ShaftTop, seatpost.RailCradle, saddle.RailFrame,
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
	// Drawing-only sizes (labels, dots, strokes) scale with the
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
		makeSegment("seatpost-setback", seatpost.ShaftTop, seatpost.RailCradle, "#59636e", 5*scale, false),
		makeSegment("saddle-profile", saddle.SitBones, saddle.BRP, "#2f363d", 4*scale, true),
		makeSegment("handlebar-rise-sweep", handlebar.BarClamp, handlebar.HandGrip, "#2f363d", 5*scale, true),
	}

	rects := append(outlineRects(stemParts), outlineRects(seatParts)...)

	landmarkSpecs := []bikeLandmarkSpec{
		{"RearAxle", placed.RearAxle, 0, -14, "middle"},
		{"RearContact", placed.RearTireContact, 0, 24, "middle"},
		{"BB", placed.BB, 0, 26, "middle"},
		{"SeatTubeTop", placed.SeatTubeTop, -14, -10, "end"},
		{"RailCradle", seatpost.RailCradle, 16, 6, "start"},
		{"RailCenter", saddle.RailCenter, 16, 24, "start"},
		{"BRP", saddle.BRP, 14, -10, "start"},
		{"SitBones", saddle.SitBones, -14, -10, "end"},
		{"HeadTubeTop", placed.HeadTubeTop, 14, 6, "start"},
		{"CoverTop", stem.CoverTop, -14, 8, "end"},
		{"SpacerTop", stem.SpacerTop, -14, -2, "end"},
		{"SteererClamp", stem.SteererClamp, -14, -14, "end"},
		{"BarClamp", stem.BarClamp, 14, 4, "start"},
		{"HandGrip", handlebar.HandGrip, -14, -12, "end"},
		{"HeadTubeBottom", placed.HeadTubeBottom, 16, 4, "start"},
		{"LowerCupBottom", placed.LowerCupBottom, 16, 18, "start"},
		{"FrontAxle", placed.FrontAxle, 0, -14, "middle"},
		{"FrontContact", placed.FrontTireContact, 0, 24, "middle"},
	}
	landmarks := make([]svgLandmark, 0, len(landmarkSpecs))
	for _, spec := range landmarkSpecs {
		lm := makeLandmark(spec.name, spec.pose, 0, 0, spec.anchor)
		lm.LabelDx = formatSVGSize(spec.dx/bikeLabelEmUnits) + "em"
		lm.LabelDy = formatSVGSize(spec.dy/bikeLabelEmUnits) + "em"
		landmarks = append(landmarks, lm)
	}

	state := submitted.withSetup(bike.Setup)
	form := state.formValues()
	forkA2cRO, cupRO, sagRO, cupLabel, cadNote := forkFieldViewState(fix, forkID, form.Sag)
	defaultsURL, cadURL, viewLinks := workbenchLinks(fix, state)

	selectedBar, _ := fix.resolveHandlebar(barID)
	minRail, maxRail := railOffsetRange(bike.Setup.Saddle, bike.Setup.Seatpost)
	postBounds := postInsertionBounds(bike.Setup.Frame, bike.Setup.Seatpost.Spec)
	return bikeView{
		MinRailOffsetMm:      minRail,
		MaxRailOffsetMm:      maxRail,
		ExampleBrpX:          formatFormFloat(fix.ExampleBrpTarget.X),
		ExampleBrpY:          formatFormFloat(fix.ExampleBrpTarget.Y),
		ExampleGripX:         formatFormFloat(fix.ExampleGripTarget.X),
		ExampleGripY:         formatFormFloat(fix.ExampleGripTarget.Y),
		MaxSpacersMm:         fix.DefaultStem.MaxSpacersMm,
		HasBuild:             true,
		Form:                 form,
		DefaultsURL:          defaultsURL,
		CadBaselineForkURL:   cadURL,
		View:                 view,
		ViewLinks:            viewLinks,
		DrawScale:            scale,
		UnitsPerPx:           viewBox.W / nominalDiagramWidthPx,
		MinPostInsertionMm:   postBounds.EffectiveMinInsertionMm,
		MaxPostInsertionMm:   postBounds.MaxAllowedInsertionMm,
		PostTotalLengthMm:    bike.Setup.Seatpost.Spec.TotalLengthMm,
		PostExposedMm:        bike.Setup.Seatpost.ExposedMm(),
		PostSetbackMm:        bike.Setup.Seatpost.Spec.SetbackMm,
		PostClampLengthMm:    bike.Setup.Seatpost.Spec.RailClampLengthMm,
		BikeName:             fix.Name,
		SaddleName:           fix.SaddleName,
		HandlebarName:        selectedBar.Name,
		Forks:                fix.Forks,
		StockForkCount:       len(fix.stockForks()),
		CadForkOption:        fix.cadForkOption(),
		SelectedForkID:       forkID,
		Handlebars:           fix.Handlebars,
		SelectedBarID:        barID,
		Seatposts:            fix.Seatposts,
		SelectedPostID:       submitted.PostID,
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
		Landmarks:            landmarks,
	}
}
