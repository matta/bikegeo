package assembly

import "bikegeo/internal/reboot/pose"

// BikeSetup specifies the complete single-bike physical build inputs for PlaceBike.
type BikeSetup struct {
	Frame    FrameSetup
	Fork     ForkSetup
	Wheels   WheelSetup
	Seatpost SeatpostSetup
	Saddle   SaddleSetup
}

// BikePlacement holds the leveled whole-bike assembly alongside the BikeSetup
// that produced it, so callers and renderers always read both landmarks and
// physical dimensions from one consistent build.
type BikePlacement struct {
	Setup    BikeSetup
	Chassis  ChassisPlacement
	Seatpost SeatpostPlacement
	Saddle   SaddlePlacement
}

// SeatAxisToPostTopMm returns the axial distance from BB along the seat tube
// axis to the top of the exposed seatpost shaft (ShaftTop).
func (b BikePlacement) SeatAxisToPostTopMm() float64 {
	return b.Chassis.SeatAxis.DistanceTo(b.Seatpost.ShaftTop)
}

// SaddleHeightMm returns the radial distance from BB to the saddle BRP (|BRP - BB|).
func (b BikePlacement) SaddleHeightMm() float64 {
	return b.Chassis.BB.DistanceTo(b.Saddle.BRP)
}

// PlaceBike constructs the leveled chassis, mounts the seatpost on SeatTubeTop,
// and mounts the saddle on SeatpostClamp in a single traced (or untraced) Pose
// tree rooted at BB.
func PlaceBike(setup BikeSetup, trace *pose.Trace) (BikePlacement, error) {
	chassis, err := PlaceChassis(setup.Frame, setup.Fork, setup.Wheels, trace)
	if err != nil {
		return BikePlacement{}, err
	}
	seatpost, err := PlaceSeatpost(chassis.SeatTubeTop, setup.Seatpost)
	if err != nil {
		return BikePlacement{}, err
	}
	saddle, err := PlaceSaddle(seatpost.SeatpostClamp, setup.Saddle)
	if err != nil {
		return BikePlacement{}, err
	}
	return BikePlacement{
		Setup:    setup,
		Chassis:  chassis,
		Seatpost: seatpost,
		Saddle:   saddle,
	}, nil
}
