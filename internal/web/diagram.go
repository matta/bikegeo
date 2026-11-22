package web

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"bikegeo/internal/assembly"
	"bikegeo/internal/pose"
	"bikegeo/internal/query"
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

type svgFitTarget struct {
	Kind          string // "brp" or "grip"
	Name          string // "Target BRP" or "Target Grip"
	Status        string // "pass" or "fail"
	X             float64
	Y             float64
	PlacedX       float64
	PlacedY       float64
	ShowMiss      bool
	ZoneShape     string // "rect" or "ellipse"
	ZoneX         float64
	ZoneY         float64
	ZoneWidth     float64
	ZoneHeight    float64
	ZoneRx        float64
	ZoneRy        float64
	ZoneTransform string
	Tooltip       string
	ShowLabel     bool
	LabelX        float64
	LabelY        float64
	Anchor        string
	LabelDx       string
	LabelDy       string
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
	// fitGateTolMm matches assembly's round-off tolerance on gate checks.
	fitGateTolMm = 1e-9
	// missLineMinMm omits near-zero miss connectors when placed == target.
	missLineMinMm = 0.1
)

// bikeLandmarkSpec places a landmark label at an offset from its point
// (written in units of an 18-unit font, rendered in ems).
type bikeLandmarkSpec struct {
	name   string
	pose   pose.Pose
	dx, dy float64
	anchor string
}

type bikeDiagram struct {
	View         diagramView
	ViewLinks    []diagramViewLink
	DrawScale    float64
	UnitsPerPx   float64
	RearAxleSVG  svgPoint
	FrontAxleSVG svgPoint
	GroundY1     float64
	GroundY2     float64
	GroundX1     float64
	GroundX2     float64
	ViewBox      string
	Rects        []svgRect
	Segments     []svgSegment
	Landmarks    []svgLandmark
	FitTargets   []svgFitTarget
}

func brpTargetFramingPts(target pose.Point) []svgPoint {
	t := toSVG(target)
	return []svgPoint{
		{X: t.X - assembly.FitGateBrpXMm, Y: t.Y - assembly.FitGateBrpYMm},
		{X: t.X + assembly.FitGateBrpXMm, Y: t.Y - assembly.FitGateBrpYMm},
		{X: t.X - assembly.FitGateBrpXMm, Y: t.Y + assembly.FitGateBrpYMm},
		{X: t.X + assembly.FitGateBrpXMm, Y: t.Y + assembly.FitGateBrpYMm},
	}
}

func gripTargetFramingPts(target pose.Point) []svgPoint {
	oval := assembly.DefaultGripOval()
	ovalPose := pose.NewPose(target, oval.MajorAxisHeadingDeg)
	pts := make([]svgPoint, 0, 4)
	for _, along := range []float64{-oval.SemiMajorMm, oval.SemiMajorMm} {
		for _, across := range []float64{-oval.SemiMinorMm, oval.SemiMinorMm} {
			pts = append(pts, toSVG(ovalPose.Translate(along, across).Origin()))
		}
	}
	return pts
}

func buildFitTargets(parsed query.Bike, bike assembly.BikePlacement, view diagramView) []svgFitTarget {
	var targets []svgFitTarget
	if parsed.BrpTarget != nil {
		target := *parsed.BrpTarget
		tSVG := toSVG(target)
		placedPose := bike.Saddle.BRP
		placedSVG := toSVG(placedPose.Origin())
		targetPose := pose.NewPose(target, 0)
		miss := placedPose.OffsetTo(targetPose)
		pass := math.Abs(miss.X) <= assembly.FitGateBrpXMm+fitGateTolMm && math.Abs(miss.Y) <= assembly.FitGateBrpYMm+fitGateTolMm
		status := "fail"
		if pass {
			status = "pass"
		}
		targets = append(targets, svgFitTarget{
			Kind:       "brp",
			Name:       "Target BRP",
			Status:     status,
			X:          tSVG.X,
			Y:          tSVG.Y,
			PlacedX:    placedSVG.X,
			PlacedY:    placedSVG.Y,
			ShowMiss:   placedPose.DistanceTo(targetPose) >= missLineMinMm,
			ZoneShape:  "rect",
			ZoneX:      tSVG.X - assembly.FitGateBrpXMm,
			ZoneY:      tSVG.Y - assembly.FitGateBrpYMm,
			ZoneWidth:  2 * assembly.FitGateBrpXMm,
			ZoneHeight: 2 * assembly.FitGateBrpYMm,
			Tooltip:    fmt.Sprintf("Target BRP (%.1f, %.1f) mm — %s (miss %+.1f, %+.1f mm)", target.X, target.Y, status, cleanZero(miss.X, 0.05), cleanZero(miss.Y, 0.05)),
			ShowLabel:  view == diagramViewBike || view == diagramViewSeat,
			LabelX:     tSVG.X,
			LabelY:     tSVG.Y,
			Anchor:     "start",
			LabelDx:    "0.78em",
			LabelDy:    "-0.67em",
		})
	}
	if parsed.GripTarget != nil {
		target := *parsed.GripTarget
		tSVG := toSVG(target)
		placedPose := bike.Handlebar.HandGrip
		placedSVG := toSVG(placedPose.Origin())
		targetPose := pose.NewPose(target, 0)
		miss := placedPose.OffsetTo(targetPose)
		oval := assembly.DefaultGripOval()
		gripR := oval.NormalizedRadius(target, placedPose.Origin())
		pass := gripR <= assembly.FitGateGripNormalizedRadius+fitGateTolMm
		status := "fail"
		if pass {
			status = "pass"
		}
		targets = append(targets, svgFitTarget{
			Kind:          "grip",
			Name:          "Target Grip",
			Status:        status,
			X:             tSVG.X,
			Y:             tSVG.Y,
			PlacedX:       placedSVG.X,
			PlacedY:       placedSVG.Y,
			ShowMiss:      placedPose.DistanceTo(targetPose) >= missLineMinMm,
			ZoneShape:     "ellipse",
			ZoneRx:        oval.SemiMajorMm,
			ZoneRy:        oval.SemiMinorMm,
			ZoneTransform: fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)", tSVG.X, tSVG.Y, -float64(oval.MajorAxisHeadingDeg)),
			Tooltip:       fmt.Sprintf("Target HandGrip (%.1f, %.1f) mm — %s (R = %.2f, miss %+.1f, %+.1f mm)", target.X, target.Y, status, gripR, cleanZero(miss.X, 0.05), cleanZero(miss.Y, 0.05)),
			ShowLabel:     view == diagramViewBike || view == diagramViewStem,
			LabelX:        tSVG.X,
			LabelY:        tSVG.Y,
			Anchor:        "start",
			LabelDx:       "0.78em",
			LabelDy:       "-0.67em",
		})
	}
	return targets
}

func buildBikeDiagram(parsed query.Bike, bike assembly.BikePlacement) bikeDiagram {
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
	if parsed.BrpTarget != nil {
		pts = append(pts, brpTargetFramingPts(*parsed.BrpTarget)...)
	}
	if parsed.GripTarget != nil {
		pts = append(pts, gripTargetFramingPts(*parsed.GripTarget)...)
	}
	wholeBox := computeViewBoxFromPoints(pts, 95.0, 95.0, 95.0, 100.0)

	stemParts := stemOutlines(stem, bike.Setup.Stem)
	seatParts := []poseOutline{
		{"seatpost-clamp", saddle.RailFrame, -bike.Setup.Seatpost.Spec.RailClampLengthMm / 2.0, bike.Setup.Seatpost.Spec.RailClampLengthMm, seatpostClampOutlineHeight},
		{"saddle-rail", saddle.RailCenter, -bike.Setup.Saddle.RailLengthMm / 2.0, bike.Setup.Saddle.RailLengthMm, saddleRailOutlineHeight},
	}

	// A close-up frames its poses and every corner of the outlines drawn
	// around them, plus any active target and tolerance zone for that view.
	var framingPoses []pose.Pose
	var framingOutlines []poseOutline
	var extraFramingPts []svgPoint
	switch view {
	case diagramViewBike:
	case diagramViewSeat:
		// Every seatpost and saddle pose above SeatTubeTop, not the seat tube.
		framingPoses = []pose.Pose{
			seatpost.ShaftTop, seatpost.RailCradle, saddle.RailFrame,
			saddle.RailCenter, saddle.BRP, saddle.SitBones,
		}
		framingOutlines = seatParts
		if parsed.BrpTarget != nil {
			extraFramingPts = brpTargetFramingPts(*parsed.BrpTarget)
		}
	case diagramViewStem:
		// Every headset, stem, and handlebar pose above HeadTubeTop, not the
		// head tube. The upper cover outline starts at HeadTubeTop, so its
		// corners keep that end of the stack in view.
		framingPoses = []pose.Pose{
			stem.CoverTop, stem.SpacerTop, stem.SteererClamp, stem.BarClamp,
			handlebar.BarRollFrame, handlebar.HandGrip,
		}
		framingOutlines = stemParts
		if parsed.GripTarget != nil {
			extraFramingPts = gripTargetFramingPts(*parsed.GripTarget)
		}
	}
	viewBox := wholeBox
	if len(framingPoses) > 0 {
		framingPts := make([]svgPoint, 0, len(framingPoses)+4*len(framingOutlines)+len(extraFramingPts))
		for _, p := range framingPoses {
			framingPts = append(framingPts, toSVG(p.Origin()))
		}
		for _, o := range framingOutlines {
			framingPts = append(framingPts, o.corners()...)
		}
		framingPts = append(framingPts, extraFramingPts...)
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

	viewLinks := make([]diagramViewLink, 0, len(diagramViews))
	for _, dv := range diagramViews {
		viewLinks = append(viewLinks, diagramViewLink{View: dv, URL: parsed.WithView(string(dv)).URL(), Current: dv == view})
	}

	return bikeDiagram{
		View:         view,
		ViewLinks:    viewLinks,
		DrawScale:    scale,
		UnitsPerPx:   viewBox.W / nominalDiagramWidthPx,
		RearAxleSVG:  rearSVG,
		FrontAxleSVG: frontSVG,
		GroundY1:     rearContactSVG.Y,
		GroundY2:     frontContactSVG.Y,
		GroundX1:     rearContactSVG.X - r - 40,
		GroundX2:     frontContactSVG.X + r + 40,
		ViewBox:      viewBox.String(),
		Rects:        rects,
		Segments:     segments,
		Landmarks:    landmarks,
		FitTargets:   buildFitTargets(parsed, bike, view),
	}
}
