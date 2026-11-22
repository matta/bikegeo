// Package web serves the HTML/SVG workbench.
package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

	"bikegeo/internal/assembly"
	"bikegeo/internal/catalog"
	"bikegeo/internal/pose"
	"bikegeo/internal/query"
)

//go:embed templates/bike.html templates/compare.html
var templateFS embed.FS

var (
	bikeTemplate    = template.Must(template.ParseFS(templateFS, "templates/bike.html"))
	compareTemplate = template.Must(template.ParseFS(templateFS, "templates/compare.html"))
)

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

// NewHandler returns the HTTP handler for the workbench. The chart-consistency check
// runs in the catalog's go test, not at startup (Q91).
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", handleCompare)
	mux.HandleFunc("/bike", handleBikeWorkbench)
	return mux
}

func renderCompareTemplate(w http.ResponseWriter, status int, view compareView) {
	var buf bytes.Buffer
	if err := compareTemplate.Execute(&buf, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
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

// LinkText is the view's link text in the diagram card.
func (v diagramView) LinkText() string {
	switch v {
	case diagramViewBike:
		return "Whole bike"
	case diagramViewSeat:
		return "Seatpost & saddle"
	case diagramViewStem:
		return "Stem & bar"
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

type bikeView struct {
	PageTitle              string
	HasBuild               bool
	ErrorMessage           string
	InvalidField           string
	Form                   bikeFormValues
	DefaultsURL            string
	BasisForkURL           string
	View                   diagramView
	ViewLinks              []diagramViewLink
	DrawScale              float64 // view width / whole-bike width; 1 for the whole bike
	UnitsPerPx             float64 // SVG units per on-screen px at the nominal width
	MinPostInsertionMm     float64
	MaxPostInsertionMm     float64
	PostTotalLengthMm      float64
	PostExposedMm          float64
	BikeName               string
	FrameOptions           []selectOption
	SizeOptions            []selectOption
	ClearTargetsURL        string
	SlidingDropoutsNote    string
	RedshiftMixedStateNote string
	LeveledReachMm         float64
	LeveledStackMm         float64
	EttMm                  float64
	SaddleName             string
	HandlebarName          string
	ForkLeading            []selectOption
	ListedForkOptions      []selectOption
	OtherForkOptions       []selectOption
	BasisForkOptions       []selectOption
	BarSelectOptions       []selectOption
	PostSelectOptions      []selectOption
	BarOptionModes         []selectOption
	ForkOptionModes        []selectOption
	PostSetbackMm          float64
	PostClampLengthMm      float64
	CupLabel               string
	ForkA2cReadOnly        bool
	CupReadOnly            bool
	SagReadOnly            bool
	BasisForkNote          string
	BasisFork              assembly.ForkSpec
	BasisLowerCupMm        float64
	LowerCupMm             float64
	SagPct                 float64
	EffectiveA2cMm         float64
	PitchDeg               pose.Deg
	LeveledHtaDeg          pose.Deg
	LeveledStaDeg          pose.Deg
	WheelbaseMm            float64
	BbDropLeveledMm        float64
	TrailMm                float64
	RearCenterMm           float64
	RcWbRatioPct           float64
	SeatAxisToPostTopMm    float64
	SaddleHeightMm         float64
	BrpToGripDistMm        float64
	SitBonesToGripRiseMm   float64
	HandLateralMm          float64
	PublishedWheelbaseMm   float64
	BasisHtaDeg            pose.Deg
	BasisWheelbaseMm       float64
	DeltaPitchDeg          pose.Deg
	WheelRadiusMm          float64
	WheelLabel             string
	RearAxleSVG            svgPoint
	FrontAxleSVG           svgPoint
	GroundY1               float64
	GroundY2               float64
	GroundX1               float64
	GroundX2               float64
	ViewBox                string
	Rects                  []svgRect
	Segments               []svgSegment
	Landmarks              []svgLandmark
	Fitting                *bikeFittingView // nil unless a fit target is set
	MinRailOffsetMm        float64
	MaxRailOffsetMm        float64
	ExampleBrpX            string
	ExampleBrpY            string
	ExampleGripX           string
	ExampleGripY           string
	MaxSpacersMm           float64
}

// bikeFittingView is the fitting summary for a submitted build and its targets.
type bikeFittingView struct {
	SubmittedScore *submittedScoreView // nil unless a HandGrip target is set (Q110)
	Saddle         *saddleFittingView  // nil unless a BRP target is set
	Cockpit        *cockpitFittingView // nil unless a HandGrip target is set
	Builds         *buildRankingView   // nil unless a HandGrip target is set
	ApplyURL       string              // the fitted build, with the submitted targets kept
}

// submittedScoreView is the submitted build's 0–100 score readout (Q110).
type submittedScoreView struct {
	Scored           bool
	OffCatalogNote   string
	Tier             assembly.FitTier
	TierClass        string
	TierLabel        string
	Score            float64
	ActiveWeight     float64
	HasBrp           bool
	GripR            float64
	BrpMissXMm       float64
	BrpMissYMm       float64
	AttributeHeaders []buildAttributeHeader
	Attributes       []assembly.AttributeScore
}

type buildAttributeHeader struct {
	Attribute assembly.AttributeName
	Label     string
}

type fitTierLegendItem struct {
	Tier       assembly.FitTier
	Class      string
	Label      string
	Definition string
}

type buildRankingView struct {
	AllForks         bool
	HasBrp           bool
	ActiveWeight     float64
	PartsText        string
	Evaluated        int
	AttributeHeaders []buildAttributeHeader
	TierLegend       []fitTierLegendItem
	Rows             []scoredBuildView
}

type scoredBuildView struct {
	Rank                    int
	Tier                    assembly.FitTier
	TierClass               string
	TierLabel               string
	BarName                 string
	ForkName                string
	HasBrp                  bool
	BrpMissXMm              float64
	BrpMissYMm              float64
	StemLengthMm, SpacersMm float64
	StemAngleDeg            pose.Deg
	GripMissMm              float64
	GripMissXMm             float64
	GripMissYMm             float64
	GripR                   float64
	Score                   float64
	Attributes              []assembly.AttributeScore
	ApplyURL                string // the candidate build, with the submitted targets kept
	ApplyAriaLabel          string
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

// buildPartsText names the parts enumerated by build fitting.
func buildPartsText(allForks, allBars bool) string {
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

// fitTierPresentation returns the CSS class suffix and label of a fit tier.
func fitTierPresentation(t assembly.FitTier) (class, label string) {
	switch t {
	case assembly.FitsAll:
		return "fits-all", "Fits all"
	case assembly.FitsBRP:
		return "fits-brp", "Fits BRP"
	case assembly.FitsGrip:
		return "fits-grip", "Fits grip"
	case assembly.FitsNeither:
		return "fits-neither", "Fits neither"
	default:
		return "fits-neither", string(t)
	}
}

// fitTierLegend defines the four fit tiers in rank order (Q61, Q90).
func fitTierLegend() []fitTierLegendItem {
	oval := assembly.DefaultGripOval()
	grip := fmt.Sprintf("HandGrip inside the ±%.0f × ±%.0f mm oval (R ≤ %.1f)", oval.SemiMinorMm, oval.SemiMajorMm, assembly.FitGateGripNormalizedRadius)
	gripOut := fmt.Sprintf("HandGrip outside the oval (R > %.1f)", assembly.FitGateGripNormalizedRadius)
	brp := fmt.Sprintf("saddle BRP within |ΔY| ≤ %.1f mm, |ΔX| ≤ %.1f mm", assembly.FitGateBrpYMm, assembly.FitGateBrpXMm)
	definitions := map[assembly.FitTier]string{
		assembly.FitsAll:     grip + " and " + brp + "; ranked by descending score.",
		assembly.FitsBRP:     "Saddle BRP within |ΔY| ≤ " + fmt.Sprintf("%.1f mm, |ΔX| ≤ %.1f mm", assembly.FitGateBrpYMm, assembly.FitGateBrpXMm) + ", but " + gripOut + "; ranked by closest HandGrip R, then descending score.",
		assembly.FitsGrip:    grip + ", but misses saddle BRP (or BRP target is unset); ranked by descending score.",
		assembly.FitsNeither: gripOut + " and misses saddle BRP (or BRP target is unset); ranked by closest HandGrip R, then descending score.",
	}
	tiers := []assembly.FitTier{assembly.FitsAll, assembly.FitsBRP, assembly.FitsGrip, assembly.FitsNeither}
	legend := make([]fitTierLegendItem, len(tiers))
	for i, t := range tiers {
		class, label := fitTierPresentation(t)
		legend[i] = fitTierLegendItem{Tier: t, Class: class, Label: label, Definition: definitions[t]}
	}
	return legend
}

// buildAttributeHeaders labels the 12 attribute score columns with their weights.
func buildAttributeHeaders(w assembly.ScoreWeights) []buildAttributeHeader {
	weight := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return []buildAttributeHeader{
		{assembly.AttrHandGrip, "Grip (" + weight(w.HandGrip) + ")"},
		{assembly.AttrSaddleBrpY, "BRP Y (" + weight(w.SaddleBrpY) + ")"},
		{assembly.AttrSaddleBrpX, "BRP X (" + weight(w.SaddleBrpX) + ")"},
		{assembly.AttrStem, "Stem (" + weight(w.Stem) + ")"},
		{assembly.AttrHandlebarSweep, "Sweep (" + weight(w.HandlebarSweep) + ")"},
		{assembly.AttrForkSag, "Sag (" + weight(w.ForkSag) + ")"},
		{assembly.AttrSpacers, "Spacers (" + weight(w.Spacers) + ")"},
		{assembly.AttrSaddleRailOffset, "Rail (" + weight(w.SaddleRailOffset) + ")"},
		{assembly.AttrRcWbRatio, "RC/WB (" + weight(w.RcWbRatio) + ")"},
		{assembly.AttrSeatpostInsertion, "Post (" + weight(w.SeatpostInsertion) + ")"},
		{assembly.AttrLeveledBbDrop, "BB drop (" + weight(w.LeveledBbDrop) + ")"},
		{assembly.AttrTrail, "Trail (" + weight(w.Trail) + ")"},
	}
}

// railOffsetRange returns the saddle rail offset input bounds ∓L for saddle
// and post.
func railOffsetRange(saddle assembly.SaddleSetup, post assembly.SeatpostSetup) (minMm, maxMm float64) {
	limit, err := assembly.BikeSetup{Saddle: saddle, Seatpost: post}.RailOffsetLimitMm()
	if err != nil {
		return 0, 0 // validate() rejects such a workbench before any page renders
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

// buildBikeView renders the placed bike for the parsed query.
func buildBikeView(
	parsed query.Bike,
	bike assembly.BikePlacement,
	basis assembly.FramesetPlacement,
) bikeView {
	view := diagramView(parsed.View)
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

	v := controlsView(parsed, formFromBike(parsed), selectedIDs{
		Frame: parsed.Frame.ID, Size: parsed.Size.Name, Fork: parsed.ForkID, Bar: parsed.Bar.ID, Post: parsed.Post.ID,
	}, "", "")
	v.PageTitle = v.BikeName + " — bikegeo"
	viewLinks := make([]diagramViewLink, 0, len(diagramViews))
	for _, dv := range diagramViews {
		viewLinks = append(viewLinks, diagramViewLink{View: dv, URL: parsed.WithView(string(dv)).URL(), Current: dv == view})
	}
	v.HasBuild = true
	v.DefaultsURL = query.DefaultBike().WithView(string(view)).URL()
	v.BasisForkURL = parsed.WithView(string(view)).WithBasisFork().URL()
	if parsed.BrpTarget != nil || parsed.GripTarget != nil {
		v.ClearTargetsURL = parsed.WithoutTargets().URL()
	}
	v.View = view
	v.ViewLinks = viewLinks
	v.DrawScale = scale
	v.UnitsPerPx = viewBox.W / nominalDiagramWidthPx
	v.PostExposedMm = bike.Setup.Seatpost.ExposedMm()
	v.SaddleName = catalog.DefaultSaddle().Name
	v.HandlebarName = parsed.Bar.Name
	v.LowerCupMm = bike.Setup.LowerCup.StackMm
	v.SagPct = bike.Setup.Fork.SagPct
	v.EffectiveA2cMm = bike.Setup.Fork.EffectiveA2cMm()
	v.PitchDeg = placed.PitchDeg()
	v.LeveledHtaDeg = placed.LeveledHeadTubeAngleDeg()
	v.LeveledStaDeg = placed.LeveledSeatTubeAngleDeg()
	v.LeveledReachMm = placed.LeveledReachMm()
	v.LeveledStackMm = placed.LeveledStackMm()
	v.EttMm = placed.EttMm()
	v.WheelbaseMm = placed.WheelbaseMm()
	v.BbDropLeveledMm = placed.LeveledBbDropMm()
	v.TrailMm = placed.TrailMm()
	v.RearCenterMm = placed.RearCenterMm()
	v.RcWbRatioPct = placed.RcWbRatioPct()
	v.SeatAxisToPostTopMm = bike.SeatAxisToPostTopMm()
	v.SaddleHeightMm = bike.SaddleHeightMm()
	v.BrpToGripDistMm = bike.BrpToGripDistMm()
	v.SitBonesToGripRiseMm = bike.SitBonesToGripRiseMm()
	v.HandLateralMm = handlebar.HandLateralMm
	v.BasisHtaDeg = basis.LeveledHeadTubeAngleDeg()
	v.BasisWheelbaseMm = basis.WheelbaseMm()
	v.DeltaPitchDeg = placed.PitchDeltaDeg(basis)
	v.WheelRadiusMm = r
	v.WheelLabel = catalog.WheelLabel
	v.RearAxleSVG = rearSVG
	v.FrontAxleSVG = frontSVG
	v.GroundY1 = rearContactSVG.Y
	v.GroundY2 = frontContactSVG.Y
	v.GroundX1 = rearContactSVG.X - r - 40
	v.GroundX2 = frontContactSVG.X + r + 40
	v.ViewBox = viewBox.String()
	v.Rects = rects
	v.Segments = segments
	v.Landmarks = landmarks
	return v
}
