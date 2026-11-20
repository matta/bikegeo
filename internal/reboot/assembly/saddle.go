package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/reboot/pose"
)

// SaddleSetup specifies a saddle's rail-frame geometry, world-relative rail
// pitch, and fore-aft clamp offset along the rails:
//   - RailCenterToBrpMm: axial distance along the rails (+X toward nose) from RailCenter to BRP.
//   - RailCenterToSitBonesMm: axial offset along the rails from RailCenter to SitBones.
//   - RailToBrpHeightMm: perpendicular offset (+Y in the rail frame, 90° CCW) from the rail axis up to BRP.
//   - SitBonesElevationMm: perpendicular offset (+Y in the rail frame) of SitBones above BRP, so
//     SitBones sits RailToBrpHeightMm + SitBonesElevationMm above the rail axis.
//   - RailLengthMm: straight clamping span of the saddle rails.
//   - RailPitchDeg: saddle rail pitch relative to world horizontal (0° level; positive raises nose CCW).
//   - RailOffsetMm: clamp position along the rails; positive moves the saddle
//     body rearward relative to RailCradle (advancing RailCenter by -RailOffsetMm).
//     It must stay within ±(RailLengthMm − clamp length)/2 so the clamp stays on the rails.
type SaddleSetup struct {
	RailCenterToBrpMm      float64
	RailCenterToSitBonesMm float64
	RailToBrpHeightMm      float64
	SitBonesElevationMm    float64
	RailLengthMm           float64
	RailPitchDeg           pose.Deg
	RailOffsetMm           float64
}

// SaddlePlacement holds the Pose-valued landmarks produced by PlaceSaddle:
//   - RailCradle: incoming clamp attachment pose (in the seatpost frame).
//   - RailFrame: pose at RailCradle re-oriented to world-relative RailPitchDeg.
//   - RailCenter: midpoint of the straight rail clamping section (in the rail frame).
//   - BRP: Biomechanical Reference Point (+RailCenterToBrpMm, +RailToBrpHeightMm) in the rail frame.
//   - SitBones: modeled sit-bone support landmark (RailCenterToSitBonesMm,
//     RailToBrpHeightMm + SitBonesElevationMm) in the rail frame.
//
// RailFrame, RailCenter, BRP, and SitBones all carry the saddle-rail
// construction frame (local +X forward along the rail toward the nose at
// world heading RailPitchDeg; local +Y 90° CCW, perpendicular to the rail).
type SaddlePlacement struct {
	RailCradle pose.Pose
	RailFrame  pose.Pose
	RailCenter pose.Pose
	BRP        pose.Pose
	SitBones   pose.Pose
}

// PlaceSaddle mounts a saddle at railCradle, orients the rails to the
// world-relative RailPitchDeg via Pose.Face, slides the saddle body along the
// rails by -RailOffsetMm to RailCenter, and branches to BRP and SitBones.
// clampLengthMm is the seatpost clamp head's fore-aft footprint; it bounds the
// rail offset.
func PlaceSaddle(railCradle pose.Pose, clampLengthMm float64, setup SaddleSetup) (SaddlePlacement, error) {
	if err := validatePose("railCradle", railCradle); err != nil {
		return SaddlePlacement{}, err
	}
	if !isFinite(setup.RailCenterToBrpMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle rail-center-to-BRP offset must be finite, got %v mm", setup.RailCenterToBrpMm)
	}
	if !isFinite(setup.RailCenterToSitBonesMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle rail-center-to-sit-bones offset must be finite, got %v mm", setup.RailCenterToSitBonesMm)
	}
	if !isFinite(setup.RailToBrpHeightMm) || setup.RailToBrpHeightMm < 0 {
		return SaddlePlacement{}, fmt.Errorf("saddle rail-to-BRP height must be finite and non-negative, got %v mm", setup.RailToBrpHeightMm)
	}
	if !isFinite(setup.SitBonesElevationMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle sit-bones elevation must be finite, got %v mm", setup.SitBonesElevationMm)
	}
	if !isFinite(setup.RailLengthMm) || setup.RailLengthMm <= 0 {
		return SaddlePlacement{}, fmt.Errorf("saddle rail length must be finite and positive, got %v mm", setup.RailLengthMm)
	}
	if !isFinite(float64(setup.RailPitchDeg)) {
		return SaddlePlacement{}, invalidInput(InputSaddleRailPitch, "saddle rail pitch must be finite, got %v°", setup.RailPitchDeg)
	}
	if !isFinite(setup.RailOffsetMm) {
		return SaddlePlacement{}, invalidInput(InputSaddleRailOffset, "saddle rail offset must be finite, got %v mm", setup.RailOffsetMm)
	}

	railLimit, err := railOffsetLimitMm(setup.RailLengthMm, clampLengthMm)
	if err != nil {
		return SaddlePlacement{}, err
	}
	if math.Abs(setup.RailOffsetMm) > railLimit {
		return SaddlePlacement{}, invalidInput(InputSaddleRailOffset, "saddle rail offset must be within ±%v mm, got %v mm", railLimit, setup.RailOffsetMm)
	}

	// Rail pitch is measured against world horizontal, so face it rather than turn.
	railFrame := railCradle.Face(setup.RailPitchDeg)
	// A positive rail offset slides the saddle rearward, against the rail heading.
	railCenter := railFrame.Advance(-setup.RailOffsetMm)
	// Both landmarks are raised in the rail frame, so the height rotates with the rails.
	brp := railCenter.Translate(setup.RailCenterToBrpMm, setup.RailToBrpHeightMm)
	sitBones := railCenter.Translate(setup.RailCenterToSitBonesMm, setup.RailToBrpHeightMm+setup.SitBonesElevationMm)

	if err := validatePlacementPoses("saddle", railCradle, railCenter, brp, sitBones); err != nil {
		return SaddlePlacement{}, err
	}

	return SaddlePlacement{
		RailCradle: railCradle,
		RailFrame:  railFrame,
		RailCenter: railCenter,
		BRP:        brp,
		SitBones:   sitBones,
	}, nil
}

// railOffsetLimitMm returns the symmetric rail offset bound
// (railLengthMm − clampLengthMm)/2: the clamp head must stay on the rails.
func railOffsetLimitMm(railLengthMm, clampLengthMm float64) (float64, error) {
	limit := (railLengthMm - clampLengthMm) / 2
	if !isFinite(limit) || limit < 0 {
		return 0, fmt.Errorf("seatpost clamp length %v mm exceeds saddle rail length %v mm", clampLengthMm, railLengthMm)
	}
	return limit, nil
}
