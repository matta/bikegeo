package assembly

import (
	"fmt"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// SaddleSetup specifies the saddle geometry, BB-to-clamp distance along the
// effective seat axis, perpendicular seatpost setback, and rail clamp offset:
//   - Saddle: local saddle geometry and world-relative rail pitch.
//   - BBToClampAlongSeatAxisMm: distance from BB along the effective seat axis
//     to the perpendicular projection of the seatpost clamp. The clamp may be
//     offset from that axis by PostSetbackMm, measured along a line
//     perpendicular to the effective seat axis.
//   - PostSetbackMm: perpendicular rearward/downward offset from that point on
//     the effective seat axis to the seatpost clamp.
//   - RailOffsetMm: clamp position along the rails (positive moves the saddle
//     body rearward relative to the front of the bicycle).
type SaddleSetup struct {
	Saddle                   model.SaddleSpec
	BBToClampAlongSeatAxisMm float64
	PostSetbackMm            float64
	RailOffsetMm             float64
}

// SaddlePlacement holds the world coordinates of a placed saddle and its clamp,
// along with copies of the construction settings used to place it.
// Callers must check the error returned by PlaceSaddle before using these fields.
type SaddlePlacement struct {
	Saddle                   model.SaddleSpec // Copy of the saddle specification used for placement.
	BBToClampAlongSeatAxisMm float64          // Distance from BB along the effective seat axis to the perpendicular projection of the seatpost clamp. The clamp may be offset from that axis by PostSetbackMm, measured along a line perpendicular to the effective seat axis.
	PostSetbackMm            float64          // Perpendicular rearward/downward offset from that point on the effective seat axis to SeatpostClamp.
	RailOffsetMm             float64          // Clamp position along the rails (positive moves the saddle body rearward relative to the front of the bicycle).
	SeatpostClamp            geom2d.Point2    // World coordinates of the modeled seatpost clamp center.
	RailCenter               geom2d.Point2    // World coordinates of the midpoint of the saddle's straight rail clamping section.
	BRP                      geom2d.Point2    // World coordinates of the saddle Biomechanical Reference Point (70 mm width mark).
	SitBones                 geom2d.Point2    // World coordinates of the modeled sit-bone support point.
}

func validateEffSTA(effStaDeg model.Deg) error {
	if !isFinite(float64(effStaDeg)) || effStaDeg <= 0 || effStaDeg >= 90 {
		return fmt.Errorf("effective STA %.2f° must be in (0°, 90°)", effStaDeg)
	}
	return nil
}

func validateSaddleSpec(spec model.SaddleSpec) error {
	if !isFinite(spec.RailCenterToBrpMm) {
		return fmt.Errorf("saddle rail-center-to-BRP offset must be finite, got %v mm", spec.RailCenterToBrpMm)
	}
	if !isFinite(spec.RailCenterToSitBonesMm) {
		return fmt.Errorf("saddle rail-center-to-sit-bones offset must be finite, got %v mm", spec.RailCenterToSitBonesMm)
	}
	if !isFinite(spec.SitBonesElevationMm) {
		return fmt.Errorf("saddle sit-bones elevation must be finite, got %v mm", spec.SitBonesElevationMm)
	}
	if !isFinite(float64(spec.RailPitchDeg)) {
		return fmt.Errorf("saddle rail pitch must be finite, got %v°", spec.RailPitchDeg)
	}
	return nil
}

func seatAxisPose(bb geom2d.Point2, effStaDeg model.Deg) pose2 {
	// Upward seat axis is at polar angle (180° - effStaDeg); local +Y is 90° CCW,
	// pointing perpendicular rearward/downward from the seat axis.
	return newPose2(bb, (180.0 - effStaDeg).Angle())
}

func saddleRailPose(clamp geom2d.Point2, railPitchDeg model.Deg) pose2 {
	// Positive rail pitch raises the nose (counter-clockwise in +X/+Y).
	return newPose2(clamp, 0).turn(railPitchDeg.Angle())
}

func placeBRPFromClamp(clamp geom2d.Point2, spec model.SaddleSpec, railOffsetMm float64) (railCenterPose pose2, brp geom2d.Point2, err error) {
	railPose := saddleRailPose(clamp, spec.RailPitchDeg)
	railCenterPose = railPose.advance(-railOffsetMm)
	brp = railCenterPose.point(spec.RailCenterToBrpMm, 0)
	if !isFinitePoint(railCenterPose.origin) || !isFinitePoint(brp) {
		return pose2{}, geom2d.Point2{}, fmt.Errorf("saddle rail placement overflow")
	}
	return railCenterPose, brp, nil
}

// SeatAxisDirection returns the unit vector pointing upward along the effective
// seat axis at effStaDeg in (0°, 90°).
func SeatAxisDirection(effStaDeg model.Deg) (geom2d.Vec2, error) {
	if err := validateEffSTA(effStaDeg); err != nil {
		return geom2d.Vec2{}, err
	}
	return seatAxisPose(geom2d.Origin(), effStaDeg).axisX, nil
}

// SaddleRailOffsetDirection returns the unit vector along which increasing
// RailOffsetMm moves the saddle (rearward along the rails, in world coordinates).
func SaddleRailOffsetDirection(railPitchDeg model.Deg) (geom2d.Vec2, error) {
	if !isFinite(float64(railPitchDeg)) {
		return geom2d.Vec2{}, fmt.Errorf("saddle rail pitch must be finite, got %v°", railPitchDeg)
	}
	p := saddleRailPose(geom2d.Origin(), railPitchDeg)
	return p.axisX.Scale(-1), nil
}

// SaddleClampToBRP returns the displacement from the seatpost clamp to the
// saddle's BRP at railOffsetMm along the rails, in world coordinates.
func SaddleClampToBRP(spec model.SaddleSpec, railOffsetMm float64) (geom2d.Vec2, error) {
	if err := validateSaddleSpec(spec); err != nil {
		return geom2d.Vec2{}, err
	}
	if !isFinite(railOffsetMm) {
		return geom2d.Vec2{}, fmt.Errorf("saddle rail offset must be finite, got %v mm", railOffsetMm)
	}
	_, brp, err := placeBRPFromClamp(geom2d.Origin(), spec, railOffsetMm)
	if err != nil {
		return geom2d.Vec2{}, err
	}
	return brp.VecFromOrigin(), nil
}

// PlaceSaddle places a seatpost clamp and saddle on the leveled frame relative
// to bb at effStaDeg and returns the resulting landmarks and setup facts.
// Callers must check the returned error before using the placement.
func PlaceSaddle(bb geom2d.Point2, effStaDeg model.Deg, setup SaddleSetup) (SaddlePlacement, error) {
	if !isFinitePoint(bb) {
		return SaddlePlacement{}, fmt.Errorf("seat axis origin must be finite, got (%v, %v)", bb.X, bb.Y)
	}
	if err := validateEffSTA(effStaDeg); err != nil {
		return SaddlePlacement{}, err
	}
	if !isFinite(setup.BBToClampAlongSeatAxisMm) || setup.BBToClampAlongSeatAxisMm < 0 {
		return SaddlePlacement{}, fmt.Errorf("BB-to-clamp distance along seat axis must be finite and non-negative, got %v mm", setup.BBToClampAlongSeatAxisMm)
	}
	if !isFinite(setup.PostSetbackMm) || setup.PostSetbackMm < 0 {
		return SaddlePlacement{}, fmt.Errorf("seatpost setback must be finite and non-negative, got %v mm", setup.PostSetbackMm)
	}
	if err := validateSaddleSpec(setup.Saddle); err != nil {
		return SaddlePlacement{}, err
	}
	if !isFinite(setup.RailOffsetMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle rail offset must be finite, got %v mm", setup.RailOffsetMm)
	}

	// Start at BB, oriented along the effective seat axis.
	p := seatAxisPose(bb, effStaDeg)
	// Advance along the effective seat axis to the perpendicular projection of the clamp.
	p = p.advance(setup.BBToClampAlongSeatAxisMm)
	// Move perpendicular rearward/downward by post setback; record projected clamp.
	p = p.translate(0, setup.PostSetbackMm)
	seatpostClamp := p.origin
	if !isFinitePoint(seatpostClamp) {
		return SaddlePlacement{}, fmt.Errorf("seatpost clamp translation overflow")
	}

	// Start a world-horizontal reference at that clamp, turn by saddle rail pitch,
	// move backward along the rails by rail offset, and locate saddle landmarks.
	railCenterPose, brp, err := placeBRPFromClamp(seatpostClamp, setup.Saddle, setup.RailOffsetMm)
	if err != nil {
		return SaddlePlacement{}, err
	}
	sitBones := railCenterPose.point(setup.Saddle.RailCenterToSitBonesMm, setup.Saddle.SitBonesElevationMm)
	if !isFinitePoint(sitBones) {
		return SaddlePlacement{}, fmt.Errorf("saddle sit-bones placement overflow")
	}

	return SaddlePlacement{
		Saddle:                   setup.Saddle,
		BBToClampAlongSeatAxisMm: setup.BBToClampAlongSeatAxisMm,
		PostSetbackMm:            setup.PostSetbackMm,
		RailOffsetMm:             setup.RailOffsetMm,
		SeatpostClamp:            seatpostClamp,
		RailCenter:               railCenterPose.origin,
		BRP:                      brp,
		SitBones:                 sitBones,
	}, nil
}
