package geometry

import (
	"fmt"
	"math"

	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// TargetRef names the physical cockpit point a fit TargetPoint refers to.
// Defined in pkg/model so geometry, ideal, and compare share one type without import cycles.
type TargetRef = model.TargetRef

const (
	// TargetHandGrip indicates TargetPoint is the rider's physical hand contact center (HandGrip).
	TargetHandGrip = model.TargetHandGrip
	// TargetStemClamp indicates TargetPoint is the handlebar/stem clamp center directly (StemClamp).
	TargetStemClamp = model.TargetStemClamp
	// TargetReferenceBarClamp indicates TargetPoint is the stem clamp center of RiderFit.ReferenceBar
	// on the fit bike, reproducing hand position through ReferenceBar's sweep/rise offset.
	// TODO: this this strictly necessary? It seems like a YAGNI thing.
	TargetReferenceBarClamp = model.TargetReferenceBarClamp
)

// Fork specifies a rigid or suspension fork with typed travel and static geometry.
type Fork struct {
	ID          string
	StaticA2cMm float64
	RakeMm      float64
	TravelMm    float64
}

// EffectiveA2c returns the fork's axle-to-crown length in mm at sagPct (0–100).
// Rigid forks (TravelMm == 0) always return StaticA2cMm.
func (f Fork) EffectiveA2c(sagPct float64) float64 {
	if f.TravelMm > 0 && sagPct > 0 {
		return f.StaticA2cMm - f.TravelMm*(sagPct/100.0)
	}
	return f.StaticA2cMm
}

// FrameSize holds the published geometry chart values for one frame size together
// with the declared CAD fork and CAD lower headset cup baseline.
type FrameSize struct {
	ID   string
	Size string

	ReachMm     float64
	StackMm     float64
	HtaDeg      model.Deg
	StaDeg      model.Deg
	HtlMm       float64
	BbDropMm    float64
	ChainstayMm float64

	ActualStaDeg      model.Deg
	SeatTubeOffsetXMm model.Opt[float64]

	SeatTubeLengthMm model.Opt[float64]
	WheelbaseMm      model.Opt[float64]
	FrontCenterMm    model.Opt[float64]

	CadFork       Fork
	CadSagPct     float64
	CadLowerCupMm float64
	Precision     model.ChartPrecision
}

// HasBentSeatTube reports whether the frame has an actual lower seat-tube angle
// differing from its nominal StaDeg (or an explicit seat-tube offset).
func (f FrameSize) HasBentSeatTube() bool {
	if f.ActualStaDeg <= 0 {
		return false
	}
	return (f.ActualStaDeg-f.StaDeg).Abs() > 0.001 ||
		(f.SeatTubeOffsetXMm.IsSet() && math.Abs(f.SeatTubeOffsetXMm.OrZero()) > 1e-9)
}

// SeatTubeOffsetX returns the horizontal bottom-bracket offset Ox in mm for a bent seat tube.
func (f FrameSize) SeatTubeOffsetX() float64 {
	if v, ok := f.SeatTubeOffsetXMm.Get(); ok {
		return v
	}
	if !f.HasBentSeatTube() {
		return 0
	}
	return CalculateBentSeatTubeOffset(f.StackMm, f.ActualStaDeg, f.StaDeg)
}

// EffectiveStaDegAt returns the effective seat-tube angle at seatHeight in the CAD attitude.
func (f FrameSize) EffectiveStaDegAt(seatHeight float64) model.Deg {
	if !f.HasBentSeatTube() || seatHeight <= 0 {
		return f.StaDeg
	}
	return CalculateEffectiveSta(f.ActualStaDeg, f.SeatTubeOffsetX(), seatHeight)
}

// FrameSizeFromBikeGeometry translates a materialized model.BikeGeometry into an explicit FrameSize.
// When CadFork.A2cMm is unset in legacy synthetic fixtures, it resolves CAD A2C from ForkA2c if within
// A2cBudget, or from Stack-implied geometric A2C (L_chart - HTL).
func FrameSizeFromBikeGeometry(bike model.BikeGeometry) (FrameSize, error) {
	cadRake := bike.GetCadRake()
	if !isPositive(cadRake) {
		return FrameSize{}, fmt.Errorf("frame %q requires positive CAD fork rake, got %v", bike.ID, cadRake)
	}

	var cadFork Fork
	var cadSag float64
	if bike.CadFork.A2cMm > 0 {
		cadSag = bike.CadFork.SagPct
		cadFork = Fork{
			ID:          bike.CadFork.Fork,
			StaticA2cMm: bike.CadFork.StaticA2c(),
			RakeMm:      cadRake,
			TravelMm:    bike.CadFork.TravelMm,
		}
	} else if budget, ok := A2cBudget(bike); ok && bike.ForkA2c > 0 && budget.Within() {
		cadFork = Fork{
			StaticA2cMm: bike.ForkA2c,
			RakeMm:      cadRake,
		}
	} else {
		structA2c, err := CalculateGeometricA2c(bike.Stack, bike.BbDrop, bike.Htl, bike.HtaDeg, cadRake)
		if err != nil {
			return FrameSize{}, fmt.Errorf("frame %q cannot infer CAD A2C: %w", bike.ID, err)
		}
		cadFork = Fork{
			StaticA2cMm: structA2c - bike.GeoHeadsetSpacerMm,
			RakeMm:      cadRake,
		}
	}

	var stlOpt, wbOpt, fcOpt model.Opt[float64]
	if bike.SeatTubeLength > 0 {
		stlOpt = model.Some(bike.SeatTubeLength)
	}
	if bike.Wheelbase > 0 {
		wbOpt = model.Some(bike.Wheelbase)
	}
	if bike.FrontCenter > 0 {
		fcOpt = model.Some(bike.FrontCenter)
	}

	return FrameSize{
		ID:                bike.ID,
		Size:              bike.Size,
		ReachMm:           bike.Reach,
		StackMm:           bike.Stack,
		HtaDeg:            bike.HtaDeg,
		StaDeg:            bike.StaDeg,
		HtlMm:             bike.Htl,
		BbDropMm:          bike.BbDrop,
		ChainstayMm:       bike.ChainstayLength,
		ActualStaDeg:      bike.ActualStaDeg,
		SeatTubeOffsetXMm: bike.SeatTubeOffsetXMm,
		SeatTubeLengthMm:  stlOpt,
		WheelbaseMm:       wbOpt,
		FrontCenterMm:     fcOpt,
		CadFork:           cadFork,
		CadSagPct:         cadSag,
		CadLowerCupMm:     bike.GeoHeadsetSpacerMm,
		Precision:         ChartPrecisionFor(bike),
	}, nil
}

// BuildSpec specifies the installed fork, sag percentage, physical lower headset cup, and wheel radii.
type BuildSpec struct {
	Fork               Fork
	SagPct             float64
	PhysicalLowerCupMm float64
	WheelRadiusFrontMm float64
	WheelRadiusRearMm  float64
}

// CockpitSpec specifies the upper headset stack, steerer spacers, stem, and handlebar setup.
type CockpitSpec struct {
	HeadsetTopStackMm float64
	SpacerMm          float64
	Stem              model.StemSpec
	StemClampHeightMm float64
	Handlebar         model.HandlebarSpec
	BarRollDeg        model.Deg
	GripInsetMm       float64
}

// SeatSpec specifies the seatpost setback, clamp length, saddle model, and rail clamp offset.
type SeatSpec struct {
	PostSetbackMm float64
	ClampLengthMm float64
	Saddle        model.SaddleSpec
	RailOffsetMm  float64
}

// HalfRailWindowMm returns half the modeled travel for a valid scene's clamp.
func (s SeatSpec) HalfRailWindowMm() float64 {
	_, half, _ := s.Saddle.ClampWindowMm(s.ClampLengthMm)
	return half
}

// RiderTarget groups the cockpit target coordinate with the physical point it describes.
type RiderTarget struct {
	Point geom2d.Point2
	Ref   TargetRef
}

// DefaultRiderTarget returns the standard target coordinate for a cockpit reference point.
func DefaultRiderTarget(ref TargetRef) RiderTarget {
	var pt geom2d.Point2
	switch ref {
	case TargetHandGrip, TargetReferenceBarClamp:
		pt = geom2d.Point2{X: 480.9, Y: 726.1}
	case TargetStemClamp:
		pt = geom2d.Point2{X: 480.9, Y: 676.1}
	}
	return RiderTarget{Point: pt, Ref: ref}
}

// RiderFit specifies the rider's biomechanical targets and explicit cockpit reference point.
type RiderFit struct {
	// FrameSeatAxisHeightMm is the radial distance from BB (0,0) along the active frame seat-tube
	// axis to the frame-only SeatAxisPoint (e.g. 746 mm), used for frame-only AETT and setback metrics.
	FrameSeatAxisHeightMm float64

	// BrpHeightMm is the radial distance from BB (0,0) to the saddle 70mm BRP point.
	BrpHeightMm float64

	// TargetSitBonesXMm is the target X coordinate of SitBones relative to BB (negative = rearward).
	TargetSitBonesXMm float64

	// Target is the 2D cockpit coordinate relative to BB (0,0), paired with its physical reference point.
	Target RiderTarget

	// ReferenceSaddle is the saddle used to record reference biomechanical coordinates.
	ReferenceSaddle model.SaddleSpec

	// ReferenceBar is the handlebar on the fit bike; required when TargetRef == TargetReferenceBarClamp.
	ReferenceBar model.Opt[model.HandlebarSpec]
}

// IdmatchSaddleFit carries the reference support point at the requested reference
// BRP height onto the active saddle. Frame-only dimensions still use referenceHeightMm.
func IdmatchSaddleFit(referenceHeightMm float64, saddle model.SaddleSpec) RiderFit {
	ref := model.ReferenceIdmatchSaddle()
	sit := model.ReferenceSitBonePoint(referenceHeightMm, model.DefaultTargetSitBoneSetbackMm())
	return RiderFit{
		FrameSeatAxisHeightMm: referenceHeightMm,
		BrpHeightMm:           saddle.BrpPointFromSitBones(sit).DistanceToOrigin(),
		TargetSitBonesXMm:     sit.X,
		ReferenceSaddle:       ref,
	}
}

// RequiredStemClamp converts fit.Target into the required 2D StemClamp coordinate C*
// for the configured handlebar, grip inset, and roll.
func RequiredStemClamp(fit RiderFit, cockpit CockpitSpec) (geom2d.Point2, error) {
	if err := validateCockpitTargetInputs(fit, cockpit); err != nil {
		return geom2d.Point2{}, err
	}
	barOff, err := GetHandlebarOffset(cockpit.Handlebar, cockpit.GripInsetMm, cockpit.BarRollDeg)
	if err != nil {
		return geom2d.Point2{}, err
	}
	barVec := geom2d.Vec2{X: barOff.X, Y: barOff.Y}

	switch fit.Target.Ref {
	case TargetHandGrip:
		return fit.Target.Point.SubVec(barVec), nil
	case TargetStemClamp:
		return fit.Target.Point, nil
	case TargetReferenceBarClamp:
		refBar, _ := fit.ReferenceBar.Get()
		refOff, err := GetHandlebarOffset(refBar, cockpit.GripInsetMm, 0.0)
		if err != nil {
			return geom2d.Point2{}, err
		}
		refVec := geom2d.Vec2{X: refOff.X, Y: refOff.Y}
		return fit.Target.Point.Add(refVec).SubVec(barVec), nil
	default:
		return geom2d.Point2{}, fmt.Errorf("invalid TargetRef %q", fit.Target.Ref)
	}
}

// BikeScene holds the complete sagittal 2D rigid-body kinematics of a single bike build.
// Coordinates use +X forward and +Y upward with Bottom Bracket BB fixed at (0, 0).
type BikeScene struct {
	Frame   FrameSize
	Build   BuildSpec
	Cockpit CockpitSpec
	Seat    SeatSpec
	Fit     RiderFit

	// Leveled structural landmarks (BB == (0,0), FA.Y == RA.Y == BbDrop)
	BB             geom2d.Point2
	RearAxle       geom2d.Point2
	HeadTubeTop    geom2d.Point2
	HeadTubeBottom geom2d.Point2
	LowerCupBottom geom2d.Point2
	FrontAxle      geom2d.Point2

	// Un-pitched chart & closure validation landmarks (BB_0 == (0,0))
	ChartL         float64
	ChartFrontAxle geom2d.Point2
	DeclFrontAxle  geom2d.Point2
	ResidualA2cMm  float64
	ResidualFcMm   model.Opt[float64]
	ResidualWbMm   model.Opt[float64]

	// Leveled directional axes
	SteererAxis       geom2d.Axis2
	NominalSeatAxis   geom2d.Axis2
	FrameSeatAxis     geom2d.Axis2
	EffectiveSeatAxis geom2d.Axis2

	//Leveled rear cockpit landmarks
	SeatAxisPoint        geom2d.Point2
	SeatpostClamp        geom2d.Point2
	SaddlePlacement      model.BRPPoint
	RailCenter           geom2d.Point2
	BRP                  geom2d.Point2
	SitBones             geom2d.Point2
	RequiredRailOffsetMm float64
	RailMarginMm         float64

	// Leveled front cockpit landmarks
	SteererClamp      geom2d.Point2
	StemClamp         geom2d.Point2
	HandGrip          geom2d.Point2
	RequiredStemClamp geom2d.Point2

	// Active leveled angles & fork lengths
	PitchDeg        model.Deg
	HtaDeg          model.Deg
	NominalStaDeg   model.Deg
	SeatAxisStaDeg  model.Deg
	EffectiveStaDeg model.Deg
	ForkEffA2cMm    float64
	StructuralA2cMm float64
}

// frameLandmarks2D holds rigid 2D frame landmarks and head-tube angle in BB-relative coordinates.
type frameLandmarks2D struct {
	BB             geom2d.Point2
	RearAxle       geom2d.Point2
	HeadTubeTop    geom2d.Point2
	HeadTubeBottom geom2d.Point2
	FrontAxle      geom2d.Point2
	HtaDeg         model.Deg
}

// frameLevelingInput specifies a rigid frame configuration and fork/rake changes to level around the rear axle.
type frameLevelingInput struct {
	Initial                frameLandmarks2D
	DeltaStructuralA2cMm   float64
	DeltaRakeMm            float64
	WheelbaseBeforePitchMm float64
}

// frameLevelingResult holds the leveled frame landmarks (re-centered with BB at Origin) and counter-clockwise pitch angle.
type frameLevelingResult struct {
	Leveled  frameLandmarks2D
	PitchDeg model.Deg
}

// frontCockpitPoints holds the positioned 2D cockpit points computed from the leveled head-tube top.
type frontCockpitPoints struct {
	SteererClamp geom2d.Point2
	StemClamp    geom2d.Point2
	HandGrip     geom2d.Point2
}

// levelFrame2D pitches rigid frame landmarks around the rear axle so both axles are level,
// then shifts BB back to Origin (0,0).
func levelFrame2D(in frameLevelingInput) frameLevelingResult {
	init := in.Initial
	u0 := SteererTubeDir(init.HtaDeg)
	n0 := u0.Perpendicular(geom2d.Clockwise)

	var theta geom2d.Angle
	if in.DeltaStructuralA2cMm != 0 || in.DeltaRakeMm != 0 {
		dyAxle := -in.DeltaStructuralA2cMm*u0.Y + in.DeltaRakeMm*n0.Y
		theta = -geom2d.AngleFromSlope(in.WheelbaseBeforePitchMm, dyAxle)
	}

	out := frameLandmarks2D{
		BB:             geom2d.Origin(),
		RearAxle:       init.RearAxle,
		HeadTubeTop:    init.HeadTubeTop,
		HeadTubeBottom: init.HeadTubeBottom,
		FrontAxle:      init.FrontAxle,
		HtaDeg:         init.HtaDeg,
	}
	if theta != 0 {
		rot := geom2d.NewRotation(init.RearAxle, theta)
		bbRot := rot.Apply(init.BB)
		shift := geom2d.Origin().Sub(bbRot)

		out.RearAxle = init.RearAxle.Add(shift)
		out.HeadTubeTop = rot.Apply(init.HeadTubeTop).Add(shift)
		out.HeadTubeBottom = rot.Apply(init.HeadTubeBottom).Add(shift)
		out.FrontAxle = rot.Apply(init.FrontAxle).Add(shift)
	}

	pitchDeg := model.DegOf(theta)
	out.HtaDeg = init.HtaDeg - pitchDeg
	return frameLevelingResult{
		Leveled:  out,
		PitchDeg: pitchDeg,
	}
}

// placeFrontCockpit positions SteererClamp, StemClamp, and HandGrip from the leveled head-tube top
// and head-tube angle using the checked assembly recipe.
func placeFrontCockpit(headTubeTop geom2d.Point2, htaDeg model.Deg, cockpit CockpitSpec) (frontCockpitPoints, error) {
	headTube, err := assembly.NewHeadTubeTop(headTubeTop, htaDeg)
	if err != nil {
		return frontCockpitPoints{}, err
	}
	stemBase, err := headTube.WithUpperStack(cockpit.HeadsetTopStackMm, cockpit.SpacerMm)
	if err != nil {
		return frontCockpitPoints{}, err
	}
	stem, err := assembly.NewStem(cockpit.StemClampHeightMm, cockpit.Stem.LengthMm, cockpit.Stem.AngleDeg)
	if err != nil {
		return frontCockpitPoints{}, err
	}
	stemPlacement, err := stemBase.AttachStem(stem)
	if err != nil {
		return frontCockpitPoints{}, err
	}
	bar, err := assembly.NewHandlebar(cockpit.Handlebar, cockpit.GripInsetMm)
	if err != nil {
		return frontCockpitPoints{}, err
	}
	barPlacement, err := stemPlacement.BarClampMount().AttachHandlebar(bar, cockpit.BarRollDeg)
	if err != nil {
		return frontCockpitPoints{}, err
	}

	return frontCockpitPoints{
		SteererClamp: stemPlacement.SteererClamp(),
		StemClamp:    stemPlacement.BarClamp(),
		HandGrip:     barPlacement.HandGrip(),
	}, nil
}

// ChartClosureInput holds the published chart inputs and CAD fork hypothesis
// needed to construct FA_chart and FA_decl without full cockpit/seat/fit specs.
type ChartClosureInput struct {
	ReachMm       float64
	StackMm       float64
	HtaDeg        model.Deg
	HtlMm         float64
	BbDropMm      float64
	ChainstayMm   float64
	WheelbaseMm   model.Opt[float64]
	FrontCenterMm model.Opt[float64]
	CadEffA2cMm   float64
	CadRakeMm     float64
	CadLowerCupMm float64
}

// ChartClosure holds the chart-implied (FA_chart) and declared-fork (FA_decl)
// front-axle points and their closure residuals.
type ChartClosure struct {
	RearAxle               geom2d.Point2
	HeadTubeTop            geom2d.Point2
	HeadTubeBottom         geom2d.Point2
	ChartL                 float64
	ChartFrontAxle         geom2d.Point2
	DeclFrontAxle          geom2d.Point2
	ImpliedStructuralA2cMm float64
	ImpliedForkA2cMm       float64
	ResidualA2cMm          float64
	ResidualFcMm           model.Opt[float64]
	ResidualWbMm           model.Opt[float64]
}

// EvaluateChartClosure constructs FA_chart, FA_decl, and the chart closure residuals
// directly from published chart dimensions and the CAD fork/lower-cup hypothesis.
func EvaluateChartClosure(in ChartClosureInput) (ChartClosure, error) {
	chartL, ok := frontEndLength(in.StackMm, in.BbDropMm, in.HtaDeg, in.CadRakeMm)
	if !ok {
		return ChartClosure{}, fmt.Errorf("steerer axis is horizontal (HTA %.2f°)", in.HtaDeg)
	}

	u0 := SteererTubeDir(in.HtaDeg)          // (-cos HTA, sin HTA) up the steerer
	n0 := u0.Perpendicular(geom2d.Clockwise) // (sin HTA, cos HTA) forward perp (rake direction)

	ra0 := geom2d.Point2{
		X: -RearCenterHoriz(in.ChainstayMm, in.BbDropMm),
		Y: in.BbDropMm,
	}
	ht0 := geom2d.Point2{X: in.ReachMm, Y: in.StackMm}
	htb0 := ht0.SubVec(u0.Scale(in.HtlMm))
	faChart := ht0.SubVec(u0.Scale(chartL)).Add(n0.Scale(in.CadRakeMm))
	faDecl := htb0.SubVec(u0.Scale(in.CadLowerCupMm + in.CadEffA2cMm)).Add(n0.Scale(in.CadRakeMm))

	resA2c := (in.CadEffA2cMm + in.CadLowerCupMm + in.HtlMm) - chartL
	var resFc, resWb model.Opt[float64]
	if pubFc, ok := in.FrontCenterMm.Get(); ok && pubFc > 0 {
		resFc = model.Some(faChart.DistanceToOrigin() - pubFc)
	}
	if pubWb, ok := in.WheelbaseMm.Get(); ok && pubWb > 0 && in.ChainstayMm > 0 {
		resWb = model.Some((faChart.X - ra0.X) - pubWb)
	}

	return ChartClosure{
		RearAxle:               ra0,
		HeadTubeTop:            ht0,
		HeadTubeBottom:         htb0,
		ChartL:                 chartL,
		ChartFrontAxle:         faChart,
		DeclFrontAxle:          faDecl,
		ImpliedStructuralA2cMm: chartL - in.HtlMm,
		ImpliedForkA2cMm:       chartL - in.HtlMm - in.CadLowerCupMm,
		ResidualA2cMm:          resA2c,
		ResidualFcMm:           resFc,
		ResidualWbMm:           resWb,
	}, nil
}

// BuildScene constructs a pure 2D sagittal BikeScene from explicit frame, build, cockpit, seat, and fit inputs.
// Every required input must be explicitly valid; BuildScene returns an error rather than inventing fallback defaults.
func BuildScene(
	frame FrameSize,
	build BuildSpec,
	cockpit CockpitSpec,
	seat SeatSpec,
	fit RiderFit,
) (BikeScene, error) {
	if err := validateSceneInputs(frame, build, cockpit, seat, fit); err != nil {
		return BikeScene{}, err
	}

	reqClamp, err := RequiredStemClamp(fit, cockpit)
	if err != nil {
		return BikeScene{}, err
	}

	// 1. Initial CAD chart frame (BB_0 = (0,0), +X forward, +Y up)
	cadEffA2c := frame.CadFork.EffectiveA2c(frame.CadSagPct)
	closure, err := EvaluateChartClosure(ChartClosureInput{
		ReachMm:       frame.ReachMm,
		StackMm:       frame.StackMm,
		HtaDeg:        frame.HtaDeg,
		HtlMm:         frame.HtlMm,
		BbDropMm:      frame.BbDropMm,
		ChainstayMm:   frame.ChainstayMm,
		WheelbaseMm:   frame.WheelbaseMm,
		FrontCenterMm: frame.FrontCenterMm,
		CadEffA2cMm:   cadEffA2c,
		CadRakeMm:     frame.CadFork.RakeMm,
		CadLowerCupMm: frame.CadLowerCupMm,
	})
	if err != nil {
		return BikeScene{}, err
	}

	hta0 := frame.HtaDeg
	u0 := SteererTubeDir(hta0)               // (-cos HTA, sin HTA) up the steerer
	n0 := u0.Perpendicular(geom2d.Clockwise) // (sin HTA, cos HTA) forward perp (rake direction)

	bb0 := geom2d.Origin()
	ra0 := closure.RearAxle
	ht0 := closure.HeadTubeTop
	htb0 := closure.HeadTubeBottom
	chartL := closure.ChartL
	faChart := closure.ChartFrontAxle
	faDecl := closure.DeclFrontAxle

	forkEffA2c := build.Fork.EffectiveA2c(build.SagPct)

	dL := (forkEffA2c + build.PhysicalLowerCupMm) - (cadEffA2c + frame.CadLowerCupMm)
	fa0 := ht0.SubVec(u0.Scale(chartL + dL)).Add(n0.Scale(build.Fork.RakeMm))

	wb0 := fa0.X - ra0.X
	if !(wb0 > 0) {
		return BikeScene{}, fmt.Errorf("degenerate wheelbase %.2f mm", wb0)
	}

	// 2. Exact rear-axle pitch leveling across A2C, sag, lower cup, and rake changes.
	dRake := build.Fork.RakeMm - frame.CadFork.RakeMm
	leveled := levelFrame2D(frameLevelingInput{
		Initial: frameLandmarks2D{
			BB:             bb0,
			RearAxle:       ra0,
			HeadTubeTop:    ht0,
			HeadTubeBottom: htb0,
			FrontAxle:      fa0,
			HtaDeg:         hta0,
		},
		DeltaStructuralA2cMm:   dL,
		DeltaRakeMm:            dRake,
		WheelbaseBeforePitchMm: wb0,
	})
	fr := leveled.Leveled
	uAct := SteererTubeDir(fr.HtaDeg)
	lowerCupBottom := fr.HeadTubeBottom.SubVec(uAct.Scale(build.PhysicalLowerCupMm))
	steererAxis := geom2d.Axis2{Origin: fr.HeadTubeTop, Direction: uAct}

	// 3. Front cockpit on leveled frame
	cockpitPts, err := placeFrontCockpit(fr.HeadTubeTop, fr.HtaDeg, cockpit)
	if err != nil {
		return BikeScene{}, err
	}

	// 4. Rear cockpit on leveled frame
	nominalStaAct := frame.StaDeg - leveled.PitchDeg
	seatAxisStaAct := frame.EffectiveStaDegAt(fit.FrameSeatAxisHeightMm) - leveled.PitchDeg
	effStaAct := frame.EffectiveStaDegAt(fit.BrpHeightMm) - leveled.PitchDeg

	nominalSeatAxis := SeatTubeAxis(fr.BB, nominalStaAct)
	frameSeatAxis := SeatTubeAxis(fr.BB, seatAxisStaAct)
	effSeatAxis := SeatTubeAxis(fr.BB, effStaAct)

	sa := frameSeatAxis.PointAt(fit.FrameSeatAxisHeightMm)

	saddlePlacement, err := SolveSaddleForBRPRadius(seat.Saddle, effStaAct, fit.BrpHeightMm, seat.PostSetbackMm, seat.RailOffsetMm)
	if err != nil {
		return BikeScene{}, err
	}
	placement := BRPPointFromSaddlePlacement(saddlePlacement)
	brp := saddlePlacement.BRP()
	sb := saddlePlacement.SitBones()
	seatpostClamp := saddlePlacement.SeatpostClamp()
	railCenter := saddlePlacement.RailCenter()

	reqRailOff := seat.RailOffsetMm
	if fit.TargetSitBonesXMm < 0 {
		targetBrpSetback := seat.Saddle.BrpSetbackFromSitBones(-fit.TargetSitBonesXMm)
		railSol, err := SolveSaddleForBRPTarget(seat.Saddle, effStaAct, fit.BrpHeightMm, targetBrpSetback, seat.PostSetbackMm)
		if err != nil {
			return BikeScene{}, fmt.Errorf("target saddle placement: %w", err)
		}
		reqRailOff = railSol.RailOffsetMm()
	}
	railMargin := seat.HalfRailWindowMm() - math.Abs(reqRailOff)

	// 5. Validation residuals from chart/declared points
	resA2c := closure.ResidualA2cMm
	resFc := closure.ResidualFcMm
	resWb := closure.ResidualWbMm

	return BikeScene{
		Frame:                frame,
		Build:                build,
		Cockpit:              cockpit,
		Seat:                 seat,
		Fit:                  fit,
		BB:                   fr.BB,
		RearAxle:             fr.RearAxle,
		HeadTubeTop:          fr.HeadTubeTop,
		HeadTubeBottom:       fr.HeadTubeBottom,
		LowerCupBottom:       lowerCupBottom,
		FrontAxle:            fr.FrontAxle,
		ChartL:               chartL,
		ChartFrontAxle:       faChart,
		DeclFrontAxle:        faDecl,
		ResidualA2cMm:        resA2c,
		ResidualFcMm:         resFc,
		ResidualWbMm:         resWb,
		SteererAxis:          steererAxis,
		NominalSeatAxis:      nominalSeatAxis,
		FrameSeatAxis:        frameSeatAxis,
		EffectiveSeatAxis:    effSeatAxis,
		SeatAxisPoint:        sa,
		SeatpostClamp:        seatpostClamp,
		RailCenter:           railCenter,
		SaddlePlacement:      placement,
		BRP:                  brp,
		SitBones:             sb,
		RequiredRailOffsetMm: reqRailOff,
		RailMarginMm:         railMargin,
		SteererClamp:         cockpitPts.SteererClamp,
		StemClamp:            cockpitPts.StemClamp,
		HandGrip:             cockpitPts.HandGrip,
		RequiredStemClamp:    reqClamp,
		PitchDeg:             leveled.PitchDeg,
		HtaDeg:               fr.HtaDeg,
		NominalStaDeg:        nominalStaAct,
		SeatAxisStaDeg:       seatAxisStaAct,
		EffectiveStaDeg:      effStaAct,
		ForkEffA2cMm:         forkEffA2c,
		StructuralA2cMm:      forkEffA2c + build.PhysicalLowerCupMm,
	}, nil
}

// WithCockpit returns a copy of s with the front cockpit re-evaluated for cockpit without re-pitching the frame.
func (s BikeScene) WithCockpit(cockpit CockpitSpec) (BikeScene, error) {
	if err := validateCockpitSpec(cockpit); err != nil {
		return BikeScene{}, err
	}
	reqClamp, err := RequiredStemClamp(s.Fit, cockpit)
	if err != nil {
		return BikeScene{}, err
	}
	cockpitPts, err := placeFrontCockpit(s.HeadTubeTop, s.HtaDeg, cockpit)
	if err != nil {
		return BikeScene{}, err
	}
	s.Cockpit = cockpit
	s.SteererClamp = cockpitPts.SteererClamp
	s.StemClamp = cockpitPts.StemClamp
	s.HandGrip = cockpitPts.HandGrip
	s.RequiredStemClamp = reqClamp
	return s, nil
}

// Reach returns the active frame reach in mm (HeadTubeTop.X relative to BB).
func (s BikeScene) Reach() float64 { return s.HeadTubeTop.X }

// Stack returns the active frame stack in mm (HeadTubeTop.Y relative to BB).
func (s BikeScene) Stack() float64 { return s.HeadTubeTop.Y }

// BbDrop returns the active bottom-bracket drop in mm (RearAxle.Y relative to BB).
func (s BikeScene) BbDrop() float64 { return s.RearAxle.Y }

// FrontCenter returns the active horizontal front-center in mm (FrontAxle.X).
func (s BikeScene) FrontCenter() float64 { return s.FrontAxle.X }

// RearCenter returns the active horizontal rear-center in mm (-RearAxle.X).
func (s BikeScene) RearCenter() float64 { return -s.RearAxle.X }

// Wheelbase returns the active horizontal wheelbase in mm (FrontAxle.X - RearAxle.X).
func (s BikeScene) Wheelbase() float64 { return s.FrontAxle.X - s.RearAxle.X }

// TrailMm returns the active ground trail in mm using the front wheel radius.
func (s BikeScene) TrailMm() float64 {
	return CalculateTrail(s.HtaDeg, s.Build.Fork.RakeMm, s.Build.WheelRadiusFrontMm)
}

// EttMm returns the active Effective Top Tube length in mm at Stack height (using NominalSeatAxis).
func (s BikeScene) EttMm() float64 {
	pt, ok := s.NominalSeatAxis.IntersectHorizontalLine(s.HeadTubeTop.Y)
	if !ok {
		return 0
	}
	return s.HeadTubeTop.X - pt.X
}

// AettMm returns the Actual Effective Top Tube length in mm at the frame-only SeatAxisPoint.Y.
func (s BikeScene) AettMm() float64 {
	pt, ok := s.SteererAxis.IntersectHorizontalLine(s.SeatAxisPoint.Y)
	if !ok {
		return 0
	}
	return pt.X - s.SeatAxisPoint.X
}

// SeatAxisActualSetbackMm returns the horizontal setback in mm from BB to the frame-only SeatAxisPoint.
func (s BikeScene) SeatAxisActualSetbackMm() float64 { return -s.SeatAxisPoint.X }

// SpareVerticalStackMm returns the vertical drop from SeatAxisPoint.Y to HeadTubeTop.Y in mm.
func (s BikeScene) SpareVerticalStackMm() float64 { return s.SeatAxisPoint.Y - s.HeadTubeTop.Y }

// SpareAxialSteererMm returns the axial steerer length needed to bridge SpareVerticalStackMm.
func (s BikeScene) SpareAxialSteererMm() float64 {
	return s.SpareVerticalStackMm() / s.HtaDeg.Angle().Sin()
}

// GripDistFromBrpMm returns the 2D Euclidean distance from BRP to HandGrip in mm.
func (s BikeScene) GripDistFromBrpMm() float64 { return s.HandGrip.DistanceTo(s.BRP) }

// GripRiseAboveSitBonesMm returns HandGrip.Y - SitBones.Y in mm.
func (s BikeScene) GripRiseAboveSitBonesMm() float64 { return s.HandGrip.Y - s.SitBones.Y }

// StemSpacerFit holds the exact stem length and spacer stack that places
// StemClamp at RequiredStemClamp for a given stem angle.
type StemSpacerFit struct {
	StemAngleDeg model.Deg
	StemLengthMm float64
	SpacerMm     float64
}

// SolveStemSpacerFit solves for the exact continuous (StemLengthMm, SpacerMm) at
// stemAngleDeg that places StemClamp at RequiredStemClamp. Returns false if the
// stem is parallel to the steerer axis.
func (s BikeScene) SolveStemSpacerFit(stemAngleDeg model.Deg) (StemSpacerFit, bool) {
	uSteer := SteererTubeDir(s.HtaDeg)
	nSteer := uSteer.Perpendicular(geom2d.Clockwise)
	uStem := nSteer.Rotate(stemAngleDeg.Angle())
	det := uSteer.X*uStem.Y - uSteer.Y*uStem.X
	if math.Abs(det) < 1e-9 {
		return StemSpacerFit{}, false
	}
	rhs := s.RequiredStemClamp.Sub(s.HeadTubeTop)
	t := (rhs.X*uStem.Y - rhs.Y*uStem.X) / det
	l := (uSteer.X*rhs.Y - uSteer.Y*rhs.X) / det
	spacer := t - (s.Cockpit.HeadsetTopStackMm + s.Cockpit.StemClampHeightMm/2.0)
	return StemSpacerFit{
		StemAngleDeg: stemAngleDeg,
		StemLengthMm: l,
		SpacerMm:     spacer,
	}, true
}

// TargetPointError returns RequiredStemClamp - StemClamp (equivalently TargetPoint - EvaluatedCockpitPoint).
func (s BikeScene) TargetPointError() geom2d.Vec2 { return s.RequiredStemClamp.Sub(s.StemClamp) }

// SeatpostExtensionMm estimates exposed axial length for the installed saddle.
func (s BikeScene) SeatpostExtensionMm() (float64, bool) {
	return s.SaddlePlacement.SeatpostExtension(s.Frame.SeatTubeLengthMm.OrZero())
}

// RearCenterWheelbaseRatio returns RearCenter / Wheelbase.
func (s BikeScene) RearCenterWheelbaseRatio() float64 { return s.RearCenter() / s.Wheelbase() }

func (s BikeScene) seatTubeDistance() float64 {
	if stl, ok := s.Frame.SeatTubeLengthMm.Get(); ok && stl > 0 {
		return stl
	}
	fallback := s.SeatpostClamp.DistanceTo(s.BB) * 0.75
	if dirY := s.NominalSeatAxis.Direction.Y; dirY > 1e-6 && s.HeadTubeTop.Y > 0 {
		horiz := s.HeadTubeTop.Y / dirY
		if horiz > 0 && (fallback <= 0 || horiz < fallback) {
			return horiz
		}
	}
	return fallback
}

func (s BikeScene) upperSeatTubeAxis() geom2d.Axis2 {
	oxVec := geom2d.Vec2{X: s.Frame.SeatTubeOffsetX(), Y: 0}.Rotate((-s.PitchDeg).Angle())
	return SeatTubeAxis(s.BB.Add(oxVec), s.Frame.ActualStaDeg-s.PitchDeg)
}

// SeatTubeBend returns the bend point for frames with a bent seat tube.
func (s BikeScene) SeatTubeBend() (geom2d.Point2, bool) {
	if !s.Frame.HasBentSeatTube() {
		return geom2d.Point2{}, false
	}
	stl := s.seatTubeDistance()
	bendT := math.Min(160.0, stl*0.35)
	return s.upperSeatTubeAxis().PointAt(bendT), true
}

// SeatTubeTop returns the 2D position of the top of the frame's seat tube.
// Uses Frame.SeatTubeLengthMm when set, or a horizontal-top-tube fallback below the saddle.
func (s BikeScene) SeatTubeTop() geom2d.Point2 {
	stl := s.seatTubeDistance()
	nominalTop := s.NominalSeatAxis.PointAt(stl)
	if !s.Frame.HasBentSeatTube() {
		return nominalTop
	}
	upper := s.upperSeatTubeAxis()
	if math.Abs(upper.Direction.Y) < 1e-6 {
		return nominalTop
	}
	t := (nominalTop.Y - upper.Origin.Y) / upper.Direction.Y
	return upper.PointAt(t)
}

func isPositive(v float64) bool {
	return v > 0 && !math.IsInf(v, 0)
}

func isNonNegative(v float64) bool {
	return v >= 0 && !math.IsInf(v, 0)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func validateCockpitTargetInputs(fit RiderFit, cockpit CockpitSpec) error {
	if !fit.Target.Ref.IsValid() {
		return fmt.Errorf("RiderFit requires a valid TargetRef (got %q)", fit.Target.Ref)
	}
	if !isFinite(fit.Target.Point.X) || !isFinite(fit.Target.Point.Y) {
		return fmt.Errorf("RiderFit Target.Point must be finite, got (%v, %v)", fit.Target.Point.X, fit.Target.Point.Y)
	}
	if !isPositive(cockpit.Handlebar.EffectiveWidthMm()) {
		return fmt.Errorf("Handlebar width must be positive, got %v mm", cockpit.Handlebar.EffectiveWidthMm())
	}
	if !isPositive(cockpit.GripInsetMm) || cockpit.GripInsetMm >= cockpit.Handlebar.EffectiveWidthMm()/2.0 {
		return fmt.Errorf("GripInsetMm %v must be in (0, %.1f)", cockpit.GripInsetMm, cockpit.Handlebar.EffectiveWidthMm()/2.0)
	}
	if fit.Target.Ref == TargetReferenceBarClamp {
		refBar, ok := fit.ReferenceBar.Get()
		if !ok || !isPositive(refBar.EffectiveWidthMm()) {
			return fmt.Errorf("TargetReferenceBarClamp requires RiderFit.ReferenceBar with positive width")
		}
		if cockpit.GripInsetMm >= refBar.EffectiveWidthMm()/2.0 {
			return fmt.Errorf("GripInsetMm %v must be less than ReferenceBar half-width %.1f", cockpit.GripInsetMm, refBar.EffectiveWidthMm()/2.0)
		}
	}
	return nil
}

func validateCockpitSpec(cockpit CockpitSpec) error {
	if !isNonNegative(cockpit.HeadsetTopStackMm) || !isNonNegative(cockpit.SpacerMm) {
		return fmt.Errorf("headset top stack (%.1f) and spacers (%.1f) must be non-negative", cockpit.HeadsetTopStackMm, cockpit.SpacerMm)
	}
	if !isPositive(cockpit.Stem.LengthMm) || !isPositive(cockpit.StemClampHeightMm) || !isFinite(float64(cockpit.Stem.AngleDeg)) {
		return fmt.Errorf("stem requires positive LengthMm and StemClampHeightMm, got len=%.1f h=%.1f", cockpit.Stem.LengthMm, cockpit.StemClampHeightMm)
	}
	if !isFinite(float64(cockpit.BarRollDeg)) {
		return fmt.Errorf("BarRollDeg must be finite")
	}
	return nil
}

func validateSceneInputs(
	frame FrameSize,
	build BuildSpec,
	cockpit CockpitSpec,
	seat SeatSpec,
	fit RiderFit,
) error {
	if !isPositive(frame.ReachMm) || !isPositive(frame.StackMm) || !isPositive(frame.HtlMm) {
		return fmt.Errorf("frame Reach (%.1f), Stack (%.1f), and HTL (%.1f) must be positive", frame.ReachMm, frame.StackMm, frame.HtlMm)
	}
	if frame.HtaDeg <= 0 || frame.HtaDeg >= 90 || !isFinite(float64(frame.HtaDeg)) {
		return fmt.Errorf("frame HTA %.2f° must be in (0°, 90°)", frame.HtaDeg)
	}
	if frame.StaDeg <= 0 || frame.StaDeg >= 90 || !isFinite(float64(frame.StaDeg)) {
		return fmt.Errorf("frame STA %.2f° must be in (0°, 90°)", frame.StaDeg)
	}
	if frame.ActualStaDeg < 0 || frame.ActualStaDeg >= 90 || !isFinite(float64(frame.ActualStaDeg)) {
		return fmt.Errorf("frame ActualStaDeg %.2f° must be in [0°, 90°)", frame.ActualStaDeg)
	}
	if !isFinite(frame.BbDropMm) || !isPositive(frame.ChainstayMm) || frame.ChainstayMm <= math.Abs(frame.BbDropMm) {
		return fmt.Errorf("frame ChainstayMm (%.1f) must exceed |BbDropMm| (%.1f)", frame.ChainstayMm, frame.BbDropMm)
	}
	if !isPositive(frame.CadFork.StaticA2cMm) || !isPositive(frame.CadFork.RakeMm) || !isNonNegative(frame.CadFork.TravelMm) {
		return fmt.Errorf("frame CadFork requires positive StaticA2cMm and RakeMm, got A2C=%.1f Rake=%.1f", frame.CadFork.StaticA2cMm, frame.CadFork.RakeMm)
	}
	if !isNonNegative(frame.CadSagPct) || frame.CadSagPct > 100 || (frame.CadFork.TravelMm == 0 && frame.CadSagPct > 0) {
		return fmt.Errorf("invalid frame CadSagPct %.1f%% for travel %.0fmm", frame.CadSagPct, frame.CadFork.TravelMm)
	}
	if !isNonNegative(frame.CadLowerCupMm) {
		return fmt.Errorf("frame CadLowerCupMm must be non-negative, got %v", frame.CadLowerCupMm)
	}
	if !isPositive(build.Fork.StaticA2cMm) || !isPositive(build.Fork.RakeMm) || !isNonNegative(build.Fork.TravelMm) {
		return fmt.Errorf("build Fork requires positive StaticA2cMm and RakeMm, got A2C=%.1f Rake=%.1f", build.Fork.StaticA2cMm, build.Fork.RakeMm)
	}
	if !isNonNegative(build.SagPct) || build.SagPct > 100 || (build.Fork.TravelMm == 0 && build.SagPct > 0) {
		return fmt.Errorf("cannot apply %.1f%% sag to fork with %.0fmm travel", build.SagPct, build.Fork.TravelMm)
	}
	if !isNonNegative(build.PhysicalLowerCupMm) {
		return fmt.Errorf("PhysicalLowerCupMm must be non-negative, got %v", build.PhysicalLowerCupMm)
	}
	if !isPositive(build.WheelRadiusFrontMm) || !isPositive(build.WheelRadiusRearMm) {
		return fmt.Errorf("wheel radii must be positive, got front=%.1f rear=%.1f", build.WheelRadiusFrontMm, build.WheelRadiusRearMm)
	}
	if err := validateCockpitSpec(cockpit); err != nil {
		return err
	}
	if !isNonNegative(seat.PostSetbackMm) || !isPositive(seat.ClampLengthMm) || !isPositive(seat.Saddle.RailLengthMm) || !isFinite(seat.RailOffsetMm) || !isFinite(float64(seat.Saddle.RailPitchDeg)) {
		return fmt.Errorf("invalid SeatSpec (postSB=%.1f clamp=%.1f railLen=%.1f railOff=%.1f)", seat.PostSetbackMm, seat.ClampLengthMm, seat.Saddle.RailLengthMm, seat.RailOffsetMm)
	}
	if _, _, ok := seat.Saddle.ClampWindowMm(seat.ClampLengthMm); !ok {
		return fmt.Errorf("%.1f mm clamp exceeds the assumed %.1f mm rail span", seat.ClampLengthMm, seat.Saddle.RailLengthMm)
	}
	if !isPositive(fit.FrameSeatAxisHeightMm) || !isPositive(fit.BrpHeightMm) {
		return fmt.Errorf("RiderFit FrameSeatAxisHeightMm (%.1f) and BrpHeightMm (%.1f) must be positive", fit.FrameSeatAxisHeightMm, fit.BrpHeightMm)
	}
	return nil
}
