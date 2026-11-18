// Package web serves the reboot HTML/SVG workbench.
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

//go:embed templates/stem.html
var templateFS embed.FS

var stemTemplate = template.Must(template.ParseFS(templateFS, "templates/stem.html"))

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
	BarClamp         pose.PoseSnapshot
	BarClampSVG      svgPoint
	BarClampRadiusMm float64
	ViewBox          string
	Rects            []svgRect
	SVGSteps         []svgStep
	Landmarks        []svgLandmark
	Steps            []pose.Step
}

// NewHandler returns the HTTP handler for the reboot workbench.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleStemWorkbench)
	return mux
}

func handleStemWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	const htaDeg = pose.Deg(68.25)
	setup := assembly.StemSetup{
		UpperCoverMm:  11.0,
		SpacersMm:     20.0,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
	}

	tr := pose.NewTrace()
	steererUp := pose.NewPose(pose.Point{X: 0, Y: 0}, 180.0-htaDeg, tr)
	placed, err := assembly.PlaceStem(steererUp, setup)
	tr.Freeze()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view := buildStemView(htaDeg, setup, placed, tr.Steps())

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

	svgSteps := make([]svgStep, 0, len(steps))
	for _, s := range steps {
		step := svgStep{
			ID: s.ID,
			Op: s.Op,
			X1: s.From.Origin.X,
			Y1: -s.From.Origin.Y,
			X2: s.To.Origin.X,
			Y2: -s.To.Origin.Y,
		}
		if s.Op != "advance" {
			step.Path = turnArcPath(s, turnArcRadiusMm)
		}
		svgSteps = append(svgSteps, step)
	}

	landmarks := []svgLandmark{
		makeLandmark("HeadTubeTop", placed.HeadTubeTop, -8, 10, "end"),
		makeLandmark("CoverTop", placed.CoverTop, -8, 2, "end"),
		makeLandmark("SpacerTop", placed.SpacerTop, -8, 2, "end"),
		makeLandmark("SteererClamp", placed.SteererClamp, -8, -4, "end"),
		makeLandmark("BarClamp", placed.BarClamp, 8, -18, "start"),
	}

	return stemView{
		HtaDeg: htaDeg,
		Setup:  setup,
		BarClamp: pose.PoseSnapshot{
			Origin:     placed.BarClamp.Origin(),
			HeadingDeg: placed.BarClamp.HeadingDeg(),
		},
		BarClampSVG: svgPoint{
			X: placed.BarClamp.Origin().X,
			Y: -placed.BarClamp.Origin().Y,
		},
		BarClampRadiusMm: barClampRingRadiusMm,
		ViewBox:          computeViewBox(landmarks),
		Rects:            rects,
		SVGSteps:         svgSteps,
		Landmarks:        landmarks,
		Steps:            steps,
	}
}

func turnArcPath(s pose.Step, radiusMm float64) string {
	delta := float64(s.To.HeadingDeg - s.From.HeadingDeg)
	if math.Abs(delta) < 1e-6 {
		return ""
	}
	cx := s.From.Origin.X
	cy := -s.From.Origin.Y
	x1 := cx + radiusMm*s.From.HeadingDeg.Cos()
	y1 := cy - radiusMm*s.From.HeadingDeg.Sin()
	x2 := cx + radiusMm*s.To.HeadingDeg.Cos()
	y2 := cy - radiusMm*s.To.HeadingDeg.Sin()

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
		x1, y1, radiusMm, radiusMm, largeArc, sweep, x2, y2)
}

func poseRect(name string, p pose.Pose, startMm, lengthMm, thicknessMm float64) svgRect {
	o := p.Origin()
	return svgRect{
		Name:      name,
		X:         startMm,
		Y:         -thicknessMm / 2.0,
		Width:     lengthMm,
		Height:    thicknessMm,
		Transform: fmt.Sprintf("translate(%.2f %.2f) rotate(%.2f)", o.X, -o.Y, -float64(p.HeadingDeg())),
	}
}

func makeLandmark(name string, p pose.Pose, dx, dy float64, anchor string) svgLandmark {
	o := p.Origin()
	return svgLandmark{
		Name:   name,
		StepID: p.StepID(),
		X:      o.X,
		Y:      -o.Y,
		LabelX: o.X + dx,
		LabelY: -o.Y + dy,
		Anchor: anchor,
	}
}

func computeViewBox(landmarks []svgLandmark) string {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, lm := range landmarks {
		minX = min(minX, lm.X)
		maxX = max(maxX, lm.X)
		minY = min(minY, lm.Y)
		maxY = max(maxY, lm.Y)
	}
	const padLeft, padRight, padTop, padBottom = 85.0, 85.0, 45.0, 40.0
	return fmt.Sprintf("%.2f %.2f %.2f %.2f",
		minX-padLeft,
		minY-padTop,
		(maxX-minX)+padLeft+padRight,
		(maxY-minY)+padTop+padBottom,
	)
}
