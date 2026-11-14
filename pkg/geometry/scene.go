package geometry

import (
	"fmt"
	"math"

	"bikegeo/pkg/geom2d"
	"bikegeo/pkg/model"
)

// TargetRef names the physical cockpit point a fit TargetPoint refers to.
// Every fit target must declare one explicitly; bare or implicit "HB" targets
// are not permitted.
type TargetRef string

const (
	// TargetHandGrip indicates TargetPoint is the rider's physical hand contact center (HandGrip).
	TargetHandGrip TargetRef = "hand_grip"
	// TargetStemClamp indicates TargetPoint is the handlebar/stem clamp center directly (StemClamp).
	TargetStemClamp TargetRef = "stem_clamp"
	// TargetReferenceBarClamp indicates TargetPoint is the stem clamp center of RiderFit.ReferenceBar
	// on the fit bike, reproducing hand position through ReferenceBar's sweep/rise offset.
	// TODO: this this strictly necessary? It seems like a YAGNI thing.
	TargetReferenceBarClamp TargetRef = "reference_bar_clamp"
)

// IsValid reports whether r is a recognized TargetRef mode.
func (r TargetRef) IsValid() bool {
	switch r {
	case TargetHandGrip, TargetStemClamp, TargetReferenceBarClamp:
		return true
	default:
		return false
	}
}

// Label returns a concise human-readable description of the TargetRef mode.
func (r TargetRef) Label() string {
	switch r {
	case TargetHandGrip:
		return "HandGrip"
	case TargetStemClamp:
		return "StemClamp"
	case TargetReferenceBarClamp:
		return "ReferenceBar StemClamp"
	default:
		return "Unknown"
	}
}

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

func (f FrameSize) effectiveStaCad(seatHeight float64) model.Deg {
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
			TravelMm:    bike.CadFork.ForkTravel(),
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

// HalfRailWindowMm returns half the usable straight rail travel window in mm.
func (s SeatSpec) HalfRailWindowMm() float64 {
	if s.Saddle.RailLengthMm <= s.ClampLengthMm {
		return 0
	}
	return (s.Saddle.RailLengthMm - s.ClampLengthMm) / 2.0
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

	// TargetPoint is the 2D target coordinate relative to BB (0,0), interpreted according to TargetRef.
	TargetPoint geom2d.Point2
	TargetRef   TargetRef

	// ReferenceSaddle is the saddle used to record reference biomechanical coordinates.
	ReferenceSaddle model.SaddleSpec

	// ReferenceBar is the handlebar on the fit bike; required when TargetRef == TargetReferenceBarClamp.
	ReferenceBar model.Opt[model.HandlebarSpec]
}

// RequiredStemClamp converts fit.TargetPoint into the required 2D StemClamp coordinate C*
// for the configured handlebar, grip inset, and roll under fit.TargetRef.
func RequiredStemClamp(fit RiderFit, cockpit CockpitSpec) (geom2d.Point2, error) {
	if err := validateCockpitTargetInputs(fit, cockpit); err != nil {
		return geom2d.Point2{}, err
	}
	barOff := GetHandlebarOffset(cockpit.Handlebar, cockpit.GripInsetMm, cockpit.BarRollDeg)
	barVec := geom2d.Vec2{X: barOff.X, Y: barOff.Y}

	switch fit.TargetRef {
	case TargetHandGrip:
		return fit.TargetPoint.SubVec(barVec), nil
	case TargetStemClamp:
		return fit.TargetPoint, nil
	case TargetReferenceBarClamp:
		refBar, _ := fit.ReferenceBar.Get()
		refOff := GetHandlebarOffset(refBar, cockpit.GripInsetMm, 0.0)
		refVec := geom2d.Vec2{X: refOff.X, Y: refOff.Y}
		return fit.TargetPoint.Add(refVec).SubVec(barVec), nil
	default:
		return geom2d.Point2{}, fmt.Errorf("invalid TargetRef %q", fit.TargetRef)
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

	// Leveled rear cockpit landmarks
	SeatAxisPoint geom2d.Point2
	SeatpostClamp geom2d.Point2
	RailCenter    geom2d.Point2
	BRP           geom2d.Point2
	SitBones      geom2d.Point2

	// Leveled front cockpit landmarks & explicit fit target
	SteererClamp      geom2d.Point2
	StemClamp         geom2d.Point2
	HandGrip          geom2d.Point2
	TargetPoint       geom2d.Point2
	TargetRef         TargetRef
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

// legacyPitchMode preserves pre-C3 constant-rake / published-wheelbase pitching during Commit 5's
// incremental cutover; removed in Commit 6 (C3).
// TODO: to be removed in Commit 6 (C3)
type legacyPitchMode struct {
	pitchWheelbaseOverride float64
	constantRakePitch      bool
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

// frameLevelingResult holds the leveled frame landmarks (re-centered with BB at Origin) and clockwise pitch angle.
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
		BB:             geom2d.Origin,
		RearAxle:       init.RearAxle,
		HeadTubeTop:    init.HeadTubeTop,
		HeadTubeBottom: init.HeadTubeBottom,
		FrontAxle:      init.FrontAxle,
		HtaDeg:         init.HtaDeg,
	}
	if theta != 0 {
		rot := geom2d.NewRotation(init.RearAxle, theta)
		bbRot := rot.Apply(init.BB)
		shift := geom2d.Origin.Sub(bbRot)

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
// and head-tube angle.
func placeFrontCockpit(headTubeTop geom2d.Point2, htaDeg model.Deg, cockpit CockpitSpec) frontCockpitPoints {
	uAct := SteererTubeDir(htaDeg)
	nAct := uAct.Perpendicular(geom2d.Clockwise)
	dSteerer := cockpit.HeadsetTopStackMm + cockpit.SpacerMm + cockpit.StemClampHeightMm/2.0
	steererClamp := headTubeTop.Add(uAct.Scale(dSteerer))
	stemDir := nAct.Rotate(cockpit.Stem.AngleDeg.Angle())
	stemClamp := steererClamp.Add(stemDir.Scale(cockpit.Stem.LengthMm))
	barOff := GetHandlebarOffset(cockpit.Handlebar, cockpit.GripInsetMm, cockpit.BarRollDeg)
	handGrip := stemClamp.Add(geom2d.Vec2{X: barOff.X, Y: barOff.Y})
	return frontCockpitPoints{
		SteererClamp: steererClamp,
		StemClamp:    stemClamp,
		HandGrip:     handGrip,
	}
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
	return buildScene(frame, build, cockpit, seat, fit, legacyPitchMode{})
}

func buildScene(
	frame FrameSize,
	build BuildSpec,
	cockpit CockpitSpec,
	seat SeatSpec,
	fit RiderFit,
	pitchMode legacyPitchMode,
) (BikeScene, error) {
	if err := validateSceneInputs(frame, build, cockpit, seat, fit); err != nil {
		return BikeScene{}, err
	}

	reqClamp, err := RequiredStemClamp(fit, cockpit)
	if err != nil {
		return BikeScene{}, err
	}

	// 1. Initial CAD chart frame (BB_0 = (0,0), +X forward, +Y up)
	hta0 := frame.HtaDeg
	u0 := SteererTubeDir(hta0)               // (-cos HTA, sin HTA) up the steerer
	n0 := u0.Perpendicular(geom2d.Clockwise) // (sin HTA, cos HTA) forward perp (rake direction)

	bb0 := geom2d.Origin
	ra0 := geom2d.Point2{
		X: -RearCenterHoriz(frame.ChainstayMm, frame.BbDropMm),
		Y: frame.BbDropMm,
	}
	ht0 := geom2d.Point2{X: frame.ReachMm, Y: frame.StackMm}
	htb0 := ht0.SubVec(u0.Scale(frame.HtlMm))

	chartL, ok := frontEndLength(frame.StackMm, frame.BbDropMm, hta0, frame.CadFork.RakeMm)
	if !ok {
		return BikeScene{}, fmt.Errorf("steerer axis is horizontal (HTA %.2f°)", hta0)
	}
	faChart := ht0.SubVec(u0.Scale(chartL)).Add(n0.Scale(frame.CadFork.RakeMm))

	cadEffA2c := frame.CadFork.EffectiveA2c(frame.CadSagPct)
	forkEffA2c := build.Fork.EffectiveA2c(build.SagPct)

	faDecl := htb0.SubVec(u0.Scale(frame.CadLowerCupMm + cadEffA2c)).Add(n0.Scale(frame.CadFork.RakeMm))

	dL := (forkEffA2c + build.PhysicalLowerCupMm) - (cadEffA2c + frame.CadLowerCupMm)
	fa0 := ht0.SubVec(u0.Scale(chartL + dL)).Add(n0.Scale(build.Fork.RakeMm))

	wb0 := fa0.X - ra0.X
	if !(wb0 > 0) {
		return BikeScene{}, fmt.Errorf("degenerate wheelbase %.2f mm", wb0)
	}

	// 2. Exact rear-axle pitch leveling across A2C, sag, lower cup, and rake changes.
	dRake := build.Fork.RakeMm - frame.CadFork.RakeMm
	wbBeforePitch := wb0
	if pitchMode.constantRakePitch {
		dRake = 0
	}
	if pitchMode.pitchWheelbaseOverride > 0 {
		wbBeforePitch = pitchMode.pitchWheelbaseOverride - dL*u0.X
	}

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
		WheelbaseBeforePitchMm: wbBeforePitch,
	})
	fr := leveled.Leveled
	uAct := SteererTubeDir(fr.HtaDeg)
	lowerCupBottom := fr.HeadTubeBottom.SubVec(uAct.Scale(build.PhysicalLowerCupMm))
	steererAxis := geom2d.Axis2{Origin: fr.HeadTubeTop, Direction: uAct}

	// 3. Front cockpit on leveled frame
	cockpitPts := placeFrontCockpit(fr.HeadTubeTop, fr.HtaDeg, cockpit)

	// 4. Rear cockpit on leveled frame
	nominalStaAct := frame.StaDeg - leveled.PitchDeg
	seatAxisStaAct := frame.effectiveStaCad(fit.FrameSeatAxisHeightMm) - leveled.PitchDeg
	effStaAct := frame.effectiveStaCad(fit.BrpHeightMm) - leveled.PitchDeg

	nominalSeatAxis := SeatTubeAxis(fr.BB, nominalStaAct)
	frameSeatAxis := SeatTubeAxis(fr.BB, seatAxisStaAct)
	effSeatAxis := SeatTubeAxis(fr.BB, effStaAct)

	sa := frameSeatAxis.PointAt(fit.FrameSeatAxisHeightMm)

	brpPerp := seat.PostSetbackMm + seat.RailOffsetMm - seat.Saddle.RailCenterToBrpMm
	if fit.BrpHeightMm <= math.Abs(brpPerp) {
		return BikeScene{}, fmt.Errorf("saddle BRP perpendicular offset %.1f mm exceeds BRP height %.1f mm", math.Abs(brpPerp), fit.BrpHeightMm)
	}
	axial := math.Sqrt(fit.BrpHeightMm*fit.BrpHeightMm - brpPerp*brpPerp)
	brp := effSeatAxis.PointWithOffset(axial, brpPerp, geom2d.CounterClockwise)
	sb := seat.Saddle.SitBonePointFromBrp(brp)
	seatpostClamp := effSeatAxis.PointWithOffset(axial, seat.PostSetbackMm, geom2d.CounterClockwise)
	railCenter := effSeatAxis.PointWithOffset(axial, seat.PostSetbackMm+seat.RailOffsetMm, geom2d.CounterClockwise)

	// 5. Validation residuals from chart/declared points
	resA2c := (cadEffA2c + frame.CadLowerCupMm + frame.HtlMm) - chartL
	var resFc, resWb model.Opt[float64]
	if pubFc, ok := frame.FrontCenterMm.Get(); ok && pubFc > 0 {
		resFc = model.Some(faChart.DistanceToOrigin() - pubFc)
	}
	if pubWb, ok := frame.WheelbaseMm.Get(); ok && pubWb > 0 {
		resWb = model.Some((faChart.X - ra0.X) - pubWb)
	}

	return BikeScene{
		Frame:             frame,
		Build:             build,
		Cockpit:           cockpit,
		Seat:              seat,
		Fit:               fit,
		BB:                fr.BB,
		RearAxle:          fr.RearAxle,
		HeadTubeTop:       fr.HeadTubeTop,
		HeadTubeBottom:    fr.HeadTubeBottom,
		LowerCupBottom:    lowerCupBottom,
		FrontAxle:         fr.FrontAxle,
		ChartL:            chartL,
		ChartFrontAxle:    faChart,
		DeclFrontAxle:     faDecl,
		ResidualA2cMm:     resA2c,
		ResidualFcMm:      resFc,
		ResidualWbMm:      resWb,
		SteererAxis:       steererAxis,
		NominalSeatAxis:   nominalSeatAxis,
		FrameSeatAxis:     frameSeatAxis,
		EffectiveSeatAxis: effSeatAxis,
		SeatAxisPoint:     sa,
		SeatpostClamp:     seatpostClamp,
		RailCenter:        railCenter,
		BRP:               brp,
		SitBones:          sb,
		SteererClamp:      cockpitPts.SteererClamp,
		StemClamp:         cockpitPts.StemClamp,
		HandGrip:          cockpitPts.HandGrip,
		TargetPoint:       fit.TargetPoint,
		TargetRef:         fit.TargetRef,
		RequiredStemClamp: reqClamp,
		PitchDeg:          leveled.PitchDeg,
		HtaDeg:            fr.HtaDeg,
		NominalStaDeg:     nominalStaAct,
		SeatAxisStaDeg:    seatAxisStaAct,
		EffectiveStaDeg:   effStaAct,
		ForkEffA2cMm:      forkEffA2c,
		StructuralA2cMm:   forkEffA2c + build.PhysicalLowerCupMm,
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
	cockpitPts := placeFrontCockpit(s.HeadTubeTop, s.HtaDeg, cockpit)
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

// FrontCenter3D returns the active 3D Euclidean distance from BB to FrontAxle in mm.
func (s BikeScene) FrontCenter3D() float64 { return s.FrontAxle.DistanceToOrigin() }

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

// SitBoneSetbackMm returns the horizontal setback of SitBones behind BB in mm.
func (s BikeScene) SitBoneSetbackMm() float64 { return -s.SitBones.X }

// BrpSetbackMm returns the horizontal setback of the saddle 70mm BRP behind BB in mm.
func (s BikeScene) BrpSetbackMm() float64 { return -s.BRP.X }

// BrpAngleFromBbDeg returns the polar elevation angle from BB to the saddle BRP in degrees.
func (s BikeScene) BrpAngleFromBbDeg() model.Deg {
	setback := s.BrpSetbackMm()
	if setback > 0 && s.BRP.Y > 0 {
		return model.DegOf(geom2d.Point2{X: setback, Y: s.BRP.Y}.PolarAngle())
	}
	return 0
}

// GripDistFromBrpMm returns the 2D Euclidean distance from BRP to HandGrip in mm.
func (s BikeScene) GripDistFromBrpMm() float64 { return s.HandGrip.DistanceTo(s.BRP) }

// GripRiseAboveSitBonesMm returns HandGrip.Y - SitBones.Y in mm.
func (s BikeScene) GripRiseAboveSitBonesMm() float64 { return s.HandGrip.Y - s.SitBones.Y }

// GripRiseAboveBrpMm returns HandGrip.Y - BRP.Y in mm.
func (s BikeScene) GripRiseAboveBrpMm() float64 { return s.HandGrip.Y - s.BRP.Y }

// StemClampDistFromBrpMm returns the 2D Euclidean distance from BRP to StemClamp in mm.
func (s BikeScene) StemClampDistFromBrpMm() float64 { return s.StemClamp.DistanceTo(s.BRP) }

// StemClampRiseAboveSitBonesMm returns StemClamp.Y - SitBones.Y in mm.
func (s BikeScene) StemClampRiseAboveSitBonesMm() float64 { return s.StemClamp.Y - s.SitBones.Y }

// EvaluatedCockpitPoint returns the physical cockpit point compared against TargetPoint for s.TargetRef.
func (s BikeScene) EvaluatedCockpitPoint() geom2d.Point2 {
	switch s.TargetRef {
	case TargetHandGrip:
		return s.HandGrip
	case TargetStemClamp:
		return s.StemClamp
	case TargetReferenceBarClamp:
		if refBar, ok := s.Fit.ReferenceBar.Get(); ok {
			refOff := GetHandlebarOffset(refBar, s.Cockpit.GripInsetMm, 0.0)
			return s.HandGrip.SubVec(geom2d.Vec2{X: refOff.X, Y: refOff.Y})
		}
		return s.StemClamp
	default:
		return s.HandGrip
	}
}

// TargetPointError returns RequiredStemClamp - StemClamp (equivalently TargetPoint - EvaluatedCockpitPoint).
func (s BikeScene) TargetPointError() geom2d.Vec2 { return s.RequiredStemClamp.Sub(s.StemClamp) }

// RailMarginMm returns the remaining straight saddle rail adjustment margin in mm (negative means past clamp stop).
func (s BikeScene) RailMarginMm() float64 {
	return s.Seat.HalfRailWindowMm() - math.Abs(s.Seat.RailOffsetMm)
}

// SeatpostExtensionMm returns BrpHeightMm minus nominal SeatTubeLengthMm if SeatTubeLengthMm was provided.
func (s BikeScene) SeatpostExtensionMm() (float64, bool) {
	stl, ok := s.Frame.SeatTubeLengthMm.Get()
	if !ok || stl <= 0 {
		return 0, false
	}
	return s.Fit.BrpHeightMm - stl, true
}

// RearCenterWheelbaseRatio returns RearCenter / Wheelbase.
func (s BikeScene) RearCenterWheelbaseRatio() float64 { return s.RearCenter() / s.Wheelbase() }

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
	if !fit.TargetRef.IsValid() {
		return fmt.Errorf("RiderFit requires a valid TargetRef (got %q)", fit.TargetRef)
	}
	if !isFinite(fit.TargetPoint.X) || !isFinite(fit.TargetPoint.Y) {
		return fmt.Errorf("RiderFit TargetPoint must be finite, got (%v, %v)", fit.TargetPoint.X, fit.TargetPoint.Y)
	}
	if !isPositive(cockpit.Handlebar.EffectiveWidthMm()) {
		return fmt.Errorf("Handlebar width must be positive, got %v mm", cockpit.Handlebar.EffectiveWidthMm())
	}
	if !isPositive(cockpit.GripInsetMm) || cockpit.GripInsetMm >= cockpit.Handlebar.EffectiveWidthMm()/2.0 {
		return fmt.Errorf("GripInsetMm %v must be in (0, %.1f)", cockpit.GripInsetMm, cockpit.Handlebar.EffectiveWidthMm()/2.0)
	}
	if fit.TargetRef == TargetReferenceBarClamp {
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
	if !isNonNegative(seat.PostSetbackMm) || !isPositive(seat.ClampLengthMm) || !isPositive(seat.Saddle.RailLengthMm) || !isFinite(seat.RailOffsetMm) {
		return fmt.Errorf("invalid SeatSpec (postSB=%.1f clamp=%.1f railLen=%.1f railOff=%.1f)", seat.PostSetbackMm, seat.ClampLengthMm, seat.Saddle.RailLengthMm, seat.RailOffsetMm)
	}
	if !isPositive(fit.FrameSeatAxisHeightMm) || !isPositive(fit.BrpHeightMm) {
		return fmt.Errorf("RiderFit FrameSeatAxisHeightMm (%.1f) and BrpHeightMm (%.1f) must be positive", fit.FrameSeatAxisHeightMm, fit.BrpHeightMm)
	}
	return nil
}
