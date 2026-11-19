package assembly

import (
	"fmt"

	"bikegeo/internal/reboot/pose"
)

// SeatpostSetup specifies the exposed seatpost dimensions mounted at SeatTubeTop:
//   - ExtensionMm: exposed seatpost length above SeatTubeTop along the seat tube axis.
//   - MinExtensionMm, MaxExtensionMm: installed limits from the post and frame.
//     A zero MaxExtensionMm leaves the upper bound unspecified.
//     TODO: replace the zero-means-unbounded sentinel with an explicit
//     "has max" flag or pointer when Milestone 2 hardware limits arrive.
//   - PostSetbackMm: perpendicular rearward/downward offset from the seat tube
//     axis (+Y in the seat-axis frame, 90° counter-clockwise from heading) to SeatpostClamp.
//   - ClampLengthMm: fore-aft footprint of the seatpost's rail clamp head.
type SeatpostSetup struct {
	ExtensionMm    float64
	MinExtensionMm float64
	MaxExtensionMm float64
	PostSetbackMm  float64
	ClampLengthMm  float64
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
		return SeatpostPlacement{}, invalidInput(InputSeatpostExtension, "seatpost extension must be finite and non-negative, got %v mm", setup.ExtensionMm)
	}
	if !isFinite(setup.MinExtensionMm) || !isFinite(setup.MaxExtensionMm) || setup.MinExtensionMm < 0 ||
		(setup.MaxExtensionMm != 0 && setup.MaxExtensionMm < setup.MinExtensionMm) {
		return SeatpostPlacement{}, fmt.Errorf("invalid seatpost extension limits: %v to %v mm", setup.MinExtensionMm, setup.MaxExtensionMm)
	}
	if setup.ExtensionMm < setup.MinExtensionMm {
		return SeatpostPlacement{}, invalidInput(InputSeatpostExtension, "seatpost extension must be at least %v mm, got %v mm", setup.MinExtensionMm, setup.ExtensionMm)
	}
	if setup.MaxExtensionMm != 0 && setup.ExtensionMm > setup.MaxExtensionMm {
		return SeatpostPlacement{}, invalidInput(InputSeatpostExtension, "seatpost extension must be at most %v mm, got %v mm", setup.MaxExtensionMm, setup.ExtensionMm)
	}
	if !isFinite(setup.PostSetbackMm) || setup.PostSetbackMm < 0 {
		return SeatpostPlacement{}, invalidInput(InputSeatpostSetback, "seatpost setback must be finite and non-negative, got %v mm", setup.PostSetbackMm)
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
