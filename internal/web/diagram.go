package web

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
	"github.com/matta/bikegeo/internal/query"
)

func toSVG(p pose.Point) svgPoint {
	return svgPoint{X: p.X(), Y: -p.Y()}
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
	Name      string
	X         float64
	Y         float64
	DotRadius float64
	Detail    bool
	ShowLabel bool
	LabelX    float64
	LabelY    float64
	Anchor    string
	LabelDy   string
	HasLeader bool
	LeaderX1  float64
	LeaderY1  float64
	LeaderX2  float64
	LeaderY2  float64
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
	LabelDy       string
	HasLeader     bool
	LeaderX1      float64
	LeaderY1      float64
	LeaderX2      float64
	LeaderY2      float64
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

// poseOutline holds an SVG rectangle and its four corners for view framing.
type poseOutline struct {
	rect    svgRect
	corners [4]svgPoint
}

// newPoseOutline draws a rectangle along p's local +X axis (anchored at
// p.Origin()) starting at startMm, with thickness centered on that line.
func newPoseOutline(name string, p pose.Pose, startMm, lengthMm, thicknessMm float64) poseOutline {
	o := toSVG(p.Origin())
	half := thicknessMm / 2.0
	outline := poseOutline{
		rect: svgRect{
			Name:      name,
			X:         startMm,
			Y:         -half,
			Width:     lengthMm,
			Height:    thicknessMm,
			Transform: fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)", o.X, o.Y, -float64(p.HeadingDeg())),
		},
	}
	for i, along := range []float64{startMm, startMm + lengthMm} {
		for j, across := range []float64{-half, half} {
			outline.corners[2*i+j] = toSVG(p.TranslateLocal(along, across).Origin())
		}
	}
	return outline
}

func outlineRects(outlines []poseOutline) []svgRect {
	rects := make([]svgRect, 0, len(outlines))
	for _, o := range outlines {
		rects = append(rects, o.rect)
	}
	return rects
}

func stemOutlines(placed assembly.StemPlacement, setup assembly.StemSetup) []poseOutline {
	return []poseOutline{
		newPoseOutline("upper-cover", placed.HeadTubeTop, 0, setup.UpperStackMm, upperCoverOutlineWidthMm),
		newPoseOutline("spacers", placed.CoverTop, 0, setup.SpacersMm, spacerOutlineWidthMm),
		newPoseOutline("steerer-clamp", placed.SteererClamp, -setup.ClampHeightMm/2.0, setup.ClampHeightMm, steererClampOutlineWidthMm),
		newPoseOutline("stem-body", placed.BarClamp, -setup.LengthMm, setup.LengthMm, stemBodyOutlineWidthMm),
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

func makeSegment(name string, from, to pose.Point, color string, width float64, illustrative bool) svgSegment {
	p1 := toSVG(from)
	p2 := toSVG(to)
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
	// ("RailPivot") beside a point on the edge of the framing.
	closeUpPadXPx = 150.0
	closeUpPadYPx = 40.0
	// fitGateTolMm matches assembly's round-off tolerance on gate checks.
	fitGateTolMm = 1e-9
	// missLineMinMm omits near-zero miss connectors when placed == target.
	missLineMinMm = 0.1

	// Label layout constants in nominal screen pixels (at nominalDiagramWidthPx).
	labelFontPx          = 13.0 // 0.8125rem at 16px root
	labelCharWidthPx     = 7.9
	labelPadXPx          = 4.0
	labelBoxHeightPx     = 19.0
	labelBoxGapPx        = 3.0
	labelViewMarginPx    = 6.0
	labelPointKeepOutPx  = 7.0
	labelLeaderMinDistPx = 18.0
	labelBaselineDy      = "0.35em"
)

var screenSpace = pose.NewCoordinateSystem("web:screen")

// bikeLandmarkSpec places a landmark label with a preferred direction and priority.
type bikeLandmarkSpec struct {
	name      string
	point     pose.Point
	prefDx    float64
	prefDy    float64
	prefAnc   string
	priority  int
	bikeMacro bool
	seatLabel bool
	stemLabel bool
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
			pts = append(pts, toSVG(ovalPose.TranslateLocal(along, across).Origin()))
		}
	}
	return pts
}

func buildFitTargets(parsed query.Bike, bike assembly.Placement, view diagramView) []svgFitTarget {
	var targets []svgFitTarget
	if parsed.BrpTarget != nil {
		target := *parsed.BrpTarget
		tSVG := toSVG(target)
		placed := bike.Saddle.BRP.Origin()
		placedSVG := toSVG(placed)
		miss := placed.OffsetTo(target)
		pass := math.Abs(miss.X()) <= assembly.FitGateBrpXMm+fitGateTolMm && math.Abs(miss.Y()) <= assembly.FitGateBrpYMm+fitGateTolMm
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
			ShowMiss:   placed.DistanceTo(target) >= missLineMinMm,
			ZoneShape:  "rect",
			ZoneX:      tSVG.X - assembly.FitGateBrpXMm,
			ZoneY:      tSVG.Y - assembly.FitGateBrpYMm,
			ZoneWidth:  2 * assembly.FitGateBrpXMm,
			ZoneHeight: 2 * assembly.FitGateBrpYMm,
			Tooltip:    fmt.Sprintf("Target BRP (%.1f, %.1f) mm — %s (miss %+.1f, %+.1f mm)", target.X(), target.Y(), status, cleanZero(miss.X(), 0.05), cleanZero(miss.Y(), 0.05)),
			ShowLabel:  view == diagramViewBike || view == diagramViewSeat,
			LabelX:     tSVG.X,
			LabelY:     tSVG.Y,
			Anchor:     "start",
			LabelDy:    labelBaselineDy,
		})
	}
	if parsed.GripTarget != nil {
		target := *parsed.GripTarget
		tSVG := toSVG(target)
		placed := bike.Handlebar.HandGrip.Origin()
		placedSVG := toSVG(placed)
		miss := placed.OffsetTo(target)
		oval := assembly.DefaultGripOval()
		gripR := oval.NormalizedRadius(target, placed)
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
			ShowMiss:      placed.DistanceTo(target) >= missLineMinMm,
			ZoneShape:     "ellipse",
			ZoneRx:        oval.SemiMajorMm,
			ZoneRy:        oval.SemiMinorMm,
			ZoneTransform: fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)", tSVG.X, tSVG.Y, -float64(oval.MajorAxisHeadingDeg)),
			Tooltip:       fmt.Sprintf("Target HandGrip (%.1f, %.1f) mm — %s (R = %.2f, miss %+.1f, %+.1f mm)", target.X(), target.Y(), status, gripR, cleanZero(miss.X(), 0.05), cleanZero(miss.Y(), 0.05)),
			ShowLabel:     view == diagramViewBike || view == diagramViewStem,
			LabelX:        tSVG.X,
			LabelY:        tSVG.Y,
			Anchor:        "start",
			LabelDy:       labelBaselineDy,
		})
	}
	return targets
}

type screenBox struct {
	minX, minY, maxX, maxY float64
}

func (b screenBox) intersects(other screenBox, gap float64) bool {
	return b.minX < other.maxX+gap && b.maxX+gap > other.minX &&
		b.minY < other.maxY+gap && b.maxY+gap > other.minY
}

func (b screenBox) inside(bounds screenBox) bool {
	return b.minX >= bounds.minX && b.maxX <= bounds.maxX &&
		b.minY >= bounds.minY && b.maxY <= bounds.maxY
}

func (b screenBox) nearestPoint(px, py float64) (float64, float64) {
	return min(max(px, b.minX), b.maxX), min(max(py, b.minY), b.maxY)
}

func (b screenBox) clampInside(bounds screenBox) screenBox {
	w := b.maxX - b.minX
	h := b.maxY - b.minY
	minX := min(max(b.minX, bounds.minX), max(bounds.minX, bounds.maxX-w))
	minY := min(max(b.minY, bounds.minY), max(bounds.minY, bounds.maxY-h))
	return screenBox{minX: minX, minY: minY, maxX: minX + w, maxY: minY + h}
}

func boxIntersectsSegment(b screenBox, x1, y1, x2, y2 float64) bool {
	dx, dy := x2-x1, y2-y1
	t0, t1 := 0.0, 1.0
	clip := func(p, q float64) bool {
		if p == 0 {
			return q >= 0
		}
		r := q / p
		if p < 0 {
			if r > t1 {
				return false
			}
			if r > t0 {
				t0 = r
			}
		} else {
			if r < t0 {
				return false
			}
			if r < t1 {
				t1 = r
			}
		}
		return true
	}
	return clip(-dx, x1-b.minX) &&
		clip(dx, b.maxX-x1) &&
		clip(-dy, y1-b.minY) &&
		clip(dy, b.maxY-y1)
}

func labelBoxWidthPx(text string) float64 {
	return float64(len(text))*labelCharWidthPx + 2*labelPadXPx
}

func boxForLabelAnchor(ax, ay, boxW, boxH float64, anchor string) screenBox {
	halfH := boxH / 2.0
	switch anchor {
	case "end":
		return screenBox{minX: ax - boxW, minY: ay - halfH, maxX: ax, maxY: ay + halfH}
	case "middle":
		return screenBox{minX: ax - boxW/2.0, minY: ay - halfH, maxX: ax + boxW/2.0, maxY: ay + halfH}
	default:
		return screenBox{minX: ax, minY: ay - halfH, maxX: ax + boxW, maxY: ay + halfH}
	}
}

func anchorForBox(b screenBox, anchor string) (float64, float64) {
	cy := (b.minY + b.maxY) / 2.0
	switch anchor {
	case "end":
		return b.maxX, cy
	case "middle":
		return (b.minX + b.maxX) / 2.0, cy
	default:
		return b.minX, cy
	}
}

func pointInViewBox(pt svgPoint, vb svgViewBox) bool {
	return pt.X >= vb.X && pt.X <= vb.X+vb.W && pt.Y >= vb.Y && pt.Y <= vb.Y+vb.H
}

type labelPlaceItem struct {
	text      string
	svgX      float64
	svgY      float64
	prefDx    float64
	prefDy    float64
	prefAnc   string
	priority  int
	isTarget  bool
	targetIdx int
	lmIdx     int
}

type labelCandidate struct {
	box       screenBox
	anchor    string
	preferred bool
}

var labelRadialDirs = []struct {
	ux, uy float64
	anchor string
}{
	{1.0, 0.0, "start"},
	{-1.0, 0.0, "end"},
	{0.0, -1.0, "middle"},
	{0.0, 1.0, "middle"},
	{0.866, -0.5, "start"},
	{-0.866, -0.5, "end"},
	{0.866, 0.5, "start"},
	{-0.866, 0.5, "end"},
	{0.5, -0.866, "start"},
	{-0.5, -0.866, "end"},
	{0.5, 0.866, "start"},
	{-0.5, 0.866, "end"},
}

func placeDiagramLabels(
	viewBox svgViewBox,
	unitsPerPx float64,
	scale float64,
	segments []svgSegment,
	landmarks []svgLandmark,
	targets []svgFitTarget,
	items []labelPlaceItem,
) {
	toPx := func(sx, sy float64) (float64, float64) {
		return (sx - viewBox.X) / unitsPerPx, (sy - viewBox.Y) / unitsPerPx
	}
	toSVGCoords := func(px, py float64) (float64, float64) {
		return viewBox.X + px*unitsPerPx, viewBox.Y + py*unitsPerPx
	}

	viewW := viewBox.W / unitsPerPx
	viewH := viewBox.H / unitsPerPx
	safeBounds := screenBox{
		minX: labelViewMarginPx,
		minY: labelViewMarginPx,
		maxX: max(labelViewMarginPx, viewW-labelViewMarginPx),
		maxY: max(labelViewMarginPx, viewH-labelViewMarginPx),
	}

	// Collect obstacle boxes for visible dots, target crosshairs, and world-axis labels.
	var pointBoxes []screenBox
	for _, lm := range landmarks {
		pt := svgPoint{X: lm.X, Y: lm.Y}
		if !pointInViewBox(pt, viewBox) {
			continue
		}
		px, py := toPx(lm.X, lm.Y)
		pointBoxes = append(pointBoxes, screenBox{
			minX: px - labelPointKeepOutPx, minY: py - labelPointKeepOutPx,
			maxX: px + labelPointKeepOutPx, maxY: py + labelPointKeepOutPx,
		})
	}
	for _, tgt := range targets {
		pt := svgPoint{X: tgt.X, Y: tgt.Y}
		if !pointInViewBox(pt, viewBox) {
			continue
		}
		px, py := toPx(tgt.X, tgt.Y)
		pointBoxes = append(pointBoxes, screenBox{
			minX: px - labelPointKeepOutPx, minY: py - labelPointKeepOutPx,
			maxX: px + labelPointKeepOutPx, maxY: py + labelPointKeepOutPx,
		})
	}

	var placedBoxes []screenBox
	// Keep labels clear of "+X" and "+Y" world-axis labels when BB is in view.
	if pointInViewBox(svgPoint{X: 0, Y: 0}, viewBox) {
		xLabelX, xLabelY := toPx(114*scale, 4*scale)
		placedBoxes = append(placedBoxes, boxForLabelAnchor(xLabelX, xLabelY-4, labelBoxWidthPx("+X"), 16, "start"))
		yLabelX, yLabelY := toPx(6*scale, -106*scale)
		placedBoxes = append(placedBoxes, boxForLabelAnchor(yLabelX, yLabelY-4, labelBoxWidthPx("+Y"), 16, "start"))
	}

	type segPx struct{ x1, y1, x2, y2 float64 }
	var segsPx []segPx
	for _, s := range segments {
		x1, y1 := toPx(s.X1, s.Y1)
		x2, y2 := toPx(s.X2, s.Y2)
		segsPx = append(segsPx, segPx{x1, y1, x2, y2})
	}
	for _, tgt := range targets {
		if !tgt.ShowMiss {
			continue
		}
		x1, y1 := toPx(tgt.PlacedX, tgt.PlacedY)
		x2, y2 := toPx(tgt.X, tgt.Y)
		segsPx = append(segsPx, segPx{x1, y1, x2, y2})
	}

	// Deterministic priority order: lower priority number first, tie-broken by text.
	order := make([]int, len(items))
	for i := range items {
		order[i] = i
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0; j-- {
			a, b := items[order[j-1]], items[order[j]]
			if b.priority < a.priority || (b.priority == a.priority && b.text < a.text) {
				order[j-1], order[j] = order[j], order[j-1]
			} else {
				break
			}
		}
	}

	radii := []float64{16, 28, 42, 58, 76, 98, 124, 156, 194}

	for _, idx := range order {
		item := items[idx]
		px, py := toPx(item.svgX, item.svgY)
		boxW := labelBoxWidthPx(item.text)
		boxH := labelBoxHeightPx

		candidates := make([]labelCandidate, 0, 1+2*len(radii)*len(labelRadialDirs))
		prefBox := boxForLabelAnchor(px+item.prefDx, py+item.prefDy, boxW, boxH, item.prefAnc)
		candidates = append(candidates, labelCandidate{box: prefBox, anchor: item.prefAnc, preferred: true})
		clampedPref := prefBox.clampInside(safeBounds)
		if clampedPref != prefBox {
			candidates = append(candidates, labelCandidate{box: clampedPref, anchor: item.prefAnc, preferred: false})
		}

		for _, r := range radii {
			for _, d := range labelRadialDirs {
				// Adjust vertical radii slightly so N/S labels clear the dot + box half-height.
				offX := d.ux * r
				offY := d.uy * r
				if d.anchor == "middle" && r < 24 {
					offY = d.uy * (r + boxH/2.0 - 4.0)
				}
				b := boxForLabelAnchor(px+offX, py+offY, boxW, boxH, d.anchor)
				candidates = append(candidates, labelCandidate{box: b, anchor: d.anchor})
				cb := b.clampInside(safeBounds)
				if cb != b {
					candidates = append(candidates, labelCandidate{box: cb, anchor: d.anchor})
				}
			}
		}

		bestFound := false
		bestPenalty := math.Inf(1)
		var bestBox screenBox
		bestAnchor := item.prefAnc

		evaluate := func(cand labelCandidate, requireNoPointHit bool, gap float64) {
			if !cand.box.inside(safeBounds) {
				return
			}
			for _, pb := range placedBoxes {
				if cand.box.intersects(pb, gap) {
					return
				}
			}
			pointHits := 0
			for _, ptb := range pointBoxes {
				if cand.box.intersects(ptb, 0) {
					if requireNoPointHit {
						return
					}
					pointHits++
				}
			}
			nx, ny := cand.box.nearestPoint(px, py)
			dist := pose.NewVector(screenSpace, nx-px, ny-py).Norm()
			penalty := dist + float64(pointHits)*200.0
			if cand.preferred {
				penalty -= 4.0
			}
			for _, s := range segsPx {
				if boxIntersectsSegment(cand.box, s.x1, s.y1, s.x2, s.y2) {
					penalty += 12.0
				}
			}
			if !bestFound || penalty < bestPenalty {
				bestFound = true
				bestPenalty = penalty
				bestBox = cand.box
				bestAnchor = cand.anchor
			}
		}

		for _, cand := range candidates {
			evaluate(cand, true, labelBoxGapPx)
		}
		if !bestFound {
			for _, cand := range candidates {
				evaluate(cand, false, labelBoxGapPx)
			}
		}
		if !bestFound {
			// Deterministic grid scan inside safeBounds guarantees zero pairwise overlap.
			for _, gap := range []float64{labelBoxGapPx, 1.0, 0.0} {
				for y := safeBounds.minY + boxH/2.0; y <= safeBounds.maxY-boxH/2.0; y += 16.0 {
					for x := safeBounds.minX; x <= safeBounds.maxX-boxW; x += 20.0 {
						b := screenBox{minX: x, minY: y - boxH/2.0, maxX: x + boxW, maxY: y + boxH/2.0}
						evaluate(labelCandidate{box: b, anchor: "start"}, false, gap)
					}
				}
				if bestFound {
					break
				}
			}
		}
		if !bestFound {
			bestBox = prefBox.clampInside(safeBounds)
			bestAnchor = item.prefAnc
		}

		placedBoxes = append(placedBoxes, bestBox)
		ax, ay := anchorForBox(bestBox, bestAnchor)
		svgLX, svgLY := toSVGCoords(ax, ay)
		nx, ny := bestBox.nearestPoint(px, py)
		dist := pose.NewVector(screenSpace, nx-px, ny-py).Norm()
		hasLeader := dist > labelLeaderMinDistPx
		lx2, ly2 := toSVGCoords(nx, ny)

		if item.isTarget {
			t := &targets[item.targetIdx]
			t.LabelX = svgLX
			t.LabelY = svgLY
			t.Anchor = bestAnchor
			t.LabelDy = labelBaselineDy
			t.HasLeader = hasLeader
			t.LeaderX1 = t.X
			t.LeaderY1 = t.Y
			t.LeaderX2 = lx2
			t.LeaderY2 = ly2
		} else {
			lm := &landmarks[item.lmIdx]
			lm.LabelX = svgLX
			lm.LabelY = svgLY
			lm.Anchor = bestAnchor
			lm.LabelDy = labelBaselineDy
			lm.HasLeader = hasLeader
			lm.LeaderX1 = lm.X
			lm.LeaderY1 = lm.Y
			lm.LeaderX2 = lx2
			lm.LeaderY2 = ly2
		}
	}
}

func buildBikeDiagram(parsed query.Bike, bike assembly.Placement) bikeDiagram {
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
		toSVG(seatpost.RailPivot.Origin()),
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
		newPoseOutline("seatpost-clamp", saddle.RailClamp, -bike.Setup.Seatpost.Spec.RailClampLengthMm/2.0, bike.Setup.Seatpost.Spec.RailClampLengthMm, seatpostClampOutlineHeight),
		newPoseOutline("saddle-rail", saddle.RailCenter, -bike.Setup.Saddle.RailLengthMm/2.0, bike.Setup.Saddle.RailLengthMm, saddleRailOutlineHeight),
	}

	// A close-up frames its poses and every corner of the outlines drawn
	// around them, plus any active target and tolerance zone for that view.
	var framingOrigins []pose.Point
	var framingOutlines []poseOutline
	var extraFramingPts []svgPoint
	switch view {
	case diagramViewBike:
	case diagramViewSeat:
		// Every seatpost and saddle pose above SeatTubeTop, not the seat tube.
		framingOrigins = []pose.Point{
			seatpost.ShaftTop.Origin(), seatpost.RailPivot.Origin(), saddle.RailClamp.Origin(),
			saddle.RailCenter.Origin(), saddle.BRP.Origin(), saddle.SitBones.Origin(),
		}
		framingOutlines = seatParts
		if parsed.BrpTarget != nil {
			extraFramingPts = brpTargetFramingPts(*parsed.BrpTarget)
		}
	case diagramViewStem:
		// Every headset, stem, and handlebar pose above HeadTubeTop, not the
		// head tube. The upper cover outline starts at HeadTubeTop, so its
		// corners keep that end of the stack in view.
		framingOrigins = []pose.Point{
			stem.CoverTop.Origin(), stem.SpacerTop.Origin(), stem.SteererClamp.Origin(), stem.BarClamp.Origin(),
			handlebar.BarCenter.Origin(), handlebar.HandGrip.Origin(),
		}
		framingOutlines = stemParts
		if parsed.GripTarget != nil {
			extraFramingPts = gripTargetFramingPts(*parsed.GripTarget)
		}
	}
	viewBox := wholeBox
	if len(framingOrigins) > 0 {
		framingPts := make([]svgPoint, 0, len(framingOrigins)+4*len(framingOutlines)+len(extraFramingPts))
		for _, p := range framingOrigins {
			framingPts = append(framingPts, toSVG(p))
		}
		for _, o := range framingOutlines {
			framingPts = append(framingPts, o.corners[:]...)
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
		makeSegment("chainstay", placed.RearAxle.Origin(), placed.BB.Origin(), "#2f363d", 6*scale, false),
		makeSegment("seat-tube", placed.SeatAxis.Origin(), placed.SeatTubeTop.Origin(), "#2f363d", 7*scale, true),
		makeSegment("seatstay", placed.RearAxle.Origin(), placed.SeatTubeTop.Origin(), "#2f363d", 4*scale, true),
		makeSegment("head-tube", placed.HeadTubeTop.Origin(), placed.HeadTubeBottom.Origin(), "#2f363d", 9*scale, false),
		makeSegment("lower-cup", placed.HeadTubeBottom.Origin(), placed.LowerCupBottom.Origin(), "#0969da", 9*scale, false),
		makeSegment("fork-a2c", placed.LowerCupBottom.Origin(), placed.ForkSteererBase.Origin(), "#59636e", 7*scale, false),
		makeSegment("fork-rake", placed.ForkSteererBase.Origin(), placed.FrontAxle.Origin(), "#59636e", 5*scale, false),
		makeSegment("seatpost", placed.SeatTubeTop.Origin(), seatpost.ShaftTop.Origin(), "#59636e", 5*scale, false),
		makeSegment("seatpost-setback", seatpost.ShaftTop.Origin(), seatpost.RailPivot.Origin(), "#59636e", 5*scale, false),
		makeSegment("saddle-profile", saddle.SitBones.Origin(), saddle.BRP.Origin(), "#2f363d", 4*scale, true),
		makeSegment("handlebar-rise-sweep", handlebar.BarClamp.Origin(), handlebar.HandGrip.Origin(), "#2f363d", 5*scale, true),
	}

	rects := append(outlineRects(stemParts), outlineRects(seatParts)...)

	landmarkSpecs := []bikeLandmarkSpec{
		{"RearAxle", placed.RearAxle.Origin(), 0, -16, "middle", 1, true, false, false},
		{"RearContact", placed.RearTireContact.Origin(), 0, 18, "middle", 1, true, false, false},
		{"BB", placed.BB.Origin(), 0, 18, "middle", 1, true, false, false},
		{"SeatTubeTop", placed.SeatTubeTop.Origin(), -14, -10, "end", 2, true, true, false},
		{"RailPivot", seatpost.RailPivot.Origin(), 14, 2, "start", 2, false, true, false},
		{"RailCenter", saddle.RailCenter.Origin(), 14, 14, "start", 2, false, true, false},
		{"BRP", saddle.BRP.Origin(), 14, -10, "start", 1, true, true, false},
		{"SitBones", saddle.SitBones.Origin(), -14, -10, "end", 1, true, true, false},
		{"HeadTubeTop", placed.HeadTubeTop.Origin(), 14, 4, "start", 2, true, false, true},
		{"CoverTop", stem.CoverTop.Origin(), -14, 6, "end", 2, false, false, true},
		{"SpacerTop", stem.SpacerTop.Origin(), -14, -2, "end", 2, false, false, true},
		{"SteererClamp", stem.SteererClamp.Origin(), -14, -12, "end", 2, false, false, true},
		{"BarClamp", stem.BarClamp.Origin(), 14, 2, "start", 2, false, false, true},
		{"HandGrip", handlebar.HandGrip.Origin(), -14, -12, "end", 1, true, false, true},
		{"HeadTubeBottom", placed.HeadTubeBottom.Origin(), 14, 2, "start", 3, false, false, false},
		{"LowerCupBottom", placed.LowerCupBottom.Origin(), 14, 14, "start", 3, false, false, false},
		{"FrontAxle", placed.FrontAxle.Origin(), 0, -16, "middle", 1, true, false, false},
		{"FrontContact", placed.FrontTireContact.Origin(), 0, 18, "middle", 1, true, false, false},
	}

	unitsPerPx := viewBox.W / nominalDiagramWidthPx
	targets := buildFitTargets(parsed, bike, view)
	landmarks := make([]svgLandmark, 0, len(landmarkSpecs))
	var labelItems []labelPlaceItem

	for i, spec := range landmarkSpecs {
		o := toSVG(spec.point)
		var show bool
		switch view {
		case diagramViewBike:
			show = spec.bikeMacro
		case diagramViewSeat:
			show = spec.seatLabel
		case diagramViewStem:
			show = spec.stemLabel
		}
		if show && !pointInViewBox(o, viewBox) {
			show = false
		}
		detail := view == diagramViewBike && !spec.bikeMacro
		dotRadius := 5.0
		if detail {
			dotRadius = 3.0
		}
		landmarks = append(landmarks, svgLandmark{
			Name:      spec.name,
			X:         o.X,
			Y:         o.Y,
			DotRadius: dotRadius,
			Detail:    detail,
			ShowLabel: show,
		})
		if show {
			labelItems = append(labelItems, labelPlaceItem{
				text:     spec.name,
				svgX:     o.X,
				svgY:     o.Y,
				prefDx:   spec.prefDx,
				prefDy:   spec.prefDy,
				prefAnc:  spec.prefAnc,
				priority: spec.priority,
				lmIdx:    i,
			})
		}
	}

	for i := range targets {
		tgt := &targets[i]
		if tgt.ShowLabel && !pointInViewBox(svgPoint{X: tgt.X, Y: tgt.Y}, viewBox) {
			tgt.ShowLabel = false
		}
		if tgt.ShowLabel {
			labelItems = append(labelItems, labelPlaceItem{
				text:      tgt.Name,
				svgX:      tgt.X,
				svgY:      tgt.Y,
				prefDx:    14,
				prefDy:    -12,
				prefAnc:   "start",
				priority:  0,
				isTarget:  true,
				targetIdx: i,
			})
		}
	}

	placeDiagramLabels(viewBox, unitsPerPx, scale, segments, landmarks, targets, labelItems)

	viewLinks := make([]diagramViewLink, 0, len(diagramViews))
	for _, dv := range diagramViews {
		viewLinks = append(viewLinks, diagramViewLink{View: dv, URL: parsed.WithView(string(dv)).URL(), Current: dv == view})
	}

	return bikeDiagram{
		View:         view,
		ViewLinks:    viewLinks,
		DrawScale:    scale,
		UnitsPerPx:   unitsPerPx,
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
		FitTargets:   targets,
	}
}
