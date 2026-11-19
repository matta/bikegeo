package assembly

import (
	"fmt"

	"bikegeo/internal/reboot/pose"
)

// SaddleSetup specifies a saddle's rail-frame geometry, world-relative rail
// pitch, and fore-aft clamp offset along the rails:
//   - RailCenterToBrpMm: axial distance along the rails (+X toward nose) from RailCenter to BRP.
//   - RailCenterToSitBonesMm: axial offset along the rails from RailCenter to SitBones.
//   - SitBonesElevationMm: perpendicular offset (+Y in the rail frame, 90° CCW) from the rail axis to SitBones.
//   - RailLengthMm: straight clamping span of the saddle rails.
//   - RailPitchDeg: saddle rail pitch relative to world horizontal (0° level; positive raises nose CCW).
//   - RailOffsetMm: clamp position along the rails; positive moves the saddle
//     body rearward relative to SeatpostClamp (advancing RailCenter by -RailOffsetMm).
type SaddleSetup struct {
	RailCenterToBrpMm      float64
	RailCenterToSitBonesMm float64
	SitBonesElevationMm    float64
	RailLengthMm           float64
	RailPitchDeg           pose.Deg
	RailOffsetMm           float64
}

// SaddlePlacement holds the Pose-valued landmarks produced by PlaceSaddle:
//   - SeatpostClamp: incoming clamp attachment pose (in the seatpost frame).
//   - RailFrame: pose at SeatpostClamp re-oriented to world-relative RailPitchDeg.
//   - RailCenter: midpoint of the straight rail clamping section (in the rail frame).
//   - BRP: Biomechanical Reference Point (+RailCenterToBrpMm, 0) in the rail frame.
//   - SitBones: modeled sit-bone support landmark (RailCenterToSitBonesMm, +SitBonesElevationMm) in the rail frame.
//
// RailFrame, RailCenter, BRP, and SitBones all carry the saddle-rail
// construction frame (local +X forward along the rail toward the nose at
// world heading RailPitchDeg; local +Y 90° CCW, perpendicular to the rail).
type SaddlePlacement struct {
	SeatpostClamp pose.Pose
	RailFrame     pose.Pose
	RailCenter    pose.Pose
	BRP           pose.Pose
	SitBones      pose.Pose
}

// PlaceSaddle mounts a saddle at seatpostClamp, orients the rails to the
// world-relative RailPitchDeg via Pose.Face, slides the saddle body along the
// rails by -RailOffsetMm to RailCenter, and branches to BRP and SitBones.
func PlaceSaddle(seatpostClamp pose.Pose, setup SaddleSetup) (SaddlePlacement, error) {
	if err := validatePose("seatpostClamp", seatpostClamp); err != nil {
		return SaddlePlacement{}, err
	}
	if !isFinite(setup.RailCenterToBrpMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle rail-center-to-BRP offset must be finite, got %v mm", setup.RailCenterToBrpMm)
	}
	if !isFinite(setup.RailCenterToSitBonesMm) {
		return SaddlePlacement{}, fmt.Errorf("saddle rail-center-to-sit-bones offset must be finite, got %v mm", setup.RailCenterToSitBonesMm)
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

	railFrame := seatpostClamp.Face("saddle rail pitch vs. world horizontal", setup.RailPitchDeg)
	railCenter := railFrame.Advance("saddle rail offset", -setup.RailOffsetMm)
	brp := railCenter.Advance("rail center to BRP", setup.RailCenterToBrpMm)
	sitBones := railCenter.Translate("rail center to sit bones", setup.RailCenterToSitBonesMm, setup.SitBonesElevationMm)

	if err := validatePlacementPoses("saddle", seatpostClamp, railCenter, brp, sitBones); err != nil {
		return SaddlePlacement{}, err
	}

	return SaddlePlacement{
		SeatpostClamp: seatpostClamp,
		RailFrame:     railFrame,
		RailCenter:    railCenter,
		BRP:           brp,
		SitBones:      sitBones,
	}, nil
}
