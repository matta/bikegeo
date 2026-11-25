package assembly

import (
	"fmt"
	"math"

	"bikegeo/internal/pose"
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
//     body rearward relative to RailPivot (advancing RailCenter by -RailOffsetMm).
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

func (s SaddleSetup) brpOffset() pose.Vec[SaddleRailDir] {
	return pose.Vec[SaddleRailDir]{X: s.RailCenterToBrpMm, Y: s.RailToBrpHeightMm}
}

func (s SaddleSetup) sitBonesOffset() pose.Vec[SaddleRailDir] {
	return pose.Vec[SaddleRailDir]{X: s.RailCenterToSitBonesMm, Y: s.RailToBrpHeightMm + s.SitBonesElevationMm}
}

// SaddlePlacement holds the Pose-valued landmarks produced by PlaceSaddle:
//   - RailPivot: incoming clamp attachment pose (in SeatTubeDir orientation).
//   - RailFrame: pose at RailPivot re-oriented to world-relative RailPitchDeg.
//   - RailCenter: midpoint of the straight rail clamping section (along the rail line).
//   - BRP: Biomechanical Reference Point (+RailCenterToBrpMm, +RailToBrpHeightMm) in RailCenter's local frame.
//   - SitBones: modeled sit-bone support landmark (RailCenterToSitBonesMm,
//     RailToBrpHeightMm + SitBonesElevationMm) in RailCenter's local frame.
//
// RailFrame, RailCenter, BRP, and SitBones all carry SaddleRailDir (local +X
// unit vector forward along the rail toward the nose at world heading
// RailPitchDeg; local +Y unit vector 90° CCW, perpendicular to the rail),
// anchored at each landmark's own Origin().
type SaddlePlacement struct {
	RailPivot  pose.Pose[SeatTubeDir, DynamicWorld]
	RailFrame  pose.Pose[SaddleRailDir, DynamicWorld]
	RailCenter pose.Pose[SaddleRailDir, DynamicWorld]
	BRP        pose.Pose[SaddleRailDir, DynamicWorld]
	SitBones   pose.Pose[SaddleRailDir, DynamicWorld]
}

// PlaceSaddle mounts a saddle at RailPivot, orients the rails to the
// world-relative RailPitchDeg via pose.Face, slides the saddle body along the
// rails by -RailOffsetMm to RailCenter, and branches to BRP and SitBones.
// clampLengthMm is the seatpost clamp head's fore-aft footprint; it bounds the
// rail offset.
func PlaceSaddle(RailPivot pose.Pose[SeatTubeDir, DynamicWorld], clampLengthMm float64, setup SaddleSetup) (SaddlePlacement, error) {
	if err := validatePose("RailPivot", RailPivot); err != nil {
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
	railFrame := pose.Face[SaddleRailDir](RailPivot, pose.Heading[DynamicWorld](setup.RailPitchDeg))
	// A positive rail offset slides the saddle rearward, against the rail heading.
	railCenter := railFrame.Advance(-setup.RailOffsetMm)
	// Both landmarks are raised in the rail frame, so the height rotates with the rails.
	brp := railCenter.Translate(setup.brpOffset())
	sitBones := railCenter.Translate(setup.sitBonesOffset())

	if err := validatePlacementPoints("saddle", RailPivot.Origin(), railCenter.Origin(), brp.Origin(), sitBones.Origin()); err != nil {
		return SaddlePlacement{}, err
	}

	return SaddlePlacement{
		RailPivot:  RailPivot,
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
