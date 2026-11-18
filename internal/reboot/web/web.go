// Package web serves the reboot HTML/SVG workbench.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	stemTemplate    = template.Must(template.ParseFS(templateFS, "templates/stem.html", "templates/trace_steps.html"))
	chassisTemplate = template.Must(template.ParseFS(templateFS, "templates/chassis.html", "templates/trace_steps.html"))
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
	ID   int
	Op   string
	X1   float64
	Y1   float64
	X2   float64
	Y2   float64
	Path string
}

type svgLandmark struct {
	Name   string
	StepID int
	X      float64
	Y      float64
	LabelX float64
	LabelY float64
	Anchor string
}

// Illustrative outline dimensions (not measured hardware geometry) used only
// to draw component silhouettes in the SVG workbench.
const (
	upperCoverOutlineWidthMm   = 46.0
	spacerOutlineWidthMm       = 34.0
	steererClampOutlineWidthMm = 38.0
	stemBodyOutlineWidthMm     = 30.0
	barClampRingRadiusMm       = 15.9 // nominal 31.8 mm clamp diameter; illustrative, not measured
	turnArcRadiusMm            = 12.0 // illustrative radius for turn-step SVG arcs
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
	mux.HandleFunc("/chassis", handleChassisWorkbench)
	return mux
}

func handleStemWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	fix := otsoFenrirFixture()
	defaultFork, ok := findForkOption(fix.Forks, fix.DefaultForkID, fix.DefaultForkID)
	if !ok {
		http.Error(w, "missing default fork option", http.StatusInternalServerError)
		return
	}
	forkSetup, err := defaultFork.Spec.Setup(defaultFork.DefaultSagPct, fix.DefaultLowerCupMm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	chassis, err := assembly.PlaceChassis(fix.Chart.Frame, forkSetup, fix.Wheels, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Slice 1 stem-only preview roots a fresh trace at the leveled HeadTubeTop;
	// Slice 3 will trace one unified tree from BB across chassis and cockpits.
	tr := pose.NewTrace()
	steererUp := pose.NewPose(chassis.HeadTubeTop.Origin(), chassis.HeadTubeTop.HeadingDeg(), tr)
	placed, err := assembly.PlaceStem(steererUp, fix.DefaultStem)
	tr.Freeze()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildStemView(chassis.LeveledHeadTubeAngleDeg(), fix.DefaultStem, placed, tr.Steps())

	var buf bytes.Buffer
	if err := stemTemplate.Execute(&buf, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func buildStemView(htaDeg pose.Deg, setup assembly.StemSetup, placed assembly.StemPlacement, steps []pose.Step) stemView {
	rects := []svgRect{
		poseRect("upper-cover", placed.HeadTubeTop, 0, setup.UpperCoverMm, upperCoverOutlineWidthMm),
		poseRect("spacers", placed.CoverTop, 0, setup.SpacersMm, spacerOutlineWidthMm),
		poseRect("steerer-clamp", placed.SteererClamp, -setup.ClampHeightMm/2.0, setup.ClampHeightMm, steererClampOutlineWidthMm),
		poseRect("stem-body", placed.BarClamp, -setup.LengthMm, setup.LengthMm, stemBodyOutlineWidthMm),
	}

	svgSteps := buildSVGSteps(steps, turnArcRadiusMm)

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
		ViewBox:          computeViewBoxFromPoints(pts, 85.0, 85.0, 45.0, 40.0),
		Rects:            rects,
		TraceSteps: svgTraceSteps{
			Steps:         svgSteps,
			LineWidth:     1.8,
			ArcWidth:      1.5,
			UseMarkers:    true,
			ZeroArcRadius: 4.5,
		},
		Landmarks: landmarks,
		Steps:     steps,
	}
}

func buildSVGSteps(steps []pose.Step, turnRadiusMm float64) []svgStep {
	svgSteps := make([]svgStep, 0, len(steps))
	for _, s := range steps {
		from := toSVG(s.From.Origin)
		to := toSVG(s.To.Origin)
		step := svgStep{
			ID: s.ID,
			Op: s.Op,
			X1: from.X,
			Y1: from.Y,
			X2: to.X,
			Y2: to.Y,
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

func computeViewBoxFromPoints(pts []svgPoint, padLeft, padRight, padTop, padBottom float64) string {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, pt := range pts {
		minX = min(minX, pt.X)
		maxX = max(maxX, pt.X)
		minY = min(minY, pt.Y)
		maxY = max(maxY, pt.Y)
	}
	return fmt.Sprintf("%.2f %.2f %.2f %.2f",
		minX-padLeft,
		minY-padTop,
		(maxX-minX)+padLeft+padRight,
		(maxY-minY)+padTop+padBottom,
	)
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

type chassisView struct {
	BikeName             string
	Forks                []forkOption
	SelectedForkID       string
	PrevFork             string
	IsCadBaseline        bool
	CadFork              assembly.ForkSetup
	LowerCupMm           float64
	SagPct               float64
	EffectiveA2cMm       float64
	PitchDeg             pose.Deg
	LeveledHtaDeg        pose.Deg
	LeveledStaDeg        pose.Deg
	WheelbaseMm          float64
	BbDropLeveledMm      float64
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
	Segments             []svgSegment
	TraceSteps           svgTraceSteps
	Landmarks            []svgLandmark
	Steps                []pose.Step
}

func handleChassisWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chassis" {
		http.NotFound(w, r)
		return
	}

	fix := otsoFenrirFixture()
	q := r.URL.Query()

	isCadBaseline := q.Get("baseline") == "cad"
	var (
		selectedForkID string
		prevForkForUI  string
		sagPct         float64
		forkSetup      assembly.ForkSetup
	)

	if isCadBaseline {
		forkSetup = fix.Chart.CadFork
		prevForkForUI = "cad"
	} else {
		selectedFork, ok := findForkOption(fix.Forks, q.Get("fork"), fix.DefaultForkID)
		if !ok {
			http.Error(w, "unknown fork option", http.StatusBadRequest)
			return
		}

		lowerCupMm := fix.DefaultLowerCupMm
		if rawCup := q.Get("cup"); rawCup != "" {
			parsed, err := strconv.ParseFloat(rawCup, 64)
			if err != nil {
				http.Error(w, "invalid lower cup value", http.StatusBadRequest)
				return
			}
			lowerCupMm = parsed
		}

		// When the user switches forks via the form, adopt the newly selected fork's
		// default sag instead of carrying over the previous fork's sag field.
		prevFork := q.Get("prev_fork")
		forkSwitched := prevFork != "" && prevFork != selectedFork.ID

		sagPct = selectedFork.DefaultSagPct
		if rawSag := q.Get("sag"); rawSag != "" && !forkSwitched {
			parsed, err := strconv.ParseFloat(rawSag, 64)
			if err != nil {
				http.Error(w, "invalid sag percentage", http.StatusBadRequest)
				return
			}
			sagPct = parsed
		}

		var err error
		forkSetup, err = selectedFork.Spec.Setup(sagPct, lowerCupMm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		selectedForkID = selectedFork.ID
		prevForkForUI = selectedFork.ID
	}

	tr := pose.NewTrace()
	placed, err := assembly.PlaceChassis(fix.Chart.Frame, forkSetup, fix.Wheels, tr)
	tr.Freeze()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cadBaseline, err := assembly.PlaceCadBaseline(fix.Chart, fix.Wheels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildChassisView(fix, selectedForkID, prevForkForUI, isCadBaseline, sagPct, forkSetup, placed, cadBaseline, tr.Steps())

	var buf bytes.Buffer
	if err := chassisTemplate.Execute(&buf, view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func findForkOption(forks []forkOption, requestedID, defaultID string) (forkOption, bool) {
	target := requestedID
	if target == "" {
		target = defaultID
	}
	for _, f := range forks {
		if f.ID == target {
			return f, true
		}
	}
	return forkOption{}, false
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

func buildChassisView(
	fix bikeFixture,
	forkID string,
	prevFork string,
	isCadBaseline bool,
	sagPct float64,
	fork assembly.ForkSetup,
	placed assembly.ChassisPlacement,
	cadBaseline assembly.ChartBaseline,
	steps []pose.Step,
) chassisView {
	// seat-tube (straight along effective STA) and seatstay (to SeatTubeTop)
	// are illustrative schematic lines rather than measured tube outlines.
	segments := []svgSegment{
		makeSegment("chainstay", placed.RearAxle, placed.BB, "#2f363d", 6, false),
		makeSegment("seat-tube", placed.SeatAxis, placed.SeatTubeTop, "#2f363d", 7, true),
		makeSegment("seatstay", placed.RearAxle, placed.SeatTubeTop, "#2f363d", 4, true),
		makeSegment("head-tube", placed.HeadTubeTop, placed.HeadTubeBottom, "#2f363d", 9, false),
		makeSegment("lower-cup", placed.HeadTubeBottom, placed.LowerCupBottom, "#0969da", 9, false),
		makeSegment("fork-a2c", placed.LowerCupBottom, placed.ForkSteererBase, "#59636e", 7, false),
		makeSegment("fork-rake", placed.ForkSteererBase, placed.FrontAxle, "#59636e", 5, false),
	}

	landmarks := []svgLandmark{
		makeLandmark("RearAxle", placed.RearAxle, 0, -14, "middle"),
		makeLandmark("RearContact", placed.RearTireContact, 0, 24, "middle"),
		makeLandmark("BB", placed.BB, 0, 26, "middle"),
		makeLandmark("SeatTubeTop", placed.SeatTubeTop, -14, -10, "end"),
		makeLandmark("HeadTubeTop", placed.HeadTubeTop, 14, -10, "start"),
		makeLandmark("HeadTubeBottom", placed.HeadTubeBottom, 16, 4, "start"),
		makeLandmark("LowerCupBottom", placed.LowerCupBottom, 16, 18, "start"),
		makeLandmark("FrontAxle", placed.FrontAxle, 0, -14, "middle"),
		makeLandmark("FrontContact", placed.FrontTireContact, 0, 24, "middle"),
	}

	r := fix.Wheels.RadiusMm
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
	}

	return chassisView{
		BikeName:             fix.Name,
		Forks:                fix.Forks,
		SelectedForkID:       forkID,
		PrevFork:             prevFork,
		IsCadBaseline:        isCadBaseline,
		CadFork:              fix.Chart.CadFork,
		LowerCupMm:           fork.LowerCupMm,
		SagPct:               sagPct,
		EffectiveA2cMm:       fork.A2cMm,
		PitchDeg:             placed.PitchDeg(),
		LeveledHtaDeg:        placed.LeveledHeadTubeAngleDeg(),
		LeveledStaDeg:        placed.LeveledSeatTubeAngleDeg(),
		WheelbaseMm:          placed.WheelbaseMm(),
		BbDropLeveledMm:      placed.LeveledBbDropMm(),
		PublishedWheelbaseMm: fix.Chart.PublishedWheelbaseMm,
		CadHtaDeg:            cadBaseline.Placement.LeveledHeadTubeAngleDeg(),
		CadWheelbaseMm:       cadBaseline.Placement.WheelbaseMm(),
		DeltaPitchDeg:        cadBaseline.DeltaPitchDeg(placed),
		ResidualAxleYMm:      cadBaseline.Residuals.AxleHeightDiffMm,
		ResidualWheelbaseMm:  cadBaseline.Residuals.WheelbaseDiffMm,
		WheelRadiusMm:        r,
		WheelLabel:           fix.WheelLabel,
		RearAxleSVG:          rearSVG,
		FrontAxleSVG:         frontSVG,
		GroundY1:             rearContactSVG.Y,
		GroundY2:             frontContactSVG.Y,
		GroundX1:             rearContactSVG.X - r - 40,
		GroundX2:             frontContactSVG.X + r + 40,
		ViewBox:              computeViewBoxFromPoints(pts, 60.0, 60.0, 50.0, 45.0),
		Segments:             segments,
		TraceSteps: svgTraceSteps{
			Steps:         buildSVGSteps(steps, 40.0),
			LineWidth:     2.0,
			ArcWidth:      2.5,
			DashArray:     "6 4",
			Opacity:       0.7,
			ZeroArcRadius: 12.0,
		},
		Landmarks: landmarks,
		Steps:     steps,
	}
}
