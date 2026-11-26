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

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/catalog"
	"github.com/matta/bikegeo/internal/compare"
	"github.com/matta/bikegeo/internal/pose"
	"github.com/matta/bikegeo/internal/query"
)

//go:embed templates/bike.html templates/compare.html
var templateFS embed.FS

var (
	bikeTemplate    = template.Must(template.ParseFS(templateFS, "templates/bike.html"))
	compareTemplate = template.Must(template.ParseFS(templateFS, "templates/compare.html"))
)

// NewHandler returns the HTTP handler for the workbench. The chart-consistency check
// runs in the catalog's go test, not at startup (Q91).
func NewHandler() http.Handler {
	return newHandler(query.CatalogFrames)
}

// newHandler is NewHandler with / ranking the frames compareFrames returns.
// Tests pass a few catalog frames trimmed to a size or fork, so a compare page
// ranks a handful of rows instead of the whole catalog.
func newHandler(compareFrames func() []compare.Frame) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		handleCompare(w, r, compareFrames)
	})
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
	FitTargets             []svgFitTarget
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

// buildBikeView renders the placed bike for the parsed query.
func buildBikeView(
	parsed query.Bike,
	bike assembly.BikePlacement,
	basis assembly.FramesetPlacement[assembly.DynamicWorld],
) bikeView {
	diagram := buildBikeDiagram(parsed, bike)
	placed := bike.Frameset
	handlebar := bike.Handlebar

	v := controlsView(parsed, formFromBike(parsed), selectedIDs{
		Frame: parsed.Frame.ID, Size: parsed.Size.Name, Fork: parsed.ForkID, Bar: parsed.Bar.ID, Post: parsed.Post.ID,
	}, "", "")
	v.PageTitle = v.BikeName + " — bikegeo"
	v.HasBuild = true
	v.DefaultsURL = query.DefaultBike().WithView(string(diagram.View)).URL()
	v.BasisForkURL = parsed.WithView(string(diagram.View)).WithBasisFork().URL()
	if parsed.BrpTarget != nil || parsed.GripTarget != nil {
		v.ClearTargetsURL = parsed.WithoutTargets().URL()
	}
	v.View = diagram.View
	v.ViewLinks = diagram.ViewLinks
	v.DrawScale = diagram.DrawScale
	v.UnitsPerPx = diagram.UnitsPerPx
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
	v.WheelRadiusMm = bike.Setup.Wheels.RadiusMm
	v.WheelLabel = catalog.WheelLabel
	v.RearAxleSVG = diagram.RearAxleSVG
	v.FrontAxleSVG = diagram.FrontAxleSVG
	v.GroundY1 = diagram.GroundY1
	v.GroundY2 = diagram.GroundY2
	v.GroundX1 = diagram.GroundX1
	v.GroundX2 = diagram.GroundX2
	v.ViewBox = diagram.ViewBox
	v.Rects = diagram.Rects
	v.Segments = diagram.Segments
	v.Landmarks = diagram.Landmarks
	v.FitTargets = diagram.FitTargets
	return v
}
