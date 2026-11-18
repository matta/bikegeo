package assembly

import (
	"fmt"

	"bikegeo/internal/reboot/pose"
)

// SeatpostSetup specifies the exposed seatpost dimensions mounted at SeatTubeTop:
//   - ExtensionMm: exposed seatpost length above SeatTubeTop along the seat tube axis.
//   - PostSetbackMm: perpendicular rearward/downward offset from the seat tube
//     axis (+Y in the seat-axis frame, 90° counter-clockwise from heading) to SeatpostClamp.
//   - ClampLengthMm: fore-aft footprint of the seatpost's rail clamp head.
type SeatpostSetup struct {
	ExtensionMm   float64
	PostSetbackMm float64
	ClampLengthMm float64
}

// SeatpostPlacement holds the Pose-valued landmarks produced by PlaceSeatpost.
// Each landmark carries the leveled seat-tube axis orientation (local +X
// pointing up and rearward along the seat tube axis; local +Y pointing 90°
// counter-clockwise, perpendicular rearward/downward).
type SeatpostPlacement struct {
	SeatTubeTop   pose.Pose
	ShaftTop      pose.Pose
	SeatpostClamp pose.Pose
}

// PlaceSeatpost mounts a seatpost starting at seatTubeTop (positioned at the
// top of the frame's seat tube and oriented up the seat tube axis).
func PlaceSeatpost(seatTubeTop pose.Pose, setup SeatpostSetup) (SeatpostPlacement, error) {
	if err := validatePose("seatTubeTop", seatTubeTop); err != nil {
		return SeatpostPlacement{}, err
	}
	if !isFinite(setup.ExtensionMm) || setup.ExtensionMm < 0 {
		return SeatpostPlacement{}, fmt.Errorf("seatpost extension must be finite and non-negative, got %v mm", setup.ExtensionMm)
	}
	if !isFinite(setup.PostSetbackMm) || setup.PostSetbackMm < 0 {
		return SeatpostPlacement{}, fmt.Errorf("seatpost setback must be finite and non-negative, got %v mm", setup.PostSetbackMm)
	}
	if !isFinite(setup.ClampLengthMm) || setup.ClampLengthMm <= 0 {
		return SeatpostPlacement{}, fmt.Errorf("seatpost clamp length must be finite and positive, got %v mm", setup.ClampLengthMm)
	}

	shaftTop := seatTubeTop.Advance("seatpost extension", setup.ExtensionMm)
	seatpostClamp := shaftTop.Translate("seatpost setback", 0, setup.PostSetbackMm)

	if err := validatePlacementPoses("seatpost", seatTubeTop, shaftTop, seatpostClamp); err != nil {
		return SeatpostPlacement{}, err
	}

	return SeatpostPlacement{
		SeatTubeTop:   seatTubeTop,
		ShaftTop:      shaftTop,
		SeatpostClamp: seatpostClamp,
	}, nil
}
