package assembly

import (
	"cmp"
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

// FitTier classifies a build against the active fit gates (Q61, Q63, Q76, Q90).
type FitTier string

const (
	FitsAll     FitTier = "fits_all"
	FitsBRP     FitTier = "fits_brp"
	FitsGrip    FitTier = "fits_grip"
	FitsNeither FitTier = "fits_neither"
)

// Rank returns the sort priority of t (1 = best, 4 = worst).
func (t FitTier) Rank() int {
	switch t {
	case FitsAll:
		return 1
	case FitsBRP:
		return 2
	case FitsGrip:
		return 3
	case FitsNeither:
		return 4
	default:
		return 5
	}
}

// AttributeName names one of the 12 Q59a scoring attributes.
type AttributeName string

const (
	AttrHandGrip          AttributeName = "HandGrip"
	AttrSaddleBrpY        AttributeName = "Saddle BRP Y"
	AttrSaddleBrpX        AttributeName = "Saddle BRP X"
	AttrStem              AttributeName = "Stem"
	AttrHandlebarSweep    AttributeName = "Handlebar sweep"
	AttrForkSag           AttributeName = "Fork sag"
	AttrSpacers           AttributeName = "Spacers"
	AttrSaddleRailOffset  AttributeName = "Saddle rail offset"
	AttrRcWbRatio         AttributeName = "RC / WB ratio"
	AttrSeatpostInsertion AttributeName = "Seatpost insertion"
	AttrLeveledBbDrop     AttributeName = "Leveled BB drop"
	AttrTrail             AttributeName = "Trail"
)

// AttributeScore holds one attribute's 0–100 score and weight for a build.
// Active is false for the four saddle-dependent attributes when no BRP target is active (Q63, Q76).
type AttributeScore struct {
	Attribute AttributeName
	Weight    float64
	Score     float64
	Active    bool
}

// ScoreWeights holds the non-negative weights for the 12 Q59a attributes.
type ScoreWeights struct {
	HandGrip          float64
	SaddleBrpY        float64
	SaddleBrpX        float64
	Stem              float64
	HandlebarSweep    float64
	ForkSag           float64
	Spacers           float64
	SaddleRailOffset  float64
	RcWbRatio         float64
	SeatpostInsertion float64
	LeveledBbDrop     float64
	Trail             float64
}

// StemOption is one discrete stem option (length, angle, and 0–100 composite score, Q58, Q97, Q100).
type StemOption struct {
	LengthMm float64
	AngleDeg pose.Deg
	Score    float64
}

// BuildScore holds a placed build's fit tier, weighted 0–100 overall score,
// gate misses, and per-attribute breakdown.
type BuildScore struct {
	Tier       FitTier
	Total      float64
	GripR      float64
	HasBrp     bool
	BrpMissXMm float64 // BrpTarget.X - placed BRP.X; 0 when !HasBrp
	BrpMissYMm float64 // BrpTarget.Y - placed BRP.Y; 0 when !HasBrp
	Attributes []AttributeScore
}

// FitOval is the legacy Area of Reasonable Fit around a HandGrip target (Q17).
// Its major and minor axes pass through the HandGrip target center (not BB),
// with the major axis oriented at MajorAxisHeadingDeg in DynamicWorld.
type FitOval struct {
	SemiMinorMm, SemiMajorMm float64
	MajorAxisHeadingDeg      pose.Deg
}

// Validate checks the oval's semi-axes and heading.
func (o FitOval) Validate() error {
	if !isFinite(o.SemiMinorMm) || !isFinite(o.SemiMajorMm) || o.SemiMinorMm <= 0 || o.SemiMajorMm <= 0 {
		return fmt.Errorf("HandGrip oval: semi-axes must be positive and finite, got %v and %v mm", o.SemiMinorMm, o.SemiMajorMm)
	}
	if !isFinite(float64(o.MajorAxisHeadingDeg)) {
		return fmt.Errorf("HandGrip oval: major axis heading must be finite, got %v°", o.MajorAxisHeadingDeg)
	}
	return nil
}

// NormalizedRadius returns R for p around center (R = 1 on the oval's edge).
func (o FitOval) NormalizedRadius(center, p pose.Point) float64 {
	return newGripOvalProjection(o).normalizedRadius(center, p)
}

// gripOvalProjection holds an oval's precomputed unit-length direction vectors
// (minorDir along the minor axis, majorDir along the major axis; anchored at
// center in normalizedRadius) so FitBuilds can evaluate NormalizedRadius
// without repeating trig per candidate.
type gripOvalProjection struct {
	minorDir, majorDir       pose.Vec // unit vectors in DynamicWorld
	semiMinorMm, semiMajorMm float64
}

func newGripOvalProjection(o FitOval) gripOvalProjection {
	majorHeading := pose.NewHeading(DynamicWorld, o.MajorAxisHeadingDeg)
	return gripOvalProjection{
		minorDir:    pose.Along(majorHeading.Turn(-90), 1),
		majorDir:    pose.Along(majorHeading, 1),
		semiMinorMm: o.SemiMinorMm,
		semiMajorMm: o.SemiMajorMm,
	}
}

func (p gripOvalProjection) normalizedRadius(center, pt pose.Point) float64 {
	d := pt.Sub(center)
	minor := (d.X()*p.minorDir.X() + d.Y()*p.minorDir.Y()) / p.semiMinorMm
	major := (d.X()*p.majorDir.X() + d.Y()*p.majorDir.Y()) / p.semiMajorMm
	return pose.Vc(DynamicWorld, minor, major).Norm()
}

// DefaultGripOval returns the Q17 HandGrip fit oval.
func DefaultGripOval() FitOval {
	return FitOval{SemiMinorMm: 10, SemiMajorMm: 20, MajorAxisHeadingDeg: 60}
}

// Validate checks the stem's length, angle, and score.
func (s StemOption) Validate() error {
	if !isFinite(s.LengthMm) || s.LengthMm <= 0 {
		return fmt.Errorf("stem option length must be positive and finite, got %v mm", s.LengthMm)
	}
	if !isFinite(float64(s.AngleDeg)) {
		return fmt.Errorf("stem option angle must be finite, got %v°", s.AngleDeg)
	}
	if !isFinite(s.Score) || s.Score < 0 || s.Score > 100 {
		return fmt.Errorf("stem option score must be within 0–100, got %v", s.Score)
	}
	return nil
}

func (w ScoreWeights) all() [12]float64 {
	return [12]float64{
		w.HandGrip, w.SaddleBrpY, w.SaddleBrpX, w.Stem, w.HandlebarSweep, w.ForkSag,
		w.Spacers, w.SaddleRailOffset, w.RcWbRatio, w.SeatpostInsertion, w.LeveledBbDrop, w.Trail,
	}
}

// Validate checks that every weight is finite and non-negative and that they
// do not all vanish.
func (w ScoreWeights) Validate() error {
	var sum float64
	for _, v := range w.all() {
		if !isFinite(v) || v < 0 {
			return fmt.Errorf("score weights must be finite and non-negative, got %v", v)
		}
		sum += v
	}
	if sum <= 0 {
		return fmt.Errorf("score weights must have a positive total")
	}
	return nil
}

// ActiveTotal returns the sum of the weights that count: the 8 non-saddle
// weights, plus the 4 saddle weights when hasBrp.
func (w ScoreWeights) ActiveTotal(hasBrp bool) float64 {
	total := w.HandGrip + w.Stem + w.HandlebarSweep + w.ForkSag + w.Spacers + w.RcWbRatio + w.LeveledBbDrop + w.Trail
	if hasBrp {
		total += w.SaddleBrpY + w.SaddleBrpX + w.SaddleRailOffset + w.SeatpostInsertion
	}
	return total
}

func clamp01(x float64) float64 { return min(max(x, 0), 1) }

func scoreThreePoint(errMag, idealMax, gateMax, gateScore, worstMax float64) float64 {
	switch {
	case errMag <= idealMax:
		return 100
	case errMag <= gateMax:
		return 100 - (100-gateScore)*(errMag-idealMax)/(gateMax-idealMax)
	case errMag < worstMax:
		return gateScore * (worstMax - errMag) / (worstMax - gateMax)
	default:
		return 0
	}
}

func scoreRange(v, idealMin, idealMax, worstMin, worstMax float64) float64 {
	if v < idealMin {
		if math.IsInf(worstMin, -1) {
			return 100
		}
		return 100 * clamp01((v-worstMin)/(idealMin-worstMin))
	}
	if v > idealMax {
		if math.IsInf(worstMax, 1) {
			return 100
		}
		return 100 * clamp01((worstMax-v)/(worstMax-idealMax))
	}
	return 100
}

func scoreForkSag(travel ForkTravel, sagPct float64) float64 {
	minSag, maxSag, ok := travel.SagRange()
	if !ok {
		return 100
	}
	switch {
	case sagPct >= maxSag:
		return 100 * clamp01((maxSag+10-sagPct)/10)
	case sagPct >= minSag:
		return 90 + 10*(sagPct-minSag)/(maxSag-minSag)
	default:
		return 90 * clamp01((sagPct-(minSag-10))/10)
	}
}

func scoreInsertionMargin(insertionMm float64, bounds InsertionBounds) float64 {
	margin := min(insertionMm-bounds.EffectiveMinInsertionMm, bounds.MaxAllowedInsertionMm-insertionMm)
	return 100.0 * clamp01(margin/30.0)
}

func scoreHandGrip(gripR float64) float64 {
	return scoreThreePoint(gripR, 0, FitGateGripNormalizedRadius, 90, 2)
}

func scoreHandlebarSweep(backsweep pose.Deg) float64 {
	return scoreRange(float64(backsweep), 16, 20, 10, 26)
}

func scoreSpacers(spacersMm float64) float64 {
	return scoreRange(spacersMm, 0, 20, math.Inf(-1), DefaultMaxSpacersMm)
}

// forkScoreBase holds the fork-invariant parts of BuildScore computed once per
// fork so FitBuilds can rank candidates without allocating AttributeScore slices.
type forkScoreBase struct {
	hasBrp       bool
	passBrp      bool
	brpMissXMm   float64
	brpMissYMm   float64
	activeWeight float64
	handGripW    float64
	brpYW        float64
	brpXW        float64
	forkSagW     float64
	railW        float64
	rcWbW        float64
	insertW      float64
	bbDropW      float64
	trailW       float64
}

func newForkScoreBase(baseScore BuildScore, weights ScoreWeights) forkScoreBase {
	a := baseScore.Attributes
	weighted := func(i int) float64 {
		if !a[i].Active {
			return 0
		}
		return a[i].Weight * a[i].Score
	}
	return forkScoreBase{
		hasBrp:       baseScore.HasBrp,
		passBrp:      baseScore.Tier == FitsAll || baseScore.Tier == FitsBRP,
		brpMissXMm:   baseScore.BrpMissXMm,
		brpMissYMm:   baseScore.BrpMissYMm,
		activeWeight: weights.ActiveTotal(baseScore.HasBrp),
		handGripW:    weights.HandGrip,
		brpYW:        weighted(1),
		brpXW:        weighted(2),
		forkSagW:     weighted(5),
		railW:        weighted(7),
		rcWbW:        weighted(8),
		insertW:      weighted(9),
		bbDropW:      weighted(10),
		trailW:       weighted(11),
	}
}

// score returns a BuildScore without its Attributes slice, matching ScoreBuild
// on Tier, Total, GripR, and BRP misses for ranking candidates in FitBuilds.
func (b forkScoreBase) score(gripR, stemW, sweepW, spacersW float64) BuildScore {
	passGrip := gripR <= FitGateGripNormalizedRadius+fitBoundTolMm
	tier := FitsNeither
	if !b.hasBrp {
		if passGrip {
			tier = FitsGrip
		}
	} else {
		switch {
		case b.passBrp && passGrip:
			tier = FitsAll
		case b.passBrp:
			tier = FitsBRP
		case passGrip:
			tier = FitsGrip
		}
	}
	// Sum in the same attribute order as ScoreBuild so floating-point rounding matches.
	weighted := b.handGripW * scoreHandGrip(gripR)
	if b.hasBrp {
		weighted += b.brpYW
		weighted += b.brpXW
	}
	weighted += stemW
	weighted += sweepW
	weighted += b.forkSagW
	weighted += spacersW
	if b.hasBrp {
		weighted += b.railW
	}
	weighted += b.rcWbW
	if b.hasBrp {
		weighted += b.insertW
	}
	weighted += b.bbDropW
	weighted += b.trailW
	return BuildScore{
		Tier:       tier,
		Total:      weighted / b.activeWeight,
		GripR:      gripR,
		HasBrp:     b.hasBrp,
		BrpMissXMm: b.brpMissXMm,
		BrpMissYMm: b.brpMissYMm,
	}
}

// ScoreBuild classifies a placed build against the fit gates and scores its 12
// attributes 0–100, combined by weights (Q59a). The four saddle attributes
// count only when target.BRP is set (Q63, Q76).
func ScoreBuild(placement BikePlacement, target FitTarget, stemScore float64, weights ScoreWeights) (BuildScore, error) {
	if target.HandGrip == nil {
		return BuildScore{}, fmt.Errorf("build scoring needs a HandGrip target")
	}
	if !isFinitePoint(*target.HandGrip) {
		return BuildScore{}, invalidInput(InputFitTargetGrip, "HandGrip fit target must be finite, got (%v, %v) mm", target.HandGrip.X(), target.HandGrip.Y())
	}
	if target.BRP != nil && !isFinitePoint(*target.BRP) {
		return BuildScore{}, invalidInput(InputFitTargetBRP, "BRP fit target must be finite, got (%v, %v) mm", target.BRP.X(), target.BRP.Y())
	}
	if !isFinite(stemScore) || stemScore < 0 || stemScore > 100 {
		return BuildScore{}, fmt.Errorf("stem score must be within 0–100, got %v", stemScore)
	}
	if err := weights.Validate(); err != nil {
		return BuildScore{}, err
	}
	hasBrp := target.BRP != nil
	activeWeight := weights.ActiveTotal(hasBrp)
	if activeWeight <= 0 {
		return BuildScore{}, fmt.Errorf("score weights have zero active weight")
	}
	setup := placement.Setup
	var bounds InsertionBounds
	if hasBrp {
		var err error
		bounds, err = SeatpostInsertionBounds(setup.Frame, setup.Seatpost.Spec)
		if err != nil {
			return BuildScore{}, err
		}
	}

	gripR := DefaultGripOval().NormalizedRadius(*target.HandGrip, placement.Handlebar.HandGrip.Origin())
	rcWb := placement.Frameset.RcWbRatioPct()
	bbDrop := placement.Frameset.LeveledBbDropMm()
	trail := placement.Frameset.TrailMm()
	brpMiss := pose.Vc(DynamicWorld, 0, 0)
	if hasBrp {
		brpMiss = target.BRP.Sub(placement.Saddle.BRP.Origin())
	}
	for _, v := range []float64{gripR, rcWb, bbDrop, trail, brpMiss.X(), brpMiss.Y()} {
		if !isFinite(v) {
			return BuildScore{}, fmt.Errorf("build scoring metric is not finite")
		}
	}

	passGrip := gripR <= FitGateGripNormalizedRadius+fitBoundTolMm
	tier := FitsNeither
	if !hasBrp {
		if passGrip {
			tier = FitsGrip
		}
	} else {
		passBrp := math.Abs(brpMiss.Y()) <= FitGateBrpYMm+fitBoundTolMm && math.Abs(brpMiss.X()) <= FitGateBrpXMm+fitBoundTolMm
		switch {
		case passBrp && passGrip:
			tier = FitsAll
		case passBrp:
			tier = FitsBRP
		case passGrip:
			tier = FitsGrip
		}
	}

	forkSag := scoreForkSag(setup.Fork.Spec.Travel, setup.Fork.SagPct)
	w := weights
	attrs := []AttributeScore{
		{Attribute: AttrHandGrip, Weight: w.HandGrip, Score: scoreHandGrip(gripR), Active: true},
		{Attribute: AttrSaddleBrpY, Weight: w.SaddleBrpY, Active: hasBrp},
		{Attribute: AttrSaddleBrpX, Weight: w.SaddleBrpX, Active: hasBrp},
		{Attribute: AttrStem, Weight: w.Stem, Score: stemScore, Active: true},
		{Attribute: AttrHandlebarSweep, Weight: w.HandlebarSweep, Score: scoreHandlebarSweep(setup.Handlebar.BacksweepDeg), Active: true},
		{Attribute: AttrForkSag, Weight: w.ForkSag, Score: forkSag, Active: true},
		{Attribute: AttrSpacers, Weight: w.Spacers, Score: scoreSpacers(setup.Stem.SpacersMm), Active: true},
		{Attribute: AttrSaddleRailOffset, Weight: w.SaddleRailOffset, Active: hasBrp},
		{Attribute: AttrRcWbRatio, Weight: w.RcWbRatio, Score: scoreRange(rcWb, 37, 39, 34, 42), Active: true},
		{Attribute: AttrSeatpostInsertion, Weight: w.SeatpostInsertion, Active: hasBrp},
		{Attribute: AttrLeveledBbDrop, Weight: w.LeveledBbDrop, Score: scoreRange(bbDrop, 65, 75, 45, 95), Active: true},
		{Attribute: AttrTrail, Weight: w.Trail, Score: scoreRange(trail, 98, 106, 78, 126), Active: true},
	}
	if hasBrp {
		attrs[1].Score = scoreThreePoint(math.Abs(brpMiss.Y()), 0.5, FitGateBrpYMm, 50, 4)
		attrs[2].Score = scoreThreePoint(math.Abs(brpMiss.X()), 1, FitGateBrpXMm, 50, 20)
		attrs[7].Score = scoreRange(setup.Saddle.RailOffsetMm, -5, 5, -15, 15)
		attrs[9].Score = scoreInsertionMargin(setup.Seatpost.InsertionMm, bounds)
	}
	var weighted float64
	for _, a := range attrs {
		if a.Active {
			weighted += a.Weight * a.Score
		}
	}
	return BuildScore{
		Tier:       tier,
		Total:      weighted / activeWeight,
		GripR:      gripR,
		HasBrp:     hasBrp,
		BrpMissXMm: brpMiss.X(),
		BrpMissYMm: brpMiss.Y(),
		Attributes: attrs,
	}, nil
}

// CompareFitRank orders builds best first: by tier, then by the tier's
// tie-break chain (Q61, Q90). It returns a negative number when a ranks ahead of b.
func CompareFitRank(a, b BuildScore) int {
	if order := cmp.Compare(a.Tier.Rank(), b.Tier.Rank()); order != 0 {
		return order
	}
	missY := cmp.Compare(math.Abs(a.BrpMissYMm), math.Abs(b.BrpMissYMm))
	missX := cmp.Compare(math.Abs(a.BrpMissXMm), math.Abs(b.BrpMissXMm))
	score := cmp.Compare(b.Total, a.Total)
	grip := cmp.Compare(a.GripR, b.GripR)
	first := func(orders ...int) int {
		for _, o := range orders {
			if o != 0 {
				return o
			}
		}
		return 0
	}
	switch a.Tier {
	case FitsAll:
		return first(score, grip, missY, missX)
	case FitsGrip:
		return first(score, missY, missX, grip)
	case FitsBRP, FitsNeither:
		return first(grip, score, missY, missX)
	default:
		return 0
	}
}
