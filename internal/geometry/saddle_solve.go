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

// SaddleRailSolution is the checked immutable result of solving the saddle rail
// clamp offset and axial seat-axis distance for a target BRP position.
type SaddleRailSolution struct {
	railOffsetMm float64
	placement    assembly.SaddlePlacement
	valid        bool
}

func (s SaddleRailSolution) mustBeValid() {
	if !s.valid {
		panic("geometry: uninitialized or invalid SaddleRailSolution")
	}
	if !isFinite(s.railOffsetMm) {
		panic(fmt.Sprintf("geometry: non-finite SaddleRailSolution rail offset %v", s.railOffsetMm))
	}
	_ = s.placement.BRP()
}

// RailOffsetMm returns the solved saddle rail clamp offset in mm.
func (s SaddleRailSolution) RailOffsetMm() float64 {
	s.mustBeValid()
	return s.railOffsetMm
}

// Placement returns the verified forward-assembled SaddlePlacement.
func (s SaddleRailSolution) Placement() assembly.SaddlePlacement {
	s.mustBeValid()
	return s.placement
}

// SolveSaddleForBRPTarget solves the axial seat-axis distance and saddle rail
// clamp offset that place the saddle's BRP at (-brpSetbackMm, +sqrt(R^2 - setback^2))
// on the upper half of the brpHeightMm radius circle, executes checked forward
// assembly, and verifies both target coordinates within 1e-8 mm.
//
// Equation:
// For fixed rear-cockpit inputs, BRP(d, r) = q + d*u + r*v, where q is the BRP
// at zero axial distance and zero rail offset, u is the checked unit seat-axis
// direction, and v is the checked unit rearward rail-offset direction.
func SolveSaddleForBRPTarget(
	saddleSpec model.SaddleSpec,
	effStaDeg model.Deg,
	brpHeightMm, brpSetbackMm, postSetbackMm float64,
) (SaddleRailSolution, error) {
	if !isFinite(brpHeightMm) || brpHeightMm <= 0 {
		return SaddleRailSolution{}, fmt.Errorf("requested BRP height must be finite and positive, got %v mm", brpHeightMm)
	}
	if !isFinite(brpSetbackMm) {
		return SaddleRailSolution{}, fmt.Errorf("requested BRP setback must be finite, got %v mm", brpSetbackMm)
	}
	axis, err := assembly.NewSeatAxis(geom2d.Origin(), effStaDeg)
	if err != nil {
		return SaddleRailSolution{}, err
	}
	saddle, err := assembly.NewSaddle(saddleSpec)
	if err != nil {
		return SaddleRailSolution{}, err
	}

	rSq := brpHeightMm * brpHeightMm
	xSq := brpSetbackMm * brpSetbackMm
	ySq := rSq - xSq
	if !isFinite(rSq) || !isFinite(xSq) || !isFinite(ySq) {
		return SaddleRailSolution{}, fmt.Errorf("saddle BRP-target solve overflow for radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	if math.Abs(brpSetbackMm) >= brpHeightMm || ySq <= 0 {
		return SaddleRailSolution{}, fmt.Errorf("unreachable upper-circle BRP target: radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	targetY := math.Sqrt(ySq)
	if !isFinite(targetY) || !(targetY > 0) {
		return SaddleRailSolution{}, fmt.Errorf("unreachable upper-circle BRP target: radius %.1f mm, setback %.1f mm", brpHeightMm, brpSetbackMm)
	}
	target := geom2d.Point2{X: -brpSetbackMm, Y: targetY}

	zeroClamp, err := axis.WithSeatpostClamp(0, postSetbackMm)
	if err != nil {
		return SaddleRailSolution{}, err
	}
	zeroPlacement, err := zeroClamp.AttachSaddle(saddle, 0)
	if err != nil {
		return SaddleRailSolution{}, err
	}

	q := zeroPlacement.BRP()
	u := axis.AxisDirection()
	v := saddle.RailOffsetDirection()

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

	clamp, err := axis.WithSeatpostClamp(d, postSetbackMm)
	if err != nil {
		return SaddleRailSolution{}, err
	}
	placement, err := clamp.AttachSaddle(saddle, r)
	if err != nil {
		return SaddleRailSolution{}, err
	}

	brp := placement.BRP()
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
		railOffsetMm: r,
		placement:    placement,
		valid:        true,
	}, nil
}

// SolveSaddleForBRPRadius solves the axial seat-axis distance that places the
// saddle's BRP at brpHeightMm radial distance from BB using the forward assembly
// model, executes checked forward assembly at that distance, and verifies that
// the assembled BRP is above BB and matches brpHeightMm within 1e-8 mm.
//
// Equation:
// For fixed rear-cockpit inputs, the BRP moves on the line BRP(d) = d*u + q,
// where u is the checked unit seat-axis direction and q is the BRP displacement
// from BB at zero axial distance. Intersecting this line with |BRP| = R yields
// d = -dot(q, u) + sqrt(R^2 - dot(q, n)^2), selecting the upper/positive-extension
// branch (+sqrt).
func SolveSaddleForBRPRadius(
	saddleSpec model.SaddleSpec,
	effStaDeg model.Deg,
	brpHeightMm, postSetbackMm, railOffsetMm float64,
) (assembly.SaddlePlacement, error) {
	if !isFinite(brpHeightMm) || brpHeightMm <= 0 {
		return assembly.SaddlePlacement{}, fmt.Errorf("requested BRP height must be finite and positive, got %v mm", brpHeightMm)
	}
	axis, err := assembly.NewSeatAxis(geom2d.Origin(), effStaDeg)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	saddle, err := assembly.NewSaddle(saddleSpec)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	zeroClamp, err := axis.WithSeatpostClamp(0, postSetbackMm)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	zeroPlacement, err := zeroClamp.AttachSaddle(saddle, railOffsetMm)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}

	u := axis.AxisDirection()
	q := zeroPlacement.BRP().VecFromOrigin()
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
	axial := -b + math.Sqrt(discriminant)
	if !isFinite(b) || !isFinite(axial) {
		return assembly.SaddlePlacement{}, fmt.Errorf("saddle BRP-radius solve overflow for STA %.2f°", effStaDeg)
	}
	if axial < 0 {
		return assembly.SaddlePlacement{}, fmt.Errorf("impossible saddle placement: negative seatpost extension %.2f mm", axial)
	}

	clamp, err := axis.WithSeatpostClamp(axial, postSetbackMm)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	placement, err := clamp.AttachSaddle(saddle, railOffsetMm)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}

	brp := placement.BRP()
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

// BRPPointFromSaddlePlacement adapts a checked assembly.SaddlePlacement into a
// legacy model.BRPPoint snapshot for existing consumers.
func BRPPointFromSaddlePlacement(
	s model.SaddleSpec,
	postSetbackMm, railOffsetMm float64,
	p assembly.SaddlePlacement,
) model.BRPPoint {
	brp := p.BRP()
	sit := p.SitBones()
	return model.BRPPoint{
		Saddle:                s,
		SeatpostSetbackMm:     postSetbackMm,
		RailOffsetMm:          railOffsetMm,
		AchievedClampOffsetMm: railOffsetMm,
		SeatAxisDistanceMm:    p.AxialDistanceMm(),
		Transform: model.SaddleTransform{
			RailCenter:   p.RailCenter(),
			RailPitchDeg: s.RailPitchDeg,
		},
		SeatpostClamp:     p.SeatpostClamp(),
		BrpX:              brp.X,
		BrpY:              brp.Y,
		SitBoneX:          sit.X,
		SitBoneY:          sit.Y,
		BrpSetbackMm:      -brp.X,
		SitBoneSetbackMm:  -sit.X,
		BrpAngleFromBbDeg: model.DegOf(geom2d.AngleFromSlope(-brp.X, brp.Y)),
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
	return BRPPointFromSaddlePlacement(s, postSetbackMm, railOffsetMm, placement), nil
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
