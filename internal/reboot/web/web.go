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
	seatpostClampOutlineHeight = 10.0 // illustrative rail-clamp head thickness
	saddleRailOutlineHeight    = 5.0  // illustrative saddle rail thickness
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
	mux.HandleFunc("/bike", handleBikeWorkbench)
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
	frameset, err := assembly.PlaceFrameset(fix.Chart.Frame, forkSetup, fix.Wheels, nil)
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

func stemOutlineRects(placed assembly.StemPlacement, setup assembly.StemSetup) []svgRect {
	return []svgRect{
		poseRect("upper-cover", placed.HeadTubeTop, 0, setup.UpperCoverMm, upperCoverOutlineWidthMm),
		poseRect("spacers", placed.CoverTop, 0, setup.SpacersMm, spacerOutlineWidthMm),
		poseRect("steerer-clamp", placed.SteererClamp, -setup.ClampHeightMm/2.0, setup.ClampHeightMm, steererClampOutlineWidthMm),
		poseRect("stem-body", placed.BarClamp, -setup.LengthMm, setup.LengthMm, stemBodyOutlineWidthMm),
	}
}

func buildStemView(htaDeg pose.Deg, setup assembly.StemSetup, placed assembly.StemPlacement, steps []pose.Step) stemView {
	rects := stemOutlineRects(placed, setup)

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

type workbenchState struct {
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
}

func defaultWorkbenchState(fix bikeFixture) workbenchState {
	defaultFork, _ := findForkOption(fix.Forks, fix.DefaultForkID, fix.DefaultForkID)
	return workbenchState{
		ForkID:        defaultFork.ID,
		ForkA2cMm:     defaultFork.Spec.StaticA2cMm,
		LowerCupMm:    fix.DefaultLowerCupMm,
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

func workbenchStateFromSetup(forkID string, staticA2cMm, sagPct float64, setup assembly.BikeSetup) workbenchState {
	return workbenchState{
		ForkID:        forkID,
		ForkA2cMm:     staticA2cMm,
		LowerCupMm:    setup.Fork.LowerCupMm,
		SagPct:        sagPct,
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
	s.ForkID = fix.CadForkID
	s.ForkA2cMm = fix.Chart.CadFork.A2cMm
	s.LowerCupMm = fix.Chart.CadFork.LowerCupMm
	s.SagPct = 0.0
	return s
}

func (s workbenchState) URL() string {
	q := url.Values{}
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
	return "/bike?" + q.Encode()
}

func (s workbenchState) formValues() bikeFormValues {
	return bikeFormValues{
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
	CadBaselineForkURL   string
	MinPostExtensionMm   float64
	MaxPostExtensionMm   float64
	BikeName             string
	SaddleName           string
	HandlebarName        string
	Forks                []forkOption
	SelectedForkID       string
	CadFork              assembly.ForkSetup
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
}

func handleBikeWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/bike" {
		http.NotFound(w, r)
		return
	}

	fix := otsoFenrirFixture()
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
		resolved.State.ForkID,
		resolved.State.ForkA2cMm,
		resolved.State.SagPct,
		bike,
		cadBaselineFork,
		tr.Steps(),
	)
	renderBikeTemplate(w, http.StatusOK, view)
}

func renderBikeError(w http.ResponseWriter, fix bikeFixture, resolved bikeResolvedInput, invalidField, msg string) {
	view := bikeView{
		HasBuild:           false,
		ErrorMessage:       msg,
		InvalidField:       invalidField,
		Form:               resolved.Form,
		CadBaselineForkURL: defaultWorkbenchState(fix).withCadBaselineFork(fix).URL(),
		MinPostExtensionMm: fix.DefaultSeatpost.MinExtensionMm,
		MaxPostExtensionMm: fix.DefaultSeatpost.MaxExtensionMm,
		BikeName:           fix.Name,
		Forks:              fix.Forks,
		SelectedForkID:     resolved.State.ForkID,
		CadFork:            fix.Chart.CadFork,
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

	selectedFork, ok := findForkOption(fix.Forks, q.Get("fork"), fix.DefaultForkID)
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
		selectedFork, _ = findForkOption(fix.Forks, fix.DefaultForkID, fix.DefaultForkID)
	}

	// When the user switches forks via the form, adopt the newly selected fork's
	// default static A2C and sag instead of carrying over the previous fork's values.
	forkSwitched := ok && prevFork != "" && prevFork != selectedFork.ID

	state.LowerCupMm = p.float("cup", defaults.LowerCupMm, "lower cup", &form.Cup)
	state.ForkA2cMm = selectedFork.Spec.StaticA2cMm
	state.SagPct = selectedFork.DefaultSagPct
	if forkSwitched {
		form.ForkA2c = formatFormFloat(state.ForkA2cMm)
		form.Sag = formatFormFloat(state.SagPct)
	} else {
		state.ForkA2cMm = p.float("fork_a2c", state.ForkA2cMm, "fork static A2C", &form.ForkA2c)
		state.SagPct = p.float("sag", state.SagPct, "fork sag", &form.Sag)
	}

	var forkSetup assembly.ForkSetup
	if p.err == nil {
		spec := selectedFork.Spec
		spec.StaticA2cMm = state.ForkA2cMm
		var err error
		forkSetup, err = spec.Setup(state.SagPct, state.LowerCupMm)
		if err != nil {
			p.fail(bikeInputField(err), err)
		}
	}

	state.SpacersMm = p.float("spacers", defaults.SpacersMm, "spacers", &form.Spacers)
	state.StemLengthMm = p.float("stem_length", defaults.StemLengthMm, "stem length", &form.StemLength)
	state.StemAngleDeg = pose.Deg(p.float("stem_angle", float64(defaults.StemAngleDeg), "stem angle", &form.StemAngle))
	state.BarRollDeg = pose.Deg(p.float("bar_roll", float64(defaults.BarRollDeg), "handlebar roll", &form.BarRoll))
	state.PostExtMm = p.float("post_ext", defaults.PostExtMm, "seatpost extension", &form.PostExt)
	state.PostSetbackMm = p.float("post_setback", defaults.PostSetbackMm, "seatpost setback", &form.PostSetback)
	state.RailOffsetMm = p.float("rail_offset", defaults.RailOffsetMm, "saddle rail offset", &form.RailOffset)
	state.RailPitchDeg = pose.Deg(p.float("rail_pitch", float64(defaults.RailPitchDeg), "saddle rail pitch", &form.RailPitch))

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
	default:
		return ""
	}
}

func formatFormFloat(v float64) string {
	if v == 0 {
		return "0.0"
	}
	if math.Abs(v*10-math.Round(v*10)) < 1e-9 {
		return fmt.Sprintf("%.1f", v)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
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

func buildBikeView(
	fix bikeFixture,
	forkID string,
	staticA2cMm float64,
	sagPct float64,
	bike assembly.BikePlacement,
	cadBaselineFork assembly.ChartBaseline,
	steps []pose.Step,
) bikeView {
	placed := bike.Frameset
	seatpost := bike.Seatpost
	saddle := bike.Saddle
	stem := bike.Stem
	handlebar := bike.Handlebar

	// seat-tube (straight along effective STA), seatstay (to SeatTubeTop),
	// saddle-profile (SitBones to BRP), and handlebar-rise-sweep (side projection
	// from BarClamp to HandGrip) are illustrative schematic lines rather than
	// measured physical outlines.
	segments := []svgSegment{
		makeSegment("chainstay", placed.RearAxle, placed.BB, "#2f363d", 6, false),
		makeSegment("seat-tube", placed.SeatAxis, placed.SeatTubeTop, "#2f363d", 7, true),
		makeSegment("seatstay", placed.RearAxle, placed.SeatTubeTop, "#2f363d", 4, true),
		makeSegment("head-tube", placed.HeadTubeTop, placed.HeadTubeBottom, "#2f363d", 9, false),
		makeSegment("lower-cup", placed.HeadTubeBottom, placed.LowerCupBottom, "#0969da", 9, false),
		makeSegment("fork-a2c", placed.LowerCupBottom, placed.ForkSteererBase, "#59636e", 7, false),
		makeSegment("fork-rake", placed.ForkSteererBase, placed.FrontAxle, "#59636e", 5, false),
		makeSegment("seatpost", placed.SeatTubeTop, seatpost.ShaftTop, "#59636e", 5, false),
		makeSegment("seatpost-setback", seatpost.ShaftTop, seatpost.SeatpostClamp, "#59636e", 5, false),
		makeSegment("saddle-profile", saddle.SitBones, saddle.BRP, "#2f363d", 4, true),
		makeSegment("handlebar-rise-sweep", handlebar.BarClamp, handlebar.HandGrip, "#2f363d", 5, true),
	}

	rects := append(stemOutlineRects(stem, bike.Setup.Stem),
		poseRect("seatpost-clamp", saddle.RailFrame, -bike.Setup.Seatpost.ClampLengthMm/2.0, bike.Setup.Seatpost.ClampLengthMm, seatpostClampOutlineHeight),
		poseRect("saddle-rail", saddle.RailCenter, -bike.Setup.Saddle.RailLengthMm/2.0, bike.Setup.Saddle.RailLengthMm, saddleRailOutlineHeight),
	)

	landmarks := []svgLandmark{
		makeLandmark("RearAxle", placed.RearAxle, 0, -14, "middle"),
		makeLandmark("RearContact", placed.RearTireContact, 0, 24, "middle"),
		makeLandmark("BB", placed.BB, 0, 26, "middle"),
		makeLandmark("SeatTubeTop", placed.SeatTubeTop, -14, -10, "end"),
		makeLandmark("SeatpostClamp", seatpost.SeatpostClamp, 16, 6, "start"),
		makeLandmark("RailCenter", saddle.RailCenter, 16, 24, "start"),
		makeLandmark("BRP", saddle.BRP, 14, -10, "start"),
		makeLandmark("SitBones", saddle.SitBones, -14, -10, "end"),
		makeLandmark("HeadTubeTop", placed.HeadTubeTop, 14, 6, "start"),
		makeLandmark("CoverTop", stem.CoverTop, -14, 8, "end"),
		makeLandmark("SpacerTop", stem.SpacerTop, -14, -2, "end"),
		makeLandmark("SteererClamp", stem.SteererClamp, -14, -14, "end"),
		makeLandmark("BarClamp", stem.BarClamp, 14, 4, "start"),
		makeLandmark("HandGrip", handlebar.HandGrip, -14, -12, "end"),
		makeLandmark("HeadTubeBottom", placed.HeadTubeBottom, 16, 4, "start"),
		makeLandmark("LowerCupBottom", placed.LowerCupBottom, 16, 18, "start"),
		makeLandmark("FrontAxle", placed.FrontAxle, 0, -14, "middle"),
		makeLandmark("FrontContact", placed.FrontTireContact, 0, 24, "middle"),
	}

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

	state := workbenchStateFromSetup(forkID, staticA2cMm, sagPct, bike.Setup)

	return bikeView{
		HasBuild:             true,
		Form:                 state.formValues(),
		CadBaselineForkURL:   state.withCadBaselineFork(fix).URL(),
		MinPostExtensionMm:   bike.Setup.Seatpost.MinExtensionMm,
		MaxPostExtensionMm:   bike.Setup.Seatpost.MaxExtensionMm,
		BikeName:             fix.Name,
		SaddleName:           fix.SaddleName,
		HandlebarName:        fix.HandlebarName,
		Forks:                fix.Forks,
		SelectedForkID:       forkID,
		CadFork:              fix.Chart.CadFork,
		LowerCupMm:           bike.Setup.Fork.LowerCupMm,
		SagPct:               sagPct,
		EffectiveA2cMm:       bike.Setup.Fork.A2cMm,
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
		ViewBox:              computeViewBoxFromPoints(pts, 60.0, 60.0, 50.0, 45.0),
		Rects:                rects,
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
