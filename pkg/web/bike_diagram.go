package web

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

type svgLine struct {
	X1, Y1, X2, Y2 string
	Class          string
}

type svgCircle struct {
	CX, CY, R string
	Class     string
}

type svgPolygon struct {
	Points string
	Class  string
}

type svgEllipse struct {
	CX, CY, RX, RY string
	Transform      string
	Class          string
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
	ViewBox  string
	Polygons []svgPolygon
	Ellipses []svgEllipse
	Lines    []svgLine
	Circles  []svgCircle
	Points   []svgPoint
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

	// Frame tubes & stays terminate at the top of the frame's seat tube (SeatTubeTop),
	// never up at SeatpostClamp inside the saddle.
	stTop := sc.SeatTubeTop()
	lines = append(lines,
		addLine(sc.BB, sc.RearAxle, "frame"),
		addLine(sc.BB, sc.HeadTubeBottom, "frame"),
		addLine(sc.HeadTubeBottom, sc.HeadTubeTop, "frame"),
		addLine(sc.RearAxle, stTop, "frame"),
		addLine(stTop, sc.HeadTubeTop, "frame"),
	)
	if bendPt, ok := sc.SeatTubeBend(); ok {
		lines = append(lines,
			addLine(sc.BB, bendPt, "frame"),
			addLine(bendPt, stTop, "frame"),
		)
	} else {
		lines = append(lines, addLine(sc.BB, stTop, "frame"))
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
		addLine(stTop, sc.SeatpostClamp, "cockpit"),
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

	points := projectSVGPoints([]svgPointSpec{
		{"BB", "BB (0,0)", sc.BB, 12, 22, 7, "frame"},
		{"RA", "RearAxle", sc.RearAxle, -95, 22, 7, "frame"},
		{"FA", "FrontAxle", sc.FrontAxle, 12, 22, 7, "frame"},
		{"STT", "SeatTubeTop", stTop, -125, -8, 6, "frame"},
		{"HTB", "HeadTubeBottom", sc.HeadTubeBottom, 12, 20, 6, "frame"},
		{"HT", "HeadTubeTop", sc.HeadTubeTop, 12, 24, 7, "frame"},
		{"SPC", "SeatpostClamp", sc.SeatpostClamp, 14, 18, 6, "seat"},
		{"RC", "RailCenter", sc.RailCenter, -110, 26, 5, "seat"},
		{"SB", "SitBones", sc.SitBones, -100, -24, 8, "contact"},
		{"BRP", "BRP", sc.BRP, 14, -14, 7, "contact"},
		{"SC", "SteererClamp", sc.SteererClamp, -135, -8, 6, "cockpit"},
		{"STC", "StemClamp", sc.StemClamp, 12, 24, 7, "contact"},
		{"GR", "HandGrip", sc.HandGrip, -110, -24, 8, "contact"},
		{"TP", fmt.Sprintf("TargetPoint (%s)", sc.TargetRef.Label()), sc.TargetPoint, 14, -18, 9, "target"},
		{"RSC", "RequiredStemClamp C*", sc.RequiredStemClamp, 14, 4, 8, "target"},
	})

	return bikeSVGView{
		ViewBox: fmt.Sprintf("%.0f %.0f %.0f %.0f", minX, -maxY, maxX-minX, maxY-minY),
		Lines:   lines,
		Circles: circles,
		Points:  points,
	}
}

type svgPointSpec struct {
	id, label string
	p         geom2d.Point2
	dx, dy    float64
	r         float64
	kind      string
}

func projectSVGPoints(specs []svgPointSpec) []svgPoint {
	f2 := func(x float64) string { return fmt.Sprintf("%.2f", x) }
	points := make([]svgPoint, 0, len(specs))
	for _, spec := range specs {
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
	return points
}

func zoomViewBox(pts []geom2d.Point2, padX, padY, minW, minH float64) string {
	minX, maxX := pts[0].X, pts[0].X
	minY, maxY := pts[0].Y, pts[0].Y
	for _, p := range pts[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	minX -= padX
	maxX += padX
	minY -= padY
	maxY += padY
	if w := maxX - minX; w < minW {
		extra := (minW - w) / 2
		minX -= extra
		maxX += extra
	}
	if h := maxY - minY; h < minH {
		extra := (minH - h) / 2
		minY -= extra
		maxY += extra
	}
	return fmt.Sprintf("%.0f %.0f %.0f %.0f", math.Floor(minX), -math.Ceil(maxY), math.Ceil(maxX-minX), math.Ceil(maxY-minY))
}

func buildCockpitZoomSVG(sc geometry.BikeScene, row model.CanonicalComparisonRow) bikeSVGView {
	f2 := func(x float64) string { return fmt.Sprintf("%.2f", x) }
	addLine := func(a, b geom2d.Point2, cls string) svgLine {
		return svgLine{X1: f2(a.X), Y1: f2(a.Y), X2: f2(b.X), Y2: f2(b.Y), Class: cls}
	}

	brp := model.BRPPoint{BrpX: sc.BRP.X, BrpY: sc.BRP.Y, SitBoneX: sc.SitBones.X, SitBoneY: sc.SitBones.Y}
	evalPt := func(st model.StemSpec, sp float64) geom2d.Point2 {
		hb := geometry.CalculateHBPointWithHandlebar(
			sc.Reach(), sc.Stack(), sc.HtaDeg, brp,
			st, sp, sc.Cockpit.Handlebar, sc.Cockpit.GripInsetMm, sc.Cockpit.BarRollDeg,
		)
		return geometry.EvaluatedPointForHB(hb, sc.TargetRef, sc.Fit.ReferenceBar)
	}

	// Build reachable [MinStem..MaxStem] x [0..MaxSpacerMm] envelope polygon per stem angle.
	stems := row.Bike.Stems
	if len(stems) == 0 {
		stems = []model.StemSpec{sc.Cockpit.Stem}
	}
	type lenRange struct{ minLen, maxLen float64 }
	byAngle := make(map[model.Deg]lenRange)
	var angles []model.Deg
	for _, st := range stems {
		r, exists := byAngle[st.AngleDeg]
		if !exists {
			byAngle[st.AngleDeg] = lenRange{st.LengthMm, st.LengthMm}
			angles = append(angles, st.AngleDeg)
			continue
		}
		if st.LengthMm < r.minLen {
			r.minLen = st.LengthMm
		}
		if st.LengthMm > r.maxLen {
			r.maxLen = st.LengthMm
		}
		byAngle[st.AngleDeg] = r
	}
	slices.Sort(angles)

	maxSpacer := math.Max(0, row.Bike.GetMaxSpacerMm())
	polygons := make([]svgPolygon, 0, len(angles))
	boxPts := []geom2d.Point2{
		sc.HeadTubeTop,
		sc.SteererClamp,
		sc.StemClamp,
		sc.HandGrip,
		sc.TargetPoint,
		sc.RequiredStemClamp,
		{X: sc.TargetPoint.X - 30, Y: sc.TargetPoint.Y - 30},
		{X: sc.TargetPoint.X + 30, Y: sc.TargetPoint.Y + 30},
	}
	for _, ang := range angles {
		lr := byAngle[ang]
		corners := []geom2d.Point2{
			evalPt(model.StemSpec{LengthMm: lr.minLen, AngleDeg: ang}, 0),
			evalPt(model.StemSpec{LengthMm: lr.maxLen, AngleDeg: ang}, 0),
			evalPt(model.StemSpec{LengthMm: lr.maxLen, AngleDeg: ang}, maxSpacer),
			evalPt(model.StemSpec{LengthMm: lr.minLen, AngleDeg: ang}, maxSpacer),
		}
		var ptsStr []string
		for _, c := range corners {
			ptsStr = append(ptsStr, fmt.Sprintf("%.2f,%.2f", c.X, c.Y))
			boxPts = append(boxPts, c)
		}
		polygons = append(polygons, svgPolygon{
			Points: strings.Join(ptsStr, " "),
			Class:  "reach-env",
		})
	}

	// Discrete candidate stock-stem grid dots.
	candHBs := geometry.MapPossibleHBsWithHandlebar(
		row.Bike, sc.Reach(), sc.Stack(), sc.HtaDeg, brp,
		sc.Cockpit.Handlebar, sc.Cockpit.GripInsetMm, sc.Cockpit.BarRollDeg,
	)
	// The cheapest stock build from the ledger scan is drawn last, on top.
	selected := row.Canonical.SelectedHB
	isSelected := func(hb model.HBPoint) bool {
		return selected != nil && hb.Stem.LengthMm == selected.Stem.LengthMm &&
			hb.Stem.AngleDeg == selected.Stem.AngleDeg && hb.SpacerMm == selected.SpacerMm
	}
	circles := make([]svgCircle, 0, len(candHBs))
	var best *svgCircle
	for _, hb := range candHBs {
		p := geometry.EvaluatedPointForHB(hb, sc.TargetRef, sc.Fit.ReferenceBar)
		c := svgCircle{CX: f2(p.X), CY: f2(p.Y), R: "1.80", Class: "cand-dot"}
		if isSelected(hb) {
			c.R, c.Class = "3.20", "cand-dot cand-best"
			best = &c
		} else {
			circles = append(circles, c)
		}
		boxPts = append(boxPts, p)
	}
	if best != nil {
		circles = append(circles, *best)
	}

	// 30-degree rotated FitOval centered on TargetPoint.
	oval := geometry.DefaultFitOval(sc.TargetPoint.X, sc.TargetPoint.Y)
	if cfr := row.Canonical.CockpitFitResult; cfr != nil && cfr.TargetOval.SemiMinorA > 0 && cfr.TargetOval.SemiMajorB > 0 {
		oval.SemiMinorA = cfr.TargetOval.SemiMinorA
		oval.SemiMajorB = cfr.TargetOval.SemiMajorB
		oval.AngleDeg = cfr.TargetOval.AngleDeg
	}
	ellipses := []svgEllipse{{
		CX:        f2(sc.TargetPoint.X),
		CY:        f2(sc.TargetPoint.Y),
		RX:        f2(oval.SemiMinorA),
		RY:        f2(oval.SemiMajorB),
		Transform: fmt.Sprintf("rotate(%.2f %.2f %.2f)", -float64(oval.AngleDeg), sc.TargetPoint.X, sc.TargetPoint.Y),
		Class:     "fit-oval",
	}}

	htStub := sc.HeadTubeTop.SubVec(sc.SteererAxis.Direction.Scale(35))
	steerEnd := sc.HeadTubeTop.Add(sc.SteererAxis.Direction.Scale(95))
	lines := []svgLine{
		addLine(htStub, sc.HeadTubeTop, "frame"),
		addLine(sc.HeadTubeTop, steerEnd, "steer-ray"),
		addLine(sc.HeadTubeTop, sc.SteererClamp, "cockpit"),
		addLine(sc.SteererClamp, sc.StemClamp, "cockpit"),
		addLine(sc.StemClamp, sc.HandGrip, "bar"),
		addLine(sc.StemClamp, sc.RequiredStemClamp, "target-err"),
	}

	points := projectSVGPoints([]svgPointSpec{
		{"HT", "HeadTubeTop", sc.HeadTubeTop, 6, 9, 3.0, "frame"},
		{"SC", "SteererClamp", sc.SteererClamp, -52, -4, 2.6, "cockpit"},
		{"STC", "StemClamp", sc.StemClamp, 6, 9, 3.0, "contact"},
		{"GR", "HandGrip", sc.HandGrip, -42, -9, 3.2, "contact"},
		{"TP", fmt.Sprintf("TargetPoint (%s)", sc.TargetRef.Label()), sc.TargetPoint, 6, -9, 3.5, "target"},
		{"RSC", "RequiredStemClamp C*", sc.RequiredStemClamp, 6, 4, 3.2, "target"},
	})

	return bikeSVGView{
		ViewBox:  zoomViewBox(boxPts, 85, 45, 260, 260),
		Polygons: polygons,
		Ellipses: ellipses,
		Lines:    lines,
		Circles:  circles,
		Points:   points,
	}
}

func buildSaddleZoomSVG(sc geometry.BikeScene) bikeSVGView {
	f2 := func(x float64) string { return fmt.Sprintf("%.2f", x) }
	addLine := func(a, b geom2d.Point2, cls string) svgLine {
		return svgLine{X1: f2(a.X), Y1: f2(a.Y), X2: f2(b.X), Y2: f2(b.Y), Class: cls}
	}

	uSeat := sc.EffectiveSeatAxis.Direction
	uRail := uSeat.Perpendicular(geom2d.CounterClockwise)
	brpPerp := sc.Seat.PostSetbackMm + sc.Seat.RailOffsetMm - sc.Seat.Saddle.RailCenterToBrpMm
	axial := math.Sqrt(math.Max(0, sc.Fit.BrpHeightMm*sc.Fit.BrpHeightMm-brpPerp*brpPerp))
	axisHead := sc.EffectiveSeatAxis.PointAt(axial)
	postBase := axisHead.SubVec(uSeat.Scale(60))
	axisTop := axisHead.Add(uSeat.Scale(35))
	halfWin := sc.Seat.HalfRailWindowMm()
	railWinStart := sc.RailCenter.Add(uRail.Scale(-halfWin))
	railWinEnd := sc.RailCenter.Add(uRail.Scale(halfWin))
	brpCaliperStart := geom2d.Point2{X: sc.SitBones.X, Y: sc.BRP.Y}

	lines := []svgLine{
		addLine(postBase, axisTop, "steer-ray"),
		addLine(postBase, axisHead, "cockpit"),
		addLine(axisHead, sc.SeatpostClamp, "cockpit"),
		addLine(railWinStart, railWinEnd, "rail-win"),
		addLine(sc.SeatpostClamp, sc.RailCenter, "cockpit"),
		addLine(sc.RailCenter, sc.SitBones, "saddle"),
		addLine(sc.SitBones, sc.BRP, "saddle"),
		addLine(brpCaliperStart, sc.BRP, "axis"),
	}

	points := projectSVGPoints([]svgPointSpec{
		{"SPC", "SeatpostClamp", sc.SeatpostClamp, 6, 9, 2.8, "seat"},
		{"RC", "RailCenter", sc.RailCenter, -44, 11, 2.6, "seat"},
		{"SB", "SitBones", sc.SitBones, -38, -8, 3.3, "contact"},
		{"BRP", "BRP (70mm)", sc.BRP, 6, -6, 3.0, "contact"},
	})

	boxPts := []geom2d.Point2{
		postBase,
		axisTop,
		sc.SeatpostClamp,
		sc.RailCenter,
		sc.SitBones,
		sc.BRP,
		railWinStart,
		railWinEnd,
		brpCaliperStart,
	}
	return bikeSVGView{
		ViewBox: zoomViewBox(boxPts, 55, 40, 220, 160),
		Lines:   lines,
		Points:  points,
	}
}
