package geometry

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/model"
)

// ToRad converts degrees to radians.
func ToRad(deg float64) float64 {
	return (deg * math.Pi) / 180.0
}

// ToDeg converts radians to degrees.
func ToDeg(rad float64) float64 {
	return (rad * 180.0) / math.Pi
}

// Round rounds a float to the specified decimal precision.
func Round(val float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(val*factor) / factor
}

// CalculateAett calculates the Actual Effective Top Tube (AETT) length in mm.
// AETT is the horizontal projection from the center of the seat tube
// (at the specified rider saddle height) forward to the steering axis.
func CalculateAett(reach, stack, htaDeg, staDeg, seatHeight float64) float64 {
	hta := ToRad(htaDeg)
	sta := ToRad(staDeg)

	// Coordinates of the seat position relative to Bottom Bracket (0,0)
	sY := seatHeight * math.Sin(sta)
	sX := -seatHeight * math.Cos(sta)

	// X-coordinate intersection on the steering axis at height sY
	iX := reach + (stack-sY)/math.Tan(hta)

	return iX - sX
}

// CalculateActualSetback calculates the horizontal setback distance in mm from the bottom bracket center (0,0)
// to the center of the seat tube / seatpost at the specified saddle height.
func CalculateActualSetback(staDeg, seatHeight float64) float64 {
	sta := ToRad(staDeg)
	return Round(seatHeight*math.Cos(sta), 1)
}

// CalculateSteererStack calculates the 'spare' steerer tube required to bring
// the handlebars level with the saddle.
// Returns:
// - verticalStack: Pure vertical distance from head tube top to saddle horizontal line (mm).
// - axialLength: Physical length along the angled steerer tube to achieve that height (mm).
func CalculateSteererStack(stack, staDeg, htaDeg, seatHeight float64) (verticalStack float64, axialLength float64) {
	sta := ToRad(staDeg)
	hta := ToRad(htaDeg)

	sY := seatHeight * math.Sin(sta)
	verticalStack = sY - stack
	axialLength = verticalStack / math.Sin(hta)

	return verticalStack, axialLength
}

// CalculateGeometricA2c reverse-engineers the geometric Structural Axle-to-Crown (A2C) length
// that CAD software assumed to generate the Stack number.
//
// Master Stack Equation:
// Stack = BB_drop - Rake * cos(HTA) + (Structural_A2C + HTL) * sin(HTA)
// => Structural_A2C = (Stack - BB_drop + Rake * cos(HTA)) / sin(HTA) - HTL
//
// Structural_A2C includes bare fork A2C plus any external lower cup stack.
func CalculateGeometricA2c(stack, bbDrop, htl, htaDeg, rake, cupStack float64) (structuralA2c float64, forkA2c float64) {
	hta := ToRad(htaDeg)

	structuralA2c = (stack - bbDrop + rake*math.Cos(hta))/math.Sin(hta) - htl
	forkA2c = structuralA2c - cupStack

	return structuralA2c, forkA2c
}

// CalculateDynamicGeometry calculates new frame geometry when fork A2C is changed,
// simulating pitching of the frame around the rear axle.
func CalculateDynamicGeometry(
	baseReach, baseStack, baseHta, baseSta, baseBbDrop, baseStructuralA2c, newStructuralA2c float64,
	wheelbase, chainstayLength float64,
) model.DynamicGeometryResult {
	if wheelbase <= 0 {
		wheelbase = 1150.0
	}
	if chainstayLength <= 0 {
		chainstayLength = 430.0
	}

	deltaA2c := newStructuralA2c - baseStructuralA2c

	// Vertical lift at the front axle
	baseHtaRad := ToRad(baseHta)
	deltaY := deltaA2c * math.Sin(baseHtaRad)

	// Rotation angle around the rear axle (radians and degrees)
	sinTheta := math.Max(-1.0, math.Min(1.0, deltaY/wheelbase))
	thetaRad := math.Asin(sinTheta)
	thetaDeg := ToDeg(thetaRad)

	newHta := baseHta - thetaDeg
	newSta := baseSta - thetaDeg

	// Rotate Reach/Stack vector around Bottom Bracket
	newReach := baseReach*math.Cos(thetaRad) - baseStack*math.Sin(thetaRad)
	newStack := baseReach*math.Sin(thetaRad) + baseStack*math.Cos(thetaRad)

	// Compute Bottom Bracket displacement pivoting around rear axle (0,0)
	cs := math.Max(chainstayLength, baseBbDrop+1.0)
	xBb := math.Sqrt(math.Max(0, cs*cs-baseBbDrop*baseBbDrop))

	// In rear axle coordinate system, BB initial coords are (xBb, -baseBbDrop)
	// Rotated by theta: yBb_new = xBb * sin(theta) + (-baseBbDrop) * cos(theta)
	// New BB drop is -yBb_new:
	newBbDrop := baseBbDrop*math.Cos(thetaRad) - xBb*math.Sin(thetaRad)

	return model.DynamicGeometryResult{
		NewHta:        Round(newHta, 2),
		NewSta:        Round(newSta, 2),
		NewReach:      Round(newReach, 1),
		NewStack:      Round(newStack, 1),
		NewBbDrop:     Round(newBbDrop, 1),
		DeltaAngleDeg: Round(thetaDeg, 2),
		DeltaReach:    Round(newReach-baseReach, 1),
		DeltaStack:    Round(newStack-baseStack, 1),
		DeltaBbDrop:   Round(newBbDrop-baseBbDrop, 1),
		TotalNewA2c:   Round(newStructuralA2c, 1),
	}
}

// CalculateFrontCenter calculates theoretical horizontal Front-Center from frame geometry.
func CalculateFrontCenter(reach, htl, htaDeg, rake, structuralA2c float64) float64 {
	hta := ToRad(htaDeg)
	return reach + (structuralA2c+htl)*math.Cos(hta) + rake*math.Sin(hta)
}

// CalculateFrontCenter3D calculates direct point-to-point Front-Center from the bottom bracket spindle to the front axle.
func CalculateFrontCenter3D(reach, htl, htaDeg, rake, structuralA2c, bbDrop float64) float64 {
	fcHoriz := CalculateFrontCenter(reach, htl, htaDeg, rake, structuralA2c)
	return math.Hypot(fcHoriz, bbDrop)
}

// BiomechanicalPosture models the rider's contact points and upper body posture on a baseline bike.
type BiomechanicalPosture struct {
	BackAngleDeg float64 // Torso angle from horizontal in degrees (hip to shoulder)
	HipX         float64 // Hip joint X coordinate relative to BB (mm)
	HipY         float64 // Hip joint Y coordinate relative to BB (mm)
	GripX        float64 // Grip center X coordinate relative to BB (mm)
	GripY        float64 // Grip center Y coordinate relative to BB (mm)
	RiderCgX     float64 // Rider center of gravity X relative to BB (mm)
	RiderCgY     float64 // Rider center of gravity Y relative to BB (mm)
	SystemCgX    float64 // Combined rider + bike center of gravity X relative to BB (mm)
	SystemCgY    float64 // Combined rider + bike center of gravity Y relative to BB (mm)
}

// WeightDistributionResult holds the static weight distribution on front and rear axles.
type WeightDistributionResult struct {
	FrontPct        float64 // Percentage of system weight on front axle
	RearPct         float64 // Percentage of system weight on rear axle
	FrontCenterCalc float64 // Horizontal distance from BB to front axle (mm)
	RearCenterCalc  float64 // Horizontal distance from BB to rear axle (mm)
	WheelbaseCalc   float64 // Total horizontal wheelbase (mm)
	BackAngleDeg    float64 // Estimated back angle in degrees
}

// GetDefaultBaselineBike returns the canonical Otso Fenrir Stainless Large baseline build
// (rigid 495mm fork, EC44 lower cup, 780mm saddle height).
func GetDefaultBaselineBike() model.BikeGeometry {
	return model.BikeGeometry{
		ID:              "otso-fenrir-stainless-l",
		Manufacturer:    "Otso",
		Model:           "Fenrir",
		Size:            "L",
		Reach:           428.0,
		Stack:           642.0,
		HtaDeg:          68.25,
		StaDeg:          75.25,
		Htl:             140.0,
		BbDrop:          72.0,
		ForkRake:        54.0,
		Wheelbase:       1158.0,
		ChainstayLength: 450.0,
		SeatHeight:      780.0,
		ForkA2c:         495.0,
		HeadsetCupType:  model.CupEC44,
	}
}

// CalculateBaselinePosture models the rider's contact points, back angle, and Center of Gravity (CoG).
func CalculateBaselinePosture(
	bike model.BikeGeometry,
	profile model.RiderProfile,
	seatHeight float64,
) BiomechanicalPosture {
	if seatHeight <= 0 {
		seatHeight = 780.0
	}
	if profile.HeightIn <= 0 {
		profile = model.DefaultRiderProfile()
	}

	staDeg := bike.StaDeg
	if staDeg <= 0 {
		staDeg = 74.71
	}
	staRad := ToRad(staDeg)

	htaDeg := bike.HtaDeg
	if htaDeg <= 0 {
		htaDeg = 67.71
	}

	// 1. Hip joint coordinates relative to Bottom Bracket (0,0)
	seatAxisX := -seatHeight * math.Cos(staRad)
	seatAxisY := seatHeight * math.Sin(staRad)
	saddleX := seatAxisX - profile.BaselineSaddleSetbackOffsetMm
	saddleY := seatAxisY
	hipX := saddleX
	hipY := saddleY + 35.0 // Greater trochanter / acetabulum sits ~35mm above saddle top

	// 2. Handlebar grip coordinates relative to Bottom Bracket (0,0)
	// Steerer angle from horizontal pointing forward-up is 90 - HTA
	stemAngleRad := ToRad(90.0 - htaDeg + profile.BaselineStemAngleDeg)
	stemX := profile.BaselineStemMm * math.Cos(stemAngleRad)
	stemY := profile.BaselineStemMm * math.Sin(stemAngleRad)

	reach := bike.Reach
	if reach <= 0 {
		reach = 421.9
	}
	stack := bike.Stack
	if stack <= 0 {
		stack = 646.0
	}

	clampX := reach + stemX
	clampY := stack + stemY

	// Bar sweep & rise
	halfWidth := profile.BaselineBarWidthMm / 2.0
	if halfWidth <= 0 {
		halfWidth = 400.0
	}
	sweepX := -halfWidth * math.Sin(ToRad(profile.BaselineBarSweepDeg))
	gripX := clampX + sweepX
	gripY := clampY + profile.BaselineBarRiseMm

	// 3. Biomechanical Triangle: Hip, Shoulder, Grip
	heightMm := profile.HeightIn * 25.4
	torsoLen := 0.288 * heightMm // Hip to shoulder (~541mm for 74")
	armLen := 0.380 * heightMm   // Shoulder to hand (~714mm for 74")

	// 3D lateral arm spread to 800mm wide bar:
	// Shoulder half-breadth ~200mm, grip half-breadth ~390mm -> lateral offset ~190mm
	lateralOffset := math.Max(0, halfWidth-200.0)
	// Effective arm reach projected in the sagittal plane with comfortable ~25° elbow flexion
	sagittalArmReach := math.Sqrt(math.Max(100.0, (0.88*armLen)*(0.88*armLen)-lateralOffset*lateralOffset))

	dx := gripX - hipX
	dy := gripY - hipY
	distHipGrip := math.Hypot(dx, dy)

	alpha := math.Atan2(dy, dx)
	cosBeta := (torsoLen*torsoLen + distHipGrip*distHipGrip - sagittalArmReach*sagittalArmReach) / (2.0 * torsoLen * distHipGrip)
	cosBeta = math.Max(-1.0, math.Min(1.0, cosBeta))
	beta := math.Acos(cosBeta)

	backAngleRad := alpha + beta
	backAngleDeg := ToDeg(backAngleRad)

	// 4. Rider Center of Gravity
	shoulderX := hipX + torsoLen*math.Cos(backAngleRad)
	shoulderY := hipY + torsoLen*math.Sin(backAngleRad)

	// Torso + Head (58% of body mass)
	upperX := hipX + 0.62*torsoLen*math.Cos(backAngleRad)
	upperY := hipY + 0.62*torsoLen*math.Sin(backAngleRad)

	// Arms (10% of body mass)
	armX := (shoulderX + gripX) / 2.0
	armY := (shoulderY + gripY) / 2.0

	// Legs (32% of body mass) between Hip and Pedals (0,0)
	legX := 0.45 * hipX
	legY := 0.40 * hipY

	riderCgX := 0.58*upperX + 0.10*armX + 0.32*legX
	riderCgY := 0.58*upperY + 0.10*armY + 0.32*legY

	// 5. System Center of Gravity (Rider + Bike)
	riderMassKg := profile.RiderWeightLbs * 0.45359237
	bikeMassKg := profile.BikeWeightLbs * 0.45359237
	bikeCgX := 0.0
	bikeCgY := 280.0 // Center of mass of hardtail frame/wheels/drivetrain near BB

	totalMass := riderMassKg + bikeMassKg
	systemCgX := (riderMassKg*riderCgX + bikeMassKg*bikeCgX) / totalMass
	systemCgY := (riderMassKg*riderCgY + bikeMassKg*bikeCgY) / totalMass

	return BiomechanicalPosture{
		BackAngleDeg: Round(backAngleDeg, 1),
		HipX:         Round(hipX, 1),
		HipY:         Round(hipY, 1),
		GripX:        Round(gripX, 1),
		GripY:        Round(gripY, 1),
		RiderCgX:     Round(riderCgX, 1),
		RiderCgY:     Round(riderCgY, 1),
		SystemCgX:    Round(systemCgX, 1),
		SystemCgY:    Round(systemCgY, 1),
	}
}

// CalculateWeightDistribution calculates static weight distribution between the axles for an active frame setup.
func CalculateWeightDistribution(
	activeReach, activeStack, activeHta, activeHtl, activeBbDrop, forkRake, structuralA2c, chainstayLength float64,
	posture BiomechanicalPosture,
) WeightDistributionResult {
	if chainstayLength <= 0 {
		chainstayLength = 440.0
	}
	cs := math.Max(chainstayLength, activeBbDrop+1.0)
	rearCenter := math.Sqrt(math.Max(0, cs*cs-activeBbDrop*activeBbDrop))
	xRear := -rearCenter

	frontCenter := CalculateFrontCenter(activeReach, activeHtl, activeHta, forkRake, structuralA2c)
	wheelbase := frontCenter + rearCenter
	if wheelbase <= 0 {
		wheelbase = 1150.0
	}

	distRear := posture.SystemCgX - xRear
	frontPct := (distRear / wheelbase) * 100.0
	rearPct := 100.0 - frontPct

	return WeightDistributionResult{
		FrontPct:        Round(frontPct, 1),
		RearPct:         Round(rearPct, 1),
		FrontCenterCalc: Round(frontCenter, 1),
		RearCenterCalc:  Round(rearCenter, 1),
		WheelbaseCalc:   Round(wheelbase, 1),
		BackAngleDeg:    Round(posture.BackAngleDeg, 1),
	}
}

// AnalyzeBikeFit runs the master analysis pipeline:
// reverse-engineers CAD baseline, applies optional canonical/target fork modifications,
// and derives all final dynamic cockpit, frame, and weight distribution metrics.
func AnalyzeBikeFit(
	bike model.BikeGeometry,
	targetA2c *float64,
	overrideSeatHeight *float64,
) model.BikeFitAnalysis {
	return AnalyzeBikeFitWithProfile(bike, targetA2c, overrideSeatHeight, nil)
}

// AnalyzeBikeFitWithProfile runs the master analysis pipeline with a customized RiderProfile.
func AnalyzeBikeFitWithProfile(
	bike model.BikeGeometry,
	targetA2c *float64,
	overrideSeatHeight *float64,
	profile *model.RiderProfile,
) model.BikeFitAnalysis {
	seatHeight := 750.0
	if overrideSeatHeight != nil && *overrideSeatHeight > 0 {
		seatHeight = *overrideSeatHeight
	} else if bike.SeatHeight > 0 {
		seatHeight = bike.SeatHeight
	}

	cupStack := bike.HeadsetCupType.CupStackMm()
	geoSpacer := cupStack
	if bike.GeoHeadsetSpacerMm != nil {
		geoSpacer = *bike.GeoHeadsetSpacerMm
	}

	// 1. Reverse engineer baseline CAD geometric A2C
	baseStructuralA2c, baseForkA2c := CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
		geoSpacer,
	)

	activeReach := bike.Reach
	activeStack := bike.Stack
	activeHta := bike.HtaDeg
	activeSta := bike.StaDeg
	activeBbDrop := bike.BbDrop
	activeStructuralA2c := baseStructuralA2c
	deltaAngle := 0.0
	deltaReach := 0.0
	deltaStack := 0.0
	deltaBbDrop := 0.0

	// 2. If target A2C provided, apply dynamic pitching using physical headset_cup_type stack
	if targetA2c != nil {
		totalNewA2c := *targetA2c + cupStack

		wb := bike.Wheelbase
		if wb <= 0 {
			wb = 1150.0
		}
		cs := bike.ChainstayLength
		if cs <= 0 {
			cs = 430.0
		}

		dyn := CalculateDynamicGeometry(
			bike.Reach,
			bike.Stack,
			bike.HtaDeg,
			bike.StaDeg,
			bike.BbDrop,
			baseStructuralA2c,
			totalNewA2c,
			wb,
			cs,
		)

		activeReach = dyn.NewReach
		activeStack = dyn.NewStack
		activeHta = dyn.NewHta
		activeSta = dyn.NewSta
		activeBbDrop = dyn.NewBbDrop
		activeStructuralA2c = dyn.TotalNewA2c
		deltaAngle = dyn.DeltaAngleDeg
		deltaReach = dyn.DeltaReach
		deltaStack = dyn.DeltaStack
		deltaBbDrop = dyn.DeltaBbDrop
	}

	// 3. Derived cockpit metrics
	aett := CalculateAett(activeReach, activeStack, activeHta, activeSta, seatHeight)
	actualSetback := CalculateActualSetback(activeSta, seatHeight)
	verticalStack, axialLength := CalculateSteererStack(activeStack, activeSta, activeHta, seatHeight)

	// 4. Infer CAD publish mode
	inferredPublishMode := "Unspecified"
	if bike.ForkA2c > 0 {
		diff := bike.ForkA2c - baseForkA2c
		a2cSlop := bike.GetGeoSlopMm().GetA2c()
		if math.Abs(diff) <= a2cSlop {
			inferredPublishMode = "Static Fork (0% Sag CAD)"
		} else if diff >= 15.0 && diff <= 35.0 {
			inferredPublishMode = fmt.Sprintf("Sagged (~20%% Sag CAD, ~%dmm sag)", int(math.Round(diff)))
		} else {
			inferredPublishMode = fmt.Sprintf("Discrepancy (%dmm deviation)", int(math.Round(diff)))
		}
	}

	// 5. Biomechanical Posture & Weight Distribution
	riderProf := model.DefaultRiderProfile()
	if profile != nil {
		riderProf = *profile
	}
	baselineBike := GetDefaultBaselineBike()
	posture := CalculateBaselinePosture(baselineBike, riderProf, seatHeight)

	activeCS := bike.ChainstayLength
	if activeCS <= 0 {
		activeCS = 440.0
	}
	csDesc := fmt.Sprintf("%.1f mm [fixed]", activeCS)
	if bike.HasAdjustableChainstay() {
		activeCS = bike.GetMaxChainstay()
		csDesc = fmt.Sprintf("%.1f mm [max of %.0f-%.0f]", activeCS, bike.GetMinChainstay(), bike.GetMaxChainstay())
	}

	htl := bike.Htl
	if htl <= 0 {
		htl = 120.0
	}
	rake := bike.ForkRake
	if rake <= 0 {
		rake = 44.0
	}

	weightDist := CalculateWeightDistribution(
		activeReach,
		activeStack,
		activeHta,
		htl,
		activeBbDrop,
		rake,
		activeStructuralA2c,
		activeCS,
		posture,
	)

	return model.BikeFitAnalysis{
		StructuralA2c:       Round(activeStructuralA2c, 1),
		ActiveReach:         Round(activeReach, 1),
		ActiveStack:         Round(activeStack, 1),
		ActiveHta:           Round(activeHta, 2),
		ActiveSta:           Round(activeSta, 2),
		ActiveBbDrop:        Round(activeBbDrop, 1),
		Aett:                Round(aett, 1),
		ActualSetback:       actualSetback,
		SpareVerticalStack:  Round(verticalStack, 1),
		SpareAxialSteerer:   Round(axialLength, 1),
		DeltaAngleDeg:       Round(deltaAngle, 2),
		DeltaReach:          Round(deltaReach, 1),
		DeltaStack:          Round(deltaStack, 1),
		DeltaBbDrop:         Round(deltaBbDrop, 1),
		InferredPublishMode: inferredPublishMode,

		WeightDistFrontPct: weightDist.FrontPct,
		WeightDistRearPct:  weightDist.RearPct,
		BackAngleDeg:       weightDist.BackAngleDeg,
		FrontCenterCalc:    weightDist.FrontCenterCalc,
		RearCenterCalc:     weightDist.RearCenterCalc,
		WheelbaseCalc:      weightDist.WheelbaseCalc,
		ActiveChainstay:    Round(activeCS, 1),
		ChainstayDesc:      csDesc,
	}
}

// MaterializeBikeGeometry projects a FrameModel for a given size and configuration into a BikeGeometry instance.
// If the target configuration has explicit published geometry for the size, it is used directly.
// If explicit values are missing, it derives them dynamically by pitching the baseline geometry around the rear axle.
func MaterializeBikeGeometry(frame model.FrameModel, size string, configID string) (model.BikeGeometry, error) {
	if size == "" {
		size = frame.GetDefaultSize()
	}

	targetConfig := frame.GetConfiguration(configID)
	if targetConfig == nil {
		targetConfig = frame.GetBaselineConfiguration()
	}
	if targetConfig == nil {
		return model.BikeGeometry{}, fmt.Errorf("frame %s has no configurations", frame.ID)
	}

	geoSpacer := frame.GetGeoHeadsetSpacerMm()

	bikeID := frame.ID
	if !strings.HasSuffix(strings.ToLower(bikeID), "-"+strings.ToLower(size)) {
		bikeID = fmt.Sprintf("%s-%s", frame.ID, strings.ToLower(size))
	}
	if !targetConfig.IsBaseline && targetConfig.ID != "" && targetConfig.ID != "stock" {
		bikeID = fmt.Sprintf("%s-%s", bikeID, targetConfig.ID)
	}

	res := model.BikeGeometry{
		ID:                     bikeID,
		Manufacturer:           frame.Manufacturer,
		Model:                  frame.Model,
		Size:                   size,
		Year:                   frame.Year,
		Category:               frame.Category,
		Notes:                  frame.Notes,
		WheelSize:              frame.WheelSize,
		HeadsetCupType:         frame.HeadsetCupType,
		GeoHeadsetSpacerMm:     frame.GeoHeadsetSpacerMm,
		GeoSlopMm:              frame.GeoSlopMm,
		SeatHeight:             frame.SeatHeight,
		Fork:                   targetConfig.Fork,
		ForkTravelMm:           targetConfig.Fork.TravelMm,
		ForkA2c:                targetConfig.Fork.A2c,
		ForkRake:               targetConfig.Fork.Rake,
		SagPercentagePublished: targetConfig.SagPercentagePublished,
		ChainstayRangeMm:       frame.ChainstayRangeMm,
	}

	// 1. Direct explicit geometry match
	if targetConfig.Geometries != nil {
		if geo, ok := targetConfig.Geometries[size]; ok && geo.Reach > 0 && geo.Stack > 0 {
			res.Reach = geo.Reach
			res.Stack = geo.Stack
			res.HtaDeg = geo.HtaDeg
			res.StaDeg = geo.StaDeg
			res.Htl = geo.Htl
			res.BbDrop = geo.BbDrop
			res.Wheelbase = geo.Wheelbase
			res.ChainstayLength = geo.ChainstayLength
			res.FrontCenter = geo.FrontCenter
			res.SeatTubeLength = geo.SeatTubeLength
			return res, nil
		}
	}

	// 2. Kinematic derivation from baseline configuration
	baseConfig := frame.GetBaselineConfiguration()
	if baseConfig == nil {
		return res, fmt.Errorf("baseline configuration missing for frame %s", frame.ID)
	}

	var baseGeo model.SizeGeometry
	var foundBase bool
	if baseConfig.Geometries != nil {
		baseGeo, foundBase = baseConfig.Geometries[size]
	}
	if !foundBase {
		// Fallback: search any other config for this size
		for _, cfg := range frame.Configurations {
			if cfg.Geometries != nil {
				if g, ok := cfg.Geometries[size]; ok && g.Reach > 0 {
					baseGeo = g
					foundBase = true
					break
				}
			}
		}
	}
	if !foundBase {
		return res, fmt.Errorf("size %s not defined in frame %s", size, frame.ID)
	}

	baseStructuralA2c := baseConfig.Fork.A2c + geoSpacer
	targetStructuralA2c := targetConfig.Fork.A2c + geoSpacer

	dyn := CalculateDynamicGeometry(
		baseGeo.Reach,
		baseGeo.Stack,
		baseGeo.HtaDeg,
		baseGeo.StaDeg,
		baseGeo.BbDrop,
		baseStructuralA2c,
		targetStructuralA2c,
		baseGeo.Wheelbase,
		baseGeo.ChainstayLength,
	)

	res.Reach = dyn.NewReach
	res.Stack = dyn.NewStack
	res.HtaDeg = dyn.NewHta
	res.StaDeg = dyn.NewSta
	res.Htl = baseGeo.Htl
	res.BbDrop = dyn.NewBbDrop
	res.Wheelbase = baseGeo.Wheelbase
	res.ChainstayLength = baseGeo.ChainstayLength
	res.FrontCenter = baseGeo.FrontCenter
	res.SeatTubeLength = baseGeo.SeatTubeLength

	return res, nil
}

