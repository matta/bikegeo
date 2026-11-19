package assembly

import "bikegeo/internal/reboot/pose"

// BikeSetup specifies the complete single-bike physical build inputs for PlaceBike.
type BikeSetup struct {
	Frame     FrameSetup
	Fork      ForkSetup
	Wheels    WheelSetup
	Seatpost  SeatpostSetup
	Saddle    SaddleSetup
	Stem      StemSetup
	Handlebar HandlebarSetup
}

// BikePlacement holds the leveled whole-bike assembly alongside the BikeSetup
// that produced it, so callers and renderers always read both landmarks and
// physical dimensions from one consistent build.
type BikePlacement struct {
	Setup     BikeSetup
	Chassis   ChassisPlacement
	Seatpost  SeatpostPlacement
	Saddle    SaddlePlacement
	Stem      StemPlacement
	Handlebar HandlebarPlacement
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

// BrpToGripDistMm returns the sagittal Euclidean distance from saddle BRP to HandGrip
// (the cockpit reach-distance reference anchor).
func (b BikePlacement) BrpToGripDistMm() float64 {
	return b.Saddle.BRP.DistanceTo(b.Handlebar.HandGrip)
}

// SitBonesToGripRiseMm returns the vertical rise of HandGrip above SitBones.
// Measured from SitBones rather than BRP so rider-fit cockpit elevation carries
// consistently across saddles through the sit-bone contact point.
func (b BikePlacement) SitBonesToGripRiseMm() float64 {
	return b.Saddle.SitBones.VerticalOffsetTo(b.Handlebar.HandGrip)
}

// PlaceBike constructs the leveled chassis, mounts the rear cockpit
// (Seatpost -> Saddle) on SeatTubeTop, and mounts the front cockpit
// (Stem -> Handlebar) on HeadTubeTop in a single traced (or untraced) Pose
// tree rooted at BB. It freezes trace (when non-nil) before returning so
// downstream callers and renderers cannot append steps to construction history.
func PlaceBike(setup BikeSetup, trace *pose.Trace) (BikePlacement, error) {
	defer trace.Freeze()
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
	stem, err := PlaceStem(chassis.HeadTubeTop, setup.Stem)
	if err != nil {
		return BikePlacement{}, err
	}
	handlebar, err := PlaceHandlebar(stem.BarClamp, setup.Handlebar)
	if err != nil {
		return BikePlacement{}, err
	}
	return BikePlacement{
		Setup:     setup,
		Chassis:   chassis,
		Seatpost:  seatpost,
		Saddle:    saddle,
		Stem:      stem,
		Handlebar: handlebar,
	}, nil
}
