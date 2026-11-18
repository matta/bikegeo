package geometry

import (
	"fmt"
	"math"

	"bikegeo/internal/assembly"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

const (
	brpRadiusResidualTolMm = 1e-8
	railAxisParallelTol    = 1e-12
)

// SaddleRailSolution holds the verified result of solving the saddle rail clamp
// offset and BB-to-clamp distance along the effective seat axis for a target BRP position.
// Callers must check the error returned by SolveSaddleForBRPTarget before using Placement.
type SaddleRailSolution struct {
	Placement assembly.SaddlePlacement
}

// SaddleStaSolution holds the verified result of solving the effective seat-tube
// angle and BB-to-clamp distance along the effective seat axis for a target BRP position.
// Callers must check the error returned by SolveSaddleSTAForBRPTarget before using these fields.
type SaddleStaSolution struct {
	EffStaDeg model.Deg
	Placement assembly.SaddlePlacement
}

func upperCircleBRPTarget(brpHeightMm, brpSetbackMm float64) (geom2d.Point2, error) {
	if !isFinite(brpHeightMm) || brpHeightMm <= 0 {
		return geom2d.Point2{}, fmt.Errorf("requested BRP height must be finite and positive, got %v mm", brpHeightMm)
	}
	if !isFinite(brpSetbackMm) {
		return geom2d.Point2{}, fmt.Errorf("requested BRP setback must be finite, got %v mm", brpSetbackMm)
	}
	rSq := brpHeightMm * brpHeightMm
	xSq := brpSetbackMm * brpSetbackMm
	ySq := rSq - xSq
	if !isFinite(rSq) || !isFinite(xSq) || !isFinite(ySq) {
		return geom2d.Point2{}, fmt.Errorf("saddle BRP-target solve overflow for radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	if math.Abs(brpSetbackMm) >= brpHeightMm || ySq <= 0 {
		return geom2d.Point2{}, fmt.Errorf("unreachable upper-circle BRP target: radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	targetY := math.Sqrt(ySq)
	if !isFinite(targetY) || !(targetY > 0) {
		return geom2d.Point2{}, fmt.Errorf("unreachable upper-circle BRP target: radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	return geom2d.Point2{X: -brpSetbackMm, Y: targetY}, nil
}

// SolveSaddleSTAForBRPTarget solves the effective seat-tube angle in (0°, 90°)
// and non-negative BB-to-clamp distance along the effective seat axis that place the saddle's BRP at
// (-brpSetbackMm, +sqrt(R^2 - setback^2)) on the upper half of the brpHeightMm
// radius circle at railOffsetMm, executes checked forward assembly, and
// verifies both target coordinates within 1e-8 mm.
//
// Equation:
// Subtracting the assembly clamp-to-BRP displacement at railOffsetMm from the
// BRP target gives the required clamp point C relative to BB. With rho = |C|
// and perpendicular post setback p:
//
//	STA = atan2(C.Y, -C.X) + asin(p / rho)
//	BB-to-clamp distance along seat axis = sqrt(rho^2 - p^2)
func SolveSaddleSTAForBRPTarget(
	saddleSpec model.SaddleSpec,
	brpHeightMm, brpSetbackMm, postSetbackMm, railOffsetMm float64,
) (SaddleStaSolution, error) {
	target, err := upperCircleBRPTarget(brpHeightMm, brpSetbackMm)
	if err != nil {
		return SaddleStaSolution{}, err
	}
	if !isFinite(postSetbackMm) || postSetbackMm < 0 {
		return SaddleStaSolution{}, fmt.Errorf("seatpost setback must be finite and non-negative, got %v mm", postSetbackMm)
	}
	disp, err := assembly.SaddleClampToBRP(saddleSpec, railOffsetMm)
	if err != nil {
		return SaddleStaSolution{}, err
	}

	clamp := target.SubVec(disp)
	if !isFinite(clamp.X) || !isFinite(clamp.Y) {
		return SaddleStaSolution{}, fmt.Errorf("saddle STA solve overflow for clamp point")
	}
	rho := clamp.DistanceToOrigin()
	if !isFinite(rho) {
		return SaddleStaSolution{}, fmt.Errorf("saddle STA solve overflow for clamp radius")
	}
	if !(rho > 0) {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: zero clamp radius leaves seat-axis angle undetermined")
	}
	if rho < postSetbackMm {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: post setback %.1f mm exceeds clamp radius %.1f mm", postSetbackMm, rho)
	}
	ratio := postSetbackMm / rho
	if !isFinite(ratio) || ratio < 0 || ratio > 1 {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: invalid post setback ratio %v", ratio)
	}
	rhoSq := rho * rho
	pSq := postSetbackMm * postSetbackMm
	radicand := rhoSq - pSq
	if !isFinite(rhoSq) || !isFinite(pSq) || !isFinite(radicand) {
		return SaddleStaSolution{}, fmt.Errorf("saddle STA solve overflow for clamp radius %.1f mm, post setback %.1f mm", rho, postSetbackMm)
	}
	if radicand < 0 {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: negative BB-to-clamp seat-axis distance radicand %v", radicand)
	}
	bbToClampAlongSeatAxisMm := math.Sqrt(radicand)
	if !isFinite(bbToClampAlongSeatAxisMm) || bbToClampAlongSeatAxisMm < 0 {
		return SaddleStaSolution{}, fmt.Errorf("saddle STA solve overflow for BB-to-clamp distance along seat axis")
	}

	effStaDeg := model.DegOf(geom2d.AngleFromSlope(-clamp.X, clamp.Y)) + model.DegOf(geom2d.FromRad(math.Asin(ratio)))
	if !isFinite(float64(effStaDeg)) || effStaDeg <= 0 || effStaDeg >= 90 {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: solved effective STA %.2f° outside (0°, 90°)", effStaDeg)
	}

	placement, err := assembly.PlaceSaddle(geom2d.Origin(), effStaDeg, assembly.SaddleSetup{
		Saddle:                   saddleSpec,
		BBToClampAlongSeatAxisMm: bbToClampAlongSeatAxisMm,
		PostSetbackMm:            postSetbackMm,
		RailOffsetMm:             railOffsetMm,
	})
	if err != nil {
		return SaddleStaSolution{}, err
	}

	brp := placement.BRP
	if !isFinite(brp.X) || !isFinite(brp.Y) {
		return SaddleStaSolution{}, fmt.Errorf("saddle STA verification failed: non-finite BRP (%v, %v)", brp.X, brp.Y)
	}
	if !(brp.Y > 0) {
		return SaddleStaSolution{}, fmt.Errorf("impossible saddle placement: BRP Y=%.2f mm is not above BB", brp.Y)
	}
	resX := math.Abs(brp.X - target.X)
	resY := math.Abs(brp.Y - target.Y)
	if !isFinite(resX) || !isFinite(resY) || resX > brpRadiusResidualTolMm || resY > brpRadiusResidualTolMm {
		return SaddleStaSolution{}, fmt.Errorf(
			"saddle STA BRP-target verification failed: got (%.9f, %.9f) mm, want (%.9f, %.9f) mm (residual %.3e, %.3e mm)",
			brp.X, brp.Y, target.X, target.Y, resX, resY,
		)
	}

	return SaddleStaSolution{
		EffStaDeg: effStaDeg,
		Placement: placement,
	}, nil
}

// SolveSaddleForBRPTarget solves the BB-to-clamp distance along the effective
// seat axis and saddle rail clamp offset that place the saddle's BRP at
// (-brpSetbackMm, +sqrt(R^2 - setback^2)) on the upper half of the brpHeightMm
// radius circle, executes checked forward assembly, and verifies both target
// coordinates within 1e-8 mm.
//
// Equation:
// For fixed rear-cockpit inputs, BRP(d, r) = q + d*u + r*v, where q is the BRP
// at zero BB-to-clamp distance along the seat axis and zero rail offset, u is
// the unit seat-axis direction, and v is the unit rearward rail-offset direction.
func SolveSaddleForBRPTarget(
	saddleSpec model.SaddleSpec,
	effStaDeg model.Deg,
	brpHeightMm, brpSetbackMm, postSetbackMm float64,
) (SaddleRailSolution, error) {
	target, err := upperCircleBRPTarget(brpHeightMm, brpSetbackMm)
	if err != nil {
		return SaddleRailSolution{}, err
	}
	zeroPlacement, err := assembly.PlaceSaddle(geom2d.Origin(), effStaDeg, assembly.SaddleSetup{
		Saddle:        saddleSpec,
		PostSetbackMm: postSetbackMm,
	})
	if err != nil {
		return SaddleRailSolution{}, err
	}
	u, err := assembly.SeatAxisDirection(effStaDeg)
	if err != nil {
		return SaddleRailSolution{}, err
	}
	v, err := assembly.SaddleRailOffsetDirection(saddleSpec.RailPitchDeg)
	if err != nil {
		return SaddleRailSolution{}, err
	}

	q := zeroPlacement.BRP

	det := u.Cross(v)
	if !isFinite(det) {
		return SaddleRailSolution{}, fmt.Errorf("saddle rail-offset solve overflow for STA %.2f°", effStaDeg)
	}
	if math.Abs(det) < railAxisParallelTol {
		return SaddleRailSolution{}, fmt.Errorf("seat axis (%.2f°) and saddle rails (%.2f°) are parallel or nearly parallel (det=%.3e)", effStaDeg, saddleSpec.RailPitchDeg, det)
	}

	rhs := target.Sub(q)
	if !isFinite(rhs.X) || !isFinite(rhs.Y) {
		return SaddleRailSolution{}, fmt.Errorf("saddle rail-offset solve overflow for target (%.1f, %.1f) mm", target.X, target.Y)
	}
	numD := rhs.Cross(v)
	numR := u.Cross(rhs)
	d := numD / det
	r := numR / det
	if !isFinite(numD) || !isFinite(numR) || !isFinite(d) || !isFinite(r) {
		return SaddleRailSolution{}, fmt.Errorf("saddle rail-offset solve overflow for STA %.2f°", effStaDeg)
	}
	if d < 0 {
		return SaddleRailSolution{}, fmt.Errorf("impossible saddle placement: negative seatpost extension %.2f mm", d)
	}
	targetProj := target.VecFromOrigin().Dot(u)
	if !isFinite(targetProj) {
		return SaddleRailSolution{}, fmt.Errorf("saddle rail-offset solve overflow for target projection")
	}
	if targetProj < 0 {
		return SaddleRailSolution{}, fmt.Errorf("impossible saddle placement: target BRP is behind/below upward seat axis")
	}

	placement, err := assembly.PlaceSaddle(geom2d.Origin(), effStaDeg, assembly.SaddleSetup{
		Saddle:                   saddleSpec,
		BBToClampAlongSeatAxisMm: d,
		PostSetbackMm:            postSetbackMm,
		RailOffsetMm:             r,
	})
	if err != nil {
		return SaddleRailSolution{}, err
	}

	brp := placement.BRP
	if !(brp.Y > 0) {
		return SaddleRailSolution{}, fmt.Errorf("impossible saddle placement: BRP Y=%.2f mm is not above BB", brp.Y)
	}
	resX := math.Abs(brp.X - target.X)
	resY := math.Abs(brp.Y - target.Y)
	if !isFinite(resX) || !isFinite(resY) || resX > brpRadiusResidualTolMm || resY > brpRadiusResidualTolMm {
		return SaddleRailSolution{}, fmt.Errorf(
			"saddle BRP-target verification failed: got (%.9f, %.9f) mm, want (%.9f, %.9f) mm (residual %.3e, %.3e mm)",
			brp.X, brp.Y, target.X, target.Y, resX, resY,
		)
	}

	return SaddleRailSolution{
		Placement: placement,
	}, nil
}

// SolveSaddleForBRPRadius solves the BB-to-clamp distance along the effective
// seat axis that places the saddle's BRP at brpHeightMm radial distance from BB
// using the forward assembly model, executes checked forward assembly at that
// distance, and verifies that the assembled BRP is above BB and matches
// brpHeightMm within 1e-8 mm.
//
// Equation:
// For fixed rear-cockpit inputs, the BRP moves on the line BRP(d) = d*u + q,
// where u is the unit seat-axis direction and q is the BRP displacement from BB
// at zero BB-to-clamp distance along the seat axis. Intersecting this line with
// |BRP| = R yields d = -dot(q, u) + sqrt(R^2 - dot(q, n)^2), selecting the
// upper/positive-extension branch (+sqrt).
func SolveSaddleForBRPRadius(
	saddleSpec model.SaddleSpec,
	effStaDeg model.Deg,
	brpHeightMm, postSetbackMm, railOffsetMm float64,
) (assembly.SaddlePlacement, error) {
	if !isFinite(brpHeightMm) || brpHeightMm <= 0 {
		return assembly.SaddlePlacement{}, fmt.Errorf("requested BRP height must be finite and positive, got %v mm", brpHeightMm)
	}
	zeroPlacement, err := assembly.PlaceSaddle(geom2d.Origin(), effStaDeg, assembly.SaddleSetup{
		Saddle:        saddleSpec,
		PostSetbackMm: postSetbackMm,
		RailOffsetMm:  railOffsetMm,
	})
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	u, err := assembly.SeatAxisDirection(effStaDeg)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}

	q := zeroPlacement.BRP.VecFromOrigin()
	// n = (-u.Y, u.X) is the counter-clockwise unit normal to u.
	perp := q.X*(-u.Y) + q.Y*u.X
	rSq := brpHeightMm * brpHeightMm
	perpSq := perp * perp
	discriminant := rSq - perpSq
	if !isFinite(rSq) || !isFinite(perpSq) || !isFinite(discriminant) {
		return assembly.SaddlePlacement{}, fmt.Errorf("saddle BRP-radius solve overflow for radius %.1f mm", brpHeightMm)
	}
	if discriminant < 0 {
		return assembly.SaddlePlacement{}, fmt.Errorf("impossible saddle placement: STA %.2f°, BRP radius %.1f mm, normal displacement %.1f mm", effStaDeg, brpHeightMm, perp)
	}

	b := q.Dot(u)
	bbToClampAlongSeatAxisMm := -b + math.Sqrt(discriminant)
	if !isFinite(b) || !isFinite(bbToClampAlongSeatAxisMm) {
		return assembly.SaddlePlacement{}, fmt.Errorf("saddle BRP-radius solve overflow for STA %.2f°", effStaDeg)
	}
	if bbToClampAlongSeatAxisMm < 0 {
		return assembly.SaddlePlacement{}, fmt.Errorf("impossible saddle placement: negative seatpost extension %.2f mm", bbToClampAlongSeatAxisMm)
	}

	placement, err := assembly.PlaceSaddle(geom2d.Origin(), effStaDeg, assembly.SaddleSetup{
		Saddle:                   saddleSpec,
		BBToClampAlongSeatAxisMm: bbToClampAlongSeatAxisMm,
		PostSetbackMm:            postSetbackMm,
		RailOffsetMm:             railOffsetMm,
	})
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}

	brp := placement.BRP
	if !(brp.Y > 0) {
		return assembly.SaddlePlacement{}, fmt.Errorf("impossible saddle placement: BRP Y=%.2f mm is not above BB", brp.Y)
	}
	actualRadius := brp.DistanceToOrigin()
	residual := math.Abs(actualRadius - brpHeightMm)
	if !isFinite(actualRadius) || !isFinite(residual) || residual > brpRadiusResidualTolMm {
		return assembly.SaddlePlacement{}, fmt.Errorf("saddle BRP-radius verification failed: got %.9f mm, want %.9f mm (residual %.3e mm)", actualRadius, brpHeightMm, residual)
	}

	return placement, nil
}

// BRPPointFromSaddlePlacement adapts an assembly.SaddlePlacement into a
// legacy model.BRPPoint snapshot for existing consumers.
func BRPPointFromSaddlePlacement(p assembly.SaddlePlacement) model.BRPPoint {
	return model.BRPPoint{
		Saddle:                   p.Saddle,
		SeatpostSetbackMm:        p.PostSetbackMm,
		RailOffsetMm:             p.RailOffsetMm,
		AchievedClampOffsetMm:    p.RailOffsetMm,
		BBToClampAlongSeatAxisMm: p.BBToClampAlongSeatAxisMm,
		Transform: model.SaddleTransform{
			RailCenter:   p.RailCenter,
			RailPitchDeg: p.Saddle.RailPitchDeg,
		},
		SeatpostClamp:     p.SeatpostClamp,
		BrpX:              p.BRP.X,
		BrpY:              p.BRP.Y,
		SitBoneX:          p.SitBones.X,
		SitBoneY:          p.SitBones.Y,
		BrpSetbackMm:      -p.BRP.X,
		SitBoneSetbackMm:  -p.SitBones.X,
		BrpAngleFromBbDeg: model.DegOf(geom2d.AngleFromSlope(-p.BRP.X, p.BRP.Y)),
	}
}

// SolveBRPPoint solves the saddle placement for brpHeightMm and returns a
// legacy model.BRPPoint snapshot.
func SolveBRPPoint(
	s model.SaddleSpec,
	staDeg model.Deg,
	brpHeightMm, postSetbackMm, railOffsetMm float64,
) (model.BRPPoint, error) {
	placement, err := SolveSaddleForBRPRadius(s, staDeg, brpHeightMm, postSetbackMm, railOffsetMm)
	if err != nil {
		return model.BRPPoint{}, err
	}
	return BRPPointFromSaddlePlacement(placement), nil
}

// MapPossibleBRPs generates all candidate BRP points within s's modeled clamp
// window in 1 mm steps, including both endpoints. Returns an error naming the
// failing candidate if any step fails.
func MapPossibleBRPs(
	s model.SaddleSpec,
	staDeg model.Deg,
	brpHeightMm, postSetbackMm, clampLengthMm float64,
) ([]model.BRPPoint, error) {
	if !isFinite(clampLengthMm) || clampLengthMm <= 0 {
		return nil, fmt.Errorf("seatpost clamp length must be finite and positive, got %v mm", clampLengthMm)
	}
	minClamp, maxClamp, ok := s.ClampWindowMm(clampLengthMm)
	if !ok {
		return nil, fmt.Errorf("saddle %q rail length %.1f mm cannot accommodate %.1f mm clamp", s.Name, s.RailLengthMm, clampLengthMm)
	}

	var points []model.BRPPoint
	for r := minClamp; ; {
		pt, err := SolveBRPPoint(s, staDeg, brpHeightMm, postSetbackMm, r)
		if err != nil {
			return nil, fmt.Errorf("candidate rail offset %.2f mm: %w", r, err)
		}
		points = append(points, pt)
		next := min(r+model.SaddleRailStepMm, maxClamp)
		if next <= r {
			break
		}
		r = next
	}
	return points, nil
}
